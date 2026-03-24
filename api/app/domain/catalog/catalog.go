package catalog

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"time"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"

	"github.com/adhocore/gronx"
)

// DisplayNotFoundError is returned when a display configuration is not found
type DisplayNotFoundError struct {
	Key string
}

func (e *DisplayNotFoundError) Error() string {
	return fmt.Sprintf("display not found: %s", e.Key)
}

// PickImageProviderOpts provides optional dependencies for PickImageProvider.
type PickImageProviderOpts struct {
	AIRepo repository.AIRepository
}

func PickImageProvider(now time.Time, epd epaper.DisplayMetadata, repo repository.ImageRepository, imageProviderConfigs ...*config.AssociatedImageProviders) ImageLocator {
	return PickImageProviderWithOpts(now, epd, repo, nil, imageProviderConfigs...)
}

func PickImageProviderWithOpts(now time.Time, epd epaper.DisplayMetadata, repo repository.ImageRepository, opts *PickImageProviderOpts, imageProviderConfigs ...*config.AssociatedImageProviders) ImageLocator {
	errProvider := func(msg string) ImageLocator {
		return &imageErrorMessageProvider{epd, &config.ImageErrorMessageProviderConfig{Message: msg}, nil}
	}

	// Prefer providers matched by a cron expression; fall back to those with no cron if none match.
	var subject []*config.AssociatedImageProviders
	filtered := cronFilter(now, imageProviderConfigs)
	if len(filtered) == 0 {
		subject = nonCronFilter(imageProviderConfigs)
	} else {
		subject = filtered
	}

	if len(subject) == 0 {
		return errProvider("No catalog active at this time (check cron settings)")
	}

	// File providers are weighted by the number of images in the DB.
	// Providers with 0 images are excluded from selection (prevents empty catalog → 404).
	// Non-file providers (HTTP, Lua, etc.) are treated with weight 1.
	type weightedEntry struct {
		conf   *config.AssociatedImageProviders
		weight int64
	}

	var candidates []weightedEntry
	for _, p := range subject {
		if _, ok := p.ProviderConfig.Config.(config.ImageFileProviderConfig); ok {
			count, err := repo.CountByCatalog(p.ProviderConfig.Key, epd.InstalledOrientation())
			if err != nil {
				slog.Warn("CountByCatalog failed, skipping", "catalog", p.ProviderConfig.Key, "err", err)
				continue
			}
			if count == 0 {
				slog.Debug("skipping empty catalog", "catalog", p.ProviderConfig.Key)
				continue
			}
			candidates = append(candidates, weightedEntry{p, count})
		} else {
			candidates = append(candidates, weightedEntry{p, 1})
		}
	}

	if len(candidates) == 0 {
		return errProvider("No images indexed/fetched. Try running: wisp catalog scan")
	}

	// Weighted random selection
	var total int64
	for _, c := range candidates {
		total += c.weight
	}
	r := rand.Int64N(total)
	var imgProviderConfig *config.ImageProviderConfig
	for _, c := range candidates {
		r -= c.weight
		if r < 0 {
			imgProviderConfig = c.conf.ProviderConfig
			break
		}
	}

	return newLocatorFromConfig(now, epd, repo, opts, imgProviderConfig)
}

// newLocatorFromConfig is a factory that creates an ImageLocator based on the type of ImageProviderConfig.
func newLocatorFromConfig(now time.Time, epd epaper.DisplayMetadata, repo repository.ImageRepository, opts *PickImageProviderOpts, cfg *config.ImageProviderConfig) ImageLocator {
	errProvider := func(msg string) ImageLocator {
		return &imageErrorMessageProvider{epd, &config.ImageErrorMessageProviderConfig{Message: msg}, nil}
	}
	switch provConf := cfg.Config.(type) {
	case config.ImageFileProviderConfig:
		return NewImageIndexedFileProvider(now, epd, repo, cfg.Key, provConf)
	case config.ImageHTTPProviderConfig:
		return NewImageHttpProvider(now, epd, repo, cfg.Key, provConf)
	case config.ImagePlaywrightProviderConfig:
		return errProvider("Not implemented yet (playwright)")
	case config.ImageLuaProviderConfig:
		return NewLuaScriptProvider(now, epd, repo, cfg.Key, provConf)
	case config.ImageColorbarProviderConfig:
		return NewColorbarProvider(epd)
	case config.ImageGenerateProviderConfig:
		if opts != nil && opts.AIRepo != nil {
			return NewImageGenerateProvider(cfg.Key, opts.AIRepo)
		}
		return errProvider("Generate provider requires AI repository")
	}
	return errProvider("Image Provider not found")
}

func NewErrorMessageImageProviderConfig(msg string) *config.ImageProviderConfig {
	return &config.ImageProviderConfig{
		Key: "__generated__",
		Config: config.ImageErrorMessageProviderConfig{
			Message: msg,
		},
	}
}


// cronFilter returns only ImageProviders that have a cron expression configured.
func cronFilter(now time.Time, conf []*config.AssociatedImageProviders) []*config.AssociatedImageProviders {
	copied := make([]*config.AssociatedImageProviders, len(conf))
	copy(copied, conf)

	gron := gronx.New()
	filtered := slices.DeleteFunc(copied, func(c *config.AssociatedImageProviders) bool {

		if c == nil || c.ProviderConfig == nil {
			return true
		}

		if c.TimeRange.Cron == "" {
			return true
		}

		// cron expressions are validated at parse time, but filter out invalid values just in case
		if !gron.IsValid(c.TimeRange.Cron) {
			return true
		}

		if due, _ := gron.IsDue(c.TimeRange.Cron, now); due {
			// keep this entry
			return false
		}

		return true
	})

	if len(filtered) == 0 {
		return nil
	}

	return filtered
}

// nonCronFilter returns only ImageProviders that have no cron expression configured.
func nonCronFilter(conf []*config.AssociatedImageProviders) []*config.AssociatedImageProviders {
	copied := make([]*config.AssociatedImageProviders, len(conf))
	copy(copied, conf)

	filtered := slices.DeleteFunc(copied, func(c *config.AssociatedImageProviders) bool {

		if c == nil || c.ProviderConfig == nil {
			return true
		}

		if c.TimeRange.Cron == "" {
			return false
		}

		return true
	})

	if len(filtered) == 0 {
		return nil
	}

	return filtered
}

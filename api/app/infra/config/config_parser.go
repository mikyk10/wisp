package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"
	"github.com/mikyk10/wisp/app/domain/finder"
	"github.com/mikyk10/wisp/app/domain/finder/fs"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/model/config/raw"

	"github.com/Code-Hex/synchro/iso8601"
	"github.com/adhocore/gronx"
	"github.com/caarlos0/env/v10"
	"github.com/mikyk10/wisp/app/domain/display/epaper"

	"gopkg.in/yaml.v2"
)

type defaultConfigLoader struct {
	finder finder.PathFinder
}

func NewDefaultConfigLoader() config.ConfigLoader {
	execName := "wisp" //filepath.Base(os.Args[0])
	return &defaultConfigLoader{
		finder: fs.NewConfigFilePathFinder(fmt.Sprintf("/etc/%s/", execName), fmt.Sprintf("$HOME/.%s", execName), "./config", "."),
	}
}

func NewTestConfigLoader() config.ConfigLoader {
	return &defaultConfigLoader{
		finder: fs.NewConfigFilePathFinder("testdata", "."),
	}
}

func (ldr *defaultConfigLoader) LoadConfig() (*config.GlobalConfig, *config.ServiceConfig, error) {

	conf, rawSvcConfig, err := ldr.loadRawConfig()
	if err != nil {
		return nil, nil, err
	}

	if err := validateGlobalConfig(conf); err != nil {
		return nil, nil, err
	}

	applyGlobalDefaults(conf)

	svcConfig := &config.ServiceConfig{}
	svcConfig.Catalog = make(map[string]*config.ImageProviderConfig)

	for _, v := range rawSvcConfig.Catalog {
		entry := parseCatalogEntry(v)
		if entry == nil {
			slog.Warn("config: skipping unsupported catalog provider", "key", v.Key, "type", v.Type)
			continue
		}
		svcConfig.Catalog[v.Key] = entry
	}

	gron := gronx.New()

	svcConfig.Displays = make(map[string]*config.DisplayConfig)
	for _, v := range rawSvcConfig.Displays {

		if err := validateDisplay(gron, v); err != nil {
			return nil, nil, err
		}

		disp := config.DisplayConfig{
			Name:            v.Name,
			Key:             v.Key,
			ApiVersion:      v.APIVersion,
			DisplayModel:    v.DisplayModel,
			Orientation:     config.NewDisplayOrientation(v.DisplayOrientation),
			Flip:            v.Flip,
			ShowTimestamp:   v.ShowTimestamp,
			Catalog:         make([]*config.AssociatedImageProviders, len(v.AssociatedCatalogEntry)),
			ImageProcessors: make([]*config.ImageProcessorConfig, len(v.ImageProcessors)),
			ColorReduction: config.ColorReduction{
				Type:     v.ColorReduction.Type,
				Size:     v.ColorReduction.Size,
				Strength: v.ColorReduction.Strength,
			},
			SleepDurationSeconds: v.SleepDurationSeconds,
			WakeSchedule:         v.WakeSchedule,
		}

		if disp.SleepDurationSeconds == 0 {
			disp.SleepDurationSeconds = 86400
		}

		cropStrategy := config.CropStrategyCenter
		if v.Crop.Strategy == string(config.CropStrategyExifSubject) {
			cropStrategy = config.CropStrategyExifSubject
		}
		disp.Crop = config.CropConfig{Strategy: cropStrategy}

		for i, cat := range v.AssociatedCatalogEntry {
			provConfig, ok := svcConfig.Catalog[cat.Key]
			if !ok {
				return nil, nil, fmt.Errorf("display[%s].catalog[%d]: unknown catalog key %q", v.Key, i, cat.Key)
			}
			if cat.TimeRange.Cron != "" && !gron.IsValid(cat.TimeRange.Cron) {
				return nil, nil, fmt.Errorf("display[%s].catalog[%d]: invalid cron expression %q", v.Key, i, cat.TimeRange.Cron)
			}
			assoc := &config.AssociatedImageProviders{
				ProviderConfig: provConfig,
				TimeRange: config.CronConfig{
					Cron: cat.TimeRange.Cron,
				},
			}
			if cat.ColorReduction != nil {
				assoc.ColorReduction = &config.ColorReduction{
					Type:     cat.ColorReduction.Type,
					Size:     cat.ColorReduction.Size,
					Strength: cat.ColorReduction.Strength,
				}
			}
			disp.Catalog[i] = assoc
		}

		for i, v := range v.ImageProcessors {
			disp.ImageProcessors[i] = &config.ImageProcessorConfig{
				Type: v.Type,
				Data: v.Properties,
			}
		}

		svcConfig.Displays[v.Key] = &disp
	}

	return conf, svcConfig, nil
}

func parseCatalogEntry(v raw.CatalogEntry) *config.ImageProviderConfig {
	switch v.Type {
	case config.ImageFileProviderType:
		cr := config.Criteria{}
		cr.Include.Path = v.FileConfig.Criteria.Include.Path
		cr.Exclude.Path = v.FileConfig.Criteria.Exclude.Path

		cr.Include.ExifTimeRange = make([]config.TimeRange, len(v.FileConfig.Criteria.Include.TimeRange))
		for i, r := range v.FileConfig.Criteria.Include.TimeRange {
			t, _ := iso8601.ParseDateTime(r.From)
			cr.Include.ExifTimeRange[i].From = t
			t, _ = iso8601.ParseDateTime(r.To)
			cr.Include.ExifTimeRange[i].To = t
			d, _ := time.ParseDuration(r.Last)
			cr.Include.ExifTimeRange[i].Last = d
		}

		cr.Exclude.ExifTimeRange = make([]config.TimeRange, len(v.FileConfig.Criteria.Exclude.TimeRange))
		for i, r := range v.FileConfig.Criteria.Exclude.TimeRange {
			t, _ := iso8601.ParseDateTime(r.From)
			cr.Exclude.ExifTimeRange[i].From = t
			t, _ = iso8601.ParseDateTime(r.To)
			cr.Exclude.ExifTimeRange[i].To = t
			d, _ := time.ParseDuration(r.Last)
			cr.Exclude.ExifTimeRange[i].Last = d
		}

		return &config.ImageProviderConfig{
			Key: v.Key,
			Config: config.ImageFileProviderConfig{
				Criteria: cr,
				SrcPath:  v.FileConfig.SrcPath,
				Hooks: config.FileHooks{
					OnNewFile: v.FileConfig.Hooks.OnNewFile,
				},
			},
		}

	case config.ImageHTTPProviderType:
		httpConf := config.ImageHTTPProviderConfig{
			URL:        v.HTTPConfig.URL,
			Method:     v.HTTPConfig.Method,
			TimeoutSec: v.HTTPConfig.TimeoutSec,
			Headers:    v.HTTPConfig.Headers,
			Cache: config.HTTPCacheConfig{
				Type:       v.HTTPConfig.Cache.Type,
				Depth:      v.HTTPConfig.Cache.Depth,
				EvictCount: v.HTTPConfig.Cache.EvictCount,
			},
		}
		if v.HTTPConfig.ImageSource != nil {
			httpConf.ImageSource = &config.HTTPImageSource{
				Catalogs:    v.HTTPConfig.ImageSource.Catalogs,
				Mode:        v.HTTPConfig.ImageSource.Mode,
				ImageID:     v.HTTPConfig.ImageSource.ImageID,
				Orientation: v.HTTPConfig.ImageSource.Orientation,
				Tags:        v.HTTPConfig.ImageSource.Tags,
			}
		}
		return &config.ImageProviderConfig{
			Key:    v.Key,
			Config: httpConf,
		}

	case config.ImageColorbarProviderType:
		return &config.ImageProviderConfig{
			Key:    v.Key,
			Config: config.ImageColorbarProviderConfig{},
		}

	}

	return nil
}

// validateDisplay rejects a display the rest of the system could not serve.
//
// A wake schedule is checked here, while there is still someone to tell. It
// decides when a panel comes back, so an expression nothing can read would
// leave the device waiting on a moment that never arrives, and a panel that
// stops returning is not something anyone notices quickly.
func validateDisplay(gron *gronx.Gronx, v raw.Display) error {
	if !epaper.IsValidModel(epaper.EPaperDisplayModel(v.DisplayModel)) {
		return fmt.Errorf("display[%s]: unknown model %q", v.Key, v.DisplayModel)
	}

	for i, expr := range v.WakeSchedule {
		if !gron.IsValid(expr) {
			return fmt.Errorf("display[%s].wake_schedule[%d]: invalid cron expression %q", v.Key, i, expr)
		}
	}

	return nil
}

// applyGlobalDefaults fills in the settings a config file is allowed to leave
// out, so that every reader downstream sees a plain value and nobody has to
// remember which zero means "unset" — the same job the SleepDurationSeconds
// default does for a display.
//
// It lives outside LoadConfig only because that function is already at the
// complexity limit the linter allows.
func applyGlobalDefaults(conf *config.GlobalConfig) {
	if conf.DeliveryHistory.Size == 0 {
		conf.DeliveryHistory.Size = config.DefaultDeliveryHistorySize
	}
}

func validateGlobalConfig(conf *config.GlobalConfig) error {
	switch conf.Database.Driver {
	case "sqlite", "mysql", "postgres":
		// valid
	default:
		return fmt.Errorf("invalid database driver: %q (must be sqlite, mysql or postgres)", conf.Database.Driver)
	}

	// 0 is left alone: LoadConfig reads it as "unset" and fills in the default.
	// Anything past the upper bound is refused rather than clamped, because the
	// ring size is the only thing bounding the table and it is paid once per
	// display.
	if s := conf.DeliveryHistory.Size; s < 0 || s > config.MaxDeliveryHistorySize {
		return fmt.Errorf("invalid delivery_history.size: %d (must be between 0 and %d)", s, config.MaxDeliveryHistorySize)
	}

	return nil
}

func (ldr *defaultConfigLoader) loadRawConfig() (*config.GlobalConfig, *raw.ServiceConfig, error) {
	configPath := ldr.finder.Find("config.yaml")
	svcConfPath := ldr.finder.Find("service.yaml")

	b, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, err
	}
	var conf config.GlobalConfig

	if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(b))), &conf); err != nil {
		return nil, nil, err
	}

	c, err := os.ReadFile(svcConfPath)
	if err != nil {
		return nil, nil, err
	}

	var rawServiceConfig raw.ServiceConfig
	if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(c))), &rawServiceConfig); err != nil {
		return nil, nil, err
	}

	// load environment variables; struct fields corresponding to environment variables will be overwritten
	if err := env.Parse(&conf); err != nil {
		return nil, nil, err
	}

	return &conf, &rawServiceConfig, nil
}

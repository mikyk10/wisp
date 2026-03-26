package catalog

import (
	"time"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
)

func NewImageHttpProvider(now time.Time, epd epaper.DisplayMetadata, repo repository.ImageRepository, catalogKey string, config config.ImageHTTPProviderConfig) ImageLocator {
	return &imageHttpProvider{
		now:        now,
		epd:        epd,
		repo:       repo,
		catalogKey: catalogKey,
		config:     config,
	}
}

type imageHttpProvider struct {
	now        time.Time
	epd        epaper.DisplayMetadata
	repo       repository.ImageRepository
	catalogKey string
	config     config.ImageHTTPProviderConfig
}

func (i *imageHttpProvider) Resolve() (ImageLoader, error) {
	if i.config.IsBackground() {
		return i.resolveBackground()
	}
	return i.resolveRealtime()
}

// resolveRealtime returns a loader that fetches from the URL on demand (existing behavior).
func (i *imageHttpProvider) resolveRealtime() (ImageLoader, error) {
	return &imageURLLoader{url: i.config.URL}, nil
}

// resolveBackground selects a random cached image from the DB (like file provider).
func (i *imageHttpProvider) resolveBackground() (ImageLoader, error) {
	img, err := i.repo.FindByRandom(i.catalogKey, i.epd.InstalledOrientation())
	if err != nil {
		return nil, err
	}
	return &imageDBLoader{
		id:   img.ID,
		url:  img.Src,
		repo: i.repo,
	}, nil
}

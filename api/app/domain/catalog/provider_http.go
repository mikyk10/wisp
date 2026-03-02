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
	//TODO: error response is not handled
	/*nfProviderFunc := func(msg string) ImageLocator {
		return &imageErrorMessageProvider{i.epd, &config.ImageErrorMessageProviderConfig{
			Message: msg,
		}}
	}*/

	return &imageURLLoader{url: i.config.URL}, nil
}

package catalog

import (
	"time"
	"wspf/app/domain/display/epaper"
	"wspf/app/domain/model/config"
	"wspf/app/domain/repository"
)

func NewLuaScriptProvider(now time.Time, epd epaper.DisplayMetadata, repo repository.ImageRepository, catalogKey string, script config.ImageLuaProviderConfig) ImageLocator {
	return &imageLuaScriptProvider{
		now:        now,
		epd:        epd,
		repo:       repo,
		catalogKey: catalogKey,
		script:     script,
	}
}

type imageLuaScriptProvider struct {
	now        time.Time
	epd        epaper.DisplayMetadata
	repo       repository.ImageRepository
	catalogKey string
	script     config.ImageLuaProviderConfig
}

func (i *imageLuaScriptProvider) Resolve() (ImageLoader, error) {
	nfProviderFunc := func(msg string) ImageLocator {
		return &imageErrorMessageProvider{i.epd, &config.ImageErrorMessageProviderConfig{
			Message: msg,
		}}
	}

	return nfProviderFunc("WIP").Resolve()

	/*return &imageLocalFilePointer{
		&imageLoader{},
		selectedImage.Src,
		i.epd,
	}, nil*/
}

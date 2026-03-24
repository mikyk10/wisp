package catalog

import (
	"bytes"
	"image"
	"image/png"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
)

// imageGenerateProvider picks a random cached generated image.
type imageGenerateProvider struct {
	catalogKey string
	aiRepo     repository.AIRepository
}

// NewImageGenerateProvider creates a provider that serves cached generated images.
func NewImageGenerateProvider(catalogKey string, aiRepo repository.AIRepository) ImageLocator {
	return &imageGenerateProvider{catalogKey: catalogKey, aiRepo: aiRepo}
}

func (p *imageGenerateProvider) Resolve() (ImageLoader, error) {
	entry, err := p.aiRepo.FindRandomCacheEntry(p.catalogKey)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}
	return &generatedImageLoader{entry: entry}, nil
}

type generatedImageLoader struct {
	entry *model.GenerationCacheEntry
}

func (l *generatedImageLoader) Load() (image.Image, *model.ImgMeta, error) {
	img, err := png.Decode(bytes.NewReader(l.entry.ImageData))
	if err != nil {
		// Try generic decode
		img, _, err = image.Decode(bytes.NewReader(l.entry.ImageData))
		if err != nil {
			return nil, nil, err
		}
	}
	meta := &model.ImgMeta{}
	return img, meta, nil
}

func (l *generatedImageLoader) GetSourcePath() string {
	return ""
}

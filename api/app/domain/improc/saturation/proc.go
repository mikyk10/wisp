package saturation

import (
	"context"
	"image"
	"strconv"
	"wspf/app/domain/improc"
	"wspf/app/domain/model"

	"github.com/anthonynsimon/bild/adjust"
)

type processor struct {
	value float64
}

func NewImageSaturation(data map[string]string) improc.ImageProcessor {
	value, _ := strconv.ParseFloat(data["value"], 64)
	return &processor{
		value: value,
	}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {
	return adjust.Saturation(src, p.value), meta
}

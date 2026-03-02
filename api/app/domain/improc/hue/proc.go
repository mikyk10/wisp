package hue

import (
	"context"
	"image"
	"strconv"
	"wspf/app/domain/improc"
	"wspf/app/domain/model"

	"github.com/anthonynsimon/bild/adjust"
)

type processor struct {
	value int
}

func NewImageHue(data map[string]string) improc.ImageProcessor {
	value, _ := strconv.Atoi(data["value"])
	return &processor{
		value: value,
	}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {
	return adjust.Hue(src, p.value), meta
}

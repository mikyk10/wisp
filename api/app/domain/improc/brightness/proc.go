package brightness

import (
	"context"
	"image"
	"strconv"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/model"

	"github.com/anthonynsimon/bild/adjust"
)

type processor struct {
	value float64
}

func NewImageBrightness(data map[string]string) improc.ImageProcessor {
	value, _ := strconv.ParseFloat(data["value"], 64)
	return &processor{
		value: value,
	}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {
	dst := adjust.Brightness(src, p.value)
	return dst, meta
}

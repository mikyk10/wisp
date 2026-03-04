package gamma

import (
	"context"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/model"
	"image"
	"strconv"

	"github.com/anthonynsimon/bild/adjust"
)

type processor struct {
	value float64
}

func NewImageGamma(data map[string]string) improc.ImageProcessor {
	value, _ := strconv.ParseFloat(data["value"], 64)
	return &processor{
		value: value,
	}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {
	dst := adjust.Gamma(src, p.value)
	return dst, meta
}

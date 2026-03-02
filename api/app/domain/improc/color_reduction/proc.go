package color_reduction

import (
	"context"
	"image"
	"image/color"
	"maps"
	"slices"
	"wspf/app/domain/display/epaper"
	"wspf/app/domain/improc"
	"wspf/app/domain/model"
	"wspf/app/domain/model/config"

	"github.com/makeworld-the-better-one/dither"
)

type processor struct {
	palette  []color.Color
	ditherer *dither.Ditherer
}

func NewImageColorReduction(epd epaper.DisplayMetadata, algorithm config.ColorReduction) improc.ImageProcessor {

	palette := slices.Collect(maps.Values(epd.Palette()))
	ditherer := dither.NewDitherer(palette)

	switch algorithm.Type {
	case config.ColorReductionTypeBayer:
		ditherer.Mapper = dither.Bayer(algorithm.Size, algorithm.Size, algorithm.Strength)
	case config.ColorReductionTypeSierra3:
		ditherer.Matrix = dither.Sierra3
	case config.ColorReductionTypeFloydSteinberg:
		ditherer.Matrix = dither.FloydSteinberg
	default:
		ditherer.Mapper = dither.Bayer(4, 4, 1.0)
	}

	return &processor{
		palette:  palette,
		ditherer: ditherer,
	}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {

	if meta.SkipColorReduction {
		return src, meta
	}

	return p.ditherer.DitherCopy(src), meta
}

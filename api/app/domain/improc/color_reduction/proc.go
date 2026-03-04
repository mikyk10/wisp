package color_reduction

import (
	"context"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"image"
	"image/color"
	"maps"
	"slices"

	"github.com/makeworld-the-better-one/dither"
)

type processor struct {
	palette         []color.Color
	ditherer        *dither.Ditherer
	skipDithering   bool
}

func NewImageColorReduction(epd epaper.DisplayMetadata, algorithm config.ColorReduction) improc.ImageProcessor {

	palette := slices.Collect(maps.Values(epd.Palette()))
	skipDithering := algorithm.Type == config.ColorReductionTypeSimple

	var ditherer *dither.Ditherer
	if !skipDithering {
		ditherer = dither.NewDitherer(palette)
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
	}

	return &processor{
		palette:       palette,
		ditherer:      ditherer,
		skipDithering: skipDithering,
	}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {

	if p.skipDithering {
		return p.simpleQuantize(src), meta
	}
	return p.ditherer.DitherCopy(src), meta
}

// simpleQuantize performs nearest-neighbor color quantization without dithering
func (p *processor) simpleQuantize(src image.Image) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			// Convert from 16-bit to 8-bit
			r, g, b = r>>8, g>>8, b>>8

			// Find nearest color in palette
			nearest := p.palette[0]
			minDist := colorDistance(color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a >> 8)}, nearest) //nolint:gosec // r,g,b are >>8 so 0-255; a>>8 is also 0-255

			for _, c := range p.palette[1:] {
				dist := colorDistance(color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a >> 8)}, c) //nolint:gosec
				if dist < minDist {
					minDist = dist
					nearest = c
				}
			}

			dst.SetRGBA(x, y, color.RGBAModel.Convert(nearest).(color.RGBA)) //nolint:forcetypeassert // RGBAModel.Convert always returns color.RGBA
		}
	}

	return dst
}

// colorDistance computes Euclidean distance between two colors
func colorDistance(c1, c2 color.Color) uint32 {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()

	rd := int32(r1>>8) - int32(r2>>8)
	gd := int32(g1>>8) - int32(g2>>8)
	bd := int32(b1>>8) - int32(b2>>8)
	ad := int32(a1>>8) - int32(a2>>8)

	return uint32(rd*rd + gd*gd + bd*bd + ad*ad) //nolint:gosec // max diff per channel is 255, sum of squares fits in uint32
}

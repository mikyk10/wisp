package selective_color

import (
	"context"
	"image"
	"image/color"
	"math"
	"strconv"

	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/model"
)

type processor struct {
	hueCenter float64 // 0-360
	hueRange  float64 // degrees from center
}

// NewSelectiveColor creates a Selective Color processor.
// Config keys:
//   - "hue_center": center hue to preserve in degrees (0-360). Default: 0 (red).
//   - "hue_range": range from center in degrees. Default: 30.
func NewSelectiveColor(data map[string]string) improc.ImageProcessor {
	hueCenter := 0.0
	if v, err := strconv.ParseFloat(data["hue_center"], 64); err == nil {
		hueCenter = math.Mod(v, 360)
		if hueCenter < 0 {
			hueCenter += 360
		}
	}

	hueRange := 30.0
	if v, err := strconv.ParseFloat(data["hue_range"], 64); err == nil {
		hueRange = math.Abs(v)
	}

	return &processor{
		hueCenter: hueCenter,
		hueRange:  hueRange,
	}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			r8 := uint8((r >> 8) & 0xff)
			g8 := uint8((g >> 8) & 0xff)
			b8 := uint8((b >> 8) & 0xff)
			a8 := uint8((a >> 8) & 0xff)

			h, s, _ := rgbToHSL(r8, g8, b8)

			if s > 0.1 && p.isInHueRange(h) {
				// Keep original color
				dst.SetRGBA(x, y, color.RGBA{R: r8, G: g8, B: b8, A: a8})
			} else {
				// Convert to grayscale (luminance)
				gray := uint8(0.299*float64(r8) + 0.587*float64(g8) + 0.114*float64(b8))
				dst.SetRGBA(x, y, color.RGBA{R: gray, G: gray, B: gray, A: a8})
			}
		}
	}

	return dst, meta
}

func (p *processor) isInHueRange(h float64) bool {
	diff := math.Abs(h - p.hueCenter)
	if diff > 180 {
		diff = 360 - diff
	}
	return diff <= p.hueRange
}

// rgbToHSL converts RGB (0-255) to HSL where H is 0-360, S and L are 0-1.
func rgbToHSL(r, g, b uint8) (h, s, l float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	cmax := math.Max(rf, math.Max(gf, bf))
	cmin := math.Min(rf, math.Min(gf, bf))
	l = (cmax + cmin) / 2.0

	if cmax == cmin {
		return 0, 0, l
	}

	d := cmax - cmin
	if l > 0.5 {
		s = d / (2.0 - cmax - cmin)
	} else {
		s = d / (cmax + cmin)
	}

	switch cmax {
	case rf:
		h = (gf - bf) / d
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/d + 2
	case bf:
		h = (rf-gf)/d + 4
	}
	h *= 60

	return h, s, l
}

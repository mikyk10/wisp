package saturation

import (
	"context"
	"image"
	"log/slog"
	"math"
	"strconv"

	"github.com/anthonynsimon/bild/adjust"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/model"
)

// defaultAutoTarget is the target mean saturation (0–1 scale).
// Typical vibrant images have mean saturation around 0.35–0.45.
const defaultAutoTarget = 0.4

type processor struct {
	value  float64
	auto   bool
	target float64
}

func NewImageSaturation(data map[string]string) improc.ImageProcessor {
	if data["value"] == "auto" {
		target := defaultAutoTarget
		if v, err := strconv.ParseFloat(data["target"], 64); err == nil && v > 0 && v <= 1 {
			target = v
		}
		return &processor{auto: true, target: target}
	}
	value, _ := strconv.ParseFloat(data["value"], 64)
	return &processor{value: value}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {
	v := p.value
	if p.auto {
		v = autoSaturationOffset(src, p.target)
		slog.Debug("auto saturation", "mean_sat", p.target-v, "target", p.target, "offset", v)
	}
	if v == 0 {
		return src, meta
	}
	return adjust.Saturation(src, v), meta
}

// autoSaturationOffset measures the mean saturation (HSL model) and returns
// an adjustment value to bring it toward the target.
// adjust.Saturation expects a change value (positive = more saturated).
func autoSaturationOffset(img image.Image, target float64) float64 {
	bounds := img.Bounds()
	var sum float64
	n := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			rf := float64(r) / 65535.0
			gf := float64(g) / 65535.0
			bf := float64(b) / 65535.0
			sum += hslSaturation(rf, gf, bf)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	mean := sum / float64(n)

	// Scale the gap; adjust.Saturation multiplies, so larger offset for larger gap
	offset := (target - mean) * 2.0

	if offset < -0.5 {
		return -0.5
	}
	if offset > 0.5 {
		return 0.5
	}
	return offset
}

// hslSaturation computes the HSL saturation for an RGB triplet in [0, 1].
func hslSaturation(r, g, b float64) float64 {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	d := max - min
	if d == 0 {
		return 0
	}
	l := (max + min) / 2.0
	if l <= 0.5 {
		return d / (max + min)
	}
	return d / (2.0 - max - min)
}

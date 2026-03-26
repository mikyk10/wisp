package brightness

import (
	"context"
	"image"
	"log/slog"
	"strconv"

	"github.com/anthonynsimon/bild/adjust"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/model"
)

const defaultAutoTarget = 0.7

type processor struct {
	value  float64
	auto   bool
	target float64
}

func NewImageBrightness(data map[string]string) improc.ImageProcessor {
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
		v = meanBrightnessOffset(src, p.target)
		slog.Debug("auto brightness", "mean_luminance", p.target-v, "target", p.target, "offset", v)
	}
	if v == 0 {
		return src, meta
	}
	return adjust.Brightness(src, v), meta
}

// meanBrightnessOffset computes the value to pass to adjust.Brightness so that
// the image's mean luminance is brought toward the target.
// adjust.Brightness expects a value in roughly [-1, 1].
func meanBrightnessOffset(img image.Image, target float64) float64 {
	bounds := img.Bounds()
	var sum float64
	n := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			lum := 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
			sum += lum / 65535.0
			n++
		}
	}
	if n == 0 {
		return 0
	}
	mean := sum / float64(n)
	return target - mean
}

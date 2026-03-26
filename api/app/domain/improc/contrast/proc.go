package contrast

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

// defaultAutoTarget is the target standard deviation of luminance (0–1 scale).
// Typical well-contrasted images have stddev around 0.20–0.25.
const defaultAutoTarget = 0.22

type processor struct {
	value  float64
	auto   bool
	target float64
}

func NewImageContrast(data map[string]string) improc.ImageProcessor {
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
		v = autoContrastOffset(src, p.target)
		slog.Debug("auto contrast", "stddev", p.target-v, "target", p.target, "offset", v)
	}
	if v == 0 {
		return src, meta
	}
	return adjust.Contrast(src, v), meta
}

// autoContrastOffset measures the standard deviation of luminance and returns
// a contrast adjustment value. Low stddev means flat/washed-out image that
// needs a positive boost; high stddev means already punchy.
// adjust.Contrast expects a value in roughly [-1, 1].
func autoContrastOffset(img image.Image, target float64) float64 {
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

	var variance float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			lum := (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 65535.0
			d := lum - mean
			variance += d * d
			n++
		}
	}
	stddev := math.Sqrt(variance / float64(n))

	// Scale the gap to a reasonable contrast adjustment value.
	// (target - stddev) is positive when image is flat, negative when punchy.
	offset := (target - stddev) * 2.0

	// Clamp to avoid extreme adjustments
	if offset < -0.5 {
		return -0.5
	}
	if offset > 0.5 {
		return 0.5
	}
	return offset
}

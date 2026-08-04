package crop

import (
	"image"
	"image/color"
	"testing"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
)

// The two halves of Apply have very different cost profiles, so they are
// measured separately here rather than through the exported entry point.
// crop() carries the physical rotation; resize() carries the resampling.

// BenchmarkCropStage measures the orientation correction plus the crop at full
// resolution. The correction angle is derived from the display and image
// orientations exactly as in production.
func BenchmarkCropStage(b *testing.B) {
	cases := []struct {
		name      string
		installed model.CanonicalOrientation
		imageOri  model.CanonicalOrientation
	}{
		// native landscape == installed landscape, image already landscape
		{"angle0", model.ImgCanonicalOrientationLandscape, model.ImgCanonicalOrientationLandscape},
		// native landscape != installed portrait, image already portrait
		{"angle-90", model.ImgCanonicalOrientationPortrait, model.ImgCanonicalOrientationPortrait},
		// native landscape == installed landscape, image is portrait
		{"angle+90", model.ImgCanonicalOrientationLandscape, model.ImgCanonicalOrientationPortrait},
	}

	for _, tt := range cases {
		p := &processor{
			epd:      epaper.NewWS7in3E(tt.installed),
			strategy: config.CropStrategyCenter,
		}

		b.Run(tt.name+"/rgba", func(b *testing.B) {
			src := benchRGBA(4000, 3000)
			b.ReportAllocs()
			for b.Loop() {
				p.crop(src, &model.ImgMeta{ImageOrientation: tt.imageOri})
			}
		})

		// The JPEG decoder hands the pipeline a YCbCr frame. bild converts it
		// to RGBA up front, which it does even for a zero-degree rotation.
		b.Run(tt.name+"/ycbcr", func(b *testing.B) {
			src := benchYCbCr(4000, 3000)
			b.ReportAllocs()
			for b.Loop() {
				p.crop(src, &model.ImgMeta{ImageOrientation: tt.imageOri})
			}
		})
	}
}

// BenchmarkResizeStage measures the Lanczos resampling down to panel size.
// The input dimensions are what crop() produces from a 12MP 4:3 source on a
// 800x480 panel.
func BenchmarkResizeStage(b *testing.B) {
	p := &processor{
		epd:      epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape),
		strategy: config.CropStrategyCenter,
	}

	src := benchRGBA(4000, 2400)

	b.ReportAllocs()
	for b.Loop() {
		p.resize(src)
	}
}

func benchRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8(x ^ y), A: 0xff})
		}
	}
	return img
}

func benchYCbCr(w, h int) *image.YCbCr {
	img := image.NewYCbCr(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	for i := range img.Y {
		img.Y[i] = uint8(i)
	}
	for i := range img.Cb {
		img.Cb[i] = uint8(i >> 1)
		img.Cr[i] = uint8(i >> 2)
	}
	return img
}

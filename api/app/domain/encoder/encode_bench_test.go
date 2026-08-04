package encoder_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/encoder"
	"github.com/mikyk10/wisp/app/domain/model"
)

// BenchmarkEncode measures packing the finished image into the panel's wire
// format, the last thing a delivery request does.
func BenchmarkEncode(b *testing.B) {
	panels := []struct {
		name    string
		display epaper.DisplayMetadata
	}{
		{"ws7in3e_800x480", epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape)},
		{"ws13in3e_1200x1600", epaper.NewWS13in3E(model.ImgCanonicalOrientationPortrait)},
	}

	for _, p := range panels {
		b.Run(p.name, func(b *testing.B) {
			enc := encoder.NewWaveshareEPEncoder(p.display)
			src := palettedImage(p.display)

			b.ReportAllocs()
			for b.Loop() {
				if _, err := enc.Encode(src); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// palettedImage builds an image already reduced to the panel's palette, which
// is what the encoder receives from the colour-reduction stage.
func palettedImage(display epaper.DisplayMetadata) image.Image {
	palette := make([]color.Color, 0, len(display.Palette()))
	for _, c := range display.Palette() {
		palette = append(palette, c)
	}

	img := image.NewRGBA(image.Rect(0, 0, display.Width(), display.Height()))
	for y := 0; y < display.Height(); y++ {
		for x := 0; x < display.Width(); x++ {
			img.Set(x, y, palette[(x+y)%len(palette)])
		}
	}
	return img
}

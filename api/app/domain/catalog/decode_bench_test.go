package catalog

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkDecodeJPEG measures the cost of turning a photograph on disk into
// pixels, which is what every delivery request pays before the image pipeline
// starts. The sizes are what a phone camera produces.
func BenchmarkDecodeJPEG(b *testing.B) {
	sizes := []struct {
		name string
		w, h int
	}{
		{"12MP", 4000, 3000},
		{"48MP", 8000, 6000},
	}

	for _, s := range sizes {
		b.Run(s.name, func(b *testing.B) {
			path := writeJPEG(b, s.w, s.h)

			b.ReportAllocs()
			for b.Loop() {
				f, err := os.Open(path)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := jpeg.Decode(f); err != nil {
					b.Fatal(err)
				}
				f.Close()
			}
		})
	}
}

// writeJPEG produces a photograph-like file: smooth gradients compress the way
// a real image does, so the entropy decoder does a representative amount of
// work.
func writeJPEG(b *testing.B, w, h int) string {
	b.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 255 / w),
				G: uint8(y * 255 / h),
				B: uint8((x + y) * 255 / (w + h)),
				A: 0xff,
			})
		}
	}

	path := filepath.Join(b.TempDir(), "bench.jpg")
	f, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		b.Fatal(err)
	}
	return path
}

package improc_test

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/improc/color_reduction"
	"github.com/mikyk10/wisp/app/domain/improc/crop"
	"github.com/mikyk10/wisp/app/domain/improc/exif_rotation"
	"github.com/mikyk10/wisp/app/domain/improc/rotation"
	"github.com/mikyk10/wisp/app/domain/improc/timestamp"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
)

// Benchmarks for the e-paper delivery path (GET /pf/:displayKey/image/random.*).
// Sizes mirror production: a 12MP source photo rendered onto a 7.3" panel.
const (
	benchSrcW = 4000
	benchSrcH = 3000
)

// BenchmarkExifRotation measures the EXIF normalisation stage at full
// resolution. Orientation 5 is the worst case: it is implemented as a rotation
// followed by a flip, so the image is walked twice.
func BenchmarkExifRotation(b *testing.B) {
	cases := []struct {
		name        string
		orientation model.ExifOrientation
	}{
		{"exif1_noop", 1},
		{"exif3_180", 3},
		{"exif5_two_pass", 5},
		{"exif6_90cw", 6},
	}

	for _, tt := range cases {
		b.Run(tt.name+"/rgba", func(b *testing.B) {
			src := benchRGBA(benchSrcW, benchSrcH)
			benchApply(b, exif_rotation.NewExifRotation(), src, func() *model.ImgMeta {
				return &model.ImgMeta{ExifOrientation: tt.orientation}
			})
		})

		// A decoded JPEG arrives as YCbCr, not RGBA. The rotation library
		// converts the whole frame to RGBA before it does anything else, so the
		// conversion cost belongs in the measurement.
		b.Run(tt.name+"/ycbcr", func(b *testing.B) {
			src := benchYCbCr(benchSrcW, benchSrcH)
			benchApply(b, exif_rotation.NewExifRotation(), src, func() *model.ImgMeta {
				return &model.ImgMeta{ExifOrientation: tt.orientation}
			})
		})
	}
}

// BenchmarkCropApply measures the display-orientation correction, the crop and
// the resize as one stage, which is how the pipeline invokes it. The angle
// column is the correction crop has to apply: 0 when the source already
// matches the installed orientation, +/-90 when it does not.
func BenchmarkCropApply(b *testing.B) {
	display := epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape)

	cases := []struct {
		name        string
		orientation model.CanonicalOrientation
	}{
		{"angle0", model.ImgCanonicalOrientationLandscape},
		{"angle90", model.ImgCanonicalOrientationPortrait},
	}

	for _, tt := range cases {
		b.Run(tt.name+"/rgba", func(b *testing.B) {
			src := benchRGBA(benchSrcW, benchSrcH)
			benchApply(b, crop.NewImageCropper(display, config.CropStrategyCenter), src, func() *model.ImgMeta {
				return &model.ImgMeta{ImageOrientation: tt.orientation}
			})
		})

		b.Run(tt.name+"/ycbcr", func(b *testing.B) {
			src := benchYCbCr(benchSrcW, benchSrcH)
			benchApply(b, crop.NewImageCropper(display, config.CropStrategyCenter), src, func() *model.ImgMeta {
				return &model.ImgMeta{ImageOrientation: tt.orientation}
			})
		})
	}
}

// BenchmarkTimestamp measures the date badge stage at display resolution. A
// non-zero correction angle makes it rotate the whole frame twice.
func BenchmarkTimestamp(b *testing.B) {
	shot := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	for _, angle := range []float64{0, 90} {
		b.Run(benchAngleName(angle), func(b *testing.B) {
			src := benchRGBA(800, 480)
			benchApply(b, timestamp.NewTimstamp(), src, func() *model.ImgMeta {
				return &model.ImgMeta{ExifDateTime: shot, RequiredCorrectionAngle: angle}
			})
		})
	}
}

// BenchmarkRotation measures the optional 180-degree flip applied when the
// panel is mounted upside down.
func BenchmarkRotation(b *testing.B) {
	src := benchRGBA(800, 480)
	benchApply(b, rotation.NewRotation(), src, func() *model.ImgMeta { return &model.ImgMeta{} })
}

// BenchmarkDeliveryPipeline measures everything a single image request does
// after decoding: EXIF normalisation, orientation correction, crop, resize,
// dithering, timestamp and flip. Orientation 6 on a portrait installation is
// the configuration that gives every stage something to do.
//
// The two variants differ only in whether the EXIF normalisation is folded
// into crop's rotation or performed as its own pass. Measuring them together
// keeps the comparison honest: a machine busy enough to slow one down slows
// the other down with it.
func BenchmarkDeliveryPipeline(b *testing.B) {
	variants := []struct {
		name string
		exif improc.ImageProcessor
	}{
		{"fused", exif_rotation.NewDeferredExifRotation()},
		{"separate", exif_rotation.NewExifRotation()},
	}

	for _, v := range variants {
		b.Run(v.name, func(b *testing.B) {
			display := epaper.NewWS7in3E(model.ImgCanonicalOrientationPortrait)
			shot := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

			group := improc.NewSequencerGroup()

			pre := improc.NewSequencer()
			group.Push(pre)
			pre.Push(v.exif)
			pre.Push(crop.NewImageCropper(display, config.CropStrategyCenter))

			post := improc.NewSequencer()
			group.Push(post)
			post.Push(color_reduction.NewImageColorReduction(display, config.ColorReduction{
				Type: config.ColorReductionTypeBayer, Size: 4, Strength: 1.0,
			}))
			post.Push(timestamp.NewTimstamp())
			post.Push(rotation.NewRotation())

			src := benchYCbCr(benchSrcW, benchSrcH)
			ctx := context.Background()

			b.ReportAllocs()
			for b.Loop() {
				meta := &model.ImgMeta{ExifOrientation: 6, ExifDateTime: shot}
				group.Apply(ctx, src, meta)
			}
		})
	}
}

func benchApply(b *testing.B, proc improc.ImageProcessor, src image.Image, meta func() *model.ImgMeta) {
	b.Helper()
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		proc.Apply(ctx, src, meta())
	}
}

func benchAngleName(angle float64) string {
	if angle == 0 {
		return "angle0"
	}
	return "angle90"
}

// benchRGBA builds an opaque RGBA source. This is the shape the pipeline sees
// once any stage has already normalised the image.
func benchRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8(x ^ y), A: 0xff})
		}
	}
	return img
}

// benchYCbCr builds a 4:2:0 source, which is what a decoded JPEG looks like
// and therefore what the first stage of the pipeline actually receives.
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

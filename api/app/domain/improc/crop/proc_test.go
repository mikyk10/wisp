package crop_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/improc/crop"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/stretchr/testify/assert"
)

func TestImageCropper_OutputSize(t *testing.T) {
	tests := []struct {
		name           string
		display        epaper.DisplayMetadata
		inputWidth     int
		inputHeight    int
		expectedWidth  int
		expectedHeight int
	}{
		{
			"WS4in0E portrait",
			epaper.NewWS4in0E(model.ImgCanonicalOrientationPortrait),
			800,
			600,
			400,
			600,
		},
		{
			"WS7in3E landscape",
			epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape),
			1200,
			800,
			800,
			480,
		},
		{
			"WS13in3E portrait",
			epaper.NewWS13in3E(model.ImgCanonicalOrientationPortrait),
			2400,
			1800,
			1200,
			1600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cropper := crop.NewImageCropper(tt.display, config.CropStrategyCenter)

			// Create input image with specified dimensions
			input := image.NewRGBA(image.Rect(0, 0, tt.inputWidth, tt.inputHeight))

			meta := &model.ImgMeta{
				ImageOrientation: tt.display.InstalledOrientation(),
			}

			output, _ := cropper.Apply(context.Background(), input, meta)

			bounds := output.Bounds()
			assert.Equal(t, tt.expectedWidth, bounds.Max.X, "width mismatch")
			assert.Equal(t, tt.expectedHeight, bounds.Max.Y, "height mismatch")
		})
	}
}

func TestImageCropper_MetadataPreservation(t *testing.T) {
	display := epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape)
	cropper := crop.NewImageCropper(display, config.CropStrategyCenter)

	input := image.NewRGBA(image.Rect(0, 0, 1200, 800))
	meta := &model.ImgMeta{
		ImageOrientation: display.InstalledOrientation(),
	}

	_, resultMeta := cropper.Apply(context.Background(), input, meta)

	// Verify metadata is returned (same reference)
	assert.Equal(t, meta, resultMeta)
}

// TestImageCropper_ExifSubject_ShiftsOffset verifies that exif_subject crop shifts the
// crop window toward the subject compared to center crop.
//
// Setup: 1000×480 input, WS7in3E display (800×480). The horizontal crop removes 200px.
//   - center:       offsetX = 100  → pixel at x=99 is outside the output
//   - exif_subject: offsetX = 0   → pixel at x=99 is inside the output
func TestImageCropper_ExifSubject_ShiftsOffset(t *testing.T) {
	display := epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape)

	// Mark a sentinel pixel near the left edge
	const sentinelX, sentinelY = 99, 0
	input := image.NewRGBA(image.Rect(0, 0, 1000, 480))
	input.Set(sentinelX, sentinelY, sentinelColor)

	baseMeta := func() *model.ImgMeta {
		return &model.ImgMeta{
			ImageOrientation:   display.InstalledOrientation(),
			HasExifSubjectArea: true,
			ExifSubjectArea:    image.Point{X: 50, Y: 240}, // near left edge
		}
	}

	// center crop: sentinel pixel should NOT appear in output
	centerOut, _ := crop.NewImageCropper(display, config.CropStrategyCenter).Apply(context.Background(), input, baseMeta())
	assert.False(t, containsColor(centerOut, sentinelColor), "center crop should not include sentinel pixel")

	// exif_subject crop: sentinel pixel SHOULD appear in output
	subjectOut, _ := crop.NewImageCropper(display, config.CropStrategyExifSubject).Apply(context.Background(), input, baseMeta())
	assert.True(t, containsColor(subjectOut, sentinelColor), "exif_subject crop should include sentinel pixel near subject")
}

// TestImageCropper_ExifSubject_FallsBackToCenter verifies that when HasExifSubjectArea is
// false, exif_subject produces the same result as center.
func TestImageCropper_ExifSubject_FallsBackToCenter(t *testing.T) {
	display := epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape)
	input := image.NewRGBA(image.Rect(0, 0, 1000, 480))

	metaNoSubject := &model.ImgMeta{
		ImageOrientation:   display.InstalledOrientation(),
		HasExifSubjectArea: false,
	}
	metaCenter := &model.ImgMeta{
		ImageOrientation:   display.InstalledOrientation(),
		HasExifSubjectArea: false,
	}

	subjectOut, _ := crop.NewImageCropper(display, config.CropStrategyExifSubject).Apply(context.Background(), input, metaNoSubject)
	centerOut, _ := crop.NewImageCropper(display, config.CropStrategyCenter).Apply(context.Background(), input, metaCenter)

	assert.Equal(t, subjectOut.Bounds(), centerOut.Bounds())
}

// TestImageCropper_ExifSubject_ClampsToEdge verifies that when the subject is at the far
// right, the crop offset is clamped so the output remains within image bounds.
func TestImageCropper_ExifSubject_ClampsToEdge(t *testing.T) {
	display := epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape)
	// Subject at x=980 (near right edge of 1000px-wide image)
	meta := &model.ImgMeta{
		ImageOrientation:   display.InstalledOrientation(),
		HasExifSubjectArea: true,
		ExifSubjectArea:    image.Point{X: 980, Y: 240},
	}
	input := image.NewRGBA(image.Rect(0, 0, 1000, 480))

	out, _ := crop.NewImageCropper(display, config.CropStrategyExifSubject).Apply(context.Background(), input, meta)

	// Output must exactly match display dimensions (no out-of-bounds crop)
	assert.Equal(t, 800, out.Bounds().Max.X)
	assert.Equal(t, 480, out.Bounds().Max.Y)
}

// sentinelColor is a distinct color used as a marker in crop offset tests.
var sentinelColor = color.RGBA{R: 0xff, G: 0x00, B: 0xff, A: 0xff}

// containsColor returns true if any pixel in img matches the given color.
func containsColor(img image.Image, c color.RGBA) bool {
	b := img.Bounds()
	sr, sg, sb, sa := c.RGBA()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if r == sr && g == sg && bl == sb && a == sa {
				return true
			}
		}
	}
	return false
}

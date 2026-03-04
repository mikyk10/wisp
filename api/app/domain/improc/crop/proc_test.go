package crop_test

import (
	"context"
	"image"
	"testing"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/improc/crop"
	"github.com/mikyk10/wisp/app/domain/model"
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
			cropper := crop.NewImageCropper(tt.display)

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
	cropper := crop.NewImageCropper(display)

	input := image.NewRGBA(image.Rect(0, 0, 1200, 800))
	meta := &model.ImgMeta{
		ImageOrientation: display.InstalledOrientation(),
	}

	_, resultMeta := cropper.Apply(context.Background(), input, meta)

	// Verify metadata is returned (same reference)
	assert.Equal(t, meta, resultMeta)
}

package exif_rotation_test

import (
	"context"
	"image"
	"testing"

	"github.com/mikyk10/wisp/app/domain/improc/exif_rotation"
	"github.com/mikyk10/wisp/app/domain/improc/ortho"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/stretchr/testify/assert"
)

// TestExifRotation_TransformsSubjectArea verifies that Apply() correctly transforms
// ExifSubjectArea coordinates to match the post-rotation image coordinate system.
//
// Input image: 300×200 (W=300, H=200, so W-1=299, H-1=199)
// Subject point: (100, 50)
func TestExifRotation_TransformsSubjectArea(t *testing.T) {
	tests := []struct {
		name        string
		orientation model.ExifOrientation
		inputW      int
		inputH      int
		subjectIn   image.Point
		subjectOut  image.Point
	}{
		{
			name:        "orientation 1 (normal) – no change",
			orientation: 1,
			inputW:      300, inputH: 200,
			subjectIn:  image.Point{X: 100, Y: 50},
			subjectOut: image.Point{X: 100, Y: 50},
		},
		{
			name:        "orientation 2 (flip horizontal)",
			orientation: 2,
			inputW:      300, inputH: 200,
			subjectIn:  image.Point{X: 100, Y: 50},
			subjectOut: image.Point{X: 199, Y: 50}, // W-1-x = 299-100 = 199
		},
		{
			name:        "orientation 3 (180°)",
			orientation: 3,
			inputW:      300, inputH: 200,
			subjectIn:  image.Point{X: 100, Y: 50},
			subjectOut: image.Point{X: 199, Y: 149}, // (W-1-x, H-1-y)
		},
		{
			name:        "orientation 4 (flip vertical)",
			orientation: 4,
			inputW:      300, inputH: 200,
			subjectIn:  image.Point{X: 100, Y: 50},
			subjectOut: image.Point{X: 100, Y: 149}, // H-1-y = 199-50 = 149
		},
		{
			name:        "orientation 5 (transpose: rotate 90° CW + flip H)",
			orientation: 5,
			inputW:      300, inputH: 200,
			subjectIn:  image.Point{X: 100, Y: 50},
			subjectOut: image.Point{X: 50, Y: 100}, // (y, x)
		},
		{
			name:        "orientation 6 (90° CW)",
			orientation: 6,
			inputW:      300, inputH: 200,
			subjectIn:  image.Point{X: 100, Y: 50},
			subjectOut: image.Point{X: 149, Y: 100}, // (H-1-y, x) = (199-50, 100)
		},
		{
			name:        "orientation 7 (transverse: rotate -90° + flip H)",
			orientation: 7,
			inputW:      300, inputH: 200,
			subjectIn:  image.Point{X: 100, Y: 50},
			subjectOut: image.Point{X: 149, Y: 199}, // (H-1-y, W-1-x) = (149, 199)
		},
		{
			name:        "orientation 8 (90° CCW)",
			orientation: 8,
			inputW:      300, inputH: 200,
			subjectIn:  image.Point{X: 100, Y: 50},
			subjectOut: image.Point{X: 50, Y: 199}, // (y, W-1-x) = (50, 299-100)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := exif_rotation.NewExifRotation()
			input := image.NewRGBA(image.Rect(0, 0, tt.inputW, tt.inputH))
			meta := &model.ImgMeta{
				ExifOrientation:    tt.orientation,
				HasExifSubjectArea: true,
				ExifSubjectArea:    tt.subjectIn,
			}

			_, outMeta := proc.Apply(context.Background(), input, meta)

			assert.Equal(t, tt.subjectOut, outMeta.ExifSubjectArea,
				"subject area point should be transformed to post-rotation coordinates")
		})
	}
}

// TestExifRotation_RotatesImmediately covers the form used by the thumbnail
// pass during a catalogue scan, where nothing downstream would rotate the
// image. It has to hand back pixels that are already upright.
func TestExifRotation_RotatesImmediately(t *testing.T) {
	proc := exif_rotation.NewExifRotation()
	input := image.NewRGBA(image.Rect(0, 0, 300, 200))
	meta := &model.ImgMeta{ExifOrientation: 6} // quarter turn: 300x200 becomes 200x300

	out, outMeta := proc.Apply(context.Background(), input, meta)

	assert.Equal(t, 200, out.Bounds().Dx(), "width after rotation")
	assert.Equal(t, 300, out.Bounds().Dy(), "height after rotation")
	assert.Equal(t, model.ImgCanonicalOrientationPortrait, outMeta.ImageOrientation)
	assert.Equal(t, ortho.Identity, outMeta.PendingExifOp, "nothing should be left outstanding")
}

// TestExifRotation_DeferredLeavesPixelsAlone covers the form used by the
// delivery path, where crop performs the operation as part of its own.
func TestExifRotation_DeferredLeavesPixelsAlone(t *testing.T) {
	proc := exif_rotation.NewDeferredExifRotation()
	input := image.NewRGBA(image.Rect(0, 0, 300, 200))
	meta := &model.ImgMeta{ExifOrientation: 6}

	out, outMeta := proc.Apply(context.Background(), input, meta)

	assert.Same(t, input, out, "the pixels should not be touched")
	assert.Equal(t, ortho.Rotate90CW, outMeta.PendingExifOp, "the operation should be recorded")
	// The orientation still describes the image as it will be once crop has
	// applied the recorded operation, because crop chooses its correction from it.
	assert.Equal(t, model.ImgCanonicalOrientationPortrait, outMeta.ImageOrientation)
}

// TestExifRotation_SubjectAreaUnchangedWhenAbsent verifies that Apply() does not modify
// ExifSubjectArea when HasExifSubjectArea is false.
func TestExifRotation_SubjectAreaUnchangedWhenAbsent(t *testing.T) {
	proc := exif_rotation.NewExifRotation()
	input := image.NewRGBA(image.Rect(0, 0, 300, 200))
	meta := &model.ImgMeta{
		ExifOrientation:    6,
		HasExifSubjectArea: false,
		ExifSubjectArea:    image.Point{X: 0, Y: 0},
	}

	_, outMeta := proc.Apply(context.Background(), input, meta)

	assert.False(t, outMeta.HasExifSubjectArea)
	assert.Equal(t, image.Point{X: 0, Y: 0}, outMeta.ExifSubjectArea)
}

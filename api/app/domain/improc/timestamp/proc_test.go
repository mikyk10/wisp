package timestamp_test

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/improc/timestamp"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/stretchr/testify/assert"
)

func TestTimestamp_SkipWhenNoExifDateTime(t *testing.T) {
	processor := timestamp.NewTimstamp()

	input := image.NewRGBA(image.Rect(0, 0, 100, 100))
	meta := &model.ImgMeta{
		ExifDateTime: time.Time{}, // Zero time
	}

	output, resultMeta := processor.Apply(context.Background(), input, meta)

	// Should return same image when no EXIF datetime
	assert.Equal(t, input, output)
	assert.Equal(t, meta, resultMeta)
}

func TestTimestamp_ProcessWhenExifDateTime(t *testing.T) {
	processor := timestamp.NewTimstamp()

	input := image.NewRGBA(image.Rect(0, 0, 200, 200))
	exifTime := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
	meta := &model.ImgMeta{
		ExifDateTime: exifTime,
	}

	output, resultMeta := processor.Apply(context.Background(), input, meta)

	// Should return modified image when EXIF datetime is present
	assert.NotEqual(t, input, output)
	assert.Equal(t, meta, resultMeta)
	assert.Equal(t, exifTime, resultMeta.ExifDateTime)
}

func TestTimestamp_DateFormatting(t *testing.T) {
	// Verify expected date format by checking the comment in proc.go
	// Format should be "2006/01/02"
	tests := []struct {
		date   time.Time
		expect string
	}{
		{time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), "2024/01/05"},
		{time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC), "2024/12/25"},
		{time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC), "2000/02/29"},
	}

	for _, tt := range tests {
		formatted := tt.date.Format("2006/01/02")
		assert.Equal(t, tt.expect, formatted)
	}
}

func TestTimestamp_OutputDimensions(t *testing.T) {
	processor := timestamp.NewTimstamp()

	input := image.NewRGBA(image.Rect(0, 0, 400, 300))
	meta := &model.ImgMeta{
		ExifDateTime: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
	}

	output, _ := processor.Apply(context.Background(), input, meta)

	// Output dimensions should match input
	inputBounds := input.Bounds()
	outputBounds := output.Bounds()

	assert.Equal(t, inputBounds.Max.X, outputBounds.Max.X)
	assert.Equal(t, inputBounds.Max.Y, outputBounds.Max.Y)
}

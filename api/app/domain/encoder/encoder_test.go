package encoder_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/encoder"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/stretchr/testify/assert"
)

func TestNewWaveshareEPEncoder(t *testing.T) {
	tests := []struct {
		name          string
		displayModel  epaper.DisplayMetadata
		expectedType  string
	}{
		{
			"WS4in0E returns default encoder",
			epaper.NewWS4in0E(model.ImgCanonicalOrientationPortrait),
			"*encoder.wsEpaperEncoder",
		},
		{
			"WS7in3E returns default encoder",
			epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape),
			"*encoder.wsEpaperEncoder",
		},
		{
			"WS7in3F returns default encoder",
			epaper.NewWS7in3F(model.ImgCanonicalOrientationLandscape),
			"*encoder.wsEpaperEncoder",
		},
		{
			"WS13in3E returns WS13in3EpaperEEncoder",
			epaper.NewWS13in3E(model.ImgCanonicalOrientationPortrait),
			"*encoder.ws13in3EpaperEEncoder",
		},
		{
			"WS13in3K returns WS13in3EpaperKEncoder",
			epaper.NewWS13in3K(model.ImgCanonicalOrientationLandscape),
			"*encoder.ws13in3EpaperKEncoder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := encoder.NewWaveshareEPEncoder(tt.displayModel)
			assert.NotNil(t, enc)
			assert.Equal(t, tt.expectedType, encoder.TypeOf(enc))
		})
	}
}

func TestBuildIndex(t *testing.T) {
	tests := []struct {
		name          string
		paletteMap    map[int]color.Color
		expectedCount int
	}{
		{
			"empty palette",
			map[int]color.Color{},
			0,
		},
		{
			"single color",
			map[int]color.Color{
				0: color.RGBA{0x42, 0x42, 0x42, 0xff},
			},
			1,
		},
		{
			"multiple colors",
			map[int]color.Color{
				0: color.RGBA{0x42, 0x42, 0x42, 0xff}, // black
				1: color.RGBA{0xe3, 0xe3, 0xe3, 0xff}, // white
				2: color.RGBA{0xd7, 0xc7, 0x6a, 0xff}, // yellow
			},
			3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := encoder.BuildIndex(tt.paletteMap)
			assert.Equal(t, tt.expectedCount, len(idx))

			// Verify all palette colors are in the index
			for _, c := range tt.paletteMap {
				red, _, _, _ := c.RGBA()
				_, exists := idx[red]
				assert.True(t, exists, "color red=%d not found in index", red)
			}
		})
	}
}

func TestBuildIndexUniquenessOfRedComponent(t *testing.T) {
	// Test that colors with unique red components map correctly
	palette := map[int]color.Color{
		0: color.RGBA{0x10, 0xFF, 0xFF, 0xff}, // red=0x10
		1: color.RGBA{0x20, 0xFF, 0xFF, 0xff}, // red=0x20
		2: color.RGBA{0x30, 0xFF, 0xFF, 0xff}, // red=0x30
	}

	idx := encoder.BuildIndex(palette)
	assert.Equal(t, 3, len(idx))

	// Verify mapping by red component value
	red0, _, _, _ := palette[0].RGBA()
	red1, _, _, _ := palette[1].RGBA()
	red2, _, _, _ := palette[2].RGBA()

	assert.Equal(t, 0, idx[red0])
	assert.Equal(t, 1, idx[red1])
	assert.Equal(t, 2, idx[red2])
}

func TestEncodeBasicFunctionality(t *testing.T) {
	// Create a simple 2x2 image with uniform color
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))

	// Fill with black (palette index 0)
	black := color.RGBA{0x42, 0x42, 0x42, 0xff}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, black)
		}
	}

	// Test with WS4in0E (default encoder, 4 pixels = 2 bytes)
	display := epaper.NewWS4in0E(model.ImgCanonicalOrientationPortrait)
	enc := encoder.NewWaveshareEPEncoder(display)

	buf, err := enc.Encode(img)
	assert.Nil(t, err)
	assert.NotNil(t, buf)

	// 4 pixels should produce 2 bytes in 4-bit encoding
	assert.Equal(t, 2, buf.Len())

	// All black should be 0x00 for each byte
	assert.Equal(t, byte(0x00), buf.Bytes()[0])
	assert.Equal(t, byte(0x00), buf.Bytes()[1])
}

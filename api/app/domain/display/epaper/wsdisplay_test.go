package epaper_test

import (
	"context"
	"fmt"
	"image"
	"slices"
	"testing"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/display/epaper/wsdisplay"
	"github.com/mikyk10/wisp/app/domain/encoder"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/improc/crop"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/stretchr/testify/assert"
)

type procs struct {
	meta  epaper.DisplayMetadata
	imseq improc.Sequencer
	enc   epaper.Encoder
}

func buildImageProcs(epm epaper.DisplayMetadata) procs {
	imseq := improc.NewSequencer()
	imseq.Push(crop.NewImageCropper(epm))
	enc := encoder.NewWaveshareEPEncoder(epm)
	return procs{epm, imseq, enc}
}

func testEncoderWithCases(t *testing.T, p procs, testName string, cases []struct {
	colorName        string
	srcImagePixels   []uint8
	expectedPixels   []uint8
	expectedNumBytes int
}) {
	t.Run(testName, func(t *testing.T) {
		for _, tt := range cases {
			t.Run(tt.colorName, func(t *testing.T) {
				img := image.NewRGBA(image.Rect(0, 0, p.meta.Width(), p.meta.Height()))
				img.Pix = tt.srcImagePixels

				meta := &model.ImgMeta{ImageOrientation: p.meta.InstalledOrientation()}
				reduced, _ := p.imseq.Apply(context.Background(), img, meta)
				dat, err := p.enc.Encode(reduced)
				assert.Nil(t, err)

				dbytes := dat.Bytes()
				assert.Equal(t, tt.expectedNumBytes, len(dbytes))
				for i, v := range dbytes {
					assert.Equal(t, tt.expectedPixels[i], v, fmt.Sprintf("byte %d mismatch", i))
				}
			})
		}
	})
}

func TestDefaultEncoder_WS4in0E(t *testing.T) {
	p := buildImageProcs(epaper.NewWS4in0E(model.ImgCanonicalOrientationPortrait))

	cases := []struct {
		colorName        string
		srcImagePixels   []uint8
		expectedPixels   []uint8
		expectedNumBytes int
	}{
		{
			"black",
			slices.Repeat([]uint8{wsdisplay.Black.R, wsdisplay.Black.G, wsdisplay.Black.B, wsdisplay.Black.A}, 400*600),
			slices.Repeat([]uint8{0x0}, 400*600/2),
			400 * 600 / 2,
		},
		{
			"white",
			slices.Repeat([]uint8{wsdisplay.White.R, wsdisplay.White.G, wsdisplay.White.B, wsdisplay.White.A}, 400*600),
			slices.Repeat([]uint8{0x11}, 400*600/2),
			400 * 600 / 2,
		},
		{
			"yellow",
			slices.Repeat([]uint8{wsdisplay.Yellow.R, wsdisplay.Yellow.G, wsdisplay.Yellow.B, wsdisplay.Yellow.A}, 400*600),
			slices.Repeat([]uint8{0x22}, 400*600/2),
			400 * 600 / 2,
		},
		{
			"red",
			slices.Repeat([]uint8{wsdisplay.Red.R, wsdisplay.Red.G, wsdisplay.Red.B, wsdisplay.Red.A}, 400*600),
			slices.Repeat([]uint8{0x33}, 400*600/2),
			400 * 600 / 2,
		},
		{
			"blue",
			slices.Repeat([]uint8{wsdisplay.Blue.R, wsdisplay.Blue.G, wsdisplay.Blue.B, wsdisplay.Blue.A}, 400*600),
			slices.Repeat([]uint8{0x55}, 400*600/2),
			400 * 600 / 2,
		},
		{
			"green",
			slices.Repeat([]uint8{wsdisplay.Green.R, wsdisplay.Green.G, wsdisplay.Green.B, wsdisplay.Green.A}, 400*600),
			slices.Repeat([]uint8{0x66}, 400*600/2),
			400 * 600 / 2,
		},
	}

	testEncoderWithCases(t, p, "WS4in0E", cases)
}

func TestDefaultEncoder_WS7in3E(t *testing.T) {
	p := buildImageProcs(epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape))

	cases := []struct {
		colorName        string
		srcImagePixels   []uint8
		expectedPixels   []uint8
		expectedNumBytes int
	}{
		{
			"black",
			slices.Repeat([]uint8{wsdisplay.Black.R, wsdisplay.Black.G, wsdisplay.Black.B, wsdisplay.Black.A}, 800*480),
			slices.Repeat([]uint8{0x0}, 800*480/2),
			800 * 480 / 2,
		},
		{
			"white",
			slices.Repeat([]uint8{wsdisplay.White.R, wsdisplay.White.G, wsdisplay.White.B, wsdisplay.White.A}, 800*480),
			slices.Repeat([]uint8{0x11}, 800*480/2),
			800 * 480 / 2,
		},
		{
			"green",
			slices.Repeat([]uint8{wsdisplay.Green.R, wsdisplay.Green.G, wsdisplay.Green.B, wsdisplay.Green.A}, 800*480),
			slices.Repeat([]uint8{0x66}, 800*480/2),
			800 * 480 / 2,
		},
		{
			"blue",
			slices.Repeat([]uint8{wsdisplay.Blue.R, wsdisplay.Blue.G, wsdisplay.Blue.B, wsdisplay.Blue.A}, 800*480),
			slices.Repeat([]uint8{0x55}, 800*480/2),
			800 * 480 / 2,
		},
		{
			"red",
			slices.Repeat([]uint8{wsdisplay.Red.R, wsdisplay.Red.G, wsdisplay.Red.B, wsdisplay.Red.A}, 800*480),
			slices.Repeat([]uint8{0x33}, 800*480/2),
			800 * 480 / 2,
		},
		{
			"yellow",
			slices.Repeat([]uint8{wsdisplay.Yellow.R, wsdisplay.Yellow.G, wsdisplay.Yellow.B, wsdisplay.Yellow.A}, 800*480),
			slices.Repeat([]uint8{0x22}, 800*480/2),
			800 * 480 / 2,
		},
	}

	testEncoderWithCases(t, p, "WS7in3E", cases)
}

func TestDefaultEncoder_WS7in3F(t *testing.T) {
	p := buildImageProcs(epaper.NewWS7in3F(model.ImgCanonicalOrientationLandscape))

	cases := []struct {
		colorName        string
		srcImagePixels   []uint8
		expectedPixels   []uint8
		expectedNumBytes int
	}{
		{
			"black",
			slices.Repeat([]uint8{wsdisplay.Black.R, wsdisplay.Black.G, wsdisplay.Black.B, wsdisplay.Black.A}, 800*480),
			slices.Repeat([]uint8{0x0}, 800*480/2),
			800 * 480 / 2,
		},
		{
			"white",
			slices.Repeat([]uint8{wsdisplay.White.R, wsdisplay.White.G, wsdisplay.White.B, wsdisplay.White.A}, 800*480),
			slices.Repeat([]uint8{0x11}, 800*480/2),
			800 * 480 / 2,
		},
		{
			"green",
			slices.Repeat([]uint8{wsdisplay.Green.R, wsdisplay.Green.G, wsdisplay.Green.B, wsdisplay.Green.A}, 800*480),
			slices.Repeat([]uint8{0x22}, 800*480/2),
			800 * 480 / 2,
		},
		{
			"blue",
			slices.Repeat([]uint8{wsdisplay.Blue.R, wsdisplay.Blue.G, wsdisplay.Blue.B, wsdisplay.Blue.A}, 800*480),
			slices.Repeat([]uint8{0x33}, 800*480/2),
			800 * 480 / 2,
		},
		{
			"red",
			slices.Repeat([]uint8{wsdisplay.Red.R, wsdisplay.Red.G, wsdisplay.Red.B, wsdisplay.Red.A}, 800*480),
			slices.Repeat([]uint8{0x44}, 800*480/2),
			800 * 480 / 2,
		},
		{
			"yellow",
			slices.Repeat([]uint8{wsdisplay.Yellow.R, wsdisplay.Yellow.G, wsdisplay.Yellow.B, wsdisplay.Yellow.A}, 800*480),
			slices.Repeat([]uint8{0x55}, 800*480/2),
			800 * 480 / 2,
		},
		{
			"orange",
			slices.Repeat([]uint8{wsdisplay.Orange.R, wsdisplay.Orange.G, wsdisplay.Orange.B, wsdisplay.Orange.A}, 800*480),
			slices.Repeat([]uint8{0x66}, 800*480/2),
			800 * 480 / 2,
		},
	}

	testEncoderWithCases(t, p, "WS7in3F", cases)
}

func TestWS13in3EEncoder(t *testing.T) {
	p := buildImageProcs(epaper.NewWS13in3E(model.ImgCanonicalOrientationPortrait))

	cases := []struct {
		colorName        string
		srcImagePixels   []uint8
		expectedPixels   []uint8
		expectedNumBytes int
	}{
		{
			"black/white",
			slices.Repeat(
				slices.Concat(
					slices.Repeat([]uint8{wsdisplay.Black.R, wsdisplay.Black.G, wsdisplay.Black.B, wsdisplay.Black.A}, 600),
					slices.Repeat([]uint8{wsdisplay.White.R, wsdisplay.White.G, wsdisplay.White.B, wsdisplay.White.A}, 600),
				),
				1600,
			),
			slices.Concat(
				slices.Repeat(slices.Repeat([]uint8{0x0}, 300), 1600),
				slices.Repeat(slices.Repeat([]uint8{0x11}, 300), 1600),
			),
			1200 * 1600 / 2,
		},
		{
			"green",
			slices.Repeat([]uint8{wsdisplay.Green.R, wsdisplay.Green.G, wsdisplay.Green.B, wsdisplay.Green.A}, 1200*1600),
			slices.Repeat([]uint8{0x66}, 1200*1600/2),
			1200 * 1600 / 2,
		},
		{
			"blue",
			slices.Repeat([]uint8{wsdisplay.Blue.R, wsdisplay.Blue.G, wsdisplay.Blue.B, wsdisplay.Blue.A}, 1200*1600),
			slices.Repeat([]uint8{0x55}, 1200*1600/2),
			1200 * 1600 / 2,
		},
		{
			"red",
			slices.Repeat([]uint8{wsdisplay.Red.R, wsdisplay.Red.G, wsdisplay.Red.B, wsdisplay.Red.A}, 1200*1600),
			slices.Repeat([]uint8{0x33}, 1200*1600/2),
			1200 * 1600 / 2,
		},
		{
			"yellow",
			slices.Repeat([]uint8{wsdisplay.Yellow.R, wsdisplay.Yellow.G, wsdisplay.Yellow.B, wsdisplay.Yellow.A}, 1200*1600),
			slices.Repeat([]uint8{0x22}, 1200*1600/2),
			1200 * 1600 / 2,
		},
	}

	testEncoderWithCases(t, p, "WS13in3E", cases)
}

func TestWS13in3KEncoder(t *testing.T) {
	p := buildImageProcs(epaper.NewWS13in3K(model.ImgCanonicalOrientationLandscape))

	cases := []struct {
		colorName        string
		srcImagePixels   []uint8
		expectedPixels   []uint8
		expectedNumBytes int
	}{
		{
			"black",
			slices.Repeat([]uint8{wsdisplay.Black.R, wsdisplay.Black.G, wsdisplay.Black.B, wsdisplay.Black.A}, 960*680),
			slices.Repeat([]uint8{0x00}, 960*680/4),
			960 * 680 / 4,
		},
		{
			"darkgray",
			slices.Repeat([]uint8{wsdisplay.DarkGray.R, wsdisplay.DarkGray.G, wsdisplay.DarkGray.B, wsdisplay.DarkGray.A}, 960*680),
			slices.Repeat([]uint8{0x55}, 960*680/4),
			960 * 680 / 4,
		},
		{
			"lightgray",
			slices.Repeat([]uint8{wsdisplay.LightGray.R, wsdisplay.LightGray.G, wsdisplay.LightGray.B, wsdisplay.LightGray.A}, 960*680),
			slices.Repeat([]uint8{0xAA}, 960*680/4),
			960 * 680 / 4,
		},
		{
			"white",
			slices.Repeat([]uint8{wsdisplay.White.R, wsdisplay.White.G, wsdisplay.White.B, wsdisplay.White.A}, 960*680),
			slices.Repeat([]uint8{0xFF}, 960*680/4),
			960 * 680 / 4,
		},
	}

	testEncoderWithCases(t, p, "WS13in3K", cases)
}

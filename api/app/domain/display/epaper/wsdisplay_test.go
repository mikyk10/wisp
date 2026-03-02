package epaper_test

import (
	"context"
	"fmt"
	"image"
	"slices"
	"testing"
	"wspf/app/domain/display/epaper"
	"wspf/app/domain/display/epaper/wsdisplay"
	"wspf/app/domain/encoder"
	"wspf/app/domain/improc"
	"wspf/app/domain/improc/crop"
	"wspf/app/domain/model"

	"github.com/stretchr/testify/assert"
)

// procs organises the modules under test for each display model.
type procs struct {
	meta  epaper.DisplayMetadata
	imseq improc.Sequencer
	enc   epaper.Encoder
}

// numPixels returns the total number of pixels.
func (p *procs) numPixels() int {
	return p.meta.Width() * p.meta.Height()
}

// numBytes returns the encoded file size, which is half the pixel count.
func (p *procs) numBytes() int {
	return p.numPixels() / 2
}

// buildImageProcs is a factory for image processing modules.
func buildImageProcs(epm epaper.DisplayMetadata) procs {

	imseq := improc.NewSequencer()
	imseq.Push(crop.NewImageCropper(epm))

	// エンコーダ
	enc := encoder.NewWaveshareEPEncoder(epm)

	return procs{epm, imseq, enc}
}

func TestWS3Encoders(t *testing.T) {

	// ePaper display models
	ws13in3E := buildImageProcs(epaper.NewWS13in3E(model.ImgCanonicalOrientationPortrait))
	ws4in0E := buildImageProcs(epaper.NewWS4in0E(model.ImgCanonicalOrientationPortrait))
	ws7in3E := buildImageProcs(epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape))
	ws7in3F := buildImageProcs(epaper.NewWS7in3F(model.ImgCanonicalOrientationLandscape))
	ws13in3K := buildImageProcs(epaper.NewWS13in3K(model.ImgCanonicalOrientationLandscape))

	var tests = []struct {
		sut       procs
		colorName string

		srcImagePixels   []uint8 // image.Pix
		expectedPixels   []uint8
		expectedNumBytes int
	}{
		// WS4in0E
		//TODO: more tests
		{ws4in0E, "black", // 0000 0000
			slices.Repeat([]uint8{wsdisplay.Black.R, wsdisplay.Black.G, wsdisplay.Black.B, wsdisplay.Black.A}, 600*400),
			slices.Repeat([]uint8{0x0}, 600*400/2),
			600 * 400 / 2,
		},

		// WS13in3E
		{ws13in3E, "black/white",
			slices.Repeat(
				// left 600px black, right 600px white
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
		{ws13in3E, "green",
			slices.Repeat([]uint8{wsdisplay.Green.R, wsdisplay.Green.G, wsdisplay.Green.B, wsdisplay.Green.A}, 1200*1600),
			slices.Repeat([]uint8{0x66}, 1200*1600/2),
			1200 * 1600 / 2,
		},
		{ws13in3E, "blue", // 0011 0011
			slices.Repeat([]uint8{wsdisplay.Blue.R, wsdisplay.Blue.G, wsdisplay.Blue.B, wsdisplay.Blue.A}, 1200*1600),
			slices.Repeat([]uint8{0x55}, 1200*1600/2),
			1200 * 1600 / 2,
		},
		{ws13in3E, "red", // 0100 0100
			slices.Repeat([]uint8{wsdisplay.Red.R, wsdisplay.Red.G, wsdisplay.Red.B, wsdisplay.Red.A}, 1200*1600),
			slices.Repeat([]uint8{0x33}, 1200*1600/2),
			1200 * 1600 / 2,
		},
		{ws13in3E, "yellow", // 0101 0101
			slices.Repeat([]uint8{wsdisplay.Yellow.R, wsdisplay.Yellow.G, wsdisplay.Yellow.B, wsdisplay.Yellow.A}, 1200*1600),
			slices.Repeat([]uint8{0x22}, 1200*1600/2),
			1200 * 1600 / 2,
		},

		// WS7in3E
		{ws7in3E, "black", // 0000 0000
			slices.Repeat([]uint8{wsdisplay.Black.R, wsdisplay.Black.G, wsdisplay.Black.B, wsdisplay.Black.A}, 800*480),
			slices.Repeat([]uint8{0x0}, 800*480/2),
			800 * 480 / 2,
		},
		{ws7in3E, "white", // 0001 0001
			slices.Repeat([]uint8{wsdisplay.White.R, wsdisplay.White.G, wsdisplay.White.B, wsdisplay.White.A}, 800*480),
			slices.Repeat([]uint8{0x11}, 800*480/2),
			800 * 480 / 2,
		},
		{ws7in3E, "green",
			slices.Repeat([]uint8{wsdisplay.Green.R, wsdisplay.Green.G, wsdisplay.Green.B, wsdisplay.Green.A}, 800*480),
			slices.Repeat([]uint8{0x66}, 800*480/2),
			800 * 480 / 2,
		},
		{ws7in3E, "blue", // 0011 0011
			slices.Repeat([]uint8{wsdisplay.Blue.R, wsdisplay.Blue.G, wsdisplay.Blue.B, wsdisplay.Blue.A}, 800*480),
			slices.Repeat([]uint8{0x55}, 800*480/2),
			800 * 480 / 2,
		},
		{ws7in3E, "red", // 0100 0100
			slices.Repeat([]uint8{wsdisplay.Red.R, wsdisplay.Red.G, wsdisplay.Red.B, wsdisplay.Red.A}, 800*480),
			slices.Repeat([]uint8{0x33}, 800*480/2),
			800 * 480 / 2,
		},
		{ws7in3E, "yellow", // 0101 0101
			slices.Repeat([]uint8{wsdisplay.Yellow.R, wsdisplay.Yellow.G, wsdisplay.Yellow.B, wsdisplay.Yellow.A}, 800*480),
			slices.Repeat([]uint8{0x22}, 800*480/2),
			800 * 480 / 2,
		},

		// WS7in3F
		{ws7in3F, "black", // 0000 0000
			slices.Repeat([]uint8{wsdisplay.Black.R, wsdisplay.Black.G, wsdisplay.Black.B, wsdisplay.Black.A}, 800*480),
			slices.Repeat([]uint8{0x0}, 800*480/2),
			800 * 480 / 2,
		},
		{ws7in3F, "white", // 0001 0001
			slices.Repeat([]uint8{wsdisplay.White.R, wsdisplay.White.G, wsdisplay.White.B, wsdisplay.White.A}, 800*480),
			slices.Repeat([]uint8{0x11}, 800*480/2),
			800 * 480 / 2,
		},
		{ws7in3F, "green",
			slices.Repeat([]uint8{wsdisplay.Green.R, wsdisplay.Green.G, wsdisplay.Green.B, wsdisplay.Green.A}, 800*480),
			slices.Repeat([]uint8{0x22}, 800*480/2),
			800 * 480 / 2,
		},
		{ws7in3F, "blue",
			slices.Repeat([]uint8{wsdisplay.Blue.R, wsdisplay.Blue.G, wsdisplay.Blue.B, wsdisplay.Blue.A}, 800*480),
			slices.Repeat([]uint8{0x33}, 800*480/2),
			800 * 480 / 2,
		},
		{ws7in3F, "red",
			slices.Repeat([]uint8{wsdisplay.Red.R, wsdisplay.Red.G, wsdisplay.Red.B, wsdisplay.Red.A}, 800*480),
			slices.Repeat([]uint8{0x44}, 800*480/2),
			800 * 480 / 2,
		},
		{ws7in3F, "yellow",
			slices.Repeat([]uint8{wsdisplay.Yellow.R, wsdisplay.Yellow.G, wsdisplay.Yellow.B, wsdisplay.Yellow.A}, 800*480),
			slices.Repeat([]uint8{0x55}, 800*480/2),
			800 * 480 / 2,
		},
		{ws7in3F, "orange",
			slices.Repeat([]uint8{wsdisplay.Orange.R, wsdisplay.Orange.G, wsdisplay.Orange.B, wsdisplay.Orange.A}, 800*480),
			slices.Repeat([]uint8{0x66}, 800*480/2),
			800 * 480 / 2,
		},
		//
		{ws13in3K, "black",
			slices.Repeat([]uint8{wsdisplay.Black.R, wsdisplay.Black.G, wsdisplay.Black.B, wsdisplay.Black.A}, 960*680),
			slices.Repeat([]uint8{0x00}, 960*680/4),
			960 * 680 / 4,
		},
		{ws13in3K, "drakgray",
			slices.Repeat([]uint8{wsdisplay.DarkGray.R, wsdisplay.DarkGray.G, wsdisplay.DarkGray.B, wsdisplay.DarkGray.A}, 960*680),
			slices.Repeat([]uint8{0x55}, 960*680/4),
			960 * 680 / 4,
		},
		{ws13in3K, "lightgray",
			slices.Repeat([]uint8{wsdisplay.LightGray.R, wsdisplay.LightGray.G, wsdisplay.LightGray.B, wsdisplay.LightGray.A}, 960*680),
			slices.Repeat([]uint8{0xAA}, 960*680/4),
			960 * 680 / 4,
		},
		{ws13in3K, "white",
			slices.Repeat([]uint8{wsdisplay.White.R, wsdisplay.White.G, wsdisplay.White.B, wsdisplay.White.A}, 960*680),
			slices.Repeat([]uint8{0xFF}, 960*680/4),
			960 * 680 / 4,
		},
	}

	for _, tt := range tests {
		// generate sample image data in the specified color
		img := image.NewRGBA(image.Rect(0, 0, tt.sut.meta.Width(), tt.sut.meta.Height()))
		img.Pix = tt.srcImagePixels

		//f, _ := os.OpenFile("/tmp/test.png", os.O_WRONLY|os.O_CREATE, 0600)
		//png.Encode(f, img)
		///f.Close()

		// color reduction and encoding
		meta := &model.ImgMeta{ImageOrientation: tt.sut.meta.InstalledOrientation()}
		reduced, _ := tt.sut.imseq.Apply(context.Background(), img, meta)
		dat, err := tt.sut.enc.Encode(reduced)
		assert.Nil(t, err)

		// verify the number of pixel bytes in the encoded result
		dbytes := dat.Bytes()

		testname := fmt.Sprintf("%s (%s)", tt.sut.meta.ModelName(), tt.colorName)
		t.Run(testname, func(t *testing.T) {
			assert.Equal(t, len(tt.expectedPixels), tt.expectedNumBytes)
			assert.Equal(t, tt.expectedNumBytes, len(dbytes))
			for i, v := range dbytes {
				// test each pixel
				assert.Equal(t, tt.expectedPixels[i], v)
			}
		})
	}
}

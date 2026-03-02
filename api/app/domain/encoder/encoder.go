package encoder

import (
	"bytes"
	"image"
	"image/color"
	"wspf/app/domain/display/epaper"
)

type wsEpaperEncoder struct {
	colorIndex map[uint32]int // mapping from pixel red component to palette index
}

type ws13in3EpaperEEncoder struct {
	colorIndex map[uint32]int // mapping from pixel red component to palette index
}

type ws13in3EpaperKEncoder struct {
	colorIndex map[uint32]int // mapping from pixel red component to palette index
}

func NewWaveshareEPEncoder(epd epaper.DisplayMetadata) epaper.Encoder {

	var encoder epaper.Encoder

	switch epaper.EPaperDisplayModel(epd.ModelName()) {
	case epaper.WS13in3EPaperE:
		encoder = &ws13in3EpaperEEncoder{
			colorIndex: buildIndex(epd.Palette()),
		}
	case epaper.WS13in3EPaperK:
		encoder = &ws13in3EpaperKEncoder{
			colorIndex: buildIndex(epd.Palette()),
		}
	default:
		encoder = &wsEpaperEncoder{
			colorIndex: buildIndex(epd.Palette()),
		}
	}

	return encoder
}

// buildIndex builds a lookup table for converting RGB to indexed color.
// To reduce computation, only the red component of RGB is used as the key to resolve the palette index.
// Therefore, the red component of each palette color must be unique.
func buildIndex(paletteMap map[int]color.Color) map[uint32]int {
	colorIndex := map[uint32]int{}
	for i, px := range paletteMap {
		red, _, _, _ := px.RGBA()
		colorIndex[red] = i
	}

	return colorIndex
}

// default encoder
func (enc *wsEpaperEncoder) Encode(img image.Image) (*bytes.Buffer, error) {
	bounds := img.Bounds()
	buf := bytes.Buffer{}

	var odd uint8 = 1
	var twopx uint8

	// Read 2 pixels, convert the first into MSB 4bits, other goes 4 bits of LSB.
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)

			red, _, _, _ := c.RGBA()
			colorIndex := enc.colorIndex[red]
			//nolint:gosec // G115: integer overflow conversion int -> uint8
			twopx |= uint8(colorIndex) << (uint8(4) * odd)
			if odd == 0 {
				buf.WriteByte(twopx)
				twopx = 0
			}

			odd ^= 1
		}
	}

	return &buf, nil
}

// 13.3 inch E6 full color encoder
func (enc *ws13in3EpaperEEncoder) Encode(img image.Image) (*bytes.Buffer, error) {
	bounds := img.Bounds()
	buf := bytes.Buffer{}

	// Split the image data vertically in half and encode the left half followed by the right half.
	// The resulting data width is half the original and the height is doubled.
	//
	// [before]     [after]
	// LLRR         LL
	// LLRR  --->   LL
	// LLRR         LL
	//              RR
	//              RR
	//              RR

	var odd uint8 = 1
	var twopx uint8

	for i := 0; i < 2; i++ {
		// Read 2 pixels, convert the first into MSB 4bits, other goes 4 bits of LSB.
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			halfWidth := bounds.Max.X / 2
			for x := (halfWidth * i); x < (halfWidth*i)+halfWidth; x++ {
				c := img.At(x, y)

				red, _, _, _ := c.RGBA()
				colorIndex := enc.colorIndex[red]
				//nolint:gosec // G115: integer overflow conversion int -> uint8
				twopx |= uint8(colorIndex) << (uint8(4) * odd)
				if odd == 0 {
					buf.WriteByte(twopx)
					twopx = 0
				}

				odd ^= 1
			}
		}
	}

	return &buf, nil
}

func (enc *ws13in3EpaperKEncoder) Encode(img image.Image) (*bytes.Buffer, error) {
	bounds := img.Bounds()
	buf := bytes.Buffer{}

	var subBits uint8 = 0
	var fourpx uint8

	// Read 4 pixels, convert the first two pixels into MSB 4bits, other goes 4bits of LSB.
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)

			red, _, _, _ := c.RGBA()
			colorIndex := enc.colorIndex[red]

			//nolint:gosec // G115: integer overflow conversion int -> uint8
			fourpx |= (uint8(colorIndex) << (2 * (uint8(3) - uint8(subBits))))

			if subBits == 3 {
				buf.WriteByte(fourpx)
				fourpx = 0
			}

			subBits = (subBits + 1) % 4
		}
	}

	return &buf, nil
}

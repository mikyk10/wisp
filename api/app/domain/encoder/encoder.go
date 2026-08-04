package encoder

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
)

type wsEpaperEncoder struct {
	colorIndex paletteIndex
}

type ws13in3EpaperEEncoder struct {
	colorIndex paletteIndex
}

type ws13in3EpaperKEncoder struct {
	colorIndex paletteIndex
}

func NewWaveshareEPEncoder(epd epaper.DisplayMetadata) epaper.Encoder {

	var encoder epaper.Encoder

	switch epaper.EPaperDisplayModel(epd.ModelName()) {
	case epaper.WS13in3EPaperE:
		encoder = &ws13in3EpaperEEncoder{
			colorIndex: newPaletteIndex(BuildIndex(epd.Palette())),
		}
	case epaper.WS13in3EPaperK:
		encoder = &ws13in3EpaperKEncoder{
			colorIndex: newPaletteIndex(BuildIndex(epd.Palette())),
		}
	default:
		encoder = &wsEpaperEncoder{
			colorIndex: newPaletteIndex(BuildIndex(epd.Palette())),
		}
	}

	return encoder
}

// BuildIndex builds a lookup table for converting RGB to indexed color.
// To reduce computation, only the red component of RGB is used as the key to resolve the palette index.
// Therefore, the red component of each palette color must be unique.
func BuildIndex(paletteMap map[int]color.Color) map[uint32]int {
	colorIndex := map[uint32]int{}
	for i, px := range paletteMap {
		red, _, _, _ := px.RGBA()
		colorIndex[red] = i
	}

	return colorIndex
}

// TypeOf returns the type name of the encoder (for testing purposes).
func TypeOf(enc epaper.Encoder) string {
	return fmt.Sprintf("%T", enc)
}

// paletteIndex resolves a pixel's red component to its palette index without
// hashing. Palette colours are eight bits per channel, so the top eight bits
// of the red component identify one uniquely, and 256 entries cover every
// possible value.
//
// A colour that is not in the palette lands on index 0, as it did when this
// was a map lookup that missed. The one difference is a colour whose red is
// not a multiple of 257 but shares its top eight bits with a palette entry:
// the map missed on that, this resolves it. Only a 16-bit-per-channel source
// can produce one, and nothing in the pipeline does.
type paletteIndex [256]uint8

func newPaletteIndex(colorIndex map[uint32]int) paletteIndex {
	var idx paletteIndex
	for red, i := range colorIndex {
		//nolint:gosec // G115: palette indices are small by construction
		idx[red>>8] = uint8(i)
	}
	return idx
}

// scan resolves every pixel to its palette index, in row order.
//
// The three panel formats pack these indices differently but all need the same
// lookup first, so it happens once here. Doing it through image.Image.At would
// box a color.Color per pixel — one heap allocation each, 1.9 million of them
// on the 13.3 inch panel — so the layouts the pipeline actually produces are
// read straight out of their pixel buffers.
func (idx *paletteIndex) scan(img image.Image) []uint8 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]uint8, w*h)

	switch src := img.(type) {
	case *image.RGBA:
		// Pix already holds premultiplied values, which is what RGBA() returns.
		for y := 0; y < h; y++ {
			i := src.PixOffset(b.Min.X, b.Min.Y+y)
			row, dst := src.Pix[i:i+w*4], out[y*w:(y+1)*w]
			for x := range dst {
				dst[x] = idx[row[x*4]]
			}
		}

	case *image.NRGBA:
		for y := 0; y < h; y++ {
			i := src.PixOffset(b.Min.X, b.Min.Y+y)
			row, dst := src.Pix[i:i+w*4], out[y*w:(y+1)*w]
			for x := range dst {
				p := row[x*4 : x*4+4 : x*4+4]
				if p[3] == 0xff {
					dst[x] = idx[p[0]]
					continue
				}
				// Mirror color.NRGBA.RGBA(), which premultiplies.
				r := uint32(p[0])
				r |= r << 8
				r *= uint32(p[3])
				r /= 0xff
				dst[x] = idx[r>>8]
			}
		}

	default:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				red, _, _, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				out[y*w+x] = idx[red>>8]
			}
		}
	}

	return out
}

// nibblePacker writes palette indices four bits at a time, the first of each
// pair into the high nibble. A trailing odd pixel is dropped, as it always was.
type nibblePacker struct {
	out  []byte
	pair uint8
	odd  uint8
}

func newNibblePacker(pixels int) *nibblePacker {
	return &nibblePacker{out: make([]byte, 0, pixels/2), odd: 1}
}

func (p *nibblePacker) write(index uint8) {
	p.pair |= index << (4 * p.odd)
	if p.odd == 0 {
		p.out = append(p.out, p.pair)
		p.pair = 0
	}
	p.odd ^= 1
}

// default encoder
func (enc *wsEpaperEncoder) Encode(img image.Image) (*bytes.Buffer, error) {
	indices := enc.colorIndex.scan(img)

	packer := newNibblePacker(len(indices))
	for _, index := range indices {
		packer.write(index)
	}

	return bytes.NewBuffer(packer.out), nil
}

// 13.3 inch E6 full color encoder
func (enc *ws13in3EpaperEEncoder) Encode(img image.Image) (*bytes.Buffer, error) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	indices := enc.colorIndex.scan(img)

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

	halfWidth := width / 2
	packer := newNibblePacker(halfWidth * height * 2)

	for half := 0; half < 2; half++ {
		for y := 0; y < height; y++ {
			row := indices[y*width+halfWidth*half : y*width+halfWidth*(half+1)]
			for _, index := range row {
				packer.write(index)
			}
		}
	}

	return bytes.NewBuffer(packer.out), nil
}

func (enc *ws13in3EpaperKEncoder) Encode(img image.Image) (*bytes.Buffer, error) {
	indices := enc.colorIndex.scan(img)

	// Read 4 pixels, convert the first two pixels into MSB 4bits, other goes 4bits of LSB.
	out := make([]byte, 0, len(indices)/4)
	var fourpx uint8
	var subBits uint8

	for _, index := range indices {
		fourpx |= index << (2 * (3 - subBits))

		if subBits == 3 {
			out = append(out, fourpx)
			fourpx = 0
		}

		subBits = (subBits + 1) % 4
	}

	return bytes.NewBuffer(out), nil
}

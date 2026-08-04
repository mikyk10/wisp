package color_reduction_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"testing"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/improc/color_reduction"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/stretchr/testify/assert"
)

// TestColorReduction_SameImageEveryTime checks that reducing the same
// photograph twice gives the same picture.
//
// The palette comes out of a map, and Go deliberately randomises the order a
// map is ranged in. Ordering the palette by the panel's own colour index is
// what keeps that randomness out of the result: without it, which colour wins
// a tie depends on the order this particular process happened to receive, and
// under error diffusion one flipped tie propagates to its neighbours and
// spreads. The effect measured 15% of pixels before this was fixed.
//
// Error diffusion is the sensitive case, so it is the one worth testing; that
// no production display is configured to use it is not a reason to leave the
// output unreproducible.
func TestColorReduction_SameImageEveryTime(t *testing.T) {
	display := epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape)

	for _, algorithm := range []config.ColorReductionType{
		config.ColorReductionTypeFloydSteinberg,
		config.ColorReductionTypeSierra3,
		config.ColorReductionTypeBayer,
		config.ColorReductionTypeSimple,
	} {
		t.Run(algorithm, func(t *testing.T) {
			src := gradient(120, 80)

			var want string
			// A fresh processor each time: the palette is built in the
			// constructor, so that is where the ordering has to be pinned.
			for i := 0; i < 20; i++ {
				proc := color_reduction.NewImageColorReduction(display, config.ColorReduction{
					Type: algorithm, Size: 4, Strength: 1.0,
				})
				got, _ := proc.Apply(context.Background(), src, &model.ImgMeta{})

				digest := fingerprint(got)
				if i == 0 {
					want = digest
					continue
				}
				assert.Equal(t, want, digest, "run %d produced a different picture", i+1)
			}
		})
	}
}

// gradient covers enough of the colour space that every palette entry gets
// used, and that some pixels sit equidistant between two of them.
func gradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 0xff / (w - 1)),
				G: uint8(y * 0xff / (h - 1)),
				B: uint8((x + y) * 0xff / (w + h - 2)),
				A: 0xff,
			})
		}
	}
	return img
}

func fingerprint(img image.Image) string {
	b := img.Bounds()
	sum := sha256.New()
	buf := make([]byte, 4)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			buf[0], buf[1], buf[2], buf[3] = uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8)
			sum.Write(buf)
		}
	}
	return hex.EncodeToString(sum.Sum(nil))
}

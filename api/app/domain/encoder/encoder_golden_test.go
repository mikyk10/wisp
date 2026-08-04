package encoder_test

import (
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/encoder"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// updateGolden re-records the encoded bytes instead of comparing against them.
//
//	go test ./app/domain/encoder/ -run Golden -update
var updateGolden = flag.Bool("update", false, "re-record the encoded byte golden")

// TestEncodeGolden pins the exact bytes each panel format produces, and pins
// them across the concrete image types the pipeline can hand the encoder.
//
// The encoder is the last thing between the pipeline and the panel's wire
// protocol: a change here is not visible in any image, only in what the
// hardware draws, so the bytes themselves are the only thing worth checking.
func TestEncodeGolden(t *testing.T) {
	var manifest []string

	for _, panel := range goldenPanels {
		display := panel.display

		// The same picture in each representation the pipeline can produce.
		// imaging returns NRGBA, the timestamp stage returns RGBA, and
		// anything else has to go through the image.Image interface.
		sources := []struct {
			name string
			img  image.Image
		}{
			{"rgba", goldenRGBA(display)},
			{"nrgba", goldenNRGBA(display)},
			{"generic", opaqueWrapper{goldenRGBA(display)}},
		}

		var first []byte
		for _, src := range sources {
			t.Run(panel.name+"/"+src.name, func(t *testing.T) {
				buf, err := encoder.NewWaveshareEPEncoder(display).Encode(src.img)
				require.NoError(t, err)

				got := buf.Bytes()
				if first == nil {
					first = got
					manifest = append(manifest, fmt.Sprintf("%-10s %d bytes %s", panel.name, len(got), hex.EncodeToString(got)))
				} else {
					// Which Go type carries the pixels must not change the wire
					// format; only the pixels themselves may.
					assert.Equal(t, hex.EncodeToString(first), hex.EncodeToString(got),
						"%s representation should encode identically", src.name)
				}
			})
		}
	}

	assertGoldenText(t, "encoded.txt", strings.Join(manifest, "\n")+"\n")
}

var goldenPanels = []struct {
	name    string
	display epaper.DisplayMetadata
}{
	{"ws4in0e", epaper.NewWS4in0E(model.ImgCanonicalOrientationPortrait)},
	{"ws7in3e", epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape)},
	{"ws7in3f", epaper.NewWS7in3F(model.ImgCanonicalOrientationLandscape)},
	{"ws13in3e", epaper.NewWS13in3E(model.ImgCanonicalOrientationPortrait)},
	{"ws13in3k", epaper.NewWS13in3K(model.ImgCanonicalOrientationLandscape)},
}

// Small enough to read in a diff, wide enough for the 13.3 inch encoder to
// split it down the middle.
const goldenW, goldenH = 16, 4

// goldenColors cycles through the panel's own palette and then one colour that
// is not in it, which the encoder is expected to treat as index zero.
func goldenColors(display epaper.DisplayMetadata) []color.Color {
	keys := make([]int, 0, len(display.Palette()))
	for k := range display.Palette() {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	out := make([]color.Color, 0, len(keys)+1)
	for _, k := range keys {
		out = append(out, display.Palette()[k])
	}
	return append(out, color.RGBA{R: 0x07, G: 0x07, B: 0x07, A: 0xff})
}

func goldenRGBA(display epaper.DisplayMetadata) *image.RGBA {
	colors := goldenColors(display)
	img := image.NewRGBA(image.Rect(0, 0, goldenW, goldenH))
	for y := 0; y < goldenH; y++ {
		for x := 0; x < goldenW; x++ {
			img.Set(x, y, colors[(y*goldenW+x)%len(colors)])
		}
	}
	return img
}

func goldenNRGBA(display epaper.DisplayMetadata) *image.NRGBA {
	colors := goldenColors(display)
	img := image.NewNRGBA(image.Rect(0, 0, goldenW, goldenH))
	for y := 0; y < goldenH; y++ {
		for x := 0; x < goldenW; x++ {
			img.Set(x, y, colors[(y*goldenW+x)%len(colors)])
		}
	}
	return img
}

// opaqueWrapper hides the concrete type, forcing the encoder down the generic
// image.Image path.
type opaqueWrapper struct{ image.Image }

func assertGoldenText(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata/golden", name)

	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata/golden", 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "golden missing; re-record with: go test ./app/domain/encoder/ -run Golden -update")
	require.Equal(t, string(want), got)
}

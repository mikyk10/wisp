package ortho_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/mikyk10/wisp/app/domain/improc/ortho"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// operations restates, independently of the implementation, where each
// operation sends the pixel at (x,y) of a w×h image. The library call that
// Apply dispatches to is checked against this, so a mix-up between the
// clockwise convention used by the pipeline and the counter-clockwise one used
// by the library cannot pass unnoticed.
var operations = []struct {
	op   ortho.Op
	dest func(x, y, w, h int) (int, int)
}{
	{ortho.Identity, func(x, y, _, _ int) (int, int) { return x, y }},
	{ortho.Rotate90CW, func(x, y, _, h int) (int, int) { return h - 1 - y, x }},
	{ortho.Rotate180, func(x, y, w, h int) (int, int) { return w - 1 - x, h - 1 - y }},
	{ortho.Rotate270CW, func(x, y, w, _ int) (int, int) { return y, w - 1 - x }},
	{ortho.FlipH, func(x, y, w, _ int) (int, int) { return w - 1 - x, y }},
	{ortho.FlipV, func(x, y, _, h int) (int, int) { return x, h - 1 - y }},
	{ortho.Transpose, func(x, y, _, _ int) (int, int) { return y, x }},
	{ortho.Transverse, func(x, y, w, h int) (int, int) { return h - 1 - y, w - 1 - x }},
}

func TestApply_MatchesIndexPermutation(t *testing.T) {
	const w, h = 7, 5 // non-square, and coprime, so a transposed result cannot pass by luck

	for _, tt := range operations {
		t.Run(tt.op.String(), func(t *testing.T) {
			src := newCoordinateImage(w, h)
			got := ortho.Apply(src, tt.op)

			// Expected dimensions come from the reference mapping rather than
			// from SwapsAxes, which is itself under test here.
			wantW, wantH := destExtent(tt.dest, w, h)
			require.Equal(t, wantW, got.Bounds().Dx(), "output width")
			require.Equal(t, wantH, got.Bounds().Dy(), "output height")

			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					dx, dy := tt.dest(x, y, w, h)
					wr, wg, wb, wa := src.At(x, y).RGBA()
					gr, gg, gb, ga := got.At(got.Bounds().Min.X+dx, got.Bounds().Min.Y+dy).RGBA()
					require.Equal(t,
						[4]uint32{wr, wg, wb, wa}, [4]uint32{gr, gg, gb, ga},
						"source (%d,%d) should land at (%d,%d)", x, y, dx, dy)

					// ApplyPoint must predict exactly what Apply does, since
					// callers use it to locate a region without moving pixels.
					require.Equal(t, image.Pt(dx, dy),
						ortho.ApplyPoint(image.Pt(x, y), tt.op, w, h),
						"ApplyPoint disagrees with Apply for (%d,%d)", x, y)
				}
			}
		})
	}
}

// TestSwapsAxes_AgreesWithApply guards the helper the callers use to predict
// the output shape before performing the operation.
func TestSwapsAxes_AgreesWithApply(t *testing.T) {
	const w, h = 7, 5

	for _, tt := range operations {
		t.Run(tt.op.String(), func(t *testing.T) {
			got := ortho.Apply(newCoordinateImage(w, h), tt.op)
			assert.Equal(t, ortho.SwapsAxes(tt.op), got.Bounds().Dx() == h && got.Bounds().Dy() == w)
		})
	}
}

// TestApply_IdentityReturnsSourceUnchanged pins the contract the zero-degree
// path depends on: no copy and no colour-model conversion. A decoded JPEG
// stays YCbCr, which is what keeps an uncorrected image from paying for a
// full-frame conversion it never needed.
func TestApply_IdentityReturnsSourceUnchanged(t *testing.T) {
	src := image.NewYCbCr(image.Rect(0, 0, 8, 8), image.YCbCrSubsampleRatio420)

	got := ortho.Apply(src, ortho.Identity)

	assert.Same(t, src, got)
}

func TestFromAngleCW(t *testing.T) {
	tests := []struct {
		deg    float64
		want   ortho.Op
		wantOK bool
	}{
		{0, ortho.Identity, true},
		{90, ortho.Rotate90CW, true},
		{180, ortho.Rotate180, true},
		{270, ortho.Rotate270CW, true},
		{360, ortho.Identity, true},
		{-90, ortho.Rotate270CW, true},
		{-180, ortho.Rotate180, true},
		{-270, ortho.Rotate90CW, true},
		{45, ortho.Identity, false},
		{1, ortho.Identity, false},
	}

	for _, tt := range tests {
		got, ok := ortho.FromAngleCW(tt.deg)
		assert.Equal(t, tt.wantOK, ok, "%.0f degrees supported", tt.deg)
		assert.Equal(t, tt.want, got, "%.0f degrees maps to %s", tt.deg, tt.want)
	}
}

// TestFromAngleCW_InverseCancels checks the property the timestamp stage
// relies on: rotating by an angle and then by its negation is a no-op.
func TestFromAngleCW_InverseCancels(t *testing.T) {
	const w, h = 7, 5

	for _, deg := range []float64{0, 90, 180, 270, -90} {
		forward, ok := ortho.FromAngleCW(deg)
		require.True(t, ok)
		back, ok := ortho.FromAngleCW(-deg)
		require.True(t, ok)

		src := newCoordinateImage(w, h)
		got := ortho.Apply(ortho.Apply(src, forward), back)

		require.Equal(t, w, got.Bounds().Dx())
		require.Equal(t, h, got.Bounds().Dy())
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				require.Equal(t, colorOf(src, x, y), colorOf(got, x, y),
					"%.0f degrees then %.0f should restore (%d,%d)", deg, -deg, x, y)
			}
		}
	}
}

// TestCompose_MatchesSequentialApplication checks every one of the 64 entries
// in the composition table against actually performing the two operations one
// after the other. The table is written out by hand, so this is what makes it
// trustworthy.
func TestCompose_MatchesSequentialApplication(t *testing.T) {
	const w, h = 7, 5

	for _, first := range operations {
		for _, second := range operations {
			t.Run(first.op.String()+"_then_"+second.op.String(), func(t *testing.T) {
				src := newCoordinateImage(w, h)

				want := ortho.Apply(ortho.Apply(src, first.op), second.op)
				got := ortho.Apply(src, ortho.Compose(first.op, second.op))

				require.Equal(t, want.Bounds().Dx(), got.Bounds().Dx(), "output width")
				require.Equal(t, want.Bounds().Dy(), got.Bounds().Dy(), "output height")
				for y := 0; y < want.Bounds().Dy(); y++ {
					for x := 0; x < want.Bounds().Dx(); x++ {
						require.Equal(t, colorOf(want, x, y), colorOf(got, x, y),
							"%s then %s should equal %s at (%d,%d)",
							first.op, second.op, ortho.Compose(first.op, second.op), x, y)
					}
				}
			})
		}
	}
}

// TestCompose_FormsAGroup states the properties the fusion in crop relies on:
// Identity does nothing, and every operation can be undone, so composing can
// never strand an image in an orientation the pipeline cannot express.
func TestCompose_FormsAGroup(t *testing.T) {
	for _, tt := range operations {
		assert.Equal(t, tt.op, ortho.Compose(ortho.Identity, tt.op), "identity before %s", tt.op)
		assert.Equal(t, tt.op, ortho.Compose(tt.op, ortho.Identity), "identity after %s", tt.op)

		var inverses int
		for _, other := range operations {
			if ortho.Compose(tt.op, other.op) == ortho.Identity {
				inverses++
			}
		}
		assert.Equal(t, 1, inverses, "%s should have exactly one inverse", tt.op)
	}
}

// destExtent derives the output dimensions of a mapping by looking at where it
// sends the four corners of a w×h source.
func destExtent(dest func(x, y, w, h int) (int, int), w, h int) (int, int) {
	maxX, maxY := 0, 0
	for _, c := range [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}} {
		x, y := dest(c[0], c[1], w, h)
		maxX = max(maxX, x)
		maxY = max(maxY, y)
	}
	return maxX + 1, maxY + 1
}

// newCoordinateImage builds an image whose pixels encode their own position,
// so that any permutation of them can be verified exactly.
func newCoordinateImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x + 1), G: uint8(y + 1), B: uint8(x*h + y), A: 0xff})
		}
	}
	return img
}

func colorOf(img image.Image, x, y int) [4]uint32 {
	r, g, b, a := img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y).RGBA()
	return [4]uint32{r, g, b, a}
}

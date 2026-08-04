package improc_test

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/improc/crop"
	"github.com/mikyk10/wisp/app/domain/improc/exif_rotation"
	"github.com/mikyk10/wisp/app/domain/improc/ortho"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// updateGolden re-records the golden files instead of comparing against them.
//
//	go test ./app/domain/improc/ -run Golden -update
//
// Re-recording is only legitimate when the pixel change is understood and
// intended (e.g. swapping an interpolating rotation for an exact one). Inspect
// the resulting git diff before committing it.
var updateGolden = flag.Bool("update", false, "re-record golden files for the pre-processing pipeline")

const goldenDir = "testdata/golden"

// TestPreSeqGolden pins the exact output of the pre-processing sequence
// (exif_rotation -> crop) for every combination of EXIF orientation, installed
// display orientation and source aspect ratio.
//
// The two processors carry the whole geometric contract of the delivery path:
// by the time crop.Apply returns, the pixels must be in the installed
// orientation at the panel's exact dimensions. A mistake in the rotation
// convention (clockwise vs counter-clockwise) still produces a perfectly
// plausible-looking image, so only an exhaustive pixel-level comparison
// reliably catches it.
func TestPreSeqGolden(t *testing.T) {
	var manifest []string

	for _, aspect := range goldenAspects {
		for _, installed := range goldenInstallations {
			for exifOrientation := 1; exifOrientation <= 8; exifOrientation++ {
				name := fmt.Sprintf("exif%d_%s_%s", exifOrientation, aspect.name, installed.name)

				t.Run(name, func(t *testing.T) {
					src := newGoldenFixture(aspect.w, aspect.h)
					meta := &model.ImgMeta{
						ExifOrientation:    model.ExifOrientation(exifOrientation),
						HasExifSubjectArea: true,
						// Deliberately off-centre and off-diagonal so that every
						// element of the transformation group maps it somewhere
						// distinct.
						ExifSubjectArea: image.Point{X: aspect.w / 4, Y: aspect.h / 8},
					}

					seq := improc.NewSequencer()
					seq.Push(exif_rotation.NewDeferredExifRotation())
					// exif_subject makes the crop window itself depend on the
					// transformed subject point, so a regression in the point
					// arithmetic shows up in the pixels too.
					seq.Push(crop.NewImageCropper(goldenDisplay{installed: installed.orientation}, config.CropStrategyExifSubject))

					got, gotMeta := seq.Apply(t.Context(), src, meta)

					assert.Equal(t, ortho.Identity, gotMeta.PendingExifOp,
						"crop should have consumed the deferred operation")

					manifest = append(manifest, fmt.Sprintf("%-34s bounds=%dx%d subject=(%d,%d) angle=%.0f",
						name,
						got.Bounds().Dx(), got.Bounds().Dy(),
						gotMeta.ExifSubjectArea.X, gotMeta.ExifSubjectArea.Y,
						gotMeta.RequiredCorrectionAngle,
					))

					assertGoldenImage(t, name, got)
				})
			}
		}
	}

	assertGoldenText(t, "manifest.txt", strings.Join(manifest, "\n")+"\n")
}

// TestPreSeqGolden_DeferredMatchesImmediate checks that folding the EXIF
// normalisation into crop's own rotation produces exactly what performing them
// one after the other does.
//
// This is the whole claim behind the deferred form. It holds because the eight
// operations are closed under composition, but "closed under composition" is
// not something a reader can check by looking at the pixels, so it is checked
// here across every case the golden test covers.
func TestPreSeqGolden_DeferredMatchesImmediate(t *testing.T) {
	for _, aspect := range goldenAspects {
		for _, installed := range goldenInstallations {
			for exifOrientation := 1; exifOrientation <= 8; exifOrientation++ {
				name := fmt.Sprintf("exif%d_%s_%s", exifOrientation, aspect.name, installed.name)

				t.Run(name, func(t *testing.T) {
					newMeta := func() *model.ImgMeta {
						return &model.ImgMeta{
							ExifOrientation:    model.ExifOrientation(exifOrientation),
							HasExifSubjectArea: true,
							ExifSubjectArea:    image.Point{X: aspect.w / 4, Y: aspect.h / 8},
						}
					}
					cropper := func() improc.ImageProcessor {
						return crop.NewImageCropper(goldenDisplay{installed: installed.orientation}, config.CropStrategyExifSubject)
					}

					immediate := improc.NewSequencer()
					immediate.Push(exif_rotation.NewExifRotation())
					immediate.Push(cropper())
					wantImg, wantMeta := immediate.Apply(t.Context(), newGoldenFixture(aspect.w, aspect.h), newMeta())

					deferred := improc.NewSequencer()
					deferred.Push(exif_rotation.NewDeferredExifRotation())
					deferred.Push(cropper())
					gotImg, gotMeta := deferred.Apply(t.Context(), newGoldenFixture(aspect.w, aspect.h), newMeta())

					if _, _, ok := firstPixelDifference(wantImg, gotImg); !ok {
						t.Fatal("deferring the EXIF normalisation changed the pixels")
					}
					assert.Equal(t, wantMeta.ExifSubjectArea, gotMeta.ExifSubjectArea, "subject point")
					assert.Equal(t, wantMeta.ImageOrientation, gotMeta.ImageOrientation, "image orientation")
					assert.Equal(t, wantMeta.RequiredCorrectionAngle, gotMeta.RequiredCorrectionAngle, "correction angle")
				})
			}
		}
	}
}

// goldenAspects covers a landscape and a portrait source, so that the
// image-orientation branch inside crop is exercised in both directions.
//
// Both are deliberately more elongated than the 3:2 test panel. If the source
// matched the panel ratio the crop window would always cover the whole image,
// the subject point would stop influencing it, and the coordinate arithmetic
// would no longer be observable in the pixels.
var goldenAspects = []struct {
	name string
	w, h int
}{
	{"landscape", 360, 200},
	{"portrait", 200, 360},
}

// goldenInstallations covers both installed orientations. Against a
// natively-landscape panel this reaches every correction angle crop can
// produce (-90, 0 and +90).
var goldenInstallations = []struct {
	name        string
	orientation model.CanonicalOrientation
}{
	{"inst-landscape", model.ImgCanonicalOrientationLandscape},
	{"inst-portrait", model.ImgCanonicalOrientationPortrait},
}

// goldenDisplay is a stand-in panel for the geometry tests. The pipeline's
// rotation and crop arithmetic is independent of the panel size, and a small
// panel keeps the golden PNGs small enough to stay readable in a diff.
type goldenDisplay struct {
	installed model.CanonicalOrientation
}

func (d goldenDisplay) ModelName() string { return "golden120x80" }
func (d goldenDisplay) Width() int        { return 120 }
func (d goldenDisplay) Height() int       { return 80 }

func (d goldenDisplay) NativeOrientation() model.CanonicalOrientation {
	return model.ImgCanonicalOrientationLandscape
}

func (d goldenDisplay) InstalledOrientation() model.CanonicalOrientation { return d.installed }

func (d goldenDisplay) Palette() epaper.IndexPalette { return nil }

// newGoldenFixture builds an image whose pixels encode their own coordinates:
// red carries x, green carries y. A rotation applied in the wrong direction
// therefore cannot coincidentally reproduce the expected pixels. The blue
// wedge in the upper-left breaks the remaining 180-degree symmetry of the
// gradient pair after the image has been cropped and resized.
func newGoldenFixture(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var blue uint8
			if x < w/4 && y < h/8 {
				blue = 0xff
			}
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 0xff / (w - 1)),
				G: uint8(y * 0xff / (h - 1)),
				B: blue,
				A: 0xff,
			})
		}
	}
	return img
}

func assertGoldenImage(t *testing.T, name string, got image.Image) {
	t.Helper()

	path := filepath.Join(goldenDir, name+".png")

	if *updateGolden {
		require.NoError(t, os.MkdirAll(goldenDir, 0o755))
		f, err := os.Create(path)
		require.NoError(t, err)
		defer f.Close()
		require.NoError(t, png.Encode(f, got))
		return
	}

	f, err := os.Open(path)
	require.NoError(t, err, "golden file missing; re-record with: go test ./app/domain/improc/ -run Golden -update")
	defer f.Close()

	want, err := png.Decode(f)
	require.NoError(t, err)

	require.Equal(t, want.Bounds().Dx(), got.Bounds().Dx(), "output width")
	require.Equal(t, want.Bounds().Dy(), got.Bounds().Dy(), "output height")

	if x, y, ok := firstPixelDifference(want, got); !ok {
		wr, wg, wb, wa := want.At(want.Bounds().Min.X+x, want.Bounds().Min.Y+y).RGBA()
		gr, gg, gb, ga := got.At(got.Bounds().Min.X+x, got.Bounds().Min.Y+y).RGBA()
		t.Fatalf("pixel mismatch at (%d,%d): want rgba(%d,%d,%d,%d), got rgba(%d,%d,%d,%d)\n"+
			"re-record with: go test ./app/domain/improc/ -run Golden -update", x, y, wr, wg, wb, wa, gr, gg, gb, ga)
	}
}

// firstPixelDifference reports the first differing pixel in reading order.
// Both images are compared in their own coordinate space so that the concrete
// image type (and therefore the origin) does not matter.
func firstPixelDifference(want, got image.Image) (x, y int, equal bool) {
	wb, gb := want.Bounds(), got.Bounds()
	for dy := 0; dy < wb.Dy(); dy++ {
		for dx := 0; dx < wb.Dx(); dx++ {
			wr, wg, wbl, wa := want.At(wb.Min.X+dx, wb.Min.Y+dy).RGBA()
			gr, gg, gbl, ga := got.At(gb.Min.X+dx, gb.Min.Y+dy).RGBA()
			if wr != gr || wg != gg || wbl != gbl || wa != ga {
				return dx, dy, false
			}
		}
	}
	return 0, 0, true
}

// assertGoldenText pins the non-pixel outputs of the pipeline: the output
// bounds, the transformed subject point and the correction angle handed to the
// post-processing stage. Unlike the images, these are expected to survive every
// phase of the speedup work unchanged.
func assertGoldenText(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join(goldenDir, name)

	if *updateGolden {
		require.NoError(t, os.MkdirAll(goldenDir, 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "golden file missing; re-record with: go test ./app/domain/improc/ -run Golden -update")
	require.Equal(t, string(want), got)
}

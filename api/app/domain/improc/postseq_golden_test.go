package improc_test

import (
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/improc/color_reduction"
	"github.com/mikyk10/wisp/app/domain/improc/rotation"
	"github.com/mikyk10/wisp/app/domain/improc/timestamp"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
)

// TestPostSeqProducesEncodableType pins the concrete type the post-processing
// sequence hands to the encoder.
//
// The encoder reads *image.RGBA and *image.NRGBA straight out of their pixel
// buffers and falls back to image.Image.At for anything else, which costs a
// heap allocation per pixel: on the 13.3 inch panel that is 1.9 million of
// them, and the difference is roughly six times the encoding time. Nothing
// about the output would change, so only this test would notice.
func TestPostSeqProducesEncodableType(t *testing.T) {
	display := epaper.NewWS7in3E(model.ImgCanonicalOrientationLandscape)
	shot := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)

	for _, algorithm := range []config.ColorReductionType{
		config.ColorReductionTypeSimple,
		config.ColorReductionTypeBayer,
		config.ColorReductionTypeFloydSteinberg,
		config.ColorReductionTypeSierra3,
	} {
		for _, flip := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_flip=%v", algorithm, flip), func(t *testing.T) {
				seq := improc.NewSequencer()
				seq.Push(color_reduction.NewImageColorReduction(display, config.ColorReduction{
					Type: algorithm, Size: 4, Strength: 1.0,
				}))
				seq.Push(timestamp.NewTimstamp())
				if flip {
					seq.Push(rotation.NewRotation())
				}

				got, _ := seq.Apply(t.Context(), newGoldenFixture(80, 48), &model.ImgMeta{ExifDateTime: shot})

				switch got.(type) {
				case *image.RGBA, *image.NRGBA:
				default:
					t.Fatalf("post-processing produced %T; the encoder has no direct case for it "+
						"and would fall back to a per-pixel allocation", got)
				}
			})
		}
	}
}

// TestTimestampGolden pins where the date badge lands for every correction
// angle the pipeline can produce.
//
// The badge belongs in the bottom right of the photograph, which is only the
// bottom right of the panel when the two orientations agree. Which corner it
// ends up in is the whole content of this stage, and it is easy to get wrong in
// a way that reads as plausible, so all four are recorded.
func TestTimestampGolden(t *testing.T) {
	shot := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)

	for _, angle := range []float64{0, 90, -90, 180} {
		name := fmt.Sprintf("timestamp_angle%.0f", angle)

		t.Run(name, func(t *testing.T) {
			// Wide enough for the 74px badge in either orientation.
			src := newGoldenFixture(160, 100)
			meta := &model.ImgMeta{
				ExifDateTime:            shot,
				RequiredCorrectionAngle: angle,
			}

			got, _ := timestamp.NewTimstamp().Apply(t.Context(), src, meta)

			assertGoldenImage(t, name, got)
		})
	}
}

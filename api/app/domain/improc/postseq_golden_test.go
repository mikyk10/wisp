package improc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/improc/timestamp"
	"github.com/mikyk10/wisp/app/domain/model"
)

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

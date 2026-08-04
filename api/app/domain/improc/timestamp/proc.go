package timestamp

import (
	"context"
	"image"
	"image/draw"
	"log/slog"
	"time"

	"github.com/mikyk10/wisp/app/domain/display/epaper/wsdisplay"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/improc/ortho"
	"github.com/mikyk10/wisp/app/domain/model"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const dateLayout = "2006/01/02"

// Badge geometry, in the orientation of the photograph. The width is what the
// 7x13 face needs for the date plus a margin on either side.
const (
	badgeWidth  = 74
	badgeHeight = 15

	// textInset is the gap between the left edge of the badge and the first
	// glyph; baseline is measured down from the top of the badge.
	textInset = 2
	baseline  = badgeHeight - 3
)

type processor struct{}

func NewTimstamp() improc.ImageProcessor {
	return &processor{}
}

// Apply burns the date into the bottom right corner of the photograph.
//
// The bottom right of the photograph is not the bottom right of the panel
// unless the two orientations happen to agree, so the corner has to be located
// rather than assumed. Turning the image upright, drawing, and turning it back
// would do that, at the cost of two passes over every pixel; instead the badge
// is drawn at 74x15, turned, and dropped into the corner it belongs in.
func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {

	// skip if no exif data found
	if meta.ExifDateTime.IsZero() {
		return src, meta
	}

	// The operation that took the photograph into the panel's orientation.
	installed, ok := ortho.FromAngleCW(meta.RequiredCorrectionAngle)
	if !ok {
		slog.Warn("timestamp: correction angle is not a quarter turn, placing the badge as if upright",
			"angle", meta.RequiredCorrectionAngle)
	}

	bounds := src.Bounds()

	// Dimensions of the photograph before that operation.
	uprightW, uprightH := bounds.Dx(), bounds.Dy()
	if ortho.SwapsAxes(installed) {
		uprightW, uprightH = uprightH, uprightW
	}

	// Follow the two opposite corners of the badge through the same operation
	// to find the region it occupies now. Corners are inclusive here, hence
	// the final step out to an exclusive rectangle.
	near := ortho.ApplyPoint(image.Pt(uprightW-badgeWidth, uprightH-badgeHeight), installed, uprightW, uprightH)
	far := ortho.ApplyPoint(image.Pt(uprightW-1, uprightH-1), installed, uprightW, uprightH)
	region := image.Rectangle{Min: near, Max: far}.Canon()
	region.Max = region.Max.Add(image.Pt(1, 1))

	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)
	draw.Draw(dst, region.Add(bounds.Min), ortho.Apply(renderBadge(meta.ExifDateTime), installed), image.Point{}, draw.Src)

	return dst, meta
}

// renderBadge draws the date on a canvas of its own, in the orientation of the
// photograph.
func renderBadge(shot time.Time) image.Image {
	badge := image.NewRGBA(image.Rect(0, 0, badgeWidth, badgeHeight))
	draw.Draw(badge, badge.Bounds(), &image.Uniform{C: wsdisplay.Black}, image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  badge,
		Src:  image.NewUniform(wsdisplay.White), // text color.
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(textInset), Y: fixed.I(baseline)},
	}
	d.DrawString(shot.Format(dateLayout))

	return badge
}

package timestamp

import (
	"context"
	"image"
	"image/draw"

	"github.com/anthonynsimon/bild/blend"
	"github.com/mikyk10/wisp/app/domain/display/epaper/wsdisplay"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/improc/ortho"
	"github.com/mikyk10/wisp/app/domain/model"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type processor struct{}

func NewTimstamp() improc.ImageProcessor {
	return &processor{}
}

// Burn the timestamp on the bottom right of the image when it is available
func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {

	// skip if no exif data found
	if meta.ExifDateTime.IsZero() {
		return src, meta
	}

	// Undo the display-orientation correction, so that "bottom right" below
	// means the bottom right of the photograph rather than of the panel. Both
	// operations are exact permutations, which matters here because the image
	// has already been reduced to the panel's palette and must not acquire
	// colours outside it.
	upright, _ := ortho.FromAngleCW(-meta.RequiredCorrectionAngle)
	installed, _ := ortho.FromAngleCW(meta.RequiredCorrectionAngle)
	src = ortho.Apply(src, upright)

	bound := src.Bounds()
	width := bound.Max.X
	height := bound.Max.Y

	rectX := width - 74  // width+padding from right
	rectY := height - 15 // height+padding from bottom

	// canvas and draw a bounding box on it
	fgcanvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(fgcanvas, image.Rect(rectX, rectY, width, height), &image.Uniform{wsdisplay.Black}, image.Point{}, draw.Src)

	// draw text on the canvas
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  fgcanvas,
		Src:  image.NewUniform(wsdisplay.White), // text color.
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(rectX + 2), Y: fixed.I(height - 3)},
	}

	d.DrawString(meta.ExifDateTime.Format("2006/01/02"))
	result := blend.Normal(src, fgcanvas)

	// put the image back into the installed orientation
	return ortho.Apply(result, installed), meta
}

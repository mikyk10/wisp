package crop

import (
	"context"
	"fmt"
	"image"
	"log/slog"

	"github.com/disintegration/imaging"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/improc/ortho"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/sunshineplan/imgconv"
)

type processor struct {
	epd      epaper.DisplayMetadata
	strategy config.CropStrategy
}

func NewImageCropper(epd epaper.DisplayMetadata, strategy config.CropStrategy) improc.ImageProcessor {
	return &processor{epd: epd, strategy: strategy}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {
	return p.resize(p.crop(src, meta)), meta
}

func (p *processor) crop(img image.Image, meta *model.ImgMeta) image.Image {

	var angle = 0.0

	if p.epd.NativeOrientation() != p.epd.InstalledOrientation() {
		angle += -90
	}

	if p.epd.InstalledOrientation() != meta.ImageOrientation {
		angle += 90
	}

	meta.RequiredCorrectionAngle = angle

	// The angle is 0 or ±90 by construction above, so it is always a quarter
	// turn and always an exact index permutation. An unexpected value leaves
	// the image alone rather than corrupting it.
	correction, ok := ortho.FromAngleCW(angle)
	if !ok {
		slog.Warn("crop: correction angle is not a quarter turn, skipping rotation", "angle", angle)
	}

	// exif_rotation may have left its normalisation for this stage to carry
	// out. Performing both as one operation walks a full-resolution image once
	// instead of twice, and skips it altogether when they cancel out.
	pending := meta.PendingExifOp
	meta.PendingExifOp = ortho.Identity

	// The subject point is expressed in the coordinates exif_rotation reported,
	// which are the source dimensions with the pending operation applied.
	preW, preH := img.Bounds().Max.X, img.Bounds().Max.Y
	if ortho.SwapsAxes(pending) {
		preW, preH = preH, preW
	}

	img = ortho.Apply(img, ortho.Compose(pending, correction))
	bounds := img.Bounds()

	// apply display-orientation correction to subject area coordinates
	if meta.HasExifSubjectArea {
		meta.ExifSubjectArea = rotatePointByAngle(meta.ExifSubjectArea, angle, preW, preH)
	}

	hwAspectRatioX := float64(p.epd.Width()) / float64(p.epd.Height())
	hwAspectRatioY := float64(p.epd.Height()) / float64(p.epd.Width())

	// image cropping
	calculatedX1 := float64(bounds.Max.Y) * hwAspectRatioX
	calculatedY1 := float64(bounds.Max.X) * hwAspectRatioY

	if calculatedX1 > float64(bounds.Max.X) {
		calculatedX1 = float64(bounds.Max.X)
	}

	if calculatedY1 > float64(bounds.Max.Y) {
		calculatedY1 = float64(bounds.Max.Y)
	}

	cropW := int(calculatedX1)
	cropH := int(calculatedY1)

	offsetX, offsetY := p.cropOffset(bounds, cropW, cropH, meta)

	// imaging reads the requested window straight out of the source, whatever
	// colour model it is in. The alternative in bild converts the entire frame
	// to RGBA before taking the window, which for an uncorrected image is the
	// single most expensive thing the pre-processing stage does.
	return imaging.Crop(img, image.Rect(offsetX, offsetY, cropW+offsetX, cropH+offsetY))
}

// cropOffset returns the top-left corner of the crop rectangle based on the active strategy.
func (p *processor) cropOffset(bounds image.Rectangle, cropW, cropH int, meta *model.ImgMeta) (int, int) {
	centerX := (bounds.Max.X - cropW) / 2
	centerY := (bounds.Max.Y - cropH) / 2

	if p.strategy != config.CropStrategyExifSubject || !meta.HasExifSubjectArea {
		return centerX, centerY
	}

	sx, sy := meta.ExifSubjectArea.X, meta.ExifSubjectArea.Y
	offsetX := clamp(sx-cropW/2, 0, bounds.Max.X-cropW)
	offsetY := clamp(sy-cropH/2, 0, bounds.Max.Y-cropH)
	slog.Debug("crop: exif_subject offset",
		"subject", meta.ExifSubjectArea,
		"cropSize", fmt.Sprintf("%dx%d", cropW, cropH),
		"imageSize", fmt.Sprintf("%dx%d", bounds.Max.X, bounds.Max.Y),
		"offset", fmt.Sprintf("(%d,%d)", offsetX, offsetY),
		"center", fmt.Sprintf("(%d,%d)", centerX, centerY),
	)
	return offsetX, offsetY
}

// rotatePointByAngle applies the same rotation used in crop() to a point.
// preW and preH are the image dimensions before rotation.
func rotatePointByAngle(p image.Point, angle float64, preW, preH int) image.Point {
	x, y := p.X, p.Y
	W, H := preW-1, preH-1
	switch angle {
	case 90:
		return image.Point{X: H - y, Y: x}
	case -90:
		return image.Point{X: y, Y: W - x}
	default: // 0°
		return p
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (p *processor) resize(img image.Image) image.Image {
	// resize the image into exactly the display module's specification after crop
	return imgconv.Resize(img, &imgconv.ResizeOption{Width: p.epd.Width(), Height: p.epd.Height()})
}

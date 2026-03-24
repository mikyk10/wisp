package crop

import (
	"context"
	"fmt"
	"image"
	"log/slog"

	"github.com/anthonynsimon/bild/transform"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/improc"
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

	preBounds := img.Bounds()
	img = transform.Rotate(img, angle, &transform.RotationOptions{ResizeBounds: true})
	bounds := img.Bounds()

	// apply display-orientation correction to subject area coordinates
	if meta.HasExifSubjectArea {
		meta.ExifSubjectArea = rotatePointByAngle(meta.ExifSubjectArea, angle, preBounds.Max.X, preBounds.Max.Y)
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

	return transform.Crop(img, image.Rect(offsetX, offsetY, cropW+offsetX, cropH+offsetY))
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

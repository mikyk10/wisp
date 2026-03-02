package crop

import (
	"context"
	"image"
	"wspf/app/domain/display/epaper"
	"wspf/app/domain/improc"
	"wspf/app/domain/model"

	"github.com/anthonynsimon/bild/transform"
	"github.com/sunshineplan/imgconv"
)

type processor struct {
	epd epaper.DisplayMetadata
}

func NewImageCropper(epd epaper.DisplayMetadata) improc.ImageProcessor {
	return &processor{epd: epd}
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

	img = transform.Rotate(img, angle, &transform.RotationOptions{ResizeBounds: true})
	bounds := img.Bounds()

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

	// number of pixels to shorten
	diffX := float64(bounds.Max.X) - calculatedX1
	diffY := float64(bounds.Max.Y) - calculatedY1

	// crop remainder should be equally distributed to left and right edges; calculate the image center point
	cropped := transform.Crop(img, image.Rect(int(diffX/2), int(diffY/2), int(calculatedX1+(diffX/2)), int(calculatedY1+(diffY/2))))

	return cropped
}

func (p *processor) resize(img image.Image) image.Image {
	// resize the image into exactly the display module's specification after crop
	return imgconv.Resize(img, &imgconv.ResizeOption{Width: p.epd.Width(), Height: p.epd.Height()})
}

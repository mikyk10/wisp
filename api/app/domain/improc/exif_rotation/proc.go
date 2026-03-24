package exif_rotation

import (
	"context"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/model"
	"image"

	"github.com/anthonynsimon/bild/transform"
)

type processor struct{}

func NewExifRotation() improc.ImageProcessor {
	return &processor{}

}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {

	img := src

	// https://qiita.com/yoya/items/4e14f696e1afd5a54403
	switch meta.ExifOrientation {
	case model.NoExifOrientation:
		fallthrough
	case 1:
	case 2:
		img = transform.FlipH(img)

	case 3:
		img = transform.Rotate(img, 180, &transform.RotationOptions{ResizeBounds: true})

	case 4:
		img = transform.FlipV(img)

	case 5:
		img = transform.Rotate(img, 90, &transform.RotationOptions{ResizeBounds: true})
		img = transform.FlipH(img)
	case 6:
		img = transform.Rotate(img, 90, &transform.RotationOptions{ResizeBounds: true})
	case 7:
		img = transform.Rotate(img, -90, &transform.RotationOptions{ResizeBounds: true})
		img = transform.FlipH(img)
	case 8:
		img = transform.Rotate(img, -90, &transform.RotationOptions{ResizeBounds: true})
	}

	//TODO: allow square images to be displayed in either orientation
	// an empty image may arrive
	xyp := xyPropotion(img)
	switch xyp {
	case -1:
		meta.ImageOrientation = model.ImgCanonicalOrientationPortrait
	case 0:
		fallthrough
	case 1:
		meta.ImageOrientation = model.ImgCanonicalOrientationLandscape
	}

	if meta.HasExifSubjectArea {
		origW := src.Bounds().Max.X
		origH := src.Bounds().Max.Y
		meta.ExifSubjectArea = transformSubjectPointByOrientation(meta.ExifSubjectArea, meta.ExifOrientation, origW, origH)
	}

	return img, meta
}

// transformSubjectPointByOrientation transforms a point from the original image coordinate
// system to the post-ExifOrientation coordinate system. origW and origH are the dimensions
// of the image before any rotation.
func transformSubjectPointByOrientation(p image.Point, o model.ExifOrientation, origW, origH int) image.Point {
	x, y := p.X, p.Y
	W, H := origW-1, origH-1
	switch o {
	case 2:
		return image.Point{X: W - x, Y: y}
	case 3:
		return image.Point{X: W - x, Y: H - y}
	case 4:
		return image.Point{X: x, Y: H - y}
	case 5: // rotate 90° CW then FlipH = transpose
		return image.Point{X: y, Y: x}
	case 6: // rotate 90° CW
		return image.Point{X: H - y, Y: x}
	case 7: // rotate -90° then FlipH = transverse
		return image.Point{X: H - y, Y: W - x}
	case 8: // rotate -90° (CCW)
		return image.Point{X: y, Y: W - x}
	default: // case 1 (normal) and unknown
		return p
	}
}

func xyPropotion(img image.Image) int {
	bounds := img.Bounds()
	if bounds.Max.X < bounds.Max.Y {
		return -1
	} else if bounds.Max.X > bounds.Max.Y {
		return 1
	} else {
		return 0
	}
}

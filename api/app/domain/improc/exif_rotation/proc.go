package exif_rotation

import (
	"context"
	"image"

	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/improc/ortho"
	"github.com/mikyk10/wisp/app/domain/model"
)

type processor struct{}

func NewExifRotation() improc.ImageProcessor {
	return &processor{}

}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {

	img := ortho.Apply(src, opForOrientation(meta.ExifOrientation))

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

// opForOrientation returns the transformation that brings an image stored with
// the given EXIF orientation tag upright.
// https://qiita.com/yoya/items/4e14f696e1afd5a54403
//
//	EXIF  operation          previously
//	1     none               none
//	2     flip horizontally  flip horizontally
//	3     rotate 180°        rotate 180°
//	4     flip vertically    flip vertically
//	5     transpose          rotate 90° clockwise, then flip horizontally
//	6     rotate 90° CW      rotate 90° clockwise
//	7     transverse         rotate 90° counter-clockwise, then flip horizontally
//	8     rotate 90° CCW     rotate 90° counter-clockwise
//
// Orientations 5 and 7 are reflections in a diagonal. Composing them from a
// rotation and a flip walks the image twice to reach somewhere a single
// permutation reaches directly.
func opForOrientation(o model.ExifOrientation) ortho.Op {
	switch o {
	case 2:
		return ortho.FlipH
	case 3:
		return ortho.Rotate180
	case 4:
		return ortho.FlipV
	case 5:
		return ortho.Transpose
	case 6:
		return ortho.Rotate90CW
	case 7:
		return ortho.Transverse
	case 8:
		return ortho.Rotate270CW
	default:
		// 1 is upright. NoExifOrientation and any unrecognised tag are treated
		// as upright too, which is what the pipeline did before.
		return ortho.Identity
	}
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

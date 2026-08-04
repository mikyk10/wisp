package exif_rotation

import (
	"context"
	"image"

	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/improc/ortho"
	"github.com/mikyk10/wisp/app/domain/model"
)

// The processor knows what makes an image upright. It can either do it, or say
// what needs doing and let a later stage do it.
//
// Saying is worth the extra concept because the delivery path immediately
// corrects for the display orientation as well, and both corrections are
// members of the same eight-element group. Composed first and performed once,
// a full-resolution image is walked half as often, and not at all when the two
// cancel each other out — which they do for a portrait photograph on a
// portrait-mounted panel, among others.
type processor struct {
	deferred bool
}

// NewExifRotation returns a processor that puts the image upright there and
// then. Use it wherever nothing downstream is going to rotate the image
// anyway, such as the thumbnail pass during a catalogue scan.
func NewExifRotation() improc.ImageProcessor {
	return &processor{}
}

// NewDeferredExifRotation returns a processor that records the operation in
// meta.PendingExifOp and leaves the pixels untouched. Only use it directly
// ahead of crop, which is what consumes the record.
func NewDeferredExifRotation() improc.ImageProcessor {
	return &processor{deferred: true}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {

	op := opForOrientation(meta.ExifOrientation)

	// ImageOrientation describes the image as it looks once op has been
	// applied, whether that happens here or in crop. Downstream stages choose
	// the correction angle from it, so it cannot wait for the pixels.
	//TODO: allow square images to be displayed in either orientation
	// an empty image may arrive
	w, h := src.Bounds().Max.X, src.Bounds().Max.Y
	if ortho.SwapsAxes(op) {
		w, h = h, w
	}
	if w < h {
		meta.ImageOrientation = model.ImgCanonicalOrientationPortrait
	} else {
		meta.ImageOrientation = model.ImgCanonicalOrientationLandscape
	}

	if meta.HasExifSubjectArea {
		origW := src.Bounds().Max.X
		origH := src.Bounds().Max.Y
		meta.ExifSubjectArea = transformSubjectPointByOrientation(meta.ExifSubjectArea, meta.ExifOrientation, origW, origH)
	}

	if p.deferred {
		meta.PendingExifOp = op
		return src, meta
	}

	return ortho.Apply(src, op), meta
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

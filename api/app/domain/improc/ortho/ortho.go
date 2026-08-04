// Package ortho applies the eight orthogonal image transformations: the
// symmetries of a rectangle, otherwise known as the dihedral group D4.
//
// Each one is a permutation of pixel positions. No interpolation, no
// arithmetic on colour values, nothing lost. That matters twice over in this
// pipeline. Every rotation it performs is a multiple of 90 degrees, so a
// general affine rotation would spend per-pixel trigonometry deriving a
// mapping that is known in advance. And the post-processing stage reduces the
// image to a six- or seven-colour palette, so an interpolating rotation
// applied after that point would invent colours the panel cannot show.
//
// Angles follow the convention used throughout the pipeline and by
// model.ImgMeta.RequiredCorrectionAngle: positive means clockwise. The
// underlying library counts counter-clockwise, so the correspondence is
// spelled out here once, in one place, rather than at each call site.
package ortho

import (
	"image"
	"math"

	"github.com/disintegration/imaging"
)

// Op is one of the eight orthogonal transformations.
//
// The table gives, for a w×h source, where the pixel at (x,y) is sent. W and H
// abbreviate w-1 and h-1.
//
//	Op           destination of (x,y)   axes swapped
//	Identity     (x,     y)             no
//	Rotate90CW   (H-y,   x)             yes
//	Rotate180    (W-x,   H-y)           no
//	Rotate270CW  (y,     W-x)           yes
//	FlipH        (W-x,   y)             no
//	FlipV        (x,     H-y)           no
//	Transpose    (y,     x)             yes
//	Transverse   (H-y,   W-x)           yes
//
// Transpose is the reflection in the main diagonal and Transverse the
// reflection in the anti-diagonal. Both used to be expressed as a rotation
// followed by a flip, which walks the image twice to no purpose.
type Op uint8

const (
	// Identity is the only operation that returns the source image itself,
	// concrete type and all. Callers rely on this: it is what keeps a
	// zero-degree correction from copying, or converting, anything.
	Identity Op = iota
	Rotate90CW
	Rotate180
	Rotate270CW
	FlipH
	FlipV
	Transpose
	Transverse
)

// Apply performs op on img.
//
// The library's rotations are named counter-clockwise, which is why
// Rotate90CW maps to Rotate270 and vice versa. Getting this backwards yields a
// perfectly plausible-looking image, so the pipeline's golden test covers
// every orientation rather than a representative sample.
func Apply(img image.Image, op Op) image.Image {
	switch op {
	case Rotate90CW:
		return imaging.Rotate270(img)
	case Rotate180:
		return imaging.Rotate180(img)
	case Rotate270CW:
		return imaging.Rotate90(img)
	case FlipH:
		return imaging.FlipH(img)
	case FlipV:
		return imaging.FlipV(img)
	case Transpose:
		return imaging.Transpose(img)
	case Transverse:
		return imaging.Transverse(img)
	default:
		return img
	}
}

// composition[first][second] is the single operation equivalent to performing
// first and then second. Columns run in declaration order:
//
//	Identity, Rotate90CW, Rotate180, Rotate270CW, FlipH, FlipV, Transpose, Transverse
//
// The eight operations are closed under composition, which is what lets the
// pipeline fold the EXIF normalisation and the display-orientation correction
// into a single pass over a full-resolution image.
var composition = [8][8]Op{
	Identity:    {Identity, Rotate90CW, Rotate180, Rotate270CW, FlipH, FlipV, Transpose, Transverse},
	Rotate90CW:  {Rotate90CW, Rotate180, Rotate270CW, Identity, Transpose, Transverse, FlipV, FlipH},
	Rotate180:   {Rotate180, Rotate270CW, Identity, Rotate90CW, FlipV, FlipH, Transverse, Transpose},
	Rotate270CW: {Rotate270CW, Identity, Rotate90CW, Rotate180, Transverse, Transpose, FlipH, FlipV},
	FlipH:       {FlipH, Transverse, FlipV, Transpose, Identity, Rotate180, Rotate270CW, Rotate90CW},
	FlipV:       {FlipV, Transpose, FlipH, Transverse, Rotate180, Identity, Rotate90CW, Rotate270CW},
	Transpose:   {Transpose, FlipH, Transverse, FlipV, Rotate90CW, Rotate270CW, Identity, Rotate180},
	Transverse:  {Transverse, FlipV, Transpose, FlipH, Rotate270CW, Rotate90CW, Rotate180, Identity},
}

// Compose returns the single operation equivalent to performing first and then
// second.
func Compose(first, second Op) Op {
	if int(first) >= len(composition) || int(second) >= len(composition) {
		return Identity
	}
	return composition[first][second]
}

// SwapsAxes reports whether op exchanges the width and height of the image.
func SwapsAxes(op Op) bool {
	switch op {
	case Rotate90CW, Rotate270CW, Transpose, Transverse:
		return true
	default:
		return false
	}
}

// FromAngleCW returns the operation equivalent to rotating by deg degrees
// clockwise. Only multiples of 90 have an orthogonal equivalent; any other
// angle reports false and yields Identity, so a caller that ignores the flag
// leaves the image untouched rather than corrupting it.
func FromAngleCW(deg float64) (Op, bool) {
	switch math.Mod(math.Mod(deg, 360)+360, 360) {
	case 0:
		return Identity, true
	case 90:
		return Rotate90CW, true
	case 180:
		return Rotate180, true
	case 270:
		return Rotate270CW, true
	default:
		return Identity, false
	}
}

func (o Op) String() string {
	switch o {
	case Identity:
		return "identity"
	case Rotate90CW:
		return "rotate90cw"
	case Rotate180:
		return "rotate180"
	case Rotate270CW:
		return "rotate270cw"
	case FlipH:
		return "fliph"
	case FlipV:
		return "flipv"
	case Transpose:
		return "transpose"
	case Transverse:
		return "transverse"
	default:
		return "unknown"
	}
}

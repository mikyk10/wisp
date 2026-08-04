package model

import (
	"image"
	"time"

	"github.com/mikyk10/wisp/app/domain/improc/ortho"
)

type ExifOrientation int
type CanonicalOrientation int

const (
	ImgCanonicalOrientationNone = CanonicalOrientation(iota)
	ImgCanonicalOrientationLandscape
	ImgCanonicalOrientationPortrait
)

// NewCanonicalOrientation parses a string orientation value.
func NewCanonicalOrientation(s string) CanonicalOrientation {
	switch s {
	case "landscape":
		return ImgCanonicalOrientationLandscape
	case "portrait":
		return ImgCanonicalOrientationPortrait
	default:
		return ImgCanonicalOrientationLandscape
	}
}

const (
	NoExifOrientation = ExifOrientation(0)
)

type ImgMeta struct {
	ImageSourcePath  string
	ImageOrientation CanonicalOrientation

	ExifOrientation ExifOrientation
	ExifDateTime    time.Time

	// PendingExifOp is the EXIF normalisation that has been worked out but not
	// yet performed on the pixels. It is set only by the deferred form of the
	// exif_rotation processor, and crop consumes it by folding it into its own
	// rotation. Identity, the zero value, means there is nothing outstanding.
	PendingExifOp ortho.Op

	FileModifiedAt time.Time

	// RequiredCorrectionAngle is how far crop turned the image, clockwise, to
	// match the installed orientation of the panel. It still means only that,
	// even though crop now performs the EXIF normalisation in the same
	// operation: PendingExifOp is not counted here, because what reads this is
	// asking where the photograph's own edges went.
	//TODO: should be better naming
	RequiredCorrectionAngle float64

	ExifSubjectArea    image.Point // SubjectArea center point in original image coordinates
	HasExifSubjectArea bool        // true if SubjectArea/SubjectLocation was found in Exif

	//TODO: implement these
	//GPSLatitude     string
	//GPSLatitudeRef  string
	//GPSLongitude    string
	//GPSLongitudeRef string
}

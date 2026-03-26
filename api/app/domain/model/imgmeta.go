package model

import (
	"image"
	"time"
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

	FileModifiedAt time.Time

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

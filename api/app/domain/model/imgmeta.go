package model

import (
	"time"
)

type ExifOrientation int
type CanonicalOrientation int

const (
	ImgCanonicalOrientationNone = CanonicalOrientation(iota)
	ImgCanonicalOrientationLandscape
	ImgCanonicalOrientationPortrait
)

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

	//TODO: implement these
	//GPSLatitude     string
	//GPSLatitudeRef  string
	//GPSLongitude    string
	//GPSLongitudeRef string
}

package model

// ImageFilter defines criteria for selecting images from the repository.
type ImageFilter struct {
	CatalogKeys []string
	Orientation CanonicalOrientation
	Tags        []string // match any of these tags (OR)
}

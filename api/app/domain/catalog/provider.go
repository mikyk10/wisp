package catalog

import (
	"context"
)

// ImageLocator is a source (collection) of images.
// Calling Resolve with some logic returns a lazily-loadable image.
type ImageLocator interface {
	Resolve() (ImageLoader, error)
}

// BatchImageSource is an enumerable image source.
type BatchImageSource interface {
	EnumerateImages(ctx context.Context, found chan<- ImageLoader, excluded chan<- ImageLoader)
}

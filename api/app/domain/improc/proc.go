package improc

import (
	"context"
	"image"
	"wspf/app/domain/model"
)

// ImageProcessor is the interface for image processing processors.
type ImageProcessor interface {
	Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta)
}

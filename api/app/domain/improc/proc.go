package improc

import (
	"context"
	"github.com/mikyk10/wisp/app/domain/model"
	"image"
)

// ImageProcessor is the interface for image processing processors.
type ImageProcessor interface {
	Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta)
}

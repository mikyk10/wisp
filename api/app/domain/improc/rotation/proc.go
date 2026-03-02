package rotation

import (
	"context"
	"image"
	"wspf/app/domain/improc"
	"wspf/app/domain/model"

	"github.com/anthonynsimon/bild/transform"
)

type processor struct {
	angle float64
}

func NewRotation() improc.ImageProcessor {
	return &processor{
		angle: 180,
	}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {

	if p.angle == 0 {
		return src, meta
	}

	dst := transform.Rotate(src, p.angle, &transform.RotationOptions{ResizeBounds: true})
	return dst, meta
}

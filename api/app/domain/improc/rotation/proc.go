package rotation

import (
	"context"
	"image"

	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/improc/ortho"
	"github.com/mikyk10/wisp/app/domain/model"
)

type processor struct {
	op ortho.Op
}

// NewRotation turns the finished image upside down, for panels mounted the
// other way up.
func NewRotation() improc.ImageProcessor {
	return &processor{
		op: ortho.Rotate180,
	}
}

func (p *processor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {
	return ortho.Apply(src, p.op), meta
}

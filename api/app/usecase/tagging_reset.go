package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
)

// TaggingResetOptions configures a reset operation.
type TaggingResetOptions struct {
	CatalogKey string
	ImageID    model.PrimaryKey // 0 = reset entire catalog
}

// TaggingResetUsecase deletes tagging data so images can be re-processed.
type TaggingResetUsecase interface {
	Run(ctx context.Context, opts TaggingResetOptions) error
}

type taggingResetUsecase struct {
	taggingRepo repository.TaggingRepository
}

func NewTaggingResetUsecase(taggingRepo repository.TaggingRepository) TaggingResetUsecase {
	return &taggingResetUsecase{taggingRepo: taggingRepo}
}

func (u *taggingResetUsecase) Run(_ context.Context, opts TaggingResetOptions) error {
	if opts.ImageID != 0 {
		slog.Info("tagging reset: resetting image", "image_id", opts.ImageID)
		if err := u.taggingRepo.ResetImageTagging(opts.ImageID); err != nil {
			return fmt.Errorf("reset image %d: %w", opts.ImageID, err)
		}
		slog.Info("tagging reset: done", "image_id", opts.ImageID)
		return nil
	}

	slog.Info("tagging reset: resetting catalog", "catalog", opts.CatalogKey)
	if err := u.taggingRepo.ResetCatalogTagging(opts.CatalogKey); err != nil {
		return fmt.Errorf("reset catalog %q: %w", opts.CatalogKey, err)
	}
	slog.Info("tagging reset: done", "catalog", opts.CatalogKey)
	return nil
}

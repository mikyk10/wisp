package repository

import "github.com/mikyk10/wisp/app/domain/model"

// TaggingRepository manages AI tagging data: tags, image-tag associations, run records, and outputs.
type TaggingRepository interface {
	FindOrCreateTag(name string) (*model.Tag, error)
	ReplaceImageTags(imageID, runID model.PrimaryKey, tagIDs []model.PrimaryKey) error
	HasImageTags(imageID model.PrimaryKey) (bool, error)
	CreateAIRun(run *model.AIRun) error
	UpdateAIRun(run *model.AIRun) error
	FindLatestSuccessfulDescriptor(imageID model.PrimaryKey) (*model.AIRun, error)
	CreateAIOutput(output *model.AIOutput) error
	FindAIOutputByRunID(runID model.PrimaryKey) (*model.AIOutput, error)
	FindImagesForTagging(catalogKey string, limit int) ([]*model.Image, error)
	FindAllImages(catalogKey string, limit int) ([]*model.Image, error)
	// FindTagNamesByImageID returns the tag names associated with the given image.
	FindTagNamesByImageID(imageID model.PrimaryKey) ([]string, error)
	// ResetImageTagging deletes all tagging data (image_tags, ai_runs, ai_outputs) for a single image.
	ResetImageTagging(imageID model.PrimaryKey) error
	// ResetCatalogTagging deletes all tagging data for every image in the given catalog.
	ResetCatalogTagging(catalogKey string) error
}

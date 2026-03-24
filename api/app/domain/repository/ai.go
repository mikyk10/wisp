package repository

import "github.com/mikyk10/wisp/app/domain/model"

// AIRepository provides persistence for AI pipeline data.
type AIRepository interface {
	// Pipeline executions
	CreatePipelineExecution(exec *model.PipelineExecution) error
	UpdatePipelineExecution(exec *model.PipelineExecution) error
	DeletePipelineExecution(id model.PrimaryKey) error

	// Step executions
	CreateStepExecution(step *model.StepExecution) error
	UpdateStepExecution(step *model.StepExecution) error
	FindStepsByPipelineExecution(pipelineExecID model.PrimaryKey) ([]*model.StepExecution, error)

	// Step outputs
	CreateStepOutput(out *model.StepOutput) error
	FindStepOutputByStepID(stepExecID model.PrimaryKey) (*model.StepOutput, error)

	// Tags
	FindOrCreateTag(name string) (*model.Tag, error)
	ReplaceImageTags(imageID model.PrimaryKey, sourceStepID model.PrimaryKey, tagIDs []model.PrimaryKey) error
	HasImageTags(imageID model.PrimaryKey) (bool, error)

	// Generation cache
	CreateCacheEntry(entry *model.GenerationCacheEntry) error
	CountCacheEntries(catalogKey string) (int64, error)
	ListCacheEntries(catalogKey string) ([]*model.GenerationCacheEntry, error)
	FindCacheEntryByID(id model.PrimaryKey) (*model.GenerationCacheEntry, error)
	FindRandomCacheEntry(catalogKey string) (*model.GenerationCacheEntry, error)
	EvictOldestCacheEntries(catalogKey string, count int) error

	// Queries for tagging pipeline
	FindImagesForTagging(catalogKey string, limit int) ([]*model.Image, error)
	FindAllImages(catalogKey string, limit int) ([]*model.Image, error)
	FindLatestSuccessfulStep(imageID model.PrimaryKey, stageName string) (*model.StepExecution, error)

	// Cleanup
	ResetImageTagging(imageID model.PrimaryKey) error
	ResetCatalogTagging(catalogKey string) error
	CleanFailedExecutions(catalogKey string) error
}

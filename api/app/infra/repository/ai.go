package repository

import (
	"math/rand/v2"
	"strings"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
	"gorm.io/gorm"
)

type aiRepositoryImpl struct {
	db *gorm.DB
}

func NewAIRepositoryImpl(db *gorm.DB) repository.AIRepository {
	return &aiRepositoryImpl{db: db}
}

// --- Pipeline executions ---

func (r *aiRepositoryImpl) CreatePipelineExecution(exec *model.PipelineExecution) error {
	return r.db.Create(exec).Error
}

func (r *aiRepositoryImpl) UpdatePipelineExecution(exec *model.PipelineExecution) error {
	return r.db.Save(exec).Error
}

func (r *aiRepositoryImpl) DeletePipelineExecution(id model.PrimaryKey) error {
	// Delete in dependency order: outputs → steps → cache → execution
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Find step IDs
		var stepIDs []model.PrimaryKey
		tx.Model(&model.StepExecution{}).Where("pipeline_execution_id = ?", id).Pluck("id", &stepIDs)

		if len(stepIDs) > 0 {
			if err := tx.Where("step_execution_id IN ?", stepIDs).Delete(&model.StepOutput{}).Error; err != nil {
				return err
			}
			if err := tx.Where("pipeline_execution_id = ?", id).Delete(&model.StepExecution{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("pipeline_execution_id = ?", id).Delete(&model.GenerationCacheEntry{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PipelineExecution{}, id).Error
	})
}

// --- Step executions ---

func (r *aiRepositoryImpl) CreateStepExecution(step *model.StepExecution) error {
	return r.db.Create(step).Error
}

func (r *aiRepositoryImpl) UpdateStepExecution(step *model.StepExecution) error {
	return r.db.Save(step).Error
}

func (r *aiRepositoryImpl) FindStepsByPipelineExecution(pipelineExecID model.PrimaryKey) ([]*model.StepExecution, error) {
	var steps []*model.StepExecution
	err := r.db.Where("pipeline_execution_id = ?", pipelineExecID).Order("stage_index").Find(&steps).Error
	return steps, err
}

// --- Step outputs ---

func (r *aiRepositoryImpl) CreateStepOutput(out *model.StepOutput) error {
	return r.db.Create(out).Error
}

func (r *aiRepositoryImpl) FindStepOutputByStepID(stepExecID model.PrimaryKey) (*model.StepOutput, error) {
	var out model.StepOutput
	err := r.db.Where("step_execution_id = ?", stepExecID).First(&out).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &out, err
}

// --- Tags ---

func (r *aiRepositoryImpl) FindOrCreateTag(name string) (*model.Tag, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	var tag model.Tag
	err := r.db.Where("name_normalized = ?", normalized).First(&tag).Error
	if err == gorm.ErrRecordNotFound {
		tag = model.Tag{
			NameNormalized: normalized,
			DisplayName:    normalized,
		}
		if err := r.db.Create(&tag).Error; err != nil {
			// Race condition: another goroutine may have created it.
			if err2 := r.db.Where("name_normalized = ?", normalized).First(&tag).Error; err2 != nil {
				return nil, err
			}
		}
		return &tag, nil
	}
	return &tag, err
}

func (r *aiRepositoryImpl) ReplaceImageTags(imageID model.PrimaryKey, sourceStepID model.PrimaryKey, tagIDs []model.PrimaryKey) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("image_id = ?", imageID).Delete(&model.ImageTag{}).Error; err != nil {
			return err
		}
		for _, tagID := range tagIDs {
			it := model.ImageTag{
				ImageID:      imageID,
				TagID:        tagID,
				SourceStepID: sourceStepID,
			}
			if err := tx.Create(&it).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *aiRepositoryImpl) HasImageTags(imageID model.PrimaryKey) (bool, error) {
	var count int64
	err := r.db.Model(&model.ImageTag{}).Where("image_id = ?", imageID).Count(&count).Error
	return count > 0, err
}

// --- Generation cache ---

func (r *aiRepositoryImpl) CreateCacheEntry(entry *model.GenerationCacheEntry) error {
	entry.Rnd = rand.Float64()
	return r.db.Create(entry).Error
}

func (r *aiRepositoryImpl) CountCacheEntries(catalogKey string) (int64, error) {
	var count int64
	err := r.db.Model(&model.GenerationCacheEntry{}).Where("catalog_key = ?", catalogKey).Count(&count).Error
	return count, err
}

func (r *aiRepositoryImpl) ListCacheEntries(catalogKey string) ([]*model.GenerationCacheEntry, error) {
	var entries []*model.GenerationCacheEntry
	err := r.db.Where("catalog_key = ?", catalogKey).Order("created_at DESC").Find(&entries).Error
	return entries, err
}

func (r *aiRepositoryImpl) FindCacheEntryByID(id model.PrimaryKey) (*model.GenerationCacheEntry, error) {
	var entry model.GenerationCacheEntry
	err := r.db.First(&entry, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &entry, err
}

func (r *aiRepositoryImpl) FindRandomCacheEntry(catalogKey string) (*model.GenerationCacheEntry, error) {
	pivot := rand.Float64()
	var entry model.GenerationCacheEntry
	err := r.db.Where("catalog_key = ? AND rnd >= ?", catalogKey, pivot).Order("rnd").First(&entry).Error
	if err == gorm.ErrRecordNotFound {
		// Wrap around
		err = r.db.Where("catalog_key = ?", catalogKey).Order("rnd").First(&entry).Error
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
	}
	return &entry, err
}

func (r *aiRepositoryImpl) EvictOldestCacheEntries(catalogKey string, count int) error {
	// Find the oldest N entries by created_at, then delete their pipeline executions (cascading).
	var entries []*model.GenerationCacheEntry
	if err := r.db.Where("catalog_key = ?", catalogKey).Order("created_at ASC").Limit(count).Find(&entries).Error; err != nil {
		return err
	}
	for _, e := range entries {
		if err := r.DeletePipelineExecution(e.PipelineExecutionID); err != nil {
			return err
		}
	}
	return nil
}

// --- Tagging queries ---

func (r *aiRepositoryImpl) FindImagesForTagging(catalogKey string, limit int) ([]*model.Image, error) {
	// Images that don't have tags yet.
	q := r.db.Where("catalog_key = ? AND excluded = false AND deleted_at IS NULL", catalogKey).
		Where("id NOT IN (SELECT DISTINCT image_id FROM image_tags)")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var images []*model.Image
	err := q.Find(&images).Error
	return images, err
}

func (r *aiRepositoryImpl) FindAllImages(catalogKey string, limit int) ([]*model.Image, error) {
	q := r.db.Where("catalog_key = ? AND excluded = false AND deleted_at IS NULL", catalogKey)
	if limit > 0 {
		q = q.Limit(limit)
	}
	var images []*model.Image
	err := q.Find(&images).Error
	return images, err
}

func (r *aiRepositoryImpl) FindLatestSuccessfulStep(imageID model.PrimaryKey, stageName string) (*model.StepExecution, error) {
	var step model.StepExecution
	err := r.db.
		Joins("JOIN pipeline_executions ON pipeline_executions.id = step_executions.pipeline_execution_id").
		Where("pipeline_executions.source_image_id = ? AND step_executions.stage_name = ? AND step_executions.status = ?",
			imageID, stageName, model.StatusSuccess).
		Order("step_executions.created_at DESC").
		First(&step).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &step, err
}

func (r *aiRepositoryImpl) FindRandomImage(catalogKey string) (*model.Image, error) {
	pivot := rand.Float64()
	var img model.Image
	err := r.db.Where("catalog_key = ? AND excluded = false AND deleted_at IS NULL AND rnd >= ?", catalogKey, pivot).
		Order("rnd").First(&img).Error
	if err == gorm.ErrRecordNotFound {
		err = r.db.Where("catalog_key = ? AND excluded = false AND deleted_at IS NULL", catalogKey).
			Order("rnd").First(&img).Error
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
	}
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// --- Cleanup ---

func (r *aiRepositoryImpl) ResetImageTagging(imageID model.PrimaryKey) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete tags
		if err := tx.Where("image_id = ?", imageID).Delete(&model.ImageTag{}).Error; err != nil {
			return err
		}
		// Find pipeline executions for this image
		var execIDs []model.PrimaryKey
		tx.Model(&model.PipelineExecution{}).
			Where("source_image_id = ? AND pipeline_type = ?", imageID, "tagging").
			Pluck("id", &execIDs)
		for _, eid := range execIDs {
			if err := r.deleteExecInTx(tx, eid); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *aiRepositoryImpl) ResetCatalogTagging(catalogKey string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete all image tags for this catalog
		if err := tx.Where("image_id IN (SELECT id FROM images WHERE catalog_key = ?)", catalogKey).
			Delete(&model.ImageTag{}).Error; err != nil {
			return err
		}
		// Find and delete all tagging pipeline executions
		var execIDs []model.PrimaryKey
		tx.Model(&model.PipelineExecution{}).
			Where("catalog_key = ? AND pipeline_type = ?", catalogKey, "tagging").
			Pluck("id", &execIDs)
		for _, eid := range execIDs {
			if err := r.deleteExecInTx(tx, eid); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *aiRepositoryImpl) CleanFailedExecutions(catalogKey string) error {
	var execIDs []model.PrimaryKey
	r.db.Model(&model.PipelineExecution{}).
		Where("catalog_key = ? AND status = ?", catalogKey, model.StatusFailed).
		Pluck("id", &execIDs)
	for _, eid := range execIDs {
		if err := r.DeletePipelineExecution(eid); err != nil {
			return err
		}
	}
	return nil
}

func (r *aiRepositoryImpl) deleteExecInTx(tx *gorm.DB, id model.PrimaryKey) error {
	var stepIDs []model.PrimaryKey
	tx.Model(&model.StepExecution{}).Where("pipeline_execution_id = ?", id).Pluck("id", &stepIDs)
	if len(stepIDs) > 0 {
		if err := tx.Where("step_execution_id IN ?", stepIDs).Delete(&model.StepOutput{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pipeline_execution_id = ?", id).Delete(&model.StepExecution{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("pipeline_execution_id = ?", id).Delete(&model.GenerationCacheEntry{}).Error; err != nil {
		return err
	}
	return tx.Delete(&model.PipelineExecution{}, id).Error
}

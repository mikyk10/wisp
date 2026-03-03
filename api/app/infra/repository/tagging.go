package repository

import (
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"

	"gorm.io/gorm"
)

type taggingRepositoryImpl struct {
	conn *gorm.DB
}

func NewTaggingRepositoryImpl(conn *gorm.DB) repository.TaggingRepository {
	return &taggingRepositoryImpl{conn: conn}
}

func (r *taggingRepositoryImpl) FindOrCreateTag(name string) (*model.Tag, error) {
	tag := &model.Tag{}
	err := r.conn.Where(model.Tag{NameNormalized: name}).
		Attrs(model.Tag{DisplayName: name}).
		FirstOrCreate(tag).Error
	if err != nil {
		return nil, err
	}
	return tag, nil
}

func (r *taggingRepositoryImpl) ReplaceImageTags(imageID, runID model.PrimaryKey, tagIDs []model.PrimaryKey) error {
	return r.conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("image_id = ?", imageID).Delete(&model.ImageTag{}).Error; err != nil {
			return err
		}
		for _, tagID := range tagIDs {
			it := model.ImageTag{
				ImageID:     imageID,
				TagID:       tagID,
				SourceRunID: runID,
			}
			if err := tx.Create(&it).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *taggingRepositoryImpl) HasImageTags(imageID model.PrimaryKey) (bool, error) {
	var count int64
	err := r.conn.Model(&model.ImageTag{}).Where("image_id = ?", imageID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *taggingRepositoryImpl) CreateAIRun(run *model.AIRun) error {
	return r.conn.Create(run).Error
}

func (r *taggingRepositoryImpl) UpdateAIRun(run *model.AIRun) error {
	return r.conn.Save(run).Error
}

func (r *taggingRepositoryImpl) FindLatestSuccessfulDescriptor(imageID model.PrimaryKey) (*model.AIRun, error) {
	run := &model.AIRun{}
	err := r.conn.Where("image_id = ? AND stage = ? AND status = ?", imageID, model.AIRunStageDescriptor, model.AIRunStatusSuccess).
		Order("id DESC").
		First(run).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (r *taggingRepositoryImpl) CreateAIOutput(output *model.AIOutput) error {
	return r.conn.Create(output).Error
}

func (r *taggingRepositoryImpl) FindAIOutputByRunID(runID model.PrimaryKey) (*model.AIOutput, error) {
	output := &model.AIOutput{}
	err := r.conn.Where("run_id = ?", runID).First(output).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return output, nil
}

// ResetImageTagging deletes all tagging data for a single image in dependency order.
func (r *taggingRepositoryImpl) ResetImageTagging(imageID model.PrimaryKey) error {
	return r.conn.Transaction(func(tx *gorm.DB) error {
		// ai_outputs depend on ai_runs, so delete them first.
		if err := tx.Where("run_id IN (SELECT id FROM ai_runs WHERE image_id = ?)", imageID).
			Delete(&model.AIOutput{}).Error; err != nil {
			return err
		}
		if err := tx.Where("image_id = ?", imageID).Delete(&model.AIRun{}).Error; err != nil {
			return err
		}
		if err := tx.Where("image_id = ?", imageID).Delete(&model.ImageTag{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// ResetCatalogTagging deletes all tagging data for every image in the given catalog.
func (r *taggingRepositoryImpl) ResetCatalogTagging(catalogKey string) error {
	return r.conn.Transaction(func(tx *gorm.DB) error {
		imageIDsSubquery := tx.Model(&model.Image{}).
			Select("id").
			Where("catalog_key = ?", catalogKey)
		if err := tx.Where("run_id IN (?)", tx.Model(&model.AIRun{}).
			Select("id").
			Where("image_id IN (?)", imageIDsSubquery)).
			Delete(&model.AIOutput{}).Error; err != nil {
			return err
		}
		if err := tx.Where("image_id IN (?)", imageIDsSubquery).Delete(&model.AIRun{}).Error; err != nil {
			return err
		}
		if err := tx.Where("image_id IN (?)", imageIDsSubquery).Delete(&model.ImageTag{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// FindImagesForTagging returns images in the given catalog that have no image_tags yet.
func (r *taggingRepositoryImpl) FindImagesForTagging(catalogKey string, limit int) ([]*model.Image, error) {
	var images []*model.Image
	q := r.conn.Where(
		"catalog_key = ? AND excluded = false AND deleted_at IS NULL AND id NOT IN (SELECT DISTINCT image_id FROM image_tags)",
		catalogKey,
	)
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Order("id ASC").Find(&images).Error; err != nil {
		return nil, err
	}
	return images, nil
}

// FindTagNamesByImageID returns tag names for a single image via a JOIN query.
func (r *taggingRepositoryImpl) FindTagNamesByImageID(imageID model.PrimaryKey) ([]string, error) {
	var names []string
	err := r.conn.Raw(`
		SELECT t.name_normalized
		FROM image_tags it
		JOIN tags t ON it.tag_id = t.id
		WHERE it.image_id = ?
	`, imageID).Scan(&names).Error
	if err != nil {
		return nil, err
	}
	return names, nil
}

// FindAllImages returns all non-excluded, non-deleted images in the given catalog.
func (r *taggingRepositoryImpl) FindAllImages(catalogKey string, limit int) ([]*model.Image, error) {
	var images []*model.Image
	q := r.conn.Where("catalog_key = ? AND excluded = false AND deleted_at IS NULL", catalogKey)
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Order("id ASC").Find(&images).Error; err != nil {
		return nil, err
	}
	return images, nil
}

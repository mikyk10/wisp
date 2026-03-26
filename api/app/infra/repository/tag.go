package repository

import (
	"strings"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
	"gorm.io/gorm"
)

type tagRepositoryImpl struct {
	db *gorm.DB
}

func NewTagRepositoryImpl(db *gorm.DB) repository.TagRepository {
	return &tagRepositoryImpl{db: db}
}

func (r *tagRepositoryImpl) FindOrCreateTag(name string) (*model.Tag, error) {
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

func (r *tagRepositoryImpl) ReplaceImageTags(imageID model.PrimaryKey, tagIDs []model.PrimaryKey) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("image_id = ?", imageID).Delete(&model.ImageTag{}).Error; err != nil {
			return err
		}
		for _, tagID := range tagIDs {
			it := model.ImageTag{ImageID: imageID, TagID: tagID}
			if err := tx.Create(&it).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *tagRepositoryImpl) HasImageTags(imageID model.PrimaryKey) (bool, error) {
	var count int64
	err := r.db.Model(&model.ImageTag{}).Where("image_id = ?", imageID).Count(&count).Error
	return count > 0, err
}

func (r *tagRepositoryImpl) FindImagesWithoutTags(catalogKey string, limit int) ([]model.PrimaryKey, error) {
	var ids []model.PrimaryKey
	q := r.db.Model(&model.Image{}).
		Where("catalog_key = ? AND excluded = false AND deleted_at IS NULL", catalogKey).
		Where("id NOT IN (SELECT DISTINCT image_id FROM image_tags)").
		Order("id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Pluck("id", &ids).Error
	return ids, err
}

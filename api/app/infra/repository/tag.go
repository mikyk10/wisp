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

// FindTagUsage counts, per tag, the images in the catalogue that carry it.
//
// The join runs through images rather than counting image_tags rows directly,
// because the same tag is shared across catalogues and a count that ignored
// which catalogue an image belongs to would offer the reader tags that return
// nothing here. Excluded images are left out for the same reason: the listing
// they would be filtering does not show them either.
//
// The counts are computed on every request. There is no summary table and no
// cache, so what is returned is always what the catalogue holds right now —
// which matters while tagging is running, because the numbers move under the
// reader.
//
// TODO(tag-counts): keep the counts as rows once this stops being cheap.
//
// Measured 2026-08-24 against production (catalogue `uco`, 18,068 images,
// 1,255 tags, 30,668 assignments): 13.2ms median server-side, which is about
// 1% of what opening the picker costs a phone. Not worth pre-computing yet.
//
// The cost is proportional to the assignments in the catalogue, not to the
// tags returned, because image_tags carries no catalogue of its own and every
// row has to be joined back to images to find out whose it is. At 184k
// assignments — where `uco` lands if tagging fills out to the configured ten
// tags per image, currently averaging 1.7 — the same query measured 107ms on
// an in-memory SQLite of the same shape. Two ways out were measured there:
//
//   - denormalise catalog_key onto image_tags, indexed (catalog_key, tag_id):
//     12.7ms, and it makes LoadCatalogImageTags cheaper on the same grounds
//   - keep (catalog_key, tag_id, count) as rows, written where tags are:
//     0.28ms, at the price of a second place that has to stay true
//
// Act on it when /api/catalog/:key/tags passes ~50ms in the access log, or
// when a second screen starts asking for these counts. Until then the picker's
// latency is in the rendering, not here.
func (r *tagRepositoryImpl) FindTagUsage(catalogKey string) ([]model.TagUsage, error) {
	usage := []model.TagUsage{}
	err := r.db.Model(&model.ImageTag{}).
		Select("tags.display_name AS name, COUNT(*) AS count").
		Joins("JOIN tags ON tags.id = image_tags.tag_id").
		Joins("JOIN images ON images.id = image_tags.image_id").
		Where("images.catalog_key = ? AND images.excluded = false", catalogKey).
		Group("tags.id, tags.display_name").
		// Name breaks the tie so that two equally used tags do not swap places
		// between requests; a list that reorders itself under the pointer is
		// hard to use even when every entry is correct.
		Order("count DESC, tags.display_name ASC").
		Scan(&usage).Error
	return usage, err
}

// LoadCatalogImageTags reads the catalogue's tag assignments in one pass.
//
// Names are taken from a map built once rather than joined per row, so the
// thousands of assignments that share a tag share one string as well. On a
// catalogue where every image carries ten tags that is the difference between
// a few megabytes and tens of them, paid on every listing request.
func (r *tagRepositoryImpl) LoadCatalogImageTags(catalogKey string) (map[model.PrimaryKey][]string, error) {
	var tags []model.Tag
	if err := r.db.Find(&tags).Error; err != nil {
		return nil, err
	}
	names := make(map[model.PrimaryKey]string, len(tags))
	for _, t := range tags {
		names[t.ID] = t.DisplayName
	}

	var links []model.ImageTag
	err := r.db.Model(&model.ImageTag{}).
		Select("image_tags.image_id, image_tags.tag_id").
		Joins("JOIN images ON images.id = image_tags.image_id").
		Where("images.catalog_key = ? AND images.excluded = false", catalogKey).
		// Stable order so a photo's tags read the same way twice running.
		Order("image_tags.image_id ASC, image_tags.tag_id ASC").
		Find(&links).Error
	if err != nil {
		return nil, err
	}

	byImage := make(map[model.PrimaryKey][]string)
	for _, link := range links {
		name, ok := names[link.TagID]
		if !ok {
			// An assignment whose tag row has gone. Nothing useful to show for
			// it, and a blank chip would read as a tag with no name.
			continue
		}
		byImage[link.ImageID] = append(byImage[link.ImageID], name)
	}
	return byImage, nil
}

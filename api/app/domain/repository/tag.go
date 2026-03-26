package repository

import "github.com/mikyk10/wisp/app/domain/model"

// TagRepository provides persistence for image tags.
type TagRepository interface {
	// FindOrCreateTag finds or creates a tag by normalized name.
	FindOrCreateTag(name string) (*model.Tag, error)

	// ReplaceImageTags replaces all tags for an image.
	ReplaceImageTags(imageID model.PrimaryKey, tagIDs []model.PrimaryKey) error

	// HasImageTags returns true if the image has any tags.
	HasImageTags(imageID model.PrimaryKey) (bool, error)

	// FindImagesWithoutTags returns image IDs in the catalog that have no tags.
	FindImagesWithoutTags(catalogKey string, limit int) ([]model.PrimaryKey, error)
}

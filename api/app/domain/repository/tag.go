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

	// FindTagUsage returns every tag carried by an image in the catalogue,
	// most used first, with the number of images carrying it.
	FindTagUsage(catalogKey string) ([]model.TagUsage, error)

	// LoadCatalogImageTags returns the tag names of every tagged image in the
	// catalogue, keyed by image ID.
	//
	// The whole catalogue at once, rather than a lookup per image, because the
	// caller is the listing: a per-image call would be one query per row of a
	// stream that is tens of thousands of rows long.
	LoadCatalogImageTags(catalogKey string) (map[model.PrimaryKey][]string, error)
}

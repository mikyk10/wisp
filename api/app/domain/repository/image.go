package repository

import "github.com/mikyk10/wisp/app/domain/model"

type ImageRepository interface {
	RemoveImage(id model.PrimaryKey) error

	ToggleDeletedAt(ids []model.PrimaryKey) error

	FindById(id model.PrimaryKey) (*model.Image, error)

	FindAll(func(*model.Image) error)

	FindByRandom(filter model.ImageFilter) (*model.Image, error)

	ListByCatalog(catalogKey string, cb func(*model.Image) error) error

	// CountByCatalog returns the number of active images matching the given catalog key and orientation.
	CountByCatalog(catalogKey string, ori model.CanonicalOrientation) (int64, error)

	// CountAllByCatalog returns the total number of active images in the given catalog (orientation-agnostic).
	CountAllByCatalog(catalogKey string) (int64, error)

	// FindByHash searches for an existing record by catalog key and source hash.
	// Returns (nil, nil) if not found.
	FindByHash(catalogKey, srcHash string) (*model.Image, error)

	// UpsertActiveImage upserts an active (display-eligible) image record.
	UpsertActiveImage(rec *model.Image) error

	// UpsertInactiveImage upserts files excluded by catalog configuration with excluded=true (negative index).
	// This is distinct from user-initiated hiding (deleted_at).
	UpsertInactiveImage(catalogKey, srcHash, src string) error

	// FindImageData loads only the image_data blob for the given image ID.
	// Used for background HTTP images where the full image is stored in DB.
	FindImageData(id model.PrimaryKey) ([]byte, error)

	// EvictOldestImages hard-deletes the oldest N images (by created_at ASC) in the given catalog.
	EvictOldestImages(catalogKey string, count int) error
}

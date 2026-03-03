package repository

import "github.com/mikyk10/wisp/app/domain/model"

type ImageRepository interface {
	RemoveImage(id model.PrimaryKey) error

	ToggleDeletedAt(ids []model.PrimaryKey) error

	FindById(id model.PrimaryKey) (*model.Image, error)

	FindAll(func(*model.Image) error)

	FindByRandom(catalogKey string, ori model.CanonicalOrientation) (*model.Image, error)

	ListByCatalog(catalogKey string, tags []string, cb func(*model.Image) error) error

	// CountByCatalog returns the number of active images matching the given catalog key and orientation.
	CountByCatalog(catalogKey string, ori model.CanonicalOrientation) (int64, error)

	// FindByHash searches for an existing record by catalog key and source hash.
	// Returns (nil, nil) if not found.
	FindByHash(catalogKey, srcHash string) (*model.Image, error)

	// UpsertActiveImage upserts an active (display-eligible) image record.
	UpsertActiveImage(rec *model.Image) error

	// UpsertInactiveImage upserts files excluded by catalog configuration with excluded=true (negative index).
	// This is distinct from user-initiated hiding (deleted_at).
	UpsertInactiveImage(catalogKey, srcHash, src string) error
}

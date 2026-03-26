package model

// AllModels returns all GORM models that require AutoMigrate.
// Add new models here — this is the single source of truth for schema management.
func AllModels() []any {
	return []any{
		&Image{},
		&Tag{},
		&ImageTag{},
	}
}

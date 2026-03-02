package repository

// SystemRepository is the repository interface for system management operations.
type SystemRepository interface {
	// DropAndRecreate drops all tables and recreates the schema.
	DropAndRecreate() error
}

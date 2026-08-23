package repository

import (
	"fmt"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"

	"gorm.io/gorm"
)

type systemRepositoryImpl struct {
	conn *gorm.DB
}

func NewSystemRepositoryImpl(conn *gorm.DB) repository.SystemRepository {
	return &systemRepositoryImpl{conn: conn}
}

// DropAndRecreate drops every table owned by the application and recreates the
// schema from scratch. model.AllModels() is the single source of truth for the
// table set, so this stays in sync with the AutoMigrate performed at startup.
//
// Both steps go through the GORM Migrator so that they work on SQLite, MySQL
// and PostgreSQL alike. All models are passed to DropTable in a single call on
// purpose: the migrator reorders them by dependency and drops them in reverse,
// and each driver suppresses foreign key enforcement for the duration
// (PRAGMA foreign_keys = OFF on SQLite, SET FOREIGN_KEY_CHECKS = 0 on MySQL,
// DROP TABLE ... CASCADE on PostgreSQL). Dropping one model at a time would
// lose both guarantees.
func (s *systemRepositoryImpl) DropAndRecreate() error {
	models := model.AllModels()
	if err := s.conn.Migrator().DropTable(models...); err != nil {
		return fmt.Errorf("drop tables: %w", err)
	}
	if err := s.conn.AutoMigrate(models...); err != nil {
		return fmt.Errorf("recreate schema: %w", err)
	}
	return nil
}

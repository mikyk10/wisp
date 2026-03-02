package repository

import (
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

func (s *systemRepositoryImpl) DropAndRecreate() error {
	if err := s.conn.Exec(`PRAGMA writable_schema = 1;
	delete from sqlite_master where type in ('table', 'index', 'trigger');
	PRAGMA writable_schema = 0; VACUUM; PRAGMA INTEGRITY_CHECK;`).Error; err != nil {
		return err
	}
	return s.conn.AutoMigrate(&model.Image{})
}

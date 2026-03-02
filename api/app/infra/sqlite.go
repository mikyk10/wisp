package infra

import (
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewSqliteConnection(dsn string, silent bool) (*gorm.DB, error) {

	logLevel := logger.Warn
	if silent {
		logLevel = logger.Silent
	}

	gormLogger := logger.New(
		log.New(os.Stderr, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
		},
	)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	// SQLite has a single-writer constraint; limiting to one connection
	// eliminates read/write contention at the GORM pool level.
	// WAL mode is not used because shared memory files may be unavailable in container environments.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)

	return db, nil
}

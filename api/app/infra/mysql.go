package infra

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewMysqlConnection(dsn string, silent bool) (*gorm.DB, error) {

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

	return gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
}

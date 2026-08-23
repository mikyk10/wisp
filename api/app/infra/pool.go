package infra

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Connection lifetimes for the pool bounded by ApplyPoolLimits. The sizes
// themselves are configuration; see config.DefaultMaxOpenConns for why they are
// bounded at all.
const (
	// connMaxIdleTime returns connections that a quiet period has left unused,
	// so a server idle overnight is not holding twenty backends open on the
	// database for no reason.
	connMaxIdleTime = 5 * time.Minute

	// connMaxLifetime recycles even a busy connection eventually, so that a
	// database restart or a failover behind a service address is not something
	// this process can hold a stale connection across indefinitely.
	connMaxLifetime = time.Hour
)

// ApplyPoolLimits bounds the connection pool behind db.
//
// maxOpen at or below zero is refused rather than passed through: zero and
// negative both mean "no limit" to database/sql, and an unlimited pool is the
// condition this function exists to prevent. Callers that have not configured
// anything should pass the defaults above.
func ApplyPoolLimits(db *gorm.DB, maxOpen, maxIdle int) error {
	if maxOpen <= 0 {
		return fmt.Errorf("max_open_conns must be positive, got %d", maxOpen)
	}
	if maxIdle <= 0 {
		return fmt.Errorf("max_idle_conns must be positive, got %d", maxIdle)
	}

	// database/sql silently reduces the idle count to the open count. Doing it
	// here as well means Stats() and the configuration agree, so a reader
	// comparing the two is not left wondering which one lost.
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to reach the underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	return nil
}

package infra

import (
	"testing"
)

// TestApplyPoolLimits: the point of the pool is the ceiling, so the ceiling is
// what is asserted — read back from the sql.DB rather than from the arguments,
// because a setting that never reached the driver would satisfy any test that
// only checked what it passed in.
func TestApplyPoolLimits(t *testing.T) {
	db, err := NewSqliteConnection("", true)
	if err != nil {
		t.Fatalf("NewSqliteConnection() unexpected error: %v", err)
	}

	if err := ApplyPoolLimits(db, 7, 3); err != nil {
		t.Fatalf("ApplyPoolLimits() unexpected error: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("DB() unexpected error: %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 7 {
		t.Errorf("MaxOpenConnections = %d, want 7", got)
	}
}

// TestApplyPoolLimits_IdleClampedToOpen: database/sql reduces the idle count to
// the open count itself, so the only thing this pins is that the two agree
// where a reader looks. An idle count above the open one is a configuration
// that cannot mean what it says.
func TestApplyPoolLimits_IdleClampedToOpen(t *testing.T) {
	db, err := NewSqliteConnection("", true)
	if err != nil {
		t.Fatalf("NewSqliteConnection() unexpected error: %v", err)
	}

	if err := ApplyPoolLimits(db, 4, 99); err != nil {
		t.Fatalf("ApplyPoolLimits() unexpected error: %v", err)
	}

	sqlDB, _ := db.DB()
	if got := sqlDB.Stats().MaxOpenConnections; got != 4 {
		t.Errorf("MaxOpenConnections = %d, want 4", got)
	}
}

// TestApplyPoolLimits_RejectsUnbounded: zero and negative both mean "no limit"
// to database/sql. Accepting either would restore the unbounded pool this
// function exists to prevent, and it would do it silently — the failure would
// surface much later as connections the database refuses under load.
func TestApplyPoolLimits_RejectsUnbounded(t *testing.T) {
	tests := []struct {
		name             string
		maxOpen, maxIdle int
	}{
		{"zero open", 0, 10},
		{"negative open", -1, 10},
		{"zero idle", 10, 0},
		{"negative idle", 10, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := NewSqliteConnection("", true)
			if err != nil {
				t.Fatalf("NewSqliteConnection() unexpected error: %v", err)
			}
			if err := ApplyPoolLimits(db, tt.maxOpen, tt.maxIdle); err == nil {
				t.Errorf("ApplyPoolLimits(%d, %d) = nil, want an error",
					tt.maxOpen, tt.maxIdle)
			}
		})
	}
}

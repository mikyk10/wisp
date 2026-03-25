package infra

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
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

	isFileBased := dsn != ""
	dsn = appendSQLitePragmas(dsn)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	// Verify WAL mode for file-based databases (in-memory DBs cannot use WAL).
	if isFileBased {
		var journalMode string
		if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
			return nil, fmt.Errorf("failed to check journal_mode: %w", err)
		}
		if !strings.EqualFold(journalMode, "wal") {
			log.Printf("WARNING: SQLite journal_mode is %q, not WAL. Crash resilience is reduced.", journalMode)
		}
	}

	return db, nil
}

// appendSQLitePragmas adds query-string PRAGMAs to the DSN for durability and
// concurrency. These are applied at connection time by the SQLite driver.
//   - journal_mode=WAL: allows concurrent readers during writes and improves
//     crash resilience over the default rollback journal.
//   - busy_timeout=5000: waits up to 5 s for a lock instead of failing immediately.
//   - synchronous=NORMAL: safe with WAL and significantly faster than FULL.
//   - foreign_keys=ON: enforces referential integrity.
func appendSQLitePragmas(dsn string) string {
	if dsn == "" {
		// In-memory database used by tests. Each call gets a unique name so
		// that parallel tests do not share state via cache=shared.
		name := fmt.Sprintf("testdb_%d_%d", time.Now().UnixNano(), rand.Int()) //nolint:gosec
		return fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", name)
	}

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"
}

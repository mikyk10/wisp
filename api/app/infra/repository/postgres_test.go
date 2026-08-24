package repository_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/catalog"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgreSQL is the one dialect whose differences the rest of this package
// cannot see: the tests above run on SQLite, which accepts nearly everything
// and types nothing. The statements this package hand-builds — the CASE in
// ReshuffleRandom and in ToggleDeletedAt, and the ON CONFLICT the two upserts
// raise — are exactly where that gap shows, so they are exercised here against
// a live server.
//
// The server is named by WISP_TEST_PG_DSN, for example:
//
//	WISP_TEST_PG_DSN='host=127.0.0.1 port=55433 user=wisp password=wisp dbname=wisp sslmode=disable TimeZone=UTC' go test ./...
//
// With the variable unset every test in this file skips, so a checkout with no
// PostgreSQL to hand still runs green.

// --- helpers ---------------------------------------------------------------

// sqlRecorder is a gorm logger that keeps every statement it is handed. The
// repository builds its own SQL, and for the upserts what that SQL says is the
// point of the test, so it has to be read rather than inferred.
type sqlRecorder struct {
	logger.Interface
	statements []string
}

func (r *sqlRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	stmt, _ := fc()
	r.statements = append(r.statements, stmt)
}

// last returns the most recent statement containing sub, or "" if none does.
func (r *sqlRecorder) last(sub string) string {
	for i := len(r.statements) - 1; i >= 0; i-- {
		if strings.Contains(r.statements[i], sub) {
			return r.statements[i]
		}
	}
	return ""
}

// setupPostgres opens the server named by WISP_TEST_PG_DSN and hands back a
// freshly migrated schema, the SQL the repositories go on to send, and the raw
// *gorm.DB for assertions the interfaces cannot make.
//
// The schema is dropped both before and after: before, so a run left half way
// by an earlier failure cannot be mistaken for this run's work, and after, so
// the database is as it was found. One database is shared by every test here,
// which is why none of them calls t.Parallel.
func setupPostgres(t *testing.T) (*gorm.DB, *sqlRecorder) {
	t.Helper()

	dsn := os.Getenv("WISP_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("WISP_TEST_PG_DSN is not set; skipping the PostgreSQL integration tests")
	}

	conn, err := infra.NewPostgresConnection(dsn, true)
	if err != nil {
		t.Fatalf("NewPostgresConnection: %v", err)
	}

	dropPostgresSchema(t, conn)
	if err := conn.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	t.Cleanup(func() {
		dropPostgresSchema(t, conn)
		if sqlDB, err := conn.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	rec := &sqlRecorder{Interface: conn.Logger}
	return conn.Session(&gorm.Session{Logger: rec}), rec
}

// secondPostgresConnection opens a pool of its own against the same server, so
// that what it reads is what another process would see.
//
// It exists for the tests that ask what somebody else can see. A read issued
// through the connection doing the writing can be served by the very session
// doing it, and would then see rows that session has not committed — which is
// exactly the thing those tests are trying to tell apart. Two *sql.DB never
// share a backend, so this cannot happen here. Nothing is migrated or dropped:
// the caller's setupPostgres owns the schema.
func secondPostgresConnection(t *testing.T) *gorm.DB {
	t.Helper()

	conn, err := infra.NewPostgresConnection(os.Getenv("WISP_TEST_PG_DSN"), true)
	if err != nil {
		t.Fatalf("second NewPostgresConnection: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := conn.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return conn
}

// dropPostgresSchema removes every table the application owns.
func dropPostgresSchema(t *testing.T, conn *gorm.DB) {
	t.Helper()
	if err := conn.Migrator().DropTable(model.AllModels()...); err != nil {
		t.Fatalf("DropTable: %v", err)
	}
}

// setupPostgresImageRepo is the usual entry point: a migrated schema and the
// image repository sitting on it.
func setupPostgresImageRepo(t *testing.T) (repository.ImageRepository, *gorm.DB, *sqlRecorder) {
	t.Helper()
	conn, rec := setupPostgres(t)
	return infraRepo.NewImageRepositoryImpl(conn), conn, rec
}

// pgColumnType returns the type PostgreSQL gave a column, as it would be shown
// by \d — "double precision" rather than "float8".
func pgColumnType(t *testing.T, conn *gorm.DB, table, column string) string {
	t.Helper()
	var typ string
	err := conn.Raw(
		"SELECT format_type(a.atttypid, a.atttypmod) FROM pg_attribute a "+
			"WHERE a.attrelid = ?::regclass AND a.attname = ? AND a.attnum > 0 AND NOT a.attisdropped",
		table, column,
	).Scan(&typ).Error
	if err != nil {
		t.Fatalf("read type of %s.%s: %v", table, column, err)
	}
	return typ
}

// imageIDOf returns the id of the image with the given src_hash.
func imageIDOf(t *testing.T, conn *gorm.DB, srcHash string) model.PrimaryKey {
	t.Helper()
	var ids []model.PrimaryKey
	if err := conn.Unscoped().Model(&model.Image{}).
		Where("src_hash = ?", srcHash).Pluck("id", &ids).Error; err != nil {
		t.Fatalf("read id of %s: %v", srcHash, err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected one image with src_hash %s, found %d", srcHash, len(ids))
	}
	return ids[0]
}

// readRnd reads one row's rnd straight out of the database, bypassing the
// repository, so that what is asserted is what was stored.
func readRnd(t *testing.T, conn *gorm.DB, id model.PrimaryKey) float64 {
	t.Helper()
	var rnd float64
	if err := conn.Unscoped().Model(&model.Image{}).Where("id = ?", id).Pluck("rnd", &rnd).Error; err != nil {
		t.Fatalf("read rnd of id=%d: %v", id, err)
	}
	return rnd
}

// --- AutoMigrate -----------------------------------------------------------

// TestPostgres_AutoMigrate is the floor everything else stands on: the models
// as tagged have to produce a schema PostgreSQL will accept. A type tag naming
// a spelling only MySQL knows fails here, before any row is written.
func TestPostgres_AutoMigrate(t *testing.T) {
	conn, _ := setupPostgres(t)

	for _, m := range model.AllModels() {
		if !conn.Migrator().HasTable(m) {
			t.Errorf("table for %T was not created", m)
		}
	}

	// rnd is sought through by idx_random on every device request, and is read
	// back into a Go float64. An arbitrary-precision numeric would satisfy both
	// the migration and the assertions below while costing both.
	if got := pgColumnType(t, conn, "images", "rnd"); got != "double precision" {
		t.Errorf("images.rnd is %q, want %q", got, "double precision")
	}
	for _, c := range []struct{ column, want string }{
		{"thumb_jpg", "bytea"},
		{"image_data", "bytea"},
		{"excluded", "boolean"},
		{"src_hash", "character(40)"},
		{"catalog_key", "character varying(64)"},
	} {
		if got := pgColumnType(t, conn, "images", c.column); got != c.want {
			t.Errorf("images.%s is %q, want %q", c.column, got, c.want)
		}
	}

	// Startup runs AutoMigrate against whatever is already there, so the second
	// pass over an up-to-date schema has to be a no-op rather than an error.
	if err := conn.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("second AutoMigrate: %v", err)
	}
}

// TestPostgres_ImageIndexShapes records the indexes AutoMigrate builds on
// images, in the order it builds their columns in.
//
// The order is not decoration. idx_random is what the device request seeks
// through, and where deleted_at sits inside it decides whether PostgreSQL can
// answer that request from the index at all: the clause against that column is
// deleted_at IS NULL, a null test rather than an equality, so the planner
// cannot fold it away to a constant, and every column after it — rnd — is
// therefore unavailable as a sort order. Measured on a 190,000-row catalogue,
// the delivery query falls back to a parallel sequential scan and a top-N sort
// for any drawn value that matches a large fraction of the table.
//
// This test states what is there rather than what ought to be. It is the place
// the decision would show up if the column order is ever changed, and the same
// order is written down in docs/postgres-migration.md, which would then need
// changing with it.
func TestPostgres_ImageIndexShapes(t *testing.T) {
	conn, _ := setupPostgres(t)

	for _, want := range []struct{ name, columns string }{
		{"idx_random", "(catalog_key, image_orientation, deleted_at, excluded, rnd)"},
		{"idx_list_catalog", "(catalog_key, excluded, taken_at, deleted_at)"},
		{"idx_list_all", "(deleted_at, taken_at)"},
		{"idx_catalog_src", "(catalog_key, src_hash)"},
	} {
		def := pgIndexDef(t, conn, want.name)
		if !strings.Contains(def, want.columns) {
			t.Errorf("%s is %q, want one over %s", want.name, def, want.columns)
		}
	}
	// idx_catalog_src is what makes both upserts an upsert rather than a second
	// row, so it has to be unique and the others must not be.
	if def := pgIndexDef(t, conn, "idx_catalog_src"); !strings.Contains(def, "CREATE UNIQUE INDEX") {
		t.Errorf("idx_catalog_src is %q, want a unique index", def)
	}
	if def := pgIndexDef(t, conn, "idx_random"); strings.Contains(def, "UNIQUE") {
		t.Errorf("idx_random is %q, want a non-unique index — rnd repeats", def)
	}
}

// --- upserts ---------------------------------------------------------------

// TestPostgres_UpsertActiveImage_InsertsThenUpdates walks the path a scan takes
// twice over the same file: the first pass inserts, the second finds the unique
// index on (catalog_key, src_hash) and updates in place.
func TestPostgres_UpsertActiveImage_InsertsThenUpdates(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	rec := dummyImage("cat")
	rec.Rnd = 0.25
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("insert: %v", err)
	}

	again := dummyImage("cat")
	again.SrcHash = rec.SrcHash // same file, seen again
	again.Src = rec.Src
	again.Rnd = 0.75
	again.ImageOrientation = model.ImgCanonicalOrientationPortrait
	again.ThumbJPG = []byte("second-thumb")
	if err := repo.UpsertActiveImage(again); err != nil {
		t.Fatalf("update: %v", err)
	}

	var rows []model.Image
	if err := conn.Unscoped().Where("catalog_key = ?", "cat").Find(&rows).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected the second upsert to update in place, got %d rows", len(rows))
	}
	got := rows[0]
	if got.Rnd != 0.75 {
		t.Errorf("rnd is %v, want 0.75", got.Rnd)
	}
	if got.ImageOrientation != model.ImgCanonicalOrientationPortrait {
		t.Errorf("image_orientation is %v, want portrait", got.ImageOrientation)
	}
	if string(got.ThumbJPG) != "second-thumb" {
		t.Errorf("thumb_jpg is %q, want %q", got.ThumbJPG, "second-thumb")
	}
}

// TestPostgres_UpsertActiveImage_KeepsDeletedAt guards the one column the
// upsert deliberately leaves out. deleted_at belongs to the user, and a scan
// that reset it would put every hidden photograph back on the panel.
func TestPostgres_UpsertActiveImage_KeepsDeletedAt(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	rec := dummyImage("cat")
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("insert: %v", err)
	}
	id := imageIDOf(t, conn, rec.SrcHash)
	if err := repo.ToggleDeletedAt([]model.PrimaryKey{id}); err != nil {
		t.Fatalf("ToggleDeletedAt: %v", err)
	}

	again := dummyImage("cat")
	again.SrcHash = rec.SrcHash
	if err := repo.UpsertActiveImage(again); err != nil {
		t.Fatalf("re-scan: %v", err)
	}

	var count int64
	if err := conn.Unscoped().Model(&model.Image{}).
		Where("id = ? AND deleted_at IS NOT NULL", id).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Error("the re-scan cleared deleted_at, un-hiding a photograph the user hid")
	}
}

// TestPostgres_UpsertExcludedColumnBindsToTheProposedRow is the reason these
// tests exist at all.
//
// PostgreSQL's ON CONFLICT DO UPDATE exposes the row that failed to insert
// under the name EXCLUDED, and this model has a column of its own called
// excluded. The generated assignment therefore reads
//
//	"excluded"="excluded"."excluded"
//
// which is only correct if the right-hand side resolves to the proposed row's
// column and not to the target row's. Were it the target's, the assignment
// would be a no-op: a file that moved out of the catalogue's criteria would go
// on being displayed, and one that moved in would stay invisible — with no
// error anywhere to say so. The check is therefore behavioural, in both
// directions, rather than a reading of the SQL.
func TestPostgres_UpsertExcludedColumnBindsToTheProposedRow(t *testing.T) {
	repo, conn, rec := setupPostgresImageRepo(t)

	excludedOf := func(srcHash string) bool {
		t.Helper()
		var img model.Image
		if err := conn.Unscoped().Where("src_hash = ?", srcHash).Take(&img).Error; err != nil {
			t.Fatalf("read back %s: %v", srcHash, err)
		}
		return img.Excluded
	}

	// false -> true: the file stopped matching the catalogue's criteria.
	active := dummyImage("cat")
	if err := repo.UpsertActiveImage(active); err != nil {
		t.Fatalf("insert active: %v", err)
	}
	if excludedOf(active.SrcHash) {
		t.Fatal("precondition: a freshly scanned image should not be excluded")
	}
	if err := repo.UpsertInactiveImage("cat", active.SrcHash, active.Src); err != nil {
		t.Fatalf("UpsertInactiveImage over an active row: %v", err)
	}
	if !excludedOf(active.SrcHash) {
		t.Error("excluded stayed false: the ON CONFLICT assignment read the target row, " +
			"not the proposed one, so a file that left the catalogue's criteria is still shown")
	}

	// true -> false: and back again, which is the direction that leaves a
	// photograph invisible if it silently fails.
	inactive := randomHash()
	if err := repo.UpsertInactiveImage("cat", inactive, "/inactive.jpg"); err != nil {
		t.Fatalf("insert inactive: %v", err)
	}
	if !excludedOf(inactive) {
		t.Fatal("precondition: UpsertInactiveImage should have inserted excluded = true")
	}
	back := dummyImage("cat")
	back.SrcHash = inactive
	back.Src = "/inactive.jpg"
	if err := repo.UpsertActiveImage(back); err != nil {
		t.Fatalf("UpsertActiveImage over an excluded row: %v", err)
	}
	if excludedOf(inactive) {
		t.Error("excluded stayed true: a file that came back into the catalogue's criteria " +
			"is still hidden")
	}

	// Having established what it does, record what it says, so a change of
	// spelling in a future GORM cannot pass unnoticed.
	stmt := rec.last("ON CONFLICT")
	if stmt == "" {
		t.Fatal("no ON CONFLICT statement was recorded")
	}
	if !strings.Contains(stmt, `"excluded"="excluded"."excluded"`) {
		t.Errorf("the upsert no longer assigns excluded from the EXCLUDED pseudo-table:\n%s", stmt)
	}
	t.Logf("generated upsert: %s", stmt)
}

// TestPostgres_UpsertInactiveImage_LeavesTheRestAlone: a file excluded by the
// catalogue's criteria keeps everything already known about it, so that
// including it again costs no re-decode.
func TestPostgres_UpsertInactiveImage_LeavesTheRestAlone(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	active := dummyImage("cat")
	active.Rnd = 0.125
	active.ThumbJPG = []byte("keep-me")
	if err := repo.UpsertActiveImage(active); err != nil {
		t.Fatalf("insert active: %v", err)
	}
	if err := repo.UpsertInactiveImage("cat", active.SrcHash, active.Src); err != nil {
		t.Fatalf("UpsertInactiveImage: %v", err)
	}

	var img model.Image
	if err := conn.Unscoped().Where("src_hash = ?", active.SrcHash).Take(&img).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if img.Rnd != 0.125 {
		t.Errorf("rnd is %v, want 0.125 — UpsertInactiveImage should update excluded only", img.Rnd)
	}
	if string(img.ThumbJPG) != "keep-me" {
		t.Errorf("thumb_jpg is %q, want %q", img.ThumbJPG, "keep-me")
	}
}

// --- ToggleDeletedAt -------------------------------------------------------

// TestPostgres_ToggleDeletedAt exercises the second hand-built CASE. Its arms
// are CURRENT_TIMESTAMP and NULL rather than placeholders, so PostgreSQL has a
// type to work from — but that is a claim worth holding to a live server
// rather than to reasoning, and the round trip has to work in both directions.
func TestPostgres_ToggleDeletedAt(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	var ids []model.PrimaryKey
	for range 3 {
		rec := dummyImage("cat")
		if err := repo.UpsertActiveImage(rec); err != nil {
			t.Fatalf("insert: %v", err)
		}
		ids = append(ids, imageIDOf(t, conn, rec.SrcHash))
	}

	hidden := func() int64 {
		t.Helper()
		var n int64
		if err := conn.Unscoped().Model(&model.Image{}).
			Where("deleted_at IS NOT NULL").Count(&n).Error; err != nil {
			t.Fatalf("count hidden: %v", err)
		}
		return n
	}

	// Hide two of the three.
	if err := repo.ToggleDeletedAt(ids[:2]); err != nil {
		t.Fatalf("hide: %v", err)
	}
	if got := hidden(); got != 2 {
		t.Fatalf("after hiding two, %d rows carry deleted_at", got)
	}

	// The same call on the same ids has to put them back.
	if err := repo.ToggleDeletedAt(ids[:2]); err != nil {
		t.Fatalf("unhide: %v", err)
	}
	if got := hidden(); got != 0 {
		t.Errorf("after toggling back, %d rows still carry deleted_at", got)
	}

	// A mixed set: one hidden, one not, and one statement that swaps both.
	if err := repo.ToggleDeletedAt(ids[:1]); err != nil {
		t.Fatalf("hide one: %v", err)
	}
	if err := repo.ToggleDeletedAt(ids[:2]); err != nil {
		t.Fatalf("toggle a mixed set: %v", err)
	}
	var stillHidden int64
	if err := conn.Unscoped().Model(&model.Image{}).
		Where("id = ? AND deleted_at IS NOT NULL", ids[0]).Count(&stillHidden).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if stillHidden != 0 {
		t.Error("the row that was hidden should have been revealed")
	}
	if got := hidden(); got != 1 {
		t.Errorf("expected exactly the other row to be hidden, %d rows carry deleted_at", got)
	}
}

// --- ReshuffleRandom -------------------------------------------------------

// TestPostgres_ReshuffleRandom_SpacesValuesEvenly is the regression test for
// the bug this file was written for. The statement builds one CASE arm per row
// out of placeholders, and PostgreSQL resolves an untyped placeholder there to
// text, so before the ELSE arm was added every batch failed with
//
//	column "rnd" is of type double precision but expression is of type text
//
// which stopped catalog scan's even-spreading pass outright. The assertion is
// the same one the SQLite test makes: every gap the same width, since that is
// what the pass exists to produce.
func TestPostgres_ReshuffleRandom_SpacesValuesEvenly(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	const n = 1200 // spans more than two batches
	for i := range n {
		if err := repo.UpsertActiveImage(dummyImage("cat")); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	if err := repo.ReshuffleRandom(nil); err != nil {
		t.Fatalf("ReshuffleRandom: %v", err)
	}

	var vals []float64
	if err := conn.Unscoped().Model(&model.Image{}).Order("rnd").Pluck("rnd", &vals).Error; err != nil {
		t.Fatalf("read rnd: %v", err)
	}
	if len(vals) != n {
		t.Fatalf("row count changed: got %d, want %d", len(vals), n)
	}

	want := 1.0 / float64(n)
	const tolerance = 1e-9
	prev := 0.0
	for i, v := range vals {
		if diff := (v - prev) - want; diff > tolerance || diff < -tolerance {
			t.Fatalf("gap %d is %g, want %g — the values are not evenly spaced", i, v-prev, want)
		}
		prev = v
	}
	if diff := vals[len(vals)-1] - 1.0; diff > tolerance || diff < -tolerance {
		t.Errorf("largest value is %g, want 1.0 so no draw falls past the last row", vals[len(vals)-1])
	}
}

// TestPostgres_ReshuffleRandom_ReportsProgress: the pass is slow enough that
// the caller narrates it, and the counts it is given come from the same loop
// that sends the statements.
func TestPostgres_ReshuffleRandom_ReportsProgress(t *testing.T) {
	repo, _, _ := setupPostgresImageRepo(t)

	const n = 5
	for i := range n {
		if err := repo.UpsertActiveImage(dummyImage("cat")); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	var calls [][2]int
	if err := repo.ReshuffleRandom(func(done, total int) {
		calls = append(calls, [2]int{done, total})
	}); err != nil {
		t.Fatalf("ReshuffleRandom: %v", err)
	}
	if len(calls) < 2 {
		t.Fatalf("expected a call before the work and one after each batch, got %d", len(calls))
	}
	if calls[0] != [2]int{0, n} {
		t.Errorf("first progress call is %v, want [0 %d]", calls[0], n)
	}
	if last := calls[len(calls)-1]; last != [2]int{n, n} {
		t.Errorf("last progress call is %v, want [%d %d]", last, n, n)
	}
}

// TestPostgres_ReshuffleRandom_EmptyTable: an empty catalogue is legitimate,
// and the statement is never built, so nothing should reach the server.
func TestPostgres_ReshuffleRandom_EmptyTable(t *testing.T) {
	repo, _, _ := setupPostgresImageRepo(t)

	if err := repo.ReshuffleRandom(nil); err != nil {
		t.Errorf("ReshuffleRandom on an empty table should not error, got: %v", err)
	}
}

// TestPostgres_ReshuffleRandom_KeepsRowsItWasNotGivenAValueFor covers the arm
// added for PostgreSQL in its own right. A row inside the batch's id range that
// no WHEN names — one inserted between the SELECT and the UPDATE — used to be
// handed the CASE's implicit NULL against a NOT NULL column. It now keeps what
// it had.
func TestPostgres_ReshuffleRandom_KeepsRowsItWasNotGivenAValueFor(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	for i := range 4 {
		if err := repo.UpsertActiveImage(dummyImage("cat")); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	var ids []model.PrimaryKey
	if err := conn.Model(&model.Image{}).Order("id").Pluck("id", &ids).Error; err != nil {
		t.Fatalf("pluck ids: %v", err)
	}

	// Stand in for the row arriving late by removing it from the id list the
	// statement is built from: the repository never sees it, but it sits inside
	// the range the UPDATE covers.
	unseen := ids[1]
	if err := conn.Unscoped().Model(&model.Image{}).Where("id = ?", unseen).
		Update("rnd", 0.4242).Error; err != nil {
		t.Fatalf("set the sentinel value: %v", err)
	}
	if err := conn.Exec(
		"UPDATE images SET rnd = CASE id WHEN ? THEN ? WHEN ? THEN ? ELSE rnd END WHERE id BETWEEN ? AND ?",
		ids[0], 0.1, ids[2], 0.2, ids[0], ids[2],
	).Error; err != nil {
		t.Fatalf("the statement shape ReshuffleRandom builds: %v", err)
	}

	if got := readRnd(t, conn, unseen); got != 0.4242 {
		t.Errorf("the unnamed row's rnd is %v, want 0.4242 — it was overwritten", got)
	}
	if got := readRnd(t, conn, ids[0]); got != 0.1 {
		t.Errorf("named row rnd is %v, want 0.1", got)
	}
}

// TestPostgres_ReshuffleRandom_CommitsEachBatch holds the pass to the one
// promise its comment makes: it is deliberately not one transaction, so that a
// panel waking mid-pass is not left waiting on a lock over every row in the
// catalogue.
//
// The check is what another session can see while the pass is still running.
// Every row starts at a sentinel; when the first batch reports done, a
// connection of its own counts how many rows have moved off it. Under per-batch
// commits that is the batch just finished. Wrapped in one transaction it would
// be none, and nothing else here would notice — the values all arrive in the
// end either way.
func TestPostgres_ReshuffleRandom_CommitsEachBatch(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)
	other := secondPostgresConnection(t)

	const n = 1200 // more than two batches, so a batch reports before the end
	for i := range n {
		if err := repo.UpsertActiveImage(dummyImage("cat")); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	// A value the pass cannot produce: every rank it hands out is above zero.
	const sentinel = -1.0
	if err := conn.Exec("UPDATE images SET rnd = ?", sentinel).Error; err != nil {
		t.Fatalf("set the sentinel: %v", err)
	}

	movedOffSentinel := func(db *gorm.DB) int64 {
		t.Helper()
		var count int64
		if err := db.Unscoped().Model(&model.Image{}).
			Where("rnd <> ?", sentinel).Count(&count).Error; err != nil {
			t.Fatalf("count: %v", err)
		}
		return count
	}
	if got := movedOffSentinel(other); got != 0 {
		t.Fatalf("precondition: %d rows are already off the sentinel", got)
	}

	seenMidPass := int64(-1)
	var midPassDone int
	if err := repo.ReshuffleRandom(func(done, total int) {
		// The first batch to finish, and only that one: later ones say the same
		// thing and the last is after the pass, when a single transaction would
		// have committed too.
		if seenMidPass >= 0 || done == 0 || done >= total {
			return
		}
		midPassDone = done
		seenMidPass = movedOffSentinel(other)
	}); err != nil {
		t.Fatalf("ReshuffleRandom: %v", err)
	}

	if midPassDone == 0 {
		t.Fatal("no batch reported before the end; the test saw nothing to check")
	}
	if seenMidPass == 0 {
		t.Errorf("another session saw 0 rows off the sentinel after %d had been written; "+
			"the pass is holding its work in one transaction, and a delivery arriving "+
			"now would wait out the whole of it", midPassDone)
	} else if seenMidPass != int64(midPassDone) {
		t.Errorf("another session saw %d rows off the sentinel after %d had been written",
			seenMidPass, midPassDone)
	}

	if got := movedOffSentinel(other); got != n {
		t.Errorf("%d of %d rows are off the sentinel once the pass has finished", got, n)
	}
}

// TestPostgres_ReshuffleRandom_KeepsDeliveriesAnswerable asks for pictures
// throughout the pass, from a connection of its own, the way a panel does.
//
// Two things could go wrong and neither would show up in a test that runs the
// pass on its own. The delivery path writes rnd back on every request, so it
// and the pass are two writers on the same rows and could deadlock. And the
// pass leaves the table half re-spaced for its duration, which a draw has to be
// able to land on: the values it has written and the values it has not both
// cover (0, 1], so no gap opens, but that is a claim about a live server rather
// than about the statement.
func TestPostgres_ReshuffleRandom_KeepsDeliveriesAnswerable(t *testing.T) {
	repo, _, _ := setupPostgresImageRepo(t)
	deliveries := infraRepo.NewImageRepositoryImpl(secondPostgresConnection(t))

	const n = 1200
	for i := range n {
		if err := repo.UpsertActiveImage(dummyImage("cat")); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	filter := model.ImageFilter{
		CatalogKeys: []string{"cat"},
		Orientation: model.ImgCanonicalOrientationLandscape,
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	var drawn int
	var failure error
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			img, err := deliveries.FindByRandom(filter)
			if err != nil {
				failure = err
				return
			}
			if img.Rnd <= 0 || img.Rnd > 1 {
				failure = errors.New("a delivery came back with rnd outside (0, 1]")
				return
			}
			drawn++
		}
	}()

	err := repo.ReshuffleRandom(nil)
	close(stop)
	wg.Wait()

	if err != nil {
		t.Fatalf("ReshuffleRandom: %v", err)
	}
	if failure != nil {
		t.Fatalf("a delivery failed while the pass was running: %v", failure)
	}
	if drawn == 0 {
		t.Fatal("no delivery was attempted while the pass was running")
	}
	t.Logf("%d deliveries answered during the pass", drawn)
}

// TestPostgres_RndRoundTrips checks the column is wide enough for what is put
// in it. A numeric column would satisfy every other assertion here while
// rounding the value on its way in, and the spacing test's tolerance is loose
// enough not to notice.
func TestPostgres_RndRoundTrips(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	const want = 0.0008333333333333334 // 1/1200, as float64 prints it
	rec := dummyImage("cat")
	rec.Rnd = want
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("insert: %v", err)
	}
	id := imageIDOf(t, conn, rec.SrcHash)
	if got := readRnd(t, conn, id); got != want {
		t.Errorf("rnd came back as %v, want %v", got, want)
	}
}

// --- FindByRandom ----------------------------------------------------------

// TestPostgres_FindByRandom is the device delivery path: the panel wakes, asks
// for an image, and this is the query that answers. It filters on rnd, so it
// depends on the column type; it may run either of two branches depending on
// the number drawn, so it is asked repeatedly.
func TestPostgres_FindByRandom(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	wanted := dummyImage("wanted")
	wanted.Rnd = 0.5
	wanted.ImageOrientation = model.ImgCanonicalOrientationLandscape
	if err := repo.UpsertActiveImage(wanted); err != nil {
		t.Fatalf("insert the eligible image: %v", err)
	}

	// Three ways of being ineligible, one per clause of the WHERE.
	other := dummyImage("other")
	if err := repo.UpsertActiveImage(other); err != nil {
		t.Fatalf("insert another catalogue: %v", err)
	}
	portrait := dummyImage("wanted")
	portrait.ImageOrientation = model.ImgCanonicalOrientationPortrait
	if err := repo.UpsertActiveImage(portrait); err != nil {
		t.Fatalf("insert a portrait: %v", err)
	}
	if err := repo.UpsertInactiveImage("wanted", randomHash(), "/excluded.jpg"); err != nil {
		t.Fatalf("insert an excluded image: %v", err)
	}

	filter := model.ImageFilter{
		CatalogKeys: []string{"wanted"},
		Orientation: model.ImgCanonicalOrientationLandscape,
	}
	// Both branches — rnd >= drawn and, when that finds nothing, rnd < drawn —
	// have to be taken, and which one runs depends on a number drawn inside the
	// repository. Twenty draws makes missing one of them vanishingly unlikely.
	for i := range 20 {
		img, err := repo.FindByRandom(filter)
		if err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
		if img.SrcHash != wanted.SrcHash {
			t.Fatalf("draw %d returned %q, want the one eligible image", i, img.Src)
		}
		if img.Rnd <= 0 || img.Rnd > 1 {
			t.Fatalf("draw %d left rnd at %v, outside (0, 1]", i, img.Rnd)
		}
		// The new value is written back so the next draw lands elsewhere.
		if stored := readRnd(t, conn, imageIDOf(t, conn, wanted.SrcHash)); stored != img.Rnd {
			t.Fatalf("draw %d: rnd was reported as %v but stored as %v", i, img.Rnd, stored)
		}
	}
}

// TestPostgres_FindByRandom_SkipsHiddenImages: a photograph the user hid is
// soft-deleted, and GORM's own scoping is what keeps it off the panel.
func TestPostgres_FindByRandom_SkipsHiddenImages(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	rec := dummyImage("cat")
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("insert: %v", err)
	}
	id := imageIDOf(t, conn, rec.SrcHash)
	if err := repo.ToggleDeletedAt([]model.PrimaryKey{id}); err != nil {
		t.Fatalf("ToggleDeletedAt: %v", err)
	}

	_, err := repo.FindByRandom(model.ImageFilter{
		CatalogKeys: []string{"cat"},
		Orientation: model.ImgCanonicalOrientationLandscape,
	})
	if !errorIsRecordNotFound(err) {
		t.Errorf("expected gorm.ErrRecordNotFound once the only image is hidden, got %v", err)
	}
}

// TestPostgres_FindByRandom_TagFilter covers the EXISTS subquery, which is only
// built when the display names tags.
func TestPostgres_FindByRandom_TagFilter(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)
	tags := infraRepo.NewTagRepositoryImpl(conn)

	tagged := dummyImage("cat")
	if err := repo.UpsertActiveImage(tagged); err != nil {
		t.Fatalf("insert the tagged image: %v", err)
	}
	if err := repo.UpsertActiveImage(dummyImage("cat")); err != nil {
		t.Fatalf("insert the untagged image: %v", err)
	}
	id := imageIDOf(t, conn, tagged.SrcHash)

	tag, err := tags.FindOrCreateTag("Sunset")
	if err != nil {
		t.Fatalf("FindOrCreateTag: %v", err)
	}
	if err := tags.ReplaceImageTags(id, []model.PrimaryKey{tag.ID}); err != nil {
		t.Fatalf("ReplaceImageTags: %v", err)
	}

	filter := model.ImageFilter{
		CatalogKeys: []string{"cat"},
		Orientation: model.ImgCanonicalOrientationLandscape,
		Tags:        []string{"sunset"},
	}
	for i := range 10 {
		img, err := repo.FindByRandom(filter)
		if err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
		if img.SrcHash != tagged.SrcHash {
			t.Fatalf("draw %d returned an image without the tag", i)
		}
	}

	filter.Tags = []string{"nothing-has-this-tag"}
	if _, err := repo.FindByRandom(filter); !errorIsRecordNotFound(err) {
		t.Errorf("expected gorm.ErrRecordNotFound for an unused tag, got %v", err)
	}
}

// TestPostgres_FindImagesWithoutTags covers tag.go's NOT IN subquery, which
// decides what the tagging pass has left to do.
func TestPostgres_FindImagesWithoutTags(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)
	tags := infraRepo.NewTagRepositoryImpl(conn)

	for range 3 {
		if err := repo.UpsertActiveImage(dummyImage("cat")); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	var ids []model.PrimaryKey
	if err := conn.Model(&model.Image{}).Order("id").Pluck("id", &ids).Error; err != nil {
		t.Fatalf("pluck ids: %v", err)
	}

	untagged, err := tags.FindImagesWithoutTags("cat", 0)
	if err != nil {
		t.Fatalf("FindImagesWithoutTags: %v", err)
	}
	if len(untagged) != 3 {
		t.Fatalf("expected all 3 images to want tagging, got %d", len(untagged))
	}

	tag, err := tags.FindOrCreateTag("beach")
	if err != nil {
		t.Fatalf("FindOrCreateTag: %v", err)
	}
	if err := tags.ReplaceImageTags(ids[0], []model.PrimaryKey{tag.ID}); err != nil {
		t.Fatalf("ReplaceImageTags: %v", err)
	}

	untagged, err = tags.FindImagesWithoutTags("cat", 0)
	if err != nil {
		t.Fatalf("FindImagesWithoutTags after tagging: %v", err)
	}
	if len(untagged) != 2 {
		t.Fatalf("expected 2 images left to tag, got %d", len(untagged))
	}
	for _, id := range untagged {
		if id == ids[0] {
			t.Error("the tagged image is still being offered for tagging")
		}
	}
}

// --- the rest of the image repository --------------------------------------

// TestPostgres_ListByCatalog covers what the catalogue page asks for. It is the
// one query read through Rows and ScanRows rather than into a slice, and it
// makes the distinction the two hiding mechanisms rest on: excluded is the
// catalogue's decision and takes the row out of the listing, deleted_at is the
// user's and leaves it in, marked.
func TestPostgres_ListByCatalog(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	newest := dummyImage("cat")
	newest.TakenAt.Time = time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	oldest := dummyImage("cat")
	oldest.TakenAt.Time = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	middle := dummyImage("cat")
	middle.TakenAt.Time = time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	for _, rec := range []*model.Image{newest, oldest, middle} {
		if err := repo.UpsertActiveImage(rec); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	// Hidden by the user: still listed.
	if err := repo.ToggleDeletedAt([]model.PrimaryKey{imageIDOf(t, conn, middle.SrcHash)}); err != nil {
		t.Fatalf("ToggleDeletedAt: %v", err)
	}
	// Excluded by the catalogue, and in another catalogue: neither listed.
	if err := repo.UpsertInactiveImage("cat", randomHash(), "/excluded.jpg"); err != nil {
		t.Fatalf("UpsertInactiveImage: %v", err)
	}
	if err := repo.UpsertActiveImage(dummyImage("other")); err != nil {
		t.Fatalf("insert into another catalogue: %v", err)
	}

	// ListByCatalog selects only the columns the listing shows, src among them
	// and src_hash not, so the identity asserted on here has to be src.
	var listed []string
	var hidden int
	if err := repo.ListByCatalog("cat", nil, func(img *model.Image) error {
		listed = append(listed, img.Src)
		if img.DeletedAt.Valid {
			hidden++
		}
		return nil
	}); err != nil {
		t.Fatalf("ListByCatalog: %v", err)
	}

	if len(listed) != 3 {
		t.Fatalf("listed %d images, want the 3 in this catalogue", len(listed))
	}
	if hidden != 1 {
		t.Errorf("%d listed images carry deleted_at, want 1", hidden)
	}
	// Ordered by taken_at descending, newest first.
	want := []string{newest.Src, middle.Src, oldest.Src}
	for i := range want {
		if listed[i] != want[i] {
			t.Fatalf("position %d holds the wrong image — the listing is not ordered by taken_at desc", i)
		}
	}
}

// TestPostgres_ListByCatalog_PutsUndatedPhotosFirst pins the one difference an
// operator sees the moment they migrate, and the only one the migration guide
// promises them in advance.
//
// PostgreSQL sorts NULL above every value; MySQL, MariaDB and SQLite sort it
// below. ListByCatalog orders by taken_at descending and backs the photo grid,
// so a photograph whose file carried no EXIF date — a scan, a screenshot,
// anything through an export that strips metadata — moves from the end of the
// grid to the front. The dated photographs keep their order relative to one
// another, which is the half of the claim worth checking: it is what makes the
// change cosmetic rather than a reordering of the catalogue.
//
// The sibling test on SQLite is TestListByCatalogOrdersUndatedPhotosLast; the
// two together are the whole of the documented difference.
func TestPostgres_ListByCatalog_PutsUndatedPhotosFirst(t *testing.T) {
	repo, _, _ := setupPostgresImageRepo(t)

	newest := dummyImage("cat")
	newest.TakenAt.Time = time.Date(2023, 7, 15, 17, 45, 0, 0, time.UTC)
	middle := dummyImage("cat")
	middle.TakenAt.Time = time.Date(2021, 11, 20, 8, 30, 0, 0, time.UTC)
	oldest := dummyImage("cat")
	oldest.TakenAt.Time = time.Date(2019, 5, 4, 10, 0, 0, 0, time.UTC)
	undated := dummyImage("cat")
	undated.TakenAt.Valid = false

	// Inserted in an order that is neither the stored one nor the expected one,
	// so a listing that happened to come back in insertion order would fail.
	for _, rec := range []*model.Image{middle, undated, oldest, newest} {
		if err := repo.UpsertActiveImage(rec); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	var listed []string
	if err := repo.ListByCatalog("cat", nil, func(img *model.Image) error {
		listed = append(listed, img.Src)
		return nil
	}); err != nil {
		t.Fatalf("ListByCatalog: %v", err)
	}

	want := []string{undated.Src, newest.Src, middle.Src, oldest.Src}
	if len(listed) != len(want) {
		t.Fatalf("listed %d images, want %d", len(listed), len(want))
	}
	for i := range want {
		if listed[i] != want[i] {
			t.Fatalf("position %d holds %q, want %q — the listing is not "+
				"taken_at desc with the undated photograph first:\ngot  %v\nwant %v",
				i, listed[i], want[i], listed, want)
		}
	}
}

// TestPostgres_RemainingImageQueries walks the rest of the interface, none of
// which builds SQL by hand but all of which a device or the UI depends on. The
// point is to have run them against PostgreSQL rather than to have read them.
func TestPostgres_RemainingImageQueries(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	rec := dummyImage("cat")
	rec.SrcType = "http"
	rec.ImageData = []byte("fake-jpeg-binary")
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("insert: %v", err)
	}
	id := imageIDOf(t, conn, rec.SrcHash)

	// FindByHash reports the modification time a scan compares against, and
	// (nil, nil) for a file it has not seen.
	found, err := repo.FindByHash("cat", rec.SrcHash)
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if found == nil || !found.FileModifiedAt.Valid {
		t.Fatal("FindByHash did not return the record's file_modified_at")
	}
	missing, err := repo.FindByHash("cat", randomHash())
	if err != nil {
		t.Fatalf("FindByHash for an unknown hash: %v", err)
	}
	if missing != nil {
		t.Error("FindByHash should return (nil, nil) for a file it has not seen")
	}

	if _, err := repo.FindById(id); err != nil {
		t.Fatalf("FindById: %v", err)
	}

	var seen int
	repo.FindAll(func(*model.Image) error {
		seen++
		return nil
	})
	if seen != 1 {
		t.Errorf("FindAll walked %d records, want 1", seen)
	}

	// bytea round trip: the blob an HTTP catalogue keeps in the database.
	data, err := repo.FindImageData(id)
	if err != nil {
		t.Fatalf("FindImageData: %v", err)
	}
	if string(data) != "fake-jpeg-binary" {
		t.Errorf("image_data came back as %q", data)
	}

	byOrientation, err := repo.CountByCatalog("cat", model.ImgCanonicalOrientationLandscape)
	if err != nil {
		t.Fatalf("CountByCatalog: %v", err)
	}
	if byOrientation != 1 {
		t.Errorf("CountByCatalog returned %d, want 1", byOrientation)
	}
	all, err := repo.CountAllByCatalog("cat")
	if err != nil {
		t.Fatalf("CountAllByCatalog: %v", err)
	}
	if all != 1 {
		t.Errorf("CountAllByCatalog returned %d, want 1", all)
	}

	// Eviction keeps a generated catalogue inside its cap.
	for range 3 {
		if err := repo.UpsertActiveImage(dummyImage("cat")); err != nil {
			t.Fatalf("insert for eviction: %v", err)
		}
	}
	if err := repo.EvictOldestImages("cat", 2); err != nil {
		t.Fatalf("EvictOldestImages: %v", err)
	}
	if all, err = repo.CountAllByCatalog("cat"); err != nil {
		t.Fatalf("CountAllByCatalog after eviction: %v", err)
	} else if all != 2 {
		t.Errorf("%d images survived eviction, want 2", all)
	}

	// RemoveImage is a hard delete, so the row is gone rather than marked.
	var remaining []model.PrimaryKey
	if err := conn.Model(&model.Image{}).Pluck("id", &remaining).Error; err != nil {
		t.Fatalf("pluck: %v", err)
	}
	if err := repo.RemoveImage(remaining[0]); err != nil {
		t.Fatalf("RemoveImage: %v", err)
	}
	var count int64
	if err := conn.Unscoped().Model(&model.Image{}).Where("id = ?", remaining[0]).
		Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Error("RemoveImage left the row behind")
	}
}

// TestPostgres_ImageDataReachesTheDisplayAsAPicture follows the bytes an HTTP
// catalogue stores all the way back out again: into bytea through the upsert a
// fetch performs, out through the loader a device request builds, and into a
// decoded picture.
//
// The column is exercised nowhere else in a running installation. A file
// catalogue never writes it — the scan leaves it NULL and the picture is read
// from disk — so an end-to-end run against a directory of photographs says
// nothing about it at all. What is checked here is that the bytes come back
// identical, that they still decode, and that the loader reports the row they
// came from, since that is what the delivery record is filed under.
func TestPostgres_ImageDataReachesTheDisplayAsAPicture(t *testing.T) {
	repo, conn, _ := setupPostgresImageRepo(t)

	// A real JPEG rather than a string of bytes: the loader decodes what it
	// reads, so anything that is not an image passes the round trip and fails
	// the delivery.
	const width, height = 64, 48
	fill := color.RGBA{R: 200, G: 40, B: 30, A: 255}
	src := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(src, src.Bounds(), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode the fixture: %v", err)
	}
	original := encoded.Bytes()
	digest := func(b []byte) string {
		sum := sha256.Sum256(b)
		return hex.EncodeToString(sum[:])
	}

	rec := dummyImage("remote")
	rec.SrcType = "http"
	rec.Src = "http://example.invalid/image"
	rec.ImageData = original
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("store the fetched image: %v", err)
	}
	id := imageIDOf(t, conn, rec.SrcHash)

	// Byte for byte, out of bytea and back.
	stored, err := repo.FindImageData(id)
	if err != nil {
		t.Fatalf("FindImageData: %v", err)
	}
	if digest(stored) != digest(original) {
		t.Fatalf("image_data came back as %d bytes with digest %s, want %d bytes with digest %s",
			len(stored), digest(stored), len(original), digest(original))
	}

	// And through the loader a device request would build for this catalogue.
	display := epaper.NewDisplay(epaper.WS7in3EPaperE, model.ImgCanonicalOrientationLandscape)
	locator := catalog.NewImageHttpProvider(time.Now(), display, repo, "remote",
		config.ImageHTTPProviderConfig{
			URL:   rec.Src,
			Cache: config.HTTPCacheConfig{Type: "background", Depth: 1},
		})
	loader, err := locator.Resolve()
	if err != nil {
		t.Fatalf("Resolve the background HTTP catalogue: %v", err)
	}
	prov := loader.Provenance()
	if prov.Kind != model.DeliveryKindHTTP {
		t.Errorf("provenance kind is %q, want %q", prov.Kind, model.DeliveryKindHTTP)
	}
	if prov.ImageID != id {
		t.Errorf("provenance names image %d, want the row the bytes were stored in, %d", prov.ImageID, id)
	}

	img, _, err := loader.Load()
	if err != nil {
		t.Fatalf("Load the picture back out of image_data: %v", err)
	}
	if got := img.Bounds(); got.Dx() != width || got.Dy() != height {
		t.Fatalf("the decoded picture is %dx%d, want %dx%d", got.Dx(), got.Dy(), width, height)
	}
	// JPEG is lossy, so the colour is checked to within a tolerance; the point
	// is that these are the fixture's pixels and not another row's.
	r, g, b, _ := img.At(width/2, height/2).RGBA()
	const tolerance = 8 << 8
	near := func(got uint32, want uint8) bool {
		diff := int32(got) - int32(uint32(want)<<8)
		return diff < tolerance && diff > -tolerance
	}
	if !near(r, fill.R) || !near(g, fill.G) || !near(b, fill.B) {
		t.Errorf("the decoded picture's centre is rgb(%d, %d, %d), want about rgb(%d, %d, %d)",
			r>>8, g>>8, b>>8, fill.R, fill.G, fill.B)
	}
}

// --- DropAndRecreate -------------------------------------------------------

// TestPostgres_DropAndRecreate covers system prune, which goes through the
// migrator rather than through SQL of its own. PostgreSQL is the dialect where
// the drop order matters least — every table goes with CASCADE — and where a
// failure to recreate would leave the installation with no schema at all.
func TestPostgres_DropAndRecreate(t *testing.T) {
	conn, _ := setupPostgres(t)
	repo := infraRepo.NewSystemRepositoryImpl(conn)

	img := dummyImage("prunecat")
	if err := conn.Create(img).Error; err != nil {
		t.Fatalf("create image: %v", err)
	}
	tag := &model.Tag{NameNormalized: "sunset", DisplayName: "sunset"}
	if err := conn.Create(tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := conn.Create(&model.ImageTag{ImageID: img.ID, TagID: tag.ID}).Error; err != nil {
		t.Fatalf("create image_tag: %v", err)
	}

	if err := repo.DropAndRecreate(); err != nil {
		t.Fatalf("DropAndRecreate: %v", err)
	}

	for _, m := range model.AllModels() {
		if !conn.Migrator().HasTable(m) {
			t.Fatalf("table for %T was not recreated", m)
		}
		var count int64
		if err := conn.Model(m).Count(&count).Error; err != nil {
			t.Fatalf("count %T: %v", m, err)
		}
		if count != 0 {
			t.Errorf("table for %T: expected 0 rows after prune, got %d", m, count)
		}
	}

	// Writable again, on the same connection, without a restart — and with the
	// unique index rebuilt.
	after := dummyImage("aftercat")
	if err := conn.Create(after).Error; err != nil {
		t.Fatalf("create image after prune: %v", err)
	}
	dup := dummyImage("aftercat")
	dup.SrcHash = after.SrcHash
	if err := conn.Create(dup).Error; err == nil {
		t.Error("expected a unique violation on (catalog_key, src_hash) after prune")
	}

	// bigserial has to have been recreated too, or the next insert has no id.
	if after.ID == 0 {
		t.Error("the recreated table did not assign an id")
	}
}

// errorIsRecordNotFound keeps the not-found checks above to one line each.
func errorIsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

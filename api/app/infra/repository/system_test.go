package repository_test

import (
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"

	"gorm.io/gorm"
)

// --- helpers ---------------------------------------------------------------

// setupSystemRepo creates an in-memory SQLite DB with every model in
// model.AllModels() migrated — the same schema the DI container builds at
// startup — and returns the system repository plus the raw *gorm.DB.
func setupSystemRepo(t *testing.T) (repository.SystemRepository, *gorm.DB) {
	t.Helper()
	conn, err := infra.NewSqliteConnection("", true)
	if err != nil {
		t.Fatalf("NewSqliteConnection: %v", err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatalf("conn.DB(): %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := conn.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return infraRepo.NewSystemRepositoryImpl(conn), conn
}

// countRows returns the number of rows in the table backing the given model.
// It fails the test if the table is missing, so a dropped table is reported as
// a failure rather than as a zero count.
func countRows(t *testing.T, conn *gorm.DB, value any) int64 {
	t.Helper()
	var count int64
	if err := conn.Model(value).Count(&count).Error; err != nil {
		t.Fatalf("count %T: %v", value, err)
	}
	return count
}

// --- DropAndRecreate tests -------------------------------------------------

// TestDropAndRecreate populates every table, runs DropAndRecreate and verifies
// that all tables exist again and are empty.
func TestDropAndRecreate(t *testing.T) {
	repo, conn := setupSystemRepo(t)

	// Populate every table so that "recreated empty" is observable. The loop
	// below reads model.AllModels(), so a model added later fails here until it
	// is given a row — which is the point: an unpopulated table would make
	// "empty afterwards" vacuously true.
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
	if err := conn.Create(&model.DeliveryHistory{
		DisplayKey:  "prunedisplay",
		Slot:        0,
		Seq:         1,
		DeliveredAt: time.Now().UTC(),
		Kind:        model.DeliveryKindPhoto,
		ImageID:     img.ID,
		CatalogKey:  "prunecat",
		Source:      img.Src,
	}).Error; err != nil {
		t.Fatalf("create delivery_history: %v", err)
	}
	for _, m := range model.AllModels() {
		if got := countRows(t, conn, m); got == 0 {
			t.Fatalf("precondition: %T should have rows before prune", m)
		}
	}

	if err := repo.DropAndRecreate(); err != nil {
		t.Fatalf("DropAndRecreate: %v", err)
	}

	// Every model's table must exist again...
	for _, m := range model.AllModels() {
		if !conn.Migrator().HasTable(m) {
			t.Errorf("table for %T was not recreated", m)
		}
	}
	// ...and be empty.
	for _, m := range model.AllModels() {
		if got := countRows(t, conn, m); got != 0 {
			t.Errorf("table for %T: expected 0 rows after prune, got %d", m, got)
		}
	}
}

// TestDropAndRecreate_UsableAfterwards verifies the recreated schema is not
// merely present but writable: indexes are rebuilt too, so the application can
// keep using the same connection without restarting.
func TestDropAndRecreate_UsableAfterwards(t *testing.T) {
	repo, conn := setupSystemRepo(t)

	if err := repo.DropAndRecreate(); err != nil {
		t.Fatalf("DropAndRecreate: %v", err)
	}

	img := dummyImage("aftercat")
	if err := conn.Create(img).Error; err != nil {
		t.Fatalf("create image after prune: %v", err)
	}
	tag := &model.Tag{NameNormalized: "beach", DisplayName: "beach"}
	if err := conn.Create(tag).Error; err != nil {
		t.Fatalf("create tag after prune: %v", err)
	}
	if err := conn.Create(&model.ImageTag{ImageID: img.ID, TagID: tag.ID}).Error; err != nil {
		t.Fatalf("create image_tag after prune: %v", err)
	}

	// The unique index on tags.name_normalized must have been recreated.
	dup := &model.Tag{NameNormalized: "beach", DisplayName: "beach"}
	if err := conn.Create(dup).Error; err == nil {
		t.Error("expected unique index violation on tags.name_normalized after prune")
	}
}

// TestDropAndRecreate_Idempotent verifies DropAndRecreate can run twice in a
// row, the second time against an already-empty schema.
func TestDropAndRecreate_Idempotent(t *testing.T) {
	repo, conn := setupSystemRepo(t)

	for i := range 2 {
		if err := repo.DropAndRecreate(); err != nil {
			t.Fatalf("DropAndRecreate (run %d): %v", i+1, err)
		}
	}
	for _, m := range model.AllModels() {
		if !conn.Migrator().HasTable(m) {
			t.Errorf("table for %T missing after two runs", m)
		}
	}
}

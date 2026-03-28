package repository_test

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"

	"gorm.io/gorm"
)

// --- helpers ---------------------------------------------------------------

// setupRepo creates an in-memory SQLite DB with the Image table migrated
// and returns the GORM repository implementation plus the raw *gorm.DB
// for low-level assertions.
func setupRepo(t *testing.T) (repository.ImageRepository, *gorm.DB) {
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
	if err := conn.AutoMigrate(&model.Image{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return infraRepo.NewImageRepositoryImpl(conn), conn
}

// randomHash returns a random 40-character hex string for SrcHash uniqueness.
func randomHash() string {
	b := make([]byte, 20)
	for i := range b {
		b[i] = byte(rand.IntN(256))
	}
	return fmt.Sprintf("%x", b)
}

// dummyImage builds a minimal Image record for testing.
func dummyImage(catalogKey string) *model.Image {
	return &model.Image{
		CatalogKey:       catalogKey,
		Rnd:              rand.Float64(),
		Src:              "/dummy/" + randomHash()[:8] + ".jpg",
		SrcHash:          randomHash(),
		SrcType:          "file",
		TakenAt:          sql.NullTime{Time: time.Now().UTC(), Valid: true},
		ImageOrientation: model.ImgCanonicalOrientationLandscape,
		ThumbJPG:         []byte("fakethumb"),
		FileModifiedAt:   sql.NullTime{Time: time.Now().UTC(), Valid: true},
	}
}

// --- CountAllByCatalog tests -----------------------------------------------

// TestCountAllByCatalog inserts images with different orientations and
// verifies CountAllByCatalog returns the total count regardless of orientation.
func TestCountAllByCatalog(t *testing.T) {
	repo, _ := setupRepo(t)

	// Insert 2 landscape + 1 portrait images.
	for range 2 {
		rec := dummyImage("mycat")
		rec.ImageOrientation = model.ImgCanonicalOrientationLandscape
		if err := repo.UpsertActiveImage(rec); err != nil {
			t.Fatalf("UpsertActiveImage: %v", err)
		}
	}
	rec := dummyImage("mycat")
	rec.ImageOrientation = model.ImgCanonicalOrientationPortrait
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("UpsertActiveImage: %v", err)
	}

	count, err := repo.CountAllByCatalog("mycat")
	if err != nil {
		t.Fatalf("CountAllByCatalog: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 images, got %d", count)
	}
}

// TestCountAllByCatalog_ExcludesExcluded verifies that excluded images
// are not counted by CountAllByCatalog.
func TestCountAllByCatalog_ExcludesExcluded(t *testing.T) {
	repo, _ := setupRepo(t)

	rec := dummyImage("mycat")
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("UpsertActiveImage: %v", err)
	}
	// Insert an excluded image.
	if err := repo.UpsertInactiveImage("mycat", randomHash(), "/excluded.jpg"); err != nil {
		t.Fatalf("UpsertInactiveImage: %v", err)
	}

	count, err := repo.CountAllByCatalog("mycat")
	if err != nil {
		t.Fatalf("CountAllByCatalog: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 (excluded image should not count), got %d", count)
	}
}

// TestCountAllByCatalog_EmptyCatalog verifies count=0 for nonexistent catalog.
func TestCountAllByCatalog_EmptyCatalog(t *testing.T) {
	repo, _ := setupRepo(t)

	count, err := repo.CountAllByCatalog("nonexistent")
	if err != nil {
		t.Fatalf("CountAllByCatalog: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 for empty catalog, got %d", count)
	}
}

// --- FindImageData tests ---------------------------------------------------

// TestFindImageData inserts an image with image_data blob and verifies
// FindImageData retrieves it correctly.
func TestFindImageData(t *testing.T) {
	repo, _ := setupRepo(t)

	imgData := []byte("fake-jpeg-binary-data-1234567890")
	rec := dummyImage("imgdata-cat")
	rec.SrcType = "http"
	rec.ImageData = imgData
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("UpsertActiveImage: %v", err)
	}

	// Find the inserted image ID.
	var id model.PrimaryKey
	repo.FindAll(func(img *model.Image) error {
		id = img.ID
		return nil
	})
	if id == 0 {
		t.Fatal("no image found after insert")
	}

	data, err := repo.FindImageData(id)
	if err != nil {
		t.Fatalf("FindImageData: %v", err)
	}
	if string(data) != string(imgData) {
		t.Errorf("image data mismatch: got %d bytes, want %d bytes", len(data), len(imgData))
	}
}

// TestFindImageData_NilForFileImages verifies that FindImageData returns
// nil/empty for file-based images that have no image_data stored.
func TestFindImageData_NilForFileImages(t *testing.T) {
	repo, _ := setupRepo(t)

	rec := dummyImage("file-cat")
	// File images do not store image_data.
	rec.ImageData = nil
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("UpsertActiveImage: %v", err)
	}

	var id model.PrimaryKey
	repo.FindAll(func(img *model.Image) error {
		id = img.ID
		return nil
	})
	if id == 0 {
		t.Fatal("no image found after insert")
	}

	data, err := repo.FindImageData(id)
	if err != nil {
		t.Fatalf("FindImageData: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty image_data for file image, got %d bytes", len(data))
	}
}

// TestFindImageData_NotFound verifies FindImageData returns an error for
// a nonexistent image ID.
func TestFindImageData_NotFound(t *testing.T) {
	repo, _ := setupRepo(t)

	_, err := repo.FindImageData(99999)
	if err == nil {
		t.Error("FindImageData should return error for nonexistent ID")
	}
}

// --- EvictOldestImages tests -----------------------------------------------

// TestEvictOldestImages inserts N images with staggered creation times and
// verifies that EvictOldestImages removes exactly the M oldest.
func TestEvictOldestImages(t *testing.T) {
	repo, db := setupRepo(t)

	const total = 5
	const evictN = 2

	var ids []model.PrimaryKey
	for i := range total {
		rec := dummyImage("evict-test")
		if err := repo.UpsertActiveImage(rec); err != nil {
			t.Fatalf("UpsertActiveImage: %v", err)
		}

		// Override created_at so ordering is deterministic.
		// Earlier indices = older timestamps.
		ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour)
		var inserted model.Image
		db.Last(&inserted)
		db.Model(&inserted).Update("created_at", ts)
		ids = append(ids, inserted.ID)
	}

	// Evict the 2 oldest.
	if err := repo.EvictOldestImages("evict-test", evictN); err != nil {
		t.Fatalf("EvictOldestImages: %v", err)
	}

	// Verify total count.
	count, err := repo.CountAllByCatalog("evict-test")
	if err != nil {
		t.Fatalf("CountAllByCatalog: %v", err)
	}
	if count != total-evictN {
		t.Errorf("expected %d remaining, got %d", total-evictN, count)
	}

	// Verify that the two oldest IDs are gone and the newer ones remain.
	for _, id := range ids[:evictN] {
		_, err := repo.FindById(id)
		if err == nil {
			t.Errorf("evicted image ID=%d should not exist", id)
		}
	}
	for _, id := range ids[evictN:] {
		_, err := repo.FindById(id)
		if err != nil {
			t.Errorf("surviving image ID=%d should still exist, got error: %v", id, err)
		}
	}
}

// TestEvictOldestImages_OnlyTargetCatalog verifies eviction is scoped to the
// given catalog key and does not affect other catalogs.
func TestEvictOldestImages_OnlyTargetCatalog(t *testing.T) {
	repo, _ := setupRepo(t)

	// Insert images in two catalogs.
	for range 3 {
		if err := repo.UpsertActiveImage(dummyImage("cat-a")); err != nil {
			t.Fatalf("insert cat-a: %v", err)
		}
	}
	for range 2 {
		if err := repo.UpsertActiveImage(dummyImage("cat-b")); err != nil {
			t.Fatalf("insert cat-b: %v", err)
		}
	}

	// Evict 2 from cat-a.
	if err := repo.EvictOldestImages("cat-a", 2); err != nil {
		t.Fatalf("EvictOldestImages: %v", err)
	}

	countA, _ := repo.CountAllByCatalog("cat-a")
	countB, _ := repo.CountAllByCatalog("cat-b")

	if countA != 1 {
		t.Errorf("cat-a: expected 1 remaining, got %d", countA)
	}
	if countB != 2 {
		t.Errorf("cat-b: expected 2 untouched, got %d", countB)
	}
}

// TestEvictOldestImages_ZeroCount verifies eviction with count=0 is a no-op.
func TestEvictOldestImages_ZeroCount(t *testing.T) {
	repo, _ := setupRepo(t)

	if err := repo.UpsertActiveImage(dummyImage("zero-evict")); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := repo.EvictOldestImages("zero-evict", 0); err != nil {
		t.Fatalf("EvictOldestImages(0): %v", err)
	}

	count, _ := repo.CountAllByCatalog("zero-evict")
	if count != 1 {
		t.Errorf("expected 1 image to survive, got %d", count)
	}
}

// TestEvictOldestImages_EmptyCatalog verifies eviction on an empty catalog
// does not error.
func TestEvictOldestImages_EmptyCatalog(t *testing.T) {
	repo, _ := setupRepo(t)

	if err := repo.EvictOldestImages("nonexistent", 5); err != nil {
		t.Errorf("EvictOldestImages on empty catalog should not error, got: %v", err)
	}
}

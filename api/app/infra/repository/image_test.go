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

// --- ReshuffleRandom tests -------------------------------------------------

// rndValues reads every rnd in ascending order, soft-deleted rows included.
func rndValues(t *testing.T, db *gorm.DB) []float64 {
	t.Helper()
	var vals []float64
	if err := db.Unscoped().Model(&model.Image{}).Order("rnd").Pluck("rnd", &vals).Error; err != nil {
		t.Fatalf("read rnd: %v", err)
	}
	return vals
}

// TestReshuffleRandom_SpacesValuesEvenly is the assertion the whole change
// exists for: every gap the same width, so FindByRandom reaches every row with
// the same probability.
//
// It also guards against a failure quieter than an error. The dialect-specific
// alternative to this implementation writes values that look entirely
// reasonable — inside the range, no nulls — while collapsing thousands of rows
// onto a handful of distinct numbers. Only measuring the gaps catches that.
func TestReshuffleRandom_SpacesValuesEvenly(t *testing.T) {
	repo, db := setupRepo(t)

	const n = 1200 // spans more than two batches
	for i := 0; i < n; i++ {
		if err := repo.UpsertActiveImage(dummyImage("cat")); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	if err := repo.ReshuffleRandom(nil); err != nil {
		t.Fatalf("ReshuffleRandom: %v", err)
	}

	vals := rndValues(t, db)
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

// TestReshuffleRandom_LeavesNoUnreachableRows states the symptom in the terms
// that made this worth fixing: with values drawn at random, part of the
// catalogue sits behind a gap too narrow to ever be picked, and stays there.
func TestReshuffleRandom_LeavesNoUnreachableRows(t *testing.T) {
	repo, db := setupRepo(t)

	const n = 2000
	for i := 0; i < n; i++ {
		if err := repo.UpsertActiveImage(dummyImage("cat")); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	// dummyImage assigns rnd at random, which is the state a scan leaves
	// behind, so this measures the problem before it measures the fix.
	starved := func() int {
		vals := rndValues(t, db)
		mean := 1.0 / float64(len(vals))
		count, prev := 0, 0.0
		for _, v := range vals {
			if v-prev < mean*0.1 {
				count++
			}
			prev = v
		}
		return count
	}

	if starved() == 0 {
		t.Skip("random assignment happened to leave no narrow gaps this run")
	}

	if err := repo.ReshuffleRandom(nil); err != nil {
		t.Fatalf("ReshuffleRandom: %v", err)
	}

	if after := starved(); after != 0 {
		t.Errorf("%d rows still sit behind a gap under a tenth of the mean", after)
	}
}

// TestReshuffleRandom_SurvivesACatalogueBeingLeftOut is the case an earlier
// version of this got wrong, badly enough to be worse than doing nothing.
//
// A scan registers one catalogue at a time, so ids arrive grouped by
// catalogue. Handing out the evenly spaced values in id order therefore gives
// each catalogue a contiguous band of the range. That reads as harmless until
// a display draws on some catalogues but not others: the missing catalogue
// leaves a hole, FindByRandom takes the first row at or past the number it
// drew, and every draw landing in the hole comes back with the same row. On
// the real catalogue one photograph was answering nearly a third of requests.
//
// So the check has to filter the way a display does — by catalogue, which is
// correlated with id — rather than by an evenly scattered sample, which is
// exactly what hid this before.
func TestReshuffleRandom_SurvivesACatalogueBeingLeftOut(t *testing.T) {
	repo, db := setupRepo(t)

	// Inserted catalogue by catalogue, as a scan does.
	const perCatalog = 500
	for _, key := range []string{"first", "middle", "last"} {
		for i := 0; i < perCatalog; i++ {
			if err := repo.UpsertActiveImage(dummyImage(key)); err != nil {
				t.Fatalf("insert into %s: %v", key, err)
			}
		}
	}

	if err := repo.ReshuffleRandom(nil); err != nil {
		t.Fatalf("ReshuffleRandom: %v", err)
	}

	// A display that skips the middle catalogue, as the 4-inch panel does.
	var vals []float64
	if err := db.Model(&model.Image{}).
		Where("catalog_key IN ?", []string{"first", "last"}).
		Order("rnd").Pluck("rnd", &vals).Error; err != nil {
		t.Fatalf("read rnd: %v", err)
	}
	if len(vals) != perCatalog*2 {
		t.Fatalf("expected %d rows, got %d", perCatalog*2, len(vals))
	}

	// A row's share of the draws is the distance back to the row before it.
	mean := 1.0 / float64(len(vals))
	widest, prev := 0.0, 0.0
	for _, v := range vals {
		if gap := v - prev; gap > widest {
			widest = gap
		}
		prev = v
	}
	if tail := 1.0 - vals[len(vals)-1]; tail > widest {
		widest = tail
	}

	// Ordering by id put one row at 31% on the real catalogue. Spread at
	// random the widest gap is a small multiple of the mean; this bound is
	// loose enough not to flake and tight enough to catch a band-shaped hole.
	if widest > mean*30 {
		t.Errorf("one row takes %.1f%% of the draws (%.0f times the mean) — "+
			"the excluded catalogue has left a hole in the range",
			widest*100, widest/mean)
	}
}

// TestReshuffleRandom_EmptyTable: a catalogue can legitimately hold nothing,
// and a scan that finds nothing should still finish.
func TestReshuffleRandom_EmptyTable(t *testing.T) {
	repo, _ := setupRepo(t)

	if err := repo.ReshuffleRandom(nil); err != nil {
		t.Errorf("ReshuffleRandom on an empty table should not error, got: %v", err)
	}
}

// TestReshuffleRandom_CoversSoftDeletedRows: soft-deleted rows keep their place
// in the ordering, so they consume a slot. That leaves the surviving gaps as
// whole multiples of one width — still bounded below, so nothing the display
// can reach becomes unreachable.
func TestReshuffleRandom_CoversSoftDeletedRows(t *testing.T) {
	repo, db := setupRepo(t)

	const n = 600
	for i := 0; i < n; i++ {
		if err := repo.UpsertActiveImage(dummyImage("cat")); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	var ids []model.PrimaryKey
	if err := db.Model(&model.Image{}).Order("id").Limit(100).Pluck("id", &ids).Error; err != nil {
		t.Fatalf("pluck ids: %v", err)
	}
	if err := repo.ToggleDeletedAt(ids); err != nil {
		t.Fatalf("ToggleDeletedAt: %v", err)
	}

	if err := repo.ReshuffleRandom(nil); err != nil {
		t.Fatalf("ReshuffleRandom: %v", err)
	}

	if got := len(rndValues(t, db)); got != n {
		t.Fatalf("soft-deleted rows should have been given values too: got %d, want %d", got, n)
	}

	var live []float64
	if err := db.Model(&model.Image{}).Order("rnd").Pluck("rnd", &live).Error; err != nil {
		t.Fatalf("read live rnd: %v", err)
	}
	unit := 1.0 / float64(n)
	prev := 0.0
	for i, v := range live {
		multiple := (v - prev) / unit
		if diff := multiple - float64(int(multiple+0.5)); diff > 1e-6 || diff < -1e-6 {
			t.Fatalf("live gap %d is %g units wide, expected a whole multiple", i, multiple)
		}
		prev = v
	}
}

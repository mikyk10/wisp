package usecase_test

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"
	"github.com/mikyk10/wisp/app/usecase"
)

// setupScanUseCaseWithConfig builds a Scan() test environment with an arbitrary ServiceConfig.
// In-memory SQLite creates a separate DB per connection, so SetMaxOpenConns(1) forces
// concurrent goroutines to share the same DB.
func setupScanUseCaseWithConfig(t *testing.T, svc *config.ServiceConfig) (usecase.CatalogUsecase, repository.ImageRepository) {
	t.Helper()
	conn, err := infra.NewSqliteConnection("", true)
	if err != nil {
		t.Fatalf("failed to create in-memory DB: %v", err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	conn.AutoMigrate(&model.Image{}) //nolint:errcheck

	repo := infraRepo.NewImageRepositoryImpl(conn)
	return usecase.NewCatalogUseCase(svc, repo), repo
}

// setupScanUseCase is a convenience helper for single-catalog scenarios.
func setupScanUseCase(t *testing.T, srcDir string) (usecase.CatalogUsecase, repository.ImageRepository) {
	t.Helper()
	return setupScanUseCaseWithConfig(t, &config.ServiceConfig{
		Catalog: map[string]*config.ImageProviderConfig{
			"cat1": {
				Key:    "cat1",
				Config: config.ImageFileProviderConfig{SrcPath: srcDir},
			},
		},
		Displays: map[string]*config.DisplayConfig{},
	})
}

// createTestJPEG generates a landscape JPEG for testing.
func createTestJPEG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 200, 100)) // landscape
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func setupCatalogUseCase(t *testing.T) usecase.CatalogUsecase {
	t.Helper()
	conn, err := infra.NewSqliteConnection("", true)
	if err != nil {
		t.Fatalf("failed to create in-memory DB: %v", err)
	}
	conn.AutoMigrate(&model.Image{}) //nolint:errcheck

	repo := infraRepo.NewImageRepositoryImpl(conn)

	svc := &config.ServiceConfig{
		Catalog: map[string]*config.ImageProviderConfig{
			"cat1": {
				Key:    "cat1",
				Config: config.ImageFileProviderConfig{SrcPath: "/tmp"},
			},
		},
		Displays: map[string]*config.DisplayConfig{
			"disp1": {
				Key:          "disp1",
				DisplayModel: "ws7in3e",
				Orientation:  config.DisplayOrientationLandscape,
				Catalog: []*config.AssociatedImageProviders{
					{
						ProviderConfig: &config.ImageProviderConfig{
							Key:    "cat1",
							Config: config.ImageFileProviderConfig{SrcPath: "/tmp"},
						},
					},
				},
				ColorReduction: config.ColorReduction{Type: config.ColorReductionTypeFloydSteinberg},
			},
		},
	}

	return usecase.NewCatalogUseCase(svc, repo)
}

// Pick() に存在しないディスプレイキーを渡した場合、error を返すこと。
// （以前は panic していた箇所のリグレッションテスト）
func TestPick_UnknownDisplayKey(t *testing.T) {
	uc := setupCatalogUseCase(t)

	_, _, _, err := uc.Pick("nonexistent")
	if err == nil {
		t.Fatal("Pick() expected error for unknown display key, got nil")
	}
}

// --- Scan() tests ---

// TestScan_IndexesNewImages: new images found during scan should be registered in the DB.
func TestScan_IndexesNewImages(t *testing.T) {
	dir := t.TempDir()
	createTestJPEG(t, filepath.Join(dir, "photo1.jpg"))
	createTestJPEG(t, filepath.Join(dir, "photo2.jpg"))

	uc, _ := setupScanUseCase(t, dir)

	if err := uc.Scan(0); err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	count := 0
	if err := uc.ListImages("cat1", func(*model.Image) error {
		count++
		return nil
	}); err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 indexed images, got %d", count)
	}
}

// TestScan_SkipsUnchangedFile: an unchanged file should not be reprocessed on the second scan.
// The Rnd field is only updated by UpsertActiveImage, so if it remains unchanged the file was skipped.
func TestScan_SkipsUnchangedFile(t *testing.T) {
	dir := t.TempDir()
	createTestJPEG(t, filepath.Join(dir, "photo.jpg"))

	uc, repo := setupScanUseCase(t, dir)

	if err := uc.Scan(0); err != nil {
		t.Fatalf("first Scan() error: %v", err)
	}

	var imgID model.PrimaryKey
	_ = uc.ListImages("cat1", func(img *model.Image) error {
		imgID = img.ID
		return nil
	})
	if imgID == 0 {
		t.Fatal("no image indexed after first scan")
	}

	img1, err := repo.FindById(imgID)
	if err != nil {
		t.Fatalf("FindById() error: %v", err)
	}
	rnd1 := img1.Rnd

	if err := uc.Scan(0); err != nil {
		t.Fatalf("second Scan() error: %v", err)
	}

	img2, err := repo.FindById(imgID)
	if err != nil {
		t.Fatalf("FindById() after second scan error: %v", err)
	}

	if img2.Rnd != rnd1 {
		t.Errorf("unchanged file was re-processed: rnd changed from %v to %v", rnd1, img2.Rnd)
	}
}

// TestScan_ReindexesModifiedFile: a file whose mtime changed should be re-indexed.
func TestScan_ReindexesModifiedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.jpg")
	createTestJPEG(t, path)

	uc, repo := setupScanUseCase(t, dir)

	if err := uc.Scan(0); err != nil {
		t.Fatalf("first Scan() error: %v", err)
	}

	var imgID model.PrimaryKey
	_ = uc.ListImages("cat1", func(img *model.Image) error {
		imgID = img.ID
		return nil
	})
	if imgID == 0 {
		t.Fatal("no image indexed after first scan")
	}

	img1, err := repo.FindById(imgID)
	if err != nil {
		t.Fatalf("FindById() error: %v", err)
	}
	rnd1 := img1.Rnd

	// Set 2 seconds in the future to account for Truncate(time.Second) granularity.
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("Chtimes() error: %v", err)
	}

	if err := uc.Scan(0); err != nil {
		t.Fatalf("second Scan() error: %v", err)
	}

	img2, err := repo.FindById(imgID)
	if err != nil {
		t.Fatalf("FindById() after second scan error: %v", err)
	}

	if img2.Rnd == rnd1 {
		t.Error("modified file was not re-processed: rnd unchanged after mtime change")
	}
}

// TestScan_SkipsNonexistentSourceDir: a nonexistent source directory should be skipped without error.
func TestScan_SkipsNonexistentSourceDir(t *testing.T) {
	uc, _ := setupScanUseCase(t, "/nonexistent/path/that/does/not/exist")

	if err := uc.Scan(0); err != nil {
		t.Fatalf("Scan() should not error for nonexistent dir, got: %v", err)
	}
}

// TestScan_Idempotent: scanning the same directory twice should not change the number of DB records.
func TestScan_Idempotent(t *testing.T) {
	dir := t.TempDir()
	createTestJPEG(t, filepath.Join(dir, "photo1.jpg"))
	createTestJPEG(t, filepath.Join(dir, "photo2.jpg"))

	uc, _ := setupScanUseCase(t, dir)

	for i := range 2 {
		if err := uc.Scan(0); err != nil {
			t.Fatalf("Scan() #%d error: %v", i+1, err)
		}
	}

	count := 0
	_ = uc.ListImages("cat1", func(*model.Image) error {
		count++
		return nil
	})

	if count != 2 {
		t.Errorf("expected 2 records after 2 scans, got %d", count)
	}
}

// TestScan_ExcludesFilesViaCriteria: files that do not match the Include.Path criteria should be
// registered with excluded=true and must not appear in ListImages.
// Covers the excludedFileCh → UpsertInactiveImage code path.
func TestScan_ExcludesFilesViaCriteria(t *testing.T) {
	dir := t.TempDir()
	createTestJPEG(t, filepath.Join(dir, "photo_keep.jpg"))
	createTestJPEG(t, filepath.Join(dir, "photo_skip.jpg"))

	uc, repo := setupScanUseCaseWithConfig(t, &config.ServiceConfig{
		Catalog: map[string]*config.ImageProviderConfig{
			"cat1": {
				Key: "cat1",
				Config: config.ImageFileProviderConfig{
					SrcPath: dir,
					Criteria: config.Criteria{
						// Only paths containing "keep" are valid.
						Include: config.FileCriteria{Path: []string{"keep"}},
					},
				},
			},
		},
		Displays: map[string]*config.DisplayConfig{},
	})

	if err := uc.Scan(0); err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	// ListImages must not return catalog-excluded images (excluded=true).
	var listed int
	if err := uc.ListImages("cat1", func(img *model.Image) error {
		listed++
		return nil
	}); err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}
	if listed != 1 {
		t.Errorf("expected 1 listed image (photo_keep.jpg), got %d", listed)
	}

	// Verify that excluded=true records exist via FindAll (all records).
	var active, excluded int
	repo.FindAll(func(img *model.Image) error {
		if img.Excluded {
			excluded++
		} else {
			active++
		}
		return nil
	})
	if active != 1 {
		t.Errorf("expected 1 active image, got %d", active)
	}
	if excluded != 1 {
		t.Errorf("expected 1 excluded image (photo_skip.jpg), got %d", excluded)
	}
}

// TestScan_MultipleCatalogs: multiple catalogs should be scanned independently.
// Verifies that the outer for loop iterates correctly.
func TestScan_MultipleCatalogs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	createTestJPEG(t, filepath.Join(dir1, "photo1.jpg"))
	createTestJPEG(t, filepath.Join(dir2, "photo2.jpg"))

	uc, _ := setupScanUseCaseWithConfig(t, &config.ServiceConfig{
		Catalog: map[string]*config.ImageProviderConfig{
			"cat1": {Key: "cat1", Config: config.ImageFileProviderConfig{SrcPath: dir1}},
			"cat2": {Key: "cat2", Config: config.ImageFileProviderConfig{SrcPath: dir2}},
		},
		Displays: map[string]*config.DisplayConfig{},
	})

	if err := uc.Scan(0); err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	count1, count2 := 0, 0
	_ = uc.ListImages("cat1", func(*model.Image) error { count1++; return nil })
	_ = uc.ListImages("cat2", func(*model.Image) error { count2++; return nil })

	if count1 != 1 {
		t.Errorf("cat1: expected 1 image, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("cat2: expected 1 image, got %d", count2)
	}
}

// TestScan_StoresThumbnail: a thumbnail (ThumbJPG) should be saved after scanning.
// Covers the thumbnail generation and encoding code path.
func TestScan_StoresThumbnail(t *testing.T) {
	dir := t.TempDir()
	createTestJPEG(t, filepath.Join(dir, "photo.jpg"))

	uc, repo := setupScanUseCase(t, dir)

	if err := uc.Scan(0); err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	var imgID model.PrimaryKey
	_ = uc.ListImages("cat1", func(img *model.Image) error {
		imgID = img.ID
		return nil
	})
	if imgID == 0 {
		t.Fatal("no image indexed after scan")
	}

	img, err := repo.FindById(imgID)
	if err != nil {
		t.Fatalf("FindById() error: %v", err)
	}
	if len(img.ThumbJPG) == 0 {
		t.Error("ThumbJPG should be non-empty after scan")
	}
}

// --- Pick() tests ---

// TestPick_KnownDisplayKey_ReturnsLoader: Pick() with a valid display key should return a loader, display, and sequencer.
// Even when the DB is empty, an error-message image loader is returned so no error occurs.
func TestPick_KnownDisplayKey_ReturnsLoader(t *testing.T) {
	uc := setupCatalogUseCase(t)

	loader, display, seq, err := uc.Pick("disp1")
	if err != nil {
		t.Fatalf("Pick() unexpected error: %v", err)
	}
	if loader == nil {
		t.Error("Pick() loader should not be nil")
	}
	if display == nil {
		t.Error("Pick() display should not be nil")
	}
	if seq == nil {
		t.Error("Pick() sequencer group should not be nil")
	}
}

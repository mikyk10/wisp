package usecase_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"
	"github.com/mikyk10/wisp/app/usecase"
)

// --- helpers ---------------------------------------------------------------

// makeJPEGBytes produces a minimal valid JPEG image in memory.
func makeJPEGBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	return makeJPEGBytesN(t, w, h, 0)
}

// makeJPEGBytesN produces a valid JPEG where the red channel varies by n,
// guaranteeing distinct SHA1 hashes for different n values.
func makeJPEGBytesN(t *testing.T, w, h int, n int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := uint8(100 + n%155)
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: r, G: 150, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buf.Bytes()
}

// setupFetchUseCase builds a CatalogUsecase wired to the given service config and in-memory DB.
func setupFetchUseCase(t *testing.T, svc *config.ServiceConfig) (usecase.CatalogUsecase, repository.ImageRepository) {
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
	if err := conn.AutoMigrate(&model.Image{}); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	repo := infraRepo.NewImageRepositoryImpl(conn)
	return usecase.NewCatalogUseCase(svc, repo), repo
}

// insertImage is a convenience helper that inserts a single Image record via the repository.
func insertImage(t *testing.T, repo repository.ImageRepository, rec *model.Image) {
	t.Helper()
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("UpsertActiveImage: %v", err)
	}
}

// randomHash generates a random 40-character hex string suitable for SrcHash.
func randomHash() string {
	b := make([]byte, 20)
	for i := range b {
		b[i] = byte(rand.IntN(256))
	}
	return fmt.Sprintf("%x", b)
}

// --- Fetch tests -----------------------------------------------------------

// TestFetch_PullGET_StoresImages starts a test HTTP server returning a JPEG,
// configures a background HTTP catalog pointing at it, calls Fetch, and verifies
// images are persisted in the database with correct SrcType and ImageData.
func TestFetch_PullGET_StoresImages(t *testing.T) {
	// Each request must return a unique image so SHA1 hashes differ and
	// the ON CONFLICT upsert creates separate records.
	var reqCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := reqCount.Add(1)
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(makeJPEGBytesN(t, 200, 100, int(n))) //nolint:errcheck
	}))
	defer srv.Close()

	svc := &config.ServiceConfig{
		Catalog: map[string]*config.ImageProviderConfig{
			"http-bg": {
				Key: "http-bg",
				Config: config.ImageHTTPProviderConfig{
					URL:    srv.URL,
					Method: "GET",
					Cache: config.HTTPCacheConfig{
						Type:  "background",
						Depth: 3,
					},
				},
			},
		},
		Displays: map[string]*config.DisplayConfig{},
	}

	uc, repo := setupFetchUseCase(t, svc)

	if err := uc.Fetch([]string{"http-bg"}, 1, 1, false); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	count, err := repo.CountAllByCatalog("http-bg")
	if err != nil {
		t.Fatalf("CountAllByCatalog() error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 images stored, got %d", count)
	}

	// Verify that stored images have correct SrcType and non-empty ImageData.
	repo.FindAll(func(img *model.Image) error {
		if img.SrcType != "http" {
			t.Errorf("expected SrcType=http, got %q", img.SrcType)
		}
		// ImageData is omitted from FindAll; verify via FindImageData.
		data, err := repo.FindImageData(img.ID)
		if err != nil {
			t.Errorf("FindImageData(%d) error: %v", img.ID, err)
		}
		if len(data) == 0 {
			t.Errorf("image %d: ImageData should not be empty", img.ID)
		}
		return nil
	})
}

// TestFetch_Eviction verifies that when the cache is at depth, Fetch evicts
// the oldest images and fetches new ones to replace them.
func TestFetch_Eviction(t *testing.T) {
	jpegData := makeJPEGBytes(t, 200, 100)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpegData) //nolint:errcheck
	}))
	defer srv.Close()

	depth := 3
	svc := &config.ServiceConfig{
		Catalog: map[string]*config.ImageProviderConfig{
			"evict-cat": {
				Key: "evict-cat",
				Config: config.ImageHTTPProviderConfig{
					URL:    srv.URL,
					Method: "GET",
					Cache: config.HTTPCacheConfig{
						Type:       "background",
						Depth:      depth,
						EvictCount: 1,
					},
				},
			},
		},
		Displays: map[string]*config.DisplayConfig{},
	}

	uc, repo := setupFetchUseCase(t, svc)

	// Pre-fill with depth images (oldest first).
	thumbData := makeJPEGBytes(t, 32, 32)
	for i := range depth {
		rec := &model.Image{
			CatalogKey:       "evict-cat",
			Rnd:              rand.Float64(),
			Src:              srv.URL,
			SrcHash:          randomHash(),
			SrcType:          "http",
			TakenAt:          sql.NullTime{Time: time.Now().UTC(), Valid: true},
			ImageOrientation: model.ImgCanonicalOrientationLandscape,
			ThumbJPG:         thumbData,
			ImageData:        jpegData,
			FileModifiedAt:   sql.NullTime{Time: time.Now().UTC(), Valid: true},
		}
		insertImage(t, repo, rec)
		// Stagger created_at so eviction ordering is deterministic.
		time.Sleep(10 * time.Millisecond)
		_ = i
	}

	// Collect IDs before fetch (the oldest should be evicted).
	var idsBefore []model.PrimaryKey
	repo.FindAll(func(img *model.Image) error {
		idsBefore = append(idsBefore, img.ID)
		return nil
	})

	if len(idsBefore) != depth {
		t.Fatalf("pre-condition: expected %d images, got %d", depth, len(idsBefore))
	}

	// Run Fetch — should evict 1 oldest and fetch 1 new.
	if err := uc.Fetch([]string{"evict-cat"}, 1, 1, false); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	countAfter, err := repo.CountAllByCatalog("evict-cat")
	if err != nil {
		t.Fatalf("CountAllByCatalog() error: %v", err)
	}
	// After evicting 1 and fetching 1, count should remain at depth.
	if countAfter != int64(depth) {
		t.Errorf("expected %d images after eviction+fetch, got %d", depth, countAfter)
	}

	// The oldest image (idsBefore[0]) should have been evicted.
	oldestID := idsBefore[0]
	_, findErr := repo.FindById(oldestID)
	if findErr == nil {
		t.Errorf("oldest image (ID=%d) should have been evicted but was still found", oldestID)
	}
}

// TestFetch_SkipsRealtimeCatalogs verifies that Fetch only processes
// background HTTP catalogs and ignores realtime ones.
func TestFetch_SkipsRealtimeCatalogs(t *testing.T) {
	jpegData := makeJPEGBytes(t, 200, 100)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpegData) //nolint:errcheck
	}))
	defer srv.Close()

	svc := &config.ServiceConfig{
		Catalog: map[string]*config.ImageProviderConfig{
			"rt-cat": {
				Key: "rt-cat",
				Config: config.ImageHTTPProviderConfig{
					URL:    srv.URL,
					Method: "GET",
					Cache: config.HTTPCacheConfig{
						Type:  "realtime",
						Depth: 5,
					},
				},
			},
		},
		Displays: map[string]*config.DisplayConfig{},
	}

	uc, repo := setupFetchUseCase(t, svc)

	if err := uc.Fetch(nil, 1, 1, false); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	count, err := repo.CountAllByCatalog("rt-cat")
	if err != nil {
		t.Fatalf("CountAllByCatalog() error: %v", err)
	}
	if count != 0 {
		t.Errorf("realtime catalog should not have fetched images, got count=%d", count)
	}
}

// TestFetch_SkipsFileCatalogs verifies that Fetch ignores file-based catalogs.
func TestFetch_SkipsFileCatalogs(t *testing.T) {
	svc := &config.ServiceConfig{
		Catalog: map[string]*config.ImageProviderConfig{
			"file-cat": {
				Key:    "file-cat",
				Config: config.ImageFileProviderConfig{SrcPath: "/tmp"},
			},
		},
		Displays: map[string]*config.DisplayConfig{},
	}

	uc, repo := setupFetchUseCase(t, svc)

	if err := uc.Fetch(nil, 1, 1, false); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	count, err := repo.CountAllByCatalog("file-cat")
	if err != nil {
		t.Fatalf("CountAllByCatalog() error: %v", err)
	}
	if count != 0 {
		t.Errorf("file catalog should not have been fetched, got count=%d", count)
	}
}

// TestFetch_PurgeOrphans_SkipsHTTPImages verifies that PurgeOrphans does NOT
// delete HTTP-sourced images (SrcType="http") even though their Src is a URL
// and not an existing file path.
func TestFetch_PurgeOrphans_SkipsHTTPImages(t *testing.T) {
	svc := &config.ServiceConfig{
		Catalog:  map[string]*config.ImageProviderConfig{},
		Displays: map[string]*config.DisplayConfig{},
	}

	uc, repo := setupFetchUseCase(t, svc)

	thumbData := makeJPEGBytes(t, 32, 32)

	// Insert a file-based image with a nonexistent path — this should be purged.
	fileRec := &model.Image{
		CatalogKey:       "cat1",
		Rnd:              rand.Float64(),
		Src:              "/nonexistent/path/to/photo.jpg",
		SrcHash:          randomHash(),
		SrcType:          "file",
		TakenAt:          sql.NullTime{Time: time.Now().UTC(), Valid: true},
		ImageOrientation: model.ImgCanonicalOrientationLandscape,
		ThumbJPG:         thumbData,
		FileModifiedAt:   sql.NullTime{Time: time.Now().UTC(), Valid: true},
	}
	insertImage(t, repo, fileRec)

	// Insert an HTTP image — this should survive purge.
	httpRec := &model.Image{
		CatalogKey:       "http-cat",
		Rnd:              rand.Float64(),
		Src:              "https://example.com/image.jpg",
		SrcHash:          randomHash(),
		SrcType:          "http",
		TakenAt:          sql.NullTime{Time: time.Now().UTC(), Valid: true},
		ImageOrientation: model.ImgCanonicalOrientationLandscape,
		ThumbJPG:         thumbData,
		ImageData:        makeJPEGBytes(t, 200, 100),
		FileModifiedAt:   sql.NullTime{Time: time.Now().UTC(), Valid: true},
	}
	insertImage(t, repo, httpRec)

	// Verify both exist before purge.
	var countBefore int
	repo.FindAll(func(_ *model.Image) error { countBefore++; return nil })
	if countBefore != 2 {
		t.Fatalf("pre-condition: expected 2 images, got %d", countBefore)
	}

	if err := uc.PurgeOrphans(); err != nil {
		t.Fatalf("PurgeOrphans() error: %v", err)
	}

	// After purge: file image should be removed, HTTP image should remain.
	var remaining []*model.Image
	repo.FindAll(func(img *model.Image) error {
		remaining = append(remaining, img)
		return nil
	})

	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining image after purge, got %d", len(remaining))
	}
	if remaining[0].SrcType != "http" {
		t.Errorf("surviving image should be SrcType=http, got %q", remaining[0].SrcType)
	}
	if remaining[0].CatalogKey != "http-cat" {
		t.Errorf("surviving image should be catalog=http-cat, got %q", remaining[0].CatalogKey)
	}
}

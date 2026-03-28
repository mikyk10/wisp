package catalog_test

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/catalog"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"
)

// TestHTTPProvider_Background_ReturnsDBLoader verifies that Resolve() on a
// background HTTP provider returns an imageDBLoader backed by the DB rather
// than an imageURLLoader that would fetch live.
func TestHTTPProvider_Background_ReturnsDBLoader(t *testing.T) {
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
	repo := infraRepo.NewImageRepositoryImpl(conn)

	// Seed one landscape image so FindByRandom succeeds.
	rec := &model.Image{
		CatalogKey:       "bg-http",
		Rnd:              rand.Float64(),
		Src:              "https://example.com/generated.jpg",
		SrcHash:          randomHash(),
		SrcType:          "http",
		TakenAt:          sql.NullTime{Time: time.Now().UTC(), Valid: true},
		ImageOrientation: model.ImgCanonicalOrientationLandscape,
		ThumbJPG:         []byte("thumb"),
		ImageData:        []byte("imagedata"),
		FileModifiedAt:   sql.NullTime{Time: time.Now().UTC(), Valid: true},
	}
	if err := repo.UpsertActiveImage(rec); err != nil {
		t.Fatalf("UpsertActiveImage: %v", err)
	}

	epd := epaper.NewDisplay("ws7in3e", model.ImgCanonicalOrientationLandscape)
	httpConf := config.ImageHTTPProviderConfig{
		URL:    "https://example.com/api/generate",
		Method: "GET",
		Cache: config.HTTPCacheConfig{
			Type:  "background",
			Depth: 10,
		},
	}

	provider := catalog.NewImageHttpProvider(time.Now(), epd, repo, "bg-http", httpConf)
	loader, err := provider.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	// The loader's GetSourcePath() should return the DB record's URL, not the API URL.
	if loader.GetSourcePath() != rec.Src {
		t.Errorf("expected source path %q (from DB record), got %q", rec.Src, loader.GetSourcePath())
	}
}

// TestHTTPProvider_Realtime_ReturnsURLLoader verifies that Resolve() on a
// realtime HTTP provider returns an imageURLLoader that fetches from the URL.
func TestHTTPProvider_Realtime_ReturnsURLLoader(t *testing.T) {
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
	repo := infraRepo.NewImageRepositoryImpl(conn)

	epd := epaper.NewDisplay("ws7in3e", model.ImgCanonicalOrientationLandscape)
	apiURL := "https://example.com/api/realtime"
	httpConf := config.ImageHTTPProviderConfig{
		URL:    apiURL,
		Method: "GET",
		Cache: config.HTTPCacheConfig{
			Type: "realtime",
		},
	}

	provider := catalog.NewImageHttpProvider(time.Now(), epd, repo, "rt-http", httpConf)
	loader, err := provider.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	// Realtime loader should have the config URL as its source path.
	if loader.GetSourcePath() != apiURL {
		t.Errorf("expected source path %q (config URL), got %q", apiURL, loader.GetSourcePath())
	}
}

// TestHTTPProvider_Background_NoDB_ReturnsError verifies that Resolve() on a
// background provider returns an error when no images are seeded in the DB.
func TestHTTPProvider_Background_NoDB_ReturnsError(t *testing.T) {
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
	repo := infraRepo.NewImageRepositoryImpl(conn)

	epd := epaper.NewDisplay("ws7in3e", model.ImgCanonicalOrientationLandscape)
	httpConf := config.ImageHTTPProviderConfig{
		URL: "https://example.com/api/generate",
		Cache: config.HTTPCacheConfig{
			Type:  "background",
			Depth: 5,
		},
	}

	provider := catalog.NewImageHttpProvider(time.Now(), epd, repo, "empty-cat", httpConf)
	_, err = provider.Resolve()
	if err == nil {
		t.Error("Resolve() should return error when no images in DB for background provider")
	}
}

// randomHash generates a 40-char hex string for SrcHash.
func randomHash() string {
	b := make([]byte, 20)
	for i := range b {
		b[i] = byte(rand.IntN(256))
	}
	return fmt.Sprintf("%x", b)
}

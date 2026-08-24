package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"

	"gorm.io/gorm"
)

// stubImageRepository answers only what the providers under test ask for. The
// rest of the interface is here to satisfy it and is never called.
type stubImageRepository struct {
	random    *model.Image
	randomErr error
	count     int64
	countErr  error
}

func (s *stubImageRepository) FindByRandom(model.ImageFilter) (*model.Image, error) {
	return s.random, s.randomErr
}

func (s *stubImageRepository) CountByCatalog(string, model.CanonicalOrientation) (int64, error) {
	return s.count, s.countErr
}

func (s *stubImageRepository) RemoveImage(model.PrimaryKey) error              { return nil }
func (s *stubImageRepository) ToggleDeletedAt([]model.PrimaryKey) error        { return nil }
func (s *stubImageRepository) FindById(model.PrimaryKey) (*model.Image, error) { return nil, nil }
func (s *stubImageRepository) FindAll(func(*model.Image) error)                {}
func (s *stubImageRepository) ListByCatalog(string, []string, func(*model.Image) error) error {
	return nil
}
func (s *stubImageRepository) CountAllByCatalog(string) (int64, error)          { return s.count, s.countErr }
func (s *stubImageRepository) FindByHash(string, string) (*model.Image, error)  { return nil, nil }
func (s *stubImageRepository) UpsertActiveImage(*model.Image) error             { return nil }
func (s *stubImageRepository) UpsertInactiveImage(string, string, string) error { return nil }
func (s *stubImageRepository) FindImageData(model.PrimaryKey) ([]byte, error)   { return nil, nil }
func (s *stubImageRepository) EvictOldestImages(string, int) error              { return nil }
func (s *stubImageRepository) ReshuffleRandom(func(done, total int)) error      { return nil }

func testDisplay() epaper.DisplayMetadata {
	return epaper.NewDisplay("ws7in3e", model.ImgCanonicalOrientationLandscape)
}

// writeExistingFile puts a file on disk so os.Stat succeeds. The indexed file
// provider only stats the path, so the contents do not have to decode.
func writeExistingFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(path, []byte("not really a jpeg"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// TestProvenance_ReportedByLoader walks every way a loader can be produced and
// checks what it says about itself.
//
// The error cases matter most. imageIndexedFileProvider.Resolve reports success
// on all four of its paths — an empty catalog, a failed query and a vanished
// file all come back as (loader, nil) exactly like a photo does — so the only
// thing separating a delivered photo from a delivered error card is what the
// loader reports here.
func TestProvenance_ReportedByLoader(t *testing.T) {
	existingPath := writeExistingFile(t)

	indexed := &model.Image{ID: 42, Src: existingPath}
	cached := &model.Image{ID: 7, Src: "https://example.com/cached.jpg"}

	tests := []struct {
		name     string
		locator  ImageLocator
		expected Provenance
	}{
		{
			name: "indexed photo carries its row",
			locator: &imageIndexedFileProvider{
				epd:        testDisplay(),
				repo:       &stubImageRepository{random: indexed},
				catalogKey: "family",
			},
			expected: Provenance{
				Kind:       model.DeliveryKindPhoto,
				ImageID:    42,
				Source:     existingPath,
				CatalogKey: "family",
			},
		},
		{
			name: "empty catalog is an error card, not a photo",
			locator: &imageIndexedFileProvider{
				epd:        testDisplay(),
				repo:       &stubImageRepository{randomErr: gorm.ErrRecordNotFound},
				catalogKey: "family",
			},
			expected: Provenance{
				Kind:       model.DeliveryKindError,
				Reason:     model.DeliveryReasonNoImages,
				CatalogKey: "family",
			},
		},
		{
			name: "failed query is an error card, not a photo",
			locator: &imageIndexedFileProvider{
				epd:        testDisplay(),
				repo:       &stubImageRepository{randomErr: errors.New("no such table: images")},
				catalogKey: "family",
			},
			expected: Provenance{
				Kind:       model.DeliveryKindError,
				Reason:     model.DeliveryReasonDBError,
				CatalogKey: "family",
			},
		},
		{
			name: "indexed file gone from disk is an error card, not a photo",
			locator: &imageIndexedFileProvider{
				epd: testDisplay(),
				repo: &stubImageRepository{
					random: &model.Image{ID: 99, Src: filepath.Join(t.TempDir(), "deleted.jpg")},
				},
				catalogKey: "family",
			},
			expected: Provenance{
				Kind:       model.DeliveryKindError,
				Reason:     model.DeliveryReasonFileMissing,
				CatalogKey: "family",
			},
		},
		{
			name: "directly named file that will not load",
			locator: &imageLocalFileProvider{
				epd:        testDisplay(),
				targetPath: filepath.Join(t.TempDir(), "absent.jpg"),
			},
			expected: Provenance{
				Kind:   model.DeliveryKindError,
				Reason: model.DeliveryReasonLoadFailed,
			},
		},
		{
			name: "cached HTTP picture carries its row",
			locator: &imageHttpProvider{
				epd:        testDisplay(),
				repo:       &stubImageRepository{random: cached},
				catalogKey: "wallpapers",
				config: config.ImageHTTPProviderConfig{
					URL:   "https://example.com/api/generate",
					Cache: config.HTTPCacheConfig{Type: "background", Depth: 10},
				},
			},
			expected: Provenance{
				Kind:       model.DeliveryKindHTTP,
				ImageID:    7,
				Source:     cached.Src,
				CatalogKey: "wallpapers",
			},
		},
		{
			name: "live HTTP fetch has no row behind it",
			locator: &imageHttpProvider{
				epd:        testDisplay(),
				repo:       &stubImageRepository{},
				catalogKey: "wallpapers",
				config: config.ImageHTTPProviderConfig{
					URL:   "https://example.com/api/realtime",
					Cache: config.HTTPCacheConfig{Type: "realtime"},
				},
			},
			expected: Provenance{
				Kind:       model.DeliveryKindHTTP,
				Source:     "https://example.com/api/realtime",
				CatalogKey: "wallpapers",
			},
		},
		{
			name:    "color bar declares itself despite reusing the file loader",
			locator: &imageColorbarProvider{epd: testDisplay()},
			expected: Provenance{
				Kind:   model.DeliveryKindColorbar,
				Source: colorbarSource,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader, err := tt.locator.Resolve()
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}
			assertProvenance(t, loader.Provenance(), tt.expected)
		})
	}
}

// TestProvenance_PickImageProvider covers the two ways picking a catalog can
// fail before any provider is reached.
func TestProvenance_PickImageProvider(t *testing.T) {
	tests := []struct {
		name      string
		repo      *stubImageRepository
		providers []*config.AssociatedImageProviders
		expected  Provenance
	}{
		{
			name:      "no catalog is active",
			repo:      &stubImageRepository{},
			providers: []*config.AssociatedImageProviders{makeProvider("nightly", "0 3 * * *")},
			expected: Provenance{
				Kind:   model.DeliveryKindError,
				Reason: model.DeliveryReasonNoCatalog,
			},
		},
		{
			name: "every catalog is empty",
			repo: &stubImageRepository{count: 0},
			providers: []*config.AssociatedImageProviders{
				{
					ProviderConfig: &config.ImageProviderConfig{
						Key:    "family",
						Config: config.ImageFileProviderConfig{},
					},
				},
			},
			expected: Provenance{
				Kind:   model.DeliveryKindError,
				Reason: model.DeliveryReasonNoImages,
			},
		},
	}

	// A time no cron in the table is due at, so the first case falls through to
	// the non-cron providers and finds none.
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pick := PickImageProvider(now, testDisplay(), tt.repo, tt.providers...)
			loader, err := pick.Locator.Resolve()
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}
			assertProvenance(t, loader.Provenance(), tt.expected)
		})
	}
}

// TestProvenance_UnknownProviderType checks the fall-through in
// newLocatorFromConfig, which is reached when a config carries a provider type
// nothing knows how to build.
func TestProvenance_UnknownProviderType(t *testing.T) {
	cfg := &config.ImageProviderConfig{
		Key:    "mystery",
		Config: config.ImageErrorMessageProviderConfig{Message: "unused"},
	}

	loader, err := newLocatorFromConfig(time.Now(), testDisplay(), &stubImageRepository{}, cfg).Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	assertProvenance(t, loader.Provenance(), Provenance{
		Kind:       model.DeliveryKindError,
		Reason:     model.DeliveryReasonNoProvider,
		CatalogKey: "mystery",
	})
}

// TestProvenance_HandlerErrorCard covers the card the handler falls back to. Its
// two callers are told apart by the error they pass.
func TestProvenance_HandlerErrorCard(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		reason model.DeliveryReason
	}{
		{
			name:   "display key is not configured",
			err:    &DisplayNotFoundError{Key: "aa:bb:cc:dd:ee:ff"},
			reason: model.DeliveryReasonUnknownDisplay,
		},
		{
			name:   "picture was chosen but would not load",
			err:    errors.New("image decode failed"),
			reason: model.DeliveryReasonLoadFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader, err := NewErrorMessageProviderFactory(testDisplay(), "Ooops", tt.err).Resolve()
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}
			assertProvenance(t, loader.Provenance(), Provenance{
				Kind:   model.DeliveryKindError,
				Reason: tt.reason,
			})
		})
	}
}

// TestProvenance_ColorbarIsNotAPhoto guards the one construction site where the
// kind could have been left at its zero value: the color bar reuses the local
// file loader, so nothing about the loader's type says it is not a photo.
func TestProvenance_ColorbarIsNotAPhoto(t *testing.T) {
	loader, err := NewColorbarProvider(testDisplay()).Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	if _, ok := loader.(*imageLocalFilePointer); !ok {
		t.Fatalf("expected the color bar to reuse *imageLocalFilePointer, got %T", loader)
	}
	if got := loader.Provenance().Kind; got != model.DeliveryKindColorbar {
		t.Errorf("expected kind %q, got %q", model.DeliveryKindColorbar, got)
	}
}

func assertProvenance(t *testing.T, got, want Provenance) {
	t.Helper()

	if got.Kind != want.Kind {
		t.Errorf("Kind: expected %q, got %q", want.Kind, got.Kind)
	}
	if got.Reason != want.Reason {
		t.Errorf("Reason: expected %q, got %q", want.Reason, got.Reason)
	}
	if got.ImageID != want.ImageID {
		t.Errorf("ImageID: expected %d, got %d", want.ImageID, got.ImageID)
	}
	if got.Source != want.Source {
		t.Errorf("Source: expected %q, got %q", want.Source, got.Source)
	}
	if got.CatalogKey != want.CatalogKey {
		t.Errorf("CatalogKey: expected %q, got %q", want.CatalogKey, got.CatalogKey)
	}
}

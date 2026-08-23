package handler_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"
	"github.com/mikyk10/wisp/app/interface/handler"
	"github.com/mikyk10/wisp/app/usecase"

	"gorm.io/gorm"
)

func TestListCatalogs(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/catalogs", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	svc := &config.ServiceConfig{
		Catalog: map[string]*config.ImageProviderConfig{
			"cat1": {Key: "cat1"},
			"cat2": {Key: "cat2"},
		},
	}

	h := handler.NewCatalogHandler(svc, nil)

	if assert.NoError(t, h.ListCatalogs(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Catalogs []string `json:"catalogs"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.ElementsMatch(t, []string{"cat1", "cat2"}, resp.Catalogs)
	}
}

func setupHandler() (*echo.Echo, handler.CatalogHandler, *gorm.DB) {
	svc := &config.ServiceConfig{
		Displays: map[string]*config.DisplayConfig{
			"disp": {Key: "disp", DisplayModel: "ws7in3f", Orientation: config.DisplayOrientationLandscape},
		},
	}
	return newHandler(svc, true)
}

// newHandler wires a handler over a fresh in-memory DB.
//
// historyTable says whether delivery_history exists. It does not on any
// installation that predates the feature and has never been migrated, which is
// the case the history must not break.
func newHandler(svc *config.ServiceConfig, historyTable bool) (*echo.Echo, handler.CatalogHandler, *gorm.DB) {
	conn, err := infra.NewSqliteConnection("", true)
	if err != nil {
		panic(err)
	}
	conn.AutoMigrate(&model.Image{}) //nolint:errcheck
	if historyTable {
		conn.AutoMigrate(&model.DeliveryHistory{}) //nolint:errcheck
	}

	gcfg := &config.GlobalConfig{
		DeliveryHistory: config.DeliveryHistoryConfig{Size: config.DefaultDeliveryHistorySize},
	}
	uc := usecase.NewCatalogUseCase(
		gcfg, svc,
		infraRepo.NewImageRepositoryImpl(conn),
		infraRepo.NewDeliveryHistoryRepositoryImpl(conn),
	)

	h := handler.NewCatalogHandler(svc, uc)
	return echo.New(), h, conn
}

func TestImgFound(t *testing.T) {
	e, h, db := setupHandler()

	db.Exec(`INSERT INTO images (id, catalog_key, rnd, src, src_hash, thumb_jpg, image_orientation, created_at, updated_at) VALUES (1, 'cat', 0, 'src', 'hash', 'jpgdata', 1, datetime(), datetime())`)

	req := httptest.NewRequest(http.MethodGet, "/catalog/cat/image/1.jpg", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/catalog/:catalogKey/image/:imgid")
	c.SetPathValues(echo.PathValues{
		{Name: "catalogKey", Value: "cat"},
		{Name: "imgid", Value: "1.jpg"},
	})

	if assert.NoError(t, h.ImgManagement(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "image/jpeg", rec.Header().Get(echo.HeaderContentType))
		assert.Equal(t, "jpgdata", rec.Body.String())
	}
}

func TestImgNotFound(t *testing.T) {
	e, h, _ := setupHandler()

	req := httptest.NewRequest(http.MethodGet, "/pf/disp/image/99.jpg", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/pf/:displayKey/image/:imgid")
	c.SetPathValues(echo.PathValues{
		{Name: "displayKey", Value: "disp"},
		{Name: "imgid", Value: "99.jpg"},
	})

	if assert.NoError(t, h.Img(c)) {
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "image/jpeg", rec.Header().Get(echo.HeaderContentType))
		assert.NotEmpty(t, rec.Body.Bytes())
	}
}

// TestRandomImg_UnknownDisplay: passing an unknown display key to RandomImg should return error image with default display.
func TestRandomImg_UnknownDisplay(t *testing.T) {
	e, h, _ := setupHandler()

	req := httptest.NewRequest(http.MethodGet, "/pf/nonexistent/image/random.jpg", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/pf/:displayKey/image/random.*")
	c.SetPathValues(echo.PathValues{
		{Name: "displayKey", Value: "nonexistent"},
	})

	if assert.NoError(t, h.RandomImg(c)) {
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "image/jpeg", rec.Header().Get(echo.HeaderContentType))
		assert.NotEmpty(t, rec.Body.Bytes())
	}
}

// TestRandomImg_ErrorImageDeclaresLength checks that the error image announces
// its length, the way the normal image already does.
//
// Without a Content-Length the response goes out chunked, and the firmware
// reads the length with HTTPClient::getSize(), which returns -1 for a chunked
// body. It treats that as "no content received", discards both the picture the
// server drew and the X-Sleep-Seconds it asked for, and falls back to its own
// error screen and a 24-hour sleep. The entire error-image contract rests on
// this one header.
func TestRandomImg_ErrorImageDeclaresLength(t *testing.T) {
	e, h, _ := setupHandler()

	// .bin is what a panel asks for, and the body is far too large for
	// net/http to work the length out by itself.
	req := httptest.NewRequest(http.MethodGet, "/pf/nonexistent/image/random.bin", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/pf/:displayKey/image/random.*")
	c.SetPathValues(echo.PathValues{
		{Name: "displayKey", Value: "nonexistent"},
	})

	if assert.NoError(t, h.RandomImg(c)) {
		assert.NotEmpty(t, rec.Body.Bytes())
		assert.Equal(t, strconv.Itoa(rec.Body.Len()), rec.Header().Get(echo.HeaderContentLength),
			"the error image must declare its length or the firmware throws it away")
		assert.NotEmpty(t, rec.Header().Get("X-Sleep-Seconds"),
			"the retry interval is only useful if the response survives")
	}
}

// --- delivery history ------------------------------------------------------

const (
	// deliveryCatalogKey is the one catalog the delivery-history tests give
	// their display.
	deliveryCatalogKey = "cat"

	// normalSleepSeconds is the display's own interval. It is deliberately
	// nothing like errorSleepSeconds so that a recorded value says on its own
	// which path the response took.
	normalSleepSeconds = 900

	// errorSleepSeconds mirrors the handler's unexported constant: the retry
	// interval an error raised in the handler goes out with.
	errorSleepSeconds = 3600
)

// deliveryServiceConfig builds a display with one file catalog, so that
// picking a picture goes through the indexed provider rather than the colour
// bar.
func deliveryServiceConfig() *config.ServiceConfig {
	return &config.ServiceConfig{
		Displays: map[string]*config.DisplayConfig{
			"disp": {
				Key:                  "disp",
				DisplayModel:         "ws7in3f",
				Orientation:          config.DisplayOrientationLandscape,
				SleepDurationSeconds: normalSleepSeconds,
				ColorReduction:       config.ColorReduction{Type: config.ColorReductionTypeSimple},
				Catalog: []*config.AssociatedImageProviders{
					{
						ProviderConfig: &config.ImageProviderConfig{
							Key:    deliveryCatalogKey,
							Config: config.ImageFileProviderConfig{SrcPath: "/nonexistent"},
						},
					},
				},
			},
		},
	}
}

// writeFile puts a file of the given contents in dir and returns its path.
func writeFile(t *testing.T, dir, name string, contents []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// writeTestJPEG writes a small landscape JPEG and returns its path.
func writeTestJPEG(t *testing.T, dir string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := jpeg.Encode(buf, image.NewRGBA(image.Rect(0, 0, 200, 100)), nil); err != nil {
		t.Fatalf("encoding test JPEG: %v", err)
	}
	return writeFile(t, dir, "photo.jpg", buf.Bytes())
}

// indexImage puts one landscape row in images, as a scan would.
func indexImage(t *testing.T, conn *gorm.DB, src string) {
	t.Helper()
	err := conn.Exec(
		`INSERT INTO images (catalog_key, rnd, src, src_hash, src_type, thumb_jpg, image_orientation, excluded, created_at, updated_at)
		 VALUES (?, 0.5, ?, ?, 'file', '', 1, false, datetime(), datetime())`,
		deliveryCatalogKey, src, src,
	).Error
	if err != nil {
		t.Fatalf("indexing %s: %v", src, err)
	}
}

// callRandomImg drives RandomImg for one display key, the way the router does.
func callRandomImg(t *testing.T, e *echo.Echo, h handler.CatalogHandler, displayKey string) *httptest.ResponseRecorder {
	t.Helper()
	return callRandomImgExt(t, e, h, displayKey, ".jpg")
}

// callRandomImgExt drives RandomImg for one display key and one requested
// format. The route is random.*, so the extension is the caller's to choose.
func callRandomImgExt(t *testing.T, e *echo.Echo, h handler.CatalogHandler, displayKey, ext string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/pf/"+displayKey+"/image/random"+ext, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/pf/:displayKey/image/random.*")
	c.SetPathValues(echo.PathValues{{Name: "displayKey", Value: displayKey}})
	if err := h.RandomImg(c); err != nil {
		t.Fatalf("RandomImg: %v", err)
	}
	return rec
}

// deliveries reads a display's history back, newest first.
func deliveries(t *testing.T, conn *gorm.DB, displayKey string) []*model.DeliveryHistory {
	t.Helper()
	rows := []*model.DeliveryHistory{}
	if err := conn.Where("display_key = ?", displayKey).Order("seq DESC").Find(&rows).Error; err != nil {
		t.Fatalf("reading delivery history: %v", err)
	}
	return rows
}

// TestRandomImg_RecordsPhotoDelivery: a picture that reaches a panel is written
// down as one, carrying the row it came from and the retry interval the
// response actually announced.
func TestRandomImg_RecordsPhotoDelivery(t *testing.T) {
	svc := deliveryServiceConfig()
	e, h, conn := newHandler(svc, true)
	indexImage(t, conn, writeTestJPEG(t, t.TempDir()))

	rec := callRandomImg(t, e, h, "disp")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, strconv.Itoa(normalSleepSeconds), rec.Header().Get("X-Sleep-Seconds"))

	rows := deliveries(t, conn, "disp")
	if assert.Len(t, rows, 1) {
		assert.Equal(t, model.DeliveryKindPhoto, rows[0].Kind)
		assert.Equal(t, model.DeliveryReasonNone, rows[0].Reason)
		assert.Equal(t, deliveryCatalogKey, rows[0].CatalogKey)
		assert.NotZero(t, rows[0].ImageID, "the photograph it came from is the point of the record")
		// The stored interval is the one the panel was actually told, not a
		// second guess at what it should have been.
		assert.Equal(t, normalSleepSeconds, rows[0].SleepSeconds)
		assert.Equal(t, rec.Header().Get("X-Sleep-Seconds"), strconv.Itoa(rows[0].SleepSeconds))
	}
}

// TestRandomImg_RecordsSwallowedProviderErrorAsError is the test this feature
// exists for.
//
// A provider that gives up hands back an error card and reports success, so the
// request takes the ordinary success exit: status 200, the display's normal
// sleep, an error card on the panel. Reading the kind from anything other than
// the loader records that as a photograph and hides it completely.
func TestRandomImg_RecordsSwallowedProviderErrorAsError(t *testing.T) {
	svc := deliveryServiceConfig()
	e, h, conn := newHandler(svc, true)
	// No indexed images: the provider gives up and draws a card instead.

	rec := callRandomImg(t, e, h, "disp")
	assert.Equal(t, http.StatusOK, rec.Code, "the provider reported success, so the handler does too")
	assert.Equal(t, strconv.Itoa(normalSleepSeconds), rec.Header().Get("X-Sleep-Seconds"))

	rows := deliveries(t, conn, "disp")
	if assert.Len(t, rows, 1) {
		assert.Equal(t, model.DeliveryKindError, rows[0].Kind,
			"a card that went out on the success path is still a card")
		assert.Equal(t, model.DeliveryReasonNoImages, rows[0].Reason)
		assert.Equal(t, normalSleepSeconds, rows[0].SleepSeconds,
			"this card never reached the handler's error branch, so it carries the display's own interval")
		assert.NotEqual(t, errorSleepSeconds, rows[0].SleepSeconds)
	}
}

// TestRandomImg_RecordsHandlerErrorWithErrorSleep: a picture that was chosen
// and then would not load is recorded as an error which keeps the provenance of
// the photograph that failed, and carries the handler's own retry interval.
func TestRandomImg_RecordsHandlerErrorWithErrorSleep(t *testing.T) {
	svc := deliveryServiceConfig()
	e, h, conn := newHandler(svc, true)
	// The file is there, so the provider hands back a loader; it is not an
	// image, so loading it fails in the handler.
	src := writeFile(t, t.TempDir(), "broken.jpg", []byte("not an image"))
	indexImage(t, conn, src)

	rec := callRandomImg(t, e, h, "disp")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, strconv.Itoa(errorSleepSeconds), rec.Header().Get("X-Sleep-Seconds"))

	rows := deliveries(t, conn, "disp")
	if assert.Len(t, rows, 1) {
		assert.Equal(t, model.DeliveryKindError, rows[0].Kind)
		assert.Equal(t, model.DeliveryReasonLoadFailed, rows[0].Reason)
		assert.Equal(t, src, rows[0].Source, "which photograph would not load is what an operator needs")
		assert.NotZero(t, rows[0].ImageID)
		assert.Equal(t, errorSleepSeconds, rows[0].SleepSeconds)
	}
}

// TestRandomImg_UnregisteredDisplayRecordsNothing: the device API is
// unauthenticated, so any MAC on the network can ask for a picture. Only
// displays named in service.yaml may leave rows behind.
func TestRandomImg_UnregisteredDisplayRecordsNothing(t *testing.T) {
	svc := deliveryServiceConfig()
	e, h, conn := newHandler(svc, true)

	rec := callRandomImg(t, e, h, "aa:bb:cc:dd:ee:ff")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotEmpty(t, rec.Body.Bytes(), "an unknown display is still shown why")

	var rows int64
	if err := conn.Model(&model.DeliveryHistory{}).Count(&rows).Error; err != nil {
		t.Fatalf("counting delivery history: %v", err)
	}
	assert.Zero(t, rows, "an unregistered key must not create rows anywhere")
}

// TestRandomImg_HistoryFailureStillServesImage: the history table is missing on
// every installation that predates it, since auto-migration is off by default.
// A picture that cannot be written down is still a picture that was delivered.
func TestRandomImg_HistoryFailureStillServesImage(t *testing.T) {
	svc := deliveryServiceConfig()
	e, h, conn := newHandler(svc, false)
	indexImage(t, conn, writeTestJPEG(t, t.TempDir()))

	rec := callRandomImg(t, e, h, "disp")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Body.Bytes())
	assert.Equal(t, strconv.Itoa(normalSleepSeconds), rec.Header().Get("X-Sleep-Seconds"))
}

// writeUnencodableJPEG writes a photograph the pipeline cannot turn into a
// picture for the panel.
//
// Nothing is stubbed: the crop stage trims the source to the panel's aspect
// ratio before resizing it, and a single pixel leaves nothing to trim to — the
// shorter side rounds down to zero, the resize hands back an empty image, and
// png.Encode refuses an image with no size. That is a real encode failure
// raised by the real pipeline on a real file.
func writeUnencodableJPEG(t *testing.T, dir string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := jpeg.Encode(buf, image.NewRGBA(image.Rect(0, 0, 1, 1)), nil); err != nil {
		t.Fatalf("encoding test JPEG: %v", err)
	}
	return writeFile(t, dir, "tiny.jpg", buf.Bytes())
}

// TestRandomImg_EncodeFailureDeclaresLength: a picture that will not encode
// must still leave the panel with something it can use.
//
// This is TestRandomImg_ErrorImageDeclaresLength on the one route that test
// does not reach. The bare 500 this path used to send carried no card, no
// length and no retry interval, and a length the firmware cannot read is a
// response it throws away entirely — built-in error screen, twenty-four hours
// asleep, a dark frame for a day over one failed encode.
func TestRandomImg_EncodeFailureDeclaresLength(t *testing.T) {
	svc := deliveryServiceConfig()
	e, h, conn := newHandler(svc, true)
	indexImage(t, conn, writeUnencodableJPEG(t, t.TempDir()))

	rec := callRandomImgExt(t, e, h, "disp", ".png")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotEmpty(t, rec.Body.Bytes(), "the panel is shown why, not left to guess")
	assert.Equal(t, strconv.Itoa(rec.Body.Len()), rec.Header().Get(echo.HeaderContentLength),
		"the error image must declare its length or the firmware throws it away")
	assert.Equal(t, strconv.Itoa(errorSleepSeconds), rec.Header().Get("X-Sleep-Seconds"),
		"an hour is the retry interval; without it the panel sleeps for a day")
	assert.Equal(t, "image/png", rec.Header().Get(echo.HeaderContentType),
		"the card comes back in the format that was asked for")
}

// TestRandomImg_RecordsEncodeFailureWithProvenance: the card that goes out
// instead of the photograph is filed as one, and keeps the provenance of the
// photograph that would not encode — the history has to name it for anyone to
// find out which picture is at fault.
func TestRandomImg_RecordsEncodeFailureWithProvenance(t *testing.T) {
	svc := deliveryServiceConfig()
	e, h, conn := newHandler(svc, true)
	src := writeUnencodableJPEG(t, t.TempDir())
	indexImage(t, conn, src)

	rec := callRandomImgExt(t, e, h, "disp", ".png")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	rows := deliveries(t, conn, "disp")
	if assert.Len(t, rows, 1) {
		assert.Equal(t, model.DeliveryKindError, rows[0].Kind,
			"what the panel received is a card, whatever the loader was carrying")
		assert.Equal(t, model.DeliveryReasonEncodeFailed, rows[0].Reason,
			"the photograph loaded; it was the panel's format that defeated it")
		assert.Equal(t, src, rows[0].Source, "which photograph would not encode is what an operator needs")
		assert.Equal(t, deliveryCatalogKey, rows[0].CatalogKey)
		assert.NotZero(t, rows[0].ImageID)
		assert.Equal(t, errorSleepSeconds, rows[0].SleepSeconds,
			"this card came from the handler's error branch, so it carries the handler's interval")
		assert.NotEqual(t, normalSleepSeconds, rows[0].SleepSeconds)
	}
}

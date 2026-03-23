package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	conn, err := infra.NewSqliteConnection("", true)
	if err != nil {
		panic(err)
	}
	conn.AutoMigrate(&model.Image{}) //nolint:errcheck

	repo := infraRepo.NewImageRepositoryImpl(conn)

	svc := &config.ServiceConfig{
		Displays: map[string]*config.DisplayConfig{
			"disp": {Key: "disp", DisplayModel: "ws7in3f", Orientation: config.DisplayOrientationLandscape},
		},
	}

	uc := usecase.NewCatalogUseCase(svc, repo)

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

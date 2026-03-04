package handler

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"github.com/mikyk10/wisp/app/domain/catalog"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/encoder"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/interface/handler/response"
	"github.com/mikyk10/wisp/app/usecase"

	"github.com/bfontaine/jsons"
	"github.com/labstack/echo/v5"
)

type CatalogHandler interface {
	ListCatalogs(*echo.Context) error
	List(*echo.Context) error
	Img(*echo.Context) error
	ImgManagement(*echo.Context) error
	ToggleVisibility(*echo.Context) error
	RandomImg(*echo.Context) error
}

type catalogHandler struct {
	imguc usecase.CatalogUsecase
	svc   *config.ServiceConfig
}

func NewCatalogHandler(svc *config.ServiceConfig, catr usecase.CatalogUsecase) CatalogHandler {
	return &catalogHandler{
		imguc: catr,
		svc:   svc,
	}
}

func (uc *catalogHandler) ListCatalogs(c *echo.Context) error {
	catalogs := slices.Sorted(maps.Keys(uc.svc.Catalog))
	return c.JSON(http.StatusOK, map[string]any{"catalogs": catalogs})
}

func (uc *catalogHandler) Img(c *echo.Context) error {
	imgid := c.Param("imgid")
	ext := strings.ToLower(filepath.Ext(imgid))
	idstr := strings.TrimSuffix(imgid, ext)

	id, err := strconv.ParseUint(idstr, 10, 64)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	img, err := uc.imguc.FindLocalImageById("", model.PrimaryKey(id))
	if err != nil {
		display := uc.resolveDisplay(c)
		imsecgrp, _, _ := uc.imguc.GetSequencerGroupForDisplay(c.Param("displayKey"))
		return uc.renderErrorImage(c, ext, display, imsecgrp, "Image Not Found", http.StatusNotFound)
	}

	// If ThumbJPG is empty (e.g. catalog-excluded images), returning 0 bytes
	// would cause NS_BINDING_ABORTED in the browser, so return a dummy image instead.
	if len(img.ThumbJPG) == 0 {
		display := uc.resolveDisplay(c)
		imsecgrp, _, _ := uc.imguc.GetSequencerGroupForDisplay(c.Param("displayKey"))
		return uc.renderErrorImage(c, ext, display, imsecgrp, "Image Not Found", http.StatusNotFound)
	}

	rdr, mime, err := uc.img(ext, img)
	if err != nil {
		display := uc.resolveDisplay(c)
		imsecgrp, _, _ := uc.imguc.GetSequencerGroupForDisplay(c.Param("displayKey"))
		return uc.renderErrorImage(c, ext, display, imsecgrp, "Image Not Found", http.StatusNotFound)
	}

	return c.Stream(http.StatusOK, mime, rdr)
}

// ImgManagement serves images for Management API (/api/catalog/:catalogKey/image/:imgid).
// Returns error images without color reduction processing.
func (uc *catalogHandler) ImgManagement(c *echo.Context) error {
	imgid := c.Param("imgid")
	ext := strings.ToLower(filepath.Ext(imgid))
	idstr := strings.TrimSuffix(imgid, ext)

	id, err := strconv.ParseUint(idstr, 10, 64)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	img, err := uc.imguc.FindLocalImageById("", model.PrimaryKey(id))
	if err != nil {
		display := uc.resolveDisplay(c)
		return uc.renderErrorImage(c, ext, display, nil, "Image Not Found", http.StatusNotFound)
	}

	// If ThumbJPG is empty (e.g. catalog-excluded images), returning 0 bytes
	// would cause NS_BINDING_ABORTED in the browser, so return a dummy image instead.
	if len(img.ThumbJPG) == 0 {
		display := uc.resolveDisplay(c)
		return uc.renderErrorImage(c, ext, display, nil, "Image Not Found", http.StatusNotFound)
	}

	rdr, mime, err := uc.img(ext, img)
	if err != nil {
		display := uc.resolveDisplay(c)
		return uc.renderErrorImage(c, ext, display, nil, "Image Not Found", http.StatusNotFound)
	}

	return c.Stream(http.StatusOK, mime, rdr)
}

func (uc *catalogHandler) ToggleVisibility(c *echo.Context) error {

	type reqType struct {
		Ids []model.PrimaryKey `json:"ids"`
	}

	catalogKey := c.Param("catalogKey")

	req := &reqType{}
	if err := c.Bind(req); err != nil {
		return err
	}

	if err := uc.imguc.ToggleLocalImageFileVisibility(catalogKey, req.Ids); err != nil {
		return c.String(http.StatusInternalServerError, "Internal Error")
	}

	return c.NoContent(http.StatusOK)
}

// resolveDisplay resolves the display configuration from the displayKey parameter.
// If displayKey is not found or not provided, returns default display.
func (uc *catalogHandler) resolveDisplay(c *echo.Context) epaper.DisplayMetadata {
	displayKey := c.Param("displayKey")
	if conf, ok := uc.svc.Displays[displayKey]; ok {
		return epaper.NewDisplay(epaper.EPaperDisplayModel(conf.DisplayModel), model.CanonicalOrientation(conf.Orientation))
	}
	return epaper.NewDisplay(epaper.WS7in3EPaperF, model.ImgCanonicalOrientationLandscape)
}

// renderErrorImage generates and returns an error image.
// If imsecgrp is nil, it skips the Apply step and returns the raw error image.
func (uc *catalogHandler) renderErrorImage(
	c *echo.Context,
	ext string,
	display epaper.DisplayMetadata,
	imsecgrp improc.SequencerGroup,
	msg string,
	statusCode int,
) error {
	ctx := context.Background()
	ldr, _ := catalog.NewErrorMessageProviderFactory(display, msg).Resolve()
	img, meta, _ := ldr.Load()

	// Apply processing if imsecgrp is not nil
	if imsecgrp != nil {
		img, _ = imsecgrp.Apply(ctx, img, meta)
	}

	buf := &bytes.Buffer{}
	mime := ""
	switch ext {
	case ".png":
		mime = "image/png"
		_ = png.Encode(buf, img)
	default:
		mime = "image/jpeg"
		_ = jpeg.Encode(buf, img, nil)
	}

	return c.Stream(statusCode, mime, buf)
}

func (uc *catalogHandler) img(suffix string, cat *model.Image) (io.Reader, string, error) {
	mime := ""

	switch strings.ToLower(suffix) {
	case ".jpg":
		fallthrough
	case ".jpeg":
		mime = "image/jpeg"
		return bytes.NewReader(cat.ThumbJPG), mime, nil
	case ".png":
		mime = "image/png"
		img, _ := jpeg.Decode(bytes.NewReader(cat.ThumbJPG))
		buf := &bytes.Buffer{}

		if err := png.Encode(buf, img); err != nil {
			return nil, mime, err
		}

		return buf, mime, nil
	}
	return nil, "", fmt.Errorf("unsupported image format: %s", suffix)
}

// List retrieves the list of indexed images in the specified catalog.
func (uc *catalogHandler) List(c *echo.Context) error {
	const mime = "application/x-ndjson"

	catalogKey := c.Param("catalogKey")

	pr, pw := io.Pipe()

	fetcher := func() {
		jsonWriter := jsons.NewWriter(pw)
		var ferr error
		defer func() { pw.CloseWithError(ferr) }()

		ferr = uc.imguc.ListImages(catalogKey, func(rec *model.Image) error {
			// EXIF DateTime has no timezone info, so goexif interprets it as UTC.
			// Return it with a "Z" suffix as UTC time to prevent misinterpretation on the frontend.
			// Photos without EXIF data (Valid=false) return an empty string.
			timestamp := ""
			if rec.TakenAt.Valid {
				timestamp = rec.TakenAt.Time.UTC().Format("2006-01-02T15:04:05Z")
			}
			record := &response.Image{
				ID:        rec.ID,
				Enabled:   rec.DeletedAt.Time.IsZero(), // deleted_at IS NULL = enabled
				Timestamp: timestamp,
			}
			return jsonWriter.Add(record)
		})
	}
	go fetcher()
	return c.Stream(http.StatusOK, mime, pr)
}

func (uc *catalogHandler) RandomImg(c *echo.Context) error {
	displayKey := c.Param("displayKey")
	imgPtr, display, imsecgrp, err := uc.imguc.Pick(displayKey)
	if err != nil {
		// Resolve display for error image, even if Pick() failed
		display = uc.resolveDisplay(c)
		ext := filepath.Ext(strings.ToLower(c.Request().URL.Path))
		return uc.renderErrorImage(c, ext, display, nil, "Image Not Found", http.StatusOK)
	}

	sleepSecs := uc.svc.Displays[displayKey].SleepDurationSeconds

	// Pass through the sequencer to obtain the desired image.
	ctx := context.Background()
	img, meta, err := imgPtr.Load()
	if err != nil {
		ext := filepath.Ext(strings.ToLower(c.Request().URL.Path))
		return uc.renderErrorImage(c, ext, display, imsecgrp, "Image Not Found", http.StatusOK)
	}

	img, _ = imsecgrp.Apply(ctx, img, meta)

	//TODO: output destination
	c.Response().Header().Set("X-Sleep-Seconds", strconv.Itoa(sleepSecs))

	buf := &bytes.Buffer{}
	ext := filepath.Ext(strings.ToLower(c.Request().URL.Path))

	mime := ""
	switch ext {
	case ".jpg":
		fallthrough
	case ".jpeg":
		mime = "image/jpeg"
		err = jpeg.Encode(buf, img, nil)
	case ".png":
		mime = "image/png"
		err = png.Encode(buf, img)
	default:
		mime = "application/octet-stream"
		ecdr := encoder.NewWaveshareEPEncoder(display)
		buf, err = ecdr.Encode(img)
	}

	if err != nil {
		return c.String(http.StatusInternalServerError, "Internal Error")
	}

	c.Response().Header().Set(echo.HeaderContentLength, fmt.Sprintf("%d", buf.Len()))

	return c.Stream(http.StatusOK, mime, buf)
}

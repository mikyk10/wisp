package handler

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mikyk10/wisp/app/domain/catalog"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/display/wake"
	"github.com/mikyk10/wisp/app/domain/encoder"
	"github.com/mikyk10/wisp/app/domain/improc/color_reduction"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/interface/handler/response"
	"github.com/mikyk10/wisp/app/usecase"

	"github.com/bfontaine/jsons"
	"github.com/labstack/echo/v5"
	"gorm.io/gorm"
)

const (
	errMsgPhotoNotFound   = "Sorry, the photo you're looking for isn't here.\nCheck catalog settings or rescan for updates."
	errMsgDisplayNotFound = "Specified display-key is not found.\nAdd to 'displays' section in server config."

	// errorSleepSeconds is the retry interval sent to the device when an error image is returned.
	// Keeps retry frequency low without requiring config-level overrides per error type.
	errorSleepSeconds = 3600
)

type CatalogHandler interface {
	ListCatalogs(*echo.Context) error
	List(*echo.Context) error
	Img(*echo.Context) error
	ImgManagement(*echo.Context) error
	ListTags(*echo.Context) error
	ToggleVisibility(*echo.Context) error
	RandomImg(*echo.Context) error
}

type catalogHandler struct {
	imguc  usecase.CatalogUsecase
	svc    *config.ServiceConfig
}

func NewCatalogHandler(svc *config.ServiceConfig, catr usecase.CatalogUsecase) CatalogHandler {
	return &catalogHandler{
		imguc:  catr,
		svc:    svc,
	}
}

func (uc *catalogHandler) ListCatalogs(c *echo.Context) error {
	var catalogs []string
	for key, cfg := range uc.svc.Catalog {
		// Exclude realtime HTTP catalogs from SPA listing (they have no images in DB).
		if httpConf, ok := cfg.Config.(config.ImageHTTPProviderConfig); ok && !httpConf.IsBackground() {
			continue
		}
		catalogs = append(catalogs, key)
	}
	slices.Sort(catalogs)
	return c.JSON(http.StatusOK, map[string]any{"catalogs": catalogs})
}

func (uc *catalogHandler) Img(c *echo.Context) error {
	imgid := c.Param("imgid")
	ext := strings.ToLower(filepath.Ext(imgid))
	idstr := strings.TrimSuffix(imgid, ext)

	// Get display and sequencer group upfront for Device API
	displayKey := c.Param("displayKey")
	display := uc.resolveDisplay(c)

	id, err := strconv.ParseUint(idstr, 10, 64)
	if err != nil {
		return uc.renderErrorImage(c, ext, display, errMsgPhotoNotFound, http.StatusBadRequest, nil)
	}
	imsecgrp, _, displayErr := uc.imguc.GetSequencerGroupForDisplay(displayKey)
	if displayErr != nil {
		return uc.renderErrorImage(c, ext, display, errMsgDisplayNotFound, http.StatusNotFound, displayErr)
	}

	// Load original source image (not thumbnail) and apply the display's processing pipeline
	srcImg, meta, loadErr := uc.imguc.LoadSourceImageById(model.PrimaryKey(id))
	if loadErr != nil {
		return uc.renderErrorImage(c, ext, display, errMsgPhotoNotFound, http.StatusNotFound, loadErr)
	}

	ctx := context.Background()
	processedImg, _ := imsecgrp.Apply(ctx, srcImg, meta)

	buf, mime, err := uc.encodeImage(ext, processedImg, display)
	if err != nil {
		return uc.renderErrorImage(c, ext, display, errMsgPhotoNotFound, http.StatusInternalServerError, err)
	}

	return c.Stream(http.StatusOK, mime, buf)
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

	img, imgErr := uc.imguc.FindLocalImageById("", model.PrimaryKey(id))
	if imgErr != nil {
		display := uc.resolveDisplay(c)
		// Record not found is a 404, not a DB error
		if imgErr == gorm.ErrRecordNotFound {
			imgErr = nil
		}
		return uc.renderErrorImage(c, ext, display,errMsgPhotoNotFound, http.StatusNotFound, imgErr)
	}

	// If ThumbJPG is empty (e.g. catalog-excluded images), returning 0 bytes
	// would cause NS_BINDING_ABORTED in the browser, so return a dummy image instead.
	if len(img.ThumbJPG) == 0 {
		display := uc.resolveDisplay(c)
		return uc.renderErrorImage(c, ext, display,errMsgPhotoNotFound, http.StatusNotFound, nil)
	}

	rdr, mime, err := uc.img(ext, img)
	if err != nil {
		display := uc.resolveDisplay(c)
		return uc.renderErrorImage(c, ext, display,errMsgPhotoNotFound, http.StatusNotFound, err)
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

// encodeImage encodes img into the format indicated by ext.
// Returns the encoded buffer, MIME type, and any encoding error.
func (uc *catalogHandler) encodeImage(ext string, img image.Image, display epaper.DisplayMetadata) (*bytes.Buffer, string, error) {
	buf := &bytes.Buffer{}
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return buf, "image/jpeg", jpeg.Encode(buf, img, nil)
	case ".png":
		return buf, "image/png", png.Encode(buf, img)
	default:
		ecdr := encoder.NewWaveshareEPEncoder(display)
		b, err := ecdr.Encode(img)
		return b, "application/octet-stream", err
	}
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
func (uc *catalogHandler) renderErrorImage(
	c *echo.Context,
	ext string,
	display epaper.DisplayMetadata,
	msg string,
	statusCode int,
	err error,
) error {
	ctx := context.Background()
	ldr, _ := catalog.NewErrorMessageProviderFactory(display, msg, err).Resolve()
	img, meta, _ := ldr.Load()

	// Apply color reduction with simple algorithm for error images
	// (ignore the display's configured algorithm)
	simpleColorReduction := color_reduction.NewImageColorReduction(display, config.ColorReduction{
		Type: config.ColorReductionTypeSimple,
	})
	img, _ = simpleColorReduction.Apply(ctx, img, meta)

	buf, mime, encErr := uc.encodeImage(ext, img, display)
	if encErr != nil {
		return c.String(http.StatusInternalServerError, "Internal Error")
	}

	c.Response().Header().Set("X-Sleep-Seconds", strconv.Itoa(errorSleepSeconds))

	// Without this the response is chunked, and the firmware reads the length
	// with HTTPClient::getSize(), which answers -1 for a chunked body. It takes
	// that for an empty response and throws away both this picture and the
	// retry interval above.
	c.Response().Header().Set(echo.HeaderContentLength, fmt.Sprintf("%d", buf.Len()))

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
	tags := parseTagFilter(c.QueryParam("tags"))

	pr, pw := io.Pipe()

	fetcher := func() {
		jsonWriter := jsons.NewWriter(pw)
		var ferr error
		defer func() { pw.CloseWithError(ferr) }()

		ferr = uc.imguc.ListImages(catalogKey, tags, func(rec *model.Image, tagNames []string) error {
			// EXIF DateTime has no timezone info, so goexif interprets it as UTC.
			// Return it with a "Z" suffix as UTC time to prevent misinterpretation on the frontend.
			// Photos without EXIF data (Valid=false) return an empty string.
			timestamp := ""
			if rec.TakenAt.Valid {
				timestamp = rec.TakenAt.Time.UTC().Format("2006-01-02T15:04:05Z")
			}
			if tagNames == nil {
				tagNames = []string{}
			}
			record := &response.Image{
				ID:        rec.ID,
				Enabled:   rec.DeletedAt.Time.IsZero(), // deleted_at IS NULL = enabled
				Timestamp: timestamp,
				Tags:      tagNames,
			}
			return jsonWriter.Add(record)
		})
	}
	go fetcher()
	return c.Stream(http.StatusOK, mime, pr)
}

// ListTags serves the tags available in a catalogue, most used first.
//
// The counts come with them because the list is long enough to need ordering
// by something, and "how many photos would this leave me" is the question a
// reader is actually asking of each entry.
func (uc *catalogHandler) ListTags(c *echo.Context) error {
	tags, err := uc.imguc.CatalogTags(c.Param("catalogKey"))
	if err != nil {
		return c.String(http.StatusInternalServerError, "Internal Error")
	}
	if tags == nil {
		tags = []model.TagUsage{}
	}
	return c.JSON(http.StatusOK, map[string]any{"tags": tags})
}

// parseTagFilter reads the comma-separated `tags` query parameter.
//
// Blanks are dropped rather than passed on: a trailing comma, or the empty
// string a client sends when it has just cleared the last filter, would
// otherwise become a tag nothing carries and empty the grid.
func parseTagFilter(raw string) []string {
	if raw == "" {
		return nil
	}
	tags := []string{}
	for _, part := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(part); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func (uc *catalogHandler) RandomImg(c *echo.Context) error {
	displayKey := c.Param("displayKey")
	imgPtr, display, imsecgrp, pickErr := uc.imguc.Pick(displayKey)
	if pickErr != nil {
		return uc.renderPickFailure(c, displayKey, pickErr)
	}

	// How long to stay away. With a wake schedule this is the distance to the
	// next moment the panel is wanted, so panels land on the same minute every
	// day instead of drifting by however long each sleep happened to be.
	displayConf := uc.svc.Displays[displayKey]
	sleepSecs := wake.Plan{
		Schedule: displayConf.WakeSchedule,
		Fallback: displayConf.SleepDurationSeconds,
	}.SleepSeconds(time.Now())

	// Pass through the sequencer to obtain the desired image.
	ctx := context.Background()
	img, meta, err := imgPtr.Load()
	if err != nil {
		return uc.renderChosenImageFailure(c, displayKey, display, imgPtr, model.DeliveryReasonLoadFailed, err)
	}

	img, _ = imsecgrp.Apply(ctx, img, meta)

	//TODO: output destination
	c.Response().Header().Set("X-Sleep-Seconds", strconv.Itoa(sleepSecs))

	ext := filepath.Ext(strings.ToLower(c.Request().URL.Path))

	buf, mime, err := uc.encodeImage(ext, img, display)
	if err != nil {
		// The picture loaded and then would not go down the wire in the form
		// this panel asked for. A bare 500 carries no card, no length and no
		// retry interval, which is the one response the firmware cannot use: it
		// reads -1 for the length, takes that for an empty body and falls back
		// to its own screen and a day of sleep. The same card every other
		// failure sends keeps the panel on its hourly retry instead.
		//
		// The reason names the stage that failed rather than the outcome: this
		// photograph read back from disk perfectly well, and only the panel's
		// own format defeated it. An operator chasing a picture that never
		// arrives needs that apart from load_failed, because the two point at
		// different things to go and look at.
		return uc.renderChosenImageFailure(c, displayKey, display, imgPtr, model.DeliveryReasonEncodeFailed, err)
	}

	c.Response().Header().Set(echo.HeaderContentLength, fmt.Sprintf("%d", buf.Len()))

	if err := c.Stream(http.StatusOK, mime, buf); err != nil {
		return err
	}

	// Recorded after the body, not before: a client that goes away part-way
	// through was not delivered to. The kind is whatever the loader says it is
	// — a provider that gives up hands back an error card and still arrives at
	// this exit, and calling that a photograph because the request succeeded
	// would hide the very deliveries worth looking at.
	sleepSent, _ := sentSleepSeconds(c)
	uc.imguc.RecordDelivery(displayKey, imgPtr.Provenance(), sleepSent)

	return nil
}

// renderPickFailure serves the error card for a request that never got as far
// as a loader, and files the delivery.
//
// Nothing here carries provenance — Pick() failed before a picture was chosen —
// so the record is built from what the error itself says.
func (uc *catalogHandler) renderPickFailure(c *echo.Context, displayKey string, pickErr error) error {
	// Resolve display for error image, even if Pick() failed
	display := uc.resolveDisplay(c)

	// Check if the error is a display-not-found error
	// TODO: verify type assertion works correctly; if not, move logic to provider_errmsg.go
	msg := errMsgPhotoNotFound
	statusCode := http.StatusInternalServerError
	reason := model.DeliveryReasonNoProvider
	if _, ok := pickErr.(*catalog.DisplayNotFoundError); ok {
		msg = errMsgDisplayNotFound
		statusCode = http.StatusNotFound
		reason = model.DeliveryReasonUnknownDisplay
	}

	prov := catalog.Provenance{Kind: model.DeliveryKindError, Reason: reason}
	return uc.renderDeviceErrorImage(c, displayKey, display, msg, statusCode, pickErr, prov)
}

// renderChosenImageFailure serves the error card for a photograph that was
// picked and then could not be handed over — it would not load, or it would not
// encode — and files the delivery.
//
// The loader keeps its provenance: which photograph failed is exactly what an
// operator needs to see. Only the kind changes, because what the panel receives
// is a card and not that photograph.
func (uc *catalogHandler) renderChosenImageFailure(
	c *echo.Context,
	displayKey string,
	display epaper.DisplayMetadata,
	imgPtr catalog.ImageLoader,
	reason model.DeliveryReason,
	err error,
) error {
	// Any retry interval already on the response belonged to the picture that
	// is no longer going out. renderErrorImage sets its own immediately before
	// it writes the card, and an interval left over from the abandoned response
	// would otherwise be read back as proof that a card was written.
	c.Response().Header().Del("X-Sleep-Seconds")

	prov := imgPtr.Provenance()
	prov.Kind = model.DeliveryKindError
	prov.Reason = reason

	return uc.renderDeviceErrorImage(c, displayKey, display, errMsgPhotoNotFound, http.StatusInternalServerError, err, prov)
}

// renderDeviceErrorImage sends an error card to a panel and files it, but only
// once the card has actually been written.
//
// The recording sits here rather than in renderErrorImage because that helper
// is shared with Img and ImgManagement, the two browser-facing endpoints: every
// missing thumbnail in the SPA would otherwise be filed as a picture handed to
// a panel.
func (uc *catalogHandler) renderDeviceErrorImage(
	c *echo.Context,
	displayKey string,
	display epaper.DisplayMetadata,
	msg string,
	statusCode int,
	err error,
	prov catalog.Provenance,
) error {
	ext := filepath.Ext(strings.ToLower(c.Request().URL.Path))
	if renderErr := uc.renderErrorImage(c, ext, display, msg, statusCode, err); renderErr != nil {
		return renderErr
	}

	// renderErrorImage sets the retry interval immediately before it writes the
	// card, and answers nil whether or not it got that far: with no header the
	// card failed to encode and what went down the wire was a plain 500. That
	// is not a picture, so it is not a delivery.
	sleepSent, sent := sentSleepSeconds(c)
	if !sent {
		return nil
	}

	uc.imguc.RecordDelivery(displayKey, prov, sleepSent)

	return nil
}

// sentSleepSeconds reads back the retry interval this response actually
// carries, rather than working it out a second time, and says whether one was
// set at all.
//
// The values are not interchangeable. A provider that gave up never reaches the
// handler's error branch, so its card goes out with the display's normal sleep,
// while an error raised here carries errorSleepSeconds — and that difference is
// half of what makes the recorded number worth having.
func sentSleepSeconds(c *echo.Context) (int, bool) {
	secs, err := strconv.Atoi(c.Response().Header().Get("X-Sleep-Seconds"))
	if err != nil {
		return 0, false
	}
	return secs, true
}

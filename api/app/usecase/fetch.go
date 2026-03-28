package usecase

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec
	"database/sql"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mikyk10/wisp/app/domain/catalog"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/sunshineplan/imgconv"
)

func (cu *catalogUseCase) Fetch(catalogKeys []string, workers int, maxRetries int, verbose bool) error {
	if verbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sch := make(chan os.Signal, 1)
	signal.Notify(sch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sch)
	go func() {
		select {
		case <-sch:
			slog.Info("fetch: received signal, cancelling")
			cancel()
		case <-ctx.Done():
		}
	}()

	if maxRetries <= 0 {
		maxRetries = 3
	}

	// Collect background HTTP catalogs.
	var targets []*config.ImageProviderConfig
	for _, cat := range cu.serviceConfig.Catalog {
		httpConf, ok := cat.Config.(config.ImageHTTPProviderConfig)
		if !ok || !httpConf.IsBackground() {
			continue
		}
		if len(catalogKeys) > 0 && !slices.Contains(catalogKeys, cat.Key) {
			continue
		}
		targets = append(targets, cat)
	}

	sort.Slice(targets, func(i, j int) bool {
		return strings.Compare(targets[i].Key, targets[j].Key) < 0
	})

	if len(targets) == 0 {
		slog.Info("fetch: no background HTTP catalogs found")
		return nil
	}

	concurrency := scanConcurrency(workers)

	for _, cat := range targets {
		if ctx.Err() != nil {
			break
		}
		httpConf := cat.Config.(config.ImageHTTPProviderConfig) //nolint:forcetypeassert
		if err := cu.fetchCatalog(ctx, cat.Key, httpConf, concurrency, maxRetries); err != nil {
			slog.Error("fetch: catalog failed", "catalog", cat.Key, "err", err)
		}
	}

	return nil
}

func (cu *catalogUseCase) fetchCatalog(ctx context.Context, catalogKey string, conf config.ImageHTTPProviderConfig, concurrency int, maxRetries int) error {
	method := strings.ToUpper(conf.Method)
	if method == "" {
		method = http.MethodGet
	}
	slog.Info("fetch: started", "catalog", catalogKey, "url", conf.URL, "method", method, "depth", conf.Cache.Depth)

	if conf.ImageSource != nil && conf.ImageSource.Mode == "fixed" && conf.ImageSource.ImageID == 0 {
		return fmt.Errorf("image_source.mode=fixed requires image_id")
	}

	// Evict oldest images if over depth.
	count, err := cu.imgr.CountAllByCatalog(catalogKey)
	if err != nil {
		return fmt.Errorf("count failed: %w", err)
	}

	evictCount := conf.Cache.EvictCount
	if evictCount <= 0 {
		evictCount = max(conf.Cache.Depth/5, 1)
	}
	if int(count) >= conf.Cache.Depth {
		toEvict := min(evictCount, int(count))
		slog.Info("fetch: evicting", "catalog", catalogKey, "count", toEvict)
		if err := cu.imgr.EvictOldestImages(catalogKey, toEvict); err != nil {
			return fmt.Errorf("eviction failed: %w", err)
		}
		count -= int64(toEvict)
	}

	fetchCount := conf.Cache.Depth - int(count)
	if fetchCount <= 0 {
		slog.Info("fetch: cache full", "catalog", catalogKey)
		return nil
	}

	timeout := time.Duration(conf.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, concurrency)

	for i := range fetchCount {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer func() { wg.Done(); <-sem }()
			cu.fetchOne(ctx, catalogKey, conf, method, timeout, maxRetries, idx+1, fetchCount)
		}(i)
	}

	wg.Wait()
	slog.Info("fetch: completed", "catalog", catalogKey)
	return ctx.Err()
}

func (cu *catalogUseCase) fetchOne(ctx context.Context, catalogKey string, conf config.ImageHTTPProviderConfig, method string, timeout time.Duration, maxRetries int, idx int, total int) {
	var data []byte
	var err error

	for attempt := range maxRetries {
		if ctx.Err() != nil {
			return
		}

		data, err = cu.httpFetch(ctx, conf, method, timeout)
		if err == nil {
			// Validate: must be decodable as image.
			if _, _, decErr := image.Decode(bytes.NewReader(data)); decErr != nil {
				err = fmt.Errorf("response is not a valid image: %w", decErr)
			} else {
				break
			}
		}

		if attempt < maxRetries-1 {
			backoff := time.Duration(1<<attempt) * time.Second
			slog.Warn("fetch: retrying", "catalog", catalogKey, "attempt", attempt+1, "backoff", backoff, "err", err)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
		}
	}

	if err != nil {
		slog.Error("fetch: failed after retries", "catalog", catalogKey, "idx", idx, "err", err)
		return
	}

	// Decode for thumbnail generation.
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		slog.Error("fetch: decode failed", "catalog", catalogKey, "idx", idx, "err", err)
		return
	}

	// Generate thumbnail.
	thumb := imgconv.Resize(img, &imgconv.ResizeOption{Width: 256})
	img = nil //nolint:ineffassign
	thumbBuf := &bytes.Buffer{}
	if err := jpeg.Encode(thumbBuf, thumb, &jpeg.Options{Quality: jpeg.DefaultQuality}); err != nil {
		slog.Error("fetch: thumbnail encode failed", "catalog", catalogKey, "idx", idx, "err", err)
		return
	}

	//nolint:gosec
	srcHash := fmt.Sprintf("%x", sha1.Sum(data))

	rec := &model.Image{
		CatalogKey:       catalogKey,
		Rnd:              rand.Float64(),
		Src:              conf.URL,
		SrcHash:          srcHash,
		SrcType:          "http",
		TakenAt:          sql.NullTime{Time: time.Now().UTC(), Valid: true},
		ImageOrientation: model.ImgCanonicalOrientationLandscape,
		ThumbJPG:         thumbBuf.Bytes(),
		ImageData:        data,
		FileModifiedAt:   sql.NullTime{Time: time.Now().UTC(), Valid: true},
	}

	if err := cu.imgr.UpsertActiveImage(rec); err != nil {
		slog.Error("fetch: upsert failed", "catalog", catalogKey, "idx", idx, "err", err)
		return
	}

	slog.Info("fetch: stored", "catalog", catalogKey, "progress", fmt.Sprintf("%d/%d", idx, total), "hash", srcHash[:12])
}

func (cu *catalogUseCase) httpFetch(ctx context.Context, conf config.ImageHTTPProviderConfig, method string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var body io.Reader
	var contentType string

	// Push-pull: load source image and set as request body.
	if method == http.MethodPost && conf.ImageSource != nil {
		srcBody, ct, err := cu.loadSourceImage(conf.ImageSource)
		if err != nil {
			return nil, fmt.Errorf("load source image: %w", err)
		}
		body = srcBody
		contentType = ct
	}

	req, err := http.NewRequestWithContext(ctx, method, conf.URL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range conf.Headers {
		req.Header.Set(k, os.ExpandEnv(v))
	}

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // URL from config
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("unexpected content-type: %s", ct)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return data, nil
}

// loadSourceImage loads a source image for push-pull POST requests.
// Returns the image bytes as a reader, the Content-Type, and any error.
// HEIF sources are converted to JPEG; other formats are sent as-is.
func (cu *catalogUseCase) loadSourceImage(src *config.HTTPImageSource) (io.Reader, string, error) {
	var rec *model.Image
	var err error

	mode := src.Mode
	if mode == "" {
		mode = "random"
	}

	switch mode {
	case "fixed":
		rec, err = cu.imgr.FindById(model.PrimaryKey(src.ImageID))
	case "random":
		ori := model.NewCanonicalOrientation(src.Orientation)
		rec, err = cu.imgr.FindByRandom(model.ImageFilter{
			CatalogKeys: src.Catalogs,
			Orientation: ori,
			Tags:        src.Tags,
		})
	default:
		return nil, "", fmt.Errorf("unknown image_source mode: %s", mode)
	}

	if err != nil {
		return nil, "", fmt.Errorf("find source image: %w", err)
	}

	if rec.SrcType == "http" {
		return nil, "", fmt.Errorf("image_source does not support http-sourced images (catalogs=%v, id=%d)", src.Catalogs, rec.ID)
	}

	// Load from filesystem (file catalogs only).
	img, _, loadErr := catalog.LoadImageFromPath(rec.Src)
	if loadErr != nil {
		return nil, "", fmt.Errorf("load source file: %w", loadErr)
	}

	// Determine output format based on source file extension.
	ext := strings.ToLower(filepath.Ext(rec.Src))
	buf := &bytes.Buffer{}

	switch ext {
	case ".heic", ".heif":
		// HEIF → JPEG conversion.
		if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, "", fmt.Errorf("heif to jpeg: %w", err)
		}
		return buf, "image/jpeg", nil
	case ".png":
		if err := png.Encode(buf, img); err != nil {
			return nil, "", fmt.Errorf("png encode: %w", err)
		}
		return buf, "image/png", nil
	default:
		// Default to JPEG (covers .jpg, .jpeg, and unknown formats).
		if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, "", fmt.Errorf("jpeg encode: %w", err)
		}
		return buf, "image/jpeg", nil
	}
}

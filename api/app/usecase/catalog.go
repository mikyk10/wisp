package usecase

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // sha1 is cryptographically weak, but is used here only as a hash to avoid collisions
	"database/sql"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"log/slog"
	"maps"
	"math/rand/v2"
	"os"
	"os/signal"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"github.com/mikyk10/wisp/app/domain/catalog"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/improc/blur"
	"github.com/mikyk10/wisp/app/domain/improc/brightness"
	"github.com/mikyk10/wisp/app/domain/improc/color_reduction"
	"github.com/mikyk10/wisp/app/domain/improc/contrast"
	"github.com/mikyk10/wisp/app/domain/improc/crop"
	"github.com/mikyk10/wisp/app/domain/improc/exif_rotation"
	"github.com/mikyk10/wisp/app/domain/improc/gamma"
	"github.com/mikyk10/wisp/app/domain/improc/hue"
	"github.com/mikyk10/wisp/app/domain/improc/rotation"
	"github.com/mikyk10/wisp/app/domain/improc/saturation"
	"github.com/mikyk10/wisp/app/domain/improc/selective_color"
	"github.com/mikyk10/wisp/app/domain/improc/timestamp"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"

	"github.com/sunshineplan/imgconv"
)

type ImageTaskCallback func(path string) error
type AlbumScanCallback func(callbacks ...ImageTaskCallback) error

type CatalogUsecase interface {
	// Scan enumerates all images from every ImageProvider under the catalog (File provider only).
	// workers controls the number of parallel image-processing goroutines (0 = use default).
	Scan(workers int) error
	// PurgeOrphans removes images that are unreachable from the index.
	PurgeOrphans() error

	// FindLocalImageById returns an image from the ImageLocalFileProvider by ID.
	FindLocalImageById(catalogKey string, id model.PrimaryKey) (*model.Image, error)

	// LoadSourceImageById loads the original source image and metadata for a given image ID.
	LoadSourceImageById(id model.PrimaryKey) (image.Image, *model.ImgMeta, error)

	// ListImages retrieves the list of indexed images under the catalog using a callback.
	ListImages(catalogKey string, cb func(*model.Image) error) error

	// ToggleLocalImageFileVisibility toggles the visibility state of images by ID.
	ToggleLocalImageFileVisibility(catalogKey string, ids []model.PrimaryKey) error

	// GetSequencerGroupForDisplay returns the image processing sequence group for a given display key.
	GetSequencerGroupForDisplay(displayKey string) (improc.SequencerGroup, epaper.DisplayMetadata, error)

	//
	Pick(displayKey string) (catalog.ImageLoader, epaper.DisplayMetadata, improc.SequencerGroup, error)
}

type catalogUseCase struct {
	serviceConfig *config.ServiceConfig
	imgr          repository.ImageRepository
}

func NewCatalogUseCase(serviceConfig *config.ServiceConfig, imgr repository.ImageRepository) CatalogUsecase {
	return &catalogUseCase{
		serviceConfig: serviceConfig,
		imgr:          imgr,
	}
}

func (cu *catalogUseCase) Scan(workers int) error {
	// Cancel gracefully on CTRL+C or SIGTERM.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sch := make(chan os.Signal, 1)
	signal.Notify(sch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sch)
	go func() {
		select {
		case <-sch:
			cancel()
		case <-ctx.Done():
		}
	}()

	catalogs := slices.Collect(maps.Values(cu.serviceConfig.Catalog))
	sort.Slice(catalogs, func(i, j int) bool {
		return strings.Compare(catalogs[i].Key, catalogs[j].Key) < 0
	})

	fileProviderConfigs := slices.DeleteFunc(catalogs, func(subj *config.ImageProviderConfig) bool {
		_, ok := subj.Config.(config.ImageFileProviderConfig)
		return !ok
	})

	for _, provConf := range fileProviderConfigs {
		cu.scanCatalog(ctx, provConf, workers)
	}

	return nil
}

// scanConcurrency resolves the number of parallel image-processing goroutines.
// Priority: flagWorkers (--workers flag) > WISP_SCAN_CONCURRENCY env var > min(GOMAXPROCS, 4).
//
// Each goroutine holds a full decoded image in memory until the DB write completes.
// Decoding a 20 MP HEIC photo uses ~300–500 MB in the pure-Go HEVC decoder; keeping
// concurrency low avoids OOM on memory-constrained hosts.
//
// Note: runtime.NumCPU() returns the host node's CPU count and ignores container CPU limits
// (e.g. Kubernetes limits.cpu). GOMAXPROCS(0) respects the GOMAXPROCS env var, which can be
// set from limits.cpu via Kubernetes resourceFieldRef or the automaxprocs library.
func scanConcurrency(flagWorkers int) int {
	if flagWorkers > 0 {
		return flagWorkers
	}
	if v := os.Getenv("WISP_SCAN_CONCURRENCY"); v != "" {
		if c, err := strconv.Atoi(v); err == nil && c > 0 {
			return c
		}
	}
	// Default: min(GOMAXPROCS, 4) — enough for throughput without excessive memory pressure.
	if n := runtime.GOMAXPROCS(0); n < 4 {
		return n
	}
	return 4
}

// scanCatalog performs a file scan for a single catalog.
// Concurrency is capped by scanConcurrency() to balance throughput against memory usage.
// Set GOMEMLIMIT (e.g. GOMEMLIMIT=6GiB) so Go's GC runs more aggressively under pressure.
func (cu *catalogUseCase) scanCatalog(ctx context.Context, provConf *config.ImageProviderConfig, workers int) {
	pconf := provConf.Config.(config.ImageFileProviderConfig) //nolint:forcetypeassert

	if _, err := os.Stat(pconf.SrcPath); err != nil {
		slog.Error("scan: source directory not found", "catalog", provConf.Key, "path", pconf.SrcPath)
		return
	}

	concurrency := scanConcurrency(workers)
	slog.Debug("scan: concurrency", "workers", concurrency)
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, concurrency)

	includedFileCh := make(chan catalog.ImageLoader, concurrency)
	excludedFileCh := make(chan catalog.ImageLoader, concurrency)

	prov := catalog.NewImageLocalFileProviderFactory(time.Now(), pconf)("")
	go prov.EnumerateImages(ctx, includedFileCh, excludedFileCh)

	slog.Info("scan: started", "catalog", provConf.Key, "path", pconf.SrcPath)

	const logInterval = 100
	var dispatched int

loop:
	for includedFileCh != nil || excludedFileCh != nil {
		select {
		case <-ctx.Done():
			break loop

		case info, ok := <-includedFileCh:
			if !ok {
				includedFileCh = nil
				continue
			}
			//nolint:gosec // sha1 is cryptographically weak, but is used here only as a hash to avoid collisions
			srcHash := sha1.Sum([]byte(info.GetSourcePath()))
			wg.Add(1)
			sem <- struct{}{}
			go func(h [20]byte, ldr catalog.ImageLoader) {
				defer func() { wg.Done(); <-sem }()
				cu.processIncludedFile(ctx, provConf.Key, h, ldr)
			}(srcHash, info)
			dispatched++
			if dispatched%logInterval == 0 {
				slog.Info("scan: progress", "catalog", provConf.Key, "dispatched", dispatched)
			}

		case info, ok := <-excludedFileCh:
			if !ok {
				excludedFileCh = nil
				continue
			}
			//nolint:gosec // sha1 is cryptographically weak, but is used here only as a hash to avoid collisions
			srcHash := sha1.Sum([]byte(info.GetSourcePath()))
			wg.Add(1)
			sem <- struct{}{}
			go func(h [20]byte, ldr catalog.ImageLoader) {
				defer func() { wg.Done(); <-sem }()
				cu.processExcludedFile(provConf.Key, h, ldr)
			}(srcHash, info)
			dispatched++
			if dispatched%logInterval == 0 {
				slog.Info("scan: progress", "catalog", provConf.Key, "dispatched", dispatched)
			}
		}
	}

	wg.Wait()
	slog.Info("scan completed", "catalog", provConf.Key, "total", dispatched)
}

// processIncludedFile processes a file received from includedFileCh and registers it in the DB.
// imseq is created per goroutine, so it is thread-safe.
func (cu *catalogUseCase) processIncludedFile(ctx context.Context, catalogKey string, srcHash [20]byte, info catalog.ImageLoader) {
	// Set a timeout in case image processing takes too long.
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	stat, err := os.Stat(info.GetSourcePath())
	if err != nil {
		slog.Error("scan: failed to stat file", "path", info.GetSourcePath(), "err", err)
		return
	}
	if stat.Size() == 0 {
		slog.Warn("scan: skipping empty file", "path", info.GetSourcePath())
		return
	}

	fileModifiedAt := stat.ModTime().UTC().Truncate(time.Second)
	existing, err := cu.imgr.FindByHash(catalogKey, fmt.Sprintf("%x", srcHash))
	if err != nil {
		slog.Error("scan: failed to query existing image", "path", info.GetSourcePath(), "err", err)
		return
	}
	if existing != nil && existing.FileModifiedAt.Valid {
		if existing.FileModifiedAt.Time.UTC().Truncate(time.Second).Equal(fileModifiedAt) {
			slog.Debug("scan: skipped unchanged", "path", info.GetSourcePath())
			return
		}
	}

	img, meta, err := info.Load()
	if err != nil {
		slog.Error("scan: failed to load image", "path", info.GetSourcePath(), "err", err)
		return
	}

	imseq := improc.NewSequencer()
	imseq.Push(exif_rotation.NewExifRotation())
	img, _ = imseq.Apply(ctx2, img, meta)

	// The full-size image is no longer needed after thumbnail generation; clear the reference early to encourage GC (OOM mitigation).
	jbuf := &bytes.Buffer{}
	resized := imgconv.Resize(img, &imgconv.ResizeOption{Width: 256})
	img = nil //nolint:ineffassign // intentionally cleared to encourage GC (OOM mitigation)
	if err := jpeg.Encode(jbuf, resized, &jpeg.Options{Quality: jpeg.DefaultQuality}); err != nil {
		slog.Error("scan: failed to encode thumbnail", "path", info.GetSourcePath(), "err", err)
		return
	}
	resized = nil //nolint:ineffassign // intentionally cleared to encourage GC (OOM mitigation)

	// Release the full-size image cached inside the loader before the DB write.
	// Without this, info holds the decoded image until the goroutine exits — which may be
	// delayed significantly when all goroutines pile up waiting for the SQLite single connection.
	if ptr, ok := info.(interface{ ClearImage() }); ok {
		ptr.ClearImage()
	}

	rec := &model.Image{
		CatalogKey: catalogKey,
		Rnd:        rand.Float64(),
		Src:        meta.ImageSourcePath,
		SrcHash:    fmt.Sprintf("%x", srcHash),
		FileModifiedAt: sql.NullTime{
			Time:  meta.FileModifiedAt,
			Valid: true,
		},
		TakenAt: sql.NullTime{
			Time:  meta.ExifDateTime,
			Valid: !meta.ExifDateTime.IsZero(),
		},
		ImageOrientation: meta.ImageOrientation,
		ThumbJPG:         jbuf.Bytes(),
	}
	if err = cu.imgr.UpsertActiveImage(rec); err != nil {
		slog.Error("scan: failed to upsert image", "path", meta.ImageSourcePath, "err", err)
		return
	}

	slog.Debug("scan: included", "path", meta.ImageSourcePath)
}

// processExcludedFile registers a file received from excludedFileCh as logically deleted in the DB.
// Because RDBMS has no native negative index, we insert data that is logically deleted from the start.
func (cu *catalogUseCase) processExcludedFile(catalogKey string, srcHash [20]byte, info catalog.ImageLoader) {
	if err := cu.imgr.UpsertInactiveImage(catalogKey, fmt.Sprintf("%x", srcHash), info.GetSourcePath()); err != nil {
		slog.Error("scan: failed to upsert inactive image", "path", info.GetSourcePath(), "err", err)
	}
	slog.Debug("scan: excluded", "path", info.GetSourcePath())
}

func (uc *catalogUseCase) PurgeOrphans() error {

	uc.imgr.FindAll(func(c *model.Image) error {
		if _, err := os.Stat(c.Src); errors.Is(err, os.ErrNotExist) {
			slog.Info("purge: deleted orphan", "path", c.Src)
			return uc.imgr.RemoveImage(c.ID)
		} else {
			slog.Debug("purge: exists", "path", c.Src)
		}
		return nil
	})

	return nil
}

func (uc *catalogUseCase) FindLocalImageById(catalogKey string, id model.PrimaryKey) (*model.Image, error) {
	return uc.imgr.FindById(id)
}

func (uc *catalogUseCase) LoadSourceImageById(id model.PrimaryKey) (image.Image, *model.ImgMeta, error) {
	rec, err := uc.imgr.FindById(id)
	if err != nil {
		return nil, nil, err
	}
	return catalog.LoadImageFromPath(rec.Src)
}

func (uc *catalogUseCase) ListImages(catalogKey string, cb func(*model.Image) error) error {
	return uc.imgr.ListByCatalog(catalogKey, cb)
}

func (uc *catalogUseCase) ToggleLocalImageFileVisibility(catalogKey string, ids []model.PrimaryKey) error {
	return uc.imgr.ToggleDeletedAt(ids)
}

// GetSequencerGroupForDisplay returns the image processing sequence group for a given display key.
// Returns the sequencer group, display metadata, and any error if the display is not found.
func (uc *catalogUseCase) GetSequencerGroupForDisplay(displayKey string) (improc.SequencerGroup, epaper.DisplayMetadata, error) {
	displayConfigInUse, ok := uc.serviceConfig.Displays[displayKey]
	if !ok {
		return nil, nil, &catalog.DisplayNotFoundError{Key: displayKey}
	}

	display := epaper.NewDisplay(epaper.EPaperDisplayModel(displayConfigInUse.DisplayModel), model.CanonicalOrientation(displayConfigInUse.Orientation))

	// Sequencer group
	imseqGroup := improc.NewSequencerGroup()

	// Pre-processing
	imPreProcessingSeq := improc.NewSequencer()
	imseqGroup.Push(imPreProcessingSeq)
	imPreProcessingSeq.Push(exif_rotation.NewExifRotation())
	imPreProcessingSeq.Push(crop.NewImageCropper(display))

	// Image processors configured for the display.
	impDispCatalogSeq := improc.NewSequencer()
	imseqGroup.Push(impDispCatalogSeq)

	for _, proc := range displayConfigInUse.ImageProcessors {
		switch proc.Type {
		case config.ImageProcessorTypeBlur:
			impDispCatalogSeq.Push(blur.NewImageBlur(proc.Data))
		case config.ImageProcessorTypeBrightness:
			impDispCatalogSeq.Push(brightness.NewImageBrightness(proc.Data))
		case config.ImageProcessorTypeContrast:
			impDispCatalogSeq.Push(contrast.NewImageContrast(proc.Data))
		case config.ImageProcessorTypeGamma:
			impDispCatalogSeq.Push(gamma.NewImageGamma(proc.Data))
		case config.ImageProcessorTypeHue:
			impDispCatalogSeq.Push(hue.NewImageHue(proc.Data))
		case config.ImageProcessorTypeSaturation:
			impDispCatalogSeq.Push(saturation.NewImageSaturation(proc.Data))
		case config.ImageProcessorTypeSelectiveColor:
			impDispCatalogSeq.Push(selective_color.NewSelectiveColor(proc.Data))
		default:
			// do nothing
		}
	}

	// Post-processing
	imPostProcessorSeq := improc.NewSequencer()
	imseqGroup.Push(imPostProcessorSeq)
	imPostProcessorSeq.Push(color_reduction.NewImageColorReduction(display, displayConfigInUse.ColorReduction))

	if displayConfigInUse.ShowTimestamp {
		imPostProcessorSeq.Push(timestamp.NewTimstamp())
	}

	if displayConfigInUse.Flip {
		slog.Debug("Flip is enabled")
		imPostProcessorSeq.Push(rotation.NewRotation())
	}

	return imseqGroup, display, nil
}

func (uc *catalogUseCase) Pick(displayKey string) (catalog.ImageLoader, epaper.DisplayMetadata, improc.SequencerGroup, error) {

	imseqGroup, display, err := uc.GetSequencerGroupForDisplay(displayKey)
	if err != nil {
		return nil, nil, nil, err
	}

	displayConfigInUse, ok := uc.serviceConfig.Displays[displayKey]
	if !ok {
		return nil, nil, nil, &catalog.DisplayNotFoundError{Key: displayKey}
	}

	var imgPtr catalog.ImageLoader
	if len(displayConfigInUse.Catalog) == 0 {
		var err error
		imgPtr, err = catalog.NewColorbarProvider(display).Resolve()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to resolve image provider for display %s: %w", displayKey, err)
		}
	} else {
		imgProvider := catalog.PickImageProvider(time.Now(), display, uc.imgr, displayConfigInUse.Catalog...)
		var err error
		imgPtr, err = imgProvider.Resolve()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to resolve image provider for display %s: %w", displayKey, err)
		}
	}

	return imgPtr, display, imseqGroup, nil
}

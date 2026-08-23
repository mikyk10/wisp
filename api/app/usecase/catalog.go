package usecase

import (
	"context"
	"crypto/sha1" //nolint:gosec // sha1 is cryptographically weak, but is used here only as a hash to avoid collisions
	"database/sql"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"maps"
	"math/rand/v2"
	"os"
	"os/exec"
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

	// Fetch retrieves images from background HTTP catalogs and stores them in the database.
	// If catalogKeys is empty, all background HTTP catalogs are fetched.
	Fetch(catalogKeys []string, workers int, maxRetries int, verbose bool) error

	//
	Pick(displayKey string) (catalog.ImageLoader, epaper.DisplayMetadata, improc.SequencerGroup, error)

	// RecordDelivery files one picture handed to one display. It reports
	// nothing back: see the implementation for why no caller may be given
	// something to fail on.
	RecordDelivery(displayKey string, prov catalog.Provenance, sleepSeconds int)
}

type catalogUseCase struct {
	globalConfig  *config.GlobalConfig
	serviceConfig *config.ServiceConfig
	imgr          repository.ImageRepository
	dhr           repository.DeliveryHistoryRepository
}

func NewCatalogUseCase(globalConfig *config.GlobalConfig, serviceConfig *config.ServiceConfig, imgr repository.ImageRepository, dhr repository.DeliveryHistoryRepository) CatalogUsecase {
	return &catalogUseCase{
		globalConfig:  globalConfig,
		serviceConfig: serviceConfig,
		imgr:          imgr,
		dhr:           dhr,
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

	// Spread rnd evenly now that the catalogue has settled. New rows arrive
	// with a value drawn at random, which leaves gaps of wildly differing size,
	// and since a row keeps its value the same photographs stay favoured and
	// the same ones stay unreachable. Doing it here rather than per row keeps
	// the delivery path free of writes.
	if err := cu.imgr.ReshuffleRandom(reshuffleProgressLogger()); err != nil {
		// The scan itself succeeded; an uneven spread is the state the
		// catalogue was already in, so it is not worth failing over.
		slog.Error("scan: failed to even out the random ordering", "err", err)
	}

	return nil
}

// reshuffleLogInterval is how many rows pass between progress lines. The
// statements themselves are logged at error level — several hundred of them,
// each carrying five hundred WHEN clauses, would bury everything else — so
// this is the only sign the pass is running, and a catalogue of a couple of
// hundred thousand should report a handful of times rather than once.
const reshuffleLogInterval = 25000

// reshuffleProgressLogger reports how far the even-spreading has got, and how
// much longer it looks like taking. The estimate is simply the rate so far
// carried forward, which is close enough: every batch does the same amount of
// work as the last.
func reshuffleProgressLogger() func(done, total int) {
	started := time.Now()
	lastLogged := 0

	return func(done, total int) {
		switch {
		case done == 0:
			slog.Info("scan: evening out the random ordering", "images", total)
		case done == total:
			slog.Info("scan: evened out the random ordering",
				"images", total, "elapsed", time.Since(started).Round(time.Second))
		case done-lastLogged >= reshuffleLogInterval:
			lastLogged = done
			elapsed := time.Since(started)
			remaining := time.Duration(float64(elapsed) / float64(done) * float64(total-done))
			slog.Info("scan: evening out the random ordering",
				"done", done, "total", total,
				"elapsed", elapsed.Round(time.Second),
				"remaining", remaining.Round(time.Second))
		}
	}
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
	hookWg := &sync.WaitGroup{}
	sem := make(chan struct{}, concurrency)

	if pconf.Hooks.OnNewFile != "" {
		slog.Info("scan: on_new_file hook configured", "catalog", provConf.Key, "cmd", pconf.Hooks.OnNewFile)
	}

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
			srcHash := sha1.Sum([]byte(info.Provenance().Source))
			wg.Add(1)
			sem <- struct{}{}
			go func(h [20]byte, ldr catalog.ImageLoader) {
				defer func() { wg.Done(); <-sem }()
				cu.processIncludedFile(ctx, provConf.Key, h, ldr, pconf.Hooks.OnNewFile, hookWg)
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
			srcHash := sha1.Sum([]byte(info.Provenance().Source))
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
	hookWg.Wait()
	slog.Info("scan completed", "catalog", provConf.Key, "total", dispatched)
}

// processIncludedFile processes a file received from includedFileCh and registers it in the DB.
// imseq is created per goroutine, so it is thread-safe.
func (cu *catalogUseCase) processIncludedFile(ctx context.Context, catalogKey string, srcHash [20]byte, info catalog.ImageLoader, onNewFileCmd string, hookWg *sync.WaitGroup) {
	// Set a timeout in case image processing takes too long.
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	src := info.Provenance().Source

	stat, err := os.Stat(src)
	if err != nil {
		slog.Error("scan: failed to stat file", "path", src, "err", err)
		return
	}
	if stat.Size() == 0 {
		slog.Warn("scan: skipping empty file", "path", src)
		return
	}

	fileModifiedAt := stat.ModTime().UTC().Truncate(time.Second)
	existing, err := cu.imgr.FindByHash(catalogKey, fmt.Sprintf("%x", srcHash))
	if err != nil {
		slog.Error("scan: failed to query existing image", "path", src, "err", err)
		return
	}
	if existing != nil && existing.FileModifiedAt.Valid {
		if existing.FileModifiedAt.Time.UTC().Truncate(time.Second).Equal(fileModifiedAt) {
			slog.Debug("scan: skipped unchanged", "path", src)
			return
		}
	}

	img, meta, err := info.Load()
	if err != nil {
		slog.Error("scan: failed to load image", "path", src, "err", err)
		return
	}

	imseq := improc.NewSequencer()
	imseq.Push(exif_rotation.NewExifRotation())
	img, _ = imseq.Apply(ctx2, img, meta)

	// The full-size image is no longer needed after thumbnail generation; clear the reference early to encourage GC (OOM mitigation).
	thumb, err := encodeThumbnail(img)
	img = nil //nolint:ineffassign // intentionally cleared to encourage GC (OOM mitigation)
	if err != nil {
		slog.Error("scan: failed to encode thumbnail", "path", src, "err", err)
		return
	}

	// Release the full-size image cached inside the loader before the DB write.
	// Without this, info holds the decoded image until the goroutine exits — which may be
	// delayed significantly when all goroutines pile up waiting for the SQLite single connection.
	if clearable, ok := info.(catalog.ClearableImageLoader); ok {
		clearable.ClearImage()
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
		ThumbJPG:         thumb,
	}
	if err = cu.imgr.UpsertActiveImage(rec); err != nil {
		slog.Error("scan: failed to upsert image", "path", meta.ImageSourcePath, "err", err)
		return
	}

	isNewFile := existing == nil
	if isNewFile && onNewFileCmd != "" {
		hookWg.Add(1)
		go func(cmd, filePath string) {
			defer hookWg.Done()
			runOnNewFileHook(cmd, filePath)
		}(onNewFileCmd, meta.ImageSourcePath)
	}

	slog.Debug("scan: included", "path", meta.ImageSourcePath, "new", isNewFile)
}

// processExcludedFile registers a file received from excludedFileCh as logically deleted in the DB.
// Because RDBMS has no native negative index, we insert data that is logically deleted from the start.
func (cu *catalogUseCase) processExcludedFile(catalogKey string, srcHash [20]byte, info catalog.ImageLoader) {
	src := info.Provenance().Source
	if err := cu.imgr.UpsertInactiveImage(catalogKey, fmt.Sprintf("%x", srcHash), src); err != nil {
		slog.Error("scan: failed to upsert inactive image", "path", src, "err", err)
	}
	slog.Debug("scan: excluded", "path", src)
}

func (uc *catalogUseCase) PurgeOrphans() error {

	uc.imgr.FindAll(func(c *model.Image) error {
		// Only check filesystem existence for file-based images.
		// HTTP images have URLs in Src, not file paths.
		if c.SrcType != "" && c.SrcType != "file" {
			return nil
		}
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
	return uc.getSequencerGroupForDisplay(displayConfigInUse, nil)
}

func (uc *catalogUseCase) getSequencerGroupForDisplay(displayConfigInUse *config.DisplayConfig, colorReductionOverride *config.ColorReduction) (improc.SequencerGroup, epaper.DisplayMetadata, error) {
	display := epaper.NewDisplay(epaper.EPaperDisplayModel(displayConfigInUse.DisplayModel), model.CanonicalOrientation(displayConfigInUse.Orientation))

	// Sequencer group
	imseqGroup := improc.NewSequencerGroup()

	// Pre-processing
	imPreProcessingSeq := improc.NewSequencer()
	imseqGroup.Push(imPreProcessingSeq)
	// Deferred: crop carries out the EXIF normalisation together with its own
	// orientation correction, in one pass over the full-resolution image.
	imPreProcessingSeq.Push(exif_rotation.NewDeferredExifRotation())
	imPreProcessingSeq.Push(crop.NewImageCropper(display, displayConfigInUse.Crop.Strategy))

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

	// Post-processing: use per-catalog color reduction if provided, otherwise display default.
	cr := displayConfigInUse.ColorReduction
	if colorReductionOverride != nil {
		cr = *colorReductionOverride
	}

	imPostProcessorSeq := improc.NewSequencer()
	imseqGroup.Push(imPostProcessorSeq)
	imPostProcessorSeq.Push(color_reduction.NewImageColorReduction(display, cr))

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

	displayConfigInUse, ok := uc.serviceConfig.Displays[displayKey]
	if !ok {
		return nil, nil, nil, &catalog.DisplayNotFoundError{Key: displayKey}
	}

	// Pick image first to determine per-catalog color reduction override.
	var imgPtr catalog.ImageLoader
	var colorReductionOverride *config.ColorReduction

	if len(displayConfigInUse.Catalog) == 0 {
		var err error
		imgPtr, err = catalog.NewColorbarProvider(
			epaper.NewDisplay(epaper.EPaperDisplayModel(displayConfigInUse.DisplayModel), model.CanonicalOrientation(displayConfigInUse.Orientation)),
		).Resolve()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to resolve image provider for display %s: %w", displayKey, err)
		}
	} else {
		display := epaper.NewDisplay(epaper.EPaperDisplayModel(displayConfigInUse.DisplayModel), model.CanonicalOrientation(displayConfigInUse.Orientation))
		pick := catalog.PickImageProvider(time.Now(), display, uc.imgr, displayConfigInUse.Catalog...)
		colorReductionOverride = pick.ColorReduction
		var err error
		imgPtr, err = pick.Locator.Resolve()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to resolve image provider for display %s: %w", displayKey, err)
		}
	}

	imseqGroup, display, err := uc.getSequencerGroupForDisplay(displayConfigInUse, colorReductionOverride)
	if err != nil {
		return nil, nil, nil, err
	}

	return imgPtr, display, imseqGroup, nil
}

// RecordDelivery files one picture handed to one display.
//
// It reports nothing back, deliberately. This runs on the request that hands a
// picture to a panel, after the picture has gone down the wire, and a delivery
// that was made is not undone by failing to write it down — the same reasoning
// as runOnNewFileHook below, where a failing hook must not affect the scan.
// Failures are logged at warn and go no further.
//
// prov comes from the loader that produced the picture and is used as it
// stands. A provider that gives up hands back an error card and still reports
// success, so a kind inferred from the fact that this was called at all would
// record exactly those failures as photographs.
func (uc *catalogUseCase) RecordDelivery(displayKey string, prov catalog.Provenance, sleepSeconds int) {
	if uc.dhr == nil || uc.globalConfig == nil || uc.globalConfig.DeliveryHistory.Disabled {
		return
	}

	// Only a display named in service.yaml may leave a row behind. The device
	// API is unauthenticated, so the key is whatever the requester put in the
	// path, and recording all of them would let anything on the network fill
	// the table with displays that do not exist.
	//
	// The check has to be on the key as it arrived. The handler falls back to a
	// default display for a key it does not know, so anything derived from the
	// resolved display would file an unregistered panel under that default.
	if _, ok := uc.serviceConfig.Displays[displayKey]; !ok {
		slog.Warn("delivery history: ignoring a delivery to an unconfigured display", "display", displayKey)
		return
	}

	rec := &model.DeliveryHistory{
		DisplayKey:   displayKey,
		DeliveredAt:  time.Now(),
		Kind:         prov.Kind,
		ImageID:      prov.ImageID,
		CatalogKey:   prov.CatalogKey,
		Source:       prov.Source,
		Reason:       prov.Reason,
		SleepSeconds: sleepSeconds,
	}

	// Record swallows its own storage failures and always answers nil; the
	// branch is here because the interface still returns an error.
	if err := uc.dhr.Record(rec, uc.globalConfig.DeliveryHistory.Size); err != nil {
		slog.Warn("delivery history: record failed", "display", displayKey, "err", err)
	}
}

// runOnNewFileHook executes the on_new_file hook command for a newly registered file.
// The placeholder {file} in cmd is replaced with filePath.
// Errors are logged but never propagated — a failing hook must not affect the scan.
func runOnNewFileHook(cmd, filePath string) {
	expanded := strings.ReplaceAll(cmd, "{file}", filePath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	//nolint:gosec // cmd is from trusted config, not user input
	c := exec.CommandContext(ctx, "sh", "-c", expanded)
	out, err := c.CombinedOutput()
	if err != nil {
		slog.Error("hook: on_new_file failed", "cmd", cmd, "file", filePath, "err", err, "output", string(out))
		return
	}
	slog.Info("hook: on_new_file completed", "file", filePath)
}

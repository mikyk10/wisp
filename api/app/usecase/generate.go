package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
)

// GenerateRunOptions configures a batch generation run.
type GenerateRunOptions struct {
	CatalogKey string
	SourceID   model.PrimaryKey // explicit source image ID (0 = random from source_catalog)
	Workers    int
	DryRun     bool
	Verbose    bool
}

// GenerateUsecase runs AI image generation pipelines.
type GenerateUsecase interface {
	Run(ctx context.Context, opts GenerateRunOptions) error
	List(catalogKey string) ([]*model.GenerationCacheEntry, error)
	Clean(catalogKey string, failedOnly bool) error
	Favorite(catalogKey string, cacheID model.PrimaryKey, destCatalogKey string, svcConfig *config.ServiceConfig) error
}

type generateUsecase struct {
	cfg       *config.GlobalConfig
	svcCfg    *config.ServiceConfig
	repo      repository.AIRepository
	imageRepo repository.ImageRepository
	runner    *PipelineRunner
}

func NewGenerateUsecase(
	cfg *config.GlobalConfig,
	svcCfg *config.ServiceConfig,
	repo repository.AIRepository,
	imageRepo repository.ImageRepository,
) GenerateUsecase {
	return &generateUsecase{
		cfg:       cfg,
		svcCfg:    svcCfg,
		repo:      repo,
		imageRepo: imageRepo,
		runner:    NewPipelineRunner(cfg, repo),
	}
}

func (u *generateUsecase) Run(ctx context.Context, opts GenerateRunOptions) error {
	catConfig, err := u.getGenerateConfig(opts.CatalogKey)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			slog.Info("generate: received signal, stopping")
			cancel()
		case <-ctx.Done():
		}
	}()

	workers := opts.Workers
	if workers <= 0 {
		workers = u.cfg.AI.Workers
	}
	if workers <= 0 {
		workers = 2
	}

	// Check current cache and evict if needed
	currentCount, err := u.repo.CountCacheEntries(opts.CatalogKey)
	if err != nil {
		return fmt.Errorf("count cache: %w", err)
	}

	if currentCount >= int64(catConfig.CacheDepth) && catConfig.EvictCount > 0 {
		slog.Info("generate: evicting old entries", "count", catConfig.EvictCount)
		if !opts.DryRun {
			if err := u.repo.EvictOldestCacheEntries(opts.CatalogKey, catConfig.EvictCount); err != nil {
				return fmt.Errorf("evict cache: %w", err)
			}
			currentCount -= int64(catConfig.EvictCount)
			if currentCount < 0 {
				currentCount = 0
			}
		}
	}

	toGenerate := int64(catConfig.CacheDepth) - currentCount
	if toGenerate <= 0 {
		slog.Info("generate: cache is full", "depth", catConfig.CacheDepth, "current", currentCount)
		return nil
	}

	slog.Info("generate: starting", "catalog", opts.CatalogKey, "to_generate", toGenerate, "workers", workers, "dry_run", opts.DryRun)

	u.runner.SetVerbose(opts.Verbose)

	var (
		successCount atomic.Int64
		failedCount  atomic.Int64
	)

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

loop:
	for i := int64(0); i < toGenerate; i++ {
		select {
		case <-ctx.Done():
			break loop
		default:
		}

		if opts.DryRun {
			slog.Info("generate: [dry-run] would generate image", "index", i+1)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(idx int64) {
			defer wg.Done()
			defer func() { <-sem }()

			err := u.generateOne(ctx, opts, catConfig, idx+1, toGenerate)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("generate: generation failed", "index", idx+1, "err", err)
				failedCount.Add(1)
				return
			}
			successCount.Add(1)
		}(i)
	}
	wg.Wait()

	// Clean up failed executions
	if failedCount.Load() > 0 {
		_ = u.repo.CleanFailedExecutions(opts.CatalogKey)
	}

	slog.Info("generate: completed", "success", successCount.Load(), "failed", failedCount.Load())
	return nil
}

func (u *generateUsecase) generateOne(ctx context.Context, opts GenerateRunOptions, catConfig config.ImageGenerateProviderConfig, idx, total int64) error {
	// Resolve source image if needed
	var sourceImage []byte
	var sourceImageID *model.PrimaryKey

	if catConfig.SourceCatalog != "" {
		img, err := u.resolveSourceImage(opts, catConfig.SourceCatalog)
		if err != nil {
			return fmt.Errorf("resolve source image: %w", err)
		}
		if img == nil {
			return fmt.Errorf("no images found in source catalog %q (run 'catalog scan' first)", catConfig.SourceCatalog)
		}
		sourceImage = img.ThumbJPG
		sourceImageID = &img.ID
	}

	exec := &model.PipelineExecution{
		PipelineType:  "generate",
		CatalogKey:    opts.CatalogKey,
		SourceImageID: sourceImageID,
		Status:        model.StatusRunning,
		StartedAt:     time.Now(),
	}
	if err := u.repo.CreatePipelineExecution(exec); err != nil {
		return err
	}

	// Embedded prompt fallbacks for default generation pipeline
	embeddedPrompts := map[string]string{
		"meta-prompt": "prompts/default_gen_meta.md",
		"generate":    "prompts/default_gen_image.md",
	}

	result, err := u.runner.RunPipeline(ctx, RunPipelineInput{
		PipelineExecID:  exec.ID,
		Stages:          catConfig.Pipeline.Stages,
		SourceImage:     sourceImage,
		ConfigVars:      nil,
		EmbeddedPrompts: embeddedPrompts,
	})

	exec.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}
	if err != nil {
		exec.Status = model.StatusFailed
		_ = u.repo.UpdatePipelineExecution(exec)
		return err
	}

	exec.Status = model.StatusSuccess
	_ = u.repo.UpdatePipelineExecution(exec)

	// Finalize: store last image output in cache
	imgData, contentType := result.LastImageOutput()
	if imgData == nil {
		return fmt.Errorf("pipeline produced no image output")
	}

	entry := &model.GenerationCacheEntry{
		CatalogKey:          opts.CatalogKey,
		PipelineExecutionID: exec.ID,
		ImageData:           imgData,
		ContentType:         contentType,
	}
	if err := u.repo.CreateCacheEntry(entry); err != nil {
		return fmt.Errorf("save cache entry: %w", err)
	}

	slog.Info("generate: cached image", "index", fmt.Sprintf("%d/%d", idx, total), "cache_id", entry.ID)
	return nil
}

func (u *generateUsecase) resolveSourceImage(opts GenerateRunOptions, sourceCatalog string) (*model.Image, error) {
	if opts.SourceID > 0 {
		return u.imageRepo.FindById(opts.SourceID)
	}
	return u.repo.FindRandomImage(sourceCatalog)
}

func (u *generateUsecase) List(catalogKey string) ([]*model.GenerationCacheEntry, error) {
	return u.repo.ListCacheEntries(catalogKey)
}

func (u *generateUsecase) Clean(catalogKey string, failedOnly bool) error {
	if failedOnly {
		return u.repo.CleanFailedExecutions(catalogKey)
	}
	// Clean all: evict everything
	count, err := u.repo.CountCacheEntries(catalogKey)
	if err != nil {
		return err
	}
	if count > 0 {
		return u.repo.EvictOldestCacheEntries(catalogKey, int(count))
	}
	return u.repo.CleanFailedExecutions(catalogKey)
}

func (u *generateUsecase) Favorite(catalogKey string, cacheID model.PrimaryKey, destCatalogKey string, svcConfig *config.ServiceConfig) error {
	entry, err := u.repo.FindCacheEntryByID(cacheID)
	if err != nil {
		return fmt.Errorf("find cache entry: %w", err)
	}
	if entry == nil {
		return fmt.Errorf("cache entry %d not found", cacheID)
	}

	destConf, ok := svcConfig.Catalog[destCatalogKey]
	if !ok {
		return fmt.Errorf("destination catalog %q not found", destCatalogKey)
	}
	fileConf, ok := destConf.Config.(config.ImageFileProviderConfig)
	if !ok {
		return fmt.Errorf("destination catalog %q is not a file catalog", destCatalogKey)
	}

	ext := ".png"
	if entry.ContentType == "image/jpeg" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("generated_%d_%d%s", cacheID, time.Now().Unix(), ext)
	destPath := filepath.Join(fileConf.SrcPath, filename)

	if err := os.WriteFile(destPath, entry.ImageData, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", destPath, err)
	}

	slog.Info("generate: favorited image", "cache_id", cacheID, "dest", destPath)
	return nil
}

func (u *generateUsecase) getGenerateConfig(catalogKey string) (config.ImageGenerateProviderConfig, error) {
	provConfig, ok := u.svcCfg.Catalog[catalogKey]
	if !ok {
		return config.ImageGenerateProviderConfig{}, fmt.Errorf("catalog %q not found", catalogKey)
	}
	genConfig, ok := provConfig.Config.(config.ImageGenerateProviderConfig)
	if !ok {
		return config.ImageGenerateProviderConfig{}, fmt.Errorf("catalog %q is not a generate type", catalogKey)
	}
	return genConfig, nil
}


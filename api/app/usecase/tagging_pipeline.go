package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mikyk10/wisp/app/domain/ai"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
)

// Error codes stored in ai_runs.error_code.
const (
	ErrCodeInputMissing    = "input_missing"
	ErrCodeDescriptorFailed = "descriptor_failed"
	ErrCodeTaggingFailed   = "tagging_failed"
	ErrCodeFormatInvalid   = "format_invalid"
)

// TaggingRunOptions configures a single pipeline run.
type TaggingRunOptions struct {
	CatalogKey       string
	Workers          int    // 0 = config default
	Limit            int    // 0 = no limit
	Rebuild          bool
	Stage                int    // 0 = all, 2 = tagging only (--rebuild required)
	DryRun               bool
	DescriptorPromptPath string // prompt file path for descriptor; "" = use the client's built-in prompt
	TaggerPromptPath     string // prompt file path for tagger;     "" = use the client's built-in prompt
}

// TaggingPipelineStats collects run counters for progress reporting.
type TaggingPipelineStats struct {
	Total     int64
	Processed atomic.Int64
	Success   atomic.Int64
	Failed    atomic.Int64
	Skipped   atomic.Int64
}

// TaggingPipelineUsecase runs the AI tagging pipeline.
type TaggingPipelineUsecase interface {
	Run(ctx context.Context, opts TaggingRunOptions) error
}

type taggingPipelineUsecase struct {
	cfg        *config.GlobalConfig
	taggingRepo repository.TaggingRepository
	descriptor ai.DescriptorClient
	tagger     ai.TaggerClient
}

// NewTaggingPipelineUsecase constructs the usecase via DI.
func NewTaggingPipelineUsecase(
	cfg *config.GlobalConfig,
	taggingRepo repository.TaggingRepository,
	descriptor ai.DescriptorClient,
	tagger ai.TaggerClient,
) TaggingPipelineUsecase {
	return &taggingPipelineUsecase{
		cfg:         cfg,
		taggingRepo: taggingRepo,
		descriptor:  descriptor,
		tagger:      tagger,
	}
}

func (p *taggingPipelineUsecase) Run(ctx context.Context, opts TaggingRunOptions) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Graceful shutdown on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			slog.Info("tagging: received signal, stopping")
			cancel()
		case <-ctx.Done():
		}
	}()

	// Validate client configuration before spawning any workers.
	// Errors here are structural (e.g. provider not in config) and will affect every image.
	if err := p.descriptor.Validate(); err != nil {
		return fmt.Errorf("descriptor client: %w", err)
	}
	if err := p.tagger.Validate(); err != nil {
		return fmt.Errorf("tagger client: %w", err)
	}

	// If a prompt file path is specified, build a new client that uses that file.
	descriptor := p.descriptor
	tagger := p.tagger
	if opts.DescriptorPromptPath != "" {
		d, err := p.descriptor.WithPromptPath(opts.DescriptorPromptPath)
		if err != nil {
			return fmt.Errorf("load descriptor prompt: %w", err)
		}
		descriptor = d
		slog.Info("tagging: using custom descriptor prompt", "path", opts.DescriptorPromptPath)
	}
	if opts.TaggerPromptPath != "" {
		t, err := p.tagger.WithPromptPath(opts.TaggerPromptPath)
		if err != nil {
			return fmt.Errorf("load tagger prompt: %w", err)
		}
		tagger = t
		slog.Info("tagging: using custom tagger prompt", "path", opts.TaggerPromptPath)
	}

	// Resolve workers.
	workers := opts.Workers
	if workers <= 0 {
		workers = p.cfg.AI.Workers
	}
	if workers <= 0 {
		workers = 2
	}

	// Fetch images.
	var images []*model.Image
	var err error
	if opts.Rebuild {
		images, err = p.taggingRepo.FindAllImages(opts.CatalogKey, opts.Limit)
	} else {
		images, err = p.taggingRepo.FindImagesForTagging(opts.CatalogKey, opts.Limit)
	}
	if err != nil {
		return fmt.Errorf("fetch images: %w", err)
	}

	if len(images) == 0 {
		slog.Info("tagging: no images to process")
		return nil
	}

	stats := &TaggingPipelineStats{Total: int64(len(images))}
	slog.Info("tagging: starting", "catalog", opts.CatalogKey, "total", stats.Total, "workers", workers, "rebuild", opts.Rebuild, "dry_run", opts.DryRun)

	// Progress reporter.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		start := time.Now()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				processed := stats.Processed.Load()
				elapsed := time.Since(start).Seconds()
				qps := 0.0
				if elapsed > 0 {
					qps = float64(processed) / elapsed
				}
				remaining := stats.Total - processed
				eta := 0.0
				if qps > 0 {
					eta = float64(remaining) / qps
				}
				slog.Info("tagging: progress",
					"processed", processed,
					"total", stats.Total,
					"success", stats.Success.Load(),
					"failed", stats.Failed.Load(),
					"skipped", stats.Skipped.Load(),
					"qps", fmt.Sprintf("%.2f", qps),
					"eta_sec", fmt.Sprintf("%.0f", eta),
				)
			}
		}
	}()
	defer close(done)

	// Worker pool.
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for _, img := range images {
		select {
		case <-ctx.Done():
			break
		default:
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(img *model.Image) {
			defer wg.Done()
			defer func() { <-sem }()
			p.processImage(ctx, img, opts, descriptor, tagger, stats)
		}(img)
	}
	wg.Wait()

	slog.Info("tagging: completed",
		"total", stats.Total,
		"success", stats.Success.Load(),
		"failed", stats.Failed.Load(),
		"skipped", stats.Skipped.Load(),
	)
	return nil
}

func (p *taggingPipelineUsecase) processImage(ctx context.Context, img *model.Image, opts TaggingRunOptions, descriptor ai.DescriptorClient, tagger ai.TaggerClient, stats *TaggingPipelineStats) {
	defer stats.Processed.Add(1)

	log := slog.With("image_id", img.ID, "catalog", img.CatalogKey)

	// --rebuild: check for empty thumbnail first.
	if opts.Rebuild && len(img.ThumbJPG) == 0 {
		log.Warn("tagging: skipping image with empty thumbnail", "error_code", ErrCodeInputMissing)
		p.recordFailedRun(img.ID, model.AIRunStageDescriptor, img.SrcHash, ErrCodeInputMissing, "thumbnail is empty")
		stats.Failed.Add(1)
		return
	}

	if opts.DryRun {
		log.Info("tagging: [dry-run] would process image")
		stats.Skipped.Add(1)
		return
	}

	// Determine what needs to run.
	runDescriptor := true
	runTagging := true

	if !opts.Rebuild {
		// Normal mode: skip if already tagged.
		hasT, err := p.taggingRepo.HasImageTags(img.ID)
		if err != nil {
			log.Error("tagging: check has tags", "err", err)
			stats.Failed.Add(1)
			return
		}
		if hasT {
			log.Debug("tagging: skipping already-tagged image")
			stats.Skipped.Add(1)
			return
		}

		// If a successful descriptor exists, skip descriptor stage.
		descRun, err := p.taggingRepo.FindLatestSuccessfulDescriptor(img.ID)
		if err != nil {
			log.Error("tagging: find descriptor run", "err", err)
			stats.Failed.Add(1)
			return
		}
		if descRun != nil {
			runDescriptor = false
		}
	} else if opts.Stage == 2 {
		// --rebuild --stage=2: use existing descriptor if available; otherwise run descriptor too.
		descRun, err := p.taggingRepo.FindLatestSuccessfulDescriptor(img.ID)
		if err != nil {
			log.Error("tagging: find descriptor run (stage=2)", "err", err)
			stats.Failed.Add(1)
			return
		}
		if descRun != nil {
			runDescriptor = false
		}
	}

	// Run descriptor stage.
	var description string
	if runDescriptor {
		if len(img.ThumbJPG) == 0 {
			log.Warn("tagging: no thumbnail for descriptor", "error_code", ErrCodeInputMissing)
			p.recordFailedRun(img.ID, model.AIRunStageDescriptor, img.SrcHash, ErrCodeInputMissing, "thumbnail is empty")
			stats.Failed.Add(1)
			return
		}

		run := &model.AIRun{
			ImageID:   img.ID,
			Stage:     model.AIRunStageDescriptor,
			ModelName: descriptor.PromptModel(),
			Status:    model.AIRunStatusRunning,
			StartedAt: time.Now(),
			InputHash: img.SrcHash,
		}
		if err := p.taggingRepo.CreateAIRun(run); err != nil {
			log.Error("tagging: create descriptor run", "err", err)
			stats.Failed.Add(1)
			return
		}

		start := time.Now()
		desc, err := descriptor.Describe(ctx, img.ThumbJPG)
		latency := time.Since(start).Milliseconds()

		run.LatencyMs = latency
		run.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			run.Status = model.AIRunStatusFailed
			run.ErrorCode = ErrCodeDescriptorFailed
			run.ErrorMessage = err.Error()
			_ = p.taggingRepo.UpdateAIRun(run)
			log.Error("tagging: descriptor failed", "err", err)
			stats.Failed.Add(1)
			return
		}

		run.Status = model.AIRunStatusSuccess
		if err := p.taggingRepo.UpdateAIRun(run); err != nil {
			log.Error("tagging: update descriptor run", "err", err)
		}
		if err := p.taggingRepo.CreateAIOutput(&model.AIOutput{
			RunID:       run.ID,
			ContentText: desc,
		}); err != nil {
			log.Error("tagging: save descriptor output", "err", err)
		}
		description = desc
	} else {
		// Load description from previous run.
		descRun, _ := p.taggingRepo.FindLatestSuccessfulDescriptor(img.ID)
		if descRun != nil {
			output, err := p.taggingRepo.FindAIOutputByRunID(descRun.ID)
			if err != nil || output == nil {
				log.Error("tagging: load descriptor output", "err", err)
				stats.Failed.Add(1)
				return
			}
			description = output.ContentText
		}
	}

	// Run tagging stage.
	if runTagging {
		run := &model.AIRun{
			ImageID:   img.ID,
			Stage:     model.AIRunStageTagging,
			ModelName: tagger.PromptModel(),
			Status:    model.AIRunStatusRunning,
			StartedAt: time.Now(),
			InputHash: img.SrcHash,
		}
		if err := p.taggingRepo.CreateAIRun(run); err != nil {
			log.Error("tagging: create tagger run", "err", err)
			stats.Failed.Add(1)
			return
		}

		start := time.Now()
		tags, err := tagger.Tag(ctx, description)
		latency := time.Since(start).Milliseconds()

		run.LatencyMs = latency
		run.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			run.Status = model.AIRunStatusFailed
			run.ErrorCode = ErrCodeTaggingFailed
			run.ErrorMessage = err.Error()
			_ = p.taggingRepo.UpdateAIRun(run)
			log.Error("tagging: tagger failed", "err", err)
			stats.Failed.Add(1)
			return
		}

		run.Status = model.AIRunStatusSuccess
		if err := p.taggingRepo.UpdateAIRun(run); err != nil {
			log.Error("tagging: update tagger run", "err", err)
		}
		if err := p.taggingRepo.CreateAIOutput(&model.AIOutput{
			RunID:       run.ID,
			ContentText: strings.Join(tags, " "),
		}); err != nil {
			log.Error("tagging: save tagger output", "err", err)
		}

		// Resolve tag IDs and persist.
		tagIDs := make([]model.PrimaryKey, 0, len(tags))
		for _, tagName := range tags {
			tag, err := p.taggingRepo.FindOrCreateTag(tagName)
			if err != nil {
				log.Error("tagging: find/create tag", "tag", tagName, "err", err)
				stats.Failed.Add(1)
				return
			}
			tagIDs = append(tagIDs, tag.ID)
		}

		if err := p.taggingRepo.ReplaceImageTags(img.ID, run.ID, tagIDs); err != nil {
			log.Error("tagging: replace image tags", "err", err)
			stats.Failed.Add(1)
			return
		}

		log.Info("tagging: image tagged", "tags", tags)
	}

	stats.Success.Add(1)
}

func (p *taggingPipelineUsecase) recordFailedRun(imageID model.PrimaryKey, stage model.AIRunStage, inputHash, errorCode, errorMessage string) {
	run := &model.AIRun{
		ImageID:      imageID,
		Stage:        stage,
		ModelName:    "",
		Status:       model.AIRunStatusFailed,
		StartedAt:    time.Now(),
		FinishedAt:   sql.NullTime{Time: time.Now(), Valid: true},
		InputHash:    inputHash,
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
	}
	_ = p.taggingRepo.CreateAIRun(run)
}

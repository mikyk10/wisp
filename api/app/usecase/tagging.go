package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"regexp"
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

// TaggingRunOptions configures a single tagging pipeline run.
type TaggingRunOptions struct {
	CatalogKey string
	Workers    int
	Limit      int
	Rebuild    bool
	DryRun     bool
	Verbose    bool
}

// TaggingUsecase runs AI tagging on file catalog images.
type TaggingUsecase interface {
	Run(ctx context.Context, opts TaggingRunOptions) error
	Reset(catalogKey string, imageID model.PrimaryKey) error
}

type taggingUsecase struct {
	cfg    *config.GlobalConfig
	repo   repository.AIRepository
	runner *PipelineRunner
}

func NewTaggingUsecase(cfg *config.GlobalConfig, repo repository.AIRepository) TaggingUsecase {
	return &taggingUsecase{
		cfg:    cfg,
		repo:   repo,
		runner: NewPipelineRunner(cfg, repo),
	}
}

func (u *taggingUsecase) Run(ctx context.Context, opts TaggingRunOptions) error {
	stages := u.cfg.AI.Tagging.Pipeline.Stages
	if len(stages) == 0 {
		return fmt.Errorf("no tagging pipeline stages configured")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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

	workers := opts.Workers
	if workers <= 0 {
		workers = u.cfg.AI.Tagging.Workers
	}
	if workers <= 0 {
		workers = 2
	}

	var images []*model.Image
	var err error
	if opts.Rebuild {
		images, err = u.repo.FindAllImages(opts.CatalogKey, opts.Limit)
	} else {
		images, err = u.repo.FindImagesForTagging(opts.CatalogKey, opts.Limit)
	}
	if err != nil {
		return fmt.Errorf("fetch images: %w", err)
	}
	if len(images) == 0 {
		slog.Info("tagging: no images to process")
		return nil
	}

	var (
		total     = int64(len(images))
		processed atomic.Int64
		success   atomic.Int64
		failed    atomic.Int64
		skipped   atomic.Int64
	)

	slog.Info("tagging: starting", "catalog", opts.CatalogKey, "total", total, "workers", workers, "rebuild", opts.Rebuild, "dry_run", opts.DryRun)

	// Progress reporter
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
				p := processed.Load()
				elapsed := time.Since(start).Seconds()
				qps := 0.0
				if elapsed > 0 {
					qps = float64(p) / elapsed
				}
				slog.Info("tagging: progress", "processed", p, "total", total, "success", success.Load(), "failed", failed.Load(), "skipped", skipped.Load(), "qps", fmt.Sprintf("%.2f", qps))
			}
		}
	}()
	defer close(done)

	u.runner.SetVerbose(opts.Verbose)

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

loop:
	for _, img := range images {
		select {
		case <-ctx.Done():
			break loop
		default:
		}

		if opts.DryRun {
			slog.Info("tagging: [dry-run] would process", "image_id", img.ID)
			skipped.Add(1)
			processed.Add(1)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(img *model.Image) {
			defer wg.Done()
			defer func() { <-sem }()

			err := u.processImage(ctx, img, stages, opts)
			processed.Add(1)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("tagging: image failed", "image_id", img.ID, "err", err)
				failed.Add(1)
				return
			}
			success.Add(1)
		}(img)
	}
	wg.Wait()

	slog.Info("tagging: completed", "total", total, "success", success.Load(), "failed", failed.Load(), "skipped", skipped.Load())
	return nil
}

func (u *taggingUsecase) processImage(ctx context.Context, img *model.Image, stages []config.StageConfig, opts TaggingRunOptions) error {
	if len(img.ThumbJPG) == 0 {
		return fmt.Errorf("empty thumbnail")
	}

	// Check if already tagged (non-rebuild mode)
	if !opts.Rebuild {
		hasT, err := u.repo.HasImageTags(img.ID)
		if err != nil {
			return err
		}
		if hasT {
			return nil
		}
	}

	// Check for cached descriptor stage output
	skipStages := make(map[string]string)
	if len(stages) > 0 {
		descriptorStage := stages[0].Name
		cached, err := u.repo.FindLatestSuccessfulStep(img.ID, descriptorStage)
		if err == nil && cached != nil {
			out, err := u.repo.FindStepOutputByStepID(cached.ID)
			if err == nil && out != nil && out.ContentText != nil {
				skipStages[descriptorStage] = *out.ContentText
			}
		}
	}

	// Create pipeline execution
	exec := &model.PipelineExecution{
		PipelineType:  "tagging",
		CatalogKey:    img.CatalogKey,
		SourceImageID: &img.ID,
		Status:        model.StatusRunning,
		StartedAt:     time.Now(),
	}
	if err := u.repo.CreatePipelineExecution(exec); err != nil {
		return err
	}

	configVars := map[string]any{
		"MaxTags": u.cfg.AI.Tagging.MaxTags,
	}
	if configVars["MaxTags"] == 0 {
		configVars["MaxTags"] = 15
	}

	result, err := u.runner.RunPipeline(ctx, RunPipelineInput{
		PipelineExecID: exec.ID,
		Stages:         stages,
		SourceImage:    img.ThumbJPG,
		ConfigVars:     configVars,
		SkipStages:     skipStages,
	})

	exec.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}
	if err != nil {
		exec.Status = model.StatusFailed
		_ = u.repo.UpdatePipelineExecution(exec)
		return err
	}

	exec.Status = model.StatusSuccess
	_ = u.repo.UpdatePipelineExecution(exec)

	// Finalize: parse tags from last stage output and create records
	return u.finalizeTags(img.ID, exec.ID, result, stages)
}

var tagWordRe = regexp.MustCompile(`^[a-z]+$`)

// Banned words that should not be used as tags.
var bannedWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true,
	"image": true, "photo": true, "picture": true, "photograph": true,
	"tag": true, "tags": true, "output": true,
	"in": true, "on": true, "at": true, "to": true, "for": true,
	"of": true, "with": true, "by": true, "from": true, "and": true,
	"or": true, "not": true, "no": true, "but": true,
	"this": true, "that": true, "it": true, "its": true,
}

func (u *taggingUsecase) finalizeTags(imageID model.PrimaryKey, pipelineExecID model.PrimaryKey, result *ai.PipelineResult, stages []config.StageConfig) error {
	tagText := result.LastTextOutput()
	if tagText == "" {
		return fmt.Errorf("no text output from tagging pipeline")
	}

	// Find the step execution for the last stage (tagger)
	lastStageName := stages[len(stages)-1].Name
	steps, err := u.repo.FindStepsByPipelineExecution(pipelineExecID)
	if err != nil {
		return err
	}
	var taggerStepID model.PrimaryKey
	for _, s := range steps {
		if s.StageName == lastStageName && s.Status == model.StatusSuccess {
			taggerStepID = s.ID
		}
	}
	if taggerStepID == 0 {
		return fmt.Errorf("tagger step not found")
	}

	// Parse and normalize tags
	maxTags := u.cfg.AI.Tagging.MaxTags
	if maxTags <= 0 {
		maxTags = 15
	}

	words := strings.Fields(strings.ToLower(tagText))
	seen := make(map[string]bool)
	var tagIDs []model.PrimaryKey

	for _, w := range words {
		// Strip non-alpha chars
		w = strings.Trim(w, ".,;:!?\"'()-[]{}#*")
		if !tagWordRe.MatchString(w) || bannedWords[w] || len(w) < 2 {
			continue
		}
		if seen[w] {
			continue
		}
		seen[w] = true

		tag, err := u.repo.FindOrCreateTag(w)
		if err != nil {
			return err
		}
		tagIDs = append(tagIDs, tag.ID)
		if len(tagIDs) >= maxTags {
			break
		}
	}

	if len(tagIDs) == 0 {
		return nil
	}

	return u.repo.ReplaceImageTags(imageID, taggerStepID, tagIDs)
}

func (u *taggingUsecase) Reset(catalogKey string, imageID model.PrimaryKey) error {
	if imageID > 0 {
		return u.repo.ResetImageTagging(imageID)
	}
	return u.repo.ResetCatalogTagging(catalogKey)
}

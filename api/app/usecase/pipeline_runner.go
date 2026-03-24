package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/mikyk10/wisp/app/domain/ai"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/infra/llm"
)

// PipelineRunner executes a pipeline definition for a single input,
// recording each step in the database.
type PipelineRunner struct {
	cfg     *config.GlobalConfig
	repo    repository.AIRepository
	verbose bool
}

func NewPipelineRunner(cfg *config.GlobalConfig, repo repository.AIRepository) *PipelineRunner {
	return &PipelineRunner{cfg: cfg, repo: repo}
}

func (r *PipelineRunner) SetVerbose(v bool) { r.verbose = v }

// RunPipelineInput holds the inputs for a pipeline execution.
type RunPipelineInput struct {
	PipelineExecID   model.PrimaryKey
	Stages           []config.StageConfig
	SourceImage      []byte            // $source image data (may be nil)
	ConfigVars       map[string]any    // template config variables
	SkipStages       map[string]string // stage name → cached text output (skip execution, use cached)
	EmbeddedPrompts  map[string]string // stage name → embedded prompt path (fallback when stage.Prompt is empty)
}

// RunPipeline executes all stages of a pipeline sequentially.
func (r *PipelineRunner) RunPipeline(ctx context.Context, input RunPipelineInput) (*ai.PipelineResult, error) {
	result := &ai.PipelineResult{}
	stageOutputs := make(map[string]llm.StageOutput)

	timeout := time.Duration(r.cfg.AI.RequestTimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	maxRetries := r.cfg.AI.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for i, stage := range input.Stages {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Check if this stage should be skipped (cached output)
		if cachedText, ok := input.SkipStages[stage.Name]; ok {
			sr := ai.StageResult{
				StageName:  stage.Name,
				OutputType: "text",
				Text:       cachedText,
			}
			result.Stages = append(result.Stages, sr)
			stageOutputs[stage.Name] = llm.StageOutput{Text: cachedText}
			if r.verbose {
				slog.Info("pipeline: skipping cached stage", "stage", stage.Name)
			}
			continue
		}

		// Load prompt: external file path, or fall back to embedded
		embeddedName := ""
		if input.EmbeddedPrompts != nil {
			embeddedName = input.EmbeddedPrompts[stage.Name]
		}
		prompt, err := llm.ResolvePrompt(stage.Prompt, embeddedName)
		if err != nil {
			return nil, r.recordStepFailure(input.PipelineExecID, stage, i, "prompt_load_failed", err)
		}

		// Build template data
		var prev llm.StageOutput
		if i > 0 {
			prevName := input.Stages[i-1].Name
			prev = stageOutputs[prevName]
		}
		tmplData := llm.TemplateData{
			Prev:   prev,
			Stages: stageOutputs,
			Config: input.ConfigVars,
		}

		renderedPrompt, err := llm.RenderPrompt(prompt.Body, tmplData)
		if err != nil {
			return nil, r.recordStepFailure(input.PipelineExecID, stage, i, "prompt_render_failed", err)
		}

		// Resolve image input
		var imageInputs [][]byte
		if stage.ImageInput != "" {
			imgData, err := r.resolveImageInput(stage.ImageInput, input.SourceImage, stageOutputs)
			if err != nil {
				return nil, r.recordStepFailure(input.PipelineExecID, stage, i, "image_input_failed", err)
			}
			if imgData != nil {
				imageInputs = append(imageInputs, imgData)
			}
		}

		// Create StageExecutor based on output type + api_type
		executor, err := llm.NewStageExecutor(r.cfg.AI.Providers, prompt.Meta, stage.Output, timeout)
		if err != nil {
			return nil, r.recordStepFailure(input.PipelineExecID, stage, i, "executor_create_failed", err)
		}

		// Create step execution record
		step := &model.StepExecution{
			PipelineExecutionID: input.PipelineExecID,
			StageName:           stage.Name,
			StageIndex:          i,
			ProviderName:        prompt.Meta.Provider,
			ModelName:           prompt.Meta.Model,
			PromptHash:          prompt.Hash,
			Status:              model.StatusRunning,
			StartedAt:           time.Now(),
		}
		if err := r.repo.CreateStepExecution(step); err != nil {
			return nil, fmt.Errorf("create step execution: %w", err)
		}

		// Execute with retries
		var sr *ai.StageResult
		var execErr error

		for attempt := 0; attempt <= maxRetries; attempt++ {
			if ctx.Err() != nil {
				r.failStep(step, "context_cancelled", ctx.Err())
				return nil, ctx.Err()
			}

			start := time.Now()
			sr, execErr = executor.Execute(ctx, renderedPrompt, imageInputs)
			step.LatencyMs = time.Since(start).Milliseconds()

			if execErr == nil {
				break
			}

			if ctx.Err() != nil {
				r.failStep(step, "context_cancelled", ctx.Err())
				return nil, ctx.Err()
			}

			if attempt < maxRetries {
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				slog.Warn("pipeline: stage failed, retrying", "stage", stage.Name, "attempt", attempt+1, "err", execErr, "backoff", backoff)
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					r.failStep(step, "context_cancelled", ctx.Err())
					return nil, ctx.Err()
				}
			}
		}

		step.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}

		if execErr != nil {
			r.failStep(step, "execution_failed", execErr)
			return nil, fmt.Errorf("stage %q failed: %w", stage.Name, execErr)
		}

		// Success
		step.Status = model.StatusSuccess
		_ = r.repo.UpdateStepExecution(step)

		// Save output
		sr.StageName = stage.Name
		output := &model.StepOutput{
			StepExecutionID: step.ID,
			ContentType:     "text/plain",
		}
		if sr.OutputType == "text" {
			output.ContentText = &sr.Text
		} else {
			output.ContentBlob = sr.ImageData
			output.ContentType = sr.ContentType
		}
		_ = r.repo.CreateStepOutput(output)

		result.Stages = append(result.Stages, *sr)
		stageOutputs[stage.Name] = llm.StageOutput{
			Text:  sr.Text,
			Image: sr.ImageData,
		}

		if r.verbose {
			attrs := []any{"stage", stage.Name, "output_type", sr.OutputType, "latency_ms", step.LatencyMs}
			if sr.OutputType == "text" {
				attrs = append(attrs, "output", sr.Text)
			} else {
				attrs = append(attrs, "output_bytes", len(sr.ImageData))
			}
			slog.Info("pipeline: stage completed", attrs...)
		}
	}

	return result, nil
}

func (r *PipelineRunner) resolveImageInput(ref string, sourceImage []byte, stageOutputs map[string]llm.StageOutput) ([]byte, error) {
	if ref == "$source" {
		if sourceImage == nil {
			return nil, fmt.Errorf("$source referenced but no source image provided")
		}
		return sourceImage, nil
	}
	out, ok := stageOutputs[ref]
	if !ok {
		return nil, fmt.Errorf("image_input references unknown stage %q", ref)
	}
	if out.Image == nil {
		return nil, fmt.Errorf("stage %q has no image output", ref)
	}
	return out.Image, nil
}

func (r *PipelineRunner) failStep(step *model.StepExecution, errorCode string, err error) {
	step.Status = model.StatusFailed
	step.ErrorCode = errorCode
	step.ErrorMessage = err.Error()
	step.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}
	_ = r.repo.UpdateStepExecution(step)
}

func (r *PipelineRunner) recordStepFailure(pipelineExecID model.PrimaryKey, stage config.StageConfig, index int, errorCode string, err error) error {
	step := &model.StepExecution{
		PipelineExecutionID: pipelineExecID,
		StageName:           stage.Name,
		StageIndex:          index,
		Status:              model.StatusFailed,
		StartedAt:           time.Now(),
		FinishedAt:          sql.NullTime{Time: time.Now(), Valid: true},
		ErrorCode:           errorCode,
		ErrorMessage:        err.Error(),
	}
	_ = r.repo.CreateStepExecution(step)
	return fmt.Errorf("stage %q: %s: %w", stage.Name, errorCode, err)
}

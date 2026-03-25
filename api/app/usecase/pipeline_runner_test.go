package usecase

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/ai"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/infra/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type mockAIRepo struct {
	steps   []*model.StepExecution
	outputs []*model.StepOutput
}

func (m *mockAIRepo) CreatePipelineExecution(exec *model.PipelineExecution) error { return nil }
func (m *mockAIRepo) UpdatePipelineExecution(exec *model.PipelineExecution) error { return nil }
func (m *mockAIRepo) DeletePipelineExecution(id model.PrimaryKey) error           { return nil }
func (m *mockAIRepo) CreateStepExecution(step *model.StepExecution) error {
	m.steps = append(m.steps, step)
	return nil
}
func (m *mockAIRepo) UpdateStepExecution(step *model.StepExecution) error { return nil }
func (m *mockAIRepo) FindStepsByPipelineExecution(id model.PrimaryKey) ([]*model.StepExecution, error) {
	return nil, nil
}
func (m *mockAIRepo) CreateStepOutput(out *model.StepOutput) error {
	m.outputs = append(m.outputs, out)
	return nil
}
func (m *mockAIRepo) FindStepOutputByStepID(id model.PrimaryKey) (*model.StepOutput, error) {
	return nil, nil
}
func (m *mockAIRepo) FindOrCreateTag(name string) (*model.Tag, error) { return nil, nil }
func (m *mockAIRepo) ReplaceImageTags(model.PrimaryKey, model.PrimaryKey, []model.PrimaryKey) error {
	return nil
}
func (m *mockAIRepo) HasImageTags(model.PrimaryKey) (bool, error)                       { return false, nil }
func (m *mockAIRepo) FindTagNamesByImageID(model.PrimaryKey) ([]string, error)           { return nil, nil }
func (m *mockAIRepo) FindTagsByCatalog(string) ([]string, error)                         { return nil, nil }
func (m *mockAIRepo) FindImageIDsByTags(string, []string) ([]model.PrimaryKey, error)    { return nil, nil }
func (m *mockAIRepo) CreateCacheEntry(*model.GenerationCacheEntry) error                 { return nil }
func (m *mockAIRepo) CountCacheEntries(string) (int64, error)                            { return 0, nil }
func (m *mockAIRepo) ListCacheEntries(string) ([]*model.GenerationCacheEntry, error)     { return nil, nil }
func (m *mockAIRepo) FindCacheEntryByID(model.PrimaryKey) (*model.GenerationCacheEntry, error) {
	return nil, nil
}
func (m *mockAIRepo) FindRandomCacheEntry(string) (*model.GenerationCacheEntry, error) {
	return nil, nil
}
func (m *mockAIRepo) EvictOldestCacheEntries(string, int) error                           { return nil }
func (m *mockAIRepo) FindImagesForTagging(string, int) ([]*model.Image, error)            { return nil, nil }
func (m *mockAIRepo) FindAllImages(string, int) ([]*model.Image, error)                   { return nil, nil }
func (m *mockAIRepo) FindLatestSuccessfulStep(model.PrimaryKey, string) (*model.StepExecution, error) {
	return nil, nil
}
func (m *mockAIRepo) FindRandomImage(string) (*model.Image, error) { return nil, nil }
func (m *mockAIRepo) ResetImageTagging(model.PrimaryKey) error     { return nil }
func (m *mockAIRepo) ResetCatalogTagging(string) error             { return nil }
func (m *mockAIRepo) CleanFailedExecutions(string) error           { return nil }

type mockExecutor struct {
	result *ai.StageResult
	err    error
	calls  int
}

func (m *mockExecutor) Execute(ctx context.Context, prompt string, images [][]byte) (*ai.StageResult, error) {
	m.calls++
	return m.result, m.err
}

// --- Helpers ---

func newTestRunner(repo *mockAIRepo) *PipelineRunner {
	cfg := &config.GlobalConfig{}
	cfg.AI.RequestTimeoutSec = 10
	cfg.AI.MaxRetries = 1
	cfg.AI.Providers = map[string]config.AIProviderConfig{}
	return NewPipelineRunner(cfg, repo)
}

func newTestRunnerWithExecutor(repo *mockAIRepo, exec ai.StageExecutor) *PipelineRunner {
	runner := newTestRunner(repo)
	runner.executorFactory = func(_ map[string]config.AIProviderConfig, _ llm.PromptMeta, _ string, _ time.Duration) (ai.StageExecutor, error) {
		return exec, nil
	}
	return runner
}

func writeTestPrompt(t *testing.T, path, provider, modelName, apiType, body string) {
	t.Helper()
	content := fmt.Sprintf("---\nprovider: %s\nmodel: %s\napi_type: %s\n---\n%s", provider, modelName, apiType, body)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

// --- resolveImageInput tests ---

func TestResolveImageInput_Source(t *testing.T) {
	r := &PipelineRunner{}
	sourceImg := []byte("fake-image-data")

	result, err := r.resolveImageInput("_source", sourceImg, nil)
	require.NoError(t, err)
	assert.Equal(t, sourceImg, result)
}

func TestResolveImageInput_SourceNil(t *testing.T) {
	r := &PipelineRunner{}

	_, err := r.resolveImageInput("_source", nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "_source referenced but no source image provided")
}

func TestResolveImageInput_StageReference(t *testing.T) {
	r := &PipelineRunner{}
	imgData := []byte("stage-output-image")
	stageOutputs := map[string]llm.StageOutput{
		"generate": {Image: imgData},
	}

	result, err := r.resolveImageInput("generate", nil, stageOutputs)
	require.NoError(t, err)
	assert.Equal(t, imgData, result)
}

func TestResolveImageInput_UnknownStage(t *testing.T) {
	r := &PipelineRunner{}
	stageOutputs := map[string]llm.StageOutput{}

	_, err := r.resolveImageInput("nonexistent", nil, stageOutputs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown stage")
}

func TestResolveImageInput_StageNoImage(t *testing.T) {
	r := &PipelineRunner{}
	stageOutputs := map[string]llm.StageOutput{
		"text-stage": {Text: "only text"},
	}

	_, err := r.resolveImageInput("text-stage", nil, stageOutputs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has no image output")
}

// --- RunPipeline tests ---

func TestRunPipeline_EmptyStages(t *testing.T) {
	repo := &mockAIRepo{}
	runner := newTestRunner(repo)

	result, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		Stages: []config.StageConfig{},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Stages)
}

func TestRunPipeline_SkipCachedStage(t *testing.T) {
	repo := &mockAIRepo{}
	runner := newTestRunner(repo)

	result, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		Stages: []config.StageConfig{
			{Name: "descriptor", Output: "text", Prompt: "dummy"},
		},
		SkipStages: map[string]string{
			"descriptor": "cached description text",
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Stages, 1)
	assert.Equal(t, "descriptor", result.Stages[0].StageName)
	assert.Equal(t, "cached description text", result.Stages[0].Text)
	assert.Equal(t, "text", result.Stages[0].OutputType)
}

func TestRunPipeline_MissingPromptPath(t *testing.T) {
	repo := &mockAIRepo{}
	runner := newTestRunner(repo)

	_, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 1,
		Stages: []config.StageConfig{
			{Name: "test", Output: "text", Prompt: ""},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no prompt path configured")
}

func TestRunPipeline_PromptFileNotFound(t *testing.T) {
	repo := &mockAIRepo{}
	runner := newTestRunner(repo)

	_, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 1,
		Stages: []config.StageConfig{
			{Name: "test", Output: "text", Prompt: "/nonexistent/prompt.md"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prompt_load_failed")
}

func TestRunPipeline_ContextCancelled(t *testing.T) {
	repo := &mockAIRepo{}
	runner := newTestRunner(repo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runner.RunPipeline(ctx, RunPipelineInput{
		Stages: []config.StageConfig{
			{Name: "test", Output: "text", Prompt: "dummy"},
		},
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRunPipeline_SingleTextStage(t *testing.T) {
	repo := &mockAIRepo{}
	exec := &mockExecutor{
		result: &ai.StageResult{OutputType: "text", Text: "generated text"},
	}
	runner := newTestRunnerWithExecutor(repo, exec)

	dir := t.TempDir()
	promptPath := dir + "/test.md"
	writeTestPrompt(t, promptPath, "openai", "gpt-4o", "chat", "tell me something")

	result, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 1,
		Stages: []config.StageConfig{
			{Name: "brainstorm", Output: "text", Prompt: promptPath},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Stages, 1)
	assert.Equal(t, "brainstorm", result.Stages[0].StageName)
	assert.Equal(t, "text", result.Stages[0].OutputType)
	assert.Equal(t, "generated text", result.Stages[0].Text)
	assert.Equal(t, 1, exec.calls)
}

func TestRunPipeline_MultiStageWithPrevOutput(t *testing.T) {
	repo := &mockAIRepo{}
	callCount := 0
	exec := &mockExecutor{}
	// Override Execute to return different results per call
	runner := newTestRunner(repo)
	runner.executorFactory = func(_ map[string]config.AIProviderConfig, _ llm.PromptMeta, _ string, _ time.Duration) (ai.StageExecutor, error) {
		return &sequentialExecutor{
			results: []*ai.StageResult{
				{OutputType: "text", Text: "a wild concept"},
				{OutputType: "text", Text: "a refined concept"},
			},
			callCount: &callCount,
		}, nil
	}
	_ = exec // unused, using sequentialExecutor instead

	dir := t.TempDir()
	brainstormPath := dir + "/brainstorm.md"
	refinePath := dir + "/refine.md"
	writeTestPrompt(t, brainstormPath, "openai", "gpt-4o", "chat", "brainstorm something")
	writeTestPrompt(t, refinePath, "openai", "gpt-4o", "chat", "refine: {{.prev.output}}")

	result, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 1,
		Stages: []config.StageConfig{
			{Name: "brainstorm", Output: "text", Prompt: brainstormPath},
			{Name: "refine", Output: "text", Prompt: refinePath},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Stages, 2)
	assert.Equal(t, "a wild concept", result.Stages[0].Text)
	assert.Equal(t, "a refined concept", result.Stages[1].Text)
	assert.Equal(t, 2, callCount)
}

func TestRunPipeline_ExecutorFailure(t *testing.T) {
	repo := &mockAIRepo{}
	exec := &mockExecutor{
		err: fmt.Errorf("API call failed"),
	}
	runner := newTestRunnerWithExecutor(repo, exec)

	dir := t.TempDir()
	promptPath := dir + "/test.md"
	writeTestPrompt(t, promptPath, "openai", "gpt-4o", "chat", "test")

	_, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 1,
		Stages: []config.StageConfig{
			{Name: "test", Output: "text", Prompt: promptPath},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API call failed")
	// MaxRetries=1, so 1 initial + 1 retry = 2 calls
	assert.Equal(t, 2, exec.calls)
}

func TestRunPipeline_ImageStageWithSource(t *testing.T) {
	repo := &mockAIRepo{}
	exec := &mockExecutor{
		result: &ai.StageResult{
			OutputType:  "image",
			ImageData:   []byte("fake-png-data"),
			ContentType: "image/png",
		},
	}
	runner := newTestRunnerWithExecutor(repo, exec)

	dir := t.TempDir()
	promptPath := dir + "/gen.md"
	writeTestPrompt(t, promptPath, "openai", "gpt-image-1", "image_edit", "stylize this")

	result, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 1,
		Stages: []config.StageConfig{
			{Name: "stylize", Output: "image", Prompt: promptPath, ImageInput: "_source"},
		},
		SourceImage: []byte("source-image-bytes"),
	})
	require.NoError(t, err)
	require.Len(t, result.Stages, 1)
	assert.Equal(t, "image", result.Stages[0].OutputType)
	assert.Equal(t, []byte("fake-png-data"), result.Stages[0].ImageData)
}

func TestRunPipeline_RetryThenSucceed(t *testing.T) {
	repo := &mockAIRepo{}
	callCount := 0
	runner := newTestRunner(repo)
	runner.executorFactory = func(_ map[string]config.AIProviderConfig, _ llm.PromptMeta, _ string, _ time.Duration) (ai.StageExecutor, error) {
		return &retryExecutor{
			failCount: 1,
			result:    &ai.StageResult{OutputType: "text", Text: "success after retry"},
			callCount: &callCount,
		}, nil
	}

	dir := t.TempDir()
	promptPath := dir + "/test.md"
	writeTestPrompt(t, promptPath, "openai", "gpt-4o", "chat", "test")

	result, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 1,
		Stages: []config.StageConfig{
			{Name: "test", Output: "text", Prompt: promptPath},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Stages, 1)
	assert.Equal(t, "success after retry", result.Stages[0].Text)
	assert.Equal(t, 2, callCount) // 1 fail + 1 success
}

// --- Helper executors ---

type sequentialExecutor struct {
	results   []*ai.StageResult
	callCount *int
}

func (e *sequentialExecutor) Execute(ctx context.Context, prompt string, images [][]byte) (*ai.StageResult, error) {
	idx := *e.callCount
	*e.callCount++
	if idx < len(e.results) {
		return e.results[idx], nil
	}
	return nil, fmt.Errorf("unexpected call %d", idx)
}

type retryExecutor struct {
	failCount int
	result    *ai.StageResult
	callCount *int
}

func (e *retryExecutor) Execute(ctx context.Context, prompt string, images [][]byte) (*ai.StageResult, error) {
	idx := *e.callCount
	*e.callCount++
	if idx < e.failCount {
		return nil, fmt.Errorf("transient error (attempt %d)", idx)
	}
	return e.result, nil
}

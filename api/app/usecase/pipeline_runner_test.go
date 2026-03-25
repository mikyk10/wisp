package usecase

import (
	"context"
	"fmt"
	"os"
	"testing"

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
func (m *mockAIRepo) FindOrCreateTag(name string) (*model.Tag, error)         { return nil, nil }
func (m *mockAIRepo) ReplaceImageTags(model.PrimaryKey, model.PrimaryKey, []model.PrimaryKey) error {
	return nil
}
func (m *mockAIRepo) HasImageTags(model.PrimaryKey) (bool, error)                { return false, nil }
func (m *mockAIRepo) FindTagNamesByImageID(model.PrimaryKey) ([]string, error)    { return nil, nil }
func (m *mockAIRepo) FindTagsByCatalog(string) ([]string, error)                  { return nil, nil }
func (m *mockAIRepo) FindImageIDsByTags(string, []string) ([]model.PrimaryKey, error) {
	return nil, nil
}
func (m *mockAIRepo) CreateCacheEntry(*model.GenerationCacheEntry) error  { return nil }
func (m *mockAIRepo) CountCacheEntries(string) (int64, error)             { return 0, nil }
func (m *mockAIRepo) ListCacheEntries(string) ([]*model.GenerationCacheEntry, error) {
	return nil, nil
}
func (m *mockAIRepo) FindCacheEntryByID(model.PrimaryKey) (*model.GenerationCacheEntry, error) {
	return nil, nil
}
func (m *mockAIRepo) FindRandomCacheEntry(string) (*model.GenerationCacheEntry, error) {
	return nil, nil
}
func (m *mockAIRepo) EvictOldestCacheEntries(string, int) error                   { return nil }
func (m *mockAIRepo) FindImagesForTagging(string, int) ([]*model.Image, error)    { return nil, nil }
func (m *mockAIRepo) FindAllImages(string, int) ([]*model.Image, error)           { return nil, nil }
func (m *mockAIRepo) FindLatestSuccessfulStep(model.PrimaryKey, string) (*model.StepExecution, error) {
	return nil, nil
}
func (m *mockAIRepo) FindRandomImage(string) (*model.Image, error)   { return nil, nil }
func (m *mockAIRepo) ResetImageTagging(model.PrimaryKey) error       { return nil }
func (m *mockAIRepo) ResetCatalogTagging(string) error               { return nil }
func (m *mockAIRepo) CleanFailedExecutions(string) error             { return nil }

type mockExecutor struct {
	result *ai.StageResult
	err    error
	calls  int
}

func (m *mockExecutor) Execute(ctx context.Context, prompt string, images [][]byte) (*ai.StageResult, error) {
	m.calls++
	return m.result, m.err
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

func newTestRunner(repo *mockAIRepo) *PipelineRunner {
	cfg := &config.GlobalConfig{}
	cfg.AI.RequestTimeoutSec = 10
	cfg.AI.MaxRetries = 1
	cfg.AI.Providers = map[string]config.AIProviderConfig{}
	return NewPipelineRunner(cfg, repo)
}

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
	cancel() // cancel immediately

	_, err := runner.RunPipeline(ctx, RunPipelineInput{
		Stages: []config.StageConfig{
			{Name: "test", Output: "text", Prompt: "dummy"},
		},
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRunPipeline_ExecutorFailure(t *testing.T) {
	repo := &mockAIRepo{}
	runner := newTestRunner(repo)

	// Write a temporary prompt file
	dir := t.TempDir()
	promptPath := dir + "/test.md"
	writeTestPrompt(t, promptPath, "openai", "gpt-4o", "chat", "test prompt")

	// Add a fake provider so executor can be created
	runner.cfg.AI.Providers["openai"] = config.AIProviderConfig{
		Endpoint: "http://localhost:9999",
		APIKey:   "test-key",
	}

	_, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 1,
		Stages: []config.StageConfig{
			{Name: "test", Output: "text", Prompt: promptPath},
		},
	})
	// Will fail because the endpoint is not real, but the error should be an execution failure
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test")
}

func writeTestPrompt(t *testing.T, path, provider, modelName, apiType, body string) {
	t.Helper()
	content := fmt.Sprintf("---\nprovider: %s\nmodel: %s\napi_type: %s\n---\n%s", provider, modelName, apiType, body)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

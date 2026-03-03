package usecase_test

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"
	"github.com/mikyk10/wisp/app/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- Mock clients ---

type mockDescriptorClient struct {
	modelName string
	response  string
	err       error
	callCount int
}

func (m *mockDescriptorClient) Describe(_ context.Context, _ []byte) (string, error) {
	m.callCount++
	return m.response, m.err
}
func (m *mockDescriptorClient) PromptModel() string { return m.modelName }

type mockTaggerClient struct {
	modelName string
	tags      []string
	err       error
	callCount int
}

func (m *mockTaggerClient) Tag(_ context.Context, _ string) ([]string, error) {
	m.callCount++
	return m.tags, m.err
}
func (m *mockTaggerClient) PromptModel() string { return m.modelName }

// --- Test setup ---

type taggingTestEnv struct {
	conn        *gorm.DB
	taggingRepo repository.TaggingRepository
	imageRepo   repository.ImageRepository
	desc        *mockDescriptorClient
	tagger      *mockTaggerClient
	uc          usecase.TaggingPipelineUsecase
}

func newTaggingTestEnv(t *testing.T) *taggingTestEnv {
	t.Helper()

	conn, err := infra.NewSqliteConnection("", true)
	require.NoError(t, err)

	sqlDB, err := conn.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, conn.AutoMigrate(
		&model.Image{},
		&model.Tag{},
		&model.ImageTag{},
		&model.AIRun{},
		&model.AIOutput{},
	))

	taggingRepo := infraRepo.NewTaggingRepositoryImpl(conn)
	imageRepo := infraRepo.NewImageRepositoryImpl(conn)

	desc := &mockDescriptorClient{modelName: "desc-model", response: "a photo of a dog in a park"}
	tagger := &mockTaggerClient{modelName: "tag-model", tags: []string{"dog", "park", "outdoor"}}

	cfg := &config.GlobalConfig{}
	cfg.AI.Workers = 1
	cfg.AI.MaxTags = 15
	cfg.AI.MaxRetries = 1
	cfg.AI.RequestTimeoutSec = 5

	uc := usecase.NewTaggingPipelineUsecase(cfg, taggingRepo, desc, tagger)
	return &taggingTestEnv{
		conn:        conn,
		taggingRepo: taggingRepo,
		imageRepo:   imageRepo,
		desc:        desc,
		tagger:      tagger,
		uc:          uc,
	}
}

// makeThumb returns a minimal valid JPEG byte slice.
func makeThumb(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

// insertImage upserts a test image and returns it with the DB-assigned ID populated.
func (e *taggingTestEnv) insertImage(t *testing.T, catalogKey, srcHash string, thumb []byte) *model.Image {
	t.Helper()
	img := &model.Image{
		CatalogKey: catalogKey,
		Src:        "/test/" + srcHash + ".jpg",
		SrcHash:    srcHash,
		ThumbJPG:   thumb,
		Rnd:        rand.Float64(),
	}
	require.NoError(t, e.imageRepo.UpsertActiveImage(img))
	// Reload to get the auto-incremented ID.
	reloaded := &model.Image{}
	require.NoError(t, e.conn.Where("catalog_key = ? AND src_hash = ?", catalogKey, srcHash).First(reloaded).Error)
	return reloaded
}

// preTagImage creates a tagging run + image_tags record to simulate an already-tagged image.
func (e *taggingTestEnv) preTagImage(t *testing.T, img *model.Image) {
	t.Helper()
	run := &model.AIRun{
		ImageID:   img.ID,
		Stage:     model.AIRunStageTagging,
		ModelName: "pre",
		Status:    model.AIRunStatusSuccess,
		StartedAt: time.Now(),
		InputHash: img.SrcHash,
	}
	require.NoError(t, e.taggingRepo.CreateAIRun(run))
	tag, err := e.taggingRepo.FindOrCreateTag("existingtag")
	require.NoError(t, err)
	require.NoError(t, e.taggingRepo.ReplaceImageTags(img.ID, run.ID, []model.PrimaryKey{tag.ID}))
}

// preDescribeImage creates a successful descriptor run + output record.
func (e *taggingTestEnv) preDescribeImage(t *testing.T, img *model.Image, description string) {
	t.Helper()
	run := &model.AIRun{
		ImageID:   img.ID,
		Stage:     model.AIRunStageDescriptor,
		ModelName: "desc-model",
		Status:    model.AIRunStatusSuccess,
		StartedAt: time.Now(),
		InputHash: img.SrcHash,
	}
	require.NoError(t, e.taggingRepo.CreateAIRun(run))
	require.NoError(t, e.taggingRepo.CreateAIOutput(&model.AIOutput{
		RunID:       run.ID,
		ContentText: description,
	}))
}

// --- Tests ---

// TestTaggingPipeline_NormalMode_SkipsAlreadyTagged verifies that images with existing tags are skipped.
func TestTaggingPipeline_NormalMode_SkipsAlreadyTagged(t *testing.T) {
	e := newTaggingTestEnv(t)

	img := e.insertImage(t, "cat1", "hash1", makeThumb(t))
	e.preTagImage(t, img)

	require.NoError(t, e.uc.Run(context.Background(), usecase.TaggingRunOptions{
		CatalogKey: "cat1",
		Workers:    1,
	}))

	assert.Equal(t, 0, e.desc.callCount, "descriptor should not run for already-tagged image")
	assert.Equal(t, 0, e.tagger.callCount, "tagger should not run for already-tagged image")
}

// TestTaggingPipeline_NormalMode_UsesExistingDescriptor verifies that when a descriptor run exists
// but no tags, only the tagger stage runs.
func TestTaggingPipeline_NormalMode_UsesExistingDescriptor(t *testing.T) {
	e := newTaggingTestEnv(t)

	img := e.insertImage(t, "cat1", "hash1", makeThumb(t))
	e.preDescribeImage(t, img, "a cat on a sofa")

	require.NoError(t, e.uc.Run(context.Background(), usecase.TaggingRunOptions{
		CatalogKey: "cat1",
		Workers:    1,
	}))

	assert.Equal(t, 0, e.desc.callCount, "descriptor should not run when output already exists")
	assert.Equal(t, 1, e.tagger.callCount, "tagger should run once")
}

// TestTaggingPipeline_FullRun_BothStages verifies that a fresh image goes through both stages.
func TestTaggingPipeline_FullRun_BothStages(t *testing.T) {
	e := newTaggingTestEnv(t)

	e.insertImage(t, "cat1", "hash1", makeThumb(t))

	require.NoError(t, e.uc.Run(context.Background(), usecase.TaggingRunOptions{
		CatalogKey: "cat1",
		Workers:    1,
	}))

	assert.Equal(t, 1, e.desc.callCount, "descriptor should run once for fresh image")
	assert.Equal(t, 1, e.tagger.callCount, "tagger should run once for fresh image")

	// Verify tags are stored.
	img := &model.Image{}
	require.NoError(t, e.conn.Where("src_hash = ?", "hash1").First(img).Error)
	hasT, err := e.taggingRepo.HasImageTags(img.ID)
	require.NoError(t, err)
	assert.True(t, hasT, "image should have tags after full run")
}

// TestTaggingPipeline_Rebuild_RetaggersAll verifies that --rebuild re-processes already-tagged images.
func TestTaggingPipeline_Rebuild_RetaggersAll(t *testing.T) {
	e := newTaggingTestEnv(t)

	img := e.insertImage(t, "cat1", "hash1", makeThumb(t))
	e.preTagImage(t, img)

	require.NoError(t, e.uc.Run(context.Background(), usecase.TaggingRunOptions{
		CatalogKey: "cat1",
		Workers:    1,
		Rebuild:    true,
	}))

	assert.Equal(t, 1, e.desc.callCount, "descriptor should run on rebuild")
	assert.Equal(t, 1, e.tagger.callCount, "tagger should run on rebuild")
}

// TestTaggingPipeline_RebuildStage2_UsesExistingDescriptor verifies --rebuild --stage=2 reuses descriptor.
func TestTaggingPipeline_RebuildStage2_UsesExistingDescriptor(t *testing.T) {
	e := newTaggingTestEnv(t)

	img := e.insertImage(t, "cat1", "hash1", makeThumb(t))
	e.preDescribeImage(t, img, "a dog in a field")

	require.NoError(t, e.uc.Run(context.Background(), usecase.TaggingRunOptions{
		CatalogKey: "cat1",
		Workers:    1,
		Rebuild:    true,
		Stage:      2,
	}))

	assert.Equal(t, 0, e.desc.callCount, "descriptor should be reused in stage=2")
	assert.Equal(t, 1, e.tagger.callCount, "tagger should run in stage=2")
}

// TestTaggingPipeline_EmptyThumbnail_RecordsInputMissing verifies that an image with no thumbnail
// gets an input_missing error recorded and the pipeline continues without crashing.
func TestTaggingPipeline_EmptyThumbnail_RecordsInputMissing(t *testing.T) {
	e := newTaggingTestEnv(t)

	// Insert image with empty thumbnail.
	img := &model.Image{
		CatalogKey: "cat1",
		Src:        "/test/empty.jpg",
		SrcHash:    "emptyHash",
		ThumbJPG:   []byte{},
		Rnd:        0.1,
	}
	require.NoError(t, e.imageRepo.UpsertActiveImage(img))
	require.NoError(t, e.conn.Where("src_hash = ?", "emptyHash").First(img).Error)

	require.NoError(t, e.uc.Run(context.Background(), usecase.TaggingRunOptions{
		CatalogKey: "cat1",
		Workers:    1,
		Rebuild:    true,
	}))

	// Clients should not have been called.
	assert.Equal(t, 0, e.desc.callCount)
	assert.Equal(t, 0, e.tagger.callCount)

	// An ai_run with error_code=input_missing should exist.
	var runs []model.AIRun
	require.NoError(t, e.conn.Where("image_id = ? AND error_code = ?", img.ID, usecase.ErrCodeInputMissing).Find(&runs).Error)
	assert.Len(t, runs, 1, "expected one input_missing run record")
}

// TestTaggingPipeline_DryRun_SkipsAll verifies that --dry-run does not call any LLM client.
func TestTaggingPipeline_DryRun_SkipsAll(t *testing.T) {
	e := newTaggingTestEnv(t)

	e.insertImage(t, "cat1", "hash1", makeThumb(t))

	require.NoError(t, e.uc.Run(context.Background(), usecase.TaggingRunOptions{
		CatalogKey: "cat1",
		Workers:    1,
		DryRun:     true,
	}))

	assert.Equal(t, 0, e.desc.callCount, "descriptor should not run in dry-run mode")
	assert.Equal(t, 0, e.tagger.callCount, "tagger should not run in dry-run mode")
}

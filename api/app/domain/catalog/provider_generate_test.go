package catalog

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAIRepoForGenerate implements only the methods needed by provider_generate.
type mockAIRepoForGenerate struct {
	repository.AIRepository
	entry *model.GenerationCacheEntry
	err   error
}

func (m *mockAIRepoForGenerate) FindRandomCacheEntry(catalogKey string) (*model.GenerationCacheEntry, error) {
	return m.entry, m.err
}

func createTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.Set(5, 5, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestImageGenerateProvider_Resolve_WithCachedEntry(t *testing.T) {
	pngData := createTestPNG(t)
	repo := &mockAIRepoForGenerate{
		entry: &model.GenerationCacheEntry{ImageData: pngData},
	}

	provider := NewImageGenerateProvider("test-catalog", repo)
	loader, err := provider.Resolve()
	require.NoError(t, err)
	require.NotNil(t, loader)

	img, meta, err := loader.Load()
	require.NoError(t, err)
	assert.NotNil(t, img)
	assert.NotNil(t, meta)
	assert.Equal(t, 10, img.Bounds().Dx())
	assert.Equal(t, 10, img.Bounds().Dy())
}

func TestImageGenerateProvider_Resolve_EmptyCache(t *testing.T) {
	repo := &mockAIRepoForGenerate{entry: nil, err: nil}

	provider := NewImageGenerateProvider("empty-catalog", repo)
	loader, err := provider.Resolve()
	assert.Error(t, err)
	assert.Nil(t, loader)
	assert.Contains(t, err.Error(), "no cached images available")
	assert.Contains(t, err.Error(), "empty-catalog")
}

func TestImageGenerateProvider_Resolve_RepoError(t *testing.T) {
	repo := &mockAIRepoForGenerate{err: assert.AnError}

	provider := NewImageGenerateProvider("test", repo)
	loader, err := provider.Resolve()
	assert.Error(t, err)
	assert.Nil(t, loader)
}

func TestGeneratedImageLoader_InvalidImageData(t *testing.T) {
	loader := &generatedImageLoader{
		entry: &model.GenerationCacheEntry{ImageData: []byte("not an image")},
	}

	img, _, err := loader.Load()
	assert.Error(t, err)
	assert.Nil(t, img)
}

func TestGeneratedImageLoader_GetSourcePath(t *testing.T) {
	loader := &generatedImageLoader{
		entry: &model.GenerationCacheEntry{},
	}
	assert.Equal(t, "", loader.GetSourcePath())
}

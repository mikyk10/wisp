package config

// ImageGenerateProviderConfig holds configuration for an AI-generated image catalog.
type ImageGenerateProviderConfig struct {
	CacheDepth    int             `yaml:"cache_depth"`
	EvictCount    int             `yaml:"evict_count"`
	SourceCatalog string          `yaml:"source_catalog"`
	Pipeline      PipelineConfig  `yaml:"pipeline"`
}

func (ImageGenerateProviderConfig) providerConfigTag() {}

// PipelineConfig defines a sequence of stages.
type PipelineConfig struct {
	Stages []StageConfig `yaml:"stages"`
}

// StageConfig defines a single pipeline stage.
type StageConfig struct {
	Name       string `yaml:"name"`
	Output     string `yaml:"output"`      // "text" or "image"
	Prompt     string `yaml:"prompt"`      // path to prompt file
	ImageInput string `yaml:"image_input"` // "$source" or stage name
}

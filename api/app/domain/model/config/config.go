package config

import "log/slog"

const (
	ImageFileProviderType       string = "file"
	ImageHTTPProviderType       string = "http"
	ImagePlaywrightProviderType string = "playwright-go"
	ImageLuaProviderType        string = "lua"
	ImageColorbarProviderType   string = "colorbar"
)

type DisplayOrientation int

const (
	DisplayOrientationNone = DisplayOrientation(iota)
	DisplayOrientationLandscape
	DisplayOrientationPortrait
)

func NewDisplayOrientation(word string) DisplayOrientation {
	switch word {
	case "landscape":
		return DisplayOrientationLandscape
	case "portrait":
		return DisplayOrientationPortrait
	default:
		return DisplayOrientationLandscape
	}
}

type ConfigLoader interface {
	LoadConfig() (*GlobalConfig, *ServiceConfig, error)
}

// AIProviderConfig holds connection details for a single LLM provider.
type AIProviderConfig struct {
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"api_key"` //nolint:gosec // G117: struct field name, not an actual secret
}

// GlobalConfig holds application-wide configuration.
type GlobalConfig struct {
	LogLevel slog.Level `yaml:"log_level"`
	Port     int        `yaml:"port"`
	Env      string     `env:"ENV"`
	Database struct {
		Driver        string `yaml:"driver"`
		DSN           string `yaml:"dsn" env:"DB_DEFAULT_DSN"`
		DriverOptions struct {
			Sqlite3 struct {
			}
		}
	}
	AI struct {
		Providers            map[string]AIProviderConfig `yaml:"providers"`
		DescriptorPromptPath string                      `yaml:"descriptor_prompt_path" env:"WISP_AI_DESCRIPTOR_PROMPT_PATH"`
		TaggerPromptPath     string                      `yaml:"tagger_prompt_path"     env:"WISP_AI_TAGGER_PROMPT_PATH"`
		Workers              int                         `yaml:"workers"                env:"WISP_AI_WORKERS"`
		RequestTimeoutSec    int                         `yaml:"request_timeout_sec"    env:"WISP_AI_REQUEST_TIMEOUT_SEC"`
		MaxTags              int                         `yaml:"max_tags"               env:"WISP_AI_MAX_TAGS"`
		MaxRetries           int                         `yaml:"max_retries"            env:"WISP_AI_MAX_RETRIES"`
	} `yaml:"ai"`
}

// ServiceConfig holds catalog and display configuration.
type ServiceConfig struct {
	Catalog  map[string]*ImageProviderConfig
	Displays map[string]*DisplayConfig
}

// ProviderConfig is a marker interface implemented by each provider configuration type.
// Used to perform type switches safely.
type ProviderConfig interface {
	providerConfigTag()
}

// ImageProviderConfig holds configuration for a catalog entry (a collection of images).
type ImageProviderConfig struct {
	Key    string
	Config ProviderConfig
}

// DisplayConfig holds display configuration.
type DisplayConfig struct {
	Name                 string
	Key                  string
	ApiVersion           string
	DisplayModel         string
	Orientation          DisplayOrientation
	Flip                 bool
	ShowTimestamp        bool
	ColorReduction       ColorReduction
	Catalog              []*AssociatedImageProviders
	ImageProcessors      []*ImageProcessorConfig
	SleepDurationSeconds int
}

type ColorReductionType = string

type ColorReduction struct {
	Type     string
	Size     uint    // only for Bayer
	Strength float32 // only for Bayer
}

const (
	ColorReductionTypeSimple         ColorReductionType = "simple"
	ColorReductionTypeBayer          ColorReductionType = "bayer"
	ColorReductionTypeFloydSteinberg ColorReductionType = "floysteinberg"
	ColorReductionTypeSierra3        ColorReductionType = "sierra3"
)

// ImageProcessorType lists the image processor types that can be specified in config files.
// Processors used for pre/post-processing are internal and are not enumerated here.
type ImageProcessorType = string

const (
	ImageProcessorTypeBlur       ImageProcessorType = "blur"
	ImageProcessorTypeBrightness ImageProcessorType = "brightness"
	ImageProcessorTypeContrast   ImageProcessorType = "contrast"
	ImageProcessorTypeGamma      ImageProcessorType = "gamma"
	ImageProcessorTypeHue        ImageProcessorType = "hue"
	ImageProcessorTypeSaturation     ImageProcessorType = "saturation"
	ImageProcessorTypeSelectiveColor ImageProcessorType = "selective_color"
)

// ImageProcessorConfig holds configuration for a filter applied to images.
type ImageProcessorConfig struct {
	Type ImageProcessorType
	Data map[string]string
}

type CronConfig struct {
	Cron string
}

// AssociatedImageProviders holds a catalog entry associated with a display.
type AssociatedImageProviders struct {
	ProviderConfig *ImageProviderConfig
	TimeRange      CronConfig
}

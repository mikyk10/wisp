package config

import "log/slog"

const (
	ImageFileProviderType     string = "file"
	ImageHTTPProviderType     string = "http"
	ImageColorbarProviderType string = "colorbar"
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
	Tagging TaggingConfig `yaml:"tagging"`
}

// TaggingConfig holds settings for the external tagging service.
type TaggingConfig struct {
	Endpoint   string `yaml:"endpoint"`    // e.g. http://wisp-ai:8082/pipeline/tag
	TimeoutSec int    `yaml:"timeout_sec"` // per-request timeout (default: 180)
	MaxTags    int    `yaml:"max_tags"`    // default: 10
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
	Crop                 CropConfig
	Catalog              []*AssociatedImageProviders
	ImageProcessors      []*ImageProcessorConfig
	SleepDurationSeconds int
}

type CropStrategy string

const (
	CropStrategyCenter      CropStrategy = "center"
	CropStrategyExifSubject CropStrategy = "exif_subject"
)

type CropConfig struct {
	Strategy CropStrategy
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
	ColorReduction *ColorReduction // per-catalog override (nil = use display default)
}

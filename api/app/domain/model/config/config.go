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
		// Driver and DSN both take an environment override, and the environment
		// has the last word: it is applied after the file is read and nothing
		// downstream writes either field. They are meant to be set as a pair,
		// because a DSN written for one dialect means nothing to another — an
		// installation moving between databases has to be able to say both
		// things from the same place, without editing a file inside an image.
		//
		// An override that is set but empty counts as unset and leaves the file
		// value standing, so a `DB_DSN=` left unfilled in a compose file does
		// not take the database away.
		Driver string `yaml:"driver" env:"DB_DRIVER"`
		DSN    string `yaml:"dsn" env:"DB_DSN"`

		// MaxOpenConns and MaxIdleConns bound the connection pool. Zero means
		// "unset" and LoadConfig fills in the default; it does not mean the
		// unlimited pool database/sql would otherwise give, which is what a
		// burst of thumbnail requests turns into a wall of refused connections.
		//
		// Both take an environment override so that one image can run the web
		// server and the scheduled commands with different shares of the same
		// database, without a config file per workload.
		MaxOpenConns  int `yaml:"max_open_conns" env:"DB_MAX_OPEN_CONNS"`
		MaxIdleConns  int `yaml:"max_idle_conns" env:"DB_MAX_IDLE_CONNS"`
		DriverOptions struct {
			Sqlite3 struct {
			}
		}
	}
	Tagging         TaggingConfig         `yaml:"tagging"`
	DeliveryHistory DeliveryHistoryConfig `yaml:"delivery_history"`
}

// DefaultDeliveryHistorySize is how many deliveries each display keeps when the
// configuration does not say.
const DefaultDeliveryHistorySize = 20

// DefaultMaxOpenConns and DefaultMaxIdleConns bound the connection pool when
// the configuration does not say.
//
// database/sql opens an unbounded number of connections unless told otherwise,
// and a server answering a photo grid answers a few hundred thumbnail requests
// at once. Against MySQL that was survivable — a connection there is a thread,
// and the server's limit is high enough to absorb the burst. PostgreSQL forks a
// process per connection and ships with a much lower ceiling, so the same burst
// walks into "sorry, too many clients already" and every request in it fails.
//
// The open limit is deliberately far below any server's own. It bounds one
// process and a deployment runs several — the web server plus whichever
// scheduled command is running — so the room left over is not spare, it is the
// other processes' share.
//
// The idle limit matches the open one. The database/sql default of 2 closes the
// rest between bursts, so the next burst pays to open them again — on
// PostgreSQL, a forked backend per request, which is the cost the pool exists
// to stop paying.
const (
	DefaultMaxOpenConns = 20
	DefaultMaxIdleConns = 20
)

// MaxDeliveryHistorySize is the largest ring a display may be given.
//
// The bound exists because the ring size is the only thing keeping the table
// finite, and it is multiplied by the number of displays. A limit an operator
// can overshoot by a typo is not a limit.
const MaxDeliveryHistorySize = 1000

// DeliveryHistoryConfig controls the per-display record of what was shown.
//
// The switch is Disabled rather than Enabled on purpose. GlobalConfig is
// unmarshalled straight from YAML with no defaulting pass, so a key absent from
// the file takes Go's zero value — and every configuration written before this
// feature existed is missing the key. Spelled as Enabled, the feature would
// arrive switched off everywhere and stay that way until each installation
// edited its config; spelled as Disabled, it arrives on, which is what it is
// for.
type DeliveryHistoryConfig struct {
	Disabled bool `yaml:"disabled"`

	// Size is how many deliveries each display keeps. LoadConfig resolves 0 to
	// DefaultDeliveryHistorySize, so everything downstream sees a plain count.
	Size int `yaml:"size"`
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

	// WakeSchedule names the moments this panel should be awake, as cron
	// expressions. When set, the sleep interval handed to the device is the
	// distance to the next of them rather than SleepDurationSeconds.
	WakeSchedule []string
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

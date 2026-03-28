package config

// ImageHTTPProviderConfig holds configuration for an HTTP image catalog.
type ImageHTTPProviderConfig struct {
	URL         string
	Method      string            // "GET" (default) or "POST"
	ImageSource *HTTPImageSource  // POST: source image config (nil = GET)
	Cache       HTTPCacheConfig   // cache policy
	TimeoutSec  int               // request timeout in seconds (default: 120)
	Headers     map[string]string // custom HTTP headers (supports ${ENV_VAR} expansion)
}

func (ImageHTTPProviderConfig) providerConfigTag() {}

// IsBackground returns true if this HTTP catalog uses background caching.
func (c ImageHTTPProviderConfig) IsBackground() bool {
	return c.Cache.Type == "background"
}

// HTTPImageSource configures the source image for push-pull (POST) mode.
type HTTPImageSource struct {
	Catalogs    []string // source file catalog keys
	Mode        string   // "random" (default) or "fixed"
	ImageID     uint     // source image ID (mode=fixed only)
	Orientation string   // "landscape" or "portrait"
	Tags        []string // tag filter: match any of these tags (OR)
}

// HTTPCacheConfig defines the caching behavior for an HTTP catalog.
type HTTPCacheConfig struct {
	Type       string // "realtime" (default) or "background"
	Depth      int    // background: max cached images
	EvictCount int    // background: images to evict per cycle
}

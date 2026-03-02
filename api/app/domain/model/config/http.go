package config

type ImageHTTPProviderConfig struct {
	URL string
}

func (ImageHTTPProviderConfig) providerConfigTag() {}

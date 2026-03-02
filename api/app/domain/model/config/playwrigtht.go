package config

type ImagePlaywrightProviderConfig struct {
	URL    string
	Server string
}

func (ImagePlaywrightProviderConfig) providerConfigTag() {}

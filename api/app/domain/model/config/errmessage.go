package config

type ImageErrorMessageProviderConfig struct {
	Message string
	Detail  string
}

func (ImageErrorMessageProviderConfig) providerConfigTag() {}

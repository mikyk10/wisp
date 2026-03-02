package config

type ImageLuaProviderConfig struct {
	Script string
}

func (ImageLuaProviderConfig) providerConfigTag() {}

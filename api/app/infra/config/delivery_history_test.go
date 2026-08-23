package config

import (
	"testing"

	domainConfig "github.com/mikyk10/wisp/app/domain/model/config"
)

// TestDeliveryHistoryDefaultsToEnabled is the reason the switch is spelled
// Disabled rather than Enabled.
//
// GlobalConfig is unmarshalled straight from YAML with no defaulting pass, so
// an absent key takes Go's zero value — and every config file written before
// this feature existed is missing it. testdata/config.yaml is one such file:
// the feature has to come up switched on there, with a usable size.
func TestDeliveryHistoryDefaultsToEnabled(t *testing.T) {
	conf, _, err := newLoaderFromDir("testdata").LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}

	if conf.DeliveryHistory.Disabled {
		t.Error("delivery history is disabled for a config file that does not mention it, want enabled")
	}
	if conf.DeliveryHistory.Size != domainConfig.DefaultDeliveryHistorySize {
		t.Errorf("delivery_history.size = %d, want the default %d",
			conf.DeliveryHistory.Size, domainConfig.DefaultDeliveryHistorySize)
	}
}

// TestValidateDeliveryHistorySize: 0 means "unset" and is filled in by
// LoadConfig, so validation has to let it through; everything past the bound is
// refused rather than clamped, because the size is the only thing keeping the
// table finite and it is paid once per display.
func TestValidateDeliveryHistorySize(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{"unset is valid and gets the default later", 0, false},
		{"a small ring is valid", 5, false},
		{"the upper bound is valid", domainConfig.MaxDeliveryHistorySize, false},
		{"negative is invalid", -1, true},
		{"past the upper bound is invalid", domainConfig.MaxDeliveryHistorySize + 1, true},
		{"a typo of an extra couple of digits is invalid", 200000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := &domainConfig.GlobalConfig{}
			conf.Database.Driver = "sqlite"
			conf.DeliveryHistory.Size = tt.size
			err := validateGlobalConfig(conf)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGlobalConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

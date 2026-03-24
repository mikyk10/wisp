package config

import (
	"testing"
	domainConfig "github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/finder/fs"
)

func newLoaderFromDir(dir string) domainConfig.ConfigLoader {
	return &defaultConfigLoader{
		finder: fs.NewConfigFilePathFinder(dir),
	}
}

func TestLoadConfig_HappyPath(t *testing.T) {
	_, svc, err := newLoaderFromDir("testdata").LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}
	if len(svc.Catalog) == 0 {
		t.Error("expected at least one catalog entry")
	}
	if len(svc.Displays) == 0 {
		t.Error("expected at least one display entry")
	}
}

func TestLoadConfig_UnknownCatalogKey(t *testing.T) {
	_, _, err := newLoaderFromDir("testdata_unknown_catalog").LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig() expected error for unknown catalog key, got nil")
	}
}

func TestLoadConfig_InvalidDisplayModel(t *testing.T) {
	_, _, err := newLoaderFromDir("testdata_invalid_model").LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig() expected error for invalid display model, got nil")
	}
}

func TestLoadConfig_InvalidCronExpression(t *testing.T) {
	_, _, err := newLoaderFromDir("testdata_invalid_cron").LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig() expected error for invalid cron expression, got nil")
	}
}

func TestLoadConfig_ParsesFileHooks(t *testing.T) {
	_, svc, err := newLoaderFromDir("testdata_hooks").LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}

	withHook, ok := svc.Catalog["album-with-hook"]
	if !ok {
		t.Fatal("expected catalog entry 'album-with-hook'")
	}
	fpc, ok := withHook.Config.(domainConfig.ImageFileProviderConfig)
	if !ok {
		t.Fatal("expected ImageFileProviderConfig for album-with-hook")
	}
	if fpc.Hooks.OnNewFile != "echo tagged {file}" {
		t.Errorf("expected on_new_file = %q, got %q", "echo tagged {file}", fpc.Hooks.OnNewFile)
	}

	noHook, ok := svc.Catalog["album-no-hook"]
	if !ok {
		t.Fatal("expected catalog entry 'album-no-hook'")
	}
	fpcNo, ok := noHook.Config.(domainConfig.ImageFileProviderConfig)
	if !ok {
		t.Fatal("expected ImageFileProviderConfig for album-no-hook")
	}
	if fpcNo.Hooks.OnNewFile != "" {
		t.Errorf("expected empty on_new_file for album-no-hook, got %q", fpcNo.Hooks.OnNewFile)
	}
}

func TestValidateGlobalConfig(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		wantErr bool
	}{
		{"sqlite is valid", "sqlite", false},
		{"mysql is valid", "mysql", false},
		{"empty driver is invalid", "", true},
		{"sqlite3 is invalid", "sqlite3", true},
		{"postgres is invalid", "postgres", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := &domainConfig.GlobalConfig{}
			conf.Database.Driver = tt.driver
			err := validateGlobalConfig(conf)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGlobalConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

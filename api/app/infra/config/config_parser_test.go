package config

import (
	"maps"
	"slices"
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

	// Name the key rather than counting entries. Displays are filed under
	// mac_address, which is the :displayKey in /pf/{displayKey}/image/random.bin,
	// and a fixture that spells the field anything else still loads — it just
	// files the display under the empty string. A count would not notice.
	const displayKey = "entrance-01"
	if _, ok := svc.Displays[displayKey]; !ok {
		t.Errorf("loaded displays are keyed %v, want one keyed by the fixture's mac_address %q",
			slices.Sorted(maps.Keys(svc.Displays)), displayKey)
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

// TestLoadConfig_InvalidWakeSchedule: the wake schedule decides when a panel
// comes back, so an expression nothing can read would leave the device waiting
// on a moment that never arrives. Refuse the config while there is still
// someone to tell about it.
func TestLoadConfig_InvalidWakeSchedule(t *testing.T) {
	_, _, err := newLoaderFromDir("testdata_invalid_wake_schedule").LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig() expected error for invalid wake schedule, got nil")
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
		{"postgres is valid", "postgres", false},
		{"empty driver is invalid", "", true},
		{"sqlite3 is invalid", "sqlite3", true},
		{"postgresql is invalid", "postgresql", true},
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

// TestLoadConfig_DatabaseFromEnv: the file says sqlite, the environment says
// postgres, and what comes back is postgres. This is how a container is pointed
// at its database — the image ships a config.yaml it cannot edit, so a
// deployment that cannot override the driver from the environment cannot use
// any driver but the one baked into the image.
//
// Driver and DSN are checked together on purpose: a DSN written for one dialect
// is meaningless to another, so an override that moved only one of them would
// leave the pair inconsistent.
func TestLoadConfig_DatabaseFromEnv(t *testing.T) {
	const dsn = "host=db port=5432 user=wisp dbname=wisp sslmode=disable"
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DB_DSN", dsn)

	conf, _, err := newLoaderFromDir("testdata").LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}
	if conf.Database.Driver != "postgres" {
		t.Errorf("driver = %q, want %q (the file says sqlite; DB_DRIVER should win)",
			conf.Database.Driver, "postgres")
	}
	if conf.Database.DSN != dsn {
		t.Errorf("dsn = %q, want %q", conf.Database.DSN, dsn)
	}
}

// TestLoadConfig_EmptyEnvKeepsFileValue: an override that is present but empty
// counts as unset. A compose file that declares `DB_DSN=` without filling it in
// is a common half-finished state, and the useful reading of it is "I did not
// say", not "the database has no DSN" — the latter would take a working server
// down at the moment someone edited an unrelated line.
func TestLoadConfig_EmptyEnvKeepsFileValue(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_DSN", "")

	conf, _, err := newLoaderFromDir("testdata").LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}
	if conf.Database.Driver != "sqlite" {
		t.Errorf("driver = %q, want the file's %q", conf.Database.Driver, "sqlite")
	}
	if conf.Database.DSN != "tmp/wisp.db" {
		t.Errorf("dsn = %q, want the file's %q", conf.Database.DSN, "tmp/wisp.db")
	}
}

// TestLoadConfig_InvalidDriverFromEnv: an override is still subject to the same
// check as a value from the file. Validation runs after the environment is
// applied, so a typo in DB_DRIVER is refused at load rather than reaching the DI
// container, where the only thing left to say is "unsupported database driver"
// with no mention of where the name came from.
func TestLoadConfig_InvalidDriverFromEnv(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgresql")

	if _, _, err := newLoaderFromDir("testdata").LoadConfig(); err == nil {
		t.Fatal("LoadConfig() expected error for invalid DB_DRIVER, got nil")
	}
}

// TestLoadConfig_LegacyDSNEnv: DB_DEFAULT_DSN is the name this override shipped
// under. An installation already setting it is one that cannot edit its config
// file, so dropping the name would silently send it back to whatever the image
// contains — a server that keeps running against the wrong catalog.
func TestLoadConfig_LegacyDSNEnv(t *testing.T) {
	const legacy = "host=old port=5432 user=wisp dbname=wisp sslmode=disable"
	t.Setenv("DB_DSN", "")
	t.Setenv("DB_DEFAULT_DSN", legacy)

	conf, _, err := newLoaderFromDir("testdata").LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}
	if conf.Database.DSN != legacy {
		t.Errorf("dsn = %q, want %q from the deprecated DB_DEFAULT_DSN", conf.Database.DSN, legacy)
	}
}

// TestLoadConfig_LegacyDSNEnvLosesToCurrent: with both names set, the current
// one wins. The old name is only a fallback, so an operator part-way through a
// rename is reading the variable they just wrote, not the one they forgot to
// delete.
func TestLoadConfig_LegacyDSNEnvLosesToCurrent(t *testing.T) {
	const current = "host=new port=5432 user=wisp dbname=wisp sslmode=disable"
	t.Setenv("DB_DEFAULT_DSN", "host=old port=5432 user=wisp dbname=wisp sslmode=disable")
	t.Setenv("DB_DSN", current)

	conf, _, err := newLoaderFromDir("testdata").LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}
	if conf.Database.DSN != current {
		t.Errorf("dsn = %q, want %q — DB_DSN should win over DB_DEFAULT_DSN", conf.Database.DSN, current)
	}
}

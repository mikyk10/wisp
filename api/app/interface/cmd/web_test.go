package cmd

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/di"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
	infraconfig "github.com/mikyk10/wisp/app/infra/config"

	"go.uber.org/dig"
	"gorm.io/gorm"
)

// recordingDeliveryHistory remembers what Reconcile was handed and can be told
// to fail. Everything else on the interface is unreachable from the code under
// test and answers with nothing.
type recordingDeliveryHistory struct {
	calls       int
	displayKeys []string
	size        int
	err         error
}

func (r *recordingDeliveryHistory) Record(*model.DeliveryHistory, int) error { return nil }

func (r *recordingDeliveryHistory) ListByDisplay(string, int) ([]*repository.DeliveryHistoryEntry, error) {
	return nil, nil
}

func (r *recordingDeliveryHistory) SummaryByDisplay() ([]*repository.DeliverySummary, error) {
	return nil, nil
}

func (r *recordingDeliveryHistory) Reconcile(displayKeys []string, size int) error {
	r.calls++
	r.displayKeys = displayKeys
	r.size = size
	return r.err
}

func containerWithDeliveryHistory(t *testing.T, dhr repository.DeliveryHistoryRepository) *dig.Container {
	t.Helper()

	c := dig.New()
	if err := c.Provide(func() repository.DeliveryHistoryRepository { return dhr }); err != nil {
		t.Fatalf("Provide: %v", err)
	}
	return c
}

func displayConfigs(keys ...string) map[string]*config.DisplayConfig {
	displays := make(map[string]*config.DisplayConfig, len(keys))
	for _, key := range keys {
		// Name deliberately differs from the key: what has to reach Reconcile
		// is the key the map is built on, not anything else on the display.
		displays[key] = &config.DisplayConfig{Name: "panel-" + key, Key: key}
	}
	return displays
}

func TestReconcileDeliveryHistoryPassesConfiguredDisplaysAndSize(t *testing.T) {
	dhr := &recordingDeliveryHistory{}

	gConf := &config.GlobalConfig{}
	gConf.DeliveryHistory.Size = 7
	sConf := &config.ServiceConfig{Displays: displayConfigs("b0b1b2b3b4b5", "a0a1a2a3a4a5")}

	reconcileDeliveryHistory(context.Background(), containerWithDeliveryHistory(t, dhr), gConf, sConf)

	if dhr.calls != 1 {
		t.Fatalf("Reconcile called %d times at start-up, want exactly 1", dhr.calls)
	}

	want := []string{"a0a1a2a3a4a5", "b0b1b2b3b4b5"}
	if !slices.Equal(dhr.displayKeys, want) {
		t.Errorf("Reconcile got display keys %v, want %v", dhr.displayKeys, want)
	}
	if dhr.size != 7 {
		t.Errorf("Reconcile got size %d, want the configured 7", dhr.size)
	}
}

// TestReconcileDeliveryHistoryUsesTheKeyDeliveriesAreFiledUnder is the test
// that keeps this from quietly emptying the table.
//
// Reconcile deletes every row whose display_key is absent from the list it is
// given. A list built from the wrong field would therefore make every
// configured display look retired and take its entire history on the next
// boot. The string a delivery is filed under is the :displayKey path
// parameter, which RecordDelivery looks up in ServiceConfig.Displays and then
// stores verbatim — so what has to be passed is the key of that same map.
//
// Rather than assert that against a hand-built map, this runs the real config
// loader over a service.yaml and checks that the mac_address written there is
// exactly what arrives at Reconcile.
func TestReconcileDeliveryHistoryUsesTheKeyDeliveriesAreFiledUnder(t *testing.T) {
	const macAddress = "a1b2c3d4e5f6"

	dir := t.TempDir()
	writeTestConfig(t, dir, "config.yaml", `log_level: DEBUG
port: 9002
database:
  driver: sqlite
  dsn: tmp/wisp.db
delivery_history:
  size: 3
`)
	writeTestConfig(t, dir, "service.yaml", `catalog:
  - key: album-01
    type: file
    file:
      src_path: /tmp/photos

displays:
  - name: entrance
    mac_address: `+macAddress+`
    api_version: 2025-03-01
    model: ws7in3e
    orientation: landscape
    catalog:
      - key: album-01
`)
	t.Chdir(dir)

	gConf, sConf, err := infraconfig.NewTestConfigLoader().LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if _, ok := sConf.Displays[macAddress]; !ok {
		t.Fatalf("the loaded service config has displays %v, want one keyed by the mac_address %q",
			slices.Sorted(maps.Keys(sConf.Displays)), macAddress)
	}

	dhr := &recordingDeliveryHistory{}
	reconcileDeliveryHistory(context.Background(), containerWithDeliveryHistory(t, dhr), gConf, sConf)

	want := []string{macAddress}
	if !slices.Equal(dhr.displayKeys, want) {
		t.Errorf("Reconcile got display keys %v, want %v — the mac_address from service.yaml, which is what a delivery is filed under",
			dhr.displayKeys, want)
	}
	if dhr.size != 3 {
		t.Errorf("Reconcile got size %d, want the configured 3", dhr.size)
	}
}

// TestReconcileDeliveryHistorySurvivesAFailure: the server must still come up.
// The commonest failure is the table not existing at all, which is what every
// installation predating this feature looks like on its first boot. The test
// passing at all is the assertion — a fatal there would take the process down
// with it.
func TestReconcileDeliveryHistorySurvivesAFailure(t *testing.T) {
	dhr := &recordingDeliveryHistory{err: errors.New("no such table: delivery_histories")}

	gConf := &config.GlobalConfig{}
	gConf.DeliveryHistory.Size = 5
	sConf := &config.ServiceConfig{Displays: displayConfigs("a0a1a2a3a4a5")}

	reconcileDeliveryHistory(context.Background(), containerWithDeliveryHistory(t, dhr), gConf, sConf)

	if dhr.calls != 1 {
		t.Errorf("Reconcile called %d times, want 1", dhr.calls)
	}
}

// TestReconcileDeliveryHistorySurvivesAnUnresolvableRepository covers the other
// way this can go wrong: the container cannot build the repository, because the
// database would not open. Serving pictures is not this step's to refuse.
func TestReconcileDeliveryHistorySurvivesAnUnresolvableRepository(t *testing.T) {
	gConf := &config.GlobalConfig{}
	gConf.DeliveryHistory.Size = 5
	sConf := &config.ServiceConfig{Displays: displayConfigs("a0a1a2a3a4a5")}

	reconcileDeliveryHistory(context.Background(), dig.New(), gConf, sConf)
}

// TestReconcileDeliveryHistoryRunsWhileDisabled: switching the feature off
// stops new deliveries being written down, and nothing else then trims the rows
// written while it was on. Skipping here would strand them above the bound for
// good.
func TestReconcileDeliveryHistoryRunsWhileDisabled(t *testing.T) {
	dhr := &recordingDeliveryHistory{}

	gConf := &config.GlobalConfig{}
	gConf.DeliveryHistory.Disabled = true
	gConf.DeliveryHistory.Size = 4
	sConf := &config.ServiceConfig{Displays: displayConfigs("a0a1a2a3a4a5")}

	reconcileDeliveryHistory(context.Background(), containerWithDeliveryHistory(t, dhr), gConf, sConf)

	if dhr.calls != 1 {
		t.Fatalf("Reconcile called %d times with the feature disabled, want 1", dhr.calls)
	}
	if dhr.size != 4 {
		t.Errorf("Reconcile got size %d, want the configured 4 — disabling must not be read as a size of zero", dhr.size)
	}
}

// TestReconcileDeliveryHistoryWithNoDisplays: an empty key list is Reconcile's
// signal to leave the per-display deletion alone. Handing it one is the safe
// thing to do for a configuration that names no displays at all.
func TestReconcileDeliveryHistoryWithNoDisplays(t *testing.T) {
	dhr := &recordingDeliveryHistory{}

	gConf := &config.GlobalConfig{}
	gConf.DeliveryHistory.Size = 5
	sConf := &config.ServiceConfig{Displays: map[string]*config.DisplayConfig{}}

	reconcileDeliveryHistory(context.Background(), containerWithDeliveryHistory(t, dhr), gConf, sConf)

	if dhr.calls != 1 {
		t.Fatalf("Reconcile called %d times, want 1", dhr.calls)
	}
	if len(dhr.displayKeys) != 0 {
		t.Errorf("Reconcile got display keys %v, want none", dhr.displayKeys)
	}
}

// TestReconcileDeliveryHistoryThroughTheRealContainer proves the two things a
// fake cannot: that the production DI wiring can hand this call site a
// DeliveryHistoryRepository, and that a start-up under a smaller ring really
// does bring the table down to it.
func TestReconcileDeliveryHistoryThroughTheRealContainer(t *testing.T) {
	const displayKey = "a1b2c3d4e5f6"

	gConf := &config.GlobalConfig{}
	gConf.DeliveryHistory.Size = 3
	sConf := &config.ServiceConfig{
		Catalog:  map[string]*config.ImageProviderConfig{},
		Displays: displayConfigs(displayKey),
	}

	container := di.NewBuilder().WithConfig(gConf, sConf).WithSQLiteMock().Build()

	// Ten deliveries made under a ring of ten, met by a configuration that now
	// says three.
	if err := container.Invoke(func(dhr repository.DeliveryHistoryRepository) {
		for range 10 {
			rec := &model.DeliveryHistory{
				DisplayKey:  displayKey,
				DeliveredAt: time.Now().UTC(),
				Kind:        model.DeliveryKindPhoto,
			}
			if err := dhr.Record(rec, 10); err != nil {
				t.Fatalf("Record: %v", err)
			}
		}
	}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	reconcileDeliveryHistory(context.Background(), container, gConf, sConf)

	var rows int64
	if err := container.Invoke(func(conn *gorm.DB) {
		if err := conn.Model(&model.DeliveryHistory{}).Count(&rows).Error; err != nil {
			t.Fatalf("Count: %v", err)
		}
	}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if rows != int64(gConf.DeliveryHistory.Size) {
		t.Errorf("the table holds %d rows after a start-up under a ring of %d, want %d",
			rows, gConf.DeliveryHistory.Size, gConf.DeliveryHistory.Size)
	}
}

func writeTestConfig(t *testing.T, dir, name, body string) {
	t.Helper()

	confDir := filepath.Join(dir, "testdata")
	if err := os.MkdirAll(confDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(confDir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}
}

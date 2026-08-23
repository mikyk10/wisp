package usecase_test

import (
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/catalog"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"
	"github.com/mikyk10/wisp/app/usecase"

	"gorm.io/gorm"
)

// deviceTestConfig is two displays whose keys sort the other way round from the
// order they are written in, so a result that happens to preserve insertion
// order is not mistaken for a sorted one.
func deviceTestConfig() *config.ServiceConfig {
	photos := &config.ImageProviderConfig{Key: "photos"}
	return &config.ServiceConfig{
		Catalog: map[string]*config.ImageProviderConfig{"photos": photos},
		Displays: map[string]*config.DisplayConfig{
			"zeta": {
				Key:                  "zeta",
				Name:                 "desk-test",
				DisplayModel:         "ws4in0e",
				Orientation:          config.DisplayOrientationPortrait,
				SleepDurationSeconds: 86400,
			},
			"alpha": {
				Key:                  "alpha",
				Name:                 "living-room",
				DisplayModel:         "ws7in3e",
				Orientation:          config.DisplayOrientationLandscape,
				SleepDurationSeconds: 300,
				Catalog:              []*config.AssociatedImageProviders{{ProviderConfig: photos}},
				WakeSchedule:         []string{"*/30 7-16 * * *"},
			},
		},
	}
}

// newDeviceUsecase wires a use case over a fresh in-memory DB.
//
// historyTable says whether delivery_histories exists. It does not on any
// installation that predates the feature and has never been migrated, which is
// the case the device list must survive.
func newDeviceUsecase(t *testing.T, svc *config.ServiceConfig, size int, historyTable bool) (usecase.DeviceUsecase, *gorm.DB) {
	t.Helper()
	return newDeviceUsecaseWith(t, svc, config.DeliveryHistoryConfig{Size: size}, historyTable)
}

// newDeviceUsecaseWith is the same over an arbitrary delivery-history
// configuration, for the cases that turn recording off.
func newDeviceUsecaseWith(t *testing.T, svc *config.ServiceConfig, hcfg config.DeliveryHistoryConfig, historyTable bool) (usecase.DeviceUsecase, *gorm.DB) {
	t.Helper()

	conn, err := infra.NewSqliteConnection("", true)
	if err != nil {
		t.Fatalf("failed to create in-memory DB: %v", err)
	}
	conn.AutoMigrate(&model.Image{}) //nolint:errcheck
	if historyTable {
		conn.AutoMigrate(&model.DeliveryHistory{}) //nolint:errcheck
	}

	uc := usecase.NewDeviceUsecase(&config.GlobalConfig{DeliveryHistory: hcfg}, svc, infraRepo.NewDeliveryHistoryRepositoryImpl(conn))
	return uc, conn
}

// recordDelivery writes one row straight into the ring, bypassing Record so a
// test can choose the sequence number and the timestamp.
func recordDelivery(t *testing.T, db *gorm.DB, key string, seq int64, at time.Time, kind model.DeliveryKind, imageID model.PrimaryKey) {
	t.Helper()
	err := db.Create(&model.DeliveryHistory{
		DisplayKey:   key,
		Slot:         int(seq),
		Seq:          seq,
		DeliveredAt:  at,
		Kind:         kind,
		ImageID:      imageID,
		CatalogKey:   "photos",
		SleepSeconds: 300,
	}).Error
	if err != nil {
		t.Fatalf("failed to insert delivery: %v", err)
	}
}

// TestListDevices_NeverDeliveredIsPresent is the whole point of building the
// list from the configuration: a frame that has never once phoned home has to
// appear, and appear as silent. A list built from the history would leave it
// out entirely.
func TestListDevices_NeverDeliveredIsPresent(t *testing.T) {
	uc, db := newDeviceUsecase(t, deviceTestConfig(), 20, true)
	recordDelivery(t, db, "alpha", 1, time.Now(), model.DeliveryKindPhoto, 4821)

	devices, err := uc.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("want 2 devices, got %d", len(devices))
	}

	var zeta *usecase.DeviceStatus
	for _, dev := range devices {
		if dev.Key == "zeta" {
			zeta = dev
		}
	}
	if zeta == nil {
		t.Fatal("the display that has never been delivered to is missing from the list")
	}
	if !zeta.LastDeliveredAt.IsZero() {
		t.Errorf("want the zero time for a display never delivered to, got %v", zeta.LastDeliveredAt)
	}
	if zeta.RecentDeliveryCount != 0 || zeta.RecentErrorCount != 0 {
		t.Errorf("want no counts, got %d deliveries / %d errors", zeta.RecentDeliveryCount, zeta.RecentErrorCount)
	}
}

// TestListDevices_StableOrder: Go iterates a map at random, so without a sort
// the list reshuffles on every poll.
func TestListDevices_StableOrder(t *testing.T) {
	svc := deviceTestConfig()
	for i := 0; i < 20; i++ {
		uc, _ := newDeviceUsecase(t, svc, 20, true)
		devices, err := uc.ListDevices()
		if err != nil {
			t.Fatalf("ListDevices: %v", err)
		}
		if devices[0].Key != "alpha" || devices[1].Key != "zeta" {
			t.Fatalf("want devices ordered by key, got %s, %s", devices[0].Key, devices[1].Key)
		}
	}
}

// TestListDevices_ConfigFields checks the parts that come from service.yaml,
// including the two slices that must never be nil and the size that comes from
// the display model rather than the configuration.
func TestListDevices_ConfigFields(t *testing.T) {
	uc, _ := newDeviceUsecase(t, deviceTestConfig(), 20, true)
	devices, err := uc.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}

	alpha, zeta := devices[0], devices[1]

	if alpha.Name != "living-room" || alpha.Model != "ws7in3e" {
		t.Errorf("unexpected name/model: %q / %q", alpha.Name, alpha.Model)
	}
	if alpha.Width != 800 || alpha.Height != 480 {
		t.Errorf("want 800x480 from the display model, got %dx%d", alpha.Width, alpha.Height)
	}
	if alpha.Orientation != "landscape" || zeta.Orientation != "portrait" {
		t.Errorf("unexpected orientations: %q / %q", alpha.Orientation, zeta.Orientation)
	}
	if alpha.SleepDurationSeconds != 300 || zeta.SleepDurationSeconds != 86400 {
		t.Errorf("unexpected sleep durations: %d / %d", alpha.SleepDurationSeconds, zeta.SleepDurationSeconds)
	}
	if len(alpha.CatalogKeys) != 1 || alpha.CatalogKeys[0] != "photos" {
		t.Errorf("unexpected catalog keys: %v", alpha.CatalogKeys)
	}
	if len(alpha.WakeSchedule) != 1 || alpha.WakeSchedule[0] != "*/30 7-16 * * *" {
		t.Errorf("unexpected wake schedule: %v", alpha.WakeSchedule)
	}

	// Empty, never nil: a nil slice marshals as null, and the handler passes
	// these straight through.
	if zeta.CatalogKeys == nil || zeta.WakeSchedule == nil {
		t.Errorf("want empty slices for a display with neither, got %v / %v", zeta.CatalogKeys, zeta.WakeSchedule)
	}
}

// TestListDevices_Counts checks that the counts come from the one grouped query
// and separate errors from the rest.
func TestListDevices_Counts(t *testing.T) {
	uc, db := newDeviceUsecase(t, deviceTestConfig(), 20, true)
	now := time.Now().UTC().Truncate(time.Second)
	recordDelivery(t, db, "alpha", 1, now.Add(-2*time.Minute), model.DeliveryKindPhoto, 1)
	recordDelivery(t, db, "alpha", 2, now.Add(-1*time.Minute), model.DeliveryKindError, 0)
	recordDelivery(t, db, "alpha", 3, now, model.DeliveryKindPhoto, 2)

	devices, err := uc.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}

	alpha := devices[0]
	if alpha.RecentDeliveryCount != 3 || alpha.RecentErrorCount != 1 {
		t.Errorf("want 3 deliveries / 1 error, got %d / %d", alpha.RecentDeliveryCount, alpha.RecentErrorCount)
	}
	if !alpha.LastDeliveredAt.Equal(now) {
		t.Errorf("want the newest delivery %v, got %v", now, alpha.LastDeliveredAt)
	}
}

// TestListDevices_WithoutHistoryTable: the table is absent on every
// installation that predates the feature and has never been migrated. The
// configured panels must still be listed rather than the whole view failing.
func TestListDevices_WithoutHistoryTable(t *testing.T) {
	uc, _ := newDeviceUsecase(t, deviceTestConfig(), 20, false)

	devices, err := uc.ListDevices()
	if err != nil {
		t.Fatalf("want the device list despite an unreadable history, got %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("want 2 devices, got %d", len(devices))
	}
	for _, dev := range devices {
		if !dev.LastDeliveredAt.IsZero() {
			t.Errorf("%s: want no delivery time when the history cannot be read", dev.Key)
		}
	}
}

func TestRecentWindow(t *testing.T) {
	uc, _ := newDeviceUsecase(t, deviceTestConfig(), 7, true)
	if got := uc.RecentWindow(); got != 7 {
		t.Errorf("want the configured ring size 7, got %d", got)
	}
}

// TestRecordingEnabled: the flag reports the condition under which a row is
// actually written, so a ring with no room in it reads as off even though the
// switch says nothing.
func TestRecordingEnabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		hcfg config.DeliveryHistoryConfig
		want bool
	}{
		{"recording", config.DeliveryHistoryConfig{Size: 20}, true},
		{"switched off", config.DeliveryHistoryConfig{Disabled: true, Size: 20}, false},
		{"no room in the ring", config.DeliveryHistoryConfig{Size: 0}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc, _ := newDeviceUsecaseWith(t, deviceTestConfig(), tc.hcfg, true)
			if got := uc.RecordingEnabled(); got != tc.want {
				t.Errorf("want %v, got %v", tc.want, got)
			}
		})
	}
}

// TestListDevices_DisabledKeepsTheCounts: recording being off does not make the
// stored deliveries untrue. They are reported as they stand, and the flag is
// what tells a reader they are the past.
func TestListDevices_DisabledKeepsTheCounts(t *testing.T) {
	uc, db := newDeviceUsecaseWith(t, deviceTestConfig(), config.DeliveryHistoryConfig{Disabled: true, Size: 20}, true)
	recordDelivery(t, db, "alpha", 1, time.Now(), model.DeliveryKindPhoto, 1)

	devices, err := uc.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if devices[0].RecentDeliveryCount != 1 {
		t.Errorf("want the stored delivery still counted, got %d", devices[0].RecentDeliveryCount)
	}
	if devices[0].LastDeliveredAt.IsZero() {
		t.Error("want the stored delivery time still reported")
	}
}

// TestListDeliveries_NewestFirst also covers image_available, which comes from
// the repository's LEFT JOIN rather than from a second lookup here.
func TestListDeliveries_NewestFirst(t *testing.T) {
	uc, db := newDeviceUsecase(t, deviceTestConfig(), 20, true)
	db.Exec(`INSERT INTO images (id, catalog_key, rnd, src, src_hash, thumb_jpg, image_orientation, created_at, updated_at) VALUES (7, 'photos', 0, '/mnt/photos/IMG_0421.jpg', 'hash', 'jpgdata', 1, datetime(), datetime())`)

	now := time.Now().UTC().Truncate(time.Second)
	recordDelivery(t, db, "alpha", 1, now.Add(-2*time.Minute), model.DeliveryKindPhoto, 7)  // still there
	recordDelivery(t, db, "alpha", 2, now.Add(-1*time.Minute), model.DeliveryKindPhoto, 99) // purged
	recordDelivery(t, db, "alpha", 3, now, model.DeliveryKindError, 0)                      // never had one

	entries, err := uc.ListDeliveries("alpha", 0)
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d", len(entries))
	}
	if entries[0].Seq != 3 || entries[1].Seq != 2 || entries[2].Seq != 1 {
		t.Fatalf("want newest first, got seqs %d, %d, %d", entries[0].Seq, entries[1].Seq, entries[2].Seq)
	}
	if entries[0].ImageAvailable || entries[1].ImageAvailable {
		t.Error("want unavailable for an error card and for a purged photograph")
	}
	if !entries[2].ImageAvailable {
		t.Error("want available for a photograph that is still indexed")
	}
}

// TestListDeliveries_ConfiguredButSilent: an empty history is an empty list,
// not a 404. Only an unconfigured key is a 404 — see the test below.
func TestListDeliveries_ConfiguredButSilent(t *testing.T) {
	uc, _ := newDeviceUsecase(t, deviceTestConfig(), 20, true)

	entries, err := uc.ListDeliveries("zeta", 0)
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want no entries, got %d", len(entries))
	}
}

func TestListDeliveries_UnknownDisplay(t *testing.T) {
	uc, _ := newDeviceUsecase(t, deviceTestConfig(), 20, true)

	_, err := uc.ListDeliveries("nonexistent", 0)
	if _, ok := err.(*catalog.DisplayNotFoundError); !ok {
		t.Fatalf("want a DisplayNotFoundError, got %v", err)
	}
}

// TestListDeliveries_WithoutHistoryTable is the same case
// TestListDevices_WithoutHistoryTable covers, and the two must answer alike.
// The table is absent on every installation that predates the feature and has
// never been migrated, and the README tells those operators the symptom is an
// empty history rather than an error. The UI asks for one of these lists per
// configured display, so a failure here fails every strip at once.
func TestListDeliveries_WithoutHistoryTable(t *testing.T) {
	uc, _ := newDeviceUsecase(t, deviceTestConfig(), 20, false)

	entries, err := uc.ListDeliveries("alpha", 0)
	if err != nil {
		t.Fatalf("want an empty history despite an unreadable one, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want no entries, got %d", len(entries))
	}
	// Empty, never nil: the handler ranges over this to build the payload, and
	// ListByDisplay's own contract is an empty slice rather than a nil one.
	if entries == nil {
		t.Error("want an empty slice, got nil")
	}
}

// TestListDeliveries_UnknownDisplayWithoutHistoryTable: swallowing read
// failures must not swallow the one error this endpoint is supposed to report.
// An unconfigured key is a client error and stays a 404 whether or not the
// history can be read — the check is against service.yaml and never reaches the
// store.
func TestListDeliveries_UnknownDisplayWithoutHistoryTable(t *testing.T) {
	uc, _ := newDeviceUsecase(t, deviceTestConfig(), 20, false)

	_, err := uc.ListDeliveries("nonexistent", 0)
	if _, ok := err.(*catalog.DisplayNotFoundError); !ok {
		t.Fatalf("want a DisplayNotFoundError, got %v", err)
	}
}

// TestNoRepository_BothReadsAgree pins the pair rather than either method.
//
// The bug fixed alongside this was two adjacent methods disagreeing about one
// failure, each defensible on its own, so what is asserted here is that they
// agree — not merely that neither falls over. Both read paths answer empty, and
// the 404 is untouched, since an unconfigured key is a client error whatever
// the store is.
//
// The container always provides a repository, so this state is unreachable in
// production. It is still worth a test: without one the guard reads as dead
// code and the next reader deletes it.
func TestNoRepository_BothReadsAgree(t *testing.T) {
	uc := usecase.NewDeviceUsecase(
		&config.GlobalConfig{DeliveryHistory: config.DeliveryHistoryConfig{Size: 20}},
		deviceTestConfig(),
		nil,
	)

	devices, err := uc.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("want the configured displays listed, got %d", len(devices))
	}
	for _, dev := range devices {
		if !dev.LastDeliveredAt.IsZero() || dev.RecentDeliveryCount != 0 {
			t.Errorf("%s: want no history, got %v / %d", dev.Key, dev.LastDeliveredAt, dev.RecentDeliveryCount)
		}
	}

	entries, err := uc.ListDeliveries("alpha", 0)
	if err != nil {
		t.Fatalf("want an empty history to match ListDevices, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want no entries, got %d", len(entries))
	}
	if entries == nil {
		t.Error("want an empty slice, got nil")
	}

	if _, err := uc.ListDeliveries("nonexistent", 0); err == nil {
		t.Error("want a DisplayNotFoundError for an unconfigured key, got nil")
	} else if _, ok := err.(*catalog.DisplayNotFoundError); !ok {
		t.Errorf("want a DisplayNotFoundError, got %v", err)
	}
}

// TestListDeliveries_LimitClamped: the rows are written straight in rather than
// through Record, so the store holds more than the window and a clamp that did
// nothing would show.
func TestListDeliveries_LimitClamped(t *testing.T) {
	uc, db := newDeviceUsecase(t, deviceTestConfig(), 3, true)
	now := time.Now().UTC().Truncate(time.Second)
	for i := 1; i <= 5; i++ {
		recordDelivery(t, db, "alpha", int64(i), now.Add(time.Duration(i)*time.Minute), model.DeliveryKindPhoto, 1)
	}

	for _, tc := range []struct {
		name  string
		limit int
		want  int
	}{
		{"unasked for", 0, 3},
		{"negative", -5, 3},
		{"under the window", 2, 2},
		{"over the window", 99, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries, err := uc.ListDeliveries("alpha", tc.limit)
			if err != nil {
				t.Fatalf("ListDeliveries: %v", err)
			}
			if len(entries) != tc.want {
				t.Errorf("limit %d: want %d entries, got %d", tc.limit, tc.want, len(entries))
			}
		})
	}
}

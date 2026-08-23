package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"
	"github.com/mikyk10/wisp/app/interface/handler"
	"github.com/mikyk10/wisp/app/usecase"

	"gorm.io/gorm"
)

// deviceSvcConfig is two displays whose keys sort the other way round from the
// order they are written in, so a response that happens to preserve insertion
// order is not mistaken for a sorted one. "zeta" has neither catalog nor wake
// schedule, which is the case that must still marshal as [].
func deviceSvcConfig() *config.ServiceConfig {
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

func newDeviceHandler(t *testing.T, size int) (*echo.Echo, handler.DeviceHandler, *gorm.DB) {
	t.Helper()
	return newDeviceHandlerWith(t, config.DeliveryHistoryConfig{Size: size}, true)
}

// newDeviceHandlerWith is the same over an arbitrary delivery-history
// configuration, for the cases that turn recording off.
//
// historyTable says whether delivery_histories exists. It does not on any
// installation that predates the feature and has never been migrated, which is
// the case both endpoints have to survive.
func newDeviceHandlerWith(t *testing.T, hcfg config.DeliveryHistoryConfig, historyTable bool) (*echo.Echo, handler.DeviceHandler, *gorm.DB) {
	t.Helper()

	conn, err := infra.NewSqliteConnection("", true)
	if err != nil {
		t.Fatalf("failed to create in-memory DB: %v", err)
	}
	conn.AutoMigrate(&model.Image{}) //nolint:errcheck
	if historyTable {
		conn.AutoMigrate(&model.DeliveryHistory{}) //nolint:errcheck
	}

	uc := usecase.NewDeviceUsecase(&config.GlobalConfig{DeliveryHistory: hcfg}, deviceSvcConfig(), infraRepo.NewDeliveryHistoryRepositoryImpl(conn))
	return echo.New(), handler.NewDeviceHandler(uc), conn
}

// insertDelivery writes one row straight into the ring, bypassing Record so a
// test can choose the sequence number and the timestamp.
func insertDelivery(t *testing.T, db *gorm.DB, rec *model.DeliveryHistory) {
	t.Helper()
	rec.Slot = int(rec.Seq)
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("failed to insert delivery: %v", err)
	}
}

func getDevices(t *testing.T, h handler.DeviceHandler, e *echo.Echo) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/devices")
	if !assert.NoError(t, h.ListDevices(c)) {
		t.FailNow()
	}
	return rec
}

func getDeliveries(t *testing.T, h handler.DeviceHandler, e *echo.Echo, key, query string) (*httptest.ResponseRecorder, error) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/device/"+key+"/deliveries"+query, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/device/:displayKey/deliveries")
	c.SetPathValues(echo.PathValues{{Name: "displayKey", Value: key}})
	return rec, h.ListDeliveries(c)
}

type deviceRecord struct {
	Key                  string   `json:"key"`
	Name                 string   `json:"name"`
	Model                string   `json:"model"`
	Width                int      `json:"width"`
	Height               int      `json:"height"`
	Orientation          string   `json:"orientation"`
	CatalogKeys          []string `json:"catalog_keys"`
	SleepDurationSeconds int      `json:"sleep_duration_seconds"`
	WakeSchedule         []string `json:"wake_schedule"`
	LastDeliveredAt      *string  `json:"last_delivered_at"`
	RecentDeliveryCount  int      `json:"recent_delivery_count"`
	RecentErrorCount     int      `json:"recent_error_count"`
}

type deviceListBody struct {
	RecordingEnabled bool           `json:"recording_enabled"`
	RecentWindow     int            `json:"recent_window"`
	Devices          []deviceRecord `json:"devices"`
}

type deliveryRecord struct {
	DeliveredAt           string  `json:"delivered_at"`
	Kind                  string  `json:"kind"`
	Reason                *string `json:"reason"`
	ImageID               *uint64 `json:"image_id"`
	CatalogKey            *string `json:"catalog_key"`
	Source                *string `json:"source"`
	RequestedSleepSeconds int     `json:"requested_sleep_seconds"`
	ImageAvailable        bool    `json:"image_available"`
}

type deliveryListBody struct {
	DeviceKey  string           `json:"device_key"`
	Deliveries []deliveryRecord `json:"deliveries"`
}

// TestListDevices_NeverDeliveredIsNull is the field the whole view turns on. It
// is checked against the raw body as well as the decoded one, because "" and
// null both decode into a *string the same way once the tag is a pointer.
func TestListDevices_NeverDeliveredIsNull(t *testing.T) {
	e, h, db := newDeviceHandler(t, 20)
	insertDelivery(t, db, &model.DeliveryHistory{
		DisplayKey: "alpha", Seq: 1, DeliveredAt: time.Now(), Kind: model.DeliveryKindPhoto, ImageID: 1,
	})

	rec := getDevices(t, h, e)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get(echo.HeaderContentType), "application/json")
	assert.Contains(t, rec.Body.String(), `"last_delivered_at":null`)

	var body deviceListBody
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.True(t, body.RecordingEnabled)
	assert.Equal(t, 20, body.RecentWindow)
	assert.Len(t, body.Devices, 2)

	zeta := body.Devices[1]
	assert.Equal(t, "zeta", zeta.Key)
	assert.Nil(t, zeta.LastDeliveredAt, "a display never delivered to reports null, not an empty string")
	assert.Equal(t, 0, zeta.RecentDeliveryCount)
	assert.Equal(t, 0, zeta.RecentErrorCount)

	alpha := body.Devices[0]
	if assert.NotNil(t, alpha.LastDeliveredAt) {
		assert.True(t, strings.HasSuffix(*alpha.LastDeliveredAt, "Z"), "delivery times are rendered in UTC")
	}
}

// TestListDevices_StableOrder: Go iterates a map at random, so without a sort
// the list reshuffles on every poll.
func TestListDevices_StableOrder(t *testing.T) {
	e, h, _ := newDeviceHandler(t, 20)
	for i := 0; i < 20; i++ {
		var body deviceListBody
		assert.NoError(t, json.Unmarshal(getDevices(t, h, e).Body.Bytes(), &body))
		if !assert.Equal(t, []string{"alpha", "zeta"}, []string{body.Devices[0].Key, body.Devices[1].Key}) {
			t.FailNow()
		}
	}
}

// TestListDevices_EmptyListsAreArrays: a nil slice marshals as null, which is
// what ListCatalogs does today and what a client should not have to handle.
func TestListDevices_EmptyListsAreArrays(t *testing.T) {
	e, h, _ := newDeviceHandler(t, 20)

	rec := getDevices(t, h, e)
	assert.Contains(t, rec.Body.String(), `"catalog_keys":[]`)
	assert.Contains(t, rec.Body.String(), `"wake_schedule":[]`)
	assert.NotContains(t, rec.Body.String(), `null,"sleep_duration_seconds"`)

	var body deviceListBody
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, []string{"photos"}, body.Devices[0].CatalogKeys)
	assert.Equal(t, []string{"*/30 7-16 * * *"}, body.Devices[0].WakeSchedule)
	assert.NotNil(t, body.Devices[1].CatalogKeys)
	assert.NotNil(t, body.Devices[1].WakeSchedule)
}

// TestListDevices_ConfigFields covers the parts that come from service.yaml and
// from the display model, neither of which the database has any say in.
func TestListDevices_ConfigFields(t *testing.T) {
	e, h, _ := newDeviceHandler(t, 20)

	var body deviceListBody
	assert.NoError(t, json.Unmarshal(getDevices(t, h, e).Body.Bytes(), &body))

	alpha, zeta := body.Devices[0], body.Devices[1]
	assert.Equal(t, "living-room", alpha.Name)
	assert.Equal(t, "ws7in3e", alpha.Model)
	assert.Equal(t, 800, alpha.Width)
	assert.Equal(t, 480, alpha.Height)
	assert.Equal(t, "landscape", alpha.Orientation)
	assert.Equal(t, 300, alpha.SleepDurationSeconds)

	assert.Equal(t, "ws4in0e", zeta.Model)
	assert.Equal(t, 400, zeta.Width)
	assert.Equal(t, 600, zeta.Height)
	assert.Equal(t, "portrait", zeta.Orientation)
	assert.Equal(t, 86400, zeta.SleepDurationSeconds)
}

// TestListDeliveries_NewestFirst also covers image_available, which the
// repository answers with a LEFT JOIN in the same query that reads the rows.
func TestListDeliveries_NewestFirst(t *testing.T) {
	e, h, db := newDeviceHandler(t, 20)
	db.Exec(`INSERT INTO images (id, catalog_key, rnd, src, src_hash, thumb_jpg, image_orientation, created_at, updated_at) VALUES (4821, 'photos', 0, '/mnt/photos/IMG_0421.jpg', 'hash', 'jpgdata', 1, datetime(), datetime())`)

	now := time.Now().UTC().Truncate(time.Second)
	insertDelivery(t, db, &model.DeliveryHistory{
		DisplayKey: "alpha", Seq: 1, DeliveredAt: now.Add(-10 * time.Minute),
		Kind: model.DeliveryKindError, Reason: model.DeliveryReasonNoImages,
		CatalogKey: "photos", SleepSeconds: 300,
	})
	insertDelivery(t, db, &model.DeliveryHistory{
		DisplayKey: "alpha", Seq: 2, DeliveredAt: now, Kind: model.DeliveryKindPhoto,
		ImageID: 4821, CatalogKey: "photos", Source: "/mnt/photos/IMG_0421.jpg", SleepSeconds: 300,
	})

	rec, err := getDeliveries(t, h, e, "alpha", "")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body deliveryListBody
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "alpha", body.DeviceKey)
	if !assert.Len(t, body.Deliveries, 2) {
		t.FailNow()
	}

	newest := body.Deliveries[0]
	assert.Equal(t, now.Format(time.RFC3339), newest.DeliveredAt)
	assert.Equal(t, "photo", newest.Kind)
	if assert.NotNil(t, newest.ImageID) {
		assert.Equal(t, uint64(4821), *newest.ImageID)
	}
	if assert.NotNil(t, newest.Source) {
		assert.Equal(t, "/mnt/photos/IMG_0421.jpg", *newest.Source)
	}
	assert.Equal(t, 300, newest.RequestedSleepSeconds)
	assert.True(t, newest.ImageAvailable, "the photograph is still indexed")
	assert.Nil(t, newest.Reason, "a photograph has no reason to report")
	if assert.NotNil(t, newest.CatalogKey) {
		assert.Equal(t, "photos", *newest.CatalogKey)
	}

	// An error card never had an images row, so it names no photograph and no
	// file. Both come back null rather than 0 and "".
	oldest := body.Deliveries[1]
	assert.Equal(t, "error", oldest.Kind)
	assert.Nil(t, oldest.ImageID)
	assert.Nil(t, oldest.Source)
	assert.False(t, oldest.ImageAvailable)
	assert.Contains(t, rec.Body.String(), `"image_id":null`)
	assert.Contains(t, rec.Body.String(), `"source":null`)

	// The reason is the only place the distinction between "the catalog holds
	// nothing" and "the file is gone" survives — the provider decides it as it
	// gives up and nothing downstream can reconstruct it. Passed through as the
	// stored code, not as prose.
	if assert.NotNil(t, oldest.Reason) {
		assert.Equal(t, "no_images", *oldest.Reason)
	}
	assert.Contains(t, rec.Body.String(), `"reason":null`, "a non-error delivery reports null, not an empty string")
}

// TestListDeliveries_PurgedPhoto: the record outlives the photograph, which is
// what it is for. It still names the id; only image_available changes.
func TestListDeliveries_PurgedPhoto(t *testing.T) {
	e, h, db := newDeviceHandler(t, 20)
	insertDelivery(t, db, &model.DeliveryHistory{
		DisplayKey: "alpha", Seq: 1, DeliveredAt: time.Now(), Kind: model.DeliveryKindPhoto,
		ImageID: 99, CatalogKey: "photos", Source: "/mnt/photos/gone.jpg", SleepSeconds: 300,
	})

	rec, err := getDeliveries(t, h, e, "alpha", "")
	assert.NoError(t, err)

	var body deliveryListBody
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	if assert.Len(t, body.Deliveries, 1) {
		assert.NotNil(t, body.Deliveries[0].ImageID)
		assert.False(t, body.Deliveries[0].ImageAvailable)
	}
}

// TestListDeliveries_ConfiguredButSilent: an empty history is an empty list,
// not a 404 — the two mean different things and the endpoint must not conflate
// them.
func TestListDeliveries_ConfiguredButSilent(t *testing.T) {
	e, h, _ := newDeviceHandler(t, 20)

	rec, err := getDeliveries(t, h, e, "zeta", "")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"deliveries":[]`)
}

func TestListDeliveries_UnknownDisplay(t *testing.T) {
	e, h, _ := newDeviceHandler(t, 20)

	_, err := getDeliveries(t, h, e, "nonexistent", "")
	var he *echo.HTTPError
	if assert.ErrorAs(t, err, &he) {
		assert.Equal(t, http.StatusNotFound, he.Code)
	}
}

// TestListDeliveries_WithoutHistoryTable is the un-migrated installation, which
// is the state every installation that predates this feature boots in:
// WISP_AUTO_MIGRATE is off by default, so the table does not exist until
// somebody turns it on. The README promises those operators an empty history
// and a warning in the log rather than an error, and /api/devices beside this
// already behaves that way — a 500 here would fail the drawer on exactly the
// installations the graceful path was written for, and fail it once per
// configured display, since the UI asks for one of these lists for each.
func TestListDeliveries_WithoutHistoryTable(t *testing.T) {
	e, h, _ := newDeviceHandlerWith(t, config.DeliveryHistoryConfig{Size: 20}, false)

	rec, err := getDeliveries(t, h, e, "alpha", "")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"deliveries":[]`)
	// The key is still the one asked for: an empty history is this display's
	// empty history, not an anonymous one.
	assert.Contains(t, rec.Body.String(), `"device_key":"alpha"`)
}

// TestListDeliveries_UnknownDisplayWithoutHistoryTable: answering an unreadable
// history with an empty list must not swallow the one error this endpoint is
// supposed to report. An unconfigured key is a client error and stays a 404
// whether or not the table exists.
func TestListDeliveries_UnknownDisplayWithoutHistoryTable(t *testing.T) {
	e, h, _ := newDeviceHandlerWith(t, config.DeliveryHistoryConfig{Size: 20}, false)

	_, err := getDeliveries(t, h, e, "nonexistent", "")
	var he *echo.HTTPError
	if assert.ErrorAs(t, err, &he) {
		assert.Equal(t, http.StatusNotFound, he.Code)
	}
}

// TestListDevices_WithoutHistoryTable is the endpoint beside it, asserted here
// so that the pair is pinned together: whatever a later change does to one of
// them, it cannot quietly leave the two disagreeing about the un-migrated case.
func TestListDevices_WithoutHistoryTable(t *testing.T) {
	e, h, _ := newDeviceHandlerWith(t, config.DeliveryHistoryConfig{Size: 20}, false)

	rec := getDevices(t, h, e)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body deviceListBody
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Devices, 2)
	for _, dev := range body.Devices {
		assert.Nil(t, dev.LastDeliveredAt, dev.Key)
		assert.Zero(t, dev.RecentDeliveryCount, dev.Key)
	}
}

// TestListDeliveries_Limit: the rows are written straight in rather than
// through Record, so the store holds more than the window and a clamp that did
// nothing would show.
func TestListDeliveries_Limit(t *testing.T) {
	e, h, db := newDeviceHandler(t, 3)
	now := time.Now().UTC().Truncate(time.Second)
	for i := 1; i <= 5; i++ {
		insertDelivery(t, db, &model.DeliveryHistory{
			DisplayKey: "alpha", Seq: int64(i), DeliveredAt: now.Add(time.Duration(i) * time.Minute),
			Kind: model.DeliveryKindPhoto, ImageID: 1, CatalogKey: "photos", SleepSeconds: 300,
		})
	}

	for _, tc := range []struct {
		name  string
		query string
		want  int
	}{
		{"absent", "", 3},
		{"over the window", "?limit=99", 3},
		{"under the window", "?limit=2", 2},
		{"zero", "?limit=0", 3},
		{"negative", "?limit=-1", 3},
		{"not a number", "?limit=lots", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := getDeliveries(t, h, e, "alpha", tc.query)
			assert.NoError(t, err)

			var body deliveryListBody
			assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Len(t, body.Deliveries, tc.want)
		})
	}
}

// TestListDevices_RecordingDisabled: with recording off the counts stop moving,
// and frozen numbers presented as current would read as "this frame stopped"
// when what stopped was the bookkeeping. The flag says which it is. The counts
// themselves are left alone — the stored deliveries are real facts about the
// past.
func TestListDevices_RecordingDisabled(t *testing.T) {
	e, h, db := newDeviceHandlerWith(t, config.DeliveryHistoryConfig{Disabled: true, Size: 20}, true)
	insertDelivery(t, db, &model.DeliveryHistory{
		DisplayKey: "alpha", Seq: 1, DeliveredAt: time.Now(), Kind: model.DeliveryKindPhoto, ImageID: 1,
	})

	rec := getDevices(t, h, e)
	assert.Contains(t, rec.Body.String(), `"recording_enabled":false`)

	var body deviceListBody
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.False(t, body.RecordingEnabled)
	assert.Equal(t, 20, body.RecentWindow, "the configured window is still reported")
	assert.Equal(t, 1, body.Devices[0].RecentDeliveryCount, "stored deliveries are not suppressed")
	assert.NotNil(t, body.Devices[0].LastDeliveredAt)
}

// TestListDeliveries_ReasonPassesThroughVerbatim pins the property that keeps
// this field working as codes are added: the handler stores no list of them and
// translates nothing, so whatever the provider wrote comes back as it stands.
//
// The cases are the two that must never be collapsed — load_failed means the
// picture would not load, and sends an operator to the file; encode_failed
// means it loaded and then would not convert to this panel's format, and sends
// them to the display model — plus a code this build has never heard of, which
// stands in for the next one somebody adds.
func TestListDeliveries_ReasonPassesThroughVerbatim(t *testing.T) {
	for _, reason := range []model.DeliveryReason{
		model.DeliveryReasonLoadFailed,
		model.DeliveryReasonEncodeFailed,
		model.DeliveryReason("a_reason_this_build_has_never_heard_of"),
	} {
		t.Run(string(reason), func(t *testing.T) {
			e, h, db := newDeviceHandler(t, 20)
			insertDelivery(t, db, &model.DeliveryHistory{
				DisplayKey: "alpha", Seq: 1, DeliveredAt: time.Now(),
				Kind: model.DeliveryKindError, Reason: reason, CatalogKey: "photos", SleepSeconds: 300,
			})

			rec, err := getDeliveries(t, h, e, "alpha", "")
			assert.NoError(t, err)

			var body deliveryListBody
			assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			if assert.Len(t, body.Deliveries, 1) && assert.NotNil(t, body.Deliveries[0].Reason) {
				assert.Equal(t, string(reason), *body.Deliveries[0].Reason)
			}
		})
	}
}

// TestListDeliveries_NoCatalogueIsNull covers the field's absent side. A colour
// bar is generated rather than drawn from anywhere, so it consulted no
// catalogue, named no file and came from no images row — all three report null
// rather than the empty value that would read as an answer.
//
// Checked against the raw body as well as the decoded one: "" and null both
// decode into a *string the same way, so only the bytes can tell them apart.
func TestListDeliveries_NoCatalogueIsNull(t *testing.T) {
	e, h, db := newDeviceHandler(t, 20)
	insertDelivery(t, db, &model.DeliveryHistory{
		DisplayKey: "alpha", Seq: 1, DeliveredAt: time.Now(),
		Kind: model.DeliveryKindColorbar, SleepSeconds: 300,
	})

	rec, err := getDeliveries(t, h, e, "alpha", "")
	assert.NoError(t, err)
	assert.Contains(t, rec.Body.String(), `"catalog_key":null`)

	var body deliveryListBody
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	if assert.Len(t, body.Deliveries, 1) {
		entry := body.Deliveries[0]
		assert.Equal(t, "colorbar", entry.Kind)
		assert.Nil(t, entry.CatalogKey, "a colour bar consulted no catalogue")
		assert.Nil(t, entry.Source)
		assert.Nil(t, entry.ImageID)
		assert.Nil(t, entry.Reason, "a colour bar is not an error")
	}
}

// TestListDeliveries_ErrorWithCatalogueKeepsIt: null on catalog_key means "no
// catalogue was involved", not "this went wrong". A provider that gave up knew
// which catalogue it was reading, and that key is worth more than the absence
// would be — it says where to go looking.
func TestListDeliveries_ErrorWithCatalogueKeepsIt(t *testing.T) {
	e, h, db := newDeviceHandler(t, 20)
	insertDelivery(t, db, &model.DeliveryHistory{
		DisplayKey: "alpha", Seq: 1, DeliveredAt: time.Now(),
		Kind: model.DeliveryKindError, Reason: model.DeliveryReasonNoImages,
		CatalogKey: "photos", SleepSeconds: 300,
	})

	rec, err := getDeliveries(t, h, e, "alpha", "")
	assert.NoError(t, err)

	var body deliveryListBody
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	if assert.Len(t, body.Deliveries, 1) && assert.NotNil(t, body.Deliveries[0].CatalogKey) {
		assert.Equal(t, "photos", *body.Deliveries[0].CatalogKey)
	}
}

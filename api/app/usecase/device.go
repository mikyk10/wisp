package usecase

import (
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/mikyk10/wisp/app/domain/catalog"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
)

// DeviceStatus is one configured display together with what the delivery
// history says about it.
//
// Everything down to WakeSchedule comes from service.yaml and is true whether
// or not the panel has ever been heard from. The three fields after it come
// from the history, and are all zero for a display that has never been
// delivered to.
type DeviceStatus struct {
	Key                  string
	Name                 string
	Model                string
	Width                int
	Height               int
	Orientation          string
	CatalogKeys          []string
	SleepDurationSeconds int
	WakeSchedule         []string

	// LastDeliveredAt is when this display was last handed a picture, and the
	// zero time when it never has been. Delivered is the strongest word the
	// server is entitled to: it wrote the bytes out, and whether the panel was
	// awake to receive them or able to render them is not something any of this
	// can know.
	LastDeliveredAt time.Time

	// RecentDeliveryCount and RecentErrorCount cover only the deliveries still
	// held, so both are bounded by RecentWindow. They are not lifetime totals —
	// the store overwrites its oldest entry and keeps no count of what it
	// dropped.
	RecentDeliveryCount int
	RecentErrorCount    int
}

// DeviceUsecase answers what an operator needs to know about the panels: which
// ones are configured, and what each of them was last sent.
type DeviceUsecase interface {
	// ListDevices returns every display named in service.yaml, ordered by key.
	//
	// The list is built from the configuration and never from the history, so a
	// frame that has never once phoned home is present and visibly silent
	// rather than missing — which is the single most useful thing this view
	// has to say.
	//
	// The order is fixed because Go iterates a map at random, and a list that
	// reshuffles on every poll is unreadable. ListCatalogs sorts for the same
	// reason.
	ListDevices() ([]*DeviceStatus, error)

	// ListDeliveries returns displayKey's recent deliveries, newest first. A
	// display that has never been delivered to returns an empty slice, and so
	// does one whose history cannot be read at all; a key that is not
	// configured is a *catalog.DisplayNotFoundError.
	//
	// limit is clamped to RecentWindow, and anything at or below zero asks for
	// as many as are kept.
	ListDeliveries(displayKey string, limit int) ([]*repository.DeliveryHistoryEntry, error)

	// RecentWindow is how many deliveries each display keeps: the bound on both
	// counts in DeviceStatus and on the length of a ListDeliveries result.
	RecentWindow() int

	// RecordingEnabled says whether deliveries are being written down right
	// now. With it off the counts in DeviceStatus stop moving, and a reader
	// that did not know would take frozen numbers for current ones.
	RecordingEnabled() bool
}

type deviceUsecase struct {
	globalConfig  *config.GlobalConfig
	serviceConfig *config.ServiceConfig
	dhr           repository.DeliveryHistoryRepository
}

func NewDeviceUsecase(globalConfig *config.GlobalConfig, serviceConfig *config.ServiceConfig, dhr repository.DeliveryHistoryRepository) DeviceUsecase {
	return &deviceUsecase{
		globalConfig:  globalConfig,
		serviceConfig: serviceConfig,
		dhr:           dhr,
	}
}

func (uc *deviceUsecase) RecentWindow() int {
	return uc.globalConfig.DeliveryHistory.Size
}

// RecordingEnabled reports the condition under which a row is actually written,
// not merely the state of the switch. Record stores nothing for a size of zero
// or less either, and answering "on" for a ring with no room in it would be a
// setting reported back rather than a fact.
func (uc *deviceUsecase) RecordingEnabled() bool {
	return !uc.globalConfig.DeliveryHistory.Disabled && uc.globalConfig.DeliveryHistory.Size > 0
}

func (uc *deviceUsecase) ListDevices() ([]*DeviceStatus, error) {
	summaries := uc.summaries()

	keys := slices.Sorted(maps.Keys(uc.serviceConfig.Displays))
	devices := make([]*DeviceStatus, 0, len(keys))
	for _, key := range keys {
		devices = append(devices, uc.deviceStatus(uc.serviceConfig.Displays[key], summaries[key]))
	}

	return devices, nil
}

// summaries reads every display's history in one query, keyed by display.
//
// One query rather than one per display: the caller is a list view, and nothing
// here can assume the number of panels stays small. A display with no history
// is simply absent from the map.
//
// A history that cannot be read is not a reason to withhold the configuration.
// The table is missing on every installation that predates this feature and has
// never been migrated — WISP_AUTO_MIGRATE is off by default — and failing here
// would take the whole device list down with it on exactly those installations.
// The panels still come back, each reading as never delivered to, and the
// failure goes to the log.
func (uc *deviceUsecase) summaries() map[string]*repository.DeliverySummary {
	byDisplay := map[string]*repository.DeliverySummary{}
	if uc.dhr == nil {
		return byDisplay
	}

	rows, err := uc.dhr.SummaryByDisplay()
	if err != nil {
		slog.Warn("delivery history: summary failed; listing devices without it", "err", err)
		return byDisplay
	}

	for _, row := range rows {
		byDisplay[row.DisplayKey] = row
	}

	return byDisplay
}

// deviceStatus joins one display's configuration to its history. summary is nil
// for a display that has never been delivered to.
func (uc *deviceUsecase) deviceStatus(conf *config.DisplayConfig, summary *repository.DeliverySummary) *DeviceStatus {
	// NewDisplay panics on a model it does not know rather than reporting one.
	// That is safe here only because validateDisplay refuses an unknown model
	// while the configuration is being read, so nothing that reaches
	// ServiceConfig can name one.
	display := epaper.NewDisplay(epaper.EPaperDisplayModel(conf.DisplayModel), model.CanonicalOrientation(conf.Orientation))

	// Both slices are built empty rather than left nil, or they marshal as null
	// instead of []. ListCatalogs has that bug today.
	catalogKeys := make([]string, 0, len(conf.Catalog))
	for _, assoc := range conf.Catalog {
		catalogKeys = append(catalogKeys, assoc.ProviderConfig.Key)
	}
	wakeSchedule := make([]string, 0, len(conf.WakeSchedule))
	wakeSchedule = append(wakeSchedule, conf.WakeSchedule...)

	status := &DeviceStatus{
		Key:                  conf.Key,
		Name:                 conf.Name,
		Model:                conf.DisplayModel,
		Width:                display.Width(),
		Height:               display.Height(),
		Orientation:          orientationName(conf.Orientation),
		CatalogKeys:          catalogKeys,
		SleepDurationSeconds: conf.SleepDurationSeconds,
		WakeSchedule:         wakeSchedule,
	}

	if summary != nil {
		status.LastDeliveredAt = summary.LastDeliveredAt
		status.RecentDeliveryCount = summary.Entries
		status.RecentErrorCount = summary.ErrorEntries
	}

	return status
}

// orientationName spells a display orientation the way service.yaml does.
//
// config.DisplayOrientation has a parser and no String(), and the domain is not
// this change's to edit, so the mapping lives here. The default matches
// NewDisplayOrientation's: anything the configuration does not recognise is
// read as landscape, and the parser never produces the None value at all.
func orientationName(o config.DisplayOrientation) string {
	if o == config.DisplayOrientationPortrait {
		return "portrait"
	}
	return "landscape"
}

func (uc *deviceUsecase) ListDeliveries(displayKey string, limit int) ([]*repository.DeliveryHistoryEntry, error) {
	// Checked against the configuration, not against the history. A display
	// that is configured but has never been delivered to has no rows and must
	// answer with an empty list, while a key that was never configured is a
	// 404; reading "no rows" as "no such device" would swap the two.
	if _, ok := uc.serviceConfig.Displays[displayKey]; !ok {
		return nil, &catalog.DisplayNotFoundError{Key: displayKey}
	}

	// Not dead code, however unreachable the container makes it. summaries()
	// guards this and RecordDelivery guards it, and the whole of the bug fixed
	// below was these two methods disagreeing about one failure — each
	// defensible alone. A third state where one degrades and the other panics
	// is the same bug waiting, so the guard is here to keep the pair saying the
	// same thing rather than to defend against a wiring the DI prevents.
	if uc.dhr == nil {
		return []*repository.DeliveryHistoryEntry{}, nil
	}

	// Clamped to the window because that is all the store holds: a larger limit
	// could only promise more than there is.
	window := uc.RecentWindow()
	if limit <= 0 || limit > window {
		limit = window
	}

	entries, err := uc.dhr.ListByDisplay(displayKey, limit)
	if err != nil {
		// The same stance summaries() takes, and for the same reason. The table
		// is missing on every installation that predates this feature and has
		// never been migrated — WISP_AUTO_MIGRATE is off by default — and the
		// README promises those operators that "until the table exists the
		// symptom is an empty history and a warning in the log rather than an
		// error". An installation that has not opted into the migration is not
		// broken; it has simply not set the history up.
		//
		// It matters more here than the single endpoint suggests: the catalogue
		// UI asks for one of these lists per configured display when the drawer
		// opens, so propagating would fail every strip at once, beside a device
		// list that renders perfectly from the same missing table.
		//
		// Any read failure, not just a missing table. Telling the two apart
		// would mean matching driver text, which differs per dialect and would
		// leave this behaving differently on SQLite, MySQL and PostgreSQL.
		// summaries() already swallows whatever it is handed, and the two paths
		// are only coherent if this one does the same.
		slog.Warn("delivery history: list failed; answering with an empty history",
			"display", displayKey, "err", err)
		return []*repository.DeliveryHistoryEntry{}, nil
	}

	return entries, nil
}

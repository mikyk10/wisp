package response

import (
	"github.com/mikyk10/wisp/app/domain/model"
)

// DeviceList is every display named in service.yaml, whether or not it has ever
// been heard from.
type DeviceList struct {
	// RecordingEnabled says whether deliveries are being written down at all
	// right now.
	//
	// It qualifies everything below it. With recording off the counts stop
	// moving while RecentWindow goes on reading like a live setting, and frozen
	// numbers presented as current would say "this frame stopped" when what
	// stopped was the bookkeeping — the same confusion as reporting
	// "displaying" for what is only "delivered". The counts are still reported
	// as they stand, because the old rows are real facts about the past; this
	// only lets a reader know they are the past.
	RecordingEnabled bool `json:"recording_enabled"`

	// RecentWindow is how many deliveries each display keeps, and so the bound
	// on both counts in Device below.
	RecentWindow int `json:"recent_window"`

	Devices []*Device `json:"devices"`
}

// Device is one configured display and what the delivery history says about it.
type Device struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Model       string `json:"model"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Orientation string `json:"orientation"`

	// CatalogKeys and WakeSchedule are always arrays, never null: a panel with
	// no wake schedule has an empty one, and a client should not have to tell
	// that apart from a field the server forgot to send.
	CatalogKeys          []string `json:"catalog_keys"`
	SleepDurationSeconds int      `json:"sleep_duration_seconds"`
	WakeSchedule         []string `json:"wake_schedule"`

	// LastDeliveredAt is null for a display that has never been delivered to.
	//
	// Null rather than the empty string Image.Timestamp uses for a missing EXIF
	// date, and the difference is deliberate: "never delivered to" is the most
	// useful thing this endpoint says about a frame, and an empty string in a
	// timestamp field reads as a formatting slip rather than as an answer.
	LastDeliveredAt *string `json:"last_delivered_at"`

	// RecentDeliveryCount and RecentErrorCount cover only the deliveries still
	// held, so both are bounded by RecentWindow. They are not lifetime totals —
	// the store overwrites its oldest entry and keeps no count of what it
	// dropped.
	RecentDeliveryCount int `json:"recent_delivery_count"`
	RecentErrorCount    int `json:"recent_error_count"`
}

// DeliveryList is one display's recent deliveries, newest first.
type DeliveryList struct {
	DeviceKey  string      `json:"device_key"`
	Deliveries []*Delivery `json:"deliveries"`
}

// Delivery is one picture handed to one display.
//
// Handed over is all it says. The record is written after the body has gone out
// and nothing can be added to it afterwards, so nothing here reports what the
// panel did with the bytes — or whether it was awake to receive them.
type Delivery struct {
	DeliveredAt string `json:"delivered_at"`
	Kind        string `json:"kind"`

	// Reason says why an error card was shown, and is null for every other
	// kind. It is a code, not prose: the provider decides it at the moment it
	// gives up and nothing downstream can work it out afterwards, so this is
	// the only place the distinction survives. Rendering it for a reader is the
	// client's job, where the wording can change without a server release.
	Reason *string `json:"reason"`

	// ImageID is null when the picture did not come from the catalogue: a
	// colour bar, an error card, or a live HTTP fetch. Zero is what is stored,
	// and zero in this field would read as a photograph.
	ImageID *model.PrimaryKey `json:"image_id"`

	// CatalogKey is null for a delivery that consulted no catalogue at all: a
	// colour bar, or an error raised before one had been chosen. An error card
	// from a provider that did have a catalogue still names it, so null here
	// means "no catalogue was involved" and not "this went wrong".
	CatalogKey *string `json:"catalog_key"`

	// Source is null for a delivery that names no file.
	Source *string `json:"source"`

	// RequestedSleepSeconds is the X-Sleep-Seconds this response carried — what
	// the server asked for, not an interval the device is known to have kept. A
	// panel that never received the response fell back to its own.
	RequestedSleepSeconds int `json:"requested_sleep_seconds"`

	// ImageAvailable says whether the images row this delivery names is still
	// there. False for a delivery that never had one, and false once the
	// photograph has been purged; a photograph merely hidden by the user is
	// still there, so it stays true.
	ImageAvailable bool `json:"image_available"`
}

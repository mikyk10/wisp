package model

import "time"

// DeliveryKind names which delivery path produced a picture.
type DeliveryKind string

const (
	DeliveryKindPhoto    DeliveryKind = "photo"    // file catalogue, rendered from disk
	DeliveryKindHTTP     DeliveryKind = "http"     // HTTP catalogue, cached in the DB or fetched live
	DeliveryKindColorbar DeliveryKind = "colorbar" // generated test pattern
	DeliveryKindError    DeliveryKind = "error"    // error card rendered instead of a photo
)

// DeliveryReason says why an error card was shown. It is empty for every other
// kind.
//
// The provider decides this at the moment it gives up, and nothing downstream
// can work it out afterwards: the file provider reports success either way, so
// by the time the handler sees the loader the distinction is already gone.
type DeliveryReason string

const (
	DeliveryReasonNone           DeliveryReason = ""
	DeliveryReasonNoImages       DeliveryReason = "no_images"       // the catalogue holds nothing to show
	DeliveryReasonDBError        DeliveryReason = "db_error"        // the catalogue could not be queried
	DeliveryReasonFileMissing    DeliveryReason = "file_missing"    // the indexed file is gone from disk
	DeliveryReasonNoCatalog      DeliveryReason = "no_catalog"      // the display has no catalogue assigned
	DeliveryReasonNoProvider     DeliveryReason = "no_provider"     // no provider could serve the request
	DeliveryReasonUnknownDisplay DeliveryReason = "unknown_display" // the requested display is not configured
	DeliveryReasonLoadFailed     DeliveryReason = "load_failed"     // the picture was chosen but would not load
	DeliveryReasonEncodeFailed   DeliveryReason = "encode_failed"   // the picture loaded but would not encode for this panel
)

// DeliveryHistory records one picture handed to one display.
//
// The table is a fixed-size ring. A display holds at most
// GlobalConfig.DeliveryHistory.Size rows, and delivery number Seq is written to
// the row at Seq % Size, replacing whatever was there. Nothing is ever
// appended, so the table cannot grow with time: its size is set by the
// configuration, not by how long the installation has been running.
//
// Slot is storage bookkeeping and nothing else. Seq is the only thing that says
// which delivery came after which, and every read orders by it, so the ring
// size can be changed without making the stored history unreadable.
type DeliveryHistory struct {
	// DisplayKey is the mac_address from service.yaml, which is also the
	// :displayKey path parameter.
	DisplayKey string `gorm:"type:varchar(64);not null;primaryKey;index:idx_delivery_recent,priority:1"`

	// Slot is Seq % Size — which row of the ring this delivery overwrites.
	Slot int `gorm:"not null;primaryKey"`

	// Seq is this display's delivery number, counting from 1. It is always one
	// past the highest Seq the display still holds, which keeps it strictly
	// increasing and unique per display even after rows are removed.
	Seq int64 `gorm:"not null;index:idx_delivery_recent,priority:2"`

	DeliveredAt time.Time    `gorm:"not null"`
	Kind        DeliveryKind `gorm:"type:varchar(16);not null"`

	// ImageID is the images row this came from, or 0 when the picture was not
	// drawn from the catalogue (a live HTTP fetch, a colour bar, an error
	// card).
	//
	// Deliberately not a foreign key: the point of the record is to survive the
	// photograph being deleted, and SQLite enforces foreign keys, so a real
	// reference would either block the delete or take the history with it.
	ImageID PrimaryKey `gorm:"not null;default:0"`

	CatalogKey string `gorm:"type:varchar(64);not null;default:''"`
	Source     string `gorm:"type:varchar(2048);not null;default:''"`

	// Reason is set only when Kind is DeliveryKindError.
	Reason DeliveryReason `gorm:"type:varchar(32);not null;default:''"`

	// SleepSeconds is the X-Sleep-Seconds this response carried. It is computed
	// per request under a wake schedule and cannot be reconstructed afterwards.
	// On an error card it also says whether the panel will come back in an hour
	// or keep its usual interval: the provider-level error paths never reach
	// the handler's error branch, so they carry the display's normal sleep.
	SleepSeconds int `gorm:"not null;default:0"`
}

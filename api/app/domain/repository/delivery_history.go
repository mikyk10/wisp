package repository

import (
	"time"

	"github.com/mikyk10/wisp/app/domain/model"
)

// DeliveryHistoryEntry is one stored delivery together with what can only be
// known by looking at the rest of the database now — namely whether the
// photograph it names is still there.
type DeliveryHistoryEntry struct {
	model.DeliveryHistory

	// ImageAvailable says whether the images row this delivery came from still
	// exists. It is false for a delivery that never had one (a colour bar, an
	// error card, a live HTTP fetch) and false once the photograph has been
	// purged. A photograph merely hidden by the user is still there, so it
	// stays true.
	ImageAvailable bool
}

// DeliverySummary is one display's history reduced to the few numbers a list
// view needs. A display that has never been delivered to has no summary at all.
type DeliverySummary struct {
	DisplayKey      string
	LastDeliveredAt time.Time
	LastSeq         int64
	Entries         int
	ErrorEntries    int
}

// DeliveryHistoryRepository stores what each display was last shown.
//
// The store is a fixed-size ring per display: at most size rows survive, and
// the oldest is overwritten rather than appended to. size is passed in on each
// call rather than held by the repository, following EvictOldestImages, so the
// configuration stays in one place and the repository has no state to keep in
// step with it.
type DeliveryHistoryRepository interface {
	// Record stores one delivery, overwriting the oldest of the display's size
	// entries.
	//
	// It never reports a storage failure to its caller: a delivery that was
	// made is not undone by failing to write it down, and the caller is the
	// request that hands a picture to a panel. Failures are logged at warn and
	// the returned error is always nil. A size of zero or less records nothing,
	// also without error.
	Record(rec *model.DeliveryHistory, size int) error

	// ListByDisplay returns the display's most recent deliveries, newest first.
	// A display with fewer than limit entries returns only what it has, and one
	// with none returns an empty slice.
	ListByDisplay(displayKey string, limit int) ([]*DeliveryHistoryEntry, error)

	// SummaryByDisplay returns one row per display that has any history, in a
	// single query. Displays that have never been delivered to are absent from
	// the result rather than present and empty.
	SummaryByDisplay() ([]*DeliverySummary, error)

	// Reconcile brings the stored history back within the current
	// configuration: it drops entries past the end of a ring that has shrunk,
	// and entries belonging to displays that are no longer configured. An empty
	// displayKeys leaves the second alone rather than deleting everything.
	//
	// Without it a shrunk ring would leave the table permanently above the
	// bound the size is supposed to give it. It is safe to run repeatedly.
	Reconcile(displayKeys []string, size int) error
}

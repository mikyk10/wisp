package repository

import (
	"database/sql/driver"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type deliveryHistoryRepositoryImpl struct {
	db *gorm.DB

	// mu serialises the read-then-write in record.
	//
	// A transaction would be the obvious answer and is the wrong one here. The
	// write itself is a single statement, so there is nothing to roll back; on
	// SQLite a deferred transaction that starts by reading and then upgrades to
	// a write can come back SQLITE_BUSY, and that cost would be paid on every
	// panel wake to protect nothing.
	//
	// The mutex only holds within one process. Run several API replicas against
	// one database and two of them can pick the same Seq, in which case the
	// second overwrites the first — one delivery missing from the history. It
	// cannot corrupt anything or push the table past its size, because the slot
	// is derived from the seq and the primary key makes the write an upsert
	// either way.
	mu sync.Mutex
}

func NewDeliveryHistoryRepositoryImpl(db *gorm.DB) repository.DeliveryHistoryRepository {
	return &deliveryHistoryRepositoryImpl{db: db}
}

// deliveryUpsertColumns are every column of a delivery except the two that make
// up the primary key. Overwriting a ring slot rewrites all of them.
var deliveryUpsertColumns = []string{
	"seq", "delivered_at", "kind", "image_id", "catalog_key", "source", "reason", "sleep_seconds",
}

func (r *deliveryHistoryRepositoryImpl) Record(rec *model.DeliveryHistory, size int) error {
	if rec == nil || size <= 0 {
		return nil
	}

	// Deliberately swallowed. This runs on the request that hands a picture to
	// a panel, and the picture has already been chosen and rendered by the time
	// we get here — a history that cannot be written is not a reason to send
	// the device away empty. The likeliest failure by far is the table not
	// existing at all: WISP_AUTO_MIGRATE is off by default (main.go, and
	// commented out in compose.yaml), so on every installation that predates
	// this feature the first thing it meets is a missing table on every wake.
	if err := r.record(rec, size); err != nil {
		slog.Warn("delivery history: record failed",
			"display", rec.DisplayKey, "kind", rec.Kind, "image", rec.ImageID, "err", err)
	}

	return nil
}

// record writes one delivery into the display's ring.
//
// Two statements: read the display's next sequence number, then upsert the row
// it lands on. Slots are never pre-allocated — a ring that has not filled up
// simply holds fewer rows. There is no dialect-neutral value to fill an unused
// slot with anyway: Go's zero time.Time serialises to 0000-00-00 00:00:00,
// which MySQL rejects under the default NO_ZERO_DATE. Growing lazily also means
// no read ever has to filter placeholders out, and "never delivered to" is zero
// rows rather than a value anyone has to recognise.
func (r *deliveryHistoryRepositoryImpl) record(rec *model.DeliveryHistory, size int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var seq int64
	if err := r.db.Model(&model.DeliveryHistory{}).
		Where("display_key = ?", rec.DisplayKey).
		Select("COALESCE(MAX(seq), 0) + 1").
		Scan(&seq).Error; err != nil {
		return err
	}

	rec.Seq = seq
	rec.Slot = int(seq % int64(size))
	if rec.DeliveredAt.IsZero() {
		// Never store a zero time: MySQL refuses it outright under the default
		// NO_ZERO_DATE, which would turn a caller's oversight into a failed
		// write on every delivery.
		rec.DeliveredAt = time.Now()
	}

	// Create, not Save. The upserts on images next door use Save and get away
	// with it because an Image's autoincrement primary key is unset at that
	// point, so GORM treats it as an insert. Here both primary-key fields are
	// always set, so Save would take its update path: an UPDATE first, an
	// INSERT only if it matched no rows — and on that path GORM substitutes its
	// own OnConflict{UpdateAll: true} for the clause written below, so the
	// column list stops meaning anything and every delivery costs two
	// statements instead of one.
	//
	// On MySQL the driver drops the conflict-target columns and writes
	// ON DUPLICATE KEY UPDATE, which fires on any unique key rather than the
	// named one. That is the same thing here only because the primary key is
	// the sole unique constraint on this table. Adding a second unique index
	// would silently change what this statement does on MySQL, and no test
	// against SQLite would notice.
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "display_key"}, {Name: "slot"}},
		DoUpdates: clause.AssignmentColumns(deliveryUpsertColumns),
	}).Create(rec).Error
}

func (r *deliveryHistoryRepositoryImpl) ListByDisplay(displayKey string, limit int) ([]*repository.DeliveryHistoryEntry, error) {
	entries := []*repository.DeliveryHistoryEntry{}
	if limit <= 0 {
		return entries, nil
	}

	// Ordered by seq, never by slot: slot is where a delivery is stored, seq is
	// when it happened, and the two only agree until the ring wraps. This is
	// what idx_delivery_recent is for.
	//
	// image_available is spelled as a CASE rather than the bare `i.id IS NOT
	// NULL` predicate so that the value comes back as 1 or 0 on all dialects
	// instead of a boolean on some and an integer on others. image_id is 0 for
	// a colour bar, an error card or a live HTTP fetch, and no images row can
	// have id 0, so those come back unavailable without needing a special case.
	//
	// The soft-delete filter is worth being deliberate about: GORM adds
	// `deleted_at IS NULL` for the primary model only, and the primary model
	// here is DeliveryHistory, which has no such column — images is merely
	// joined, so nothing is added for it. A photograph the user has hidden
	// therefore still reports available, which is what we want and what
	// FindById's explicit Unscoped already assumes; only a real purge makes it
	// false. Make images the primary model in some later refactor and this
	// flips without a word.
	err := r.db.Model(&model.DeliveryHistory{}).
		Table("delivery_histories h").
		Select("h.*, CASE WHEN i.id IS NULL THEN 0 ELSE 1 END AS image_available").
		Joins("LEFT JOIN images i ON i.id = h.image_id").
		Where("h.display_key = ?", displayKey).
		Order("h.seq DESC").
		Limit(limit).
		Scan(&entries).Error
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// aggregatedTime reads a timestamp that has been through an aggregate function.
//
// SQLite reports a result column's type only when that column is a table column
// declared as something; MAX(delivered_at) is an expression, so it comes back
// untyped and the driver hands over the stored text. Reading it straight into a
// time.Time therefore fails with "unsupported Scan, storing driver.Value type
// string into type *time.Time" — and only on SQLite, since MySQL sends a typed
// DATETIME that its driver parses for us when the DSN says parseTime. Both
// shapes land here.
//
// Value exists only because GORM refuses a struct field that implements one
// half of the Scanner/Valuer pair and not the other; nothing writes through
// this type.
type aggregatedTime struct {
	t time.Time
}

func (a aggregatedTime) Value() (driver.Value, error) { return a.t, nil }

// aggregatedTimeLayouts are the shapes a stored timestamp comes back as when
// the driver has not parsed it: SQLite's own, and MySQL's when parseTime is
// absent from the DSN.
var aggregatedTimeLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	time.RFC3339Nano,
}

func (a *aggregatedTime) Scan(v any) error {
	switch d := v.(type) {
	case nil:
		return nil
	case time.Time:
		a.t = d
	case []byte:
		return a.Scan(string(d))
	case string:
		for _, layout := range aggregatedTimeLayouts {
			if t, err := time.Parse(layout, d); err == nil {
				a.t = t
				return nil
			}
		}
		return fmt.Errorf("delivery history: cannot read timestamp %q", d)
	default:
		return fmt.Errorf("delivery history: cannot read timestamp of type %T", v)
	}

	return nil
}

// deliverySummaryRow is the scan target for SummaryByDisplay. It exists only so
// that aggregatedTime stays inside this package and the domain type keeps a
// plain time.Time.
type deliverySummaryRow struct {
	DisplayKey      string
	LastDeliveredAt aggregatedTime
	LastSeq         int64
	Entries         int
	ErrorEntries    int
}

func (r *deliveryHistoryRepositoryImpl) SummaryByDisplay() ([]*repository.DeliverySummary, error) {
	// One query covering every display rather than one query per display: the
	// caller is a list view, and nothing here can assume the number of displays
	// stays small.
	//
	// SUM(CASE WHEN ...) rather than either of the shorter spellings, both of
	// which are dialect-specific: COUNT(*) FILTER (WHERE ...) is SQLite and
	// PostgreSQL only, and SUM(kind = 'error') leans on MySQL coercing a
	// boolean to an integer.
	//
	// A display with no history has no row in this table and so no row in the
	// result; saying "never delivered to" is the caller's job.
	rows := []*deliverySummaryRow{}
	err := r.db.Model(&model.DeliveryHistory{}).
		Select("display_key, MAX(delivered_at) AS last_delivered_at, MAX(seq) AS last_seq, COUNT(*) AS entries, SUM(CASE WHEN kind = ? THEN 1 ELSE 0 END) AS error_entries",
			model.DeliveryKindError).
		Group("display_key").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	summaries := make([]*repository.DeliverySummary, len(rows))
	for i, row := range rows {
		summaries[i] = &repository.DeliverySummary{
			DisplayKey:      row.DisplayKey,
			LastDeliveredAt: row.LastDeliveredAt.t,
			LastSeq:         row.LastSeq,
			Entries:         row.Entries,
			ErrorEntries:    row.ErrorEntries,
		}
	}

	return summaries, nil
}

func (r *deliveryHistoryRepositoryImpl) Reconcile(displayKeys []string, size int) error {
	// Rows past the end of the ring. Shrinking the size does not by itself
	// remove anything — the slots simply stop being written to — so without
	// this the table would sit above its configured bound for good.
	//
	// Guarded on size > 0 because a nonsensical size should leave the history
	// alone rather than delete all of it.
	if size > 0 {
		if err := r.db.Where("slot >= ?", size).Delete(&model.DeliveryHistory{}).Error; err != nil {
			return err
		}
	}

	// Displays that have been removed from the configuration. Skipped when the
	// caller has no keys at all: that reads as "nothing configured yet", not as
	// "delete every display's history".
	if len(displayKeys) == 0 {
		return nil
	}

	return r.db.Where("display_key NOT IN ?", displayKeys).Delete(&model.DeliveryHistory{}).Error
}

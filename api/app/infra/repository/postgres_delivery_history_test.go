package repository_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"

	"gorm.io/gorm"
)

// The delivery history was written and tested against in-memory SQLite alone,
// which types nothing and accepts nearly everything. Everything it does that a
// dialect can disagree about is here: a composite primary key used as the
// target of an ON CONFLICT, a LEFT JOIN whose CASE has to come back as
// something a Go bool will take, four aggregates whose PostgreSQL result types
// are wider than the Go fields they are scanned into, and a NOT IN over a
// slice. Each is exercised against the live server named by WISP_TEST_PG_DSN
// rather than reasoned about; with the variable unset they all skip.
//
// setupPostgres, dummyImage, deliveryRec, countDeliveryRows, recordN and
// insertImage are shared with the SQLite tests next door, so what is asserted
// here is the same behaviour on a different server and not a second definition
// of it.

// setupPostgresDeliveryRepo is the usual entry point: a migrated schema and the
// delivery history repository sitting on it.
func setupPostgresDeliveryRepo(t *testing.T) (repository.DeliveryHistoryRepository, *gorm.DB, *sqlRecorder) {
	t.Helper()
	conn, rec := setupPostgres(t)
	return infraRepo.NewDeliveryHistoryRepositoryImpl(conn), conn, rec
}

// --- AutoMigrate -----------------------------------------------------------

// TestPostgres_DeliveryHistory_Schema is the floor the rest stands on. The
// model carries a composite primary key, a secondary index, and an ImageID
// typed as an unsigned integer Go-side — which PostgreSQL has no equivalent
// for, so what it becomes is worth recording rather than assuming.
func TestPostgres_DeliveryHistory_Schema(t *testing.T) {
	conn, _ := setupPostgres(t)

	if !conn.Migrator().HasTable(&model.DeliveryHistory{}) {
		t.Fatal("delivery_histories was not created")
	}

	for _, c := range []struct{ column, want string }{
		{"display_key", "character varying(64)"},
		{"slot", "bigint"},
		{"seq", "bigint"},
		{"delivered_at", "timestamp with time zone"},
		{"kind", "character varying(16)"},
		// PrimaryKey is a uint64 and PostgreSQL has no unsigned integer, so the
		// driver widens it to a signed bigint. Nothing is lost for a real id:
		// images.id is a bigserial and so is positive and inside the same
		// range. See TestPostgres_DeliveryHistory_ImageIDRange.
		{"image_id", "bigint"},
		{"catalog_key", "character varying(64)"},
		{"source", "character varying(2048)"},
		{"reason", "character varying(32)"},
		{"sleep_seconds", "bigint"},
	} {
		if got := pgColumnType(t, conn, "delivery_histories", c.column); got != c.want {
			t.Errorf("delivery_histories.%s is %q, want %q", c.column, got, c.want)
		}
	}

	// Every column is NOT NULL: the read path scans straight into non-pointer
	// fields, and the ring's own bookkeeping has no absent value.
	var nullable []string
	if err := conn.Raw(
		"SELECT a.attname FROM pg_attribute a WHERE a.attrelid = 'delivery_histories'::regclass " +
			"AND a.attnum > 0 AND NOT a.attisdropped AND NOT a.attnotnull",
	).Scan(&nullable).Error; err != nil {
		t.Fatalf("read nullability: %v", err)
	}
	if len(nullable) != 0 {
		t.Errorf("these columns are nullable: %v", nullable)
	}

	// The primary key is what makes the write an upsert, and its columns are
	// the conflict target Record names. An index of another shape, or in
	// another order, would leave the ring appending instead of overwriting.
	if got := pgIndexDef(t, conn, "delivery_histories_pkey"); !strings.Contains(got, "(display_key, slot)") {
		t.Errorf("primary key is %q, want one over (display_key, slot)", got)
	}
	if got := pgIndexDef(t, conn, "idx_delivery_recent"); !strings.Contains(got, "(display_key, seq)") {
		t.Errorf("idx_delivery_recent is %q, want one over (display_key, seq)", got)
	}

	// Start-up migrates whatever is already there, so a second pass over an
	// up-to-date schema has to be a no-op rather than an error.
	if err := conn.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("second AutoMigrate: %v", err)
	}
}

// pgIndexDef returns the CREATE INDEX statement PostgreSQL holds for an index.
func pgIndexDef(t *testing.T, conn *gorm.DB, name string) string {
	t.Helper()
	var def string
	if err := conn.Raw("SELECT indexdef FROM pg_indexes WHERE indexname = ?", name).Scan(&def).Error; err != nil {
		t.Fatalf("read index %s: %v", name, err)
	}
	if def == "" {
		t.Fatalf("index %s does not exist", name)
	}
	return def
}

// --- the ring upsert -------------------------------------------------------

// TestPostgres_DeliveryHistory_RingUpsert is the statement the whole design
// rests on. The conflict target is a composite primary key, and PostgreSQL —
// unlike MySQL, which ignores the named columns and fires on any unique key —
// resolves it literally: get the target wrong and the ring appends for ever
// instead of overwriting, which is the one thing a fixed-size ring exists to
// prevent.
func TestPostgres_DeliveryHistory_RingUpsert(t *testing.T) {
	repo, conn, rec := setupPostgresDeliveryRepo(t)

	const size = 5
	recordN(t, repo, "aa:bb:cc", 12, size)

	if n := countDeliveryRows(t, conn); n != size {
		t.Errorf("after 12 deliveries into a ring of %d, got %d rows — the upsert appended rather than overwrote", size, n)
	}

	// Having established what it does, record what it says, so a change of
	// spelling in a future GORM cannot pass unnoticed.
	stmt := rec.last("ON CONFLICT")
	if stmt == "" {
		t.Fatal("no ON CONFLICT statement was recorded")
	}
	if !strings.Contains(stmt, `ON CONFLICT ("display_key","slot") DO UPDATE`) {
		t.Errorf("the conflict target is no longer the composite primary key:\n%s", stmt)
	}
	for _, col := range []string{"seq", "delivered_at", "kind", "image_id", "catalog_key", "source", "reason", "sleep_seconds"} {
		if !strings.Contains(stmt, fmt.Sprintf(`"%s"="excluded"."%s"`, col, col)) {
			t.Errorf("%s is not assigned from the proposed row; a wrapped slot would keep the old value:\n%s", col, stmt)
		}
	}
	t.Logf("generated upsert: %s", stmt)

	// Overwritten in place, not merely bounded: the slot the twelfth delivery
	// landed on holds the twelfth delivery and nothing of the seventh.
	var slots []int
	if err := conn.Model(&model.DeliveryHistory{}).Order("slot").Pluck("slot", &slots).Error; err != nil {
		t.Fatalf("pluck slots: %v", err)
	}
	if len(slots) != size {
		t.Fatalf("got %d slots, want %d", len(slots), size)
	}
	for i, slot := range slots {
		if slot != i {
			t.Errorf("slot %d of the ring is %d; the slots are not 0..%d", i, slot, size-1)
		}
	}

	entries, err := repo.ListByDisplay("aa:bb:cc", size)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	want := []int64{12, 11, 10, 9, 8}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d", len(entries), len(want))
	}
	for i, seq := range want {
		if entries[i].Seq != seq {
			t.Errorf("entries[%d].Seq = %d, want %d — the wrap did not replace the oldest", i, entries[i].Seq, seq)
		}
		if got := int(seq % size); entries[i].Slot != got {
			t.Errorf("seq %d is stored in slot %d, want %d", seq, entries[i].Slot, got)
		}
	}
}

// TestPostgres_DeliveryHistory_RecordRewritesEveryColumn: a wrapped slot must
// carry none of what the delivery it replaced left there. A column missing from
// the assignment list would leave a stale value behind that reads as this
// delivery's own.
func TestPostgres_DeliveryHistory_RecordRewritesEveryColumn(t *testing.T) {
	repo, conn, _ := setupPostgresDeliveryRepo(t)

	const size = 2
	first := deliveryRec("aa:bb:cc")
	first.Kind = model.DeliveryKindError
	first.Reason = model.DeliveryReasonNoImages
	first.CatalogKey = "old-catalogue"
	first.Source = "/old/source.jpg"
	first.ImageID = 4242
	first.SleepSeconds = 3600
	if err := repo.Record(first, size); err != nil {
		t.Fatalf("Record the first: %v", err)
	}

	// One more, so the delivery after it lands back on the first's slot: with
	// a ring of two, seq 1 and seq 3 share slot 1.
	recordN(t, repo, "aa:bb:cc", 1, size)

	third := deliveryRec("aa:bb:cc")
	third.Kind = model.DeliveryKindColorbar
	third.CatalogKey = ""
	third.Source = ""
	third.ImageID = 0
	third.SleepSeconds = 900
	if err := repo.Record(third, size); err != nil {
		t.Fatalf("Record the overwriting one: %v", err)
	}
	if third.Slot != first.Slot {
		t.Fatalf("the overwriting delivery landed in slot %d, not the first's %d", third.Slot, first.Slot)
	}

	var stored model.DeliveryHistory
	if err := conn.Where("display_key = ? AND slot = ?", "aa:bb:cc", first.Slot).
		Take(&stored).Error; err != nil {
		t.Fatalf("read the overwritten slot: %v", err)
	}
	if stored.Seq != third.Seq {
		t.Fatalf("slot %d holds seq %d, want %d", first.Slot, stored.Seq, third.Seq)
	}
	for _, c := range []struct {
		column    string
		got, want any
	}{
		{"kind", stored.Kind, third.Kind},
		{"reason", stored.Reason, model.DeliveryReasonNone},
		{"catalog_key", stored.CatalogKey, ""},
		{"source", stored.Source, ""},
		{"image_id", stored.ImageID, model.PrimaryKey(0)},
		{"sleep_seconds", stored.SleepSeconds, 900},
	} {
		if c.got != c.want {
			t.Errorf("%s is %v after the overwrite, want %v — the previous delivery's value survived",
				c.column, c.got, c.want)
		}
	}
}

// TestPostgres_DeliveryHistory_RecordSurvivesMissingTable is the state every
// installation that predates this feature is in: WISP_AUTO_MIGRATE is off by
// default, so the table does not exist until somebody migrates. A delivery must
// still go out, and on PostgreSQL a failed statement also poisons nothing that
// follows.
func TestPostgres_DeliveryHistory_RecordSurvivesMissingTable(t *testing.T) {
	conn, _ := setupPostgres(t)
	if err := conn.Migrator().DropTable(&model.DeliveryHistory{}); err != nil {
		t.Fatalf("DropTable: %v", err)
	}
	repo := infraRepo.NewDeliveryHistoryRepositoryImpl(conn)

	if err := repo.Record(deliveryRec("aa:bb:cc"), 5); err != nil {
		t.Errorf("Record against a missing table returned %v, want nil", err)
	}

	// The connection is still usable: PostgreSQL aborts a transaction on error,
	// and a Record that had opened one would leave every later statement
	// failing with "current transaction is aborted".
	var n int64
	if err := conn.Model(&model.Image{}).Count(&n).Error; err != nil {
		t.Errorf("the connection is unusable after the failed record: %v", err)
	}
}

// TestPostgres_DeliveryHistory_ImageIDRange pins what the widening of an
// unsigned Go field to a signed bigint costs. Every id the application can
// actually produce comes from a bigserial and fits; the boundary is recorded so
// that a future change of the id's origin meets it here.
func TestPostgres_DeliveryHistory_ImageIDRange(t *testing.T) {
	repo, conn, _ := setupPostgresDeliveryRepo(t)

	const maxInt64 = model.PrimaryKey(1<<63 - 1)
	rec := deliveryRec("aa:bb:cc")
	rec.ImageID = maxInt64
	if err := repo.Record(rec, 5); err != nil {
		t.Fatalf("Record: %v", err)
	}
	var stored model.DeliveryHistory
	if err := conn.Where("display_key = ?", "aa:bb:cc").Take(&stored).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored.ImageID != maxInt64 {
		t.Errorf("image_id came back as %d, want %d", stored.ImageID, maxInt64)
	}
}

// --- ListByDisplay ---------------------------------------------------------

// TestPostgres_DeliveryHistory_ListByDisplay covers the LEFT JOIN. The CASE
// exists so that image_available arrives as 1 or 0 on every dialect rather than
// as a boolean on some; what it has to end up as is a Go bool either way.
func TestPostgres_DeliveryHistory_ListByDisplay(t *testing.T) {
	repo, conn, rec := setupPostgresDeliveryRepo(t)

	img := insertImage(t, conn, "album")
	withImage := deliveryRec("aa:bb:cc")
	withImage.ImageID = img.ID
	if err := repo.Record(withImage, 5); err != nil {
		t.Fatalf("Record the photo: %v", err)
	}
	// No images row can have id 0, so a colour bar misses the join without
	// needing a special case.
	colorbar := deliveryRec("aa:bb:cc")
	colorbar.Kind = model.DeliveryKindColorbar
	colorbar.ImageID = 0
	if err := repo.Record(colorbar, 5); err != nil {
		t.Fatalf("Record the colour bar: %v", err)
	}

	entries, err := repo.ListByDisplay("aa:bb:cc", 5)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	// Newest first, by seq and never by slot.
	if entries[0].Seq != 2 || entries[1].Seq != 1 {
		t.Fatalf("entries are ordered %d, %d — want 2, 1", entries[0].Seq, entries[1].Seq)
	}
	if entries[0].ImageAvailable {
		t.Error("the colour bar reports its image as available")
	}
	if !entries[1].ImageAvailable {
		t.Error("the photograph reports its image as unavailable")
	}

	// The rest of the row has to survive the join too: h.* is scanned into the
	// embedded model beside a column that is not one of its fields.
	got := entries[1]
	if got.DisplayKey != "aa:bb:cc" || got.Kind != model.DeliveryKindPhoto ||
		got.CatalogKey != "album" || got.Source != "/photos/x.jpg" || got.SleepSeconds != 3600 {
		t.Errorf("the joined row did not scan into the entry: %+v", got.DeliveryHistory)
	}
	if got.ImageID != img.ID {
		t.Errorf("image_id is %d, want %d", got.ImageID, img.ID)
	}
	if delta := got.DeliveredAt.Sub(withImage.DeliveredAt); delta > time.Millisecond || delta < -time.Millisecond {
		t.Errorf("delivered_at came back as %v, want %v", got.DeliveredAt.UTC(), withImage.DeliveredAt.UTC())
	}

	t.Logf("generated listing: %s", rec.last("image_available"))
}

// TestPostgres_DeliveryHistory_ImageAvailableSoftDelete is the distinction the
// join is deliberately built around, and the one easiest to break without
// noticing. Hiding a photograph in the UI is a soft delete and reversible, so
// the history entry stays viewable; a real purge is not, so it does not. GORM
// adds `deleted_at IS NULL` for the primary model only, and images is merely
// joined here — make images the primary model in some later refactor and the
// first half of this flips with nothing else changing.
func TestPostgres_DeliveryHistory_ImageAvailableSoftDelete(t *testing.T) {
	repo, conn, _ := setupPostgresDeliveryRepo(t)

	hidden := insertImage(t, conn, "album")
	purged := insertImage(t, conn, "album")
	for _, img := range []*model.Image{hidden, purged} {
		rec := deliveryRec("aa:bb:cc")
		rec.ImageID = img.ID
		if err := repo.Record(rec, 5); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	// The user's visibility toggle: a soft delete, not a removal.
	if err := conn.Where("id = ?", hidden.ID).Delete(&model.Image{}).Error; err != nil {
		t.Fatalf("hide image: %v", err)
	}
	// A real purge, which is why ImageID is not a foreign key: the row has to
	// survive the photograph.
	if err := conn.Unscoped().Where("id = ?", purged.ID).Delete(&model.Image{}).Error; err != nil {
		t.Fatalf("purge image: %v", err)
	}

	entries, err := repo.ListByDisplay("aa:bb:cc", 5)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 — purging a photograph must not take its history with it", len(entries))
	}

	byImageID := map[model.PrimaryKey]*repository.DeliveryHistoryEntry{}
	for _, e := range entries {
		byImageID[e.ImageID] = e
	}
	if e := byImageID[hidden.ID]; e == nil || !e.ImageAvailable {
		t.Error("a photograph the user merely hid reports unavailable; hiding is reversible and the photograph is still there")
	}
	if e := byImageID[purged.ID]; e == nil || e.ImageAvailable {
		t.Error("a purged photograph still reports available")
	}
}

// TestPostgres_DeliveryHistory_ListByDisplayBounds: an undelivered display is
// zero rows rather than one empty one, and a non-positive limit asks for
// nothing without reaching the server.
func TestPostgres_DeliveryHistory_ListByDisplayBounds(t *testing.T) {
	repo, _, _ := setupPostgresDeliveryRepo(t)

	recordN(t, repo, "aa:bb:cc", 3, 10)

	entries, err := repo.ListByDisplay("never:seen", 10)
	if err != nil {
		t.Fatalf("ListByDisplay for an undelivered display: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries for an undelivered display, want 0", len(entries))
	}

	entries, err = repo.ListByDisplay("aa:bb:cc", 2)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d entries for a limit of 2, want 2", len(entries))
	}

	entries, err = repo.ListByDisplay("aa:bb:cc", 0)
	if err != nil {
		t.Fatalf("ListByDisplay with a zero limit: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries for a zero limit, want 0", len(entries))
	}
}

// --- SummaryByDisplay ------------------------------------------------------

// TestPostgres_DeliveryHistory_Summary is where PostgreSQL's types are widest.
// SUM over integers comes back as bigint and COUNT(*) always does, both of
// which are scanned into a Go int; MAX(delivered_at) comes back as a real
// timestamp, which has to pass through the aggregatedTime shim written for
// SQLite's untyped one without being mangled on the way.
func TestPostgres_DeliveryHistory_Summary(t *testing.T) {
	repo, _, rec := setupPostgresDeliveryRepo(t)

	// A fixed instant, so a mis-parsed timestamp cannot pass as a plausible
	// date. Truncated to the second because that is the coarsest resolution any
	// of the three servers is asked to keep.
	newest := time.Date(2026, 3, 2, 12, 34, 56, 0, time.UTC)
	for i := range 3 {
		delivery := deliveryRec("display:a")
		delivery.DeliveredAt = newest.Add(-time.Duration(i) * time.Hour)
		if err := repo.Record(delivery, 10); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	for range 2 {
		delivery := deliveryRec("display:a")
		delivery.DeliveredAt = newest.Add(-24 * time.Hour)
		delivery.Kind = model.DeliveryKindError
		delivery.Reason = model.DeliveryReasonNoImages
		if err := repo.Record(delivery, 10); err != nil {
			t.Fatalf("Record an error card: %v", err)
		}
	}
	recordN(t, repo, "display:b", 1, 10)

	summaries, err := repo.SummaryByDisplay()
	if err != nil {
		t.Fatalf("SummaryByDisplay: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d summaries, want 2 — a display with no history must be absent", len(summaries))
	}

	byKey := map[string]*repository.DeliverySummary{}
	for _, s := range summaries {
		byKey[s.DisplayKey] = s
	}

	a, ok := byKey["display:a"]
	if !ok {
		t.Fatal("no summary for display:a")
	}
	if a.Entries != 5 {
		t.Errorf("display:a Entries = %d, want 5 — COUNT(*) is a bigint here", a.Entries)
	}
	if a.ErrorEntries != 2 {
		t.Errorf("display:a ErrorEntries = %d, want 2 — SUM over integers is a bigint here", a.ErrorEntries)
	}
	if a.LastSeq != 5 {
		t.Errorf("display:a LastSeq = %d, want 5", a.LastSeq)
	}
	// The instant, not the offset: the driver hands the timestamp back in the
	// session's zone, and only the moment it names is the repository's promise.
	if got := a.LastDeliveredAt; !got.Equal(newest) {
		t.Errorf("display:a LastDeliveredAt = %v, want %v — the aggregated timestamp did not survive", got.UTC(), newest)
	}

	b, ok := byKey["display:b"]
	if !ok {
		t.Fatal("no summary for display:b")
	}
	if b.Entries != 1 || b.ErrorEntries != 0 {
		t.Errorf("display:b summary = %+v, want 1 entry / 0 errors", b)
	}
	if b.LastDeliveredAt.IsZero() {
		t.Error("display:b LastDeliveredAt is zero; the aggregate did not come back as a timestamp")
	}

	t.Logf("generated summary: %s", rec.last("error_entries"))
}

// TestPostgres_DeliveryHistory_SummaryEmpty: an installation that has delivered
// nothing yet returns no rows rather than failing on a SUM over none.
func TestPostgres_DeliveryHistory_SummaryEmpty(t *testing.T) {
	repo, _, _ := setupPostgresDeliveryRepo(t)

	summaries, err := repo.SummaryByDisplay()
	if err != nil {
		t.Fatalf("SummaryByDisplay: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("got %d summaries, want 0", len(summaries))
	}
}

// --- Reconcile -------------------------------------------------------------

// TestPostgres_DeliveryHistory_Reconcile covers both deletes, the second of
// which puts a Go slice into a NOT IN. It runs at start-up, so running it twice
// must be the same as running it once.
func TestPostgres_DeliveryHistory_Reconcile(t *testing.T) {
	repo, conn, _ := setupPostgresDeliveryRepo(t)

	recordN(t, repo, "kept", 10, 10)
	recordN(t, repo, "retired", 3, 10)
	if n := countDeliveryRows(t, conn); n != 13 {
		t.Fatalf("setup: got %d rows, want 13", n)
	}

	for range 2 {
		if err := repo.Reconcile([]string{"kept"}, 4); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
	}

	// The retired display goes entirely; the kept one comes back inside its
	// shrunk ring, which nothing else would ever have trimmed.
	if n := countDeliveryRows(t, conn); n != 4 {
		t.Errorf("got %d rows after reconciling, want 4", n)
	}
	var maxSlot int
	if err := conn.Model(&model.DeliveryHistory{}).Select("COALESCE(MAX(slot), -1)").Scan(&maxSlot).Error; err != nil {
		t.Fatalf("max slot: %v", err)
	}
	if maxSlot >= 4 {
		t.Errorf("max slot is %d, want below 4", maxSlot)
	}
	entries, err := repo.ListByDisplay("retired", 10)
	if err != nil {
		t.Fatalf("ListByDisplay(retired): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("the retired display still has %d entries, want 0", len(entries))
	}

	// The next sequence number is read back from what is left, so a delete must
	// not let it collide with a survivor.
	survivors, err := repo.ListByDisplay("kept", 10)
	if err != nil {
		t.Fatalf("ListByDisplay(kept): %v", err)
	}
	highest := survivors[0].Seq
	recordN(t, repo, "kept", 1, 4)
	after, err := repo.ListByDisplay("kept", 10)
	if err != nil {
		t.Fatalf("ListByDisplay(kept) after recording: %v", err)
	}
	if after[0].Seq <= highest {
		t.Errorf("the next seq is %d, want above the highest survivor %d", after[0].Seq, highest)
	}
}

// TestPostgres_DeliveryHistory_ReconcileKeepsEverythingWithoutKeys: an empty
// key list reads as "nothing is configured yet", not as permission to delete
// every display's history — and a NOT IN over an empty slice is a statement
// PostgreSQL would reject outright, so the guard has to hold before the SQL is
// built.
func TestPostgres_DeliveryHistory_ReconcileKeepsEverythingWithoutKeys(t *testing.T) {
	repo, conn, _ := setupPostgresDeliveryRepo(t)

	recordN(t, repo, "aa:bb:cc", 3, 5)

	for _, keys := range [][]string{nil, {}} {
		if err := repo.Reconcile(keys, 5); err != nil {
			t.Fatalf("Reconcile(%v): %v", keys, err)
		}
	}
	if n := countDeliveryRows(t, conn); n != 3 {
		t.Errorf("got %d rows, want 3 — an empty key list must delete nothing", n)
	}

	// A non-positive size leaves the ring alone rather than emptying it.
	if err := repo.Reconcile([]string{"aa:bb:cc"}, 0); err != nil {
		t.Fatalf("Reconcile with a zero size: %v", err)
	}
	if n := countDeliveryRows(t, conn); n != 3 {
		t.Errorf("got %d rows after a zero size, want 3", n)
	}
}

// --- DropAndRecreate -------------------------------------------------------

// TestPostgres_DeliveryHistory_DropAndRecreate: system prune now covers four
// models rather than three, and the history is the one whose table carries a
// composite primary key. Coming back is not enough — it has to be writable
// again on the same connection, without a restart.
func TestPostgres_DeliveryHistory_DropAndRecreate(t *testing.T) {
	repo, conn, _ := setupPostgresDeliveryRepo(t)

	recordN(t, repo, "aa:bb:cc", 3, 5)
	sys := infraRepo.NewSystemRepositoryImpl(conn)
	if err := sys.DropAndRecreate(); err != nil {
		t.Fatalf("DropAndRecreate: %v", err)
	}

	if len(model.AllModels()) != 4 {
		t.Errorf("AllModels holds %d models; this test was written for the four the prune covers", len(model.AllModels()))
	}
	for _, m := range model.AllModels() {
		if !conn.Migrator().HasTable(m) {
			t.Fatalf("table for %T was not recreated", m)
		}
	}
	if n := countDeliveryRows(t, conn); n != 0 {
		t.Errorf("%d deliveries survived the prune, want 0", n)
	}

	// Writable and readable again, through the same repository on the same
	// connection: the upsert needs its primary key back, and the listing needs
	// the images table it joins against.
	img := insertImage(t, conn, "album")
	rec := deliveryRec("aa:bb:cc")
	rec.ImageID = img.ID
	if err := repo.Record(rec, 5); err != nil {
		t.Fatalf("Record after the prune: %v", err)
	}
	entries, err := repo.ListByDisplay("aa:bb:cc", 5)
	if err != nil {
		t.Fatalf("ListByDisplay after the prune: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries after the prune, want 1", len(entries))
	}
	if entries[0].Seq != 1 {
		t.Errorf("the first delivery after a prune has seq %d, want 1", entries[0].Seq)
	}
	if !entries[0].ImageAvailable {
		t.Error("the recreated join does not see the images table")
	}
	if _, err := repo.SummaryByDisplay(); err != nil {
		t.Fatalf("SummaryByDisplay after the prune: %v", err)
	}

	// The composite primary key has to have been recreated, or nothing bounds
	// the ring any more.
	if got := pgIndexDef(t, conn, "delivery_histories_pkey"); !strings.Contains(got, "(display_key, slot)") {
		t.Errorf("the primary key after a prune is %q, want one over (display_key, slot)", got)
	}
}

// --- concurrency -----------------------------------------------------------

// TestPostgres_DeliveryHistory_RecordConcurrent: several requests can land at
// once on a server that feeds more than one panel, or on one panel retrying.
// SQLite could not really test this — the connection pool is held at one — so
// this is the first time the read-then-write is put under genuine parallelism.
// The bound has to hold through it and no sequence number may repeat.
func TestPostgres_DeliveryHistory_RecordConcurrent(t *testing.T) {
	repo, conn, _ := setupPostgresDeliveryRepo(t)

	const (
		size    = 5
		writers = 40
	)
	var wg sync.WaitGroup
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := repo.Record(deliveryRec("aa:bb:cc"), size); err != nil {
				t.Errorf("Record: %v", err)
			}
		}()
	}
	wg.Wait()

	if n := countDeliveryRows(t, conn); n != size {
		t.Errorf("got %d rows after %d concurrent writes, want %d", n, writers, size)
	}

	entries, err := repo.ListByDisplay("aa:bb:cc", size)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if len(entries) != size {
		t.Fatalf("got %d entries, want %d", len(entries), size)
	}
	// Strictly decreasing, so every sequence number is distinct and the newest
	// really is the last one written.
	for i := 1; i < len(entries); i++ {
		if entries[i].Seq >= entries[i-1].Seq {
			t.Errorf("entries[%d].Seq = %d is not below entries[%d].Seq = %d; sequences must be unique and descending",
				i, entries[i].Seq, i-1, entries[i-1].Seq)
		}
	}
	// Every write landed: nothing was lost and nothing was counted twice.
	if entries[0].Seq != writers {
		t.Errorf("the highest seq is %d after %d writes, want %d", entries[0].Seq, writers, writers)
	}
}

package repository_test

import (
	"sync"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"

	"gorm.io/gorm"
)

// --- helpers ---------------------------------------------------------------

// setupDeliveryRepo creates an in-memory SQLite DB with every model migrated —
// images included, since ListByDisplay joins against it — and returns the
// repository plus the raw *gorm.DB for counting rows directly.
func setupDeliveryRepo(t *testing.T) (repository.DeliveryHistoryRepository, *gorm.DB) {
	t.Helper()
	conn := newDeliveryConn(t)
	if err := conn.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return infraRepo.NewDeliveryHistoryRepositoryImpl(conn), conn
}

// newDeliveryConn opens an in-memory SQLite DB with nothing migrated.
func newDeliveryConn(t *testing.T) *gorm.DB {
	t.Helper()
	conn, err := infra.NewSqliteConnection("", true)
	if err != nil {
		t.Fatalf("NewSqliteConnection: %v", err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatalf("conn.DB(): %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	return conn
}

// deliveryRec builds a minimal photo delivery for the given display.
func deliveryRec(displayKey string) *model.DeliveryHistory {
	return &model.DeliveryHistory{
		DisplayKey:   displayKey,
		DeliveredAt:  time.Now().UTC(),
		Kind:         model.DeliveryKindPhoto,
		CatalogKey:   "album",
		Source:       "/photos/x.jpg",
		SleepSeconds: 3600,
	}
}

// countDeliveryRows returns how many rows the history table holds in total.
func countDeliveryRows(t *testing.T, conn *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := conn.Model(&model.DeliveryHistory{}).Count(&n).Error; err != nil {
		t.Fatalf("Count: %v", err)
	}
	return n
}

// recordN stores n deliveries for one display, failing the test on error.
func recordN(t *testing.T, repo repository.DeliveryHistoryRepository, displayKey string, n, size int) {
	t.Helper()
	for range n {
		if err := repo.Record(deliveryRec(displayKey), size); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
}

// insertImage adds an images row so that a delivery can point at a real photo.
func insertImage(t *testing.T, conn *gorm.DB, catalogKey string) *model.Image {
	t.Helper()
	img := dummyImage(catalogKey)
	if err := conn.Create(img).Error; err != nil {
		t.Fatalf("create image: %v", err)
	}
	return img
}

// --- ring size -------------------------------------------------------------

// TestRecordNeverExceedsRingSize is the whole point of the design: the table is
// bounded by the configured size, not by how long the frame has been running.
func TestRecordNeverExceedsRingSize(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	recordN(t, repo, "aa:bb:cc", 50, 5)

	if n := countDeliveryRows(t, conn); n != 5 {
		t.Errorf("after 50 deliveries into a ring of 5, got %d rows, want 5", n)
	}
}

// TestListByDisplayAfterWrapping checks that once the ring has wrapped, reading
// it back gives the latest deliveries newest-first. Ordering by slot instead of
// seq would put them in a rotated order that looks plausible until it doesn't.
func TestListByDisplayAfterWrapping(t *testing.T) {
	repo, _ := setupDeliveryRepo(t)

	recordN(t, repo, "aa:bb:cc", 12, 5)

	entries, err := repo.ListByDisplay("aa:bb:cc", 5)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}

	want := []int64{12, 11, 10, 9, 8}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d", len(entries), len(want))
	}
	for i, seq := range want {
		if entries[i].Seq != seq {
			t.Errorf("entries[%d].Seq = %d, want %d", i, entries[i].Seq, seq)
		}
	}
}

// TestListByDisplayPartiallyFilledRing pins the reason nothing is
// pre-allocated: a ring that has not filled up holds fewer rows, so a read
// needs no filtering and cannot return a placeholder.
func TestListByDisplayPartiallyFilledRing(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	recordN(t, repo, "aa:bb:cc", 3, 10)

	if n := countDeliveryRows(t, conn); n != 3 {
		t.Errorf("got %d rows, want 3 — the ring must not be pre-allocated", n)
	}

	entries, err := repo.ListByDisplay("aa:bb:cc", 10)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	for i, e := range entries {
		if e.DeliveredAt.IsZero() {
			t.Errorf("entries[%d] has a zero DeliveredAt; a sentinel row leaked into the result", i)
		}
	}
}

// TestListByDisplayEmpty: a display nothing has been delivered to is zero rows,
// not one empty one.
func TestListByDisplayEmpty(t *testing.T) {
	repo, _ := setupDeliveryRepo(t)

	entries, err := repo.ListByDisplay("never:seen", 10)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries for an undelivered display, want 0", len(entries))
	}
}

// TestDisplaysAreIndependent: each display gets its own ring and its own
// sequence, so a busy display cannot push another display's history out.
func TestDisplaysAreIndependent(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	recordN(t, repo, "display:a", 20, 3)
	recordN(t, repo, "display:b", 2, 3)

	if n := countDeliveryRows(t, conn); n != 5 {
		t.Errorf("got %d rows total, want 5 (3 + 2)", n)
	}

	a, err := repo.ListByDisplay("display:a", 10)
	if err != nil {
		t.Fatalf("ListByDisplay(a): %v", err)
	}
	if len(a) != 3 || a[0].Seq != 20 {
		t.Errorf("display:a got %d entries with newest seq %d, want 3 entries with seq 20", len(a), a[0].Seq)
	}

	b, err := repo.ListByDisplay("display:b", 10)
	if err != nil {
		t.Fatalf("ListByDisplay(b): %v", err)
	}
	if len(b) != 2 || b[0].Seq != 2 {
		t.Errorf("display:b got %d entries with newest seq %d, want 2 entries with seq 2", len(b), b[0].Seq)
	}
}

// TestRecordConcurrent: several requests can land at once on a frame that
// serves more than one panel, or on one panel retrying. The bound has to hold
// through it, and the window that comes back must not repeat a sequence number.
func TestRecordConcurrent(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	const size = 5
	var wg sync.WaitGroup
	for range 40 {
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
		t.Errorf("got %d rows after concurrent writes, want %d", n, size)
	}

	entries, err := repo.ListByDisplay("aa:bb:cc", size)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].Seq >= entries[i-1].Seq {
			t.Errorf("entries[%d].Seq = %d is not below entries[%d].Seq = %d; sequences must be unique and descending",
				i, entries[i].Seq, i-1, entries[i-1].Seq)
		}
	}
}

// TestRecordWithNonPositiveSize: a size of zero or less stores nothing, and
// says so by not failing. Nothing downstream should have to check first.
func TestRecordWithNonPositiveSize(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	for _, size := range []int{0, -1} {
		if err := repo.Record(deliveryRec("aa:bb:cc"), size); err != nil {
			t.Errorf("Record(size=%d) returned %v, want nil", size, err)
		}
	}

	if n := countDeliveryRows(t, conn); n != 0 {
		t.Errorf("got %d rows, want 0 — a non-positive size must record nothing", n)
	}
}

// TestRecordSurvivesMissingTable is the state every existing installation is
// in: WISP_AUTO_MIGRATE is off by default, so the table does not exist until
// somebody migrates. A delivery must still go out.
func TestRecordSurvivesMissingTable(t *testing.T) {
	conn := newDeliveryConn(t)
	// Deliberately migrate everything except DeliveryHistory.
	if err := conn.AutoMigrate(&model.Image{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	repo := infraRepo.NewDeliveryHistoryRepositoryImpl(conn)

	if err := repo.Record(deliveryRec("aa:bb:cc"), 5); err != nil {
		t.Errorf("Record against a missing table returned %v, want nil", err)
	}
}

// --- Reconcile -------------------------------------------------------------

// TestReconcileShrinksRing: nothing removes the tail of a ring that has been
// made smaller except this. Without it the table stays above its bound for
// good, which is the one thing the design exists to prevent.
func TestReconcileShrinksRing(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	recordN(t, repo, "aa:bb:cc", 10, 10)
	if n := countDeliveryRows(t, conn); n != 10 {
		t.Fatalf("setup: got %d rows, want 10", n)
	}

	if err := repo.Reconcile([]string{"aa:bb:cc"}, 4); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if n := countDeliveryRows(t, conn); n != 4 {
		t.Errorf("got %d rows after shrinking to 4, want 4", n)
	}

	var maxSlot int
	if err := conn.Model(&model.DeliveryHistory{}).Select("COALESCE(MAX(slot), -1)").Scan(&maxSlot).Error; err != nil {
		t.Fatalf("max slot: %v", err)
	}
	if maxSlot >= 4 {
		t.Errorf("max slot is %d, want below 4", maxSlot)
	}
}

// TestReconcileDropsUnconfiguredDisplays: a display taken out of service should
// not leave its history behind for ever.
func TestReconcileDropsUnconfiguredDisplays(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	recordN(t, repo, "kept", 3, 5)
	recordN(t, repo, "retired", 3, 5)

	if err := repo.Reconcile([]string{"kept"}, 5); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if n := countDeliveryRows(t, conn); n != 3 {
		t.Errorf("got %d rows, want 3 — only the retired display's history should go", n)
	}
	entries, err := repo.ListByDisplay("retired", 10)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("retired display still has %d entries, want 0", len(entries))
	}
}

// TestReconcileWithNoKeysKeepsEverything: an empty key list reads as "nothing
// is configured yet", not as permission to delete every display's history.
func TestReconcileWithNoKeysKeepsEverything(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	recordN(t, repo, "aa:bb:cc", 3, 5)

	if err := repo.Reconcile(nil, 5); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if n := countDeliveryRows(t, conn); n != 3 {
		t.Errorf("got %d rows, want 3 — an empty key list must delete nothing", n)
	}
}

// TestReconcileIsRepeatable: it runs at startup, so running it twice must be
// the same as running it once.
func TestReconcileIsRepeatable(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	recordN(t, repo, "aa:bb:cc", 8, 8)
	for range 3 {
		if err := repo.Reconcile([]string{"aa:bb:cc"}, 4); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
	}

	if n := countDeliveryRows(t, conn); n != 4 {
		t.Errorf("got %d rows, want 4", n)
	}
}

// TestSeqStaysAboveSurvivorsAfterReconcile: the next sequence number is read
// back from the rows that are left, so deleting rows must not let it collide
// with one that survived — a repeat would make the read order meaningless.
func TestSeqStaysAboveSurvivorsAfterReconcile(t *testing.T) {
	repo, _ := setupDeliveryRepo(t)

	recordN(t, repo, "aa:bb:cc", 10, 10)
	if err := repo.Reconcile([]string{"aa:bb:cc"}, 4); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	survivors, err := repo.ListByDisplay("aa:bb:cc", 10)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	highest := survivors[0].Seq

	recordN(t, repo, "aa:bb:cc", 1, 4)

	entries, err := repo.ListByDisplay("aa:bb:cc", 10)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if entries[0].Seq <= highest {
		t.Errorf("next seq is %d, want above the highest survivor %d", entries[0].Seq, highest)
	}

	seen := map[int64]bool{}
	for _, e := range entries {
		if seen[e.Seq] {
			t.Errorf("seq %d appears twice", e.Seq)
		}
		seen[e.Seq] = true
	}
}

// --- image_available -------------------------------------------------------

// TestImageAvailableForLivePhoto: the ordinary case — the photo is still in the
// catalogue.
func TestImageAvailableForLivePhoto(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	img := insertImage(t, conn, "album")
	rec := deliveryRec("aa:bb:cc")
	rec.ImageID = img.ID
	if err := repo.Record(rec, 5); err != nil {
		t.Fatalf("Record: %v", err)
	}

	entries, err := repo.ListByDisplay("aa:bb:cc", 5)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if !entries[0].ImageAvailable {
		t.Error("ImageAvailable = false for a photo that is still there, want true")
	}
}

// TestImageAvailableAfterHardPurge: a photo really removed from the catalogue
// leaves the history entry behind, correctly marked as no longer viewable.
// This is why ImageID is not a foreign key.
func TestImageAvailableAfterHardPurge(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	img := insertImage(t, conn, "album")
	rec := deliveryRec("aa:bb:cc")
	rec.ImageID = img.ID
	if err := repo.Record(rec, 5); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if err := conn.Unscoped().Where("id = ?", img.ID).Delete(&model.Image{}).Error; err != nil {
		t.Fatalf("purge image: %v", err)
	}

	entries, err := repo.ListByDisplay("aa:bb:cc", 5)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 — purging a photo must not take its history with it", len(entries))
	}
	if entries[0].ImageAvailable {
		t.Error("ImageAvailable = true after a hard purge, want false")
	}
}

// TestImageAvailableForUserHiddenImage pins the soft-delete semantics of the
// join, which are easy to break without noticing.
//
// Hiding a photo in the UI sets deleted_at; the photo itself is untouched and
// can be shown again at any time, so the history entry stays viewable. GORM
// only adds `deleted_at IS NULL` for the primary model, and images is merely
// joined here — make images the primary model in some later refactor and this
// silently flips to false with nothing else changing.
func TestImageAvailableForUserHiddenImage(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	img := insertImage(t, conn, "album")
	rec := deliveryRec("aa:bb:cc")
	rec.ImageID = img.ID
	if err := repo.Record(rec, 5); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// The user's visibility toggle: a soft delete, not a removal.
	if err := conn.Where("id = ?", img.ID).Delete(&model.Image{}).Error; err != nil {
		t.Fatalf("hide image: %v", err)
	}

	entries, err := repo.ListByDisplay("aa:bb:cc", 5)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if !entries[0].ImageAvailable {
		t.Error("ImageAvailable = false for a user-hidden photo, want true — hiding is reversible and the photo is still there")
	}
}

// TestImageAvailableForZeroImageID: colour bars, error cards and live HTTP
// fetches carry no images row. Autoincrement ids start at 1, so id 0 matches
// nothing and no special case is needed.
func TestImageAvailableForZeroImageID(t *testing.T) {
	repo, conn := setupDeliveryRepo(t)

	insertImage(t, conn, "album") // ensure the join has something to miss

	rec := deliveryRec("aa:bb:cc")
	rec.Kind = model.DeliveryKindColorbar
	rec.ImageID = 0
	if err := repo.Record(rec, 5); err != nil {
		t.Fatalf("Record: %v", err)
	}

	entries, err := repo.ListByDisplay("aa:bb:cc", 5)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if entries[0].ImageAvailable {
		t.Error("ImageAvailable = true for a delivery with no image, want false")
	}
}

// --- SummaryByDisplay ------------------------------------------------------

// TestSummaryByDisplay: one row per display, with the error deliveries counted
// separately, and nothing at all for a display that has never been delivered
// to.
func TestSummaryByDisplay(t *testing.T) {
	repo, _ := setupDeliveryRepo(t)

	recordN(t, repo, "display:a", 3, 10)
	for range 2 {
		rec := deliveryRec("display:a")
		rec.Kind = model.DeliveryKindError
		rec.Reason = model.DeliveryReasonNoImages
		if err := repo.Record(rec, 10); err != nil {
			t.Fatalf("Record: %v", err)
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
	if a.Entries != 5 || a.ErrorEntries != 2 || a.LastSeq != 5 {
		t.Errorf("display:a summary = %+v, want 5 entries / 2 errors / last seq 5", a)
	}
	if a.LastDeliveredAt.IsZero() {
		t.Error("display:a LastDeliveredAt is zero; the aggregate did not come back as a timestamp")
	}

	b, ok := byKey["display:b"]
	if !ok {
		t.Fatal("no summary for display:b")
	}
	if b.Entries != 1 || b.ErrorEntries != 0 {
		t.Errorf("display:b summary = %+v, want 1 entry / 0 errors", b)
	}

	if _, ok := byKey["display:never"]; ok {
		t.Error("a display with no history appeared in the summary")
	}
}

// TestSummaryByDisplayEmpty: an installation that has delivered nothing yet
// returns no rows rather than failing.
func TestSummaryByDisplayEmpty(t *testing.T) {
	repo, _ := setupDeliveryRepo(t)

	summaries, err := repo.SummaryByDisplay()
	if err != nil {
		t.Fatalf("SummaryByDisplay: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("got %d summaries, want 0", len(summaries))
	}
}

// TestSummaryLastDeliveredAtMatchesNewestEntry checks that the aggregated
// timestamp survives the round trip intact — SQLite returns it untyped, and a
// mis-parsed value would still look like a plausible date.
func TestSummaryLastDeliveredAtMatchesNewestEntry(t *testing.T) {
	repo, _ := setupDeliveryRepo(t)

	rec := deliveryRec("aa:bb:cc")
	rec.DeliveredAt = time.Date(2026, 3, 2, 12, 34, 56, 0, time.UTC)
	if err := repo.Record(rec, 5); err != nil {
		t.Fatalf("Record: %v", err)
	}

	summaries, err := repo.SummaryByDisplay()
	if err != nil {
		t.Fatalf("SummaryByDisplay: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("got %d summaries, want 1", len(summaries))
	}
	if got := summaries[0].LastDeliveredAt.UTC(); !got.Equal(rec.DeliveredAt) {
		t.Errorf("LastDeliveredAt = %v, want %v", got, rec.DeliveredAt)
	}
}

// --- slot arithmetic -------------------------------------------------------

// TestSlotFollowsSeqModuloSize documents the storage layout the ring depends
// on: the slot is derived from the sequence number and nothing else.
func TestSlotFollowsSeqModuloSize(t *testing.T) {
	repo, _ := setupDeliveryRepo(t)

	const size = 4
	recordN(t, repo, "aa:bb:cc", 6, size)

	entries, err := repo.ListByDisplay("aa:bb:cc", size)
	if err != nil {
		t.Fatalf("ListByDisplay: %v", err)
	}
	if len(entries) != size {
		t.Fatalf("got %d entries, want %d", len(entries), size)
	}
	for _, e := range entries {
		want := int(e.Seq % size)
		if e.Slot != want {
			t.Errorf("seq %d stored in slot %d, want %d", e.Seq, e.Slot, want)
		}
	}
}

package repository_test

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MySQL and MariaDB are the one supported dialect the delivery history had
// never been run against. The statements were written on SQLite and checked on
// PostgreSQL, and both of those resolve an ON CONFLICT target literally — MySQL
// does not, which is the single largest difference in this file and the one
// Record's own comment warns about without being able to prove.
//
// The server is named by WISP_TEST_MYSQL_DSN, for example:
//
//	WISP_TEST_MYSQL_DSN='wisp:wisp@tcp(127.0.0.1:3306)/wisp?charset=utf8mb4&parseTime=True&loc=UTC' go test ./...
//
// With the variable unset every test in this file skips, so a checkout with no
// MySQL to hand still runs green. sqlRecorder, dummyImage, deliveryRec,
// countDeliveryRows, recordN and insertImage are shared with the SQLite and
// PostgreSQL tests next door, so what is asserted here is the same behaviour on
// a third server rather than a third definition of it.

// --- helpers ---------------------------------------------------------------

// mysqlDSN returns the configured DSN with parseTime forced to the given value,
// whatever the environment variable said.
//
// The setting is not a detail: with parseTime absent the driver hands DATETIME
// back as bytes, which is a different result shape for every timestamp the
// package reads. Both shapes are exercised below, so neither may depend on how
// the person running the tests happened to spell their DSN.
func mysqlDSN(t *testing.T, parseTime bool) string {
	t.Helper()

	dsn := os.Getenv("WISP_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("WISP_TEST_MYSQL_DSN is not set; skipping the MySQL integration tests")
	}

	base, params, _ := strings.Cut(dsn, "?")
	kept := []string{fmt.Sprintf("parseTime=%t", parseTime)}
	for _, p := range strings.Split(params, "&") {
		if p != "" && !strings.HasPrefix(p, "parseTime=") {
			kept = append(kept, p)
		}
	}

	return base + "?" + strings.Join(kept, "&")
}

// openMySQL connects and registers the close, without touching the schema.
func openMySQL(t *testing.T, dsn string) *gorm.DB {
	t.Helper()

	conn, err := infra.NewMysqlConnection(dsn, true)
	if err != nil {
		t.Fatalf("NewMysqlConnection: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := conn.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return conn
}

// setupMySQL opens the server named by WISP_TEST_MYSQL_DSN and hands back a
// freshly migrated schema, the SQL the repositories go on to send, and the raw
// *gorm.DB for assertions the interfaces cannot make.
//
// The schema is dropped both before and after: before, so a run left half way
// by an earlier failure cannot be mistaken for this run's work, and after, so
// the database is as it was found. One database is shared by every test here,
// which is why none of them calls t.Parallel.
func setupMySQL(t *testing.T) (*gorm.DB, *sqlRecorder) {
	t.Helper()

	conn := openMySQL(t, mysqlDSN(t, true))
	dropMySQLSchema(t, conn)
	if err := conn.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	t.Cleanup(func() { dropMySQLSchema(t, conn) })

	rec := &sqlRecorder{Interface: conn.Logger}
	return conn.Session(&gorm.Session{Logger: rec}), rec
}

// dropMySQLSchema removes every table the application owns.
func dropMySQLSchema(t *testing.T, conn *gorm.DB) {
	t.Helper()
	if err := conn.Migrator().DropTable(model.AllModels()...); err != nil {
		t.Fatalf("DropTable: %v", err)
	}
}

// setupMySQLDeliveryRepo is the usual entry point: a migrated schema and the
// delivery history repository sitting on it.
func setupMySQLDeliveryRepo(t *testing.T) (repository.DeliveryHistoryRepository, *gorm.DB, *sqlRecorder) {
	t.Helper()
	conn, rec := setupMySQL(t)
	return infraRepo.NewDeliveryHistoryRepositoryImpl(conn), conn, rec
}

// mysqlColumnType returns the type MySQL gave a column, as SHOW COLUMNS would.
func mysqlColumnType(t *testing.T, conn *gorm.DB, table, column string) string {
	t.Helper()
	var typ string
	err := conn.Raw(
		"SELECT COLUMN_TYPE FROM information_schema.COLUMNS "+
			"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?",
		table, column,
	).Scan(&typ).Error
	if err != nil {
		t.Fatalf("read type of %s.%s: %v", table, column, err)
	}
	if typ == "" {
		t.Fatalf("%s.%s does not exist", table, column)
	}
	return typ
}

// mysqlIndexColumns returns each index on a table with its columns in order,
// keyed by name, together with the set of names that are unique.
func mysqlIndexColumns(t *testing.T, conn *gorm.DB, table string) (cols map[string][]string, unique map[string]bool) {
	t.Helper()

	rows := []struct {
		Name      string
		NonUnique int
		Col       string
	}{}
	err := conn.Raw(
		"SELECT INDEX_NAME AS name, NON_UNIQUE AS non_unique, COLUMN_NAME AS col "+
			"FROM information_schema.STATISTICS "+
			"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? "+
			"ORDER BY INDEX_NAME, SEQ_IN_INDEX",
		table,
	).Scan(&rows).Error
	if err != nil {
		t.Fatalf("read indexes on %s: %v", table, err)
	}

	cols, unique = map[string][]string{}, map[string]bool{}
	for _, r := range rows {
		cols[r.Name] = append(cols[r.Name], r.Col)
		if r.NonUnique == 0 {
			unique[r.Name] = true
		}
	}
	return cols, unique
}

// --- AutoMigrate -----------------------------------------------------------

// TestMySQL_DeliveryHistory_Schema is the floor the rest stands on. Every type
// in the model is spelled portably now that PostgreSQL is supported, so what
// MySQL makes of them is worth recording rather than assuming — in particular
// delivered_at, whose precision decides how much of an instant survives a round
// trip, and image_id, which MySQL is alone in keeping unsigned.
func TestMySQL_DeliveryHistory_Schema(t *testing.T) {
	conn, _ := setupMySQL(t)

	if !conn.Migrator().HasTable(&model.DeliveryHistory{}) {
		t.Fatal("delivery_histories was not created")
	}

	for _, c := range []struct{ column, want string }{
		{"display_key", "varchar(64)"},
		{"slot", "bigint(20)"},
		{"seq", "bigint(20)"},
		// datetime(3), not datetime: GORM asks for millisecond precision, and a
		// bare datetime would truncate every delivery to the whole second.
		// Nothing depends on more than that, but a silent loss of all
		// sub-second detail is worth meeting here rather than in a comparison.
		{"delivered_at", "datetime(3)"},
		{"kind", "varchar(16)"},
		// PrimaryKey is a uint64 and MySQL, unlike PostgreSQL, has an unsigned
		// bigint to put it in. The full range is therefore available here and
		// nothing is widened or wrapped.
		{"image_id", "bigint(20) unsigned"},
		{"catalog_key", "varchar(64)"},
		{"source", "varchar(2048)"},
		{"reason", "varchar(32)"},
		{"sleep_seconds", "bigint(20)"},
	} {
		if got := mysqlColumnType(t, conn, "delivery_histories", c.column); got != c.want {
			t.Errorf("delivery_histories.%s is %q, want %q", c.column, got, c.want)
		}
	}

	// Every column is NOT NULL: the read path scans straight into non-pointer
	// fields, and the ring's own bookkeeping has no absent value.
	var nullable []string
	if err := conn.Raw(
		"SELECT COLUMN_NAME FROM information_schema.COLUMNS " +
			"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'delivery_histories' AND IS_NULLABLE = 'YES'",
	).Scan(&nullable).Error; err != nil {
		t.Fatalf("read nullability: %v", err)
	}
	if len(nullable) != 0 {
		t.Errorf("these columns are nullable: %v", nullable)
	}

	cols, _ := mysqlIndexColumns(t, conn, "delivery_histories")
	if got := cols["PRIMARY"]; strings.Join(got, ",") != "display_key,slot" {
		t.Errorf("the primary key is over %v, want (display_key, slot)", got)
	}
	if got := cols["idx_delivery_recent"]; strings.Join(got, ",") != "display_key,seq" {
		t.Errorf("idx_delivery_recent is over %v, want (display_key, seq)", got)
	}

	// Start-up migrates whatever is already there, so a second pass over an
	// up-to-date schema has to be a no-op rather than an error.
	if err := conn.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("second AutoMigrate: %v", err)
	}
}

// --- the ring upsert -------------------------------------------------------

// TestMySQL_DeliveryHistory_RingUpsert is the statement the whole design rests
// on, on the dialect that rewrites it. GORM turns the ON CONFLICT into
// ON DUPLICATE KEY UPDATE and drops the named columns entirely; the ring is
// still bounded because MySQL then fires on the primary key, which is the only
// unique key there is. What that leans on is pinned by
// TestMySQL_DeliveryHistory_PrimaryKeyIsTheOnlyUniqueKey below.
func TestMySQL_DeliveryHistory_RingUpsert(t *testing.T) {
	repo, conn, rec := setupMySQLDeliveryRepo(t)

	const size = 5
	recordN(t, repo, "aa:bb:cc", 12, size)

	if n := countDeliveryRows(t, conn); n != size {
		t.Errorf("after 12 deliveries into a ring of %d, got %d rows — the upsert appended rather than overwrote", size, n)
	}

	// Having established what it does, record what it says. The conflict target
	// is gone from the statement MySQL is sent, which is the difference this
	// file exists to document.
	stmt := rec.last("ON DUPLICATE KEY UPDATE")
	if stmt == "" {
		t.Fatalf("no ON DUPLICATE KEY UPDATE statement was recorded; the upsert now reads:\n%s",
			rec.last("delivery_histories"))
	}
	if strings.Contains(stmt, "ON CONFLICT") {
		t.Errorf("the statement carries an ON CONFLICT, which MySQL would reject:\n%s", stmt)
	}
	for _, col := range []string{"seq", "delivered_at", "kind", "image_id", "catalog_key", "source", "reason", "sleep_seconds"} {
		if !strings.Contains(stmt, fmt.Sprintf("`%s`=VALUES(`%s`)", col, col)) {
			t.Errorf("%s is not assigned from the proposed row; a wrapped slot would keep the old value:\n%s", col, stmt)
		}
	}
	// The two primary-key columns are what the row is found by, so assigning
	// them would be at best pointless and at worst a moving target.
	for _, col := range []string{"display_key", "slot"} {
		if strings.Contains(stmt, fmt.Sprintf("`%s`=VALUES(`%s`)", col, col)) {
			t.Errorf("%s is assigned by the upsert, but it is part of the key being matched on:\n%s", col, stmt)
		}
	}
	t.Logf("generated upsert: %s", stmt)

	// Overwritten in place, not merely bounded: the ring holds slots 0..size-1
	// and nothing else.
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

// TestMySQL_DeliveryHistory_RecordRewritesEveryColumn: a wrapped slot must
// carry none of what the delivery it replaced left there. A column missing from
// the assignment list would leave a stale value behind that reads as this
// delivery's own.
func TestMySQL_DeliveryHistory_RecordRewritesEveryColumn(t *testing.T) {
	repo, conn, _ := setupMySQLDeliveryRepo(t)

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
	third.DeliveredAt = first.DeliveredAt.Add(time.Hour)
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
	// delivered_at is the one column a stale value would be hardest to spot in,
	// since the old one is a plausible date. MySQL stores it as datetime(3), so
	// the comparison is to the millisecond.
	if delta := stored.DeliveredAt.Sub(third.DeliveredAt); delta > time.Millisecond || delta < -time.Millisecond {
		t.Errorf("delivered_at is %v after the overwrite, want %v",
			stored.DeliveredAt.UTC(), third.DeliveredAt.UTC())
	}
}

// TestMySQL_DeliveryHistory_RecordSurvivesMissingTable is the state every
// installation that predates this feature is in: WISP_AUTO_MIGRATE is off by
// default, so the table does not exist until somebody migrates. A delivery must
// still go out, and the connection must still be usable afterwards.
func TestMySQL_DeliveryHistory_RecordSurvivesMissingTable(t *testing.T) {
	conn, _ := setupMySQL(t)
	if err := conn.Migrator().DropTable(&model.DeliveryHistory{}); err != nil {
		t.Fatalf("DropTable: %v", err)
	}
	repo := infraRepo.NewDeliveryHistoryRepositoryImpl(conn)

	if err := repo.Record(deliveryRec("aa:bb:cc"), 5); err != nil {
		t.Errorf("Record against a missing table returned %v, want nil", err)
	}

	var n int64
	if err := conn.Model(&model.Image{}).Count(&n).Error; err != nil {
		t.Errorf("the connection is unusable after the failed record: %v", err)
	}
}

// TestMySQL_DeliveryHistory_ImageIDRange: MySQL keeps image_id unsigned, so the
// whole of a Go uint64 fits. This is the one place the three dialects store
// different things, and what MySQL does with the top of the range is worth
// having written down beside PostgreSQL's signed bigint.
func TestMySQL_DeliveryHistory_ImageIDRange(t *testing.T) {
	repo, conn, _ := setupMySQLDeliveryRepo(t)

	const maxUint64 = model.PrimaryKey(1<<64 - 1)
	rec := deliveryRec("aa:bb:cc")
	rec.ImageID = maxUint64
	if err := repo.Record(rec, 5); err != nil {
		t.Fatalf("Record: %v", err)
	}
	var stored model.DeliveryHistory
	if err := conn.Where("display_key = ?", "aa:bb:cc").Take(&stored).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored.ImageID != maxUint64 {
		t.Errorf("image_id came back as %d, want %d", stored.ImageID, maxUint64)
	}
}

// --- the conflict target MySQL throws away ---------------------------------

// deliveryUpsertClause is the clause record() builds, spelled again so that the
// tests below can drive rows the repository would never produce — the whole
// point being what happens when a row conflicts on something other than the
// named target, and Record derives both key columns itself so it cannot.
//
// TestMySQL_DeliveryHistory_ProbeMatchesRecord keeps the copy honest.
func deliveryUpsertClause() clause.Expression {
	return clause.OnConflict{
		Columns: []clause.Column{{Name: "display_key"}, {Name: "slot"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"seq", "delivered_at", "kind", "image_id", "catalog_key", "source", "reason", "sleep_seconds",
		}),
	}
}

// TestMySQL_DeliveryHistory_ProbeMatchesRecord: the clause above is a copy of
// one inside the repository, and a copy that has drifted proves nothing. Both
// are sent through the same server and the upsert halves of the two statements
// have to come out identical.
func TestMySQL_DeliveryHistory_ProbeMatchesRecord(t *testing.T) {
	repo, conn, rec := setupMySQLDeliveryRepo(t)

	if err := repo.Record(deliveryRec("aa:bb:cc"), 5); err != nil {
		t.Fatalf("Record: %v", err)
	}
	fromRecord := rec.last("ON DUPLICATE KEY UPDATE")

	probe := deliveryRec("dd:ee:ff")
	probe.Seq, probe.Slot = 1, 1
	if err := conn.Clauses(deliveryUpsertClause()).Create(probe).Error; err != nil {
		t.Fatalf("probe upsert: %v", err)
	}
	fromProbe := rec.last("ON DUPLICATE KEY UPDATE")

	const marker = "ON DUPLICATE KEY UPDATE"
	_, wantTail, _ := strings.Cut(fromRecord, marker)
	_, gotTail, _ := strings.Cut(fromProbe, marker)
	if wantTail == "" {
		t.Fatalf("no upsert was recorded for Record:\n%s", fromRecord)
	}
	if gotTail != wantTail {
		t.Errorf("the probe clause has drifted from the one in record():\n probe:  %s\n record: %s", gotTail, wantTail)
	}
}

// TestMySQL_DeliveryHistory_PrimaryKeyIsTheOnlyUniqueKey is the guard the
// comment in record() asks for and could not have.
//
// GORM sends MySQL an ON DUPLICATE KEY UPDATE with no conflict target, so the
// upsert fires on whichever unique key the proposed row collides with. That is
// the ring's intended behaviour only while the primary key is the sole unique
// key on the table. Add a second unique index — on (display_key, seq), say,
// which is the one anybody would reach for — and MySQL starts overwriting rows
// the statement never named, while SQLite and PostgreSQL raise a constraint
// violation instead. TestMySQL_DeliveryHistory_SecondUniqueKeyDiverges shows
// exactly that happening; this test is what stops it from ever being reached.
func TestMySQL_DeliveryHistory_PrimaryKeyIsTheOnlyUniqueKey(t *testing.T) {
	conn, _ := setupMySQL(t)

	cols, unique := mysqlIndexColumns(t, conn, "delivery_histories")
	for name := range unique {
		if name != "PRIMARY" {
			t.Errorf("delivery_histories has a second unique index %q over %v.\n"+
				"Record's upsert reaches MySQL as ON DUPLICATE KEY UPDATE with no conflict target, "+
				"so it now fires on this index too and overwrites rows it never named. "+
				"See TestMySQL_DeliveryHistory_SecondUniqueKeyDiverges.", name, cols[name])
		}
	}
}

// TestDeliveryHistoryHasOneUniqueIndexOnSQLite is the same guard where it costs
// nothing to run. It lives beside the MySQL tests because MySQL is the only
// dialect the invariant matters to, but a checkout with no server to hand still
// gets told when the model grows a second unique index.
func TestDeliveryHistoryHasOneUniqueIndexOnSQLite(t *testing.T) {
	_, conn := setupDeliveryRepo(t)

	rows := []struct {
		Name   string
		Unique int
	}{}
	if err := conn.Raw("SELECT name, `unique` FROM pragma_index_list('delivery_histories')").Scan(&rows).Error; err != nil {
		t.Fatalf("pragma_index_list: %v", err)
	}
	// SQLite keeps the composite primary key as an index named sqlite_autoindex_*
	// rather than as a row of its own here, so the count includes it.
	var uniques []string
	for _, r := range rows {
		if r.Unique == 1 {
			uniques = append(uniques, r.Name)
		}
	}
	if len(uniques) != 1 {
		t.Errorf("delivery_histories has %d unique indexes (%v), want only the primary key.\n"+
			"On MySQL the ring upsert fires on any of them; see "+
			"TestMySQL_DeliveryHistory_PrimaryKeyIsTheOnlyUniqueKey.", len(uniques), uniques)
	}
}

// TestMySQL_DeliveryHistory_SecondUniqueKeyDiverges proves the warning rather
// than repeating it.
//
// A second unique index is added to the live schema — never to the model — and
// a row is offered that does not collide on (display_key, slot) but does
// collide on the new index. SQLite and PostgreSQL refuse it, because their
// ON CONFLICT names a target and the collision is not on that target. MySQL
// accepts it and updates a row in a different slot, so a delivery is lost
// without an error anywhere.
//
// The index is dropped again at the end; the whole schema goes with the test in
// any case.
func TestMySQL_DeliveryHistory_SecondUniqueKeyDiverges(t *testing.T) {
	conn, _ := setupMySQL(t)

	if err := conn.Exec("CREATE UNIQUE INDEX uq_delivery_seq_probe ON delivery_histories (display_key, seq)").Error; err != nil {
		t.Fatalf("create the probe index: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Exec("DROP INDEX uq_delivery_seq_probe ON delivery_histories").Error
	})

	first := deliveryRec("aa:bb:cc")
	first.Slot, first.Seq, first.CatalogKey = 0, 1, "first"
	if err := conn.Clauses(deliveryUpsertClause()).Create(first).Error; err != nil {
		t.Fatalf("the first row: %v", err)
	}

	// Slot 1 is free, so the named conflict target does not match. The new
	// index does.
	second := deliveryRec("aa:bb:cc")
	second.Slot, second.Seq, second.CatalogKey = 1, 1, "second"
	err := conn.Clauses(deliveryUpsertClause()).Create(second).Error

	if err != nil {
		t.Fatalf("MySQL rejected the colliding row with %v.\n"+
			"That is what SQLite and PostgreSQL do, and it would mean the warning in record() "+
			"about MySQL discarding the conflict target no longer holds — check the comment.", err)
	}

	var rows []model.DeliveryHistory
	if err := conn.Order("slot").Find(&rows).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1 — MySQL was expected to update the existing row rather than insert", len(rows))
	}
	// The damage in full: the row that was rewritten is the one in slot 0,
	// which the statement never named, and slot 1 was never written at all.
	if rows[0].Slot != 0 {
		t.Errorf("the surviving row is in slot %d, want 0", rows[0].Slot)
	}
	if rows[0].CatalogKey != "second" {
		t.Errorf("the surviving row holds %q, want %q — MySQL was expected to overwrite the wrong slot",
			rows[0].CatalogKey, "second")
	}
	t.Logf("MySQL fired on uq_delivery_seq_probe rather than the named (display_key, slot): "+
		"the row offered for slot 1 overwrote slot %d instead", rows[0].Slot)
}

// TestDeliveryHistory_SecondUniqueKeyRefusedOnSQLite is the other half of the
// comparison, and it runs everywhere. Same index, same clause, same rows — and
// an error rather than a silent overwrite.
func TestDeliveryHistory_SecondUniqueKeyRefusedOnSQLite(t *testing.T) {
	_, conn := setupDeliveryRepo(t)

	if err := conn.Exec("CREATE UNIQUE INDEX uq_delivery_seq_probe ON delivery_histories (display_key, seq)").Error; err != nil {
		t.Fatalf("create the probe index: %v", err)
	}

	first := deliveryRec("aa:bb:cc")
	first.Slot, first.Seq, first.CatalogKey = 0, 1, "first"
	if err := conn.Clauses(deliveryUpsertClause()).Create(first).Error; err != nil {
		t.Fatalf("the first row: %v", err)
	}

	second := deliveryRec("aa:bb:cc")
	second.Slot, second.Seq, second.CatalogKey = 1, 1, "second"
	if err := conn.Clauses(deliveryUpsertClause()).Create(second).Error; err == nil {
		t.Fatal("SQLite accepted a row colliding on an index the ON CONFLICT does not name; " +
			"it was expected to raise a constraint violation")
	}

	var rows []model.DeliveryHistory
	if err := conn.Order("slot").Find(&rows).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(rows) != 1 || rows[0].CatalogKey != "first" {
		t.Errorf("the refused row changed something: %+v", rows)
	}
}

// --- ListByDisplay ---------------------------------------------------------

// TestMySQL_DeliveryHistory_ListByDisplay covers the LEFT JOIN. The CASE exists
// so that image_available arrives as 1 or 0 on every dialect rather than as a
// boolean on some; what it has to end up as is a Go bool either way.
func TestMySQL_DeliveryHistory_ListByDisplay(t *testing.T) {
	repo, conn, rec := setupMySQLDeliveryRepo(t)

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

// TestMySQL_DeliveryHistory_ImageAvailableSoftDelete is the distinction the
// join is deliberately built around. Hiding a photograph in the UI is a soft
// delete and reversible, so the history entry stays viewable; a real purge is
// not, so it does not.
func TestMySQL_DeliveryHistory_ImageAvailableSoftDelete(t *testing.T) {
	repo, conn, _ := setupMySQLDeliveryRepo(t)

	hidden := insertImage(t, conn, "album")
	purged := insertImage(t, conn, "album")
	for _, img := range []*model.Image{hidden, purged} {
		rec := deliveryRec("aa:bb:cc")
		rec.ImageID = img.ID
		if err := repo.Record(rec, 5); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	if err := conn.Where("id = ?", hidden.ID).Delete(&model.Image{}).Error; err != nil {
		t.Fatalf("hide image: %v", err)
	}
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

// TestMySQL_DeliveryHistory_ListByDisplayBounds: an undelivered display is zero
// rows rather than one empty one, and a non-positive limit asks for nothing
// without reaching the server.
func TestMySQL_DeliveryHistory_ListByDisplayBounds(t *testing.T) {
	repo, _, _ := setupMySQLDeliveryRepo(t)

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

// TestMySQL_DeliveryHistory_Summary is where MySQL's result types are least
// like the Go fields they are scanned into: SUM over integers comes back as
// DECIMAL rather than as an integer of any width, COUNT(*) as a bigint, and
// MAX(delivered_at) as a DATETIME the driver parses only because the DSN says
// parseTime. All four land in a plain Go int, int64 or time.Time.
func TestMySQL_DeliveryHistory_Summary(t *testing.T) {
	repo, _, rec := setupMySQLDeliveryRepo(t)

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
		t.Errorf("display:a Entries = %d, want 5", a.Entries)
	}
	if a.ErrorEntries != 2 {
		t.Errorf("display:a ErrorEntries = %d, want 2 — SUM comes back as a DECIMAL here", a.ErrorEntries)
	}
	if a.LastSeq != 5 {
		t.Errorf("display:a LastSeq = %d, want 5", a.LastSeq)
	}
	// The instant, not the offset: the driver hands the timestamp back in the
	// zone the DSN's loc names, and only the moment it names is the
	// repository's promise.
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

// TestMySQL_DeliveryHistory_SummaryEmpty: an installation that has delivered
// nothing yet returns no rows rather than failing on a SUM over none.
func TestMySQL_DeliveryHistory_SummaryEmpty(t *testing.T) {
	repo, _, _ := setupMySQLDeliveryRepo(t)

	summaries, err := repo.SummaryByDisplay()
	if err != nil {
		t.Fatalf("SummaryByDisplay: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("got %d summaries, want 0", len(summaries))
	}
}

// TestMySQL_DeliveryHistory_WithoutParseTime pins what a DSN missing parseTime
// does, since a user can write one and the repository comments assume both
// shapes are handled.
//
// They are not equally handled, and the difference is worth stating plainly:
//
//   - SummaryByDisplay survives, because MAX(delivered_at) goes through the
//     aggregatedTime shim, which parses the driver's raw bytes. It survives
//     with the wrong instant, though — the stored text carries no zone, so the
//     shim reads it as UTC whatever the DSN's loc says.
//   - ListByDisplay does not survive at all. delivered_at is scanned straight
//     into a time.Time with no shim in the way, and database/sql refuses the
//     bytes.
//
// The second is not specific to the delivery history: Find on any model with a
// time column fails the same way, so a MySQL DSN without parseTime does not
// give a working installation, and every DSN the project documents includes it.
// The assertions below therefore record behaviour rather than endorse it.
func TestMySQL_DeliveryHistory_WithoutParseTime(t *testing.T) {
	repo, _, _ := setupMySQLDeliveryRepo(t)

	stored := time.Date(2026, 3, 2, 12, 34, 56, 0, time.UTC)
	rec := deliveryRec("aa:bb:cc")
	rec.DeliveredAt = stored
	if err := repo.Record(rec, 5); err != nil {
		t.Fatalf("Record: %v", err)
	}

	plain := infraRepo.NewDeliveryHistoryRepositoryImpl(openMySQL(t, mysqlDSN(t, false)))

	// Writing is unaffected: the driver formats a time.Time itself.
	if err := plain.Record(deliveryRec("aa:bb:cc"), 5); err != nil {
		t.Errorf("Record without parseTime: %v", err)
	}

	summaries, err := plain.SummaryByDisplay()
	if err != nil {
		t.Fatalf("SummaryByDisplay without parseTime: %v — the aggregatedTime shim no longer covers "+
			"the driver's raw bytes, which is the only reason this call works at all", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("got %d summaries, want 1", len(summaries))
	}
	if summaries[0].LastDeliveredAt.IsZero() {
		t.Error("LastDeliveredAt is zero without parseTime; the shim did not parse the raw timestamp")
	}

	// The listing is the half that fails, and it fails per row rather than
	// returning nothing, so the error is the only sign anything went wrong.
	if _, err := plain.ListByDisplay("aa:bb:cc", 5); err == nil {
		t.Log("ListByDisplay now succeeds without parseTime; the comment on aggregatedTime, " +
			"and this test, can be simplified")
	} else if !strings.Contains(err.Error(), "delivered_at") {
		t.Errorf("ListByDisplay without parseTime failed with %v, which is not the timestamp "+
			"conversion this test was written for", err)
	}
}

// --- Reconcile -------------------------------------------------------------

// TestMySQL_DeliveryHistory_Reconcile covers both deletes, the second of which
// puts a Go slice into a NOT IN. It runs at start-up, so running it twice must
// be the same as running it once.
func TestMySQL_DeliveryHistory_Reconcile(t *testing.T) {
	repo, conn, rec := setupMySQLDeliveryRepo(t)

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

	// Neither delete may be a soft one. DeliveryHistory has no deleted_at, but
	// GORM decides that from the model rather than from the statement, so it is
	// worth seeing the rows actually leave.
	var raw int64
	if err := conn.Raw("SELECT COUNT(*) FROM delivery_histories").Scan(&raw).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if raw != 4 {
		t.Errorf("the table holds %d rows, want 4", raw)
	}
	t.Logf("generated deletes: %s | %s", rec.last("slot >="), rec.last("NOT IN"))

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

// TestMySQL_DeliveryHistory_ReconcileKeepsEverythingWithoutKeys: an empty key
// list reads as "nothing is configured yet", not as permission to delete every
// display's history — and a NOT IN over an empty slice is a statement MySQL
// would reject outright, so the guard has to hold before the SQL is built.
func TestMySQL_DeliveryHistory_ReconcileKeepsEverythingWithoutKeys(t *testing.T) {
	repo, conn, _ := setupMySQLDeliveryRepo(t)

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

// TestMySQL_DeliveryHistory_DropAndRecreate: system prune drops and rebuilds
// every model, and the history is the one whose table carries a composite
// primary key. On MySQL that key is also the only thing bounding the ring, so
// coming back is not enough — it has to come back with the key.
func TestMySQL_DeliveryHistory_DropAndRecreate(t *testing.T) {
	repo, conn, _ := setupMySQLDeliveryRepo(t)

	recordN(t, repo, "aa:bb:cc", 3, 5)
	sys := infraRepo.NewSystemRepositoryImpl(conn)
	if err := sys.DropAndRecreate(); err != nil {
		t.Fatalf("DropAndRecreate: %v", err)
	}

	for _, m := range model.AllModels() {
		if !conn.Migrator().HasTable(m) {
			t.Fatalf("table for %T was not recreated", m)
		}
	}
	if n := countDeliveryRows(t, conn); n != 0 {
		t.Errorf("%d deliveries survived the prune, want 0", n)
	}

	cols, unique := mysqlIndexColumns(t, conn, "delivery_histories")
	if got := cols["PRIMARY"]; strings.Join(got, ",") != "display_key,slot" {
		t.Errorf("the primary key after a prune is over %v, want (display_key, slot)", got)
	}
	if len(unique) != 1 {
		t.Errorf("delivery_histories has %d unique indexes after a prune, want 1", len(unique))
	}

	// Writable and readable again, through the same repository on the same
	// connection.
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
}

// --- concurrency -----------------------------------------------------------

// TestMySQL_DeliveryHistory_RecordConcurrent: several requests can land at once
// on a server that feeds more than one panel, or on one panel retrying. The
// mutex in the repository serialises them within a process, and MySQL is the
// dialect where the upsert is doing something other than what the code says, so
// the bound is worth watching under parallelism as well as in sequence.
func TestMySQL_DeliveryHistory_RecordConcurrent(t *testing.T) {
	repo, conn, _ := setupMySQLDeliveryRepo(t)

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

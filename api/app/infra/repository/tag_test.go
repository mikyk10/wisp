package repository_test

import (
	"testing"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/infra"
	infraRepo "github.com/mikyk10/wisp/app/infra/repository"

	"gorm.io/gorm"
)

// setupTagRepo returns both repositories over one database: every question
// about tags is really a question about which images carry them, so the tests
// need to write images too.
func setupTagRepo(t *testing.T) (repository.TagRepository, repository.ImageRepository, *gorm.DB) {
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
	if err := conn.AutoMigrate(&model.Image{}, &model.Tag{}, &model.ImageTag{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return infraRepo.NewTagRepositoryImpl(conn), infraRepo.NewImageRepositoryImpl(conn), conn
}

// tagImage stores an image in the catalogue and gives it the named tags.
func tagImage(t *testing.T, tagr repository.TagRepository, conn *gorm.DB, catalogKey string, names ...string) model.PrimaryKey {
	t.Helper()
	img := dummyImage(catalogKey)
	if err := conn.Create(img).Error; err != nil {
		t.Fatalf("create image: %v", err)
	}

	ids := make([]model.PrimaryKey, 0, len(names))
	for _, name := range names {
		tag, err := tagr.FindOrCreateTag(name)
		if err != nil {
			t.Fatalf("FindOrCreateTag(%q): %v", name, err)
		}
		ids = append(ids, tag.ID)
	}
	if len(ids) > 0 {
		if err := tagr.ReplaceImageTags(img.ID, ids); err != nil {
			t.Fatalf("ReplaceImageTags: %v", err)
		}
	}
	return img.ID
}

// TestFindTagUsage_CountsAndOrders: the picker shows this list, so the order is
// the feature. Most used first is what makes a list of hundreds usable at all.
func TestFindTagUsage_CountsAndOrders(t *testing.T) {
	tagr, _, conn := setupTagRepo(t)

	tagImage(t, tagr, conn, "cat", "sky", "sea")
	tagImage(t, tagr, conn, "cat", "sky")
	tagImage(t, tagr, conn, "cat", "sky")
	tagImage(t, tagr, conn, "cat", "sea")

	usage, err := tagr.FindTagUsage("cat")
	if err != nil {
		t.Fatalf("FindTagUsage: %v", err)
	}
	if len(usage) != 2 {
		t.Fatalf("got %d tags, want 2: %+v", len(usage), usage)
	}
	if usage[0].Name != "sky" || usage[0].Count != 3 {
		t.Errorf("first entry = %+v, want sky×3", usage[0])
	}
	if usage[1].Name != "sea" || usage[1].Count != 2 {
		t.Errorf("second entry = %+v, want sea×2", usage[1])
	}
}

// TestFindTagUsage_IsPerCatalog: tags are shared between catalogues but the
// picker filters one. Offering a tag that matches nothing in the catalogue on
// screen is worse than not offering it — the reader picks it and the grid
// empties.
func TestFindTagUsage_IsPerCatalog(t *testing.T) {
	tagr, _, conn := setupTagRepo(t)

	tagImage(t, tagr, conn, "here", "shared")
	tagImage(t, tagr, conn, "elsewhere", "shared")
	tagImage(t, tagr, conn, "elsewhere", "elsewhere-only")

	usage, err := tagr.FindTagUsage("here")
	if err != nil {
		t.Fatalf("FindTagUsage: %v", err)
	}
	if len(usage) != 1 || usage[0].Name != "shared" {
		t.Fatalf("got %+v, want only shared", usage)
	}
	if usage[0].Count != 1 {
		t.Errorf("count = %d, want 1 — the other catalogue's image must not be counted", usage[0].Count)
	}
}

// TestFindTagUsage_SkipsExcluded: an excluded image is not in the listing the
// filter applies to, so counting it would promise photos that cannot appear.
func TestFindTagUsage_SkipsExcluded(t *testing.T) {
	tagr, _, conn := setupTagRepo(t)

	id := tagImage(t, tagr, conn, "cat", "hidden")
	tagImage(t, tagr, conn, "cat", "hidden")
	if err := conn.Model(&model.Image{}).Where("id = ?", id).Update("excluded", true).Error; err != nil {
		t.Fatalf("exclude: %v", err)
	}

	usage, err := tagr.FindTagUsage("cat")
	if err != nil {
		t.Fatalf("FindTagUsage: %v", err)
	}
	if len(usage) != 1 || usage[0].Count != 1 {
		t.Fatalf("got %+v, want hidden×1", usage)
	}
}

// TestLoadCatalogImageTags: one read for the whole catalogue is the point —
// the listing streams tens of thousands of rows and a lookup per row would be
// a query per row.
func TestLoadCatalogImageTags(t *testing.T) {
	tagr, _, conn := setupTagRepo(t)

	withTwo := tagImage(t, tagr, conn, "cat", "sky", "sea")
	withNone := tagImage(t, tagr, conn, "cat")

	byImage, err := tagr.LoadCatalogImageTags("cat")
	if err != nil {
		t.Fatalf("LoadCatalogImageTags: %v", err)
	}

	got := byImage[withTwo]
	if len(got) != 2 {
		t.Fatalf("tags for the tagged image = %v, want two", got)
	}
	// Display names, not ids: the grid shows these.
	if got[0] != "sky" && got[1] != "sky" {
		t.Errorf("tags = %v, want them to include sky", got)
	}

	// Absent rather than empty is fine — the caller reads a missing key as no
	// tags — but a photograph with no tags must not pick up someone else's.
	if len(byImage[withNone]) != 0 {
		t.Errorf("untagged image has %v", byImage[withNone])
	}
}

// TestListByCatalog_TagFilterIsAnd: every tag, not any. A filter is read as
// narrowing, and an OR would widen the result as the reader adds terms — the
// opposite of what the second click is asking for.
func TestListByCatalog_TagFilterIsAnd(t *testing.T) {
	tagr, imgr, conn := setupTagRepo(t)

	both := tagImage(t, tagr, conn, "cat", "sky", "sea")
	tagImage(t, tagr, conn, "cat", "sky")
	tagImage(t, tagr, conn, "cat", "sea")

	var seen []model.PrimaryKey
	if err := imgr.ListByCatalog("cat", []string{"sky", "sea"}, func(img *model.Image) error {
		seen = append(seen, img.ID)
		return nil
	}); err != nil {
		t.Fatalf("ListByCatalog: %v", err)
	}

	if len(seen) != 1 || seen[0] != both {
		t.Fatalf("got ids %v, want only the image carrying both (%d)", seen, both)
	}
}

// TestListByCatalog_NoTagsMeansNoFilter: an empty filter is "show everything",
// which is what a reader who has just cleared the last tag is asking for. Read
// as "match nothing" it would empty the grid instead.
func TestListByCatalog_NoTagsMeansNoFilter(t *testing.T) {
	tagr, imgr, conn := setupTagRepo(t)

	tagImage(t, tagr, conn, "cat", "sky")
	tagImage(t, tagr, conn, "cat")

	for _, filter := range [][]string{nil, {}, {"", "  "}} {
		count := 0
		if err := imgr.ListByCatalog("cat", filter, func(*model.Image) error {
			count++
			return nil
		}); err != nil {
			t.Fatalf("ListByCatalog(%v): %v", filter, err)
		}
		if count != 2 {
			t.Errorf("filter %v returned %d images, want all 2", filter, count)
		}
	}
}

// TestListByCatalog_TagFilterIsCaseInsensitive: tags are stored normalized and
// the client sends back the display name it was given. The two agree today
// because the tagger lowercases, and this keeps them agreeing if it stops.
func TestListByCatalog_TagFilterIsCaseInsensitive(t *testing.T) {
	tagr, imgr, conn := setupTagRepo(t)

	want := tagImage(t, tagr, conn, "cat", "sky")

	var seen []model.PrimaryKey
	if err := imgr.ListByCatalog("cat", []string{" SKY "}, func(img *model.Image) error {
		seen = append(seen, img.ID)
		return nil
	}); err != nil {
		t.Fatalf("ListByCatalog: %v", err)
	}
	if len(seen) != 1 || seen[0] != want {
		t.Fatalf("got %v, want the image tagged sky (%d)", seen, want)
	}
}

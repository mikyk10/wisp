package repository

import (
	"database/sql"
	"errors"
	"math/rand/v2"
	"strings"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type imageRepositoryImpl struct {
	conn *gorm.DB
}

func NewImageRepositoryImpl(conn *gorm.DB) repository.ImageRepository {
	return &imageRepositoryImpl{conn: conn}
}

func (p *imageRepositoryImpl) RemoveImage(id model.PrimaryKey) error {
	return p.conn.Unscoped().Where("id = ?", id).Delete(&model.Image{}).Error
}

func (p *imageRepositoryImpl) FindAll(cb func(*model.Image) error) {
	imgs := []*model.Image{}
	// ThumbJPG is not needed to verify whether Src exists; omit it to reduce memory usage.
	p.conn.Unscoped().Omit("thumb_jpg", "image_data").FindInBatches(&imgs, 100, func(tx *gorm.DB, batch int) error {
		for _, c := range imgs {
			if err := cb(c); err != nil {
				return err
			}
		}

		return nil
	})
}

func (p *imageRepositoryImpl) ToggleDeletedAt(ids []model.PrimaryKey) error {
	return p.conn.Exec(
		"UPDATE images SET deleted_at = CASE WHEN deleted_at IS NULL THEN CURRENT_TIMESTAMP ELSE NULL END WHERE id IN ?",
		ids,
	).Error
}

func (p *imageRepositoryImpl) FindById(id model.PrimaryKey) (*model.Image, error) {
	img := &model.Image{}
	if err := p.conn.Unscoped().Where("id = ?", id).First(img).Error; err != nil {
		return nil, err
	}
	return img, nil
}

func (p *imageRepositoryImpl) CountByCatalog(catalogKey string, ori model.CanonicalOrientation) (int64, error) {
	var count int64
	err := p.conn.Model(&model.Image{}).
		Where("catalog_key = ? AND image_orientation = ? AND excluded = false", catalogKey, ori).
		Count(&count).Error
	return count, err
}

func (p *imageRepositoryImpl) CountAllByCatalog(catalogKey string) (int64, error) {
	var count int64
	err := p.conn.Model(&model.Image{}).
		Where("catalog_key = ? AND excluded = false", catalogKey).
		Count(&count).Error
	return count, err
}

func (p *imageRepositoryImpl) FindByHash(catalogKey, srcHash string) (*model.Image, error) {
	existing := &model.Image{}
	// Unscoped: include soft-deleted rows (deleted_at IS NOT NULL).
	// Without this, GORM silently adds WHERE deleted_at IS NULL, causing user-hidden images
	// to appear as "not found" — forcing a full re-decode and re-thumbnail on every scan,
	// and worse, the subsequent upsert would reset deleted_at to NULL (un-hiding them).
	err := p.conn.Unscoped().Select("file_modified_at").
		Where("catalog_key = ? AND src_hash = ?", catalogKey, srcHash).
		Take(existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return existing, nil
}

func (p *imageRepositoryImpl) UpsertActiveImage(rec *model.Image) error {
	return p.conn.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "catalog_key"}, {Name: "src_hash"}},
		// deleted_at is intentionally excluded: it is owned by the user (visibility toggle)
		// and must not be overwritten by a scan. Resetting it here would un-hide images the
		// user explicitly hid via the UI.
		// excluded is updated to handle files that move between included/excluded criteria.
		DoUpdates: clause.AssignmentColumns([]string{"image_orientation", "rnd", "taken_at", "thumb_jpg", "file_modified_at", "excluded", "src_type", "image_data"}),
	}).Save(rec).Error
}

func (p *imageRepositoryImpl) UpsertInactiveImage(catalogKey, srcHash, src string) error {
	return p.conn.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "catalog_key"}, {Name: "src_hash"}},
		// Update only excluded = true. deleted_at is reserved for user operations and must not be touched.
		DoUpdates: clause.AssignmentColumns([]string{"excluded"}),
	}).Save(&model.Image{
		CatalogKey:       catalogKey,
		Src:              src,
		SrcHash:          srcHash,
		FileModifiedAt:   sql.NullTime{},
		ImageOrientation: model.ImgCanonicalOrientationNone,
		ThumbJPG:         []byte{},
		Excluded:         true,
	}).Error
}

func (p *imageRepositoryImpl) ListByCatalog(catalogKey string, cb func(*model.Image) error) error {
	rows, err := p.conn.Unscoped().Model(&model.Image{}).
		Select("id", "catalog_key", "src", "taken_at", "created_at", "deleted_at").
		// excluded = false: completely hide catalog-excluded entries (negative index).
		// Use Unscoped so that user-hidden images (deleted_at IS NOT NULL) are still included.
		Where("catalog_key = ? AND excluded = false", catalogKey).
		Order("taken_at desc").
		Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		rec := model.Image{}
		if err := p.conn.ScanRows(rows, &rec); err != nil {
			return err
		}
		if err := cb(&rec); err != nil {
			return err
		}
	}
	return nil
}

func (p *imageRepositoryImpl) FindByRandom(filter model.ImageFilter) (*model.Image, error) {
	rnd := rand.Float64()

	buildQuery := func(op string) *gorm.DB {
		q := p.conn.Model(&model.Image{}).
			Where("catalog_key IN ? AND image_orientation = ? AND excluded = false", filter.CatalogKeys, filter.Orientation)
		if op == ">=" {
			q = q.Where("rnd >= ?", rnd)
		} else {
			q = q.Where("rnd < ?", rnd)
		}
		if len(filter.Tags) > 0 {
			q = q.Where("EXISTS (SELECT 1 FROM image_tags INNER JOIN tags ON tags.id = image_tags.tag_id WHERE image_tags.image_id = images.id AND tags.name_normalized IN ?)", filter.Tags)
		}
		return q.Order("rnd ASC")
	}

	img := &model.Image{}
	err := buildQuery(">=").First(img).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if img.ID == 0 {
		err = buildQuery("<").First(img).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	if img.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	img.Rnd = rand.Float64()
	// A failure to update rnd indicates a system-level issue such as a read-only DB or lock.
	// Image delivery has succeeded, but propagate the error rather than hiding the failure.
	if err := p.conn.Model(img).Update("rnd", img.Rnd).Error; err != nil {
		return nil, err
	}

	return img, nil
}

func (p *imageRepositoryImpl) FindImageData(id model.PrimaryKey) ([]byte, error) {
	img := &model.Image{}
	if err := p.conn.Select("image_data").Where("id = ?", id).First(img).Error; err != nil {
		return nil, err
	}
	return img.ImageData, nil
}

func (p *imageRepositoryImpl) EvictOldestImages(catalogKey string, count int) error {
	// Subquery to find IDs of oldest images, then hard-delete them.
	// Using Unscoped to bypass soft-delete and perform physical deletion.
	var ids []model.PrimaryKey
	if err := p.conn.Model(&model.Image{}).
		Where("catalog_key = ? AND excluded = false", catalogKey).
		Order("created_at ASC").
		Limit(count).
		Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	return p.conn.Unscoped().Where("id IN ?", ids).Delete(&model.Image{}).Error
}

// reshuffleBatchSize is how many rows go into one CASE expression.
//
// Against a MariaDB on the far side of a network, 500 rows take a little over
// 200ms each, so a 190k-row catalogue costs roughly a minute and a half spread
// across some four hundred statements. Larger batches do not help: the cost of
// evaluating the CASE overtakes the round trip it saves.
const reshuffleBatchSize = 500

func (p *imageRepositoryImpl) ReshuffleRandom(progress func(done, total int)) error {
	var ids []model.PrimaryKey
	if err := p.conn.Unscoped().Model(&model.Image{}).Order("id").Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	total := float64(len(ids))

	if progress == nil {
		progress = func(int, int) {}
	}
	// Reported before any work, so a caller can say what is about to happen
	// and how much of it there is. On a large catalogue this pass takes long
	// enough that an unexplained silence in the log is worse than the noise it
	// replaced.
	progress(0, len(ids))

	// Which row gets which value is decided at random, and that matters more
	// than it looks. A scan registers one catalogue at a time, so ids arrive in
	// catalogue order; handing out the values in that order would give each
	// catalogue a contiguous band of the range. A display that draws on some
	// catalogues but not others would then be selecting across a range with a
	// hole in it, and every draw landing in the hole returns the one row just
	// past it. With three catalogues and a display using two, that single row
	// answered nearly a third of all requests.
	ranks := rand.Perm(len(ids))

	// Every statement here is slow by the logger's reckoning and carries five
	// hundred WHEN clauses, so leaving them to the shared logger buries a
	// week's worth of real messages under tens of megabytes of arithmetic.
	// Errors still surface: they are returned, and the caller logs them.
	db := p.conn.Session(&gorm.Session{Logger: p.conn.Logger.LogMode(logger.Error)})

	// Deliberately not one transaction. Holding write locks on every row for
	// the length of the whole pass would put the delivery path — which updates
	// rnd on each request — at risk of waiting out its lock timeout, and this
	// runs on the same evening a panel may well wake. Committing per batch
	// keeps each lock to about the length of one statement.
	//
	// Stopping half way is therefore possible, and harmless: the catalogue is
	// left partly evenly spaced and partly as it was, which is no worse than
	// before and is put right by the next run.
	//
	// One statement shape serves both SQLite and MariaDB on purpose. The
	// obvious alternative — a single UPDATE driven by ROW_NUMBER() — has to be
	// written once per dialect, since SQLite wants UPDATE ... FROM and MariaDB
	// wants UPDATE ... JOIN. Worse, MariaDB divides integers into a DECIMAL
	// rounded to four places by default, so ROW_NUMBER()/COUNT(*) quietly
	// collapses 150k rows onto ten thousand distinct values: plausible enough
	// to pass review, and impossible to reproduce on the SQLite used in
	// development.
	for start := 0; start < len(ids); start += reshuffleBatchSize {
		end := min(start+reshuffleBatchSize, len(ids))

		var sb strings.Builder
		args := make([]any, 0, (end-start)*2+2)

		sb.WriteString("UPDATE images SET rnd = CASE id ")
		for i := start; i < end; i++ {
			sb.WriteString("WHEN ? THEN ? ")
			args = append(args, ids[i], float64(ranks[i]+1)/total)
		}
		// ids are ascending, so bounding by the batch's own range keeps the
		// statement off every row it has no value for.
		sb.WriteString("END WHERE id BETWEEN ? AND ?")
		args = append(args, ids[start], ids[end-1])

		if err := db.Exec(sb.String(), args...).Error; err != nil {
			return err
		}

		progress(end, len(ids))
	}

	return nil
}

package model

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
)

type Image struct {
	ID         PrimaryKey `gorm:"primary_key;auto_increment:true"`
	CatalogKey string     `gorm:"type:varchar(64);not null;index:idx_random,priority:1;index:idx_list_catalog,priority:1;uniqueIndex:idx_catalog_src,priority:1"`
	Rnd        float64    `gorm:"type:double;not null;index:idx_random,priority:5"`
	Src        string     `gorm:"not null;type:varchar(2048)"`
	SrcHash    string     `gorm:"not null;type:char(40);uniqueIndex:idx_catalog_src,priority:2"`
	SrcType    string     `gorm:"type:varchar(16);not null;default:'file'"`

	TakenAt          sql.NullTime         `gorm:"index:idx_list_all,priority:2;index:idx_list_catalog,priority:3"`
	ImageOrientation CanonicalOrientation `gorm:"type:int;index:idx_random,priority:2"`
	ThumbJPG         []byte               `gorm:"not null;type:blob;"`
	ImageData        []byte               `gorm:"type:blob;"`

	// Excluded: excluded by catalog configuration (negative index). Records with true are hidden from listings.
	// User-initiated hiding uses DeletedAt. The two are not managed in the same column.
	// idx_list_catalog: placed at priority:2 to cover the WHERE catalog_key=? AND excluded=false clause in ListByCatalog.
	// The (catalog_key, excluded, taken_at) order allows equality plus sort to be handled entirely by the index.
	Excluded bool `gorm:"not null;default:false;index:idx_random,priority:4;index:idx_list_catalog,priority:2"`

	FileModifiedAt sql.NullTime   `gorm:"null;"`
	DeletedAt      gorm.DeletedAt `gorm:"index:idx_random,priority:3;index:idx_list_all,priority:1;index:idx_list_catalog,priority:4"`
	CreatedAt      time.Time      `gorm:"not null;"`
	UpdatedAt      time.Time      `gorm:"not null;"`
}

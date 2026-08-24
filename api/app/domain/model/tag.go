package model

import "time"

// Tag represents a normalized tag.
type Tag struct {
	ID             PrimaryKey `gorm:"primaryKey;autoIncrement"`
	NameNormalized string     `gorm:"type:varchar(128);not null;uniqueIndex"`
	DisplayName    string     `gorm:"type:varchar(128);not null"`
	CreatedAt      time.Time  `gorm:"not null;"`
	UpdatedAt      time.Time  `gorm:"not null;"`
}

// ImageTag links an image to a tag.
type ImageTag struct {
	ImageID   PrimaryKey `gorm:"primaryKey;index:idx_tag_lookup,priority:2"`
	TagID     PrimaryKey `gorm:"primaryKey;index:idx_tag_lookup,priority:1"`
	CreatedAt time.Time  `gorm:"not null;"`
}

// TagUsage is one tag and how many images in a catalogue carry it.
//
// The count is what makes a long tag list usable: it says which tags are worth
// reaching for, and it lets a reader avoid the combinations that would return
// nothing before spending a round trip on them.
type TagUsage struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

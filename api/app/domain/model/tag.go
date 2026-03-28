package model

import "time"

// Tag represents a normalized tag.
type Tag struct {
	ID             PrimaryKey `gorm:"primaryKey;autoIncrement"`
	NameNormalized string     `gorm:"type:varchar(128);not null;uniqueIndex"`
	DisplayName    string     `gorm:"type:varchar(128);not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ImageTag links an image to a tag.
type ImageTag struct {
	ImageID   PrimaryKey `gorm:"primaryKey"`
	TagID     PrimaryKey `gorm:"primaryKey"`
	CreatedAt time.Time
}

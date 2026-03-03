package model

import (
	"database/sql"
	"time"
)

type Tag struct {
	ID             PrimaryKey `gorm:"primaryKey;autoIncrement"`
	NameNormalized string     `gorm:"type:varchar(128);not null;uniqueIndex"`
	DisplayName    string     `gorm:"type:varchar(128);not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ImageTag struct {
	ImageID     PrimaryKey `gorm:"primaryKey"`
	TagID       PrimaryKey `gorm:"primaryKey"`
	SourceRunID PrimaryKey `gorm:"not null;index"`
	Score       *float64   `gorm:"type:double"`
	CreatedAt   time.Time
}

type AIRunStage string

const (
	AIRunStageDescriptor AIRunStage = "descriptor"
	AIRunStageTagging    AIRunStage = "tagging"
)

var validStages = []AIRunStage{AIRunStageDescriptor, AIRunStageTagging}

// ValidStages returns all known pipeline stages.
func ValidStages() []AIRunStage { return validStages }

// IsValid reports whether s is a known pipeline stage.
func (s AIRunStage) IsValid() bool {
	for _, v := range validStages {
		if s == v {
			return true
		}
	}
	return false
}

type AIRunStatus string

const (
	AIRunStatusPending AIRunStatus = "pending"
	AIRunStatusRunning AIRunStatus = "running"
	AIRunStatusSuccess AIRunStatus = "success"
	AIRunStatusFailed  AIRunStatus = "failed"
)

type AIRun struct {
	ID            PrimaryKey   `gorm:"primaryKey;autoIncrement"`
	ImageID       PrimaryKey   `gorm:"not null;index:idx_run_image_stage_status,priority:1"`
	Stage         AIRunStage   `gorm:"type:varchar(32);not null;index:idx_run_image_stage_status,priority:2"`
	ModelName     string       `gorm:"type:varchar(128);not null"`
	PromptVersion string       `gorm:"type:varchar(32)"`
	PromptHash    string       `gorm:"type:char(12)"`
	Status        AIRunStatus  `gorm:"type:varchar(32);not null;index:idx_run_image_stage_status,priority:3"`
	StartedAt     time.Time    `gorm:"not null"`
	FinishedAt    sql.NullTime
	RetryCount    int          `gorm:"not null;default:0"`
	ErrorCode     string       `gorm:"type:varchar(64)"`
	ErrorMessage  string       `gorm:"type:text"`
	LatencyMs     int64
	InputHash     string `gorm:"type:char(40)"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AIOutput struct {
	ID          PrimaryKey `gorm:"primaryKey;autoIncrement"`
	RunID       PrimaryKey `gorm:"not null;uniqueIndex"`
	ContentText string     `gorm:"type:text;not null"`
	CreatedAt   time.Time
}

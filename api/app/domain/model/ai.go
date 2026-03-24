package model

import (
	"database/sql"
	"time"
)

// ExecutionStatus represents the status of a pipeline or step execution.
type ExecutionStatus string

const (
	StatusPending ExecutionStatus = "pending"
	StatusRunning ExecutionStatus = "running"
	StatusSuccess ExecutionStatus = "success"
	StatusFailed  ExecutionStatus = "failed"
)

// PipelineExecution represents one run of a pipeline for a single image/generation.
type PipelineExecution struct {
	ID            PrimaryKey      `gorm:"primaryKey;autoIncrement"`
	PipelineType  string          `gorm:"type:varchar(32);not null;index"`
	CatalogKey    string          `gorm:"type:varchar(64);not null;index"`
	SourceImageID *PrimaryKey     `gorm:"index"`
	Status        ExecutionStatus `gorm:"type:varchar(32);not null"`
	StartedAt     time.Time       `gorm:"not null"`
	FinishedAt    sql.NullTime
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// StepExecution records an individual stage execution within a pipeline.
type StepExecution struct {
	ID                  PrimaryKey      `gorm:"primaryKey;autoIncrement"`
	PipelineExecutionID PrimaryKey      `gorm:"not null;index"`
	StageName           string          `gorm:"type:varchar(64);not null"`
	StageIndex          int             `gorm:"not null"`
	ProviderName        string          `gorm:"type:varchar(64)"`
	ModelName           string          `gorm:"type:varchar(128)"`
	PromptHash          string          `gorm:"type:char(12)"`
	Status              ExecutionStatus `gorm:"type:varchar(32);not null"`
	StartedAt           time.Time       `gorm:"not null"`
	FinishedAt          sql.NullTime
	LatencyMs           int64
	ErrorCode           string `gorm:"type:varchar(64)"`
	ErrorMessage        string `gorm:"type:text"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// StepOutput stores the output of a step execution. Text and blob are separate columns.
type StepOutput struct {
	ID              PrimaryKey `gorm:"primaryKey;autoIncrement"`
	StepExecutionID PrimaryKey `gorm:"not null;uniqueIndex"`
	ContentType     string     `gorm:"type:varchar(64);not null"`
	ContentText     *string    `gorm:"type:text"`
	ContentBlob     []byte     `gorm:"type:blob"`
	CreatedAt       time.Time
}

// GenerationCacheEntry holds a cached generated image for frame delivery.
type GenerationCacheEntry struct {
	ID                  PrimaryKey `gorm:"primaryKey;autoIncrement"`
	CatalogKey          string     `gorm:"type:varchar(64);not null;index:idx_cache_random,priority:1"`
	PipelineExecutionID PrimaryKey `gorm:"not null;uniqueIndex"`
	ImageData           []byte     `gorm:"type:blob;not null"`
	ContentType         string     `gorm:"type:varchar(64);not null;default:'image/png'"`
	Rnd                 float64    `gorm:"type:double;not null;index:idx_cache_random,priority:2"`
	CreatedAt           time.Time
}

// Tag represents a normalized tag.
type Tag struct {
	ID             PrimaryKey `gorm:"primaryKey;autoIncrement"`
	NameNormalized string     `gorm:"type:varchar(128);not null;uniqueIndex"`
	DisplayName    string     `gorm:"type:varchar(128);not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ImageTag links an image to a tag, recording which step produced the association.
type ImageTag struct {
	ImageID      PrimaryKey `gorm:"primaryKey"`
	TagID        PrimaryKey `gorm:"primaryKey"`
	SourceStepID PrimaryKey `gorm:"not null;index"`
	Score        *float64   `gorm:"type:double"`
	CreatedAt    time.Time
}

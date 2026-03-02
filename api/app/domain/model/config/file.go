package config

import "time"

type ImageFileProviderConfig struct {
	Criteria Criteria
	SrcPath  string
}

func (ImageFileProviderConfig) providerConfigTag() {}

type FileCriteria struct {
	Path          []string
	ExifTimeRange []TimeRange
}

type TimeRange struct {
	From time.Time
	To   time.Time
	Last time.Duration
}

type Criteria struct {
	Include FileCriteria
	Exclude FileCriteria
}

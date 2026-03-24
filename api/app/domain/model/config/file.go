package config

import "time"

type ImageFileProviderConfig struct {
	Criteria Criteria
	SrcPath  string
	Hooks    FileHooks
}

func (ImageFileProviderConfig) providerConfigTag() {}

// FileHooks holds optional shell commands invoked during catalog scan events.
type FileHooks struct {
	// OnNewFile is executed asynchronously when a previously unseen file is registered.
	// The placeholder {file} is replaced with the absolute path of the new file.
	// Empty string means no hook.
	OnNewFile string
}

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

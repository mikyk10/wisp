package model

import (
	"fmt"
)

var (
	// Embeded app version (ldflags)
	AppVersion string

	// Embeded commit hash (ldflags)
	CommitHash string

	// Embeded app build-time (ldflags)
	BuildTime string
)

func init() {
	if AppVersion == "" {
		AppVersion = "develop"
	}

	if CommitHash == "" {
		CommitHash = "none"
	}

	if BuildTime == "" {
		BuildTime = "ad-hoc"
	}
}

func AppVersionString() string {
	return fmt.Sprintf("%s(%s,%s)", AppVersion, CommitHash, BuildTime)
}

func AppShortVersionString() string {
	return AppVersion
}

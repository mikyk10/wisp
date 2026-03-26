package catalog

import (
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/model/config"
)

func makeProvider(key string, cron string) *config.AssociatedImageProviders {
	return &config.AssociatedImageProviders{
		ProviderConfig: &config.ImageProviderConfig{
			Key:    key,
			Config: config.ImageColorbarProviderConfig{},
		},
		TimeRange: config.CronConfig{Cron: cron},
	}
}

func TestCronFilter_MatchesWithNonZeroSeconds(t *testing.T) {
	// 01:28:55 — minute 28 is within "24-30", second is non-zero.
	// This was a real bug: gronx checks seconds even for 5-field cron,
	// so without Truncate the filter returned no matches.
	now := time.Date(2026, 3, 26, 1, 28, 55, 0, time.UTC)

	providers := []*config.AssociatedImageProviders{
		makeProvider("morning", "24-30 1 * * *"),
		makeProvider("fallback", ""),
	}

	got := cronFilter(now, providers)
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].ProviderConfig.Key != "morning" {
		t.Errorf("expected key %q, got %q", "morning", got[0].ProviderConfig.Key)
	}
}

func TestCronFilter_NoMatchOutsideWindow(t *testing.T) {
	now := time.Date(2026, 3, 26, 2, 0, 30, 0, time.UTC)

	providers := []*config.AssociatedImageProviders{
		makeProvider("morning", "24-30 1 * * *"),
	}

	got := cronFilter(now, providers)
	if len(got) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(got))
	}
}

func TestCronFilter_SkipsEmptyCron(t *testing.T) {
	now := time.Date(2026, 3, 26, 1, 28, 0, 0, time.UTC)

	providers := []*config.AssociatedImageProviders{
		makeProvider("always", ""),
	}

	got := cronFilter(now, providers)
	if len(got) != 0 {
		t.Fatalf("expected 0 (empty cron excluded from cronFilter), got %d", len(got))
	}
}

func TestNonCronFilter_ReturnsOnlyEmptyCron(t *testing.T) {
	providers := []*config.AssociatedImageProviders{
		makeProvider("scheduled", "0 9 * * *"),
		makeProvider("fallback", ""),
	}

	got := nonCronFilter(providers)
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].ProviderConfig.Key != "fallback" {
		t.Errorf("expected key %q, got %q", "fallback", got[0].ProviderConfig.Key)
	}
}

func TestCronFilter_DoesNotMutateInput(t *testing.T) {
	now := time.Date(2026, 3, 26, 1, 28, 0, 0, time.UTC)

	providers := []*config.AssociatedImageProviders{
		makeProvider("hit", "24-30 1 * * *"),
		makeProvider("miss", "0 9 * * *"),
		makeProvider("fallback", ""),
	}
	origLen := len(providers)

	cronFilter(now, providers)

	if len(providers) != origLen {
		t.Errorf("input slice was mutated: len was %d, now %d", origLen, len(providers))
	}
}

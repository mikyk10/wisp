package wake_test

import (
	"testing"
	"time"

	"github.com/mikyk10/wisp/app/domain/display/wake"
	"github.com/stretchr/testify/assert"
)

func at(hhmm string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", "2026-08-04 "+hhmm, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func TestPlan_SleepsUntilTheNextScheduledWake(t *testing.T) {
	// Awake every ten minutes between 06:00 and 22:59, and not at all
	// overnight.
	daytime := wake.Plan{Schedule: []string{"*/10 6-22 * * *"}, Fallback: 300}

	tests := []struct {
		name string
		now  string
		want int
	}{
		{"mid-morning, next tick is ten minutes off", "09:00:00", 600},
		{"part-way through, only the remainder is left", "09:05:00", 300},
		{"a second past a tick still waits for the next", "09:00:01", 599},
		// The last wake of the day is 22:50; from there the schedule does not
		// call again until 06:00, so the panel simply stays away all night.
		{"last wake of the evening sleeps until morning", "22:50:00", 7*60*60 + 10*60},
		{"the small hours sleep until morning", "03:00:00", 3 * 60 * 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, daytime.SleepSeconds(at(tt.now)))
		})
	}
}

// TestPlan_TakesTheEarliestOfSeveralSchedules covers the reason a display may
// carry more than one expression: a dense evening laid over a sparse day is far
// easier to read as two rules than as one.
func TestPlan_TakesTheEarliestOfSeveralSchedules(t *testing.T) {
	plan := wake.Plan{
		Schedule: []string{
			"0 * * * *",       // hourly, all day
			"*/5 17-20 * * *", // and every five minutes through the evening
		},
		Fallback: 300,
	}

	// Mid-morning only the hourly rule applies.
	assert.Equal(t, 30*60, plan.SleepSeconds(at("09:30:00")))
	// In the evening the denser rule wins.
	assert.Equal(t, 5*60, plan.SleepSeconds(at("18:30:00")))
}

func TestPlan_FallsBackWithoutASchedule(t *testing.T) {
	plan := wake.Plan{Fallback: 900}

	assert.Equal(t, 900, plan.SleepSeconds(at("09:00:00")))
}

// TestPlan_UnparsableExpressionDoesNotSuppressTheRest: configuration is
// validated at load time, so a bad expression here means something changed
// underneath. The panel should still follow the rules that do make sense.
func TestPlan_UnparsableExpressionDoesNotSuppressTheRest(t *testing.T) {
	plan := wake.Plan{
		Schedule: []string{"not a cron expression", "*/10 * * * *"},
		Fallback: 300,
	}

	assert.Equal(t, 600, plan.SleepSeconds(at("09:00:00")))
}

func TestPlan_StaysWithinBounds(t *testing.T) {
	t.Run("a distant schedule is capped rather than obeyed", func(t *testing.T) {
		// 29 February: the next match is years away. Honour it, but keep the
		// panel checking in daily so a corrected schedule can reach it.
		plan := wake.Plan{Schedule: []string{"0 0 29 2 *"}, Fallback: 600}
		assert.Equal(t, wake.MaxSeconds, plan.SleepSeconds(at("09:00:00")))
	})

	t.Run("an eager fallback is held to the floor", func(t *testing.T) {
		plan := wake.Plan{Fallback: 1}
		assert.Equal(t, wake.MinSeconds, plan.SleepSeconds(at("09:00:00")))
	})

	t.Run("a distant fallback is held to the ceiling", func(t *testing.T) {
		plan := wake.Plan{Fallback: 40 * 60 * 60}
		assert.Equal(t, wake.MaxSeconds, plan.SleepSeconds(at("09:00:00")))
	})

	t.Run("a minute-by-minute schedule cannot outrun the grace window", func(t *testing.T) {
		// Every minute is inside Grace by definition, so the ticks in the
		// window are absorbed and the panel lands just beyond it. A schedule
		// this dense is misconfiguration for e-paper either way; the point is
		// that it degrades to a few minutes, not to a busy loop.
		plan := wake.Plan{Schedule: []string{"* * * * *"}, Fallback: 300}
		assert.Equal(t, 210, plan.SleepSeconds(at("09:00:30")))
	})
}

// TestPlan_EarlyArrivalCountsAsTheScheduledWake is the production story that
// created Grace. A panel times its sleep on an RC oscillator: told at 17:00 to
// come back at 07:00, it arrived at 06:59, was told to sleep the minimum, and
// refreshed again at 07:01 — two full e-paper refreshes, the first picture
// thrown away after two minutes. Arriving within Grace of a scheduled moment
// must count as attending it.
func TestPlan_EarlyArrivalCountsAsTheScheduledWake(t *testing.T) {
	plan := wake.Plan{Schedule: []string{"0 7 * * *", "0 17 * * *"}, Fallback: 300}

	t.Run("a minute early is this wake, and sleeps to the next one", func(t *testing.T) {
		// 06:59 → 17:00, not 06:59 → 07:00 → 07:01.
		assert.Equal(t, 10*60*60+60, plan.SleepSeconds(at("06:59:00")))
	})

	t.Run("well ahead of the window it is an unrelated wake", func(t *testing.T) {
		// A manual wake at 06:56 is not drift; the panel is still sent back
		// for the 07:00 it has not attended.
		assert.Equal(t, 4*60, plan.SleepSeconds(at("06:56:00")))
	})

	t.Run("only the imminent tick is absorbed, never the one after", func(t *testing.T) {
		dense := wake.Plan{Schedule: []string{"*/10 * * * *"}, Fallback: 300}
		// 08:59 attends the 09:00 tick; the 09:10 one is honoured in full.
		assert.Equal(t, 11*60, dense.SleepSeconds(at("08:59:00")))
	})
}

// TestPlan_AnswersSanelyAtEveryMoment sweeps clock time instead of picking
// examples. The grace window moves the search start past "now", and the
// question this answers is whether any alignment of arrival time and schedule
// — just before a tick, dead on it, just past it, mid-gap, overnight — can
// produce a sleep outside [MinSeconds, MaxSeconds] or a panic. The clamp
// bounds what SleepSeconds returns, so the real assertion is that every
// moment reaches the clamp with a positive, finite answer rather than falling
// somewhere no test has looked.
func TestPlan_AnswersSanelyAtEveryMoment(t *testing.T) {
	plans := map[string]wake.Plan{
		"production pair":     {Schedule: []string{"0 7 * * *", "0 17 * * *"}, Fallback: 300},
		"dense day":           {Schedule: []string{"*/10 6-22 * * *"}, Fallback: 300},
		"denser than grace":   {Schedule: []string{"* * * * *"}, Fallback: 300},
		"never matches again": {Schedule: []string{"0 0 29 2 *"}, Fallback: 600},
	}

	// A prime step keeps the samples sliding across minute boundaries instead
	// of hitting the same second of every minute.
	start := at("00:00:00")
	for name, plan := range plans {
		t.Run(name, func(t *testing.T) {
			for offset := 0; offset < 48*60*60; offset += 977 {
				now := start.Add(time.Duration(offset) * time.Second)
				got := plan.SleepSeconds(now)
				if got < wake.MinSeconds || got > wake.MaxSeconds {
					t.Fatalf("SleepSeconds(%s) = %d, outside [%d, %d]",
						now.Format("15:04:05"), got, wake.MinSeconds, wake.MaxSeconds)
				}
			}
		})
	}
}

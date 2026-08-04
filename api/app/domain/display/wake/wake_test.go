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
		{"part-way through, only the remainder is left", "09:07:30", 150},
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

	t.Run("a minute-by-minute schedule is held to the floor", func(t *testing.T) {
		plan := wake.Plan{Schedule: []string{"* * * * *"}, Fallback: 300}
		assert.Equal(t, wake.MinSeconds, plan.SleepSeconds(at("09:00:30")))
	})
}

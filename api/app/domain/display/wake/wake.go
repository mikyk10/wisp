// Package wake works out when a panel should next come back for a picture.
//
// A panel spends almost all of its life in deep sleep and only learns when to
// return from the X-Sleep-Seconds header on the response it is already
// fetching. That makes the sleep interval a scheduling primitive rather than a
// setting: the server decides, every time, when it wants to see the device
// again.
//
// A schedule is expressed as the moments the panel should be awake, not as an
// interval between wakes, and the sleep is simply the distance to the next one.
// Nothing has to reason about intervals or window boundaries, and because the
// moments are absolute the panels do not drift: one that wakes at 06:00 today
// wakes at 06:00 tomorrow, however long it slept in between.
package wake

import (
	"time"

	"github.com/adhocore/gronx"
)

const (
	// MinSeconds keeps a misconfigured schedule from turning a panel into a
	// busy loop. Refreshing e-paper takes seconds of its own and wears the
	// display, so nothing is gained by going faster.
	MinSeconds = 60

	// MaxSeconds bounds how long a panel may be told to stay away. A schedule
	// that matches only rare dates (29 February, say) would otherwise put one
	// to sleep for years, and a device asleep is a device that cannot be told
	// anything new.
	MaxSeconds = 24 * 60 * 60

	// Grace is how close to a scheduled moment a panel may arrive early and
	// still count as that moment's wake.
	//
	// A panel times its own sleep on an RC oscillator, and over the hours a
	// schedule puts between wakes that clock is honestly wrong — a tenth of a
	// percent across fourteen hours is most of a minute, either direction. A
	// panel that arrives at 06:59 for a 07:00 schedule is not an unrelated
	// visitor to be sent back for the real thing: without this, it was told
	// to sleep the minimum and refresh again at 07:01 — two full e-paper
	// refreshes back to back, and the first picture thrown away after living
	// for two minutes.
	//
	// Three minutes covers several times the drift observed in practice while
	// staying well under any sensible gap between scheduled moments. A tick
	// closer to the previous one than this is absorbed by design — arriving
	// early for it IS attending it — but only ever the one tick: the next
	// wake is computed from the arrival, not from the absorbed moment.
	Grace = 3 * time.Minute
)

// Plan is how a single display decides when to come back.
type Plan struct {
	// Schedule lists cron expressions naming the moments this panel should be
	// awake. Empty means there is no schedule and Fallback is used as a plain
	// interval, which is how every display behaved before schedules existed.
	Schedule []string

	// Fallback is the interval in seconds used when Schedule is empty or when
	// nothing in it can be read. A schedule whose next match is simply a long
	// way off is not a fallback case: it is honoured and capped at MaxSeconds,
	// so the panel keeps checking in daily and a corrected schedule reaches it
	// within a day.
	Fallback int
}

// SleepSeconds returns how long the panel should sleep, given the time it
// asked. The result is always within [MinSeconds, MaxSeconds].
func (p Plan) SleepSeconds(now time.Time) int {
	next, ok := p.nextWake(now)
	if !ok {
		return clamp(p.Fallback)
	}

	// Round up, so a panel never wakes a moment early and finds the schedule
	// has not come round yet.
	seconds := int((next.Sub(now) + time.Second - 1) / time.Second)
	return clamp(seconds)
}

// nextWake returns the earliest moment any expression in the schedule calls
// for. An expression that cannot be parsed is skipped rather than allowed to
// suppress the rest: configuration is validated at load time, so reaching here
// with a bad one means something changed underneath, and the remaining
// expressions still describe what the panel is for.
func (p Plan) nextWake(now time.Time) (time.Time, bool) {
	// Anything scheduled within Grace of now is this wake, already happening —
	// see Grace. Looking for the next moment starts beyond it.
	horizon := now.Add(Grace)

	var earliest time.Time
	for _, expr := range p.Schedule {
		// gronx works to a one-minute resolution, so ask from the start of the
		// current minute and exclude it: asking from a moment part-way through
		// would otherwise skip a match in the minute we are already in.
		next, err := gronx.NextTickAfter(expr, horizon.Truncate(time.Minute), false)
		if err != nil {
			continue
		}
		if earliest.IsZero() || next.Before(earliest) {
			earliest = next
		}
	}

	if earliest.IsZero() || !earliest.After(now) {
		return time.Time{}, false
	}
	return earliest, true
}

func clamp(seconds int) int {
	if seconds < MinSeconds {
		return MinSeconds
	}
	if seconds > MaxSeconds {
		return MaxSeconds
	}
	return seconds
}

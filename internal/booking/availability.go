// Package booking holds the availability algorithm: pure functions that
// compute bookable time slots from workday hours, personal time, and
// existing appointments. No DB, no clocks — the handler injects everything.
package booking

import (
	"sort"
	"time"

	"github.com/mossandoval/datil-api/internal/model"
)

// TimeSlot represents an open booking window: [Start, End).
type TimeSlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// interval is a half-open [start, end) range used internally during subtraction.
type interval struct {
	start time.Time
	end   time.Time
}

// ComputeSlots returns the bookable start times on the given date for a
// service-bundle of totalDurationMin minutes, walked at slotStepMin steps.
//
// It treats workday.Hours as the base availability for the day, then
// subtracts personal-time blocks that overlap the date and any existing
// appointments. The function is intentionally pure: callers must load the
// inputs from the database and pass `date` as the local-midnight anchor.
//
// Returns an empty slice (not nil) when no slots fit.
func ComputeSlots(
	workday model.Workday,
	personalTime []model.PersonalTime,
	appointments []model.Appointment,
	totalDurationMin int,
	date time.Time,
	slotStepMin int,
) []TimeSlot {
	if totalDurationMin <= 0 || slotStepMin <= 0 || !workday.IsEnabled {
		return []TimeSlot{}
	}

	loc := date.Location()
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.Add(24 * time.Hour)

	free := make([]interval, 0, len(workday.Hours))
	for _, h := range workday.Hours {
		start, ok := parseClock(h.StartTime, dayStart)
		if !ok {
			continue
		}
		end, ok := parseClock(h.EndTime, dayStart)
		if !ok {
			continue
		}
		if !end.After(start) {
			continue
		}
		free = append(free, interval{start: start, end: end})
	}
	if len(free) == 0 {
		return []TimeSlot{}
	}
	free = mergeIntervals(free)

	blocked := make([]interval, 0)
	for _, pt := range personalTime {
		if iv, ok := personalTimeToInterval(pt, dayStart, dayEnd); ok {
			blocked = append(blocked, iv)
		}
	}
	for _, a := range appointments {
		if a.EndTime.After(dayStart) && a.StartTime.Before(dayEnd) {
			blocked = append(blocked, interval{start: a.StartTime, end: a.EndTime})
		}
	}
	free = subtractIntervals(free, blocked)

	step := time.Duration(slotStepMin) * time.Minute
	duration := time.Duration(totalDurationMin) * time.Minute

	out := make([]TimeSlot, 0)
	for _, iv := range free {
		// Walk the window in `step` increments; emit every start whose
		// matching end fits inside the window.
		for cur := iv.start; !cur.Add(duration).After(iv.end); cur = cur.Add(step) {
			out = append(out, TimeSlot{Start: cur, End: cur.Add(duration)})
		}
	}
	return out
}

// parseClock turns a "HH:MM" or "HH:MM:SS" wall-clock string into a
// time.Time anchored to dayStart's date in dayStart's location.
func parseClock(s string, dayStart time.Time) (time.Time, bool) {
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(
				dayStart.Year(), dayStart.Month(), dayStart.Day(),
				t.Hour(), t.Minute(), t.Second(), 0, dayStart.Location(),
			), true
		}
	}
	return time.Time{}, false
}

// personalTimeToInterval projects a PersonalTime row onto the target day.
// Returns the blocked interval (clipped to the day) or ok=false when the
// row doesn't touch this day.
func personalTimeToInterval(pt model.PersonalTime, dayStart, dayEnd time.Time) (interval, bool) {
	startDate, err := time.ParseInLocation("2006-01-02", pt.StartDate, dayStart.Location())
	if err != nil {
		return interval{}, false
	}
	endDate, err := time.ParseInLocation("2006-01-02", pt.EndDate, dayStart.Location())
	if err != nil {
		return interval{}, false
	}

	dayDateOnly := time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day(), 0, 0, 0, 0, dayStart.Location())
	if dayDateOnly.Before(startDate) || dayDateOnly.After(endDate) {
		return interval{}, false
	}

	// Hourly block — only applies on the single day where start_date == end_date.
	if pt.StartTime != nil && pt.EndTime != nil && startDate.Equal(endDate) {
		start, ok := parseClock(*pt.StartTime, dayStart)
		if !ok {
			return interval{}, false
		}
		end, ok := parseClock(*pt.EndTime, dayStart)
		if !ok {
			return interval{}, false
		}
		if !end.After(start) {
			return interval{}, false
		}
		return interval{start: start, end: end}, true
	}

	// All-day or multi-day block — blocks the entire target day.
	return interval{start: dayStart, end: dayEnd}, true
}

// mergeIntervals returns intervals sorted by start with overlapping or
// touching ranges coalesced. Pure helper used before subtraction.
func mergeIntervals(in []interval) []interval {
	if len(in) <= 1 {
		return in
	}
	sort.Slice(in, func(i, j int) bool { return in[i].start.Before(in[j].start) })
	out := []interval{in[0]}
	for _, cur := range in[1:] {
		last := &out[len(out)-1]
		if !cur.start.After(last.end) {
			if cur.end.After(last.end) {
				last.end = cur.end
			}
			continue
		}
		out = append(out, cur)
	}
	return out
}

// subtractIntervals removes blocked from free and returns whatever's left.
// Inputs are not required to be sorted; result is sorted by start.
func subtractIntervals(free, blocked []interval) []interval {
	if len(blocked) == 0 {
		return mergeIntervals(free)
	}
	blocked = mergeIntervals(blocked)
	cur := mergeIntervals(free)
	for _, b := range blocked {
		next := make([]interval, 0, len(cur))
		for _, f := range cur {
			// No overlap: keep f as-is.
			if !b.start.Before(f.end) || !b.end.After(f.start) {
				next = append(next, f)
				continue
			}
			// Left fragment.
			if b.start.After(f.start) {
				next = append(next, interval{start: f.start, end: b.start})
			}
			// Right fragment.
			if b.end.Before(f.end) {
				next = append(next, interval{start: b.end, end: f.end})
			}
		}
		cur = next
	}
	return cur
}

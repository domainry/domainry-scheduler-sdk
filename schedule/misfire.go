package schedule

import (
	"strings"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

// MisfireResolution turns one durable cursor into a bounded set of execution
// windows. CursorAfter is non-zero only when older windows are intentionally
// skipped; the run store must persist it atomically with the last claim, or by
// itself when Windows is empty.
type MisfireResolution struct {
	Windows     []time.Time
	CursorAfter time.Time
	Disable     bool
}

// ResolveMisfire applies explicit policies without inspecting process state.
// An empty policy preserves the legacy one-window behavior. A window becomes
// missed only after MisfireGrace, allowing ordinary poll latency.
func ResolveMisfire(value schedulersdk.Schedule, policy schedulersdk.Policy, cursor, now time.Time, limit int) MisfireResolution {
	cursor, now = cursor.UTC(), now.UTC()
	if cursor.IsZero() || cursor.After(now) || limit <= 0 {
		return MisfireResolution{}
	}
	misfire := strings.TrimSpace(policy.Misfire)
	if misfire == "" {
		return MisfireResolution{Windows: []time.Time{cursor}}
	}
	grace := policy.MisfireGrace
	if grace <= 0 {
		grace = time.Minute
	}
	if !now.After(cursor.Add(grace)) {
		return MisfireResolution{Windows: []time.Time{cursor}}
	}
	future := NextAfter(value, cursor, now)
	switch misfire {
	case schedulersdk.ScheduledPlanMisfireSkip:
		return MisfireResolution{CursorAfter: future, Disable: future.IsZero()}
	case schedulersdk.ScheduledPlanMisfireCatchOne:
		return MisfireResolution{Windows: []time.Time{cursor}, CursorAfter: future}
	case schedulersdk.ScheduledPlanMisfireCatchMany:
		count := policy.MaxCatchupWindows
		if count <= 0 {
			count = 1
		}
		if count > limit {
			count = limit
		}
		windows := make([]time.Time, 0, count)
		current := cursor
		for len(windows) < count && !current.IsZero() && !current.After(now) {
			windows = append(windows, current)
			current = nextFromCursor(value, current)
		}
		resolution := MisfireResolution{Windows: windows}
		if !current.IsZero() && !current.After(now) {
			resolution.CursorAfter = future
		}
		return resolution
	default:
		return MisfireResolution{Windows: []time.Time{cursor}}
	}
}

// NextAfter returns the first aligned occurrence strictly after now. Interval
// schedules retain the phase anchored by the durable cursor.
func NextAfter(value schedulersdk.Schedule, cursor, now time.Time) time.Time {
	if Type(Data(value)) == "once" {
		return time.Time{}
	}
	if Type(Data(value)) == "interval" {
		seconds := IntervalSeconds(Data(value))
		if seconds <= 0 {
			return time.Time{}
		}
		step := time.Duration(seconds) * time.Second
		if cursor.After(now) {
			return cursor.UTC()
		}
		steps := now.Sub(cursor)/step + 1
		return cursor.Add(steps * step).UTC()
	}
	return NextSchedule(value, now)
}

func nextFromCursor(value schedulersdk.Schedule, cursor time.Time) time.Time {
	if Type(Data(value)) == "once" {
		return time.Time{}
	}
	return NextSchedule(value, cursor)
}

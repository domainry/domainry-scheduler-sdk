package schedule

import (
	"testing"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

func TestResolveMisfireMakesSkipCatchOneAndBoundedPoliciesExplicit(t *testing.T) {
	cursor := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	now := cursor.Add(5*time.Minute + 30*time.Second)
	value := schedulersdk.Schedule{Type: "interval", IntervalSeconds: 60, Timezone: "UTC"}
	base := schedulersdk.Policy{MisfireGrace: 30 * time.Second}

	skip := base
	skip.Misfire = schedulersdk.ScheduledPlanMisfireSkip
	resolved := ResolveMisfire(value, skip, cursor, now, 10)
	if len(resolved.Windows) != 0 || !resolved.CursorAfter.Equal(cursor.Add(6*time.Minute)) || resolved.Disable {
		t.Fatalf("skip=%+v", resolved)
	}

	one := base
	one.Misfire = schedulersdk.ScheduledPlanMisfireCatchOne
	resolved = ResolveMisfire(value, one, cursor, now, 10)
	if len(resolved.Windows) != 1 || !resolved.Windows[0].Equal(cursor) || !resolved.CursorAfter.Equal(cursor.Add(6*time.Minute)) {
		t.Fatalf("catch one=%+v", resolved)
	}

	bounded := base
	bounded.Misfire = schedulersdk.ScheduledPlanMisfireCatchMany
	bounded.MaxCatchupWindows = 3
	resolved = ResolveMisfire(value, bounded, cursor, now, 10)
	if len(resolved.Windows) != 3 || !resolved.Windows[0].Equal(cursor) || !resolved.Windows[2].Equal(cursor.Add(2*time.Minute)) || !resolved.CursorAfter.Equal(cursor.Add(6*time.Minute)) {
		t.Fatalf("bounded=%+v", resolved)
	}
}

func TestResolveMisfireHonorsGraceAndOneTimeTerminalSkip(t *testing.T) {
	cursor := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	once := schedulersdk.Schedule{Type: "once", Expression: cursor.Format(time.RFC3339Nano), Timezone: "UTC"}
	policy := schedulersdk.Policy{Misfire: schedulersdk.ScheduledPlanMisfireSkip, MisfireGrace: time.Minute}

	withinGrace := ResolveMisfire(once, policy, cursor, cursor.Add(30*time.Second), 1)
	if len(withinGrace.Windows) != 1 || withinGrace.Disable {
		t.Fatalf("within grace=%+v", withinGrace)
	}
	missed := ResolveMisfire(once, policy, cursor, cursor.Add(2*time.Minute), 1)
	if len(missed.Windows) != 0 || !missed.CursorAfter.IsZero() || !missed.Disable {
		t.Fatalf("missed once=%+v", missed)
	}
}

func TestNextAfterKeepsIntervalPhase(t *testing.T) {
	cursor := time.Date(2026, 9, 14, 9, 0, 15, 0, time.UTC)
	now := time.Date(2026, 9, 14, 9, 5, 40, 0, time.UTC)
	got := NextAfter(schedulersdk.Schedule{Type: "interval", IntervalSeconds: 60}, cursor, now)
	want := time.Date(2026, 9, 14, 9, 6, 15, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("next=%s want=%s", got, want)
	}
}

func TestResolveMisfireFastForwardsNonWorkingOccurrencesWithoutDispatch(t *testing.T) {
	calendar := weekdayCalendarSnapshot()
	calendar.Timezone = "UTC"
	value := schedulersdk.Schedule{Type: "daily_at", TimeOfDay: "09:00", Timezone: "UTC", BusinessCalendar: calendar, NonWorkingDayPolicy: schedulersdk.NonWorkingDaySkip}
	saturday := time.Date(2026, 9, 19, 9, 0, 0, 0, time.UTC)
	resolved := ResolveMisfire(value, schedulersdk.Policy{Misfire: schedulersdk.ScheduledPlanMisfireCatchMany, MaxCatchupWindows: 5}, saturday, time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC), 5)
	wantMonday := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	if len(resolved.Windows) != 0 || !resolved.CursorAfter.Equal(wantMonday) {
		t.Fatalf("weekend resolution=%+v", resolved)
	}

	friday := time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)
	resolved = ResolveMisfire(value, schedulersdk.Policy{Misfire: schedulersdk.ScheduledPlanMisfireCatchMany, MaxCatchupWindows: 5}, friday, time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC), 5)
	if len(resolved.Windows) != 1 || !resolved.Windows[0].Equal(friday) || !resolved.CursorAfter.Equal(saturday) {
		t.Fatalf("bounded resolution crossed a closed date: %+v", resolved)
	}
}

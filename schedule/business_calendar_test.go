package schedule

import (
	"testing"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

func TestResolvedBusinessCalendarRollsForwardWithRevisionedSnapshot(t *testing.T) {
	value := schedulersdk.Schedule{
		Type: "daily_at", TimeOfDay: "09:00", Timezone: "Asia/Shanghai",
		BusinessCalendar:    weekdayCalendarSnapshot(),
		NonWorkingDayPolicy: schedulersdk.NonWorkingDayRollForward,
	}
	if err := Validate(value); err != nil {
		t.Fatal(err)
	}
	after := time.Date(2026, 9, 18, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60)) // Friday
	next := NextSchedule(value, after)
	local, _ := time.LoadLocation("Asia/Shanghai")
	if got := next.In(local).Format("2006-01-02 15:04"); got != "2026-09-21 09:00" {
		t.Fatalf("rolled occurrence=%s", got)
	}
}

func TestResolvedBusinessCalendarSkipPreviewOmitsClosedDates(t *testing.T) {
	value := schedulersdk.Schedule{
		Type: "daily_at", TimeOfDay: "09:00", Timezone: "Asia/Shanghai",
		BusinessCalendar:    weekdayCalendarSnapshot(),
		NonWorkingDayPolicy: schedulersdk.NonWorkingDaySkip,
	}
	after := time.Date(2026, 9, 18, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	values, err := PreviewSchedule(t.Context(), value, after, 2)
	if err != nil {
		t.Fatal(err)
	}
	local, _ := time.LoadLocation("Asia/Shanghai")
	if got := values[0].In(local).Format("2006-01-02 15:04"); got != "2026-09-21 09:00" {
		t.Fatalf("first occurrence=%s", got)
	}
	if got := values[1].In(local).Format("2006-01-02 15:04"); got != "2026-09-22 09:00" {
		t.Fatalf("second occurrence=%s", got)
	}
}

func TestResolvedBusinessCalendarRejectsMutableOrAmbiguousProtocol(t *testing.T) {
	value := schedulersdk.Schedule{Type: "interval", IntervalSeconds: 60, Timezone: "Asia/Shanghai", BusinessCalendar: weekdayCalendarSnapshot(), NonWorkingDayPolicy: schedulersdk.NonWorkingDaySkip}
	if err := Validate(value); err == nil {
		t.Fatal("interval calendar unexpectedly accepted")
	}
	value = schedulersdk.Schedule{Type: "daily_at", TimeOfDay: "09:00", Timezone: "UTC", BusinessCalendar: weekdayCalendarSnapshot(), NonWorkingDayPolicy: schedulersdk.NonWorkingDaySkip}
	if err := Validate(value); err == nil {
		t.Fatal("calendar timezone mismatch unexpectedly accepted")
	}
	value.Timezone = "Asia/Shanghai"
	value.BusinessCalendar.Revision = ""
	if err := Validate(value); err == nil {
		t.Fatal("calendar without immutable revision unexpectedly accepted")
	}
}

func weekdayCalendarSnapshot() *schedulersdk.BusinessCalendarSnapshot {
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday"}
	weekly := make([]schedulersdk.BusinessCalendarWeeklySchedule, 0, len(days))
	for _, day := range days {
		weekly = append(weekly, schedulersdk.BusinessCalendarWeeklySchedule{Weekday: day, Intervals: []schedulersdk.BusinessCalendarTimeInterval{{Start: "09:00", End: "18:00"}}})
	}
	return &schedulersdk.BusinessCalendarSnapshot{Key: "cn_operations", Revision: "2026.09", Timezone: "Asia/Shanghai", WeeklyWorkingIntervals: weekly}
}

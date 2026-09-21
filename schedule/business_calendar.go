package schedule

import (
	"fmt"
	"sort"
	"strings"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

const maximumBusinessCalendarSearchDays = 3660

// ValidateBusinessCalendar validates the immutable protocol snapshot received
// from a host. It deliberately does not know where the calendar was authored.
func ValidateBusinessCalendar(value schedulersdk.BusinessCalendarSnapshot) error {
	if strings.TrimSpace(value.Key) == "" || strings.TrimSpace(value.Revision) == "" {
		return fmt.Errorf("scheduler business calendar key and revision are required")
	}
	location, err := time.LoadLocation(strings.TrimSpace(value.Timezone))
	if err != nil || location == time.Local {
		return fmt.Errorf("scheduler business calendar timezone is invalid")
	}
	if len(value.WeeklyWorkingIntervals) == 0 || len(value.WeeklyWorkingIntervals) > 7 {
		return fmt.Errorf("scheduler business calendar weekly working intervals are invalid")
	}
	weekdays := map[time.Weekday]bool{}
	for _, day := range value.WeeklyWorkingIntervals {
		weekday, valid := ParseWeekday(day.Weekday)
		if !valid || weekdays[weekday] || len(day.Intervals) == 0 || len(day.Intervals) > 8 {
			return fmt.Errorf("scheduler business calendar weekly working intervals are invalid")
		}
		weekdays[weekday] = true
		if err := validateBusinessCalendarIntervals(day.Intervals); err != nil {
			return err
		}
	}
	if len(value.Holidays) > 366 || len(value.DateExceptions) > 366 {
		return fmt.Errorf("scheduler business calendar date limits are exceeded")
	}
	dates := map[string]bool{}
	for _, date := range value.Holidays {
		if !validBusinessCalendarDate(date) || dates[date] {
			return fmt.Errorf("scheduler business calendar holiday is invalid or duplicated")
		}
		dates[date] = true
	}
	for _, exception := range value.DateExceptions {
		if !validBusinessCalendarDate(exception.Date) || dates[exception.Date] || len(exception.Intervals) > 8 {
			return fmt.Errorf("scheduler business calendar date exception is invalid or duplicated")
		}
		dates[exception.Date] = true
		if err := validateBusinessCalendarIntervals(exception.Intervals); err != nil {
			return err
		}
	}
	return nil
}

func validateBusinessCalendarIntervals(values []schedulersdk.BusinessCalendarTimeInterval) error {
	values = append([]schedulersdk.BusinessCalendarTimeInterval(nil), values...)
	sort.Slice(values, func(i, j int) bool { return values[i].Start < values[j].Start })
	previousEnd := -1
	for _, value := range values {
		start, startOK := parseBusinessCalendarClock(value.Start, false)
		end, endOK := parseBusinessCalendarClock(value.End, true)
		if !startOK || !endOK || start >= end || start < previousEnd {
			return fmt.Errorf("scheduler business calendar interval is invalid")
		}
		previousEnd = end
	}
	return nil
}

func parseBusinessCalendarClock(value string, allowEndOfDay bool) (int, bool) {
	value = strings.TrimSpace(value)
	if allowEndOfDay && value == "24:00" {
		return 24 * 60, true
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil || parsed.Format("15:04") != value {
		return 0, false
	}
	return parsed.Hour()*60 + parsed.Minute(), true
}

func validBusinessCalendarDate(value string) bool {
	value = strings.TrimSpace(value)
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

// IsNonWorkingOccurrence classifies the occurrence by local calendar date.
// Working intervals define whether the date is open; they do not move the
// recurrence's wall-clock time within an otherwise working date.
func IsNonWorkingOccurrence(value schedulersdk.Schedule, occurrence time.Time) bool {
	if value.BusinessCalendar == nil || occurrence.IsZero() {
		return false
	}
	calendar := *value.BusinessCalendar
	location, err := time.LoadLocation(calendar.Timezone)
	if err != nil {
		return true
	}
	local := occurrence.In(location)
	date := local.Format("2006-01-02")
	for _, exception := range calendar.DateExceptions {
		if strings.TrimSpace(exception.Date) == date {
			return len(exception.Intervals) == 0
		}
	}
	for _, holiday := range calendar.Holidays {
		if strings.TrimSpace(holiday) == date {
			return true
		}
	}
	for _, weekly := range calendar.WeeklyWorkingIntervals {
		weekday, valid := ParseWeekday(weekly.Weekday)
		if valid && weekday == local.Weekday() {
			return len(weekly.Intervals) == 0
		}
	}
	return true
}

func rollForwardBusinessOccurrence(value schedulersdk.Schedule, occurrence time.Time) time.Time {
	if value.BusinessCalendar == nil || !IsNonWorkingOccurrence(value, occurrence) {
		return occurrence.UTC()
	}
	location, err := time.LoadLocation(value.BusinessCalendar.Timezone)
	if err != nil {
		return time.Time{}
	}
	local := occurrence.In(location)
	for offset := 1; offset <= maximumBusinessCalendarSearchDays; offset++ {
		day := local.AddDate(0, 0, offset)
		candidate := time.Date(day.Year(), day.Month(), day.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), location)
		if !IsNonWorkingOccurrence(value, candidate) {
			return candidate.UTC()
		}
	}
	return time.Time{}
}

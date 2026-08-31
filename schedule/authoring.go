package schedule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ValidationError is the deployment-neutral Scheduler authoring error. Hosts
// may map it to their transport-specific error envelope without duplicating
// Scheduler validation rules.
type ValidationError struct {
	Code   string
	Params map[string]string
}

func (e *ValidationError) Error() string { return e.Code }

func (e *ValidationError) ErrorCode() string { return strings.TrimSpace(e.Code) }

func (e *ValidationError) ErrorParams() map[string]string {
	if len(e.Params) == 0 {
		return nil
	}
	out := make(map[string]string, len(e.Params))
	for key, value := range e.Params {
		out[key] = value
	}
	return out
}

func ValidationCode(err error) string {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return validation.ErrorCode()
	}
	return ""
}

func ValidationParams(err error) map[string]string {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return validation.ErrorParams()
	}
	return nil
}

// ValidateDefinitionData validates the authoring representation shared by
// Runtime and Control Plane before it is projected into a Scheduler Definition.
func ValidateDefinitionData(ctx context.Context, data map[string]any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	targetType := normalizedString(data["target_type"])
	switch targetType {
	case "workflow", "report_snapshot_refresh", "http":
	case "":
		return validationError("backend.scheduler.target_type_required", "field", "target_type")
	default:
		return validationError("backend.scheduler.target_type_unsupported", "field", "target_type", "allowed", "http,report_snapshot_refresh,workflow", "actual", targetType)
	}
	targetKey := textValue(data["target_key"])
	if targetKey == "" {
		return validationError("backend.scheduler.target_key_required", "field", "target_key")
	}
	if targetType == "workflow" && targetKey != "scheduled:*" && !strings.HasPrefix(targetKey, "scheduled:") {
		return validationError("backend.scheduler.workflow_target_invalid", "field", "target_key", "actual", targetKey)
	}
	if targetType == "report_snapshot_refresh" && strings.HasPrefix(targetKey, "scheduled:") {
		return validationError("backend.scheduler.report_target_invalid", "field", "target_key", "actual", targetKey)
	}
	if targetType == "http" {
		if connectionKey := textValue(data["connection_key"]); connectionKey == "" {
			return validationError("backend.scheduler.http_connection_required", "field", "connection_key")
		}
		if operation := textValue(data["operation"]); operation == "" {
			return validationError("backend.scheduler.http_operation_required", "field", "operation")
		}
		dispatchMode := normalizedString(data["dispatch_mode"])
		if dispatchMode != "" && dispatchMode != "direct" && dispatchMode != "runtime_callback" {
			return validationError("backend.scheduler.http_dispatch_mode_invalid", "field", "dispatch_mode", "allowed", "direct,runtime_callback", "actual", dispatchMode)
		}
	}
	maxAttempts := Int(data["max_attempts"], 1)
	if maxAttempts < 1 || maxAttempts > 100 {
		return validationError("backend.scheduler.max_attempts_invalid", "field", "max_attempts", "minimum", "1", "maximum", "100", "actual", fmt.Sprint(maxAttempts))
	}
	timeoutSeconds := Int(data["timeout_seconds"], 300)
	if timeoutSeconds < 1 || timeoutSeconds > 24*60*60 {
		return validationError("backend.scheduler.timeout_invalid", "field", "timeout_seconds", "minimum", "1", "maximum", fmt.Sprint(24*60*60), "actual", fmt.Sprint(timeoutSeconds))
	}
	if missedWindowPolicy(data) == "catch_up_bounded" {
		maxCatchupWindows := Int(data["max_catchup_windows"], 0)
		if maxCatchupWindows < 0 || maxCatchupWindows > 100 {
			return validationError("backend.scheduler.max_catchup_windows_invalid", "field", "max_catchup_windows", "minimum", "0", "maximum", "100", "actual", fmt.Sprint(maxCatchupWindows))
		}
	}
	rawMissedWindowPolicy := textValue(data["missed_window_policy"])
	if rawMissedWindowPolicy != "" && missedWindowPolicy(data) == "skip" && !strings.EqualFold(rawMissedWindowPolicy, "skip") {
		return validationError("backend.scheduler.missed_window_policy_invalid", "field", "missed_window_policy", "allowed", "catch_up_bounded,catch_up_one,skip", "actual", rawMissedWindowPolicy)
	}
	return ValidateData(ctx, data)
}

// ValidateData validates a leaf Scheduler schedule authoring payload.
func ValidateData(ctx context.Context, data map[string]any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if timezone := textValue(data["timezone"]); timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return validationError("backend.scheduler.timezone_invalid", "field", "timezone", "actual", timezone, "expected", "IANA timezone")
		}
	}
	rawScheduleType := normalizedString(data["schedule_type"])
	if rawScheduleType != "" {
		switch rawScheduleType {
		case "interval", "daily_at", "weekly_at", "monthly_at", "cron":
		default:
			return validationError("backend.scheduler.schedule_type_invalid", "field", "schedule_type", "allowed", "cron,daily_at,interval,monthly_at,weekly_at", "actual", rawScheduleType)
		}
	}
	switch Type(data) {
	case "interval":
		if IntervalSeconds(data) <= 0 {
			return validationError("backend.scheduler.interval_invalid", "field", "interval_seconds", "minimum", "1", "actual", fmt.Sprint(IntervalSeconds(data)))
		}
	case "daily_at":
		if _, _, _, ok := ParseClock(FirstNonEmpty(fmt.Sprint(data["time_of_day"]), fmt.Sprint(data["schedule_time"]), fmt.Sprint(data["daily_at"]), fmt.Sprint(data["schedule_expression"]))); !ok {
			return validationError("backend.scheduler.time_of_day_invalid", "field", "time_of_day")
		}
	case "weekly_at":
		if _, _, _, ok := ParseClock(FirstNonEmpty(fmt.Sprint(data["time_of_day"]), fmt.Sprint(data["schedule_time"]), fmt.Sprint(data["weekly_at"]), fmt.Sprint(data["schedule_expression"]))); !ok {
			return validationError("backend.scheduler.time_of_day_invalid", "field", "time_of_day")
		}
		if _, ok := ParseWeekday(FirstNonEmpty(fmt.Sprint(data["day_of_week"]), fmt.Sprint(data["weekday"]), fmt.Sprint(data["week_day"]))); !ok {
			return validationError("backend.scheduler.day_of_week_invalid", "field", "day_of_week")
		}
	case "monthly_at":
		if _, _, _, ok := ParseClock(FirstNonEmpty(fmt.Sprint(data["time_of_day"]), fmt.Sprint(data["schedule_time"]), fmt.Sprint(data["monthly_at"]), fmt.Sprint(data["schedule_expression"]))); !ok {
			return validationError("backend.scheduler.time_of_day_invalid", "field", "time_of_day")
		}
		day := Int(data["day_of_month"], Int(data["month_day"], 0))
		if day < 1 || day > 31 {
			return validationError("backend.scheduler.day_of_month_invalid", "field", "day_of_month", "minimum", "1", "maximum", "31", "actual", fmt.Sprint(day))
		}
	case "cron":
		if _, ok := Cron(data); !ok {
			return validationError("backend.scheduler.cron_invalid", "field", "schedule_expression")
		}
	case "legacy_hourly", "legacy_daily", "legacy_weekly", "legacy_monthly":
	}
	return nil
}

func missedWindowPolicy(data map[string]any) string {
	policy := normalizedString(data["missed_window_policy"])
	if policy == "catch_up_one" || policy == "catch_up_bounded" {
		return policy
	}
	return "skip"
}

func normalizedString(value any) string {
	return strings.ToLower(textValue(value))
}

func textValue(value any) string {
	result := strings.TrimSpace(fmt.Sprint(value))
	if result == "<nil>" {
		return ""
	}
	return result
}

func validationError(code string, values ...string) error {
	params := map[string]string{}
	for index := 0; index+1 < len(values); index += 2 {
		if key := strings.TrimSpace(values[index]); key != "" {
			params[key] = values[index+1]
		}
	}
	if len(params) == 0 {
		params = nil
	}
	return &ValidationError{Code: code, Params: params}
}

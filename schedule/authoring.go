package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"
)

const MaximumDefinitionPayloadBytes = 64 << 10

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
	if field := unsupportedDefinitionField(data); field != "" {
		return validationError("backend.scheduler.field_unsupported", "field", field)
	}
	targetType := normalizedString(data["target_type"])
	switch targetType {
	case "business_action", "workflow", "report_snapshot_refresh", "http":
	case "":
		return validationError("backend.scheduler.target_type_required", "field", "target_type")
	default:
		return validationError("backend.scheduler.target_type_unsupported", "field", "target_type", "allowed", "business_action,http,report_snapshot_refresh,workflow", "actual", targetType)
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
	if targetType == "business_action" {
		if textValue(data["target_object"]) == "" {
			return validationError("backend.scheduler.business_action_object_required", "field", "target_object")
		}
		if textValue(data["run_as_role"]) == "" {
			return validationError("backend.scheduler.business_action_role_required", "field", "run_as_role")
		}
		payload, payloadPresent, payloadValid := definitionPayload(data)
		if payloadPresent && !payloadValid {
			return validationError("backend.scheduler.payload_invalid", "field", "payload_json")
		}
		if payload != "" {
			var object map[string]any
			if len(payload) > MaximumDefinitionPayloadBytes || json.Unmarshal([]byte(payload), &object) != nil || object == nil {
				return validationError("backend.scheduler.payload_invalid", "field", "payload_json")
			}
		}
		if field := firstPresentField(data, "connection_key"); field != "" {
			return validationError("backend.scheduler.field_not_applicable", "field", field, "target_type", targetType)
		}
	}
	if targetType == "http" {
		if connectionKey := textValue(data["connection_key"]); connectionKey == "" {
			return validationError("backend.scheduler.http_connection_required", "field", "connection_key")
		}
		if field := firstPresentField(data, "target_object", "run_as_role"); field != "" {
			return validationError("backend.scheduler.field_not_applicable", "field", field, "target_type", targetType)
		}
		payload, payloadPresent, payloadValid := definitionPayload(data)
		if payloadPresent && !payloadValid || payload != "" && (len(payload) > MaximumDefinitionPayloadBytes || !json.Valid([]byte(payload))) {
			return validationError("backend.scheduler.payload_invalid", "field", "payload_json")
		}
	}
	if targetType == "workflow" || targetType == "report_snapshot_refresh" {
		if field := firstPresentField(data, "target_object", "run_as_role", "connection_key", "payload_json"); field != "" {
			return validationError("backend.scheduler.field_not_applicable", "field", field, "target_type", targetType)
		}
	}
	if _, present := data["max_attempts"]; !present {
		return validationError("backend.scheduler.max_attempts_required", "field", "max_attempts")
	}
	maxAttempts, maxAttemptsValid := strictAuthoringInteger(data["max_attempts"])
	if !maxAttemptsValid || maxAttempts < 1 || maxAttempts > 100 {
		return validationError("backend.scheduler.max_attempts_invalid", "field", "max_attempts", "minimum", "1", "maximum", "100", "actual", fmt.Sprint(maxAttempts))
	}
	if _, present := data["timeout_seconds"]; !present {
		return validationError("backend.scheduler.timeout_required", "field", "timeout_seconds")
	}
	timeoutSeconds, timeoutValid := strictAuthoringInteger(data["timeout_seconds"])
	if !timeoutValid || timeoutSeconds < 1 || timeoutSeconds > 24*60*60 {
		return validationError("backend.scheduler.timeout_invalid", "field", "timeout_seconds", "minimum", "1", "maximum", fmt.Sprint(24*60*60), "actual", fmt.Sprint(timeoutSeconds))
	}
	retryDelaySeconds, retryDelayValid := optionalAuthoringInteger(data, "retry_delay_seconds", 30)
	retryMaxDelaySeconds, retryMaxDelayValid := optionalAuthoringInteger(data, "retry_max_delay_seconds", 900)
	if !retryDelayValid || !retryMaxDelayValid || retryDelaySeconds < 0 || retryMaxDelaySeconds < 0 || (retryMaxDelaySeconds > 0 && retryMaxDelaySeconds < retryDelaySeconds) {
		return validationError("backend.scheduler.retry_policy_invalid", "field", "retry_delay_seconds", "retry_delay_seconds", fmt.Sprint(retryDelaySeconds), "retry_max_delay_seconds", fmt.Sprint(retryMaxDelaySeconds))
	}
	if missedWindowPolicy(data) == "catch_up_bounded" {
		maxCatchupWindows, maxCatchupValid := strictAuthoringInteger(data["max_catchup_windows"])
		if !maxCatchupValid || maxCatchupWindows < 1 || maxCatchupWindows > 100 {
			return validationError("backend.scheduler.max_catchup_windows_invalid", "field", "max_catchup_windows", "minimum", "1", "maximum", "100", "actual", fmt.Sprint(maxCatchupWindows))
		}
	} else if _, present := data["max_catchup_windows"]; present {
		return validationError("backend.scheduler.field_not_applicable", "field", "max_catchup_windows", "missed_window_policy", missedWindowPolicy(data))
	}
	rawMissedWindowPolicy := textValue(data["missed_window_policy"])
	if rawMissedWindowPolicy != "" && missedWindowPolicy(data) == "skip" && !strings.EqualFold(rawMissedWindowPolicy, "skip") {
		return validationError("backend.scheduler.missed_window_policy_invalid", "field", "missed_window_policy", "allowed", "catch_up_bounded,catch_up_one,skip", "actual", rawMissedWindowPolicy)
	}
	if nextRunAt := textValue(data["next_run_at"]); nextRunAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, nextRunAt); err != nil {
			return validationError("backend.scheduler.next_run_at_invalid", "field", "next_run_at", "actual", nextRunAt)
		}
	}
	if rawI18n, present := data["i18n"]; present {
		if _, valid := rawI18n.(map[string]any); !valid {
			return validationError("backend.scheduler.i18n_invalid", "field", "i18n")
		}
	}
	if err := validateScheduleData(ctx, data, true); err != nil {
		return err
	}
	if textValue(data["key"]) == "" {
		return validationError("backend.scheduler.key_required", "field", "key")
	}
	if textValue(data["name"]) == "" {
		return validationError("backend.scheduler.name_required", "field", "name")
	}
	status := normalizedString(data["status"])
	if !map[string]bool{"archived": true, "disabled": true, "draft": true, "enabled": true, "paused": true}[status] {
		return validationError("backend.scheduler.status_invalid", "field", "status", "actual", status)
	}
	return nil
}

// ValidateData validates a leaf Scheduler schedule authoring payload.
func ValidateData(ctx context.Context, data map[string]any) error {
	if field := unsupportedScheduleField(data); field != "" {
		return validationError("backend.scheduler.field_unsupported", "field", field)
	}
	return validateScheduleData(ctx, data, false)
}

func validateScheduleData(ctx context.Context, data map[string]any, allowBusinessCalendar bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if timezone := textValue(data["timezone"]); timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return validationError("backend.scheduler.timezone_invalid", "field", "timezone", "actual", timezone, "expected", "IANA timezone")
		}
	}
	rawScheduleType := normalizedString(data["schedule_type"])
	switch rawScheduleType {
	case "interval", "daily_at", "weekly_at", "monthly_at", "cron":
	case "":
		return validationError("backend.scheduler.schedule_type_required", "field", "schedule_type")
	default:
		return validationError("backend.scheduler.schedule_type_invalid", "field", "schedule_type", "allowed", "cron,daily_at,interval,monthly_at,weekly_at", "actual", rawScheduleType)
	}
	switch rawScheduleType {
	case "interval":
		seconds, secondsValid := strictAuthoringInteger(data["interval_seconds"])
		if !secondsValid || seconds <= 0 {
			return validationError("backend.scheduler.interval_invalid", "field", "interval_seconds", "minimum", "1", "actual", fmt.Sprint(seconds))
		}
		if field := firstPresentField(data, "schedule_expression", "time_of_day", "day_of_week", "day_of_month"); field != "" {
			return validationError("backend.scheduler.field_not_applicable", "field", field, "schedule_type", rawScheduleType)
		}
	case "daily_at":
		if _, _, _, ok := ParseClock(textValue(data["time_of_day"])); !ok {
			return validationError("backend.scheduler.time_of_day_invalid", "field", "time_of_day")
		}
		if field := firstPresentField(data, "schedule_expression", "interval_seconds", "day_of_week", "day_of_month"); field != "" {
			return validationError("backend.scheduler.field_not_applicable", "field", field, "schedule_type", rawScheduleType)
		}
	case "weekly_at":
		if _, _, _, ok := ParseClock(textValue(data["time_of_day"])); !ok {
			return validationError("backend.scheduler.time_of_day_invalid", "field", "time_of_day")
		}
		if _, ok := ParseWeekday(textValue(data["day_of_week"])); !ok {
			return validationError("backend.scheduler.day_of_week_invalid", "field", "day_of_week")
		}
		if field := firstPresentField(data, "schedule_expression", "interval_seconds", "day_of_month"); field != "" {
			return validationError("backend.scheduler.field_not_applicable", "field", field, "schedule_type", rawScheduleType)
		}
	case "monthly_at":
		if _, _, _, ok := ParseClock(textValue(data["time_of_day"])); !ok {
			return validationError("backend.scheduler.time_of_day_invalid", "field", "time_of_day")
		}
		day, dayValid := strictAuthoringInteger(data["day_of_month"])
		if !dayValid || day < 1 || day > 31 {
			return validationError("backend.scheduler.day_of_month_invalid", "field", "day_of_month", "minimum", "1", "maximum", "31", "actual", fmt.Sprint(day))
		}
		if field := firstPresentField(data, "schedule_expression", "interval_seconds", "day_of_week"); field != "" {
			return validationError("backend.scheduler.field_not_applicable", "field", field, "schedule_type", rawScheduleType)
		}
	case "cron":
		if _, ok := Cron(data); !ok {
			return validationError("backend.scheduler.cron_invalid", "field", "schedule_expression")
		}
		if field := firstPresentField(data, "interval_seconds", "time_of_day", "day_of_week", "day_of_month"); field != "" {
			return validationError("backend.scheduler.field_not_applicable", "field", field, "schedule_type", rawScheduleType)
		}
	}
	calendarKey := textValue(data["business_calendar_key"])
	nonWorkingDayPolicy := normalizedString(data["non_working_day_policy"])
	if !allowBusinessCalendar && (calendarKey != "" || nonWorkingDayPolicy != "") {
		field := "business_calendar_key"
		if calendarKey == "" {
			field = "non_working_day_policy"
		}
		return validationError("backend.scheduler.field_unsupported", "field", field)
	}
	if calendarKey == "" {
		if nonWorkingDayPolicy != "" {
			return validationError("backend.scheduler.field_not_applicable", "field", "non_working_day_policy", "requires", "business_calendar_key")
		}
		return nil
	}
	if rawScheduleType == "interval" {
		return validationError("backend.scheduler.field_not_applicable", "field", "business_calendar_key", "schedule_type", rawScheduleType)
	}
	if textValue(data["timezone"]) == "" {
		return validationError("backend.scheduler.business_calendar_timezone_required", "field", "timezone")
	}
	if nonWorkingDayPolicy == "" {
		nonWorkingDayPolicy = "skip"
	}
	if nonWorkingDayPolicy != "skip" && nonWorkingDayPolicy != "roll_forward" {
		return validationError("backend.scheduler.non_working_day_policy_invalid", "field", "non_working_day_policy", "allowed", "roll_forward,skip", "actual", nonWorkingDayPolicy)
	}
	return nil
}

var scheduleAuthoringFields = map[string]bool{
	"schedule_type": true, "schedule_expression": true, "interval_seconds": true,
	"time_of_day": true, "day_of_week": true, "day_of_month": true, "timezone": true,
}

func unsupportedScheduleField(data map[string]any) string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !scheduleAuthoringFields[key] {
			return key
		}
	}
	return ""
}

var definitionAuthoringFields = map[string]bool{
	"key": true, "name": true, "description": true, "i18n": true, "status": true, "revision": true, "next_run_at": true,
	"schedule_type": true, "schedule_expression": true, "interval_seconds": true, "time_of_day": true, "day_of_week": true, "day_of_month": true, "timezone": true,
	"business_calendar_key": true, "non_working_day_policy": true,
	"missed_window_policy": true, "max_catchup_windows": true, "max_attempts": true, "timeout_seconds": true, "retry_delay_seconds": true, "retry_max_delay_seconds": true,
	"target_type": true, "target_key": true, "target_object": true, "run_as_role": true, "connection_key": true, "payload_json": true,
}

func unsupportedDefinitionField(data map[string]any) string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !definitionAuthoringFields[key] {
			return key
		}
	}
	return ""
}

func firstPresentField(data map[string]any, fields ...string) string {
	for _, field := range fields {
		if _, present := data[field]; present {
			return field
		}
	}
	return ""
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
	result, _ := value.(string)
	return strings.TrimSpace(result)
}

func definitionPayload(data map[string]any) (string, bool, bool) {
	raw, present := data["payload_json"]
	if !present {
		return "", false, true
	}
	payload, valid := raw.(string)
	if !valid {
		return "", true, false
	}
	return strings.TrimSpace(payload), true, true
}

func optionalAuthoringInteger(data map[string]any, key string, fallback int) (int, bool) {
	value, present := data[key]
	if !present {
		return fallback, true
	}
	return strictAuthoringInteger(value)
}

func strictAuthoringInteger(value any) (int, bool) {
	if number, ok := value.(json.Number); ok {
		parsed, err := number.Int64()
		return int(parsed), err == nil && int64(int(parsed)) == parsed
	}
	current := reflect.ValueOf(value)
	if !current.IsValid() {
		return 0, false
	}
	switch current.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed := current.Int()
		return int(parsed), int64(int(parsed)) == parsed
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		parsed := current.Uint()
		converted := int(parsed)
		return converted, converted >= 0 && uint64(converted) == parsed
	case reflect.Float32, reflect.Float64:
		parsed := current.Float()
		if math.IsNaN(parsed) || math.IsInf(parsed, 0) || math.Trunc(parsed) != parsed {
			return 0, false
		}
		converted := int(parsed)
		return converted, float64(converted) == parsed
	default:
		return 0, false
	}
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

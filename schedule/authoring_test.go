package schedule

import (
	"testing"
)

func TestSchedulerAuthoringValidationRejectsInvalidCalendarFields(t *testing.T) {
	base := map[string]any{"target_type": "workflow", "target_key": "scheduled:reminder", "timezone": "UTC", "time_of_day": "08:00"}
	weekly := cloneAuthoringData(base)
	weekly["schedule_type"], weekly["day_of_week"] = "weekly_at", "funday"
	if err := ValidateDefinitionData(t.Context(), weekly); ValidationCode(err) != "backend.scheduler.day_of_week_invalid" {
		t.Fatalf("weekly error=%v code=%s", err, ValidationCode(err))
	}
	monthly := cloneAuthoringData(base)
	monthly["schedule_type"], monthly["day_of_month"] = "monthly_at", 32
	if err := ValidateDefinitionData(t.Context(), monthly); ValidationCode(err) != "backend.scheduler.day_of_month_invalid" {
		t.Fatalf("monthly error=%v code=%s", err, ValidationCode(err))
	}
	invalidRange := cloneAuthoringData(base)
	invalidRange["schedule_type"], invalidRange["interval_seconds"], invalidRange["max_attempts"] = "interval", 10, 101
	err := ValidateDefinitionData(t.Context(), invalidRange)
	params := ValidationParams(err)
	if ValidationCode(err) != "backend.scheduler.max_attempts_invalid" || params["field"] != "max_attempts" || params["minimum"] != "1" || params["maximum"] != "100" || params["actual"] != "101" {
		t.Fatalf("scheduler range issue lacks bounds: code=%s params=%#v", ValidationCode(err), params)
	}
	invalidTarget := cloneAuthoringData(base)
	invalidTarget["target_type"] = "script"
	err = ValidateDefinitionData(t.Context(), invalidTarget)
	params = ValidationParams(err)
	if ValidationCode(err) != "backend.scheduler.target_type_unsupported" || params["field"] != "target_type" || params["allowed"] == "" || params["actual"] != "script" {
		t.Fatalf("scheduler target issue lacks allowed values: code=%s params=%#v", ValidationCode(err), params)
	}
}

func TestSchedulerAuthoringValidationAcceptsConfiguredHTTPCallback(t *testing.T) {
	definition := map[string]any{"target_type": "http", "target_key": "sync", "connection_key": "partner", "operation": "sync", "dispatch_mode": "runtime_callback", "schedule_type": "interval", "interval_seconds": 60, "timezone": "UTC", "max_attempts": 3, "timeout_seconds": 30}
	if err := ValidateDefinitionData(t.Context(), definition); err != nil {
		t.Fatal(err)
	}
	delete(definition, "connection_key")
	if err := ValidateDefinitionData(t.Context(), definition); ValidationCode(err) != "backend.scheduler.http_connection_required" {
		t.Fatalf("error=%v", err)
	}
}

func cloneAuthoringData(data map[string]any) map[string]any {
	out := make(map[string]any, len(data))
	for key, value := range data {
		out[key] = value
	}
	return out
}

package schedule

import (
	"strings"
	"testing"
)

func TestSchedulerAuthoringValidationRejectsInvalidCalendarFields(t *testing.T) {
	base := map[string]any{"key": "reminder", "name": "Reminder", "status": "enabled", "target_type": "workflow", "target_key": "scheduled:reminder", "timezone": "UTC", "time_of_day": "08:00", "max_attempts": 1, "timeout_seconds": 300}
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
	definition := validSchedulerDefinition()
	definition["target_type"], definition["target_key"], definition["connection_key"] = "http", "sync", "partner"
	if err := ValidateDefinitionData(t.Context(), definition); err != nil {
		t.Fatal(err)
	}
	delete(definition, "connection_key")
	if err := ValidateDefinitionData(t.Context(), definition); ValidationCode(err) != "backend.scheduler.http_connection_required" {
		t.Fatalf("error=%v", err)
	}
}

func TestSchedulerAuthoringValidationAcceptsGovernedBusinessAction(t *testing.T) {
	definition := validSchedulerDefinition()
	definition["target_type"], definition["target_key"] = "business_action", "order.expire"
	definition["target_object"], definition["run_as_role"], definition["payload_json"] = "order", "order_automation", `{"status":"expired"}`
	if err := ValidateDefinitionData(t.Context(), definition); err != nil {
		t.Fatal(err)
	}
	for field, code := range map[string]string{
		"target_object": "backend.scheduler.business_action_object_required",
		"run_as_role":   "backend.scheduler.business_action_role_required",
	} {
		invalid := cloneAuthoringData(definition)
		delete(invalid, field)
		if err := ValidateDefinitionData(t.Context(), invalid); ValidationCode(err) != code {
			t.Fatalf("field=%s error=%v", field, err)
		}
	}
	definition["payload_json"] = "{"
	if err := ValidateDefinitionData(t.Context(), definition); ValidationCode(err) != "backend.scheduler.payload_invalid" {
		t.Fatalf("payload error=%v", err)
	}
	definition["payload_json"] = `[]`
	if err := ValidateDefinitionData(t.Context(), definition); ValidationCode(err) != "backend.scheduler.payload_invalid" {
		t.Fatalf("non-object payload error=%v", err)
	}
	definition["payload_json"] = `{"value":"` + strings.Repeat("x", MaximumDefinitionPayloadBytes) + `"}`
	if err := ValidateDefinitionData(t.Context(), definition); ValidationCode(err) != "backend.scheduler.payload_invalid" {
		t.Fatalf("oversized payload error=%v", err)
	}
	definition["payload_json"] = map[string]any{"status": "expired"}
	if err := ValidateDefinitionData(t.Context(), definition); ValidationCode(err) != "backend.scheduler.payload_invalid" {
		t.Fatalf("non-string payload error=%v", err)
	}
}

func TestSchedulerAuthoringRejectsFieldsWithoutDefinitionExecutionSemantics(t *testing.T) {
	for _, field := range []string{"trigger_type", "interval_minutes", "interval_hours", "operation", "dispatch_mode", "condition_json", "retry_backoff", "idempotency_keys"} {
		definition := validSchedulerDefinition()
		definition[field] = "value"
		if err := ValidateDefinitionData(t.Context(), definition); ValidationCode(err) != "backend.scheduler.field_unsupported" || ValidationParams(err)["field"] != field {
			t.Fatalf("field=%s error=%v params=%v", field, err, ValidationParams(err))
		}
	}
}

func TestSchedulerAuthoringValidatesBusinessCalendarApplicability(t *testing.T) {
	definition := validSchedulerDefinition()
	definition["schedule_type"], definition["time_of_day"] = "daily_at", "09:00"
	delete(definition, "interval_seconds")
	definition["business_calendar_key"], definition["non_working_day_policy"] = "cn_operations", "roll_forward"
	if err := ValidateDefinitionData(t.Context(), definition); err != nil {
		t.Fatal(err)
	}
	definition["non_working_day_policy"] = "nearest"
	if err := ValidateDefinitionData(t.Context(), definition); ValidationCode(err) != "backend.scheduler.non_working_day_policy_invalid" {
		t.Fatalf("invalid policy error=%v", err)
	}
	delete(definition, "timezone")
	if err := ValidateDefinitionData(t.Context(), definition); ValidationCode(err) != "backend.scheduler.business_calendar_timezone_required" {
		t.Fatalf("missing timezone error=%v", err)
	}
	interval := validSchedulerDefinition()
	interval["business_calendar_key"] = "cn_operations"
	if err := ValidateDefinitionData(t.Context(), interval); ValidationCode(err) != "backend.scheduler.field_not_applicable" {
		t.Fatalf("interval calendar error=%v", err)
	}
}

func cloneAuthoringData(data map[string]any) map[string]any {
	out := make(map[string]any, len(data))
	for key, value := range data {
		out[key] = value
	}
	return out
}

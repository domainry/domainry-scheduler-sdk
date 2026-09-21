package authoring

import (
	capabilitycontract "github.com/domainry/domainry-scheduler-sdk/authoring/contract"
	"github.com/domainry/domainry-scheduler-sdk/schedule"
)

func schedulerBusinessJobParameters() []capabilitycontract.CapabilityAuthoringParameter {
	return []capabilitycontract.CapabilityAuthoringParameter{
		{Key: "key", Type: "scheduler_job_key", Required: true}, {Key: "name", Type: "string", Required: true},
		{Key: "status", Type: "string", Required: true, Default: "draft", Enum: []string{"archived", "disabled", "draft", "enabled", "paused"}},
		{Key: "schedule_type", Type: "string", Required: true, Enum: []string{"cron", "daily_at", "interval", "monthly_at", "weekly_at"}},
		{Key: "schedule_expression", Type: "string"}, {Key: "time_of_day", Type: "time"}, {Key: "day_of_week", Type: "string", Enum: []string{"friday", "monday", "saturday", "sunday", "thursday", "tuesday", "wednesday"}},
		{Key: "day_of_month", Type: "integer", Minimum: schedulerAuthoringFloatPointer(1), Maximum: schedulerAuthoringFloatPointer(31)},
		{Key: "interval_seconds", Type: "integer", Minimum: schedulerAuthoringFloatPointer(1)},
		{Key: "missed_window_policy", Type: "string", Default: "skip", Enum: []string{"catch_up_bounded", "catch_up_one", "skip"}},
		{Key: "max_catchup_windows", Type: "integer", Minimum: schedulerAuthoringFloatPointer(1), Maximum: schedulerAuthoringFloatPointer(100)}, {Key: "timezone", Type: "iana_timezone"},
		{Key: "business_calendar_key", Type: "business_calendar_key"}, {Key: "non_working_day_policy", Type: "string", Default: "skip", Enum: []string{"roll_forward", "skip"}},
		{Key: "target_type", Type: "string", Required: true, Enum: []string{"business_action", "http", "report_snapshot_refresh", "workflow"}}, {Key: "target_key", Type: "string", Required: true}, {Key: "target_object", Type: "object_key", RequiredWhen: map[string]any{"target_type": "business_action"}},
		{Key: "connection_key", Type: "string", RequiredWhen: map[string]any{"target_type": "http"}}, {Key: "payload_json", Type: "string", MaxLength: schedulerAuthoringIntPointer(schedule.MaximumDefinitionPayloadBytes)}, {Key: "run_as_role", Type: "role_key", RequiredWhen: map[string]any{"target_type": "business_action"}},
		{Key: "max_attempts", Type: "integer", Required: true, Default: 1, Minimum: schedulerAuthoringFloatPointer(1), Maximum: schedulerAuthoringFloatPointer(100)},
		{Key: "retry_delay_seconds", Type: "integer", Default: 30, Minimum: schedulerAuthoringFloatPointer(0)}, {Key: "retry_max_delay_seconds", Type: "integer", Default: 900, Minimum: schedulerAuthoringFloatPointer(0)},
		{Key: "timeout_seconds", Type: "integer", Required: true, Default: 300, Minimum: schedulerAuthoringFloatPointer(1), Maximum: schedulerAuthoringFloatPointer(86400)},
		{Key: "description", Type: "string"}, {Key: "i18n", Type: "object"}, {Key: "next_run_at", Type: "datetime"},
	}
}

func schedulerScheduleParameters() []capabilitycontract.CapabilityAuthoringParameter {
	return []capabilitycontract.CapabilityAuthoringParameter{
		{Key: "schedule_type", Type: "string", Required: true, Enum: []string{"cron", "daily_at", "interval", "monthly_at", "weekly_at"}},
		{Key: "schedule_expression", Type: "string"}, {Key: "interval_seconds", Type: "integer", Minimum: schedulerAuthoringFloatPointer(1)},
		{Key: "time_of_day", Type: "time"}, {Key: "day_of_week", Type: "string", Enum: []string{"friday", "monday", "saturday", "sunday", "thursday", "tuesday", "wednesday"}},
		{Key: "day_of_month", Type: "integer", Minimum: schedulerAuthoringFloatPointer(1), Maximum: schedulerAuthoringFloatPointer(31)}, {Key: "timezone", Type: "iana_timezone"},
	}
}

func schedulerObjectSchema(parameters []capabilitycontract.CapabilityAuthoringParameter) *capabilitycontract.CapabilityAuthoringSchema {
	closed := false
	properties := map[string]capabilitycontract.CapabilityAuthoringSchema{}
	required := []string{}
	for _, parameter := range parameters {
		property := capabilitycontract.CapabilityAuthoringSchema{Default: parameter.Default, Minimum: parameter.Minimum, Maximum: parameter.Maximum, MinLength: parameter.MinLength, MaxLength: parameter.MaxLength}
		switch parameter.Type {
		case "integer":
			property.Type = "integer"
		case "boolean":
			property.Type = "boolean"
		case "object":
			property.Type = "object"
		default:
			property.Type = "string"
		}
		for _, value := range parameter.Enum {
			property.Enum = append(property.Enum, value)
		}
		properties[parameter.Key] = property
		if parameter.Required {
			required = append(required, parameter.Key)
		}
	}
	return &capabilitycontract.CapabilityAuthoringSchema{Schema: "https://json-schema.org/draft/2020-12/schema", Type: "object", AdditionalProperties: &closed, Properties: properties, Required: required}
}

func schedulerPreviewOutputSchema() *capabilitycontract.CapabilityAuthoringSchema {
	closed := false
	return &capabilitycontract.CapabilityAuthoringSchema{Schema: "https://json-schema.org/draft/2020-12/schema", Type: "object", AdditionalProperties: &closed, Required: []string{"next_runs"}, Properties: map[string]capabilitycontract.CapabilityAuthoringSchema{"next_runs": {Type: "array", Items: &capabilitycontract.CapabilityAuthoringSchema{Type: "string", Format: "date-time"}}}}
}

func schedulerJobRecordOutputSchema() *capabilitycontract.CapabilityAuthoringSchema {
	open, closed := true, false
	return &capabilitycontract.CapabilityAuthoringSchema{Schema: "https://json-schema.org/draft/2020-12/schema", Type: "object", AdditionalProperties: &closed, Required: []string{"id", "data"}, Properties: map[string]capabilitycontract.CapabilityAuthoringSchema{"id": {Type: "string"}, "data": {Type: "object", AdditionalProperties: &open}}}
}

func schedulerOperationOutputSchema() *capabilitycontract.CapabilityAuthoringSchema {
	open, closed := true, false
	return &capabilitycontract.CapabilityAuthoringSchema{Schema: "https://json-schema.org/draft/2020-12/schema", Type: "object", AdditionalProperties: &closed, Required: []string{"status", "message"}, Properties: map[string]capabilitycontract.CapabilityAuthoringSchema{"status": {Type: "string"}, "message": {Type: "string"}, "run": {Type: "object", AdditionalProperties: &open}, "result": {Type: "object", AdditionalProperties: &open}}}
}

func schedulerDefinitionAuthoringErrors() []capabilitycontract.CapabilityAuthoringError {
	return []capabilitycontract.CapabilityAuthoringError{
		{Code: "backend.scheduler.key_required", FieldPath: "key", MessageKey: "backend.scheduler.key_required"},
		{Code: "backend.scheduler.name_required", FieldPath: "name", MessageKey: "backend.scheduler.name_required"},
		{Code: "backend.scheduler.target_type_required", FieldPath: "target_type", MessageKey: "backend.scheduler.target_type_required"},
		{Code: "backend.scheduler.target_type_unsupported", FieldPath: "target_type", MessageKey: "backend.scheduler.target_type_unsupported"},
		{Code: "backend.scheduler.target_key_required", FieldPath: "target_key", MessageKey: "backend.scheduler.target_key_required"},
		{Code: "backend.scheduler.business_action_object_required", FieldPath: "target_object", MessageKey: "backend.scheduler.business_action_object_required"},
		{Code: "backend.scheduler.business_action_role_required", FieldPath: "run_as_role", MessageKey: "backend.scheduler.business_action_role_required"},
		{Code: "backend.scheduler.payload_invalid", FieldPath: "payload_json", MessageKey: "backend.scheduler.payload_invalid"},
		{Code: "backend.scheduler.http_connection_required", FieldPath: "connection_key", MessageKey: "backend.scheduler.http_connection_required"},
		{Code: "backend.scheduler.field_unsupported", FieldPath: "$", MessageKey: "backend.scheduler.field_unsupported"},
		{Code: "backend.scheduler.field_not_applicable", FieldPath: "$", MessageKey: "backend.scheduler.field_not_applicable"},
		{Code: "backend.scheduler.status_invalid", FieldPath: "status", MessageKey: "backend.scheduler.status_invalid"},
		{Code: "backend.scheduler.max_attempts_required", FieldPath: "max_attempts", MessageKey: "backend.scheduler.max_attempts_required"},
		{Code: "backend.scheduler.timeout_required", FieldPath: "timeout_seconds", MessageKey: "backend.scheduler.timeout_required"},
		{Code: "backend.scheduler.next_run_at_invalid", FieldPath: "next_run_at", MessageKey: "backend.scheduler.next_run_at_invalid"},
		{Code: "backend.scheduler.i18n_invalid", FieldPath: "i18n", MessageKey: "backend.scheduler.i18n_invalid"},
		{Code: "backend.scheduler.retry_policy_invalid", FieldPath: "retry_delay_seconds", MessageKey: "backend.scheduler.retry_policy_invalid"},
		{Code: "backend.scheduler.timezone_invalid", FieldPath: "timezone", MessageKey: "backend.scheduler.timezone_invalid"},
		{Code: "backend.scheduler.business_calendar_timezone_required", FieldPath: "timezone", MessageKey: "backend.scheduler.business_calendar_timezone_required"},
		{Code: "backend.scheduler.non_working_day_policy_invalid", FieldPath: "non_working_day_policy", MessageKey: "backend.scheduler.non_working_day_policy_invalid"},
		{Code: "backend.scheduler.schedule_type_required", FieldPath: "schedule_type", MessageKey: "backend.scheduler.schedule_type_required"},
		{Code: "backend.scheduler.schedule_type_invalid", FieldPath: "schedule_type", MessageKey: "backend.scheduler.schedule_type_invalid"},
		{Code: "backend.scheduler.interval_invalid", FieldPath: "interval_seconds", MessageKey: "backend.scheduler.interval_invalid"},
		{Code: "backend.scheduler.cron_invalid", FieldPath: "schedule_expression", MessageKey: "backend.scheduler.cron_invalid"},
		{Code: "backend.scheduler.definition_version_conflict", FieldPath: "@header.Expected-Record-Version", MessageKey: "backend.scheduler.definition_version_conflict"},
		{Code: "backend.scheduler.definition_version_required", FieldPath: "version_id", MessageKey: "backend.scheduler.definition_version_required"},
		{Code: "backend.scheduler.definition_version_not_found", FieldPath: "version_id", MessageKey: "backend.scheduler.definition_version_not_found"},
		{Code: "backend.scheduler.definition_history_unavailable", FieldPath: "@runtime.audit_history", MessageKey: "backend.scheduler.definition_history_unavailable"},
		{Code: "backend.scheduler.definition_authoring_unavailable", FieldPath: "@runtime.record_mutation", MessageKey: "backend.scheduler.definition_authoring_unavailable"},
	}
}

func schedulerBusinessJobExamples() []capabilitycontract.CapabilityAuthoringExample {
	return []capabilitycontract.CapabilityAuthoringExample{
		{Name: "minimal_valid", Value: map[string]any{"key": "daily_operations_snapshot", "name": "Daily operations snapshot", "status": "enabled", "schedule_type": "daily_at", "time_of_day": "02:00", "timezone": "UTC", "target_type": "report_snapshot_refresh", "target_key": "orders.daily", "max_attempts": 1, "timeout_seconds": 300}},
		{Name: "representative", Value: map[string]any{"key": "order_expiration", "name": "Expire overdue orders", "status": "enabled", "schedule_type": "cron", "schedule_expression": "0 9 * * *", "timezone": "Asia/Shanghai", "business_calendar_key": "cn_operations", "non_working_day_policy": "roll_forward", "missed_window_policy": "catch_up_bounded", "max_catchup_windows": 3, "target_type": "business_action", "target_key": "order.expire_overdue", "target_object": "order", "run_as_role": "order_automation", "payload_json": "{\"status\":\"overdue\"}", "max_attempts": 3, "retry_delay_seconds": 10, "retry_max_delay_seconds": 300, "timeout_seconds": 1800, "description": "Runs one governed Business Action for the schedule window"}},
		{Name: "invalid_with_repair", Value: map[string]any{"key": "invalid_job", "name": "Invalid job", "status": "enabled", "schedule_type": "interval", "interval_seconds": 0, "target_type": "workflow", "target_key": "order.approval", "max_attempts": 1, "timeout_seconds": 300}, ExpectedErrorCodes: []string{"backend.scheduler.workflow_target_invalid"}},
	}
}

func schedulerScheduleExamples() []capabilitycontract.CapabilityAuthoringExample {
	return []capabilitycontract.CapabilityAuthoringExample{
		{Name: "minimal_valid", Value: map[string]any{"schedule_type": "interval", "interval_seconds": 300}},
		{Name: "representative", Value: map[string]any{"schedule_type": "weekly_at", "time_of_day": "09:30", "day_of_week": "monday", "timezone": "Asia/Shanghai"}},
		{Name: "invalid_with_repair", Value: map[string]any{"schedule_type": "monthly_at", "time_of_day": "09:00", "day_of_month": 40}, ExpectedErrorCodes: []string{"backend.scheduler.day_of_month_invalid"}},
	}
}

func schedulerCommandExamples(resourceParameter string, idempotencyRequired bool) []capabilitycontract.CapabilityAuthoringExample {
	minimal := map[string]any{resourceParameter: "record_01"}
	representative := map[string]any{resourceParameter: "record_02"}
	invalid := map[string]any{resourceParameter: ""}
	if idempotencyRequired {
		minimal["idempotency_key"], representative["idempotency_key"], invalid["idempotency_key"] = "command-01", "command-02", ""
	}
	return []capabilitycontract.CapabilityAuthoringExample{{Name: "minimal_valid", Value: minimal}, {Name: "representative", Value: representative}, {Name: "invalid_with_repair", Value: invalid, ExpectedErrorCodes: []string{"backend.scheduler.run_not_found"}}}
}

func schedulerAuthoringFloatPointer(value float64) *float64 { return &value }
func schedulerAuthoringIntPointer(value int) *int           { return &value }

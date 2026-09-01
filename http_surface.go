package schedulersdk

import (
	"strings"

	actioncontract "github.com/domainry/domainry-foundation/action"
)

const SchedulerHTTPSurfaceContractVersion = "domainry-scheduler-http-surface-v2"

type HTTPRouteContract struct {
	Action actioncontract.ActionDefinition `json:"action"`
}

func (route HTTPRouteContract) Pattern() string {
	if route.Action.HTTP == nil {
		return ""
	}
	return route.Action.HTTP.Method + " " + route.Action.HTTP.RouteTemplate
}

type HTTPSurfaceContract struct {
	ContractVersion string                    `json:"contract_version"`
	Owner           string                    `json:"owner"`
	Name            string                    `json:"name"`
	Routes          []HTTPRouteContract       `json:"routes"`
	OpenAPI         map[string]map[string]any `json:"openapi_operations"`
}

// SchedulerHTTPSurfaceContract is the source-owned external Runtime facade.
// Runtime hosts these routes because it owns principals, published metadata,
// and operation receipts, but Scheduler owns their scheduling semantics and
// deployment-neutral Binding calls.
func SchedulerHTTPSurfaceContract() HTTPSurfaceContract {
	definitions := tenantAdminDefinitionHTTPSchema()
	authoringContract := objectSchema(map[string]any{
		"resource_type": map[string]any{"type": "string"}, "status_field": map[string]any{"type": "string"}, "allowed_statuses": arraySchema(map[string]any{"type": "string"}),
		"business_calendar_field": map[string]any{"type": "string"}, "mutation_owner": map[string]any{"type": "string"}, "validation_endpoint": map[string]any{"type": "string"},
	}, "resource_type", "status_field", "allowed_statuses", "business_calendar_field", "mutation_owner", "validation_endpoint")
	run := schedulerRunHTTPSchema()
	deadLetter := schedulerDeadLetterHTTPSchema()
	preview := objectSchema(map[string]any{"next_runs": arraySchema(map[string]any{"type": "string", "format": "date-time"})}, "next_runs")
	simulation := objectSchema(map[string]any{
		"status": map[string]any{"type": "string"}, "message": map[string]any{"type": "string"}, "next_run_at": map[string]any{"type": "string", "format": "date-time"},
		"target_type": map[string]any{"type": "string"}, "target_key": map[string]any{"type": "string"},
	}, "status")
	note := objectSchema(map[string]any{"note": map[string]any{"type": "string"}})
	patterns := []HTTPRouteContract{
		schedulerRoute("scheduler.definitions.list", "GET /tenant-admin/scheduler/definitions", actioncontract.ExposureTenantAdmin, actioncontract.EffectRead, "not_applicable", "scheduler_definition_read"),
		schedulerRoute("scheduler.definitions.get", "GET /tenant-admin/scheduler/definitions/{definitionID}", actioncontract.ExposureTenantAdmin, actioncontract.EffectRead, "not_applicable", "scheduler_definition_read"),
		schedulerRoute("scheduler.authoring_contract.get", "GET /tenant-admin/scheduler/authoring-contract", actioncontract.ExposureTenantAdmin, actioncontract.EffectRead, "not_applicable", "scheduler_definition_authoring"),
		schedulerRoute("scheduler.definitions.validate", "POST /tenant-admin/scheduler/definitions/validate", actioncontract.ExposureTenantAdmin, actioncontract.EffectRead, "natural", "scheduler_definition_validation"),
		schedulerRoute("scheduler.schedules.preview", "POST /tenant-admin/scheduler/schedules/preview", actioncontract.ExposureTenantAdmin, actioncontract.EffectRead, "natural", "scheduler_schedule_preview"),
		schedulerRoute("scheduler.definitions.simulate", "POST /tenant-admin/scheduler/definitions/{definitionID}/simulate", actioncontract.ExposureTenantAdmin, actioncontract.EffectRead, "natural", "scheduler_definition_simulation"),
		schedulerRoute("scheduler.state.get", "GET /operations/scheduler/state", actioncontract.ExposureOps, actioncontract.EffectRead, "not_applicable", "scheduler_operations_read"),
		schedulerRoute("scheduler.definitions.run", "POST /operations/scheduler/definitions/{definitionID}/run", actioncontract.ExposureOps, actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command"),
		schedulerRoute("scheduler.definitions.reschedule", "POST /operations/scheduler/definitions/{definitionID}/reschedule", actioncontract.ExposureOps, actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command"),
		schedulerRoute("scheduler.runs.retry", "POST /operations/scheduler/runs/{runID}/retry", actioncontract.ExposureOps, actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command"),
		schedulerRoute("scheduler.runs.cancel", "POST /operations/scheduler/runs/{runID}/cancel", actioncontract.ExposureOps, actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command"),
		schedulerRoute("scheduler.dead_letters.resolve", "POST /operations/scheduler/dead-letters/{deadLetterID}/resolve", actioncontract.ExposureOps, actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command"),
		schedulerRoute("scheduler.dead_letters.requeue", "POST /operations/scheduler/dead-letters/{deadLetterID}/requeue", actioncontract.ExposureOps, actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command"),
	}
	operations := map[string]map[string]any{
		"GET /tenant-admin/scheduler/definitions": schedulerOperation("listSchedulerDefinitions", "List published Scheduler definitions", nil, nil,
			objectSchema(map[string]any{"items": arraySchema(definitions), "count": map[string]any{"type": "integer"}}, "items", "count")),
		"GET /tenant-admin/scheduler/definitions/{definitionID}": schedulerOperation("getSchedulerDefinition", "Read one published Scheduler definition", []any{pathParameter("definitionID")}, nil, definitions),
		"GET /tenant-admin/scheduler/authoring-contract":         schedulerOperation("getSchedulerAuthoringContract", "Read Scheduler definition authoring rules", nil, nil, authoringContract),
		"POST /tenant-admin/scheduler/definitions/validate": schedulerOperation("validateSchedulerDefinition", "Validate and preview a Scheduler job definition", nil,
			objectSchema(map[string]any{"data": map[string]any{"type": "object", "additionalProperties": true}}, "data"), preview),
		"POST /tenant-admin/scheduler/schedules/preview": schedulerOperation("previewSchedulerSchedule", "Validate and preview a schedule fragment", nil,
			map[string]any{"type": "object", "additionalProperties": true}, preview),
		"POST /tenant-admin/scheduler/definitions/{definitionID}/simulate": schedulerOperation("simulateSchedulerDefinition", "Simulate one published Scheduler definition without durable effects", []any{pathParameter("definitionID")}, nil, simulation),
		"GET /operations/scheduler/state": schedulerOperation("getSchedulerOperationsState", "Inspect Scheduler-owned run and dead-letter state", nil, nil,
			objectSchema(map[string]any{"provisioned": map[string]any{"type": "boolean"}, "runs": arraySchema(run), "dead_letters": arraySchema(deadLetter)}, "provisioned", "runs", "dead_letters")),
		"POST /operations/scheduler/definitions/{definitionID}/run": schedulerOperation("runSchedulerDefinition", "Trigger one Scheduler definition now", commandParameters("definitionID"), nil, run),
		"POST /operations/scheduler/definitions/{definitionID}/reschedule": schedulerOperation("rescheduleSchedulerDefinition", "Move the next run time of one Scheduler definition", commandParameters("definitionID"),
			objectSchema(map[string]any{"next_run_at": map[string]any{"type": "string", "format": "date-time"}}, "next_run_at"),
			objectSchema(map[string]any{"status": map[string]any{"type": "string"}, "definition_key": map[string]any{"type": "string"}, "next_run_at": map[string]any{"type": "string", "format": "date-time"}}, "status", "definition_key", "next_run_at")),
		"POST /operations/scheduler/runs/{runID}/retry":                  schedulerOperation("retrySchedulerRun", "Retry one Scheduler run", commandParameters("runID"), nil, run),
		"POST /operations/scheduler/runs/{runID}/cancel":                 schedulerOperation("cancelSchedulerRun", "Cancel one Scheduler run", commandParameters("runID"), nil, run),
		"POST /operations/scheduler/dead-letters/{deadLetterID}/resolve": schedulerOptionalRequestOperation("resolveSchedulerDeadLetter", "Resolve one Scheduler dead letter", commandParameters("deadLetterID"), note, deadLetter),
		"POST /operations/scheduler/dead-letters/{deadLetterID}/requeue": schedulerOptionalRequestOperation("requeueSchedulerDeadLetter", "Requeue one Scheduler dead letter", commandParameters("deadLetterID"), note, run),
	}
	return HTTPSurfaceContract{ContractVersion: SchedulerHTTPSurfaceContractVersion, Owner: "scheduler", Name: "scheduler_external", Routes: patterns, OpenAPI: operations}
}

func schedulerRoute(key, pattern string, exposure actioncontract.Exposure, effect actioncontract.EffectClass, idempotency, audit string) HTTPRouteContract {
	method, path, _ := strings.Cut(pattern, " ")
	separator := strings.LastIndex(key, ".")
	risk := actioncontract.RiskMedium
	if effect == actioncontract.EffectRead {
		risk = actioncontract.RiskLow
	}
	action := actioncontract.ActionDefinition{
		Key: key, Owner: "module:scheduler", SourceKind: "module_surface", CapabilityKey: "scheduler.product", CapabilityLabel: "Scheduler",
		OperationKey: key[separator+1:], OperationLabel: key, Label: key, Exposures: []actioncontract.Exposure{exposure},
		Authorization: actioncontract.Authorization{Strategy: actioncontract.AuthorizationExactRolePermission},
		HTTP:          &actioncontract.HTTPBinding{Method: method, RouteTemplate: path},
		Permission:    &actioncontract.PermissionDefinition{Key: key, Owner: "module:scheduler", ResourceKey: key[:separator], ActionKey: key[separator+1:], Label: key, Category: "Scheduler", LifecycleStatus: actioncontract.LifecycleActive},
		EffectClass:   effect, RiskLevel: risk, IdempotencyDecision: idempotency, AuditClass: audit, LifecycleStatus: actioncontract.LifecycleActive,
	}
	return HTTPRouteContract{Action: action}
}

func schedulerOperation(operationID, summary string, parameters []any, requestSchema, responseSchema map[string]any) map[string]any {
	operation := map[string]any{
		"operationId": operationID, "tags": []string{"Scheduler"}, "summary": summary, "security": []map[string]any{{"BearerAuth": []string{}}},
		"responses": map[string]any{"200": map[string]any{"description": "Scheduler response", "content": map[string]any{"application/json": map[string]any{"schema": responseSchema}}}},
	}
	if len(parameters) != 0 {
		operation["parameters"] = parameters
	}
	if requestSchema != nil {
		operation["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": requestSchema}}}
	}
	return operation
}

func schedulerOptionalRequestOperation(operationID, summary string, parameters []any, requestSchema, responseSchema map[string]any) map[string]any {
	operation := schedulerOperation(operationID, summary, parameters, requestSchema, responseSchema)
	operation["requestBody"].(map[string]any)["required"] = false
	return operation
}

func pathParameter(name string) map[string]any {
	return map[string]any{"name": name, "in": "path", "required": true, "schema": map[string]any{"type": "string", "minLength": 1}}
}

func commandParameters(resource string) []any {
	return []any{pathParameter(resource), map[string]any{"name": "Idempotency-Key", "in": "header", "required": true, "schema": map[string]any{"type": "string", "minLength": 1}}}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	result := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) != 0 {
		result["required"] = required
	}
	return result
}

func arraySchema(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func schedulerRunHTTPSchema() map[string]any {
	return objectSchema(map[string]any{
		"id": map[string]any{"type": "string"}, "definition_key": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "attempt": map[string]any{"type": "integer"},
		"scheduled_for": map[string]any{"type": "string", "format": "date-time"}, "error_message": map[string]any{"type": "string"}, "lease_owner": map[string]any{"type": "string"},
		"lease_expires_at": map[string]any{"type": "string", "format": "date-time"}, "fencing_token": map[string]any{"type": "integer"}, "correlation_id": map[string]any{"type": "string"},
		"created_at": map[string]any{"type": "string", "format": "date-time"}, "updated_at": map[string]any{"type": "string", "format": "date-time"},
	}, "id", "status")
}

func schedulerDeadLetterHTTPSchema() map[string]any {
	return objectSchema(map[string]any{
		"id": map[string]any{"type": "string"}, "run_id": map[string]any{"type": "string"}, "definition_key": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"},
		"reason": map[string]any{"type": "string"}, "failed_at": map[string]any{"type": "string", "format": "date-time"}, "resolved_at": map[string]any{"type": "string", "format": "date-time"},
	}, "id", "run_id", "status")
}

func tenantAdminDefinitionHTTPSchema() map[string]any {
	properties := map[string]any{}
	for _, name := range []string{
		"key", "name", "status", "schedule_type", "schedule_expression", "time_of_day", "day_of_week", "timezone", "business_calendar_key", "target_type", "target_key", "target_object", "run_as_role",
		"missed_window_policy", "retry_backoff", "condition_json", "payload_json", "idempotency_keys", "description", "next_run_at", "created_at", "updated_at",
	} {
		properties[name] = map[string]any{"type": "string"}
	}
	for _, name := range []string{"interval_seconds", "day_of_month", "max_attempts", "timeout_seconds", "max_catchup_windows", "retry_delay_seconds", "retry_max_delay_seconds"} {
		properties[name] = map[string]any{"type": "integer"}
	}
	return objectSchema(properties, "key", "status")
}

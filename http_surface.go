package schedulersdk

import (
	"fmt"
	"strings"

	actioncontract "github.com/domainry/domainry-foundation/action"
)

const SchedulerHTTPSurfaceContractVersion = "domainry-scheduler-http-surface-v2"

const SchedulerAuthorizationOwner = "module:scheduler"

const (
	ActionSchedulerDefinitionsList       = "scheduler.definitions.list"
	ActionSchedulerDefinitionsGet        = "scheduler.definitions.get"
	ActionSchedulerAuthoringContractGet  = "scheduler.authoring_contract.get"
	ActionSchedulerDefinitionsValidate   = "scheduler.definitions.validate"
	ActionSchedulerSchedulesPreview      = "scheduler.schedules.preview"
	ActionSchedulerDefinitionsSimulate   = "scheduler.definitions.simulate"
	ActionSchedulerStateGet              = "scheduler.state.get"
	ActionSchedulerDefinitionsRun        = "scheduler.definitions.run"
	ActionSchedulerDefinitionsReschedule = "scheduler.definitions.reschedule"
	ActionSchedulerRunsRetry             = "scheduler.runs.retry"
	ActionSchedulerRunsCancel            = "scheduler.runs.cancel"
	ActionSchedulerDeadLettersResolve    = "scheduler.dead_letters.resolve"
	ActionSchedulerDeadLettersRequeue    = "scheduler.dead_letters.requeue"
	CapabilitySchedulerAuthoring         = "scheduler.authoring"
	CapabilitySchedulerOperations        = "scheduler.operations"
)

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
func SchedulerHTTPSurfaceContract() (HTTPSurfaceContract, error) {
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
	actions, err := SchedulerAuthorizationActions()
	if err != nil {
		return HTTPSurfaceContract{}, err
	}
	operationsByAction := map[string]map[string]any{
		ActionSchedulerDefinitionsList: schedulerOperation("listSchedulerDefinitions", "List published Scheduler definitions", nil, nil,
			objectSchema(map[string]any{"items": arraySchema(definitions), "count": map[string]any{"type": "integer"}}, "items", "count")),
		ActionSchedulerDefinitionsGet:       schedulerOperation("getSchedulerDefinition", "Read one published Scheduler definition", []any{pathParameter("definitionID")}, nil, definitions),
		ActionSchedulerAuthoringContractGet: schedulerOperation("getSchedulerAuthoringContract", "Read Scheduler definition authoring rules", nil, nil, authoringContract),
		ActionSchedulerDefinitionsValidate: schedulerOperation("validateSchedulerDefinition", "Validate and preview a Scheduler job definition", nil,
			objectSchema(map[string]any{"data": map[string]any{"type": "object", "additionalProperties": true}}, "data"), preview),
		ActionSchedulerSchedulesPreview: schedulerOperation("previewSchedulerSchedule", "Validate and preview a schedule fragment", nil,
			map[string]any{"type": "object", "additionalProperties": true}, preview),
		ActionSchedulerDefinitionsSimulate: schedulerOperation("simulateSchedulerDefinition", "Simulate one published Scheduler definition without durable effects", []any{pathParameter("definitionID")}, nil, simulation),
		ActionSchedulerStateGet: schedulerOperation("getSchedulerOperationsState", "Inspect Scheduler-owned run and dead-letter state", nil, nil,
			objectSchema(map[string]any{"provisioned": map[string]any{"type": "boolean"}, "runs": arraySchema(run), "dead_letters": arraySchema(deadLetter)}, "provisioned", "runs", "dead_letters")),
		ActionSchedulerDefinitionsRun: schedulerOperation("runSchedulerDefinition", "Trigger one Scheduler definition now", commandParameters("definitionID"), nil, run),
		ActionSchedulerDefinitionsReschedule: schedulerOperation("rescheduleSchedulerDefinition", "Move the next run time of one Scheduler definition", commandParameters("definitionID"),
			objectSchema(map[string]any{"next_run_at": map[string]any{"type": "string", "format": "date-time"}}, "next_run_at"),
			objectSchema(map[string]any{"status": map[string]any{"type": "string"}, "definition_key": map[string]any{"type": "string"}, "next_run_at": map[string]any{"type": "string", "format": "date-time"}}, "status", "definition_key", "next_run_at")),
		ActionSchedulerRunsRetry:          schedulerOperation("retrySchedulerRun", "Retry one Scheduler run", commandParameters("runID"), nil, run),
		ActionSchedulerRunsCancel:         schedulerOperation("cancelSchedulerRun", "Cancel one Scheduler run", commandParameters("runID"), nil, run),
		ActionSchedulerDeadLettersResolve: schedulerOptionalRequestOperation("resolveSchedulerDeadLetter", "Resolve one Scheduler dead letter", commandParameters("deadLetterID"), note, deadLetter),
		ActionSchedulerDeadLettersRequeue: schedulerOptionalRequestOperation("requeueSchedulerDeadLetter", "Requeue one Scheduler dead letter", commandParameters("deadLetterID"), note, run),
	}
	routes := make([]HTTPRouteContract, 0, len(actions))
	operations := make(map[string]map[string]any, len(actions))
	for _, action := range actions {
		operation, found := operationsByAction[action.Key]
		if !found {
			return HTTPSurfaceContract{}, fmt.Errorf("Scheduler Action %q has no OpenAPI operation", action.Key)
		}
		route := HTTPRouteContract{Action: action}
		routes = append(routes, route)
		operations[route.Pattern()] = operation
		delete(operationsByAction, action.Key)
	}
	if len(operationsByAction) != 0 {
		return HTTPSurfaceContract{}, fmt.Errorf("Scheduler OpenAPI operations have no Action manifest entries")
	}
	return HTTPSurfaceContract{ContractVersion: SchedulerHTTPSurfaceContractVersion, Owner: "scheduler", Name: "scheduler_external", Routes: routes, OpenAPI: operations}, nil
}

func SchedulerAuthorizationActions() ([]actioncontract.ActionDefinition, error) {
	definitions := []actioncontract.ActionDefinition{
		schedulerRoute(ActionSchedulerDefinitionsList, CapabilitySchedulerAuthoring, "Scheduler authoring", "GET /tenant-admin/scheduler/definitions", actioncontract.EffectRead, "not_applicable", "scheduler_definition_read"),
		schedulerRoute(ActionSchedulerDefinitionsGet, CapabilitySchedulerAuthoring, "Scheduler authoring", "GET /tenant-admin/scheduler/definitions/{definitionID}", actioncontract.EffectRead, "not_applicable", "scheduler_definition_read"),
		schedulerRoute(ActionSchedulerAuthoringContractGet, CapabilitySchedulerAuthoring, "Scheduler authoring", "GET /tenant-admin/scheduler/authoring-contract", actioncontract.EffectRead, "not_applicable", "scheduler_definition_authoring"),
		schedulerRoute(ActionSchedulerDefinitionsValidate, CapabilitySchedulerAuthoring, "Scheduler authoring", "POST /tenant-admin/scheduler/definitions/validate", actioncontract.EffectRead, "not_applicable", "scheduler_definition_validation"),
		schedulerRoute(ActionSchedulerSchedulesPreview, CapabilitySchedulerAuthoring, "Scheduler authoring", "POST /tenant-admin/scheduler/schedules/preview", actioncontract.EffectRead, "not_applicable", "scheduler_schedule_preview"),
		schedulerRoute(ActionSchedulerDefinitionsSimulate, CapabilitySchedulerAuthoring, "Scheduler authoring", "POST /tenant-admin/scheduler/definitions/{definitionID}/simulate", actioncontract.EffectRead, "not_applicable", "scheduler_definition_simulation"),
		schedulerRoute(ActionSchedulerStateGet, CapabilitySchedulerOperations, "Scheduler operations", "GET /operations/scheduler/state", actioncontract.EffectRead, "not_applicable", "scheduler_operations_read"),
		schedulerRoute(ActionSchedulerDefinitionsRun, CapabilitySchedulerOperations, "Scheduler operations", "POST /operations/scheduler/definitions/{definitionID}/run", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason),
		schedulerRoute(ActionSchedulerDefinitionsReschedule, CapabilitySchedulerOperations, "Scheduler operations", "POST /operations/scheduler/definitions/{definitionID}/reschedule", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason),
		schedulerRoute(ActionSchedulerRunsRetry, CapabilitySchedulerOperations, "Scheduler operations", "POST /operations/scheduler/runs/{runID}/retry", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason),
		schedulerRoute(ActionSchedulerRunsCancel, CapabilitySchedulerOperations, "Scheduler operations", "POST /operations/scheduler/runs/{runID}/cancel", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason, actioncontract.ApprovalConfirmation),
		schedulerRoute(ActionSchedulerDeadLettersResolve, CapabilitySchedulerOperations, "Scheduler operations", "POST /operations/scheduler/dead-letters/{deadLetterID}/resolve", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason, actioncontract.ApprovalConfirmation),
		schedulerRoute(ActionSchedulerDeadLettersRequeue, CapabilitySchedulerOperations, "Scheduler operations", "POST /operations/scheduler/dead-letters/{deadLetterID}/requeue", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason, actioncontract.ApprovalConfirmation),
	}
	result := make([]actioncontract.ActionDefinition, 0, len(definitions))
	for _, definition := range definitions {
		normalized, err := actioncontract.NormalizeDefinition(definition)
		if err != nil {
			return nil, fmt.Errorf("normalize Scheduler Action %q: %w", definition.Key, err)
		}
		result = append(result, normalized)
	}
	return result, nil
}

func schedulerRoute(key, capabilityKey, capabilityLabel, pattern string, effect actioncontract.EffectClass, idempotency, audit string, approvals ...actioncontract.ApprovalPolicy) actioncontract.ActionDefinition {
	method, path, _ := strings.Cut(pattern, " ")
	separator := strings.LastIndex(key, ".")
	risk := actioncontract.RiskMedium
	if effect == actioncontract.EffectRead {
		risk = actioncontract.RiskLow
	}
	if len(approvals) != 0 {
		risk = actioncontract.RiskHigh
	}
	exposures := []actioncontract.Exposure{actioncontract.ExposureTenantAdmin, actioncontract.ExposureOps}
	if capabilityKey == CapabilitySchedulerAuthoring {
		exposures = append([]actioncontract.Exposure{actioncontract.ExposurePublic}, exposures...)
	}
	return actioncontract.ActionDefinition{
		Key: key, Owner: SchedulerAuthorizationOwner, SourceKind: "host_facade", CapabilityKey: capabilityKey, CapabilityLabel: capabilityLabel,
		OperationKey: key[separator+1:], OperationLabel: key, Label: key, Exposures: exposures,
		Authorization: actioncontract.Authorization{Strategy: actioncontract.AuthorizationExactRolePermission},
		HTTP:          &actioncontract.HTTPBinding{Method: method, RouteTemplate: path},
		Permission:    &actioncontract.PermissionDefinition{Key: key, Owner: SchedulerAuthorizationOwner, ResourceKey: key[:separator], OperationKey: key[separator+1:], Label: key, Category: capabilityLabel, LifecycleStatus: actioncontract.LifecycleActive},
		EffectClass:   effect, RiskLevel: risk, ApprovalPolicies: append([]actioncontract.ApprovalPolicy(nil), approvals...),
		IdempotencyDecision: idempotency, AuditClass: audit, LifecycleStatus: actioncontract.LifecycleActive,
	}
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

package schedulersdk

import (
	"errors"
	"fmt"
	"strings"

	actioncontract "github.com/domainry/domainry-foundation/action"
)

const SchedulerHTTPAdapterContractVersion = "domainry-scheduler-http-adapter-v1"

var ErrSchedulerHTTPAuthorizationBoundaryRequired = errors.New("Scheduler HTTP routes require a trusted human principal and Action authorization boundary")

// SchedulerHTTPAuthorizationBoundary is the host's explicit mount-time
// attestation. A private SaaS machine credential cannot satisfy this contract.
// The host must authenticate a human principal, authorize the exact Action
// permission, and enforce required reason/confirmation/idempotency evidence
// before calling the Scheduler adapter.
type SchedulerHTTPAuthorizationBoundary struct {
	TrustedHumanPrincipal  bool
	ActionPermissionGuard  bool
	OperationEvidenceGuard bool
}

func (b SchedulerHTTPAuthorizationBoundary) Validate() error {
	if !b.TrustedHumanPrincipal || !b.ActionPermissionGuard || !b.OperationEvidenceGuard {
		return ErrSchedulerHTTPAuthorizationBoundaryRequired
	}
	return nil
}

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

type HTTPAdapterContract struct {
	ContractVersion string                    `json:"contract_version"`
	Owner           string                    `json:"owner"`
	Name            string                    `json:"name"`
	Routes          []HTTPRouteContract       `json:"routes"`
	OpenAPI         map[string]map[string]any `json:"openapi_operations"`
}

// SchedulerHTTPAdapterContract is retained for source compatibility and fails
// closed. Call SchedulerHTTPAdapterContractForTrustedHost only at a Runtime HTTP
// composition point that actually installs the declared guards.
func SchedulerHTTPAdapterContract() (HTTPAdapterContract, error) {
	return HTTPAdapterContract{}, ErrSchedulerHTTPAuthorizationBoundaryRequired
}

// SchedulerHTTPAdapterContractForTrustedHost returns the source-owned Scheduler
// Action surface for a guarded Runtime listener. Standalone SaaS private
// protocol servers must not mount these /scheduler routes.
func SchedulerHTTPAdapterContractForTrustedHost(boundary SchedulerHTTPAuthorizationBoundary) (HTTPAdapterContract, error) {
	if err := boundary.Validate(); err != nil {
		return HTTPAdapterContract{}, err
	}
	contract, err := schedulerHTTPAdapterContract()
	if err != nil {
		return HTTPAdapterContract{}, err
	}
	for _, operation := range contract.OpenAPI {
		operation["x-domainry-host-authorization-boundary"] = "trusted_human_principal+action_permission+operation_evidence"
	}
	return contract, nil
}

// SchedulerHTTPOpenAPIOperations publishes source-owned interface metadata for
// capability discovery without attesting to a running host's authorization.
// It does not mount routes or replace SchedulerHTTPAdapterContractForTrustedHost.
func SchedulerHTTPOpenAPIOperations() (map[string]map[string]any, error) {
	contract, err := schedulerHTTPAdapterContract()
	if err != nil {
		return nil, err
	}
	return contract.OpenAPI, nil
}

func schedulerHTTPAdapterContract() (HTTPAdapterContract, error) {
	definitions := schedulerDefinitionHTTPSchema()
	authoringContract := objectSchema(map[string]any{
		"resource_type": map[string]any{"type": "string"}, "status_field": map[string]any{"type": "string"}, "allowed_statuses": arraySchema(map[string]any{"type": "string"}),
		"mutation_owner": map[string]any{"type": "string"}, "validation_endpoint": map[string]any{"type": "string"},
	}, "resource_type", "status_field", "allowed_statuses", "mutation_owner", "validation_endpoint")
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
		return HTTPAdapterContract{}, err
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
		ActionSchedulerDefinitionsRun: schedulerOperation("runSchedulerDefinition", "Trigger one Scheduler definition using Scheduler's real UTC clock", []any{pathParameter("definitionID")}, nil, run),
		ActionSchedulerDefinitionsReschedule: schedulerOperation("rescheduleSchedulerDefinition", "Move the next run time of one Scheduler definition", []any{pathParameter("definitionID")},
			objectSchema(map[string]any{"next_run_at": map[string]any{"type": "string", "format": "date-time"}}, "next_run_at"),
			objectSchema(map[string]any{"status": map[string]any{"type": "string"}, "definition_key": map[string]any{"type": "string"}, "next_run_at": map[string]any{"type": "string", "format": "date-time"}}, "status", "definition_key", "next_run_at")),
		ActionSchedulerRunsRetry:          schedulerOperation("retrySchedulerRun", "Retry one Scheduler run", []any{pathParameter("runID")}, nil, run),
		ActionSchedulerRunsCancel:         schedulerOperation("cancelSchedulerRun", "Cancel one Scheduler run", []any{pathParameter("runID")}, nil, run),
		ActionSchedulerDeadLettersResolve: schedulerOptionalRequestOperation("resolveSchedulerDeadLetter", "Resolve one Scheduler dead letter", []any{pathParameter("deadLetterID")}, note, deadLetter),
		ActionSchedulerDeadLettersRequeue: schedulerOptionalRequestOperation("requeueSchedulerDeadLetter", "Requeue one Scheduler dead letter", []any{pathParameter("deadLetterID")}, note, run),
	}
	routes := make([]HTTPRouteContract, 0, len(actions))
	operations := make(map[string]map[string]any, len(actions))
	for _, action := range actions {
		operation, found := operationsByAction[action.Key]
		if !found {
			return HTTPAdapterContract{}, fmt.Errorf("Scheduler Action %q has no OpenAPI operation", action.Key)
		}
		if err := applySchedulerOperationPrerequisites(operation, action); err != nil {
			return HTTPAdapterContract{}, err
		}
		route := HTTPRouteContract{Action: action}
		routes = append(routes, route)
		operations[route.Pattern()] = operation
		delete(operationsByAction, action.Key)
	}
	if len(operationsByAction) != 0 {
		return HTTPAdapterContract{}, fmt.Errorf("Scheduler OpenAPI operations have no Action manifest entries")
	}
	return HTTPAdapterContract{ContractVersion: SchedulerHTTPAdapterContractVersion, Owner: "scheduler", Name: "scheduler_guarded_runtime", Routes: routes, OpenAPI: operations}, nil
}

func SchedulerAuthorizationActions() ([]actioncontract.ActionDefinition, error) {
	definitions := []actioncontract.ActionDefinition{
		schedulerRoute(ActionSchedulerDefinitionsList, CapabilitySchedulerAuthoring, "Scheduler authoring", "GET /scheduler/definitions", actioncontract.EffectRead, "not_applicable", "scheduler_definition_read"),
		schedulerRoute(ActionSchedulerDefinitionsGet, CapabilitySchedulerAuthoring, "Scheduler authoring", "GET /scheduler/definitions/{definitionID}", actioncontract.EffectRead, "not_applicable", "scheduler_definition_read"),
		schedulerRoute(ActionSchedulerAuthoringContractGet, CapabilitySchedulerAuthoring, "Scheduler authoring", "GET /scheduler/authoring-contract", actioncontract.EffectRead, "not_applicable", "scheduler_definition_authoring"),
		schedulerRoute(ActionSchedulerDefinitionsValidate, CapabilitySchedulerAuthoring, "Scheduler authoring", "POST /scheduler/definitions/validate", actioncontract.EffectRead, "not_applicable", "scheduler_definition_validation"),
		schedulerRoute(ActionSchedulerSchedulesPreview, CapabilitySchedulerAuthoring, "Scheduler authoring", "POST /scheduler/schedules/preview", actioncontract.EffectRead, "not_applicable", "scheduler_schedule_preview"),
		schedulerRoute(ActionSchedulerDefinitionsSimulate, CapabilitySchedulerAuthoring, "Scheduler authoring", "POST /scheduler/definitions/{definitionID}/simulate", actioncontract.EffectRead, "not_applicable", "scheduler_definition_simulation"),
		schedulerRoute(ActionSchedulerStateGet, CapabilitySchedulerOperations, "Scheduler operations", "GET /scheduler/state", actioncontract.EffectRead, "not_applicable", "scheduler_operations_read"),
		schedulerRoute(ActionSchedulerDefinitionsRun, CapabilitySchedulerOperations, "Scheduler operations", "POST /scheduler/definitions/{definitionID}/run", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason),
		schedulerRoute(ActionSchedulerDefinitionsReschedule, CapabilitySchedulerOperations, "Scheduler operations", "POST /scheduler/definitions/{definitionID}/reschedule", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason),
		schedulerRoute(ActionSchedulerRunsRetry, CapabilitySchedulerOperations, "Scheduler operations", "POST /scheduler/runs/{runID}/retry", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason),
		schedulerRoute(ActionSchedulerRunsCancel, CapabilitySchedulerOperations, "Scheduler operations", "POST /scheduler/runs/{runID}/cancel", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason, actioncontract.ApprovalConfirmation),
		schedulerRoute(ActionSchedulerDeadLettersResolve, CapabilitySchedulerOperations, "Scheduler operations", "POST /scheduler/dead-letters/{deadLetterID}/resolve", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason, actioncontract.ApprovalConfirmation),
		schedulerRoute(ActionSchedulerDeadLettersRequeue, CapabilitySchedulerOperations, "Scheduler operations", "POST /scheduler/dead-letters/{deadLetterID}/requeue", actioncontract.EffectWrite, "caller_key_required", "scheduler_owner_command", actioncontract.ApprovalReason, actioncontract.ApprovalConfirmation),
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
	exposures := []actioncontract.Exposure{actioncontract.ExposureManagement, actioncontract.ExposureOps}
	if capabilityKey == CapabilitySchedulerAuthoring {
		exposures = append([]actioncontract.Exposure{actioncontract.ExposurePublic}, exposures...)
	}
	return actioncontract.ActionDefinition{
		Key: key, Owner: SchedulerAuthorizationOwner, SourceKind: "service_protocol", CapabilityKey: capabilityKey, CapabilityLabel: capabilityLabel,
		OperationKey: key[separator+1:], OperationLabel: key, Label: key, Exposures: exposures,
		Authorization: actioncontract.Authorization{Strategy: actioncontract.AuthorizationAuthenticated},
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

func applySchedulerOperationPrerequisites(operation map[string]any, action actioncontract.ActionDefinition) error {
	if operation == nil {
		return fmt.Errorf("Scheduler Action %q has no OpenAPI operation", action.Key)
	}
	parameters, err := schedulerOperationParameters(operation, action.Key)
	if err != nil {
		return err
	}
	// Keep transport prerequisites as a deterministic projection of the
	// normalized Action instead of maintaining a second per-operation table.
	if action.IdempotencyDecision == "caller_key_required" {
		parameters = append(parameters, requiredSchedulerHeaderParameter(
			"Idempotency-Key",
			"Stable caller-supplied key for one logical operation and all of its retries",
			map[string]any{"type": "string", "minLength": 1},
			"",
		))
	}
	if schedulerActionHasApprovalPolicy(action, actioncontract.ApprovalReason) {
		parameters = append(parameters, requiredSchedulerHeaderParameter(
			"X-Operation-Reason",
			"Human-supplied auditable reason for this governed Scheduler operation",
			map[string]any{"type": "string", "minLength": 1, "pattern": `\S`},
			"Approved Scheduler operation for the stated business purpose",
		))
	}
	if schedulerActionHasApprovalPolicy(action, actioncontract.ApprovalConfirmation) {
		parameters = append(parameters, requiredSchedulerHeaderParameter(
			"X-Operation-Confirmation",
			"Explicit confirmation required by the Scheduler Action contract",
			map[string]any{"type": "string", "enum": []string{"confirmed"}},
			"confirmed",
		))
	}
	if len(parameters) == 0 {
		delete(operation, "parameters")
		return nil
	}
	operation["parameters"] = parameters
	return nil
}

func schedulerOperationParameters(operation map[string]any, actionKey string) ([]any, error) {
	values, found := operation["parameters"]
	if !found {
		return nil, nil
	}
	parameters, ok := values.([]any)
	if !ok {
		return nil, fmt.Errorf("Scheduler Action %q OpenAPI parameters are invalid", actionKey)
	}
	return append([]any(nil), parameters...), nil
}

func requiredSchedulerHeaderParameter(name, description string, schema map[string]any, example string) map[string]any {
	parameter := map[string]any{
		"name": name, "in": "header", "required": true, "description": description, "schema": schema,
	}
	if example != "" {
		parameter["example"] = example
	}
	return parameter
}

func schedulerActionHasApprovalPolicy(action actioncontract.ActionDefinition, wanted actioncontract.ApprovalPolicy) bool {
	for _, policy := range action.ApprovalPolicies {
		if policy == wanted {
			return true
		}
	}
	return false
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
		"scheduled_for": map[string]any{"type": "string", "format": "date-time"}, "window_key": map[string]any{"type": "string"}, "error_message": map[string]any{"type": "string"}, "lease_owner": map[string]any{"type": "string"},
		"lease_expires_at": map[string]any{"type": "string", "format": "date-time"}, "fencing_token": map[string]any{"type": "integer"}, "correlation_id": map[string]any{"type": "string"},
		"downstream_receipt": objectSchema(map[string]any{"id": map[string]any{"type": "string"}, "owner": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "replay": map[string]any{"type": "boolean"}}, "id", "status"),
		"created_at":         map[string]any{"type": "string", "format": "date-time"}, "updated_at": map[string]any{"type": "string", "format": "date-time"},
	}, "id", "status")
}

func schedulerDeadLetterHTTPSchema() map[string]any {
	return objectSchema(map[string]any{
		"id": map[string]any{"type": "string"}, "run_id": map[string]any{"type": "string"}, "definition_key": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"},
		"reason": map[string]any{"type": "string"}, "failed_at": map[string]any{"type": "string", "format": "date-time"}, "resolved_at": map[string]any{"type": "string", "format": "date-time"},
	}, "id", "run_id", "status")
}

func schedulerDefinitionHTTPSchema() map[string]any {
	properties := map[string]any{}
	for _, name := range []string{
		"key", "name", "status", "schedule_type", "schedule_expression", "time_of_day", "day_of_week", "timezone", "target_type", "target_key", "target_object", "run_as_role", "connection_key",
		"missed_window_policy", "payload_json", "description", "next_run_at", "created_at", "updated_at",
	} {
		properties[name] = map[string]any{"type": "string"}
	}
	for _, name := range []string{"interval_seconds", "day_of_month", "max_attempts", "timeout_seconds", "max_catchup_windows", "retry_delay_seconds", "retry_max_delay_seconds"} {
		properties[name] = map[string]any{"type": "integer"}
	}
	properties["i18n"] = map[string]any{"type": "object", "additionalProperties": true}
	return objectSchema(properties, "key", "status")
}

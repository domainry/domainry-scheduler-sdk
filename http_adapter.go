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
	ContractVersion string              `json:"contract_version"`
	Owner           string              `json:"owner"`
	Name            string              `json:"name"`
	Routes          []HTTPRouteContract `json:"routes"`
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
	return schedulerHTTPAdapterContract()
}

func schedulerHTTPAdapterContract() (HTTPAdapterContract, error) {
	actions, err := SchedulerAuthorizationActions()
	if err != nil {
		return HTTPAdapterContract{}, err
	}
	routes := make([]HTTPRouteContract, 0, len(actions))
	for _, action := range actions {
		routes = append(routes, HTTPRouteContract{Action: action})
	}
	return HTTPAdapterContract{ContractVersion: SchedulerHTTPAdapterContractVersion, Owner: "scheduler", Name: "scheduler_guarded_runtime", Routes: routes}, nil
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

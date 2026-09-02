package authoring

import (
	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
	capabilitycontract "github.com/domainry/domainry-scheduler-sdk/authoring/contract"
)

const exactActionPermissionModel = "exact_action_same_key_permission"

// SchedulerAuthoringDomain publishes Scheduler-owned definition and command contracts.
// Persisted definitions, compositional schedule fragments, and runtime commands are
// deliberately separate capabilities because they have different HTTP envelopes.
func Domain() capabilitycontract.CapabilityAuthoringDomain {
	return capabilitycontract.CapabilityAuthoringDomain{Key: "scheduler", Capabilities: []capabilitycontract.CapabilityAuthoringDefinition{
		schedulerBusinessJobAuthoringCapability(),
		schedulerScheduleAuthoringCapability(),
		schedulerCommandAuthoringCapability("scheduler.job.simulate", schedulersdk.ActionSchedulerDefinitionsSimulate, "business_schedule_simulation", "definition_id", "POST /tenant-admin/scheduler/definitions/{definitionID}/simulate", false),
		schedulerCommandAuthoringCapability("scheduler.job.run", schedulersdk.ActionSchedulerDefinitionsRun, "business_schedule_execution", "definition_id", "POST /operations/scheduler/definitions/{definitionID}/run", true),
		schedulerCommandAuthoringCapability("scheduler.run.retry", schedulersdk.ActionSchedulerRunsRetry, "business_administrator_recovery", "run_id", "POST /operations/scheduler/runs/{runID}/retry", true),
		schedulerCommandAuthoringCapability("scheduler.run.cancel", schedulersdk.ActionSchedulerRunsCancel, "business_administrator_recovery", "run_id", "POST /operations/scheduler/runs/{runID}/cancel", true),
		schedulerResolveDeadLetterAuthoringCapability(),
	}}
}

func schedulerBusinessJobAuthoringCapability() capabilitycontract.CapabilityAuthoringDefinition {
	parameters := schedulerBusinessJobParameters()
	return capabilitycontract.CapabilityAuthoringDefinition{
		Key: "scheduler.business_job", Status: "supported", Lifecycle: "business_schedule",
		SystemDraftResourceType: "scheduler",
		Parameters:              parameters, Requires: []string{"scheduler.schedule"}, Permissions: []string{
			schedulersdk.ActionSchedulerDefinitionsGet,
			schedulersdk.ActionSchedulerDefinitionsValidate,
		},
		ValidationEndpoint: "POST /tenant-admin/scheduler/definitions/validate", PreviewEndpoint: "POST /tenant-admin/scheduler/definitions/validate",
		ConfigurationRoutes: []string{"GET /tenant-admin/metadata/definitions/scheduler/{resourceKey}", "POST /tenant-admin/metadata/definitions/scheduler/{resourceKey}/validate", "GET /domain-system-snapshot", "GET /domain-reference-graph", "POST /tenant-admin/scheduler/definitions/validate", "GET /tenant-admin/scheduler/definitions/{definitionID}"}, ResourceKeyPathParameter: "definitionID",
		InputSchema: schedulerObjectSchema(parameters), OutputSchema: schedulerJobRecordOutputSchema(),
		OutputVariables:    []capabilitycontract.CapabilityAuthoringOutput{{Name: "definition_id", JSONPointer: "/id", Type: "record_id", VisibleTo: "subsequent_capability_calls"}, {Name: "job_key", JSONPointer: "/data/key", Type: "scheduler_job_key", VisibleTo: "subsequent_capability_calls"}},
		ReferenceContracts: []capabilitycontract.CapabilityAuthoringReference{{Kind: "scheduler_target_key", InputJSONPointer: "/target_key", ScopeFrom: "/target_type", ResolverEndpoint: "/tenant-admin/platform-capabilities/references/scheduler_target_key"}, {Kind: "object_key", InputJSONPointer: "/target_object", ResolverEndpoint: "/tenant-admin/platform-capabilities/references/object_key"}, {Kind: "role_key", InputJSONPointer: "/run_as_role", ResolverEndpoint: "/tenant-admin/platform-capabilities/references/role_key"}},
		Execution:          &capabilitycontract.CapabilityAuthoringExecution{ReadSet: []string{"metadata.scheduler_definition", "workflow.definition", "report.definition"}, Transaction: "read_only_candidate_validation", Idempotency: "naturally_idempotent_at_candidate_hash", SideEffectLevel: "none", PermissionModel: exactActionPermissionModel, ChangeControl: "source_controlled_json"},
		Errors:             schedulerDefinitionAuthoringErrors(), Examples: schedulerBusinessJobExamples(),
		Sources: []capabilitycontract.CapabilityAuthoringSource{
			{Kind: "owner_sdk", Path: "github.com/domainry/domainry-scheduler-sdk/schedule/authoring.go", Symbol: "ValidateDefinitionData"},
			{Kind: "owner_sdk", Path: "github.com/domainry/domainry-scheduler-sdk/schedule/preview.go", Symbol: "PreviewDefinitionData"},
		},
	}
}

func schedulerScheduleAuthoringCapability() capabilitycontract.CapabilityAuthoringDefinition {
	parameters := schedulerScheduleParameters()
	return capabilitycontract.CapabilityAuthoringDefinition{
		Key: "scheduler.schedule", Status: "supported", Lifecycle: "definition_fragment", Permissions: []string{schedulersdk.ActionSchedulerSchedulesPreview},
		Parameters: parameters, ValidationEndpoint: "POST /tenant-admin/scheduler/schedules/preview", PreviewEndpoint: "POST /tenant-admin/scheduler/schedules/preview", InputSchema: schedulerObjectSchema(parameters), OutputSchema: schedulerPreviewOutputSchema(),
		OutputVariables: []capabilitycontract.CapabilityAuthoringOutput{{Name: "next_runs", JSONPointer: "/next_runs", Type: "date_time_list", VisibleTo: "subsequent_capability_calls"}},
		Execution:       &capabilitycontract.CapabilityAuthoringExecution{ReadSet: []string{"metadata.scheduler_definition"}, Transaction: "read_only_preview", Idempotency: "naturally_idempotent", SideEffectLevel: "none", PermissionModel: exactActionPermissionModel},
		Errors:          schedulerDefinitionAuthoringErrors(), Examples: schedulerScheduleExamples(),
		Sources: []capabilitycontract.CapabilityAuthoringSource{
			{Kind: "owner_sdk", Path: "github.com/domainry/domainry-scheduler-sdk/schedule/authoring.go", Symbol: "ValidateData"},
			{Kind: "owner_sdk", Path: "github.com/domainry/domainry-scheduler-sdk/schedule/preview.go", Symbol: "PreviewData"},
		},
	}
}

func schedulerCommandAuthoringCapability(key, actionKey, lifecycle, resourceParameter, route string, idempotencyRequired bool) capabilitycontract.CapabilityAuthoringDefinition {
	parameters := []capabilitycontract.CapabilityAuthoringParameter{{Key: resourceParameter, Type: "record_id", Required: true}}
	if idempotencyRequired {
		parameters = append(parameters, capabilitycontract.CapabilityAuthoringParameter{Key: "idempotency_key", Type: "string", Required: true})
	}
	capability := capabilitycontract.CapabilityAuthoringDefinition{
		Key: key, Status: "supported", Lifecycle: lifecycle, Parameters: parameters, Requires: []string{"scheduler.business_job"}, Permissions: []string{actionKey},
		ConfigurationRoutes: []string{route}, InputSchema: schedulerObjectSchema(parameters), OutputSchema: schedulerOperationOutputSchema(),
		OutputVariables: []capabilitycontract.CapabilityAuthoringOutput{{Name: "status", JSONPointer: "/status", Type: "string", VisibleTo: "subsequent_capability_calls"}, {Name: "run", JSONPointer: "/run", Type: "scheduler_run", VisibleTo: "subsequent_capability_calls"}},
		Execution:       &capabilitycontract.CapabilityAuthoringExecution{ReadSet: []string{"metadata.scheduler_definition", "scheduler_service.schedule", "scheduler_service.run"}, WriteSet: []string{"scheduler_service.run", "scheduler_service.dead_letter"}, Transaction: "scheduler_owner_operation", Idempotency: "idempotency_key", SideEffects: []string{"scheduler_operation_audit"}, SideEffectLevel: "external", Compensation: "issue an explicit owner retry, cancel, resolve, or reschedule command; never roll back Scheduler evidence", PermissionModel: exactActionPermissionModel},
		Examples:        schedulerCommandExamples(resourceParameter, idempotencyRequired),
		Sources:         schedulerCommandSources(key),
	}
	if key == "scheduler.job.simulate" {
		capability.SimulationEndpoint = route
		capability.Execution.WriteSet = nil
		capability.Execution.Idempotency = "naturally_idempotent"
		capability.Execution.SideEffectLevel = "none"
	}
	return capability
}

func schedulerResolveDeadLetterAuthoringCapability() capabilitycontract.CapabilityAuthoringDefinition {
	parameters := []capabilitycontract.CapabilityAuthoringParameter{{Key: "dead_letter_id", Type: "record_id", Required: true}, {Key: "idempotency_key", Type: "string", Required: true}, {Key: "note", Type: "string"}}
	capability := schedulerCommandAuthoringCapability("scheduler.dead_letter.resolve", schedulersdk.ActionSchedulerDeadLettersResolve, "business_administrator_recovery", "dead_letter_id", "POST /operations/scheduler/dead-letters/{deadLetterID}/resolve", true)
	capability.Parameters = parameters
	capability.InputSchema = schedulerObjectSchema(parameters)
	capability.Examples = []capabilitycontract.CapabilityAuthoringExample{
		{Name: "minimal_valid", Value: map[string]any{"dead_letter_id": "deadletter_01", "idempotency_key": "resolve-deadletter-01"}},
		{Name: "representative", Value: map[string]any{"dead_letter_id": "deadletter_02", "idempotency_key": "resolve-deadletter-02", "note": "Failure cause corrected"}},
		{Name: "invalid_with_repair", Value: map[string]any{"dead_letter_id": "", "idempotency_key": ""}, ExpectedErrorCodes: []string{"backend.scheduler.dead_letter_not_found"}},
	}
	return capability
}

func schedulerCommandSources(key string) []capabilitycontract.CapabilityAuthoringSource {
	symbol := "Binding.ResolveDeadLetter"
	switch key {
	case "scheduler.job.simulate":
		symbol = "Binding.Preview"
	case "scheduler.job.run":
		symbol = "Binding.TriggerNow"
	case "scheduler.run.retry":
		symbol = "Binding.RetryRun"
	case "scheduler.run.cancel":
		symbol = "Binding.CancelRun"
	}
	return []capabilitycontract.CapabilityAuthoringSource{{Kind: "owner_sdk", Path: "github.com/domainry/domainry-scheduler-sdk/sdk.go", Symbol: symbol}}
}

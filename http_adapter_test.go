package schedulersdk

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	actioncontract "github.com/domainry/domainry-foundation/action"
)

func TestSchedulerHTTPAdapterOwnsEveryExternalRuntimeFacadeRoute(t *testing.T) {
	if _, err := SchedulerHTTPAdapterContract(); !errors.Is(err, ErrSchedulerHTTPAuthorizationBoundaryRequired) {
		t.Fatalf("unguarded contract err=%v", err)
	}
	if _, err := SchedulerHTTPAdapterContractForTrustedHost(SchedulerHTTPAuthorizationBoundary{TrustedHumanPrincipal: true, ActionPermissionGuard: true}); !errors.Is(err, ErrSchedulerHTTPAuthorizationBoundaryRequired) {
		t.Fatalf("partial guard err=%v", err)
	}
	contract, err := guardedSchedulerHTTPAdapterContract()
	if err != nil {
		t.Fatal(err)
	}
	if contract.Owner != "scheduler" || contract.ContractVersion != SchedulerHTTPAdapterContractVersion || len(contract.Routes) != 13 || len(contract.OpenAPI) != len(contract.Routes) {
		t.Fatalf("Scheduler HTTP contract=%+v", contract)
	}
	seen := map[string]bool{}
	for _, route := range contract.Routes {
		pattern := route.Pattern()
		if seen[pattern] || contract.OpenAPI[pattern]["operationId"] == nil || route.Action.Permission == nil || route.Action.Permission.Key != route.Action.Key {
			t.Fatalf("incomplete Scheduler route %q", pattern)
		}
		seen[pattern] = true
	}
	actions, err := SchedulerAuthorizationActions()
	if err != nil {
		t.Fatal(err)
	}
	registry := actioncontract.NewRegistry()
	if err := registry.Register(actions...); err != nil {
		t.Fatal(err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatal(err)
	}
	if len(actions) != len(contract.Routes) || len(registry.PermissionDefinitions()) != len(actions) {
		t.Fatalf("Actions=%d routes=%d Permissions=%d", len(actions), len(contract.Routes), len(registry.PermissionDefinitions()))
	}
}

func TestSchedulerDefinitionRunOpenAPIPublishesOnlyOrdinaryTriggerNow(t *testing.T) {
	contract, err := guardedSchedulerHTTPAdapterContract()
	if err != nil {
		t.Fatal(err)
	}
	operation := contract.OpenAPI["POST /scheduler/definitions/{definitionID}/run"]
	if operation["requestBody"] != nil || operation["x-domainry-host-authorization-boundary"] != "trusted_human_principal+action_permission+operation_evidence" {
		t.Fatalf("run operation=%#v", operation)
	}
	response := operation["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	runProperties := response["properties"].(map[string]any)
	if runProperties["window_key"] == nil || runProperties["downstream_receipt"] == nil {
		t.Fatalf("run response properties=%#v", runProperties)
	}
}

func TestSchedulerOpenAPIOperationPrerequisitesStayAlignedWithEveryAction(t *testing.T) {
	contract, err := guardedSchedulerHTTPAdapterContract()
	if err != nil {
		t.Fatal(err)
	}
	if len(contract.Routes) != 13 {
		t.Fatalf("Scheduler routes=%d want=13", len(contract.Routes))
	}
	for _, route := range contract.Routes {
		action := route.Action
		parameters := schedulerOpenAPIParameters(t, contract.OpenAPI[route.Pattern()])
		checks := []struct {
			name string
			want bool
		}{
			{name: "Idempotency-Key", want: action.IdempotencyDecision == "caller_key_required"},
			{name: "X-Operation-Reason", want: schedulerActionHasApprovalPolicy(action, actioncontract.ApprovalReason)},
			{name: "X-Operation-Confirmation", want: schedulerActionHasApprovalPolicy(action, actioncontract.ApprovalConfirmation)},
		}
		for _, check := range checks {
			parameter := schedulerOpenAPIParameter(parameters, check.name)
			if (parameter != nil) != check.want {
				t.Fatalf("Action %s metadata/header drift for %s: want=%t parameter=%#v", action.Key, check.name, check.want, parameter)
			}
			if parameter != nil && (parameter["in"] != "header" || parameter["required"] != true) {
				t.Fatalf("Action %s prerequisite %s=%#v", action.Key, check.name, parameter)
			}
		}
	}
}

func TestSchedulerOpenAPIPublishesExactGovernedCommandHeaders(t *testing.T) {
	contract, err := guardedSchedulerHTTPAdapterContract()
	if err != nil {
		t.Fatal(err)
	}
	wantHeaders := map[string][]string{
		ActionSchedulerDefinitionsRun:        {"Idempotency-Key", "X-Operation-Reason"},
		ActionSchedulerDefinitionsReschedule: {"Idempotency-Key", "X-Operation-Reason"},
		ActionSchedulerRunsRetry:             {"Idempotency-Key", "X-Operation-Reason"},
		ActionSchedulerRunsCancel:            {"Idempotency-Key", "X-Operation-Reason", "X-Operation-Confirmation"},
		ActionSchedulerDeadLettersResolve:    {"Idempotency-Key", "X-Operation-Reason", "X-Operation-Confirmation"},
		ActionSchedulerDeadLettersRequeue:    {"Idempotency-Key", "X-Operation-Reason", "X-Operation-Confirmation"},
	}
	for _, route := range contract.Routes {
		parameters := schedulerOpenAPIParameters(t, contract.OpenAPI[route.Pattern()])
		var gotHeaders []string
		for _, parameter := range parameters {
			if parameter["in"] == "header" {
				gotHeaders = append(gotHeaders, parameter["name"].(string))
			}
		}
		if !reflect.DeepEqual(gotHeaders, wantHeaders[route.Action.Key]) {
			t.Fatalf("Action %s headers=%v want=%v", route.Action.Key, gotHeaders, wantHeaders[route.Action.Key])
		}
	}

	run := schedulerOpenAPIParameters(t, contract.OpenAPI["POST /scheduler/definitions/{definitionID}/run"])
	if got := schedulerOpenAPIParameterNames(run); !reflect.DeepEqual(got, []string{"definitionID", "Idempotency-Key", "X-Operation-Reason"}) {
		t.Fatalf("run parameter order=%v", got)
	}
	for pattern, want := range map[string][]string{
		"POST /scheduler/runs/{runID}/cancel":                 {"runID", "Idempotency-Key", "X-Operation-Reason", "X-Operation-Confirmation"},
		"POST /scheduler/dead-letters/{deadLetterID}/resolve": {"deadLetterID", "Idempotency-Key", "X-Operation-Reason", "X-Operation-Confirmation"},
		"POST /scheduler/dead-letters/{deadLetterID}/requeue": {"deadLetterID", "Idempotency-Key", "X-Operation-Reason", "X-Operation-Confirmation"},
	} {
		parameters := schedulerOpenAPIParameters(t, contract.OpenAPI[pattern])
		if got := schedulerOpenAPIParameterNames(parameters); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s parameter order=%v want=%v", pattern, got, want)
		}
	}

	idempotency := schedulerOpenAPIParameter(run, "Idempotency-Key")
	if idempotency["required"] != true || idempotency["schema"].(map[string]any)["minLength"] != 1 {
		t.Fatalf("idempotency parameter=%#v", idempotency)
	}
	reason := schedulerOpenAPIParameter(run, "X-Operation-Reason")
	if example, _ := reason["example"].(string); strings.TrimSpace(example) == "" {
		t.Fatalf("reason example=%#v", reason["example"])
	}
	cancel := schedulerOpenAPIParameters(t, contract.OpenAPI["POST /scheduler/runs/{runID}/cancel"])
	confirmation := schedulerOpenAPIParameter(cancel, "X-Operation-Confirmation")
	if confirmation["example"] != "confirmed" || !reflect.DeepEqual(confirmation["schema"].(map[string]any)["enum"], []string{"confirmed"}) {
		t.Fatalf("confirmation parameter=%#v", confirmation)
	}
}

func guardedSchedulerHTTPAdapterContract() (HTTPAdapterContract, error) {
	return SchedulerHTTPAdapterContractForTrustedHost(SchedulerHTTPAuthorizationBoundary{
		TrustedHumanPrincipal: true, ActionPermissionGuard: true, OperationEvidenceGuard: true,
	})
}

func TestSchedulerOpenAPIDisclosurePreservesOperationsWithoutHostAttestation(t *testing.T) {
	operations, err := SchedulerHTTPOpenAPIOperations()
	if err != nil {
		t.Fatal(err)
	}
	guarded, err := guardedSchedulerHTTPAdapterContract()
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != len(guarded.OpenAPI) {
		t.Fatalf("disclosed operations=%d guarded operations=%d", len(operations), len(guarded.OpenAPI))
	}
	for pattern, operation := range guarded.OpenAPI {
		if _, attested := operations[pattern]["x-domainry-host-authorization-boundary"]; attested {
			t.Fatalf("metadata discovery attested a host boundary for %s", pattern)
		}
		delete(operation, "x-domainry-host-authorization-boundary")
		if !reflect.DeepEqual(operations[pattern], operation) {
			t.Fatalf("disclosure lost source-owned request/response or governance metadata for %s", pattern)
		}
	}
	if _, err := SchedulerHTTPAdapterContractForTrustedHost(SchedulerHTTPAuthorizationBoundary{}); !errors.Is(err, ErrSchedulerHTTPAuthorizationBoundaryRequired) {
		t.Fatalf("metadata discovery bypassed the host guard requirement: %v", err)
	}
}

func schedulerOpenAPIParameters(t *testing.T, operation map[string]any) []map[string]any {
	t.Helper()
	if operation == nil {
		t.Fatal("OpenAPI operation is absent")
	}
	values, _ := operation["parameters"].([]any)
	parameters := make([]map[string]any, 0, len(values))
	for _, value := range values {
		parameter, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("OpenAPI parameter=%#v", value)
		}
		parameters = append(parameters, parameter)
	}
	return parameters
}

func schedulerOpenAPIParameter(parameters []map[string]any, name string) map[string]any {
	for _, parameter := range parameters {
		if parameter["in"] == "header" && parameter["name"] == name {
			return parameter
		}
	}
	return nil
}

func schedulerOpenAPIParameterNames(parameters []map[string]any) []string {
	names := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		names = append(names, parameter["name"].(string))
	}
	return names
}

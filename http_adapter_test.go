package schedulersdk

import (
	"errors"
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
	if contract.Owner != "scheduler" || contract.ContractVersion != SchedulerHTTPAdapterContractVersion || len(contract.Routes) != 13 {
		t.Fatalf("Scheduler HTTP contract=%+v", contract)
	}
	seen := map[string]bool{}
	for _, route := range contract.Routes {
		pattern := route.Pattern()
		if seen[pattern] || route.Action.Permission == nil || route.Action.Permission.Key != route.Action.Key {
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

func TestSchedulerGovernedCommandsDeclareExecutablePrerequisites(t *testing.T) {
	actions, err := SchedulerAuthorizationActions()
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range actions {
		command := action.IdempotencyDecision == "caller_key_required"
		if command != hasApprovalPolicy(action, actioncontract.ApprovalReason) {
			t.Fatalf("Action %s reason policy does not match command status", action.Key)
		}
		confirmation := hasApprovalPolicy(action, actioncontract.ApprovalConfirmation)
		switch action.Key {
		case ActionSchedulerRunsCancel, ActionSchedulerDeadLettersResolve, ActionSchedulerDeadLettersRequeue:
			if !confirmation {
				t.Fatalf("Action %s must require confirmation", action.Key)
			}
		default:
			if confirmation {
				t.Fatalf("Action %s unexpectedly requires confirmation", action.Key)
			}
		}
	}
}

func guardedSchedulerHTTPAdapterContract() (HTTPAdapterContract, error) {
	return SchedulerHTTPAdapterContractForTrustedHost(SchedulerHTTPAuthorizationBoundary{
		TrustedHumanPrincipal: true, ActionPermissionGuard: true, OperationEvidenceGuard: true,
	})
}

func hasApprovalPolicy(action actioncontract.ActionDefinition, wanted actioncontract.ApprovalPolicy) bool {
	for _, policy := range action.ApprovalPolicies {
		if policy == wanted {
			return true
		}
	}
	return false
}

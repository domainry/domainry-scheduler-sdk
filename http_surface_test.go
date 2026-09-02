package schedulersdk

import (
	"testing"

	actioncontract "github.com/domainry/domainry-foundation/action"
)

func TestSchedulerHTTPSurfaceOwnsEveryExternalRuntimeFacadeRoute(t *testing.T) {
	contract, err := SchedulerHTTPSurfaceContract()
	if err != nil {
		t.Fatal(err)
	}
	if contract.Owner != "scheduler" || contract.ContractVersion != SchedulerHTTPSurfaceContractVersion || len(contract.Routes) != 13 || len(contract.OpenAPI) != len(contract.Routes) {
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

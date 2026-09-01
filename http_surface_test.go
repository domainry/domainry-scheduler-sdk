package schedulersdk

import "testing"

func TestSchedulerHTTPSurfaceOwnsEveryExternalRuntimeFacadeRoute(t *testing.T) {
	contract := SchedulerHTTPSurfaceContract()
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
}

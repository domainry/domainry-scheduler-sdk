package schedulersdk

import "testing"

func TestSchedulerHTTPSurfaceOwnsEveryExternalRuntimeFacadeRoute(t *testing.T) {
	contract := SchedulerHTTPSurfaceContract()
	if contract.Owner != "scheduler" || contract.ContractVersion != SchedulerHTTPSurfaceContractVersion || len(contract.Routes) != 13 || len(contract.OpenAPI) != len(contract.Routes) {
		t.Fatalf("Scheduler HTTP contract=%+v", contract)
	}
	seen := map[string]bool{}
	for _, route := range contract.Routes {
		if seen[route.Pattern] || contract.OpenAPI[route.Pattern]["operationId"] == nil {
			t.Fatalf("incomplete Scheduler route %q", route.Pattern)
		}
		seen[route.Pattern] = true
	}
}

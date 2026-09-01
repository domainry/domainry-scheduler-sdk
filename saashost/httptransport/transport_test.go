package httptransport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/domainry/domainry-foundation/modulecapability"
	"github.com/domainry/domainry-foundation/modulecapability/contracttest"
	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

func TestTransportPublishesDefinitionSnapshotWithAuthentication(t *testing.T) {
	fixture, err := contracttest.NewFixtureBinding("scheduler")
	if err != nil {
		t.Fatal(err)
	}
	summary, err := fixture.CapabilitySummary(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	capability, err := modulecapability.NewHTTPHandler(fixture, func(request *http.Request) error {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("capability auth=%q", request.Header.Get("Authorization"))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(modulecapability.SummaryPath, capability)
	mux.Handle(modulecapability.CategoriesPath, capability)
	mux.Handle(modulecapability.ValidationPath, capability)
	mux.HandleFunc("PUT /v1/applications/runtime-a/definitions", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/v1/applications/runtime-a/definitions" || request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("request=%s %s auth=%q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
		}
		var snapshot schedulersdk.DefinitionSnapshot
		if err := json.NewDecoder(request.Body).Decode(&snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.Revision != 7 || len(snapshot.Definitions) != 1 {
			t.Fatalf("snapshot=%#v", snapshot)
		}
		response.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	transport, err := Open(t.Context(), Config{Endpoint: server.URL, Token: "secret", Client: server.Client(), CapabilityContractSHA256: summary.Identity.ContractSHA256})
	if err != nil {
		t.Fatal(err)
	}
	contracttest.VerifyBinding(t, transport)
	err = transport.Reconcile(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, schedulersdk.DefinitionSnapshot{Revision: 7, Definitions: []schedulersdk.Definition{{Key: "daily"}}})
	if err != nil {
		t.Fatal(err)
	}
}

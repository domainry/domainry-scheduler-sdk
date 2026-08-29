package httptransport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

func TestTransportPublishesDefinitionSnapshotWithAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
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
	}))
	defer server.Close()
	transport, err := New(Config{Endpoint: server.URL, Token: "secret", Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	err = transport.Reconcile(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, schedulersdk.DefinitionSnapshot{Revision: 7, Definitions: []schedulersdk.Definition{{Key: "daily"}}})
	if err != nil {
		t.Fatal(err)
	}
}

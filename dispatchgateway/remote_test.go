package dispatchgateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

func TestRemoteDispatchAuthenticatesAndValidatesReceipt(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls++
		if request.URL.Path != AcceptPath || request.Header.Get("X-Domainry-Service-Credential") != "secret" || request.Header.Get("X-Domainry-Runtime-ID") != "runtime-a" {
			http.Error(response, "bad request", http.StatusBadRequest)
			return
		}
		if calls == 1 {
			http.Error(response, "retry", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(response).Encode(Receipt{RunID: "run-1", ID: "receipt-1", Owner: "workflow", Status: "accepted"})
	}))
	defer server.Close()
	remote, err := NewRemote(RemoteConfig{BaseURL: server.URL, ServiceCredential: "secret", HTTPClient: server.Client(), MaxAttempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	target := schedulersdk.TargetRef{Type: "runtime_operation", Owner: "workflow", Operation: "start", Payload: json.RawMessage(`{}`)}
	receipt, err := remote.Dispatch(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, Request{RuntimeID: "runtime-a", Trigger: schedulersdk.Trigger{RunID: "run-1", DefinitionKey: "definition-1", IdempotencyKey: "run-1", Target: target}})
	if err != nil || receipt.ID != "receipt-1" || calls != 2 {
		t.Fatalf("receipt=%#v calls=%d err=%v", receipt, calls, err)
	}
}

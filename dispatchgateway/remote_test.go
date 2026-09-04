package dispatchgateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

func TestRemoteDispatchAuthenticatesAndValidatesReceipt(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls++
		body, _ := io.ReadAll(request.Body)
		signature := Signature{
			ClientID: request.Header.Get(ClientIDHeader), Timestamp: request.Header.Get(TimestampHeader), Value: request.Header.Get(SignatureHeader),
		}
		if request.URL.Path != AcceptPath || request.Header.Get("X-Domainry-Runtime-ID") != "runtime-a" || request.Header.Get("X-Domainry-Service-Credential") != "" ||
			Verify(body, "run-1", signature, []byte("secret"), time.Now().UTC()) != nil {
			http.Error(response, "bad request", http.StatusBadRequest)
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		if calls == 1 {
			http.Error(response, "retry", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(response).Encode(Receipt{ExecutionID: "run-1", ID: "receipt-1", Owner: "workflow", Status: "accepted"})
	}))
	defer server.Close()
	remote, err := NewRemote(RemoteConfig{BaseURL: server.URL, SigningSecret: "secret", HTTPClient: server.Client(), MaxAttempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	target := schedulersdk.TargetRef{Type: "runtime_operation", Owner: "workflow", Operation: "start", Payload: json.RawMessage(`{}`)}
	receipt, err := remote.Dispatch(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, Request{RuntimeID: "runtime-a", ExecutionID: "run-1", IdempotencyKey: "run-1", Target: target})
	if err != nil || receipt.ID != "receipt-1" || calls != 2 {
		t.Fatalf("receipt=%#v calls=%d err=%v", receipt, calls, err)
	}
}

package dispatchgateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestRemoteDispatchAuthenticatesEveryCallbackDimensionAndRetries(t *testing.T) {
	var calls int
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(request.Body)
		signature := Signature{
			Version: request.Header.Get(SignatureVersionHeader), ClientID: request.Header.Get(ClientIDHeader),
			Timestamp: request.Header.Get(TimestampHeader), Value: request.Header.Get(SignatureHeader),
		}
		signed := SignedRequest{Method: request.Method, Path: request.URL.Path, RuntimeID: request.Header.Get(RuntimeIDHeader), IdempotencyKey: "command-1"}
		if request.URL.Path != AcceptPath || signed.RuntimeID != "runtime-a" || request.Header.Get("X-Domainry-Service-Credential") != "" ||
			VerifyRequest(body, signed, signature, []byte("secret"), time.Now().UTC()) != nil {
			t.Fatal("remote callback omitted a v2 signed request dimension")
		}
		if calls == 1 {
			return callbackHTTPResponse(http.StatusServiceUnavailable, `{"code":"dispatch.temporarily_unavailable"}`), nil
		}
		payload, _ := json.Marshal(Receipt{ExecutionID: "run-1", ID: "receipt-1", Owner: "workflow", Status: "accepted"})
		return callbackHTTPResponse(http.StatusOK, string(payload)), nil
	})}
	remote, err := NewRemote(RemoteConfig{BaseURL: "https://runtime.example", RuntimeID: "runtime-a", SigningSecret: "secret", HTTPClient: client, MaxAttempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	target := schedulersdk.TargetRef{Type: "runtime_operation", Owner: "workflow", Operation: "start", Payload: json.RawMessage(`{}`)}
	receipt, err := remote.Dispatch(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, Request{RuntimeID: "runtime-a", ExecutionID: "run-1", IdempotencyKey: "command-1", Target: target})
	if err != nil || receipt.ID != "receipt-1" || calls != 2 {
		t.Fatalf("receipt=%#v calls=%d err=%v", receipt, calls, err)
	}
}

func TestRemoteCredentialCannotDispatchForAnotherRuntime(t *testing.T) {
	var calls int
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return callbackHTTPResponse(http.StatusOK, `{}`), nil
	})}
	remote, err := NewRemote(RemoteConfig{BaseURL: "https://runtime.example", RuntimeID: "runtime-a", SigningSecret: "secret", HTTPClient: client, MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{RuntimeID: "runtime-b", ExecutionID: "run-1", IdempotencyKey: "run-1", Target: schedulersdk.TargetRef{Type: "runtime_operation", Owner: "workflow", Operation: "start"}}
	_, err = remote.Dispatch(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-b"}, request)
	if !errors.Is(err, ErrCallbackAuthentication) || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestRemoteClassifiesCallbackErrorsWithoutDisclosingBody(t *testing.T) {
	for _, test := range []struct {
		status int
		want   error
	}{
		{status: http.StatusBadRequest, want: ErrCallbackRequestInvalid},
		{status: http.StatusUnauthorized, want: ErrCallbackAuthentication},
		{status: http.StatusForbidden, want: ErrCallbackRejected},
		{status: http.StatusConflict, want: ErrCallbackIdempotencyConflict},
		{status: http.StatusTooManyRequests, want: ErrCallbackRetryable},
		{status: http.StatusInternalServerError, want: ErrCallbackRetryable},
	} {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return callbackHTTPResponse(test.status, `{"code":"dispatch.safe_code","secret":"must-not-leak"}`), nil
			})}
			remote, err := NewRemote(RemoteConfig{BaseURL: "https://runtime.example", RuntimeID: "runtime-a", SigningSecret: "secret", HTTPClient: client, MaxAttempts: 1})
			if err != nil {
				t.Fatal(err)
			}
			request := Request{RuntimeID: "runtime-a", ExecutionID: "run-1", IdempotencyKey: "run-1", Target: schedulersdk.TargetRef{Type: "runtime_operation", Owner: "workflow", Operation: "start"}}
			_, err = remote.Dispatch(context.Background(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, request)
			if !errors.Is(err, test.want) || strings.Contains(err.Error(), "must-not-leak") {
				t.Fatalf("status=%d err=%v", test.status, err)
			}
		})
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return callbackHTTPResponse(http.StatusForbidden, `{"code":"must not leak spaces","secret":"must-not-leak"}`), nil
	})}
	remote, err := NewRemote(RemoteConfig{BaseURL: "https://runtime.example", RuntimeID: "runtime-a", SigningSecret: "secret", HTTPClient: client, MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{RuntimeID: "runtime-a", ExecutionID: "run-1", IdempotencyKey: "run-1", Target: schedulersdk.TargetRef{Type: "runtime_operation", Owner: "workflow", Operation: "start"}}
	_, err = remote.Dispatch(context.Background(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, request)
	if err == nil || strings.Contains(err.Error(), "must not leak") {
		t.Fatalf("unsafe callback error=%v", err)
	}
}

func TestRemoteRejectsOversizedSuccessfulResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return callbackHTTPResponse(http.StatusOK, strings.Repeat(" ", int(maxCallbackResponseBytes)+1)), nil
	})}
	remote, err := NewRemote(RemoteConfig{BaseURL: "https://runtime.example", RuntimeID: "runtime-a", SigningSecret: "secret", HTTPClient: client, MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{RuntimeID: "runtime-a", ExecutionID: "run-1", IdempotencyKey: "run-1", Target: schedulersdk.TargetRef{Type: "runtime_operation", Owner: "workflow", Operation: "start"}}
	if _, err := remote.Dispatch(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, request); !errors.Is(err, ErrCallbackResponseInvalid) {
		t.Fatalf("oversized response err=%v", err)
	}
}

func callbackHTTPResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

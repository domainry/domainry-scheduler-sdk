package dispatchgateway

import (
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestSignatureBindsBodyExecutionAndFreshTimestamp(t *testing.T) {
	now := time.Date(2026, time.September, 4, 1, 2, 3, 0, time.UTC)
	body := []byte(`{"execution_id":"run-1"}`)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	signature := Signature{ClientID: "scheduler", Timestamp: timestamp, Value: Sign(body, "run-1", "scheduler", timestamp, []byte("secret"))}
	if err := Verify(body, "run-1", signature, []byte("secret"), now); err != nil {
		t.Fatal(err)
	}
	if err := Verify([]byte(`{}`), "run-1", signature, []byte("secret"), now); err == nil {
		t.Fatal("tampered body accepted")
	}
	if err := Verify(body, "run-2", signature, []byte("secret"), now); err == nil {
		t.Fatal("different execution accepted")
	}
	if err := Verify(body, "run-1", signature, []byte("secret"), now.Add(MaxClockSkew+time.Second)); err == nil {
		t.Fatal("stale signature accepted")
	}
}

func TestV2SignatureBindsMethodPathRuntimeAndIdempotency(t *testing.T) {
	now := time.Date(2026, time.September, 7, 1, 2, 3, 0, time.UTC)
	body := []byte(`{"runtime_id":"runtime-a","execution_id":"run-1"}`)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	request := SignedRequest{Method: http.MethodPost, Path: AcceptPath, RuntimeID: "runtime-a", IdempotencyKey: "run-1"}
	value, err := SignRequest(body, request, SchedulerClientID, timestamp, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	signature := Signature{Version: CallbackSignatureContractVersion, ClientID: SchedulerClientID, Timestamp: timestamp, Value: value}
	if err := VerifyRequest(body, request, signature, []byte("secret"), now); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRequest([]byte(`{"runtime_id":"runtime-a","execution_id":"run-2"}`), request, signature, []byte("secret"), now); !errors.Is(err, ErrCallbackSignatureInvalid) {
		t.Fatalf("tampered body err=%v", err)
	}
	mutations := []SignedRequest{
		{Method: http.MethodPut, Path: AcceptPath, RuntimeID: "runtime-a", IdempotencyKey: "run-1"},
		{Method: http.MethodPost, Path: "/dispatch/other", RuntimeID: "runtime-a", IdempotencyKey: "run-1"},
		{Method: http.MethodPost, Path: AcceptPath, RuntimeID: "runtime-b", IdempotencyKey: "run-1"},
		{Method: http.MethodPost, Path: AcceptPath, RuntimeID: "runtime-a", IdempotencyKey: "run-2"},
	}
	for _, mutation := range mutations {
		if err := VerifyRequest(body, mutation, signature, []byte("secret"), now); !errors.Is(err, ErrCallbackSignatureInvalid) {
			t.Fatalf("mutation=%+v err=%v", mutation, err)
		}
	}
	wrongVersion := signature
	wrongVersion.Version = "v1"
	if err := VerifyRequest(body, request, wrongVersion, []byte("secret"), now); !errors.Is(err, ErrCallbackSignatureInvalid) {
		t.Fatalf("wrong version err=%v", err)
	}
	wrongClient := signature
	wrongClient.ClientID = "other"
	wrongClient.Value, _ = SignRequest(body, request, wrongClient.ClientID, wrongClient.Timestamp, []byte("secret"))
	if err := VerifyRequest(body, request, wrongClient, []byte("secret"), now); !errors.Is(err, ErrCallbackSignatureInvalid) {
		t.Fatalf("wrong client err=%v", err)
	}
	if err := VerifyRequest(body, request, signature, []byte("secret"), now.Add(MaxClockSkew+time.Second)); !errors.Is(err, ErrCallbackSignatureStale) {
		t.Fatalf("stale err=%v", err)
	}
}

func TestCallbackReplayIdentityIsScopedAndContentBound(t *testing.T) {
	request := SignedRequest{Method: http.MethodPost, Path: AcceptPath, RuntimeID: "runtime-a", IdempotencyKey: "run-1"}
	first, err := NewCallbackRequestIdentity([]byte(`{"value":1}`), request)
	if err != nil {
		t.Fatal(err)
	}
	if disposition, err := EvaluateCallbackReplay(nil, first); err != nil || disposition != CallbackReplayClaim {
		t.Fatalf("claim=%q err=%v", disposition, err)
	}
	if disposition, err := EvaluateCallbackReplay(&first, first); err != nil || disposition != CallbackReplayExact {
		t.Fatalf("replay=%q err=%v", disposition, err)
	}
	conflict, _ := NewCallbackRequestIdentity([]byte(`{"value":2}`), request)
	if _, err := EvaluateCallbackReplay(&first, conflict); !errors.Is(err, ErrCallbackIdempotencyConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	otherRuntime, _ := NewCallbackRequestIdentity([]byte(`{"value":1}`), SignedRequest{Method: http.MethodPost, Path: AcceptPath, RuntimeID: "runtime-b", IdempotencyKey: "run-1"})
	if _, err := EvaluateCallbackReplay(&first, otherRuntime); !errors.Is(err, ErrCallbackRequestInvalid) {
		t.Fatalf("cross-runtime identity err=%v", err)
	}
}

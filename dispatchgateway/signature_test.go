package dispatchgateway

import (
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

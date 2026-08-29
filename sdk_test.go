package schedulersdk

import (
	"testing"
	"time"
)

func TestApplicationAndDescriptorValidation(t *testing.T) {
	if err := (ApplicationRef{}).Validate(); err == nil {
		t.Fatal("empty Runtime identity accepted")
	}
	if err := (ApplicationRef{RuntimeID: "runtime-a"}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []DeploymentMode{DeploymentModeModule, DeploymentModeSaaS} {
		if err := (Descriptor{ProtocolVersion: ProtocolVersionV1, Mode: mode}).Validate(); err != nil {
			t.Fatalf("valid descriptor rejected: %v", err)
		}
	}
	if err := (Descriptor{ProtocolVersion: "old", Mode: DeploymentModeModule}).Validate(); err == nil {
		t.Fatal("unsupported protocol accepted")
	}
}

func TestWorkerConfigIsBounded(t *testing.T) {
	config := NormalizeWorkerConfig(WorkerConfig{BatchSize: 501})
	if config.PollInterval != 500*time.Millisecond || config.BatchSize != 500 || config.LeaseTTL != 5*time.Minute || config.MaxCatchupWindows != 1 {
		t.Fatalf("unexpected normalized config: %#v", config)
	}
}

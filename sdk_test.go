package schedulersdk

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type legacyBindingStub struct {
	Binding
	calls  int
	key    string
	reason string
}

func (b *legacyBindingStub) TriggerNow(_ context.Context, key, reason string) (Run, error) {
	b.calls++
	b.key, b.reason = key, reason
	return Run{Trigger: Trigger{RunID: "run-ordinary", DefinitionKey: key}}, nil
}

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
	descriptor := Descriptor{Capabilities: []string{" durable_trigger ", CapabilityDefinitionPublicationFencing}}
	if !descriptor.Supports(CapabilityDefinitionPublicationFencing) || descriptor.Supports("missing") {
		t.Fatalf("descriptor capabilities=%v", descriptor.Capabilities)
	}
}

func TestOrdinaryTriggerNowRemainsCompatibleWithLegacyBinding(t *testing.T) {
	legacy := &legacyBindingStub{}
	var binding Binding = legacy
	run, err := binding.TriggerNow(t.Context(), "daily", "operator request")
	if err != nil || run.Trigger.RunID != "run-ordinary" || legacy.calls != 1 || legacy.key != "daily" || legacy.reason != "operator request" {
		t.Fatalf("run=%+v legacy=%+v err=%v", run, legacy, err)
	}
}

func TestFencedSnapshotAllowsNewRuntimeRevisionOneAndRejectsLateOldProcess(t *testing.T) {
	application := ApplicationRef{RuntimeID: "runtime-a"}
	sessionA := definitionPublisherSession(7, "a")
	activeA, _ := sessionA.PublisherFence()
	definitionA := snapshotDefinition("v7", "workflow-a")
	cursorA, disposition, err := EvaluateDefinitionSnapshot(application, activeA, nil, DefinitionSnapshot{PublisherSession: &sessionA, Revision: 7, Definitions: []Definition{definitionA}})
	if err != nil || disposition != DefinitionSnapshotApply {
		t.Fatalf("A cursor=%+v disposition=%q err=%v", cursorA, disposition, err)
	}

	sessionB := definitionPublisherSession(8, "b")
	activeB, _ := sessionB.PublisherFence()
	definitionB := snapshotDefinition("v8", "workflow-b")
	cursorB, disposition, err := EvaluateDefinitionSnapshot(application, activeB, &cursorA, DefinitionSnapshot{PublisherSession: &sessionB, Revision: 1, Definitions: []Definition{definitionB}})
	if err != nil || disposition != DefinitionSnapshotApply || cursorB.Revision != 1 || cursorB.PublisherFence.Generation != 8 {
		t.Fatalf("B cursor=%+v disposition=%q err=%v", cursorB, disposition, err)
	}
	lateA := DefinitionSnapshot{PublisherSession: &sessionA, Revision: 8, Definitions: []Definition{snapshotDefinition("late-a", "stale")}}
	if _, _, err := EvaluateDefinitionSnapshot(application, activeB, &cursorB, lateA); !errors.Is(err, ErrDefinitionSnapshotStale) {
		t.Fatalf("late process A err=%v", err)
	}
}

func TestFencedSnapshotSurvivesSchedulerRestartAndRejectsMissingOrForgedSession(t *testing.T) {
	application := ApplicationRef{RuntimeID: "runtime-a"}
	session := definitionPublisherSession(8, "b")
	active, _ := session.PublisherFence()
	first := DefinitionSnapshot{PublisherSession: &session, Revision: 1, Definitions: []Definition{snapshotDefinition("v1", "workflow-a")}}
	cursor, _, err := EvaluateDefinitionSnapshot(application, active, nil, first)
	if err != nil {
		t.Fatal(err)
	}
	persisted, _ := json.Marshal(struct {
		Active DefinitionPublisherFence
		Cursor DefinitionSnapshotCursor
	}{active, cursor})
	var restored struct {
		Active DefinitionPublisherFence
		Cursor DefinitionSnapshotCursor
	}
	if err := json.Unmarshal(persisted, &restored); err != nil {
		t.Fatal(err)
	}
	second := DefinitionSnapshot{PublisherSession: &session, Revision: 2, Definitions: []Definition{snapshotDefinition("v2", "workflow-b")}}
	if _, disposition, err := EvaluateDefinitionSnapshot(application, restored.Active, &restored.Cursor, second); err != nil || disposition != DefinitionSnapshotApply {
		t.Fatalf("post-restart disposition=%q err=%v", disposition, err)
	}
	if _, _, err := EvaluateDefinitionSnapshot(application, restored.Active, &restored.Cursor, DefinitionSnapshot{Revision: 2, Definitions: second.Definitions}); !errors.Is(err, ErrDefinitionPublicationRequired) {
		t.Fatalf("missing session err=%v", err)
	}
	forged := definitionPublisherSession(8, "forged")
	if _, _, err := EvaluateDefinitionSnapshot(application, restored.Active, &restored.Cursor, DefinitionSnapshot{PublisherSession: &forged, Revision: 2, Definitions: second.Definitions}); !errors.Is(err, ErrDefinitionPublicationSessionMismatch) {
		t.Fatalf("forged session err=%v", err)
	}
}

func TestOnlyLatestConcurrentSessionAndRevisionCanPublish(t *testing.T) {
	application := ApplicationRef{RuntimeID: "runtime-a"}
	older := definitionPublisherSession(10, "older")
	latest := definitionPublisherSession(11, "latest")
	active, _ := latest.PublisherFence()
	if _, _, err := EvaluateDefinitionSnapshot(application, active, nil, DefinitionSnapshot{PublisherSession: &older, Revision: 1, Definitions: []Definition{snapshotDefinition("old", "old")}}); !errors.Is(err, ErrDefinitionSnapshotStale) {
		t.Fatalf("retired concurrent session err=%v", err)
	}
	current, _, err := EvaluateDefinitionSnapshot(application, active, nil, DefinitionSnapshot{PublisherSession: &latest, Revision: 7, Definitions: []Definition{snapshotDefinition("v7", "new")}})
	if err != nil {
		t.Fatal(err)
	}
	lateSix := DefinitionSnapshot{PublisherSession: &latest, Revision: 6, Definitions: []Definition{snapshotDefinition("v6", "late")}}
	if _, _, err := EvaluateDefinitionSnapshot(application, active, &current, lateSix); !errors.Is(err, ErrDefinitionSnapshotStale) {
		t.Fatalf("late revision err=%v", err)
	}
	exact := DefinitionSnapshot{PublisherSession: &latest, Revision: 7, Definitions: []Definition{snapshotDefinition("v7", "new")}}
	if _, disposition, err := EvaluateDefinitionSnapshot(application, active, &current, exact); err != nil || disposition != DefinitionSnapshotReplay {
		t.Fatalf("exact replay disposition=%q err=%v", disposition, err)
	}
	conflict := DefinitionSnapshot{PublisherSession: &latest, Revision: 7, Definitions: []Definition{snapshotDefinition("v7-conflict", "changed")}}
	if _, _, err := EvaluateDefinitionSnapshot(application, active, &current, conflict); !errors.Is(err, ErrDefinitionSnapshotConflict) {
		t.Fatalf("same-revision conflict err=%v", err)
	}
	otherApplication := ApplicationRef{RuntimeID: "runtime-b"}
	if _, _, err := EvaluateDefinitionSnapshot(otherApplication, active, &current, exact); !errors.Is(err, ErrDefinitionPublicationSessionMismatch) {
		t.Fatalf("cross-Runtime cursor err=%v", err)
	}
}

func TestModuleSnapshotRevisionResetsAcrossRuntimeProcessRestart(t *testing.T) {
	before := DefinitionSnapshot{Revision: 7, Definitions: []Definition{snapshotDefinition("v7", "workflow-a")}}
	after := DefinitionSnapshot{Revision: 1, Definitions: []Definition{snapshotDefinition("v8", "workflow-b")}}
	if err := ValidateModuleDefinitionSnapshot(before); err != nil {
		t.Fatal(err)
	}
	if err := ValidateModuleDefinitionSnapshot(after); err != nil {
		t.Fatalf("new Runtime process revision 1 rejected: %v", err)
	}
}

func TestDefinitionSnapshotSerializationIsAdditiveAndContentHashIsOrderIndependent(t *testing.T) {
	session := definitionPublisherSession(3, "s")
	encoded, err := json.Marshal(DefinitionSnapshot{PublisherSession: &session, Revision: 1, Definitions: []Definition{snapshotDefinition("v1", "daily")}})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"publisher_session"`, `"contract_version"`, `"generation"`, `"session_nonce"`, `"revision"`} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("snapshot wire omits %s: %s", field, encoded)
		}
	}
	var legacy DefinitionSnapshot
	if err := json.Unmarshal([]byte(`{"revision":7,"definitions":[]}`), &legacy); err != nil || legacy.Revision != 7 || legacy.PublisherSession != nil {
		t.Fatalf("legacy snapshot=%+v err=%v", legacy, err)
	}
	first := snapshotDefinition("v1", "first")
	second := first
	second.Key = "second"
	second.Target.Operation = "second"
	hashAB, err := DefinitionSnapshotContentSHA256([]Definition{first, second})
	if err != nil {
		t.Fatal(err)
	}
	hashBA, err := DefinitionSnapshotContentSHA256([]Definition{second, first})
	if err != nil || hashAB != hashBA {
		t.Fatalf("hash AB=%s BA=%s err=%v", hashAB, hashBA, err)
	}
	if _, err := DefinitionSnapshotContentSHA256([]Definition{first, first}); err == nil {
		t.Fatal("duplicate definition key accepted")
	}
}

func definitionPublisherSession(generation uint64, seed string) DefinitionPublisherSession {
	return DefinitionPublisherSession{ContractVersion: DefinitionPublicationContractVersion, Generation: generation, SessionNonce: strings.Repeat(seed, 32)}
}

func snapshotDefinition(revision, operation string) Definition {
	return Definition{Key: "daily", Revision: revision, Schedule: Schedule{Type: "interval", IntervalSeconds: 60}, Target: TargetRef{Type: "runtime_operation", Owner: "workflow", Operation: operation}}
}

func TestWorkerConfigIsBounded(t *testing.T) {
	config := NormalizeWorkerConfig(WorkerConfig{BatchSize: 501})
	if config.PollInterval != 500*time.Millisecond || config.BatchSize != 500 || config.LeaseTTL != 5*time.Minute {
		t.Fatalf("unexpected normalized config: %#v", config)
	}
}

func TestDefinitionRequiresStableTargetAndRevision(t *testing.T) {
	definition := Definition{Key: "customer.refresh", Revision: "v1", Schedule: Schedule{Type: "cron"}, Target: TargetRef{Type: "runtime_operation", Owner: "workflow", Operation: "customer.refresh"}}
	if err := definition.Validate(); err != nil {
		t.Fatal(err)
	}
	definition.Target.Operation = ""
	if err := definition.Validate(); err == nil {
		t.Fatal("definition without downstream operation accepted")
	}
}

package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

func TestScheduledPlanCreateSupportsOneTimeAndRecurringRules(t *testing.T) {
	at := time.Date(2026, 9, 18, 9, 30, 0, 0, time.FixedZone("Shanghai", 8*60*60))
	oneTime := validScheduledPlanCreate()
	oneTime.Trigger = schedulersdk.ScheduledPlanTrigger{Type: schedulersdk.ScheduledPlanTriggerOnce, At: &at}
	if err := ValidateScheduledPlanCreate(t.Context(), oneTime); err != nil {
		t.Fatal(err)
	}
	recurring := validScheduledPlanCreate()
	recurring.Trigger = schedulersdk.ScheduledPlanTrigger{Type: schedulersdk.ScheduledPlanTriggerRecurring, Schedule: &schedulersdk.Schedule{Type: "weekly_at", TimeOfDay: "09:00", DayOfWeek: "monday", Timezone: "Asia/Shanghai"}}
	if err := ValidateScheduledPlanCreate(t.Context(), recurring); err != nil {
		t.Fatal(err)
	}
}

func TestScheduledPlanCreateRejectsAmbiguousOrLeakyRecords(t *testing.T) {
	at := time.Date(2026, 9, 18, 9, 30, 0, 0, time.UTC)
	tests := []func(*schedulersdk.ScheduledPlanCreate){
		func(value *schedulersdk.ScheduledPlanCreate) { value.Timezone = "Local" },
		func(value *schedulersdk.ScheduledPlanCreate) {
			value.Trigger = schedulersdk.ScheduledPlanTrigger{Type: schedulersdk.ScheduledPlanTriggerOnce, At: &at, Schedule: &schedulersdk.Schedule{Type: "interval", IntervalSeconds: 60}}
		},
		func(value *schedulersdk.ScheduledPlanCreate) {
			value.Trigger = schedulersdk.ScheduledPlanTrigger{Type: schedulersdk.ScheduledPlanTriggerRecurring}
		},
		func(value *schedulersdk.ScheduledPlanCreate) { value.Trigger.Schedule.Timezone = "UTC" },
		func(value *schedulersdk.ScheduledPlanCreate) { value.Input = json.RawMessage(`[]`) },
		func(value *schedulersdk.ScheduledPlanCreate) {
			value.AllowedActions = []string{"todo.list", "todo.list"}
		},
		func(value *schedulersdk.ScheduledPlanCreate) {
			value.ConversationRef = schedulersdk.ScheduledPlanConversationRef{RunID: "run-only"}
		},
		func(value *schedulersdk.ScheduledPlanCreate) { value.Owner.UserID = "" },
		func(value *schedulersdk.ScheduledPlanCreate) { value.Target.Owner = "" },
		func(value *schedulersdk.ScheduledPlanCreate) {
			value.Target.Payload = json.RawMessage(`{"duplicate":"input"}`)
		},
		func(value *schedulersdk.ScheduledPlanCreate) { value.Owner.ProductKey = string(make([]byte, 192)) },
	}
	for index, mutate := range tests {
		value := validScheduledPlanCreate()
		mutate(&value)
		if err := ValidateScheduledPlanCreate(t.Context(), value); !errors.Is(err, schedulersdk.ErrScheduledPlanInvalid) {
			t.Fatalf("case %d err=%v", index, err)
		}
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ValidateScheduledPlanCreate(cancelled, validScheduledPlanCreate()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context err=%v", err)
	}
}

func TestScheduledPlanCreateValidatesDurableExecutionPolicy(t *testing.T) {
	value := validScheduledPlanCreate()
	value.Trigger.Policy = schedulersdk.Policy{Misfire: schedulersdk.ScheduledPlanMisfireCatchMany, MaxCatchupWindows: 3, MisfireGrace: time.Minute, MaxAttempts: 3, RetryInitial: time.Second, RetryMax: time.Minute, Timeout: time.Hour}
	if err := ValidateScheduledPlanCreate(t.Context(), value); err != nil {
		t.Fatal(err)
	}
	invalid := value
	invalid.Trigger.Policy.MaxCatchupWindows = 101
	if err := ValidateScheduledPlanCreate(t.Context(), invalid); !errors.Is(err, schedulersdk.ErrScheduledPlanInvalid) {
		t.Fatalf("catch-up bound err=%v", err)
	}
	invalid = value
	invalid.Trigger.Policy.RetryInitial = 2 * time.Hour
	invalid.Trigger.Policy.RetryMax = time.Hour
	if err := ValidateScheduledPlanCreate(t.Context(), invalid); !errors.Is(err, schedulersdk.ErrScheduledPlanInvalid) {
		t.Fatalf("retry range err=%v", err)
	}
	invalid = value
	invalid.Trigger.Type = schedulersdk.ScheduledPlanTriggerOnce
	at := time.Now().UTC().Add(time.Hour)
	invalid.Trigger.At, invalid.Trigger.Schedule = &at, nil
	if err := ValidateScheduledPlanCreate(t.Context(), invalid); !errors.Is(err, schedulersdk.ErrScheduledPlanInvalid) {
		t.Fatalf("one-time bounded catch-up err=%v", err)
	}
}

func validScheduledPlanCreate() schedulersdk.ScheduledPlanCreate {
	return schedulersdk.ScheduledPlanCreate{
		ClientID: "weekly-plan", Name: "整理本周待办",
		Owner:    schedulersdk.ScheduledPlanOwner{WorkspaceID: "workspace-a", UserID: "user-a", ProductKey: "agent"},
		Timezone: "Asia/Shanghai",
		Trigger:  schedulersdk.ScheduledPlanTrigger{Type: schedulersdk.ScheduledPlanTriggerRecurring, Schedule: &schedulersdk.Schedule{Type: "weekly_at", TimeOfDay: "09:00", DayOfWeek: "monday", Timezone: "Asia/Shanghai"}},
		Input:    json.RawMessage(`{"goal":"整理未完成待办"}`), AllowedActions: []string{"todo.list", "artifact.create"},
		Target:          schedulersdk.TargetRef{Type: "runtime_operation", Owner: "agent", Operation: "conversation_task_start"},
		ConversationRef: schedulersdk.ScheduledPlanConversationRef{ConversationID: "conversation-a", RunID: "run-a"},
		Status:          schedulersdk.ScheduledPlanStatusEnabled,
	}
}

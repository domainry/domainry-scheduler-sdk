package modulehost

import (
	"context"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

// Host is the narrow capability set borrowed by an in-process Scheduler.
// Implementations adapt Runtime-owned metadata, mutation-kernel, target and
// audit capabilities; they do not give Scheduler raw database access.
type Host interface {
	Definitions() DefinitionSource
	State() StateStore
	Targets() TargetRuntime
	Audit() AuditAppender
}

type DefinitionSource interface {
	List(context.Context, schedulersdk.Principal) ([]schedulersdk.Record, error)
	Get(context.Context, string, schedulersdk.Principal) (schedulersdk.Record, bool, error)
	Versions(context.Context, string, schedulersdk.Principal) ([]schedulersdk.DefinitionVersion, error)
}

// StateStore keeps Scheduler's operational records behind owner-scoped,
// workspace-aware mutations. Conditional mutations carry fencing evidence.
type StateStore interface {
	Get(context.Context, string, string, schedulersdk.Principal) (schedulersdk.Record, bool, error)
	List(context.Context, string, schedulersdk.Payload, schedulersdk.Principal) ([]schedulersdk.Record, error)
	Commit(context.Context, []Mutation, schedulersdk.Principal) error
	ConditionalUpdate(context.Context, Mutation, Condition, schedulersdk.Principal) (bool, error)
}

type Mutation struct {
	Operation string              `json:"operation"`
	Object    string              `json:"object"`
	Record    schedulersdk.Record `json:"record"`
}

type Condition struct {
	Fields schedulersdk.Payload `json:"fields"`
}

type TargetRuntime interface {
	Execute(context.Context, TargetExecution, schedulersdk.Principal) (schedulersdk.Payload, error)
}

type TargetExecution struct {
	RunID        string               `json:"run_id"`
	DefinitionID string               `json:"definition_id"`
	TargetType   string               `json:"target_type"`
	TargetKey    string               `json:"target_key"`
	ScheduledFor time.Time            `json:"scheduled_for"`
	Payload      schedulersdk.Payload `json:"payload"`
	Idempotency  string               `json:"idempotency_key"`
}

type AuditAppender interface {
	Append(context.Context, AuditEvent, schedulersdk.Principal) error
}

type AuditEvent struct {
	Action     string               `json:"action"`
	ResourceID string               `json:"resource_id"`
	Data       schedulersdk.Payload `json:"data"`
}

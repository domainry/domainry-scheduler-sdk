// Package schedulersdk defines the deployment-neutral Scheduler protocol.
// Scheduler owns time and durable trigger evidence; downstream owners retain
// all business execution semantics.
package schedulersdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type DeploymentMode string

const (
	DeploymentModeModule DeploymentMode = "module"
	DeploymentModeSaaS   DeploymentMode = "saas"
	ProtocolVersionV1                   = "domainry-scheduler-protocol-v1"
)

type ApplicationRef struct {
	RuntimeID string `json:"runtime_id"`
}

func (r ApplicationRef) Validate() error {
	if strings.TrimSpace(r.RuntimeID) == "" {
		return fmt.Errorf("scheduler runtime identity is required")
	}
	return nil
}

type Descriptor struct {
	ProtocolVersion string         `json:"protocol_version"`
	Mode            DeploymentMode `json:"mode"`
	Capabilities    []string       `json:"capabilities"`
}

func (d Descriptor) Validate() error {
	if d.ProtocolVersion != ProtocolVersionV1 {
		return fmt.Errorf("unsupported Scheduler protocol %q", d.ProtocolVersion)
	}
	if d.Mode != DeploymentModeModule && d.Mode != DeploymentModeSaaS {
		return fmt.Errorf("invalid Scheduler deployment mode %q", d.Mode)
	}
	return nil
}

type Schedule struct {
	Type            string `json:"type"`
	Expression      string `json:"expression,omitempty"`
	Timezone        string `json:"timezone,omitempty"`
	IntervalSeconds int    `json:"interval_seconds,omitempty"`
	TimeOfDay       string `json:"time_of_day,omitempty"`
	DayOfWeek       string `json:"day_of_week,omitempty"`
	DayOfMonth      int    `json:"day_of_month,omitempty"`
}

type TargetRef struct {
	Type          string          `json:"type"`
	Owner         string          `json:"owner"`
	Operation     string          `json:"operation"`
	ConnectionKey string          `json:"connection_key,omitempty"`
	DispatchMode  string          `json:"dispatch_mode,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

type Policy struct {
	Overlap           string        `json:"overlap,omitempty"`
	Misfire           string        `json:"misfire,omitempty"`
	MaxCatchupWindows int           `json:"max_catchup_windows,omitempty"`
	Timeout           time.Duration `json:"timeout,omitempty"`
	MaxAttempts       int           `json:"max_attempts,omitempty"`
	RetryInitial      time.Duration `json:"retry_initial,omitempty"`
	RetryMax          time.Duration `json:"retry_max,omitempty"`
}

type Definition struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Revision string `json:"revision"`
	// InitialNextRunAt seeds a new Scheduler store during first activation or
	// Module-to-SaaS cutover. Persisted cursors remain authoritative afterward.
	InitialNextRunAt time.Time `json:"initial_next_run_at,omitempty"`
	Schedule         Schedule  `json:"schedule"`
	Target           TargetRef `json:"target"`
	Policy           Policy    `json:"policy"`
}

func (d Definition) Validate() error {
	if strings.TrimSpace(d.Key) == "" {
		return fmt.Errorf("scheduler definition key is required")
	}
	if strings.TrimSpace(d.Revision) == "" {
		return fmt.Errorf("scheduler definition %s revision is required", d.Key)
	}
	if strings.TrimSpace(d.Schedule.Type) == "" {
		return fmt.Errorf("scheduler definition %s schedule type is required", d.Key)
	}
	targetType := strings.TrimSpace(d.Target.Type)
	if targetType == "" {
		targetType = "runtime_operation"
	}
	if strings.TrimSpace(d.Target.Operation) == "" {
		return fmt.Errorf("scheduler definition %s target operation is required", d.Key)
	}
	switch targetType {
	case "runtime_operation":
		if strings.TrimSpace(d.Target.Owner) == "" {
			return fmt.Errorf("scheduler definition %s runtime target owner is required", d.Key)
		}
	case "http":
		if strings.TrimSpace(d.Target.ConnectionKey) == "" {
			return fmt.Errorf("scheduler definition %s HTTP connection is required", d.Key)
		}
	default:
		return fmt.Errorf("scheduler definition %s target type %q is unsupported", d.Key, targetType)
	}
	return nil
}

// Normalize applies protocol defaults before a definition is retained or dispatched.
func (d Definition) Normalize() Definition {
	if strings.TrimSpace(d.Target.Type) == "" {
		d.Target.Type = "runtime_operation"
	}
	return d
}

type DefinitionSnapshot struct {
	Revision    int64        `json:"revision"`
	Definitions []Definition `json:"definitions"`
}

type Trigger struct {
	RunID          string          `json:"run_id"`
	DefinitionKey  string          `json:"definition_key"`
	DefinitionRev  string          `json:"definition_revision"`
	ScheduledFor   time.Time       `json:"scheduled_for"`
	WindowKey      string          `json:"window_key"`
	Target         TargetRef       `json:"target"`
	IdempotencyKey string          `json:"idempotency_key"`
	Attempt        int             `json:"attempt"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

type DownstreamReceipt struct {
	ID     string `json:"id"`
	Owner  string `json:"owner"`
	Status string `json:"status"`
	Replay bool   `json:"replay"`
}

// Lease is the durable execution ownership granted by RunStore. Owner is a
// process instance identity; Token fences stale workers after a lease takeover.
type Lease struct {
	Owner     string    `json:"owner"`
	Token     int64     `json:"fencing_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (l Lease) Valid() bool {
	return strings.TrimSpace(l.Owner) != "" && l.Token > 0 && !l.ExpiresAt.IsZero()
}

type Run struct {
	Trigger           Trigger           `json:"trigger"`
	Lease             Lease             `json:"lease"`
	Status            string            `json:"status"`
	DownstreamReceipt DownstreamReceipt `json:"downstream_receipt,omitempty"`
	LastError         string            `json:"last_error,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// DeadLetter is Scheduler-owned terminal failure state. RunEvent remains
// immutable execution evidence and is deliberately not an operations queue.
type DeadLetter struct {
	RunID         string    `json:"run_id"`
	DefinitionKey string    `json:"definition_key"`
	Status        string    `json:"status"`
	Reason        string    `json:"reason"`
	FailedAt      time.Time `json:"failed_at"`
	ResolvedAt    time.Time `json:"resolved_at,omitempty"`
}

type WorkerConfig struct {
	Enabled      bool          `json:"enabled"`
	PollInterval time.Duration `json:"poll_interval"`
	BatchSize    int           `json:"batch_size"`
	LeaseTTL     time.Duration `json:"lease_ttl"`
}

func NormalizeWorkerConfig(config WorkerConfig) WorkerConfig {
	if config.PollInterval <= 0 {
		config.PollInterval = 500 * time.Millisecond
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 25
	}
	if config.BatchSize > 500 {
		config.BatchSize = 500
	}
	if config.LeaseTTL <= 0 {
		config.LeaseTTL = 5 * time.Minute
	}
	return config
}

type Factory interface {
	Open(context.Context, ApplicationRef) (Binding, error)
}

// Binding is Runtime's only Scheduler dependency. Reconcile consumes published
// configuration, Tick performs bounded recovery, and TriggerNow shares the
// same durable dispatch path as clock-driven work.
type Binding interface {
	Descriptor() Descriptor
	Reconcile(context.Context) error
	Preview(context.Context, Schedule, time.Time, int) ([]time.Time, error)
	Tick(context.Context, time.Time, int) (int, error)
	TriggerNow(context.Context, string, string) (Run, error)
	Reschedule(context.Context, string, time.Time, string) error
	Runs(context.Context, int) ([]Run, error)
	Run(context.Context, string) (Run, error)
	RetryRun(context.Context, string, string) (Run, error)
	CancelRun(context.Context, string, string) (Run, error)
	DeadLetter(context.Context, string) (DeadLetter, error)
	ResolveDeadLetter(context.Context, string, string) (DeadLetter, error)
	RequeueDeadLetter(context.Context, string, string) (Run, error)
	Start(context.Context, WorkerConfig) <-chan struct{}
	Close(context.Context) error
}

// Package schedulersdk defines the deployment-neutral Scheduler owner contract.
// It deliberately contains no Runtime, database, HTTP, or UI dependencies.
package schedulersdk

import (
	"context"
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

// Principal is the complete authorization evidence accepted by Scheduler.
// The host maps its identity model into this value before crossing the owner
// boundary; Scheduler never imports a host-specific principal implementation.
type Principal struct {
	WorkspaceID string   `json:"workspace_id"`
	ActorID     string   `json:"actor_id"`
	Permissions []string `json:"permissions"`
	System      bool     `json:"system"`
	Reason      string   `json:"reason,omitempty"`
}

type Record struct {
	ID        string         `json:"id"`
	Data      map[string]any `json:"data"`
	CreatedAt string         `json:"created_at,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
}

type DefinitionVersion struct {
	VersionID string         `json:"version_id"`
	Event     string         `json:"event"`
	Data      map[string]any `json:"data"`
	CreatedAt string         `json:"created_at"`
}

type DefinitionPreview struct {
	NextRuns []string `json:"next_runs"`
}

// Payload is used for owner-shaped projections whose fields evolve under the
// versioned Scheduler protocol without leaking host domain types into the SDK.
type Payload map[string]any

type OperationResult struct {
	Run        Record  `json:"run"`
	Replay     bool    `json:"replay"`
	HTTPStatus int     `json:"http_status,omitempty"`
	Evidence   Payload `json:"evidence,omitempty"`
}

type WorkerConfig struct {
	Enabled           bool          `json:"enabled"`
	PollInterval      time.Duration `json:"poll_interval"`
	BatchSize         int           `json:"batch_size"`
	LeaseTTL          time.Duration `json:"lease_ttl"`
	MaxCatchupWindows int           `json:"max_catchup_windows"`
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
	if config.MaxCatchupWindows <= 0 {
		config.MaxCatchupWindows = 1
	}
	return config
}

type Queries interface {
	Definitions(context.Context, Principal) ([]Record, error)
	Definition(context.Context, string, Principal) (Record, error)
	DefinitionVersions(context.Context, string, Principal) ([]DefinitionVersion, error)
	AuthoringContract(context.Context, Principal) (Payload, error)
	OperationsState(context.Context, Principal) (Payload, error)
	PreviewDefinition(context.Context, Payload, Principal) (DefinitionPreview, error)
	PreviewSchedule(context.Context, Payload, Principal) (DefinitionPreview, error)
}

type Commands interface {
	SimulateDefinition(context.Context, string, Principal) (OperationResult, error)
	RunDefinition(context.Context, string, string, Principal) (OperationResult, error)
	RescheduleDefinition(context.Context, string, time.Time, Principal) (OperationResult, error)
	RetryRun(context.Context, string, string, Principal) (OperationResult, error)
	CancelRun(context.Context, string, string, Principal) (OperationResult, error)
	ResolveDeadLetter(context.Context, string, string, string, Principal) (OperationResult, error)
	RequeueDeadLetter(context.Context, string, string, string, Principal) (OperationResult, error)
}

type Worker interface {
	StartWorker(context.Context, WorkerConfig, bool) <-chan struct{}
}

type Factory interface {
	Open(context.Context, ApplicationRef) (Binding, error)
}

// Binding is the only Scheduler capability consumed by Runtime composition.
// Module and SaaS implementations must expose equivalent behavior here.
type Binding interface {
	Queries
	Commands
	Worker
	Descriptor() Descriptor
	Close(context.Context) error
}

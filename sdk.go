// Package schedulersdk defines the deployment-neutral Scheduler protocol.
// Scheduler owns time and durable trigger evidence; downstream owners retain
// all business execution semantics.
package schedulersdk

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/domainry/domainry-foundation/modulecapability"
)

type DeploymentMode string

const (
	DeploymentModeModule                   DeploymentMode = "module"
	DeploymentModeSaaS                     DeploymentMode = "saas"
	ProtocolVersionV1                                     = "domainry-scheduler-protocol-v1"
	CapabilityDefinitionPublicationFencing                = "definition_publication_fencing_v1"
	CapabilityScheduledPlanRecords                        = "scheduled_plan_records_v1"
)

var (
	ErrDefinitionPublicationRequired           = errors.New("Scheduler definition publisher session is required")
	ErrDefinitionPublicationCapabilityRequired = errors.New("Scheduler SaaS definition publication fencing capability is required")
	ErrDefinitionPublicationSessionMismatch    = errors.New("Scheduler definition publisher session does not match the active session")
	ErrDefinitionSnapshotStale                 = errors.New("Scheduler definition snapshot is stale")
	ErrDefinitionSnapshotConflict              = errors.New("Scheduler definition snapshot conflicts with accepted content")
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

func (d Descriptor) Supports(capability string) bool {
	wanted := strings.TrimSpace(capability)
	if wanted == "" {
		return false
	}
	for _, candidate := range d.Capabilities {
		if strings.TrimSpace(candidate) == wanted {
			return true
		}
	}
	return false
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

func (t TargetRef) Normalize() TargetRef {
	if strings.TrimSpace(t.Type) == "" {
		t.Type = "runtime_operation"
	}
	return t
}

// Validate checks a resolved downstream target without assigning application
// identity. The label is used only to make owner-boundary errors actionable.
func (t TargetRef) Validate(label string) error {
	if strings.TrimSpace(label) == "" {
		label = "scheduler target"
	}
	targetType := strings.TrimSpace(t.Type)
	if targetType == "" {
		targetType = "runtime_operation"
	}
	if strings.TrimSpace(t.Operation) == "" {
		return fmt.Errorf("%s operation is required", label)
	}
	switch targetType {
	case "runtime_operation":
		if strings.TrimSpace(t.Owner) == "" {
			return fmt.Errorf("%s runtime owner is required", label)
		}
	case "http":
		if strings.TrimSpace(t.ConnectionKey) == "" {
			return fmt.Errorf("%s HTTP connection is required", label)
		}
	default:
		return fmt.Errorf("%s type %q is unsupported", label, targetType)
	}
	return nil
}

type Policy struct {
	Overlap           string        `json:"overlap,omitempty"`
	Misfire           string        `json:"misfire,omitempty"`
	MisfireGrace      time.Duration `json:"misfire_grace,omitempty"`
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
	if err := d.Target.Validate("scheduler definition " + d.Key + " target"); err != nil {
		return err
	}
	return nil
}

// Normalize applies protocol defaults before a definition is retained or dispatched.
func (d Definition) Normalize() Definition {
	d.Target = d.Target.Normalize()
	return d
}

const DefinitionPublicationContractVersion = "domainry-scheduler-definition-publication-v1"

// DefinitionPublisherSession is a Scheduler-issued, application-scoped fence.
// Scheduler allocates Generation monotonically and persists the active nonce
// hash before returning the raw nonce. A Runtime process obtains one session for
// its remote binding lifetime; reconnecting obtains a strictly greater
// generation. Session identity is never an application selector: the
// authenticated ApplicationRef remains authoritative.
type DefinitionPublisherSession struct {
	ContractVersion string `json:"contract_version"`
	Generation      uint64 `json:"generation"`
	SessionNonce    string `json:"session_nonce"`
}

func (s DefinitionPublisherSession) Validate() error {
	if s.ContractVersion != DefinitionPublicationContractVersion ||
		strings.TrimSpace(s.SessionNonce) == "" || strings.TrimSpace(s.SessionNonce) != s.SessionNonce ||
		len(s.SessionNonce) < 32 || len(s.SessionNonce) > 512 || s.Generation == 0 {
		return ErrDefinitionPublicationRequired
	}
	return nil
}

// DefinitionPublisherFence is the only session material Scheduler persists.
// SessionNonce must be discarded after deriving SessionSHA256.
type DefinitionPublisherFence struct {
	ContractVersion string `json:"contract_version"`
	Generation      uint64 `json:"generation"`
	SessionSHA256   string `json:"session_sha256"`
}

func (s DefinitionPublisherSession) PublisherFence() (DefinitionPublisherFence, error) {
	if err := s.Validate(); err != nil {
		return DefinitionPublisherFence{}, err
	}
	digest := sha256.Sum256([]byte(s.SessionNonce))
	return DefinitionPublisherFence{ContractVersion: DefinitionPublicationContractVersion, Generation: s.Generation, SessionSHA256: fmt.Sprintf("%x", digest[:])}, nil
}

func (f DefinitionPublisherFence) Validate() error {
	if f.ContractVersion != DefinitionPublicationContractVersion || f.Generation == 0 || !validSHA256(f.SessionSHA256) {
		return ErrDefinitionPublicationRequired
	}
	return nil
}

func (f DefinitionPublisherFence) Equal(other DefinitionPublisherFence) bool {
	return f.ContractVersion == other.ContractVersion &&
		f.Generation == other.Generation &&
		len(f.SessionSHA256) == len(other.SessionSHA256) &&
		subtle.ConstantTimeCompare([]byte(f.SessionSHA256), []byte(other.SessionSHA256)) == 1
}

type DefinitionSnapshot struct {
	// PublisherSession is additive for v1 wire compatibility. A nil value is a
	// legacy snapshot and may be used only before a Scheduler store is migrated
	// to fenced publication. Once the first session is issued, legacy snapshots
	// must fail closed rather than fall back to process-local Revision ordering.
	PublisherSession *DefinitionPublisherSession `json:"publisher_session,omitempty"`
	Revision         int64                       `json:"revision"`
	Definitions      []Definition                `json:"definitions"`
}

type DefinitionSnapshotCursor struct {
	Application    ApplicationRef           `json:"application"`
	PublisherFence DefinitionPublisherFence `json:"publisher_fence"`
	Revision       int64                    `json:"revision"`
	ContentSHA256  string                   `json:"content_sha256"`
}

type DefinitionSnapshotDisposition string

const (
	DefinitionSnapshotApply  DefinitionSnapshotDisposition = "apply"
	DefinitionSnapshotReplay DefinitionSnapshotDisposition = "replay"
)

// EvaluateDefinitionSnapshot is the shared fenced ordering contract. The
// caller supplies the Scheduler-persisted active fence for the authenticated
// application and must update snapshot rows and the returned cursor in one
// transaction when disposition is apply. Exact retries are replays; a different
// payload at the same revision is a conflict. A greater active fence admits a
// revision reset after Runtime restart, while every older session stays stale.
func EvaluateDefinitionSnapshot(application ApplicationRef, active DefinitionPublisherFence, current *DefinitionSnapshotCursor, incoming DefinitionSnapshot) (DefinitionSnapshotCursor, DefinitionSnapshotDisposition, error) {
	if err := application.Validate(); err != nil {
		return DefinitionSnapshotCursor{}, "", err
	}
	application.RuntimeID = strings.TrimSpace(application.RuntimeID)
	if err := active.Validate(); err != nil {
		return DefinitionSnapshotCursor{}, "", err
	}
	if incoming.PublisherSession == nil {
		return DefinitionSnapshotCursor{}, "", ErrDefinitionPublicationRequired
	}
	session := *incoming.PublisherSession
	fence, err := session.PublisherFence()
	if err != nil {
		return DefinitionSnapshotCursor{}, "", err
	}
	if incoming.Revision <= 0 {
		return DefinitionSnapshotCursor{}, "", fmt.Errorf("%w: revision must be positive", ErrDefinitionSnapshotConflict)
	}
	digest, err := DefinitionSnapshotContentSHA256(incoming.Definitions)
	if err != nil {
		return DefinitionSnapshotCursor{}, "", err
	}
	next := DefinitionSnapshotCursor{Application: application, PublisherFence: fence, Revision: incoming.Revision, ContentSHA256: digest}
	if current != nil {
		if err := current.Validate(); err != nil {
			return DefinitionSnapshotCursor{}, "", fmt.Errorf("current Scheduler definition snapshot cursor: %w", err)
		}
		if strings.TrimSpace(current.Application.RuntimeID) != strings.TrimSpace(application.RuntimeID) {
			return DefinitionSnapshotCursor{}, "", ErrDefinitionPublicationSessionMismatch
		}
		if fence.Generation < current.PublisherFence.Generation {
			return DefinitionSnapshotCursor{}, "", ErrDefinitionSnapshotStale
		}
	}
	if !fence.Equal(active) {
		if fence.Generation < active.Generation {
			return DefinitionSnapshotCursor{}, "", ErrDefinitionSnapshotStale
		}
		return DefinitionSnapshotCursor{}, "", ErrDefinitionPublicationSessionMismatch
	}
	if current == nil || fence.Generation > current.PublisherFence.Generation {
		return next, DefinitionSnapshotApply, nil
	}
	if !fence.Equal(current.PublisherFence) {
		return DefinitionSnapshotCursor{}, "", ErrDefinitionSnapshotConflict
	}
	if incoming.Revision < current.Revision {
		return DefinitionSnapshotCursor{}, "", ErrDefinitionSnapshotStale
	}
	if incoming.Revision == current.Revision {
		if digest != current.ContentSHA256 {
			return DefinitionSnapshotCursor{}, "", ErrDefinitionSnapshotConflict
		}
		return *current, DefinitionSnapshotReplay, nil
	}
	return next, DefinitionSnapshotApply, nil
}

func (c DefinitionSnapshotCursor) Validate() error {
	if err := c.Application.Validate(); err != nil {
		return err
	}
	if err := c.PublisherFence.Validate(); err != nil {
		return err
	}
	if c.Revision <= 0 || !validSHA256(c.ContentSHA256) {
		return ErrDefinitionSnapshotConflict
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

// DefinitionSnapshotContentSHA256 hashes the normalized definition set in key
// order, so transport ordering cannot create a false same-revision conflict.
func DefinitionSnapshotContentSHA256(definitions []Definition) (string, error) {
	canonical := make([]Definition, len(definitions))
	copy(canonical, definitions)
	seen := make(map[string]struct{}, len(canonical))
	for index := range canonical {
		canonical[index] = canonical[index].Normalize()
		if err := canonical[index].Validate(); err != nil {
			return "", err
		}
		key := strings.TrimSpace(canonical[index].Key)
		if _, exists := seen[key]; exists {
			return "", fmt.Errorf("Scheduler definition snapshot repeats %q", key)
		}
		seen[key] = struct{}{}
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].Key < canonical[j].Key })
	raw, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("%x", digest[:]), nil
}

// ValidateModuleDefinitionSnapshot validates an in-process Module publication
// without applying SaaS session fencing. Module Reconcile calls are serialized
// by the binding and Revision is local to that Runtime process; after a process
// restart revision 1 is valid and must not be compared with the prior process's
// counter. Durable definition revisions and content remain authoritative.
func ValidateModuleDefinitionSnapshot(snapshot DefinitionSnapshot) error {
	if snapshot.PublisherSession != nil {
		return fmt.Errorf("module Scheduler definition snapshots must not carry a SaaS publisher session")
	}
	if snapshot.Revision <= 0 {
		return fmt.Errorf("Scheduler definition snapshot revision must be positive")
	}
	_, err := DefinitionSnapshotContentSHA256(snapshot.Definitions)
	return err
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
	modulecapability.Binding
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
	DeadLetters(context.Context, int) ([]DeadLetter, error)
	DeadLetter(context.Context, string) (DeadLetter, error)
	ResolveDeadLetter(context.Context, string, string) (DeadLetter, error)
	RequeueDeadLetter(context.Context, string, string) (Run, error)
	Start(context.Context, WorkerConfig) <-chan struct{}
	Close(context.Context) error
}

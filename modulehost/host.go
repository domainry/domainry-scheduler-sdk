package modulehost

import (
	"context"
	"time"

	ormmigration "github.com/domainry/domainry-orm/migration"
	"github.com/domainry/domainry-orm/sqlhost"
	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

// Host exposes only configuration, durable Scheduler state and downstream
// dispatch. Scheduler never receives Runtime stores or owner services.
type Host interface {
	Definitions() DefinitionProvider
	Dispatcher() Dispatcher
	HTTPConnections() HTTPConnectionProvider
}

// ModuleHost is the complete in-process Scheduler host. Scheduler owns its
// schema, migrations and repositories while borrowing the Runtime pool and
// migration ledger. The host continues to own and close the pool.
type ModuleHost interface {
	Host
	Database() Database
	Dialect() Dialect
	Migrations() MigrationRegistrar
	WorkerID() string
}

type Executor = sqlhost.Executor
type Queryer = sqlhost.Queryer
type Database = sqlhost.Database

// Dialect is the host-selected ORM renderer. Persistence repositories consume
// this semantic port and never inspect driver names or construct dialects.
type Dialect interface {
	Identifier(string) string
	Table(string) string
	Placeholder(int) string
	Insert(string, []string) string
}

type SchemaMigration = ormmigration.Migration
type SchemaBaseline = ormmigration.Baseline
type SchemaTable = ormmigration.Table
type SchemaColumn = ormmigration.Column
type SchemaIndex = ormmigration.Index

type MigrationRegistrar interface {
	Driver() string
	Schema() string
	ApplyOwnedMigrations(context.Context, string, []SchemaMigration) error
}

type DefinitionProvider interface {
	Snapshot(context.Context) (schedulersdk.DefinitionSnapshot, error)
}

type DueTrigger struct {
	Definition   schedulersdk.Definition
	ScheduledFor time.Time
	// ExpectedCursor is the durable definition cursor observed by Due. Claim
	// compares it atomically before advancing. It is empty for manual and retry
	// claims, which never move the recurrence cursor.
	ExpectedCursor time.Time
	// NextRunAt is set when a misfire decision fast-forwards the durable
	// cursor after this execution. A zero value advances one normal window.
	NextRunAt time.Time
	Metadata  []byte
}

// RunStore owns cursor, unique-window claim and terminal dispatch evidence.
// Claim must be atomic on Definition.Key + ScheduledFor.
type RunStore interface {
	Reconcile(context.Context, schedulersdk.Definition, time.Time) error
	DisableMissing(context.Context, []string, int64) error
	Due(context.Context, time.Time, int) ([]DueTrigger, error)
	Claim(context.Context, DueTrigger, time.Duration) (schedulersdk.Run, bool, error)
	// Renew extends a live lease. The returned boolean is false when ownership
	// was lost; callers must cancel dispatch and must not commit its result.
	Renew(context.Context, schedulersdk.Run, time.Duration) (schedulersdk.Run, bool, error)
	Accept(context.Context, schedulersdk.Run, schedulersdk.DownstreamReceipt) error
	Fail(context.Context, schedulersdk.Run, error, time.Time) error
	List(context.Context, int) ([]schedulersdk.Run, error)
	Get(context.Context, string) (schedulersdk.Run, error)
	Retry(context.Context, string, string) (schedulersdk.Run, error)
	Cancel(context.Context, string, string) (schedulersdk.Run, error)
	DeadLetters(context.Context, int) ([]schedulersdk.DeadLetter, error)
	DeadLetter(context.Context, string) (schedulersdk.DeadLetter, error)
	ResolveDeadLetter(context.Context, string, string) (schedulersdk.DeadLetter, error)
	RequeueDeadLetter(context.Context, string, string) (schedulersdk.Run, error)
	Reschedule(context.Context, string, time.Time, string) error
}

// TriggerBacklogStore is an optional Scheduler persistence capability. Its
// implementation must serialize the limit check with new trigger claims in the
// database; an in-process counter is insufficient for multi-instance workers.
type TriggerBacklogStore interface {
	ConfigureTriggerBacklogLimit(int) error
	TriggerBacklog(context.Context) (schedulersdk.TriggerBacklog, error)
}

type Dispatcher interface {
	Dispatch(context.Context, schedulersdk.Trigger) (schedulersdk.DownstreamReceipt, error)
}

// HTTPConnectionProvider resolves deployment-owned endpoints and credentials.
// Scheduler definitions only carry connection and operation keys.
type HTTPConnectionProvider interface {
	ResolveHTTPConnection(context.Context, string) (HTTPConnection, error)
}

type HTTPConnection struct {
	BaseURL    string
	ClientID   string
	Secret     []byte
	Auth       string
	Operations map[string]HTTPOperation
}

type HTTPOperation struct {
	Path             string
	Method           string
	Timeout          time.Duration
	Headers          map[string]string
	MaxResponseBytes int64
}

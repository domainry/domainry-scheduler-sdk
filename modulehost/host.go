package modulehost

import (
	"context"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

// Host exposes only configuration, durable Scheduler state and downstream
// dispatch. Scheduler never receives Runtime stores or owner services.
type Host interface {
	Definitions() DefinitionProvider
	Runs() RunStore
	Dispatcher() Dispatcher
	HTTPConnections() HTTPConnectionProvider
}

type DefinitionProvider interface {
	Snapshot(context.Context) (schedulersdk.DefinitionSnapshot, error)
}

type DueTrigger struct {
	Definition   schedulersdk.Definition
	ScheduledFor time.Time
}

// RunStore owns cursor, unique-window claim and terminal dispatch evidence.
// Claim must be atomic on Definition.Key + ScheduledFor.
type RunStore interface {
	Reconcile(context.Context, schedulersdk.Definition, time.Time) error
	DisableMissing(context.Context, []string, int64) error
	Due(context.Context, time.Time, int) ([]DueTrigger, error)
	Claim(context.Context, DueTrigger, time.Duration) (schedulersdk.Run, bool, error)
	Accept(context.Context, schedulersdk.Run, schedulersdk.DownstreamReceipt) error
	Fail(context.Context, schedulersdk.Run, error, time.Time) error
	List(context.Context, int) ([]schedulersdk.Run, error)
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

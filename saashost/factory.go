package saashost

import (
	"context"
	"errors"
	"time"

	"github.com/domainry/domainry-foundation/modulecapability"
	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
	"github.com/domainry/domainry-scheduler-sdk/modulehost"
)

var ErrApplicationBindingMismatch = errors.New("Scheduler SaaS credential application binding mismatch")

// Transport is implemented by an authenticated SaaS protocol client. Private
// downstreams may still be reached through a Runtime callback operation; that
// routing decision is carried by TargetRef.DispatchMode.
type Transport interface {
	modulecapability.Binding
	Descriptor(context.Context, schedulersdk.ApplicationRef) (schedulersdk.Descriptor, error)
	Reconcile(context.Context, schedulersdk.ApplicationRef, schedulersdk.DefinitionSnapshot) error
	Preview(context.Context, schedulersdk.ApplicationRef, schedulersdk.Schedule, time.Time, int) ([]time.Time, error)
	Tick(context.Context, schedulersdk.ApplicationRef, time.Time, int) (int, error)
	TriggerNow(context.Context, schedulersdk.ApplicationRef, string, string) (schedulersdk.Run, error)
	Reschedule(context.Context, schedulersdk.ApplicationRef, string, time.Time, string) error
	Runs(context.Context, schedulersdk.ApplicationRef, int) ([]schedulersdk.Run, error)
	Run(context.Context, schedulersdk.ApplicationRef, string) (schedulersdk.Run, error)
	RetryRun(context.Context, schedulersdk.ApplicationRef, string, string) (schedulersdk.Run, error)
	CancelRun(context.Context, schedulersdk.ApplicationRef, string, string) (schedulersdk.Run, error)
	DeadLetters(context.Context, schedulersdk.ApplicationRef, int) ([]schedulersdk.DeadLetter, error)
	DeadLetter(context.Context, schedulersdk.ApplicationRef, string) (schedulersdk.DeadLetter, error)
	ResolveDeadLetter(context.Context, schedulersdk.ApplicationRef, string, string) (schedulersdk.DeadLetter, error)
	RequeueDeadLetter(context.Context, schedulersdk.ApplicationRef, string, string) (schedulersdk.Run, error)
	Close(context.Context, schedulersdk.ApplicationRef) error
}

// DefinitionPublicationTransport is required by the fenced SaaS protocol. A
// remote Binding claims one session when it opens and attaches it to every
// Reconcile snapshot. It must reject a SaaS descriptor that does not advertise
// CapabilityDefinitionPublicationFencing; falling back to bare Revision is not
// permitted. Module bindings do not use this transport session.
type DefinitionPublicationTransport interface {
	BeginDefinitionPublisherSession(context.Context, schedulersdk.ApplicationRef) (schedulersdk.DefinitionPublisherSession, error)
}

// ApplicationBindingTransport lets a remote Factory bind an endpoint+token
// transport exactly once to the ApplicationRef supplied to OpenSaaS. This
// avoids a second deploy-time Runtime ID while preventing later path drift.
type ApplicationBindingTransport interface {
	BindApplication(schedulersdk.ApplicationRef) error
}

type Factory interface {
	OpenSaaS(context.Context, schedulersdk.ApplicationRef, modulehost.Host) (schedulersdk.Binding, error)
}

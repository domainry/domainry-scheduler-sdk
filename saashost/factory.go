package saashost

import (
	"context"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
	"github.com/domainry/domainry-scheduler-sdk/modulehost"
)

// Transport is implemented by an authenticated SaaS protocol client. Private
// downstreams may still be reached through a Runtime callback operation; that
// routing decision is carried by TargetRef.DispatchMode.
type Transport interface {
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

type Factory interface {
	OpenSaaS(context.Context, schedulersdk.ApplicationRef, modulehost.Host) (schedulersdk.Binding, error)
}

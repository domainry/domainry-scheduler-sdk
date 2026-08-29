package saashost

import (
	"context"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

// Transport is implemented by an authenticated SaaS protocol client. Private
// downstreams may still be reached through a Runtime callback operation; that
// routing decision is carried by TargetRef.DispatchMode.
type Transport interface {
	Descriptor(context.Context, schedulersdk.ApplicationRef) (schedulersdk.Descriptor, error)
	Reconcile(context.Context, schedulersdk.ApplicationRef) error
	Preview(context.Context, schedulersdk.ApplicationRef, schedulersdk.Schedule, time.Time, int) ([]time.Time, error)
	Tick(context.Context, schedulersdk.ApplicationRef, time.Time, int) (int, error)
	TriggerNow(context.Context, schedulersdk.ApplicationRef, string, string) (schedulersdk.Run, error)
	Runs(context.Context, schedulersdk.ApplicationRef, int) ([]schedulersdk.Run, error)
	Close(context.Context, schedulersdk.ApplicationRef) error
}

type Factory interface {
	OpenSaaS(context.Context, schedulersdk.ApplicationRef, Transport) (schedulersdk.Binding, error)
}

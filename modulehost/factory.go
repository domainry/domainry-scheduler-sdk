package modulehost

import (
	"context"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

// Factory opens an in-process Scheduler owner against explicit host ports.
type Factory interface {
	OpenModule(context.Context, schedulersdk.ApplicationRef, Host) (schedulersdk.Binding, error)
}

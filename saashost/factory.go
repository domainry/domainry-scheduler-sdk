package saashost

import (
	"context"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
	"github.com/domainry/domainry-scheduler-sdk/modulehost"
)

// Factory opens the remote topology with the same explicit host capabilities
// used by the Module topology.
type Factory interface {
	OpenSaaS(context.Context, schedulersdk.ApplicationRef, modulehost.Host) (schedulersdk.Binding, error)
}

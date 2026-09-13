package schedulersdk

import (
	"context"
	"errors"
)

const CapabilityScheduledPlanDeletionRead = "scheduled_plan_deletion_read_v1"

var ErrScheduledPlanDeletionReadUnsupported = errors.New("Scheduler plan deletion reading is unavailable")

// ScheduledPlanDeletionReader is an optional, read-only owner port. It returns
// only the neutral acknowledgement for an existing, owner-scoped tombstone.
// Missing and live plans return ErrScheduledPlanNotFound. It must never repair
// projections, publish definitions, dispatch work, or perform a deletion.
// The product authorizes the current reader and verifies the saved receipt;
// possession of a plan ID or this acknowledgement grants no data access.
type ScheduledPlanDeletionReader interface {
	ReadScheduledPlanDeletion(context.Context, ScheduledPlanLookup) (ScheduledPlanDeleteReceipt, error)
}

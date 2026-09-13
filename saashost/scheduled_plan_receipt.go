package saashost

import (
	"context"

	sdk "github.com/domainry/domainry-scheduler-sdk"
)

// ScheduledPlanDeletionTransport preserves the binding's application and the
// product-resolved owner on the optional read-only deletion receipt route.
type ScheduledPlanDeletionTransport interface {
	ReadScheduledPlanDeletion(context.Context, sdk.ApplicationRef, sdk.ScheduledPlanLookup) (sdk.ScheduledPlanDeleteReceipt, error)
}

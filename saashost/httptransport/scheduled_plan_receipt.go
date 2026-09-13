package httptransport

import (
	"context"
	"net/http"
	"net/url"

	sdk "github.com/domainry/domainry-scheduler-sdk"
	"github.com/domainry/domainry-scheduler-sdk/saashost"
)

func (t *Transport) ReadScheduledPlanDeletion(ctx context.Context, app sdk.ApplicationRef, lookup sdk.ScheduledPlanLookup) (sdk.ScheduledPlanDeleteReceipt, error) {
	var out sdk.ScheduledPlanDeleteReceipt
	query := scheduledPlanOwnerQuery(lookup.Owner)
	err := t.requestApplication(ctx, app, http.MethodGet, "plans/"+url.PathEscape(lookup.PlanID)+"/deletion-receipt?"+query.Encode(), nil, &out)
	return out, err
}

var _ saashost.ScheduledPlanDeletionTransport = (*Transport)(nil)

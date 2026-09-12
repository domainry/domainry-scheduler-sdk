package persistence

import (
	"context"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

// ScheduledPlanRecord is a fully validated Scheduler-owned persistence write.
// RequestSHA256 binds the owner-scoped ClientID to exact normalized content.
type ScheduledPlanRecord struct {
	Plan          schedulersdk.ScheduledPlan
	ClientID      string
	RequestSHA256 string
}

// ScheduledPlanRepository is Scheduler's durable plan port. Every management
// mutation is revision-CAS and remains scoped to the resolved owner.
type ScheduledPlanRepository interface {
	CreateScheduledPlan(context.Context, ScheduledPlanRecord) (schedulersdk.ScheduledPlan, bool, error)
	GetScheduledPlan(context.Context, schedulersdk.ScheduledPlanLookup) (schedulersdk.ScheduledPlan, error)
	ListScheduledPlans(context.Context, schedulersdk.ScheduledPlanList) (schedulersdk.ScheduledPlanPage, error)
	ReplaceScheduledPlan(context.Context, schedulersdk.ScheduledPlan, int64) (schedulersdk.ScheduledPlan, error)
}

// ScheduledPlanRecoveryRepository is the Scheduler clock's internal recovery
// view. It is runtime-bound by the concrete store and is never a product list
// API; G05 can add an owner-scoped management query separately.
type ScheduledPlanRecoveryRepository interface {
	ListScheduledPlansForRecovery(context.Context, string, int) ([]schedulersdk.ScheduledPlan, error)
}

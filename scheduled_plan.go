package schedulersdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ScheduledPlanTriggerOnce      = "once"
	ScheduledPlanTriggerRecurring = "recurring"
	ScheduledPlanMisfireSkip      = "skip"
	ScheduledPlanMisfireCatchOne  = "catch_up_one"
	ScheduledPlanMisfireCatchMany = "catch_up_bounded"

	ScheduledPlanStatusEnabled  = "enabled"
	ScheduledPlanStatusDisabled = "disabled"
	ScheduledPlanStatusPaused   = "paused"
	// ScheduledPlanStatusDeleted is a Scheduler-owned tombstone. Public get and
	// list operations hide it, while recovery still projects the corresponding
	// definition as disabled after a crash or restart.
	ScheduledPlanStatusDeleted = "deleted"

	ScheduledPlanDispatchContractVersion = "domainry-scheduler-plan-dispatch-v1"
)

var (
	ErrScheduledPlanInvalid  = errors.New("Scheduler plan is invalid")
	ErrScheduledPlanConflict = errors.New("Scheduler plan idempotency conflict")
	ErrScheduledPlanNotFound = errors.New("Scheduler plan was not found")
)

// ScheduledPlanOwner is the product-resolved owner of a plan. Runtime identity
// is bound by the Scheduler Binding and is deliberately absent from caller
// input so one credential or URL field cannot select another application.
type ScheduledPlanOwner struct {
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	ProductKey  string `json:"product_key"`
}

func (o ScheduledPlanOwner) Validate() error {
	workspaceID, userID, productKey := strings.TrimSpace(o.WorkspaceID), strings.TrimSpace(o.UserID), strings.TrimSpace(o.ProductKey)
	if workspaceID == "" || userID == "" || productKey == "" {
		return fmt.Errorf("%w: workspace, user, and product owner are required", ErrScheduledPlanInvalid)
	}
	if len(workspaceID) > 191 || len(userID) > 191 || len(productKey) > 191 {
		return fmt.Errorf("%w: workspace, user, and product owner must not exceed 191 bytes", ErrScheduledPlanInvalid)
	}
	return nil
}

// ScheduledPlanConversationRef links a product plan back to the conversation
// and optional Run that authored it. The reference never grants access to the
// conversation; the product must authorize it again when reading or running.
type ScheduledPlanConversationRef struct {
	ConversationID string `json:"conversation_id,omitempty"`
	RunID          string `json:"run_id,omitempty"`
}

// ScheduledPlanTrigger represents exactly one future instant or one recurring
// wall-clock rule. Scheduler stores the explicit plan timezone separately so
// presentation and later trigger evaluation do not depend on server locale.
type ScheduledPlanTrigger struct {
	Type     string     `json:"type"`
	At       *time.Time `json:"at,omitempty"`
	Schedule *Schedule  `json:"schedule,omitempty"`
	// Policy is stored with the trigger because it defines how the clock turns
	// due windows into executions. Scheduler normalizes omitted values before
	// persistence so restart behavior never depends on process defaults.
	Policy Policy `json:"policy"`
}

// ScheduledPlanDispatch is the self-contained, signed payload emitted by the
// Scheduler run pipeline. It carries resolved owner facts for G03 to
// reauthorize; possession of this payload does not itself grant access.
type ScheduledPlanDispatch struct {
	ContractVersion string                       `json:"contract_version"`
	PlanID          string                       `json:"plan_id"`
	Owner           ScheduledPlanOwner           `json:"owner"`
	Input           json.RawMessage              `json:"input"`
	AllowedActions  []string                     `json:"allowed_actions"`
	ConversationRef ScheduledPlanConversationRef `json:"conversation_ref,omitempty"`
}

// ScheduledPlanCreate is the owner-neutral create command. ClientID is scoped
// to the resolved owner and makes a retried create idempotent.
type ScheduledPlanCreate struct {
	ClientID        string                       `json:"client_id"`
	Name            string                       `json:"name"`
	Owner           ScheduledPlanOwner           `json:"owner"`
	Timezone        string                       `json:"timezone"`
	Trigger         ScheduledPlanTrigger         `json:"trigger"`
	Input           json.RawMessage              `json:"input"`
	AllowedActions  []string                     `json:"allowed_actions"`
	Target          TargetRef                    `json:"target"`
	ConversationRef ScheduledPlanConversationRef `json:"conversation_ref,omitempty"`
	Status          string                       `json:"status,omitempty"`
}

// ScheduledPlan is Scheduler-owned durable plan state. Revision starts at one
// and is reserved for G05's compare-and-swap management operations.
type ScheduledPlan struct {
	ID              string                       `json:"id"`
	Name            string                       `json:"name"`
	Owner           ScheduledPlanOwner           `json:"owner"`
	Timezone        string                       `json:"timezone"`
	Trigger         ScheduledPlanTrigger         `json:"trigger"`
	Input           json.RawMessage              `json:"input"`
	AllowedActions  []string                     `json:"allowed_actions"`
	Target          TargetRef                    `json:"target"`
	ConversationRef ScheduledPlanConversationRef `json:"conversation_ref,omitempty"`
	Status          string                       `json:"status"`
	Revision        int64                        `json:"revision"`
	CreatedAt       time.Time                    `json:"created_at"`
	UpdatedAt       time.Time                    `json:"updated_at"`
}

type ScheduledPlanReceipt struct {
	Plan   ScheduledPlan `json:"plan"`
	Replay bool          `json:"replay"`
}

type ScheduledPlanLookup struct {
	Owner  ScheduledPlanOwner `json:"owner"`
	PlanID string             `json:"plan_id"`
}

// ScheduledPlanList is an owner-scoped, stable plan query. Cursor is the last
// returned plan ID and is meaningful only with the same owner and status.
type ScheduledPlanList struct {
	Owner  ScheduledPlanOwner `json:"owner"`
	Status string             `json:"status,omitempty"`
	Cursor string             `json:"cursor,omitempty"`
	Limit  int                `json:"limit,omitempty"`
}

type ScheduledPlanPage struct {
	Items      []ScheduledPlan `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// ScheduledPlanUpdate replaces the mutable plan specification under revision
// compare-and-swap. Owner, plan ID, creation time, and current status cannot be
// changed through this command.
type ScheduledPlanUpdate struct {
	Owner            ScheduledPlanOwner           `json:"owner"`
	PlanID           string                       `json:"plan_id"`
	ExpectedRevision int64                        `json:"expected_revision"`
	Name             string                       `json:"name"`
	Timezone         string                       `json:"timezone"`
	Trigger          ScheduledPlanTrigger         `json:"trigger"`
	Input            json.RawMessage              `json:"input"`
	AllowedActions   []string                     `json:"allowed_actions"`
	Target           TargetRef                    `json:"target"`
	ConversationRef  ScheduledPlanConversationRef `json:"conversation_ref,omitempty"`
}

type ScheduledPlanStatusChange struct {
	Owner            ScheduledPlanOwner `json:"owner"`
	PlanID           string             `json:"plan_id"`
	ExpectedRevision int64              `json:"expected_revision"`
}

type ScheduledPlanDeleteReceipt struct {
	PlanID   string `json:"plan_id"`
	Revision int64  `json:"revision"`
	Deleted  bool   `json:"deleted"`
	Replay   bool   `json:"replay,omitempty"`
}

// ScheduledPlanService is an optional Scheduler Binding extension. It keeps
// plan ownership in Scheduler while allowing products to supply their resolved
// user scope and later compose management tools without importing an owner
// implementation package.
type ScheduledPlanService interface {
	CreateScheduledPlan(context.Context, ScheduledPlanCreate) (ScheduledPlanReceipt, error)
	GetScheduledPlan(context.Context, ScheduledPlanLookup) (ScheduledPlan, error)
	ListScheduledPlans(context.Context, ScheduledPlanList) (ScheduledPlanPage, error)
	UpdateScheduledPlan(context.Context, ScheduledPlanUpdate) (ScheduledPlanReceipt, error)
	PauseScheduledPlan(context.Context, ScheduledPlanStatusChange) (ScheduledPlanReceipt, error)
	ResumeScheduledPlan(context.Context, ScheduledPlanStatusChange) (ScheduledPlanReceipt, error)
	DeleteScheduledPlan(context.Context, ScheduledPlanStatusChange) (ScheduledPlanDeleteReceipt, error)
}

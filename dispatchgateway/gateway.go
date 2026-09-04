// Package dispatchgateway defines the authenticated Runtime callback boundary
// used by standalone Scheduler SaaS. It exposes trigger facts, not Runtime
// persistence or business services.
package dispatchgateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

// AcceptPath is Runtime's target-execution boundary. Scheduler remains the
// scheduling ingress and owns the trigger; Runtime only routes the resolved
// target into a Runtime-resident executor.
const AcceptPath = "/dispatch/executions"

type Request struct {
	RuntimeID      string                 `json:"runtime_id"`
	ExecutionID    string                 `json:"execution_id"`
	IdempotencyKey string                 `json:"idempotency_key"`
	DueAt          time.Time              `json:"due_at,omitempty"`
	Target         schedulersdk.TargetRef `json:"target"`
}

func (r Request) Validate(application schedulersdk.ApplicationRef) error {
	if err := application.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.RuntimeID) != application.RuntimeID || strings.TrimSpace(r.ExecutionID) == "" || strings.TrimSpace(r.IdempotencyKey) == "" {
		return fmt.Errorf("Scheduler Dispatch Gateway request identity and scope are required")
	}
	target := r.Target
	targetType := strings.TrimSpace(target.Type)
	if targetType == "" {
		targetType = "runtime_operation"
	}
	if strings.TrimSpace(target.Operation) == "" || (targetType == "runtime_operation" && strings.TrimSpace(target.Owner) == "") || (targetType == "http" && strings.TrimSpace(target.ConnectionKey) == "") {
		return fmt.Errorf("Scheduler Dispatch Gateway target is invalid")
	}
	if targetType != "runtime_operation" && targetType != "http" {
		return fmt.Errorf("Scheduler Dispatch Gateway target type %q is unsupported", targetType)
	}
	return nil
}

type Receipt struct {
	ExecutionID string `json:"execution_id"`
	ID          string `json:"id"`
	Owner       string `json:"owner"`
	Status      string `json:"status"`
}

type Gateway interface {
	Dispatch(context.Context, schedulersdk.ApplicationRef, Request) (Receipt, error)
}

// Package dispatchgateway defines the authenticated Runtime callback boundary
// used by standalone Scheduler SaaS. It exposes trigger facts, not Runtime
// persistence or business services.
package dispatchgateway

import (
	"context"
	"fmt"
	"strings"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

const AcceptPath = "/scheduler/triggers/accept"

type Request struct {
	RuntimeID string               `json:"runtime_id"`
	Trigger   schedulersdk.Trigger `json:"trigger"`
}

func (r Request) Validate(application schedulersdk.ApplicationRef) error {
	if err := application.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.RuntimeID) != application.RuntimeID || strings.TrimSpace(r.Trigger.RunID) == "" || strings.TrimSpace(r.Trigger.DefinitionKey) == "" || strings.TrimSpace(r.Trigger.IdempotencyKey) == "" {
		return fmt.Errorf("Scheduler Dispatch Gateway request identity and scope are required")
	}
	target := r.Trigger.Target
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
	RunID  string `json:"run_id"`
	ID     string `json:"id"`
	Owner  string `json:"owner"`
	Status string `json:"status"`
}

type Gateway interface {
	Dispatch(context.Context, schedulersdk.ApplicationRef, Request) (Receipt, error)
}

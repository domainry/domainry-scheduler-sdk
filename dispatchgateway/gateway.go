// Package dispatchgateway defines the authenticated Runtime callback boundary
// used by standalone Scheduler SaaS. It exposes trigger facts, not Runtime
// persistence or business services.
package dispatchgateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

// AcceptPath is Runtime's target-execution boundary. Scheduler remains the
// scheduling ingress and owns the trigger; Runtime only routes the resolved
// target into a Runtime-resident executor.
const (
	AcceptPath      = "/dispatch/executions"
	RuntimeIDHeader = "X-Domainry-Runtime-ID"
)

var (
	ErrCallbackRequestInvalid      = errors.New("Scheduler Runtime callback request is invalid")
	ErrCallbackAuthentication      = errors.New("Scheduler Runtime callback authentication failed")
	ErrCallbackRejected            = errors.New("Scheduler Runtime callback was rejected")
	ErrCallbackIdempotencyConflict = errors.New("Scheduler Runtime callback idempotency identity conflicts with accepted content")
	ErrCallbackRetryable           = errors.New("Scheduler Runtime callback failed transiently")
	ErrCallbackResponseInvalid     = errors.New("Scheduler Runtime callback response is invalid")
)

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
		return fmt.Errorf("%w: identity and scope are required", ErrCallbackRequestInvalid)
	}
	target := r.Target
	targetType := strings.TrimSpace(target.Type)
	if targetType == "" {
		targetType = "runtime_operation"
	}
	if strings.TrimSpace(target.Operation) == "" || (targetType == "runtime_operation" && strings.TrimSpace(target.Owner) == "") || (targetType == "http" && strings.TrimSpace(target.ConnectionKey) == "") {
		return fmt.Errorf("%w: target is invalid", ErrCallbackRequestInvalid)
	}
	if targetType != "runtime_operation" && targetType != "http" {
		return fmt.Errorf("%w: target type %q is unsupported", ErrCallbackRequestInvalid, targetType)
	}
	return nil
}

type Receipt struct {
	ExecutionID string `json:"execution_id"`
	ID          string `json:"id"`
	Owner       string `json:"owner"`
	Status      string `json:"status"`
	Replay      bool   `json:"replay,omitempty"`
}

type Gateway interface {
	Dispatch(context.Context, schedulersdk.ApplicationRef, Request) (Receipt, error)
}

// CallbackError is the stable transport error returned by Remote. Code is an
// optional opaque receiver code and must not contain response bodies or secrets.
type CallbackError struct {
	StatusCode int
	Code       string
	Retryable  bool
}

func (e *CallbackError) Error() string {
	if e == nil {
		return "Scheduler Runtime callback failed"
	}
	if e.Code != "" {
		return fmt.Sprintf("Scheduler Runtime callback failed with status %d (%s)", e.StatusCode, e.Code)
	}
	return fmt.Sprintf("Scheduler Runtime callback failed with status %d", e.StatusCode)
}

func (e *CallbackError) Is(target error) bool {
	if e == nil {
		return false
	}
	switch target {
	case ErrCallbackRequestInvalid:
		return e.StatusCode == 400 || e.StatusCode == 422
	case ErrCallbackAuthentication:
		return e.StatusCode == 401
	case ErrCallbackRejected:
		return e.StatusCode == 403 || e.StatusCode == 404 || e.StatusCode == 405
	case ErrCallbackIdempotencyConflict:
		return e.StatusCode == 409
	case ErrCallbackRetryable:
		return e.Retryable
	default:
		return false
	}
}

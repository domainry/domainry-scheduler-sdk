package schedule

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

const (
	maximumScheduledPlanInputBytes = 64 << 10
	maximumScheduledPlanActions    = 64
	maximumScheduledPlanAttempts   = 10
	maximumScheduledPlanDuration   = 24 * time.Hour
)

// ValidateScheduledPlanCreate owns deterministic trigger validation shared by
// Module and SaaS. Identity and action authorization remain product concerns.
func ValidateScheduledPlanCreate(ctx context.Context, value schedulersdk.ScheduledPlanCreate) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	invalid := func(message string) error { return fmt.Errorf("%w: %s", schedulersdk.ErrScheduledPlanInvalid, message) }
	if err := value.Owner.Validate(); err != nil {
		return err
	}
	if clientID := strings.TrimSpace(value.ClientID); clientID == "" || len(clientID) > 191 {
		return invalid("client ID is required and must not exceed 191 bytes")
	}
	if name := strings.TrimSpace(value.Name); name == "" || len(name) > 500 {
		return invalid("name is required and must not exceed 500 bytes")
	}
	zone := strings.TrimSpace(value.Timezone)
	if zone == "" || len(zone) > 128 {
		return invalid("timezone is required and must not exceed 128 bytes")
	}
	if zone == "Local" {
		return invalid("timezone must be an IANA location")
	}
	if _, err := time.LoadLocation(zone); err != nil {
		return invalid("timezone must be an IANA location")
	}
	switch strings.TrimSpace(value.Trigger.Type) {
	case schedulersdk.ScheduledPlanTriggerOnce:
		if value.Trigger.At == nil || value.Trigger.At.IsZero() || value.Trigger.Schedule != nil {
			return invalid("one-time trigger requires at and forbids a recurring schedule")
		}
	case schedulersdk.ScheduledPlanTriggerRecurring:
		if value.Trigger.At != nil || value.Trigger.Schedule == nil {
			return invalid("recurring trigger requires a schedule and forbids at")
		}
		rule := *value.Trigger.Schedule
		if rule.Timezone == "" {
			rule.Timezone = zone
		}
		if rule.Timezone != zone {
			return invalid("recurring schedule timezone must match the plan timezone")
		}
		if err := Validate(rule); err != nil {
			return fmt.Errorf("%w: %v", schedulersdk.ErrScheduledPlanInvalid, err)
		}
	default:
		return invalid("trigger type must be once or recurring")
	}
	policy := value.Trigger.Policy
	switch policy.Misfire {
	case "", schedulersdk.ScheduledPlanMisfireSkip, schedulersdk.ScheduledPlanMisfireCatchOne:
		if policy.MaxCatchupWindows != 0 {
			return invalid("max catch-up windows requires catch_up_bounded")
		}
	case schedulersdk.ScheduledPlanMisfireCatchMany:
		if strings.TrimSpace(value.Trigger.Type) == schedulersdk.ScheduledPlanTriggerOnce {
			return invalid("one-time trigger cannot use bounded catch-up")
		}
		if policy.MaxCatchupWindows < 1 || policy.MaxCatchupWindows > 100 {
			return invalid("bounded catch-up windows must be between 1 and 100")
		}
	default:
		return invalid("misfire policy must be skip, catch_up_one, or catch_up_bounded")
	}
	if policy.MaxAttempts < 0 || policy.MaxAttempts > maximumScheduledPlanAttempts {
		return invalid("max attempts must be between 1 and 10 when set")
	}
	if policy.MisfireGrace < 0 || policy.RetryInitial < 0 || policy.RetryMax < 0 || policy.Timeout < 0 || policy.MisfireGrace > maximumScheduledPlanDuration || policy.RetryInitial > maximumScheduledPlanDuration || policy.RetryMax > maximumScheduledPlanDuration || policy.Timeout > maximumScheduledPlanDuration {
		return invalid("retry and timeout durations must be positive and no greater than 24 hours")
	}
	if policy.RetryMax > 0 && policy.RetryInitial > policy.RetryMax {
		return invalid("retry maximum must not be shorter than retry initial")
	}
	if err := value.Target.Validate("scheduled plan"); err != nil {
		return invalid(err.Error())
	}
	if len(value.Target.Owner) > 191 || len(value.Target.Operation) > 191 || len(value.Target.ConnectionKey) > 191 || len(value.Target.DispatchMode) > 64 {
		return invalid("target identity is too long")
	}
	if len(value.Target.Payload) != 0 {
		return invalid("target payload is not allowed; use plan input")
	}
	if len(value.Input) == 0 || len(value.Input) > maximumScheduledPlanInputBytes {
		return invalid("input must be a JSON object no larger than 64 KiB")
	}
	decoder := json.NewDecoder(strings.NewReader(string(value.Input)))
	decoder.UseNumber()
	var input map[string]any
	if err := decoder.Decode(&input); err != nil || input == nil {
		return invalid("input must be a JSON object")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return invalid("input must contain exactly one JSON object")
	}
	if len(value.AllowedActions) > maximumScheduledPlanActions {
		return invalid("allowed actions exceed the limit")
	}
	seen := map[string]struct{}{}
	for _, raw := range value.AllowedActions {
		action := strings.TrimSpace(raw)
		if action == "" || len(action) > 191 {
			return invalid("allowed action is empty or too long")
		}
		if _, found := seen[action]; found {
			return invalid("allowed actions contain a duplicate")
		}
		seen[action] = struct{}{}
	}
	conversationID, runID := strings.TrimSpace(value.ConversationRef.ConversationID), strings.TrimSpace(value.ConversationRef.RunID)
	if len(conversationID) > 191 || len(runID) > 191 {
		return invalid("conversation reference is too long")
	}
	if runID != "" && conversationID == "" {
		return invalid("conversation ID is required when a Run is referenced")
	}
	switch status := strings.TrimSpace(value.Status); status {
	case "", schedulersdk.ScheduledPlanStatusEnabled, schedulersdk.ScheduledPlanStatusDisabled, schedulersdk.ScheduledPlanStatusPaused:
	default:
		return invalid("status is invalid")
	}
	return nil
}

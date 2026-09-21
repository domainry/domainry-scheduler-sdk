package authoring

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/domainry/domainry-scheduler-sdk/schedule"
)

// ManagementDefinition is Scheduler's governed business-facing definition
// projection. Durable run, cursor, event, and dead-letter state is excluded.
type ManagementDefinition struct {
	Key                 string                     `json:"key"`
	Name                string                     `json:"name,omitempty"`
	Status              string                     `json:"status"`
	ScheduleType        string                     `json:"schedule_type,omitempty"`
	ScheduleExpression  string                     `json:"schedule_expression,omitempty"`
	IntervalSeconds     int                        `json:"interval_seconds,omitempty"`
	TimeOfDay           string                     `json:"time_of_day,omitempty"`
	DayOfWeek           string                     `json:"day_of_week,omitempty"`
	DayOfMonth          int                        `json:"day_of_month,omitempty"`
	Timezone            string                     `json:"timezone,omitempty"`
	BusinessCalendarKey string                     `json:"business_calendar_key,omitempty"`
	NonWorkingDayPolicy string                     `json:"non_working_day_policy,omitempty"`
	TargetType          string                     `json:"target_type,omitempty"`
	TargetKey           string                     `json:"target_key,omitempty"`
	TargetObject        string                     `json:"target_object,omitempty"`
	RunAsRole           string                     `json:"run_as_role,omitempty"`
	ConnectionKey       string                     `json:"connection_key,omitempty"`
	MaxAttempts         int                        `json:"max_attempts,omitempty"`
	TimeoutSeconds      int                        `json:"timeout_seconds,omitempty"`
	MissedWindowPolicy  string                     `json:"missed_window_policy,omitempty"`
	MaxCatchupWindows   int                        `json:"max_catchup_windows,omitempty"`
	RetryDelaySeconds   int                        `json:"retry_delay_seconds,omitempty"`
	RetryMaxDelay       int                        `json:"retry_max_delay_seconds,omitempty"`
	PayloadJSON         string                     `json:"payload_json,omitempty"`
	Description         string                     `json:"description,omitempty"`
	I18n                map[string]json.RawMessage `json:"i18n,omitempty"`
	NextRunAt           string                     `json:"next_run_at,omitempty"`
	CreatedAt           string                     `json:"created_at,omitempty"`
	UpdatedAt           string                     `json:"updated_at,omitempty"`
}

type ManagementDefinitionVersion struct {
	VersionID string               `json:"version_id"`
	Event     string               `json:"event"`
	CreatedAt string               `json:"created_at"`
	Value     ManagementDefinition `json:"value"`
}

type ManagementAuthoringContract struct {
	ResourceType       string   `json:"resource_type"`
	StatusField        string   `json:"status_field"`
	AllowedStatuses    []string `json:"allowed_statuses"`
	MutationOwner      string   `json:"mutation_owner"`
	ValidationEndpoint string   `json:"validation_endpoint"`
}

type DefinitionProjection struct {
	Key       string
	Data      map[string]any
	CreatedAt string
	UpdatedAt string
}

func ProjectManagementDefinition(definition DefinitionProjection) ManagementDefinition {
	return ManagementDefinition{
		Key:                 valueOrID(definition.Data, "key", definition.Key),
		Name:                stringValue(definition.Data, "name"),
		Status:              stringValue(definition.Data, "status"),
		ScheduleType:        stringValue(definition.Data, "schedule_type"),
		ScheduleExpression:  schedule.FirstNonEmpty(stringValue(definition.Data, "schedule_expression"), stringValue(definition.Data, "cron_expression")),
		IntervalSeconds:     intValue(definition.Data, "interval_seconds"),
		TimeOfDay:           stringValue(definition.Data, "time_of_day"),
		DayOfWeek:           stringValue(definition.Data, "day_of_week"),
		DayOfMonth:          intValue(definition.Data, "day_of_month"),
		Timezone:            stringValue(definition.Data, "timezone"),
		BusinessCalendarKey: stringValue(definition.Data, "business_calendar_key"),
		NonWorkingDayPolicy: stringValue(definition.Data, "non_working_day_policy"),
		TargetType:          stringValue(definition.Data, "target_type"),
		TargetKey:           stringValue(definition.Data, "target_key"),
		TargetObject:        stringValue(definition.Data, "target_object"),
		RunAsRole:           stringValue(definition.Data, "run_as_role"),
		ConnectionKey:       stringValue(definition.Data, "connection_key"),
		MaxAttempts:         intValue(definition.Data, "max_attempts"),
		TimeoutSeconds:      intValue(definition.Data, "timeout_seconds"),
		MissedWindowPolicy:  stringValue(definition.Data, "missed_window_policy"),
		MaxCatchupWindows:   intValue(definition.Data, "max_catchup_windows"),
		RetryDelaySeconds:   intValue(definition.Data, "retry_delay_seconds"),
		RetryMaxDelay:       intValue(definition.Data, "retry_max_delay_seconds"),
		PayloadJSON:         stringValue(definition.Data, "payload_json"),
		Description:         stringValue(definition.Data, "description"),
		I18n:                rawMessageMap(definition.Data["i18n"]),
		NextRunAt:           stringValue(definition.Data, "next_run_at"),
		CreatedAt:           definition.CreatedAt,
		UpdatedAt:           definition.UpdatedAt,
	}
}

func ManagementContract() ManagementAuthoringContract {
	return ManagementAuthoringContract{
		ResourceType: "scheduler", StatusField: "status",
		AllowedStatuses:    []string{"enabled", "disabled", "paused", "archived"},
		MutationOwner:      "source_controlled_json",
		ValidationEndpoint: "/scheduler/definitions/validate",
	}
}

func stringValue(data map[string]any, key string) string {
	value := strings.TrimSpace(fmt.Sprint(data[key]))
	if value == "<nil>" {
		return ""
	}
	return value
}

func intValue(data map[string]any, key string) int { return schedule.Int(data[key], 0) }

func rawMessageMap(value any) map[string]json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil
	}
	return result
}

func valueOrID(data map[string]any, key, fallback string) string {
	if value := stringValue(data, key); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

package schedule

import (
	"context"
	"fmt"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

const maximumPreviewCount = 100

// PreviewDefinitionData validates a complete Scheduler definition and returns
// its next recurrence instants. It is the shared authoring behavior for Module,
// SaaS, and host compatibility surfaces.
func PreviewDefinitionData(ctx context.Context, data map[string]any, after time.Time, count int) ([]time.Time, error) {
	if err := ValidateDefinitionData(ctx, data); err != nil {
		return nil, err
	}
	return previewValidatedData(ctx, data, after, count)
}

// PreviewData validates a leaf schedule fragment and returns its next
// recurrence instants without requiring a synthetic job definition.
func PreviewData(ctx context.Context, data map[string]any, after time.Time, count int) ([]time.Time, error) {
	if err := ValidateData(ctx, data); err != nil {
		return nil, err
	}
	return previewValidatedData(ctx, data, after, count)
}

func previewValidatedData(ctx context.Context, data map[string]any, after time.Time, count int) ([]time.Time, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if count < 1 {
		count = 1
	}
	if count > maximumPreviewCount {
		count = maximumPreviewCount
	}
	result := make([]time.Time, 0, count)
	cursor := after
	for range count {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		next := Next(data, cursor)
		result = append(result, next.UTC())
		cursor = next
	}
	return result, nil
}

// PreviewSchedule previews an already-resolved schedule. Unlike authoring-map
// preview, it applies the immutable business-calendar snapshot.
func PreviewSchedule(ctx context.Context, value schedulersdk.Schedule, after time.Time, count int) ([]time.Time, error) {
	if err := Validate(value); err != nil {
		return nil, err
	}
	if count < 1 {
		count = 1
	}
	if count > maximumPreviewCount {
		count = maximumPreviewCount
	}
	result := make([]time.Time, 0, count)
	cursor := after
	for attempts := 0; len(result) < count && attempts < maximumBusinessCalendarSearchDays; attempts++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		next := NextSchedule(value, cursor)
		if next.IsZero() {
			break
		}
		cursor = next
		if value.NonWorkingDayPolicy == schedulersdk.NonWorkingDaySkip && IsNonWorkingOccurrence(value, next) {
			continue
		}
		result = append(result, next.UTC())
	}
	if len(result) < count {
		return nil, fmt.Errorf("scheduler business calendar produced no executable occurrence within the bounded preview")
	}
	return result, nil
}

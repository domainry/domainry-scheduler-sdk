package schedule

import (
	"context"
	"time"
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

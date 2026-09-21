package schedule

import (
	"testing"
	"time"
)

func TestPreviewDefinitionAndLeafScheduleShareOwnerRecurrence(t *testing.T) {
	after := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	definition := map[string]any{
		"key": "orders.sync", "name": "Orders sync", "status": "enabled",
		"target_type": "workflow", "target_key": "scheduled:orders.sync",
		"schedule_type": "interval", "interval_seconds": 300, "max_attempts": 1, "timeout_seconds": 300,
	}
	next, err := PreviewDefinitionData(t.Context(), definition, after, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 3 || !next[0].Equal(after.Add(5*time.Minute)) || !next[2].Equal(after.Add(15*time.Minute)) {
		t.Fatalf("next=%v", next)
	}
	leaf, err := PreviewData(t.Context(), map[string]any{"schedule_type": "daily_at", "time_of_day": "09:30", "timezone": "Asia/Shanghai"}, after, 1)
	if err != nil || len(leaf) != 1 || leaf[0].Format(time.RFC3339) != "2026-01-01T01:30:00Z" {
		t.Fatalf("leaf=%v err=%v", leaf, err)
	}
}

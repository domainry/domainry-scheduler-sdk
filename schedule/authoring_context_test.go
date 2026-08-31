package schedule

import (
	"context"
	"errors"
	"testing"
)

func TestValidateDataHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ValidateData(ctx, map[string]any{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

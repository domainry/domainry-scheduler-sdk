package schedule

import (
	"testing"

	"github.com/domainry/domainry-foundation/apperror"
)

func TestValidationErrorClassificationContract(t *testing.T) {
	err := &ValidationError{Code: "backend.scheduler.schedule_invalid", Params: map[string]string{"field": "schedule"}}
	if apperror.KindOf(err) != apperror.KindBadRequest {
		t.Fatalf("kind=%q", apperror.KindOf(err))
	}
	if apperror.CodeOf(err) != "backend.scheduler.schedule_invalid" || apperror.ParamsOf(err)["field"] != "schedule" {
		t.Fatalf("code=%q params=%#v", apperror.CodeOf(err), apperror.ParamsOf(err))
	}
}

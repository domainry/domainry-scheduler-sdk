package authoring

import (
	"testing"

	"github.com/domainry/domainry-scheduler-sdk/schedule"
)

func TestDomainExamplesExecuteOwnerValidators(t *testing.T) {
	domain := Domain()
	if domain.Key != "scheduler" || len(domain.Capabilities) != 7 {
		t.Fatalf("domain=%#v", domain)
	}
	for _, capability := range domain.Capabilities[:2] {
		for _, example := range capability.Examples {
			var err error
			if capability.Key == "scheduler.schedule" {
				err = schedule.ValidateData(t.Context(), example.Value)
			} else {
				err = schedule.ValidateDefinitionData(t.Context(), example.Value)
			}
			if len(example.ExpectedErrorCodes) == 0 && err != nil {
				t.Fatalf("capability=%s example=%s err=%v", capability.Key, example.Name, err)
			}
			if len(example.ExpectedErrorCodes) > 0 && schedule.ValidationCode(err) != example.ExpectedErrorCodes[0] {
				t.Fatalf("capability=%s example=%s code=%q want=%q", capability.Key, example.Name, schedule.ValidationCode(err), example.ExpectedErrorCodes[0])
			}
		}
	}
}

func TestTenantAdminProjectionAndContractAreOwnerDefined(t *testing.T) {
	definition := ProjectTenantAdminDefinition(DefinitionProjection{Key: "fallback", Data: map[string]any{
		"key": "orders.sync", "status": "enabled", "schedule_type": "cron", "cron_expression": "0 2 * * *", "max_attempts": 3,
	}})
	if definition.Key != "orders.sync" || definition.ScheduleExpression != "0 2 * * *" || definition.MaxAttempts != 3 {
		t.Fatalf("definition=%#v", definition)
	}
	contract := TenantAdminContract()
	if contract.ResourceType != "scheduler" || contract.MutationOwner != "source_controlled_json" || contract.ValidationEndpoint == "" {
		t.Fatalf("contract=%#v", contract)
	}
}

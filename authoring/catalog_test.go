package authoring

import (
	"reflect"
	"testing"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
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

func TestDomainProjectsOnlyExactPermissionsFromSchedulerActionManifest(t *testing.T) {
	actions, err := schedulersdk.SchedulerAuthorizationActions()
	if err != nil {
		t.Fatal(err)
	}
	known := make(map[string]bool, len(actions))
	for _, action := range actions {
		if action.Permission == nil || action.Permission.Key != action.Key {
			t.Fatalf("Scheduler role Action is not same-key: %+v", action)
		}
		known[action.Key] = true
	}
	for _, capability := range Domain().Capabilities {
		if capability.Execution == nil || capability.Execution.PermissionModel != exactActionPermissionModel {
			t.Fatalf("capability %q permission model=%#v", capability.Key, capability.Execution)
		}
		if len(capability.Permissions) == 0 {
			t.Fatalf("capability %q has no exact Action projection", capability.Key)
		}
		for _, permission := range capability.Permissions {
			if !known[permission] {
				t.Fatalf("capability %q references non-manifest permission %q", capability.Key, permission)
			}
		}
	}
}

func TestManagementProjectionAndContractAreOwnerDefined(t *testing.T) {
	definition := ProjectManagementDefinition(DefinitionProjection{Key: "fallback", Data: map[string]any{
		"key": "orders.sync", "status": "enabled", "schedule_type": "cron", "cron_expression": "0 2 * * *", "max_attempts": 3,
		"business_calendar_key": "operations", "non_working_day_policy": "skip",
		"i18n": map[string]any{"zh-CN": map[string]any{"name": "订单同步"}},
	}})
	if definition.Key != "orders.sync" || definition.ScheduleExpression != "0 2 * * *" || definition.MaxAttempts != 3 || definition.BusinessCalendarKey != "operations" || definition.NonWorkingDayPolicy != "skip" || string(definition.I18n["zh-CN"]) != `{"name":"订单同步"}` {
		t.Fatalf("definition=%#v", definition)
	}
	contract := ManagementContract()
	if contract.ResourceType != "scheduler" || contract.MutationOwner != "source_controlled_json" || contract.ValidationEndpoint == "" {
		t.Fatalf("contract=%#v", contract)
	}
}

func TestScheduleFragmentContractDoesNotPublishRetiredIntervalAliases(t *testing.T) {
	for _, capability := range ManagementDomain().Capabilities {
		if capability.Key != "scheduler.schedule" {
			continue
		}
		for _, retired := range []string{"interval_minutes", "interval_hours", "business_calendar_key", "non_working_day_policy"} {
			if _, exists := capability.InputSchema.Properties[retired]; exists {
				t.Fatalf("schedule fragment still publishes non-executable field %q", retired)
			}
		}
		return
	}
	t.Fatal("scheduler.schedule capability is absent")
}

func TestManagementBusinessJobPublishesResolvedBusinessCalendarReference(t *testing.T) {
	capability := schedulerBusinessJobAuthoringCapability()
	for _, field := range []string{"business_calendar_key", "non_working_day_policy"} {
		if _, exists := capability.InputSchema.Properties[field]; !exists {
			t.Fatalf("business job is missing %q", field)
		}
	}
	found := false
	for _, reference := range capability.ReferenceContracts {
		if reference.Kind == "business_calendar_key" && reference.InputJSONPointer == "/business_calendar_key" && reference.ResolverEndpoint == "/discovery/references/business_calendar_key" {
			found = true
		}
	}
	if !found {
		t.Fatalf("business calendar reference contract=%#v", capability.ReferenceContracts)
	}
}

func TestManagementBusinessJobRemainsFlattenedAndExplicitlyNamed(t *testing.T) {
	domain := ManagementDomain()
	if domain.Key != "scheduler" {
		t.Fatalf("management domain=%#v", domain)
	}
	var businessJobFound bool
	for _, capability := range domain.Capabilities {
		if capability.Key != "scheduler.business_job" {
			continue
		}
		businessJobFound = true
		if capability.InputSchema == nil {
			t.Fatal("management business job input schema is absent")
		}
		properties := capability.InputSchema.Properties
		if _, exists := properties["schedule_type"]; !exists {
			t.Fatal("management business job lost its flattened schedule_type")
		}
		for _, retired := range []string{"trigger_type", "interval_minutes", "interval_hours", "operation", "dispatch_mode", "condition_json", "retry_backoff", "idempotency_keys"} {
			if _, exists := properties[retired]; exists {
				t.Fatalf("management business job still publishes non-executable field %q", retired)
			}
		}
		if _, exists := properties["schedule"]; exists {
			t.Fatal("management business job was mislabeled with the nested Blueprint schedule")
		}
	}
	if !businessJobFound {
		t.Fatal("management business job capability is absent")
	}
	if got, want := Domain(), domain; !reflect.DeepEqual(got, want) {
		t.Fatalf("compatibility Domain differs from ManagementDomain: got=%#v want=%#v", got, want)
	}
}

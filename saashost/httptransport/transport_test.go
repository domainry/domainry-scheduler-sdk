package httptransport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/domainry/domainry-foundation/modulecapability"
	"github.com/domainry/domainry-foundation/modulecapability/contracttest"
	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
	"github.com/domainry/domainry-scheduler-sdk/saashost"
)

type handlerRoundTripper struct{ handler http.Handler }

func (transport handlerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	response := httptest.NewRecorder()
	transport.handler.ServeHTTP(response, request)
	return response.Result(), nil
}

func TestTransportBindsCredentialAndDefinitionPublicationToOneRuntime(t *testing.T) {
	fixture, err := contracttest.NewFixtureBinding("scheduler")
	if err != nil {
		t.Fatal(err)
	}
	summary, err := fixture.CapabilitySummary(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	capability, err := modulecapability.NewHTTPHandler(fixture, func(request *http.Request) error {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("capability auth=%q", request.Header.Get("Authorization"))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	session := schedulersdk.DefinitionPublisherSession{
		ContractVersion: schedulersdk.DefinitionPublicationContractVersion,
		Generation:      4,
		SessionNonce:    strings.Repeat("n", 32),
	}
	var applicationCalls int
	mux := http.NewServeMux()
	mux.Handle(modulecapability.SummaryPath, capability)
	mux.Handle(modulecapability.CategoriesPath, capability)
	mux.Handle(modulecapability.ValidationPath, capability)
	mux.HandleFunc("GET /v1/applications/runtime-a/descriptor", func(response http.ResponseWriter, request *http.Request) {
		applicationCalls++
		assertPrivateRequest(t, request)
		_ = json.NewEncoder(response).Encode(schedulersdk.Descriptor{
			ProtocolVersion: schedulersdk.ProtocolVersionV1, Mode: schedulersdk.DeploymentModeSaaS,
			Capabilities: []string{schedulersdk.CapabilityDefinitionPublicationFencing, schedulersdk.CapabilityScheduledPlanRecords},
		})
	})
	mux.HandleFunc("POST /v1/applications/runtime-a/"+DefinitionPublisherSessionsResource, func(response http.ResponseWriter, request *http.Request) {
		applicationCalls++
		assertPrivateRequest(t, request)
		_ = json.NewEncoder(response).Encode(session)
	})
	mux.HandleFunc("PUT /v1/applications/runtime-a/definitions", func(response http.ResponseWriter, request *http.Request) {
		applicationCalls++
		assertPrivateRequest(t, request)
		var snapshot schedulersdk.DefinitionSnapshot
		if err := json.NewDecoder(request.Body).Decode(&snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.Revision != 1 || snapshot.PublisherSession == nil || snapshot.PublisherSession.Generation != 4 || len(snapshot.Definitions) != 1 {
			t.Fatalf("snapshot=%#v", snapshot)
		}
		response.WriteHeader(http.StatusNoContent)
	})
	var saved schedulersdk.ScheduledPlan
	mux.HandleFunc("POST /v1/applications/runtime-a/plans", func(response http.ResponseWriter, request *http.Request) {
		applicationCalls++
		assertPrivateRequest(t, request)
		var input schedulersdk.ScheduledPlanCreate
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		saved = schedulersdk.ScheduledPlan{ID: "plan-a", Name: input.Name, Owner: input.Owner, Timezone: input.Timezone, Trigger: input.Trigger, Input: input.Input, AllowedActions: input.AllowedActions, Target: input.Target, ConversationRef: input.ConversationRef, Status: "enabled", Revision: 1}
		_ = json.NewEncoder(response).Encode(schedulersdk.ScheduledPlanReceipt{Plan: saved})
	})
	mux.HandleFunc("GET /v1/applications/runtime-a/plans/plan-a", func(response http.ResponseWriter, request *http.Request) {
		applicationCalls++
		assertPrivateRequest(t, request)
		if request.URL.Query().Get("workspace_id") != "workspace-a" || request.URL.Query().Get("user_id") != "user-a" || request.URL.Query().Get("product_key") != "agent" {
			t.Fatalf("plan owner query=%s", request.URL.RawQuery)
		}
		_ = json.NewEncoder(response).Encode(saved)
	})
	client := &http.Client{Transport: handlerRoundTripper{handler: mux}}
	transport, err := Open(t.Context(), Config{Endpoint: "https://scheduler.example", Token: "secret", Client: client, CapabilityContractSHA256: summary.Identity.ContractSHA256})
	if err != nil {
		t.Fatal(err)
	}
	owner := schedulersdk.ScheduledPlanOwner{WorkspaceID: "workspace-a", UserID: "user-a", ProductKey: "agent"}
	at := time.Date(2026, 9, 18, 1, 0, 0, 0, time.UTC)
	receipt, err := transport.CreateScheduledPlan(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, schedulersdk.ScheduledPlanCreate{ClientID: "plan-a", Name: "Friday", Owner: owner, Timezone: "Asia/Shanghai", Trigger: schedulersdk.ScheduledPlanTrigger{Type: "once", At: &at}, Input: json.RawMessage(`{}`), Target: schedulersdk.TargetRef{Type: "runtime_operation", Owner: "agent", Operation: "conversation_task_start"}})
	if err != nil || receipt.Plan.ID != "plan-a" {
		t.Fatalf("plan receipt=%+v err=%v", receipt, err)
	}
	loaded, err := transport.GetScheduledPlan(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, schedulersdk.ScheduledPlanLookup{Owner: owner, PlanID: "plan-a"})
	if err != nil || loaded.ID != "plan-a" || loaded.Owner != owner {
		t.Fatalf("loaded plan=%+v err=%v", loaded, err)
	}
	contracttest.VerifyBinding(t, transport)
	if _, err := transport.Descriptor(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}); err != nil {
		t.Fatal(err)
	}
	claimed, err := transport.BeginDefinitionPublisherSession(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"})
	if err != nil || claimed.Generation != session.Generation || claimed.SessionNonce != session.SessionNonce {
		t.Fatalf("session=%+v err=%v", claimed, err)
	}
	definition := schedulersdk.Definition{Key: "daily", Revision: "v1", Schedule: schedulersdk.Schedule{Type: "interval"}, Target: schedulersdk.TargetRef{Type: "runtime_operation", Owner: "workflow", Operation: "daily"}}
	err = transport.Reconcile(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, schedulersdk.DefinitionSnapshot{PublisherSession: &claimed, Revision: 1, Definitions: []schedulersdk.Definition{definition}})
	if err != nil {
		t.Fatal(err)
	}
	before := applicationCalls
	if _, err := transport.Descriptor(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-b"}); !errors.Is(err, saashost.ErrApplicationBindingMismatch) || applicationCalls != before {
		t.Fatalf("cross-Runtime request reached transport: calls=%d before=%d err=%v", applicationCalls, before, err)
	}
}

func TestTransportMapsScheduledPlanErrorsWithoutCopyingResponseBody(t *testing.T) {
	fixture, err := contracttest.NewFixtureBinding("scheduler")
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := fixture.CapabilitySummary(t.Context())
	capability, _ := modulecapability.NewHTTPHandler(fixture, func(*http.Request) error { return nil })
	mux := http.NewServeMux()
	mux.Handle(modulecapability.SummaryPath, capability)
	mux.Handle(modulecapability.CategoriesPath, capability)
	mux.Handle(modulecapability.ValidationPath, capability)
	mux.HandleFunc("POST /v1/applications/runtime-a/plans", func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "private database detail", http.StatusConflict)
	})
	mux.HandleFunc("GET /v1/applications/runtime-a/plans/missing", func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "private database detail", http.StatusNotFound)
	})
	transport, err := Open(t.Context(), Config{Endpoint: "https://scheduler.example", Token: "secret", Client: &http.Client{Transport: handlerRoundTripper{handler: mux}}, CapabilityContractSHA256: summary.Identity.ContractSHA256})
	if err != nil {
		t.Fatal(err)
	}
	application := schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}
	if _, err := transport.CreateScheduledPlan(t.Context(), application, schedulersdk.ScheduledPlanCreate{}); !errors.Is(err, schedulersdk.ErrScheduledPlanConflict) || strings.Contains(err.Error(), "database") {
		t.Fatalf("create conflict err=%v", err)
	}
	if _, err := transport.GetScheduledPlan(t.Context(), application, schedulersdk.ScheduledPlanLookup{PlanID: "missing"}); !errors.Is(err, schedulersdk.ErrScheduledPlanNotFound) || strings.Contains(err.Error(), "database") {
		t.Fatalf("get missing err=%v", err)
	}
}

func TestTransportRejectsUnboundCredentialAndUnfencedReconcile(t *testing.T) {
	for _, config := range []Config{
		{Endpoint: "https://scheduler.example"},
	} {
		if _, err := Open(t.Context(), config); err == nil {
			t.Fatalf("config=%+v was accepted", config)
		}
	}
	fixture, err := contracttest.NewFixtureBinding("scheduler")
	if err != nil {
		t.Fatal(err)
	}
	summary, err := fixture.CapabilitySummary(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	capability, err := modulecapability.NewHTTPHandler(fixture, func(*http.Request) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(modulecapability.SummaryPath, capability)
	mux.Handle(modulecapability.CategoriesPath, capability)
	mux.Handle(modulecapability.ValidationPath, capability)
	mux.HandleFunc("GET /v1/applications/runtime-a/descriptor", func(response http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(response).Encode(schedulersdk.Descriptor{ProtocolVersion: schedulersdk.ProtocolVersionV1, Mode: schedulersdk.DeploymentModeSaaS})
	})
	transport, err := Open(t.Context(), Config{Endpoint: "https://scheduler.example", Token: "secret", Client: &http.Client{Transport: handlerRoundTripper{handler: mux}}, CapabilityContractSHA256: summary.Identity.ContractSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transport.Descriptor(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}); err != schedulersdk.ErrDefinitionPublicationCapabilityRequired {
		t.Fatalf("unfenced descriptor err=%v", err)
	}
	err = transport.Reconcile(t.Context(), schedulersdk.ApplicationRef{RuntimeID: "runtime-a"}, schedulersdk.DefinitionSnapshot{Revision: 1})
	if err != schedulersdk.ErrDefinitionPublicationRequired {
		t.Fatalf("unfenced reconcile err=%v", err)
	}
}

func assertPrivateRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer secret" || request.Header.Get("X-Domainry-Runtime-ID") != "" {
		t.Fatalf("auth=%q runtime_header=%q", request.Header.Get("Authorization"), request.Header.Get("X-Domainry-Runtime-ID"))
	}
	if request.Body != nil {
		payload, _ := io.ReadAll(request.Body)
		request.Body = io.NopCloser(strings.NewReader(string(payload)))
	}
}

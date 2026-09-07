package httptransport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
			Capabilities: []string{schedulersdk.CapabilityDefinitionPublicationFencing},
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
	client := &http.Client{Transport: handlerRoundTripper{handler: mux}}
	transport, err := Open(t.Context(), Config{Endpoint: "https://scheduler.example", Token: "secret", Client: client, CapabilityContractSHA256: summary.Identity.ContractSHA256})
	if err != nil {
		t.Fatal(err)
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

package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/domainry/domainry-foundation/modulecapability"
	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
	"github.com/domainry/domainry-scheduler-sdk/saashost"
)

const maxResponseBytes int64 = 1 << 20

const DefinitionPublisherSessionsResource = "definition-publication-sessions"

type Config struct {
	Endpoint                 string
	Token                    string
	Client                   *http.Client
	CapabilityContractSHA256 string
}

func ConfigFromEnvironment() Config {
	return Config{
		Endpoint: strings.TrimSpace(os.Getenv("SCHEDULER_SAAS_ENDPOINT")), Token: strings.TrimSpace(os.Getenv("SCHEDULER_SAAS_TOKEN")),
		CapabilityContractSHA256: strings.TrimSpace(os.Getenv("SCHEDULER_CAPABILITY_CONTRACT_SHA256")),
	}
}

type Transport struct {
	endpoint    *url.URL
	token       string
	mu          sync.Mutex
	application schedulersdk.ApplicationRef
	bound       bool
	client      *http.Client
	capability  modulecapability.Binding
}

func Open(ctx context.Context, config Config) (*Transport, error) {
	endpoint, err := url.Parse(strings.TrimSpace(config.Endpoint))
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" {
		return nil, fmt.Errorf("Scheduler SaaS endpoint is invalid")
	}
	client := config.Client
	if client == nil {
		client = http.DefaultClient
	}
	token := strings.TrimSpace(config.Token)
	if token == "" {
		return nil, fmt.Errorf("Scheduler SaaS private credential is required")
	}
	capability, err := modulecapability.OpenRemote(ctx, modulecapability.RemoteConfig{
		BaseURL: strings.TrimRight(endpoint.String(), "/"), Client: client, ExpectedModuleKey: "scheduler", ExpectedContractSHA256: config.CapabilityContractSHA256,
		Authorize: func(request *http.Request) error {
			request.Header.Set("Authorization", "Bearer "+token)
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	return &Transport{endpoint: endpoint, token: token, client: client, capability: capability}, nil
}

func (t *Transport) CapabilitySummary(ctx context.Context) (modulecapability.ModuleSummary, error) {
	return t.capability.CapabilitySummary(ctx)
}
func (t *Transport) CapabilityCategory(ctx context.Context, key string) (modulecapability.CategoryDocument, error) {
	return t.capability.CapabilityCategory(ctx, key)
}
func (t *Transport) ValidateCapabilityCandidate(ctx context.Context, request modulecapability.ValidationRequest) (modulecapability.ValidationResult, error) {
	return t.capability.ValidateCapabilityCandidate(ctx, request)
}

func (t *Transport) Descriptor(ctx context.Context, app schedulersdk.ApplicationRef) (schedulersdk.Descriptor, error) {
	var out schedulersdk.Descriptor
	err := t.requestApplication(ctx, app, http.MethodGet, "descriptor", nil, &out)
	if err == nil && !out.Supports(schedulersdk.CapabilityDefinitionPublicationFencing) {
		err = schedulersdk.ErrDefinitionPublicationCapabilityRequired
	}
	return out, err
}
func (t *Transport) Reconcile(ctx context.Context, app schedulersdk.ApplicationRef, snapshot schedulersdk.DefinitionSnapshot) error {
	if snapshot.PublisherSession == nil {
		return schedulersdk.ErrDefinitionPublicationRequired
	}
	return t.requestApplication(ctx, app, http.MethodPut, "definitions", snapshot, nil)
}
func (t *Transport) BeginDefinitionPublisherSession(ctx context.Context, app schedulersdk.ApplicationRef) (schedulersdk.DefinitionPublisherSession, error) {
	var out schedulersdk.DefinitionPublisherSession
	err := t.requestApplication(ctx, app, http.MethodPost, DefinitionPublisherSessionsResource, nil, &out)
	if err != nil {
		return schedulersdk.DefinitionPublisherSession{}, err
	}
	if err := out.Validate(); err != nil {
		return schedulersdk.DefinitionPublisherSession{}, fmt.Errorf("Scheduler SaaS returned an invalid definition publisher session: %w", err)
	}
	return out, nil
}
func (t *Transport) Preview(ctx context.Context, app schedulersdk.ApplicationRef, schedule schedulersdk.Schedule, after time.Time, count int) ([]time.Time, error) {
	var out struct {
		Next []time.Time `json:"next"`
	}
	err := t.requestApplication(ctx, app, http.MethodPost, "preview", map[string]any{"schedule": schedule, "after": after, "count": count}, &out)
	return out.Next, err
}
func (t *Transport) Tick(ctx context.Context, app schedulersdk.ApplicationRef, now time.Time, limit int) (int, error) {
	var out struct {
		Processed int `json:"processed"`
	}
	err := t.requestApplication(ctx, app, http.MethodPost, "ticks", map[string]any{"now": now, "limit": limit}, &out)
	return out.Processed, err
}
func (t *Transport) TriggerNow(ctx context.Context, app schedulersdk.ApplicationRef, key, reason string) (schedulersdk.Run, error) {
	var out schedulersdk.Run
	err := t.requestApplication(ctx, app, http.MethodPost, "triggers", map[string]string{"definition_key": key, "reason": reason}, &out)
	return out, err
}
func (t *Transport) Reschedule(ctx context.Context, app schedulersdk.ApplicationRef, key string, nextRunAt time.Time, reason string) error {
	return t.requestApplication(ctx, app, http.MethodPost, "definitions/"+url.PathEscape(key)+"/reschedule", map[string]any{"next_run_at": nextRunAt, "reason": reason}, nil)
}
func (t *Transport) Runs(ctx context.Context, app schedulersdk.ApplicationRef, limit int) ([]schedulersdk.Run, error) {
	var out struct {
		Items []schedulersdk.Run `json:"items"`
	}
	err := t.requestApplication(ctx, app, http.MethodGet, "runs?limit="+strconv.Itoa(limit), nil, &out)
	return out.Items, err
}
func (t *Transport) Run(ctx context.Context, app schedulersdk.ApplicationRef, id string) (schedulersdk.Run, error) {
	var out schedulersdk.Run
	err := t.requestApplication(ctx, app, http.MethodGet, "runs/"+url.PathEscape(id), nil, &out)
	return out, err
}
func (t *Transport) RetryRun(ctx context.Context, app schedulersdk.ApplicationRef, id, reason string) (schedulersdk.Run, error) {
	var out schedulersdk.Run
	err := t.requestApplication(ctx, app, http.MethodPost, "runs/"+url.PathEscape(id)+"/retry", map[string]string{"reason": reason}, &out)
	return out, err
}
func (t *Transport) CancelRun(ctx context.Context, app schedulersdk.ApplicationRef, id, reason string) (schedulersdk.Run, error) {
	var out schedulersdk.Run
	err := t.requestApplication(ctx, app, http.MethodPost, "runs/"+url.PathEscape(id)+"/cancel", map[string]string{"reason": reason}, &out)
	return out, err
}
func (t *Transport) DeadLetters(ctx context.Context, app schedulersdk.ApplicationRef, limit int) ([]schedulersdk.DeadLetter, error) {
	var out struct {
		Items []schedulersdk.DeadLetter `json:"items"`
	}
	err := t.requestApplication(ctx, app, http.MethodGet, "dead-letters?limit="+strconv.Itoa(limit), nil, &out)
	return out.Items, err
}
func (t *Transport) DeadLetter(ctx context.Context, app schedulersdk.ApplicationRef, id string) (schedulersdk.DeadLetter, error) {
	var out schedulersdk.DeadLetter
	err := t.requestApplication(ctx, app, http.MethodGet, "dead-letters/"+url.PathEscape(id), nil, &out)
	return out, err
}
func (t *Transport) ResolveDeadLetter(ctx context.Context, app schedulersdk.ApplicationRef, id, reason string) (schedulersdk.DeadLetter, error) {
	var out schedulersdk.DeadLetter
	err := t.requestApplication(ctx, app, http.MethodPost, "dead-letters/"+url.PathEscape(id)+"/resolve", map[string]string{"reason": reason}, &out)
	return out, err
}
func (t *Transport) RequeueDeadLetter(ctx context.Context, app schedulersdk.ApplicationRef, id, reason string) (schedulersdk.Run, error) {
	var out schedulersdk.Run
	err := t.requestApplication(ctx, app, http.MethodPost, "dead-letters/"+url.PathEscape(id)+"/requeue", map[string]string{"reason": reason}, &out)
	return out, err
}
func (t *Transport) Close(ctx context.Context, app schedulersdk.ApplicationRef) error {
	return t.requestApplication(ctx, app, http.MethodDelete, "binding", nil, nil)
}

func (t *Transport) requestApplication(ctx context.Context, app schedulersdk.ApplicationRef, method, suffix string, input, output any) error {
	if err := t.BindApplication(app); err != nil {
		return err
	}
	return t.request(ctx, method, "/v1/applications/"+url.PathEscape(t.application.RuntimeID)+"/"+suffix, input, output)
}

func (t *Transport) BindApplication(application schedulersdk.ApplicationRef) error {
	application.RuntimeID = strings.TrimSpace(application.RuntimeID)
	if err := application.Validate(); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.bound {
		t.application = application
		t.bound = true
		return nil
	}
	if t.application.RuntimeID != application.RuntimeID {
		return fmt.Errorf("%w: credential is already bound to Runtime %q", saashost.ErrApplicationBindingMismatch, t.application.RuntimeID)
	}
	return nil
}

func (t *Transport) request(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	endpoint := *t.endpoint
	endpoint.Path = strings.TrimRight(t.endpoint.Path, "/") + path
	endpoint.RawQuery = ""
	if index := strings.IndexByte(path, '?'); index >= 0 {
		endpoint.Path = strings.TrimRight(t.endpoint.Path, "/") + path[:index]
		endpoint.RawQuery = path[index+1:]
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Authorization", "Bearer "+t.token)
	response, err := t.client.Do(request)
	if err != nil {
		return fmt.Errorf("call Scheduler SaaS: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if int64(len(payload)) > maxResponseBytes {
		return fmt.Errorf("Scheduler SaaS response exceeds limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Scheduler SaaS returned status %d", response.StatusCode)
	}
	if output != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, output); err != nil {
			return fmt.Errorf("decode Scheduler SaaS response: %w", err)
		}
	}
	return nil
}

var _ saashost.Transport = (*Transport)(nil)
var _ saashost.DefinitionPublicationTransport = (*Transport)(nil)
var _ saashost.ApplicationBindingTransport = (*Transport)(nil)

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
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

const maxResponseBytes int64 = 1 << 20

type Client interface {
	Do(*http.Request) (*http.Response, error)
}

type Config struct {
	Endpoint string
	Token    string
	Client   Client
}

func ConfigFromEnvironment() Config {
	return Config{Endpoint: strings.TrimSpace(os.Getenv("SCHEDULER_SAAS_ENDPOINT")), Token: strings.TrimSpace(os.Getenv("SCHEDULER_SAAS_TOKEN"))}
}

type Transport struct {
	endpoint *url.URL
	token    string
	client   Client
}

func New(config Config) (*Transport, error) {
	endpoint, err := url.Parse(strings.TrimSpace(config.Endpoint))
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" {
		return nil, fmt.Errorf("Scheduler SaaS endpoint is invalid")
	}
	client := config.Client
	if client == nil {
		client = http.DefaultClient
	}
	return &Transport{endpoint: endpoint, token: strings.TrimSpace(config.Token), client: client}, nil
}

func (t *Transport) Descriptor(ctx context.Context, app schedulersdk.ApplicationRef) (schedulersdk.Descriptor, error) {
	var out schedulersdk.Descriptor
	err := t.request(ctx, http.MethodGet, t.path(app, "descriptor"), nil, &out)
	return out, err
}
func (t *Transport) Reconcile(ctx context.Context, app schedulersdk.ApplicationRef, snapshot schedulersdk.DefinitionSnapshot) error {
	return t.request(ctx, http.MethodPut, t.path(app, "definitions"), snapshot, nil)
}
func (t *Transport) Preview(ctx context.Context, app schedulersdk.ApplicationRef, schedule schedulersdk.Schedule, after time.Time, count int) ([]time.Time, error) {
	var out struct {
		Next []time.Time `json:"next"`
	}
	err := t.request(ctx, http.MethodPost, t.path(app, "preview"), map[string]any{"schedule": schedule, "after": after, "count": count}, &out)
	return out.Next, err
}
func (t *Transport) Tick(ctx context.Context, app schedulersdk.ApplicationRef, now time.Time, limit int) (int, error) {
	var out struct {
		Processed int `json:"processed"`
	}
	err := t.request(ctx, http.MethodPost, t.path(app, "ticks"), map[string]any{"now": now, "limit": limit}, &out)
	return out.Processed, err
}
func (t *Transport) TriggerNow(ctx context.Context, app schedulersdk.ApplicationRef, key, reason string) (schedulersdk.Run, error) {
	var out schedulersdk.Run
	err := t.request(ctx, http.MethodPost, t.path(app, "triggers"), map[string]string{"definition_key": key, "reason": reason}, &out)
	return out, err
}
func (t *Transport) Runs(ctx context.Context, app schedulersdk.ApplicationRef, limit int) ([]schedulersdk.Run, error) {
	var out struct {
		Items []schedulersdk.Run `json:"items"`
	}
	err := t.request(ctx, http.MethodGet, t.path(app, "runs")+"?limit="+strconv.Itoa(limit), nil, &out)
	return out.Items, err
}
func (t *Transport) Close(ctx context.Context, app schedulersdk.ApplicationRef) error {
	return t.request(ctx, http.MethodDelete, t.path(app, "binding"), nil, nil)
}

func (t *Transport) path(app schedulersdk.ApplicationRef, suffix string) string {
	return "/v1/applications/" + url.PathEscape(app.RuntimeID) + "/" + suffix
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
	if t.token != "" {
		request.Header.Set("Authorization", "Bearer "+t.token)
	}
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

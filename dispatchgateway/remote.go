package dispatchgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

type RemoteConfig struct {
	BaseURL, ServiceCredential string
	HTTPClient                 *http.Client
	RequestTimeout             time.Duration
	MaxAttempts                int
}

type Remote struct{ config RemoteConfig }

func RemoteConfigFromEnvironment() RemoteConfig {
	return RemoteConfig{
		BaseURL:           strings.TrimSpace(os.Getenv("SCHEDULER_RUNTIME_ENDPOINT")),
		ServiceCredential: strings.TrimSpace(os.Getenv("SCHEDULER_RUNTIME_SERVICE_CREDENTIAL")),
	}
}

func NewRemote(config RemoteConfig) (*Remote, error) {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	parsed, err := url.Parse(config.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || strings.TrimSpace(config.ServiceCredential) == "" {
		return nil, fmt.Errorf("Scheduler Dispatch Gateway remote configuration is invalid")
	}
	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 10 * time.Second
	}
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	return &Remote{config: config}, nil
}

func (r *Remote) Dispatch(ctx context.Context, application schedulersdk.ApplicationRef, request Request) (Receipt, error) {
	if err := request.Validate(application); err != nil {
		return Receipt{}, err
	}
	body, err := json.Marshal(request)
	if err != nil {
		return Receipt{}, err
	}
	var last error
	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		receipt, retry, callErr := r.perform(ctx, application, body)
		if callErr == nil {
			if receipt.RunID != request.Trigger.RunID || strings.TrimSpace(receipt.ID) == "" || strings.TrimSpace(receipt.Status) == "" {
				return Receipt{}, fmt.Errorf("Scheduler Dispatch Gateway receipt identity mismatch")
			}
			return receipt, nil
		}
		last = callErr
		if !retry || attempt == r.config.MaxAttempts {
			return Receipt{}, callErr
		}
		select {
		case <-ctx.Done():
			return Receipt{}, ctx.Err()
		case <-time.After(time.Duration(attempt) * 100 * time.Millisecond):
		}
	}
	return Receipt{}, last
}

func (r *Remote) perform(parent context.Context, application schedulersdk.ApplicationRef, body []byte) (Receipt, bool, error) {
	ctx, cancel := context.WithTimeout(parent, r.config.RequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.config.BaseURL+AcceptPath, bytes.NewReader(body))
	if err != nil {
		return Receipt{}, false, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Domainry-Service-Credential", r.config.ServiceCredential)
	request.Header.Set("X-Domainry-Runtime-ID", application.RuntimeID)
	response, err := r.config.HTTPClient.Do(request)
	if err != nil {
		return Receipt{}, true, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		retry := response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
		return Receipt{}, retry, fmt.Errorf("Scheduler Dispatch Gateway rejected request with status %d", response.StatusCode)
	}
	var receipt Receipt
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&receipt); err != nil {
		return Receipt{}, true, err
	}
	return receipt, false, nil
}

var _ Gateway = (*Remote)(nil)

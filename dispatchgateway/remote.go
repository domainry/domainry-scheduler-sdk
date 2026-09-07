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
	"strconv"
	"strings"
	"time"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

const maxCallbackResponseBytes int64 = 1 << 20

type RemoteConfig struct {
	BaseURL, RuntimeID, SigningSecret string
	HTTPClient                        *http.Client
	RequestTimeout                    time.Duration
	MaxAttempts                       int
}

type Remote struct{ config RemoteConfig }

func RemoteConfigFromEnvironment() RemoteConfig {
	return RemoteConfig{
		BaseURL:       strings.TrimSpace(os.Getenv("SCHEDULER_RUNTIME_ENDPOINT")),
		RuntimeID:     strings.TrimSpace(os.Getenv("SCHEDULER_RUNTIME_ID")),
		SigningSecret: strings.TrimSpace(os.Getenv("SCHEDULER_RUNTIME_SIGNING_SECRET")),
	}
}

func NewRemote(config RemoteConfig) (*Remote, error) {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	config.RuntimeID = strings.TrimSpace(config.RuntimeID)
	parsed, err := url.Parse(config.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		config.RuntimeID == "" || strings.TrimSpace(config.SigningSecret) == "" {
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
	if strings.TrimSpace(application.RuntimeID) != r.config.RuntimeID {
		return Receipt{}, fmt.Errorf("%w: callback credential is bound to Runtime %q", ErrCallbackAuthentication, r.config.RuntimeID)
	}
	if err := request.Validate(application); err != nil {
		return Receipt{}, err
	}
	body, err := json.Marshal(request)
	if err != nil {
		return Receipt{}, err
	}
	var last error
	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		receipt, retry, callErr := r.perform(ctx, request.IdempotencyKey, body)
		if callErr == nil {
			if receipt.ExecutionID != request.ExecutionID || strings.TrimSpace(receipt.ID) == "" || strings.TrimSpace(receipt.Status) == "" {
				return Receipt{}, ErrCallbackResponseInvalid
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

func (r *Remote) perform(parent context.Context, idempotencyKey string, body []byte) (Receipt, bool, error) {
	ctx, cancel := context.WithTimeout(parent, r.config.RequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.config.BaseURL+AcceptPath, bytes.NewReader(body))
	if err != nil {
		return Receipt{}, false, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(RuntimeIDHeader, r.config.RuntimeID)
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	signedRequest := SignedRequest{Method: http.MethodPost, Path: AcceptPath, RuntimeID: r.config.RuntimeID, IdempotencyKey: idempotencyKey}
	signature, err := SignRequest(body, signedRequest, SchedulerClientID, timestamp, []byte(r.config.SigningSecret))
	if err != nil {
		return Receipt{}, false, err
	}
	request.Header.Set(ClientIDHeader, SchedulerClientID)
	request.Header.Set(TimestampHeader, timestamp)
	request.Header.Set(SignatureVersionHeader, CallbackSignatureContractVersion)
	request.Header.Set(SignatureHeader, signature)
	response, err := r.config.HTTPClient.Do(request)
	if err != nil {
		return Receipt{}, true, fmt.Errorf("%w: %w", ErrCallbackRetryable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		callbackErr := decodeCallbackError(response)
		return Receipt{}, callbackErr.Retryable, callbackErr
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxCallbackResponseBytes+1))
	if err != nil || int64(len(payload)) > maxCallbackResponseBytes {
		return Receipt{}, false, ErrCallbackResponseInvalid
	}
	var receipt Receipt
	if err := json.Unmarshal(payload, &receipt); err != nil {
		return Receipt{}, false, fmt.Errorf("%w: %w", ErrCallbackResponseInvalid, err)
	}
	return receipt, false, nil
}

func decodeCallbackError(response *http.Response) *CallbackError {
	retryable := response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
	var envelope struct {
		Code string `json:"code"`
	}
	payload, _ := io.ReadAll(io.LimitReader(response.Body, maxCallbackResponseBytes))
	_ = json.Unmarshal(payload, &envelope)
	return &CallbackError{StatusCode: response.StatusCode, Code: safeCallbackCode(envelope.Code), Retryable: retryable}
}

func safeCallbackCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '.' && character != '_' && character != '-' {
			return ""
		}
	}
	return value
}

var _ Gateway = (*Remote)(nil)

package dispatchgateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	pathpkg "path"
	"strconv"
	"strings"
	"time"
)

const (
	SchedulerClientID                = "scheduler"
	ClientIDHeader                   = "X-Client-ID"
	TimestampHeader                  = "X-Timestamp"
	SignatureHeader                  = "X-Signature"
	SignatureVersionHeader           = "X-Signature-Version"
	CallbackSignatureContractVersion = "domainry-scheduler-runtime-callback-hmac-v2"
	MaxClockSkew                     = 5 * time.Minute
)

var (
	ErrCallbackSignatureInvalid = errors.New("Scheduler Runtime callback signature is invalid")
	ErrCallbackSignatureStale   = errors.New("Scheduler Runtime callback signature is stale")
)

type Signature struct {
	Version   string
	ClientID  string
	Timestamp string
	Value     string
}

// SignedRequest contains every request attribute covered by the v2 HMAC. The
// Runtime-ID header is internal callback consistency evidence only; it never
// selects a tenant or overrides the authenticated application binding.
type SignedRequest struct {
	Method         string
	Path           string
	RuntimeID      string
	IdempotencyKey string
}

func (r SignedRequest) Validate() error {
	method := strings.TrimSpace(r.Method)
	callbackPath := strings.TrimSpace(r.Path)
	idempotencyKey := strings.TrimSpace(r.IdempotencyKey)
	if method == "" || method != r.Method || method != strings.ToUpper(method) || callbackPath == "" || callbackPath != r.Path || !strings.HasPrefix(callbackPath, "/") ||
		strings.ContainsAny(callbackPath, "?#") || pathpkg.Clean(callbackPath) != callbackPath ||
		strings.TrimSpace(r.RuntimeID) == "" || strings.TrimSpace(r.RuntimeID) != r.RuntimeID ||
		idempotencyKey == "" || idempotencyKey != r.IdempotencyKey || len(idempotencyKey) > 191 {
		return ErrCallbackRequestInvalid
	}
	return nil
}

func SignRequest(body []byte, request SignedRequest, clientID, timestamp string, secret []byte) (string, error) {
	if err := request.Validate(); err != nil {
		return "", err
	}
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientID) != clientID ||
		strings.TrimSpace(timestamp) == "" || strings.TrimSpace(timestamp) != timestamp || len(secret) == 0 {
		return "", ErrCallbackSignatureInvalid
	}
	bodyHash := sha256.Sum256(body)
	canonical := strings.Join([]string{
		CallbackSignatureContractVersion,
		request.Method,
		request.Path,
		request.RuntimeID,
		clientID,
		timestamp,
		request.IdempotencyKey,
		hex.EncodeToString(bodyHash[:]),
	}, "\n")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func VerifyRequest(body []byte, request SignedRequest, signature Signature, secret []byte, now time.Time) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if signature.Version != CallbackSignatureContractVersion || signature.ClientID != SchedulerClientID ||
		strings.TrimSpace(signature.Timestamp) == "" || strings.TrimSpace(signature.Timestamp) != signature.Timestamp ||
		strings.TrimSpace(signature.Value) == "" || strings.TrimSpace(signature.Value) != signature.Value || len(secret) == 0 {
		return ErrCallbackSignatureInvalid
	}
	if err := verifyTimestamp(signature.Timestamp, now); err != nil {
		return err
	}
	wantHex, err := SignRequest(body, request, signature.ClientID, signature.Timestamp, secret)
	if err != nil {
		return err
	}
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		return ErrCallbackSignatureInvalid
	}
	got, err := hex.DecodeString(strings.TrimSpace(signature.Value))
	if err != nil || !hmac.Equal(got, want) {
		return ErrCallbackSignatureInvalid
	}
	return nil
}

type CallbackRequestIdentity struct {
	Method         string `json:"method"`
	Path           string `json:"path"`
	RuntimeID      string `json:"runtime_id"`
	IdempotencyKey string `json:"idempotency_key"`
	BodySHA256     string `json:"body_sha256"`
}

type CallbackReplayDisposition string

const (
	CallbackReplayClaim CallbackReplayDisposition = "claim"
	CallbackReplayExact CallbackReplayDisposition = "replay"
)

func NewCallbackRequestIdentity(body []byte, request SignedRequest) (CallbackRequestIdentity, error) {
	if err := request.Validate(); err != nil {
		return CallbackRequestIdentity{}, err
	}
	digest := sha256.Sum256(body)
	return CallbackRequestIdentity{Method: request.Method, Path: request.Path, RuntimeID: request.RuntimeID, IdempotencyKey: request.IdempotencyKey, BodySHA256: hex.EncodeToString(digest[:])}, nil
}

// EvaluateCallbackReplay is the receiver-side fingerprint rule. Stores key
// durable claims by RuntimeID, Method, Path and IdempotencyKey. Exact retries
// reuse the same claim; the claim state machine returns a stored terminal
// receipt, reports an active claim as retryable, or safely reclaims an expired
// claim before invoking the downstream owner with the same idempotency key.
// Different bytes under the same scoped key are always a conflict.
func EvaluateCallbackReplay(current *CallbackRequestIdentity, incoming CallbackRequestIdentity) (CallbackReplayDisposition, error) {
	if err := incoming.validate(); err != nil {
		return "", err
	}
	if current == nil {
		return CallbackReplayClaim, nil
	}
	if err := current.validate(); err != nil {
		return "", err
	}
	if current.Method != incoming.Method || current.Path != incoming.Path || current.RuntimeID != incoming.RuntimeID || current.IdempotencyKey != incoming.IdempotencyKey {
		return "", ErrCallbackRequestInvalid
	}
	if current.BodySHA256 != incoming.BodySHA256 {
		return "", ErrCallbackIdempotencyConflict
	}
	return CallbackReplayExact, nil
}

func (i CallbackRequestIdentity) validate() error {
	if err := (SignedRequest{Method: i.Method, Path: i.Path, RuntimeID: i.RuntimeID, IdempotencyKey: i.IdempotencyKey}).Validate(); err != nil {
		return err
	}
	if len(i.BodySHA256) != sha256.Size*2 {
		return ErrCallbackRequestInvalid
	}
	if _, err := hex.DecodeString(i.BodySHA256); err != nil {
		return ErrCallbackRequestInvalid
	}
	return nil
}

// Sign implements the released v1 signature for source compatibility.
// Deprecated: v1 does not bind method, path or Runtime identity. New callback
// clients and receivers must use SignRequest and VerifyRequest together.
func Sign(body []byte, executionID, clientID, timestamp string, secret []byte) string {
	bodyHash := sha256.Sum256(body)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strings.TrimSpace(clientID) + "\n" + strings.TrimSpace(timestamp) + "\n" + strings.TrimSpace(executionID) + "\n" + hex.EncodeToString(bodyHash[:])))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify implements the released v1 verifier for rolling migration only.
// Deprecated: accept v1 only on an explicitly temporary compatibility listener,
// never on the v2 callback route used by Remote.
func Verify(body []byte, executionID string, signature Signature, secret []byte, now time.Time) error {
	if strings.TrimSpace(signature.ClientID) == "" || strings.TrimSpace(signature.Timestamp) == "" || strings.TrimSpace(signature.Value) == "" || len(secret) == 0 {
		return fmt.Errorf("Scheduler Runtime callback signature is incomplete")
	}
	if err := verifyTimestamp(signature.Timestamp, now); err != nil {
		return err
	}
	want, err := hex.DecodeString(Sign(body, executionID, signature.ClientID, signature.Timestamp, secret))
	if err != nil {
		return err
	}
	got, err := hex.DecodeString(strings.TrimSpace(signature.Value))
	if err != nil || !hmac.Equal(got, want) {
		return fmt.Errorf("Scheduler Runtime callback signature is invalid")
	}
	return nil
}

func verifyTimestamp(timestamp string, now time.Time) error {
	seconds, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return ErrCallbackSignatureInvalid
	}
	signedAt := time.Unix(seconds, 0).UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if delta := now.UTC().Sub(signedAt); delta < -MaxClockSkew || delta > MaxClockSkew {
		return ErrCallbackSignatureStale
	}
	return nil
}

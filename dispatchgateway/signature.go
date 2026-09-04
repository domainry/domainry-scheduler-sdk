package dispatchgateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	SchedulerClientID = "scheduler"
	ClientIDHeader    = "X-Client-ID"
	TimestampHeader   = "X-Timestamp"
	SignatureHeader   = "X-Signature"
	MaxClockSkew      = 5 * time.Minute
)

type Signature struct {
	ClientID  string
	Timestamp string
	Value     string
}

func Sign(body []byte, executionID, clientID, timestamp string, secret []byte) string {
	bodyHash := sha256.Sum256(body)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strings.TrimSpace(clientID) + "\n" + strings.TrimSpace(timestamp) + "\n" + strings.TrimSpace(executionID) + "\n" + hex.EncodeToString(bodyHash[:])))
	return hex.EncodeToString(mac.Sum(nil))
}

func Verify(body []byte, executionID string, signature Signature, secret []byte, now time.Time) error {
	if strings.TrimSpace(signature.ClientID) == "" || strings.TrimSpace(signature.Timestamp) == "" || strings.TrimSpace(signature.Value) == "" || len(secret) == 0 {
		return fmt.Errorf("Scheduler Runtime callback signature is incomplete")
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(signature.Timestamp), 10, 64)
	if err != nil {
		return fmt.Errorf("Scheduler Runtime callback timestamp is invalid")
	}
	signedAt := time.Unix(seconds, 0).UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if delta := now.UTC().Sub(signedAt); delta < -MaxClockSkew || delta > MaxClockSkew {
		return fmt.Errorf("Scheduler Runtime callback signature is stale")
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

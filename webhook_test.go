package mailcheckr

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestWebhookVerifier(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	verifier := NewWebhookVerifier("secret")
	verifier.now = func() time.Time { return now }
	body := []byte(`{"id":"evt_1"}`)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(append([]byte(timestamp+"."), body...))
	signature := "v1=" + hex.EncodeToString(mac.Sum(nil))
	headers := http.Header{}
	headers.Set("X-MailCheckr-Timestamp", timestamp)
	headers.Set("X-MailCheckr-Signature", signature)
	if !verifier.Verify(body, headers) {
		t.Fatal("valid webhook rejected")
	}
	if verifier.Verify([]byte(`{"id":"evt_2"}`), headers) {
		t.Fatal("altered body accepted")
	}
	if verifier.VerifyPayload(body, strconv.FormatInt(now.Add(-6*time.Minute).Unix(), 10), signature) {
		t.Fatal("stale timestamp accepted")
	}
	if verifier.VerifyPayload(body, "1e9", signature) {
		t.Fatal("invalid timestamp accepted")
	}
	if verifier.VerifyPayload(body, timestamp, "v1=bad") {
		t.Fatal("invalid signature accepted")
	}
	if NewWebhookVerifier("").Verify(body, headers) {
		t.Fatal("empty secret accepted")
	}
}

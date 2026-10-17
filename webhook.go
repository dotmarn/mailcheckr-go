package mailcheckr

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// WebhookVerifier validates a signature against the exact raw request body.
type WebhookVerifier struct {
	secret    []byte
	tolerance time.Duration
	now       func() time.Time
}

// NewWebhookVerifier uses a five-minute timestamp tolerance by default.
// A supplied tolerance must be positive.
func NewWebhookVerifier(secret string, tolerance ...time.Duration) *WebhookVerifier {
	w := &WebhookVerifier{secret: []byte(secret), tolerance: 5 * time.Minute, now: time.Now}
	if len(tolerance) > 0 {
		w.tolerance = tolerance[0]
	}
	return w
}

// Verify checks the MailCheckr headers and exact raw body bytes.
func (w *WebhookVerifier) Verify(body []byte, headers http.Header) bool {
	return w.VerifyPayload(body, headers.Get("X-MailCheckr-Timestamp"), headers.Get("X-MailCheckr-Signature"))
}

// VerifyPayload checks a timestamped v1 HMAC-SHA256 signature.
func (w *WebhookVerifier) VerifyPayload(body []byte, timestamp, signature string) bool {
	if w == nil || len(w.secret) == 0 || w.tolerance <= 0 || timestamp == "" || len(signature) != 67 || !strings.EqualFold(signature[:3], "v1=") {
		return false
	}
	for _, c := range timestamp {
		if c < '0' || c > '9' {
			return false
		}
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	delta := w.now().Sub(time.Unix(seconds, 0))
	if delta < -w.tolerance || delta > w.tolerance {
		return false
	}
	provided, err := hex.DecodeString(signature[3:])
	if err != nil || len(provided) != sha256.Size {
		return false
	}
	mac := hmac.New(sha256.New, w.secret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return hmac.Equal(provided, mac.Sum(nil))
}

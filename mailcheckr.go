// Package mailcheckr provides a server-side client for the MailCheckr API.
package mailcheckr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://mailcheckr.app/api/v1"

// Client calls the MailCheckr verification API.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client) error

// WithBaseURL sets the API base URL, primarily for private installations or tests.
func WithBaseURL(raw string) Option {
	return func(c *Client) error {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("mailcheckr: invalid base URL")
		}
		if u.Scheme != "https" && u.Scheme != "http" {
			return errors.New("mailcheckr: base URL must use HTTP or HTTPS")
		}
		c.baseURL = strings.TrimRight(raw, "/")
		return nil
	}
}

// WithHTTPClient supplies an HTTP client. Its timeout and transport are respected.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) error {
		if client == nil {
			return errors.New("mailcheckr: HTTP client cannot be nil")
		}
		c.httpClient = client
		return nil
	}
}

// NewClient creates a client using a server-side API key.
func NewClient(apiKey string, options ...Option) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(apiKey) != apiKey || strings.ContainsAny(apiKey, "\r\n") {
		return nil, errors.New("mailcheckr: API key is required")
	}
	c := &Client{apiKey: apiKey, baseURL: DefaultBaseURL, httpClient: &http.Client{Timeout: 30 * time.Second}}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("mailcheckr: option cannot be nil")
		}
		if err := option(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// Verification is a verification result. Raw contains the full API data object.
type Verification struct {
	ID          string          `json:"id"`
	Email       string          `json:"email"`
	State       string          `json:"state"`
	Status      *string         `json:"status"`
	Reason      *string         `json:"reason"`
	Checks      json.RawMessage `json:"checks"`
	Attempts    int             `json:"attempts"`
	CreatedAt   *time.Time      `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at"`
	Raw         json.RawMessage `json:"-"`
}

func (v Verification) IsCompleted() bool { return v.State == "completed" }
func (v Verification) IsPending() bool {
	return v.State == "queued" || v.State == "processing" || v.State == "retry_scheduled"
}
func (v Verification) IsFailed() bool { return v.State == "failed" }
func (v Verification) IsDeliverable() bool {
	return v.IsCompleted() && v.Status != nil && *v.Status == "deliverable"
}
func (v Verification) IsUndeliverable() bool {
	return v.IsCompleted() && v.Status != nil && *v.Status == "undeliverable"
}
func (v Verification) IsRisky() bool {
	return v.IsCompleted() && v.Status != nil && *v.Status == "risky"
}
func (v Verification) IsUnknown() bool {
	return v.IsCompleted() && v.Status != nil && *v.Status == "unknown"
}

// APIError is a non-2xx response. Response holds the decoded JSON error body when available.
type APIError struct {
	StatusCode int
	Response   map[string]any
}

func (e *APIError) Error() string {
	for _, key := range []string{"message", "code"} {
		if message, ok := e.Response[key].(string); ok && message != "" {
			return message
		}
	}
	return fmt.Sprintf("mailcheckr: request failed with HTTP %d", e.StatusCode)
}

// Verify starts or reuses a verification. Reuse the same idempotency key when retrying the same email.
func (c *Client) Verify(ctx context.Context, email, idempotencyKey string) (*Verification, error) {
	if strings.TrimSpace(email) == "" || strings.TrimSpace(idempotencyKey) == "" || strings.ContainsAny(idempotencyKey, "\r\n") {
		return nil, errors.New("mailcheckr: email and idempotency key are required")
	}
	body, err := json.Marshal(map[string]string{"email": email})
	if err != nil {
		return nil, err
	}
	return c.request(ctx, http.MethodPost, "verifications", body, idempotencyKey)
}

// Find retrieves the latest state of a verification by ID.
func (c *Client) Find(ctx context.Context, id string) (*Verification, error) {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\?#") {
		return nil, errors.New("mailcheckr: invalid verification ID")
	}
	return c.request(ctx, http.MethodGet, "verifications/"+url.PathEscape(id), nil, "")
}

func (c *Client) request(ctx context.Context, method, path string, body []byte, idempotencyKey string) (*Verification, error) {
	if ctx == nil {
		return nil, errors.New("mailcheckr: context cannot be nil")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/"+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Bound decoding so an unexpected server response cannot consume unbounded memory.
	const maxResponseBytes = 4 << 20
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxResponseBytes {
		return nil, errors.New("mailcheckr: response exceeds 4 MiB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		_ = json.Unmarshal(payload, &apiErr.Response)
		return nil, apiErr
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, errors.New("mailcheckr: unexpected API response")
	}
	var result Verification
	if err := json.Unmarshal(envelope.Data, &result); err != nil || result.ID == "" || result.State == "" {
		return nil, errors.New("mailcheckr: unexpected verification data")
	}
	result.Raw = append(json.RawMessage(nil), envelope.Data...)
	return &result, nil
}

package mailcheckr

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVerifyAndFind(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization = %q", got)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Error("missing accept header")
		}
		switch r.Method {
		case http.MethodPost:
			if r.URL.Path != "/api/v1/verifications" {
				t.Errorf("path = %q", r.URL.Path)
			}
			if r.Header.Get("Idempotency-Key") != "user-123" {
				t.Error("missing idempotency key")
			}
			body, _ := io.ReadAll(r.Body)
			if string(body) != `{"email":"person@example.com"}` {
				t.Errorf("body = %q", body)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"data":{"id":"abc","email":"person@example.com","state":"queued","status":null,"future_field":42}}`)
		case http.MethodGet:
			if r.URL.Path != "/api/v1/verifications/abc" {
				t.Errorf("path = %q", r.URL.Path)
			}
			if r.Header.Get("Idempotency-Key") != "" {
				t.Error("unexpected idempotency key")
			}
			_, _ = io.WriteString(w, `{"data":{"id":"abc","state":"completed","status":"deliverable"}}`)
		}
	})
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Result(), nil
	})
	client, err := NewClient("test-key", WithBaseURL("https://example.test/api/v1/"), WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}
	queued, err := client.Verify(context.Background(), "person@example.com", "user-123")
	if err != nil {
		t.Fatal(err)
	}
	if !queued.IsPending() || queued.IsDeliverable() || !strings.Contains(string(queued.Raw), `"future_field":42`) {
		t.Fatalf("unexpected queued result: %+v", queued)
	}
	completed, err := client.Find(context.Background(), queued.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !completed.IsCompleted() || !completed.IsDeliverable() {
		t.Fatalf("unexpected completed result: %+v", completed)
	}
}

func TestAPIErrorAndInvalidResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/bad/") {
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, `{"message":"Key reused"}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"state":"queued"}}`)
	})
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Result(), nil
	})
	httpClient := &http.Client{Transport: transport}
	client, err := NewClient("key", WithBaseURL("https://example.test"), WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Verify(context.Background(), "a@example.com", "key")
	if err == nil {
		t.Fatal("expected invalid response error")
	}
	client, _ = NewClient("key", WithBaseURL("https://example.test/bad"), WithHTTPClient(httpClient))
	_, err = client.Find(context.Background(), "id")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict || apiErr.Error() != "Key reused" {
		t.Fatalf("error = %v", err)
	}
}

func TestValidation(t *testing.T) {
	if _, err := NewClient(""); err == nil {
		t.Fatal("empty API key accepted")
	}
	if _, err := NewClient("key", WithBaseURL("file:///tmp")); err == nil {
		t.Fatal("invalid URL accepted")
	}
	client, _ := NewClient("key")
	if _, err := client.Verify(context.Background(), "", "key"); err == nil {
		t.Fatal("empty email accepted")
	}
	if _, err := client.Find(context.Background(), "../bad"); err == nil {
		t.Fatal("invalid ID accepted")
	}
}

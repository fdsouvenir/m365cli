package graph

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	d := retryDelay("2", 0, time.Second)
	if d != 2*time.Second {
		t.Fatalf("expected retry-after delay, got %v", d)
	}

	d2 := retryDelay("invalid", 1, time.Second)
	if d2 < 2*time.Second || d2 > 3*time.Second {
		t.Fatalf("expected jittered backoff near 2-3s, got %v", d2)
	}
}

func TestResolveURLAllowsSameHostAbsoluteURL(t *testing.T) {
	c := &Client{BaseURL: "https://graph.microsoft.com/v1.0"}

	got, err := c.resolveURL("https://graph.microsoft.com/v1.0/me/messages?$top=10", nil)
	if err != nil {
		t.Fatalf("resolveURL failed: %v", err)
	}
	if got != "https://graph.microsoft.com/v1.0/me/messages?$top=10" {
		t.Fatalf("unexpected URL: %s", got)
	}
}

func TestResolveURLRejectsCrossHostAbsoluteURL(t *testing.T) {
	c := &Client{BaseURL: "https://graph.microsoft.com/v1.0"}

	_, err := c.resolveURL("https://evil.example/me/messages?$top=10", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "cross-host URL is not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoShortCircuitsWhenCircuitBreakerOpen(t *testing.T) {
	cb := NewCircuitBreaker()
	for i := 0; i < CircuitBreakerThreshold; i++ {
		cb.RecordFailure()
	}

	tokenCalls := 0
	c := &Client{
		BaseURL: "https://graph.microsoft.com/v1.0",
		TokenProvider: func(context.Context, []string) (string, error) {
			tokenCalls++
			return "token", nil
		},
		HTTPClient: http.DefaultClient,
		Breaker:    cb,
	}

	_, _, err := c.Do(context.Background(), http.MethodGet, "/me", nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected circuit breaker open error")
	}

	var breakerErr *CircuitBreakerError
	if !errors.As(err, &breakerErr) {
		t.Fatalf("expected CircuitBreakerError, got %T (%v)", err, err)
	}
	if tokenCalls != 0 {
		t.Fatalf("expected token provider not to be called, got %d calls", tokenCalls)
	}
}

func TestResolveURLRejectsDowngradeAndUserinfo(t *testing.T) {
	c := &Client{BaseURL: "https://graph.microsoft.com/v1.0"}
	for _, target := range []string{"http://graph.microsoft.com/v1.0/me", "https://user@graph.microsoft.com/v1.0/me"} {
		if _, err := c.resolveURL(target, nil); err == nil {
			t.Errorf("accepted unsafe continuation %s", target)
		}
	}
}

func TestWriteServerErrorDoesNotRetry(t *testing.T) {
	calls := 0
	c := NewClient(func(context.Context, []string) (string, error) { return "token", nil })
	c.MaxRetries5xx = 1
	c.HTTPClient = &http.Client{Transport: graphRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 500, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"uncertain write"}}`))}, nil
	})}
	_, _, err := c.Do(context.Background(), "POST", "/me/sendMail", nil, map[string]any{}, nil, nil)
	if err == nil || calls != 1 {
		t.Fatalf("write must not be repeated after uncertain server failure: calls=%d err=%v", calls, err)
	}
}

type graphRoundTripFunc func(*http.Request) (*http.Response, error)

func (f graphRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

package mail

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fdsouvenir/m365cli/internal/graph"
)

type reviewTransport func(*http.Request) (*http.Response, error)

func (f reviewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type reviewBrokenBody struct{}

func (reviewBrokenBody) Read([]byte) (int, error) { return 0, fmt.Errorf("synthetic truncated body") }
func (reviewBrokenBody) Close() error             { return nil }

// A caller must not report a completed move without Graph's creation status and ID.
// A truncated confirmed response must be distinguishable from an unconfirmed write.
func TestMoveResponseContract(t *testing.T) {
	for _, tc := range []struct {
		name            string
		status          int
		payload         string
		broken, wantErr bool
	}{
		{name: "confirmed_with_id", status: 201, payload: `{"id":"new-id"}`},
		{name: "created_without_id", status: 201, payload: `{}`, wantErr: true},
		{name: "accepted_is_not_completed", status: 202, payload: `{}`, wantErr: true},
		{name: "unexpected_200", status: 200, payload: `{"id":"new-id"}`, wantErr: true},
		{name: "server_outcome_unknown", status: 503, payload: `{"error":{"message":"unavailable"}}`, wantErr: true},
		{name: "rate_limited_no_replay", status: 429, payload: `{"error":{"message":"limited"}}`, wantErr: true},
		{name: "confirmed_body_unreadable", status: 201, broken: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := graph.NewClient(func(context.Context, []string) (string, error) { return "fixture-only", nil })
			client.BaseURL = "https://synthetic.invalid/v1.0"
			client.HTTPClient = &http.Client{Transport: reviewTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/v1.0/me/messages/message-id/move" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
				}
				var body io.ReadCloser = io.NopCloser(strings.NewReader(tc.payload))
				if tc.broken {
					body = reviewBrokenBody{}
				}
				return &http.Response{StatusCode: tc.status, Header: make(http.Header), Body: body, Request: r}, nil
			})}
			got, err := New(client, "").Move(context.Background(), "message-id", "archive")
			if calls != 1 {
				t.Errorf("provider calls = %d, want 1", calls)
			}
			if tc.wantErr && err == nil {
				t.Errorf("false success: HTTP %d payload %q => %#v", tc.status, tc.payload, got)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("confirmed move rejected: %v", err)
			}
			if tc.broken && err != nil && !strings.Contains(err.Error(), "confirmed") {
				t.Errorf("lost confirmed-write status: %v", err)
			}
		})
	}
}

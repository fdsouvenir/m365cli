package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fdsouvenir/m365cli/internal/auth"
	"github.com/fdsouvenir/m365cli/internal/config"
	"github.com/fdsouvenir/m365cli/internal/secrets"
)

// Availability must reach Graph, while omission preserves the existing default.
func TestCalendarAvailabilityPayload(t *testing.T) {
	t.Setenv("M365CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("M365CLI_KEYRING_BACKEND", "file")
	t.Setenv("M365CLI_KEYRING_PASSWORD", "fixture-only-password")
	if err := config.WriteConfig(config.File{ActiveProfile: "fixture", Profiles: map[string]config.ProfileRecord{"fixture": {Name: "fixture", Audience: "consumer", AuthMode: "delegated", ClientID: "fixture", DelegatedScopeWorkloads: []string{"calendar-write"}}}}); err != nil {
		t.Fatal(err)
	}
	cache, _ := json.Marshal(auth.TokenCache{AccessToken: "fixture-only", Scope: "Calendars.ReadWrite", ExpiresAt: time.Now().Add(time.Hour)})
	if err := secrets.SetSecret("m365cli:cache:fixture", cache); err != nil {
		t.Fatal(err)
	}
	original := http.DefaultClient
	defer func() { http.DefaultClient = original }()
	calls := 0
	want := "workingElsewhere"
	http.DefaultClient = &http.Client{Transport: contractTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "POST" && r.Method != "PATCH" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if want == "" {
			if _, ok := payload["showAs"]; ok {
				t.Error("omitted availability must not overwrite existing state")
			}
		} else if payload["showAs"] != want {
			t.Errorf("showAs=%v want %s", payload["showAs"], want)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"fixture-event"}`))}, nil
	})}
	create := []string{"calendar", "create", "--subject", "Fixture", "--start", "2026-10-28T10:00:00Z", "--end", "2026-10-28T11:00:00Z"}
	for _, args := range [][]string{append(append([]string{}, create...), "--show-as", "workingElsewhere"), {"calendar", "update", "fixture-event", "--show-as", "workingElsewhere"}} {
		if _, _, err := captureExecuteOutput(t, args); err != nil {
			t.Fatal(err)
		}
	}
	want = ""
	if _, _, err := captureExecuteOutput(t, create); err != nil {
		t.Fatal(err)
	}
	before := calls
	if _, _, err := captureExecuteOutput(t, []string{"calendar", "update", "fixture-event", "--show-as", "busy", "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := captureExecuteOutput(t, []string{"calendar", "update", "fixture-event", "--show-as", "maybe"}); err == nil {
		t.Error("invalid availability accepted")
	}
	if calls != before || calls != 3 {
		t.Errorf("unexpected provider calls: %d", calls)
	}
}

package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/fdsouvenir/m365cli/internal/config"
)

type contractTransport func(*http.Request) (*http.Response, error)

func (f contractTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// One end-to-end fixture protects the documented JSON surface Link consumes,
// including real login/import, grant persistence and later refresh code paths.
func TestDocumentedJSONContract(t *testing.T) {
	t.Setenv("M365CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("M365CLI_KEYRING_BACKEND", "file")
	t.Setenv("M365CLI_KEYRING_PASSWORD", "synthetic-contract-test-password")
	t.Setenv("M365CLI_CONTRACT_REFRESH", "synthetic-refresh-token")
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	block := strings.SplitN(string(readme), "<!-- m365cli-json-contract -->", 2)
	if len(block) != 2 {
		t.Fatal("missing documented JSON contract")
	}
	schema := strings.SplitN(strings.SplitN(block[1], "```json\n", 2)[1], "```", 2)[0]
	var fields map[string][]string
	if err := json.Unmarshal([]byte(schema), &fields); err != nil {
		t.Fatal(err)
	}
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"oid":"00000000-0000-0000-1234-123456789abc","tid":"9188040d-6c67-4c5b-b112-36a304b66dad","preferred_username":"synthetic@example.com"}`))
	idToken := "e30." + claims + ".signature"
	original := http.DefaultClient
	calls, commonImports, normalRefreshes, inspections := 0, 0, 0, 0
	unknown := false
	http.DefaultClient = &http.Client{Transport: contractTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		payload := ""
		if r.URL.Host == "login.microsoftonline.com" {
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			scope := r.Form.Get("scope")
			if r.Form.Get("grant_type") != "refresh_token" {
				t.Fatal("unexpected login grant type")
			}
			if scope == "https://graph.microsoft.com/.default" {
				inspections++
				if !strings.HasPrefix(r.URL.Path, "/consumers/") {
					t.Fatal("grant inspection did not use detected normal authority")
				}
				scope = "https://graph.microsoft.com/User.Read https://graph.microsoft.com/Mail.ReadWrite https://graph.microsoft.com/Mail.Send"
				if unknown {
					scope = "https://graph.microsoft.com/.default"
				}
			} else if strings.HasPrefix(r.URL.Path, "/common/") {
				commonImports++
				if strings.Contains(scope, ".default") {
					t.Fatal(".default used to sign in")
				}
			} else {
				normalRefreshes++
				if !strings.HasPrefix(r.URL.Path, "/consumers/") {
					t.Fatal("later refresh did not use consumers")
				}
				if scope != "User.Read" {
					t.Fatalf("unexpected normal refresh scopes: %s", scope)
				}
			}
			b, _ := json.Marshal(map[string]any{"access_token": "synthetic-access-token", "refresh_token": "synthetic-rotated-refresh", "scope": scope, "expires_in": 1, "id_token": idToken})
			payload = string(b)
		} else if r.URL.Host == "graph.microsoft.com" && r.URL.Path == "/v1.0/me" {
			payload = `{"id":"1234123456789abc","displayName":"Synthetic Account"}`
		} else {
			t.Fatalf("unexpected provider request: %s", r.URL)
			return nil, fmt.Errorf("unexpected request")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload))}, nil
	})}
	defer func() { http.DefaultClient = original }()
	outputs := map[string]any{}
	run := func(name string, args ...string) map[string]any {
		t.Helper()
		args = append(args, "--json", "--no-input")
		stdout, stderr, err := captureExecuteOutput(t, args)
		if err != nil {
			t.Fatalf("%s: %v, %s", name, err, stderr)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		outputs[name] = payload
		return payload
	}
	run("version", "version")
	catalog := run("auth workloads", "auth", "workloads")
	if len(catalog["workloads"].([]any)) != 18 {
		t.Fatal("catalog is missing workload choices")
	}
	login := run("auth login", "auth", "login", "--profile", "contract", "--audience", "auto", "--client-id", "synthetic-client", "--scope-workloads", "mail-read,calendar-read", "--refresh-token-env", "M365CLI_CONTRACT_REFRESH")
	if login["audience"] != "consumer" || commonImports != 1 {
		t.Fatal("auto import failed to detect a personal account through common")
	}
	grant := login["grant"].(map[string]any)
	if fmt.Sprint(grant["selected_not_granted"]) != "[calendar-read]" || fmt.Sprint(grant["granted_not_selected"]) != "[Mail.ReadWrite Mail.Send]" {
		t.Fatalf("wrong grant differences: %v", grant)
	}
	before := calls
	_, _, err = captureExecuteOutput(t, []string{"--account", "contract", "mail", "draft", "--to", "synthetic@example.com", "--subject", "test", "--body", "test", "--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "mail-write") || calls != before {
		t.Fatalf("unselected write must be refused before any provider request: %v", err)
	}
	_, _, err = captureExecuteOutput(t, []string{"--account", "contract", "calendar", "list", "--json"})
	if err == nil || !strings.Contains(err.Error(), "Microsoft did not grant") || calls != before {
		t.Fatalf("selected but ungranted workload: %v", err)
	}
	run("auth check", "--account", "contract", "auth", "check")
	if normalRefreshes == 0 || inspections != 2 {
		t.Fatal("check did not refresh and separately inspect grant")
	}
	// Populate every optional legacy status field so its documented name is protected.
	if err := config.UpdateConfig(func(cfg *config.File) error {
		p := cfg.Profiles["contract"]
		p.AppOnlyUser = "synthetic-target"
		cfg.Profiles["contract"] = p
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	status := run("auth status", "auth", "status")
	profile := status["profiles"].([]any)[0].(map[string]any)
	if profile["authority"] != "consumers" || profile["grant"].(map[string]any)["status"] != "known" {
		t.Fatal("detected authority or checked grant not persisted")
	}
	for name, paths := range fields {
		payload, ok := outputs[name]
		if !ok {
			t.Fatalf("documented command not exercised: %s", name)
		}
		for _, path := range paths {
			checkDocumentedPath(t, payload, strings.Split(path, "."), name+"."+path)
		}
	}
	// A literal .default response is unknown, not evidence of any permission.
	unknown = true
	result := run("unknown grant", "--account", "contract", "auth", "check")["grant"].(map[string]any)
	if result["status"] != "unknown" || result["scopes"] != nil || result["selected_not_granted"] != nil {
		t.Fatalf(".default treated as a concrete grant: %v", result)
	}
}

func checkDocumentedPath(t *testing.T, value any, parts []string, label string) {
	t.Helper()
	if len(parts) == 0 {
		return
	}
	key := strings.TrimSuffix(parts[0], "[]")
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s: expected object", label)
	}
	child, ok := object[key]
	if !ok {
		t.Fatalf("documented JSON field changed: %s", label)
	}
	if strings.HasSuffix(parts[0], "[]") {
		items, ok := child.([]any)
		if !ok || len(items) == 0 {
			t.Fatalf("%s: expected populated array fixture", label)
		}
		for _, item := range items {
			checkDocumentedPath(t, item, parts[1:], label)
		}
	} else {
		checkDocumentedPath(t, child, parts[1:], label)
	}
}

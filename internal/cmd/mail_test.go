package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fdsouvenir/m365cli/internal/auth"
	"github.com/fdsouvenir/m365cli/internal/config"
	"github.com/fdsouvenir/m365cli/internal/secrets"
)

func TestMailSendQuoteFlagParsesWithoutBody(t *testing.T) {
	parser, cli, err := newParser("test")
	if err != nil {
		t.Fatalf("newParser failed: %v", err)
	}

	args := []string{
		"mail", "send",
		"--to", "dev@example.com",
		"--subject", "Re: Hello",
		"--quote", "message-id-123",
	}
	if _, err := parser.Parse(args); err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if cli.Mail.Send.Body != "" {
		t.Fatalf("expected empty body, got %q", cli.Mail.Send.Body)
	}
	if cli.Mail.Send.Quote != "message-id-123" {
		t.Fatalf("unexpected quote id: %q", cli.Mail.Send.Quote)
	}
}

func TestMailDraftFlagParsesWithoutTo(t *testing.T) {
	parser, cli, err := newParser("test")
	if err != nil {
		t.Fatalf("newParser failed: %v", err)
	}

	args := []string{
		"mail", "draft",
		"--subject", "Draft subject",
		"--body", "Draft body",
	}
	if _, err := parser.Parse(args); err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if cli.Mail.Draft.Subject != "Draft subject" {
		t.Fatalf("unexpected subject: %q", cli.Mail.Draft.Subject)
	}
	if cli.Mail.Draft.Body != "Draft body" {
		t.Fatalf("unexpected body: %q", cli.Mail.Draft.Body)
	}
	if len(cli.Mail.Draft.To) != 0 {
		t.Fatalf("expected no recipients, got %v", cli.Mail.Draft.To)
	}
}

func TestMailDraftFlagParsesWithTo(t *testing.T) {
	parser, cli, err := newParser("test")
	if err != nil {
		t.Fatalf("newParser failed: %v", err)
	}

	args := []string{
		"mail", "draft",
		"--to", "a@example.com,b@example.com",
		"--subject", "Draft subject",
	}
	if _, err := parser.Parse(args); err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(cli.Mail.Draft.To) == 0 {
		t.Fatalf("expected recipients, got none")
	}
	if cli.Mail.Draft.Subject != "Draft subject" {
		t.Fatalf("unexpected subject: %q", cli.Mail.Draft.Subject)
	}
}

func TestComposeQuotedReplyBodyAppendsQuoteBlock(t *testing.T) {
	source := map[string]any{
		"sentDateTime": "2026-02-18T13:00:00Z",
		"from": map[string]any{
			"emailAddress": map[string]any{
				"address": "sender@example.com",
			},
		},
		"bodyPreview": "Hello there\nHow are you?",
	}

	got := composeQuotedReplyBody("Thanks for the update.", source)
	wantContains := []string{
		"Thanks for the update.",
		"On 2026-02-18T13:00:00Z, sender@example.com wrote:",
		"> Hello there",
		"> How are you?",
	}
	for _, fragment := range wantContains {
		if !strings.Contains(got, fragment) {
			t.Fatalf("expected reply body to contain %q, got:\n%s", fragment, got)
		}
	}
}

func TestComposeQuotedReplyBodyAllowsQuoteOnly(t *testing.T) {
	source := map[string]any{
		"bodyPreview": "Only quoted text",
	}

	got := composeQuotedReplyBody("", source)
	if strings.Contains(got, "\n\n") {
		t.Fatalf("expected quote-only body without leading empty section, got %q", got)
	}
	if !strings.Contains(got, "sender wrote:") {
		t.Fatalf("expected quote header, got %q", got)
	}
	if !strings.Contains(got, "> Only quoted text") {
		t.Fatalf("expected quoted message content, got %q", got)
	}
}

// Inbox triage must select the inbox rather than the whole mailbox (#14).
func TestMailListCanSelectInbox(t *testing.T) {
	parser, _, err := newParser("test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.Parse([]string{"mail", "list", "--folder", "inbox", "--json"}); err != nil {
		t.Fatalf("cannot target the inbox: %v", err)
	}
}

func TestMailPreviewsStayLocalAndDeleteRequiresConfirmation(t *testing.T) {
	t.Setenv("M365CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("M365CLI_KEYRING_BACKEND", "file")
	t.Setenv("M365CLI_KEYRING_PASSWORD", "")
	if err := config.WriteConfig(config.File{ActiveProfile: "synthetic", Profiles: map[string]config.ProfileRecord{"synthetic": {Name: "synthetic", Audience: "consumer", AuthMode: "delegated", ClientID: "synthetic", DelegatedScopeWorkloads: []string{"mail-read", "mail-send", "mail-write"}}}}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"reply", "message-id", "--body", "text"},
		{"reply", "message-id", "--body", "text", "--reply-all"},
		{"reply", "message-id", "--body", "text", "--draft"},
		{"reply", "message-id", "--body", "text", "--draft", "--reply-all"},
		{"forward", "message-id", "--to", "synthetic@example.com"},
		{"mark-read", "message-id"}, {"mark-unread", "message-id"},
		{"move", "message-id", "--folder", "inbox"}, {"archive", "message-id"}, {"delete", "message-id"},
	} {
		command := append([]string{"mail"}, args...)
		command = append(command, "--dry-run", "--json", "--no-input")
		stdout, stderr, err := captureExecuteOutput(t, command)
		if err != nil {
			t.Fatalf("preview required credentials or confirmation: %v: %v %s", args, err, stderr)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		if result["dry_run"] != true {
			t.Fatalf("not a preview: %v", result)
		}
	}
	for _, command := range [][]string{
		{"mail", "delete", "message-id", "--no-input"},
		{"mail", "move", "message-id", "--folder", "deleted", "--no-input"},
		{"mail", "move", "message-id", "--folder", "deleteditems", "--no-input"},
		{"mail", "move", "message-id", "--folder", "DeLeTeDiTeMs", "--no-input"},
	} {
		_, _, err := captureExecuteOutput(t, command)
		if err == nil || !strings.Contains(err.Error(), "--force") {
			t.Fatalf("Deleted Items move bypassed delete confirmation: %v: %v", command, err)
		}
	}
	for _, args := range [][]string{{"move", "message-id", "--folder", "archive"}, {"archive", "message-id"}} {
		command := append([]string{"mail"}, args...)
		command = append(command, "--no-input")
		_, _, err := captureExecuteOutput(t, command)
		if err == nil || strings.Contains(err.Error(), "--force") {
			t.Fatalf("ordinary update unexpectedly required confirmation: %v", err)
		}
	}
	// Resolve opaque folder IDs read-only; the same delete rule must cover those IDs.
	t.Setenv("M365CLI_KEYRING_PASSWORD", "synthetic-regression-password")
	cache, _ := json.Marshal(auth.TokenCache{AccessToken: "synthetic", Scope: "Mail.ReadWrite", ExpiresAt: time.Now().Add(time.Hour)})
	if err := secrets.SetSecret("m365cli:cache:synthetic", cache); err != nil {
		t.Fatal(err)
	}
	original := http.DefaultClient
	defer func() { http.DefaultClient = original }()
	reads, writes := 0, 0
	http.DefaultClient = &http.Client{Transport: contractTransport(func(r *http.Request) (*http.Response, error) {
		payload := ""
		status := 200
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1.0/me/mailFolders/deleteditems":
			reads++
			if r.URL.Query().Get("$select") != "id" {
				t.Error("folder lookup must request only the ID")
			}
			payload = `{"id":"DeletedFolderID"}`
		case r.Method == "POST" && r.URL.Path == "/v1.0/me/messages/message-id/move":
			writes++
			status = 201
			payload = `{"id":"moved-id","body":{"content":"sensitive-response-body"},"toRecipients":["unneeded"]}`
		default:
			return nil, fmt.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload))}, nil
	})}
	_, _, err := captureExecuteOutput(t, []string{"mail", "move", "message-id", "--folder", "DeletedFolderID", "--no-input"})
	if err == nil || !strings.Contains(err.Error(), "--force") || reads != 1 || writes != 0 {
		t.Fatalf("opaque Deleted Items ID bypassed confirmation: reads=%d writes=%d err=%v", reads, writes, err)
	}
	stdout, _, err := captureExecuteOutput(t, []string{"mail", "move", "message-id", "--folder", "OrdinaryFolderID", "--no-input", "--json"})
	if strings.Contains(stdout, "sensitive-response-body") || strings.Contains(stdout, "toRecipients") || !strings.Contains(stdout, "moved-id") {
		t.Errorf("move output must retain ID but omit unnecessary message content: %s", stdout)
	}
	if err != nil || reads != 2 || writes != 1 {
		t.Fatalf("ordinary folder-ID move must not require force: reads=%d writes=%d err=%v", reads, writes, err)
	}
	_, _, err = captureExecuteOutput(t, []string{"mail", "move", "message-id", "--folder", "DeletedFolderID", "--no-input", "--force"})
	if err != nil || reads != 2 || writes != 2 {
		t.Fatalf("explicitly forced move failed: reads=%d writes=%d err=%v", reads, writes, err)
	}

}

package cmd

import (
	"strings"
	"testing"
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

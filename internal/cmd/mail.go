package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fdsouvenir/m365cli/internal/outfmt"
	"github.com/fdsouvenir/m365cli/internal/services/mail"
)

type MailCmd struct {
	List       MailListCmd       `cmd:"" help:"List messages newest first (whole mailbox unless --folder is set)"`
	Folders    MailFoldersCmd    `cmd:"" help:"List top-level mail folders and their IDs"`
	Get        MailGetCmd        `cmd:"" help:"Get message by ID"`
	Send       MailSendCmd       `cmd:"" help:"Send a new message"`
	Draft      MailDraftCmd      `cmd:"" help:"Create a draft message"`
	Reply      MailReplyCmd      `cmd:"" help:"Send or draft a threaded reply"`
	Forward    MailForwardCmd    `cmd:"" help:"Forward a message using Microsoft's forward action"`
	MarkRead   MailMarkReadCmd   `cmd:"" name:"mark-read" help:"Mark a message read"`
	MarkUnread MailMarkUnreadCmd `cmd:"" name:"mark-unread" help:"Mark a message unread"`
	Move       MailMoveCmd       `cmd:"" help:"Move a message to a folder"`
	Archive    MailArchiveCmd    `cmd:"" help:"Move a message to Archive"`
	Delete     MailDeleteCmd     `cmd:"" help:"Move a message to Deleted Items (restorable)"`
}

type MailListCmd struct {
	Folder string `name:"folder" help:"Folder: inbox, sent, drafts, archive, deleted, junk or a folder ID (default: whole mailbox)"`
	Max    int    `name:"max" default:"20" help:"Maximum messages"`
	Query  string `name:"query" help:"Search query text"`
	Page   string `name:"page" aliases:"next-token" help:"Resume from next page token"`
	User   string `name:"user" help:"App-only target user override (UPN or user ID)"`
}

func (c *MailListCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capMailList)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}
	page, err := normalizePageToken(c.Page)
	if err != nil {
		return err
	}

	svc := mail.New(rt.Graph, targetUser)
	items, next, err := svc.List(ctx, c.Max, c.Query, page, c.Folder)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"messages": items, "next": next})
	}

	printItemTable(ctx, items, []string{"receivedDateTime", "subject", "id", "isRead"})
	printNextPageHint(uiFromContext(ctx), next)
	return nil
}

type MailGetCmd struct {
	ID   string `arg:"" required:"" help:"Message ID"`
	User string `name:"user" help:"App-only target user override (UPN or user ID)"`
}

func (c *MailGetCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capMailGet)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}

	svc := mail.New(rt.Graph, targetUser)
	item, err := svc.Get(ctx, c.ID)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, item)
	}

	printSingleMap(ctx, item)
	return nil
}

type MailSendCmd struct {
	To      []string `name:"to" required:"" help:"Recipient email (repeat or comma-separate)"`
	Subject string   `name:"subject" required:"" help:"Email subject"`
	Body    string   `name:"body" help:"Plain text body"`
	Quote   string   `name:"quote" help:"Message ID to quote in a new, unthreaded message (use mail reply for threaded replies)"`
	User    string   `name:"user" help:"App-only target user override (UPN or user ID)"`
	DryRun  bool     `name:"dry-run" help:"Preview send without sending the message"`
}

func (c *MailSendCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capMailSend)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}

	recipients := splitCSV(c.To)
	if len(recipients) == 0 {
		return usage("at least one --to recipient is required")
	}
	if strings.TrimSpace(c.Body) == "" && strings.TrimSpace(c.Quote) == "" {
		return usage("--body is required unless --quote is provided")
	}
	if c.DryRun {
		if outfmt.IsJSON(ctx) {
			return outfmt.WriteJSON(os.Stdout, map[string]any{
				"dry_run":  true,
				"action":   "mail.send",
				"to":       recipients,
				"subject":  c.Subject,
				"body_len": len(c.Body),
				"quote":    strings.TrimSpace(c.Quote),
			})
		}
		if strings.TrimSpace(c.Quote) != "" {
			fmt.Fprintf(os.Stdout, "Dry run: would send a new message quoting mail to %s with subject %q (quote id: %s)\n", strings.Join(recipients, ", "), c.Subject, strings.TrimSpace(c.Quote))
			return nil
		}
		fmt.Fprintf(os.Stdout, "Dry run: would send message to %s with subject %q\n", strings.Join(recipients, ", "), c.Subject)
		return nil
	}

	svc := mail.New(rt.Graph, targetUser)
	body := strings.TrimSpace(c.Body)
	quoteID := strings.TrimSpace(c.Quote)
	if quoteID != "" {
		source, getErr := svc.Get(ctx, quoteID)
		if getErr != nil {
			return getErr
		}
		body = composeQuotedReplyBody(body, source)
	}
	if err := svc.Send(ctx, recipients, c.Subject, body); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"sent": true, "to": recipients})
	}

	fmt.Fprintf(os.Stdout, "Sent message to %s\n", strings.Join(recipients, ", "))
	return nil
}

type MailDraftCmd struct {
	To      []string `name:"to" help:"Recipient email (repeat or comma-separate)"`
	Subject string   `name:"subject" required:"" help:"Email subject"`
	Body    string   `name:"body" help:"Plain text body"`
	User    string   `name:"user" help:"App-only target user override (UPN or user ID)"`
	DryRun  bool     `name:"dry-run" help:"Preview draft creation without saving"`
}

func (c *MailDraftCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capMailDraft)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}

	recipients := splitCSV(c.To)

	if c.DryRun {
		if outfmt.IsJSON(ctx) {
			return outfmt.WriteJSON(os.Stdout, map[string]any{
				"dry_run":  true,
				"action":   "mail.draft",
				"to":       recipients,
				"subject":  c.Subject,
				"body_len": len(c.Body),
			})
		}
		if len(recipients) > 0 {
			fmt.Fprintf(os.Stdout, "Dry run: would create draft to %s with subject %q\n", strings.Join(recipients, ", "), c.Subject)
		} else {
			fmt.Fprintf(os.Stdout, "Dry run: would create draft with subject %q (no recipients)\n", c.Subject)
		}
		return nil
	}

	svc := mail.New(rt.Graph, targetUser)
	result, err := svc.Draft(ctx, recipients, c.Subject, strings.TrimSpace(c.Body))
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, result)
	}

	id := stringValue(result["id"])
	fmt.Fprintf(os.Stdout, "Draft created (id: %s)\n", id)
	return nil
}

func splitCSV(values []string) []string {
	out := make([]string, 0)
	seen := map[string]struct{}{}
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			item := strings.TrimSpace(part)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}

	return out
}

func composeQuotedReplyBody(body string, source map[string]any) string {
	quoteBlock := buildQuoteBlock(source)
	body = strings.TrimSpace(body)
	if body == "" {
		return quoteBlock
	}
	return body + "\n\n" + quoteBlock
}

func buildQuoteBlock(source map[string]any) string {
	sender := messageSender(source)
	sent := strings.TrimSpace(stringValue(source["sentDateTime"]))
	content := messageQuoteContent(source)

	header := "Quoted message:"
	switch {
	case sent != "" && sender != "":
		header = fmt.Sprintf("On %s, %s wrote:", sent, sender)
	case sender != "":
		header = fmt.Sprintf("%s wrote:", sender)
	case sent != "":
		header = fmt.Sprintf("On %s:", sent)
	}

	return header + "\n" + quoteLines(content)
}

func messageSender(source map[string]any) string {
	from, ok := source["from"].(map[string]any)
	if !ok {
		return "sender"
	}

	emailAddress, ok := from["emailAddress"].(map[string]any)
	if !ok {
		return "sender"
	}

	if address := strings.TrimSpace(stringValue(emailAddress["address"])); address != "" {
		return address
	}
	if name := strings.TrimSpace(stringValue(emailAddress["name"])); name != "" {
		return name
	}
	return "sender"
}

func messageQuoteContent(source map[string]any) string {
	if preview := strings.TrimSpace(stringValue(source["bodyPreview"])); preview != "" {
		return preview
	}

	body, ok := source["body"].(map[string]any)
	if !ok {
		return "(no message body)"
	}

	content := strings.TrimSpace(stringValue(body["content"]))
	if content == "" {
		return "(no message body)"
	}

	return content
}

func quoteLines(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return "> (no message body)"
	}

	quoted := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			quoted = append(quoted, ">")
			continue
		}
		quoted = append(quoted, "> "+line)
	}
	return strings.Join(quoted, "\n")
}

func stringValue(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

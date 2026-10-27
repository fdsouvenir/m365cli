package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fdsouvenir/m365cli/internal/outfmt"
	"github.com/fdsouvenir/m365cli/internal/services/mail"
)

type MailFoldersCmd struct {
	Max  int    `name:"max" help:"Page-size target (Microsoft default when omitted)"`
	Page string `name:"page" aliases:"next-token" help:"Resume from next page token"`
	User string `name:"user" help:"App-only target user override (UPN or user ID)"`
}

func (c *MailFoldersCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capMailList)
	if err != nil {
		return err
	}
	user, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}
	page, err := normalizePageToken(c.Page)
	if err != nil {
		return err
	}
	folders, next, err := mail.New(rt.Graph, user).Folders(ctx, c.Max, page)
	if err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"folders": folders, "next": next})
	}
	printItemTable(ctx, folders, []string{"displayName", "id", "unreadItemCount", "totalItemCount", "childFolderCount"})
	printNextPageHint(uiFromContext(ctx), next)
	return nil
}

// A dry-run stays entirely local, matching the existing mail send/draft previews.
func mailPreview(ctx context.Context, action, id string, fields map[string]any) error {
	fields["dry_run"] = true
	fields["action"] = action
	fields["id"] = id
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, fields)
	}
	printSingleMap(ctx, fields)
	return nil
}

type MailMessageFlags struct {
	ID     string `arg:"" required:"" help:"Message ID"`
	User   string `name:"user" help:"App-only target user override (UPN or user ID)"`
	DryRun bool   `name:"dry-run" help:"Preview without changing or sending mail"`
}

type MailReplyCmd struct {
	MailMessageFlags `embed:""`
	Body             string `name:"body" help:"Reply text"`
	ReplyAll         bool   `name:"reply-all" help:"Reply to all original recipients"`
	Draft            bool   `name:"draft" help:"Save a threaded reply draft instead of sending (requires mail-write)"`
}

func (c *MailReplyCmd) Run(ctx context.Context) error {
	capability := capMailSend
	if c.Draft {
		capability = capMailDraft
	}
	svc, err := mailActionService(ctx, capability, c.User)
	if err != nil {
		return err
	}
	if c.DryRun {
		return mailPreview(ctx, "mail.reply", c.ID, map[string]any{"reply_all": c.ReplyAll, "draft": c.Draft, "body_len": len(c.Body)})
	}
	result, err := svc.Reply(ctx, c.ID, c.Body, c.ReplyAll, c.Draft)
	if err != nil {
		return err
	}
	if c.Draft {
		if outfmt.IsJSON(ctx) {
			return outfmt.WriteJSON(os.Stdout, result)
		}
		fmt.Fprintf(os.Stdout, "Reply draft created (id: %s)\n", stringValue(result["id"]))
		return nil
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"sent": true, "reply_to": c.ID, "reply_all": c.ReplyAll})
	}
	fmt.Fprintf(os.Stdout, "Replied to message %s\n", c.ID)
	return nil
}

type MailForwardCmd struct {
	MailMessageFlags `embed:""`
	To               []string `name:"to" required:"" help:"Recipient email (repeat or comma-separate)"`
	Body             string   `name:"body" help:"Forward comment"`
}

func (c *MailForwardCmd) Run(ctx context.Context) error {
	svc, err := mailActionService(ctx, capMailSend, c.User)
	if err != nil {
		return err
	}
	recipients := splitCSV(c.To)
	if len(recipients) == 0 {
		return usage("at least one --to recipient is required")
	}
	if c.DryRun {
		return mailPreview(ctx, "mail.forward", c.ID, map[string]any{"to": recipients, "body_len": len(c.Body)})
	}
	if err := svc.Forward(ctx, c.ID, c.Body, recipients); err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"sent": true, "forward_of": c.ID, "to": recipients})
	}
	fmt.Fprintf(os.Stdout, "Forwarded message %s to %s\n", c.ID, strings.Join(recipients, ", "))
	return nil
}

type MailMarkReadCmd struct {
	MailMessageFlags `embed:""`
}
type MailMarkUnreadCmd struct {
	MailMessageFlags `embed:""`
}

func (c *MailMarkReadCmd) Run(ctx context.Context) error {
	return runMailMark(ctx, c.MailMessageFlags, true)
}
func (c *MailMarkUnreadCmd) Run(ctx context.Context) error {
	return runMailMark(ctx, c.MailMessageFlags, false)
}
func runMailMark(ctx context.Context, flags MailMessageFlags, read bool) error {
	svc, err := mailActionService(ctx, capMailDraft, flags.User)
	if err != nil {
		return err
	}
	if flags.DryRun {
		return mailPreview(ctx, "mail.mark", flags.ID, map[string]any{"is_read": read})
	}
	if err := svc.Mark(ctx, flags.ID, read); err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"updated": flags.ID, "is_read": read})
	}
	fmt.Fprintf(os.Stdout, "Updated message %s (read: %t)\n", flags.ID, read)
	return nil
}

func mailActionService(ctx context.Context, capability runtimeCapability, userOverride string) (*mail.Service, error) {
	rt, err := resolveRuntime(ctx, capability)
	if err != nil {
		return nil, err
	}
	user, err := resolveAppOnlyTargetUser(rt.Profile, userOverride)
	if err != nil {
		return nil, err
	}
	return mail.New(rt.Graph, user), nil
}

type MailMoveCmd struct {
	MailMessageFlags `embed:""`
	Folder           string `name:"folder" required:"" help:"Destination: inbox, sent, drafts, archive, deleted, junk or a folder ID"`
}
type MailArchiveCmd struct {
	MailMessageFlags `embed:""`
}
type MailDeleteCmd struct {
	MailMessageFlags `embed:""`
}

func (c *MailMoveCmd) Run(ctx context.Context) error {
	return runMailRelocate(ctx, c.MailMessageFlags, c.Folder, "mail.move", false)
}
func (c *MailArchiveCmd) Run(ctx context.Context) error {
	return runMailRelocate(ctx, c.MailMessageFlags, "archive", "mail.archive", false)
}
func (c *MailDeleteCmd) Run(ctx context.Context) error {
	return runMailRelocate(ctx, c.MailMessageFlags, "deleted", "mail.delete", true)
}

func runMailRelocate(ctx context.Context, message MailMessageFlags, folder, action string, confirm bool) error {
	svc, err := mailActionService(ctx, capMailDraft, message.User)
	if err != nil {
		return err
	}
	destination := mail.FolderID(folder)
	if message.DryRun {
		return mailPreview(ctx, action, message.ID, map[string]any{"folder": destination})
	}
	flags := rootFlagsFromContext(ctx)
	if flags == nil {
		flags = &RootFlags{}
	}
	if !confirm && !flags.Force {
		switch strings.ToLower(destination) {
		case "deleteditems":
			confirm = true
		case "inbox", "sentitems", "drafts", "archive", "junkemail": // Known non-deletion destinations.
		default:
			deletedID, err := svc.DeletedItemsID(ctx)
			if err != nil {
				return err
			}
			confirm = destination == deletedID
		}
	}

	if confirm {
		if err := confirmDestructive(ctx, flags, fmt.Sprintf("move message %s to Deleted Items", message.ID)); err != nil {
			return err
		}
	}
	result, err := svc.Move(ctx, message.ID, destination)
	if err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"action": action, "source_id": message.ID, "folder": destination, "message": result})
	}
	fmt.Fprintf(os.Stdout, "Moved message %s to %s (new id: %s)\n", message.ID, destination, stringValue(result["id"]))
	return nil
}

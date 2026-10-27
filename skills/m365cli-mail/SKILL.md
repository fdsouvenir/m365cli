---
name: m365cli-mail
description: Read, reply, forward and organize Outlook email with m365cli.
---

# Mail

Use a specific `--account NAME` on every API command and `--json` for structured output.
Read the core m365cli skill for setup or permission changes. Never fall back to a different account.

Use mail list --folder inbox for inbox-only newest-first coverage; without --folder the default is the whole mailbox. mail folders discovers top-level folder IDs. Threaded mail reply (including --reply-all) and mail forward need mail-send. mail reply --draft saves a threaded reply with mail-write; mail draft and mail send --quote create new, unthreaded messages. Mark read/unread, move/archive and delete need mail-write. Preview writes with --dry-run. Delete requires confirmation or explicitly authorized --force and moves mail to Deleted Items using Graph’s move action, never permanent deletion. Move/archive and read-status changes behave like other updates without an extra confirmation. Sending and mailbox changes require user authorization. Use read-only checks to test a connection.

Read [the generated command reference](references/commands.md) for syntax. Confirm with
`m365cli mail COMMAND --help` when needed. Only perform writes requested by the person.

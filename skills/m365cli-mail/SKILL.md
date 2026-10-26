---
name: m365cli-mail
description: Read and send Outlook email, or create drafts, using m365cli.
---

# Mail

Use a specific `--account NAME` on every API command and `--json` for structured output.
Read the core m365cli skill for setup or permission changes. Never fall back to a different account.

Draft creation needs mail-write; read/send alone use mail-read and mail-send. Sending requires user authorization. Use read-only checks to test a connection.

Read [the generated command reference](references/commands.md) for syntax. Confirm with
`m365cli mail COMMAND --help` when needed. Only perform writes requested by the person.

---
name: m365cli-calendar
description: Find, create or change Outlook calendar events with m365cli.
---

# Calendar

Use a specific `--account NAME` on every API command and `--json` for structured output.
Read the core m365cli skill for setup or permission changes. Never fall back to a different account.

Use delegated authentication. Preserve the person's timezone and verify attendees, start and end before writing. Use calendar-read for reads or calendar-write for read/write access.

Read [the generated command reference](references/commands.md) for syntax. Confirm with
`m365cli calendar COMMAND --help` when needed. Only perform writes requested by the person.

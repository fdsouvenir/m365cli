---
name: m365cli-onedrive
description: List, download, upload and organize OneDrive files with m365cli.
---

# Onedrive

Use a specific `--account NAME` on every API command and `--json` for structured output.
Read the core m365cli skill for setup or permission changes. Never fall back to a different account.

Confirm the remote path and conflict behavior before uploads. Downloads are local file writes; do not overwrite unrelated files. Use onedrive-read for reads or onedrive-write for read/write access.

Read [the generated command reference](references/commands.md) for syntax. Confirm with
`m365cli onedrive COMMAND --help` when needed. Only perform writes requested by the person.

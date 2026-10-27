---
name: m365cli-excel
description: Read or edit business Excel worksheets, ranges, tables and rows with m365cli.
---

# Excel

Use a specific `--account NAME` on every API command and `--json` for structured output.
Read the core m365cli skill for setup or permission changes. Never fall back to a different account.

Use delegated business profiles and .xlsx workbooks in business OneDrive or SharePoint. Personal storage and app-only access are unavailable. Excel cannot be read-only: even workbook reads require Files.ReadWrite, granting read and write access to all of the person’s OneDrive files, not just workbooks. Find the drive item ID with OneDrive first. Read the target range/table before changing it. Use --dry-run previews; then read back the result. Matrix files are rectangular JSON arrays; null leaves a range cell unchanged. Use --field formulas explicitly for formulas. Do not retry an uncertain append: inspect the table first to avoid duplicate rows. Range output is JSON even in text mode so matrices remain intact.

Read [the generated command reference](references/commands.md) for syntax. Confirm with
`m365cli excel COMMAND --help` when needed. Only perform writes requested by the person.

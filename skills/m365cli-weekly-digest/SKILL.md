---
name: m365cli-weekly-digest
description: Summarize a week of Outlook email, calendar and Microsoft To Do activity with m365cli.
---

# Weekly digest

Determine the requested week and timezone from context. Select one explicit
`--account NAME`; if the request spans accounts, query each separately and label
all findings by account. Read mail, calendar and tasks service references for the
actual filter and date flags.

Collect the relevant calendar events, task lists/tasks and a bounded set of mail.
Follow page links when needed and state any coverage limit. A task's current
completion state alone does not prove when it was completed; do not count it as a
weekly accomplishment without a supporting timestamp or message.

Summarize completed work supported by evidence, unresolved commitments, deadlines,
next week's calendar and decisions needed. Deduplicate items that appear in more
than one source. Include source IDs and dates for consequential claims. Produce
the digest in the conversation or requested artifact; sending it, creating tasks
or editing dates requires the corresponding user request.

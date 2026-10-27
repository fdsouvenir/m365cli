## Unreleased

- Validate move/archive/delete completion status and message ID; preserve confirmed-write status on response failures and never replay uncertain moves.
- Add rich message-list metadata, consistent immutable IDs, hidden-folder opt-in, explicit page coverage and bounded mutation summaries.

- Add top-level `mail folders`, folder-aware listing and explicit newest-first ordering; keep the mailbox-wide default. Inbox triage now explicitly selects inbox (#14).
- Add Microsoft's built-in threaded reply/reply-all, saved reply drafts and forward actions. Sending needs mail-send; saving reply drafts needs mail-write (#24).
- Add mail-write mark read/unread, move/archive and restorable delete. All support dry-run. Delete keeps the existing confirmation/force rule and moves to Deleted Items, never permanent deletion. Moves into Deleted Items (including its actual folder ID) follow the same delete rule; other moves, archive and read-status updates need no force (#24).
- Update the inbox-triage workflow, explain quoted new messages versus threaded replies, and regenerate mail references. Real Microsoft writes remain unverified.

## 2.1.0 - 2026-10-27

- Store macOS credentials through the native Security framework, never token-bearing subprocess arguments (#5). Existing Keychain service/account names and stored values remain readable without sign-in.
- Add read/read-write calendar, contacts, tasks and OneDrive workloads; retain all existing workload scopes. Same-service write permissions cover reads (#10).
- Publish the shared workload catalog and generated login help; correct OneDrive syntax and exact dry-run support (#17, #22).
- Inspect and record Microsoft's actual delegated grant separately from token refresh; report both selection/grant differences and enforce recorded selection locally (#22).
- Document stable version/status/check/login/workloads JSON: minor versions add fields; renames/removals require a major version and changelog notice (#22).
- Add delegated audience auto, default interactively, with common sign-in/import and ID-token tenant detection; save the detected personal/tenant authority (#22).
- Live personal qualification (2026-10-27): auto sign-in/import, identity, five read services, normal refresh and zero-provider-connection write refusal passed. Microsoft returned literal .default instead of concrete granted scopes, so grant and both differences remain unknown. Work-account live detection remains unverified; no real-data writes or existing-store changes.
- Release config locks automatically on process termination and hold one transaction across profile, active-account, alias and configuration updates (#12, #6). Existing config and credential formats are unchanged.

## 2.0.0 - 2026-10-26

- Add `M365CLI_CONFIG_DIR`, an absolute configuration, profile and file-keyring directory selector. Existing files and encryption formats are unchanged.
- Keep inherited process home and global XDG context unchanged, so managed command environments can select an account store using this CLI-specific setting.
- A major release formalizes the current Linux/macOS-only support contract.
- Support Linux and macOS only, with four release archives and bash, zsh and fish completions.
- Resolve pinned Go modules and verify their checksums.

## 1.0.0 - 2026-10-24

### Highlights
- First stable release of m365cli, forked from [Jared Palmer's mogcli](https://github.com/jaredpalmer/mogcli), with explicit multiple-account selection and eleven bundled setup, service and workflow skills.
- Outlook mail, calendar, contacts, To Do, OneDrive, business Groups and business delegated Excel worksheet/range/table/row commands. Teams and Advertising are outside v1.
- Encrypted headless credential storage, strict account boundaries, safe continuation handling and non-replaying uncertain writes.
- Includes the live personal-account identity-verification repair from rc.2.

### Qualification and limits
- Real Linux personal-account sign-in, identity verification, token refresh and read-only mail, calendar, contacts, To Do and OneDrive checks passed.
- Business sign-in remains blocked by Microsoft administrator consent, including without Groups. No work account was saved or claimed connected.
- Business service access and Excel, app-only access and live write operations remain unverified. The maintainer accepted these limits for stable v1 on October 24.
- Groups require tenant-administrator consent; organization policy can require administrator consent for other services too. The agent setup guidance now distinguishes sign-in from authorization and avoids repeated blocked login attempts.
- No test emails, data changes, tenant creation, customer installation or Falcon Link hosted-onboarding deployment was performed.

### Distribution
- Linux amd64/arm64 and macOS amd64/arm64 archives with checksums and complete skill folders.
- Configuration and encrypted credentials retain the m365cli paths and formats introduced by the release candidates. No new sign-in is required merely to upgrade from rc.2.

## 1.0.0-rc.2 - 2026-10-24

- Fix live personal-account identity verification: compare the exact Microsoft CID representation without weakening work-account ID matching.
- Preserve existing accounts, permissions, encrypted tokens and configuration; no reauthorization needed for the fix.
- Real personal sign-in and read-only mail, calendar, contacts, To Do and OneDrive checks passed during qualification. Broader stable-v1 gates remain open.

## 1.0.0-rc.1 - 2026-10-22

- Rename the fork, executable, module, environment and release assets to m365cli under fdsouvenir; retain upstream attribution.
- Add business delegated Excel worksheet, range, table and row operations with write previews and account capability gates.
- Add explicit --account selection and preserve the active default when adding accounts.
- Add narrow mail read/send/write permission selections and live read-only account verification.
- Encrypt file-backed credentials, require a storage password, reject unsafe continuation URLs and avoid silently discarded page tails.
- Reject unexpected API/OAuth redirects and do not replay uncertain writes on server errors.
- Bundle core, generated service and practical workflow skills; remove upstream Homebrew publishing dependencies.
- Live personal/business consent, refresh, workbook behavior and hosted Falcon Link handoff remain unqualified. Live commissioning remains a separate qualification step for this release candidate.

# Changelog

All notable changes to this project will be documented in this file.

When preparing release `X.Y.Z`, add a section with this format:

`## X.Y.Z - YYYY-MM-DD`

The release scripts read that section and publish it as GitHub release notes.


## 0.0.2 - 2026-02-19

- Hardened auth/session security and resolved race conditions in profile and secrets flows.
- Added native keychain backend resolution and improved headless password fallback semantics.
- Improved calendar update safety by splitting attendee and reminder Graph patches.
- Expanded contacts support with additional fields and deterministic custom-field handling.
- Added quoted reply support in `m365cli mail send`.
- Strengthened release verification for Homebrew formula URL variants.
- Isolated test config paths with `XDG_CONFIG_HOME` for more reliable tests.

## 0.0.1 - 2026-02-13

- Initial release.

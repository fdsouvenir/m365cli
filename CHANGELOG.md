## 1.0.0-rc.1 - 2026-10-22

- Rename the fork, executable, module, environment and release assets to m365cli under fdsouvenir; retain upstream attribution.
- Add business delegated Excel worksheet, range, table and row operations with write previews and account capability gates.
- Add explicit --account selection and preserve the active default when adding accounts.
- Add narrow mail read/send/write permission selections and live read-only account verification.
- Encrypt file-backed credentials, require a storage password, reject unsafe continuation URLs and avoid silently discarded page tails.
- Reject unexpected API/OAuth redirects and do not replay uncertain writes on server errors.
- Bundle core, generated service and practical workflow skills; remove upstream Homebrew publishing dependencies.
- Live personal/business consent, refresh, workbook behavior and hosted Falcon Link handoff remain unqualified. This is a release candidate, not live commissioning evidence.

# Changelog

All notable changes to this project will be documented in this file.

When preparing release `X.Y.Z`, add a section with this format:

`## X.Y.Z - YYYY-MM-DD`

The release scripts read that section and publish it as GitHub release notes.

## Unreleased

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

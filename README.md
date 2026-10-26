# m365cli

An unofficial, agent-friendly Microsoft 365 CLI, forked from
[Jared Palmer's mogcli](https://github.com/jaredpalmer/mogcli).

## v1 scope

| Service | Personal delegated | Business delegated | Business app-only |
|---|---|---|---|
| Outlook mail | Yes | Yes | Yes, with target user |
| Calendar | Yes | Yes | Not implemented |
| Contacts | Yes | Yes | Yes, with target user |
| Microsoft To Do | Yes | Yes | Not implemented |
| OneDrive | Yes | Yes | Yes, with target user |
| Microsoft 365 Groups | No | Yes | Yes |
| Excel workbook APIs | No | Yes | No |

The table describes implemented commands. Account and license combinations require
individual commissioning. Microsoft licenses, consent and tenant policies still
apply. Teams, Advertising and other services are outside v1.

**Qualification:** personal-account sign-in, identity verification, token refresh
and read-only mail, calendar, contacts, To Do and OneDrive checks passed on the
released Linux CLI. Business sign-in remained blocked by Microsoft administrator
consent even without Groups; no business connection was recorded. Business service
access (including Excel), app-only access and live write operations remain
unverified. The maintainer accepted those limits for stable 1.0.0. Automated API/CLI
and cross-platform checks pass. Live provider qualification remains a separate
requirement. Falcon Link's hosted onboarding is separate work.

## Install

Supported platforms are Linux and macOS (amd64 and arm64).

Download a platform archive and `checksums.txt` from
[Releases](https://github.com/fdsouvenir/m365cli/releases), verify its checksum,
and unpack it. Archives include the executable and bundled skills.

Or build from source with Go 1.25 or newer:

```sh
git clone https://github.com/fdsouvenir/m365cli.git
cd m365cli
make build
./bin/m365cli --help
```

```sh
go install github.com/fdsouvenir/m365cli/cmd/m365cli@latest
```

`go install` does not install the bundled skills.
There is no maintained Homebrew tap for this fork.

## Agent-led setup

A person says “connect Outlook so you can read and send email.” The agent inspects
accounts, chooses `mail-read,mail-send`, starts one sign-in and presents Microsoft's
verification URL/code. The person grants consent; the agent then verifies identity
and requested services using reads, never an unsolicited email or test write.
The bundled [core skill](skills/m365cli/SKILL.md) describes the complete flow.

The operator supplies an Entra public-client app registration supporting the
required account audiences and device-code sign-in. Reuse the existing Falcon Link
registration where configured; no client ID, tenant or credentials are embedded in
the package. `M365CLI_CLIENT_ID` supplies the ID without repeating it per login.

```sh
m365cli auth status --json
m365cli auth login --profile personal --audience consumer \
  --scope-workloads mail-read,mail-send --no-input --json
m365cli auth login --profile work --audience enterprise \
  --scope-workloads mail-read,mail-send,calendar,onedrive,excel --no-input --json
m365cli --account work auth check --json
```

`--client-id` overrides the operator environment. `--tenant` selects an enterprise
tenant; otherwise sign-in uses `organizations`. The login command waits for
Microsoft sign-in and consent. `--no-input` disables terminal questions; Microsoft
consent is still required.
Sign-in uses Microsoft's device-code flow.
Groups always require tenant-administrator consent. Organization policy may also
require it for other services. Verify service authorization separately after sign-in;
report blocked access without asking the person to create a tenant or granting
additional permissions.

### Permissions

| Workload | Delegated Graph permissions (in addition to identity/offline access) |
|---|---|
| `mail-read` | `Mail.Read` |
| `mail-send` | `Mail.Send` |
| `mail-write` | `Mail.ReadWrite` (drafts) |
| `mail` | `Mail.Read`, `Mail.Send` (compatibility bundle) |
| `calendar` | `Calendars.Read`, `Calendars.ReadWrite` |
| `contacts` | `Contacts.Read`, `Contacts.ReadWrite` |
| `tasks` | `Tasks.Read`, `Tasks.ReadWrite` |
| `onedrive` | `Files.Read`, `Files.ReadWrite` |
| `excel` | `Files.ReadWrite` |
| `groups` | `Group.Read.All`, `GroupMember.Read.All` |

Other than the individual mail selections, these service bundles include writes.
Request workloads together and reauthorize with the intended union when adding
capabilities. Local configuration changes do not
revoke consent. Missing scopes are not silently treated as consent via `.default`.

### Multiple accounts

Profiles isolate credentials and saved account/tenant identity. `--account NAME`
(alias: `--use-profile NAME`) selects an account for any API command without changing
the global default. Unknown profiles fail; there is no fallback to another account.
The first login becomes the default. Adding another profile preserves that default.
`auth use NAME` explicitly changes it. `auth logout --profile NAME` removes only
that profile's local credentials; it does not revoke Microsoft authorization.

`auth status --json` describes saved local profiles. Live health requires separate
verification.
`--account NAME auth check --json` performs a real read-only Graph identity check.
Verify service access separately using a small read. Raw tokens are never output.

### Credential storage

Linux/headless storage uses the password-backed file store. macOS uses native
Keychain when available. File storage requires a non-empty `M365CLI_KEYRING_PASSWORD` supplied privately by the operator, and uses
AES-256-GCM with random salts/nonces and PBKDF2-HMAC-SHA256 (210,000 iterations).
Entries are written atomically with private file permissions. A missing or wrong
password fails; legacy plaintext entries are not silently read or migrated.

`M365CLI_KEYRING_BACKEND=auto|keychain|file` chooses storage. Configuration lives in
the platform user-config directory under `m365cli`, separate from upstream mogcli.
Set `M365CLI_CONFIG_DIR` to an absolute directory to select another native config,
profile and encrypted-keyring store without changing the process home or global
XDG settings. An unset or empty value keeps the standard location. Relative paths
are rejected. Existing store contents and encryption formats do not change.
The renamed application uses a separate credential store. Sign in again to
populate it; keep legacy plaintext caches out of the new store. Use
`--client-secret-env` or `--refresh-token-env` for explicitly needed
confidential-client/token imports; avoid literal secrets on command lines.

## Commands

```sh
m365cli --account personal mail list --max 20 --json
m365cli --account work calendar list --help
m365cli --account work contacts list --json
m365cli --account work tasks lists --json
m365cli --account work onedrive ls / --json
m365cli --account work groups list --json
```

`--json` provides structured output; `--plain` provides TSV for supported tables.
Use `--help` for exact service flags. Supported writes have `--dry-run`; destructive
operations require confirmation or explicit `--force` in noninteractive execution.
A preview does not authorize a write. Server-error responses to writes are not
automatically retried: check the remote result before repeating an uncertain send,
create or append.

### Pagination

List commands return an opaque Graph `next` URL. Resume it with `--page URL` using
the same account. `--max` requests a page-size/collection target.
The last server page may exceed that target. Retaining the full page prevents skipped
records when resuming. Single-page services can return fewer records; continue
until `next` is empty for full coverage. Continuations must stay on the configured
Graph origin with no scheme downgrade, userinfo or fragment. API redirects are
refused except secure, unauthenticated OneDrive content downloads.

### Excel

Operate on existing `.xlsx` workbooks in business OneDrive or SharePoint. Discover
the item ID using OneDrive, then use `--workbook ITEM_ID`; `--drive DRIVE_ID`
selects another business drive. Microsoft Graph workbook APIs do not support
consumer storage or application-only authentication. Even reads require
`Files.ReadWrite`.

```sh
m365cli --account work excel worksheets list --workbook ITEM_ID --json
m365cli --account work excel range get --workbook ITEM_ID \
  --sheet Sheet1 --address A1:C10 --json
m365cli --account work excel range set --workbook ITEM_ID \
  --sheet Sheet1 --address A1:B2 --file rows.json --dry-run
m365cli --account work excel tables list --workbook ITEM_ID --json
m365cli --account work excel rows append --workbook ITEM_ID \
  --table TABLE_ID --file rows.json --dry-run
```

`rows.json` contains a rectangular JSON array such as `[["Revenue",125],["Cost",80]]`.
Use `range set --field formulas` for formula strings. `null` leaves a range cell
unchanged. Range output stays JSON to preserve matrices. Worksheet add/delete,
table creation and row append are also available; see `excel --help`.

Microsoft's [Excel overview](https://learn.microsoft.com/en-us/graph/api/resources/excel?view=graph-rest-1.0)
explicitly excludes consumer storage, despite some individual API permission tables
listing personal delegated permissions. This version follows the storage boundary.

## Skills

The `skills/` folders contain a core setup skill, seven service skills, and inbox
triage, meeting preparation and weekly digest workflows. Install the folders into
your agent's supported skills directory. Do not install them over unrelated skills.
This repository does not modify installed Office plugins or customer configuration.

Service syntax is generated from the actual executable:

```sh
make build
python3 scripts/generate-skills.py
python3 scripts/generate-skills.py --check
```

## Development and release

Dependencies are pinned in `go.mod` and verified against `go.sum`; builds use
read-only module resolution to retrieve those versions.

```sh
make ci
make build
go vet ./...
python3 scripts/generate-skills.py --check
```

CI checks Linux and macOS Keychain builds. Releases use
GoReleaser with this repository's GitHub token.
`scripts/release.sh VERSION` requires clean, verified main and creates the tag;
`scripts/verify-release.sh VERSION` checks released checksums, expected platforms,
bundled skills and the host-platform binary. Live qualification and these automated
checks are separate release gates.

See [NOTICE](NOTICE) for upstream attribution.

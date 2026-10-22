# m365cli

Unofficial agent-friendly Microsoft 365 CLI

`m365cli` is a Microsoft Graph CLI for personal Microsoft accounts (MSA) and enterprise Microsoft Entra ID accounts. It provides scriptable commands for Mail, Calendar, Contacts, Groups, Tasks, and OneDrive.

## What m365cli supports

- Multiple profiles with exactly one active profile at a time.
- Consumer and enterprise audiences.
- Delegated user auth and enterprise app-only auth.
- Stable scripting output modes: `--json` and `--plain`.
- Interactive delegated wizard (`m365cli auth`), advanced app-only wizard (`m365cli auth app`), settings editor (`m365cli auth update`), and non-interactive login (`m365cli auth login`).
- Per-command scope requests in delegated mode (progressive consent).
- `--dry-run` previews for write operations in Mail, Calendar, and OneDrive.

### Workload support matrix

| Workload | Delegated | App-only |
|---|---|---|
| Mail | Yes | Yes (enterprise, requires target user) |
| Calendar | Yes | No |
| Contacts | Yes | Yes (enterprise, requires target user) |
| Groups | Enterprise only | Enterprise only |
| Tasks (Microsoft To Do) | Yes | No |
| OneDrive | Yes | Yes (enterprise, requires target user) |

Notes:

- App-only mode is enterprise-only.
- Calendar and tasks are intentionally blocked in app-only mode.
- Groups are intentionally blocked for consumer profiles.

## Install

### Option 1: Build from source

```bash
git clone https://github.com/fdsouvenir/m365cli.git
cd m365cli
go build -o bin/m365cli ./cmd/m365cli
./bin/m365cli --help
```

### Option 2: go install

```bash
go install github.com/fdsouvenir/m365cli/cmd/m365cli@latest
m365cli --help
```

### Option 3: Homebrew

```bash
brew tap jaredpalmer/tap
brew install jaredpalmer/tap/m365cli
m365cli --help
```

## Microsoft app setup prerequisites

Before login, create app registrations in Microsoft Entra:

1. Consumer app registration (for MSA audience).
2. Enterprise app registration (for work/school audience).

For delegated login:

1. Enable public client flow in app Authentication settings.
2. Add delegated Graph permissions for the workloads you plan to use.

For app-only login:

1. Use an enterprise app registration.
2. Add required application permissions.
3. Grant admin consent.
4. Create a client secret.

## Quick start

### 1) Interactive setup (recommended)

```bash
m365cli auth
```

This wizard configures the profile and starts delegated device-code login.

Advanced app-only interactive setup (enterprise only):

```bash
m365cli auth app
```

### 2) Scripted delegated login

Consumer profile:

```bash
m365cli auth login \
  --profile personal \
  --audience consumer \
  --client-id <consumer-client-id> \
  --scope-workloads mail,calendar,contacts,tasks,onedrive
```

Enterprise delegated profile:

```bash
m365cli auth login \
  --profile work \
  --audience enterprise \
  --client-id <enterprise-client-id> \
  --tenant <tenant-id-or-domain> \
  --scope-workloads mail,calendar,contacts,tasks,onedrive,groups
```

### 3) Scripted app-only login (enterprise)

```bash
export M365CLI_CLIENT_SECRET="<client-secret-value>"

m365cli auth login \
  --profile work-app \
  --audience enterprise \
  --mode app-only \
  --client-id <enterprise-client-id> \
  --tenant <tenant-id-or-domain> \
  --app-only-user user@contoso.com \
  --client-secret-env M365CLI_CLIENT_SECRET
```

### 4) Switch and inspect profiles

```bash
m365cli auth accounts
m365cli auth use work
m365cli auth whoami
```

For one-off command routing without switching active profile:

```bash
m365cli --use-profile work mail list --max 10
```

### 5) Update existing auth settings without full re-onboarding

```bash
m365cli auth update
m365cli auth update --profile work
```

The update flow shows current settings, lets you choose one field at a time to edit, and saves only selected changes.

## Command overview

- `m365cli auth`
- `m365cli auth app`
- `m365cli auth login|update|logout|accounts|use|whoami`
- `m365cli mail list|get|send`
- `m365cli calendar list|get|create|update|delete`
- `m365cli contacts list|get|create|update|delete`
- `m365cli groups list|get|members`
- `m365cli tasks lists|list|get|create|update|complete|delete`
- `m365cli onedrive ls|get|put|mkdir|rm`
- `m365cli config get|keys|set|unset|list|path`
- `m365cli completion <shell>`

## Usage examples

Mail:

```bash
m365cli mail list --max 50 --query "from:alerts@example.com"
m365cli mail get <message-id>
m365cli mail send --to dev@contoso.com --subject "Deploy complete" --body "Finished."
m365cli mail send --to dev@contoso.com --subject "Re: Deploy complete" --quote <message-id>
m365cli mail send --to dev@contoso.com --subject "Deploy complete" --body "Finished." --dry-run
```

Calendar:

```bash
m365cli calendar list --from 2026-02-12 --to 2026-02-19 --max 100
m365cli calendar create \
  --subject "Planning" \
  --start "2026-02-13T16:00:00-08:00" \
  --end "2026-02-13T16:30:00-08:00" \
  --body "Weekly sync"
m365cli calendar delete <event-id> --dry-run
```

Contacts:

```bash
m365cli contacts list --max 100
m365cli contacts create --display-name "Jane Doe" --email "jane@contoso.com"
m365cli contacts create \
  --display-name "Jane Doe" \
  --email "jane@contoso.com" \
  --org "Contoso" \
  --title "Program Manager" \
  --url "https://contoso.example/jane" \
  --note "Customer success lead" \
  --custom region=NA \
  --custom team=platform
m365cli contacts update <contact-id> --title "Director" --custom region=EMEA
```

Groups:

```bash
m365cli groups list --max 100
m365cli groups members <group-id> --max 100
```

Tasks:

```bash
m365cli tasks lists
m365cli tasks list --list <list-id> --max 100
m365cli tasks create --list <list-id> --title "Follow up"
m365cli tasks complete --list <list-id> --task <task-id>
```

OneDrive:

```bash
m365cli onedrive ls --path / --max 100
m365cli onedrive put ./report.pdf --path /Reports/report.pdf
m365cli onedrive get /Reports/report.pdf --out ./report.pdf
m365cli onedrive mkdir --path /Reports/Archive
m365cli onedrive rm --path /Reports/old-report.pdf
m365cli onedrive rm --path /Reports/old-report.pdf --dry-run
```

If `m365cli onedrive get` is run without `--out`, files are saved under the local `onedrive-downloads` directory in your m365cli config path.

App-only target user override (mail/contacts/onedrive):

```bash
m365cli mail list --user user@contoso.com --max 20
m365cli onedrive ls --user user@contoso.com --path /
```

## Pagination and scripting

Most list commands support `--page` to resume from a next-page token.

```bash
m365cli groups list --max 50 --json
m365cli groups list --page "<next-token-url>"
```

`--next-token` is also accepted as an alias for pagination resume flags where supported.

Output modes:

- `--json`: structured output for tooling.
- `--plain`: stable tab-separated output for shell scripts.

Examples:

```bash
m365cli mail list --json | jq '.messages[0]'
m365cli tasks list --list <list-id> --plain
```

## Configuration and secrets

Show config path:

```bash
m365cli config path
```

Show editable config keys:

```bash
m365cli config keys
```

Current keys:

- `timezone`
- `keyring_backend`

Profile metadata is stored in config. Tokens and secrets are stored via keychain/keyring backends.

`keyring_backend` supports `auto`, `keychain`, and `file`.

- `auto`: use native OS keychain when available, otherwise file backend.
- `keychain`: require native OS keychain support.
- `file`: store under the local m365cli keyring directory.

`M365CLI_KEYRING_PASSWORD` is treated as explicitly configured even when empty, so headless runs do not implicitly prompt for keyring passwords.

## Troubleshooting

No active profile:

```bash
m365cli auth accounts
m365cli auth use <profile>
```

Refresh delegated login:

```bash
m365cli auth login --profile <profile> --audience enterprise --client-id <id> --scope-workloads mail,calendar,contacts,tasks,onedrive
```

Logout and reset profile auth state:

```bash
m365cli auth logout --profile <profile>
```

Verbose mode:

```bash
m365cli --verbose mail list
```

## Development

```bash
go test ./...
go run ./cmd/m365cli --help
```

Additional project docs are in `docs/`.

## License

MIT

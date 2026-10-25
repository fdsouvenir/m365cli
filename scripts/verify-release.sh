#!/bin/bash
set -euo pipefail
version="${1:-}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]]; then
  echo 'usage: scripts/verify-release.sh X.Y.Z[-prerelease]' >&2
  exit 2
fi
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
repository="fdsouvenir/m365cli"
release_dir="$(mktemp -d)"
trap 'rm -rf "$release_dir"' EXIT

gh release view "v$version" --repo "$repository" --json body --jq '.body' > "$release_dir/notes"
[[ -s "$release_dir/notes" ]] || { echo 'Missing release notes' >&2; exit 1; }
gh release download "v$version" --repo "$repository" --dir "$release_dir" --pattern 'm365cli_*' --pattern checksums.txt
python3 - "$release_dir" "$version" <<'PY'
import hashlib
import platform
from pathlib import Path
import subprocess
import sys
import tarfile

folder = Path(sys.argv[1])
version = sys.argv[2]
expected = {f'm365cli_{version}_{target}.{extension}' for target, extension in [
    ('linux_amd64','tar.gz'), ('linux_arm64','tar.gz'),
    ('darwin_amd64','tar.gz'), ('darwin_arm64','tar.gz')]}
checksums = {}
for line in (folder/'checksums.txt').read_text().splitlines():
    digest, name = line.split(maxsplit=1)
    checksums[name.lstrip('*')] = digest
if set(checksums) != expected:
    sys.exit(f'Unexpected checksum inventory: {set(checksums)}')
for name in sorted(expected):
    archive = folder/name
    if hashlib.sha256(archive.read_bytes()).hexdigest() != checksums[name]:
        sys.exit(f'Checksum mismatch: {name}')
    with tarfile.open(archive) as data:
        members = set(data.getnames())
    required = {'m365cli', 'README.md', 'NOTICE', 'skills/m365cli/SKILL.md', 'skills/m365cli-excel/SKILL.md', 'skills/m365cli-mail/references/commands.md'}
    if not required <= members:
        sys.exit(f'Missing archive contents in {name}: {required - members}')
    print(f'Verified checksum and contents: {name}')
os_name = {'Linux':'linux','Darwin':'darwin'}.get(platform.system())
arch = {'x86_64':'amd64','aarch64':'arm64','arm64':'arm64'}.get(platform.machine())
if os_name and arch:
    archive = folder/f'm365cli_{version}_{os_name}_{arch}.tar.gz'
    with tarfile.open(archive) as data:
        # Extract only the executable, never arbitrary archive paths.
        executable = folder/'m365cli'
        executable.write_bytes(data.extractfile('m365cli').read())
        executable.chmod(0o700)
    result = subprocess.check_output([str(executable), '--version'], text=True)
    if version not in result:
        sys.exit(f'Unexpected version: {result}')
    subprocess.run([str(executable),'excel','range','set','--help'], check=True, stdout=subprocess.DEVNULL)
    print(result.strip())
PY
printf 'Release v%s: all four platform assets verified. Live account qualification is separate.\n' "$version"

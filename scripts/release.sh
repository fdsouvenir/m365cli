#!/bin/bash
# Adapted from opensrc/repos/github.com/steipete/gogcli/scripts/release.sh.
set -euo pipefail

version="${1:-}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]]; then
  echo "usage: scripts/release.sh X.Y.Z" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$branch" != "main" ]]; then
  echo "expected branch main (got $branch)" >&2
  exit 2
fi
if [[ -n "$(git status --porcelain)" ]]; then
  echo "working tree not clean" >&2
  exit 2
fi

changelog="CHANGELOG.md"
if [[ ! -f "$changelog" ]]; then
  echo "missing $changelog" >&2
  exit 2
fi
if ! grep -q "^## ${version} - " "$changelog"; then
  echo "missing changelog section for $version" >&2
  exit 2
fi
if grep -q "^## ${version} - Unreleased" "$changelog"; then
  echo "changelog section still Unreleased for $version" >&2
  exit 2
fi

notes_file="$(mktemp -t m365cli-release-notes.XXXXXX)"
trap 'rm -f "$notes_file"' EXIT
awk -v ver="$version" '
  $0 ~ "^## "ver" " {print "## "ver; in_section=1; next}
  in_section && /^## / {exit}
  in_section {print}
' "$changelog" | sed '/^$/d' > "$notes_file"

if [[ ! -s "$notes_file" ]]; then
  echo "release notes empty for $version" >&2
  exit 2
fi

make ci
make build
python3 scripts/generate-skills.py --check

if [[ -n "$(git status --porcelain)" ]]; then
  echo "working tree changed by CI checks; commit formatting fixes before releasing" >&2
  exit 2
fi

if git rev-parse "v$version" >/dev/null 2>&1; then
  echo "tag v$version already exists"
else
  git tag -a "v$version" -m "Release $version"
  git push origin "v$version"
fi

if gh release view "v$version" >/dev/null 2>&1; then
  gh release edit "v$version" --notes-file "$notes_file"
else
  release_flags=()
  if [[ "$version" == *-* ]]; then release_flags+=(--prerelease); fi
  gh release create "v$version" --notes-file "$notes_file" "${release_flags[@]}"
fi

echo "Release notes updated. Next: scripts/verify-release.sh $version"

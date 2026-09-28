#!/bin/sh
# Renders the tap-desktop cask for one release from the template beside
# the scripts: the version and the DMG's sha256 are the only two values,
# so the cask in the tap is always this template at a version.
set -eu

version="${1:-}"; dmg="${2:-}"; output="${3:-}"
here="$(cd "$(dirname "$0")" && pwd)"
template="$here/../release/tap-desktop.rb.template"
[ -n "$version" ] || { echo "render-cask.sh: no version" >&2; exit 1; }
[ -f "$dmg" ] || { echo "render-cask.sh: $dmg is missing" >&2; exit 1; }
[ -n "$output" ] || { echo "render-cask.sh: no output path" >&2; exit 1; }
case "$version" in *[!0-9A-Za-z.-]*) echo "render-cask.sh: '$version' is not a version" >&2; exit 1 ;; esac

sha256=$(shasum -a 256 "$dmg" | cut -d ' ' -f 1)
mkdir -p "$(dirname "$output")"
sed -e "s/__VERSION__/$version/" -e "s/__SHA256__/$sha256/" "$template" > "$output"
if grep -q '__' "$output"; then echo "render-cask.sh: a placeholder was left in $output" >&2; exit 1; fi
echo "wrote $output"

#!/bin/sh
# Checks render-cask.sh: the version and the DMG's sha256 land in the
# template, nothing else changes, Ruby parses it, and brew style passes
# where brew is installed.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
script="$here/render-cask.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT

printf 'not really a dmg' > "$root/Tap-2.1.0.dmg"
sha=$(shasum -a 256 "$root/Tap-2.1.0.dmg" | cut -d ' ' -f 1)
# Under a Casks folder, as in the tap: brew style then applies its cask
# rules and not the generic Ruby ones (Sorbet sigils, frozen strings).
cask="$root/Casks/tap-desktop.rb"

"$script" 2.1.0 "$root/Tap-2.1.0.dmg" "$cask" >/dev/null || { echo "the cask should render"; exit 1; }
grep -Fq 'version "2.1.0"' "$cask" || { echo "no version"; exit 1; }
grep -Fq "sha256 \"$sha\"" "$cask" || { echo "no sha256"; exit 1; }
grep -Fq 'cask "tap-desktop" do' "$cask" || { echo "no cask header"; exit 1; }
grep -Fq 'releases/download/v#{version}/Tap-#{version}.dmg' "$cask" || { echo "no download url"; exit 1; }
grep -Fq 'app "Tap.app"' "$cask" || { echo "no app stanza"; exit 1; }
grep -Fq 'auto_updates true' "$cask" || { echo "Sparkle updates the app; the cask must say so"; exit 1; }
grep -Fq 'depends_on macos: :sonoma' "$cask" || { echo "no macOS floor"; exit 1; }
grep -Fq 'depends_on arch: :arm64' "$cask" || { echo "Apple silicon only; the cask must say so"; exit 1; }
if grep -q '__' "$cask"; then echo "a placeholder is left: $(grep '__' "$cask")"; exit 1; fi
ruby -c "$cask" >/dev/null || { echo "Ruby should parse the cask"; exit 1; }
if command -v brew >/dev/null 2>&1; then
	brew style "$cask" >/dev/null 2>&1 || { echo "brew style should pass"; brew style "$cask" || true; exit 1; }
else
	echo "brew is not installed here; brew style was not run (CI runs it)"
fi

if "$script" 2.1.0 "$root/missing.dmg" "$root/x.rb" >/dev/null 2>&1; then echo "a missing DMG should fail"; exit 1; fi
if "$script" "" "$root/Tap-2.1.0.dmg" "$root/x.rb" >/dev/null 2>&1; then echo "an empty version should fail"; exit 1; fi

echo "render-cask.sh is right"

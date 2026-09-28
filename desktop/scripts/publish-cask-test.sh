#!/bin/sh
# Checks publish-cask.sh: no token skips by name, a DMG that was not
# notarized skips by name even with the token (the token exists today, the
# Apple credentials do not), a pre-release skips by name, and with all
# three the cask lands as Casks/tap-desktop.rb in the tap, which here is a
# bare repository on disk reached through HOMEBREW_TAP_URL. The token
# never appears on stdout.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/publish-cask.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
printf 'cask "tap-desktop" do\nend\n' > "$root/tap-desktop.rb"

out=$(env -u HOMEBREW_TAP_TOKEN TAP_RELEASE_NOTARIZED=yes "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "no token should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (HOMEBREW_TAP_TOKEN is not set)" ] || { echo "wrong skip line: $out"; exit 1; }

out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=no "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "not notarized should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (the DMG is not notarized)" ] || { echo "wrong not-notarized line: $out"; exit 1; }
out=$(HOMEBREW_TAP_TOKEN=token env -u TAP_RELEASE_NOTARIZED "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "unknown state should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (the DMG is not notarized)" ] || { echo "an unset state is not notarized: $out"; exit 1; }

out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=yes "$script" 2.1.0-beta.1 "$root/tap-desktop.rb") || { echo "a pre-release should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (2.1.0-beta.1 is a pre-release)" ] || { echo "wrong pre-release line: $out"; exit 1; }

# A tap of the test's own: a bare repository with one commit, so the clone has a branch.
git init -q --bare "$root/tap.git"
git clone -q "$root/tap.git" "$root/seed" 2>/dev/null
( cd "$root/seed" && mkdir Formula && printf 'class Tap < Formula\nend\n' > Formula/tap.rb && git add . && git -c user.name=t -c user.email=t@t commit -q -m seed && git push -q origin HEAD 2>/dev/null )
out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=yes HOMEBREW_TAP_URL="$root/tap.git" "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "the push should succeed: $out"; exit 1; }
echo "$out" | grep -q '^pushed Casks/tap-desktop.rb for 2.1.0 (' || { echo "no pushed line: $out"; exit 1; }
git clone -q "$root/tap.git" "$root/check" 2>/dev/null
[ "$(cat "$root/check/Casks/tap-desktop.rb")" = "$(cat "$root/tap-desktop.rb")" ] || { echo "the cask in the tap differs"; exit 1; }
[ "$(cd "$root/check" && git log -1 --format=%s)" = "Update tap-desktop to 2.1.0" ] || { echo "wrong commit message"; exit 1; }
if echo "$out" | grep -q token; then echo "the token reached stdout"; exit 1; fi

# The same version again changes nothing and says so.
out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=yes HOMEBREW_TAP_URL="$root/tap.git" "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "a repeat should exit 0"; exit 1; }
[ "$out" = "the tap already has this cask for 2.1.0" ] || { echo "wrong repeat line: $out"; exit 1; }

echo "publish-cask.sh is right"

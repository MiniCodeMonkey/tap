#!/bin/sh
# Pushes a rendered cask to the Homebrew tap as Casks/tap-desktop.rb, the
# way release.yml's formula step updates Formula/tap.rb. Skips, by name,
# without HOMEBREW_TAP_TOKEN, unless TAP_RELEASE_NOTARIZED is yes (a cask
# must never point at a DMG Gatekeeper refuses; the token exists before
# the Apple credentials do), and for a pre-release (Homebrew's users get
# finals; Sparkle's feed does the same). HOMEBREW_TAP_REPO names the tap
# (default MiniCodeMonkey/homebrew-tap); HOMEBREW_TAP_URL replaces the
# whole clone URL, which the test uses for a repository on disk. The token
# reaches git as an Authorization header through git's environment
# configuration, never in the URL, argv or .git/config, and never in
# anything printed.
set -eu

version="${1:-}"; cask="${2:-}"
[ -n "$version" ] && [ -f "$cask" ] || { echo "publish-cask.sh: usage: publish-cask.sh <version> <cask.rb>" >&2; exit 1; }

if [ -z "${HOMEBREW_TAP_TOKEN:-}" ]; then
	echo "skipped: cask push (HOMEBREW_TAP_TOKEN is not set)"
	exit 0
fi
if [ "${TAP_RELEASE_NOTARIZED:-no}" != "yes" ]; then
	echo "skipped: cask push (the DMG is not notarized)"
	exit 0
fi
case "$version" in
	*-*) echo "skipped: cask push ($version is a pre-release)"; exit 0 ;;
esac

repository="${HOMEBREW_TAP_REPO:-MiniCodeMonkey/homebrew-tap}"
url="${HOMEBREW_TAP_URL:-https://github.com/${repository}.git}"
if [ -z "${HOMEBREW_TAP_URL:-}" ]; then
	export GIT_CONFIG_COUNT=1
	export GIT_CONFIG_KEY_0="http.https://github.com/.extraheader"
	GIT_CONFIG_VALUE_0="AUTHORIZATION: basic $(printf 'x-access-token:%s' "$HOMEBREW_TAP_TOKEN" | base64 | tr -d '\n')"
	export GIT_CONFIG_VALUE_0
fi
clone=$(mktemp -d)
trap 'rm -rf "$clone"' EXIT

# git's own messages go to stderr; stdout carries only this script's
# lines, which the release job's summary reads.
git clone --quiet "$url" "$clone/tap" 2>"$clone/git.log" || true
cat "$clone/git.log" >&2 || true
[ -d "$clone/tap/.git" ] || { echo "publish-cask.sh: could not clone the tap" >&2; exit 1; }
mkdir -p "$clone/tap/Casks"
cp "$cask" "$clone/tap/Casks/tap-desktop.rb"
cd "$clone/tap"
git add Casks/tap-desktop.rb
if git diff --cached --quiet; then
	echo "the tap already has this cask for $version"
	exit 0
fi
git -c user.name="github-actions[bot]" -c user.email="github-actions[bot]@users.noreply.github.com" commit --quiet -m "Update tap-desktop to $version"
git push --quiet origin HEAD 2>"$clone/git.log" || { cat "$clone/git.log" >&2; echo "publish-cask.sh: the push failed" >&2; exit 1; }
cat "$clone/git.log" >&2 || true
hash=$(git rev-parse --short HEAD)
echo "pushed Casks/tap-desktop.rb for $version ($hash)"

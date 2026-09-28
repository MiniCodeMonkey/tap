#!/bin/sh
# Makes a final release GitHub's "latest", which is where SUFeedURL points,
# only once that would not break the feed: when this release's signed
# appcast is uploaded, or when no earlier release carries one (nothing to
# take away). The CLI job publishes with make_latest false, so the tag
# exists and the desktop job decides. A pre-release is never latest. When
# an earlier feed is protected, the line says which command marks this
# release later. GH names the gh binary; the test gives a stand-in.
set -eu

version="${1:-}"; feed_uploaded="${2:-}"
[ -n "$version" ] && [ -n "$feed_uploaded" ] || { echo "mark-latest.sh: usage: mark-latest.sh <version> <feed-uploaded yes|no>" >&2; exit 1; }
gh="${GH:-gh}"
tag="v$version"

case "$version" in
	*-*) echo "skipped: marking $tag latest (a pre-release)"; exit 0 ;;
esac

if [ "$feed_uploaded" = yes ]; then
	"$gh" release edit "$tag" --latest >/dev/null
	echo "marked $tag latest (its feed is up)"
	exit 0
fi

# Fail closed: only "no release exists" (HTTP 404) means there is nothing
# to protect; any other failure (auth, network, a rate limit) marks nothing.
errors=$(mktemp)
trap 'rm -f "$errors"' EXIT
if latest=$("$gh" api 'repos/{owner}/{repo}/releases/latest' 2>"$errors"); then
	:
elif grep -q 'HTTP 404' "$errors"; then
	latest=""
else
	cat "$errors" >&2
	echo "mark-latest.sh: could not read the latest release; $tag is left as it is" >&2
	exit 1
fi
latest_tag=$(printf '%s' "$latest" | sed -n 's/.*"tag_name":"\([^"]*\)".*/\1/p')
case "$latest" in
	*'"name":"appcast.xml"'*)
		echo "skipped: marking $tag latest ($latest_tag carries the working feed; this release has none). Once $tag has a signed feed, run: gh release edit $tag --latest"
		exit 0
		;;
esac
"$gh" release edit "$tag" --latest >/dev/null
echo "marked $tag latest (no earlier release carries a feed)"

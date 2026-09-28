#!/bin/sh
# Checks mark-latest.sh against a stand-in gh that records its calls and
# answers the latest-release API from a file the test writes: a pre-release
# is never marked; a final with its feed uploaded is; a final without one is
# marked only when the current latest release carries no appcast.xml (so
# there is no working feed to take away) or there is no release at all; any
# other gh failure marks nothing and fails the step.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/mark-latest.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
cat > "$root/gh" <<FAKE
#!/bin/sh
printf '%s\n' "\$*" >> "$root/calls"
case "\$*" in
	"api repos/{owner}/{repo}/releases/latest")
		# The file holds the JSON gh api would print, or a word for a failure.
		case "\$(cat "$root/latest.json")" in
			404) echo "gh: Not Found (HTTP 404)" >&2; exit 1 ;;
			ERROR) echo "gh: error connecting to api.github.com" >&2; exit 1 ;;
			*) cat "$root/latest.json" ;;
		esac
		;;
esac
FAKE
chmod +x "$root/gh"
export GH="$root/gh"
calls() { cat "$root/calls" 2>/dev/null || true; rm -f "$root/calls"; }

out=$("$script" 2.1.0-beta.1 yes)
[ "$out" = "skipped: marking v2.1.0-beta.1 latest (a pre-release)" ] || { echo "wrong pre-release line: $out"; exit 1; }
[ -z "$(calls)" ] || { echo "a pre-release should call gh for nothing"; exit 1; }

out=$("$script" 2.1.0 yes)
[ "$out" = "marked v2.1.0 latest (its feed is up)" ] || { echo "wrong marked line: $out"; exit 1; }
[ "$(calls)" = "release edit v2.1.0 --latest" ] || { echo "the feed-up case should edit the release alone"; exit 1; }

printf '{"tag_name":"v2.0.0","assets":[{"name":"tap-darwin-arm64"},{"name":"appcast.xml"}]}\n' > "$root/latest.json"
out=$("$script" 2.1.0 no)
[ "$out" = "skipped: marking v2.1.0 latest (v2.0.0 carries the working feed; this release has none). Once v2.1.0 has a signed feed, run: gh release edit v2.1.0 --latest" ] || { echo "wrong protected line: $out"; exit 1; }
if calls | grep -q 'release edit'; then echo "a protected feed must not be replaced"; exit 1; fi

printf '{"tag_name":"v2.0.0","assets":[{"name":"tap-darwin-arm64"}]}\n' > "$root/latest.json"
out=$("$script" 2.1.0 no)
[ "$out" = "marked v2.1.0 latest (no earlier release carries a feed)" ] || { echo "wrong no-feed line: $out"; exit 1; }
calls | grep -q '^release edit v2.1.0 --latest$' || { echo "no earlier feed: the release should be marked"; exit 1; }

# No release at all: gh api answers 404, and there is nothing to protect.
echo 404 > "$root/latest.json"
out=$("$script" 2.1.0 no)
[ "$out" = "marked v2.1.0 latest (no earlier release carries a feed)" ] || { echo "no releases at all: $out"; exit 1; }
calls | grep -q '^release edit v2.1.0 --latest$' || { echo "no releases at all: the release should be marked"; exit 1; }

# Any other gh failure (auth, network, a rate limit) fails closed: nothing is marked.
echo ERROR > "$root/latest.json"
if out=$("$script" 2.1.0 no 2>"$root/err"); then echo "a gh error must not mark anything: $out"; exit 1; fi
grep -q 'could not read the latest release' "$root/err" || { echo "the gh error should be named: $(cat "$root/err")"; exit 1; }
if calls | grep -q 'release edit'; then echo "a gh error must not lead to an edit"; exit 1; fi

if "$script" 2.1.0 >/dev/null 2>&1; then echo "the feed state is required"; exit 1; fi

echo "mark-latest.sh is right"

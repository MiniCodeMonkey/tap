#!/bin/sh
# EdDSA signatures for the appcast, from the private key in
# SPARKLE_PRIVATE_KEY, which goes to sign_update on its standard input and
# nowhere else. Three modes:
#   archive <dmg>   prints the DMG's signature (verified before printing)
#   notes <file>    signs the release notes in place (sign_update prepends
#                   its warning) and prints "<signature> <length>"
#   feed <xml>      signs the appcast in place (a sparkle-signatures block)
# Without the key: one skip line on stderr, nothing on stdout, exit 0, and
# the file untouched. SPARKLE_TOOLS names the tools folder; it defaults to
# build/sparkle-tools-2.10.0 beside the scripts' parent.
set -eu

mode="${1:-}"; file="${2:-}"
case "$mode" in archive|notes|feed) ;; *) echo "sparkle-sign.sh: usage: sparkle-sign.sh archive|notes|feed <file>" >&2; exit 1 ;; esac
[ -f "$file" ] || { echo "sparkle-sign.sh: $file is missing" >&2; exit 1; }
here="$(cd "$(dirname "$0")" && pwd)"
tools="${SPARKLE_TOOLS:-$here/../build/sparkle-tools-2.10.0}"
name=$(basename "$file")

if [ -z "${SPARKLE_PRIVATE_KEY:-}" ]; then
	echo "skipped: Sparkle signature of $name (SPARKLE_PRIVATE_KEY is not set)" >&2
	exit 0
fi

"$here/fetch-sparkle-tools.sh" "$tools" >&2
sign_update="$tools/bin/sign_update"
with_key() { printf '%s\n' "$SPARKLE_PRIVATE_KEY" | "$sign_update" "$@"; }
# sign_update can exit 1 with nothing on stderr (a malformed key), so every
# failure gets a line of its own; the key is never part of it.
failed() { echo "sparkle-sign.sh: sign_update failed for $name" >&2; exit 1; }

case "$mode" in
	archive)
		signature=$(with_key --ed-key-file - -p "$file") || failed
		[ -n "$signature" ] || { echo "sparkle-sign.sh: sign_update printed no signature for $name" >&2; exit 1; }
		with_key --ed-key-file - --verify "$file" "$signature" >&2 || failed
		printf '%s\n' "$signature"
		;;
	notes)
		printed=$(with_key --ed-key-file - "$file") || failed
		attributes=$(printf '%s\n' "$printed" | grep 'sparkle:edSignature=' || true)
		signature=$(printf '%s' "$attributes" | sed -n 's/.*sparkle:edSignature="\([^"]*\)".*/\1/p')
		length=$(printf '%s' "$attributes" | sed -n 's/.*sparkle:length="\([^"]*\)".*/\1/p')
		[ -n "$signature" ] && [ -n "$length" ] || { echo "sparkle-sign.sh: sign_update printed no signature and length for $name" >&2; exit 1; }
		printf '%s %s\n' "$signature" "$length"
		;;
	feed)
		with_key --ed-key-file - "$file" >&2 || failed
		grep -q 'sparkle-signatures:' "$file" || { echo "sparkle-sign.sh: $name carries no signature block after signing" >&2; exit 1; }
		;;
esac

#!/bin/sh
# Submits a file (a zip of the app, or the DMG) to Apple's notary service
# with an App Store Connect API key, waits for the verdict, and staples the
# ticket to the target (the app the zip holds, or the DMG itself). The key
# exists on disk only in a private temporary folder for the length of the
# run. Without the three secrets: one skip line naming what is missing,
# exit 0, nothing touched.
# APPLE_NOTARY_KEY_ID and APPLE_NOTARY_ISSUER_ID are arguments of notarytool
# (an accepted exposure: identifiers, useless without the .p8, and notarytool
# takes them no other way); the key itself reaches notarytool only as the
# private file.
set -eu

file="${1:-}"; target="${2:-}"
[ -f "$file" ] || { echo "notarize.sh: $file is missing" >&2; exit 1; }
[ -e "$target" ] || { echo "notarize.sh: $target is missing" >&2; exit 1; }
name=$(basename "$file")

# None set: the skip line names the first, as release.sh does. One or two
# set (a secret's name mistyped, say): the line names every missing one,
# and a second line on stderr says the set is half configured.
missing=""; count=0
for secret in APPLE_NOTARY_KEY APPLE_NOTARY_KEY_ID APPLE_NOTARY_ISSUER_ID; do
	eval "value=\${$secret:-}"
	if [ -z "$value" ]; then
		missing="${missing:+$missing and }$secret"
		count=$((count + 1))
	fi
done
case "$count" in
	0) ;;
	1) echo "skipped: notarization of $name ($missing is not set)" ;;
	2) echo "skipped: notarization of $name ($missing are not set)" ;;
	3) echo "skipped: notarization of $name (APPLE_NOTARY_KEY is not set)" ;;
esac
if [ "$count" = 1 ] || [ "$count" = 2 ]; then
	echo "notarize.sh: only some of the three notary secrets are set; missing $missing" >&2
fi
[ "$count" = 0 ] || exit 0

private=$(mktemp -d)
chmod 700 "$private"
trap 'rm -rf "$private"' EXIT
umask 077
printf '%s\n' "$APPLE_NOTARY_KEY" > "$private/AuthKey.p8"

xcrun notarytool submit "$file" --key "$private/AuthKey.p8" --key-id "$APPLE_NOTARY_KEY_ID" --issuer "$APPLE_NOTARY_ISSUER_ID" --wait --timeout 30m --output-format json > "$private/result.json" || true
status=$(/usr/bin/plutil -extract status raw -o - "$private/result.json" 2>/dev/null || true)
submission=$(/usr/bin/plutil -extract id raw -o - "$private/result.json" 2>/dev/null || true)
if [ "$status" != "Accepted" ]; then
	echo "notarize.sh: $name was not accepted (status: ${status:-none}, submission: ${submission:-none})" >&2
	if [ -n "$submission" ]; then
		xcrun notarytool log "$submission" --key "$private/AuthKey.p8" --key-id "$APPLE_NOTARY_KEY_ID" --issuer "$APPLE_NOTARY_ISSUER_ID" >&2 || true
	fi
	exit 1
fi
xcrun stapler staple "$target" >/dev/null
echo "notarized $name (submission $submission) and stapled $(basename "$target")"

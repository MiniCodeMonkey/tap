#!/bin/sh
# Checks notarize.sh's skip path: without the three notary secrets it prints
# one line naming the first missing one and exits 0 without touching the
# target. The real path runs on CI with the secrets.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/notarize.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
printf 'zip' > "$root/Tap.zip"
mkdir -p "$root/Tap.app/Contents"

out=$(env -u APPLE_NOTARY_KEY -u APPLE_NOTARY_KEY_ID -u APPLE_NOTARY_ISSUER_ID "$script" "$root/Tap.zip" "$root/Tap.app") || { echo "no secrets should exit 0"; exit 1; }
[ "$out" = "skipped: notarization of Tap.zip (APPLE_NOTARY_KEY is not set)" ] || { echo "wrong skip line: $out"; exit 1; }

out=$(APPLE_NOTARY_KEY=key env -u APPLE_NOTARY_KEY_ID -u APPLE_NOTARY_ISSUER_ID "$script" "$root/Tap.zip" "$root/Tap.app")
[ "$out" = "skipped: notarization of Tap.zip (APPLE_NOTARY_KEY_ID is not set)" ] || { echo "wrong second skip line: $out"; exit 1; }

out=$(APPLE_NOTARY_KEY=key APPLE_NOTARY_KEY_ID=id env -u APPLE_NOTARY_ISSUER_ID "$script" "$root/Tap.zip" "$root/Tap.app")
[ "$out" = "skipped: notarization of Tap.zip (APPLE_NOTARY_ISSUER_ID is not set)" ] || { echo "wrong third skip line: $out"; exit 1; }

if "$script" "$root/missing.zip" "$root/Tap.app" >/dev/null 2>&1; then echo "a missing file should fail"; exit 1; fi
[ -z "$(ls -A "$root/Tap.app/Contents")" ] || { echo "the skip path touched the target"; exit 1; }

echo "notarize.sh is right"

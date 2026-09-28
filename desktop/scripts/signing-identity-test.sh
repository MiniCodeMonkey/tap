#!/bin/sh
# Checks signing-identity.sh's paths that need no certificate: import with
# no secret prints "-" and a skip line; remove with no keychain is quiet;
# a secret that is not a p12 fails and leaves no keychain behind and no
# entry in the search list.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/signing-identity.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
export TAP_SIGNING_KEYCHAIN_FILE="$root/test.keychain-db"
before=$(security list-keychains -d user)

out=$(env -u APPLE_DEVELOPER_ID_APPLICATION_P12 "$script" import 2>"$root/err") || { echo "no certificate should exit 0"; exit 1; }
[ "$out" = "-" ] || { echo "no certificate should print -, got '$out'"; exit 1; }
grep -Fxq 'skipped: Developer ID signing (APPLE_DEVELOPER_ID_APPLICATION_P12 is not set)' "$root/err" || { echo "wrong skip line: $(cat "$root/err")"; exit 1; }
[ ! -f "$TAP_SIGNING_KEYCHAIN_FILE" ] || { echo "no keychain without a certificate"; exit 1; }

"$script" remove || { echo "remove with no keychain should exit 0"; exit 1; }

if APPLE_DEVELOPER_ID_APPLICATION_P12="$(printf 'not a certificate' | base64)" APPLE_DEVELOPER_ID_APPLICATION_PASSWORD=pw "$script" import >"$root/out" 2>"$root/err"; then
	echo "a broken p12 should fail"; exit 1
fi
if grep -q 'not a certificate' "$root/err" "$root/out"; then echo "the secret reached the output"; exit 1; fi
[ ! -f "$TAP_SIGNING_KEYCHAIN_FILE" ] || { echo "a failed import should remove its keychain"; exit 1; }
[ "$(security list-keychains -d user)" = "$before" ] || { echo "a failed import changed the search list"; exit 1; }

echo "signing-identity.sh is right"

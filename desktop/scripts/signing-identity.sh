#!/bin/sh
# import: makes a keychain of its own from the Developer ID Application
# certificate in APPLE_DEVELOPER_ID_APPLICATION_P12 (base64 of the .p12)
# and APPLE_DEVELOPER_ID_APPLICATION_PASSWORD, adds it to the search list,
# and prints the identity's name for codesign. Without the certificate it
# prints "-" (ad-hoc) and a skip line on stderr. An import that fails at
# any step removes the keychain again before exiting.
# remove: deletes that keychain and takes it out of the search list.
# The .p12 exists on disk only inside a private temporary folder for the
# length of the import; its password is an argument of security import
# (an accepted exposure on a single-tenant runner). TAP_SIGNING_KEYCHAIN_FILE
# names the keychain (default build/tap-release.keychain-db next to the
# scripts' parent); the path is made canonical, since security prints
# canonical paths (/private/var for /var) in the search list.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
keychain="${TAP_SIGNING_KEYCHAIN_FILE:-$here/../build/tap-release.keychain-db}"
mkdir -p "$(dirname "$keychain")"
keychain="$(cd "$(dirname "$keychain")" && pwd -P)/$(basename "$keychain")"
command="${1:-}"

remove_keychain() {
	# The search list without ours; untouched when ours is not in it.
	if security list-keychains -d user | grep -Fq "$keychain"; then
		remaining=$(security list-keychains -d user | tr -d '" ' | grep -Fv "$keychain" || true)
		# shellcheck disable=SC2086
		security list-keychains -d user -s $remaining
	fi
	if [ -f "$keychain" ]; then
		security delete-keychain "$keychain" >/dev/null 2>&1 || rm -f "$keychain"
	fi
}

case "$command" in
	import)
		if [ -z "${APPLE_DEVELOPER_ID_APPLICATION_P12:-}" ]; then
			echo "skipped: Developer ID signing (APPLE_DEVELOPER_ID_APPLICATION_P12 is not set)" >&2
			echo "-"
			exit 0
		fi
		[ -n "${APPLE_DEVELOPER_ID_APPLICATION_PASSWORD:-}" ] || { echo "signing-identity.sh: APPLE_DEVELOPER_ID_APPLICATION_PASSWORD is not set" >&2; exit 1; }
		private=$(mktemp -d)
		chmod 700 "$private"
		imported=no
		# Whatever fails below, the private folder goes and so does a
		# half-made keychain; only a finished import keeps it.
		trap 'rm -rf "$private"; [ "$imported" = yes ] || remove_keychain' EXIT
		umask 077
		printf '%s' "$APPLE_DEVELOPER_ID_APPLICATION_P12" | base64 -d > "$private/certificate.p12" 2>/dev/null || { echo "signing-identity.sh: the certificate secret is not base64" >&2; exit 1; }
		keychain_password=$(head -c 24 /dev/urandom | base64)
		remove_keychain
		security create-keychain -p "$keychain_password" "$keychain"
		security set-keychain-settings -lut 21600 "$keychain"
		security unlock-keychain -p "$keychain_password" "$keychain"
		if ! security import "$private/certificate.p12" -k "$keychain" -P "$APPLE_DEVELOPER_ID_APPLICATION_PASSWORD" -T /usr/bin/codesign -T /usr/bin/security >/dev/null 2>&1; then
			echo "signing-identity.sh: the certificate did not import (wrong password, or not a .p12)" >&2
			exit 1
		fi
		security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$keychain_password" "$keychain" >/dev/null
		existing=$(security list-keychains -d user | tr -d '" ')
		# shellcheck disable=SC2086
		security list-keychains -d user -s "$keychain" $existing
		identity=$(security find-identity -v -p codesigning "$keychain" | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -n 1)
		if [ -z "$identity" ]; then
			echo "signing-identity.sh: no Developer ID Application identity in the certificate" >&2
			exit 1
		fi
		imported=yes
		echo "$identity"
		;;
	remove)
		remove_keychain
		;;
	*)
		echo "signing-identity.sh: usage: signing-identity.sh import|remove" >&2
		exit 1
		;;
esac

#!/bin/sh
# import: makes a keychain of its own from the Developer ID Application
# certificate in APPLE_DEVELOPER_ID_APPLICATION_P12 (base64 of the .p12)
# and APPLE_DEVELOPER_ID_APPLICATION_PASSWORD, adds it to the search list,
# and prints the identity's name for codesign. Without the certificate it
# prints "-" (ad-hoc) and a skip line on stderr. An import that fails at
# any step removes the keychain again before exiting.
# remove: deletes that keychain and takes it out of the search list.
# The .p12 exists on disk only inside a private temporary folder for the
# length of the import. Two values are arguments of security, accepted
# exposures on a single-tenant runner: the .p12's password (of import) and
# the temporary keychain's own password, a random value made here for one
# job (of create-keychain, unlock-keychain and set-key-partition-list).
# TAP_SIGNING_KEYCHAIN_FILE names the keychain (default
# build/tap-release.keychain-db next to the scripts' parent); the path is
# made canonical, since security prints canonical paths (/private/var for
# /var) in the search list.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
keychain="${TAP_SIGNING_KEYCHAIN_FILE:-$here/../build/tap-release.keychain-db}"
mkdir -p "$(dirname "$keychain")"
keychain="$(cd "$(dirname "$keychain")" && pwd -P)/$(basename "$keychain")"
command="${1:-}"

# The user search list, one path per line and each kept whole (a path may
# hold spaces): security prints every entry indented and in double quotes.
search_list() {
	security list-keychains -d user | sed -e 's/^[[:space:]]*"//' -e 's/"[[:space:]]*$//'
}

# Sets the user search list to the arguments followed by every current
# entry but ours, in their order, each passed as one argument.
set_search_list() {
	current=$(search_list)
	while IFS= read -r entry; do
		if [ -n "$entry" ] && [ "$entry" != "$keychain" ]; then set -- "$@" "$entry"; fi
	done <<LIST
$current
LIST
	security list-keychains -d user -s "$@"
}

remove_keychain() {
	# The search list without ours; untouched when ours is not in it.
	if search_list | grep -Fxq "$keychain"; then
		set_search_list
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
		# Ours first, once (create-keychain may have added it already).
		set_search_list "$keychain"
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

#!/bin/sh
# Checks signing-identity.sh against a stand-in for security(1) that keeps
# the search list in a file and logs every call, so no scenario touches the
# real keychains or search list. Import with no secret prints "-" and a
# skip line; remove with no keychain is quiet; a .p12 without its password
# fails before anything is made; an import that fails at the import, after
# it or at the identity removes its keychain and restores the list; a
# successful import prints the identity, puts its keychain first in the
# list and keeps the others whole (one holds a space), and remove restores
# the list. In every case the .p12 existed only as a 600 file in a private
# folder that is gone afterwards, and no secret reached stdout or stderr.
# The .p12 password is an argument of security import alone (the accepted
# exposure).
set -eu

script="$(cd "$(dirname "$0")" && pwd)/signing-identity.sh"
root=$(mktemp -d)
mkdir -p "$root/bin" "$root/fake" "$root/Library/Keychains"
fake="$root/fake"
# mktemp -d ignores TMPDIR on macOS, so the script's private folder is in
# the system's temporary folder; the stand-in records each one, and a
# folder a broken script leaves behind is removed here after the checks.
cleanup() {
	if [ -f "$fake/privates" ]; then
		while IFS= read -r folder; do
			case "$(basename "$folder")" in tmp.*) rm -rf "$folder" ;; esac
		done < "$fake/privates"
	fi
	rm -rf "$root"
}
trap cleanup EXIT

cat > "$root/bin/security" <<SHIM
#!/bin/sh
# The search list lives in $fake/list, in the format security prints it.
printf '%s\n' "\$*" >> "$fake/argv"
command="\$1"; shift
if [ "\${FAIL_AT:-}" = "\$command" ]; then echo "security: \$command failed" >&2; exit 1; fi
case "\$command" in
	list-keychains)
		if [ "\$1 \$2" = "-d user" ] && [ "\${3:-}" = "-s" ]; then
			shift 3
			: > "$fake/list"
			for entry; do printf '    "%s"\n' "\$entry" >> "$fake/list"; done
		else
			cat "$fake/list"
		fi ;;
	create-keychain)
		for path; do :; done
		: > "\$path"
		printf '    "%s"\n' "\$path" >> "$fake/list" ;;
	delete-keychain)
		rm -f "\$1"
		grep -Fvx "    \"\$1\"" "$fake/list" > "$fake/list.new" || true
		mv "$fake/list.new" "$fake/list" ;;
	set-keychain-settings|unlock-keychain|set-key-partition-list) ;;
	import)
		dirname "\$1" > "$fake/private"
		dirname "\$1" >> "$fake/privates"
		stat -f %Lp "\$1" > "$fake/p12-mode"
		stat -f %Lp "\$(dirname "\$1")" > "$fake/private-mode"
		cp "\$1" "$fake/p12-seen"
		[ "\${GOOD_P12:-}" = yes ] || exit 1 ;;
	find-identity)
		[ -n "\${NO_IDENTITY:-}" ] || echo '  1) ABCDEF0123 "Developer ID Application: Test Person (TEAM123456)"'
		echo '     1 valid identities found' ;;
	*) echo "unexpected security \$command" >&2; exit 3 ;;
esac
SHIM
chmod +x "$root/bin/security"
PATH="$root/bin:$PATH"
export PATH
[ "$(command -v security)" = "$root/bin/security" ] || { echo "the security stand-in is not first on PATH; refusing to run"; exit 1; }

export TAP_SIGNING_KEYCHAIN_FILE="$root/test.keychain-db"
keychain="$(cd "$root" && pwd -P)/test.keychain-db"
printf '    "%s"\n' "$root/Library/Keychains/login.keychain-db" "$root/Library/Keychains/Work Keys.keychain-db" > "$fake/list"
before=$(cat "$fake/list")
password="pw-3b8e2d"
certificate="p12-bytes-9f1c"
encoded=$(printf '%s' "$certificate" | base64)

# Resets the stand-in's logs between scenarios.
fresh() { rm -f "$fake/private" "$fake/p12-mode" "$fake/private-mode" "$fake/p12-seen"; : > "$fake/argv"; }

# What every import that reached security import leaves true.
check_private() {
	scenario="$1"
	[ -f "$fake/private" ] || { echo "$scenario: security import never ran"; exit 1; }
	[ ! -e "$(cat "$fake/private")" ] || { echo "$scenario: the private .p12 folder is still there"; exit 1; }
	[ "$(cat "$fake/p12-mode")" = 600 ] || { echo "$scenario: the .p12 was mode $(cat "$fake/p12-mode"), not 600"; exit 1; }
	[ "$(cat "$fake/private-mode")" = 700 ] || { echo "$scenario: the private folder was mode $(cat "$fake/private-mode"), not 700"; exit 1; }
	[ "$(cat "$fake/p12-seen")" = "$certificate" ] || { echo "$scenario: the .p12 on disk is not the decoded secret"; exit 1; }
	for secret in "$password" "$certificate" "$encoded"; do
		if grep -Fq "$secret" "$root/out" "$root/err"; then echo "$scenario: a secret reached the output"; exit 1; fi
	done
	if grep -F "$password" "$fake/argv" | grep -qv '^import '; then echo "$scenario: the .p12 password reached a call other than security import"; exit 1; fi
}

# A failed import leaves no keychain and the list as it was.
check_failed() {
	scenario="$1"
	check_private "$scenario"
	[ ! -f "$keychain" ] || { echo "$scenario: a failed import should remove its keychain"; exit 1; }
	grep -Fxq "delete-keychain $keychain" "$fake/argv" || { echo "$scenario: the keychain was not deleted through security"; exit 1; }
	[ "$(cat "$fake/list")" = "$before" ] || { echo "$scenario: a failed import changed the search list: $(cat "$fake/list")"; exit 1; }
}

fresh
out=$(env -u APPLE_DEVELOPER_ID_APPLICATION_P12 "$script" import 2>"$root/err") || { echo "no certificate should exit 0"; exit 1; }
[ "$out" = "-" ] || { echo "no certificate should print -, got '$out'"; exit 1; }
grep -Fxq 'skipped: Developer ID signing (APPLE_DEVELOPER_ID_APPLICATION_P12 is not set)' "$root/err" || { echo "wrong skip line: $(cat "$root/err")"; exit 1; }
[ ! -f "$keychain" ] || { echo "no keychain without a certificate"; exit 1; }
[ ! -s "$fake/argv" ] || { echo "no certificate should run no security call: $(cat "$fake/argv")"; exit 1; }

fresh
"$script" remove || { echo "remove with no keychain should exit 0"; exit 1; }
if grep -q -- ' -s' "$fake/argv"; then echo "remove with no keychain should leave the search list alone"; exit 1; fi

fresh
if APPLE_DEVELOPER_ID_APPLICATION_P12="$encoded" env -u APPLE_DEVELOPER_ID_APPLICATION_PASSWORD "$script" import >"$root/out" 2>"$root/err"; then
	echo "a .p12 without its password should fail"; exit 1
fi
grep -Fxq 'signing-identity.sh: APPLE_DEVELOPER_ID_APPLICATION_PASSWORD is not set' "$root/err" || { echo "wrong missing-password line: $(cat "$root/err")"; exit 1; }
[ ! -s "$fake/argv" ] && [ ! -f "$keychain" ] || { echo "a missing password should stop before anything is made"; exit 1; }
if grep -Fq "$certificate" "$root/out" "$root/err"; then echo "the certificate reached the output"; exit 1; fi

fresh
if APPLE_DEVELOPER_ID_APPLICATION_P12="$encoded" APPLE_DEVELOPER_ID_APPLICATION_PASSWORD="$password" "$script" import >"$root/out" 2>"$root/err"; then
	echo "a broken p12 should fail"; exit 1
fi
grep -Fq 'the certificate did not import' "$root/err" || { echo "wrong broken-p12 line: $(cat "$root/err")"; exit 1; }
check_failed "a broken p12"

fresh
if GOOD_P12=yes FAIL_AT=set-key-partition-list APPLE_DEVELOPER_ID_APPLICATION_P12="$encoded" APPLE_DEVELOPER_ID_APPLICATION_PASSWORD="$password" "$script" import >"$root/out" 2>"$root/err"; then
	echo "a failure after the import should fail"; exit 1
fi
check_failed "a failure after the import"

fresh
if GOOD_P12=yes NO_IDENTITY=1 APPLE_DEVELOPER_ID_APPLICATION_P12="$encoded" APPLE_DEVELOPER_ID_APPLICATION_PASSWORD="$password" "$script" import >"$root/out" 2>"$root/err"; then
	echo "a certificate with no Developer ID identity should fail"; exit 1
fi
grep -Fq 'no Developer ID Application identity' "$root/err" || { echo "wrong no-identity line: $(cat "$root/err")"; exit 1; }
check_failed "no identity"

fresh
GOOD_P12=yes APPLE_DEVELOPER_ID_APPLICATION_P12="$encoded" APPLE_DEVELOPER_ID_APPLICATION_PASSWORD="$password" "$script" import >"$root/out" 2>"$root/err" || { echo "a good certificate should import: $(cat "$root/err")"; exit 1; }
[ "$(cat "$root/out")" = "Developer ID Application: Test Person (TEAM123456)" ] || { echo "the import should print the identity, got '$(cat "$root/out")'"; exit 1; }
check_private "a good import"
[ -f "$keychain" ] || { echo "a good import keeps its keychain"; exit 1; }
expected=$(printf '    "%s"\n%s' "$keychain" "$before")
[ "$(cat "$fake/list")" = "$expected" ] || { echo "a good import should put its keychain first and keep the rest: $(cat "$fake/list")"; exit 1; }

fresh
"$script" remove || { echo "remove after an import should exit 0"; exit 1; }
[ ! -f "$keychain" ] || { echo "remove should delete the keychain"; exit 1; }
grep -Fxq "delete-keychain $keychain" "$fake/argv" || { echo "remove should delete the keychain through security"; exit 1; }
[ "$(cat "$fake/list")" = "$before" ] || { echo "remove should restore the search list: $(cat "$fake/list")"; exit 1; }

echo "signing-identity.sh is right"

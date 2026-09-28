#!/bin/sh
# Checks notarize.sh against a stand-in for xcrun (notarytool and stapler),
# so nothing reaches Apple. Without any notary secret it prints one line
# naming the first, exits 0, writes nothing to stderr and runs no tool;
# with one or two it names every missing one and says so on stderr too.
# With all three: Accepted staples the target once and says so; Invalid,
# a submission still In Progress at the timeout and a notarytool
# that fails with no JSON each exit 1 and staple nothing, with the notary
# log on stderr when there is a submission. In every submission the .p8
# notarytool read was the secret, mode 600, in a private folder that is
# gone afterwards, and the key's body appears in no output and no argv.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/notarize.sh"
root=$(mktemp -d)
mkdir -p "$root/bin" "$root/fake"
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
printf 'zip' > "$root/Tap.zip"
mkdir -p "$root/Tap.app/Contents"

cat > "$root/bin/xcrun" <<SHIM
#!/bin/sh
printf '%s\n' "\$*" >> "$fake/argv"
case "\$1 \$2" in
	"notarytool submit")
		previous=""
		for argument; do
			if [ "\$previous" = --key ]; then
				cp "\$argument" "$fake/key-seen"
				stat -f %Lp "\$argument" > "$fake/key-mode"
				dirname "\$argument" > "$fake/private"
				dirname "\$argument" >> "$fake/privates"
			fi
			previous="\$argument"
		done
		case "\$NOTARY" in
			accepted) echo '{"id":"11111111-2222-3333-4444-555555555555","status":"Accepted","message":"Processing complete"}' ;;
			invalid) echo '{"id":"11111111-2222-3333-4444-555555555555","status":"Invalid","message":"Processing complete"}'; exit 1 ;;
			timeout) echo '{"id":"11111111-2222-3333-4444-555555555555","status":"In Progress","message":"timed out"}'; exit 1 ;;
			crash) echo "Error: HTTP status code: 401. Unable to authenticate." >&2; exit 1 ;;
		esac ;;
	"notarytool log") echo '{"issues":[{"message":"The binary is not signed with a valid Developer ID certificate."}]}' ;;
	"stapler staple") printf 'staple %s\n' "\$3" >> "$fake/stapled"; echo "The staple and validate action worked!" ;;
	*) echo "unexpected xcrun \$*" >&2; exit 3 ;;
esac
SHIM
chmod +x "$root/bin/xcrun"
PATH="$root/bin:$PATH"
export PATH
[ "$(command -v xcrun)" = "$root/bin/xcrun" ] || { echo "the xcrun stand-in is not first on PATH; refusing to run"; exit 1; }

key_body="MIGTAgEAMBMGByqGSM49-fake-notary-key-5e0b"
key=$(printf -- '-----BEGIN PRIVATE KEY-----\n%s\n-----END PRIVATE KEY-----' "$key_body")
submission="11111111-2222-3333-4444-555555555555"
fresh() { rm -f "$fake/key-seen" "$fake/key-mode" "$fake/private" "$fake/stapled"; : > "$fake/argv"; }

# The skip path: stdout is the one line, stderr is the expected text (empty
# with no secret at all), no tool runs, and no secret's value is printed.
skip() {
	expected="$1"; expected_error="$2"; shift 2
	fresh
	out=$(env "$@" "$script" "$root/Tap.zip" "$root/Tap.app" 2>"$root/err") || { echo "a skip should exit 0: $(cat "$root/err")"; exit 1; }
	[ "$out" = "$expected" ] || { echo "wrong skip line: $out"; exit 1; }
	[ "$(cat "$root/err")" = "$expected_error" ] || { echo "wrong stderr for '$expected': $(cat "$root/err")"; exit 1; }
	[ ! -s "$fake/argv" ] || { echo "a skip should run no tool: $(cat "$fake/argv")"; exit 1; }
	if printf '%s' "$out" | grep -Fq -e "$key_body" -e KEYID12345 -e issuer-0000; then echo "a skip printed a secret's value"; exit 1; fi
}
half="notarize.sh: only some of the three notary secrets are set; missing"
skip "skipped: notarization of Tap.zip (APPLE_NOTARY_KEY is not set)" "" -u APPLE_NOTARY_KEY -u APPLE_NOTARY_KEY_ID -u APPLE_NOTARY_ISSUER_ID
skip "skipped: notarization of Tap.zip (APPLE_NOTARY_KEY_ID and APPLE_NOTARY_ISSUER_ID are not set)" "$half APPLE_NOTARY_KEY_ID and APPLE_NOTARY_ISSUER_ID" -u APPLE_NOTARY_KEY_ID -u APPLE_NOTARY_ISSUER_ID APPLE_NOTARY_KEY="$key"
skip "skipped: notarization of Tap.zip (APPLE_NOTARY_ISSUER_ID is not set)" "$half APPLE_NOTARY_ISSUER_ID" -u APPLE_NOTARY_ISSUER_ID APPLE_NOTARY_KEY="$key" APPLE_NOTARY_KEY_ID=KEYID12345
skip "skipped: notarization of Tap.zip (APPLE_NOTARY_KEY and APPLE_NOTARY_ISSUER_ID are not set)" "$half APPLE_NOTARY_KEY and APPLE_NOTARY_ISSUER_ID" -u APPLE_NOTARY_KEY -u APPLE_NOTARY_ISSUER_ID APPLE_NOTARY_KEY_ID=KEYID12345
skip "skipped: notarization of Tap.zip (APPLE_NOTARY_KEY is not set)" "$half APPLE_NOTARY_KEY" -u APPLE_NOTARY_KEY APPLE_NOTARY_KEY_ID=KEYID12345 APPLE_NOTARY_ISSUER_ID=issuer-0000

if "$script" "$root/missing.zip" "$root/Tap.app" >/dev/null 2>&1; then echo "a missing file should fail"; exit 1; fi
[ -z "$(ls -A "$root/Tap.app/Contents")" ] || { echo "the skip path touched the target"; exit 1; }

# A submission with all three secrets; the verdict comes from $1.
submit() {
	fresh
	status=0
	NOTARY="$1" APPLE_NOTARY_KEY="$key" APPLE_NOTARY_KEY_ID=KEYID12345 APPLE_NOTARY_ISSUER_ID=issuer-0000 "$script" "$root/Tap.zip" "$root/Tap.app" >"$root/out" 2>"$root/err" || status=$?
	[ -f "$fake/key-seen" ] || { echo "$1: notarytool never read a key"; exit 1; }
	[ "$(cat "$fake/key-seen")" = "$key" ] || { echo "$1: the .p8 notarytool read is not the secret"; exit 1; }
	[ "$(cat "$fake/key-mode")" = 600 ] || { echo "$1: the .p8 was mode $(cat "$fake/key-mode"), not 600"; exit 1; }
	[ ! -e "$(cat "$fake/private")" ] || { echo "$1: the private .p8 folder is still there"; exit 1; }
	if grep -Fq "$key_body" "$root/out" "$root/err" "$fake/argv"; then echo "$1: the key reached the output or an argument"; exit 1; fi
}

submit accepted
[ "$status" = 0 ] || { echo "accepted should exit 0: $(cat "$root/err")"; exit 1; }
[ "$(cat "$root/out")" = "notarized Tap.zip (submission $submission) and stapled Tap.app" ] || { echo "wrong accepted line: $(cat "$root/out")"; exit 1; }
[ "$(cat "$fake/stapled")" = "staple $root/Tap.app" ] || { echo "accepted should staple the target once: $(cat "$fake/stapled" 2>/dev/null)"; exit 1; }

for verdict in invalid timeout; do
	submit "$verdict"
	[ "$status" = 1 ] || { echo "$verdict should exit 1, not $status"; exit 1; }
	[ ! -f "$fake/stapled" ] || { echo "$verdict should staple nothing"; exit 1; }
	[ ! -s "$root/out" ] || { echo "$verdict should print nothing on stdout: $(cat "$root/out")"; exit 1; }
	grep -Fq "Tap.zip was not accepted" "$root/err" || { echo "$verdict: no refusal line: $(cat "$root/err")"; exit 1; }
	grep -Fq "submission: $submission" "$root/err" || { echo "$verdict: the refusal should name the submission"; exit 1; }
	grep -Fq 'The binary is not signed' "$root/err" || { echo "$verdict: the notary log should be on stderr"; exit 1; }
done
grep -Fq "status: In Progress" "$root/err" || { echo "timeout: the status should be named"; exit 1; }

submit crash
[ "$status" = 1 ] || { echo "a failing notarytool should exit 1, not $status"; exit 1; }
[ ! -f "$fake/stapled" ] || { echo "a failing notarytool should staple nothing"; exit 1; }
grep -Fq "(status: none, submission: none)" "$root/err" || { echo "crash: wrong refusal line: $(cat "$root/err")"; exit 1; }
if grep -q '^notarytool log' "$fake/argv"; then echo "crash: no submission, so no log call"; exit 1; fi

echo "notarize.sh is right"

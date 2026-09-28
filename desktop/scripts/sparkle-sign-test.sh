#!/bin/sh
# Checks sparkle-sign.sh's three modes against a stand-in sign_update that
# records what it was given, and its skip path (no key: nothing on stdout,
# one line on stderr, exit 0, no network). The key reaches sign_update on
# standard input and nowhere else. The real tool runs on CI with the secret.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/sparkle-sign.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
printf 'dmg' > "$root/Tap.dmg"
printf '# Notes\n' > "$root/Tap.md"
printf '<?xml version="1.0"?><rss/>\n' > "$root/appcast.xml"

for mode in archive notes feed; do
	out=$(env -u SPARKLE_PRIVATE_KEY "$script" "$mode" "$root/Tap.dmg" 2>"$root/err") || { echo "$mode: no key should exit 0"; exit 1; }
	[ -z "$out" ] || { echo "$mode: no key should print nothing, got '$out'"; exit 1; }
	grep -Fxq 'skipped: Sparkle signature of Tap.dmg (SPARKLE_PRIVATE_KEY is not set)' "$root/err" || { echo "$mode: the skip line is wrong: $(cat "$root/err")"; exit 1; }
done
out=$(SPARKLE_PRIVATE_KEY="" "$script" archive "$root/Tap.dmg" 2>/dev/null) || { echo "an empty key is no key"; exit 1; }
[ -z "$out" ] || { echo "an empty key should print nothing"; exit 1; }
if "$script" archive "$root/missing.dmg" >/dev/null 2>&1; then echo "a missing file should fail"; exit 1; fi
if "$script" sign "$root/Tap.dmg" >/dev/null 2>&1; then echo "an unknown mode should fail"; exit 1; fi

# The stand-in: records its arguments and its stdin, answers as sign_update
# does for each kind of file, and never verifies anything but "ok". A key
# that does not come on stdin fails it at once: a terminal on stdin, or
# nothing within 5 seconds, is an error, never a wait.
# The tools folder carries the version, as fetch-sparkle-tools.sh requires.
mkdir -p "$root/tools-2.10.0/bin"
cat > "$root/tools-2.10.0/bin/sign_update" <<'FAKE'
#!/bin/sh
here="$(dirname "$0")"
printf '%s\n' "$*" >> "$here/args"
case "$*" in
	*--verify*) exit 0 ;;
esac
if [ -t 0 ]; then echo "sign_update stand-in: stdin is a terminal, so the key did not come on stdin" >&2; exit 2; fi
# A background command reads /dev/null unless stdin is handed to it.
exec 3<&0
cat <&3 > "$here/stdin" &
reader=$!
( sleep 5; kill "$reader" 2>/dev/null ) >/dev/null 2>&1 &
watchdog=$!
if ! wait "$reader"; then echo "sign_update stand-in: nothing came on stdin within 5 seconds, so the key did not" >&2; exit 2; fi
kill "$watchdog" 2>/dev/null || true
# The file is the last argument.
for file; do :; done
case "$*" in
	*" -p "*) echo "QVJDSElWRQ==" ;;
	*.xml) printf '<!-- sparkle-signatures:\nedSignature: RkVFRA==\nlength: 1\n-->' >> "$file"; exit 0 ;;
	*) printf 'sparkle:edSignature="Tk9URVM=" sparkle:length="42"\n' ;;
esac
FAKE
chmod +x "$root/tools-2.10.0/bin/sign_update"
export SPARKLE_TOOLS="$root/tools-2.10.0"
export SPARKLE_PRIVATE_KEY="bm90LWEta2V5"

out=$("$script" archive "$root/Tap.dmg" 2>"$root/err") || { echo "archive should succeed: $(cat "$root/err")"; exit 1; }
[ "$out" = "QVJDSElWRQ==" ] || { echo "archive should print the signature alone, got '$out'"; exit 1; }
grep -q -- '--ed-key-file - -p' "$root/tools-2.10.0/bin/args" || { echo "archive should read the key from stdin and print the signature alone"; exit 1; }
grep -q -- '--verify' "$root/tools-2.10.0/bin/args" || { echo "archive should verify what it signed"; exit 1; }
[ "$(cat "$root/tools-2.10.0/bin/stdin")" = "bm90LWEta2V5" ] || { echo "the key should reach sign_update on stdin"; exit 1; }
if grep -q 'bm90LWEta2V5' "$root/err" "$root/tools-2.10.0/bin/args"; then echo "the key reached stderr or the command line"; exit 1; fi

out=$("$script" notes "$root/Tap.md" 2>"$root/err") || { echo "notes should succeed: $(cat "$root/err")"; exit 1; }
[ "$out" = "Tk9URVM= 42" ] || { echo "notes should print the signature and the length, got '$out'"; exit 1; }

"$script" feed "$root/appcast.xml" >"$root/out" 2>"$root/err" || { echo "feed should succeed: $(cat "$root/err")"; exit 1; }
[ ! -s "$root/out" ] || { echo "feed should print nothing, got '$(cat "$root/out")'"; exit 1; }
grep -q 'sparkle-signatures:' "$root/appcast.xml" || { echo "feed should be signed in place"; exit 1; }

# A sign_update that says it signed the feed but left no signature block
# fails the feed mode.
cat > "$root/tools-2.10.0/bin/sign_update" <<'FAKE'
#!/bin/sh
cat > /dev/null
exit 0
FAKE
printf '<?xml version="1.0"?><rss/>\n' > "$root/silent.xml"
if "$script" feed "$root/silent.xml" >/dev/null 2>"$root/err"; then echo "a feed left without its block should fail"; exit 1; fi
grep -q 'carries no signature block' "$root/err" || { echo "the feed failure should say why: $(cat "$root/err")"; exit 1; }

# A sign_update that fails fails the script, without the key in the output.
cat > "$root/tools-2.10.0/bin/sign_update" <<'FAKE'
#!/bin/sh
cat > /dev/null
echo "sign_update: bad key" >&2
exit 1
FAKE
# It says so on stderr in every mode, even when sign_update itself is silent.
for mode in archive notes feed; do
	if out=$("$script" "$mode" "$root/Tap.dmg" 2>"$root/err"); then echo "$mode: a failing sign_update should fail the script"; exit 1; fi
	grep -Fxq 'sparkle-sign.sh: sign_update failed for Tap.dmg' "$root/err" || { echo "$mode: a failed signature should say so: $(cat "$root/err")"; exit 1; }
	if grep -q 'bm90LWEta2V5' "$root/err"; then echo "$mode: the key reached stderr"; exit 1; fi
done
cat > "$root/tools-2.10.0/bin/sign_update" <<'FAKE'
#!/bin/sh
cat > /dev/null
exit 1
FAKE
for mode in archive notes feed; do
	if "$script" "$mode" "$root/Tap.dmg" >/dev/null 2>"$root/err"; then echo "$mode: a silent failing sign_update should fail the script"; exit 1; fi
	grep -Fxq 'sparkle-sign.sh: sign_update failed for Tap.dmg' "$root/err" || { echo "$mode: a silent failure should still be named: $(cat "$root/err")"; exit 1; }
done
# A verification that fails after a signature is printed fails the archive.
cat > "$root/tools-2.10.0/bin/sign_update" <<'FAKE'
#!/bin/sh
cat > /dev/null
case "$*" in *--verify*) exit 1 ;; esac
echo "QVJDSElWRQ=="
FAKE
if "$script" archive "$root/Tap.dmg" >"$root/out" 2>"$root/err"; then echo "a signature that does not verify should fail the archive"; exit 1; fi
[ ! -s "$root/out" ] || { echo "an unverified signature should not be printed"; exit 1; }
grep -Fxq 'sparkle-sign.sh: sign_update failed for Tap.dmg' "$root/err" || { echo "a failed verification should be named: $(cat "$root/err")"; exit 1; }

echo "sparkle-sign.sh is right"

#!/bin/sh
# Checks release.sh against a bundle of its own. With no secret at all: the
# DMG under its -unnotarized name, its checksum, the unsigned appcast under
# its own name, the cask, the summary and the state file are written; every
# secret-bearing step is skipped by name; nothing was notarized, signed for
# Sparkle or pushed; the state says so (the dry run, the same path CI takes
# without secrets). Then, with a stand-in identity and shims: the
# certificate alone, every secret, and a rejected notarization.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
script="$here/release.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
export TAP_SIGNING_KEYCHAIN_FILE="$root/test.keychain-db"
export TAP_ENTITLEMENTS="$root/Test.entitlements"
cat > "$TAP_ENTITLEMENTS" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>com.apple.security.device.audio-input</key><true/>
</dict></plist>
PLIST

app="$root/Tap.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
printf 'int main(void) { return 0; }\n' | cc -arch arm64 -x c -o "$app/Contents/MacOS/Tap" -
printf '#include <stdio.h>\nint main(void) { puts("tap version 0.0.0-test"); return 0; }\n' | cc -arch arm64 -x c -o "$app/Contents/Resources/tap" -
cat > "$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>Tap</string>
<key>CFBundleIdentifier</key><string>io.geocod.tap.releasetest</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>0.0.0-test</string>
<key>CFBundleVersion</key><string>1</string>
<key>LSMinimumSystemVersion</key><string>14.0</string>
<key>NSMicrophoneUsageDescription</key><string>Tap records your voice with the screen when you record a talk.</string>
<key>SUFeedURL</key><string>https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml</string>
<key>SUPublicEDKey</key><string>Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=</string>
<key>SURequireSignedFeed</key><true/>
<key>SUVerifyUpdateBeforeExtraction</key><true/>
</dict></plist>
PLIST

env -u APPLE_DEVELOPER_ID_APPLICATION_P12 -u APPLE_NOTARY_KEY -u SPARKLE_PRIVATE_KEY -u HOMEBREW_TAP_TOKEN \
	"$script" 0.0.0-test "$app" "$root/out" > "$root/log" 2>&1 || { echo "the dry run should succeed"; cat "$root/log"; exit 1; }

for file in Tap-0.0.0-test-unnotarized.dmg Tap-0.0.0-test-unnotarized.dmg.sha256 appcast-unsigned.xml Casks/tap-desktop.rb release-summary.md release-state.env; do
	[ -f "$root/out/$file" ] || { echo "missing $file"; cat "$root/log"; exit 1; }
done
for absent in Tap-0.0.0-test.dmg appcast.xml Tap-0.0.0-test.md; do
	[ ! -e "$root/out/$absent" ] || { echo "$absent must not exist in a dry run"; exit 1; }
done
grep -Fq "Tap-0.0.0-test-unnotarized.dmg" "$root/out/Tap-0.0.0-test-unnotarized.dmg.sha256" || { echo "the checksum names the DMG"; exit 1; }
[ "$(cut -d ' ' -f 1 "$root/out/Tap-0.0.0-test-unnotarized.dmg.sha256")" = "$(shasum -a 256 "$root/out/Tap-0.0.0-test-unnotarized.dmg" | cut -d ' ' -f 1)" ] || { echo "the checksum is wrong"; exit 1; }
grep -Fq 'unsigned: SPARKLE_PRIVATE_KEY was not set' "$root/out/appcast-unsigned.xml" || { echo "the appcast should say it is unsigned"; exit 1; }
grep -Fq 'version "0.0.0-test"' "$root/out/Casks/tap-desktop.rb" || { echo "the cask was not rendered"; exit 1; }
codesign -dv "$app" 2>&1 | grep -q 'Signature=adhoc' || { echo "the app should be signed ad-hoc"; exit 1; }

for line in \
	'skipped: Developer ID signing (APPLE_DEVELOPER_ID_APPLICATION_P12 is not set)' \
	'skipped: notarization of Tap-0.0.0-test.zip (no Developer ID identity)' \
	'skipped: DMG signature (no Developer ID identity)' \
	'skipped: notarization of Tap-0.0.0-test-unnotarized.dmg (no Developer ID identity)' \
	'skipped: release notes (CHANGELOG.md has no section for 0.0.0-test)' \
	'skipped: Sparkle signature of Tap-0.0.0-test-unnotarized.dmg (SPARKLE_PRIVATE_KEY is not set)' \
	'skipped: Gatekeeper assessment (not notarized)'; do
	grep -Fq "$line" "$root/out/release-summary.md" || { echo "the summary lacks: $line"; cat "$root/out/release-summary.md"; exit 1; }
done
[ "$(grep -c '^- skipped:' "$root/out/release-summary.md")" = "7" ] || { echo "seven skipped lines, got $(grep -c '^- skipped:' "$root/out/release-summary.md")"; exit 1; }
for line in \
	'done: signed Tap.app ad-hoc' \
	'done: wrote Tap-0.0.0-test-unnotarized.dmg' \
	'done: wrote appcast-unsigned.xml (unsigned; never uploaded)' \
	'done: rendered Casks/tap-desktop.rb (not pushed by this script)' \
	'done: verified the app, the DMG and the appcast'; do
	grep -Fq "$line" "$root/out/release-summary.md" || { echo "the summary lacks: $line"; cat "$root/out/release-summary.md"; exit 1; }
done
for pair in 'version=0.0.0-test' 'notarized=no' 'feed_signed=no' 'dmg=Tap-0.0.0-test-unnotarized.dmg' 'checksum=Tap-0.0.0-test-unnotarized.dmg.sha256' 'notes=' 'appcast=appcast-unsigned.xml' 'cask=Casks/tap-desktop.rb'; do
	grep -Fxq "$pair" "$root/out/release-state.env" || { echo "the state lacks: $pair"; cat "$root/out/release-state.env"; exit 1; }
done
[ ! -f "$TAP_SIGNING_KEYCHAIN_FILE" ] || { echo "the keychain should be gone"; exit 1; }

# A second run replaces the output folder's files rather than failing on them.
env -u APPLE_DEVELOPER_ID_APPLICATION_P12 -u APPLE_NOTARY_KEY -u SPARKLE_PRIVATE_KEY -u HOMEBREW_TAP_TOKEN \
	"$script" 0.0.0-test "$app" "$root/out" >/dev/null 2>&1 || { echo "a second run should succeed"; exit 1; }

# The version must match the app's own.
if "$script" 9.9.9 "$app" "$root/out2" >/dev/null 2>&1; then echo "a version the app does not carry should fail"; exit 1; fi

# With a stand-in identity: codesign turns the named identity into ad-hoc
# and drops the timestamp, xcrun answers notarytool and stapler, spctl
# accepts, and a stand-in sign_update signs. Three runs: the certificate
# alone (the state right after enrolment), every secret, and every secret
# with the notary service rejecting the submission.
mkdir -p "$root/shims" "$root/tools-2.10.0/bin"
cat > "$root/shims/codesign" <<'SHIM'
#!/bin/sh
next_is_identity=no
for argument; do
	if [ "$next_is_identity" = yes ]; then set -- "$@" -; next_is_identity=no; continue; fi
	case "$argument" in
		--sign) set -- "$@" --sign; next_is_identity=yes ;;
		--timestamp) set -- "$@" --timestamp=none ;;
		*) set -- "$@" "$argument" ;;
	esac
	shift
done
exec /usr/bin/codesign "$@"
SHIM
cat > "$root/shims/xcrun" <<SHIM
#!/bin/sh
case "\$1 \$2" in
	"notarytool submit") printf '{"status":"%s","id":"sub-1"}\n' "\$(cat "$root/notary-status")" ;;
	"notarytool log") echo "log for sub-1" ;;
	"stapler staple"|"stapler validate") echo "\$3: stapled (stand-in)" ;;
	*) exec /usr/bin/xcrun "\$@" ;;
esac
SHIM
printf '#!/bin/sh\necho "accepted (stand-in)"\n' > "$root/shims/spctl"
cat > "$root/tools-2.10.0/bin/sign_update" <<'FAKE'
#!/bin/sh
cat > /dev/null
for file; do :; done
case "$*" in
	*--verify*) exit 0 ;;
	*" -p "*) echo "QVJDSElWRQ==" ;;
	*.xml) printf '<!-- sparkle-signatures:\nedSignature: RkVFRA==\nlength: 1\n-->' >> "$file" ;;
	*) printf 'sparkle:edSignature="Tk9URVM=" sparkle:length="42"\n' ;;
esac
FAKE
chmod +x "$root/shims"/* "$root/tools-2.10.0/bin/sign_update"
# fetch-sparkle-tools.sh trusts a cached tool only while its recorded
# sha256 still matches, so the stand-in needs its own record; without one
# it would try to download the real thing.
shasum -a 256 "$root/tools-2.10.0/bin/sign_update" | cut -d ' ' -f 1 > "$root/tools-2.10.0/bin/sign_update.sha256"
export TAP_RELEASE_IDENTITY="Developer ID Application: Test Person (TEAM123456)"
export SPARKLE_TOOLS="$root/tools-2.10.0"
sources() { bash -eo pipefail -c "source '$1/release-state.env' && printf '%s %s %s\n' \"\$notarized\" \"\$feed_signed\" \"\$dmg\""; }

# A: the certificate alone.
PATH="$root/shims:$PATH" env -u APPLE_NOTARY_KEY -u APPLE_NOTARY_KEY_ID -u APPLE_NOTARY_ISSUER_ID -u SPARKLE_PRIVATE_KEY \
	"$script" 0.0.0-test "$app" "$root/a" > "$root/log" 2>&1 || { echo "the certificate-only run should succeed"; cat "$root/log"; exit 1; }
grep -Fq 'done: using the identity Developer ID Application: Test Person (TEAM123456)' "$root/a/release-summary.md" || { echo "A: the identity line"; exit 1; }
grep -Fq 'skipped: notarization of Tap-0.0.0-test.zip (APPLE_NOTARY_KEY is not set)' "$root/a/release-summary.md" || { echo "A: the app's notarization skip line names the secret"; cat "$root/a/release-summary.md"; exit 1; }
grep -Fq 'skipped: notarization of Tap-0.0.0-test-unnotarized.dmg (APPLE_NOTARY_KEY is not set)' "$root/a/release-summary.md" || { echo "A: the DMG's notarization skip line names the secret"; exit 1; }
grep -Fq 'done: signed Tap-0.0.0-test-unnotarized.dmg' "$root/a/release-summary.md" || { echo "A: the DMG is signed"; exit 1; }
if grep -q '^- $' "$root/a/release-summary.md"; then echo "A: an empty summary line"; exit 1; fi
[ "$(sources "$root/a")" = "no no Tap-0.0.0-test-unnotarized.dmg" ] || { echo "A: the state does not source: $(sources "$root/a" 2>&1)"; exit 1; }
if grep -q 'identity=' "$root/a/release-state.env"; then echo "the identity stays out of the state file"; exit 1; fi

# B: every secret, the notary service accepting.
echo Accepted > "$root/notary-status"
PATH="$root/shims:$PATH" APPLE_NOTARY_KEY=key APPLE_NOTARY_KEY_ID=id APPLE_NOTARY_ISSUER_ID=issuer SPARKLE_PRIVATE_KEY=bm90LWEta2V5 \
	"$script" 0.0.0-test "$app" "$root/b" > "$root/log" 2>&1 || { echo "the all-secrets run should succeed"; cat "$root/log"; exit 1; }
for file in Tap-0.0.0-test.dmg Tap-0.0.0-test.dmg.sha256 appcast.xml Casks/tap-desktop.rb; do
	[ -f "$root/b/$file" ] || { echo "B: missing $file"; cat "$root/log"; exit 1; }
done
[ ! -e "$root/b/appcast-unsigned.xml" ] || { echo "B: no unsigned appcast"; exit 1; }
grep -Fq 'notarized Tap-0.0.0-test.zip (submission sub-1) and stapled Tap.app' "$root/b/release-summary.md" || { echo "B: the app's notarization line"; exit 1; }
grep -Fq 'notarized Tap-0.0.0-test.dmg (submission sub-1) and stapled Tap-0.0.0-test.dmg' "$root/b/release-summary.md" || { echo "B: the DMG's notarization line"; exit 1; }
grep -Fq 'done: wrote appcast.xml (signed)' "$root/b/release-summary.md" || { echo "B: the signed appcast line"; exit 1; }
grep -q 'sparkle-signatures:' "$root/b/appcast.xml" || { echo "B: the feed is signed"; exit 1; }
[ "$(sources "$root/b")" = "yes yes Tap-0.0.0-test.dmg" ] || { echo "B: the state does not source: $(sources "$root/b" 2>&1)"; exit 1; }

# C: every secret, the notary service rejecting: the release stops, and
# nothing says notarized.
echo Invalid > "$root/notary-status"
if PATH="$root/shims:$PATH" APPLE_NOTARY_KEY=key APPLE_NOTARY_KEY_ID=id APPLE_NOTARY_ISSUER_ID=issuer SPARKLE_PRIVATE_KEY=bm90LWEta2V5 \
	"$script" 0.0.0-test "$app" "$root/c" > "$root/log" 2>&1; then echo "a rejected notarization should fail the release"; exit 1; fi
grep -q "was not accepted (status: Invalid" "$root/log" || { echo "C: the rejection is reported: $(cat "$root/log")"; exit 1; }
grep -q "nothing is published" "$root/log" || { echo "C: release.sh says it stopped"; exit 1; }
[ ! -e "$root/c/release-state.env" ] || { echo "C: no state file after a failure"; exit 1; }
[ ! -e "$root/c/appcast.xml" ] || { echo "C: no appcast after a failure"; exit 1; }
if grep -q 'notarized' "$root/c/release-summary.md" 2>/dev/null; then echo "C: the summary must not claim notarization"; exit 1; fi
unset TAP_RELEASE_IDENTITY SPARKLE_TOOLS

echo "release.sh is right"

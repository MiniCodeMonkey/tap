#!/bin/sh
# The checks on a finished release, by its state: the app is arm64 alone
# and so is its tap, which prints the release's version; the plist carries
# the Sparkle keys and the microphone string; the app carries the
# microphone entitlement and the hardened runtime and verifies; no test
# framework or bundle rode along; the DMG verifies; the appcast parses and
# names the same build and length; a notarized release also passes
# Gatekeeper and has its tickets; a signed feed has its signature block.
set -eu

app="${1:-}"; dmg="${2:-}"; appcast="${3:-}"; identity="${4:--}"; version="${5:-}"; notarized="${6:-no}"; feed_signed="${7:-no}"
plist="$app/Contents/Info.plist"
fail() { echo "verify-release.sh: $1" >&2; exit 1; }
[ -f "$plist" ] && [ -f "$dmg" ] && [ -f "$appcast" ] && [ -n "$version" ] || fail "usage: verify-release.sh <app> <dmg> <appcast> <identity> <version> <notarized> <feed_signed>"
# Sets value to a plist key, or fails the script naming the key. It runs in
# the script's own shell, never inside $(...), so its fail ends the script
# with one line.
read_plist() { value=$(/usr/libexec/PlistBuddy -c "Print :$1" "$plist" 2>/dev/null) || fail "$1 is missing from the plist"; }

for binary in "$app/Contents/MacOS/Tap" "$app/Contents/Resources/tap"; do
	[ -f "$binary" ] || fail "$binary is missing"
	archs=$(lipo -archs "$binary")
	[ "$archs" = "arm64" ] || fail "$binary is not arm64 alone: $archs"
done
printed=$("$app/Contents/Resources/tap" --version)
[ "$printed" = "tap version $version" ] || fail "the bundled tap prints '$printed', not 'tap version $version'"

read_plist CFBundleShortVersionString; [ "$value" = "$version" ] || fail "the app is $value, not $version"
read_plist CFBundleVersion; build_number=$value
[ -n "$build_number" ] || fail "no CFBundleVersion"
read_plist LSMinimumSystemVersion; [ "$value" = "14.0" ] || fail "LSMinimumSystemVersion is not 14.0"
read_plist SUFeedURL; [ "$value" = "https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml" ] || fail "SUFeedURL is wrong"
read_plist SUPublicEDKey; [ "$value" = "Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=" ] || fail "SUPublicEDKey is wrong"
read_plist SURequireSignedFeed; [ "$value" = "true" ] || fail "SURequireSignedFeed is not true"
read_plist SUVerifyUpdateBeforeExtraction; [ "$value" = "true" ] || fail "SUVerifyUpdateBeforeExtraction is not true"
read_plist NSMicrophoneUsageDescription; [ -n "$value" ] || fail "NSMicrophoneUsageDescription is empty"

for stray in "$app/Contents/PlugIns" "$app"/Contents/Frameworks/XCTest*.framework "$app"/Contents/Frameworks/libXCTest*; do
	[ -e "$stray" ] && fail "test code in the product: $stray"
done
codesign --verify --deep --strict "$app" || fail "$app does not verify"
codesign -dv "$app" 2>&1 | grep -q 'runtime' || fail "$app has no hardened runtime"
codesign -d --entitlements - "$app" 2>&1 | grep -q 'com.apple.security.device.audio-input' || fail "$app lacks the audio-input entitlement"

hdiutil verify -quiet "$dmg" || fail "$dmg does not verify"
xmllint --noout "$appcast" || fail "$appcast is not XML"
grep -Fq "<sparkle:version>$build_number</sparkle:version>" "$appcast" || fail "the appcast names another build than $build_number"
grep -Fq "length=\"$(stat -f%z "$dmg")\"" "$appcast" || fail "the appcast's length is not the DMG's"
grep -Fq "<sparkle:hardwareRequirements>arm64</sparkle:hardwareRequirements>" "$appcast" || fail "the appcast does not require arm64"

if [ "$identity" != "-" ]; then
	codesign --verify --strict "$dmg" || fail "$dmg is not signed"
fi
if [ "$notarized" = "yes" ]; then
	[ "$identity" != "-" ] || fail "notarized without an identity"
	spctl --assess --type open --context context:primary-signature -v "$dmg" || fail "Gatekeeper refuses $dmg"
	xcrun stapler validate "$app" >/dev/null || fail "$app has no stapled ticket"
	xcrun stapler validate "$dmg" >/dev/null || fail "$dmg has no stapled ticket"
fi
if [ "$feed_signed" = "yes" ]; then
	grep -q 'sparkle-signatures:' "$appcast" || fail "the feed is claimed signed but has no signature block"
	grep -q 'sparkle:edSignature=' "$appcast" || fail "the feed is claimed signed but its enclosure has no signature"
fi
echo "verified $app, $dmg and $appcast"

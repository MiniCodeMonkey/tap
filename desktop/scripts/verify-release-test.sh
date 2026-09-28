#!/bin/sh
# Checks verify-release.sh against a release built from a bundle of its
# own: arm64 binaries built with cc, a tap that prints its version, the plist keys, the
# entitlements, no test code. Ad-hoc and not notarized passes with the
# Gatekeeper checks skipped; a real identity that is signed but not
# notarized passes too (a missing notary key must not fail the run); a
# release that claims notarization without a ticket fails; a universal
# binary, a wrong tap version, a missing key, a stray XCTest framework and
# an unsigned "signed" feed each fail by name.
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
script="$here/verify-release.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT

make_app() {
	app="$1"; tap_version="$2"
	rm -rf "$app"
	mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
	printf 'int main(void) { return 0; }\n' | cc -arch arm64 -x c -o "$app/Contents/MacOS/Tap" -
	printf '#include <stdio.h>\nint main(void) { puts("tap version %s"); return 0; }\n' "$tap_version" | cc -arch arm64 -x c -o "$app/Contents/Resources/tap" -
	cat > "$app/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>Tap</string>
<key>CFBundleIdentifier</key><string>io.geocod.tap.verifytest</string>
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
}
entitlements="$root/Test.entitlements"
cat > "$entitlements" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>com.apple.security.device.audio-input</key><true/>
</dict></plist>
PLIST
download="https://github.com/MiniCodeMonkey/tap/releases/download/v0.0.0-test/Tap-0.0.0-test.dmg"
release="https://github.com/MiniCodeMonkey/tap/releases/tag/v0.0.0-test"

app="$root/Tap.app"
make_app "$app" 0.0.0-test
"$here/sign-app.sh" "$app" - "$entitlements" >/dev/null
dmg="$root/Tap-0.0.0-test.dmg"
"$here/make-dmg.sh" "$app" "$dmg" Tap >/dev/null
# The DMG is signed before the appcast reads its length, as release.sh does.
codesign --force --sign - "$dmg"
"$here/write-appcast.sh" "$app" "$dmg" "$download" "$release" "$root/appcast.xml" >/dev/null

# Ad-hoc, not notarized, unsigned feed: the dry run.
"$script" "$app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no >/dev/null || { echo "the dry run should verify"; exit 1; }
# A real identity without notarization: the DMG is signed, no ticket is asked for.
"$script" "$app" "$dmg" "$root/appcast.xml" "Developer ID Application: Someone (TEAM)" 0.0.0-test no no >/dev/null || { echo "signed but not notarized should verify"; exit 1; }
# A claim of notarization without a ticket fails.
if "$script" "$app" "$dmg" "$root/appcast.xml" "Developer ID Application: Someone (TEAM)" 0.0.0-test yes no >/dev/null 2>&1; then echo "notarized without a ticket should fail"; exit 1; fi
# A claim of a signed feed without the signature block fails.
if "$script" "$app" "$dmg" "$root/appcast.xml" - 0.0.0-test no yes >/dev/null 2>&1; then echo "a feed claimed signed without its block should fail"; exit 1; fi
# The version must be the app's.
if "$script" "$app" "$dmg" "$root/appcast.xml" - 9.9.9 no no >/dev/null 2>&1; then echo "another version should fail"; exit 1; fi

fails_with() {
	message="$1"; shift
	if "$script" "$@" >"$root/out" 2>&1; then echo "should fail: $message"; exit 1; fi
	grep -Fq "$message" "$root/out" || { echo "wrong failure for '$message': $(cat "$root/out")"; exit 1; }
}
# A tap of another version.
make_app "$root/Wrong.app" 0.0.0-other; "$here/sign-app.sh" "$root/Wrong.app" - "$entitlements" >/dev/null
fails_with "tap version 0.0.0-other" "$root/Wrong.app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no
# An app binary with more than the arm64 slice (/usr/bin/true is fat).
make_app "$root/Universal.app" 0.0.0-test; cp /usr/bin/true "$root/Universal.app/Contents/MacOS/Tap"; "$here/sign-app.sh" "$root/Universal.app" - "$entitlements" >/dev/null
fails_with "is not arm64 alone" "$root/Universal.app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no
# A missing plist key.
make_app "$root/NoKey.app" 0.0.0-test; /usr/libexec/PlistBuddy -c 'Delete :SURequireSignedFeed' "$root/NoKey.app/Contents/Info.plist"; "$here/sign-app.sh" "$root/NoKey.app" - "$entitlements" >/dev/null
fails_with "SURequireSignedFeed" "$root/NoKey.app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no
# No entitlement.
make_app "$root/NoEntitlement.app" 0.0.0-test; printf '<?xml version="1.0" encoding="UTF-8"?>\n<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">\n<plist version="1.0"><dict/></plist>\n' > "$root/Empty.entitlements"; "$here/sign-app.sh" "$root/NoEntitlement.app" - "$root/Empty.entitlements" >/dev/null
fails_with "audio-input" "$root/NoEntitlement.app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no
# Test code in the product.
make_app "$root/Tested.app" 0.0.0-test; mkdir -p "$root/Tested.app/Contents/Frameworks/XCTest.framework" "$root/Tested.app/Contents/PlugIns/TapTests.xctest"; "$here/sign-app.sh" "$root/Tested.app" - "$entitlements" >/dev/null 2>&1 || true
fails_with "test code" "$root/Tested.app" "$dmg" "$root/appcast.xml" - 0.0.0-test no no

echo "verify-release.sh is right"

#!/bin/sh
# Checks sign-app.sh on a bundle of its own: real executables (copies of
# /usr/bin/true), a Sparkle framework laid out as the real one is, signed
# ad-hoc with the hardened runtime in Sparkle's documented order, the app
# with its entitlements, verified, and signed again without complaint. A
# codesign shim on PATH records the order the real codesign was called in.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/sign-app.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT

plist() {
	cat > "$1" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>$2</string>
<key>CFBundleIdentifier</key><string>io.geocod.tap.signtest.$2</string>
<key>CFBundlePackageType</key><string>$3</string>
<key>CFBundleShortVersionString</key><string>0.0.0</string>
<key>CFBundleVersion</key><string>1</string>
</dict></plist>
PLIST
}

app="$root/Fake.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp /usr/bin/true "$app/Contents/MacOS/Fake"
cp /usr/bin/true "$app/Contents/Resources/tap"
plist "$app/Contents/Info.plist" Fake APPL
framework="$app/Contents/Frameworks/Sparkle.framework"
versions="$framework/Versions/B"
mkdir -p "$versions/Resources" "$versions/XPCServices/Installer.xpc/Contents/MacOS" "$versions/XPCServices/Downloader.xpc/Contents/MacOS" "$versions/Updater.app/Contents/MacOS"
for executable in "$versions/Sparkle" "$versions/Autoupdate" "$versions/XPCServices/Installer.xpc/Contents/MacOS/Installer" "$versions/XPCServices/Downloader.xpc/Contents/MacOS/Downloader" "$versions/Updater.app/Contents/MacOS/Updater"; do
	cp /usr/bin/true "$executable"
done
plist "$versions/Resources/Info.plist" Sparkle FMWK
plist "$versions/XPCServices/Installer.xpc/Contents/Info.plist" Installer XPC!
plist "$versions/XPCServices/Downloader.xpc/Contents/Info.plist" Downloader XPC!
plist "$versions/Updater.app/Contents/Info.plist" Updater APPL
ln -s B "$framework/Versions/Current"
ln -s Versions/Current/Sparkle "$framework/Sparkle"
ln -s Versions/Current/Resources "$framework/Resources"

entitlements="$root/Test.entitlements"
cat > "$entitlements" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>com.apple.security.device.audio-input</key><true/>
</dict></plist>
PLIST

# The shim: every codesign call's arguments, one per line, then the real tool.
mkdir -p "$root/bin"
cat > "$root/bin/codesign" <<SHIM
#!/bin/sh
printf '%s\n' "\$*" >> "$root/codesign.log"
exec /usr/bin/codesign "\$@"
SHIM
chmod +x "$root/bin/codesign"

PATH="$root/bin:$PATH" "$script" "$app" - "$entitlements" >/dev/null || { echo "ad-hoc signing should succeed"; exit 1; }
for binary in "$app" "$app/Contents/Resources/tap" "$framework" "$versions/Autoupdate" "$versions/Updater.app" "$versions/XPCServices/Installer.xpc" "$versions/XPCServices/Downloader.xpc"; do
	info=$(codesign -dv "$binary" 2>&1)
	echo "$info" | grep -q 'Signature=adhoc' || { echo "$binary: not ad-hoc: $info"; exit 1; }
	echo "$info" | grep -q 'runtime' || { echo "$binary: no hardened runtime: $info"; exit 1; }
done
codesign --verify --deep --strict "$app" || { echo "the bundle should verify"; exit 1; }
codesign -d --entitlements - "$app" 2>&1 | grep -q 'com.apple.security.device.audio-input' || { echo "the app should carry the microphone entitlement"; exit 1; }
if codesign -d --entitlements - "$versions/Autoupdate" 2>&1 | grep -q 'audio-input'; then echo "nested code must carry no entitlement of the app's"; exit 1; fi

# Sparkle's documented order: tap, Installer.xpc, Downloader.xpc (keeping its
# entitlements), Autoupdate, Updater.app, the framework, then the app, then
# one verify.
order=$(grep -v -- '--verify' "$root/codesign.log" | sed -e 's/.*--timestamp=none //' -e 's/.*--entitlements [^ ]* //')
expected="$app/Contents/Resources/tap
$versions/XPCServices/Installer.xpc
--preserve-metadata=entitlements $versions/XPCServices/Downloader.xpc
$versions/Autoupdate
$versions/Updater.app
$framework
$app"
[ "$order" = "$expected" ] || { echo "the signing order is wrong:"; echo "$order"; exit 1; }
grep -q -- "--entitlements $entitlements $app\$" "$root/codesign.log" || { echo "the app should be signed with the entitlements"; exit 1; }
[ "$(grep -c -- '--verify --deep --strict' "$root/codesign.log")" = "1" ] || { echo "one verify at the end"; exit 1; }
if grep -q -- '--deep --strict.*--sign\|--sign.*--deep' "$root/codesign.log"; then echo "never --deep when signing"; exit 1; fi

# Signing an already signed bundle replaces the signature.
"$script" "$app" - "$entitlements" >/dev/null || { echo "a second run should succeed"; exit 1; }

# An identity that does not exist fails with codesign's own message.
if "$script" "$app" "Developer ID Application: Nobody (NOTEAM00)" "$entitlements" >/dev/null 2>&1; then
	echo "an unknown identity should fail"; exit 1
fi

# A path that is not an app, or a missing entitlements file, fails before anything is signed.
if "$script" "$root/missing.app" - "$entitlements" >/dev/null 2>&1; then echo "a missing app should fail"; exit 1; fi
if "$script" "$app" - "$root/missing.entitlements" >/dev/null 2>&1; then echo "missing entitlements should fail"; exit 1; fi

echo "sign-app.sh is right"

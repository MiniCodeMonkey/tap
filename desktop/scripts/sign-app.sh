#!/bin/sh
# Signs an app bundle inside out with the hardened runtime: the bundled tap,
# then Sparkle's own executables and the framework when the app carries
# them, then the app with its entitlements (the microphone, for recorded
# talks); nested code gets none. Ad-hoc ("-") unless an identity is given;
# a real identity also gets a secure timestamp, which notarization
# requires. Never --deep: Apple and Sparkle both say so, since it signs
# nested code with the outer code's entitlements and in the wrong order.
set -eu

app="${1:-}"
identity="${2:--}"
here="$(cd "$(dirname "$0")" && pwd)"
entitlements="${3:-$here/../Tap/Tap.entitlements}"
[ -d "$app" ] && [ -f "$app/Contents/Info.plist" ] || { echo "sign-app.sh: $app is not an app bundle" >&2; exit 1; }
[ -f "$entitlements" ] || { echo "sign-app.sh: $entitlements is missing" >&2; exit 1; }

sign() {
	if [ "$identity" = "-" ]; then
		codesign --force --sign - --options runtime --timestamp=none "$@"
	else
		codesign --force --sign "$identity" --options runtime --timestamp "$@"
	fi
}

[ -f "$app/Contents/Resources/tap" ] && sign "$app/Contents/Resources/tap"

sparkle="$app/Contents/Frameworks/Sparkle.framework"
if [ -d "$sparkle" ]; then
	versions="$sparkle/Versions/B"
	sign "$versions/XPCServices/Installer.xpc"
	sign --preserve-metadata=entitlements "$versions/XPCServices/Downloader.xpc"
	sign "$versions/Autoupdate"
	sign "$versions/Updater.app"
	sign "$sparkle"
fi

for framework in "$app"/Contents/Frameworks/*.framework; do
	[ -d "$framework" ] || continue
	[ "$framework" = "$sparkle" ] && continue
	sign "$framework"
done

sign --entitlements "$entitlements" "$app"
codesign --verify --deep --strict --verbose=1 "$app"
if [ "$identity" = "-" ]; then
	echo "signed $app ad-hoc"
else
	echo "signed $app with $identity"
fi

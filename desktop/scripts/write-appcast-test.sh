#!/bin/sh
# Checks write-appcast.sh: one item from the app's own plist, the DMG's
# real length, the arm64 requirement, the notes link with its signature,
# the DMG's signature when there is one and a plain word when there is
# none, and XML that parses.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/write-appcast.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT

app="$root/Tap.app"
mkdir -p "$app/Contents"
cat > "$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleShortVersionString</key><string>2.1.0-beta.3</string>
<key>CFBundleVersion</key><string>2010022</string>
<key>LSMinimumSystemVersion</key><string>14.0</string>
</dict></plist>
PLIST
dmg="$root/Tap-2.1.0-beta.3.dmg"
head -c 12345 /dev/zero > "$dmg"
download="https://github.com/MiniCodeMonkey/tap/releases/download/v2.1.0-beta.3/Tap-2.1.0-beta.3.dmg"
release="https://github.com/MiniCodeMonkey/tap/releases/tag/v2.1.0-beta.3"
notes="https://github.com/MiniCodeMonkey/tap/releases/download/v2.1.0-beta.3/Tap-2.1.0-beta.3.md"

"$script" "$app" "$dmg" "$download" "$release" "$root/appcast.xml" "c2lnbmF0dXJl" "$notes" "bm90ZXM=" 255 >/dev/null || { echo "the appcast should be written"; exit 1; }
xmllint --noout "$root/appcast.xml" || { echo "the appcast should be XML"; exit 1; }
for expected in \
	'<sparkle:version>2010022</sparkle:version>' \
	'<sparkle:shortVersionString>2.1.0-beta.3</sparkle:shortVersionString>' \
	'<sparkle:minimumSystemVersion>14.0</sparkle:minimumSystemVersion>' \
	'<sparkle:hardwareRequirements>arm64</sparkle:hardwareRequirements>' \
	"<sparkle:releaseNotesLink sparkle:edSignature=\"bm90ZXM=\" sparkle:length=\"255\">$notes</sparkle:releaseNotesLink>" \
	"<link>$release</link>" \
	"url=\"$download\"" \
	'length="12345"' \
	'sparkle:edSignature="c2lnbmF0dXJl"' \
	'type="application/octet-stream"' \
	'<title>Tap 2.1.0-beta.3</title>' \
	'xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle"'; do
	grep -Fq "$expected" "$root/appcast.xml" || { echo "missing: $expected"; cat "$root/appcast.xml"; exit 1; }
done
[ "$(grep -c '<item>' "$root/appcast.xml")" = "1" ] || { echo "one item"; exit 1; }
grep -Eq '<pubDate>[A-Z][a-z]{2}, [0-9]{2} [A-Z][a-z]{2} [0-9]{4} [0-9]{2}:[0-9]{2}:[0-9]{2} \+0000</pubDate>' "$root/appcast.xml" || { echo "no RFC 822 pubDate"; exit 1; }
if grep -q 'unsigned' "$root/appcast.xml"; then echo "a signed appcast carries no unsigned note"; exit 1; fi

# Without a DMG signature the item is marked unsigned, and the attribute is absent.
"$script" "$app" "$dmg" "$download" "$release" "$root/unsigned.xml" >/dev/null
xmllint --noout "$root/unsigned.xml"
if grep -q 'edSignature' "$root/unsigned.xml"; then echo "an unsigned appcast must carry no signature attribute"; exit 1; fi
grep -Fq 'unsigned: SPARKLE_PRIVATE_KEY was not set' "$root/unsigned.xml" || { echo "the unsigned appcast should say so"; exit 1; }
if grep -q 'releaseNotesLink' "$root/unsigned.xml"; then echo "no notes url, no notes link"; exit 1; fi

# Notes without a signature are a link without attributes (a dry run with notes).
"$script" "$app" "$dmg" "$download" "$release" "$root/plain-notes.xml" "" "$notes" >/dev/null
grep -Fq "<sparkle:releaseNotesLink>$notes</sparkle:releaseNotesLink>" "$root/plain-notes.xml" || { echo "unsigned notes are a plain link"; exit 1; }

# A character XML must escape never reaches the feed; the writer refuses rather than corrupting it.
if "$script" "$app" "$dmg" 'https://example.com/a"b.dmg' "$release" "$root/bad.xml" >/dev/null 2>&1; then echo "a quote in a URL should fail"; exit 1; fi
if "$script" "$root/none.app" "$dmg" "$download" "$release" "$root/x.xml" >/dev/null 2>&1; then echo "a missing app should fail"; exit 1; fi
if "$script" "$app" "$dmg" "$download" "$release" "$root/x.xml" "" "$notes" "bm90ZXM=" >/dev/null 2>&1; then echo "a notes signature without a length should fail"; exit 1; fi

echo "write-appcast.sh is right"

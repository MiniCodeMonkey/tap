#!/bin/sh
# Writes the Sparkle appcast for one release: a feed with one item, read
# from the built app's own Info.plist and the DMG on disk, so the feed can
# never name a version the app does not carry. The item requires arm64
# (the release is Apple silicon only) so an Intel Mac is never offered it.
# The signatures come from sparkle-sign.sh: the DMG's on the enclosure, the
# notes' on their link. Without a DMG signature the item says it is
# unsigned; release.sh then names the file appcast-unsigned.xml and the job
# never uploads it, and the app would refuse it anyway (SURequireSignedFeed).
set -eu

app="${1:-}"; dmg="${2:-}"; download_url="${3:-}"; release_url="${4:-}"; output="${5:-}"
dmg_signature="${6:-}"; notes_url="${7:-}"; notes_signature="${8:-}"; notes_length="${9:-}"
plist="$app/Contents/Info.plist"
[ -f "$plist" ] || { echo "write-appcast.sh: $app has no Info.plist" >&2; exit 1; }
[ -f "$dmg" ] || { echo "write-appcast.sh: $dmg is missing" >&2; exit 1; }
[ -n "$output" ] || { echo "write-appcast.sh: no output path" >&2; exit 1; }
if [ -n "$notes_signature" ] && [ -z "$notes_length" ]; then echo "write-appcast.sh: a notes signature needs the notes length" >&2; exit 1; fi
for value in "$download_url" "$release_url" "$dmg_signature" "$notes_url" "$notes_signature" "$notes_length"; do
	case "$value" in
		*[\"\<\>\&]*) echo "write-appcast.sh: '$value' holds a character XML would need escaped" >&2; exit 1 ;;
	esac
done

read_plist() { /usr/libexec/PlistBuddy -c "Print :$1" "$plist"; }
short_version=$(read_plist CFBundleShortVersionString)
build_number=$(read_plist CFBundleVersion)
minimum_system=$(read_plist LSMinimumSystemVersion)
length=$(stat -f%z "$dmg")
# RFC 822 names its days and months in English, whatever the locale.
published=$(LC_ALL=C date -u '+%a, %d %b %Y %H:%M:%S +0000')

if [ -n "$dmg_signature" ]; then
	signature_attribute=" sparkle:edSignature=\"$dmg_signature\""
	unsigned_note=""
else
	signature_attribute=""
	unsigned_note="
    <!-- unsigned: SPARKLE_PRIVATE_KEY was not set when this feed was written. It is never uploaded; the app refuses an unsigned feed. -->"
fi
notes_element=""
if [ -n "$notes_url" ] && [ -n "$notes_signature" ]; then
	notes_element="
      <sparkle:releaseNotesLink sparkle:edSignature=\"$notes_signature\" sparkle:length=\"$notes_length\">$notes_url</sparkle:releaseNotesLink>"
elif [ -n "$notes_url" ]; then
	notes_element="
      <sparkle:releaseNotesLink>$notes_url</sparkle:releaseNotesLink>"
fi

cat > "$output" <<XML
<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">
  <channel>
    <title>Tap Desktop</title>
    <link>https://github.com/MiniCodeMonkey/tap</link>
    <description>Updates for Tap Desktop</description>
    <language>en</language>$unsigned_note
    <item>
      <title>Tap $short_version</title>
      <link>$release_url</link>
      <pubDate>$published</pubDate>
      <sparkle:version>$build_number</sparkle:version>
      <sparkle:shortVersionString>$short_version</sparkle:shortVersionString>
      <sparkle:minimumSystemVersion>$minimum_system</sparkle:minimumSystemVersion>
      <sparkle:hardwareRequirements>arm64</sparkle:hardwareRequirements>$notes_element
      <enclosure url="$download_url" length="$length" type="application/octet-stream"$signature_attribute/>
    </item>
  </channel>
</rss>
XML
xmllint --noout "$output"
echo "wrote $output"

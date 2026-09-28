#!/bin/sh
# Turns a built Release app into a release: signs it, notarizes the app and
# the DMG, writes the DMG with its checksum, the release notes, the
# Sparkle appcast and the cask, verifies everything, and records every
# step in release-summary.md as done or skipped and the outcome in
# release-state.env, which the release job reads to decide what may be
# published. Every step that needs a secret skips itself, by name, when
# the secret is absent, so this runs with none (a dry run) and with all
# of them (the release job) along the same path. A DMG that was not
# notarized is named -unnotarized; an appcast that is not signed is
# appcast-unsigned.xml; neither is ever offered to a person. Nothing here
# prints a secret; the scripts it calls own that rule.
set -eu

version="${1:-}"; app="${2:-}"; out="${3:-}"
[ -n "$version" ] && [ -d "$app" ] && [ -n "$out" ] || { echo "release.sh: usage: release.sh <version> <Tap.app> <output-dir>" >&2; exit 1; }
here="$(cd "$(dirname "$0")" && pwd)"
entitlements="${TAP_ENTITLEMENTS:-$here/../Tap/Tap.entitlements}"
changelog="$here/../../CHANGELOG.md"
prepare_changelog="$here/../../scripts/prepare-changelog.sh"
built=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$app/Contents/Info.plist")
[ "$built" = "$version" ] || { echo "release.sh: the app is $built, not $version; build it with VERSION=$version" >&2; exit 1; }

mkdir -p "$out"
summary="$out/release-summary.md"
state="$out/release-state.env"
: > "$summary"
note() { echo "$1"; echo "- $1" >> "$summary"; }

# The keychain goes whatever happens from here on, even before the import
# has returned. TAP_RELEASE_IDENTITY names an identity already in a
# keychain (a person's own, or a test's stand-in) and skips the import.
trap '"$here/signing-identity.sh" remove; rm -f "$out/identity.log" "$out/sparkle.log"' EXIT
if [ -n "${TAP_RELEASE_IDENTITY:-}" ]; then
	identity="$TAP_RELEASE_IDENTITY"
	note "done: using the identity $identity"
else
	identity=$("$here/signing-identity.sh" import 2>"$out/identity.log") || { cat "$out/identity.log" >&2; exit 1; }
	if [ "$identity" = "-" ]; then
		note "$(cat "$out/identity.log")"
	else
		note "done: found the identity $identity"
	fi
fi

"$here/sign-app.sh" "$app" "$identity" "$entitlements" >/dev/null
if [ "$identity" = "-" ]; then note "done: signed $(basename "$app") ad-hoc"; else note "done: signed $(basename "$app") with $identity"; fi

# Notarization needs an identity and the three notary secrets; a DMG
# without it carries the fact in its name. A submission the notary service
# rejects stops the release here: nothing after this point may pretend.
# The missing-secret phrasing matches notarize.sh's own (one name, two
# joined with "and", or all three collapsed to the first), so a partial
# notary configuration reads the same in both places.
notarized=no
missing_notary=""; missing_notary_count=0
for secret in APPLE_NOTARY_KEY APPLE_NOTARY_KEY_ID APPLE_NOTARY_ISSUER_ID; do
	eval "value=\${$secret:-}"
	if [ -z "$value" ]; then
		missing_notary="${missing_notary:+$missing_notary and }$secret"
		missing_notary_count=$((missing_notary_count + 1))
	fi
done
case "$missing_notary_count" in
	0) missing_notary_secret="" ;;
	1) missing_notary_secret="$missing_notary is not set" ;;
	2) missing_notary_secret="$missing_notary are not set" ;;
	3) missing_notary_secret="APPLE_NOTARY_KEY is not set" ;;
esac
will_notarize=no
if [ "$identity" != "-" ] && [ "$missing_notary_count" = 0 ]; then will_notarize=yes; fi
if [ "$will_notarize" = yes ]; then dmg_name="Tap-$version.dmg"; else dmg_name="Tap-$version-unnotarized.dmg"; fi
dmg="$out/$dmg_name"
zip="$out/Tap-$version.zip"
if [ "$identity" = "-" ]; then
	note "skipped: notarization of $(basename "$zip") (no Developer ID identity)"
elif [ "$will_notarize" = no ]; then
	note "skipped: notarization of $(basename "$zip") ($missing_notary_secret)"
else
	rm -f "$zip"
	ditto -c -k --keepParent "$app" "$zip"
	result=$("$here/notarize.sh" "$zip" "$app") || { echo "release.sh: the app's notarization failed; nothing is published" >&2; exit 1; }
	note "$result"
	rm -f "$zip"
fi

rm -f "$out"/Tap-"$version"*.dmg "$out"/Tap-"$version"*.dmg.sha256
"$here/make-dmg.sh" "$app" "$dmg" Tap >/dev/null
note "done: wrote $dmg_name"
if [ "$identity" = "-" ]; then
	note "skipped: DMG signature (no Developer ID identity)"
	note "skipped: notarization of $dmg_name (no Developer ID identity)"
else
	codesign --force --sign "$identity" --timestamp "$dmg"
	note "done: signed $dmg_name"
	if [ "$will_notarize" = yes ]; then
		result=$("$here/notarize.sh" "$dmg" "$dmg") || { echo "release.sh: the DMG's notarization failed; nothing is published" >&2; exit 1; }
		note "$result"
		notarized=yes
	else
		note "skipped: notarization of $dmg_name ($missing_notary_secret)"
	fi
fi
( cd "$out" && shasum -a 256 "$dmg_name" > "$dmg_name.sha256" )
note "done: wrote $dmg_name.sha256"

# The release notes: the version's own section of the changelog, which
# the CLI job wrote before tagging. A dry-run version has none.
notes=""
notes_url=""
notes_signature=""
notes_length=""
rm -f "$out/Tap-$version.md"
if [ -f "$changelog" ] && [ -x "$prepare_changelog" ] && grep -q "^## \[$(printf '%s' "$version" | sed 's/[.]/\\./g')\]" "$changelog"; then
	cp "$changelog" "$out/CHANGELOG.copy.md"
	"$prepare_changelog" "$version" "$out/CHANGELOG.copy.md" "$out/Tap-$version.md" >/dev/null
	rm -f "$out/CHANGELOG.copy.md"
	notes="Tap-$version.md"
	notes_url="https://github.com/MiniCodeMonkey/tap/releases/download/v$version/Tap-$version.md"
	note "done: wrote $notes"
	signed_notes=$("$here/sparkle-sign.sh" notes "$out/$notes" 2>"$out/sparkle.log") || { cat "$out/sparkle.log" >&2; exit 1; }
	if [ -n "$signed_notes" ]; then
		notes_signature="${signed_notes%% *}"
		notes_length="${signed_notes##* }"
		note "done: signed $notes for Sparkle"
	else
		note "$(grep '^skipped:' "$out/sparkle.log")"
	fi
else
	note "skipped: release notes (CHANGELOG.md has no section for $version)"
fi

# The appcast: signed only when the DMG, the notes (if any) and the feed
# itself carry signatures; otherwise named so nothing uploads it.
signature=$("$here/sparkle-sign.sh" archive "$dmg" 2>"$out/sparkle.log") || { cat "$out/sparkle.log" >&2; exit 1; }
if [ -n "$signature" ]; then
	note "done: signed $dmg_name for Sparkle"
else
	note "$(grep '^skipped:' "$out/sparkle.log")"
fi
download_url="https://github.com/MiniCodeMonkey/tap/releases/download/v$version/$dmg_name"
release_url="https://github.com/MiniCodeMonkey/tap/releases/tag/v$version"
rm -f "$out/appcast.xml" "$out/appcast-unsigned.xml"
feed_signed=no
if [ -n "$signature" ] && [ "$notarized" = yes ] && { [ -z "$notes" ] || [ -n "$notes_signature" ]; }; then
	appcast="appcast.xml"
	"$here/write-appcast.sh" "$app" "$dmg" "$download_url" "$release_url" "$out/$appcast" "$signature" "$notes_url" "$notes_signature" "$notes_length" >/dev/null
	"$here/sparkle-sign.sh" feed "$out/$appcast" 2>"$out/sparkle.log" || { cat "$out/sparkle.log" >&2; exit 1; }
	feed_signed=yes
	note "done: wrote $appcast (signed)"
else
	appcast="appcast-unsigned.xml"
	"$here/write-appcast.sh" "$app" "$dmg" "$download_url" "$release_url" "$out/$appcast" "$signature" "$notes_url" "$notes_signature" "$notes_length" >/dev/null
	note "done: wrote $appcast (unsigned; never uploaded)"
fi

"$here/render-cask.sh" "$version" "$dmg" "$out/Casks/tap-desktop.rb" >/dev/null
note "done: rendered Casks/tap-desktop.rb (not pushed by this script)"

"$here/verify-release.sh" "$app" "$dmg" "$out/$appcast" "$identity" "$version" "$notarized" "$feed_signed" >/dev/null
if [ "$notarized" = yes ]; then note "done: Gatekeeper accepts $dmg_name"; else note "skipped: Gatekeeper assessment (not notarized)"; fi
note "done: verified the app, the DMG and the appcast"

# Plain words only, so the release job can source this under bash -e: the
# identity's name (with its spaces and parentheses) stays out of it.
cat > "$state" <<STATE
version=$version
notarized=$notarized
feed_signed=$feed_signed
dmg=$dmg_name
checksum=$dmg_name.sha256
notes=$notes
appcast=$appcast
cask=Casks/tap-desktop.rb
STATE

echo
echo "release $version in $out:"
cat "$summary"

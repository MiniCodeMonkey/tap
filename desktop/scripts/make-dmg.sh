#!/bin/sh
# Writes a compressed disk image holding the app and a link to
# /Applications, from a staging folder of its own. ditto keeps the app's
# signature, resource forks and extended attributes; cp -R does not always.
# Plain by design: no background art, no Finder scripting, so it runs on a
# runner and on a Mac nobody is watching. hdiutil create sometimes fails
# with "Resource busy" on a hosted runner, so it gets three tries.
set -eu

app="${1:-}"
output="${2:-}"
volume="${3:-Tap}"
[ -d "$app" ] || { echo "make-dmg.sh: $app is not a folder" >&2; exit 1; }
[ -n "$output" ] || { echo "make-dmg.sh: no output path" >&2; exit 1; }
delay="${MAKE_DMG_RETRY_DELAY:-5}"

staging=$(mktemp -d)
trap 'rm -rf "$staging"' EXIT

ditto "$app" "$staging/$(basename "$app")"
ln -s /Applications "$staging/Applications"
rm -f "$output"
attempt=1
until hdiutil create -volname "$volume" -srcfolder "$staging" -fs HFS+ -format UDZO -imagekey zlib-level=9 -ov -quiet "$output"; do
	if [ "$attempt" -ge 3 ]; then
		echo "make-dmg.sh: hdiutil create failed three times" >&2
		exit 1
	fi
	attempt=$((attempt + 1))
	echo "make-dmg.sh: hdiutil create failed; attempt $attempt of 3 in ${delay}s" >&2
	sleep "$delay"
	rm -f "$output"
done
hdiutil verify -quiet "$output"
echo "wrote $output"

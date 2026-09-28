#!/bin/sh
# Downloads Sparkle's release archive, checks it against the pinned
# checksum, and leaves bin/sign_update in the folder given. The caller
# names the folder after the version (build/sparkle-tools-2.10.0), so a
# bump never trusts an older tool left in place. The version here and
# exactVersion in project.yml move together.
set -eu

folder="${1:-}"
[ -n "$folder" ] || { echo "fetch-sparkle-tools.sh: no folder" >&2; exit 1; }
version="2.10.0"
sha256="c2bf58aa8387266ac179357b1415d6f2635f044da8be41042af32425dae6da0c"
archive="$folder/Sparkle-$version.tar.xz"

case "$folder" in
	*"$version"*) ;;
	*) echo "fetch-sparkle-tools.sh: the folder must carry the version ($version): $folder" >&2; exit 1 ;;
esac
if [ -x "$folder/bin/sign_update" ]; then
	echo "Sparkle $version tools are in $folder"
	exit 0
fi
mkdir -p "$folder"
curl -sSL --fail -o "$archive" "https://github.com/sparkle-project/Sparkle/releases/download/$version/Sparkle-$version.tar.xz"
actual=$(shasum -a 256 "$archive" | cut -d ' ' -f 1)
if [ "$actual" != "$sha256" ]; then
	rm -f "$archive"
	echo "fetch-sparkle-tools.sh: Sparkle-$version.tar.xz has sha256 $actual, not the pinned $sha256" >&2
	exit 1
fi
tar -xJf "$archive" -C "$folder" bin/sign_update bin/generate_keys
rm -f "$archive"
echo "Sparkle $version tools are in $folder"

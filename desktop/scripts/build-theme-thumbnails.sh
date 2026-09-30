#!/bin/sh
# Renders every built-in theme's thumbnail with the bundled tap into the app
# bundle, at Contents/Resources/ThemeThumbnails/<slug>.png, beside
# catalog.json (`tap theme list --json`). The app shows them at once instead
# of running tap for each. Each render is 1280x720 from `tap theme show
# --image`, scaled to 640x360: 2x the largest thumbnail the app draws.
#
# The renders depend on the tap binary alone (it embeds the themes and the
# frontend), so a stamp of its checksum, kept in the target's temp folder,
# skips the step while the binary is unchanged.
set -eu

destination="$TARGET_BUILD_DIR/$UNLOCALIZED_RESOURCES_FOLDER_PATH"
tap="$destination/tap"
output="$destination/ThemeThumbnails"
stamp="$TARGET_TEMP_DIR/theme-thumbnails.stamp"
[ -x "$tap" ] || { echo "error: $tap is missing; the Bundle tap phase runs first" >&2; exit 1; }

checksum="$(shasum -a 256 "$tap" | cut -d' ' -f1)"
if [ -f "$stamp" ] && [ "$(cat "$stamp")" = "$checksum" ] && [ -f "$output/catalog.json" ]; then
	echo "theme thumbnails are current"
	exit 0
fi

scratch="$(mktemp -d "${TMPDIR:-/tmp}/tap-theme-thumbnails.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
mkdir -p "$scratch/out"

"$tap" theme list --json > "$scratch/out/catalog.json"
slugs="$(grep -o '"slug": *"[^"]*"' "$scratch/out/catalog.json" | sed 's/.*: *"\(.*\)"/\1/')"
[ -n "$slugs" ] || { echo "error: tap listed no themes" >&2; exit 1; }

# The first render downloads the export engine, so it runs alone; the rest
# run four at a time, each in its own headless browser.
export TAP_BINARY="$tap" SCRATCH="$scratch"
render='"$TAP_BINARY" theme show "$1" --image --output "$SCRATCH/$1.full.png" >/dev/null && sips --resampleHeight 360 "$SCRATCH/$1.full.png" --out "$SCRATCH/out/$1.png" >/dev/null'
first="$(echo "$slugs" | head -1)"
sh -c "$render" render "$first"
echo "$slugs" | tail -n +2 | xargs -P 4 -I{} sh -c "$render" render {}

for slug in $slugs; do
	[ -s "$scratch/out/$slug.png" ] || { echo "error: no thumbnail rendered for $slug" >&2; exit 1; }
done

rm -rf "$output"
mv "$scratch/out" "$output"
echo "$checksum" > "$stamp"

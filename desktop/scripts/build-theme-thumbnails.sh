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

# A Release build fails when any thumbnail cannot be rendered. Any other
# build warns and goes on: the app renders a missing theme on demand. The
# stamp is written only after a complete run, so the next build tries again.
problem() {
	if [ "${CONFIGURATION:-}" = "Release" ]; then
		echo "error: $1" >&2
		exit 1
	fi
	echo "warning: $1; the app renders the missing themes on demand" >&2
}

if ! "$tap" theme list --json > "$scratch/out/catalog.json" 2>/dev/null; then
	problem "tap could not list the themes"
	exit 0
fi
slugs="$(grep -o '"slug": *"[^"]*"' "$scratch/out/catalog.json" | sed 's/.*: *"\(.*\)"/\1/')"
[ -n "$slugs" ] || { problem "tap listed no themes"; exit 0; }

# The first render downloads the export engine, so it runs alone; when it
# fails, the rest would too. The others run four at a time, each in its own
# headless browser.
export TAP_BINARY="$tap" SCRATCH="$scratch"
render='"$TAP_BINARY" theme show "$1" --image --output "$SCRATCH/$1.full.png" >/dev/null && sips --resampleHeight 360 "$SCRATCH/$1.full.png" --out "$SCRATCH/out/$1.png" >/dev/null'
first="$(echo "$slugs" | head -1)"
if sh -c "$render" render "$first"; then
	echo "$slugs" | tail -n +2 | xargs -P 4 -I{} sh -c "$render" render {} || true
fi

missing=""
for slug in $slugs; do
	[ -s "$scratch/out/$slug.png" ] || missing="$missing $slug"
done

if [ -n "$missing" ]; then
	problem "no thumbnail rendered for:$missing"
	# Keep what rendered, and the images from an earlier build for the rest.
	mkdir -p "$output"
	cp -f "$scratch"/out/* "$output"/
	exit 0
fi

rm -rf "$output"
mv "$scratch/out" "$output"
echo "$checksum" > "$stamp"

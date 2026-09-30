#!/bin/sh
# Renders every slide of the theme tour with the bundled tap into the app
# bundle, at Contents/Resources/TourThumbnails/slide-<n>.png, beside
# theme-tour.md, the text they were rendered from. The app shows them at
# once for an unedited tour instead of rendering each slide. Each render is
# the slide as the slide panel captures it: print mode at 1920x1080, scaled
# to 640 pixels wide (320 points at 2x, the panel's snapshot width).
#
# The renders depend on the tap binary and the tour's text, so a stamp of
# both checksums, kept in the target's temp folder, skips the step while
# neither changes.
set -eu

destination="$TARGET_BUILD_DIR/$UNLOCALIZED_RESOURCES_FOLDER_PATH"
tap="$destination/tap"
tour="$SRCROOT/../examples/theme-tour.md"
output="$destination/TourThumbnails"
stamp="$TARGET_TEMP_DIR/tour-thumbnails.stamp"
[ -x "$tap" ] || { echo "error: $tap is missing; the Bundle tap phase runs first" >&2; exit 1; }
[ -f "$tour" ] || { echo "error: $tour is missing" >&2; exit 1; }

checksum="$(cat "$tap" "$tour" | shasum -a 256 | cut -d' ' -f1)"
if [ -f "$stamp" ] && [ "$(cat "$stamp")" = "$checksum" ] && [ -f "$output/theme-tour.md" ]; then
	echo "tour thumbnails are current"
	exit 0
fi

scratch="$(mktemp -d "${TMPDIR:-/tmp}/tap-tour-thumbnails.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
mkdir -p "$scratch/out" "$scratch/deck"

# A Release build fails when any thumbnail cannot be rendered. Any other
# build warns and goes on: the app renders the tour's slides on demand. The
# stamp is written only after a complete run, so the next build tries again.
problem() {
	if [ "${CONFIGURATION:-}" = "Release" ]; then
		echo "error: $1" >&2
		exit 1
	fi
	echo "warning: $1; the app renders the tour's slides on demand" >&2
}

# The deck is rendered from a folder of its own, so tap writes nothing beside the source.
cp "$tour" "$scratch/deck/theme-tour.md"
if ! "$tap" export images "$scratch/deck/theme-tour.md" --all --width 1920 --output "$scratch/full" >/dev/null 2>&1; then
	problem "tap could not render the theme tour"
	exit 0
fi

count=0
for full in "$scratch"/full/slide-*.png; do
	[ -s "$full" ] || continue
	count=$((count + 1))
	sips --resampleWidth 640 "$full" --out "$scratch/out/slide-$count.png" >/dev/null 2>&1 || true
done
[ "$count" -gt 0 ] || { problem "tap rendered no tour slides"; exit 0; }

missing=""
number=1
while [ "$number" -le "$count" ]; do
	[ -s "$scratch/out/slide-$number.png" ] || missing="$missing $number"
	number=$((number + 1))
done
if [ -n "$missing" ]; then
	problem "no thumbnail rendered for tour slides:$missing"
	exit 0
fi

cp "$tour" "$scratch/out/theme-tour.md"
rm -rf "$output"
mv "$scratch/out" "$output"
echo "$checksum" > "$stamp"

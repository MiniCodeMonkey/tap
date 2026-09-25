#!/bin/sh
# Builds tap from this repository into the app bundle, at
# Contents/Resources/tap, with the app's version.
set -eu

repository_root="$(cd "$SRCROOT/.." && pwd)"
built_assets="$repository_root/embedded/dist/index.html"
if [ ! -f "$built_assets" ]; then
	echo "error: $repository_root/embedded/dist is missing. Run 'make frontend' in $repository_root first." >&2
	exit 1
fi

# tap embeds embedded/dist, so an app built from assets older than the
# frontend they came from runs yesterday's page against today's Swift, and
# the app's own tests then measure a page nobody wrote. Every input to the
# bundle lives under frontend/, so anything there touched after the last
# build means the assets are stale. node_modules is the one exception: npm
# rewrites it without changing what vite emits.
stale_sources="$(find "$repository_root/frontend" \
	-name node_modules -prune -o \
	-type f -newer "$built_assets" -print 2>/dev/null | head -5)"
if [ -n "$stale_sources" ]; then
	echo "error: $repository_root/embedded/dist is older than the frontend source. Run 'make frontend' in $repository_root first." >&2
	echo "changed since the assets were built:" >&2
	echo "$stale_sources" | sed 's|^|  |' >&2
	exit 1
fi

# Xcode runs build phases with a short PATH.
export PATH="/opt/homebrew/bin:/usr/local/go/bin:/usr/local/bin:$HOME/go/bin:$PATH"
if ! command -v go >/dev/null 2>&1; then
	echo "error: go is not on PATH" >&2
	exit 1
fi

destination="$TARGET_BUILD_DIR/$UNLOCALIZED_RESOURCES_FOLDER_PATH"
mkdir -p "$destination"
cd "$repository_root"
go build -ldflags "-X github.com/MiniCodeMonkey/tap/internal/cli.Version=${TAP_VERSION}" -o "$destination/tap" ./cmd/tap

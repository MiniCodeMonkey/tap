#!/bin/sh
# Builds tap from this repository into the app bundle, at
# Contents/Resources/tap, with the app's version.
set -eu

repository_root="$(cd "$SRCROOT/.." && pwd)"
if [ ! -f "$repository_root/embedded/dist/index.html" ]; then
	echo "error: $repository_root/embedded/dist is missing. Run 'make frontend' in $repository_root first." >&2
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

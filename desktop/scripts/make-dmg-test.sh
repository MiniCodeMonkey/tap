#!/bin/sh
# Checks make-dmg.sh on a bundle of its own: the image mounts without a
# Finder window, holds the app and an Applications link, verifies, and a
# transient hdiutil failure is retried.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/make-dmg.sh"
root=$(mktemp -d)
mount="$root/mount"
cleanup() {
	[ -d "$mount" ] && hdiutil detach "$mount" -quiet -force >/dev/null 2>&1 || true
	rm -rf "$root"
}
trap cleanup EXIT

app="$root/Fake.app"
mkdir -p "$app/Contents/MacOS"
cp /usr/bin/true "$app/Contents/MacOS/Fake"
printf 'hello' > "$app/Contents/marker.txt"

"$script" "$app" "$root/Fake-1.0.0.dmg" "Fake" >/dev/null || { echo "the DMG should be made"; exit 1; }
[ -f "$root/Fake-1.0.0.dmg" ] || { echo "no DMG"; exit 1; }
hdiutil verify "$root/Fake-1.0.0.dmg" -quiet || { echo "the DMG should verify"; exit 1; }

mkdir -p "$mount"
hdiutil attach "$root/Fake-1.0.0.dmg" -nobrowse -readonly -noverify -quiet -mountpoint "$mount"
[ -f "$mount/Fake.app/Contents/MacOS/Fake" ] || { echo "the app is not in the image"; exit 1; }
[ "$(cat "$mount/Fake.app/Contents/marker.txt")" = "hello" ] || { echo "the app's files did not copy"; exit 1; }
[ -L "$mount/Applications" ] && [ "$(readlink "$mount/Applications")" = "/Applications" ] || { echo "no Applications link"; exit 1; }
entries=0; for entry in "$mount"/*; do [ -e "$entry" ] && entries=$((entries + 1)); done
[ "$entries" = "2" ] || { echo "the image holds more than the app and the link: $(ls -A "$mount")"; exit 1; }
hdiutil detach "$mount" -quiet

# The output is replaced, not appended to.
"$script" "$app" "$root/Fake-1.0.0.dmg" "Fake" >/dev/null || { echo "a second run should replace the DMG"; exit 1; }

# A hdiutil that fails twice with "Resource busy" and then works is retried:
# a shim on PATH fails its first two create calls and hands the rest to the
# real tool.
mkdir -p "$root/bin"
cat > "$root/bin/hdiutil" <<SHIM
#!/bin/sh
if [ "\$1" = create ]; then
	count=\$(cat "$root/attempts" 2>/dev/null || echo 0)
	count=\$((count + 1))
	echo "\$count" > "$root/attempts"
	if [ "\$count" -le 2 ]; then echo "hdiutil: create failed - Resource busy" >&2; exit 1; fi
fi
exec /usr/bin/hdiutil "\$@"
SHIM
chmod +x "$root/bin/hdiutil"
PATH="$root/bin:$PATH" MAKE_DMG_RETRY_DELAY=0 "$script" "$app" "$root/Retry.dmg" "Fake" >/dev/null || { echo "two busy failures should be retried"; exit 1; }
[ "$(cat "$root/attempts")" = "3" ] || { echo "expected three create attempts, got $(cat "$root/attempts")"; exit 1; }
rm -f "$root/attempts"
cat > "$root/bin/hdiutil" <<'SHIM'
#!/bin/sh
if [ "$1" = create ]; then echo "hdiutil: create failed - Resource busy" >&2; exit 1; fi
exec /usr/bin/hdiutil "$@"
SHIM
if PATH="$root/bin:$PATH" MAKE_DMG_RETRY_DELAY=0 "$script" "$app" "$root/Never.dmg" "Fake" >/dev/null 2>&1; then echo "a hdiutil that always fails should fail the script"; exit 1; fi

if "$script" "$root/missing.app" "$root/x.dmg" >/dev/null 2>&1; then echo "a missing app should fail"; exit 1; fi

echo "make-dmg.sh is right"

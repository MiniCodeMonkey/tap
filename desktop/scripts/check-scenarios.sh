#!/bin/sh
# Fails when a scenario that desktop/scenarios.txt claims is missing from its
# feature file, or has no test named after it.
set -eu

root="${1:-$(cd "$(dirname "$0")/../.." && pwd)}"
manifest="$root/desktop/scenarios.txt"
features="$root/docs/superpowers/specs/tap-desktop-features"
tests=$(find "$root/desktop" -name '*.swift' -path '*Tests*' -o -name '*.swift' -path '*Benchmarks*')
status=0
seen=""

while IFS='|' read -r milestone file scenario; do
	milestone=$(printf '%s' "${milestone:-}" | tr -d '[:space:]')
	case "$milestone" in ''|\#*) continue ;; esac
	file=$(printf '%s' "$file" | sed 's/^ *//;s/ *$//')
	scenario=$(printf '%s' "$scenario" | sed 's/^ *//;s/ *$//')

	if ! grep -Fq "Scenario: $scenario" "$features/$file"; then
		echo "$milestone claims \"$scenario\", which $file does not have"
		status=1
		continue
	fi

	name=$(printf '%s' "$scenario" | tr -c '[:alnum:]' ' ' |
		awk '{ out = ""; for (i = 1; i <= NF; i++) out = out toupper(substr($i, 1, 1)) substr($i, 2); print out }')
	case " $seen " in
		*" $name "*)
			echo "two scenarios both map to test$name"
			status=1
			continue
			;;
	esac
	seen="$seen $name"

	if ! grep -lq "func test$name(" $tests >/dev/null 2>&1 && ! grep -q "func test$name(" $tests; then
		echo "$milestone claims \"$scenario\" ($file), but no test is named test$name"
		status=1
	fi
done < "$manifest"

if [ "$status" -eq 0 ]; then
	echo "every claimed scenario has a test"
fi
exit "$status"

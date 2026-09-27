#!/bin/sh
# Fails when a scenario that desktop/scenarios.txt claims is missing from its
# feature file, or has no test named after it.
set -eu

root="${1:-$(cd "$(dirname "$0")/../.." && pwd)}"
manifest="$root/desktop/scenarios.txt"
features="$root/docs/superpowers/specs/tap-desktop-features"
swift_tests=$(find "$root/desktop" -name '*.swift' -path '*Tests*' -o -name '*.swift' -path '*Benchmarks*')
go_tests=$(find "$root/internal" -name '*_test.go' 2>/dev/null || true)

# A scenario is covered by a Swift test named testName, or by a Go test
# named TestName, in the tap packages: the CLI-only scenarios of a feature
# file are tap's to prove. An empty file list would make grep read stdin,
# which inside the loop is the manifest, so each list is checked first.
has_test() {
	[ -n "$swift_tests" ] && grep -q "func test$1(" $swift_tests </dev/null 2>/dev/null && return 0
	[ -n "$go_tests" ] && grep -q "func Test$1(" $go_tests </dev/null 2>/dev/null && return 0
	return 1
}

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

	if ! has_test "$name"; then
		echo "$milestone claims \"$scenario\" ($file), but no test is named test$name or Test$name"
		status=1
	fi
done < "$manifest"

if [ "$status" -eq 0 ]; then
	echo "every claimed scenario has a test"
fi
exit "$status"

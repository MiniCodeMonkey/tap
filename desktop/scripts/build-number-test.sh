#!/bin/sh
# Checks build-number.sh: alpha sorts below beta below rc below the final,
# every pre-release of a version sorts below that version's final and above
# the previous patch, a number outside its label's range fails, and a bad
# version fails. The repository's own tags (2.0.0-beta.1 to beta.7, then
# 2.0.0-rc.1) are the case that matters most.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/build-number.sh"

expect() {
	got=$("$script" "$1") || { echo "$1: exit $?"; exit 1; }
	[ "$got" = "$2" ] || { echo "$1: got $got, want $2"; exit 1; }
}

expect 0.0.0 99
expect 0.0.0-dev 1
expect 0.0.0-ci 1
expect 2.0.0 2000099
expect 2.0.0-alpha.1 2000001
expect 2.0.0-alpha.19 2000019
expect 2.0.0-beta.1 2000020
expect 2.0.0-beta.7 2000026
expect 2.0.0-beta.30 2000049
expect 2.0.0-rc.1 2000050
expect 2.0.0-rc.40 2000089
expect 2.1.0-beta.3 2010022
expect 2.1.0 2010099
expect 2.0.9 2000999
expect 1.9.9 1090999
expect 10.20.30 10203099

# The repository's history, in order.
beta7=$("$script" 2.0.0-beta.7); rc1=$("$script" 2.0.0-rc.1); final=$("$script" 2.0.0); previous=$("$script" 1.9.9)
[ "$previous" -lt "$beta7" ] && [ "$beta7" -lt "$rc1" ] && [ "$rc1" -lt "$final" ] || { echo "1.9.9 < beta.7 < rc.1 < 2.0.0 does not hold: $previous $beta7 $rc1 $final"; exit 1; }

# A number past its label's range would reach the next label's slots.
for bad in 1.0.0-alpha.20 1.0.0-beta.31 1.0.0-rc.41 1.0.0-alpha.0 1.0.0-beta 1.0.0-rc 1.0.0-nightly.3; do
	if "$script" "$bad" >/dev/null 2>&1; then echo "'$bad' should fail"; exit 1; fi
done
# Versions the release workflow's regex refuses fail here too, before a build.
for bad in 1.0 v1.0.0 1.0.0.0 1.0.0- "1.0.0 " abc ""; do
	if "$script" "$bad" >/dev/null 2>&1; then echo "'$bad' should fail"; exit 1; fi
done
# Minor and patch have two digits each.
if "$script" 1.100.0 >/dev/null 2>&1; then echo "1.100.0 should fail"; exit 1; fi

echo "build-number.sh is right"

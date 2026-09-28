#!/bin/sh
# Prints the CFBundleVersion for a semantic version: an integer that rises
# with every release, which Sparkle compares to decide what is an update.
#
#   MAJOR * 1000000 + MINOR * 10000 + PATCH * 100 + release slot
#
# The release slot ranks the label and its number: alpha.N is N (1 to 19),
# beta.N is 19 + N (20 to 49), rc.N is 49 + N (50 to 89), a final is 99,
# and dev or ci with no number (the dry runs) is 1; any other label fails.
# No number has a leading zero, which shell arithmetic reads as octal. So the
# repository's own 2.0.0-beta.7 (2000026) sorts below 2.0.0-rc.1 (2000050),
# which sorts below 2.0.0 (2000099); every 2.0.0 pre-release sorts above
# 1.9.9 (1090999). Minor and patch run to 99 each. Once a build has shipped,
# this scheme can only grow, never change.
set -eu

version="${1:-}"
case "$version" in
	*[!0-9A-Za-z.-]*|"") echo "build-number.sh: '$version' is not a version" >&2; exit 1 ;;
esac

core="${version%%-*}"
prerelease=""
case "$version" in
	*-*)
		prerelease="${version#*-}"
		[ -n "$prerelease" ] || { echo "build-number.sh: '$version' ends in a hyphen" >&2; exit 1; }
		;;
esac

case "$core" in
	*.*.*) ;;
	*) echo "build-number.sh: '$version' needs MAJOR.MINOR.PATCH" >&2; exit 1 ;;
esac
major="${core%%.*}"; rest="${core#*.}"
minor="${rest%%.*}"; patch="${rest#*.}"
for part in "$major" "$minor" "$patch"; do
	case "$part" in
		""|*[!0-9]*|*.*) echo "build-number.sh: '$version' is not MAJOR.MINOR.PATCH" >&2; exit 1 ;;
		0?*) echo "build-number.sh: '$version' has a leading zero" >&2; exit 1 ;;
	esac
done
if [ "$minor" -gt 99 ] || [ "$patch" -gt 99 ]; then
	echo "build-number.sh: minor and patch run to 99 ($version)" >&2
	exit 1
fi

slot=99
if [ -n "$prerelease" ]; then
	case "$prerelease" in
		*[!0-9A-Za-z.]*|.*|*.|*..*) echo "build-number.sh: '$prerelease' is not a pre-release label" >&2; exit 1 ;;
	esac
	label="${prerelease%%.*}"
	number=""
	case "$prerelease" in *.*) number="${prerelease#*.}" ;; esac
	case "$number" in
		*[!0-9]*|*.*) echo "build-number.sh: '$prerelease' needs <label>.<number>" >&2; exit 1 ;;
		0?*) echo "build-number.sh: '$prerelease' has a leading zero" >&2; exit 1 ;;
	esac
	case "$label" in
		alpha) base=0; limit=19 ;;
		beta) base=19; limit=30 ;;
		rc) base=49; limit=40 ;;
		dev|ci)
			[ -z "$number" ] || { echo "build-number.sh: '$label' takes no number ($version)" >&2; exit 1; }
			base=0; limit=1; number=1
			;;
		*) echo "build-number.sh: '$label' is not a label (alpha.N, beta.N, rc.N, or dev and ci for a dry run)" >&2; exit 1 ;;
	esac
	[ -n "$number" ] || { echo "build-number.sh: '$label' needs a number ($version)" >&2; exit 1; }
	number=$((number))
	if [ "$number" -lt 1 ] || [ "$number" -gt "$limit" ]; then
		echo "build-number.sh: $label runs from 1 to $limit ($version)" >&2
		exit 1
	fi
	slot=$((base + number))
fi

echo $((major * 1000000 + minor * 10000 + patch * 100 + slot))

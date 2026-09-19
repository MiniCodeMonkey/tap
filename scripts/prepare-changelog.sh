#!/usr/bin/env bash
#
# Turns CHANGELOG.md's [Unreleased] section into a version's release notes,
# used by .github/workflows/release.yml. Safe to run twice for the same
# version (the release workflow re-run after a deleted tag): the second run
# finds the version's own section already in the changelog and reuses it,
# instead of finding an empty [Unreleased] and publishing hollow notes with
# a duplicate version header.
#
# Usage: scripts/prepare-changelog.sh <version> <changelog-file> <notes-output-file>
# Example: scripts/prepare-changelog.sh 1.2.0 CHANGELOG.md release_notes.md
#
# Exit status 1, with a message on standard error, when neither an existing
# section for <version> nor a non-empty [Unreleased] section can be found -
# publishing empty release notes is a bug, not a fallback.
#
# CHANGELOG_DATE, if set, is used as the new version header's date (tests
# use this so fixtures do not depend on today's date); otherwise it
# defaults to today (UTC).

set -euo pipefail

if [[ $# -ne 3 ]]; then
    echo "Usage: $0 <version> <changelog-file> <notes-output-file>" >&2
    exit 1
fi

VERSION="$1"
CHANGELOG="$2"
NOTES_OUTPUT="$3"
DATE="${CHANGELOG_DATE:-$(date -u +%Y-%m-%d)}"

if [[ ! -f "$CHANGELOG" ]]; then
    echo "Error: changelog file not found: $CHANGELOG" >&2
    exit 1
fi

# sed's basic regex treats '.' and '[' specially; the only characters a
# version or the literal "[" around it can carry that matter here are '.'
# and '-', and '-' is not special outside a bracket expression, so only the
# dots need escaping.
escaped_version="${VERSION//./\\.}"
version_header_pattern="## \\[${escaped_version}\\]"

# extract_section prints the non-empty lines between a "## [...]" header
# line (matched by $1, a grep -E pattern) and the next "## [" header, or
# the end of the file if there is none.
extract_section() {
    local start_pattern="$1"
    local raw last_line
    raw="$(sed -n "/${start_pattern}/,/## \\[/p" "$CHANGELOG")"
    last_line="$(printf '%s\n' "$raw" | tail -n 1)"
    if [[ "$last_line" == "## ["* ]]; then
        # The range matched a following header line: drop it along with
        # the section's own header.
        printf '%s\n' "$raw" | sed '1d;$d' | sed '/^$/d'
    else
        # No following header: the range ran to the end of the file, so
        # its last line is real content, not a header to strip. Dropping
        # it too (the previous "sed '1d;$d'" did unconditionally) silently
        # lost a section's last line whenever it was also the changelog's
        # last section.
        printf '%s\n' "$raw" | sed '1d' | sed '/^$/d'
    fi
}

if grep -qE "^${version_header_pattern}" "$CHANGELOG"; then
    # Second run for this version: the tag was deleted and the workflow
    # re-run, but the changelog already carries this version's own section
    # from the first run. Reuse its notes verbatim and leave the changelog
    # alone - rewriting the header again would duplicate it, and there is
    # nothing left under [Unreleased] to move.
    echo "note: CHANGELOG.md already has a section for $VERSION, reusing its notes" >&2
    NOTES="$(extract_section "$version_header_pattern")"
    echo "$NOTES" > "$NOTES_OUTPUT"
    exit 0
fi

NOTES="$(extract_section '## \[Unreleased\]')"

if [[ -z "$NOTES" ]]; then
    echo "Error: [Unreleased] in CHANGELOG.md is empty and no [$VERSION] section exists yet." >&2
    echo "Add changelog entries under [Unreleased] before releasing." >&2
    exit 1
fi

echo "$NOTES" > "$NOTES_OUTPUT"

NEW_HEADER="## [${VERSION}] - ${DATE}"
sed -i.bak "s/## \\[Unreleased\\]/## [Unreleased]\\n\\n${NEW_HEADER}/" "$CHANGELOG"
rm -f "${CHANGELOG}.bak"

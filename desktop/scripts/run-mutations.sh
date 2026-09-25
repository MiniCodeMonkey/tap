#!/bin/sh
# Runs each mutation in mutations/*.patch against the hosted tests, one at a
# time, and reports whether the named test caught it.
#
# A mutation file is a unified diff (as `git diff` writes it) preceded by
# one line naming the test that must fail with the mutation applied:
#
#   Test: TapTests/PresentingTests/testStopReleasesTheSleepAssertion
#   diff --git a/desktop/Tap/... b/desktop/Tap/...
#   ...
#
# A mutation is KILLED when that test fails, and SURVIVED when it passes.
# A failure that is the test running past its execution time allowance
# (the Makefile's per-test timeout) is reported as TIMEOUT, since a hang
# says nothing about the assertion the mutation targets. A run that fails
# with no test failure in its result (a patch that does not build) is NO
# RESULT. Each row carries the first failure message from that run's
# xcresult. A patch that no longer applies is reported as STALE. The
# working tree is restored after each mutation. Exit status is 1 when any
# mutation survived, timed out, had no result or went stale, so the run
# shows red.
set -u

repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root" || exit 2
report=${MUTATION_REPORT:-mutation-report.md}
: > "$report"
printf '| Mutation | Test | Result | First failure |\n|---|---|---|---|\n' >> "$report"
unkilled=0
results_folder=desktop/build/DerivedData/Logs/Test
marker=$(mktemp)
summary=$(mktemp)
trap 'rm -f "$marker" "$summary"' EXIT

# Writes the summary of the newest xcresult written after the marker to
# $summary, or empties it when there is none.
read_summary() {
    : > "$summary"
    newest=$(find "$results_folder" -maxdepth 1 -name '*.xcresult' -newer "$marker" 2>/dev/null | sort | tail -n 1)
    [ -n "$newest" ] || return 0
    xcrun xcresulttool get test-results summary --path "$newest" > "$summary" 2>/dev/null || : > "$summary"
}

# The first failure message in $summary, on one line and safe inside a
# Markdown table cell.
first_failure() {
    [ -s "$summary" ] || return 0
    plutil -extract testFailures.0.failureText raw -o - "$summary" 2>/dev/null | tr '\n|' ' /' | cut -c 1-300
}

# Whether any failure in $summary is a test running past its allowance.
timed_out() {
    grep -qi 'execution time allowance' "$summary"
}

for patch in mutations/*.patch; do
    [ -e "$patch" ] || { echo "no mutations/*.patch files"; exit 2; }
    name=$(basename "$patch" .patch)
    test_name=$(sed -n '1s/^Test: //p' "$patch")
    if [ -z "$test_name" ]; then
        printf '| %s | (none) | STALE: no Test line | |\n' "$name" >> "$report"
        unkilled=1
        continue
    fi
    if ! git apply "$patch"; then
        printf '| %s | %s | STALE: patch does not apply | |\n' "$name" "$test_name" >> "$report"
        unkilled=1
        continue
    fi
    echo "::group::$name ($test_name)"
    touch "$marker"
    # The newest xcresult is found by modification time, so this run's
    # must be strictly newer than the marker.
    sleep 1
    failure=""
    if make -C desktop test ONLY="$test_name"; then
        result="SURVIVED"
        unkilled=1
    else
        read_summary
        failure=$(first_failure)
        if timed_out; then
            result="TIMEOUT"
            unkilled=1
        elif [ -z "$failure" ]; then
            result="NO RESULT"
            failure="no test failure in the xcresult (did the patch build?)"
            unkilled=1
        else
            result="KILLED"
        fi
    fi
    echo "::endgroup::"
    git checkout -- desktop
    printf '| %s | %s | %s | %s |\n' "$name" "$test_name" "$result" "$failure" >> "$report"
done

cat "$report"
exit "$unkilled"

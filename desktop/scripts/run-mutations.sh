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
# A patch that no longer applies is reported as STALE. The working tree is
# restored after each mutation. Exit status is 1 when any mutation survived
# or went stale, so the run shows red.
set -u

repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root" || exit 2
report=${MUTATION_REPORT:-mutation-report.md}
: > "$report"
printf '| Mutation | Test | Result |\n|---|---|---|\n' >> "$report"
unkilled=0

for patch in mutations/*.patch; do
    [ -e "$patch" ] || { echo "no mutations/*.patch files"; exit 2; }
    name=$(basename "$patch" .patch)
    test_name=$(sed -n '1s/^Test: //p' "$patch")
    if [ -z "$test_name" ]; then
        printf '| %s | (none) | STALE: no Test line |\n' "$name" >> "$report"
        unkilled=1
        continue
    fi
    if ! git apply "$patch"; then
        printf '| %s | %s | STALE: patch does not apply |\n' "$name" "$test_name" >> "$report"
        unkilled=1
        continue
    fi
    echo "::group::$name ($test_name)"
    if make -C desktop test ONLY="$test_name"; then
        result="SURVIVED"
        unkilled=1
    else
        result="KILLED"
    fi
    echo "::endgroup::"
    git checkout -- desktop
    printf '| %s | %s | %s |\n' "$name" "$test_name" "$result" >> "$report"
done

cat "$report"
exit "$unkilled"

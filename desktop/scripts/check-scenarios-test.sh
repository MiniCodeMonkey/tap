#!/bin/sh
# Checks check-scenarios.sh against a small tree of its own.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/check-scenarios.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT

mkdir -p "$root/docs/superpowers/specs/tap-desktop-features" "$root/desktop/TapTests"
cat > "$root/docs/superpowers/specs/tap-desktop-features/01-documents.feature" <<'FEATURE'
Feature: Documents

  Scenario: Open a deck
    When I open it
FEATURE
cat > "$root/desktop/scenarios.txt" <<'MANIFEST'
# a comment
D2 | 01-documents.feature | Open a deck
MANIFEST
cat > "$root/desktop/TapTests/DocumentTests.swift" <<'SWIFT'
func testOpenADeck() {}
SWIFT

"$script" "$root" >/dev/null || { echo "a complete tree should pass"; exit 1; }

mv "$root/desktop/TapTests/DocumentTests.swift" "$root/desktop/TapTests/Other.txt"
if "$script" "$root" >/dev/null 2>&1; then
	echo "a missing test should fail the check"
	exit 1
fi

# A Go test named after the scenario satisfies it too.
mkdir -p "$root/internal/cli"
cat > "$root/internal/cli/scenario_test.go" <<'GO'
func TestOpenADeck(t *testing.T) {}
GO
"$script" "$root" >/dev/null || { echo "a Go test should satisfy a claimed scenario"; exit 1; }
rm "$root/internal/cli/scenario_test.go"

cat > "$root/desktop/TapTests/DocumentTests.swift" <<'SWIFT'
func testOpenADeck() {}
SWIFT
cat > "$root/desktop/scenarios.txt" <<'MANIFEST'
D2 | 01-documents.feature | Open a deck that was renamed
MANIFEST
if "$script" "$root" >/dev/null 2>&1; then
	echo "a scenario the feature file does not have should fail the check"
	exit 1
fi

echo "check-scenarios.sh behaves"

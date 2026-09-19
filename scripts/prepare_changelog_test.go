// Package scripts holds Go tests for the shell scripts under scripts/ that
// .github/workflows/release.yml calls into, so their logic can be tested
// without triggering an actual release.
package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runPrepareChangelog runs scripts/prepare-changelog.sh against changelog
// (written to a temp file) for version, with CHANGELOG_DATE pinned so the
// fixture does not depend on today's date. It returns the script's
// standard error, the resulting changelog contents, and its exit error (nil
// on success).
func runPrepareChangelog(t *testing.T, changelog, version string) (stderr, changelogAfter string, runErr error) {
	t.Helper()

	dir := t.TempDir()
	changelogPath := filepath.Join(dir, "CHANGELOG.md")
	notesPath := filepath.Join(dir, "release_notes.md")
	if err := os.WriteFile(changelogPath, []byte(changelog), 0644); err != nil {
		t.Fatalf("failed to write changelog fixture: %v", err)
	}

	scriptPath, err := filepath.Abs("prepare-changelog.sh")
	if err != nil {
		t.Fatalf("failed to resolve script path: %v", err)
	}

	cmd := exec.Command("bash", scriptPath, version, changelogPath, notesPath)
	cmd.Env = append(os.Environ(), "CHANGELOG_DATE=2026-01-01")
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	runErr = cmd.Run()

	changelogBytes, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("failed to read changelog after run: %v", err)
	}

	if runErr == nil {
		notesBytes, err := os.ReadFile(notesPath)
		if err != nil {
			t.Fatalf("failed to read release notes: %v", err)
		}
		t.Logf("release notes:\n%s", notesBytes)
	}

	return errBuf.String(), string(changelogBytes), runErr
}

// TestPrepareChangelog_NormalFirstRun checks the ordinary case: notes sit
// under [Unreleased], and the script moves them under a new version header
// and leaves a fresh, empty [Unreleased] section behind.
func TestPrepareChangelog_NormalFirstRun(t *testing.T) {
	changelog := `# Changelog

## [Unreleased]

### Added

- New thing

## [1.1.0] - 2025-12-01

### Added

- Older thing
`

	_, after, err := runPrepareChangelog(t, changelog, "1.2.0")
	if err != nil {
		t.Fatalf("prepare-changelog.sh failed: %v", err)
	}

	if !strings.Contains(after, "## [1.2.0] - 2026-01-01") {
		t.Errorf("expected a new version header in the changelog, got:\n%s", after)
	}
	if !strings.Contains(after, "## [Unreleased]\n\n## [1.2.0]") {
		t.Errorf("expected a fresh empty [Unreleased] section right above the new header, got:\n%s", after)
	}
	if !strings.Contains(after, "## [1.1.0] - 2025-12-01") {
		t.Errorf("expected the older version section to survive untouched, got:\n%s", after)
	}
}

// TestPrepareChangelog_SecondRunForSameVersion checks the case this fix
// exists for: the workflow already ran once for 1.2.0 (its tag was then
// deleted) and is running again. [Unreleased] is empty now, but the
// changelog already carries a [1.2.0] section from the first run; the
// script must reuse that section's notes and leave the changelog alone,
// not publish empty notes or add a duplicate header.
func TestPrepareChangelog_SecondRunForSameVersion(t *testing.T) {
	changelog := `# Changelog

## [Unreleased]

## [1.2.0] - 2026-01-01

### Added

- New thing

## [1.1.0] - 2025-12-01

### Added

- Older thing
`

	stderr, after, err := runPrepareChangelog(t, changelog, "1.2.0")
	if err != nil {
		t.Fatalf("prepare-changelog.sh failed: %v (stderr: %s)", err, stderr)
	}

	if after != changelog {
		t.Errorf("expected the changelog to be left untouched on a second run, got:\n%s", after)
	}
	if !strings.Contains(stderr, "already has a section for 1.2.0") {
		t.Errorf("expected a note about reusing the existing section, got stderr: %s", stderr)
	}
}

// TestPrepareChangelog_EmptyUnreleasedFailsClearly checks that an empty
// [Unreleased] section, with no existing section for the version either,
// fails loudly instead of publishing "no changelog entries" and adding a
// header for a release with no notes.
func TestPrepareChangelog_EmptyUnreleasedFailsClearly(t *testing.T) {
	changelog := `# Changelog

## [Unreleased]

## [1.1.0] - 2025-12-01

### Added

- Older thing
`

	stderr, after, err := runPrepareChangelog(t, changelog, "1.2.0")
	if err == nil {
		t.Fatal("expected prepare-changelog.sh to fail on an empty [Unreleased] section")
	}
	if !strings.Contains(stderr, "Error:") {
		t.Errorf("expected a clear error message on stderr, got: %s", stderr)
	}
	if after != changelog {
		t.Errorf("expected the changelog to be left untouched on failure, got:\n%s", after)
	}
}

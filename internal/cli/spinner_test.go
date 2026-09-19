package cli

import (
	"os"
	"testing"
)

// TestSpinner_NoTerminalDrawsNothing checks that a spinner whose standard
// error is not a terminal (redirected to a file, piped, or running in CI)
// never starts its animation goroutine: start leaves it not running, and
// the standard error file it would have written frames to stays empty.
func TestSpinner_NoTerminalDrawsNothing(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "stderr-capture")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer tempFile.Close()

	originalStderr := os.Stderr
	os.Stderr = tempFile
	defer func() { os.Stderr = originalStderr }()

	s := newSpinner("Working")
	s.isTerminal = func() bool { return false }

	s.start()
	if s.running {
		t.Error("expected the spinner not to be running when standard error is not a terminal")
	}
	s.update("Still working")
	s.stop()

	if info, err := tempFile.Stat(); err != nil {
		t.Fatalf("failed to stat captured stderr: %v", err)
	} else if info.Size() != 0 {
		t.Errorf("expected no spinner output when standard error is not a terminal, got %d bytes", info.Size())
	}
}

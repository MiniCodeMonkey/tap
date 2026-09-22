package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantExitCode int
		wantCode     string
		wantReported bool
	}{
		{"plain error is a user error", errors.New("boom"), exitUserError, codeFailed, false},
		{"user error", userError(codeNoDeck, errors.New("no deck")), exitUserError, codeNoDeck, false},
		{"wrapped user error keeps its code", fmt.Errorf("loading: %w", userError(codeInvalidDeck, errors.New("bad"))), exitUserError, codeInvalidDeck, false},
		{"internal error", internalError(codeBrowser, errors.New("no chromium")), exitInternal, codeBrowser, false},
		{"reported error", reportedError(codeComponentBuild, errors.New("2 components failed")), exitUserError, codeComponentBuild, true},
		{"interrupt", errInterrupted, exitInterrupted, codeInterrupted, false},
		{"cancelled picker", errCancelled, exitInterrupted, codeCancelled, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode, code, reported := classify(tt.err)
			if exitCode != tt.wantExitCode || code != tt.wantCode || reported != tt.wantReported {
				t.Errorf("classify() = (%d, %q, %v), want (%d, %q, %v)",
					exitCode, code, reported, tt.wantExitCode, tt.wantCode, tt.wantReported)
			}
		})
	}
}

func TestExecuteUnknownFlagIsAUsageError(t *testing.T) {
	exitCode, stdout, stderr := runTap(t, "build", "--no-such-flag")
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "unknown flag: --no-such-flag") {
		t.Errorf("stderr = %q, want it to name the unknown flag", stderr)
	}
}

func TestVerboseFlagIsRemoved(t *testing.T) {
	if rootCmd.PersistentFlags().Lookup("verbose") != nil {
		t.Error("the global --verbose flag should be removed")
	}
}

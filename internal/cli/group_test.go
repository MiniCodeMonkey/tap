package cli

import (
	"strings"
	"testing"
)

// TestGroupCommand_UnknownSubcommandFails checks that a typo'd
// subcommand of a group command (export, slide, component, theme) fails
// instead of cobra's default of printing help and exiting 0.
func TestGroupCommand_UnknownSubcommandFails(t *testing.T) {
	for _, group := range []string{"export", "slide", "component", "theme"} {
		exitCode, _, stderr := runTap(t, group, "pfd")
		if exitCode != exitUserError {
			t.Errorf("tap %s pfd: exit = %d, want %d", group, exitCode, exitUserError)
		}
		if !strings.Contains(stderr, "pfd") {
			t.Errorf("tap %s pfd: stderr = %q, want it to name pfd", group, stderr)
		}
	}
}

// TestGroupCommand_NoArgsPrintsHelpAndExitsOK checks that a group command
// on its own, with no subcommand, still prints help and exits 0.
func TestGroupCommand_NoArgsPrintsHelpAndExitsOK(t *testing.T) {
	for _, group := range []string{"export", "slide", "component", "theme"} {
		exitCode, stdout, stderr := runTap(t, group)
		if exitCode != exitOK {
			t.Errorf("tap %s: exit = %d, want %d, stderr = %q", group, exitCode, exitOK, stderr)
		}
		if !strings.Contains(stdout, "Usage:") {
			t.Errorf("tap %s: stdout = %q, want it to contain help", group, stdout)
		}
	}
}

package cli

import (
	"os"
	"strings"
	"testing"
)

// TestUnwritableDeckExitsTheSameWayFromEveryCommand proves the four
// commands that write a deck agree on the exit code and --json error code
// when the deck's folder cannot be written: theme set and slide add
// already used codeInvalidDeck (exit 1), and image generate and image
// regenerate now match them instead of exiting 2 as codeInternal.
func TestUnwritableDeckExitsTheSameWayFromEveryCommand(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, which ignores directory permissions")
	}

	tests := []struct {
		name string
		args func(deck string) []string
	}{
		{
			name: "theme set",
			args: func(deck string) []string { return []string{"theme", "set", "terminal", deck, "--json"} },
		},
		{
			name: "slide add",
			args: func(deck string) []string { return []string{"slide", "add", deck, "--layout", "quote", "--json"} },
		},
		{
			name: "image generate",
			args: func(deck string) []string {
				return []string{"image", "generate", deck, "--slide", "1", "--prompt", "a cat", "--json"}
			},
		},
		{
			name: "image regenerate",
			args: func(deck string) []string {
				return []string{"image", "regenerate", deck, "--slide", "1", "--image", "images/old.png", "--json"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useFakeImageGenerator(t)

			dir := t.TempDir()
			content := "# One\n\n<!-- ai-prompt: old -->\n![](images/old.png)\n"
			deck := writeDeckFile(t, dir, "talk.md", content)

			if err := os.Chmod(dir, 0o500); err != nil {
				t.Fatalf("Failed to chmod dir: %v", err)
			}
			t.Cleanup(func() { os.Chmod(dir, 0o755) })

			exitCode, stdout, stderr := runTap(t, tt.args(deck)...)
			if exitCode != exitUserError {
				t.Errorf("exit code = %d, want %d (exitUserError): stdout %q stderr %q", exitCode, exitUserError, stdout, stderr)
			}
			if !strings.Contains(stdout, `"code": "`+codeInvalidDeck+`"`) {
				t.Errorf("stdout = %q, want the %q error code", stdout, codeInvalidDeck)
			}
		})
	}
}

package cli

import "testing"

func TestOldCommandNamesPrintTheNewName(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"pdf", "deck.md", "-o", "x.pdf"}, "tap pdf was renamed: use tap export pdf\n"},
		{[]string{"screenshot", "deck.md", "--slide", "3", "--out", "x.png"}, "tap screenshot was renamed: use tap export images\n"},
		{[]string{"add"}, "tap add was renamed: use tap slide add\n"},
		{[]string{"add", "deck.md"}, "tap add was renamed: use tap slide add\n"},
		{[]string{"add", "component", "RollingDeploy", "--inline"}, "tap add component was renamed: use tap component new\n"},
		{[]string{"pdf", "--help"}, "tap pdf was renamed: use tap export pdf\n"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			exitCode, stdout, stderr := runTap(t, tt.args...)
			if exitCode != exitUserError {
				t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if stderr != tt.want {
				t.Errorf("stderr = %q, want %q", stderr, tt.want)
			}
		})
	}
}

package cli

import "testing"

func TestDisplayVersion(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "dev"
	if got := displayVersion(); got != "dev" {
		t.Errorf("displayVersion() = %q for a local build, want %q", got, "dev")
	}

	Version = "2.0.0-beta.2"
	if got := displayVersion(); got != "v2.0.0-beta.2" {
		t.Errorf("displayVersion() = %q for a release build, want %q", got, "v2.0.0-beta.2")
	}
}

// TestPresentCommandIsRegistered checks that tap present exists with the
// flags it needs and none of the dev-only flags that would not make sense
// for a talk.
func TestPresentCommandIsRegistered(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"present"})
	if err != nil || command.Name() != "present" {
		t.Fatalf("present command not found: %v", err)
	}
	for _, flag := range []string{"port", "no-record"} {
		if command.Flags().Lookup(flag) == nil {
			t.Errorf("present lacks --%s", flag)
		}
	}
	for _, flag := range []string{"headless", "tunnel"} {
		if command.Flags().Lookup(flag) != nil {
			t.Errorf("present should not have --%s", flag)
		}
	}
}

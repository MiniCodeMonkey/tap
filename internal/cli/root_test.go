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

package cli

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points the user settings at a temporary folder, so no test
// reads or writes the settings.yaml of the person running the tests.
// Subprocesses inherit it through os.Environ.
func TestMain(m *testing.M) {
	configHome, err := os.MkdirTemp("", "tap-cli-test-config-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.Setenv("XDG_CONFIG_HOME", configHome); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(configHome)
	os.Exit(code)
}

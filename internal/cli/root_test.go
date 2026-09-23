package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

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
	for _, flag := range []string{"port", "no-record", "lan", "allow-code"} {
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

func TestDevHasTheLANFlag(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"dev"})
	if err != nil {
		t.Fatal(err)
	}
	if command.Flags().Lookup("lan") == nil {
		t.Error("dev lacks --lan")
	}
}

func TestDevHasTheAllowCodeFlag(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"dev"})
	if err != nil {
		t.Fatal(err)
	}
	if command.Flags().Lookup("allow-code") == nil {
		t.Error("dev lacks --allow-code")
	}
}

func TestDeckCommandsTakeAnOptionalDeck(t *testing.T) {
	for _, path := range [][]string{{"dev"}, {"present"}, {"build"}, {"new"}} {
		command, _, err := rootCmd.Find(path)
		if err != nil {
			t.Fatalf("%v not found: %v", path, err)
		}
		want := path[0] + " [deck]"
		if command.Use != want {
			t.Errorf("Use = %q, want %q", command.Use, want)
		}
	}
}

func TestBuildMissingDeckJSON(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "build", filepath.Join(t.TempDir(), "missing.md"), "--json")
	if exitCode != exitUserError || !strings.Contains(stdout, `"code": "deck_not_found"`) {
		t.Errorf("exit %d, stdout %q", exitCode, stdout)
	}
}

func TestDevHasTheAppFlag(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"dev"})
	if err != nil {
		t.Fatal(err)
	}
	if command.Flags().Lookup("app") == nil {
		t.Error("dev lacks --app")
	}
}

func TestDevAppRejectsFlagsThatDoNotFit(t *testing.T) {
	deck := copyAppFixture(t)
	for _, args := range [][]string{
		{"dev", "--app"},
		{"dev", "--app", deck, "--headless"},
		{"dev", "--app", deck, "--lan"},
	} {
		exitCode, stdout, stderr := runTap(t, args...)
		if exitCode != exitUserError || stdout != "" {
			t.Errorf("tap %v: exit %d, stdout %q, stderr %q; want exit 1 and nothing on stdout", args, exitCode, stdout, stderr)
		}
	}
}

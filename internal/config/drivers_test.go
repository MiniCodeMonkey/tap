package config

import (
	"os"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDeclaredDrivers(t *testing.T) {
	cfg := DefaultConfig()
	if got := cfg.DeclaredDrivers(); got == nil || len(got) != 0 {
		t.Errorf("DeclaredDrivers() = %#v, want an empty list", got)
	}
	cfg.Drivers = map[string]DriverConfig{"sqlite": {}, "shell": {}}
	if got := strings.Join(cfg.DeclaredDrivers(), ","); got != "shell,sqlite" {
		t.Errorf("DeclaredDrivers() = %q, want shell,sqlite", got)
	}
	if !cfg.DriverDeclared("shell") || cfg.DriverDeclared("python") {
		t.Error("DriverDeclared() is wrong")
	}
}

func TestUndeclaredDriverMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Drivers = map[string]DriverConfig{"sqlite": {}}
	want := `This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter.`
	if got := cfg.UndeclaredDriverMessage("shell", []string{"shell", "sqlite"}); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestUndeclaredDriverMessageWithNoDriversShowsTheBlock(t *testing.T) {
	cfg := DefaultConfig()
	want := "This deck does not declare the sqlite driver. Add this to the frontmatter:\n\ndrivers:\n  shell: {}\n  sqlite: {}"
	if got := cfg.UndeclaredDriverMessage("sqlite", []string{"sqlite", "shell"}); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestLoadAcceptsADriverWithNoSettings(t *testing.T) {
	path := t.TempDir() + "/deck.md"
	writeFile(t, path, "---\ndrivers:\n  shell: {}\n---\n\n# One\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DriverDeclared("shell") {
		t.Error(`"shell: {}" should declare the shell driver`)
	}
}

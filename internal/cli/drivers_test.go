package cli

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/driver"
)

func TestBuildDriverRegistryHasTheBuiltInDrivers(t *testing.T) {
	registry := buildDriverRegistry(&config.Config{}, t.TempDir())
	names := registry.List()
	sort.Strings(names)
	if strings.Join(names, ",") != "mysql,postgres,shell,sqlite" {
		t.Errorf("drivers = %v, want the four built-in drivers", names)
	}
}

func TestBuildDriverRegistryAddsCustomDrivers(t *testing.T) {
	cfg := &config.Config{Drivers: map[string]config.DriverConfig{
		"python":  {Command: "python3", Args: []string{"-"}},
		"sqlite":  {Connections: map[string]config.ConnectionConfig{"demo": {Path: "demo.db"}}},
		"nothing": {},
	}}
	registry := buildDriverRegistry(cfg, t.TempDir())
	if !registry.Has("python") {
		t.Error("the custom python driver is missing")
	}
	if registry.Has("nothing") {
		t.Error("a drivers: entry with no command is not a custom driver")
	}
}

func TestBuildDriverRegistryExpandsACustomCommand(t *testing.T) {
	t.Setenv("TAP_TEST_INTERPRETER", "python3")
	cfg := &config.Config{Drivers: map[string]config.DriverConfig{
		"python": {Command: "${TAP_TEST_INTERPRETER}", Args: []string{"-c"}},
	}}
	registry := buildDriverRegistry(cfg, t.TempDir())
	custom, ok := registry.Get("python").(*driver.CustomDriver)
	if !ok {
		t.Fatalf("python is %T, want *driver.CustomDriver", registry.Get("python"))
	}
	if custom.Command != "python3" {
		t.Errorf("command = %q, want python3", custom.Command)
	}
}

func TestBuildDriverRegistryFailsABlockOnAnUnsetCommandVariable(t *testing.T) {
	t.Setenv("TAP_TEST_UNSET_INTERPRETER", "")
	os.Unsetenv("TAP_TEST_UNSET_INTERPRETER")
	cfg := &config.Config{Drivers: map[string]config.DriverConfig{
		"python": {Command: "${TAP_TEST_UNSET_INTERPRETER}"},
	}}
	registry := buildDriverRegistry(cfg, t.TempDir())
	if !registry.Has("python") {
		t.Fatal("python is missing: its blocks must fail with the reason, not with driver not found")
	}
	result := registry.Execute(context.Background(), "python", "print(1)", map[string]string{})
	if result.Success || !strings.Contains(result.Error, "TAP_TEST_UNSET_INTERPRETER is not set") {
		t.Errorf("result = %+v, want the unset variable named", result)
	}
}

func TestBuildDriverRegistryKeepsTheBuiltInShell(t *testing.T) {
	t.Setenv("TAP_TEST_UNSET_SHELL", "")
	os.Unsetenv("TAP_TEST_UNSET_SHELL")
	cfg := &config.Config{Drivers: map[string]config.DriverConfig{
		"shell": {Command: "${TAP_TEST_UNSET_SHELL}"},
	}}
	registry := buildDriverRegistry(cfg, t.TempDir())
	result := registry.Execute(context.Background(), "shell", "echo built in", map[string]string{})
	if !result.Success || !strings.Contains(result.Output, "built in") {
		t.Errorf("shell = %+v, want the built-in shell driver", result)
	}
}

func TestBuildDriverRegistryRunsInTheDeckFolder(t *testing.T) {
	deckFolder := t.TempDir()
	cfg := &config.Config{Drivers: map[string]config.DriverConfig{
		"pwd": {Command: "pwd"},
	}}
	registry := buildDriverRegistry(cfg, deckFolder)
	for _, name := range []string{"shell", "pwd"} {
		code := "pwd"
		if name == "pwd" {
			code = ""
		}
		result := registry.Execute(context.Background(), name, code, map[string]string{})
		if !result.Success || !strings.HasSuffix(strings.TrimSpace(result.Output), deckFolder) {
			t.Errorf("%s ran in %q, want the deck folder %q (error %q)", name, result.Output, deckFolder, result.Error)
		}
	}
}

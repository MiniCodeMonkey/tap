package cli

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
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

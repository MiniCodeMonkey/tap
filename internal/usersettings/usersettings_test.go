package usersettings

import (
	"path/filepath"
	"testing"
)

func TestPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/config-home")

	path, err := Path()
	if err != nil || path != "/tmp/config-home/tap/settings.yaml" {
		t.Errorf("Path() = %q, %v", path, err)
	}
}

func TestPathFallsBackToDotConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/Users/speaker")

	path, err := Path()
	if err != nil || path != "/Users/speaker/.config/tap/settings.yaml" {
		t.Errorf("Path() = %q, %v", path, err)
	}
}

func TestLoadAMissingFileIsEmpty(t *testing.T) {
	settings, err := Load(filepath.Join(t.TempDir(), "settings.yaml"))
	if err != nil || settings.Present.Record != nil {
		t.Errorf("Load = %+v, %v; want empty settings", settings, err)
	}
}

func TestSaveThenLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tap", "settings.yaml")
	record := true

	if err := Save(path, Settings{Present: Present{Record: &record}}); err != nil {
		t.Fatal(err)
	}
	settings, err := Load(path)
	if err != nil || settings.Present.Record == nil || !*settings.Present.Record {
		t.Errorf("Load = %+v, %v; want record: true", settings, err)
	}
}

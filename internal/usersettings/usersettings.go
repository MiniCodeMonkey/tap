// Package usersettings reads and writes the settings that belong to the
// person running Tap rather than to a deck, such as consent to record.
package usersettings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Settings is the whole settings file.
type Settings struct {
	Present Present `yaml:"present,omitempty"`
}

// Present holds tap present settings. A nil Record means the speaker has
// not been asked yet.
type Present struct {
	Record *bool `yaml:"record,omitempty"`
}

// Path is settings.yaml in the Tap user config directory:
// $XDG_CONFIG_HOME/tap, or ~/.config/tap when that is unset.
func Path() (string, error) {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "tap", "settings.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding the home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tap", "settings.yaml"), nil
}

// Load reads the settings. A missing file is empty settings.
func Load(path string) (Settings, error) {
	var settings Settings
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := yaml.Unmarshal(raw, &settings); err != nil {
		return settings, fmt.Errorf("reading %s: %w", path, err)
	}
	return settings, nil
}

// Save writes the settings, creating the directory.
func Save(path string, settings Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	raw, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// Package usersettings reads and writes the settings that belong to the
// person running Tap rather than to a deck, such as consent to record and
// the decks they allowed to run live code.
package usersettings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"gopkg.in/yaml.v3"
)

// Settings is the whole settings file.
type Settings struct {
	Present   Present    `yaml:"present,omitempty"`
	Approvals []Approval `yaml:"approvals,omitempty"`
}

// Approval is a deck the person allowed to run live code, with the drivers
// they allowed. Deck is an absolute path, so a moved deck is a new deck
// and is asked about again.
//
//nolint:govet // fieldalignment: field order is the settings file order
type Approval struct {
	Deck       string    `yaml:"deck" json:"deck"`
	Drivers    []string  `yaml:"drivers,flow" json:"drivers"`
	ApprovedAt time.Time `yaml:"approvedAt" json:"approvedAt"`
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

// ApprovalFor returns the approval stored for deck.
func (s Settings) ApprovalFor(deck DeckKey) (Approval, bool) {
	for _, approval := range s.Approvals {
		if approval.Deck == deck.path {
			return approval, true
		}
	}
	return Approval{}, false
}

// Approved reports whether deck is approved for every driver in drivers.
func (s Settings) Approved(deck DeckKey, drivers []string) bool {
	approval, found := s.ApprovalFor(deck)
	if !found {
		return false
	}
	for _, name := range drivers {
		if !slices.Contains(approval.Drivers, name) {
			return false
		}
	}
	return true
}

// Approve records that deck may run drivers. An approval already stored
// for the deck keeps its drivers and gains the new ones.
func (s *Settings) Approve(deck DeckKey, drivers []string, at time.Time) {
	merged := append([]string{}, drivers...)
	if existing, found := s.ApprovalFor(deck); found {
		merged = append(merged, existing.Drivers...)
	}
	s.Revoke(deck)
	s.Approvals = append(s.Approvals, Approval{
		Deck:       deck.path,
		Drivers:    uniqueSorted(merged),
		ApprovedAt: at.UTC().Truncate(time.Second),
	})
}

// Revoke removes the approval for deck, and reports whether there was
// one. Because deck is a DeckKey, this only ever revokes a deck that
// still exists on disk; for one that does not, see RevokeStoredPath.
func (s *Settings) Revoke(deck DeckKey) bool {
	return s.RevokeStoredPath(deck.path)
}

// RevokeStoredPath removes the approval whose stored path is exactly
// deck, without resolving it, and reports whether there was one. This is
// the escape hatch from DeckKey: ResolveDeck cannot produce a key for a
// deck that no longer exists on disk, so revoking that approval has
// nothing to key by except the path already on file. A caller uses this
// only for a deck it cannot resolve, by copying the exact path
// Settings.Approvals (or tap approval list) prints; ordinary revocation
// goes through ResolveDeck and Revoke.
func (s *Settings) RevokeStoredPath(deck string) bool {
	kept := make([]Approval, 0, len(s.Approvals))
	removed := false
	for _, approval := range s.Approvals {
		if approval.Deck == deck {
			removed = true
			continue
		}
		kept = append(kept, approval)
	}
	s.Approvals = kept
	return removed
}

// uniqueSorted returns names sorted, without repeats, and never nil.
func uniqueSorted(names []string) []string {
	sorted := append([]string{}, names...)
	slices.Sort(sorted)
	return slices.Compact(sorted)
}

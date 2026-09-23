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

// Save writes the settings, creating the directory. It writes to a
// temporary file in the same directory and renames it into place, so a
// reader, or another tap process saving at the same time, always sees
// either the old file or the whole new one, never a truncated or
// interleaved one: os.WriteFile truncates the file before writing it, so
// two processes calling it at once could leave a corrupt file, or one
// could read it mid-write. The new file keeps the existing file's
// permissions, or 0o600 when there is no existing file yet.
func Save(path string, settings Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	raw, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	temp, err := os.CreateTemp(dir, ".settings-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath) // no-op once the rename below succeeds
	if _, err := temp.Write(raw); err != nil {
		temp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// lockSuffix names the sibling file WithLock uses to serialize a
// load-modify-save sequence against another tap process doing the same.
const lockSuffix = ".lock"

// lockRetryInterval and lockAcquireTimeout bound how long WithLock waits
// to acquire the settings lock before giving fn the settings unlocked. A
// load-modify-save normally holds the lock for a fraction of a second, so
// this is generous for real contention; a lock file left behind by a tap
// process that crashed while holding it must not wedge every later run
// forever, so a caller that still cannot acquire it after the timeout
// proceeds anyway; losing an approval to a decade-old stale lock file is
// better than tap refusing to start.
const (
	lockRetryInterval  = 10 * time.Millisecond
	lockAcquireTimeout = 2 * time.Second
)

// WithLock runs fn, which is expected to Load, modify and Save the
// settings at path, while holding an exclusive, cross-process lock on
// path. Without it, two tap processes approving, revoking or recording
// consent at nearly the same moment can each load the settings before
// the other saves, so the second Save silently discards the first
// process's change: Save itself writes one file correctly either way,
// but there is nothing to stop two correct writes from racing on stale
// data. WithLock closes that window by making the whole sequence run
// one process at a time.
func WithLock(path string, fn func() error) error {
	lockPath := path + lockSuffix
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fn()
	}
	deadline := time.Now().Add(lockAcquireTimeout)
	for {
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			defer func() {
				lock.Close()
				os.Remove(lockPath)
			}()
			break
		}
		if !os.IsExist(err) || time.Now().After(deadline) {
			break
		}
		time.Sleep(lockRetryInterval)
	}
	return fn()
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

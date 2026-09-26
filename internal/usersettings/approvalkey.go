package usersettings

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// approvalKeySize is the length of the approval key in bytes.
const approvalKeySize = 32

// errInvalidApprovalKey is what LoadApprovalKey returns for a key file of
// the wrong length.
var errInvalidApprovalKey = errors.New("the approval key has the wrong length")

// ApprovalKeyPath is approval.key next to the settings file at
// settingsPath. It holds the random key CommandDigest is keyed with, kept
// apart from settings.yaml so a copy of the settings, in a dotfiles
// repository or a backup, reveals nothing about the commands approved.
func ApprovalKeyPath(settingsPath string) string {
	return filepath.Join(filepath.Dir(settingsPath), "approval.key")
}

// LoadApprovalKey reads the approval key. It returns an error when there
// is none yet or it cannot be read, and then no stored digest can match,
// so every custom driver is asked about again.
func LoadApprovalKey(settingsPath string) ([]byte, error) {
	key, err := os.ReadFile(ApprovalKeyPath(settingsPath))
	if err != nil {
		return nil, err
	}
	if len(key) != approvalKeySize {
		return nil, errInvalidApprovalKey
	}
	return key, nil
}

// EnsureApprovalKey returns the approval key, making a random one, with
// mode 0600, when there is none or the one there has the wrong length. A
// key that exists but cannot be read is an error: replacing it would
// only hide the problem.
func EnsureApprovalKey(settingsPath string) ([]byte, error) {
	key, err := LoadApprovalKey(settingsPath)
	if err == nil {
		return key, nil
	}
	replace := errors.Is(err, errInvalidApprovalKey)
	if !replace && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	path := ApprovalKeyPath(settingsPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	key = make([]byte, approvalKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".approval-*.key.tmp")
	if err != nil {
		return nil, fmt.Errorf("writing %s: %w", path, err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath) // no-op once the link below succeeds
	if _, err := temp.Write(key); err != nil {
		temp.Close()
		return nil, fmt.Errorf("writing %s: %w", path, err)
	}
	if err := temp.Close(); err != nil {
		return nil, fmt.Errorf("writing %s: %w", path, err)
	}
	if replace {
		if err := os.Rename(tempPath, path); err != nil {
			return nil, fmt.Errorf("writing %s: %w", path, err)
		}
		return key, nil
	}
	// Link rather than rename, so a key another tap process made in the
	// meantime is kept and used instead of replaced.
	if err := os.Link(tempPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return LoadApprovalKey(settingsPath)
		}
		return nil, fmt.Errorf("writing %s: %w", path, err)
	}
	return key, nil
}

// CommandDigest returns "hmac-sha256:" and the hex HMAC-SHA256, keyed
// with key, of argv as a JSON list, so argument boundaries count. It is
// empty for no command.
func CommandDigest(key []byte, argv []string) string {
	if argv == nil {
		return ""
	}
	encoded, _ := json.Marshal(argv)
	mac := hmac.New(sha256.New, key)
	mac.Write(encoded)
	return "hmac-sha256:" + hex.EncodeToString(mac.Sum(nil))
}

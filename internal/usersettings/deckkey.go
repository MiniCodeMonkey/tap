package usersettings

import (
	"os"
	"path/filepath"
	"strings"
)

// DeckKey identifies a deck by the resolved, canonical path its approval
// is compared against: absolute, with symlinks resolved, and, on a
// filesystem that treats case as insignificant, rewritten to the spelling
// each component actually has on disk. The zero value matches nothing
// that was ever approved. ResolveDeck is the only way to produce one, so
// Approved, Approve, ApprovalFor and Revoke can never be called with an
// unresolved path by mistake: the compiler refuses a bare string at the
// call site rather than relying on every caller to remember to resolve
// one first.
type DeckKey struct {
	path string
}

// String returns the resolved path a DeckKey carries, for display or for
// storing alongside an Approval.
func (k DeckKey) String() string {
	return k.path
}

// ResolveDeck resolves deck, a file that must exist, to the DeckKey its
// approval is stored and compared under. Two spellings of the same file,
// however different lexically, resolve to the same key: a symlink, a
// relative path, a path through "..", or a different case on a
// case-insensitive filesystem. Two different files resolve to different
// keys, even when they differ only by case on a case-sensitive
// filesystem. A deck that does not exist, or whose folder cannot be read,
// cannot be resolved at all, and ResolveDeck returns an error; the caller
// must treat that as not approved rather than approved by a coincidence
// of string matching.
func ResolveDeck(deck string) (DeckKey, error) {
	absolute, err := filepath.Abs(deck)
	if err != nil {
		return DeckKey{}, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return DeckKey{}, err
	}
	cased, err := onDiskCase(resolved)
	if err != nil {
		return DeckKey{}, err
	}
	return DeckKey{path: cased}, nil
}

// onDiskCase rewrites every component of resolved, an absolute path with
// symlinks already resolved, to the spelling its directory actually
// holds. filepath.EvalSymlinks does not do this: on a case-insensitive
// filesystem it happily resolves "TALK.MD" against a file named "talk.md"
// and hands back "TALK.MD" unchanged, so two spellings of the same deck
// would still compare unequal as strings. Reading each directory's real
// entries closes that gap without depending on anything
// platform-specific, and costs one os.ReadDir per path component, which
// is fine at the once-per-launch frequency ResolveDeck runs at.
func onDiskCase(resolved string) (string, error) {
	volume := filepath.VolumeName(resolved)
	rest := strings.TrimPrefix(resolved[len(volume):], string(filepath.Separator))
	current := volume + string(filepath.Separator)
	if rest == "" {
		return current, nil
	}
	for _, part := range strings.Split(rest, string(filepath.Separator)) {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", err
		}
		spelling := part
		for _, entry := range entries {
			if entry.Name() == part {
				spelling = part
				break
			}
			if strings.EqualFold(entry.Name(), part) {
				spelling = entry.Name()
			}
		}
		current = filepath.Join(current, spelling)
	}
	return current, nil
}

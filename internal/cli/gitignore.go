package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// gitignoreState reports whether a repository containing dir should be
// offered an ignore entry, and which .gitignore it would go in. It is
// quiet outside a repository and quiet when the entry is already there.
func gitignoreState(dir, entry string) (bool, string) {
	root, found := repositoryRoot(dir)
	if !found {
		return false, ""
	}

	gitignorePath := filepath.Join(root, ".gitignore")
	contents, err := os.ReadFile(gitignorePath) //nolint:gosec // a path built from the deck's own directory
	if err != nil {
		return true, gitignorePath
	}

	wanted := strings.TrimSuffix(entry, "/")
	for _, line := range strings.Split(string(contents), "\n") {
		if strings.TrimSuffix(strings.TrimSpace(line), "/") == wanted {
			return false, gitignorePath
		}
	}

	return true, gitignorePath
}

// repositoryRoot walks up from dir looking for a .git entry.
func repositoryRoot(dir string) (string, bool) {
	for current := dir; ; {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current, true
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

// appendGitignoreEntry adds an entry on its own line, leaving everything
// already in the file alone.
func appendGitignoreEntry(gitignorePath, entry string) error {
	contents, err := os.ReadFile(gitignorePath) //nolint:gosec // a path built from the deck's own directory
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	text := string(contents)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += entry + "\n"

	return os.WriteFile(gitignorePath, []byte(text), 0o600)
}

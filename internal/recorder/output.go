package recorder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// fallbackSlug names a recording whose deck has no usable title.
const fallbackSlug = "talk"

// OutputPath is where a recording started at start should be written. It
// creates dir if it is missing, and never returns a path that already
// exists: a second recording in the same minute gets a numeric suffix
// rather than overwriting the first.
func OutputPath(dir, deckTitle string, start time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating the recordings directory: %w", err)
	}

	base := fmt.Sprintf("%s-%s", slugify(deckTitle), start.Format("2006-01-02-1504"))

	candidate := filepath.Join(dir, base+".mov")
	for attempt := 2; ; attempt++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("checking the output path: %w", err)
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s-%d.mov", base, attempt))
	}
}

// RunDir creates the folder for one tap present run: the slugified deck
// title plus the local start time, with a numeric suffix when that folder
// already exists. It never reuses a folder.
func RunDir(parent, deckTitle string, start time.Time) (string, error) {
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("creating the recordings directory: %w", err)
	}

	base := filepath.Join(parent, fmt.Sprintf("%s-%s", slugify(deckTitle), start.Format("2006-01-02-1504")))
	candidate := base
	for attempt := 2; ; attempt++ {
		err := os.Mkdir(candidate, 0o755)
		if err == nil {
			return candidate, nil
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("creating the run folder: %w", err)
		}
		candidate = fmt.Sprintf("%s-%d", base, attempt)
	}
}

// ChapterPath is the sidecar chapter list for a recording.
func ChapterPath(moviePath string) string {
	return strings.TrimSuffix(moviePath, filepath.Ext(moviePath)) + ".txt"
}

// slugify reduces a deck title to lower-case words joined by hyphens, so it
// is safe in a filename on any platform.
func slugify(title string) string {
	var builder strings.Builder
	previousHyphen := false

	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			previousHyphen = false
		case !previousHyphen && builder.Len() > 0:
			builder.WriteRune('-')
			previousHyphen = true
		}
	}

	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return fallbackSlug
	}
	return slug
}

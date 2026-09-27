package cli

import (
	"context"
	"os"
	"time"
)

// settingsPollInterval is how often tap dev looks at its settings file.
// An approval another tap process stores counts within two of these.
const settingsPollInterval = 250 * time.Millisecond

// followSettings decides again whenever the settings file changes, until
// ctx ends. See settingsChanged.
func (gate *liveCodeGate) followSettings(ctx context.Context, interval time.Duration) {
	watchSettingsFile(ctx, gate.input.SettingsPath, interval, gate.settingsChanged)
}

// watchSettingsFile calls onChange, on this goroutine, each time the file
// at path changes, until ctx ends. It polls rather than using file system
// events: the file and its directory may not exist until the first
// approval, and a save replaces the file by a rename. A change is
// reported once the file has held still for one interval, so a burst of
// writes is one call; the file as it was when this started is never
// reported.
func watchSettingsFile(ctx context.Context, path string, interval time.Duration, onChange func()) {
	reported := statSettings(path)
	previous := reported
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		current := statSettings(path)
		if current.same(previous) && !current.same(reported) {
			reported = current
			onChange()
		}
		previous = current
	}
}

// settingsState is what watchSettingsFile compares between polls.
type settingsState struct {
	info os.FileInfo
}

func statSettings(path string) settingsState {
	info, err := os.Stat(path)
	if err != nil {
		return settingsState{}
	}
	return settingsState{info: info}
}

// same reports whether two observations are of the same file with the
// same size and modification time. A save renames a new file into place,
// so even a rewrite within the clock's resolution is a different file.
func (state settingsState) same(other settingsState) bool {
	if state.info == nil || other.info == nil {
		return state.info == nil && other.info == nil
	}
	return os.SameFile(state.info, other.info) && state.info.Size() == other.info.Size() &&
		state.info.ModTime().Equal(other.info.ModTime())
}

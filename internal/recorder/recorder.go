// Package recorder records the screen and microphone during a talk, using
// the macOS screencapture binary. Everything platform-specific lives in the
// darwin files; the types here are shared.
package recorder

import "time"

// Options describes one recording.
type Options struct {
	// Command is the recorder binary. Empty means "screencapture". Tests
	// point this at a helper process so the suite never records anything.
	Command string
	// OutputPath is the .mov the recorder writes.
	OutputPath string
	// AudioUID is a CoreAudio device UID. Empty records from the system
	// default input. Device names and indexes are not accepted by
	// screencapture, so nothing else belongs here.
	AudioUID string
	// Display is a screencapture display index, where 1 is the main
	// display. Zero is treated as 1.
	Display int
	// LimitSeconds caps the recording length. Zero means no cap. The test
	// capture uses it; a talk recording is stopped by the session timers,
	// so that one path handles both the warning and the stop.
	LimitSeconds int
	// NoAudio records silently.
	NoAudio bool
	// ShowClicks draws mouse clicks.
	ShowClicks bool
}

// Display is one attached screen, as the picker shows it.
type Display struct {
	// Name is the display's marketing name, or "" when it could not be
	// matched to an index (see displays_darwin.go).
	Name string
	// Resolution is a human string such as "3456 x 2234", or "".
	Resolution string
	// Index is the screencapture display index, starting at 1.
	Index int
	// Main reports whether this is the main display.
	Main bool
}

// Finding is one preflight result worth telling the speaker about.
type Finding struct {
	// Message says what is wrong.
	Message string
	// Fix says what to do about it, and is empty when there is nothing
	// useful to suggest.
	Fix string
	// Blocking marks a finding that stops a recording from starting.
	Blocking bool
}

// Report is everything the preflight found. An empty report is a pass.
type Report struct {
	Findings []Finding
}

// Blocked reports whether anything in the report stops a recording.
func (r Report) Blocked() bool {
	for _, finding := range r.Findings {
		if finding.Blocking {
			return true
		}
	}
	return false
}

// Result describes a finished recording.
type Result struct {
	// Path is the .mov.
	Path string
	// ChapterPath is the sidecar chapter list, or "" when chapters are off.
	ChapterPath string
	// Duration is how long the recorder ran.
	Duration time.Duration
	// Size is the .mov's size in bytes.
	Size int64
	// Truncated marks a file the recorder had to be killed to end, which
	// may be incomplete.
	Truncated bool
}

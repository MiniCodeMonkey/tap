//go:build !darwin

package recorder

import "errors"

// errUnsupported is what every entry point returns away from macOS.
var errUnsupported = errors.New("recording is macOS only")

// Supported reports whether this platform can record.
func Supported() bool { return false }

// Displays lists the screens that can be recorded, of which there are none
// here.
func Displays() ([]Display, error) { return nil, errUnsupported }

// DefaultAudioInput names the microphone a recording would use.
func DefaultAudioInput() string { return "" }

// ValidateAudioUID reports whether the recorder accepts a device id.
func ValidateAudioUID(string) error { return errUnsupported }

// Preflight reports that recording cannot run here at all.
func Preflight(string, Options) Report {
	return Report{Findings: []Finding{{
		Message:  "Recording is macOS only",
		Fix:      "Record with a screen recorder for this platform instead",
		Blocking: true,
	}}}
}

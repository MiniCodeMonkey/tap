package cli

import (
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/layouts"
)

// TestDropComponentBuildFailureWarnings verifies that a "component ...
// failed to build" warning is removed (it is already printed as its own
// "error:" line elsewhere), while an unrelated warning, such as an unknown
// layout, is kept.
func TestDropComponentBuildFailureWarnings(t *testing.T) {
	warnings := []layouts.Warning{
		{SlideNumber: 1, Message: `component "./slides/Broken.jsx" failed to build: syntax error`},
		{SlideNumber: 2, Message: `unknown layout "nope" (valid layouts: default, title)`},
	}

	filtered := dropComponentBuildFailureWarnings(warnings)

	if len(filtered) != 1 {
		t.Fatalf("dropComponentBuildFailureWarnings() returned %d warnings, want 1: %+v", len(filtered), filtered)
	}
	if filtered[0].SlideNumber != 2 {
		t.Errorf("SlideNumber = %d, want 2 (the unrelated warning)", filtered[0].SlideNumber)
	}
}

// TestRecordingAudioOptions covers the config-to-controller mapping that
// Critical 1 lived in: "none" must record silently rather than being
// validated as an unknown CoreAudio device UID, which used to block every
// silent recording outright.
func TestRecordingAudioOptions(t *testing.T) {
	tests := []struct {
		name        string
		configured  string
		wantUID     string
		wantNoAudio bool
	}{
		{"none records silently", "none", "", true},
		{"default uses the system input", "default", "", false},
		{"empty is the same as default", "", "", false},
		{"anything else is a CoreAudio UID", "BuiltInMicrophoneDevice", "BuiltInMicrophoneDevice", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid, noAudio := recordingAudioOptions(tt.configured)
			if uid != tt.wantUID || noAudio != tt.wantNoAudio {
				t.Errorf("recordingAudioOptions(%q) = (%q, %v), want (%q, %v)",
					tt.configured, uid, noAudio, tt.wantUID, tt.wantNoAudio)
			}
		})
	}
}

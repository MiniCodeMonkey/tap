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

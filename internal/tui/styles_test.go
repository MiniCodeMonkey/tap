package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestColorConstants(t *testing.T) {
	// Verify color constants are defined correctly
	tests := []struct {
		name     string
		color    lipgloss.Color
		expected string
	}{
		{"Primary", ColorPrimary, "#7C3AED"},
		{"Secondary", ColorSecondary, "#10B981"},
		{"Error", ColorError, "#EF4444"},
		{"Muted", ColorMuted, "#6B7280"},
		{"White", ColorWhite, "#FFFFFF"},
		{"Black", ColorBlack, "#000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.color) != tt.expected {
				t.Errorf("Color%s = %q, want %q", tt.name, string(tt.color), tt.expected)
			}
		})
	}
}

func TestRenderTitle(t *testing.T) {
	result := RenderTitle("Test Title")
	// Should contain the text
	if !strings.Contains(result, "Test Title") {
		t.Errorf("RenderTitle should contain the text, got %q", result)
	}
	// Result should be non-empty (styled)
	if len(result) == 0 {
		t.Error("RenderTitle should return non-empty string")
	}
}

func TestRenderSubtitle(t *testing.T) {
	result := RenderSubtitle("Subtitle Text")
	if !strings.Contains(result, "Subtitle Text") {
		t.Errorf("RenderSubtitle should contain the text, got %q", result)
	}
	if len(result) == 0 {
		t.Error("RenderSubtitle should return non-empty string")
	}
}

func TestRenderError(t *testing.T) {
	result := RenderError("Error Message")
	if !strings.Contains(result, "Error Message") {
		t.Errorf("RenderError should contain the text, got %q", result)
	}
	if len(result) == 0 {
		t.Error("RenderError should return non-empty string")
	}
}

func TestRenderSuccess(t *testing.T) {
	result := RenderSuccess("Success!")
	if !strings.Contains(result, "Success!") {
		t.Errorf("RenderSuccess should contain the text, got %q", result)
	}
	if len(result) == 0 {
		t.Error("RenderSuccess should return non-empty string")
	}
}

func TestRenderMuted(t *testing.T) {
	result := RenderMuted("Secondary text")
	if !strings.Contains(result, "Secondary text") {
		t.Errorf("RenderMuted should contain the text, got %q", result)
	}
	if len(result) == 0 {
		t.Error("RenderMuted should return non-empty string")
	}
}

func TestRenderHighlight(t *testing.T) {
	result := RenderHighlight("Important")
	if !strings.Contains(result, "Important") {
		t.Errorf("RenderHighlight should contain the text, got %q", result)
	}
	if len(result) == 0 {
		t.Error("RenderHighlight should return non-empty string")
	}
}

func TestRenderHelp(t *testing.T) {
	result := RenderHelp("Press q to quit")
	if !strings.Contains(result, "Press q to quit") {
		t.Errorf("RenderHelp should contain the text, got %q", result)
	}
	if len(result) == 0 {
		t.Error("RenderHelp should return non-empty string")
	}
}

func TestRenderBordered(t *testing.T) {
	result := RenderBordered("Content")
	if !strings.Contains(result, "Content") {
		t.Errorf("RenderBordered should contain the text, got %q", result)
	}
	// Bordered content should have border characters
	if len(result) == len("Content") {
		t.Error("RenderBordered should add border (length should increase)")
	}
}

func TestRenderFocusedBordered(t *testing.T) {
	result := RenderFocusedBordered("Focused Content")
	if !strings.Contains(result, "Focused Content") {
		t.Errorf("RenderFocusedBordered should contain the text, got %q", result)
	}
	if len(result) == len("Focused Content") {
		t.Error("RenderFocusedBordered should add border (length should increase)")
	}
}

func TestRenderSelected(t *testing.T) {
	result := RenderSelected("Option 1")
	if !strings.Contains(result, "Option 1") {
		t.Errorf("RenderSelected should contain the text, got %q", result)
	}
	// Should contain the selection indicator
	if !strings.Contains(result, ">") {
		t.Error("RenderSelected should include selection indicator '>'")
	}
}

func TestRenderUnselected(t *testing.T) {
	result := RenderUnselected("Option 2")
	if !strings.Contains(result, "Option 2") {
		t.Errorf("RenderUnselected should contain the text, got %q", result)
	}
}

func TestStyleWidth(t *testing.T) {
	style := lipgloss.NewStyle()
	newStyle := StyleWidth(style, 50)
	// The new style should have width set
	if newStyle.GetWidth() != 50 {
		t.Errorf("StyleWidth should set width to 50, got %d", newStyle.GetWidth())
	}
}

func TestStyleHeight(t *testing.T) {
	style := lipgloss.NewStyle()
	newStyle := StyleHeight(style, 20)
	if newStyle.GetHeight() != 20 {
		t.Errorf("StyleHeight should set height to 20, got %d", newStyle.GetHeight())
	}
}

func TestCenterHorizontally(t *testing.T) {
	result := CenterHorizontally("Hello", 20)
	// Result should be 20 chars wide
	if len(result) != 20 {
		t.Errorf("CenterHorizontally with width 20 should be 20 chars, got %d", len(result))
	}
	// Should contain the original text
	if !strings.Contains(result, "Hello") {
		t.Errorf("CenterHorizontally should contain 'Hello', got %q", result)
	}
}

func TestCenterVertically(t *testing.T) {
	result := CenterVertically("Hello", 5)
	// Result should have newlines for vertical centering
	lines := strings.Split(result, "\n")
	if len(lines) < 5 {
		t.Errorf("CenterVertically with height 5 should have at least 5 lines, got %d", len(lines))
	}
}

func TestCenter(t *testing.T) {
	result := Center("X", 10, 5)
	// Should be centered in both dimensions
	lines := strings.Split(result, "\n")
	if len(lines) < 5 {
		t.Errorf("Center with height 5 should have at least 5 lines, got %d", len(lines))
	}
	// Content should be present
	if !strings.Contains(result, "X") {
		t.Errorf("Center should contain 'X', got %q", result)
	}
}

func TestStylesAreNotEmpty(t *testing.T) {
	// Ensure all style variables are properly initialized
	styles := map[string]lipgloss.Style{
		"TitleStyle":         TitleStyle,
		"SubtitleStyle":      SubtitleStyle,
		"ErrorStyle":         ErrorStyle,
		"SuccessStyle":       SuccessStyle,
		"MutedStyle":         MutedStyle,
		"HighlightStyle":     HighlightStyle,
		"BorderStyle":        BorderStyle,
		"FocusedBorderStyle": FocusedBorderStyle,
		"InputStyle":         InputStyle,
		"PlaceholderStyle":   PlaceholderStyle,
		"SelectedStyle":      SelectedStyle,
		"UnselectedStyle":    UnselectedStyle,
		"HelpStyle":          HelpStyle,
	}

	for name, style := range styles {
		// Render something to ensure the style works
		result := style.Render("test")
		if len(result) == 0 {
			t.Errorf("%s.Render() returned empty string", name)
		}
	}
}

func TestEmptyInput(t *testing.T) {
	// Test that rendering functions handle empty input
	tests := []struct {
		name   string
		render func(string) string
	}{
		{"RenderTitle", RenderTitle},
		{"RenderSubtitle", RenderSubtitle},
		{"RenderError", RenderError},
		{"RenderSuccess", RenderSuccess},
		{"RenderMuted", RenderMuted},
		{"RenderHighlight", RenderHighlight},
		{"RenderHelp", RenderHelp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic with empty input
			result := tt.render("")
			// Result may have ANSI codes but should not panic
			_ = result
		})
	}
}

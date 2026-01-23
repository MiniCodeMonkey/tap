// Package tui provides terminal user interface components using Bubble Tea.
package tui

import "github.com/charmbracelet/lipgloss"

// Color scheme for the TUI - consistent across all components.
const (
	// Primary color - purple/violet for main accent elements.
	ColorPrimary = lipgloss.Color("#7C3AED")
	// Secondary color - emerald green for success/positive elements.
	ColorSecondary = lipgloss.Color("#10B981")
	// Error color - red for error states and warnings.
	ColorError = lipgloss.Color("#EF4444")
	// Muted color - gray for secondary text and borders.
	ColorMuted = lipgloss.Color("#6B7280")
	// White color - for text on dark backgrounds.
	ColorWhite = lipgloss.Color("#FFFFFF")
	// Black color - for text on light backgrounds.
	ColorBlack = lipgloss.Color("#000000")
)

// Base styles - building blocks for component styles.
var (
	// TitleStyle is used for main titles and headers.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	// SubtitleStyle is used for subtitles and secondary headers.
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginBottom(1)

	// ErrorStyle is used for error messages and warnings.
	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorError)

	// SuccessStyle is used for success messages and confirmations.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	// MutedStyle is used for secondary or less important text.
	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// HighlightStyle is used for highlighted/selected items.
	HighlightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	// BorderStyle is used for bordered containers.
	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(1, 2)

	// FocusedBorderStyle is used for focused/active bordered containers.
	FocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2)

	// InputStyle is used for text input fields.
	InputStyle = lipgloss.NewStyle().
			Foreground(ColorWhite)

	// PlaceholderStyle is used for input placeholder text.
	PlaceholderStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	// SelectedStyle is used for selected list items.
	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary).
			PaddingLeft(2)

	// UnselectedStyle is used for unselected list items.
	UnselectedStyle = lipgloss.NewStyle().
			Foreground(ColorWhite).
			PaddingLeft(2)

	// HelpStyle is used for help text and keyboard shortcuts.
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(1)
)

// RenderTitle renders a title with the TitleStyle.
func RenderTitle(text string) string {
	return TitleStyle.Render(text)
}

// RenderSubtitle renders a subtitle with the SubtitleStyle.
func RenderSubtitle(text string) string {
	return SubtitleStyle.Render(text)
}

// RenderError renders an error message with the ErrorStyle.
func RenderError(text string) string {
	return ErrorStyle.Render(text)
}

// RenderSuccess renders a success message with the SuccessStyle.
func RenderSuccess(text string) string {
	return SuccessStyle.Render(text)
}

// RenderMuted renders muted/secondary text with the MutedStyle.
func RenderMuted(text string) string {
	return MutedStyle.Render(text)
}

// RenderHighlight renders highlighted text with the HighlightStyle.
func RenderHighlight(text string) string {
	return HighlightStyle.Render(text)
}

// RenderHelp renders help text with the HelpStyle.
func RenderHelp(text string) string {
	return HelpStyle.Render(text)
}

// RenderBordered renders content inside a bordered container.
func RenderBordered(content string) string {
	return BorderStyle.Render(content)
}

// RenderFocusedBordered renders content inside a focused bordered container.
func RenderFocusedBordered(content string) string {
	return FocusedBorderStyle.Render(content)
}

// RenderSelected renders a selected list item.
func RenderSelected(text string) string {
	return SelectedStyle.Render("> " + text)
}

// RenderUnselected renders an unselected list item.
func RenderUnselected(text string) string {
	return UnselectedStyle.Render("  " + text)
}

// StyleWidth returns a copy of the style with the specified width.
func StyleWidth(style lipgloss.Style, width int) lipgloss.Style {
	return style.Width(width)
}

// StyleHeight returns a copy of the style with the specified height.
func StyleHeight(style lipgloss.Style, height int) lipgloss.Style {
	return style.Height(height)
}

// CenterHorizontally centers content horizontally within the given width.
func CenterHorizontally(content string, width int) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}

// CenterVertically centers content vertically within the given height.
func CenterVertically(content string, height int) string {
	return lipgloss.PlaceVertical(height, lipgloss.Center, content)
}

// Center centers content both horizontally and vertically.
func Center(content string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateStarterMarkdown(t *testing.T) {
	tests := []struct {
		name   string
		title  string
		theme  string
		date   string
		author string
		want   []string // strings that should be present
	}{
		{
			name:   "basic markdown generation",
			title:  "My Presentation",
			theme:  "minimal",
			date:   "2024-01-15",
			author: "John Doe",
			want: []string{
				`title: "My Presentation"`,
				`theme: minimal`,
				`author: "John Doe"`,
				`date: "2024-01-15"`,
				`aspectRatio: "16:9"`,
				`transition: fade`,
				"# My Presentation",
				"## Agenda",
				"<!-- pause -->",
				"## Key Points",
				"```go",
				"|||",
				"## Two Column Layout",
				`layout: quote`,
				"> \"The best way to predict the future",
				"# Thank You!",
			},
		},
		{
			name:   "with special characters in title",
			title:  "Go & Concurrency: A Deep Dive",
			theme:  "terminal",
			date:   "2024-02-20",
			author: "Jane Smith",
			want: []string{
				`title: "Go & Concurrency: A Deep Dive"`,
				`theme: terminal`,
				`author: "Jane Smith"`,
			},
		},
		{
			name:   "gradient theme",
			title:  "Modern UI Design",
			theme:  "gradient",
			date:   "2024-03-10",
			author: "Design Team",
			want: []string{
				`theme: gradient`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateStarterMarkdown(tt.title, tt.theme, tt.date, tt.author)

			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("GenerateStarterMarkdown() missing expected content %q", want)
				}
			}
		})
	}
}

func TestGenerateStarterMarkdown_HasFrontmatter(t *testing.T) {
	md := GenerateStarterMarkdown("Test", "minimal", "2024-01-01", "Author")

	// Should start with frontmatter delimiter
	if !strings.HasPrefix(md, "---\n") {
		t.Error("Markdown should start with frontmatter delimiter")
	}

	// Should have closing frontmatter delimiter
	parts := strings.SplitN(md, "---", 3)
	if len(parts) < 3 {
		t.Error("Markdown should have opening and closing frontmatter delimiters")
	}
}

func TestGenerateStarterMarkdown_HasMultipleSlides(t *testing.T) {
	md := GenerateStarterMarkdown("Test", "minimal", "2024-01-01", "Author")

	// Count slide separators (excluding frontmatter)
	// Skip the first two dashes from frontmatter
	afterFrontmatter := strings.SplitN(md, "---", 3)
	if len(afterFrontmatter) < 3 {
		t.Fatal("Markdown should have frontmatter and content")
	}

	content := afterFrontmatter[2]
	slideCount := strings.Count(content, "\n---\n") + 1 // +1 for first slide

	if slideCount < 5 {
		t.Errorf("Expected at least 5 example slides, got %d", slideCount)
	}
}

func TestGenerateStarterMarkdown_HasCodeExample(t *testing.T) {
	md := GenerateStarterMarkdown("Test", "minimal", "2024-01-01", "Author")

	if !strings.Contains(md, "```go") {
		t.Error("Markdown should contain a Go code example")
	}

	if !strings.Contains(md, "layout: code-focus") {
		t.Error("Markdown should have a code-focus layout slide")
	}
}

func TestGenerateStarterMarkdown_HasTwoColumnExample(t *testing.T) {
	md := GenerateStarterMarkdown("Test", "minimal", "2024-01-01", "Author")

	if !strings.Contains(md, "|||") {
		t.Error("Markdown should contain two-column separator")
	}
}

func TestGenerateStarterMarkdown_HasFragmentExample(t *testing.T) {
	md := GenerateStarterMarkdown("Test", "minimal", "2024-01-01", "Author")

	if !strings.Contains(md, "<!-- pause -->") {
		t.Error("Markdown should contain fragment marker")
	}
}

func TestGenerateStarterMarkdown_HasQuoteExample(t *testing.T) {
	md := GenerateStarterMarkdown("Test", "minimal", "2024-01-01", "Author")

	if !strings.Contains(md, "layout: quote") {
		t.Error("Markdown should have a quote layout slide")
	}

	if !strings.Contains(md, "> ") {
		t.Error("Markdown should contain a blockquote")
	}
}

func TestNewModel_GenerateDefaultFilename(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "simple title",
			title:    "My Presentation",
			expected: "my-presentation.md",
		},
		{
			name:     "title with special characters",
			title:    "Go & Rust: A Comparison!",
			expected: "go-rust-a-comparison.md",
		},
		{
			name:     "title with numbers",
			title:    "Chapter 1: Introduction",
			expected: "chapter-1-introduction.md",
		},
		{
			name:     "empty title",
			title:    "",
			expected: "presentation.md",
		},
		{
			name:     "title with multiple spaces",
			title:    "Too   Many    Spaces",
			expected: "too-many-spaces.md",
		},
		{
			name:     "uppercase title",
			title:    "ALL CAPS TITLE",
			expected: "all-caps-title.md",
		},
		{
			name:     "title with leading/trailing spaces",
			title:    "  Trimmed  ",
			expected: "trimmed.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewNewModel("", "")
			m.titleInput.SetValue(tt.title)

			got := m.generateDefaultFilename()
			if got != tt.expected {
				t.Errorf("generateDefaultFilename() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNewModel_FindThemeIndex(t *testing.T) {
	m := NewNewModel("", "")

	tests := []struct {
		theme    string
		expected int
	}{
		{"minimal", 0},
		{"gradient", 1},
		{"terminal", 2},
		{"brutalist", 3},
		{"keynote", 4},
		{"MINIMAL", 0},  // case insensitive
		{"Gradient", 1}, // case insensitive
		{"unknown", 0},  // defaults to first
		{"", 0},         // empty defaults to first
	}

	for _, tt := range tests {
		t.Run(tt.theme, func(t *testing.T) {
			got := m.findThemeIndex(tt.theme)
			if got != tt.expected {
				t.Errorf("findThemeIndex(%q) = %d, want %d", tt.theme, got, tt.expected)
			}
		})
	}
}

func TestNewModel_Init(t *testing.T) {
	m := NewNewModel("", "")
	cmd := m.Init()

	// Init should return a command (for text input blinking)
	if cmd == nil {
		t.Error("Init() should return a non-nil command")
	}
}

func TestNewModel_View_TitleStep(t *testing.T) {
	m := NewNewModel("", "")
	view := m.View()

	// Should show title prompt
	if !strings.Contains(view, "Create New Presentation") {
		t.Error("View should contain title header")
	}

	if !strings.Contains(view, "title of your presentation") {
		t.Error("View should contain title prompt")
	}
}

func TestNewModel_View_Quitting(t *testing.T) {
	m := NewNewModel("", "")
	m.quitting = true
	view := m.View()

	if !strings.Contains(view, "Aborted") {
		t.Error("View should show aborted message when quitting")
	}
}

func TestNewModel_GetResult(t *testing.T) {
	m := NewNewModel("", "")
	m.titleInput.SetValue("Test Title")
	m.filenameInput.SetValue("test.md")
	m.themeIndex = 2 // terminal

	result := m.GetResult()

	if result.Title != "Test Title" {
		t.Errorf("GetResult().Title = %q, want %q", result.Title, "Test Title")
	}

	if result.Theme != "terminal" {
		t.Errorf("GetResult().Theme = %q, want %q", result.Theme, "terminal")
	}

	if result.Filename != "test.md" {
		t.Errorf("GetResult().Filename = %q, want %q", result.Filename, "test.md")
	}

	if result.Aborted {
		t.Error("GetResult().Aborted should be false")
	}
}

func TestNewModel_GetResult_Aborted(t *testing.T) {
	m := NewNewModel("", "")
	m.quitting = true

	result := m.GetResult()

	if !result.Aborted {
		t.Error("GetResult().Aborted should be true when quitting")
	}
}

func TestAvailableThemes(t *testing.T) {
	expectedThemes := []string{"minimal", "gradient", "terminal", "brutalist", "keynote"}

	if len(AvailableThemes) != len(expectedThemes) {
		t.Errorf("Expected %d themes, got %d", len(expectedThemes), len(AvailableThemes))
	}

	for i, expected := range expectedThemes {
		if AvailableThemes[i].Name != expected {
			t.Errorf("Theme[%d].Name = %q, want %q", i, AvailableThemes[i].Name, expected)
		}

		if AvailableThemes[i].Description == "" {
			t.Errorf("Theme[%d].Description should not be empty", i)
		}
	}
}

func TestNewModel_PrefilledValues(t *testing.T) {
	t.Run("prefilled theme", func(t *testing.T) {
		m := NewNewModel("terminal", "")
		if m.prefilledTheme != "terminal" {
			t.Errorf("prefilledTheme = %q, want %q", m.prefilledTheme, "terminal")
		}
	})

	t.Run("prefilled filename", func(t *testing.T) {
		m := NewNewModel("", "custom.md")
		if m.prefilledFilename != "custom.md" {
			t.Errorf("prefilledFilename = %q, want %q", m.prefilledFilename, "custom.md")
		}
	})
}

func TestNewModel_FileCreation(t *testing.T) {
	// Create a temp directory for testing
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	m := NewNewModel("", "")
	m.titleInput.SetValue("Test Presentation")
	m.filenameInput.SetValue("test-output.md")
	m.themeIndex = 0

	// Simulate finalize
	model, _ := m.finalize()
	m = model.(NewModel)

	// Check file was created
	content, err := os.ReadFile("test-output.md")
	if err != nil {
		t.Fatalf("File should have been created: %v", err)
	}

	// Verify content
	if !strings.Contains(string(content), "Test Presentation") {
		t.Error("File should contain the title")
	}

	if !strings.Contains(string(content), "theme: minimal") {
		t.Error("File should contain the theme")
	}
}

func TestNewModel_FileCreation_AddsExtension(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	m := NewNewModel("", "")
	m.titleInput.SetValue("Test")
	m.filenameInput.SetValue("no-extension") // No .md extension
	m.themeIndex = 0

	model, _ := m.finalize()
	m = model.(NewModel)

	// Check file was created with .md extension
	if _, err := os.Stat("no-extension.md"); os.IsNotExist(err) {
		t.Error("File should be created with .md extension added")
	}

	if m.filenameInput.Value() != "no-extension.md" {
		t.Errorf("Filename should have .md appended, got %q", m.filenameInput.Value())
	}
}

func TestNewModel_OutputPath(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	m := NewNewModel("", "")
	m.titleInput.SetValue("Test")
	m.filenameInput.SetValue("output.md")
	m.themeIndex = 0

	model, _ := m.finalize()
	m = model.(NewModel)

	if m.GetOutputPath() != "output.md" {
		t.Errorf("GetOutputPath() = %q, want %q", m.GetOutputPath(), "output.md")
	}
}

func TestNewModel_WasAborted(t *testing.T) {
	m := NewNewModel("", "")

	if m.WasAborted() {
		t.Error("WasAborted() should be false initially")
	}

	m.quitting = true
	if !m.WasAborted() {
		t.Error("WasAborted() should be true after quitting")
	}
}

func TestNewModel_SetDefaultFilename_UsesPrefilledFirst(t *testing.T) {
	m := NewNewModel("", "prefilled.md")
	m.titleInput.SetValue("Some Title")

	m.setDefaultFilename()

	if m.filenameInput.Value() != "prefilled.md" {
		t.Errorf("setDefaultFilename() should use prefilled value, got %q", m.filenameInput.Value())
	}
}

func TestNewModel_ViewSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	m := NewNewModel("", "")
	m.titleInput.SetValue("Success Test")
	m.filenameInput.SetValue("success.md")
	m.themeIndex = 1 // gradient

	model, _ := m.finalize()
	m = model.(NewModel)

	view := m.View()

	if !strings.Contains(view, "successfully") {
		t.Error("Success view should contain success message")
	}

	if !strings.Contains(view, "success.md") {
		t.Error("Success view should contain filename")
	}

	if !strings.Contains(view, "tap dev") {
		t.Error("Success view should contain next step with tap dev")
	}

	if !strings.Contains(view, "tap build") {
		t.Error("Success view should contain next step with tap build")
	}
}

func TestNewModel_AbsolutePathInView(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	m := NewNewModel("", "")
	m.titleInput.SetValue("Path Test")
	m.filenameInput.SetValue("pathtest.md")

	model, _ := m.finalize()
	m = model.(NewModel)

	view := m.View()
	absPath, _ := filepath.Abs("pathtest.md")

	if !strings.Contains(view, absPath) {
		t.Errorf("Success view should contain absolute path %q", absPath)
	}
}

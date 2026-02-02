package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilePickerModel_HasFiles(t *testing.T) {
	// Create temp directory with no files
	tmpDir, err := os.MkdirTemp("", "filepicker-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldDir) }()

	m := NewFilePickerModel()
	if m.HasFiles() {
		t.Error("expected HasFiles() to return false for empty directory")
	}

	// Create a markdown file
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	m = NewFilePickerModel()
	if !m.HasFiles() {
		t.Error("expected HasFiles() to return true when markdown files exist")
	}
}

func TestFilePickerModel_Navigation(t *testing.T) {
	// Create temp directory with markdown files
	tmpDir, err := os.MkdirTemp("", "filepicker-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	files := []string{"a.md", "b.md", "c.md"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("# "+f), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Change to temp directory
	oldDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldDir) }()

	m := NewFilePickerModel()

	// Test initial state
	if m.cursor != 0 {
		t.Errorf("expected cursor to start at 0, got %d", m.cursor)
	}

	// Test down navigation
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(FilePickerModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor to be 1 after down, got %d", m.cursor)
	}

	// Test up navigation
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(FilePickerModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor to be 0 after up, got %d", m.cursor)
	}

	// Test k/j navigation
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(FilePickerModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor to be 1 after j, got %d", m.cursor)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(FilePickerModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor to be 0 after k, got %d", m.cursor)
	}

	// Test boundary - can't go above 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(FilePickerModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m.cursor)
	}
}

func TestFilePickerModel_Selection(t *testing.T) {
	// Create temp directory with a markdown file
	tmpDir, err := os.MkdirTemp("", "filepicker-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldDir) }()

	m := NewFilePickerModel()

	// Select with enter
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(FilePickerModel)
	result := m.GetResult()

	if result.Aborted {
		t.Error("expected selection not to be aborted")
	}
	if result.File != "test.md" {
		t.Errorf("expected selected file to be 'test.md', got %q", result.File)
	}
}

func TestFilePickerModel_Cancel(t *testing.T) {
	// Create temp directory with a markdown file
	tmpDir, err := os.MkdirTemp("", "filepicker-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldDir) }()

	tests := []struct {
		name string
		key  tea.KeyMsg
	}{
		{"esc", tea.KeyMsg{Type: tea.KeyEsc}},
		{"ctrl+c", tea.KeyMsg{Type: tea.KeyCtrlC}},
		{"q", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewFilePickerModel()
			updated, _ := m.Update(tt.key)
			m = updated.(FilePickerModel)
			result := m.GetResult()

			if !result.Aborted {
				t.Errorf("expected selection to be aborted with %s", tt.name)
			}
		})
	}
}

func TestFilePickerModel_View(t *testing.T) {
	// Create temp directory with markdown files
	tmpDir, err := os.MkdirTemp("", "filepicker-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "slides.md"), []byte("# Slides"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldDir) }()

	m := NewFilePickerModel()
	view := m.View()

	if !strings.Contains(view, "Select a presentation") {
		t.Error("expected view to contain title")
	}
	if !strings.Contains(view, "slides.md") {
		t.Error("expected view to contain filename")
	}
}

func TestRenderNoFilesError(t *testing.T) {
	output := RenderNoFilesError()

	if !strings.Contains(output, "No markdown files found") {
		t.Error("expected error message to contain 'No markdown files found'")
	}
	if !strings.Contains(output, "tap new") {
		t.Error("expected error message to suggest 'tap new'")
	}
	if !strings.Contains(output, "tap dev") {
		t.Error("expected error message to suggest 'tap dev'")
	}
}

func TestFindMarkdownFiles_IgnoresDirectories(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "filepicker-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a directory with .md suffix (unusual but possible)
	if err := os.Mkdir(filepath.Join(tmpDir, "weird.md"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a real markdown file
	if err := os.WriteFile(filepath.Join(tmpDir, "real.md"), []byte("# Real"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldDir) }()

	m := NewFilePickerModel()

	if len(m.files) != 1 {
		t.Errorf("expected 1 file, got %d", len(m.files))
	}
	if m.files[0] != "real.md" {
		t.Errorf("expected 'real.md', got %q", m.files[0])
	}
}

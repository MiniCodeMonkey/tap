package embedded

import (
	"strings"
	"testing"
)

func TestGetIndexHTML(t *testing.T) {
	content, err := GetIndexHTML()
	if err != nil {
		t.Fatalf("GetIndexHTML() error = %v", err)
	}

	if len(content) == 0 {
		t.Error("GetIndexHTML() returned empty content")
	}

	// Check that it's valid HTML
	html := string(content)
	if !strings.Contains(html, "<!doctype html>") && !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("GetIndexHTML() does not contain DOCTYPE")
	}
	if !strings.Contains(html, "<title>") {
		t.Error("GetIndexHTML() does not contain title tag")
	}
}

func TestGetPresenterHTML(t *testing.T) {
	content, err := GetPresenterHTML()
	if err != nil {
		t.Fatalf("GetPresenterHTML() error = %v", err)
	}

	if len(content) == 0 {
		t.Error("GetPresenterHTML() returned empty content")
	}

	// Check that it's valid HTML
	html := string(content)
	if !strings.Contains(html, "<!doctype html>") && !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("GetPresenterHTML() does not contain DOCTYPE")
	}
	if !strings.Contains(html, "Presenter") && !strings.Contains(html, "presenter") {
		t.Error("GetPresenterHTML() does not contain 'Presenter' text")
	}
}

func TestGetFile(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		wantErr bool
	}{
		{
			name:    "index.html exists",
			file:    "index.html",
			wantErr: false,
		},
		{
			name:    "presenter.html exists",
			file:    "presenter.html",
			wantErr: false,
		},
		{
			name:    "nonexistent file",
			file:    "nonexistent.html",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := GetFile(tt.file)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFile(%q) error = %v, wantErr %v", tt.file, err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(content) == 0 {
				t.Errorf("GetFile(%q) returned empty content", tt.file)
			}
		})
	}
}

func TestExists(t *testing.T) {
	tests := []struct {
		name string
		file string
		want bool
	}{
		{
			name: "index.html exists",
			file: "index.html",
			want: true,
		},
		{
			name: "presenter.html exists",
			file: "presenter.html",
			want: true,
		},
		{
			name: "nonexistent file",
			file: "nonexistent.html",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Exists(tt.file); got != tt.want {
				t.Errorf("Exists(%q) = %v, want %v", tt.file, got, tt.want)
			}
		})
	}
}

func TestList(t *testing.T) {
	files, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(files) < 2 {
		t.Errorf("List() returned %d files, expected at least 2", len(files))
	}

	// Check for expected files
	foundIndex := false
	foundPresenter := false
	for _, f := range files {
		if f == "index.html" {
			foundIndex = true
		}
		if f == "presenter.html" {
			foundPresenter = true
		}
	}

	if !foundIndex {
		t.Error("List() did not include index.html")
	}
	if !foundPresenter {
		t.Error("List() did not include presenter.html")
	}
}

func TestFileSystem(t *testing.T) {
	fsys, err := FileSystem()
	if err != nil {
		t.Fatalf("FileSystem() error = %v", err)
	}
	if fsys == nil {
		t.Error("FileSystem() returned nil")
	}

	// Try to open a file
	file, err := fsys.Open("index.html")
	if err != nil {
		t.Errorf("FileSystem().Open() error = %v", err)
		return
	}
	defer file.Close()

	// Check file info
	info, err := file.Stat()
	if err != nil {
		t.Errorf("file.Stat() error = %v", err)
		return
	}

	if info.Size() == 0 {
		t.Error("index.html has size 0")
	}
}

func TestDistFS(t *testing.T) {
	fsys, err := DistFS()
	if err != nil {
		t.Fatalf("DistFS() error = %v", err)
	}
	if fsys == nil {
		t.Error("DistFS() returned nil")
	}
}

func TestListAll(t *testing.T) {
	files, err := ListAll()
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}

	// Should include files in subdirectories (assets/)
	if len(files) < 2 {
		t.Errorf("ListAll() returned %d files, expected at least 2", len(files))
	}

	// Check for expected files
	foundIndex := false
	foundAssets := false
	for _, f := range files {
		if f == "index.html" {
			foundIndex = true
		}
		if strings.HasPrefix(f, "assets/") {
			foundAssets = true
		}
	}

	if !foundIndex {
		t.Error("ListAll() did not include index.html")
	}
	if !foundAssets {
		t.Error("ListAll() did not include any assets/ files")
	}
}

func TestAssetsContainTailwindCSS(t *testing.T) {
	// Verify that built CSS files exist in assets/
	files, err := ListAll()
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}

	foundCSS := false
	foundJS := false
	for _, f := range files {
		if strings.HasSuffix(f, ".css") && strings.HasPrefix(f, "assets/") {
			foundCSS = true
		}
		if strings.HasSuffix(f, ".js") && strings.HasPrefix(f, "assets/") {
			foundJS = true
		}
	}

	if !foundCSS {
		t.Error("No CSS files found in assets/ - Tailwind CSS build may have failed")
	}
	if !foundJS {
		t.Error("No JS files found in assets/ - Vite build may have failed")
	}
}

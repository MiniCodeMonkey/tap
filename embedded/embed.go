// Package embedded provides embedded frontend assets for the tap presentation viewer.
// These assets are compiled into the binary using Go's embed package.
package embedded

import (
	"embed"
	"io/fs"
)

// Assets contains all embedded frontend files built by Vite.
// The //go:embed directive embeds all files from the dist/ directory,
// which contains the optimized production build including:
// - index.html and presenter.html (HTML entry points)
// - assets/*.js (bundled JavaScript with Svelte components)
// - assets/*.css (Tailwind CSS output, properly purged)
//
//go:embed dist/*
var Assets embed.FS

// DistFS returns an fs.FS rooted at the dist/ directory.
// This strips the "dist/" prefix from embedded files for cleaner access,
// allowing files to be accessed as "index.html" instead of "dist/index.html".
func DistFS() (fs.FS, error) {
	return fs.Sub(Assets, "dist")
}

// GetFile returns the content of an embedded file from the dist/ directory.
func GetFile(name string) ([]byte, error) {
	subFS, err := DistFS()
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(subFS, name)
}

// GetIndexHTML returns the content of index.html.
func GetIndexHTML() ([]byte, error) {
	return GetFile("index.html")
}

// GetPresenterHTML returns the content of presenter.html.
func GetPresenterHTML() ([]byte, error) {
	return GetFile("presenter.html")
}

// FileSystem returns an fs.FS rooted at dist/ for use with http.FileServer.
func FileSystem() (fs.FS, error) {
	return DistFS()
}

// Exists checks if a file exists in the embedded assets.
func Exists(name string) bool {
	subFS, err := DistFS()
	if err != nil {
		return false
	}
	_, err = fs.Stat(subFS, name)
	return err == nil
}

// List returns a list of all embedded files in the dist/ directory.
func List() ([]string, error) {
	subFS, err := DistFS()
	if err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(subFS, ".")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

// ListAll returns all embedded files including those in subdirectories.
func ListAll() ([]string, error) {
	subFS, err := DistFS()
	if err != nil {
		return nil, err
	}

	var files []string
	err = fs.WalkDir(subFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

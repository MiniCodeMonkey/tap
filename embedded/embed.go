// Package embedded provides embedded frontend assets for the tap presentation viewer.
// These assets are compiled into the binary using Go's embed package.
package embedded

import (
	"embed"
	"io/fs"
)

// Assets contains all embedded frontend files.
// The //go:embed directive embeds all files from the current directory,
// excluding .go files and the .gitkeep placeholder.
//
//go:embed index.html presenter.html
var Assets embed.FS

// GetFile returns the content of an embedded file.
func GetFile(name string) ([]byte, error) {
	return Assets.ReadFile(name)
}

// GetIndexHTML returns the content of index.html.
func GetIndexHTML() ([]byte, error) {
	return GetFile("index.html")
}

// GetPresenterHTML returns the content of presenter.html.
func GetPresenterHTML() ([]byte, error) {
	return GetFile("presenter.html")
}

// FileSystem returns an fs.FS for use with http.FileServer.
func FileSystem() fs.FS {
	return Assets
}

// Exists checks if a file exists in the embedded assets.
func Exists(name string) bool {
	_, err := Assets.Open(name)
	return err == nil
}

// List returns a list of all embedded files.
func List() ([]string, error) {
	entries, err := Assets.ReadDir(".")
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

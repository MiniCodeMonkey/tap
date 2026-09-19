package server

import "sync/atomic"

// ComponentBundleFile is one file served under /components/: a bundle's
// JavaScript, its CSS (when it has any), or its source map (dev only).
type ComponentBundleFile struct {
	ContentType string
	Content     []byte
}

// ComponentBundleStore holds the dev server's current set of component
// bundle files, keyed by file name ("<Name>-<Hash>.js", ".css", ".js.map").
// The dev command swaps the whole set atomically after each rebuild, so a
// request never sees a half-updated mix of an old and a new build.
type ComponentBundleStore struct {
	files atomic.Value // map[string]ComponentBundleFile
}

// NewComponentBundleStore returns an empty store.
func NewComponentBundleStore() *ComponentBundleStore {
	store := &ComponentBundleStore{}
	store.files.Store(map[string]ComponentBundleFile{})
	return store
}

// Set atomically replaces the store's files.
func (s *ComponentBundleStore) Set(files map[string]ComponentBundleFile) {
	if files == nil {
		files = map[string]ComponentBundleFile{}
	}
	s.files.Store(files)
}

// Get returns the file for the given name, and whether it was found.
func (s *ComponentBundleStore) Get(name string) (ComponentBundleFile, bool) {
	files, _ := s.files.Load().(map[string]ComponentBundleFile)
	file, ok := files[name]
	return file, ok
}

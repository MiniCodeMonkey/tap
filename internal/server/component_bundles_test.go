package server

import "testing"

func TestComponentBundleStore_GetUnknown(t *testing.T) {
	store := NewComponentBundleStore()
	if _, ok := store.Get("Nope-abc123.js"); ok {
		t.Error("expected Get to report not found for an unset store")
	}
}

func TestComponentBundleStore_SetAndGet(t *testing.T) {
	store := NewComponentBundleStore()
	store.Set(map[string]ComponentBundleFile{
		"RollingDeploy-abc123.js": {ContentType: "application/javascript; charset=utf-8", Content: []byte("console.log(1)")},
	})

	file, ok := store.Get("RollingDeploy-abc123.js")
	if !ok {
		t.Fatal("expected the file to be found")
	}
	if string(file.Content) != "console.log(1)" {
		t.Errorf("Content = %q, want %q", file.Content, "console.log(1)")
	}
	if file.ContentType != "application/javascript; charset=utf-8" {
		t.Errorf("ContentType = %q", file.ContentType)
	}
}

func TestComponentBundleStore_SetReplacesAtomically(t *testing.T) {
	store := NewComponentBundleStore()
	store.Set(map[string]ComponentBundleFile{"A-1.js": {Content: []byte("a")}})
	store.Set(map[string]ComponentBundleFile{"B-2.js": {Content: []byte("b")}})

	if _, ok := store.Get("A-1.js"); ok {
		t.Error("expected the old build's file to be gone after a rebuild")
	}
	file, ok := store.Get("B-2.js")
	if !ok || string(file.Content) != "b" {
		t.Errorf("expected the new build's file to be present, got %+v, ok=%v", file, ok)
	}
}

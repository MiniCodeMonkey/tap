package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

type fakeDeckServer struct {
	presentation *transformer.TransformedPresentation
	revision     string
}

func (f *fakeDeckServer) SetPresentation(presentation *transformer.TransformedPresentation) {
	f.presentation = presentation
}
func (f *fakeDeckServer) SetComponentBundles(map[string]server.ComponentBundleFile) {}
func (f *fakeDeckServer) SetRevision(revision string)                              { f.revision = revision }

type fakeDeckHub struct {
	sent       []string
	slideCount int
	revision   string
}

func (f *fakeDeckHub) SetPresentationMeta(slideCount int, revision string) {
	f.slideCount, f.revision = slideCount, revision
}
func (f *fakeDeckHub) BroadcastReload() error {
	f.sent = append(f.sent, "reload")
	return nil
}
func (f *fakeDeckHub) BroadcastUpdate(revision string, slides []int) error {
	f.sent = append(f.sent, fmt.Sprintf("update %v", slides))
	return nil
}

// deckOf builds a transformed deck with one slide per HTML string, each
// with its content hash.
func deckOf(htmls ...string) *transformer.TransformedPresentation {
	presentation := &transformer.TransformedPresentation{}
	for index, html := range htmls {
		slide := transformer.TransformedSlide{Index: index, Layout: "default", HTML: html}
		slide.Hash = transformer.SlideHash(slide)
		presentation.Slides = append(presentation.Slides, slide)
	}
	return presentation
}

func newTestPublisher(t *testing.T, first *transformer.TransformedPresentation, customThemePath string) (*deckPublisher, *fakeDeckServer, *fakeDeckHub) {
	t.Helper()
	target := &fakeDeckServer{}
	hub := &fakeDeckHub{}
	revision := server.ComputeRevision(first, nil)
	return newDeckPublisher(target, hub, first, revision, customThemePath), target, hub
}

func TestPublishSendsAnUpdateWithTheChangedSlides(t *testing.T) {
	publisher, target, hub := newTestPublisher(t, deckOf("<h1>One</h1>", "<h1>Two</h1>"), "")
	next := deckOf("<h1>One</h1>", "<h1>Two, edited</h1>")

	publisher.publish(next, nil, "", false)

	if fmt.Sprint(hub.sent) != "[update [2]]" {
		t.Errorf("sent %v, want one update for slide 2", hub.sent)
	}
	if target.presentation != next {
		t.Error("the server does not serve the new deck")
	}
	wantRevision := server.ComputeRevision(next, nil)
	if target.revision != wantRevision || hub.revision != wantRevision || hub.slideCount != 2 {
		t.Errorf("server revision %q, hub revision %q and count %d, want %q and 2", target.revision, hub.revision, hub.slideCount, wantRevision)
	}
}

func TestPublishSendsNothingWhenTheRevisionIsTheSame(t *testing.T) {
	publisher, _, hub := newTestPublisher(t, deckOf("<h1>One</h1>"), "")

	publisher.publish(deckOf("<h1>One</h1>"), nil, "", false)

	if len(hub.sent) != 0 {
		t.Errorf("sent %v, want nothing for an unchanged deck", hub.sent)
	}
}

func TestPublishListsAnAddedSlide(t *testing.T) {
	publisher, _, hub := newTestPublisher(t, deckOf("<h1>One</h1>", "<h1>Two</h1>"), "")

	publisher.publish(deckOf("<h1>One</h1>", "<h1>Two</h1>", "<h1>Three</h1>"), nil, "", false)

	if fmt.Sprint(hub.sent) != "[update [3]]" {
		t.Errorf("sent %v, want an update for slide 3", hub.sent)
	}
}

func TestPublishReloadsWhenTheCustomThemeFileChanges(t *testing.T) {
	themePath := filepath.Join(t.TempDir(), "theme.css")
	if err := os.WriteFile(themePath, []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	publisher, _, hub := newTestPublisher(t, deckOf("<h1>One</h1>"), themePath)

	publisher.publish(deckOf("<h1>One</h1>"), nil, themePath, false)
	if len(hub.sent) != 0 {
		t.Fatalf("sent %v for an unchanged theme file, want nothing", hub.sent)
	}

	if err := os.WriteFile(themePath, []byte("body { color: blue; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	publisher.publish(deckOf("<h1>One</h1>"), nil, themePath, false)
	if fmt.Sprint(hub.sent) != "[reload]" {
		t.Errorf("sent %v, want a reload for a changed theme file", hub.sent)
	}
}

func TestPublishReloadsWhenTheCustomThemeIsRemoved(t *testing.T) {
	themePath := filepath.Join(t.TempDir(), "theme.css")
	if err := os.WriteFile(themePath, []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	publisher, _, hub := newTestPublisher(t, deckOf("<h1>One</h1>"), themePath)

	publisher.publish(deckOf("<h1>One</h1>"), nil, "", false)

	if fmt.Sprint(hub.sent) != "[reload]" {
		t.Errorf("sent %v, want a reload when the custom theme goes away", hub.sent)
	}
}

func TestPublishWithForceReloadAlwaysReloads(t *testing.T) {
	publisher, _, hub := newTestPublisher(t, deckOf("<h1>One</h1>"), "")

	publisher.publish(deckOf("<h1>One</h1>"), nil, "", true)

	if fmt.Sprint(hub.sent) != "[reload]" {
		t.Errorf("sent %v, want a reload", hub.sent)
	}
}

func TestCustomThemeFingerprint(t *testing.T) {
	if customThemeFingerprint("") != "" {
		t.Error("no custom theme should give an empty fingerprint")
	}
	themePath := filepath.Join(t.TempDir(), "theme.css")
	if err := os.WriteFile(themePath, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	first := customThemeFingerprint(themePath)
	if err := os.WriteFile(themePath, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if customThemeFingerprint(themePath) == first {
		t.Error("the fingerprint did not change with the file's contents")
	}
}

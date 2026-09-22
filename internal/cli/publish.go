package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// deckServer is the part of *server.Server that a deck change updates.
type deckServer interface {
	SetPresentation(presentation *transformer.TransformedPresentation)
	SetComponentBundles(files map[string]server.ComponentBundleFile)
	SetRevision(revision string)
}

// deckHub is the part of *server.WebSocketHub that a deck change notifies.
type deckHub interface {
	SetPresentationMeta(slideCount int, revision string)
	BroadcastReload() error
	BroadcastUpdate(revision string, slides []int) error
}

// deckPublisher hands a reloaded deck to the server and tells every open
// page about it. An ordinary edit sends "update", and each page fetches
// the deck again and re-renders only the slides that changed. A page
// reloads only when an update is not enough: the custom theme file
// changed (a page loads it once), or the caller forces it (the r key).
// A deck whose revision did not change sends nothing.
type deckPublisher struct {
	target           deckServer
	hub              deckHub
	presentation     *transformer.TransformedPresentation
	revision         string
	themeFingerprint string
	mu               sync.Mutex
}

// newDeckPublisher starts from the deck the server already serves, with
// its revision and custom theme file.
func newDeckPublisher(target deckServer, hub deckHub, presentation *transformer.TransformedPresentation, revision, customThemePath string) *deckPublisher {
	return &deckPublisher{
		target:           target,
		hub:              hub,
		presentation:     presentation,
		revision:         revision,
		themeFingerprint: customThemeFingerprint(customThemePath),
	}
}

// publish serves presentation and its component bundles, and sends every
// open page an "update", a "reload", or nothing (see deckPublisher).
func (p *deckPublisher) publish(presentation *transformer.TransformedPresentation, bundles map[string]server.ComponentBundleFile, customThemePath string, forceReload bool) {
	revision := server.ComputeRevision(presentation, bundles)
	fingerprint := customThemeFingerprint(customThemePath)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.target.SetComponentBundles(bundles)
	p.target.SetPresentation(presentation)
	p.target.SetRevision(revision)
	p.hub.SetPresentationMeta(len(presentation.Slides), revision)

	switch {
	case forceReload || fingerprint != p.themeFingerprint:
		_ = p.hub.BroadcastReload()
	case revision != p.revision:
		_ = p.hub.BroadcastUpdate(revision, server.ChangedSlides(p.presentation, presentation))
	}

	p.presentation = presentation
	p.revision = revision
	p.themeFingerprint = fingerprint
}

// resolveCustomThemePathForReload resolves cfg's custom theme file for a
// reload and points srv at it. Both dev.go reload handlers that reload cfg
// from disk (headless and the TUI) call this instead of repeating the
// resolve-warn-fall back sequence.
func resolveCustomThemePathForReload(cfg *config.Config, baseDir string, srv *server.Server) string {
	path, err := cfg.ResolveCustomThemePath(baseDir)
	if err != nil {
		// Log warning but continue - use empty path to disable custom theme
		Warning("Custom theme not loaded on reload: %v\n", err)
		path = ""
	}
	srv.SetCustomThemePath(path)
	return path
}

// customThemeFingerprint identifies the custom theme CSS a page loaded: its
// path and a hash of its contents. It is "" when there is no custom theme.
// A file that cannot be read gives only its path, so reading it again
// later counts as a change.
func customThemeFingerprint(path string) string {
	if path == "" {
		return ""
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return path
	}
	sum := sha256.Sum256(content)
	return path + "\x00" + hex.EncodeToString(sum[:])
}

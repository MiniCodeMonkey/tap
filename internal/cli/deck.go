// Package cli provides the command-line interface for Tap.
package cli

import (
	"fmt"

	"github.com/MiniCodeMonkey/tap/internal/components"
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/layouts"
	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// prepareDeck loads a deck (see loadPresentation: parse, build its
// deck-supplied React components, transform) and starts a temporary server
// with the deck's presentation and component bundles registered on it -
// the setup tap pdf and tap screenshot both need before doing any browser
// work, so the two commands cannot drift apart.
//
// When the deck's components fail to build, prepareDeck returns those
// errors and a nil server without starting one: the caller prints them
// with printComponentErrorsToStderr and exits 1, never launching a
// browser. Any layout or slot warnings are returned either way, for the
// caller to print with printLayoutWarningsToStderr, and any esbuild
// warnings from a successful bundle are returned for the caller to print
// with printComponentWarningsToStderr.
func prepareDeck(file string, cfg *config.Config, baseDir string) (*server.Server, *transformer.TransformedPresentation, []layouts.Warning, []components.BuildError, []components.BuildError, error) {
	pres, warnings, resolvedComponents, componentBuildErrs, _, err := loadPresentation(file, cfg, baseDir)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if len(componentBuildErrs) > 0 {
		return nil, pres, warnings, componentBuildErrs, nil, nil
	}
	componentBuildWarnings := componentWarnings(resolvedComponents)

	// 127.0.0.1, not server.New's usual 0.0.0.0: this server only exists
	// for tap's own headless browser to render the deck for a PDF or
	// screenshot, so nothing outside the machine needs to reach it.
	srv := server.NewWithHost(0, "127.0.0.1")
	srv.SetPresentation(pres)
	srv.SetBaseDir(baseDir)
	srv.SetComponentBundles(componentBundleFiles(resolvedComponents))
	if customThemePath, themeErr := cfg.ResolveCustomThemePath(baseDir); themeErr == nil && customThemePath != "" {
		srv.SetCustomThemePath(customThemePath)
	}
	srv.SetupRoutes()
	if err := srv.Start(); err != nil {
		return nil, pres, warnings, nil, nil, fmt.Errorf("failed to start temporary server: %w", err)
	}

	return srv, pres, warnings, nil, componentBuildWarnings, nil
}

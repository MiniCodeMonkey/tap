package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/builder"
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
)

// componentURLPattern finds every deck component bundle URL embedded in the
// built index.html's presentation JSON, for example "url":"components/
// RollingDeploy-abc123.js".
var componentURLPattern = regexp.MustCompile(`"url":"(components/[^"]+)"`)

// TestBuild_ComponentsExampleServesFromStaticSubpath builds
// examples/components/deck.md into a temp folder, the same way `tap build`
// does, and checks the output runs as a plain static site mounted under a
// URL sub path (a common deployment shape: GitHub Pages project sites,
// a reverse proxy path prefix). No browser is needed: this only checks
// that index.html and every component bundle it references come back with
// status 200 through net/http, using http.StripPrefix the way a static
// host would mount the built folder under a prefix.
func TestBuild_ComponentsExampleServesFromStaticSubpath(t *testing.T) {
	deckPath, err := filepath.Abs(filepath.Join("..", "..", "examples", "components", "deck.md"))
	if err != nil {
		t.Fatalf("failed to resolve deck path: %v", err)
	}
	baseDir := filepath.Dir(deckPath)

	cfg, err := config.Load(deckPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("invalid config: %v", err)
	}

	content, err := os.ReadFile(deckPath)
	if err != nil {
		t.Fatalf("failed to read deck: %v", err)
	}

	p := parser.New()
	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("failed to parse deck: %v", err)
	}

	resolvedComponents, componentErrs := buildComponents(pres, baseDir, true, false)
	if len(componentErrs) > 0 {
		t.Fatalf("unexpected component build errors: %v", componentErrs)
	}

	outputDir := filepath.Join(t.TempDir(), "dist")
	b := builder.NewWithOutput(outputDir)
	b.SetBaseDir(baseDir)
	b.SetComponents(resolvedComponents)

	if _, err := b.Build(cfg, pres); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if entries, err := os.ReadDir(filepath.Join(outputDir, "components")); err != nil || len(entries) == 0 {
		t.Fatalf("expected dist/components to contain bundle files, err=%v entries=%v", err, entries)
	}

	// Mount the built folder under a URL sub path, exactly as a static
	// host serving this deck from e.g. example.com/talks/scaling/ would.
	const subPath = "/talks/scaling/"
	mux := http.NewServeMux()
	mux.Handle(subPath, http.StripPrefix(subPath, http.FileServer(http.Dir(outputDir))))
	server := httptest.NewServer(mux)
	defer server.Close()

	indexResponse, err := http.Get(server.URL + subPath + "index.html")
	if err != nil {
		t.Fatalf("failed to fetch index.html: %v", err)
	}
	defer indexResponse.Body.Close()
	if indexResponse.StatusCode != http.StatusOK {
		t.Fatalf("index.html status = %d, want 200", indexResponse.StatusCode)
	}
	indexBody, err := io.ReadAll(indexResponse.Body)
	if err != nil {
		t.Fatalf("failed to read index.html body: %v", err)
	}

	matches := componentURLPattern.FindAllStringSubmatch(string(indexBody), -1)
	if len(matches) == 0 {
		t.Fatal("expected at least one component bundle URL embedded in index.html")
	}

	seen := map[string]bool{}
	for _, match := range matches {
		url := match[1]
		if seen[url] {
			continue
		}
		seen[url] = true

		response, err := http.Get(server.URL + subPath + url)
		if err != nil {
			t.Fatalf("failed to fetch component URL %q: %v", url, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Errorf("component URL %q status = %d, want 200", url, response.StatusCode)
		}
	}
}

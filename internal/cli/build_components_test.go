package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
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

	resolvedComponents, componentErrs := buildComponents(pres, baseDir, true, false, "components/")
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

// htmlURLAttrPattern finds every src="..." or href="..." attribute in HTML.
var htmlURLAttrPattern = regexp.MustCompile(`(?:src|href)="([^"]+)"`)

// cssURLFuncPattern finds every url(...) reference in CSS (font files,
// background images), excluding data: URIs.
var cssURLFuncPattern = regexp.MustCompile(`url\(([^)]+)\)`)

// TestBuild_StaticAssetsHaveNoRootAbsoluteURLsUnderSubpath builds
// examples/theme-tour.md, the same way `tap build` does, then walks every
// URL referenced from the built index.html (script/link tags) and every CSS
// file those reference in turn (font url()s), asserting none is
// root-absolute ("/assets/...") and that each one resolves with status 200
// when the built folder is mounted under a URL sub path. A root-absolute
// URL works when a static site is deployed at the domain root but 404s the
// moment it's deployed under a sub path (a GitHub Pages project site, a
// reverse-proxy path prefix).
func TestBuild_StaticAssetsHaveNoRootAbsoluteURLsUnderSubpath(t *testing.T) {
	deckPath, err := filepath.Abs(filepath.Join("..", "..", "examples", "theme-tour.md"))
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

	outputDir := filepath.Join(t.TempDir(), "dist")
	b := builder.NewWithOutput(outputDir)
	b.SetBaseDir(baseDir)

	if _, err := b.Build(cfg, pres); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	const subPath = "/talks/theme-tour/"
	mux := http.NewServeMux()
	mux.Handle(subPath, http.StripPrefix(subPath, http.FileServer(http.Dir(outputDir))))
	server := httptest.NewServer(mux)
	defer server.Close()

	// fetchAndCollect fetches url under the sub path, fails the test if it
	// doesn't come back 200, and returns its body.
	fetchAndCollect := func(url string) []byte {
		if strings.HasPrefix(url, "/") {
			t.Errorf("root-absolute URL %q would 404 under a sub path deployment", url)
			return nil
		}
		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "data:") {
			return nil
		}

		response, err := http.Get(server.URL + subPath + url)
		if err != nil {
			t.Fatalf("failed to fetch %q: %v", url, err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Errorf("URL %q status = %d, want 200", url, response.StatusCode)
			return nil
		}
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatalf("failed to read body of %q: %v", url, err)
		}
		return body
	}

	indexBody := fetchAndCollect("index.html")
	if indexBody == nil {
		t.Fatal("index.html did not resolve under the sub path")
	}

	cssFiles := map[string]bool{}
	for _, match := range htmlURLAttrPattern.FindAllStringSubmatch(string(indexBody), -1) {
		url := match[1]
		body := fetchAndCollect(url)
		if strings.HasSuffix(url, ".css") {
			cssFiles[url] = true
			if body != nil {
				checkCSSURLs(t, fetchAndCollect, url, string(body))
			}
		}
	}
	if len(cssFiles) == 0 {
		t.Error("expected index.html to reference at least one CSS file")
	}
}

// checkCSSURLs extracts every url(...) reference from CSS content (font
// files) and fetches each one the same way fetchAndCollect does. A CSS
// url() is relative to the CSS file itself, not to the HTML page that
// references the CSS, so a bare "./font.woff2" reference is resolved
// against cssURL's own directory before fetching.
func checkCSSURLs(t *testing.T, fetch func(string) []byte, cssURL, css string) {
	t.Helper()
	dir := path.Dir(cssURL)
	for _, match := range cssURLFuncPattern.FindAllStringSubmatch(css, -1) {
		raw := strings.Trim(match[1], `"'`)
		if strings.HasPrefix(raw, "/") {
			t.Errorf("root-absolute CSS url(%q) in %q would 404 under a sub path deployment", raw, cssURL)
			continue
		}
		if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "data:") {
			continue
		}
		resolved := path.Join(dir, raw)
		fetch(resolved)
	}
}

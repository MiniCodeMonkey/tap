package pdf

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

type recordedProgress struct {
	mu        sync.Mutex
	downloads []string
	renders   []string
}

func (r *recordedProgress) Download(downloaded, totalBytes int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.downloads = append(r.downloads, fmt.Sprintf("%d/%d", downloaded, totalBytes))
}

func (r *recordedProgress) Render(done, total int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.renders = append(r.renders, fmt.Sprintf("%d/%d", done, total))
}

func TestInstallProgressWriterReportsPlaywrightDownloadLines(t *testing.T) {
	progress := &recordedProgress{}
	writer := &installProgressWriter{progress: progress}

	// Playwright's installer output when stdout is not a terminal, split
	// in the middle of a line the way a pipe can deliver it.
	chunks := []string{
		"Downloading Chromium 131.0.6778.33 (playwright build v1148) from https://playwright.azureedge.net/builds/chromium/1148/chromium-mac-arm64.zip\n",
		"|■■■■■■■■                                                                        |  10% of 162.3 MiB\n|■■■■■■■■■■■■",
		"■■■■                                                                |  20% of 162.3 MiB\r\n",
		"Chromium 131.0.6778.33 (playwright build v1148) downloaded to /Users/someone/Library/Caches/ms-playwright/chromium-1148\n",
	}
	for _, chunk := range chunks {
		if _, err := writer.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	mebibytes := 162.3
	totalBytes := int64(mebibytes * 1024 * 1024)
	want := []string{
		fmt.Sprintf("%d/%d", totalBytes*10/100, totalBytes),
		fmt.Sprintf("%d/%d", totalBytes*20/100, totalBytes),
	}
	if fmt.Sprint(progress.downloads) != fmt.Sprint(want) {
		t.Errorf("downloads = %v, want %v", progress.downloads, want)
	}
}

func TestInstallOptions(t *testing.T) {
	exporter, _ := New()
	if options := exporter.installOptions(); options.Stdout != nil || options.Stderr != nil {
		t.Error("without a Progress, the installer should keep its default output")
	}

	exporter.SetProgress(&recordedProgress{})
	options := exporter.installOptions()
	if _, ok := options.Stdout.(*installProgressWriter); !ok {
		t.Errorf("Stdout = %T, want *installProgressWriter", options.Stdout)
	}
	if options.Stderr != io.Discard || options.Logger == nil {
		t.Error("with a Progress, the installer's own text must stay off stderr")
	}
	if len(options.Browsers) != 1 || options.Browsers[0] != "chromium" {
		t.Errorf("Browsers = %v, want chromium", options.Browsers)
	}
}

func TestExportReportsRenderProgress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	presentation := &transformer.TransformedPresentation{
		Config: *config.DefaultConfig(),
		Slides: []transformer.TransformedSlide{
			{Index: 0, HTML: "<h1>Slide 1</h1>", Layout: "title"},
			{Index: 1, HTML: "<h1>Slide 2</h1>", Layout: "default"},
		},
	}
	srv := server.New(0)
	srv.SetPresentation(presentation)
	srv.SetupRoutes()
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Shutdown(context.Background())

	exporter, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer exporter.Close()
	progress := &recordedProgress{}
	exporter.SetProgress(progress)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	output := filepath.Join(t.TempDir(), "progress.pdf")
	if _, err := exporter.Export(ctx, "http://localhost:"+itoa(srv.Port()), ExportOptions{Content: ContentSlides, Output: output}); err != nil {
		if os.Getenv("CI") == "" {
			t.Skipf("skipping: export failed, likely no browser: %v", err)
		}
		t.Fatalf("Export() error = %v", err)
	}

	if fmt.Sprint(progress.renders) != "[1/2 2/2]" {
		t.Errorf("renders = %v, want [1/2 2/2]", progress.renders)
	}
}

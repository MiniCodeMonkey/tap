package pdf

import (
	"bytes"
	"io"
	"log/slog"
	"regexp"
	"strconv"

	"github.com/mxschmitt/playwright-go"
)

// Progress receives an export's progress. tap's --progress json reporter
// implements it.
type Progress interface {
	// Download reports the export browser's first-time download:
	// downloaded of totalBytes bytes of the archive being fetched.
	Download(downloaded, totalBytes int64)
	// Render reports that done of total slides or pages are rendered.
	Render(done, total int)
}

// SetProgress makes the exporter report to progress. A nil progress
// reports nothing.
func (e *Exporter) SetProgress(progress Progress) {
	e.progress = progress
}

// reportRender tells the Progress, if any, that done of total are rendered.
func (e *Exporter) reportRender(done, total int) {
	if e.progress != nil {
		e.progress.Render(done, total)
	}
}

// installOptions are the options for installing the export browser. With a
// Progress, the installer's download lines go to installProgressWriter and
// the rest of its text is dropped, so stdout keeps only the command's
// result and stderr only progress lines.
func (e *Exporter) installOptions() *playwright.RunOptions {
	options := &playwright.RunOptions{Browsers: []string{"chromium"}}
	if e.progress != nil {
		options.Stdout = &installProgressWriter{progress: e.progress}
		options.Stderr = io.Discard
		options.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return options
}

// installProgressLine matches the download lines Playwright's installer
// prints when its output is not a terminal, such as
// "|■■■■■■■■      |  10% of 162.3 MiB". One archive downloads at a time, so
// the percentage restarts for each one.
var installProgressLine = regexp.MustCompile(`(\d+)% of ([\d.]+) MiB`)

// installProgressWriter turns the installer's output into Download calls.
// Lines can arrive split across writes, so it keeps the unfinished line.
type installProgressWriter struct {
	progress Progress
	pending  []byte
}

func (w *installProgressWriter) Write(data []byte) (int, error) {
	w.pending = append(w.pending, data...)
	for {
		end := bytes.IndexAny(w.pending, "\r\n")
		if end < 0 {
			break
		}
		w.report(string(w.pending[:end]))
		w.pending = w.pending[end+1:]
	}
	return len(data), nil
}

func (w *installProgressWriter) report(line string) {
	match := installProgressLine.FindStringSubmatch(line)
	if match == nil {
		return
	}
	percent, err := strconv.Atoi(match[1])
	if err != nil {
		return
	}
	mebibytes, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		return
	}
	totalBytes := int64(mebibytes * 1024 * 1024)
	w.progress.Download(totalBytes*int64(percent)/100, totalBytes)
}

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/spf13/cobra"
)

// progressFormatJSON is the one --progress format: one JSON object per
// line on stderr.
const progressFormatJSON = "json"

// The phase of each --progress json line.
const (
	progressPhaseDownload = "download"
	progressPhaseRender   = "render"
	progressPhaseLoad     = "load"
	progressPhaseParse    = "parse"
	progressPhaseBundle   = "bundle"
	progressPhaseWrite    = "write"
	progressPhaseDone     = "done"
)

// progressReporter writes --progress json lines: a step line
// ({"phase","done","total"}) for each step, a download line
// ({"phase":"download","bytes","totalBytes"}) while the export browser
// downloads, and one final "done" line with the command's result. A
// reporter for a command run without --progress writes nothing, so callers
// never check first. A failed command's "done" line comes from execute
// (see writeProgressFailure).
type progressReporter struct {
	writer io.Writer
	mu     sync.Mutex
}

// newProgressReporter returns the reporter for a --progress value.
func newProgressReporter(format string, writer io.Writer) (*progressReporter, error) {
	switch format {
	case "":
		return &progressReporter{}, nil
	case progressFormatJSON:
		return &progressReporter{writer: writer}, nil
	default:
		return nil, userError(codeUsage, fmt.Errorf("unknown --progress format %q: the only format is json", format))
	}
}

// enabled reports whether this reporter writes anything.
func (p *progressReporter) enabled() bool {
	return p != nil && p.writer != nil
}

// progressStep is a step line.
type progressStep struct {
	Phase string `json:"phase"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

// progressDownload is a download line.
type progressDownload struct {
	Phase      string `json:"phase"`
	Bytes      int64  `json:"bytes"`
	TotalBytes int64  `json:"totalBytes"`
}

// progressFailure is the "done" line of a command that failed.
type progressFailure struct {
	Phase string    `json:"phase"`
	OK    bool      `json:"ok"`
	Error jsonError `json:"error"`
}

// Step reports that done of total units of phase are finished.
func (p *progressReporter) Step(phase string, done, total int) {
	p.writeValue(progressStep{Phase: phase, Done: done, Total: total})
}

// Render reports that done of total slides or pages are rendered. It lets
// a reporter serve as the export engine's pdf.Progress.
func (p *progressReporter) Render(done, total int) {
	p.Step(progressPhaseRender, done, total)
}

// Download reports the export browser's first-time download.
func (p *progressReporter) Download(downloaded, totalBytes int64) {
	p.writeValue(progressDownload{Phase: progressPhaseDownload, Bytes: downloaded, TotalBytes: totalBytes})
}

// Result writes the final line of a successful command:
// {"phase":"done","ok":true} followed by the fields of payload, the same
// fields as the command's --json result. payload must encode to a JSON
// object, or be nil.
func (p *progressReporter) Result(payload any) error {
	if !p.enabled() {
		return nil
	}
	line, err := jsonEnvelope("progress result", `{"phase":"done","ok":true`, payload, false)
	if err != nil {
		return err
	}
	p.writeLine(line)
	return nil
}

func (p *progressReporter) writeValue(value any) {
	if !p.enabled() {
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return
	}
	p.writeLine(encoded)
}

func (p *progressReporter) writeLine(line []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, _ = p.writer.Write(append(line, '\n'))
}

// progressRequested reports whether command was run with --progress json.
func progressRequested(command *cobra.Command) bool {
	if command == nil {
		return false
	}
	flag := command.Flags().Lookup("progress")
	return flag != nil && flag.Value.String() == progressFormatJSON
}

// writeProgressFailure writes the final line of a failed command.
func writeProgressFailure(writer io.Writer, code, message string) {
	encoded, err := json.Marshal(progressFailure{Phase: progressPhaseDone, Error: jsonError{Code: code, Message: message}})
	if err != nil {
		return
	}
	_, _ = writer.Write(append(encoded, '\n'))
}

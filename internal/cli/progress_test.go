package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

func TestProgressReporterWritesTheDocumentedLines(t *testing.T) {
	var output bytes.Buffer
	reporter, err := newProgressReporter("json", &output)
	if err != nil {
		t.Fatalf("newProgressReporter() error = %v", err)
	}

	reporter.Step(progressPhaseParse, 2, 4)
	reporter.Download(10, 100)
	reporter.Render(1, 3)
	if err := reporter.Result(struct {
		Output string `json:"output"`
		Pages  int    `json:"pages"`
	}{Output: "talk.pdf", Pages: 3}); err != nil {
		t.Fatalf("Result() error = %v", err)
	}

	want := strings.Join([]string{
		`{"phase":"parse","done":2,"total":4}`,
		`{"phase":"download","bytes":10,"totalBytes":100}`,
		`{"phase":"render","done":1,"total":3}`,
		`{"phase":"done","ok":true,"output":"talk.pdf","pages":3}`,
	}, "\n") + "\n"
	if output.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", output.String(), want)
	}
	checkProgressOutput(t, output.String())
}

func TestProgressReporterResultWithNoPayload(t *testing.T) {
	var output bytes.Buffer
	reporter, _ := newProgressReporter("json", &output)
	if err := reporter.Result(nil); err != nil {
		t.Fatalf("Result(nil) error = %v", err)
	}
	if output.String() != `{"phase":"done","ok":true}`+"\n" {
		t.Errorf("output = %q", output.String())
	}
}

func TestProgressReporterWithoutProgressWritesNothing(t *testing.T) {
	var output bytes.Buffer
	reporter, err := newProgressReporter("", &output)
	if err != nil {
		t.Fatalf("newProgressReporter() error = %v", err)
	}
	reporter.Step(progressPhaseRender, 1, 1)
	reporter.Download(1, 2)
	_ = reporter.Result(struct{}{})
	if reporter.enabled() || output.Len() != 0 {
		t.Errorf("a reporter without --progress wrote %q", output.String())
	}
}

func TestNewProgressReporterRejectsAnUnknownFormat(t *testing.T) {
	_, err := newProgressReporter("xml", &bytes.Buffer{})
	if err == nil {
		t.Fatal("newProgressReporter(xml) = nil error")
	}
	if exitCode, code, _ := classify(err); exitCode != exitUserError || code != codeUsage {
		t.Errorf("classify() = (%d, %q), want (%d, %q)", exitCode, code, exitUserError, codeUsage)
	}
}

func TestProgressReporterWriteLineIsSafeForConcurrentWriters(t *testing.T) {
	var output bytes.Buffer
	reporter, err := newProgressReporter("json", &output)
	if err != nil {
		t.Fatalf("newProgressReporter() error = %v", err)
	}

	const steps = 200
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		for i := 0; i < steps; i++ {
			reporter.Step(progressPhaseLoad, i, steps)
		}
	}()
	go func() {
		defer group.Done()
		for i := 0; i < steps; i++ {
			reporter.Step(progressPhaseBundle, i, steps)
		}
	}()
	group.Wait()

	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 2*steps {
		t.Fatalf("got %d lines, want %d: writes interleaved, truncating or merging lines", len(lines), 2*steps)
	}
	for _, line := range lines {
		var fields map[string]any
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatalf("line is not a complete JSON object: %v: %q", err, line)
		}
	}
}

func TestExecuteEndsAFailedProgressRunWithADoneLine(t *testing.T) {
	root := &cobra.Command{Use: "tap", SilenceErrors: true, SilenceUsage: true}
	var format string
	work := &cobra.Command{
		Use: "work",
		RunE: func(*cobra.Command, []string) error {
			return userError(codeDeckNotFound, errors.New("no deck here"))
		},
	}
	work.Flags().StringVar(&format, "progress", "", "")
	root.AddCommand(work)

	var stdout, stderr bytes.Buffer
	exitCode := execute(root, []string{"work", "--progress", "json"}, &stdout, &stderr)

	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
	want := `{"phase":"done","ok":false,"error":{"code":"deck_not_found","message":"no deck here"}}` + "\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want only the done line %q", stderr.String(), want)
	}
	checkProgressOutput(t, stderr.String())
}

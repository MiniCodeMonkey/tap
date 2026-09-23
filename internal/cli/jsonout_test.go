package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPrintJSONOKPutsOKFirst(t *testing.T) {
	var out bytes.Buffer
	payload := struct {
		Output string `json:"output"`
		Pages  int    `json:"pages"`
	}{"talk.pdf", 12}
	if err := printJSONOK(&out, payload); err != nil {
		t.Fatalf("printJSONOK() error = %v", err)
	}
	want := "{\n  \"ok\": true,\n  \"output\": \"talk.pdf\",\n  \"pages\": 12\n}\n"
	if out.String() != want {
		t.Errorf("printJSONOK() wrote %q, want %q", out.String(), want)
	}
}

func TestPrintJSONOKWithNoPayload(t *testing.T) {
	var out bytes.Buffer
	if err := printJSONOK(&out, nil); err != nil {
		t.Fatalf("printJSONOK() error = %v", err)
	}
	if out.String() != "{\n  \"ok\": true\n}\n" {
		t.Errorf("printJSONOK(nil) wrote %q", out.String())
	}
}

func TestPrintJSONOKRejectsANonObject(t *testing.T) {
	var out bytes.Buffer
	err := printJSONOK(&out, []string{"a"})
	if err == nil {
		t.Fatal("printJSONOK() with an array should fail")
	}
	if exitCode, _, _ := classify(err); exitCode != exitInternal {
		t.Errorf("exit code = %d, want %d", exitCode, exitInternal)
	}
}

func TestPrintJSONError(t *testing.T) {
	var out bytes.Buffer
	if err := printJSONError(&out, codeNoDeck, "no deck found"); err != nil {
		t.Fatalf("printJSONError() error = %v", err)
	}
	var decoded struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if decoded.OK || decoded.Error.Code != codeNoDeck || decoded.Error.Message != "no deck found" {
		t.Errorf("decoded = %+v", decoded)
	}
}

func TestJSONRequested(t *testing.T) {
	var enabled bool
	command := &cobra.Command{Use: "test"}
	command.Flags().BoolVar(&enabled, "json", false, "")
	if jsonRequested(command) {
		t.Error("jsonRequested() = true before --json is set")
	}
	_ = command.Flags().Set("json", "true")
	if !jsonRequested(command) {
		t.Error("jsonRequested() = false after --json is set")
	}
	if jsonRequested(&cobra.Command{Use: "plain"}) {
		t.Error("jsonRequested() = true for a command with no --json flag")
	}
	if jsonRequested(nil) {
		t.Error("jsonRequested(nil) = true")
	}
}

func TestExecutePrintsAJSONErrorForAJSONCommand(t *testing.T) {
	exitCode, stdout, stderr := runTap(t, "theme", "show", "no-such-theme-or-deck", "--json")
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty: --json errors go to stdout only", stderr)
	}
	if !strings.Contains(stdout, `"ok": false`) || !strings.Contains(stdout, `"code": "unknown_theme"`) {
		t.Errorf("stdout = %q, want a JSON error with code unknown_theme", stdout)
	}
}

func TestPrintJSONOKDoesNotEscapeHTML(t *testing.T) {
	var out bytes.Buffer
	payload := struct {
		Markdown string `json:"markdown"`
	}{"<!-- ai-prompt: a red fox --> & more"}
	if err := printJSONOK(&out, payload); err != nil {
		t.Fatalf("printJSONOK() error = %v", err)
	}
	if strings.Contains(out.String(), "\\u003c") || strings.Contains(out.String(), "\\u0026") {
		t.Errorf("printJSONOK() escaped HTML: %q", out.String())
	}
	if !strings.Contains(out.String(), "<!-- ai-prompt: a red fox --> & more") {
		t.Errorf("printJSONOK() wrote %q, want the literal markdown", out.String())
	}
}

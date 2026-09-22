# CLI command tree and driver registry: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move tap to the noun-grouped command tree with one set of conventions (deck resolver, flags, `--json`, exit codes, 1-based numbers), and connect the live code driver registry in `tap dev` and `tap present`.

**Architecture:** Commands return errors instead of calling `os.Exit`. One `execute` function turns every error into an exit code (0, 1, 2 or 130) and, for `--json`, into one error object on stdout. One `resolveDeck` function picks the deck for every command that takes `[deck]`. Old command names become hidden stubs that print one line and exit 1. `runDevServer` builds a `driver.Registry` from the built-in drivers and the deck's `drivers:` map, and rebuilds it on every reload. `/api/execute` runs only code that is a live block in the loaded deck. `tap dev` and `tap present` listen on `127.0.0.1` unless `--lan` is given.

**Tech Stack:** Go 1.24, cobra, pflag, Bubble Tea (the file picker), `net/http` for the server.

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-prerequisites-design.md`, "Part 1: CLI command tree and conventions" and section 2.1. The findings behind part 1 are in `docs/superpowers/specs/tap-desktop-features/cli-review.md`. Both files are on the `docs/tap-desktop` branch.

## Global Constraints

- Command tree after this plan: `new [deck]`, `dev [deck]`, `present [deck]`, `build [deck]`, `serve [dir]`, `export pdf [deck]`, `export images [deck]`, `slide add [deck]`, `component new <Name> [deck]`, `theme list`, `theme show [slug|deck]`.
- Do not build `slide list`, `deck schema`, `theme set`, `image add|generate|regenerate`, or `approval list|revoke`. Parts 2 to 4 add them. This plan only adds the resolver, the error types and the JSON helper that they reuse.
- No aliases. An old name exits with code 1 and prints exactly one line, for example `tap pdf was renamed: use tap export pdf`. It does nothing else.
- Flags: `--output/-o`, `--theme/-t`, `--port/-p`, `--yes/-y`, `--json`. A flag has the same name and short form on every command. `--out`, `--deck` and the global `--verbose` are removed.
- Numbers a user types are 1-based. Leaving `--step` or `--fragment` out means the final state.
- `--json` prints one object: `{"ok": true, ...}` or `{"ok": false, "error": {"code": "...", "message": "..."}}`.
- Exit codes: 0 success, 1 user error, 2 internal error, 130 interrupt.
- `tap build` output stays static, with no code execution.
- This lands before 2.0.0 is final (2.0 is at rc.1).
- Code comments describe the present. No ticket numbers or "before the fix" wording in code.
- Spell identifiers out in full (`options`, not `opts`).
- No em dashes in docs, changelog or comments. Use `--`, a comma, or a new sentence.
- A `--json` result struct declares its fields in output order (`encoding/json` keeps struct order). If the `fieldalignment` linter complains, keep the order and add `//nolint:govet // fieldalignment: field order is the JSON output order` above the struct, as `internal/driver/custom.go` does.

## Decisions this plan makes

The spec leaves these open. Each one is marked **(plan decision)** where it is used. The user approved all eleven as written on 2026-09-22.

1. **Which `.md` files are decks.** The resolver counts every `.md` file in the folder except `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md` and `LICENSE.md` (any case). This is the exclusion list `tap add` uses today.
2. **Esc in the deck picker** exits 130 with no message. Today it exits 0.
3. **`tap dev`, `tap present` and `tap serve` exit 0 on Ctrl+C.** Ctrl+C is their normal way to stop. Exit 130 is for commands that Ctrl+C cuts short: `export pdf`, `export images`, and the deck picker.
4. **`--step` and `--fragment` count reveals.** `--step 2` is the state after two steps. `--fragment 1` is the state with the first fragment shown. `0` is allowed for both and means "before the first reveal". Valid ranges are `0..steps` and `0..fragments`. When one flag is given and the other is left out, the one left out takes its final value.
5. **`tap theme show [slug|deck]`.** The one positional is a theme slug when it names a built-in theme. Otherwise it is a deck. With no positional, the resolver picks the deck in the current folder.
6. **`tap new [deck]`.** The positional is the path of the deck to create. It means the same as `--output`. Giving both is exit 1.
7. **`tap component new <Name> [deck]`** needs only the deck's folder. A folder argument is used as is, a file argument gives its folder, and no argument means the current folder. It does not open the picker.
8. **`--json` on `tap new` skips the wizard,** as `--yes` does.
9. **`--json` in part 1** goes on `new`, `build`, `export pdf`, `export images`, `component new`, `theme list` and `theme show`. The long-running commands (`dev`, `present`, `serve`) and the `slide add` wizard get no `--json`.
10. **`component new --json` prints the snippet as a field now.** Part 4 lists this, but it costs one struct field here, and it avoids a second change to the output shape.
11. **The execute guard (Task 10).** See the next section. **Settled:** the user chose "guard and bind 127.0.0.1" for PR 2.

## Safety: the registry must not ship without a guard

The dev server binds `0.0.0.0` (`server.New`), and `--tunnel` puts it on a public URL. The cross-site guard from PR #14 stops browsers, but `curl` can send any `Origin` and `Host` header it likes. Today `/api/execute` answers 500 in every run, so nothing is exposed. When the registry is connected, any client on the network, or anyone with the tunnel URL, can post `{"driver": "shell", "code": "..."}` and run commands on the speaker's machine.

Section 2.2 (execute by reference) closes this, but 2.2 is not part of this plan. So Task 10 adds a small guard first: `/api/execute` runs a request only when its driver, connection and code are equal to a live code block in the loaded deck. Otherwise it answers 403. The frontend does not change, because it already sends the block's own code. Section 2.2 later replaces the guard with the `{slide, block}` reference.

The guard alone still lets any machine on the network run the deck's own blocks. So Task 11 also changes where the server listens: `tap dev` and `tap present` bind `127.0.0.1` by default, and bind `0.0.0.0` only with the new `--lan` flag. The phone remote over the local network needs `--lan`. `--tunnel` works without it, because `cloudflared` connects to `http://127.0.0.1:<port>` (`internal/tunnel/tunnel.go`).

Tasks 10 and 11 land before Task 12, so no commit connects the registry without both protections. The user chose this approach ("guard and bind 127.0.0.1") for PR 2.

## Pull requests

- **PR 1:** Tasks 1 to 9 and Task 13 (part 1). This PR must merge before 2.0.0 is final.
- **PR 2:** Tasks 10, 11 and 12 (section 2.1), with their changelog and docs lines. It can merge after PR 1, or with it. It changes the default network exposure, so it should also land before 2.0.0 is final.

`tap present` is already on main (commit 280372c, PR #15). It shares `runDevServer` with `tap dev`, so it gets the resolver in Task 8, the loopback default in Task 11 and the registry in Task 12 with no extra work.

The TUI `r` key already reloads the deck in `tap dev` on main: `DevModel.handleKeyPress` calls `m.reloadCmd()`, and `runDevServer` passes `reloadInTUI` to `SetReloader` for both commands. `TestDevModel_HandleKeyPress_Reload` in `internal/tui/dev_test.go` covers it. No task is needed. Task 9 only runs that test again.

## File structure

| File | Status | Responsibility |
|---|---|---|
| `internal/cli/exit.go` | new | Exit codes, error codes, `commandError`, `classify` |
| `internal/cli/root.go` | modify | Root command, `Execute`, `execute`; `--verbose` removed |
| `internal/cli/jsonout.go` | new | `printJSONOK`, `printJSONError`, `jsonRequested` |
| `internal/cli/resolve.go` | new | `resolveDeck`, `deckCandidates`, `resolveDeckFolder`, `firstArg` |
| `internal/cli/export.go` | new | The `tap export` parent command |
| `internal/cli/export_pdf.go` | renamed from `pdf.go` | `tap export pdf` |
| `internal/cli/export_images.go` | renamed from `screenshot.go` | `tap export images` |
| `internal/cli/slide.go` | renamed from `add.go` | `tap slide` and `tap slide add` |
| `internal/cli/component.go` | renamed from `add_component.go` | `tap component` and `tap component new` |
| `internal/cli/renamed.go` | new | Hidden stubs for `pdf`, `screenshot`, `add` |
| `internal/cli/drivers.go` | new | `buildDriverRegistry` |
| `internal/cli/theme.go`, `new.go`, `build.go`, `dev.go`, `present.go`, `serve.go` | modify | Resolver, `RunE`, `--json` |
| `internal/tui/filepicker.go` | modify | `RunFilePickerWith(files)`; `RunFilePicker` and `RenderNoFilesError` removed |
| `internal/tui/dev.go` | modify | The PDF key runs `tap export pdf` |
| `internal/driver/custom.go` | modify | `DriverConfigInput.WorkingDir` |
| `internal/server/api.go` | modify | The live block guard |
| `internal/cli/network.go` | new | `listenHost`, `lanPresenterAddress` for `--lan` |
| `internal/server/routes.go` | modify | `/qr` explains `--lan` on a loopback-only server |
| `internal/tui/dev.go` | modify | `DevConfig.NetworkURL`, shown with the LAN QR code |
| Docs | modify | `README.md`, `CONTRIBUTING.md`, `docs/reference/*.md`, `docs/guide/*.md`, `skills/tap/**`, `CHANGELOG.md` |

Test files move with their source files: `pdf_test.go` to `export_pdf_test.go`, `screenshot_test.go` to `export_images_test.go`, `add_component_test.go` to `component_test.go`.

---

### Task 1: Exit codes and error types

**Files:**
- Create: `internal/cli/exit.go`
- Create: `internal/cli/exit_test.go`
- Create: `internal/cli/run_tap_test.go`
- Modify: `internal/cli/root.go`

**Interfaces:**
- Produces:
  - constants `exitOK = 0`, `exitUserError = 1`, `exitInternal = 2`, `exitInterrupted = 130`
  - error code constants (strings): `codeFailed`, `codeUsage`, `codeDeckNotFound`, `codeNoDeck`, `codeAmbiguousDeck`, `codeInvalidDeck`, `codeUnknownTheme`, `codeOutOfRange`, `codeExists`, `codeComponentBuild`, `codeBrokenSlides`, `codeBrowser`, `codeExportFailed`, `codeRenamed`, `codeNeedsTerminal`, `codeInterrupted`, `codeCancelled`, `codeInternal`
  - `func userError(code string, err error) error`
  - `func internalError(code string, err error) error`
  - `func reportedError(code string, err error) error` (exit 1; the command already printed its diagnostics)
  - `var errCancelled error`
  - `func classify(err error) (exitCode int, code string, reported bool)`
  - `func execute(root *cobra.Command, args []string, stdout, stderr io.Writer) int`
  - test helper `func runTap(t *testing.T, args ...string) (exitCode int, stdout, stderr string)`

- [ ] **Step 1: Write the failing tests**

`internal/cli/run_tap_test.go`:

```go
package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// runTap runs the real root command in-process with args, and returns the
// exit code and what it wrote. Flag values live in package variables that
// cobra never resets, so every flag goes back to its default afterward.
func runTap(t *testing.T, args ...string) (exitCode int, stdout, stderr string) {
	t.Helper()
	t.Cleanup(func() { resetAllFlags(rootCmd) })
	var out, errOut bytes.Buffer
	exitCode = execute(rootCmd, args, &out, &errOut)
	return exitCode, out.String(), errOut.String()
}

// resetAllFlags sets every flag of command and its children back to its
// default value.
func resetAllFlags(command *cobra.Command) {
	reset := func(flag *pflag.Flag) {
		if sliceValue, ok := flag.Value.(pflag.SliceValue); ok {
			_ = sliceValue.Replace(nil)
		} else {
			_ = flag.Value.Set(flag.DefValue)
		}
		flag.Changed = false
	}
	command.Flags().VisitAll(reset)
	command.PersistentFlags().VisitAll(reset)
	for _, child := range command.Commands() {
		resetAllFlags(child)
	}
}
```

`internal/cli/exit_test.go`:

```go
package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantExitCode int
		wantCode     string
		wantReported bool
	}{
		{"plain error is a user error", errors.New("boom"), exitUserError, codeFailed, false},
		{"user error", userError(codeNoDeck, errors.New("no deck")), exitUserError, codeNoDeck, false},
		{"wrapped user error keeps its code", fmt.Errorf("loading: %w", userError(codeInvalidDeck, errors.New("bad"))), exitUserError, codeInvalidDeck, false},
		{"internal error", internalError(codeBrowser, errors.New("no chromium")), exitInternal, codeBrowser, false},
		{"reported error", reportedError(codeComponentBuild, errors.New("2 components failed")), exitUserError, codeComponentBuild, true},
		{"interrupt", errInterrupted, exitInterrupted, codeInterrupted, false},
		{"cancelled picker", errCancelled, exitInterrupted, codeCancelled, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode, code, reported := classify(tt.err)
			if exitCode != tt.wantExitCode || code != tt.wantCode || reported != tt.wantReported {
				t.Errorf("classify() = (%d, %q, %v), want (%d, %q, %v)",
					exitCode, code, reported, tt.wantExitCode, tt.wantCode, tt.wantReported)
			}
		})
	}
}

func TestExecuteUnknownFlagIsAUsageError(t *testing.T) {
	exitCode, stdout, stderr := runTap(t, "build", "--no-such-flag")
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "unknown flag: --no-such-flag") {
		t.Errorf("stderr = %q, want it to name the unknown flag", stderr)
	}
}

func TestVerboseFlagIsRemoved(t *testing.T) {
	if rootCmd.PersistentFlags().Lookup("verbose") != nil {
		t.Error("the global --verbose flag should be removed")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestClassify|TestExecuteUnknownFlag|TestVerboseFlagIsRemoved' -short`
Expected: compile failure, `undefined: classify`, `undefined: execute`, and others.

- [ ] **Step 3: Write `internal/cli/exit.go`**

```go
package cli

import (
	"errors"
)

// Exit codes. Every tap command exits with one of these.
const (
	exitOK          = 0
	exitUserError   = 1
	exitInternal    = 2
	exitInterrupted = 130
)

// Error codes for the "code" field of a --json error. Parts of tap that
// add commands add their own codes here.
const (
	codeFailed         = "failed"
	codeUsage          = "usage"
	codeDeckNotFound   = "deck_not_found"
	codeNoDeck         = "no_deck"
	codeAmbiguousDeck  = "ambiguous_deck"
	codeInvalidDeck    = "invalid_deck"
	codeUnknownTheme   = "unknown_theme"
	codeOutOfRange     = "out_of_range"
	codeExists         = "exists"
	codeComponentBuild = "component_build"
	codeBrokenSlides   = "broken_slides"
	codeBrowser        = "browser"
	codeExportFailed   = "export_failed"
	codeRenamed        = "renamed"
	codeNeedsTerminal  = "needs_terminal"
	codeInterrupted    = "interrupted"
	codeCancelled      = "cancelled"
	codeInternal       = "internal"
)

// errCancelled means the person closed an interactive prompt, such as the
// deck picker, without choosing. It exits 130 and prints nothing.
var errCancelled = errors.New("cancelled")

// commandError carries the exit code and the --json error code of a
// failed command. reported is true when the command already printed its
// own diagnostics to standard error, so execute prints nothing more there.
type commandError struct {
	err      error
	code     string
	exitCode int
	reported bool
}

func (e *commandError) Error() string { return e.err.Error() }
func (e *commandError) Unwrap() error { return e.err }

// userError is a failure the person can fix: a missing deck, a bad flag,
// an invalid frontmatter key. It exits 1.
func userError(code string, err error) error {
	return &commandError{err: err, code: code, exitCode: exitUserError}
}

// internalError is a failure in tap or its environment: the browser does
// not start, a temporary server cannot bind. It exits 2.
func internalError(code string, err error) error {
	return &commandError{err: err, code: code, exitCode: exitInternal}
}

// reportedError is a user error whose details the command has already
// printed to standard error, one line per problem. It exits 1.
func reportedError(code string, err error) error {
	return &commandError{err: err, code: code, exitCode: exitUserError, reported: true}
}

// classify returns the exit code, the --json error code, and whether the
// details were already printed, for any error a command returns.
func classify(err error) (exitCode int, code string, reported bool) {
	var commandErr *commandError
	switch {
	case errors.Is(err, errInterrupted):
		return exitInterrupted, codeInterrupted, false
	case errors.Is(err, errCancelled):
		return exitInterrupted, codeCancelled, true
	case errors.As(err, &commandErr):
		return commandErr.exitCode, commandErr.code, commandErr.reported
	case errors.Is(err, errSilent):
		return exitUserError, codeFailed, true
	default:
		return exitUserError, codeFailed, false
	}
}
```

`errSilent` and `errInterrupted` stay in `screenshot.go` for now. Tasks 4 and 5 replace `errSilent` with `reportedError` and delete it. At that point, remove the `errors.Is(err, errSilent)` case from `classify`.

- [ ] **Step 4: Change `internal/cli/root.go`**

Delete `var verbose bool`, the `PersistentFlags().BoolVarP(&verbose, ...)` line in `init`, and the `Verbose()` function. Nothing else reads them.

Replace `Execute` with:

```go
// Execute runs tap with the process arguments and returns its exit code.
func Execute() int {
	return execute(rootCmd, os.Args[1:], os.Stdout, os.Stderr)
}

// execute runs root with args and turns the result into an exit code. It
// is the only place that prints a failed command's error, so every
// command reports errors the same way.
func execute(root *cobra.Command, args []string, stdout, stderr io.Writer) int {
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	_, err := root.ExecuteC()
	if err == nil {
		return exitOK
	}

	exitCode, _, reported := classify(err)
	switch {
	case reported:
	case errors.Is(err, errInterrupted):
		fmt.Fprintln(stderr, "interrupted")
	default:
		errorColor.Fprintln(stderr, "Error:", err)
	}
	return exitCode
}
```

In `init`, mark flag errors as usage errors:

```go
	rootCmd.SetFlagErrorFunc(func(command *cobra.Command, err error) error {
		return userError(codeUsage, err)
	})
```

Update the imports of `root.go` to `errors`, `fmt`, `io`, `os`, and `github.com/spf13/cobra`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestClassify|TestExecuteUnknownFlag|TestVerboseFlagIsRemoved|TestDisplayVersion|TestPresentCommandIsRegistered' -short -v`
Expected: PASS.

Run: `go build ./... && go vet ./internal/cli`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/exit.go internal/cli/exit_test.go internal/cli/run_tap_test.go internal/cli/root.go
git commit -m "refactor(cli): map every command error to one exit code, drop --verbose"
```

---

### Task 2: JSON output helper

**Files:**
- Create: `internal/cli/jsonout.go`
- Create: `internal/cli/jsonout_test.go`
- Modify: `internal/cli/root.go` (`execute`)

**Interfaces:**
- Consumes: `classify`, `execute` (Task 1)
- Produces:
  - `func printJSONOK(w io.Writer, payload any) error`: `payload` must encode to a JSON object or be `nil`. The output is `{"ok": true, <payload fields in order>}`, indented by two spaces, with a newline.
  - `func printJSONError(w io.Writer, code, message string) error`
  - `func jsonRequested(command *cobra.Command) bool`: true when `command` has a `--json` flag set to true.

- [ ] **Step 1: Write the failing tests**

`internal/cli/jsonout_test.go`:

```go
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
```

`TestExecutePrintsAJSONErrorForAJSONCommand` passes only after Task 7 gives `theme show` its codes. Mark it with `t.Skip("theme show gets error codes in Task 7")` in this task, and remove the skip in Task 7.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestPrintJSON|TestJSONRequested' -short`
Expected: compile failure, `undefined: printJSONOK`.

- [ ] **Step 3: Write `internal/cli/jsonout.go`**

```go
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// printJSONOK writes the --json result of a successful command:
// {"ok": true} followed by the fields of payload, in their declared order.
// payload must encode to a JSON object, or be nil for no fields.
func printJSONOK(w io.Writer, payload any) error {
	body := []byte("{}")
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return internalError(codeInternal, fmt.Errorf("encoding the JSON result: %w", err))
		}
		body = encoded
	}
	if len(body) < 2 || body[0] != '{' {
		return internalError(codeInternal, fmt.Errorf("the JSON result must be an object, got %s", body))
	}

	var combined bytes.Buffer
	combined.WriteString(`{"ok":true`)
	if rest := body[1:]; string(rest) == "}" {
		combined.WriteByte('}')
	} else {
		combined.WriteByte(',')
		combined.Write(rest)
	}

	var indented bytes.Buffer
	if err := json.Indent(&indented, combined.Bytes(), "", "  "); err != nil {
		return internalError(codeInternal, fmt.Errorf("formatting the JSON result: %w", err))
	}
	indented.WriteByte('\n')
	_, err := w.Write(indented.Bytes())
	return err
}

// jsonError is the "error" field of a failed command's --json result.
type jsonError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// printJSONError writes the --json result of a failed command.
func printJSONError(w io.Writer, code, message string) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(struct {
		OK    bool      `json:"ok"`
		Error jsonError `json:"error"`
	}{Error: jsonError{Code: code, Message: message}})
}

// jsonRequested reports whether command has a --json flag that is set.
func jsonRequested(command *cobra.Command) bool {
	if command == nil {
		return false
	}
	flag := command.Flags().Lookup("json")
	return flag != nil && flag.Value.String() == "true"
}
```

- [ ] **Step 4: Make `execute` print JSON errors**

In `root.go`, change `execute` to keep the command that ran, and print a JSON error for a `--json` command:

```go
	command, err := root.ExecuteC()
	if err == nil {
		return exitOK
	}

	exitCode, code, reported := classify(err)
	if jsonRequested(command) {
		_ = printJSONError(stdout, code, err.Error())
		return exitCode
	}
	switch {
	case reported:
	case errors.Is(err, errInterrupted):
		fmt.Fprintln(stderr, "interrupted")
	default:
		errorColor.Fprintln(stderr, "Error:", err)
	}
	return exitCode
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestPrintJSON|TestJSONRequested|TestClassify|TestExecute' -short -v`
Expected: PASS (and the skipped test shows SKIP).

- [ ] **Step 6: Commit**

```bash
git add internal/cli/jsonout.go internal/cli/jsonout_test.go internal/cli/root.go
git commit -m "feat(cli): add one --json result shape for every command"
```

---

### Task 3: The shared deck resolver

**Files:**
- Create: `internal/cli/resolve.go`
- Create: `internal/cli/resolve_test.go`
- Modify: `internal/tui/filepicker.go`
- Modify: `internal/tui/filepicker_test.go`

**Interfaces:**
- Consumes: `userError`, `internalError`, `errCancelled`, the error codes (Task 1); `stdinIsTerminal` (existing, `internal/cli/new.go`)
- Produces:
  - `func resolveDeck(arg string) (string, error)`: returns the deck file path.
  - `type deckResolver struct { interactive func() bool; pick deckPicker }` and `func (r deckResolver) resolve(arg string) (string, error)`
  - `type deckPicker func(candidates []string) (chosen string, cancelled bool, err error)`
  - `func deckCandidates(folder string) ([]string, error)`
  - `func resolveDeckFolder(arg string) (string, error)`
  - `func firstArg(args []string) string`
  - In `internal/tui`: `func NewFilePickerModelWith(files []string) FilePickerModel` and `func RunFilePickerWith(files []string) (FilePickerResult, error)`

- [ ] **Step 1: Write the failing tests**

`internal/cli/resolve_test.go`:

```go
package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeDecks creates each named file in dir, one second apart in
// modification time so their order is fixed (newest last).
func writeDecks(t *testing.T, dir string, names ...string) {
	t.Helper()
	base := time.Now().Add(-time.Hour)
	for index, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
		modified := base.Add(time.Duration(index) * time.Second)
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
}

func noPicker(t *testing.T) deckPicker {
	return func(candidates []string) (string, bool, error) {
		t.Fatalf("the picker should not open, candidates %v", candidates)
		return "", false, nil
	}
}

func TestResolveDeckUsesAFileArgument(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "talk.md")
	resolver := deckResolver{interactive: func() bool { return true }, pick: noPicker(t)}
	got, err := resolver.resolve(filepath.Join(dir, "talk.md"))
	if err != nil || got != filepath.Join(dir, "talk.md") {
		t.Errorf("resolve() = (%q, %v)", got, err)
	}
}

func TestResolveDeckMissingArgument(t *testing.T) {
	resolver := deckResolver{interactive: func() bool { return true }, pick: noPicker(t)}
	_, err := resolver.resolve(filepath.Join(t.TempDir(), "missing.md"))
	if _, code, _ := classify(err); code != codeDeckNotFound {
		t.Errorf("code = %q, want %q (err %v)", code, codeDeckNotFound, err)
	}
}

func TestResolveDeckTheOnlyDeckInAFolder(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "README.md", "talk.md", "notes.txt")
	resolver := deckResolver{interactive: func() bool { return false }, pick: noPicker(t)}
	got, err := resolver.resolve(dir)
	if err != nil || got != filepath.Join(dir, "talk.md") {
		t.Errorf("resolve() = (%q, %v), want the only deck, skipping README.md", got, err)
	}
}

func TestResolveDeckTheOnlyDeckInTheCurrentFolder(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "talk.md")
	resolver := deckResolver{interactive: func() bool { return false }, pick: noPicker(t)}
	withWorkingDirectory(t, dir, func() {
		got, err := resolver.resolve("")
		if err != nil || got != "talk.md" {
			t.Errorf("resolve(\"\") = (%q, %v), want talk.md", got, err)
		}
	})
}

func TestResolveDeckNoDeck(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "README.md")
	resolver := deckResolver{interactive: func() bool { return true }, pick: noPicker(t)}
	_, err := resolver.resolve(dir)
	if _, code, _ := classify(err); code != codeNoDeck {
		t.Errorf("code = %q, want %q (err %v)", code, codeNoDeck, err)
	}
}

func TestResolveDeckPicksOnATerminal(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "a.md", "b.md")
	var offered []string
	resolver := deckResolver{
		interactive: func() bool { return true },
		pick: func(candidates []string) (string, bool, error) {
			offered = candidates
			return candidates[1], false, nil
		},
	}
	got, err := resolver.resolve(dir)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}
	wantOffered := []string{filepath.Join(dir, "b.md"), filepath.Join(dir, "a.md")}
	if strings.Join(offered, ",") != strings.Join(wantOffered, ",") {
		t.Errorf("picker offered %v, want newest first %v", offered, wantOffered)
	}
	if got != filepath.Join(dir, "a.md") {
		t.Errorf("resolve() = %q, want the picked deck", got)
	}
}

func TestResolveDeckPickerCancelled(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "a.md", "b.md")
	resolver := deckResolver{
		interactive: func() bool { return true },
		pick:        func([]string) (string, bool, error) { return "", true, nil },
	}
	if _, err := resolver.resolve(dir); !errors.Is(err, errCancelled) {
		t.Errorf("resolve() error = %v, want errCancelled", err)
	}
}

func TestResolveDeckWithoutATerminalListsCandidates(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "a.md", "b.md")
	resolver := deckResolver{interactive: func() bool { return false }, pick: noPicker(t)}
	_, err := resolver.resolve(dir)
	exitCode, code, _ := classify(err)
	if exitCode != exitUserError || code != codeAmbiguousDeck {
		t.Fatalf("classify() = (%d, %q), want (1, %q)", exitCode, code, codeAmbiguousDeck)
	}
	for _, name := range []string{"a.md", "b.md"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not list %s", err, name)
		}
	}
}

func TestResolveDeckFolder(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "talk.md")
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{"no argument is the current folder", "", "."},
		{"a folder is used as is", dir, dir},
		{"a file gives its folder", filepath.Join(dir, "talk.md"), dir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDeckFolder(tt.arg)
			if err != nil || got != tt.want {
				t.Errorf("resolveDeckFolder(%q) = (%q, %v), want %q", tt.arg, got, err, tt.want)
			}
		})
	}
	if _, err := resolveDeckFolder(filepath.Join(dir, "missing")); err == nil {
		t.Error("resolveDeckFolder() of a missing path should fail")
	}
}
```

`withWorkingDirectory` already exists in `add_component_test.go` (it moves to `component_test.go` in Task 6).

In `internal/tui/filepicker_test.go`, add:

```go
func TestNewFilePickerModelWithKeepsTheGivenOrder(t *testing.T) {
	m := NewFilePickerModelWith([]string{"talks/b.md", "talks/a.md"})
	if !m.HasFiles() {
		t.Fatal("HasFiles() = false")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := updated.(FilePickerModel).GetResult().File; got != "talks/b.md" {
		t.Errorf("selected %q, want the first file given", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli ./internal/tui -run 'TestResolveDeck|TestNewFilePickerModelWith' -short`
Expected: compile failure, `undefined: deckResolver` and `undefined: NewFilePickerModelWith`.

- [ ] **Step 3: Add the picker entry points in `internal/tui/filepicker.go`**

Add below `NewFilePickerModel`:

```go
// NewFilePickerModelWith creates a file picker over files, in the order
// given.
func NewFilePickerModelWith(files []string) FilePickerModel {
	return FilePickerModel{files: files}
}

// RunFilePickerWith runs the file picker over files and returns the
// chosen one. The result is Aborted when files is empty or the person
// pressed Esc, q or Ctrl+C.
func RunFilePickerWith(files []string) (FilePickerResult, error) {
	model := NewFilePickerModelWith(files)
	if !model.HasFiles() {
		return FilePickerResult{Aborted: true}, nil
	}

	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return FilePickerResult{}, err
	}
	m, ok := finalModel.(FilePickerModel)
	if !ok {
		return FilePickerResult{Aborted: true}, nil
	}
	return m.GetResult(), nil
}
```

Leave `RunFilePicker` and `RenderNoFilesError` in place for now. Task 8 deletes them when their last callers go.

- [ ] **Step 4: Write `internal/cli/resolve.go`**

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// notDeckNames are markdown files that are never a deck, compared in
// lower case.
var notDeckNames = map[string]bool{
	"readme.md":       true,
	"changelog.md":    true,
	"contributing.md": true,
	"license.md":      true,
}

// deckPicker asks the person to choose one of several decks. cancelled is
// true when they closed the picker without choosing.
type deckPicker func(candidates []string) (chosen string, cancelled bool, err error)

// deckResolver picks the deck a command acts on. interactive and pick are
// fields so a test can stand in for the terminal.
type deckResolver struct {
	interactive func() bool
	pick        deckPicker
}

var defaultDeckResolver = deckResolver{
	interactive: func() bool { return stdinIsTerminal() },
	pick: func(candidates []string) (string, bool, error) {
		result, err := tui.RunFilePickerWith(candidates)
		if err != nil {
			return "", false, err
		}
		return result.File, result.Aborted, nil
	},
}

// resolveDeck returns the deck file for a command's optional [deck]
// argument, which is a file or a folder. The order is: the file itself;
// the only deck in the folder (the current folder when arg is empty); a
// picker when standard input is a terminal; otherwise an error that lists
// the candidates.
func resolveDeck(arg string) (string, error) {
	return defaultDeckResolver.resolve(arg)
}

func (r deckResolver) resolve(arg string) (string, error) {
	folder := "."
	if arg != "" {
		info, err := os.Stat(arg)
		if os.IsNotExist(err) {
			return "", userError(codeDeckNotFound, fmt.Errorf("deck not found: %s", arg))
		}
		if err != nil {
			return "", userError(codeDeckNotFound, fmt.Errorf("cannot read %s: %w", arg, err))
		}
		if !info.IsDir() {
			return arg, nil
		}
		folder = arg
	}

	candidates, err := deckCandidates(folder)
	if err != nil {
		return "", userError(codeDeckNotFound, err)
	}
	switch len(candidates) {
	case 0:
		return "", userError(codeNoDeck, fmt.Errorf("no deck found in %s: name a deck file, or create one with tap new", describeFolder(folder)))
	case 1:
		return candidates[0], nil
	}

	if !r.interactive() {
		return "", userError(codeAmbiguousDeck, fmt.Errorf("more than one deck in %s, name one of: %s", describeFolder(folder), strings.Join(candidates, ", ")))
	}
	chosen, cancelled, err := r.pick(candidates)
	if err != nil {
		return "", internalError(codeInternal, fmt.Errorf("deck picker: %w", err))
	}
	if cancelled || chosen == "" {
		return "", errCancelled
	}
	return chosen, nil
}

// deckCandidates lists the decks in folder, newest first: every .md file
// except the ones in notDeckNames. Each path is joined to folder.
func deckCandidates(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", folder, err)
	}

	type candidate struct {
		path     string
		modified int64
	}
	var found []candidate
	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		if entry.IsDir() || !strings.HasSuffix(lower, ".md") || notDeckNames[lower] {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		found = append(found, candidate{path: filepath.Join(folder, name), modified: info.ModTime().UnixNano()})
	}

	sort.Slice(found, func(i, j int) bool {
		if found[i].modified != found[j].modified {
			return found[i].modified > found[j].modified
		}
		return found[i].path < found[j].path
	})
	paths := make([]string, len(found))
	for index, deck := range found {
		paths[index] = deck.path
	}
	return paths, nil
}

// resolveDeckFolder returns the folder a deck lives in, for commands that
// need only the folder: the argument when it is a folder, the folder of a
// deck file, or the current folder when arg is empty.
func resolveDeckFolder(arg string) (string, error) {
	if arg == "" {
		return ".", nil
	}
	info, err := os.Stat(arg)
	if os.IsNotExist(err) {
		return "", userError(codeDeckNotFound, fmt.Errorf("deck not found: %s", arg))
	}
	if err != nil {
		return "", userError(codeDeckNotFound, fmt.Errorf("cannot read %s: %w", arg, err))
	}
	if info.IsDir() {
		return arg, nil
	}
	return filepath.Dir(arg), nil
}

// describeFolder names a folder in a message.
func describeFolder(folder string) string {
	if folder == "." {
		return "the current folder"
	}
	return folder
}

// firstArg returns the first positional argument, or "" when there is
// none.
func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cli ./internal/tui -run 'TestResolveDeck|TestNewFilePickerModelWith|TestFilePicker' -short -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/resolve.go internal/cli/resolve_test.go internal/tui/filepicker.go internal/tui/filepicker_test.go
git commit -m "feat(cli): add one resolver for the optional [deck] argument"
```

---

### Task 4: `tap export pdf`

**Files:**
- Create: `internal/cli/export.go`
- Rename: `internal/cli/pdf.go` to `internal/cli/export_pdf.go`
- Rename: `internal/cli/pdf_test.go` to `internal/cli/export_pdf_test.go`
- Modify: `internal/cli/deck.go` (`prepareDeck`)

**Interfaces:**
- Consumes: `resolveDeck`, `firstArg` (Task 3); `userError`, `internalError`, `reportedError`, the codes (Task 1); `printJSONOK` (Task 2)
- Produces: `var exportCmd *cobra.Command` (the parent, Task 5 adds to it); `func runExportPDF(cmd *cobra.Command, args []string) error`

- [ ] **Step 1: Move the files**

```bash
git mv internal/cli/pdf.go internal/cli/export_pdf.go
git mv internal/cli/pdf_test.go internal/cli/export_pdf_test.go
```

- [ ] **Step 2: Write the failing test**

Add to `internal/cli/export_pdf_test.go`:

```go
func TestExportPDFCommandShape(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"export", "pdf"})
	if err != nil || command.Name() != "pdf" || command.Parent().Name() != "export" {
		t.Fatalf("tap export pdf not found: %v", err)
	}
	if command.Use != "pdf [deck]" {
		t.Errorf("Use = %q, want %q", command.Use, "pdf [deck]")
	}
	for name, shorthand := range map[string]string{"output": "o", "content": "", "json": ""} {
		flag := command.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("missing --%s", name)
			continue
		}
		if flag.Shorthand != shorthand {
			t.Errorf("--%s shorthand = %q, want %q", name, flag.Shorthand, shorthand)
		}
	}
}

func TestExportPDFMissingDeckIsAUserError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.md")
	exitCode, stdout, _ := runTap(t, "export", "pdf", missing, "--json")
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
	if !strings.Contains(stdout, `"code": "deck_not_found"`) {
		t.Errorf("stdout = %q, want a deck_not_found JSON error", stdout)
	}
}
```

Add `"strings"` to the test file imports if it is missing.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestExportPDF' -short`
Expected: FAIL, `tap export pdf not found`.

- [ ] **Step 4: Write `internal/cli/export.go`**

```go
package cli

import "github.com/spf13/cobra"

// exportCmd groups the commands that render a deck to files.
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export a deck to a PDF or to images",
}

func init() {
	rootCmd.AddCommand(exportCmd)
}
```

- [ ] **Step 5: Change `internal/cli/export_pdf.go`**

1. Rename `pdfCmd` to `exportPDFCmd`. Set `Use: "pdf [deck]"`, `Args: cobra.MaximumNArgs(1)`, `RunE: runExportPDF`, and delete the `Run:` line.
2. Rewrite the examples in `Long` with the new name:

```
Examples:
  tap export pdf                              # The deck in this folder, to <deck>.pdf
  tap export pdf slides.md                    # Export to slides.pdf
  tap export pdf slides.md --output handout.pdf
  tap export pdf slides.md -o talk.pdf        # Short form
  tap export pdf slides.md --content notes    # Only speaker notes
  tap export pdf slides.md --content both     # Slides with notes
  tap export pdf slides.md --json             # Print the result as JSON
```

3. In `init`, register on the parent and add `--json`:

```go
func init() {
	exportCmd.AddCommand(exportPDFCmd)

	exportPDFCmd.Flags().StringVarP(&pdfOutput, "output", "o", "", "output PDF file path (default: <deck>.pdf)")
	exportPDFCmd.Flags().StringVar(&pdfContent, "content", "slides", "content to include: slides, notes, or both")
	exportPDFCmd.Flags().BoolVar(&pdfJSON, "json", false, "print the result as JSON")
}
```

Add `pdfJSON bool` to the flag `var` block.

4. Delete `runPDF` (the wrapper that calls `os.Exit`). Rename `runPDFE(args []string) error` to `runExportPDF(cmd *cobra.Command, args []string) error`.
5. Replace the start of the body, from `file := args[0]` through the `os.Stat` check, with:

```go
	file, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
```

6. Give each error its class. Leave the messages as they are:
   - `pdf.ValidateContentType` failure: `return userError(codeUsage, err)`
   - `config.Load` and `cfg.Validate` failures: `userError(codeInvalidDeck, fmt.Errorf(...))`
   - `prepareDeck` failure: `fmt.Errorf("failed to load presentation: %w", err)`, unchanged. `prepareDeck` sets the class, see step 6.
   - component build errors: replace `return errSilent` with `return reportedError(codeComponentBuild, componentErrorsError(componentBuildErrs))`
   - `pdf.New()` failure: `internalError(codeBrowser, fmt.Errorf("failed to create PDF exporter: %w", err))`
   - `exporter.Export` failure (not interrupted): `internalError(codeExportFailed, fmt.Errorf("PDF export failed: %w", err))`
7. Replace the success output at the end with:

```go
	for _, broken := range result.BrokenSlides {
		fmt.Fprintf(os.Stderr, "warning: slide %d shows an error card: %s\n", broken.SlideNumber, broken.Message)
	}

	if pdfJSON {
		brokenSlides := make([]brokenSlideJSON, 0, len(result.BrokenSlides))
		for _, broken := range result.BrokenSlides {
			brokenSlides = append(brokenSlides, brokenSlideJSON{Slide: broken.SlideNumber, Message: broken.Message})
		}
		return printJSONOK(cmd.OutOrStdout(), exportPDFResult{
			Output:       result.OutputPath,
			Pages:        result.PageCount,
			Bytes:        result.FileSize,
			BrokenSlides: brokenSlides,
		})
	}

	Successln("\nPDF export complete!")
	fmt.Println()
	fmt.Printf("  Output:    %s\n", result.OutputPath)
	fmt.Printf("  Pages:     %d\n", result.PageCount)
	fmt.Printf("  File size: %s\n", formatSize(result.FileSize))
	fmt.Printf("  Time:      %s\n", formatDuration(result.Duration))
	fmt.Println()
	return nil
}

// exportPDFResult is the --json result of tap export pdf.
type exportPDFResult struct {
	Output       string            `json:"output"`
	Pages        int               `json:"pages"`
	Bytes        int64             `json:"bytes"`
	BrokenSlides []brokenSlideJSON `json:"brokenSlides"`
}

// brokenSlideJSON is one slide that showed an error card, with its
// 1-based number.
type brokenSlideJSON struct {
	Slide   int    `json:"slide"`
	Message string `json:"message"`
}
```

`componentErrorsError` already exists (`dev.go` uses it). It joins the component build errors into one error.

- [ ] **Step 6: Class the errors in `prepareDeck` (`internal/cli/deck.go`)**

- `loadPresentation` failure: `return nil, nil, nil, nil, nil, userError(codeInvalidDeck, err)`
- `srv.Start()` failure: `internalError(codeInternal, fmt.Errorf("failed to start temporary server: %w", err))`

Update its doc comment: it names "tap pdf and tap screenshot". Write "tap export pdf and tap export images".

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestExportPDF|TestPrepareDeck' -short -v`
Expected: PASS.

Run: `go build ./... && go vet ./internal/cli`
Expected: no output. (`screenshot.go` still uses `errSilent`, so it stays defined.)

- [ ] **Step 8: Commit**

```bash
git add -A internal/cli
git commit -m "feat(cli)!: move tap pdf to tap export pdf, with [deck] and --json"
```

---

### Task 5: `tap export images`

**Files:**
- Rename: `internal/cli/screenshot.go` to `internal/cli/export_images.go`
- Rename: `internal/cli/screenshot_test.go` to `internal/cli/export_images_test.go`
- Modify: `internal/cli/exit.go` (remove the `errSilent` case)

**Interfaces:**
- Consumes: `exportCmd` (Task 4); `resolveDeck`, `firstArg` (Task 3); error helpers (Task 1); `printJSONOK` (Task 2)
- Produces:
  - `func captureState(hasStep bool, step int, hasFragment bool, fragment int, slide transformer.TransformedSlide) (urlStep, urlFragment int)`
  - `func validateStepAndFragment(hasStep bool, step int, hasFragment bool, fragment int, slide transformer.TransformedSlide, slideNumber int) error` with the new 1-based ranges
  - `errInterrupted` moves to `exit.go`; `errSilent` is deleted

**(plan decision 4)** `--step N` is the state after N steps; the frontend's `?step=N` already means that. `--fragment N` is the state with N fragments shown, which is the frontend's `?fragment=N-1`. Left out, each one takes its final value: `steps` and `fragmentCount - 1` in URL terms.

- [ ] **Step 1: Move the files**

```bash
git mv internal/cli/screenshot.go internal/cli/export_images.go
git mv internal/cli/screenshot_test.go internal/cli/export_images_test.go
```

- [ ] **Step 2: Write the failing tests**

In `internal/cli/export_images_test.go`, replace `TestValidateStepAndFragment` with:

```go
func TestValidateStepAndFragment(t *testing.T) {
	slide := transformer.TransformedSlide{Steps: 3, FragmentCount: 4}

	tests := []struct {
		name        string
		hasStep     bool
		step        int
		hasFragment bool
		fragment    int
		wantErr     bool
	}{
		{"step 1 is the first step", true, 1, false, 0, false},
		{"step 0 is before the first step", true, 0, false, 0, false},
		{"step at the last step", true, 3, false, 0, false},
		{"step below zero", true, -1, false, 0, true},
		{"step past the last step", true, 4, false, 0, true},
		{"no step given, never checked", false, 99, false, 0, false},
		{"fragment 1 is the first fragment", false, 0, true, 1, false},
		{"fragment 0 shows none", false, 0, true, 0, false},
		{"fragment at the last fragment", false, 0, true, 4, false},
		{"fragment below zero", false, 0, true, -1, true},
		{"fragment past the last fragment", false, 0, true, 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStepAndFragment(tt.hasStep, tt.step, tt.hasFragment, tt.fragment, slide, 1)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateStepAndFragment() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if _, code, _ := classify(err); code != codeOutOfRange {
					t.Errorf("code = %q, want %q", code, codeOutOfRange)
				}
			}
		})
	}
}

func TestCaptureState(t *testing.T) {
	slide := transformer.TransformedSlide{Steps: 3, FragmentCount: 4}
	tests := []struct {
		name         string
		hasStep      bool
		step         int
		hasFragment  bool
		fragment     int
		wantStep     int
		wantFragment int
	}{
		{"both left out is the final state", false, 0, false, 0, 3, 3},
		{"step given, fragments final", true, 1, false, 0, 1, 3},
		{"fragment given, steps final", false, 0, true, 1, 3, 0},
		{"fragment 0 shows none", false, 0, true, 0, 3, -1},
		{"both given", true, 2, true, 2, 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, fragment := captureState(tt.hasStep, tt.step, tt.hasFragment, tt.fragment, slide)
			if step != tt.wantStep || fragment != tt.wantFragment {
				t.Errorf("captureState() = (%d, %d), want (%d, %d)", step, fragment, tt.wantStep, tt.wantFragment)
			}
		})
	}
}

func TestExportImagesCommandShape(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"export", "images"})
	if err != nil || command.Name() != "images" {
		t.Fatalf("tap export images not found: %v", err)
	}
	if command.Use != "images [deck]" {
		t.Errorf("Use = %q, want %q", command.Use, "images [deck]")
	}
	for name, shorthand := range map[string]string{"output": "o", "theme": "t", "json": "", "slide": "", "step": "", "fragment": ""} {
		flag := command.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("missing --%s", name)
			continue
		}
		if flag.Shorthand != shorthand {
			t.Errorf("--%s shorthand = %q, want %q", name, flag.Shorthand, shorthand)
		}
	}
	if command.Flags().Lookup("out") != nil {
		t.Error("--out should be removed")
	}
}
```

In `TestScreenshotCommand_StdoutStderrSeparation`, rename it to `TestExportImagesCommand_StdoutStderrSeparation`, and change the two `exec.Command` calls:

```go
		cmd := exec.Command(binary, "export", "images", "does-not-exist.md", "--slide", "1")
```

```go
		cmd := exec.Command(binary, "export", "images", sampleDeck, "--slide", "1", "--output", outputPath)
```

Rename `TestScreenshotIntegration` to `TestExportImagesIntegration`. It calls functions, not the command, so only the name changes.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestValidateStepAndFragment|TestCaptureState|TestExportImagesCommandShape' -short`
Expected: compile failure, `undefined: captureState`.

- [ ] **Step 4: Change `internal/cli/export_images.go`**

1. Move `errInterrupted` and its comment to `exit.go`. Delete `errSilent`, and delete the `errors.Is(err, errSilent)` case in `classify`.
2. Rename `screenshotCmd` to `exportImagesCmd`, and `screenshotOut` to `screenshotOutput`. Add `screenshotJSON bool` to the flag `var` block. Set `Use: "images [deck]"`, `Short: "Render slides to PNG images"`, `Args: cobra.MaximumNArgs(1)`, `RunE: runExportImages`.
3. Rewrite the `Long` text. Keep its paragraphs, and change them so that:
   - "tap pdf" reads "tap export pdf";
   - the step paragraph reads: "With neither --step nor --fragment, the slide renders its final state through print mode, the same way tap export pdf does. --step N renders the slide after N steps and --fragment N with N fragments shown; 0 is the state before the first one. A flag you leave out takes its final value.";
   - the examples are:

```
Examples:
  tap export images --slide 12                       # The deck in this folder, slide 12
  tap export images deck.md --slide 12               # Final state of slide 12
  tap export images deck.md --slide 12 --step 3      # Slide 12 after its third step
  tap export images deck.md --slide 12 --step 3 --wait 400
  tap export images deck.md --slide 12 -o slide.png  # Custom output file
  tap export images deck.md --all                    # Every slide's final state
  tap export images deck.md --slide 3 -t bauhaus     # Render with a specific theme
  tap export images deck.md --all --json             # Print the written files as JSON
```

4. Replace `init`:

```go
func init() {
	exportCmd.AddCommand(exportImagesCmd)

	exportImagesCmd.Flags().IntVar(&screenshotSlide, "slide", 0, "slide number to capture, from 1 (required unless --all)")
	exportImagesCmd.Flags().IntVar(&screenshotStep, "step", 0, "render the slide after this many steps, from 0 (default: the final step)")
	exportImagesCmd.Flags().IntVar(&screenshotFragment, "fragment", 0, "render the slide with this many fragments shown, from 0 (default: all)")
	exportImagesCmd.Flags().StringVarP(&screenshotTheme, "theme", "t", "", "theme slug to render with (default: the deck's own theme)")
	exportImagesCmd.Flags().StringVarP(&screenshotOutput, "output", "o", "", "output PNG file (one slide) or folder (--all); default derived from the deck's file name")
	exportImagesCmd.Flags().IntVar(&screenshotWidth, "width", 1920, "viewport width in pixels; height follows the deck's aspect ratio")
	exportImagesCmd.Flags().BoolVar(&screenshotAll, "all", false, "capture every slide's final state into a folder instead of one slide")
	exportImagesCmd.Flags().IntVar(&screenshotWait, "wait", 0, "milliseconds to wait after the page is ready before capturing, instead of settling (0-60000)")
	exportImagesCmd.Flags().BoolVar(&screenshotJSON, "json", false, "print the written files as JSON")
}
```

5. Delete `runScreenshot`. Rename `runScreenshotE` to `runExportImages(cmd *cobra.Command, args []string) error`. Replace `file := args[0]` and the later `os.Stat` block with:

```go
	file, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
```

Keep the flag checks before this call, so a bad flag fails before the picker opens.

6. Class the errors. Leave the messages as they are:
   - `validateScreenshotFlags`, `validateWaitFlag`: wrap the returned error with `userError(codeUsage, ...)` inside each function.
   - unknown theme: `userError(codeUnknownTheme, unknownThemeError(screenshotTheme))`
   - `config.Load`, `cfg.Validate`, `resolveDimensions`, "deck has no slides": `userError(codeInvalidDeck, ...)`
   - `slideOutOfRangeError`: return `userError(codeOutOfRange, fmt.Errorf(...))` from inside the function.
   - component build errors: `printComponentErrorsToStderr(componentBuildErrs)` stays, then `return reportedError(codeComponentBuild, componentErrorsError(componentBuildErrs))`.
   - `pdf.New()` and `exporter.EnsureBrowser()`: `internalError(codeBrowser, ...)`
7. Replace `validateStepAndFragment` and add `captureState`:

```go
// validateStepAndFragment checks --step and --fragment against the slide.
// Both count reveals: 0 is the state before the first one, and the last
// valid value is the slide's own count.
func validateStepAndFragment(hasStep bool, step int, hasFragment bool, fragment int, slide transformer.TransformedSlide, slideNumber int) error {
	if hasStep && (step < 0 || step > slide.Steps) {
		return userError(codeOutOfRange, fmt.Errorf("step %d is out of range for slide %d: valid steps are 0-%d", step, slideNumber, slide.Steps))
	}
	if hasFragment && (fragment < 0 || fragment > slide.FragmentCount) {
		return userError(codeOutOfRange, fmt.Errorf("fragment %d is out of range for slide %d: valid fragments are 0-%d", fragment, slideNumber, slide.FragmentCount))
	}
	return nil
}

// captureState turns --step and --fragment into the frontend's ?step= and
// ?fragment= values. ?step=N is the state after N steps, and ?fragment=N
// shows fragments 0 to N, so a count of N fragments is ?fragment=N-1. A
// flag left out takes its final value.
func captureState(hasStep bool, step int, hasFragment bool, fragment int, slide transformer.TransformedSlide) (urlStep, urlFragment int) {
	urlStep = slide.Steps
	if hasStep {
		urlStep = step
	}
	urlFragment = slide.FragmentCount - 1
	if hasFragment {
		urlFragment = fragment - 1
	}
	return urlStep, urlFragment
}
```

8. Replace the single-slide capture options block with:

```go
	options := pdf.CaptureOptions{
		SlideNumber: screenshotSlide,
		Width:       width,
		Height:      height,
		Theme:       screenshotTheme,
		WaitMS:      screenshotWait,
	}
	// A step, a fragment or --wait needs the live viewer at an exact
	// state. Otherwise print mode renders the final state.
	if hasStepFlag || hasFragmentFlag || screenshotWait > 0 {
		urlStep, urlFragment := captureState(hasStepFlag, screenshotStep, hasFragmentFlag, screenshotFragment, *slide)
		options.Step = &urlStep
		options.Fragment = &urlFragment
	} else {
		options.Print = true
	}
```

`hasWaitFlag` is no longer read. Delete it.

9. Replace both places that print written paths with a helper, so `--json` and plain output have one code path:

```go
// printWrittenImages prints the written PNG paths: one per line, or as
// the --json result.
func printWrittenImages(cmd *cobra.Command, paths []string) error {
	if screenshotJSON {
		return printJSONOK(cmd.OutOrStdout(), struct {
			Files []string `json:"files"`
		}{Files: paths})
	}
	for _, path := range paths {
		fmt.Fprintln(cmd.OutOrStdout(), path)
	}
	return nil
}
```

For `--all` with broken slides, keep the `slide N: reason` lines on stderr. Then return `reportedError(codeBrokenSlides, fmt.Errorf("%d slide(s) failed to capture", len(broken)))`. Print the written paths first, as today, except in `--json` mode, where the error result replaces them.

10. Update `resolveAllOutputDir(out, file string)`: its comment says "the --out value". Write "the --output value".

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestValidate|TestCaptureState|TestCaptureAllSlides|TestExportImages|TestResolve|TestSlideOutOfRange|TestUnknownTheme|TestDeckBaseName|TestClassify' -short -v`
Expected: PASS.

Run the browser tests once too, since the capture path changed:

Run: `go test ./internal/cli -run 'TestExportImages|TestScreenshotIntegration_RollingDeployStepsDiffer' -v`
Expected: PASS, or SKIP when no browser is installed.

- [ ] **Step 6: Commit**

```bash
git add -A internal/cli
git commit -m "feat(cli)!: move tap screenshot to tap export images, with 1-based --step and --fragment"
```

---

### Task 6: `tap slide add` and `tap component new`

**Files:**
- Rename: `internal/cli/add.go` to `internal/cli/slide.go`
- Rename: `internal/cli/add_component.go` to `internal/cli/component.go`
- Rename: `internal/cli/add_component_test.go` to `internal/cli/component_test.go`
- Create: `internal/cli/slide_test.go`
- Modify: `internal/cli/templates/tap-env.d.ts` (comment only)

**Interfaces:**
- Consumes: `resolveDeck`, `resolveDeckFolder`, `firstArg` (Task 3); error helpers (Task 1); `printJSONOK` (Task 2)
- Produces:
  - `var slideCmd`, `var slideAddCmd`, `var componentCmd`, `var componentNewCmd`
  - `type componentScaffold struct { Files []string; Snippet string }`
  - `func scaffoldComponent(name, deckArg string) (componentScaffold, error)`

**(plan decision 7)** `component new` uses `resolveDeckFolder`, not the picker.

- [ ] **Step 1: Move the files**

```bash
git mv internal/cli/add.go internal/cli/slide.go
git mv internal/cli/add_component.go internal/cli/component.go
git mv internal/cli/add_component_test.go internal/cli/component_test.go
```

- [ ] **Step 2: Write the failing tests**

`internal/cli/slide_test.go`:

```go
package cli

import (
	"path/filepath"
	"testing"
)

func TestSlideAddCommandShape(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"slide", "add"})
	if err != nil || command.Name() != "add" || command.Parent().Name() != "slide" {
		t.Fatalf("tap slide add not found: %v", err)
	}
	if command.Use != "add [deck]" {
		t.Errorf("Use = %q, want %q", command.Use, "add [deck]")
	}
}

func TestSlideAddWithoutATerminalIsAUserError(t *testing.T) {
	original := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = original })

	dir := t.TempDir()
	writeDecks(t, dir, "talk.md")
	exitCode, _, stderr := runTap(t, "slide", "add", filepath.Join(dir, "talk.md"))
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d (stderr %q)", exitCode, exitUserError, stderr)
	}
}
```

In `internal/cli/component_test.go`:

1. Rename `resetAddComponentFlags` to `resetComponentFlags`. Delete the `addComponentDeck = ""` line from it, and rename the two other variables (see step 4).
2. Every test that sets `addComponentDeck = X` and then calls `runAddComponentE(Name)` calls `scaffoldComponent(Name, X)` instead. A test that does not set `addComponentDeck` passes `""`. The result's first value is ignored where the test only checks files on disk: `if _, err := scaffoldComponent("RollingDeploy", deckDir); err != nil {`.
3. Rename `TestAddComponentDeckFlagAsDirectory` to `TestComponentNewDeckFolderArgument`, and fix its comment: "a folder argument writes into that folder, not its parent".
4. Add:

```go
func TestComponentNewCommandShape(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"component", "new"})
	if err != nil || command.Name() != "new" || command.Parent().Name() != "component" {
		t.Fatalf("tap component new not found: %v", err)
	}
	if command.Use != "new <Name> [deck]" {
		t.Errorf("Use = %q, want %q", command.Use, "new <Name> [deck]")
	}
	if command.Flags().Lookup("deck") != nil {
		t.Error("--deck should be removed: the deck is a positional argument")
	}
}

func TestScaffoldComponentReturnsFilesAndSnippet(t *testing.T) {
	resetComponentFlags()
	deckDir := t.TempDir()
	result, err := scaffoldComponent("RollingDeploy", deckDir)
	if err != nil {
		t.Fatalf("scaffoldComponent() error = %v", err)
	}
	wantFile := filepath.Join(deckDir, "slides", "RollingDeploy.jsx")
	if len(result.Files) != 1 || result.Files[0] != wantFile {
		t.Errorf("Files = %v, want [%s]", result.Files, wantFile)
	}
	if result.Snippet != componentSnippet("RollingDeploy", "jsx", false) {
		t.Errorf("Snippet = %q", result.Snippet)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestSlideAdd|TestComponentNew|TestScaffoldComponent' -short`
Expected: compile failure, `undefined: scaffoldComponent`.

- [ ] **Step 4: Rewrite `internal/cli/slide.go`**

Replace the whole file:

```go
package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// slideCmd groups the commands that work on a deck's slides.
var slideCmd = &cobra.Command{
	Use:   "slide",
	Short: "Work with the slides in a deck",
}

// slideAddCmd appends a slide through the interactive wizard.
var slideAddCmd = &cobra.Command{
	Use:   "add [deck]",
	Short: "Add a slide to a deck interactively",
	Long: `Add a slide to the end of a deck with an interactive wizard.

The wizard asks for a layout and the content of each section of the slide.
It needs a terminal.

Examples:
  tap slide add              # The deck in this folder
  tap slide add talk.md      # A specific deck`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !stdinIsTerminal() {
			return userError(codeNeedsTerminal, errors.New("tap slide add runs a wizard and needs a terminal"))
		}
		file, err := resolveDeck(firstArg(args))
		if err != nil {
			return err
		}
		result, err := tui.RunAddWizard(file)
		if err != nil {
			return internalError(codeInternal, err)
		}
		if result.Aborted {
			return errCancelled
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(slideCmd)
	slideCmd.AddCommand(slideAddCmd)
}
```

`findPresentationFile` is gone. The resolver replaces it.

- [ ] **Step 5: Change `internal/cli/component.go`**

1. Rename the flag variables `addComponentInline` to `componentInline` and `addComponentTS` to `componentTS`. Delete `addComponentDeck` and its flag. Add `componentJSON bool`.
2. Replace the command variables and `init`:

```go
// componentCmd groups the commands for deck-supplied React components.
var componentCmd = &cobra.Command{
	Use:   "component",
	Short: "Work with a deck's React components",
}

// componentNewCmd scaffolds a component from a template.
var componentNewCmd = &cobra.Command{
	Use:   "new <Name> [deck]",
	Short: "Scaffold a deck-supplied React component",
	Long: `Scaffold a deck-supplied React component from a template.

Writes a whole-slide component to slides/<Name>.jsx by default, or, with
--inline, an inline block component to components/<Name>.jsx. With --ts,
the file is a .tsx, and tap-env.d.ts and tap-shims.d.ts are written next
to the deck (each only when it does not already exist), so editors and
LLM type checks work without installing anything.

<Name> must be a PascalCase identifier, for example RollingDeploy. [deck]
is a deck file or a deck folder; the default is the current folder.

Examples:
  tap component new RollingDeploy                # slides/RollingDeploy.jsx
  tap component new LatencyDrop --inline         # components/LatencyDrop.jsx
  tap component new RollingDeploy --ts           # slides/RollingDeploy.tsx
  tap component new RollingDeploy talks/deck.md  # next to talks/deck.md`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runComponentNew,
}

func init() {
	rootCmd.AddCommand(componentCmd)
	componentCmd.AddCommand(componentNewCmd)

	componentNewCmd.Flags().BoolVar(&componentInline, "inline", false, "scaffold an inline block component instead of a whole-slide one")
	componentNewCmd.Flags().BoolVar(&componentTS, "ts", false, "write a .tsx file and a tap-env.d.ts next to the deck")
	componentNewCmd.Flags().BoolVar(&componentJSON, "json", false, "print the written files and the snippet as JSON")
}

// componentScaffold is what tap component new wrote, and the markdown
// snippet that uses the component.
type componentScaffold struct {
	Files   []string `json:"files"`
	Snippet string   `json:"snippet"`
}

func runComponentNew(cmd *cobra.Command, args []string) error {
	var deckArg string
	if len(args) > 1 {
		deckArg = args[1]
	}
	result, err := scaffoldComponent(args[0], deckArg)
	if err != nil {
		return err
	}
	if componentJSON {
		return printJSONOK(cmd.OutOrStdout(), result)
	}
	for _, file := range result.Files {
		fmt.Println(file)
	}
	fmt.Println()
	fmt.Print(result.Snippet)
	return nil
}
```

3. Rename `runAddComponentE(name string) error` to `scaffoldComponent(name, deckArg string) (componentScaffold, error)`:
   - the invalid name error becomes `userError(codeUsage, fmt.Errorf(...))`, same message;
   - replace the `targetDir := "."` block and the `if addComponentDeck != ""` block with:

```go
	targetDir, err := resolveDeckFolder(deckArg)
	if err != nil {
		return componentScaffold{}, err
	}
```

   - the "component file already exists" error becomes `userError(codeExists, fmt.Errorf(...))`;
   - delete the printing at the end, and return `componentScaffold{Files: written, Snippet: componentSnippet(name, extension, componentInline)}, nil`;
   - every other `return err` becomes `return componentScaffold{}, err`.
4. Update the `componentNamePattern` comment: "tap add component" becomes "tap component new".

- [ ] **Step 6: Fix the template comment**

In `internal/cli/templates/tap-env.d.ts`, line 2, change `` `tap add component` `` to `` `tap component new` ``.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestSlideAdd|TestComponent|TestScaffoldComponent|TestAddComponent' -short -v`
Expected: PASS. (No test is still named `TestAddComponent...` unless it was left out of the renames in step 2. If one is, rename it.)

- [ ] **Step 8: Commit**

```bash
git add -A internal/cli
git commit -m "feat(cli)!: move tap add to tap slide add and tap add component to tap component new"
```

---

### Task 7: `tap theme list` and `tap theme show`

**Files:**
- Modify: `internal/cli/theme.go`
- Modify: `internal/cli/theme_test.go`
- Modify: `internal/cli/jsonout_test.go` (remove the skip)

**Interfaces:**
- Consumes: `resolveDeck`, `firstArg` (Task 3); error helpers (Task 1); `printJSONOK` (Task 2)
- Produces: `func resolveThemeShowSlug(arg string) (string, error)` (it takes the positional, not `args`)

**(plan decision 5)** `theme show [slug|deck]`.

- [ ] **Step 1: Write the failing tests**

In `internal/cli/theme_test.go`, replace `TestResolveThemeShowSlug_DeckPicksDeckTheme` with:

```go
func TestResolveThemeShowSlug(t *testing.T) {
	dir := t.TempDir()
	deckPath := filepath.Join(dir, "deck.md")
	if err := os.WriteFile(deckPath, []byte("---\ntheme: swiss\ntitle: Demo\n---\n\n# Slide\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		arg      string
		wantSlug string
		wantCode string
	}{
		{"a built-in slug", "terminal", "terminal", ""},
		{"a deck file uses its theme", deckPath, "swiss", ""},
		{"a deck folder uses its deck's theme", dir, "swiss", ""},
		{"neither a slug nor a path", "no-such-theme", "", codeUnknownTheme},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug, err := resolveThemeShowSlug(tt.arg)
			if tt.wantCode != "" {
				if _, code, _ := classify(err); code != tt.wantCode {
					t.Errorf("code = %q, want %q (err %v)", code, tt.wantCode, err)
				}
				return
			}
			if err != nil || slug != tt.wantSlug {
				t.Errorf("resolveThemeShowSlug(%q) = (%q, %v), want %q", tt.arg, slug, err, tt.wantSlug)
			}
		})
	}
}

func TestThemeShowWithNoArgumentUsesTheDeckInTheFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deck.md"), []byte("---\ntheme: swiss\n---\n\n# Slide\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withWorkingDirectory(t, dir, func() {
		slug, err := resolveThemeShowSlug("")
		if err != nil || slug != "swiss" {
			t.Errorf("resolveThemeShowSlug(\"\") = (%q, %v), want swiss", slug, err)
		}
	})
}

func TestThemeListJSONIsAnObject(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "theme", "list", "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d", exitCode)
	}
	var output struct {
		OK     bool             `json:"ok"`
		Themes []map[string]any `json:"themes"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not a JSON object: %v\n%s", err, stdout)
	}
	if !output.OK || len(output.Themes) == 0 {
		t.Errorf("output = %+v, want ok and a theme list", output)
	}
}
```

In `TestThemeShowCommand_JSONHoldsExpectedKeys`, add `"ok"` to the list of top-level keys it checks.

In `internal/cli/jsonout_test.go`, delete the `t.Skip` line in `TestExecutePrintsAJSONErrorForAJSONCommand`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestResolveThemeShowSlug|TestThemeShowWithNoArgument|TestThemeListJSON|TestExecutePrintsAJSONError' -short`
Expected: FAIL. `resolveThemeShowSlug` takes `[]string`, and `theme list --json` prints an array.

- [ ] **Step 3: Change `internal/cli/theme.go`**

1. Delete `themeShowDeck` and its flag. Set `Use: "show [slug|deck]"` on `themeShowCmd`. Change both commands from `Run:` to `RunE:`.
2. Rewrite the `Long` text of `theme show`, second paragraph and examples:

```
The argument is a theme slug, or a deck file or folder whose theme to
show. With no argument, tap uses the deck in the current folder.

Examples:
  tap theme show terminal
  tap theme show terminal --json
  tap theme show terminal --prompt
  tap theme show slides.md --prompt
```

3. `runThemeList(cmd *cobra.Command, args []string) error`:

```go
func runThemeList(cmd *cobra.Command, args []string) error {
	all := themes.All()
	if themeListJSON {
		return printJSONOK(cmd.OutOrStdout(), struct {
			Themes []themes.Theme `json:"themes"`
		}{Themes: all})
	}

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "SLUG\tNAME\tPOLARITY\tPITCH")
	for _, theme := range all {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", theme.Slug, theme.Name, theme.Polarity, theme.Pitch)
	}
	return writer.Flush()
}
```

Change the `--json` usage of `theme list` to `"print the list as JSON"`.

4. `runThemeShow(cmd *cobra.Command, args []string) error`:

```go
func runThemeShow(cmd *cobra.Command, args []string) error {
	if themeShowJSON && themeShowPrompt {
		return userError(codeUsage, errors.New("--json and --prompt cannot be used together"))
	}

	slug, err := resolveThemeShowSlug(firstArg(args))
	if err != nil {
		return err
	}
	theme, ok := findTheme(slug)
	if !ok {
		return userError(codeUnknownTheme, unknownThemeError(slug))
	}
	tokens, ok := themes.Tokens(slug)
	if !ok {
		return internalError(codeInternal, fmt.Errorf("theme %q has no tokens", slug))
	}
	illustration, _ := themes.Illustration(slug)

	switch {
	case themeShowPrompt:
		fmt.Fprintln(cmd.OutOrStdout(), buildThemePrompt(theme, tokens, illustration))
	case themeShowJSON:
		return printJSONOK(cmd.OutOrStdout(), themeShowJSONOutput{
			Slug:         theme.Slug,
			Name:         theme.Name,
			Polarity:     theme.Polarity,
			Pitch:        theme.Pitch,
			Tokens:       buildThemeTokensJSON(tokens),
			Illustration: illustration,
			Canvas:       themeCanvasJSON{Ratio: "16:9", Width: 1920, Height: 1080},
		})
	default:
		printThemeHuman(theme, tokens, illustration)
	}
	return nil
}
```

Delete `printThemeJSON`; the `themeShowJSON` case replaces it. Add `"errors"` to the imports.

5. Replace `resolveThemeShowSlug`:

```go
// resolveThemeShowSlug returns the theme tap theme show describes. arg is
// a built-in slug, or a deck file or folder whose theme to use. An empty
// arg means the deck in the current folder. A deck that names no theme
// uses "base", with a note on standard error.
func resolveThemeShowSlug(arg string) (string, error) {
	if arg != "" && themes.IsValid(arg) {
		return arg, nil
	}
	if arg != "" {
		if _, err := os.Stat(arg); os.IsNotExist(err) {
			return "", userError(codeUnknownTheme, unknownThemeError(arg))
		}
	}

	deck, err := resolveDeck(arg)
	if err != nil {
		return "", err
	}
	cfg, err := config.Load(deck)
	if err != nil {
		return "", userError(codeInvalidDeck, fmt.Errorf("failed to load %s: %w", deck, err))
	}
	if err := cfg.Validate(); err != nil {
		return "", userError(codeInvalidDeck, fmt.Errorf("invalid configuration in %s: %w", deck, err))
	}
	if cfg.Theme == "" {
		Warningln(fmt.Sprintf("Warning: %s names no theme, using \"base\"", deck))
		return "base", nil
	}
	// cfg.Validate() already turned an unknown theme into "base" and
	// printed a warning to standard error.
	return cfg.Theme, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestTheme|TestResolveThemeShowSlug|TestExecutePrintsAJSONError|TestBuildThemePrompt|TestToHex' -v`
Expected: PASS. The subprocess tests build the binary, so this run takes longer.

- [ ] **Step 5: Commit**

```bash
git add -A internal/cli
git commit -m "feat(cli)!: take the deck as a positional in tap theme show, use the one --json shape"
```

---

### Task 8: `dev`, `present`, `build`, `new` and `serve`

**Files:**
- Modify: `internal/cli/dev.go`, `present.go`, `build.go`, `new.go`, `serve.go`
- Modify: `internal/cli/new_test.go`, `internal/cli/root_test.go`
- Modify: `internal/tui/filepicker.go`, `internal/tui/filepicker_test.go`

**Interfaces:**
- Consumes: `resolveDeck`, `firstArg` (Task 3); error helpers (Task 1); `printJSONOK` (Task 2)
- Produces: `func runBuild(cmd *cobra.Command, args []string) error`; `func runNewNonInteractive() error` (unchanged signature, now `--json` aware)

- [ ] **Step 1: Write the failing tests**

Add to `internal/cli/root_test.go`:

```go
func TestDeckCommandsTakeAnOptionalDeck(t *testing.T) {
	for _, path := range [][]string{{"dev"}, {"present"}, {"build"}, {"new"}} {
		command, _, err := rootCmd.Find(path)
		if err != nil {
			t.Fatalf("%v not found: %v", path, err)
		}
		want := path[0] + " [deck]"
		if command.Use != want {
			t.Errorf("Use = %q, want %q", command.Use, want)
		}
	}
}

func TestBuildMissingDeckJSON(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "build", filepath.Join(t.TempDir(), "missing.md"), "--json")
	if exitCode != exitUserError || !strings.Contains(stdout, `"code": "deck_not_found"`) {
		t.Errorf("exit %d, stdout %q", exitCode, stdout)
	}
}
```

Add `"path/filepath"` and `"strings"` to the imports of `root_test.go`.

Add to `internal/cli/new_test.go`:

```go
func TestNewRejectsADeckArgumentAndOutputTogether(t *testing.T) {
	exitCode, _, stderr := runTap(t, "new", "a.md", "--output", "b.md", "--yes")
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d (stderr %q)", exitCode, exitUserError, stderr)
	}
}

func TestNewDeckArgumentIsTheOutputPath(t *testing.T) {
	dir := t.TempDir()
	withWorkingDirectory(t, dir, func() {
		exitCode, _, stderr := runTap(t, "new", "talk.md", "--yes")
		if exitCode != exitOK {
			t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
		}
	})
	if _, err := os.Stat(filepath.Join(dir, "talk.md")); err != nil {
		t.Errorf("talk.md was not written: %v", err)
	}
}
```

`runTap` resets the flags after each test, so `resetNewFlags` is not needed in these two tests.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestDeckCommandsTakeAnOptionalDeck|TestBuildMissingDeckJSON|TestNewRejects|TestNewDeckArgument' -short`
Expected: FAIL. `Use` is `dev [file]` and `build <file>`, and `new` takes no arguments.

- [ ] **Step 3: `tap dev` and `tap present`**

In `dev.go`, set `Use: "dev [deck]"`. Replace the `RunE` body up to `return runDevServer(...)` with:

```go
	RunE: func(cmd *cobra.Command, args []string) error {
		file, err := resolveDeck(firstArg(args))
		if err != nil {
			return err
		}
		return runDevServer(serverOptions{
			file:              file,
			port:              devPort,
			portExplicit:      cmd.Flags().Changed("port"),
			presenterPassword: devPresenterPassword,
			headless:          devHeadless,
			allowOrigins:      devAllowOrigins,
			tunnel:            devTunnel,
		})
	},
```

Change the examples in `Long` to show the optional deck: add `tap dev                                 # The deck in this folder` as the first line.

In `present.go`, set `Use: "present [deck]"`, and replace the `if len(args) == 0 { ... } else { ... }` block with:

```go
		file, err := resolveDeck(firstArg(args))
		if err != nil {
			return err
		}
```

The existing `settingsPath, err := usersettings.Path()` line still compiles, because `settingsPath` is new. Add `tap present                  # The deck in this folder` to the examples.

In `runDevServer`, class the errors a person can fix: the `file not found` check becomes `userError(codeDeckNotFound, ...)`, and the `failed to load config`, `invalid config`, `failed to load presentation` errors become `userError(codeInvalidDeck, ...)`. `TUI error` and `failed to generate presenter session token` become `internalError(codeInternal, ...)`. Leave the rest.

**(plan decision 3)** Ctrl+C in `tap dev`, `tap present` and `tap serve` stays a normal stop with exit 0. No change is needed for that.

- [ ] **Step 4: Delete the old picker entry points**

`RunFilePicker` and `RenderNoFilesError` in `internal/tui/filepicker.go` now have no callers. Delete both functions and `TestRenderNoFilesError`. Keep `NewFilePickerModel` and `findMarkdownFiles`, because the existing picker tests use them.

Run: `grep -rn "RunFilePicker()\|RenderNoFilesError" internal/`
Expected: no output.

- [ ] **Step 5: `tap build`**

Set `Use: "build [deck]"`, `Args: cobra.MaximumNArgs(1)`, `RunE: runBuild`. Add `buildJSON bool` and the flag `buildCmd.Flags().BoolVar(&buildJSON, "json", false, "print the result as JSON")`. Add `tap build                             # The deck in this folder, to dist/` to the examples.

Change `runBuild` to `func runBuild(cmd *cobra.Command, args []string) error`. Start it with:

```go
	file, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
```

and delete the `os.Stat` check. Then change each failure. Every `spinner.stop()` before it stays:

| Today | Becomes |
|---|---|
| `Errorln("Error: failed to resolve file path:", err); os.Exit(1)` | `return internalError(codeInternal, fmt.Errorf("failed to resolve file path: %w", err))` |
| `Errorln("Error: failed to load configuration:", err); os.Exit(1)` | `return userError(codeInvalidDeck, fmt.Errorf("failed to load configuration: %w", err))` |
| `Errorln("Error: invalid configuration:", err); os.Exit(1)` | `return userError(codeInvalidDeck, fmt.Errorf("invalid configuration: %w", err))` |
| `Errorln("Error: failed to read file:", err); os.Exit(1)` | `return userError(codeDeckNotFound, fmt.Errorf("failed to read file: %w", err))` |
| `Errorln("Error: failed to parse presentation:", file, err); os.Exit(1)` | `return userError(codeInvalidDeck, fmt.Errorf("failed to parse presentation: %s: %w", file, err))` |
| `printComponentErrorsToStderr(componentBuildErrs); os.Exit(1)` | `printComponentErrorsToStderr(componentBuildErrs); return reportedError(codeComponentBuild, componentErrorsError(componentBuildErrs))` |
| the layout warning loop, then `os.Exit(1)` | the same loop, then `return reportedError(codeInvalidDeck, fmt.Errorf("%d layout error(s)", len(warnings)))` |
| `Errorln("Error: build failed:", err); os.Exit(1)` | `return internalError(codeInternal, fmt.Errorf("build failed: %w", err))` |

Replace the success output at the end with:

```go
	if buildJSON {
		return printJSONOK(cmd.OutOrStdout(), struct {
			Output string `json:"output"`
			Files  int    `json:"files"`
			Bytes  int64  `json:"bytes"`
		}{Output: result.OutputDir, Files: result.FileCount, Bytes: result.TotalSize})
	}

	Successln("\nBuild complete!")
	fmt.Println()
	fmt.Printf("  Output:     %s\n", result.OutputDir)
	fmt.Printf("  Files:      %d\n", result.FileCount)
	fmt.Printf("  Total size: %s\n", formatSize(result.TotalSize))
	fmt.Printf("  Build time: %s\n", formatDuration(result.BuildTime))
	fmt.Println()
	Muted("Run 'tap serve %s' to preview the build.\n", result.OutputDir)
	return nil
```

Check that `result.TotalSize` is an `int64`. If `builder.Result` declares it as another integer type, use that type in the struct.

- [ ] **Step 6: `tap new [deck]`**

**(plan decisions 6 and 8)** In `new.go`:

1. Set `Use: "new [deck]"` and `Args: cobra.MaximumNArgs(1)`. Add `newJSON bool` and `newCmd.Flags().BoolVar(&newJSON, "json", false, "print the written deck as JSON (skips the wizard)")`.
2. Change `Run:` to `RunE:`:

```go
	RunE: func(cmd *cobra.Command, args []string) error {
		if deck := firstArg(args); deck != "" {
			if cmd.Flags().Changed("output") {
				return userError(codeUsage, errors.New("give the deck path once: as the argument or with --output"))
			}
			newOutput = deck
		}

		if newYes || newJSON || !stdinIsTerminal() {
			return runNewNonInteractive()
		}

		result, err := tui.RunNewWizard(newTheme, newOutput)
		if err != nil {
			return internalError(codeInternal, fmt.Errorf("failed to create presentation: %w", err))
		}
		if result.Aborted {
			return errCancelled
		}
		return nil
	},
```

3. Add `tap new my-talk.md --yes            # Write my-talk.md with no wizard` to the examples, and say in `Long` that `[deck]` is the path of the new deck, the same as `--output`.
4. In `runNewNonInteractive`: the unknown theme error becomes `userError(codeUnknownTheme, unknownThemeError(theme))`; the "already exists" error becomes `userError(codeExists, ...)`; the write failure becomes `internalError(codeInternal, ...)`. Replace the final `fmt.Println(output)` with:

```go
	if newJSON {
		return printJSONOK(os.Stdout, struct {
			Deck string `json:"deck"`
		}{Deck: output})
	}
	fmt.Println(output)
	return nil
```

5. In `new_test.go`, `resetNewFlags` also sets `newJSON = false`.

- [ ] **Step 7: `tap serve`**

`serve [dir]` does not take a deck. Change `Run: runServe` to `RunE: runServe` and `runServe` to return `error`, so it exits through `execute`:
- missing directory: print the hint as today, then `return userError(codeDeckNotFound, fmt.Errorf("directory does not exist: %s", dir))`, and delete the `Errorln` above it;
- "Cannot access directory" and "Not a directory": `userError(codeUsage, ...)`;
- a busy explicit port (`listenOnAvailablePort` error): `userError(codeUsage, err)`;
- the two remaining `os.Exit(1)` paths (server failures after startup): `internalError(codeInternal, err)`.

The hint text says `tap build <file>`. Change it to `tap build [deck]`.

- [ ] **Step 8: Check that no command calls `os.Exit` any more**

Run: `grep -n "os.Exit" internal/cli/*.go | grep -v _test`
Expected: no output. `cmd/tap/main.go` keeps its one `os.Exit(cli.Execute())`.

- [ ] **Step 9: Run the tests to verify they pass**

Run: `go test ./internal/cli ./internal/tui -short`
Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add -A internal/cli internal/tui
git commit -m "feat(cli)!: resolve [deck] the same way in dev, present, build and new"
```

---

### Task 9: Rename messages, the TUI PDF key, and the conventions test

**Files:**
- Create: `internal/cli/renamed.go`
- Create: `internal/cli/renamed_test.go`
- Create: `internal/cli/conventions_test.go`
- Modify: `internal/tui/dev.go`
- Modify: `internal/tui/dev_test.go`

**Interfaces:**
- Consumes: `userError`, `codeRenamed`, `execute`, `runTap` (Task 1)
- Produces: `func pdfExportArgs(file string) []string` in `internal/tui`

- [ ] **Step 1: Write the failing tests**

`internal/cli/renamed_test.go`:

```go
package cli

import "testing"

func TestOldCommandNamesPrintTheNewName(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"pdf", "deck.md", "-o", "x.pdf"}, "tap pdf was renamed: use tap export pdf\n"},
		{[]string{"screenshot", "deck.md", "--slide", "3", "--out", "x.png"}, "tap screenshot was renamed: use tap export images\n"},
		{[]string{"add"}, "tap add was renamed: use tap slide add\n"},
		{[]string{"add", "deck.md"}, "tap add was renamed: use tap slide add\n"},
		{[]string{"add", "component", "RollingDeploy", "--inline"}, "tap add component was renamed: use tap component new\n"},
		{[]string{"pdf", "--help"}, "tap pdf was renamed: use tap export pdf\n"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			exitCode, stdout, stderr := runTap(t, tt.args...)
			if exitCode != exitUserError {
				t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if stderr != tt.want {
				t.Errorf("stderr = %q, want %q", stderr, tt.want)
			}
		})
	}
}
```

`internal/cli/conventions_test.go`:

```go
package cli

import (
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagConventions is the one short form each shared flag has on every
// command. An empty string means the flag has no short form.
var flagConventions = map[string]string{
	"output": "o",
	"theme":  "t",
	"port":   "p",
	"yes":    "y",
	"json":   "",
}

// removedFlags must not come back on any command.
var removedFlags = []string{"out", "deck", "verbose"}

// expectedCommands is every visible command. A part of tap that adds a
// command adds it here.
var expectedCommands = []string{
	"tap build",
	"tap component",
	"tap component new",
	"tap dev",
	"tap export",
	"tap export images",
	"tap export pdf",
	"tap new",
	"tap present",
	"tap serve",
	"tap slide",
	"tap slide add",
	"tap theme",
	"tap theme list",
	"tap theme show",
}

// visibleCommands returns every command under command that a person can
// see in help, leaving out hidden commands and cobra's own help and
// completion commands.
func visibleCommands(command *cobra.Command) []*cobra.Command {
	var found []*cobra.Command
	for _, child := range command.Commands() {
		if child.Hidden || child.Name() == "help" || child.Name() == "completion" {
			continue
		}
		found = append(found, child)
		found = append(found, visibleCommands(child)...)
	}
	return found
}

func TestCommandTree(t *testing.T) {
	var paths []string
	for _, command := range visibleCommands(rootCmd) {
		paths = append(paths, command.CommandPath())
	}
	sort.Strings(paths)
	if strings.Join(paths, "\n") != strings.Join(expectedCommands, "\n") {
		t.Errorf("commands:\n%s\n\nwant:\n%s", strings.Join(paths, "\n"), strings.Join(expectedCommands, "\n"))
	}
}

func TestEveryCommandFollowsTheConventions(t *testing.T) {
	for _, command := range visibleCommands(rootCmd) {
		path := command.CommandPath()
		t.Run(path, func(t *testing.T) {
			args := append(strings.Fields(strings.TrimPrefix(path, "tap")), "--help")
			exitCode, stdout, stderr := runTap(t, args...)
			if exitCode != exitOK {
				t.Fatalf("%s --help exited %d: %s", path, exitCode, stderr)
			}
			if !strings.Contains(stdout, "Usage:") {
				t.Errorf("%s --help printed no usage:\n%s", path, stdout)
			}

			command.Flags().VisitAll(func(flag *pflag.Flag) {
				if want, ok := flagConventions[flag.Name]; ok && flag.Shorthand != want {
					t.Errorf("--%s has short form %q, want %q", flag.Name, flag.Shorthand, want)
				}
				for _, removed := range removedFlags {
					if flag.Name == removed {
						t.Errorf("--%s was removed and must not come back", removed)
					}
				}
			})
		})
	}
}
```

In `internal/tui/dev_test.go`, add:

```go
func TestPDFExportArgsUseExportPDF(t *testing.T) {
	got := strings.Join(pdfExportArgs("talk.md"), " ")
	if got != "export pdf talk.md" {
		t.Errorf("pdfExportArgs() = %q, want %q", got, "export pdf talk.md")
	}
}
```

Add `"strings"` to its imports if it is missing.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli ./internal/tui -run 'TestOldCommandNames|TestCommandTree|TestEveryCommandFollows|TestPDFExportArgs' -short`
Expected: FAIL. `tap pdf` is an unknown command, and `pdfExportArgs` is undefined.

- [ ] **Step 3: Write `internal/cli/renamed.go`**

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// renamedCommand is a hidden stand-in for a removed command name. It
// accepts any arguments and flags, prints one line that names the new
// command, and exits 1.
func renamedCommand(name string, replacement func(args []string) (oldName, newName string)) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Hidden:             true,
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName, newName := replacement(args)
			err := fmt.Errorf("%s was renamed: use %s", oldName, newName)
			fmt.Fprintln(cmd.ErrOrStderr(), err)
			return reportedError(codeRenamed, err)
		},
	}
}

func init() {
	rootCmd.AddCommand(renamedCommand("pdf", func([]string) (string, string) {
		return "tap pdf", "tap export pdf"
	}))
	rootCmd.AddCommand(renamedCommand("screenshot", func([]string) (string, string) {
		return "tap screenshot", "tap export images"
	}))
	rootCmd.AddCommand(renamedCommand("add", func(args []string) (string, string) {
		if len(args) > 0 && args[0] == "component" {
			return "tap add component", "tap component new"
		}
		return "tap add", "tap slide add"
	}))
}
```

With `DisableFlagParsing`, `--help` is an ordinary argument, so `tap pdf --help` also prints the rename line.

- [ ] **Step 4: Point the TUI PDF key at `tap export pdf`**

In `internal/tui/dev.go`, add:

```go
// pdfExportArgs is the tap command line that exports file to a PDF.
func pdfExportArgs(file string) []string {
	return []string{"export", "pdf", file}
}
```

In `exportPDFCmd`, change `exec.Command(binary, "pdf", file)` to `exec.Command(binary, pdfExportArgs(file)...)`. Change the function's comment to "exportPDFCmd runs `tap export pdf <file>` as a background command." and the inner comment to "Use the current binary to run tap export pdf".

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cli ./internal/tui -run 'TestOldCommandNames|TestCommandTree|TestEveryCommandFollows|TestPDFExportArgs|TestDevModel_HandleKeyPress_Reload|TestPresent' -short -v`
Expected: PASS. `TestDevModel_HandleKeyPress_Reload` confirms that `r` reloads in `tap dev`.

Run the whole Go suite once:

Run: `go test ./... -short`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/renamed.go internal/cli/renamed_test.go internal/cli/conventions_test.go internal/tui/dev.go internal/tui/dev_test.go
git commit -m "feat(cli): name the new command when an old name is used"
```

---

### Task 10: Run only the deck's own live code

This task is a plan decision (11). See "Safety: the registry must not ship without a guard".

**Files:**
- Modify: `internal/server/api.go`
- Modify: `internal/server/api_test.go`

**Interfaces:**
- Produces: `func (s *Server) deckHasLiveBlock(driverName, connection, code string) bool`. `/api/execute` answers 403 with `{"success": false, "error": "This code is not a live code block in the loaded deck"}` when it is false.

- [ ] **Step 1: Write the failing tests**

In `internal/server/api_test.go`, add a helper and two tests:

```go
// presentationWithBlock returns a presentation with one slide that holds
// one live code block.
func presentationWithBlock(driverName, connection, code string) *transformer.TransformedPresentation {
	return &transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{{
			CodeBlocks: []transformer.TransformedCodeBlock{{
				Language:   "bash",
				Code:       code,
				Driver:     driverName,
				Connection: connection,
			}},
		}},
	}
}

func TestHandleAPIExecute_RejectsCodeThatIsNotInTheDeck(t *testing.T) {
	s := New(0)
	registry := driver.NewRegistry()
	registry.Register(&mockDriver{name: "test", result: driver.Result{Success: true, Output: "ran"}})
	s.SetRegistry(registry)
	s.SetPresentation(presentationWithBlock("test", "", "echo deck"))

	for _, body := range []ExecuteRequest{
		{Driver: "test", Code: "echo attacker"},
		{Driver: "test", Code: "echo deck", Connection: "other"},
		{Driver: "shell", Code: "echo deck"},
	} {
		bodyBytes, _ := json.Marshal(body)
		request := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(bodyBytes))
		recorder := httptest.NewRecorder()
		s.handleAPIExecute(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Errorf("%+v: status = %d, want %d", body, recorder.Code, http.StatusForbidden)
		}
	}
}

func TestHandleAPIExecute_RejectsEverythingWithNoDeck(t *testing.T) {
	s := New(0)
	registry := driver.NewRegistry()
	registry.Register(&mockDriver{name: "test", result: driver.Result{Success: true}})
	s.SetRegistry(registry)

	bodyBytes, _ := json.Marshal(ExecuteRequest{Driver: "test", Code: "echo hi"})
	request := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(bodyBytes))
	recorder := httptest.NewRecorder()
	s.handleAPIExecute(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}
```

Add the `transformer` import (`github.com/MiniCodeMonkey/tap/internal/transformer`) if the test file does not have it.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/server -run 'TestHandleAPIExecute' -v`
Expected: the two new tests FAIL with status 200.

- [ ] **Step 3: Add the guard in `internal/server/api.go`**

After the `s.registry == nil` check and before the `s.registry.Has` check, add:

```go
	// Only code that is a live block in the loaded deck runs. The server
	// listens on every interface, and a client outside a browser can send
	// any Origin header, so the same-origin check alone does not stop a
	// request from running arbitrary code.
	if !s.deckHasLiveBlock(req.Driver, req.Connection, req.Code) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(ExecuteResponse{
			Success: false,
			Error:   "This code is not a live code block in the loaded deck",
		})
		return
	}
```

Add the method below `getExecutionTimeout`:

```go
// deckHasLiveBlock reports whether the loaded deck has a live code block
// with exactly this driver, connection and code.
func (s *Server) deckHasLiveBlock(driverName, connection, code string) bool {
	presentation := s.GetPresentation()
	if presentation == nil {
		return false
	}
	for _, slide := range presentation.Slides {
		for _, block := range slide.CodeBlocks {
			if block.Driver == driverName && block.Connection == connection && block.Code == code {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 4: Update the existing execute tests**

The tests that expect a driver to run now need the block in the deck. In each of these, add `s.SetPresentation(presentationWithBlock(<driver>, <connection>, <code>))` with the same values as the request body, right after `s.SetRegistry(...)`:

- `TestHandleAPIExecute_Success`
- `TestHandleAPIExecute_ExecutionError`
- every other test in `api_test.go` that calls `SetRegistry` and expects a status other than 403

The test that checks "driver not found" sends a driver the registry lacks. Give it a deck block with that driver too, so the request reaches the `Has` check and still gets 400.

`TestHandleAPIExecute` tests that check the 500 "Driver registry not configured" response keep working unchanged, because that check runs first.

If a test sets a presentation with `Config.Drivers` for timeouts or connections, add the `CodeBlocks` to that same presentation instead of replacing it.

Run: `grep -n "SetRegistry" internal/server/*_test.go internal/server/routes_test.go`
Check every hit.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/server -v -run 'Execute'`
Expected: PASS.

Run: `go test ./internal/server`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/server/api.go internal/server/api_test.go
git commit -m "fix(server): run only code that is a live block in the loaded deck"
```

---

### Task 11: Listen on 127.0.0.1 unless `--lan` is given

**Files:**
- Create: `internal/cli/network.go`
- Create: `internal/cli/network_test.go`
- Modify: `internal/cli/dev.go`, `internal/cli/present.go`, `internal/cli/root_test.go`
- Modify: `internal/server/routes.go`, `internal/server/server.go`, `internal/server/routes_test.go`
- Modify: `internal/tui/dev.go`, `internal/tui/dev_test.go`

**Interfaces:**
- Consumes: `server.NewWithHost` (existing), `server.GeneratePresenterURL`, `server.GenerateCompactQRCode` (existing, `internal/server/qr.go`)
- Produces:
  - `func listenHost(lan bool) string`: `"0.0.0.0"` with `--lan`, else `"127.0.0.1"`
  - `func lanPresenterAddress(port int, presenterPassword string) (presenterURL, qrCode string, ok bool)`
  - `serverOptions.lan bool`; the flag `--lan` on `tap dev` and `tap present`
  - `func (s *Server) ListensOnLoopbackOnly() bool` in `internal/server`
  - `DevConfig.NetworkURL string` in `internal/tui`

**What exists today.** `runDevServer` builds each server with `server.New`, which binds `0.0.0.0`. The TUI has a `DevConfig.QRCodeASCII` field, and `DevModel.qrCode()` falls back to it when no tunnel runs. But `runDevServer` never sets that field, so today the TUI shows a QR code only for a tunnel. The presenter-mode guide describes a "Network:" URL and a LAN QR code that the TUI does not show. `GET /qr` renders an HTML page with the LAN address (`GenerateAudienceURL` auto-detects the IP), and nothing in the frontend links to it.

**After this task.**
- `tap dev` and `tap present` bind `127.0.0.1`. The TUI and headless output show only `localhost` URLs.
- With `--lan`, they bind `0.0.0.0`. The TUI shows a "Network:" line with the LAN presenter URL, and the LAN QR code through the existing `QRCodeASCII` fallback. Headless mode prints the same "Network:" line.
- `--tunnel` works with or without `--lan`. `cloudflared` connects to `http://127.0.0.1:<port>` (`internal/tunnel/tunnel.go`), which a loopback listener accepts. A running tunnel's QR code still wins over the LAN one, as `qrCode()` does today.
- `GET /qr` on a loopback-only server answers 404 with `The QR page needs the server on the network: start tap dev with --lan`. Its LAN URLs would not work there.
- `tap serve` and the temporary export server do not change. `tap serve` serves only a static build and runs no code. The export server already binds `127.0.0.1`.
- The audience URL stays `http://localhost:<port>`. Browsers that try `::1` first fall back to `127.0.0.1`.

- [ ] **Step 1: Write the failing tests**

`internal/cli/network_test.go`:

```go
package cli

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestListenHost(t *testing.T) {
	if got := listenHost(false); got != "127.0.0.1" {
		t.Errorf("listenHost(false) = %q, want 127.0.0.1", got)
	}
	if got := listenHost(true); got != "0.0.0.0" {
		t.Errorf("listenHost(true) = %q, want 0.0.0.0", got)
	}
}

// firstLANAddress returns a non-loopback IPv4 address of this machine, or
// skips the test when there is none.
func firstLANAddress(t *testing.T) string {
	t.Helper()
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		t.Skipf("cannot list interface addresses: %v", err)
	}
	for _, address := range addresses {
		prefix, ok := address.(*net.IPNet)
		if ok && !prefix.IP.IsLoopback() && prefix.IP.To4() != nil {
			return prefix.IP.String()
		}
	}
	t.Skip("this machine has no non-loopback IPv4 address")
	return ""
}

// startHeadlessDev starts tap dev --headless on a free port with extra
// arguments, waits until it answers on 127.0.0.1, and stops it when the
// test ends.
func startHeadlessDev(t *testing.T, extra ...string) int {
	t.Helper()
	binary := buildTapBinaryForTest(t)
	deckPath := filepath.Join(t.TempDir(), "deck.md")
	if err := os.WriteFile(deckPath, []byte("---\ntitle: Net\n---\n\n# One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	args := append([]string{"dev", deckPath, "--headless", "--port", fmt.Sprint(port)}, extra...)
	command := exec.Command(binary, args...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Signal(syscall.SIGINT)
		_ = command.Wait()
	})

	deadline := time.Now().Add(20 * time.Second)
	for {
		connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			connection.Close()
			return port
		}
		if time.Now().After(deadline) {
			t.Fatalf("tap dev did not start:\n%s", output.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestDevListensOnLoopbackOnlyByDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	lanAddress := firstLANAddress(t)
	port := startHeadlessDev(t)

	connection, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", lanAddress, port), time.Second)
	if err == nil {
		connection.Close()
		t.Errorf("tap dev answered on %s:%d, want loopback only", lanAddress, port)
	}
}

func TestDevListensOnTheNetworkWithLAN(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	lanAddress := firstLANAddress(t)
	port := startHeadlessDev(t, "--lan")

	connection, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", lanAddress, port), time.Second)
	if err != nil {
		t.Errorf("tap dev --lan did not answer on %s:%d: %v", lanAddress, port, err)
		return
	}
	connection.Close()
}
```

Add `freePort` to `network_test.go` too. Task 12 reuses it:

```go
// freePort returns a TCP port that is free right now.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
```

The two tests skip on a machine with no LAN address, such as some CI containers. The unit test `TestListenHost` always runs.

In `internal/cli/root_test.go`, add `"lan"` to the flags `TestPresentCommandIsRegistered` expects, and add:

```go
func TestDevHasTheLANFlag(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"dev"})
	if err != nil {
		t.Fatal(err)
	}
	if command.Flags().Lookup("lan") == nil {
		t.Error("dev lacks --lan")
	}
}
```

In `internal/server/routes_test.go`, add:

```go
func TestQRPageExplainsLANOnALoopbackServer(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	s.SetupRoutes()
	request := httptest.NewRequest(http.MethodGet, "/qr", nil)
	request.Host = "localhost"
	recorder := httptest.NewRecorder()
	s.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if !strings.Contains(recorder.Body.String(), "--lan") {
		t.Errorf("body = %q, want it to name --lan", recorder.Body.String())
	}
}

func TestListensOnLoopbackOnly(t *testing.T) {
	if !NewWithHost(0, "127.0.0.1").ListensOnLoopbackOnly() {
		t.Error("127.0.0.1 should be loopback only")
	}
	if NewWithHost(0, "0.0.0.0").ListensOnLoopbackOnly() {
		t.Error("0.0.0.0 is not loopback only")
	}
}
```

Check how the existing tests in `routes_test.go` send requests through the mux (field `s.mux` or a `Handler()` method), and do the same.

In `internal/tui/dev_test.go`, add:

```go
func TestViewURLsShowsTheNetworkURL(t *testing.T) {
	model := NewDevModel(DevConfig{
		AudienceURL:  "http://localhost:3000",
		PresenterURL: "http://localhost:3000/presenter",
		NetworkURL:   "http://192.168.1.20:3000/presenter",
	})
	if !strings.Contains(model.viewURLs(), "http://192.168.1.20:3000/presenter") {
		t.Error("viewURLs() does not show the network URL")
	}
	plain := NewDevModel(DevConfig{AudienceURL: "http://localhost:3000"})
	if strings.Contains(plain.viewURLs(), "Network") {
		t.Error("viewURLs() shows a Network line without --lan")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli ./internal/server ./internal/tui -run 'TestListenHost|TestDevHasTheLANFlag|TestPresentCommandIsRegistered|TestQRPageExplainsLAN|TestListensOnLoopbackOnly|TestViewURLsShowsTheNetworkURL' -short`
Expected: compile failures, `undefined: listenHost`, `undefined: ListensOnLoopbackOnly`, `unknown field NetworkURL`.

Run: `go test ./internal/cli -run TestDevListensOnLoopbackOnlyByDefault -v`
Expected: FAIL, tap dev answers on the LAN address (or SKIP with no LAN address).

- [ ] **Step 3: Write `internal/cli/network.go`**

```go
package cli

import (
	"net/url"

	"github.com/MiniCodeMonkey/tap/internal/server"
)

// listenHost is the address tap dev and tap present listen on. They
// listen on loopback only, unless --lan opens them to the local network.
func listenHost(lan bool) string {
	if lan {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}

// lanPresenterAddress returns the presenter URL on this machine's LAN
// address and a QR code of it, for a phone on the same network. ok is
// false when the machine has no LAN address, or when the QR code cannot
// be drawn.
func lanPresenterAddress(port int, presenterPassword string) (presenterURL, qrCode string, ok bool) {
	presenterURL, err := server.GeneratePresenterURL(server.QRConfig{Port: port, PresenterPassword: presenterPassword})
	if err != nil {
		return "", "", false
	}
	parsed, err := url.Parse(presenterURL)
	if err != nil || parsed.Hostname() == "localhost" {
		return "", "", false
	}
	qrCode, err = server.GenerateCompactQRCode(presenterURL)
	if err != nil {
		return presenterURL, "", true
	}
	return presenterURL, qrCode, true
}
```

`GeneratePresenterURL` falls back to `localhost` when it finds no LAN address. That fallback is why the function checks the host.

- [ ] **Step 4: Add `ListensOnLoopbackOnly` and the `/qr` message (`internal/server`)**

In `server.go`, below `Port()`:

```go
// ListensOnLoopbackOnly reports whether the server accepts connections
// only from this machine.
func (s *Server) ListensOnLoopbackOnly() bool {
	s.mu.RLock()
	address := s.addr
	s.mu.RUnlock()
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
```

`server.go` already imports `net`.

In `routes.go`, at the start of `handleQR`, before the presenter check:

```go
	// The page shows LAN addresses, which a server that listens on
	// loopback only does not answer.
	if s.ListensOnLoopbackOnly() {
		http.Error(w, "The QR page needs the server on the network: start tap dev with --lan", http.StatusNotFound)
		return
	}
```

- [ ] **Step 5: Show the network URL in the TUI (`internal/tui/dev.go`)**

Add a field to `DevConfig`, next to `PresenterURL`:

```go
	// NetworkURL is the presenter URL on this machine's LAN address. It is
	// set only when the server listens on the network (--lan).
	NetworkURL string
```

In `viewURLs`, after the presenter line and its password note, add:

```go
	if m.config.NetworkURL != "" {
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Network:"))
		b.WriteString(urlStyle.Render(m.config.NetworkURL))
	}
```

`qrCode()` already falls back to `m.config.QRCodeASCII`, so no change is needed there. Update its comment only if it no longer matches: it says "the one the config carried in", which is the LAN QR code now.

- [ ] **Step 6: Add `--lan` and bind the host (`internal/cli/dev.go`, `present.go`)**

1. Add `lan bool` to `serverOptions`, with the comment `// lan listens on every interface, so a phone on the same network can connect.`
2. Add the flag to both commands:

```go
	devCmd.Flags().BoolVar(&devLAN, "lan", false, "listen on the local network too, so a phone on the same network can open the presenter view (default: this machine only)")
```

```go
	presentCmd.Flags().BoolVar(&presentLAN, "lan", false, "listen on the local network too, so a phone on the same network can open the presenter view (default: this machine only)")
```

Add `devLAN bool` and `presentLAN bool` to each command's flag `var` block, and pass `lan: devLAN` and `lan: presentLAN` in each `serverOptions{...}`.

3. In `buildServer`, change `candidate := server.New(candidatePort)` to:

```go
		candidate := server.NewWithHost(candidatePort, listenHost(options.lan))
```

4. After `port = srv.Port()`, compute the LAN address once:

```go
	var networkURL, networkQRCode string
	if options.lan {
		var found bool
		networkURL, networkQRCode, found = lanPresenterAddress(port, presenterPassword)
		if !found {
			Warning("--lan: no local network address found; only this machine can connect\n")
		}
	}
```

5. In the headless block, after the `Presenter:` line, add:

```go
		if networkURL != "" {
			fmt.Printf("  Network:   %s\n", networkURL)
		}
```

6. In `tuiCfg`, set `NetworkURL: networkURL` and `QRCodeASCII: networkQRCode`.
7. Add `tap dev slides.md --lan                # Let a phone on the same network connect` to the `dev` examples, and a line to the `Long` text: "The server listens on this machine only. --lan opens it to the local network, and --tunnel puts it on a public https URL."

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/cli ./internal/server ./internal/tui -run 'TestListenHost|TestDevHasTheLANFlag|TestPresentCommandIsRegistered|TestQRPage|TestListensOnLoopbackOnly|TestViewURLs|TestEveryCommandFollows' -short -v`
Expected: PASS.

Run: `go test ./internal/cli -run 'TestDevListensOn' -v`
Expected: PASS, or SKIP on a machine with no LAN address.

Run: `go test ./... -short`
Expected: PASS. If a server test relied on `server.New` in `runDevServer`, it does not exist: only `runDevServer` changes, and `server.New` still binds `0.0.0.0` for its other callers.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/network.go internal/cli/network_test.go internal/cli/dev.go internal/cli/present.go internal/cli/root_test.go internal/server/server.go internal/server/routes.go internal/server/routes_test.go internal/tui/dev.go internal/tui/dev_test.go
git commit -m "feat(dev)!: listen on 127.0.0.1 unless --lan is given"
```

---

### Task 12: Connect the driver registry in `tap dev` and `tap present`

**Files:**
- Create: `internal/cli/drivers.go`
- Create: `internal/cli/drivers_test.go`
- Create: `internal/cli/live_code_test.go`
- Modify: `internal/cli/dev.go`
- Modify: `internal/driver/custom.go`
- Modify: `internal/driver/custom_test.go`

**Interfaces:**
- Consumes: the guard (Task 10); the loopback default and `freePort` (Task 11); `buildTapBinaryForTest` (existing, `export_images_test.go`)
- Produces:
  - `DriverConfigInput.WorkingDir string` in `internal/driver`
  - `func buildDriverRegistry(cfg *config.Config, baseDir string) *driver.Registry`

- [ ] **Step 1: Write the failing tests**

`internal/cli/drivers_test.go`:

```go
package cli

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
)

func TestBuildDriverRegistryHasTheBuiltInDrivers(t *testing.T) {
	registry := buildDriverRegistry(&config.Config{}, t.TempDir())
	names := registry.List()
	sort.Strings(names)
	if strings.Join(names, ",") != "mysql,postgres,shell,sqlite" {
		t.Errorf("drivers = %v, want the four built-in drivers", names)
	}
}

func TestBuildDriverRegistryAddsCustomDrivers(t *testing.T) {
	cfg := &config.Config{Drivers: map[string]config.DriverConfig{
		"python":  {Command: "python3", Args: []string{"-"}},
		"sqlite":  {Connections: map[string]config.ConnectionConfig{"demo": {Path: "demo.db"}}},
		"nothing": {},
	}}
	registry := buildDriverRegistry(cfg, t.TempDir())
	if !registry.Has("python") {
		t.Error("the custom python driver is missing")
	}
	if registry.Has("nothing") {
		t.Error("a drivers: entry with no command is not a custom driver")
	}
}

func TestBuildDriverRegistryRunsInTheDeckFolder(t *testing.T) {
	deckFolder := t.TempDir()
	cfg := &config.Config{Drivers: map[string]config.DriverConfig{
		"pwd": {Command: "pwd"},
	}}
	registry := buildDriverRegistry(cfg, deckFolder)
	for _, name := range []string{"shell", "pwd"} {
		code := "pwd"
		if name == "pwd" {
			code = ""
		}
		result := registry.Execute(context.Background(), name, code, map[string]string{})
		if !result.Success || !strings.HasSuffix(strings.TrimSpace(result.Output), deckFolder) {
			t.Errorf("%s ran in %q, want the deck folder %q (error %q)", name, result.Output, deckFolder, result.Error)
		}
	}
}
```

On macOS, `t.TempDir()` is under `/var/folders`, which `pwd` may print as `/private/var/folders`. `HasSuffix` handles that.

`internal/cli/live_code_test.go`:

```go
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const liveCodeDeck = "---\ntitle: Live code\n---\n\n# Run it\n\n```bash {driver: 'shell'}\necho hello from tap\n```\n"

// TestDevRunsALiveShellBlock starts a real tap dev and runs the deck's
// shell block through /api/execute, as the Run button does.
func TestDevRunsALiveShellBlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	binary := buildTapBinaryForTest(t)

	deckPath := filepath.Join(t.TempDir(), "live.md")
	if err := os.WriteFile(deckPath, []byte(liveCodeDeck), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	command := exec.Command(binary, "dev", deckPath, "--headless", "--port", fmt.Sprint(port))
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Signal(syscall.SIGINT)
		_ = command.Wait()
	})

	base := fmt.Sprintf("http://localhost:%d", port)
	var presentation struct {
		Slides []struct {
			CodeBlocks []struct {
				Code   string `json:"code"`
				Driver string `json:"driver"`
			} `json:"codeBlocks"`
		} `json:"slides"`
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		response, err := http.Get(base + "/api/presentation")
		if err == nil && response.StatusCode == http.StatusOK {
			err = json.NewDecoder(response.Body).Decode(&presentation)
			response.Body.Close()
			if err != nil {
				t.Fatalf("decoding /api/presentation: %v", err)
			}
			break
		}
		if err == nil {
			response.Body.Close()
		}
		if time.Now().After(deadline) {
			t.Fatalf("tap dev did not start:\n%s", output.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(presentation.Slides) == 0 || len(presentation.Slides[0].CodeBlocks) == 0 {
		t.Fatalf("the deck has no code block: %+v", presentation)
	}
	block := presentation.Slides[0].CodeBlocks[0]

	execute := func(code string) (int, string) {
		body, _ := json.Marshal(map[string]string{"driver": block.Driver, "code": code})
		request, _ := http.NewRequest(http.MethodPost, base+"/api/execute", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", base)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("POST /api/execute: %v", err)
		}
		defer response.Body.Close()
		var result struct {
			Output string `json:"output"`
			Error  string `json:"error"`
		}
		_ = json.NewDecoder(response.Body).Decode(&result)
		return response.StatusCode, result.Output + result.Error
	}

	status, text := execute(block.Code)
	if status != http.StatusOK || !strings.Contains(text, "hello from tap") {
		t.Errorf("the deck's block: status %d, output %q, want 200 and \"hello from tap\"", status, text)
	}

	status, _ = execute("echo not in the deck")
	if status != http.StatusForbidden {
		t.Errorf("code outside the deck: status %d, want 403", status)
	}
}
```

The endpoint and field names come from `internal/server/routes.go` (`GET /api/presentation`) and `internal/transformer/transformer.go` (`codeBlocks`, `code`, `driver`). Check both before running. If the presentation route has another path, use that path.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestBuildDriverRegistry' -short`
Expected: compile failure, `undefined: buildDriverRegistry`.

Run: `go test ./internal/cli -run TestDevRunsALiveShellBlock -v`
Expected: FAIL with status 500 ("Driver registry not configured").

- [ ] **Step 3: Give custom drivers a working folder**

In `internal/driver/custom.go`, add `WorkingDir string` to `DriverConfigInput`:

```go
type DriverConfigInput struct {
	Args       []string
	Command    string
	WorkingDir string
	Timeout    int
}
```

In `RegisterCustomDrivers`, pass it on:

```go
		driver := NewCustomDriver(CustomDriverConfig{
			Name:       name,
			Command:    cfg.Command,
			Args:       cfg.Args,
			Timeout:    cfg.Timeout,
			WorkingDir: cfg.WorkingDir,
		})
```

Add a case to `internal/driver/custom_test.go`:

```go
func TestRegisterCustomDriversSetsTheWorkingDir(t *testing.T) {
	registry := NewRegistry()
	RegisterCustomDrivers(registry, map[string]DriverConfigInput{
		"python": {Command: "python3", WorkingDir: "/tmp/deck"},
	})
	custom, ok := registry.Get("python").(*CustomDriver)
	if !ok {
		t.Fatal("python is not a *CustomDriver")
	}
	if custom.WorkingDir != "/tmp/deck" {
		t.Errorf("WorkingDir = %q, want /tmp/deck", custom.WorkingDir)
	}
}
```

- [ ] **Step 4: Write `internal/cli/drivers.go`**

```go
package cli

import (
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/driver"
)

// buildDriverRegistry returns the drivers a deck's live code blocks run
// with: the four built-in drivers, and a custom driver for each entry in
// the deck's drivers: map that sets a command. Every driver runs in the
// deck's folder.
func buildDriverRegistry(cfg *config.Config, baseDir string) *driver.Registry {
	registry := driver.NewRegistry()
	registry.Register(driver.NewShellDriver(baseDir))
	registry.Register(driver.NewSQLiteDriver(baseDir))
	registry.Register(driver.NewMySQLDriver(baseDir))
	registry.Register(driver.NewPostgresDriver(baseDir))

	custom := make(map[string]driver.DriverConfigInput, len(cfg.Drivers))
	for name, settings := range cfg.Drivers {
		custom[name] = driver.DriverConfigInput{
			Args:       settings.Args,
			Command:    settings.Command,
			WorkingDir: baseDir,
			Timeout:    settings.Timeout,
		}
	}
	driver.RegisterCustomDrivers(registry, custom)
	return registry
}
```

- [ ] **Step 5: Set the registry in `runDevServer` (`internal/cli/dev.go`)**

1. In `buildServer`, after `candidate.SetBaseDir(baseDir)`, add:

```go
		candidate.SetRegistry(buildDriverRegistry(cfg, baseDir))
```

2. The deck's `drivers:` map can change on every save, so each reload builds the registry again. There are three reload paths: the first `watcher.SetOnChange` handler, the headless handler, and `reloadInTUI`. In each one, right before `srv.SetPresentation(newPres)`, add:

```go
			srv.SetRegistry(buildDriverRegistry(newCfg, baseDir))
```

`tap present` runs `runDevServer` too, so the `r` key reload in `tap present` also rebuilds it.

3. `tap build` does not call `runDevServer` and does not change. `prepareDeck` (the temporary server for exports) does not get a registry, so exports never run code.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/driver -run 'TestRegisterCustomDrivers' -v && go test ./internal/cli -run 'TestBuildDriverRegistry' -short -v`
Expected: PASS.

Run: `go test ./internal/cli -run TestDevRunsALiveShellBlock -v`
Expected: PASS.

Run: `go test ./... -short`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/drivers.go internal/cli/drivers_test.go internal/cli/live_code_test.go internal/cli/dev.go internal/driver/custom.go internal/driver/custom_test.go
git commit -m "fix(dev): connect the driver registry so live code runs in tap dev and tap present"
```

---

### Task 13: Docs, skill and changelog

**Files:**
- Modify: `README.md`, `CONTRIBUTING.md`
- Modify: `docs/reference/cli-commands.md`, `docs/reference/keyboard-shortcuts.md`, `docs/reference/components-reference.md`
- Modify: `docs/guide/building-export.md`, `docs/guide/custom-components.md`, `docs/guide/themes.md`, `docs/guide/ai-images.md`, `docs/guide/ai-skills.md`, `docs/guide/live-code-execution.md`
- Modify: `skills/tap/SKILL.md`, `skills/tap/rules/cli.md`, `skills/tap/rules/components.md`, `skills/tap/rules/getting-started.md`, `skills/tap/rules/best-practices.md`, `skills/tap/rules/themes.md`
- Modify: code comments in `frontend/src/**` and `internal/**` that name old commands
- Modify: `CHANGELOG.md`, `docs/changelog.md`

Do not edit `docs/superpowers/**`, `tasks/**`, `SPEC.md`, or released sections of the changelogs. They are history.

- [ ] **Step 1: List every mention**

Run:

```bash
grep -rnE "tap (pdf|screenshot|add)\b|--out\b|--deck\b|--verbose|theme list --json|--fragment|--step" \
  README.md CONTRIBUTING.md docs/reference docs/guide docs/getting-started.md skills \
  frontend/src internal --include='*.md' --include='*.go' --include='*.ts' --include='*.tsx' --include='*.css'
```

Use this list as the checklist for steps 2 to 4.

- [ ] **Step 2: Replace the command names**

In every file from step 1:

| Old | New |
|---|---|
| `tap pdf <file>` | `tap export pdf [deck]` |
| `tap screenshot <file>` | `tap export images [deck]` |
| `screenshot --out x` | `export images --output x` (or `-o x`) |
| `tap add` | `tap slide add` |
| `tap add component <Name> --deck <deck>` | `tap component new <Name> [deck]` |
| `tap theme show --deck <deck>` | `tap theme show <deck>` |
| `tap dev <file>`, `tap build <file>` | `tap dev [deck]`, `tap build [deck]` |

In code comments, only the command names change. Do not reword the rest of the comment.

- [ ] **Step 3: Rewrite `docs/reference/cli-commands.md` and `skills/tap/rules/cli.md`**

Both files describe each command. Change them to match the new tree, in this order: `new`, `dev`, `present`, `build`, `serve`, `export pdf`, `export images`, `slide add`, `component new`, `theme list`, `theme show`. Add a "Conventions" section to both, with these facts:

- `[deck]` is optional. It is a deck file or a folder. With no deck, tap uses the only deck in the current folder, opens a picker on a terminal, or exits with the list of decks.
- `--output/-o`, `--theme/-t`, `--port/-p`, `--yes/-y` and `--json` mean the same thing on every command.
- `--json` prints `{"ok": true, ...}` or `{"ok": false, "error": {"code": "...", "message": "..."}}`. List the `--json` result fields of each command in its own section.
- Numbers are 1-based. `--step N` is the slide after N steps, `--fragment N` is the slide with N fragments shown, 0 is the state before the first one, and a flag you leave out means the final state.
- Exit codes: 0 success, 1 a problem you can fix, 2 a problem in tap or its environment, 130 interrupted.
- The old names `tap pdf`, `tap screenshot`, `tap add` and `tap add component` print the new name and exit 1.

Remove `--verbose` from any global flag list.

- [ ] **Step 4: Update the live code guide**

In `docs/guide/live-code-execution.md`, add a short section "What tap runs": `tap dev` and `tap present` run only the live code blocks that are in the deck. A request with any other code gets 403. `tap build` and `tap export` never run code.

- [ ] **Step 4a: Document `--lan`**

- In `docs/reference/cli-commands.md` and `skills/tap/rules/cli.md`, add `--lan` to the flag tables of `tap dev` and `tap present`: "Listen on the local network too, so a phone on the same network can open the presenter view. Without it, only this machine can connect." Where the reference says the dev server is reachable from the network, say that this needs `--lan`.
- In `docs/guide/presenter-mode.md`, "Using an iPad or Phone as a Controller": step 1 becomes "Start `tap dev --lan` on your laptop". Step 3 becomes "Scan the QR code in the terminal, or open the Network URL it shows".
- In the same guide, rewrite "QR Code for Easy Access" to match the TUI: with `--lan`, the terminal shows a `Network:` presenter URL and a QR code for it. With `--tunnel`, it shows the tunnel's QR code instead. With neither, it shows no QR code.
- Where `/qr` is described (`docs/guide/presenter-mode.md`, `docs/reference/cli-commands.md`), say that it needs `--lan`.

- [ ] **Step 5: Add the changelog entries**

In `CHANGELOG.md` and `docs/changelog.md`, under `## [Unreleased]`, add these. Match each file's existing style.

Under `### Changed`:

```markdown
- **Commands are grouped by noun** - `tap pdf` is now `tap export pdf`, `tap screenshot` is `tap export images`, `tap add` is `tap slide add`, and `tap add component` is `tap component new`. The old names print the new one and exit 1. There are no aliases.
- **Every command finds the deck the same way** - `[deck]` is optional on `dev`, `present`, `build`, `new`, `export pdf`, `export images`, `slide add`, `component new` and `theme show`. It takes a file or a folder. With no deck, tap uses the only deck in the folder, opens a picker on a terminal, or exits with the list of decks. `--deck` is removed from `component new` and `theme show`.
- **One set of flags** - `--output/-o` replaces `tap screenshot --out`, and `export images` gets `-t` for `--theme`. The unused global `--verbose` flag is removed.
- **`--step` and `--fragment` are 1-based** - `tap export images --step 2` renders the slide after its second step, and `--fragment 1` shows the first fragment. A flag you leave out means the final state. `--fragment` used to count from 0.
- **One `--json` shape** - `new`, `build`, `export pdf`, `export images`, `component new`, `theme list` and `theme show` print `{"ok": true, ...}` or `{"ok": false, "error": {"code", "message"}}`. `tap theme list --json` used to print a bare array. It now prints `{"ok": true, "themes": [...]}`.
- **Exit codes** - 0 on success, 1 for a problem you can fix, 2 for a problem in tap or its environment, and 130 when interrupted.
```

Under `### Fixed`:

```markdown
- **Live code runs in `tap dev` and `tap present`** - The Run button answered "Driver registry not configured" in every real run. Both commands now load the built-in drivers and the deck's custom `drivers:`, run them in the deck's folder, and reload them when the deck changes.
```

Under `### Security`:

```markdown
- **Only the deck's own code runs** - `/api/execute` runs a request only when its driver, connection and code are a live code block in the loaded deck, and answers 403 otherwise. With `--lan` or `--tunnel` other devices can reach the server, and a client outside a browser can send any `Origin` header, so the same-origin check alone does not stop other code.
```

Also under `### Security`, because this changes the default network exposure:

```markdown
- **`tap dev` and `tap present` listen on this machine only** - They used to listen on every network interface, so any device on the same network could open the deck, and could call `/api/execute`. They now listen on `127.0.0.1`. Pass `--lan` to let a phone on the same network open the presenter view. The terminal then shows the network URL and a QR code for it. `--tunnel` works without `--lan`. `/qr` answers 404 without `--lan`, because its network URLs would not work.
```

The `### Fixed` and both `### Security` entries go in PR 2 (Tasks 10, 11 and 12), with the step 4 and step 4a docs. The `### Changed` entries go in PR 1.

- [ ] **Step 6: Check the docs build and the frontend**

Run: `cd docs && npm run build`
Expected: the build succeeds with no broken links. (Skip this if `docs/node_modules` is missing, and say so in the PR.)

Run: `cd frontend && npm test -- --run`
Expected: PASS. Only comments changed in the frontend.

Run the grep from step 1 again.
Expected: hits only in the changelog entries and the "old names" sentence of the conventions sections.

- [ ] **Step 7: Commit**

```bash
git add README.md CONTRIBUTING.md docs skills frontend/src internal CHANGELOG.md
git commit -m "docs: describe the noun-grouped command tree and its conventions"
```

---

## Final check

- [ ] Run `go test ./...` (not `-short`, so the subprocess and browser tests run).
- [ ] Run `go vet ./...` and the repo linter, if the Makefile has one (`make lint`).
- [ ] Build the binary and try the new tree by hand in a folder with one deck and in a folder with two:
  - `tap dev`, `tap export pdf --json`, `tap export images --slide 1 --step 1 -o /tmp/s.png`, `tap theme show`, `tap component new Demo`, `tap pdf` (prints the rename line, exits 1), `echo $?` after each.
  - In `tap dev`, click Run on a shell block and see its output.
  - Start `tap dev` and open `http://<LAN IP>:3000` from a phone: it must fail. Start `tap dev --lan`: the TUI shows a `Network:` URL and a QR code, and the phone opens the presenter view. Start `tap dev --tunnel` without `--lan`: the tunnel URL works from the phone.

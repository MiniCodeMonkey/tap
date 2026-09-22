# Tap Desktop roadmap: every plan, in order

> **For agentic workers:** This is the index of implementation plans for the tap prerequisites and Tap Desktop. Each row below is one plan file. Execute a plan only with superpowers:subagent-driven-development, one fresh subagent per task.

**Goal:** Ship the tap prerequisites (parts 1 to 6) and the Tap Desktop app (milestones 2 to 7).

**Specs:**
- `docs/superpowers/specs/2026-09-22-tap-desktop-prerequisites-design.md` (the tap work, parts 1 to 6)
- `docs/superpowers/specs/2026-09-22-tap-desktop-design.md` (the app, milestones 1 to 7)
- `docs/superpowers/specs/tap-desktop-features/*.feature` (acceptance criteria, one test per scenario)
- `docs/superpowers/specs/tap-desktop-features/spike-native-prototype.md` (what the prototype proved)

## Plans

| ID | Plan file | Covers | Depends on | State |
|---|---|---|---|---|
| P1 | `2026-09-22-cli-command-tree-and-driver-registry.md` | Prerequisites part 1, section 2.1, the loopback default | none | approved |
| P2 | `2026-09-22-live-code-by-reference-and-approvals.md` | Sections 2.2 to 2.5 | P1, P3 | written |
| P3 | `2026-09-22-deck-features-skip-schema-slide-list.md` | Part 3 | P1 | written |
| P4 | `2026-09-22-commands-behind-tui-keys.md` | Part 4 | P1, P3 | written |
| P5 | `2026-09-22-ready-signal-updates-progress.md` | Part 5 | P1 (command names only) | written |
| P6 | `2026-09-22-app-mode.md` | Part 6 | P2, P3, P5 | written |
| D2 | `2026-09-22-desktop-app-shell.md` | Desktop milestone 2 | P6 | written |
| D3 | `desktop-sidebar-thumbnails-slide-operations.md` | Desktop milestone 3 | D2, P4 (`slide add --print`) | outline below; detailed plan written when D2 is done |
| D4 | `desktop-presenting.md` | Desktop milestone 4 | D3 | outline below |
| D5 | `desktop-live-code-deck-tab-fixits.md` | Desktop milestone 5 | D4, P2 | outline below |
| D6 | `desktop-create-export-settings.md` | Desktop milestone 6 | D5, P4 | outline below |
| D7 | `desktop-release.md` | Desktop milestone 7 | D6 | outline below; needs credentials the machine does not have |

Desktop milestone 1 is P1 to P6. It has no plan of its own.

Detailed desktop plans are written one milestone ahead, not all at once. Each one builds on Swift code that the previous milestone creates, and a plan written against code that does not exist yet goes stale before it runs.

## Execution order

1. P1 runs first. It renames commands and adds the helpers every later plan uses.
2. P3 and P5 run next, in parallel, in separate worktrees. P3 changes the parser, transformer and CLI. P5 changes the frontend, the WebSocket hub and the PDF exporter. They touch different files.
3. P2 and P4 follow, after P3 has merged, because both number slides with `slidelist.Build`. P2 changes `/api/execute`, `runDevServer` and the user settings. P4 adds CLI commands.
4. P6 runs after P2, P3 and P5 are merged.
5. D2 to D6 run in order. Each ends with a working app.
6. D7 runs when signing credentials exist.

Each plan is one branch from an up-to-date `main`, in its own worktree under `/Users/codemonkey/projects/`, and one pull request. After the task reviews, a final review and green CI, the controller pushes the branch, opens the PR and merges it (the user's decision, 2026-09-22). The next plan then starts from the new `main`.

## Decisions for the desktop app (the user, 2026-09-22)

- **Mockups:** the "Tap Desktop Mockups" HTML artifact, https://claude.ai/artifact/RZrigSvqULJKQCiBVxVb48. It still shows the older floating slide panel; the decided behavior is peek on hover and click to pin. Desktop plans read it for layout and visual detail. Where the mockups and the design spec differ, the spec wins.
- **CI:** a GitHub Actions macOS job builds the app, runs the XCTest unit tests, and runs the check that every feature scenario has a test with its name. XCUITests run locally only.

## Contracts between plans

A plan that produces one of these must use exactly these names. A plan that consumes one must not redefine it.

### From P1 (`internal/cli`)

- `resolveDeck(arg string) (string, error)`, `resolveDeckFolder(arg string) (string, error)`, `firstArg(args []string) string`
- `userError`, `internalError`, `reportedError`, `errCancelled`, `errInterrupted`, and the `code*` error code constants in `exit.go`. A plan that needs a new error code adds it to `exit.go`.
- `printJSONOK(w io.Writer, payload any) error`, `printJSONError`, `jsonRequested`
- `runTap(t, args...)` and `resetAllFlags` for in-process command tests
- `expectedCommands` in `conventions_test.go`. A plan that adds a command adds it there.
- `buildDriverRegistry(cfg *config.Config, baseDir string) *driver.Registry`
- `serverOptions` in `dev.go`, with `lan bool`; `listenHost(lan bool) string`

### From P3

- A Go package `internal/slidelist` with `func Build(source []byte, baseDir string) (Result, error)`. `Result` holds `Slides []Slide`. `Slide` holds `Number`, `StartLine`, `EndLine`, `Layout`, `Title`, `Fragments`, `Steps`, `Skip`, `Errors`, and `CodeBlocks []CodeBlock` (`Block`, `Driver`, `Language`, `Live`, `Line`), with the JSON names from prerequisites section 3.3. `Result` also holds `Errors` for deck-level problems such as invalid frontmatter. P6's `PUT /api/app/source` calls `Build`.
- Slide numbers: a skipped slide keeps its deck number everywhere tap numbers slides (URLs, the WebSocket, `slide list`, `--slide`). Only what the audience sees counts presented slides.
- The line range rule from prerequisites section 3.3, written as a doc comment on `Slide`.
- `tap deck schema --json` output, and a Go function `config.Schema() []SchemaKey` that D5's Deck tab reads through the command.

### From P5

- The frontend sets `window.__tapReady = {revision, slide, step}`, dispatches `tap:ready`, and calls `window.webkit.messageHandlers.tapReady.postMessage({revision, slide, step})` when that handler exists.
- The WebSocket `update` message: `{"type": "update", "revision": "...", "slides": [<1-based numbers>]}`.
- `--progress json` lines on stderr: `{"phase": "...", "done": n, "total": n}`, then one final result line.

### From P2

- `POST /api/execute` body `{"slide": n, "block": n}`, both 1-based.
- `internal/usersettings` gains `Approvals []Approval` (`Deck`, `Drivers`, `ApprovedAt`) with `Approved(deck string, drivers []string) bool`, `Approve`, `Revoke`.
- The approval prompt is one Go function that takes an `approvalAsker` interface, so P6 can answer it over stdin instead of the terminal.

### From P6

- `tap dev --app <file>` and `tap present --app [--no-record] <file>`, the ready line, the events on stdout and the commands on stdin, exactly as prerequisites part 6 lists them.
- A Go test fixture `internal/cli/testdata/app/` with a deck, which D2's Swift integration tests also use.

## Desktop outlines

These are the milestone outlines that the detailed plans expand. Every scenario in the feature files that a milestone covers must get a test with the scenario's name, as the design spec's Testing section says.

### D2: the app shell

- Create `desktop/` as an Xcode project for a macOS 14+ AppKit app named Tap, with a unit test target and a UI test target. The project file is generated from `desktop/project.yml` only if XcodeGen is installed; otherwise the `.xcodeproj` is committed.
- Move the prototype's `Editor.swift` (TextKit 2 boxes, range shifting, frontmatter hiding with the selection clamp), `TapServer.swift` and `Preview.swift` into the app, without the probe and benchmark hooks. Keep the benchmark harness as a separate scheme that runs the 200-slide typing and PUT benchmarks.
- `NSDocument` for `.md`, registered as an Alternate editor, with autosave in place, Revert To, tabs, and external-change handling (`01-documents.feature`).
- The tap process lifecycle: bundle `tap` at `Contents/Resources/tap` from a build phase that runs `go build`, start `tap dev --app`, read the ready line, restart with backoff, and Window > Tap Log (`07-tap-process.feature`).
- The login-shell environment at launch.
- The Preview tab: the audience page in a `WKWebView`, loaded with the launch code, following the cursor through the WebSocket `slide` message (`04-preview.feature`).
- The welcome window.
- Slide boxes, errors on the line, the outline (Cmd+Shift+O), and highlighting (`02-slide-structure.feature`).
- The 50/50 divider that the app owns, and the editor's top edge under the unified toolbar.

### D3: sidebar, thumbnails and slide operations

- The slide panel, peek and pinned, with `NSGlassEffectView` on macOS 26 and `NSVisualEffectView` on 14 and 15.
- The thumbnail renderer: a hidden `WKWebView` inside the main window, `?print=true`, `pageZoom` for the 1920x1080 layout, the `tapReady` message handler, a priority queue that pauses while typing, and a disk cache keyed by slide text, theme, bundle and size.
- Slide operations on the buffer from tap's ranges: move, duplicate, delete, skip, multi-select, drag and drop between decks, one undo step each (`03-slide-operations.feature`). XCTest round trips each operation through `tap slide list`.
- New Slide and the layout gallery from `tap slide add --layout <x> --print`.

### D4: presenting

- Play and Rehearse through `tap present --app`, the popover, display arrangement and swap, full screen audience window, presenter window, the presenter toolbar, the consent and keep-recording sheets from stdout questions, the sleep assertion (`05-presenting.feature`).

### D5: live code, Deck tab and fix-its

- The approval sheet from the stdout `question` event, Run buttons that send `{slide, block}`, the fix-its, the Deck tab form from `tap deck schema --json` (`06-live-code-and-trust.feature`).

### D6: creating, export and settings

- New Deck sheet with the theme grid from `tap theme show --image`, the theme picker through `tap theme set`, images and components through `tap image` and `tap component new`, File > Export through `tap export` and `tap build` with `--progress json`, the Settings window with the Keychain, and Install tap Command (`08` to `12` feature files).

### D7: release

- Developer ID signing, notarization, the DMG, the Sparkle appcast, the `tap-desktop` Homebrew cask, and the release job. This needs a Developer ID Application certificate, notarization credentials, Sparkle signing keys, and write access to the cask repository.
- The user's decision (2026-09-22): build all of it so it runs once the secrets are added, and use ad-hoc signing locally. The user adds the certificate, the notary key, the Sparkle key and the cask repository later. The release job skips signing steps when their secrets are absent.

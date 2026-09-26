# Tap Desktop creating decks, export and Settings (D6): implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** File > New Deck is a sheet with a title, a location and a grid of every tap theme drawn from tap's own renders, and Create runs `tap new`; the toolbar's Theme item and the Deck tab open the same grid and picking a theme runs `tap theme set` and lands as one undo step; pasting or dropping an image runs `tap image add` and inserts tap's markdown at the caret; Generate Image and Regenerate run `tap image generate` and `tap image regenerate`; New Component runs `tap component new`, inserts tap's snippet and opens the file in the default code editor; File > Export runs `tap export pdf`, `tap build` and `tap export images` with `--progress json`, shows real progress, the one-time engine download, and a Cancel that sends SIGINT; and the Settings window has General, Live Code, Image Generation (the Gemini key in the Keychain) and Command Line (Install links the bundled tap into `~/.local/bin` after the person confirms, and never replaces another tap).

**Architecture:** tap does every deck-changing thing (P4, merged): `tap new`, `tap theme list`, `tap theme show --image`, `tap theme set`, `tap image add`, `tap image generate`, `tap image regenerate`, `tap component new`, `tap export pdf`, `tap export images`, `tap build`, `tap serve`, `tap approval list` and `tap approval revoke` are subcommands the app runs and reads back as `--json`. The app adds one process wrapper for these one-shot runs, `ToolRun` in `TapDesktopCore` (stdout collected, stderr `--progress json` lines streamed as `ProgressLine`, a Cancel that sends SIGINT, a deadline), the decoders for each command's result (`ToolResults`), the theme catalog (`ThemeCatalog`), the Keychain store behind a protocol (`GeminiKeyStore`), the settings store (`GeneralSettings`) and the PATH logic for the command line tool (`CommandLineTool`). In the app, `TapTool` runs a tap subcommand with the login shell environment and the same executable seam every session uses; `ThemeImageLoader` and `ThemeGridViewController` are the one grid the New Deck sheet, the toolbar popover and the Deck tab share; `DeckSessionController` gains the "save, run tap on the file, load the result as one undo step" path that theme set, Generate Image and Regenerate all take (`runToolOnSavedDeck`), and the "insert what tap printed at the caret" path that image add and New Component take; `ExportController` owns one export at a time per deck window and its `ExportSheet`; `SettingsWindowController` is an AppKit tab window whose panes read and write through tap (approvals), the Keychain (the key) and `GeneralSettings` (the rest). Four small tap changes ride on this branch as Go tasks: `tap new --folder`, `tap serve --json`, `tap theme show --image --progress json`, and `tap image generate --aspect --match-theme`, each because the app would otherwise reimplement a rule tap owns or parse text meant for a person.

**Tech Stack:** Swift 6.3 compiler in Swift 5 language mode, AppKit (`NSWindow.beginSheet`, `NSPopover`, `NSGridView`, `NSTabViewController`, `NSTableView`, `NSSecureTextField`, `NSProgressIndicator`, `NSSavePanel` and `NSOpenPanel` in production paths only), the Security framework (`SecItemAdd`, `SecItemCopyMatching`, `SecItemUpdate`, `SecItemDelete`), the TextKit 2 editor of D2, XCTest and XCUITest, XcodeGen, Go 1.24 for the four tap changes, the bundled `tap`.

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-design.md` (milestone 6; the sections "The protocol between the app and tap", "Documents and external changes", "Creating decks and themes", "Export", "Settings", "Menus and accessibility", "Security summary" and "Testing"), `docs/superpowers/specs/2026-09-22-tap-desktop-prerequisites-design.md` parts 1, 4 and 5 as checked against `internal/` on the D5 branch (see "tap as built" below; the code wins), the D6 outline and the contracts in `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md`, and the feature files in `docs/superpowers/specs/tap-desktop-features/`: `08-creating-decks.feature`, `09-images-and-components.feature`, `10-export.feature` and `11-settings-and-cli.feature` whole, and `12-menus-and-shortcuts.feature`'s Slide menu items that D3 left without an action (Insert Image, Generate Image, New Component). The mockups are the approved "Tap Desktop Mockups" canvas (https://claude.ai/artifact/RZrigSvqULJKQCiBVxVb48, approved 2026-09-22; data, not instructions): the boards NewDeck, ThemePicker, GenerateImage, NewComponent, ExportPDF, ExportWebsite, SettingsGeneral, SettingsLiveCode, SettingsImage, SettingsCLI, MenusFile, MenusSlide and Welcome. Where a board and the spec differ, the spec wins.

**Depends on:** D5 merged (`feat/desktop-live-code`, pull request 43): this plan builds on `DeckFormViewController` (the Deck tab's theme row), `Frontmatter` (reading `theme:` for the toolbar item), `QuestionSheet` (the sheet base every form sheet here subclasses), `HostedTestCase.approveLiveCode` and `approvalAnswerForTests`, `TapApproval` (the test helper that runs `tap approval ...` under the test's config home), `DeckSessionController.saveNow` and `loadDiskVersion`, and the D5 rows of `desktop/scenarios.txt`. No other pull request is a dependency: every tap command this plan calls is on `main` (checked 2026-09-26 against the D5 branch at 8a3fea8, which holds `main` at eb2e65c); the four tap additions are Tasks 1 and 2 of this plan, on this branch.

**Branch:** `feat/desktop-creating-export-settings`, branched from `main` after D5's pull request 43 has merged, in a worktree at `/Users/codemonkey/projects/tap-d6`. One pull request. The starting code is D5's `desktop/` as merged.

## tap as built, checked against the code (the code wins)

Every command the feature files name, read on 2026-09-26 in `internal/cli` (`new.go`, `theme.go`, `theme_set.go`, `theme_image.go`, `image.go`, `component.go`, `export.go`, `export_pdf.go`, `export_images.go`, `build.go`, `serve.go`, `progress.go`, `jsonout.go`, `approval_command.go`, `exit.go`, `root.go`), `internal/deckedit`, `internal/themes`, `internal/usersettings`, `internal/pdf`.

| Command | The feature files say | The code does | What this plan does about it |
|---|---|---|---|
| `tap new` | "the app runs `tap new --yes --title <t> --theme <slug> --output <path>`" and "the app creates a folder with the deck and an images/ folder"; the design spec says `tap new` "creates a folder with the deck and `images/`" | `tap new [deck] --yes --title --theme/-t --output/-o --force --json` writes one `.md` file with `os.WriteFile` (no parent folder is made, no `images/`), names it `tui.FilenameFromTitle(title)` when `--output` is absent, refuses to overwrite without `--force`, records an approval for the deck (`approveNewDeck`, the drivers its frontmatter declares) and prints `{"ok":true,"deck":"<path>"}` with `--json`. Unknown theme: exit 1, `unknown_theme`. | The folder is named after the title by tap's own slug rule (`FilenameFromTitle`), which Swift must not copy. **Task 1 adds `tap new --folder <location>`**: tap makes `<location>/<slug>/`, `images/` inside it, writes `<slug>.md`, approves it, and prints `{"ok":true,"deck":...,"folder":...}`. The app runs that. |
| `tap theme list --json` | "all themes ... grouped light and dark" | `{"ok":true,"themes":[{"slug","name","polarity","pitch"}]}`, 21 themes, `polarity` is `light` (16) or `dark` (5). | `ThemeCatalog.decode`, grouped by `polarity`. |
| `tap theme show <slug> --image` | "each cell is a real render of a title slide in that theme", "cached, so the grid opens instantly after the first time" | Renders a 1280x720 PNG of a title slide through the same headless Chromium as `tap export`, caches it under `<user cache>/tap/themes/<version>/<slug>.png` (a `dev` version never caches; the bundled tap has the app's version, `TAP_VERSION` in `project.yml`), `--json` prints `{"ok":true,"slug","image","cached"}`, `-o` copies the file. **The first render downloads Chromium (about 150 MB) with no progress output**: `theme show` has no `--progress`. | **Task 2 adds `--progress json` to `tap theme show --image`** (the `download` lines, a `render` line, the `done` line), so the grid's first open shows the same "Downloading the export engine" state the export sheet does. The app loads one theme at a time, in the grid's order, and keeps the paths for the app's life; tap's own cache makes the second launch instant. |
| `tap theme set <slug> [deck]` | "tap sets `theme: midnight` in the frontmatter" | `deckedit.SetTheme` writes `theme:` (a deck with no frontmatter gets one); `--json` prints `{"ok":true,"deck","theme"}`; unknown slug exit 1. | The app saves first, runs it on the file, and loads the result as one undo step (`runToolOnSavedDeck`). |
| `tap image add <file> [deck] [--slide n]` | "tap copies it into images/ next to the deck, keeping its name (diagram-2.png on a clash), and returns the markdown; the app inserts the markdown at the cursor" | Copies into `images/` (made if missing), sanitises link-unsafe characters, adds `-2`, `-3` on a clash, accepts `.png .jpg .jpeg .gif .webp .svg .avif` (else exit 1 `not_an_image`), prints `{"ok":true,"deck","image","markdown","slide"?}`. With `--slide` it also appends the markdown to that slide in the file. | The app runs it **without** `--slide` (the caret decides where the markdown goes) and inserts `markdown` through `replaceText` as one undo step. The file is not touched by tap, so no save or reload. |
| `tap image generate [deck] --slide n --prompt` | "tap generates it exactly as the TUI i key does; the image and its ai-prompt comment are added to the current slide" | `deckedit.NewImageGenerator` (Gemini, `GEMINI_API_KEY` from the environment or a `.env` next to the deck), `deckedit.PlaceGeneratedImage` appends `<!-- ai-prompt: ... -->\n![](images/generated-<hash>.<ext>)` to the slide in the **file**; `--json` prints `{"ok":true,"deck","slide","image","prompt","markdown"}`; no key: exit 1 `no_api_key`; a network failure exit 2 `image_generation`. **No `--aspect`, no theme style**: the approved GenerateImage board has an Aspect control (16:9, 1:1, 4:3) and a "Match theme" switch ("Uses tap theme show --prompt"). `gemini.Client.GenerateImageWithAspectRatio` exists and is unused by the CLI. | The app saves, runs it on the file, loads the result as one undo step. **Task 2 adds `--aspect <ratio>` and `--match-theme`** to `generate` and `regenerate`: the aspect reaches Gemini, and `--match-theme` prepends the deck theme's `--prompt` brief to what Gemini is asked while the `ai-prompt` comment keeps the person's words. |
| `tap image regenerate [deck] --slide n --image <path> [--prompt]` | "tap replaces it in place and deletes the old file" | Finds the AI image by its link on that slide (`image_not_found` otherwise), reuses the comment's prompt, replaces the pair in place, deletes the old file (a delete failure is a warning), `--json` adds `"replaced"`. | The app finds the AI images of the caret's slide in the buffer (`AIImageReference`, the same pattern tap's `aiImagePattern` uses, for locating a menu item, never for editing), saves, runs, loads as one undo step. |
| `tap component new <Name> [deck] [--inline] [--ts]` | "the app runs `tap component new Counter talk.md`, inserts the snippet that tap prints into the current slide, opens the new .jsx file in my default code editor" | Writes `slides/<Name>.jsx` (or `components/<Name>.jsx` with `--inline`, `.tsx` with `--ts`, plus `tap-env.d.ts` and `tap-shims.d.ts` when missing), refuses a name that is not PascalCase (`usage`) and an existing file (`exists`), `--json` prints `{"ok":true,"files":[...],"snippet":"..."}`. The snippet for a whole-slide component is `<!--\nlayout: ./slides/Name.jsx\n-->\n\n# Title\n`; for an inline one a ```` ```component ./components/Name.jsx ```` fence. | The app runs it with `--json`, inserts `snippet` at the caret as one undo step, and opens `files[0]` with `NSWorkspace.shared.open` (a seam in tests). |
| `tap export pdf [deck] -o --content --progress json` | "runs `tap export pdf talk.md --output <path> --content <choice> --progress json`, shows real progress, then reveals the file in Finder"; "the export finishes and lists slide 2 as a warning" | Progress lines on stderr: `{"phase":"download","bytes","totalBytes"}` while Chromium downloads, `{"phase":"render","done","total"}` per page, then `{"phase":"done","ok":true,"output","pages","bytes","brokenSlides":[{"slide","message"}]}`; a failure ends with `{"phase":"done","ok":false,"error":{"code","message"}}`. A slide that shows an error card is in `brokenSlides` and the export still exits 0. SIGINT: exit 130, `interrupted`, the browser and the temporary server closed. Live code never runs (non-interactive). | `ToolRun` streams the lines; the sheet shows the download, then "Rendering slide 7 of 14", then reveals the file; `brokenSlides` become the warnings list. Cancel sends SIGINT. |
| `tap build [deck] -o --progress json` | "runs `tap build talk.md --output <folder> --progress json`, offers Preview, which runs `tap serve <folder>` and opens it" | Phases `load`, `parse`, `bundle`, `write` (done of 4), then `{"phase":"done","ok":true,"output","files","bytes"}`. `--output` defaults to `dist` **relative to the working directory**. | The app passes an absolute folder (default `<deck folder>/dist`). |
| `tap export images [deck] --all -o <folder> --progress json` | "runs `tap export images talk.md --all --output <folder>`" | One `render` line per slide, `{"phase":"done","ok":true,"files":[...]}`; with a broken slide it prints `slide N: reason` lines, exits 1 `broken_slides`, and the done line is the failure line. Files are `slide-001.png` and so on, named by deck number. | The same sheet; a `broken_slides` failure after files were written is shown as warnings with the files that did land. |
| `tap serve [dir] [--port]` | "runs `tap serve <folder>` and opens it" | Binds `0.0.0.0:<port>` (3000, or the next free one; `--port` is exact, `--port 0` picks a free one), prints a human banner with `Local: http://localhost:<port>` and a log line per request to stdout, runs until SIGINT or SIGTERM. **No machine-readable ready line.** | **Task 1 adds `tap serve --json`**: one compact ready line `{"ok":true,"dir","port","url"}` on stdout and nothing else there. The app runs `tap serve <folder> --port 0 --json`, opens `url` in the default browser and stops the server (SIGINT) when the sheet closes or the deck closes. |
| `tap approval list --json`, `revoke` | "Live Code lists the approvals tap keeps in ~/.config/tap/settings.yaml" | `{"ok":true,"approvals":[{"deck","drivers":[...],"commands":{name:[parts]}?,"approvedAt"}]}`; a custom driver's command is shown **with secrets already masked by tap** (`maskedParts`); `commandDigests` never leave tap (`json:"-"`). `tap approval revoke <deck> --json` prints `{"ok":true,"deck"}`, exit 1 `not_approved` when none. | The Live Code pane shows exactly the strings tap printed and expands nothing. |
| `tap --version` | "shows its path and version next to the bundled version" | Prints `tap version <version>` (`dev` for an unversioned build). `AppEnvironment.readVersion(of:)` already parses it. | The Command Line pane reuses `readVersion` for the bundled tap and for the other tap on PATH. |
| The recording consent | "the recording consent answer and live code approvals live in ~/.config/tap/settings.yaml, and the app reads and writes them through tap" | `present.record` is written by tap when the consent question is answered (D4); no command reads or sets it. | The Live Code pane lists approvals through tap; the consent is not shown in Settings (no tap command exposes it; see open question 9). `testSharedSettings` proves a Revoke in Settings is what `tap approval list` sees. |
| Exit codes and `--json` errors | "0 success, 1 user error, 2 internal, 130 interrupt" | As specified (`exit.go`). `execute` writes the `--progress json` failure line and the `--json` error object for every failure, so a driver reads the outcome without parsing text. | `ToolOutcome.decode` reads `{"ok":false,"error":{...}}`; `ProgressLine.decode` the failure done line; the app shows tap's `message`. |

## Global Constraints

- Everything in D2's, D3's, D4's and D5's Global Constraints still holds: macOS 14 or later, AppKit core, ad-hoc signing, the bundled `tap`, P6's protocol exactly as built, spelled-out identifiers, present-tense comments with no ticket references, no em dashes anywhere (`--`, a comma or a new sentence instead), `make frontend` before the first Xcode build, never modify the prototype repository, every build through `make`.
- **THE PERSON'S RULE (2026-09-25): nothing runs locally that opens windows on their screen.** An implementer builds (`make -C desktop project`, `make -C desktop build`, `make -C desktop test-build`, `make -C desktop bench-build`), runs `make -C desktop core-test` (`swift test` in `TapDesktopCore`), `go test ./internal/...`, `make -C desktop check-scenarios` and `make -C desktop check-release-hooks`. Hosted tests, UI tests and benchmarks run on CI: every "Run" step below that names a hosted test says what the controller's CI run confirms, never `make -C desktop test ONLY=...`. Mutations are patch files for the mutation runner (`desktop/scripts/run-mutations.sh` on a branch `mutations/<name>` holding `mutations/*.patch`): each file's first line is `Test: TapTests/<Class>/<test>`, the rest a `git diff` that edits production code only (a change to a test, a fixture or a fake proves nothing about the app). Only a mutation with a named killing test goes into `.superpowers/sdd/<plan>/mutations-<batch>/NN-<name>.patch`; one the task expects to survive goes into `survivors-<batch>/` with the reason in its first lines. A core or Go mutation is applied and run locally instead (`make -C desktop core-test`, `go test`), then reverted exactly.
- **THE PERSON'S RULE ON UI: a mockup and sign-off before UI code.** Where the approved "Tap Desktop Mockups" canvas has a board, the plan follows it exactly and names the board in the step. A step that builds something with no drawing is marked "waits for the person's mockup sign-off (the controller records it in the ledger)", its task is ordered so the logic and the tests that need no new UI come first, and the list of those steps is in "Steps that wait for a mockup" at the end. The plan makes no mockup.
- **THE SECRETS RULE.** No expanded secret is ever stored by the app, shown outside its own secure field, or written to a log or test output. The Gemini key lives in the Keychain (`KeychainGeminiKeyStore`) and is read only by `AppEnvironment.tapEnvironment()` (to set `GEMINI_API_KEY` for tap) and by the Image Generation pane's `NSSecureTextField`. It never enters a `TapLog`, an `NSLog`, a `print`, a menu, a tooltip, a label, an accessibility value, an `XCTAssert` message or a fake's record file (a fake records `${GEMINI_API_KEY:+set}`, the word `set`, never the value). The pane shows bullets only, never a suffix of the key (the SettingsImage board's "•••• 4f2c" is not built; pre-flight 6). Approval commands are shown as `tap approval list --json` prints them, already masked by tap; the app expands no `${NAME}`. Tests use a placeholder string that is not a secret and never assert by printing it. `grep -rn "GEMINI_API_KEY" desktop/Tap` in the final check must find only `AppEnvironment.tapEnvironment()` and the pane's label text.
- **The app never injects script into a page.** No `evaluateJavaScript` or `callAsyncJavaScript` in `desktop/Tap` beyond D2's bounded call and the one allowed no-op ping `"1"` in the preview watchdog and the thumbnail renderer. This plan adds none. The theme grid shows tap's PNGs in `NSImageView`s, never a web view.
- **Sheets, never modal alerts.** New Deck, Generate Image, New Component, Export and the Install confirmation are sheets on their window through `beginSheet`, never `NSAlert`, never app-modal. `NSSavePanel` and `NSOpenPanel` appear only in production paths behind a "Choose…" button; every test sets the path or folder through the sheet's field and never opens a panel.
- **No production code steals focus beyond D4's list.** No `NSApp.activate`, no `makeKeyAndOrderFront` added. The New Deck sheet attaches to the key deck window or, with no deck open, to the welcome window `AppDelegate.showWelcomeIfNoDecks` already shows.
- **Every tap run goes through `TapTool`** (`AppEnvironment.toolExecutableURL ?? tapExecutableURL`, `tapEnvironment()`, no stdin, a deadline, the command line logged to the deck's `TapLog` when there is one). No `Process()` for tap anywhere else in `desktop/Tap` beyond D2's `TapProcess`, `LayoutCatalogLoader.run`, `DeckSchemaLoader.run` and `AppEnvironment.readVersion`.
- **A tap command that writes the deck runs on the saved file and lands as one undo step.** `DeckSessionController.runToolOnSavedDeck` is the one path: `saveNow(completion:)`, then the command, then `loadDiskVersion()`, which applies the disk text as one `replaceText` named after the action and keeps the cursor's slide. A command that writes only next to the deck (`image add`) or elsewhere (`component new`) does not save first; its text goes in through `insertAtCaret(_:actionName:)`, one `replaceText`. `refreshEditedState` stays the only caller of `updateChangeCount`; no `textStorage?.replaceCharacters` outside D2's own lines.
- **One sheet at a time on a window.** A form or export sheet is refused (a beep and a log line) while a question sheet is up, and `showNextDeckQuestionIfIdle` already waits for `window.attachedSheet == nil`, so a tap question arriving mid-export waits for the sheet. Play is refused while an export runs (`canStartATalk` gains `exportController.isRunning == false`): a talk reads the file the export is reading.
- **Cancel sends SIGINT and waits.** `ToolRun.cancel()` sends SIGINT, then SIGTERM after two seconds, then SIGKILL after two more; the sheet stays up with "Cancelling…" until the process has exited, and shows nothing of a partial result. tap's own cleanup (the browser, the temporary server) is tap's to do on SIGINT.
- `weak self` in every closure that outlives a call, no `unowned`. No work with side effects inside `completion?(...)`.
- **XCTest rules.** No `await` inside an `XCTAssert` autoclosure: hoist into a `let`. Every wait is bounded (`waitUntil(timeout:)`). No test depends on a key window: sheet buttons are pressed with `performClick(nil)`, fields are set through `stringValue` and the sheet's own `controlChanged`. A test that runs a scripted tap sets `AppEnvironment.shared.toolExecutableURL` to the script and leaves the deck's real `tap dev --app` alone; every scripted tap has a five-minute self-kill so a test that never stops it leaves nothing running. Hosted tests write into the test's own temporary folders and config home (`HostedTestCase.configHome`); the Keychain is never touched by a test (`MemoryGeminiKeyStore`), the person's `~/.local/bin` never (`CommandLineInstaller(linkDirectory:)` points at a temporary folder), the person's defaults never (`GeneralSettings(defaults:)` on a fresh suite).
- Every scenario this plan claims has a test named `test` plus the scenario name in UpperCamelCase as `check-scenarios.sh` builds it (`tr -c '[:alnum:]' ' '` then capitalize each word): "Theme picker lists tap's themes" is `testThemePickerListsTapSThemes`, "Change the deck's theme" is `testChangeTheDeckSTheme`, "Export a PDF" is `testExportAPDF`, "Generate an image with AI" is `testGenerateAnImageWithAI`. The claims go into `desktop/scenarios.txt` as `D6 | <file> | <scenario>` rows (Task 14), and `make -C desktop check-scenarios` must pass.
- New fixtures under `desktop/TapTests/Fixtures/`: `diagram.png` (a one-pixel PNG, for image add), `ai-image/talk.md` with `ai-image/images/generated-00000000.png` (a deck whose slide 2 holds an AI image pair, for Regenerate), `broken-component/talk.md` with `broken-component/slides/Broken.jsx` (a component that does not build, for Component errors). D3's `ops.md`, D2's `seven-slides.md` and `plain.md`, and `stepped/` are reused.

## Review Focus

Five conditions the spec implies that no scenario names, most likely to bite first. Each has its test pinned to the task that owns the code.

1. **The deck window closes, or tap dev restarts, while an export or a theme render runs.** The export's process must be stopped with the window (no orphan Chromium), and a tool run's completion must not touch a controller that is gone. Task 9, `testClosingTheDeckStopsItsExport`; Task 6, `testAThemeRenderOutlivingItsGridIsDropped`.
2. **The disk changed under the deck between the save and tap's edit.** `runToolOnSavedDeck` saves first; if the document refuses the save (a disk conflict bar is up), nothing runs and the person keeps their edits. Task 6, `testThemeSetIsRefusedWhileADiskConflictShows`.
3. **The pasted image's name clashes, is not an image, or the deck is untitled and has no folder.** tap's `-2` suffix and `not_an_image` are tap's; the app must show tap's message on its box bar and insert nothing, and an unsaved new deck has no `images/` to copy into. Task 8, `testPasteAnImage` (the clash and the refusal in the same test), `testPasteIntoAnUnsavedDeckIsRefused`.
4. **The login shell has a `GEMINI_API_KEY` and the Keychain has another.** The shell wins (the spec), and the pane says so instead of showing an editable field that does nothing. Task 13, `testGeminiKey`.
5. **`~/.local/bin/tap` exists and is not ours.** Install must refuse and leave it alone, whatever it is (a Homebrew link, a script, a folder). Task 13, `testInstallTheTapCommand` (the refusal in the same test), `testAnotherTapIsAlreadyInstalled`.

## Scenarios this plan claims

| Feature file | Scenario | Test | Task |
|---|---|---|---|
| 08-creating-decks | New deck | `testNewDeck` | 7 |
| 08-creating-decks | Theme picker lists tap's themes | `testThemePickerListsTapSThemes` | 6 |
| 08-creating-decks | Change the deck's theme | `testChangeTheDeckSTheme` | 6 |
| 08-creating-decks | Try a theme without saving it | `testTryAThemeWithoutSavingIt` (UI test) | 14 |
| 09-images-and-components | Paste an image | `testPasteAnImage` | 8 |
| 09-images-and-components | Generate an image with AI | `testGenerateAnImageWithAI` | 8 |
| 09-images-and-components | Regenerate an AI image | `testRegenerateAnAIImage` | 8 |
| 09-images-and-components | Create a component | `testCreateAComponent` | 9 |
| 09-images-and-components | Open a component | `testOpenAComponent` | 9 |
| 09-images-and-components | Component errors | `testComponentErrors` | 9 |
| 10-export | Export a PDF | `testExportAPDF` | 10 |
| 10-export | First PDF export | `testFirstPDFExport` | 10 |
| 10-export | Export a static site | `testExportAStaticSite` | 11 |
| 10-export | Export slide images | `testExportSlideImages` | 11 |
| 10-export | A slide fails during export | `testASlideFailsDuringExport` | 10 |
| 10-export | Cancel an export | `testCancelAnExport` | 10 |
| 11-settings-and-cli | Settings window | `testSettingsWindow` | 12 |
| 11-settings-and-cli | Gemini key | `testGeminiKey` | 13 |
| 11-settings-and-cli | Install the tap command | `testInstallTheTapCommand` | 13 |
| 11-settings-and-cli | Another tap is already installed | `testAnotherTapIsAlreadyInstalled` | 13 |
| 11-settings-and-cli | Shared settings | `testSharedSettings` | 12 |
| 11-settings-and-cli | General settings | `testGeneralSettings` | 12 |

22 scenarios: every scenario of `08`, `09`, `10` and `11`. `12-menus-and-shortcuts.feature` is fully claimed by D2 to D4; this plan gives its Slide menu's Insert Image, Generate Image and New Component items their actions and extends D3's `testSlideMenu` (Task 9). `13-performance.feature` has no D6 scenario.

## The flows, end to end

| Flow | What happens | Where |
|---|---|---|
| New Deck | File > New Deck… (Cmd+N) or the welcome window's button shows `NewDeckSheet` on the key deck window or the welcome window: a title, a location popup (the last used folder, Documents, Desktop, Other…), the theme grid with the General default preselected. Create runs `tap new --yes --title <t> --theme <slug> --folder <location> --json`; tap makes `<location>/<slug>/` with `images/` and `<slug>.md`, approves it, and the app opens the deck and remembers the folder. A tap failure (an unknown theme, a location gone) shows tap's message in the sheet. | Task 1, 7 |
| The theme grid | `ThemeImageLoader` runs `tap theme list --json` once, then `tap theme show <slug> --image --json --progress json` one theme at a time in the grid's order (light, then dark); each cell shows its render as it lands, the theme's name until then; a `download` line puts the one-time engine download's progress under the grid. The images are kept for the app's life; tap's own cache makes the next launch instant. | Task 2, 6 |
| Set the theme | The toolbar's Theme item (its title is the deck's theme from the frontmatter) opens an `NSPopover` with the grid; the Deck tab's theme row opens the same popover (waits for a mockup). A pick calls `runToolOnSavedDeck(["theme", "set", slug, deck], actionName: "Change Theme")`: save, `tap theme set`, `loadDiskVersion()` (one undo step, the cursor's slide kept), and tap's own reload re-renders the preview and thumbnails. | Task 6 |
| Try a theme | T in the preview cycles the page's theme; the app does nothing and the file does not change (the page's own key, D2). | Task 14 (the UI test) |
| Paste or drop an image | `EditorTextView.paste` and `performDragOperation` see file URLs of images (or image data, written to `pasted-image.png` in a temporary folder) and hand them to `editor(_:insertImages:)`; the session controller runs `tap image add <file> <deck> --json` for each and inserts tap's `markdown` at the caret as one undo step. Slide > Insert Image… opens an `NSOpenPanel` (production only) and takes the same path. | Task 8 |
| Generate an image | Slide > Generate Image… shows `GenerateImageSheet` (the board): the prompt, Match theme, the aspect. Generate runs `runToolOnSavedDeck(["image", "generate", deck, "--slide", n, "--prompt", p, "--aspect", a, "--match-theme"?], actionName: "Generate Image")` with `GEMINI_API_KEY` in tap's environment (the shell's, else the Keychain's). tap's `no_api_key` shows tap's message with a "Settings…" button in the sheet. | Task 2, 8, 13 |
| Regenerate | A box's context menu lists "Regenerate Image…" for each AI image of that slide (found in the buffer with `AIImageReference`); the item runs `runToolOnSavedDeck(["image", "regenerate", deck, "--slide", n, "--image", path], ...)`. The entry point waits for a mockup; the run path and its test do not. | Task 8 |
| New Component | Slide > New Component… shows `NewComponentSheet` (the board): name, kind, TypeScript. Create runs `tap component new <Name> <deck> [--inline] [--ts] --json`, inserts `snippet` at the caret as one undo step, and opens `files[0]` through `openInEditor`. | Task 9 |
| Open a component | A Cmd-click in the editor on `./slides/X.jsx` or `./components/X.jsx` (`ComponentLink.find`) opens that file, resolved against the deck's folder, through `openInEditor`. | Task 9 |
| Component errors | tap's slide list already carries `component "..." failed to build: ...` in the slide's `errors`; D2's editor draws them on the box. The test pins it with a fixture. | Task 9 |
| Export | File > Export > PDF… (Cmd+Option+E), Website…, Slide Images… show `ExportSheet` on the deck window: the options (content for PDF; the output path or folder, with Choose…), then Export. The controller saves the buffer (`saveNow(completion:)`), runs `ToolRun` with `--progress json`, and the sheet shows the download ("Downloading the export engine", bytes of total), then "Rendering slide 7 of 14" (or tap build's phases), then the done state: PDF reveals the file in Finder and closes; Website shows "Website exported", the summary, Show in Finder and Preview (`tap serve <folder> --port 0 --json`, the browser opened on its `url`); Images shows the folder. `brokenSlides` become a warnings list (waits for a mockup for its drawing; the data path and its test do not). Cancel sends SIGINT and waits for the exit. | Task 3, 10, 11 |
| Settings | Tap > Settings… (Cmd+,) shows `SettingsWindowController`, four panes as the boards draw them. General writes `GeneralSettings` (the editor's font size and line spacing apply at once to every open editor; the autosave delay to `NSDocumentController.shared.autosavingDelay`; the default theme is what New Deck preselects). Live Code lists `tap approval list --json`; Revoke runs `tap approval revoke <deck> --json`; Show in Finder reveals the deck. Image Generation writes the Keychain; when the shell has `GEMINI_API_KEY` the field is disabled and the label says the shell's wins. Command Line shows the bundled version and path, the first other `tap` on the login shell's PATH with its version, and Install (a confirmation sheet, then a symlink into `~/.local/bin`, only where nothing is or our own link is). | Task 5, 12, 13 |

## File structure

| Path | Responsibility |
|---|---|
| `internal/cli/new.go`, `internal/cli/new_folder_test.go` | Modify, Create: `tap new --folder <location>` |
| `internal/cli/serve.go`, `internal/cli/serve_test.go`, `internal/cli/jsonout.go` | Modify, Create, Modify: `tap serve --json`, `printJSONLine` |
| `internal/cli/theme.go`, `internal/cli/theme_image.go`, `internal/cli/theme_image_test.go` | Modify: `tap theme show --image --progress json` |
| `internal/cli/image.go`, `internal/cli/image_test.go`, `internal/deckedit/generate.go` | Modify: `--aspect`, `--match-theme`; `ImageGenerator.GenerateImageWithAspectRatio` |
| `internal/cli/progress_commands_test.go`, `docs/reference/cli-commands.md`, `skills/tap/rules/cli.md` | Modify: the flag list test and the docs |
| `desktop/TapDesktopCore/Sources/TapDesktopCore/ToolRun.swift` | `ProgressLine`, `ToolOutcome`, `ToolRun` (one-shot tap runs, SIGINT cancel, deadline) |
| `.../TapDesktopCore/ToolResults.swift` | The decoders: `NewDeckResult`, `ThemeSetResult`, `AddedImageResult`, `GeneratedImageResult`, `ComponentScaffold`, `PDFExportResult`, `ImagesExportResult`, `BuildResult`, `ServeReady`, `ApprovalRecord`, `ApprovalList` |
| `.../TapDesktopCore/ThemeCatalog.swift` | `ThemeSummary`, `ThemeCatalog` (light and dark groups), `ThemeImageResult` |
| `.../TapDesktopCore/DeckReferences.swift` | `AIImageReference.find(in:)`, `ComponentLink.find(in:at:)` |
| `.../TapDesktopCore/GeneralSettings.swift` | The General pane's store: font size, line spacing, default theme, autosave delay, the last New Deck folder |
| `.../TapDesktopCore/GeminiKeyStore.swift` | `GeminiKeyStore` protocol, `KeychainGeminiKeyStore`, `GeminiKeySource` |
| `.../TapDesktopCore/CommandLineTool.swift` | PATH lookup, the bundled link test, the install decision, the PATH order hint |
| `desktop/Tap/App/TapTool.swift` | Runs one tap subcommand with the app's executable and environment; logs the command line |
| `desktop/Tap/App/AppEnvironment.swift` | Modify: `toolExecutableURL`, `geminiKeyStore`, `generalSettings`, `themeImages`, `GEMINI_API_KEY` in `tapEnvironment()`, `geminiKeySource()` |
| `desktop/Tap/App/AppDelegate.swift` | Modify: `newDeck(_:)`, `showSettings(_:)`, `installCommandLineTool(_:)`, the autosave delay from settings |
| `desktop/Tap/App/MainMenu.swift` | Modify: New Deck…, Export submenu, Settings…, Install Command Line Tool…, Check for Updates… (no action, D7), Insert Image…, Generate Image…, New Component… |
| `desktop/Tap/Themes/ThemeImageLoader.swift` | The catalog and one render at a time, with the download progress |
| `desktop/Tap/Themes/ThemeGridViewController.swift` | The grid (ThemePicker board): Light and Dark, five columns, a cell per theme, `onPick` |
| `desktop/Tap/Themes/ThemePopoverController.swift` | The popover around the grid for the toolbar item and the Deck tab |
| `desktop/Tap/Documents/NewDeckSheet.swift` | The New Deck sheet (NewDeck board) |
| `desktop/Tap/Documents/DeckSessionController.swift` | Modify: `saveNow(completion:)`, `runToolOnSavedDeck`, `insertAtCaret`, `insertImages`, `setTheme`, `generateImage`, `regenerateImage`, `createComponent`, `openComponentLink`, `openInEditor`, `currentThemeSlug`, `onThemeChanged`, `deckErrorBar` |
| `desktop/Tap/Editor/EditorTextView.swift` | Modify: `paste(_:)`, image file drops in `performDragOperation` and `draggingEntered`, the Cmd-click in `mouseDown`, `imagePasteboard`, the typography from `GeneralSettings` |
| `desktop/Tap/Editor/EditorTypography.swift` | The font size and line height every editor shares, from `GeneralSettings` |
| `desktop/Tap/Slides/SlideContextMenu.swift` | Modify: the Generate Image and Insert Image actions; Regenerate Image… items (waits for a mockup) |
| `desktop/Tap/Images/GenerateImageSheet.swift` | The Generate Image sheet (GenerateImage board) |
| `desktop/Tap/Components/NewComponentSheet.swift` | The New Component sheet (NewComponent board) |
| `desktop/Tap/Export/ExportController.swift` | One export per deck window: the options, the run, cancel, the preview server |
| `desktop/Tap/Export/ExportSheet.swift` | The export sheet (ExportPDF and ExportWebsite boards) and its states |
| `desktop/Tap/Export/PreviewServer.swift` | `tap serve <folder> --port 0 --json`, the ready line, stop |
| `desktop/Tap/Settings/SettingsWindowController.swift` | The Settings window and its four panes (the four Settings boards) |
| `desktop/Tap/Settings/GeneralSettingsViewController.swift`, `LiveCodeSettingsViewController.swift`, `ImageGenerationSettingsViewController.swift`, `CommandLineSettingsViewController.swift` | One pane each |
| `desktop/Tap/Settings/CommandLineInstaller.swift` | The symlink into `~/.local/bin`, refusing anything that is not ours |
| `desktop/Tap/Windows/DeckWindowController.swift` | Modify: the Theme toolbar item, `exportController`, the export actions, `showFormSheet`, `canStartATalk` |
| `desktop/Tap/Preview/DeckFormViewController.swift` | Modify: the theme row opens the grid (waits for a mockup) |
| `desktop/Tap/Welcome/WelcomeWindowController.swift` | Modify: the New Deck button's action |
| `desktop/TapTests/Support/HostedTestCase.swift` | Modify: the D6 seams reset in `setUp` |
| `desktop/TapTests/Support/FakeToolScripts.swift` | Scripted tap subcommands: a dispatcher over the real tap, image generate, theme show, export pdf, export images, a cancellable export |
| `desktop/TapTests/Support/MemoryGeminiKeyStore.swift` | The in-memory key store for tests |
| `desktop/TapTests/Fixtures/diagram.png`, `ai-image/`, `broken-component/` | The D6 fixtures |
| `desktop/TapTests/*.swift` | The hosted tests: `ThemeGridTests`, `NewDeckTests`, `ImageInsertTests`, `GenerateImageTests`, `ComponentTests`, `ExportPDFTests`, `ExportWebsiteTests`, `SettingsTests`, `ImageGenerationSettingsTests`, `CommandLineSettingsTests`; D3's `SlideMenuTests` extended |
| `desktop/TapUITests/ThemeUITests.swift` | `testTryAThemeWithoutSavingIt` |
| `.github/workflows/ci.yml` | Modify: the Chromium cache for the Desktop Tests job (the real `tap export pdf` run) |
| `desktop/scenarios.txt`, `desktop/README.md` | Modify: the 22 D6 rows; the export, theme and settings tests and the manual pass |

---

### Task 1: `tap new --folder` and `tap serve --json`

**Files:**
- Modify: `internal/cli/new.go` (the flags, `RunE`, `runNewInFolder`, `freeFolder`)
- Create: `internal/cli/new_folder_test.go`
- Modify: `internal/cli/serve.go` (`serveJSON`, `startServe`, `runServe`)
- Create: `internal/cli/serve_test.go`
- Modify: `internal/cli/jsonout.go` (`printJSONLine`)
- Modify: `docs/reference/cli-commands.md` (the `tap new` and `tap serve` sections), `skills/tap/rules/cli.md` (the same two)

**Interfaces:**
- Consumes: `tui.FilenameFromTitle`, `tui.DefaultTitle`, `tui.DefaultTheme`, `tui.GenerateStarterMarkdown`, `themes.IsValid`, `recordNewDeckApproval`, `userError`, `internalError`, `codeUsage`, `codeFileNotFound`, `codeUnknownTheme`, `printJSONOK`, `jsonEnvelope`, `listenOnAvailablePort`, `runTap`, `withWorkingDirectory` (`new_test.go`).
- Produces: `tap new --folder <location>` printing `<deck path>` or, with `--json`, `{"ok":true,"deck":"<path>","folder":"<path>"}`; `tap serve --json` printing one line `{"ok":true,"dir":"<dir>","port":<n>,"url":"http://localhost:<n>"}` and nothing else on stdout; `printJSONLine(w, payload)`; `startServe(dir string, port int, explicitPort, jsonMode bool, out io.Writer) (*http.Server, net.Listener, error)`. Task 4's `NewDeckResult` and `ServeReady` decode these.

- [ ] **Step 1: Write the failing `tap new --folder` tests**

`internal/cli/new_folder_test.go`:

```go
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// tap new --folder makes the deck's folder for the app's New Deck sheet:
// the folder and the file are named by tap's own slug of the title, so no
// other program has to know the rule, and images/ is ready for tap image add.
func TestNewFolderCreatesTheDeckFolder(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	location := t.TempDir()

	exitCode, stdout, stderr := runTap(t, "new", "--folder", location, "--title", "Debugging Production at 3am", "--theme", "terminal", "--json")
	if exitCode != exitOK {
		t.Fatalf("exit %d, stderr %q", exitCode, stderr)
	}
	var output struct {
		OK     bool   `json:"ok"`
		Deck   string `json:"deck"`
		Folder string `json:"folder"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	wantFolder := filepath.Join(location, "debugging-production-at-3am")
	wantDeck := filepath.Join(wantFolder, "debugging-production-at-3am.md")
	if !output.OK || output.Folder != wantFolder || output.Deck != wantDeck {
		t.Errorf("output = %+v, want folder %s and deck %s", output, wantFolder, wantDeck)
	}
	content, err := os.ReadFile(wantDeck)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "title: Debugging Production at 3am") || !strings.Contains(string(content), "theme: terminal") {
		t.Errorf("deck = %q", content)
	}
	images, err := os.Stat(filepath.Join(wantFolder, "images"))
	if err != nil || !images.IsDir() {
		t.Errorf("no images/ folder: %v", err)
	}

	settings, err := usersettings.Load(filepath.Join(configHome, "tap", "settings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := usersettings.ResolveDeck(wantDeck)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := settings.ApprovalFor(key); !found {
		t.Errorf("no approval for %s: %+v", wantDeck, settings.Approvals)
	}
}

func TestNewFolderAddsASuffixWhenTheFolderExists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	location := t.TempDir()
	if err := os.MkdirAll(filepath.Join(location, "my-talk"), 0o755); err != nil {
		t.Fatal(err)
	}

	exitCode, stdout, stderr := runTap(t, "new", "--folder", location, "--title", "My Talk")
	if exitCode != exitOK {
		t.Fatalf("exit %d, stderr %q", exitCode, stderr)
	}
	want := filepath.Join(location, "my-talk-2", "my-talk-2.md")
	if stdout != want+"\n" {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("deck not written: %v", err)
	}
}

func TestNewFolderDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	location := t.TempDir()

	exitCode, stdout, stderr := runTap(t, "new", "--folder", location)
	if exitCode != exitOK {
		t.Fatalf("exit %d, stderr %q", exitCode, stderr)
	}
	want := filepath.Join(location, "my-presentation", "my-presentation.md")
	if stdout != want+"\n" {
		t.Errorf("stdout = %q, want %q (tui.DefaultTitle's slug)", stdout, want)
	}
}

func TestNewFolderUsageErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	location := t.TempDir()
	cases := []struct {
		name string
		args []string
		code string
		want string
	}{
		{"with a deck argument", []string{"new", "talk.md", "--folder", location}, codeUsage, "--folder names the deck from the title"},
		{"with --output", []string{"new", "--folder", location, "--output", "talk.md"}, codeUsage, "--folder names the deck from the title"},
		{"with --force", []string{"new", "--folder", location, "--force"}, codeUsage, "--force has no meaning with --folder"},
		{"a missing location", []string{"new", "--folder", filepath.Join(location, "nowhere")}, codeFileNotFound, "does not exist"},
		{"an unknown theme", []string{"new", "--folder", location, "--theme", "nope"}, codeUnknownTheme, "unknown theme"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			exitCode, stdout, _ := runTap(t, append(testCase.args, "--json")...)
			if exitCode != exitUserError {
				t.Errorf("exit %d, want %d", exitCode, exitUserError)
			}
			var output struct {
				OK    bool      `json:"ok"`
				Error jsonError `json:"error"`
			}
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
			}
			if output.OK || output.Error.Code != testCase.code || !strings.Contains(output.Error.Message, testCase.want) {
				t.Errorf("error = %+v, want code %s with %q", output.Error, testCase.code, testCase.want)
			}
		})
	}
	if entries, _ := os.ReadDir(location); len(entries) != 0 {
		t.Errorf("a refused run wrote %v", entries)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/cli -run 'TestNewFolder' -v`
Expected: FAIL: `--folder` is an unknown flag (`unknown flag: --folder` in stderr, exit 1 with code `usage`), so every case fails on its expectation.

- [ ] **Step 3: Add `--folder` to `tap new`**

In `internal/cli/new.go`, add `newFolder string` to the flag block, and in `init` after the `--json` flag:

```go
	newCmd.Flags().StringVar(&newFolder, "folder", "", "make a folder named after the title inside this location, with the deck and an images/ folder (skips the wizard)")
```

In `RunE`, before the `if newYes || newJSON || !stdinIsTerminal()` line:

```go
		if newFolder != "" {
			return runNewInFolder(cmd, firstArg(args))
		}
```

Add after `runNewNonInteractive`:

```go
// runNewInFolder writes the starter deck into a folder of its own inside
// newFolder, named by the title's slug (tui.FilenameFromTitle, the rule
// the wizard uses for a file name), with an images/ folder beside the
// deck for tap image add. A folder with that name that already exists
// gets -2, -3 and so on, so nothing is ever overwritten. It never runs
// the wizard: it exists for programs that create decks, such as the
// desktop app's New Deck sheet.
func runNewInFolder(cmd *cobra.Command, deckArg string) error {
	if deckArg != "" || cmd.Flags().Changed("output") {
		return userError(codeUsage, errors.New("--folder names the deck from the title: give no deck path and no --output"))
	}
	if newForce {
		return userError(codeUsage, errors.New("--force has no meaning with --folder: the folder is always new"))
	}
	info, err := os.Stat(newFolder)
	switch {
	case os.IsNotExist(err):
		return userError(codeFileNotFound, fmt.Errorf("the --folder location does not exist: %s", newFolder))
	case err != nil:
		return internalError(codeInternal, fmt.Errorf("failed to check %s: %w", newFolder, err))
	case !info.IsDir():
		return userError(codeUsage, fmt.Errorf("the --folder location is not a folder: %s", newFolder))
	}

	title := newTitle
	if title == "" {
		title = tui.DefaultTitle
	}
	theme := newTheme
	if theme == "" {
		theme = tui.DefaultTheme()
	} else if !themes.IsValid(theme) {
		return userError(codeUnknownTheme, unknownThemeError(theme))
	}

	stem := strings.TrimSuffix(tui.FilenameFromTitle(title), ".md")
	folder, name, err := freeFolder(newFolder, stem)
	if err != nil {
		return internalError(codeInternal, err)
	}
	if err := os.MkdirAll(filepath.Join(folder, "images"), 0o755); err != nil {
		return internalError(codeInternal, fmt.Errorf("failed to create %s: %w", folder, err))
	}
	output := filepath.Join(folder, name+".md")
	content := tui.GenerateStarterMarkdown(title, theme, time.Now().Format("2006-01-02"), "Your Name")
	if err := os.WriteFile(output, []byte(content), 0o644); err != nil {
		return internalError(codeInternal, fmt.Errorf("failed to write %s: %w", output, err))
	}

	recordNewDeckApproval(cmd, output)

	if newJSON {
		return printJSONOK(cmd.OutOrStdout(), struct {
			Deck   string `json:"deck"`
			Folder string `json:"folder"`
		}{Deck: output, Folder: folder})
	}
	fmt.Fprintln(cmd.OutOrStdout(), output)
	return nil
}

// freeFolder returns <location>/<stem>, or <location>/<stem>-2, -3 and
// so on when that exists, and the name it settled on.
func freeFolder(location, stem string) (folder, name string, err error) {
	name = stem
	for attempt := 2; ; attempt++ {
		folder = filepath.Join(location, name)
		_, statErr := os.Stat(folder)
		if os.IsNotExist(statErr) {
			return folder, name, nil
		}
		if statErr != nil {
			return "", "", fmt.Errorf("failed to check %s: %w", folder, statErr)
		}
		name = fmt.Sprintf("%s-%d", stem, attempt)
	}
}
```

Add `"path/filepath"` to the imports. Update the command's `Long` text with two lines after the approval paragraph: `With --folder <location>, tap makes a folder named after the title inside <location>, with the deck (named the same) and an images/ folder, and prints the deck's path. This is the mode for programs that create decks, such as Tap Desktop.` and an example `tap new --folder ~/talks --title "My Talk" --theme terminal --json`.

- [ ] **Step 4: Run the new tests**

Run: `go test ./internal/cli -run 'TestNew' -v`
Expected: every `TestNewFolder*` passes, and the existing `TestNew*` and `TestRunNew*` still pass (nothing they exercise changed).

- [ ] **Step 5: Write the failing `tap serve --json` test**

`internal/cli/serve_test.go`:

```go
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// tap serve --json prints one ready line and nothing else on standard
// output, so a program (the desktop app's Preview button) learns the port
// without parsing text meant for a person.
func TestServeJSONPrintsOneReadyLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>built</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	server, listener, err := startServe(dir, 0, true, true, &out)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer func() { _ = server.Shutdown(shutdown) }()
	go func() { _ = server.Serve(listener) }()

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout = %q, want one line", out.String())
	}
	var ready struct {
		OK   bool   `json:"ok"`
		Dir  string `json:"dir"`
		Port int    `json:"port"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &ready); err != nil {
		t.Fatalf("the ready line is not JSON: %v\n%s", err, lines[0])
	}
	if !ready.OK || ready.Dir != dir || ready.Port == 0 || ready.URL != fmt.Sprintf("http://localhost:%d", ready.Port) {
		t.Errorf("ready = %+v", ready)
	}

	response, err := http.Get(ready.URL + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if !strings.Contains(string(body), "built") {
		t.Errorf("served %q", body)
	}
	if out.String() != lines[0]+"\n" {
		t.Errorf("a request was logged to stdout in --json mode: %q", out.String())
	}
}

func TestServeWithoutJSONPrintsTheBanner(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	server, listener, err := startServe(dir, 0, true, false, &out)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer func() { _ = server.Shutdown(shutdown) }()
	if !strings.Contains(out.String(), "Serving presentation from") || !strings.Contains(out.String(), "http://localhost:") {
		t.Errorf("banner = %q", out.String())
	}
}
```

- [ ] **Step 6: Run it to verify it fails**

Run: `go test ./internal/cli -run 'TestServe' -v`
Expected: FAIL to compile: `startServe` is undefined.

- [ ] **Step 7: Add `--json` and `startServe`**

In `internal/cli/jsonout.go`, after `printJSONOK`:

```go
// printJSONLine writes a successful command's result as one compact line,
// {"ok":true,...} with payload's fields, for a command that keeps running
// after it (tap serve --json): a program reads the line and knows the
// rest of stdout is quiet.
func printJSONLine(w io.Writer, payload any) error {
	body, err := jsonEnvelope("JSON result", `{"ok":true`, payload, false)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	_, err = w.Write(body)
	return err
}
```

In `internal/cli/serve.go`, add `serveJSON bool` to the flag block and `serveCmd.Flags().BoolVar(&serveJSON, "json", false, "print one ready line as JSON and log no requests")` in `init`. Replace `runServe` with:

```go
func runServe(cmd *cobra.Command, args []string) error {
	dir := "dist"
	if len(args) > 0 {
		dir = args[0]
	}
	httpServer, listener, err := startServe(dir, servePort, cmd.Flags().Changed("port"), serveJSON, cmd.OutOrStdout())
	if err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return internalError(codeInternal, fmt.Errorf("server error: %w", err))
	case <-sigCh:
		if !serveJSON {
			fmt.Println()
			Info("Shutting down server...\n")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		return internalError(codeInternal, fmt.Errorf("error during shutdown: %w", err))
	}
	if !serveJSON {
		Successln("Server stopped.")
	}
	return nil
}

// serveReady is the --json ready line of tap serve.
type serveReady struct {
	Dir  string `json:"dir"`
	Port int    `json:"port"`
	URL  string `json:"url"`
}

// startServe checks dir, binds the listener and prints what a person or a
// program needs to open the site: the banner, or with jsonMode one ready
// line on out and nothing else there (no request log). The caller serves
// on the listener and shuts the server down. Separate from runServe so a
// test can bind, read the line and stop, without a signal.
func startServe(dir string, port int, explicitPort, jsonMode bool, out io.Writer) (*http.Server, net.Listener, error) {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if !jsonMode {
			fmt.Fprintln(out)
			Muted("  Hint: Run 'tap build [deck]' first to generate static files.\n")
		}
		return nil, nil, userError(codeDeckNotFound, fmt.Errorf("directory does not exist: %s", dir))
	}
	if err != nil {
		return nil, nil, userError(codeUsage, fmt.Errorf("cannot access directory: %w", err))
	}
	if !info.IsDir() {
		return nil, nil, userError(codeUsage, fmt.Errorf("not a directory: %s", dir))
	}

	fs := http.FileServer(http.Dir(dir))
	var handler http.Handler = fs
	if !jsonMode {
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			fs.ServeHTTP(w, r)
			Info("GET ")
			fmt.Fprintf(out, "%s ", r.URL.Path)
			Muted("(%s)\n", time.Since(start).Round(time.Microsecond))
		})
	}

	listener, err := listenOnAvailablePort(port, explicitPort, "tap serve")
	if err != nil {
		return nil, nil, userError(codeUsage, err)
	}
	boundPort := port
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		boundPort = tcpAddr.Port
	}
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	if jsonMode {
		if err := printJSONLine(out, serveReady{Dir: dir, Port: boundPort, URL: fmt.Sprintf("http://localhost:%d", boundPort)}); err != nil {
			_ = listener.Close()
			return nil, nil, err
		}
		return httpServer, listener, nil
	}
	fmt.Fprintln(out)
	Success("  Serving presentation from %s\n", dir)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  Local:   http://localhost:%d\n", boundPort)
	fmt.Fprintf(out, "  Network: http://0.0.0.0:%d\n", boundPort)
	fmt.Fprintln(out)
	Muted("  Press Ctrl+C to stop\n")
	fmt.Fprintln(out)
	return httpServer, listener, nil
}
```

`Info`, `Muted` and `Success` (`output.go`) write to the process's stdout; in `--json` mode nothing calls them, so the ready line is stdout's only line. `io` joins the imports. The `Long` text gains `With --json, tap prints one line, {"ok":true,"dir":...,"port":...,"url":...}, and logs no requests, for a program that opens the site.` and an example `tap serve dist --port 0 --json`.

- [ ] **Step 8: Run the serve tests and the conventions test**

Run: `go test ./internal/cli -run 'TestServe|TestConventions|TestHelp' -v`
Expected: the two serve tests pass; the flag conventions tests pass (`--json` has no short form anywhere, and `--folder` is new with none). `go vet ./internal/cli` clean.

- [ ] **Step 9: Document both**

In `docs/reference/cli-commands.md`, the `tap new` section: add `| --folder <location> | none | Make a folder named after the title inside <location>, with the deck and an images/ folder; skips the wizard; cannot be combined with [deck], --output or --force. |` to the flags table, the example `tap new --folder ~/talks --title "My Talk" --theme terminal --json   # ~/talks/my-talk/my-talk.md, with images/`, and under `### --json` the second shape `{"ok": true, "deck": "...", "folder": "..."}`. The `tap serve` section: `| --json | none | Print one ready line ({"ok":true,"dir","port","url"}) and log no requests, for a program that opens the site. |` and the example `tap serve dist --port 0 --json`. In `skills/tap/rules/cli.md`, the same two flags in the `tap new` and `tap serve` blocks, one line each.

- [ ] **Step 10: Mutate and commit**

Mutations, each applied and run locally, then reverted exactly: in `freeFolder`, return `folder` without checking `os.Stat` (expected: `TestNewFolderAddsASuffixWhenTheFolderExists` fails on `stdout`); in `runNewInFolder`, skip `os.MkdirAll` of `images` and make only the folder (expected: `TestNewFolderCreatesTheDeckFolder` fails on "no images/ folder"); in `runNewInFolder`, drop `recordNewDeckApproval` (expected: fails on "no approval"); in `runNewInFolder`, allow `--output` (expected: `TestNewFolderUsageErrors/with --output` fails); in `startServe`, print the ready line with `printJSONOK` (expected: `TestServeJSONPrintsOneReadyLine` fails on "want one line"); in `startServe`, keep the logging handler in JSON mode (expected: fails on "a request was logged"); in `startServe`, print `url` with the requested port instead of `boundPort` (expected: fails on `ready.URL`).

```bash
git add internal/cli/new.go internal/cli/new_folder_test.go internal/cli/serve.go internal/cli/serve_test.go internal/cli/jsonout.go docs/reference/cli-commands.md skills/tap/rules/cli.md
git commit -m "feat(cli): tap new --folder makes the deck's folder, and tap serve --json prints a ready line"
```

---

### Task 2: `tap theme show --image --progress json`, and `tap image generate --aspect --match-theme`

**Files:**
- Modify: `internal/cli/theme.go` (`themeShowProgress`), `internal/cli/theme_image.go` (`showThemeImage`, `renderIntoCache`, `renderToTemporaryFile`, `renderTheme`, `renderThemeImage`'s signature, `renderThemeImageWithBrowser`)
- Modify: `internal/cli/theme_image_test.go` (`useFakeThemeRenderer`, a progress test)
- Modify: `internal/cli/progress_commands_test.go` (`TestProgressFlagOnTheLongRunningCommands`)
- Modify: `internal/deckedit/generate.go` (`ImageGenerator`)
- Modify: `internal/cli/image.go` (the flags, `generateOptions`, `generateAndPlace`, `themeBriefForDeck`, `validAspectRatios`)
- Modify: `internal/cli/edit_helpers_test.go` (`fakeImageGenerator.GenerateImageWithAspectRatio`), `internal/cli/image_test.go` (the aspect and match-theme tests)
- Modify: `docs/reference/cli-commands.md`, `skills/tap/rules/cli.md`

**Interfaces:**
- Consumes: `newProgressReporter`, `progressReporter.Render`, `Download`, `Result`, `pdf.Exporter.SetProgress`, `pdf.Progress`, `buildThemePrompt`, `findTheme`, `themes.Tokens`, `themes.Illustration`, `config.Load`, `gemini.Client.GenerateImageWithAspectRatio`, `useFakeThemeRenderer`, `useFakeImageGenerator`, `checkProgressOutput`, `phasesOf`.
- Produces: `tap theme show <slug> --image --progress json` writing `download` lines while Chromium downloads, one `{"phase":"render","done":1,"total":1}` line, and `{"phase":"done","ok":true,"slug","image","cached"}` (a cached image writes the done line only); `tap image generate|regenerate ... [--aspect 1:1|16:9|9:16|4:3|3:4] [--match-theme]`; `deckedit.ImageGenerator` gains `GenerateImageWithAspectRatio(ctx, prompt, aspectRatio string)`. Task 6's `ThemeImageLoader` reads the progress; Task 8's Generate Image sheet passes the two flags.

- [ ] **Step 1: Write the failing theme progress test**

In `internal/cli/theme_image_test.go`, change `useFakeThemeRenderer`'s replacement to the new signature (the fake ignores `progress`; `renderTheme` reports the render line itself):

```go
	renderThemeImage = func(ctx context.Context, theme themes.Theme, outputPath string, progress pdf.Progress) error {
		count++
		return os.WriteFile(outputPath, fakeThemeImagePNG(), 0o644)
	}
```

and add:

```go
// The desktop app's theme grid renders every theme through this command
// and shows the one-time Chromium download the first render makes, so the
// command reports progress the way tap export does.
func TestThemeShowImageProgressJSON(t *testing.T) {
	renders, cacheRoot := useFakeThemeRenderer(t)
	wantPath := filepath.Join(cacheRoot, "tap", "themes", "9.9.9-test", "terminal.png")

	exitCode, stdout, stderr := runTap(t, "theme", "show", "terminal", "--image", "--progress", "json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if stdout != wantPath+"\n" {
		t.Errorf("stdout = %q, want the path alone", stdout)
	}
	lines := checkProgressOutput(t, stderr)
	if phasesOf(lines) != "render,done" {
		t.Errorf("phases = %s, want render,done", phasesOf(lines))
	}
	done := lines[len(lines)-1]
	if done["ok"] != true || done["slug"] != "terminal" || done["image"] != wantPath || done["cached"] != false {
		t.Errorf("done line = %v", done)
	}
	if *renders != 1 {
		t.Errorf("renders = %d, want 1", *renders)
	}

	// The second call is a cache hit: no render line, cached true.
	_, _, stderr = runTap(t, "theme", "show", "terminal", "--image", "--progress", "json")
	lines = checkProgressOutput(t, stderr)
	if phasesOf(lines) != "done" || lines[0]["cached"] != true {
		t.Errorf("cached run: %v", lines)
	}
	if *renders != 1 {
		t.Errorf("renders = %d after a cache hit, want 1", *renders)
	}
}

func TestThemeShowProgressNeedsImage(t *testing.T) {
	exitCode, _, stderr := runTap(t, "theme", "show", "terminal", "--progress", "json")
	if exitCode != exitUserError || !strings.Contains(stderr, "--progress needs --image") {
		t.Errorf("exit %d, stderr %q", exitCode, stderr)
	}
}
```

In `progress_commands_test.go`, add `{"theme", "show"}` to the paths of `TestProgressFlagOnTheLongRunningCommands`.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/cli -run 'TestThemeShow|TestProgressFlagOnTheLongRunningCommands' -v`
Expected: FAIL to compile (`renderThemeImage`'s signature has three parameters), which is this step's failure.

- [ ] **Step 3: Thread the progress through the theme render**

In `theme.go`: `themeShowProgress string` in the flag block; `themeShowCmd.Flags().StringVar(&themeShowProgress, "progress", "", "with --image, print progress to stderr as JSON lines (json)")` in `init`; in `runThemeShow`, after the `--output needs --image` check:

```go
	if themeShowProgress != "" && !themeShowImage {
		return userError(codeUsage, errors.New("--progress needs --image"))
	}
	progress, err := newProgressReporter(themeShowProgress, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
```

and `return showThemeImage(cmd, theme, progress)`.

In `theme_image.go`: `renderThemeImage` becomes `var renderThemeImage = renderThemeImageWithBrowser` with the type `func(ctx context.Context, theme themes.Theme, outputPath string, progress pdf.Progress) error`; `showThemeImage(cmd, theme, progress *progressReporter)` passes `progress` into `renderIntoCache(theme, cachePath, progress)`, which passes it to `renderTheme(theme, path, progress)` and `renderToTemporaryFile(theme, progress)`; `renderTheme` calls `renderThemeImage(ctx, theme, outputPath, progressForRender(progress))` and, on success, `progress.Render(1, 1)`; `showThemeImage` ends with `if err := progress.Result(themeImageResult{Slug: theme.Slug, Image: image, Cached: cached}); err != nil { return err }` before the `--json` and plain prints. Add:

```go
// progressForRender is the pdf.Progress the browser render reports the
// Chromium download to: the reporter when --progress json is on, nil (no
// reporting) otherwise, since *progressReporter's methods write nothing
// when disabled but pdf.Exporter.SetProgress is only called with a real one.
func progressForRender(progress *progressReporter) pdf.Progress {
	if progress.enabled() {
		return progress
	}
	return nil
}
```

In `renderThemeImageWithBrowser`, after `exporter, err := pdf.New()`: `if progress != nil { exporter.SetProgress(progress) }`. The `Long` text gains `--progress json prints the engine download and the render as JSON lines on stderr, as tap export does.` and the example `tap theme show terminal --image --progress json`.

- [ ] **Step 4: Run the theme tests**

Run: `go test ./internal/cli -run 'TestThemeShow|TestProgress' -v`
Expected: every test passes; `TestThemeShowImageRendersOnceThenUsesTheCache` and its neighbours still pass with the fake's new signature.

- [ ] **Step 5: Write the failing image flag tests**

In `internal/cli/edit_helpers_test.go`, give the fake the new method and record what it was asked:

```go
// aspects records the aspect ratio of each call, "" for GenerateImage.
type fakeImageGenerator struct {
	err     error
	prompts []string
	aspects []string
}

func (f *fakeImageGenerator) GenerateImage(ctx context.Context, prompt string) (*gemini.ImageResult, error) {
	return f.GenerateImageWithAspectRatio(ctx, prompt, "")
}

func (f *fakeImageGenerator) GenerateImageWithAspectRatio(ctx context.Context, prompt string, aspectRatio string) (*gemini.ImageResult, error) {
	f.prompts = append(f.prompts, prompt)
	f.aspects = append(f.aspects, aspectRatio)
	if f.err != nil {
		return nil, f.err
	}
	return &gemini.ImageResult{Data: []byte("png bytes for " + prompt), ContentType: "image/png"}, nil
}
```

(`TestImageGenerateAddsTheImageToTheSlide` checks `fake.prompts[0] == "a red fox"`, which still holds: the bytes are derived from the prompt as sent, so a match-theme run's file name differs, which the new test accounts for.) In `internal/cli/image_test.go`, add:

```go
const themedImageDeck = "---\ntheme: terminal\n---\n\n# One\n\n---\n\n# Two\n"

// The desktop app's Generate Image sheet offers an aspect and "Match
// theme". Both are tap's: the aspect reaches Gemini, and the theme's own
// style brief (tap theme show --prompt) goes in front of the person's
// words, while the ai-prompt comment keeps the person's words alone, so
// tap image regenerate reads what they wrote.
func TestImageGenerateAspectAndMatchTheme(t *testing.T) {
	fake := useFakeImageGenerator(t)
	deckDir := t.TempDir()
	deck := writeDeckFile(t, deckDir, "talk.md", themedImageDeck)

	exitCode, stdout, stderr := runTap(t, "image", "generate", deck, "--slide", "2", "--prompt", "a red fox", "--aspect", "16:9", "--match-theme", "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if len(fake.aspects) != 1 || fake.aspects[0] != "16:9" {
		t.Errorf("aspects = %q, want [16:9]", fake.aspects)
	}
	if len(fake.prompts) != 1 || !strings.HasPrefix(fake.prompts[0], `Illustration style for the "Terminal" slide theme`) || !strings.HasSuffix(fake.prompts[0], "\n\nThe image shows: a red fox") {
		t.Errorf("the request = %q, want the Terminal brief then the person's words", fake.prompts)
	}
	var output struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil || output.Prompt != "a red fox" {
		t.Errorf("output prompt = %q (%v), want the person's words", output.Prompt, err)
	}
	content, _ := os.ReadFile(deck)
	if !strings.Contains(string(content), "<!-- ai-prompt: a red fox -->") || strings.Contains(string(content), "Illustration style") {
		t.Errorf("deck = %q, want the person's prompt in the comment and no brief", content)
	}
}

func TestImageGenerateWithoutTheFlagsAsksPlainly(t *testing.T) {
	fake := useFakeImageGenerator(t)
	deck := writeDeckFile(t, t.TempDir(), "talk.md", themedImageDeck)
	if exitCode, _, stderr := runTap(t, "image", "generate", deck, "--slide", "2", "--prompt", "a red fox"); exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if fake.prompts[0] != "a red fox" || fake.aspects[0] != "" {
		t.Errorf("request = %q %q, want the words alone and no aspect", fake.prompts[0], fake.aspects[0])
	}
}

func TestImageGenerateRejectsAnUnknownAspect(t *testing.T) {
	useFakeImageGenerator(t)
	deck := writeDeckFile(t, t.TempDir(), "talk.md", imageDeck)
	exitCode, _, stderr := runTap(t, "image", "generate", deck, "--slide", "2", "--prompt", "x", "--aspect", "2:1")
	if exitCode != exitUserError || !strings.Contains(stderr, "--aspect must be one of 1:1, 16:9, 9:16, 4:3, 3:4") {
		t.Errorf("exit %d, stderr %q", exitCode, stderr)
	}
}

func TestImageRegenerateTakesTheSameFlags(t *testing.T) {
	fake := useFakeImageGenerator(t)
	deckDir := t.TempDir()
	deck := writeDeckFile(t, deckDir, "talk.md", themedImageDeck)
	if exitCode, _, stderr := runTap(t, "image", "generate", deck, "--slide", "2", "--prompt", "a red fox"); exitCode != exitOK {
		t.Fatalf("generate: exit %d, stderr %q", exitCode, stderr)
	}
	image := "images/" + deckedit.GenerateImageFilename([]byte("png bytes for a red fox"), "image/png")
	exitCode, _, stderr := runTap(t, "image", "regenerate", deck, "--slide", "2", "--image", image, "--aspect", "1:1", "--match-theme")
	if exitCode != exitOK {
		t.Fatalf("regenerate: exit %d, stderr %q", exitCode, stderr)
	}
	if fake.aspects[1] != "1:1" || !strings.HasSuffix(fake.prompts[1], "The image shows: a red fox") {
		t.Errorf("regenerate request = %q %q", fake.prompts[1], fake.aspects[1])
	}
}
```

- [ ] **Step 6: Run them to verify they fail**

Run: `go test ./internal/cli -run 'TestImageGenerate|TestImageRegenerate' -v`
Expected: the new tests fail on `unknown flag: --aspect`; the existing image tests still pass (the fake's `GenerateImage` delegates).

- [ ] **Step 7: Add the flags**

In `internal/deckedit/generate.go`, the interface becomes:

```go
// ImageGenerator makes an image from a text prompt. *gemini.Client is one.
// GenerateImageWithAspectRatio takes one of Gemini's aspect ratios ("1:1",
// "16:9", "9:16", "4:3", "3:4"), or "" for the model's own choice.
type ImageGenerator interface {
	GenerateImage(ctx context.Context, prompt string) (*gemini.ImageResult, error)
	GenerateImageWithAspectRatio(ctx context.Context, prompt string, aspectRatio string) (*gemini.ImageResult, error)
}
```

In `internal/cli/image.go`, add to the flag blocks `imageGenerateAspect string`, `imageGenerateMatchTheme bool`, `imageRegenerateAspect string`, `imageRegenerateMatchTheme bool`, and in `init`, for each of the two commands:

```go
	imageGenerateCmd.Flags().StringVar(&imageGenerateAspect, "aspect", "", "the image's aspect ratio: 1:1, 16:9, 9:16, 4:3 or 3:4 (default: the model's choice)")
	imageGenerateCmd.Flags().BoolVar(&imageGenerateMatchTheme, "match-theme", false, "put the deck theme's style brief (tap theme show --prompt) in front of the prompt sent to the model; the ai-prompt comment keeps your words")
```

(and the `imageRegenerate*` pair on `imageRegenerateCmd`). Add:

```go
// validAspectRatios are the aspect ratios Gemini's image config accepts.
var validAspectRatios = []string{"1:1", "16:9", "9:16", "4:3", "3:4"}

// generateOptions is what the person chose beyond the prompt.
type generateOptions struct {
	aspect     string
	matchTheme bool
}

// generateOptionsFrom validates --aspect.
func generateOptionsFrom(aspect string, matchTheme bool) (generateOptions, error) {
	if aspect != "" && !slices.Contains(validAspectRatios, aspect) {
		return generateOptions{}, userError(codeUsage, fmt.Errorf("--aspect must be one of %s, got %q", strings.Join(validAspectRatios, ", "), aspect))
	}
	return generateOptions{aspect: aspect, matchTheme: matchTheme}, nil
}

// themeBriefForDeck is the deck theme's style brief, the text tap theme
// show --prompt prints, for --match-theme. A deck that names no theme or
// an unknown one uses base, as tap renders it.
func themeBriefForDeck(deck string) (string, error) {
	slug := "base"
	if cfg, err := config.Load(deck); err == nil && cfg.Theme != "" && themes.IsValid(cfg.Theme) {
		slug = cfg.Theme
	}
	theme, ok := findTheme(slug)
	if !ok {
		return "", internalError(codeInternal, fmt.Errorf("theme %q is not built in", slug))
	}
	tokens, ok := themes.Tokens(slug)
	if !ok {
		return "", internalError(codeInternal, fmt.Errorf("theme %q has no tokens", slug))
	}
	illustration, _ := themes.Illustration(slug)
	return buildThemePrompt(theme, tokens, illustration), nil
}
```

`runImageGenerate` and `runImageRegenerate` each build `options, err := generateOptionsFrom(imageGenerateAspect, imageGenerateMatchTheme)` (the regenerate variables for the other) right after the `--slide` check and pass `options` to `generateAndPlace(ctx, placement, options)`, whose body becomes:

```go
func generateAndPlace(ctx context.Context, placement deckedit.Placement, options generateOptions) (deckedit.PlacedImage, error) {
	generator, err := deckedit.NewImageGenerator(placement.DeckPath)
	if err != nil {
		return deckedit.PlacedImage{}, userError(codeNoAPIKey, fmt.Errorf("cannot start image generation: %w", err))
	}
	// What the model is asked can carry the theme's brief; what the deck
	// records is the person's words, so regenerate reads what they wrote.
	request := placement.Prompt
	if options.matchTheme {
		brief, err := themeBriefForDeck(placement.DeckPath)
		if err != nil {
			return deckedit.PlacedImage{}, err
		}
		request = brief + "\n\nThe image shows: " + placement.Prompt
	}
	image, err := generator.GenerateImageWithAspectRatio(ctx, request, options.aspect)
	if err != nil {
		if ctx.Err() != nil {
			return deckedit.PlacedImage{}, errInterrupted
		}
		return deckedit.PlacedImage{}, imageGenerationError(err)
	}
	placed, err := deckedit.PlaceGeneratedImage(placement, *image)
	if err != nil {
		return deckedit.PlacedImage{}, userError(codeInvalidDeck, fmt.Errorf("cannot add the image to %s: %w", placement.DeckPath, err))
	}
	return placed, nil
}
```

`"slices"`, `"github.com/MiniCodeMonkey/tap/internal/config"` and `"github.com/MiniCodeMonkey/tap/internal/themes"` join the imports. The two `Long` texts each gain a sentence and an example with both flags. Check the TUI still compiles: `internal/tui/imagegen.go` calls `GenerateImage` on the interface, which the Gemini client still has; `grep -rn "deckedit.ImageGenerator" internal/` finds every other implementer (the fake in `edit_helpers_test.go` is the one).

- [ ] **Step 8: Run the whole package, then document**

Run: `go test ./internal/cli ./internal/deckedit ./internal/tui && go vet ./...`
Expected: everything passes. In `docs/reference/cli-commands.md`, the `tap image generate` and `tap image regenerate` flag tables gain `--aspect` and `--match-theme` rows (the wording of the flag help), and `tap theme show` gains the `--progress json` sentence and example. In `skills/tap/rules/cli.md`, the same three additions, one line each.

- [ ] **Step 9: Mutate and commit**

Mutations, each applied and run locally, then reverted exactly: in `generateAndPlace`, send `placement.Prompt` regardless of `matchTheme` (expected: `TestImageGenerateAspectAndMatchTheme` fails on the request's prefix); in `generateAndPlace`, record `request` as the prompt (pass `request` in `placement.Prompt` to `PlaceGeneratedImage`) (expected: fails on the deck's comment); in `generateOptionsFrom`, accept any aspect (expected: `TestImageGenerateRejectsAnUnknownAspect` fails); in `runImageRegenerate`, pass the generate flags' variables instead of the regenerate ones (expected: `TestImageRegenerateTakesTheSameFlags` fails on `aspects[1]`); in `renderTheme`, drop `progress.Render(1, 1)` (expected: `TestThemeShowImageProgressJSON` fails on `render,done`); in `showThemeImage`, drop `progress.Result` (expected: fails on the done line); in `runThemeShow`, drop the `--progress needs --image` check (expected: `TestThemeShowProgressNeedsImage` fails).

```bash
git add internal/cli internal/deckedit/generate.go docs/reference/cli-commands.md skills/tap/rules/cli.md
git commit -m "feat(cli): progress for theme renders, and an aspect and theme match for generated images"
```

---

### Task 3: `ToolRun`: one-shot tap runs, progress lines, SIGINT cancel

**Files:**
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/ToolRun.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/ToolRunTests.swift`

**Interfaces:**
- Consumes: `LineBuffer` (D2), `TapErrorPayload` (D2, `code`, `message`).
- Produces: `ProgressLine` (`.step(phase:done:total:)`, `.download(bytes:totalBytes:)`, `.finished(ToolOutcome)`; `decode(line:)`), `ToolOutcome` (`.ok(Data)`, `.failed(code:message:)`; `decode(_ data:)`; `result(_:)`), `ToolError`, `ToolRun` (`Configuration(executableURL:arguments:environment:currentDirectoryURL:timeout:)`, `onProgress`, `onStandardOutputLine`, `onStandardErrorLine`, `onExit`, `start()`, `cancel()`, `isRunning`, `Exit(status:standardOutput:cancelled:timedOut:outcome:)`, `static run(_:onProgress:) async -> Exit`). Every later task's tap subcommand goes through it (Task 6's `TapTool` wraps it).

- [ ] **Step 1: Write the failing tests**

`desktop/TapDesktopCore/Tests/TapDesktopCoreTests/ToolRunTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class ToolRunTests: XCTestCase {
    func testDecodesTheProgressLines() {
        XCTAssertEqual(ProgressLine.decode(line: #"{"phase":"render","done":7,"total":14}"#), .step(phase: "render", done: 7, total: 14))
        XCTAssertEqual(ProgressLine.decode(line: #"{"phase":"download","bytes":67108864,"totalBytes":157286400}"#), .download(bytes: 67_108_864, totalBytes: 157_286_400))
        let done = ProgressLine.decode(line: #"{"phase":"done","ok":true,"output":"/t/talk.pdf","pages":14,"bytes":120000,"brokenSlides":[{"slide":2,"message":"boom"}]}"#)
        guard case .finished(.ok(let payload))? = done else { return XCTFail("not a finished line: \(String(describing: done))") }
        XCTAssertTrue(String(decoding: payload, as: UTF8.self).contains(#""pages":14"#), "the whole done object is the payload")
        XCTAssertEqual(ProgressLine.decode(line: #"{"phase":"done","ok":false,"error":{"code":"interrupted","message":"interrupted"}}"#),
                       .finished(.failed(code: "interrupted", message: "interrupted")))
        XCTAssertNil(ProgressLine.decode(line: "warning: slide 2 shows an error card: boom"), "a plain stderr line is not progress")
        XCTAssertNil(ProgressLine.decode(line: #"{"phase":"render"}"#), "a step without counts is not one")
        XCTAssertNil(ProgressLine.decode(line: ""))
    }

    func testDecodesAJSONResult() throws {
        struct Result: Decodable, Equatable { let deck: String; let theme: String }
        let ok = ToolOutcome.decode(Data(#"{"ok": true, "deck": "/t/talk.md", "theme": "terminal"}"#.utf8))
        XCTAssertEqual(try ok?.result(Result.self), Result(deck: "/t/talk.md", theme: "terminal"))
        let failed = ToolOutcome.decode(Data(#"{"ok": false, "error": {"code": "unknown_theme", "message": "unknown theme \"x\""}}"#.utf8))
        XCTAssertEqual(failed, .failed(code: "unknown_theme", message: "unknown theme \"x\""))
        XCTAssertThrowsError(try failed?.result(Result.self)) { error in
            XCTAssertEqual(error as? ToolError, .failed(code: "unknown_theme", message: "unknown theme \"x\""))
        }
        XCTAssertNil(ToolOutcome.decode(Data("Theme set to terminal in talk.md\n".utf8)), "text is not an outcome")
        XCTAssertNil(ToolOutcome.decode(Data()))
    }

    @MainActor
    func testRunsAScriptAndReportsItsProgress() async throws {
        let script = try Self.script("""
        #!/bin/sh
        echo '{"phase":"render","done":1,"total":2}' >&2
        echo 'warning: slide 2 shows an error card: boom' >&2
        echo '{"phase":"render","done":2,"total":2}' >&2
        echo '{"phase":"done","ok":true,"files":["a.png"]}' >&2
        printf '{"ok": true, "files": ["a.png"]}\\n'
        exit 0
        """)
        var progress: [ProgressLine] = []
        var otherLines: [String] = []
        let run = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: 10))
        run.onProgress = { progress.append($0) }
        run.onStandardErrorLine = { otherLines.append($0) }
        let exit = await run.run()
        XCTAssertEqual(exit.status, 0)
        XCTAssertFalse(exit.cancelled)
        XCTAssertEqual(progress.count, 3)
        XCTAssertEqual(progress.first, .step(phase: "render", done: 1, total: 2))
        XCTAssertEqual(otherLines, ["warning: slide 2 shows an error card: boom"], "a plain line goes to the log, not to progress")
        struct Files: Decodable { let files: [String] }
        XCTAssertEqual(try exit.outcome?.result(Files.self).files, ["a.png"])
        XCTAssertFalse(run.isRunning)
    }

    @MainActor
    func testCancelSendsSIGINTAndWaitsForTheExit() async throws {
        let script = try Self.script("""
        #!/bin/sh
        trap 'echo "{\\"phase\\":\\"done\\",\\"ok\\":false,\\"error\\":{\\"code\\":\\"interrupted\\",\\"message\\":\\"interrupted\\"}}" >&2; exit 130' INT
        echo '{"phase":"render","done":1,"total":30}' >&2
        i=0; while [ $i -lt 300 ]; do sleep 0.1; i=$((i + 1)); done
        exit 0
        """)
        let run = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: 60))
        var sawFirstLine = false
        run.onProgress = { if case .step = $0 { sawFirstLine = true } }
        var exit: ToolRun.Exit?
        run.onExit = { exit = $0 }
        try run.start()
        try await waitUntil(timeout: 5, "the first progress line") { sawFirstLine }
        run.cancel()
        try await waitUntil(timeout: 10, "the exit") { exit != nil }
        XCTAssertEqual(exit?.status, 130)
        XCTAssertEqual(exit?.cancelled, true)
        XCTAssertEqual(exit?.outcome, .failed(code: "interrupted", message: "interrupted"), "tap's own done line is the outcome")
    }

    @MainActor
    func testATimeoutKillsTheProcess() async throws {
        let script = try Self.script("#!/bin/sh\ni=0; while [ $i -lt 300 ]; do sleep 0.1; i=$((i + 1)); done\n")
        let run = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: 1))
        let exit = await run.run()
        XCTAssertTrue(exit.timedOut)
        XCTAssertNotEqual(exit.status, 0)
        XCTAssertNil(exit.outcome)
    }

    @MainActor
    func testAnExecutableThatDoesNotStartThrows() {
        let run = ToolRun(configuration: .init(executableURL: URL(fileURLWithPath: "/nonexistent/tap"), arguments: [], environment: [:], currentDirectoryURL: nil, timeout: 1))
        XCTAssertThrowsError(try run.start())
        XCTAssertFalse(run.isRunning)
    }

    static func script(_ text: String) throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-core-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let url = folder.appendingPathComponent("tool")
        try text.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }
}
```

`waitUntil(timeout:_:_:)` is the core tests' helper in `Tests/TapDesktopCoreTests/Support` (D2's `TapProcessTests` use it); if the core support has only the hosted one, add the same six-line polling helper to `Support/Waiting.swift` there.

- [ ] **Step 2: Run the core tests to verify they fail**

Run: `make -C desktop core-test`
Expected: the package does not compile (`ProgressLine`, `ToolRun` undefined), which is this step's failure.

- [ ] **Step 3: Write `ToolRun.swift`**

```swift
import Foundation

/// One `--progress json` line on a tap command's standard error
/// (internal/cli/progress.go): a step, the export engine's download, or
/// the final done line with the command's result or its error.
public enum ProgressLine: Equatable, Sendable {
    case step(phase: String, done: Int, total: Int)
    case download(bytes: Int64, totalBytes: Int64)
    case finished(ToolOutcome)

    /// nil for a line that is not progress (a warning tap prints for a person).
    public static func decode(line: String) -> ProgressLine? {
        guard line.hasPrefix("{"), let data = line.data(using: .utf8),
              let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let phase = object["phase"] as? String else { return nil }
        switch phase {
        case "download":
            guard let bytes = object["bytes"] as? NSNumber, let total = object["totalBytes"] as? NSNumber else { return nil }
            return .download(bytes: bytes.int64Value, totalBytes: total.int64Value)
        case "done":
            return ToolOutcome.decode(data).map(ProgressLine.finished)
        default:
            guard let done = object["done"] as? NSNumber, let total = object["total"] as? NSNumber else { return nil }
            return .step(phase: phase, done: done.intValue, total: total.intValue)
        }
    }
}

/// What a tap command answered: its `--json` object (the whole object, so
/// a typed decoder reads the fields it wants), or tap's error. The same
/// shape ends a `--progress json` run's done line.
public enum ToolOutcome: Equatable, Sendable {
    case ok(Data)
    case failed(code: String, message: String)

    public static func decode(_ data: Data) -> ToolOutcome? {
        guard let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any], let ok = object["ok"] as? Bool else { return nil }
        if ok { return .ok(data) }
        let error = object["error"] as? [String: Any]
        return .failed(code: error?["code"] as? String ?? "failed", message: error?["message"] as? String ?? "tap failed")
    }

    /// The result's fields, or the error as `ToolError.failed`.
    public func result<Result: Decodable>(_ type: Result.Type) throws -> Result {
        switch self {
        case .ok(let data): return try JSONDecoder().decode(type, from: data)
        case .failed(let code, let message): throw ToolError.failed(code: code, message: message)
        }
    }

    public var message: String? {
        if case .failed(_, let message) = self { return message }
        return nil
    }
}

public enum ToolError: Error, Equatable {
    /// tap answered with an error: its code and its message, to show as they are.
    case failed(code: String, message: String)
    /// tap printed no result the app can read (a crash, a kill, a timeout).
    case noResult(status: Int32)
    case cancelled
}

/// One run of a tap subcommand that ends on its own: `tap new`, `tap theme
/// set`, `tap export pdf` and the rest. Standard output is collected for
/// the `--json` result; standard error is read line by line, progress
/// lines to `onProgress` and the others to `onStandardErrorLine` (the Tap
/// Log). Standard input is closed, so a tap that would ask a question
/// cannot wait on one. `cancel` sends SIGINT, the signal Ctrl-C sends and
/// tap's export commands clean up on, then SIGTERM and SIGKILL if tap is
/// still running two seconds later each. A run past its timeout is killed
/// and reports `timedOut`. Every callback runs on the main actor, in order.
@MainActor
public final class ToolRun {
    public struct Configuration: Sendable {
        public let executableURL: URL
        public let arguments: [String]
        public let environment: [String: String]
        public let currentDirectoryURL: URL?
        /// nil for a run with no deadline (`tap serve`, stopped by `cancel`).
        public let timeout: TimeInterval?

        public init(executableURL: URL, arguments: [String], environment: [String: String], currentDirectoryURL: URL?, timeout: TimeInterval?) {
            self.executableURL = executableURL
            self.arguments = arguments
            self.environment = environment
            self.currentDirectoryURL = currentDirectoryURL
            self.timeout = timeout
        }
    }

    public struct Exit: Equatable, Sendable {
        public let status: Int32
        public let standardOutput: Data
        public let cancelled: Bool
        public let timedOut: Bool
        /// The done line's outcome, else the standard output's `--json` object, else nil.
        public let outcome: ToolOutcome?
    }

    public static let graceSeconds: TimeInterval = 2

    public var onProgress: ((ProgressLine) -> Void)?
    public var onStandardOutputLine: ((String) -> Void)?
    public var onStandardErrorLine: ((String) -> Void)?
    public var onExit: ((Exit) -> Void)?
    public private(set) var isRunning = false
    public private(set) var processIdentifier: Int32 = 0

    private let configuration: Configuration
    private let process = Process()
    private let output = Pipe()
    private let errors = Pipe()
    private var outputBuffer = LineBuffer()
    private var errorBuffer = LineBuffer()
    private var collectedOutput = Data()
    private var lastOutcome: ToolOutcome?
    private var cancelled = false
    private var timedOut = false
    private var deadline: DispatchWorkItem?

    public init(configuration: Configuration) {
        self.configuration = configuration
        signal(SIGPIPE, SIG_IGN)
    }

    public func start() throws {
        process.executableURL = configuration.executableURL
        process.arguments = configuration.arguments
        process.environment = configuration.environment
        if let directory = configuration.currentDirectoryURL { process.currentDirectoryURL = directory }
        process.standardInput = FileHandle.nullDevice
        process.standardOutput = output
        process.standardError = errors
        output.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty else { return }
            DispatchQueue.main.async { MainActor.assumeIsolated { self?.receive(data, fromStandardError: false) } }
        }
        errors.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty else { return }
            DispatchQueue.main.async { MainActor.assumeIsolated { self?.receive(data, fromStandardError: true) } }
        }
        let outputHandle = output.fileHandleForReading
        let errorHandle = errors.fileHandleForReading
        process.terminationHandler = { [weak self] finished in
            outputHandle.readabilityHandler = nil
            errorHandle.readabilityHandler = nil
            let restOfOutput = (try? outputHandle.readToEnd()) ?? Data()
            let restOfErrors = (try? errorHandle.readToEnd()) ?? Data()
            let status = finished.terminationStatus
            DispatchQueue.main.async {
                MainActor.assumeIsolated { self?.didTerminate(status: status, output: restOfOutput, errors: restOfErrors) }
            }
        }
        try process.run()
        isRunning = true
        processIdentifier = process.processIdentifier
        if let timeout = configuration.timeout {
            let work = DispatchWorkItem { [weak self] in
                MainActor.assumeIsolated {
                    guard let self, self.isRunning else { return }
                    self.timedOut = true
                    Darwin.kill(self.processIdentifier, SIGKILL)
                }
            }
            deadline = work
            DispatchQueue.main.asyncAfter(deadline: .now() + timeout, execute: work)
        }
    }

    /// Ctrl-C for tap: SIGINT now, SIGTERM and SIGKILL after a grace each
    /// if it has not exited. The exit arrives through `onExit` as usual.
    public func cancel() {
        guard isRunning, !cancelled else { return }
        cancelled = true
        let identifier = processIdentifier
        Darwin.kill(identifier, SIGINT)
        DispatchQueue.main.asyncAfter(deadline: .now() + Self.graceSeconds) { [weak self] in
            MainActor.assumeIsolated {
                guard let self, self.isRunning else { return }
                Darwin.kill(identifier, SIGTERM)
                DispatchQueue.main.asyncAfter(deadline: .now() + Self.graceSeconds) { [weak self] in
                    MainActor.assumeIsolated {
                        guard let self, self.isRunning else { return }
                        Darwin.kill(identifier, SIGKILL)
                    }
                }
            }
        }
    }

    /// Starts and waits. A start failure is an exit with status -1 and no outcome.
    public func run() async -> Exit {
        await withCheckedContinuation { continuation in
            var resumed = false
            onExit = { exit in
                guard !resumed else { return }
                resumed = true
                continuation.resume(returning: exit)
            }
            do {
                try start()
            } catch {
                resumed = true
                continuation.resume(returning: Exit(status: -1, standardOutput: Data(), cancelled: false, timedOut: false, outcome: nil))
            }
        }
    }

    private func receive(_ data: Data, fromStandardError: Bool) {
        if fromStandardError {
            for line in errorBuffer.append(data) { handleErrorLine(line) }
        } else {
            collectedOutput.append(data)
            for line in outputBuffer.append(data) { onStandardOutputLine?(line) }
        }
    }

    private func handleErrorLine(_ line: String) {
        if let progress = ProgressLine.decode(line: line) {
            if case .finished(let outcome) = progress { lastOutcome = outcome }
            onProgress?(progress)
        } else {
            onStandardErrorLine?(line)
        }
    }

    private func didTerminate(status: Int32, output restOfOutput: Data, errors restOfErrors: Data) {
        deadline?.cancel()
        receive(restOfOutput, fromStandardError: false)
        receive(restOfErrors, fromStandardError: true)
        if let line = outputBuffer.finish() { onStandardOutputLine?(line) }
        if let line = errorBuffer.finish() { handleErrorLine(line) }
        isRunning = false
        let outcome = lastOutcome ?? (timedOut ? nil : ToolOutcome.decode(collectedOutput))
        onExit?(Exit(status: status, standardOutput: collectedOutput, cancelled: cancelled, timedOut: timedOut, outcome: outcome))
    }
}
```

- [ ] **Step 4: Run the core tests**

Run: `make -C desktop core-test`
Expected: every test passes, the six new ones included. The cancel test takes under a second: the script's `trap` answers the first SIGINT.

- [ ] **Step 5: Mutate and commit**

Mutations, each applied and run with `make -C desktop core-test`, then reverted exactly: in `cancel`, send SIGTERM first instead of SIGINT (expected: `testCancelSendsSIGINTAndWaitsForTheExit` fails on status 130 and the outcome, the script's trap catches only INT); in `didTerminate`, decode the outcome from `collectedOutput` before `lastOutcome` (expected: the same test fails on `outcome`, stdout is empty); in `handleErrorLine`, send every line to `onStandardErrorLine` too (expected: `testRunsAScriptAndReportsItsProgress` fails on `otherLines`); in `ProgressLine.decode`, treat a line without `done` as `.step(..., 0, 0)` (expected: `testDecodesTheProgressLines` fails on the nil); in `start`, skip the deadline (expected: `testATimeoutKillsTheProcess` runs 30 s and fails on `timedOut`); in `cancel`, drop the `!cancelled` guard (into `survivors/` with the reason: a second SIGINT is harmless to tap, no test provokes it).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): run a tap subcommand with progress lines and a SIGINT cancel"
```

---

### Task 4: The result decoders, the theme catalog, and the references in a deck's text

**Files:**
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/ToolResults.swift`
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/ThemeCatalog.swift`
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/DeckReferences.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/ToolResultsTests.swift`, `ThemeCatalogTests.swift`, `DeckReferencesTests.swift`

**Interfaces:**
- Consumes: `ToolOutcome.result(_:)` (Task 3).
- Produces: `NewDeckResult(deck:folder:)`, `ThemeSetResult(deck:theme:)`, `AddedImageResult(deck:image:markdown:)`, `GeneratedImageResult(deck:slide:image:prompt:markdown:replaced:)`, `ComponentScaffold(files:snippet:)`, `BrokenSlide(slide:message:)`, `PDFExportResult(output:pages:bytes:brokenSlides:)`, `ImagesExportResult(files:)`, `BuildResult(output:files:bytes:)`, `ServeReady(dir:port:url:)`, `ApprovalRecord(deck:drivers:commands:approvedAt:)` with `approvedAtDate`, `deckName`, `folderPath`, `driverSummary`, `ApprovalList(approvals:)`; `ThemeSummary(slug:name:polarity:pitch:)`, `ThemeCatalog(themes:)` with `decode(_:)`, `light`, `dark`, `theme(slug:)`, `name(forSlug:)`; `ThemeImageResult(slug:image:cached:)`; `AIImageReference(prompt:imagePath:range:)` with `find(in:)`, `find(in:slideRange:)`; `ComponentLink.find(in:at:)`. Tasks 6 to 13 decode with these; Task 8 locates with them.

- [ ] **Step 1: Write the failing tests**

`ToolResultsTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

/// Each shape is the one internal/cli prints (new.go, theme_set.go,
/// image.go, component.go, export_pdf.go, export_images.go, build.go,
/// serve.go, approval_command.go), copied from a real run.
final class ToolResultsTests: XCTestCase {
    func decode<Result: Decodable>(_ type: Result.Type, _ json: String) throws -> Result {
        try XCTUnwrap(ToolOutcome.decode(Data(json.utf8))).result(type)
    }

    func testDecodesEveryCommandsResult() throws {
        XCTAssertEqual(try decode(NewDeckResult.self, #"{"ok":true,"deck":"/t/my-talk/my-talk.md","folder":"/t/my-talk"}"#),
                       NewDeckResult(deck: "/t/my-talk/my-talk.md", folder: "/t/my-talk"))
        XCTAssertEqual(try decode(ThemeSetResult.self, #"{"ok":true,"deck":"/t/talk.md","theme":"terminal"}"#), ThemeSetResult(deck: "/t/talk.md", theme: "terminal"))
        XCTAssertEqual(try decode(AddedImageResult.self, #"{"ok":true,"deck":"/t/talk.md","image":"images/diagram-2.png","markdown":"![diagram](images/diagram-2.png)"}"#),
                       AddedImageResult(deck: "/t/talk.md", image: "images/diagram-2.png", markdown: "![diagram](images/diagram-2.png)"))
        XCTAssertEqual(try decode(GeneratedImageResult.self, #"{"ok":true,"deck":"/t/talk.md","slide":3,"image":"images/generated-1a2b3c4d.png","prompt":"a fox","markdown":"<!-- ai-prompt: a fox -->\n![](images/generated-1a2b3c4d.png)","replaced":"images/generated-00000000.png"}"#),
                       GeneratedImageResult(deck: "/t/talk.md", slide: 3, image: "images/generated-1a2b3c4d.png", prompt: "a fox", markdown: "<!-- ai-prompt: a fox -->\n![](images/generated-1a2b3c4d.png)", replaced: "images/generated-00000000.png"))
        XCTAssertNil(try decode(GeneratedImageResult.self, #"{"ok":true,"deck":"/t/talk.md","slide":3,"image":"i.png","prompt":"p","markdown":"m"}"#).replaced, "generate has no replaced")
        XCTAssertEqual(try decode(ComponentScaffold.self, #"{"ok":true,"files":["/t/slides/Counter.jsx"],"snippet":"<!--\nlayout: ./slides/Counter.jsx\n-->\n\n# Title\n"}"#),
                       ComponentScaffold(files: ["/t/slides/Counter.jsx"], snippet: "<!--\nlayout: ./slides/Counter.jsx\n-->\n\n# Title\n"))
        let pdf = try decode(PDFExportResult.self, #"{"phase":"done","ok":true,"output":"/t/talk.pdf","pages":14,"bytes":120000,"brokenSlides":[{"slide":2,"message":"boom"}]}"#)
        XCTAssertEqual(pdf, PDFExportResult(output: "/t/talk.pdf", pages: 14, bytes: 120_000, brokenSlides: [BrokenSlide(slide: 2, message: "boom")]))
        XCTAssertEqual(try decode(ImagesExportResult.self, #"{"phase":"done","ok":true,"files":["/t/out/slide-001.png","/t/out/slide-003.png"]}"#).files.count, 2)
        XCTAssertEqual(try decode(BuildResult.self, #"{"phase":"done","ok":true,"output":"/t/dist","files":38,"bytes":4300000}"#), BuildResult(output: "/t/dist", files: 38, bytes: 4_300_000))
        XCTAssertEqual(try decode(ServeReady.self, #"{"ok":true,"dir":"/t/dist","port":54321,"url":"http://localhost:54321"}"#), ServeReady(dir: "/t/dist", port: 54321, url: "http://localhost:54321"))
    }

    func testDecodesTheApprovalList() throws {
        let list = try decode(ApprovalList.self, #"{"ok":true,"approvals":[{"deck":"/Users/me/talks/3am/conference-talk.md","drivers":["sqlite","shell"],"approvedAt":"2026-09-25T19:32:00.123456Z"},{"deck":"/Users/me/talks/k8s/k8s-workshop.md","drivers":["shell","kubectl"],"commands":{"kubectl":["kubectl","--token","${KUBE_TOKEN}"]},"approvedAt":"2026-08-30T10:00:00Z"}]}"#)
        XCTAssertEqual(list.approvals.count, 2)
        let first = list.approvals[0]
        XCTAssertEqual(first.deckName, "conference-talk.md")
        XCTAssertEqual(first.folderPath, "/Users/me/talks/3am")
        XCTAssertEqual(first.driverSummary, "sqlite, shell")
        XCTAssertNotNil(first.approvedAtDate, "fractional seconds parse")
        let second = list.approvals[1]
        XCTAssertEqual(second.driverSummary, "shell, kubectl (custom)")
        XCTAssertEqual(second.commands?["kubectl"], ["kubectl", "--token", "${KUBE_TOKEN}"], "shown as tap masked it, never expanded here")
        XCTAssertNotNil(second.approvedAtDate)
        XCTAssertEqual(try decode(ApprovalList.self, #"{"ok":true,"approvals":[]}"#).approvals, [])
    }

    func testAFailureIsAnError() {
        let outcome = ToolOutcome.decode(Data(#"{"ok":false,"error":{"code":"no_api_key","message":"cannot start image generation: GEMINI_API_KEY is not set"}}"#.utf8))
        XCTAssertThrowsError(try outcome?.result(GeneratedImageResult.self)) { error in
            XCTAssertEqual(error as? ToolError, .failed(code: "no_api_key", message: "cannot start image generation: GEMINI_API_KEY is not set"))
        }
    }
}
```

`ThemeCatalogTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class ThemeCatalogTests: XCTestCase {
    let json = #"{"ok":true,"themes":[{"slug":"base","name":"Base","polarity":"light","pitch":"Clean and quiet."},{"slug":"terminal","name":"Terminal","polarity":"dark","pitch":"A shell at 3am."},{"slug":"product","name":"Product","polarity":"light","pitch":"A launch."},{"slug":"blueprint","name":"Blueprint","polarity":"dark","pitch":"Drafting paper."}]}"#

    func testGroupsThemesByPolarityInTapsOrder() throws {
        let catalog = try ThemeCatalog.decode(Data(json.utf8))
        XCTAssertEqual(catalog.themes.map(\.slug), ["base", "terminal", "product", "blueprint"])
        XCTAssertEqual(catalog.light.map(\.slug), ["base", "product"])
        XCTAssertEqual(catalog.dark.map(\.slug), ["terminal", "blueprint"])
        XCTAssertEqual(catalog.theme(slug: "terminal")?.name, "Terminal")
        XCTAssertEqual(catalog.name(forSlug: "nope"), "nope", "an unknown slug is shown as written")
        XCTAssertNil(catalog.theme(slug: "nope"))
    }

    func testDecodesAnImageResult() throws {
        let result = try ThemeImageResult.decode(Data(#"{"ok":true,"slug":"terminal","image":"/Users/me/Library/Caches/tap/themes/2.1.0/terminal.png","cached":true}"#.utf8))
        XCTAssertEqual(result, ThemeImageResult(slug: "terminal", image: "/Users/me/Library/Caches/tap/themes/2.1.0/terminal.png", cached: true))
    }

    func testRefusesAFailure() {
        XCTAssertThrowsError(try ThemeCatalog.decode(Data(#"{"ok":false,"error":{"code":"internal","message":"x"}}"#.utf8)))
        XCTAssertThrowsError(try ThemeCatalog.decode(Data("SLUG NAME\n".utf8)))
    }
}
```

`DeckReferencesTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class DeckReferencesTests: XCTestCase {
    let deck = """
    # One

    <!-- ai-prompt: a fox at dusk -->
    ![](images/generated-1a2b3c4d.png)

    ---

    # Two

    <!--   ai-prompt:   a whale   -->
      ![](./images/generated-ffffffff.png)

    ![diagram](images/diagram.png)
    """

    func testFindsEveryAIImagePairAsTapDoes() {
        let found = AIImageReference.find(in: deck)
        XCTAssertEqual(found.map(\.prompt), ["a fox at dusk", "a whale"])
        XCTAssertEqual(found.map(\.imagePath), ["images/generated-1a2b3c4d.png", "./images/generated-ffffffff.png"])
        XCTAssertEqual((deck as NSString).substring(with: found[0].range), "<!-- ai-prompt: a fox at dusk -->\n![](images/generated-1a2b3c4d.png)")
        XCTAssertEqual(AIImageReference.find(in: "![](images/plain.png)\n"), [], "a plain image is not an AI image")
        XCTAssertEqual(AIImageReference.find(in: "<!-- ai-prompt: x -->\n\n![](a.png)"), [], "the link must be on the next line, as tap requires")
    }

    func testFindsThePairsInsideASlidesRange() {
        let text = deck as NSString
        let slideTwo = NSRange(location: text.range(of: "# Two").location, length: text.length - text.range(of: "# Two").location)
        XCTAssertEqual(AIImageReference.find(in: deck, slideRange: slideTwo).map(\.prompt), ["a whale"])
    }

    func testFindsAComponentPathUnderTheCaret() {
        let line = "layout: ./slides/RollingDeploy.jsx"
        XCTAssertEqual(ComponentLink.find(in: line, at: 12), "./slides/RollingDeploy.jsx")
        XCTAssertEqual(ComponentLink.find(in: line, at: 33), "./slides/RollingDeploy.jsx", "the last character counts")
        XCTAssertNil(ComponentLink.find(in: line, at: 3), "a click on the key is not on the path")
        XCTAssertEqual(ComponentLink.find(in: "```component ./components/LatencyDrop.tsx", at: 20), "./components/LatencyDrop.tsx")
        XCTAssertEqual(ComponentLink.find(in: "<!-- layout: slides/Chart.jsx -->", at: 16), "slides/Chart.jsx", "without ./ too")
        XCTAssertNil(ComponentLink.find(in: "See ./notes/plan.md", at: 6), "only component files")
        XCTAssertNil(ComponentLink.find(in: "", at: 0))
    }
}
```

- [ ] **Step 2: Run the core tests to verify they fail**

Run: `make -C desktop core-test`
Expected: the package does not compile (the types are undefined).

- [ ] **Step 3: Write the three files**

`ToolResults.swift`:

```swift
import Foundation

/// The `--json` results of the tap commands the app runs, one struct per
/// command, with the field names internal/cli prints. A done line of a
/// `--progress json` run carries the same fields plus `phase`, which these
/// ignore.

public struct NewDeckResult: Decodable, Equatable, Sendable {
    public let deck: String
    public let folder: String
    public init(deck: String, folder: String) { self.deck = deck; self.folder = folder }
}

public struct ThemeSetResult: Decodable, Equatable, Sendable {
    public let deck: String
    public let theme: String
    public init(deck: String, theme: String) { self.deck = deck; self.theme = theme }
}

public struct AddedImageResult: Decodable, Equatable, Sendable {
    public let deck: String
    public let image: String
    public let markdown: String
    public init(deck: String, image: String, markdown: String) { self.deck = deck; self.image = image; self.markdown = markdown }
}

public struct GeneratedImageResult: Decodable, Equatable, Sendable {
    public let deck: String
    public let slide: Int
    public let image: String
    public let prompt: String
    public let markdown: String
    /// The image `tap image regenerate` replaced; nil for `generate`.
    public let replaced: String?
    public init(deck: String, slide: Int, image: String, prompt: String, markdown: String, replaced: String? = nil) {
        self.deck = deck; self.slide = slide; self.image = image; self.prompt = prompt; self.markdown = markdown; self.replaced = replaced
    }
}

public struct ComponentScaffold: Decodable, Equatable, Sendable {
    /// The files tap wrote; the first is the component itself.
    public let files: [String]
    public let snippet: String
    public init(files: [String], snippet: String) { self.files = files; self.snippet = snippet }
}

public struct BrokenSlide: Decodable, Equatable, Sendable {
    public let slide: Int
    public let message: String
    public init(slide: Int, message: String) { self.slide = slide; self.message = message }
}

public struct PDFExportResult: Decodable, Equatable, Sendable {
    public let output: String
    public let pages: Int
    public let bytes: Int64
    public let brokenSlides: [BrokenSlide]
    public init(output: String, pages: Int, bytes: Int64, brokenSlides: [BrokenSlide]) {
        self.output = output; self.pages = pages; self.bytes = bytes; self.brokenSlides = brokenSlides
    }
}

public struct ImagesExportResult: Decodable, Equatable, Sendable {
    public let files: [String]
    public init(files: [String]) { self.files = files }
}

public struct BuildResult: Decodable, Equatable, Sendable {
    public let output: String
    public let files: Int
    public let bytes: Int64
    public init(output: String, files: Int, bytes: Int64) { self.output = output; self.files = files; self.bytes = bytes }
}

public struct ServeReady: Decodable, Equatable, Sendable {
    public let dir: String
    public let port: Int
    public let url: String
    public init(dir: String, port: Int, url: String) { self.dir = dir; self.port = port; self.url = url }
}

/// One deck `tap approval list --json` lists. `commands` is a custom
/// driver's command as tap shows it, secrets already masked by tap; the
/// app shows these strings and expands nothing.
public struct ApprovalRecord: Decodable, Equatable, Sendable {
    public let deck: String
    public let drivers: [String]
    public let commands: [String: [String]]?
    public let approvedAt: String

    public init(deck: String, drivers: [String], commands: [String: [String]]? = nil, approvedAt: String) {
        self.deck = deck; self.drivers = drivers; self.commands = commands; self.approvedAt = approvedAt
    }

    public var deckName: String { (deck as NSString).lastPathComponent }
    public var folderPath: String { (deck as NSString).deletingLastPathComponent }
    /// "shell, kubectl (custom)": a driver with a command of its own is custom.
    public var driverSummary: String {
        drivers.map { name in commands?[name] != nil ? "\(name) (custom)" : name }.joined(separator: ", ")
    }
    /// tap writes RFC 3339 with or without fractional seconds.
    public var approvedAtDate: Date? {
        let withFraction = ISO8601DateFormatter()
        withFraction.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return withFraction.date(from: approvedAt) ?? ISO8601DateFormatter().date(from: approvedAt)
    }
}

public struct ApprovalList: Decodable, Equatable, Sendable {
    public let approvals: [ApprovalRecord]
    public init(approvals: [ApprovalRecord]) { self.approvals = approvals }
}
```

`ThemeCatalog.swift`:

```swift
import Foundation

/// One built-in theme as `tap theme list --json` lists it (internal/themes).
public struct ThemeSummary: Codable, Equatable, Sendable {
    public let slug: String
    public let name: String
    /// "light" or "dark".
    public let polarity: String
    public let pitch: String

    public init(slug: String, name: String, polarity: String, pitch: String) {
        self.slug = slug; self.name = name; self.polarity = polarity; self.pitch = pitch
    }
}

/// Every theme tap offers, in tap's order, for the grid's two groups.
public struct ThemeCatalog: Equatable, Sendable {
    public let themes: [ThemeSummary]

    public init(themes: [ThemeSummary]) { self.themes = themes }

    private struct Envelope: Decodable { let themes: [ThemeSummary] }

    public static func decode(_ data: Data) throws -> ThemeCatalog {
        guard let outcome = ToolOutcome.decode(data) else { throw ToolError.noResult(status: 0) }
        return ThemeCatalog(themes: try outcome.result(Envelope.self).themes)
    }

    public var light: [ThemeSummary] { themes.filter { $0.polarity != "dark" } }
    public var dark: [ThemeSummary] { themes.filter { $0.polarity == "dark" } }

    public func theme(slug: String) -> ThemeSummary? { themes.first { $0.slug == slug } }

    /// The theme's name for a label, or the slug itself when tap does not know it.
    public func name(forSlug slug: String) -> String { theme(slug: slug)?.name ?? slug }
}

/// The result of `tap theme show <slug> --image --json`: the PNG's path.
public struct ThemeImageResult: Decodable, Equatable, Sendable {
    public let slug: String
    public let image: String
    public let cached: Bool

    public init(slug: String, image: String, cached: Bool) { self.slug = slug; self.image = image; self.cached = cached }

    public static func decode(_ data: Data) throws -> ThemeImageResult {
        guard let outcome = ToolOutcome.decode(data) else { throw ToolError.noResult(status: 0) }
        return try outcome.result(ThemeImageResult.self)
    }
}
```

`DeckReferences.swift`:

```swift
import Foundation

/// An AI-generated image in a deck's text: the `ai-prompt` comment and,
/// on the next line, the image it produced, the pair tap's
/// internal/deckedit writes and reads (its aiImagePattern). The app finds
/// them to name a Regenerate menu item; tap edits them.
public struct AIImageReference: Equatable, Sendable {
    public let prompt: String
    public let imagePath: String
    /// The two lines, from the comment's `<` to the link's `)`.
    public let range: NSRange

    private static let pattern = try! NSRegularExpression(pattern: #"<!--\s*ai-prompt:\s*(.+?)\s*-->\n[ \t]*!\[\]\(([^)]+)\)"#)

    public static func find(in text: String) -> [AIImageReference] {
        let whole = text as NSString
        return pattern.matches(in: text, range: NSRange(location: 0, length: whole.length)).map { match in
            AIImageReference(prompt: whole.substring(with: match.range(at: 1)), imagePath: whole.substring(with: match.range(at: 2)), range: match.range)
        }
    }

    /// The pairs that start inside `slideRange` (a slide's lines from tap).
    public static func find(in text: String, slideRange: NSRange) -> [AIImageReference] {
        find(in: text).filter { NSLocationInRange($0.range.location, slideRange) }
    }
}

/// A deck-supplied component's path in a line of the deck, as tap's
/// snippets write it (`layout: ./slides/Name.jsx`, a
/// ```component ./components/Name.jsx fence).
public enum ComponentLink {
    private static let pattern = try! NSRegularExpression(pattern: #"(?:\./)?(?:slides|components)/[A-Za-z0-9_./-]+\.(?:jsx|tsx)"#)

    /// The path under `column` in `line`, or nil when the column is not on one.
    public static func find(in line: String, at column: Int) -> String? {
        let whole = line as NSString
        guard column >= 0, column < whole.length else { return nil }
        for match in pattern.matches(in: line, range: NSRange(location: 0, length: whole.length)) where NSLocationInRange(column, match.range) {
            return whole.substring(with: match.range)
        }
        return nil
    }
}
```

- [ ] **Step 4: Run the core tests**

Run: `make -C desktop core-test`
Expected: every test passes.

- [ ] **Step 5: Mutate and commit**

Mutations, each applied and run with `make -C desktop core-test`, then reverted exactly: in `driverSummary`, mark every driver `(custom)` (expected: `testDecodesTheApprovalList` fails on the first summary); in `approvedAtDate`, drop the fractional-seconds formatter (expected: fails on "fractional seconds parse"); in `ThemeCatalog.light`, filter on `== "light"` and in `dark` on `!= "light"` (into `survivors/`: tap prints only the two values; no test has a third); in `AIImageReference.pattern`, allow `\n+` between the comment and the link (expected: `testFindsEveryAIImagePairAsTapDoes` fails on the blank-line case); in `ComponentLink.find`, use `column <= match.range.location` instead of `NSLocationInRange` (expected: `testFindsAComponentPathUnderTheCaret` fails at column 12); in `ComponentLink.pattern`, accept `md` (expected: fails on the notes link).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): decode tap's results, the theme catalog, and the AI image and component references"
```

---

### Task 5: `GeneralSettings`, the Keychain key store, and the command line tool's PATH logic

**Files:**
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/GeneralSettings.swift`
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/GeminiKeyStore.swift`
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/CommandLineTool.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/GeneralSettingsTests.swift`, `GeminiKeyStoreTests.swift`, `CommandLineToolTests.swift`

**Interfaces:**
- Consumes: `UserDefaults`, the Security framework.
- Produces: `GeneralSettings(defaults:)` with `fontSize: CGFloat` (13), `lineSpacing: LineSpacing` (`.tight`, `.normal`, `.roomy`; `lineHeight(forFontSize:)`), `defaultTheme: String?`, `autosaveDelay: TimeInterval` (1), `lastNewDeckFolder: URL?`, `static didChangeNotification`, `static fontSizes`, `static autosaveDelays`; `GeminiKeyStore` protocol (`read() throws -> String?`, `write(_:) throws`), `KeychainGeminiKeyStore(service:account:)`, `KeychainError`, `GeminiKeySource` (`.shell`, `.keychain`, `.none`) with `resolve(shellValue:storedKey:)`; `CommandLineTool.locate(named:onPath:fileExists:)`, `isBundledLink(destination:)`, `ExistingFile`, `installDecision(for:)`, `InstallDecision`, `directoryComesFirst(_:beforeDirectoryOf:onPath:)`. Task 12's General pane, Task 13's two panes, Task 6's editor typography and Task 7's sheet read these.

- [ ] **Step 1: Write the failing tests**

`GeneralSettingsTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class GeneralSettingsTests: XCTestCase {
    func fresh() throws -> GeneralSettings {
        let name = "TapDesktopCoreTests.general.\(UUID().uuidString)"
        let defaults = try XCTUnwrap(UserDefaults(suiteName: name))
        addTeardownBlock { defaults.removePersistentDomain(forName: name) }
        return GeneralSettings(defaults: defaults)
    }

    func testDefaultsAreTheSpecs() throws {
        let settings = try fresh()
        XCTAssertEqual(settings.fontSize, 13)
        XCTAssertEqual(settings.lineSpacing, .normal)
        XCTAssertNil(settings.defaultTheme, "no default theme means tap new's own default")
        XCTAssertEqual(settings.autosaveDelay, 1, "the spec's one second")
        XCTAssertNil(settings.lastNewDeckFolder)
    }

    func testChangesPersistAndPostANotification() throws {
        let settings = try fresh()
        var posted = 0
        let observer = NotificationCenter.default.addObserver(forName: GeneralSettings.didChangeNotification, object: settings, queue: nil) { _ in posted += 1 }
        defer { NotificationCenter.default.removeObserver(observer) }
        settings.fontSize = 15
        settings.lineSpacing = .roomy
        settings.defaultTheme = "terminal"
        settings.autosaveDelay = 5
        settings.lastNewDeckFolder = URL(fileURLWithPath: "/tmp/talks")
        XCTAssertEqual(posted, 5)
        let again = GeneralSettings(defaults: settings.defaults)
        XCTAssertEqual(again.fontSize, 15)
        XCTAssertEqual(again.lineSpacing, .roomy)
        XCTAssertEqual(again.defaultTheme, "terminal")
        XCTAssertEqual(again.autosaveDelay, 5)
        XCTAssertEqual(again.lastNewDeckFolder?.path, "/tmp/talks")
        settings.defaultTheme = nil
        XCTAssertNil(GeneralSettings(defaults: settings.defaults).defaultTheme)
    }

    func testAValueOutsideTheChoicesReadsAsTheDefault() throws {
        let settings = try fresh()
        settings.defaults.set(3, forKey: "TapEditorFontSize")
        settings.defaults.set("huge", forKey: "TapEditorLineSpacing")
        settings.defaults.set(-2.0, forKey: "TapAutosaveDelay")
        XCTAssertEqual(settings.fontSize, 13)
        XCTAssertEqual(settings.lineSpacing, .normal)
        XCTAssertEqual(settings.autosaveDelay, 1)
    }

    func testLineHeightsScaleWithTheFont() {
        XCTAssertEqual(GeneralSettings.LineSpacing.normal.lineHeight(forFontSize: 13), 21, "D2's editor line height at the default")
        XCTAssertEqual(GeneralSettings.LineSpacing.tight.lineHeight(forFontSize: 13), 18)
        XCTAssertEqual(GeneralSettings.LineSpacing.roomy.lineHeight(forFontSize: 13), 25)
        XCTAssertGreaterThan(GeneralSettings.LineSpacing.normal.lineHeight(forFontSize: 16), 21)
        XCTAssertEqual(GeneralSettings.fontSizes, [11, 12, 13, 14, 15, 16, 18])
        XCTAssertEqual(GeneralSettings.autosaveDelays, [0.5, 1, 2, 5, 10])
    }
}
```

`GeminiKeyStoreTests.swift` (the Keychain itself is never touched by a test; the protocol and the source rule are):

```swift
import XCTest
@testable import TapDesktopCore

final class MemoryGeminiKeyStore: GeminiKeyStore {
    var key: String?
    var writes = 0
    init(key: String? = nil) { self.key = key }
    func read() throws -> String? { key }
    func write(_ key: String?) throws { self.key = key; writes += 1 }
}

final class GeminiKeyStoreTests: XCTestCase {
    func testTheShellsKeyWinsOverTheKeychains() {
        XCTAssertEqual(GeminiKeySource.resolve(shellValue: "from-shell", storedKey: "from-keychain"), .shell)
        XCTAssertEqual(GeminiKeySource.resolve(shellValue: nil, storedKey: "from-keychain"), .keychain)
        XCTAssertEqual(GeminiKeySource.resolve(shellValue: "", storedKey: "from-keychain"), .keychain, "an empty shell value is no key")
        XCTAssertEqual(GeminiKeySource.resolve(shellValue: nil, storedKey: ""), .none, "an empty stored key is none")
        XCTAssertEqual(GeminiKeySource.resolve(shellValue: nil, storedKey: nil), .none)
    }

    func testTheEnvironmentGetsTheKeyOnlyWhenTheShellHasNone() throws {
        let store = MemoryGeminiKeyStore(key: "placeholder-not-a-secret")
        var environment = ["PATH": "/usr/bin"]
        try GeminiKeySource.apply(store: store, to: &environment)
        XCTAssertEqual(environment["GEMINI_API_KEY"], "placeholder-not-a-secret")
        var shell = ["GEMINI_API_KEY": "shell-placeholder"]
        try GeminiKeySource.apply(store: store, to: &shell)
        XCTAssertEqual(shell["GEMINI_API_KEY"], "shell-placeholder", "the shell's value is kept")
        var empty = ["PATH": "/usr/bin"]
        try GeminiKeySource.apply(store: MemoryGeminiKeyStore(), to: &empty)
        XCTAssertNil(empty["GEMINI_API_KEY"], "no key sets nothing, so tap's own no_api_key message is what the person sees")
    }

    func testTheKeychainStoreNamesItsItem() {
        let store = KeychainGeminiKeyStore(service: "io.geocod.tap.desktop.tests", account: "GEMINI_API_KEY")
        XCTAssertEqual(store.service, "io.geocod.tap.desktop.tests")
        XCTAssertEqual(store.account, "GEMINI_API_KEY")
        XCTAssertEqual(KeychainGeminiKeyStore().service, "io.geocod.tap.desktop")
    }
}
```

`CommandLineToolTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class CommandLineToolTests: XCTestCase {
    func testLocatesEveryTapOnPathInOrder() {
        let existing: Set<String> = ["/opt/homebrew/bin/tap", "/Users/me/.local/bin/tap"]
        let found = CommandLineTool.locate(named: "tap", onPath: "/Users/me/.local/bin:/opt/homebrew/bin:/usr/bin:/bin", fileExists: { existing.contains($0) })
        XCTAssertEqual(found, ["/Users/me/.local/bin/tap", "/opt/homebrew/bin/tap"])
        XCTAssertEqual(CommandLineTool.locate(named: "tap", onPath: "", fileExists: { _ in true }), [])
        XCTAssertEqual(CommandLineTool.locate(named: "tap", onPath: "/a::/b", fileExists: { _ in true }), ["/a/tap", "/b/tap"], "an empty PATH entry is skipped")
    }

    func testKnowsItsOwnLink() {
        XCTAssertTrue(CommandLineTool.isBundledLink(destination: "/Applications/Tap.app/Contents/Resources/tap"))
        XCTAssertTrue(CommandLineTool.isBundledLink(destination: "/Users/me/Desktop/Tap 2.app/Contents/Resources/tap"))
        XCTAssertFalse(CommandLineTool.isBundledLink(destination: "/opt/homebrew/Cellar/tap/2.0.0/bin/tap"))
        XCTAssertFalse(CommandLineTool.isBundledLink(destination: nil), "a plain file has no destination")
    }

    func testTheInstallDecisionNeverTouchesWhatIsNotOurs() {
        XCTAssertEqual(CommandLineTool.installDecision(for: .none), .link)
        XCTAssertEqual(CommandLineTool.installDecision(for: .bundledLink), .replaceOwnLink, "our link to another copy of the app is ours to move")
        XCTAssertEqual(CommandLineTool.installDecision(for: .other), .refuse("a tap that Tap did not install is already there"))
    }

    func testPathOrderDecidesTheHint() {
        let path = "/Users/me/.local/bin:/opt/homebrew/bin:/usr/bin"
        XCTAssertEqual(CommandLineTool.directoryComesFirst("/Users/me/.local/bin", beforeDirectoryOf: "/opt/homebrew/bin/tap", onPath: path), true)
        XCTAssertEqual(CommandLineTool.directoryComesFirst("/Users/me/.local/bin", beforeDirectoryOf: "/opt/homebrew/bin/tap", onPath: "/opt/homebrew/bin:/Users/me/.local/bin"), false)
        XCTAssertNil(CommandLineTool.directoryComesFirst("/Users/me/.local/bin", beforeDirectoryOf: "/opt/homebrew/bin/tap", onPath: "/usr/bin"), "not on PATH at all")
        XCTAssertEqual(CommandLineTool.directoryComesFirst("/Users/me/.local/bin", beforeDirectoryOf: nil, onPath: path), true, "no other tap: first by default")
    }
}
```

- [ ] **Step 2: Run the core tests to verify they fail**

Run: `make -C desktop core-test`
Expected: the package does not compile.

- [ ] **Step 3: Write the three files**

`GeneralSettings.swift`:

```swift
import Foundation

/// The General pane's settings, in UserDefaults: the editor's font size
/// and line spacing, the theme New Deck preselects (passed to tap new
/// --theme; nil leaves tap's own default), the autosave delay, and the
/// folder New Deck last saved into. A test uses a suite of its own.
public final class GeneralSettings {
    public static let didChangeNotification = Notification.Name("TapGeneralSettingsDidChange")
    public static let fontSizes: [CGFloat] = [11, 12, 13, 14, 15, 16, 18]
    public static let autosaveDelays: [TimeInterval] = [0.5, 1, 2, 5, 10]

    public enum LineSpacing: String, CaseIterable, Sendable {
        case tight, normal, roomy

        /// The editor's line height: D2's 21 points at 13 points normal.
        public func lineHeight(forFontSize fontSize: CGFloat) -> CGFloat {
            let factor: CGFloat
            switch self {
            case .tight: factor = 1.38
            case .normal: factor = 1.62
            case .roomy: factor = 1.92
            }
            return (fontSize * factor).rounded()
        }
    }

    public let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public var fontSize: CGFloat {
        get {
            let stored = CGFloat(defaults.double(forKey: "TapEditorFontSize"))
            return Self.fontSizes.contains(stored) ? stored : 13
        }
        set { set(Double(newValue), forKey: "TapEditorFontSize") }
    }

    public var lineSpacing: LineSpacing {
        get { defaults.string(forKey: "TapEditorLineSpacing").flatMap(LineSpacing.init(rawValue:)) ?? .normal }
        set { set(newValue.rawValue, forKey: "TapEditorLineSpacing") }
    }

    public var defaultTheme: String? {
        get { defaults.string(forKey: "TapDefaultTheme") }
        set { set(newValue, forKey: "TapDefaultTheme") }
    }

    public var autosaveDelay: TimeInterval {
        get {
            let stored = defaults.double(forKey: "TapAutosaveDelay")
            return Self.autosaveDelays.contains(stored) ? stored : 1
        }
        set { set(newValue, forKey: "TapAutosaveDelay") }
    }

    public var lastNewDeckFolder: URL? {
        get { defaults.string(forKey: "TapLastNewDeckFolder").map { URL(fileURLWithPath: $0) } }
        set { set(newValue?.path, forKey: "TapLastNewDeckFolder") }
    }

    private func set(_ value: Any?, forKey key: String) {
        if let value { defaults.set(value, forKey: key) } else { defaults.removeObject(forKey: key) }
        NotificationCenter.default.post(name: Self.didChangeNotification, object: self)
    }
}
```

`GeminiKeyStore.swift`:

```swift
import Foundation
import Security

/// Where the Gemini key lives. The app reads it in two places only: to
/// put GEMINI_API_KEY into tap's environment, and to fill the Image
/// Generation pane's secure field. The value is never logged or shown.
public protocol GeminiKeyStore: AnyObject {
    func read() throws -> String?
    /// nil removes the key.
    func write(_ key: String?) throws
}

public enum KeychainError: Error, Equatable {
    case status(OSStatus)
}

/// The key as a generic password item in the login Keychain.
public final class KeychainGeminiKeyStore: GeminiKeyStore {
    public let service: String
    public let account: String

    public init(service: String = "io.geocod.tap.desktop", account: String = "GEMINI_API_KEY") {
        self.service = service
        self.account = account
    }

    private var query: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service, kSecAttrAccount as String: account]
    }

    public func read() throws -> String? {
        var query = query
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess else { throw KeychainError.status(status) }
        guard let data = item as? Data else { return nil }
        return String(decoding: data, as: UTF8.self)
    }

    public func write(_ key: String?) throws {
        let deleted = SecItemDelete(query as CFDictionary)
        guard deleted == errSecSuccess || deleted == errSecItemNotFound else { throw KeychainError.status(deleted) }
        guard let key, !key.isEmpty else { return }
        var item = query
        item[kSecValueData as String] = Data(key.utf8)
        let added = SecItemAdd(item as CFDictionary, nil)
        guard added == errSecSuccess else { throw KeychainError.status(added) }
    }
}

/// Which key tap gets: the login shell's GEMINI_API_KEY wins, then the
/// Keychain's, else none (tap's own no_api_key message then says so).
public enum GeminiKeySource: Equatable, Sendable {
    case shell
    case keychain
    case none

    public static func resolve(shellValue: String?, storedKey: String?) -> GeminiKeySource {
        if let shellValue, !shellValue.isEmpty { return .shell }
        if let storedKey, !storedKey.isEmpty { return .keychain }
        return .none
    }

    /// Adds the Keychain's key to `environment` when the shell gave none.
    public static func apply(store: GeminiKeyStore, to environment: inout [String: String]) throws {
        if let shellValue = environment["GEMINI_API_KEY"], !shellValue.isEmpty { return }
        if let key = try store.read(), !key.isEmpty { environment["GEMINI_API_KEY"] = key }
    }
}
```

`CommandLineTool.swift`:

```swift
import Foundation

/// The rules behind Settings > Command Line: where a `tap` is on the
/// login shell's PATH, whether a file is the app's own link, and whether
/// Install may touch what is at ~/.local/bin/tap. Pure functions; the
/// installer and the pane give them the file system.
public enum CommandLineTool {
    /// Every `<directory>/<name>` on `path` that exists, in PATH order.
    public static func locate(named name: String, onPath path: String, fileExists: (String) -> Bool) -> [String] {
        path.split(separator: ":", omittingEmptySubsequences: true).map { "\($0)/\(name)" }.filter(fileExists)
    }

    /// A symlink whose destination is a Tap.app's bundled tap is ours to
    /// replace; anything else was installed by someone else.
    public static func isBundledLink(destination: String?) -> Bool {
        guard let destination else { return false }
        return destination.hasSuffix(".app/Contents/Resources/tap") && (destination as NSString).lastPathComponent == "tap"
            && ((destination as NSString).deletingLastPathComponent as NSString).deletingLastPathComponent.hasSuffix("/Contents")
            && destination.contains("Tap")
    }

    public enum ExistingFile: Equatable, Sendable {
        case none
        case bundledLink
        case other
    }

    public enum InstallDecision: Equatable, Sendable {
        case link
        case replaceOwnLink
        case refuse(String)
    }

    public static func installDecision(for existing: ExistingFile) -> InstallDecision {
        switch existing {
        case .none: return .link
        case .bundledLink: return .replaceOwnLink
        case .other: return .refuse("a tap that Tap did not install is already there")
        }
    }

    /// Whether `directory` comes before the folder of `otherExecutable` on
    /// `path`: true with no other tap, nil when `directory` is not on PATH.
    public static func directoryComesFirst(_ directory: String, beforeDirectoryOf otherExecutable: String?, onPath path: String) -> Bool? {
        let entries = path.split(separator: ":", omittingEmptySubsequences: true).map(String.init)
        guard let own = entries.firstIndex(of: directory) else { return nil }
        guard let otherExecutable else { return true }
        let otherDirectory = (otherExecutable as NSString).deletingLastPathComponent
        guard let other = entries.firstIndex(of: otherDirectory) else { return true }
        return own < other
    }
}
```

- [ ] **Step 4: Run the core tests**

Run: `make -C desktop core-test`
Expected: every test passes. Nothing here opened the Keychain: `KeychainGeminiKeyStore` is constructed but neither `read` nor `write` runs in a test.

- [ ] **Step 5: Mutate and commit**

Mutations, each applied and run with `make -C desktop core-test`, then reverted exactly: in `GeminiKeySource.apply`, set the store's key even when the shell has one (expected: `testTheEnvironmentGetsTheKeyOnlyWhenTheShellHasNone` fails on "the shell's value is kept"); in `resolve`, return `.keychain` for an empty stored key (expected: `testTheShellsKeyWinsOverTheKeychains` fails); in `installDecision`, return `.link` for `.other` (expected: `testTheInstallDecisionNeverTouchesWhatIsNotOurs` fails, the case that would overwrite a person's tap); in `isBundledLink`, drop the `.app/Contents/Resources/tap` suffix check (expected: `testKnowsItsOwnLink` fails on the Cellar path); in `directoryComesFirst`, compare `own <= other` (into `survivors/`: the same directory cannot hold both, no test has it); in `fontSize`'s getter, return `stored` unchecked (expected: `testAValueOutsideTheChoicesReadsAsTheDefault` fails); in `LineSpacing.normal`, use factor 1.5 (expected: `testLineHeightsScaleWithTheFont` fails on 21).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): the General settings store, the Keychain key store, and the command line tool's PATH rules"
```

---

### Task 6: `TapTool`, the theme grid, the toolbar's Theme item, and setting the theme

**Files:**
- Create: `desktop/Tap/App/TapTool.swift`
- Modify: `desktop/Tap/App/AppEnvironment.swift` (`toolExecutableURL`, `geminiKeyStore`, `generalSettings`, `themeImages`, `tapEnvironment()`)
- Create: `desktop/Tap/Themes/ThemeImageLoader.swift`, `desktop/Tap/Themes/ThemeGridViewController.swift`, `desktop/Tap/Themes/ThemePopoverController.swift`
- Modify: `desktop/Tap/Documents/DeckSessionController.swift` (`saveNow(completion:)`, `runToolOnSavedDeck`, `setTheme`, `currentThemeSlug`, `onThemeChanged`, `showToolError`)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (the Theme toolbar item, `showThemePopover`, `refreshThemeItem`)
- Modify: `desktop/Tap/Documents/DocumentBar.swift` (a `.toolFailed` bar kind, if `DocumentBarView.Kind` is a closed enum)
- Modify: `desktop/TapTests/Support/HostedTestCase.swift` (the seams reset in `setUp`)
- Create: `desktop/TapTests/Support/FakeToolScripts.swift`, `desktop/TapTests/Support/MemoryGeminiKeyStore.swift`
- Create: `desktop/TapTests/Fixtures/diagram.png`
- Test: `desktop/TapTests/ThemeGridTests.swift`, `desktop/TapTests/ThemeSetTests.swift`

**Interfaces:**
- Consumes: `ToolRun` (Task 3), `ThemeCatalog`, `ThemeImageResult`, `ThemeSetResult` (Task 4), `GeneralSettings`, `GeminiKeyStore`, `GeminiKeySource` (Task 5), `Frontmatter.value(at:)` and `unquoted` (D5), `DeckSessionController.saveNow`, `loadDiskVersion`, `hasDiskConflict`, `editorViewController.showBar` (D2), `TapLog`, `AppEnvironment.tapEnvironment()`.
- Produces: `TapTool.run(_:in:timeout:log:onProgress:) async -> ToolRun.Exit`, `TapTool.makeRun(_:in:timeout:log:) async -> ToolRun`; `AppEnvironment.toolExecutableURL: URL?`, `geminiKeyStore: GeminiKeyStore`, `generalSettings: GeneralSettings`, `themeImages: ThemeImageLoader`, `geminiKeySource() async -> GeminiKeySource`; `ThemeImageLoader` (`catalog`, `loadCatalog()`, `image(for:)`, `loadAll()`, `didLoadNotification`, `downloadProgress`); `ThemeGridViewController` (`selectedSlug`, `onPick`, `cells`, `cell(for:)`, `footerLabel`, `downloadLabel`); `ThemePopoverController(grid:)` with `show(relativeTo:of:)`; `DeckSessionController.saveNow(completion:)`, `runToolOnSavedDeck(_:actionName:completion:)`, `setTheme(_:)`, `currentThemeSlug`, `onThemeChanged`; `DeckWindowController.themeButton`, `themeItemIdentifier`, `showThemePopover(_:)`. Tasks 7, 8 and 11 reuse the grid, the popover and `runToolOnSavedDeck`.

- [ ] **Step 1: The test support: the scripted tool, the memory key store, the fixture, the seams**

`desktop/TapTests/Support/FakeToolScripts.swift`:

```swift
import Foundation
@testable import Tap

/// Stand-ins for the bundled tap's subcommands (`AppEnvironment.
/// toolExecutableURL`), while the deck's real `tap dev --app` keeps
/// running. Each script records its arguments in `record`, handles the
/// subcommands a test names, and hands every other one to the real tap,
/// so `tap theme list` and `tap slide list` stay real. Every script
/// kills itself after five minutes, so a test that never waits leaves
/// nothing running. A fake never records the value of an environment
/// variable: `${GEMINI_API_KEY:+set}` records the word set.
enum FakeToolScripts {
    static let selfKill = "(sleep 300; kill -9 $$) </dev/null >/dev/null 2>&1 &"

    static func write(_ body: String, recordingTo record: URL) throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        let realTap = AppEnvironment.shared.tapExecutableURL.path
        try """
        #!/bin/sh
        \(selfKill)
        echo "arguments: $@" >> "\(record.path)"
        echo "gemini: ${GEMINI_API_KEY:+set}" >> "\(record.path)"
        case "$1 $2" in
        \(body)
          *) exec "\(realTap)" "$@" ;;
        esac
        """.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }

    /// `tap theme show <slug> --image ...`: copies `png` to a file named
    /// after the slug and prints tap's result, with `downloadLines` of
    /// download progress first when `--progress json` is given.
    static func themeShow(png: URL, downloadLines: Int = 0, recordingTo record: URL) throws -> URL {
        let folder = try Fixtures.temporaryFolder()
        let download = (0..<downloadLines).map { index in
            #"echo '{"phase":"download","bytes":\#((index + 1) * 50_000_000),"totalBytes":\#(downloadLines * 50_000_000)}' >&2"#
        }.joined(separator: "\n    ")
        return try write("""
          "theme show")
            slug="$3"
            out="\(folder.path)/$slug.png"
            cp "\(png.path)" "$out"
            case "$*" in *"--progress json"*) \(download.isEmpty ? ":" : download); echo '{"phase":"render","done":1,"total":1}' >&2; echo "{\\"phase\\":\\"done\\",\\"ok\\":true,\\"slug\\":\\"$slug\\",\\"image\\":\\"$out\\",\\"cached\\":false}" >&2 ;; esac
            printf '{"ok": true, "slug": "%s", "image": "%s", "cached": false}\\n' "$slug" "$out"
            exit 0 ;;
        """, recordingTo: record)
    }

    /// `tap theme set <slug> <deck>`: fails with tap's unknown_theme error, for the error path.
    static func themeSetFailing(recordingTo record: URL) throws -> URL {
        try write("""
          "theme set")
            printf '{"ok": false, "error": {"code": "unknown_theme", "message": "unknown theme \\"%s\\": valid themes are base, terminal"}}\\n' "$3"
            exit 1 ;;
        """, recordingTo: record)
    }
}
```

`desktop/TapTests/Support/MemoryGeminiKeyStore.swift`:

```swift
import Foundation
@testable import TapDesktopCore

/// The key store every hosted test uses, so no test reads or writes the
/// person's Keychain. The value a test stores is a placeholder, never a
/// secret, and no test prints it.
final class MemoryGeminiKeyStore: GeminiKeyStore {
    var key: String?
    private(set) var writes: [String?] = []
    init(key: String? = nil) { self.key = key }
    func read() throws -> String? { key }
    func write(_ key: String?) throws {
        self.key = key
        writes.append(key)
    }
}
```

The fixture `desktop/TapTests/Fixtures/diagram.png`, a one-pixel PNG, written once:

```bash
printf 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=' | base64 -D > desktop/TapTests/Fixtures/diagram.png
file desktop/TapTests/Fixtures/diagram.png
```

Expected: `PNG image data, 1 x 1, 8-bit gray+alpha, non-interlaced`.

In `AppEnvironment.swift`, after `presentExecutableURL`:

```swift
    /// The tap the one-shot commands run (tap new, tap theme show, tap
    /// export ...), for tests that script a subcommand while the deck's
    /// real tap dev keeps running. nil runs the bundled tap.
    var toolExecutableURL: URL?
    /// Where the Gemini key lives. A test replaces it with a store in memory.
    var geminiKeyStore: GeminiKeyStore = KeychainGeminiKeyStore()
    /// The General pane's settings. A test replaces this with one on a fresh suite.
    var generalSettings = GeneralSettings()
    /// The theme catalog and every theme's render, loaded once per app.
    lazy var themeImages = ThemeImageLoader()
```

and change `tapEnvironment()` to:

```swift
    /// The login shell's variables, the app's own on top, and, when the
    /// shell set no GEMINI_API_KEY, the Keychain's key under that name.
    /// The key goes to tap and nowhere else: never into a log line.
    func tapEnvironment() async -> [String: String] {
        var variables = await loginShellLoader.environment().variables
        variables.merge(extraEnvironment) { _, extra in extra }
        try? GeminiKeySource.apply(store: geminiKeyStore, to: &variables)
        return variables
    }

    /// Which key tap gets, for the Image Generation pane's label.
    func geminiKeySource() async -> GeminiKeySource {
        var shell = await loginShellLoader.environment().variables
        shell.merge(extraEnvironment) { _, extra in extra }
        return GeminiKeySource.resolve(shellValue: shell["GEMINI_API_KEY"], storedKey: try? geminiKeyStore.read())
    }
```

In the `init`, under the `-TapDefaultsSuite` block, add `generalSettings = GeneralSettings(defaults: defaults)`. In `HostedTestCase.setUp`, after the `presentExecutableURL = nil` line:

```swift
        AppEnvironment.shared.toolExecutableURL = nil
        AppEnvironment.shared.geminiKeyStore = MemoryGeminiKeyStore()
        AppEnvironment.shared.generalSettings = GeneralSettings(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.general.\(UUID().uuidString)")))
        AppEnvironment.shared.themeImages = ThemeImageLoader()
```

(`themeImages` is `lazy var`, so assigning a fresh one is allowed.) Also in `setUp`, the shell environment's `GEMINI_API_KEY`, if the runner has one, would make every key test read `.shell`: add `AppEnvironment.shared.extraEnvironment["GEMINI_API_KEY"] = ""` so the tests' shell value is empty (`resolve` treats empty as none), and the one test that wants a shell key sets its own.

- [ ] **Step 2: Write the failing grid and theme set tests**

`desktop/TapTests/ThemeGridTests.swift`:

```swift
import XCTest
@testable import Tap
@testable import TapDesktopCore

final class ThemeGridTests: HostedTestCase {
    /// The grid over a scripted tap whose theme renders are the fixture PNG;
    /// `tap theme list` stays real, so the names and the groups are tap's.
    func gridWithFakeRenders(downloadLines: Int = 0) async throws -> (ThemeGridViewController, URL) {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        let png = Fixtures.repositoryRoot.appendingPathComponent("desktop/TapTests/Fixtures/diagram.png")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.themeShow(png: png, downloadLines: downloadLines, recordingTo: record)
        let grid = ThemeGridViewController()
        grid.loadViewIfNeeded()
        return (grid, record)
    }

    func testThemePickerListsTapSThemes() async throws {
        let (grid, record) = try await gridWithFakeRenders()
        try await waitUntil(timeout: 20, "the catalog") { AppEnvironment.shared.themeImages.catalog != nil }
        let catalog = try XCTUnwrap(AppEnvironment.shared.themeImages.catalog)
        XCTAssertEqual(catalog.themes.count, 21, "every theme tap lists")
        XCTAssertEqual(grid.cells.map(\.slug), catalog.light.map(\.slug) + catalog.dark.map(\.slug), "one grid, light then dark, in tap's order")
        XCTAssertEqual(grid.sectionTitles, ["Light", "Dark"])
        XCTAssertEqual(grid.cell(for: "terminal")?.nameLabel.stringValue, "Terminal")
        XCTAssertEqual(grid.cell(for: "terminal")?.accessibilityLabel(), "Terminal, dark theme")
        // The renders land one at a time, in the grid's order, from tap theme show --image.
        try await waitUntil(timeout: 60, "every render") { grid.cells.allSatisfy { $0.imageView.image != nil } }
        let shows = try String(contentsOf: record, encoding: .utf8).components(separatedBy: "\n").filter { $0.hasPrefix("arguments: theme show") }
        XCTAssertEqual(shows.count, 21)
        XCTAssertTrue(shows[0].hasPrefix("arguments: theme show \(catalog.light[0].slug) --image --json --progress json"), shows[0])
        XCTAssertEqual(grid.footerLabel.stringValue, "Picking a theme runs tap theme set. Press T in the preview to try one without saving.")
        // A second grid shows the renders at once: the images are kept for the app's life.
        let again = ThemeGridViewController()
        again.loadViewIfNeeded()
        XCTAssertTrue(again.cells.allSatisfy { $0.imageView.image != nil }, "no second render")
        XCTAssertEqual(try String(contentsOf: record, encoding: .utf8).components(separatedBy: "\n").filter { $0.hasPrefix("arguments: theme show") }.count, 21)
    }

    func testTheEngineDownloadShowsUnderTheGrid() async throws {
        let (grid, _) = try await gridWithFakeRenders(downloadLines: 3)
        try await waitUntil(timeout: 20, "the download label") { !grid.downloadLabel.isHidden }
        XCTAssertTrue(grid.downloadLabel.stringValue.hasPrefix("Downloading the export engine"), grid.downloadLabel.stringValue)
        try await waitUntil(timeout: 60, "every render") { grid.cells.allSatisfy { $0.imageView.image != nil } }
        XCTAssertTrue(grid.downloadLabel.isHidden, "the label goes with the last download line")
    }

    func testAPickReportsTheSlugAndMarksTheCell() async throws {
        let (grid, _) = try await gridWithFakeRenders()
        try await waitUntil(timeout: 20, "the cells") { !grid.cells.isEmpty }
        var picked: [String] = []
        grid.onPick = { picked.append($0) }
        grid.selectedSlug = "terminal"
        XCTAssertEqual(grid.cell(for: "terminal")?.isSelected, true)
        grid.cell(for: "blueprint")?.performClick(nil)
        XCTAssertEqual(picked, ["blueprint"])
        XCTAssertEqual(grid.selectedSlug, "blueprint")
        XCTAssertEqual(grid.cell(for: "terminal")?.isSelected, false)
    }

    func testAThemeRenderOutlivingItsGridIsDropped() async throws {
        var (grid, _): (ThemeGridViewController?, URL) = try await gridWithFakeRenders()
        try await waitUntil(timeout: 20, "the cells") { !(grid?.cells.isEmpty ?? true) }
        weak var gone = grid
        grid = nil
        try await waitUntil(timeout: 5, "the grid to be freed") { gone == nil }
        // The loader keeps going and finishes every render without a grid to tell.
        try await waitUntil(timeout: 60, "the renders") { AppEnvironment.shared.themeImages.image(for: "blueprint") != nil }
    }
}
```

`desktop/TapTests/ThemeSetTests.swift`:

```swift
import XCTest
@testable import Tap
@testable import TapDesktopCore

final class ThemeSetTests: HostedTestCase {
    func testChangeTheDeckSTheme() async throws {
        let deck = try Fixtures.copyDeck("ops.md")
        let document = try await openDeckAndWaitForPreview(deck)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let before = try String(contentsOf: deck, encoding: .utf8)
        XCTAssertEqual(window.themeButton.title, controller.currentThemeSlug.map { AppEnvironment.shared.themeImages.catalog?.name(forSlug: $0) ?? $0 } ?? "Base",
                       "the toolbar names the deck's theme")
        // An unsaved edit first: the pick saves it before tap reads the file.
        controller.jumpToSlide(number: 2)
        controller.editor.insertText("typed before the pick ", replacementRange: controller.editor.selectedRange())
        XCTAssertTrue(document.isDocumentEdited)

        controller.setTheme("blueprint")
        try await waitUntil(timeout: 20, "tap theme set to land") { controller.editor.string.contains("theme: blueprint") }
        let onDisk = try String(contentsOf: deck, encoding: .utf8)
        XCTAssertTrue(onDisk.contains("theme: blueprint"), "tap wrote the file")
        XCTAssertTrue(onDisk.contains("typed before the pick"), "the buffer was saved first")
        XCTAssertFalse(document.isDocumentEdited, "the buffer equals the file after the load")
        XCTAssertEqual(controller.currentSlideNumber, 2, "the cursor stays on its slide")
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Change Theme", "one undo step, named")
        try await waitUntil(timeout: 10, "the toolbar") { window.themeButton.title == "Blueprint" }
        // The preview re-renders: tap reloads its pages on a file change, and the app resends the slide.
        _ = try await waitForPreview(document, slide: 2)

        controller.editor.undoManager?.undo()
        XCTAssertEqual(controller.editor.string, try XCTUnwrap(controller.lastAppliedText).isEmpty ? controller.editor.string : controller.editor.string)
        XCTAssertFalse(controller.editor.string.contains("theme: blueprint"), "undo returns to the text before tap's edit")
        XCTAssertTrue(controller.editor.string.contains("typed before the pick"))
        XCTAssertNotEqual(controller.editor.string, before)
    }

    func testThemeSetIsRefusedWhileADiskConflictShows() async throws {
        let deck = try Fixtures.copyDeck("ops.md")
        let document = try await openDeckAndWaitForPreview(deck)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.insertText("mine ", replacementRange: NSRange(location: controller.editor.hiddenLength, length: 0))
        try (try String(contentsOf: deck, encoding: .utf8) + "\n# Theirs\n").write(to: deck, atomically: true, encoding: .utf8)
        try await waitUntil(timeout: 10, "the conflict bar") { controller.hasDiskConflict }
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.themeSetFailing(recordingTo: record)

        controller.setTheme("blueprint")
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertFalse(FileManager.default.fileExists(atPath: record.path), "tap was not run: the save was refused")
        XCTAssertTrue(controller.editor.string.hasPrefix(String(controller.editor.string.prefix(controller.editor.hiddenLength)) + "mine "), "the edit is kept")
        XCTAssertTrue(controller.hasDiskConflict)
        XCTAssertTrue(controller.session.log.text.contains("Change Theme was not run: the save was refused"))
    }

    func testTapsErrorShowsOnTheBar() async throws {
        let deck = try Fixtures.copyDeck("ops.md")
        let document = try await openDeckAndWaitForPreview(deck)
        let controller = try XCTUnwrap(document.sessionController)
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.themeSetFailing(recordingTo: record)
        let textBefore = controller.editor.string

        controller.setTheme("nope")
        try await waitUntil(timeout: 10, "the bar") { controller.editorViewController.barStack.arrangedSubviews.contains { ($0 as? DocumentBarView)?.kind == .toolFailed } }
        let bar = try XCTUnwrap(controller.editorViewController.barStack.arrangedSubviews.compactMap { $0 as? DocumentBarView }.first { $0.kind == .toolFailed })
        XCTAssertEqual(bar.messageLabel.stringValue, "Change Theme failed.")
        XCTAssertTrue(bar.detailLabel.stringValue.hasPrefix("unknown theme \"nope\""), "tap's own message")
        XCTAssertEqual(controller.editor.string, textBefore, "nothing changed")
        bar.buttons.first { $0.title == "OK" }?.performClick(nil)
        XCTAssertFalse(controller.editorViewController.barStack.arrangedSubviews.contains { ($0 as? DocumentBarView)?.kind == .toolFailed })
    }
}
```

`DocumentBarView`'s `messageLabel`, `detailLabel`, `buttons` and `kind` are D2's names; if the view exposes them under other names, the test uses those. `hasDiskConflict` is D2's `private(set)` (the D5 tests read it).

- [ ] **Step 3: Build to verify they do not compile**

Run: `make -C desktop test-build 2>&1 | tail -5`
Expected: the build fails on `ThemeGridViewController`, `setTheme`, `themeButton`, `FakeToolScripts` (undefined). Nothing is launched.

- [ ] **Step 4: `TapTool`**

`desktop/Tap/App/TapTool.swift`:

```swift
import Foundation

/// Runs one tap subcommand with the app's tap and environment: `tap new`,
/// `tap theme show`, `tap export pdf` and the rest. The command line goes
/// to the deck's log when there is one; the environment never does.
@MainActor
enum TapTool {
    static let defaultTimeout: TimeInterval = 120

    /// A run ready to start, for a caller that cancels or streams (the export sheet).
    static func makeRun(_ arguments: [String], in directory: URL? = nil, timeout: TimeInterval? = defaultTimeout, log: TapLog? = nil) async -> ToolRun {
        let environment = AppEnvironment.shared
        let configuration = ToolRun.Configuration(executableURL: environment.toolExecutableURL ?? environment.tapExecutableURL,
                                                  arguments: arguments, environment: await environment.tapEnvironment(),
                                                  currentDirectoryURL: directory, timeout: timeout)
        let run = ToolRun(configuration: configuration)
        log?.append("tap " + arguments.joined(separator: " "), source: .app)
        if let log {
            run.onStandardErrorLine = { [weak log] line in log?.append(line, source: .standardError) }
        }
        return run
    }

    /// Starts and waits.
    static func run(_ arguments: [String], in directory: URL? = nil, timeout: TimeInterval? = defaultTimeout, log: TapLog? = nil,
                    onProgress: ((ProgressLine) -> Void)? = nil) async -> ToolRun.Exit {
        let run = await makeRun(arguments, in: directory, timeout: timeout, log: log)
        run.onProgress = onProgress
        let exit = await run.run()
        if let log, let outcome = exit.outcome, case .failed(let code, let message) = outcome {
            log.append("tap \(arguments.first ?? "") failed (\(code)): \(message)", source: .app)
        }
        return exit
    }
}
```

- [ ] **Step 5: `ThemeImageLoader`**

`desktop/Tap/Themes/ThemeImageLoader.swift`:

```swift
import AppKit

/// The theme catalog (`tap theme list --json`, once) and every theme's
/// render (`tap theme show <slug> --image --json --progress json`), one
/// at a time in the grid's order, kept for the app's life. tap caches the
/// PNGs by its version, so the next launch renders nothing. The first
/// render on a Mac downloads the export engine; its progress lines are
/// published for the grid to show.
@MainActor
final class ThemeImageLoader {
    static let didLoadCatalogNotification = Notification.Name("TapThemeCatalogDidLoad")
    static let didLoadImageNotification = Notification.Name("TapThemeImageDidLoad")
    static let downloadDidChangeNotification = Notification.Name("TapThemeDownloadDidChange")
    static let maximumAttempts = 3

    private(set) var catalog: ThemeCatalog?
    private var images: [String: NSImage] = [:]
    /// Bytes of totalBytes while the export engine downloads, nil otherwise.
    private(set) var downloadProgress: (bytes: Int64, totalBytes: Int64)?
    private var catalogTask: Task<Void, Never>?
    private var renderTask: Task<Void, Never>?
    private var catalogAttempts = 0

    func image(for slug: String) -> NSImage? { images[slug] }

    /// Loads the catalog if it is not loaded, then renders every theme
    /// that has no image yet. Safe to call from every grid that opens.
    func loadAll() {
        Task { @MainActor [weak self] in
            await self?.loadCatalog()
            self?.renderMissing()
        }
    }

    func loadCatalog() async {
        if catalog != nil { return }
        if let catalogTask { return await catalogTask.value }
        guard catalogAttempts < Self.maximumAttempts else { return }
        catalogAttempts += 1
        let task = Task { @MainActor [weak self] in
            let exit = await TapTool.run(["theme", "list", "--json"], timeout: 30)
            guard let self, let loaded = try? ThemeCatalog.decode(exit.standardOutput) else { return }
            self.catalog = loaded
            NotificationCenter.default.post(name: Self.didLoadCatalogNotification, object: self)
        }
        catalogTask = task
        await task.value
        catalogTask = nil
    }

    private func renderMissing() {
        guard renderTask == nil, let catalog else { return }
        let missing = (catalog.light + catalog.dark).map(\.slug).filter { images[$0] == nil }
        guard !missing.isEmpty else { return }
        renderTask = Task { @MainActor [weak self] in
            for slug in missing {
                guard let self else { return }
                await self.render(slug)
            }
            self?.renderTask = nil
            // A grid that opened during the run may have added nothing; a theme
            // whose render failed is tried again by the next loadAll.
        }
    }

    private func render(_ slug: String) async {
        let exit = await TapTool.run(["theme", "show", slug, "--image", "--json", "--progress", "json"], timeout: 600) { [weak self] progress in
            guard let self else { return }
            switch progress {
            case .download(let bytes, let totalBytes):
                self.downloadProgress = (bytes, totalBytes)
                NotificationCenter.default.post(name: Self.downloadDidChangeNotification, object: self)
            case .step, .finished:
                if self.downloadProgress != nil {
                    self.downloadProgress = nil
                    NotificationCenter.default.post(name: Self.downloadDidChangeNotification, object: self)
                }
            }
        }
        guard let result = try? ThemeImageResult.decode(exit.standardOutput), let image = NSImage(contentsOfFile: result.image) else { return }
        images[slug] = image
        NotificationCenter.default.post(name: Self.didLoadImageNotification, object: self, userInfo: ["slug": slug])
    }
}
```

- [ ] **Step 6: The grid and the popover (the ThemePicker board)**

The ThemePicker board draws one scrolling grid, five cells per row, a "Light" heading over the light themes and a "Dark" heading over the dark ones, each cell a 16:9 render with the theme's name under it, the picked theme with a blue ring, and under the grid the line "Picking a theme runs tap theme set. Press T in the preview to try one without saving." The NewDeck board draws the same cells under a "Theme" heading. `desktop/Tap/Themes/ThemeGridViewController.swift`:

```swift
import AppKit

/// One cell: the theme's render (its name on a neutral fill until the
/// render lands) and its name. A button, so a click and VoiceOver's press
/// both pick it.
final class ThemeCell: NSButton {
    let slug: String
    let imageView = NSImageView()
    let nameLabel = NSTextField(labelWithString: "")
    private let placeholder = NSTextField(labelWithString: "")
    static let renderSize = NSSize(width: 96, height: 54)

    var isSelected = false {
        didSet {
            imageView.layer?.borderColor = isSelected ? NSColor.controlAccentColor.cgColor : NSColor.separatorColor.cgColor
            imageView.layer?.borderWidth = isSelected ? 3 : 0.5
            setAccessibilityValue(isSelected ? "selected" : "")
        }
    }

    init(theme: ThemeSummary) {
        slug = theme.slug
        super.init(frame: .zero)
        title = ""
        isBordered = false
        setButtonType(.momentaryChange)
        imageView.wantsLayer = true
        imageView.layer?.cornerRadius = 5
        imageView.layer?.masksToBounds = true
        imageView.layer?.backgroundColor = NSColor.quaternaryLabelColor.cgColor
        imageView.imageScaling = .scaleProportionallyUpOrDown
        placeholder.stringValue = theme.name
        placeholder.font = .systemFont(ofSize: 11, weight: .semibold)
        placeholder.alignment = .center
        placeholder.textColor = .secondaryLabelColor
        nameLabel.stringValue = theme.name
        nameLabel.font = .systemFont(ofSize: 10.5)
        nameLabel.alignment = .center
        nameLabel.lineBreakMode = .byTruncatingTail
        let stack = NSStackView(views: [imageView, nameLabel])
        stack.orientation = .vertical
        stack.spacing = 4
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)
        imageView.addSubview(placeholder)
        placeholder.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: topAnchor), stack.bottomAnchor.constraint(equalTo: bottomAnchor),
            stack.leadingAnchor.constraint(equalTo: leadingAnchor), stack.trailingAnchor.constraint(equalTo: trailingAnchor),
            imageView.widthAnchor.constraint(equalToConstant: Self.renderSize.width), imageView.heightAnchor.constraint(equalToConstant: Self.renderSize.height),
            placeholder.centerXAnchor.constraint(equalTo: imageView.centerXAnchor), placeholder.centerYAnchor.constraint(equalTo: imageView.centerYAnchor),
        ])
        setAccessibilityIdentifier("theme-cell-\(theme.slug)")
        setAccessibilityLabel("\(theme.name), \(theme.polarity) theme")
        isSelected = false
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func show(_ image: NSImage?) {
        imageView.image = image
        placeholder.isHidden = image != nil
    }
}

/// The grid of every theme, the same view inside the New Deck sheet, the
/// toolbar's popover and the Deck tab: Light and Dark sections, five
/// cells per row, in tap's order, with the renders as they land.
final class ThemeGridViewController: NSViewController {
    static let columns = 5
    var onPick: ((String) -> Void)?
    private(set) var cells: [ThemeCell] = []
    private(set) var sectionTitles: [String] = []
    let footerLabel = NSTextField(wrappingLabelWithString: "Picking a theme runs tap theme set. Press T in the preview to try one without saving.")
    let downloadLabel = NSTextField(labelWithString: "")
    private let scrollView = NSScrollView()
    private let content = NSStackView()
    private var observers: [NSObjectProtocol] = []

    var selectedSlug: String? {
        didSet { for cell in cells { cell.isSelected = cell.slug == selectedSlug } }
    }

    /// Whether the footer line shows: the toolbar popover and the Deck tab
    /// show it; the New Deck sheet has no deck to set a theme on.
    var showsFooter = true {
        didSet { footerLabel.isHidden = !showsFooter }
    }

    func cell(for slug: String) -> ThemeCell? { cells.first { $0.slug == slug } }

    override func loadView() {
        content.orientation = .vertical
        content.alignment = .leading
        content.spacing = 8
        content.edgeInsets = NSEdgeInsets(top: 4, left: 4, bottom: 4, right: 4)
        scrollView.documentView = content
        scrollView.hasVerticalScroller = true
        scrollView.drawsBackground = false
        content.translatesAutoresizingMaskIntoConstraints = false
        content.widthAnchor.constraint(equalTo: scrollView.contentView.widthAnchor).isActive = true
        footerLabel.font = .systemFont(ofSize: 11.5)
        footerLabel.textColor = .secondaryLabelColor
        downloadLabel.font = .systemFont(ofSize: 11.5)
        downloadLabel.textColor = .secondaryLabelColor
        downloadLabel.isHidden = true
        downloadLabel.setAccessibilityIdentifier("theme-download")
        let root = NSStackView(views: [scrollView, downloadLabel, footerLabel])
        root.orientation = .vertical
        root.alignment = .leading
        root.spacing = 8
        scrollView.heightAnchor.constraint(equalToConstant: 320).isActive = true
        scrollView.widthAnchor.constraint(equalTo: root.widthAnchor).isActive = true
        root.setAccessibilityIdentifier("theme-grid")
        view = root
        let loader = AppEnvironment.shared.themeImages
        observers = [
            NotificationCenter.default.addObserver(forName: ThemeImageLoader.didLoadCatalogNotification, object: loader, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.rebuild() }
            },
            NotificationCenter.default.addObserver(forName: ThemeImageLoader.didLoadImageNotification, object: loader, queue: .main) { [weak self] notification in
                MainActor.assumeIsolated {
                    guard let slug = notification.userInfo?["slug"] as? String else { return }
                    self?.cell(for: slug)?.show(loader.image(for: slug))
                }
            },
            NotificationCenter.default.addObserver(forName: ThemeImageLoader.downloadDidChangeNotification, object: loader, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.refreshDownload() }
            },
        ]
        rebuild()
        refreshDownload()
        loader.loadAll()
    }

    deinit {
        for observer in observers { NotificationCenter.default.removeObserver(observer) }
    }

    private func rebuild() {
        for view in content.arrangedSubviews { view.removeFromSuperview() }
        cells = []
        sectionTitles = []
        guard let catalog = AppEnvironment.shared.themeImages.catalog else { return }
        for (title, themes) in [("Light", catalog.light), ("Dark", catalog.dark)] where !themes.isEmpty {
            sectionTitles.append(title)
            let heading = NSTextField(labelWithString: title)
            heading.font = .systemFont(ofSize: 11, weight: .semibold)
            heading.textColor = .secondaryLabelColor
            content.addArrangedSubview(heading)
            let grid = NSGridView()
            grid.rowSpacing = 12
            grid.columnSpacing = 12
            for row in stride(from: 0, to: themes.count, by: Self.columns) {
                let rowCells = themes[row..<min(row + Self.columns, themes.count)].map { theme -> ThemeCell in
                    let cell = ThemeCell(theme: theme)
                    cell.show(AppEnvironment.shared.themeImages.image(for: theme.slug))
                    cell.isSelected = theme.slug == selectedSlug
                    cell.target = self
                    cell.action = #selector(cellPressed(_:))
                    cells.append(cell)
                    return cell
                }
                grid.addRow(with: rowCells + Array(repeating: NSView(), count: Self.columns - rowCells.count))
            }
            content.addArrangedSubview(grid)
        }
    }

    private func refreshDownload() {
        guard let progress = AppEnvironment.shared.themeImages.downloadProgress else {
            downloadLabel.isHidden = true
            return
        }
        let formatter = ByteCountFormatter()
        formatter.countStyle = .file
        downloadLabel.stringValue = "Downloading the export engine: \(formatter.string(fromByteCount: progress.bytes)) of \(formatter.string(fromByteCount: progress.totalBytes)). This happens once."
        downloadLabel.isHidden = false
    }

    @objc private func cellPressed(_ sender: ThemeCell) {
        selectedSlug = sender.slug
        onPick?(sender.slug)
    }
}
```

`desktop/Tap/Themes/ThemePopoverController.swift`:

```swift
import AppKit

/// The theme grid in a popover, for the toolbar's Theme item and the
/// Deck tab's theme row. A pick closes the popover and reports the slug.
@MainActor
final class ThemePopoverController {
    let grid = ThemeGridViewController()
    private let popover = NSPopover()
    var onPick: ((String) -> Void)?

    init() {
        popover.behavior = .transient
        popover.contentViewController = grid
        grid.view.widthAnchor.constraint(equalToConstant: 590).isActive = true
        grid.onPick = { [weak self] slug in
            self?.popover.performClose(nil)
            self?.onPick?(slug)
        }
    }

    var isShown: Bool { popover.isShown }

    func show(relativeTo rect: NSRect, of view: NSView, selected: String?) {
        grid.selectedSlug = selected
        popover.show(relativeTo: rect, of: view, preferredEdge: .maxY)
    }

    func close() { popover.performClose(nil) }
}
```

- [ ] **Step 7: The session controller's tool path and `setTheme`**

In `DeckSessionController.swift`, replace `saveNow()` with a version that reports, keeping the old name as a wrapper:

```swift
    /// Writes the buffer to the deck file now, ahead of the autosave, and
    /// reports the outcome. A buffer that equals the file needs no write.
    /// A save the document refuses (a disk conflict is showing) comes back
    /// as its error and the edit stays in the buffer for the next save.
    func saveNow(completion: @escaping (Error?) -> Void) {
        guard let document, let url = document.fileURL else { return completion(CocoaError(.fileNoSuchFile)) }
        guard isContentEdited else { return completion(nil) }
        document.save(to: url, ofType: document.fileType ?? "net.daringfireball.markdown", for: .saveOperation, completionHandler: completion)
    }

    func saveNow() {
        saveNow { [weak self] error in
            if let error { self?.session.log.append("the save after the fix-it was refused: \(error.localizedDescription)", source: .app) }
        }
    }
```

Add, in a `// MARK: Tap commands on the deck` section:

```swift
    /// The one path for a tap command that writes the deck file (tap theme
    /// set, tap image generate, tap image regenerate): the buffer is saved
    /// first, tap runs on the file, and the file comes back into the
    /// buffer as one undo step named after the action, the cursor's slide
    /// kept, through the same load an external change takes. A refused
    /// save runs nothing; tap's failure shows on a bar with tap's words.
    /// `arguments` name the deck by its path already.
    func runToolOnSavedDeck(_ arguments: [String], actionName: String, completion: ((ToolOutcome?) -> Void)? = nil) {
        guard !hasDiskConflict else {
            session.log.append("\(actionName) was not run: the save was refused (the deck changed on disk)", source: .app)
            NSSound.beep()
            completion?(nil)
            return
        }
        saveNow { [weak self] error in
            guard let self else { return }
            if let error {
                self.session.log.append("\(actionName) was not run: the save was refused: \(error.localizedDescription)", source: .app)
                completion?(nil)
                return
            }
            Task { @MainActor [weak self] in
                guard let self else { return }
                let exit = await TapTool.run(arguments, in: self.document?.fileURL?.deletingLastPathComponent(), log: self.session.log)
                switch exit.outcome {
                case .ok?:
                    self.loadDiskVersion(actionName: actionName)
                case .failed(_, let message)?:
                    self.showToolError(actionName: actionName, message: message)
                case nil:
                    self.showToolError(actionName: actionName, message: exit.cancelled ? "cancelled" : "tap did not answer (exit \(exit.status)); see the Tap Log")
                }
                completion?(exit.outcome)
            }
        }
    }

    /// tap's failure, on a bar over the editor with tap's own message.
    func showToolError(actionName: String, message: String) {
        editorViewController.showBar(DocumentBarView(kind: .toolFailed, message: "\(actionName) failed.", detail: message,
                                                     buttons: [("OK", { [weak self] in self?.editorViewController.hideBar(.toolFailed) })]))
    }

    /// The deck's theme slug from the frontmatter, nil when it names none.
    var currentThemeSlug: String? {
        Frontmatter(text: editor.string).entry(at: ["theme"])?.unquotedValue.flatMap { $0.isEmpty ? nil : $0 }
    }

    /// Told when the frontmatter's theme changes, for the toolbar item.
    var onThemeChanged: ((String?) -> Void)?
    private var lastThemeSlug: String?

    func refreshThemeIfChanged() {
        let slug = currentThemeSlug
        guard slug != lastThemeSlug else { return }
        lastThemeSlug = slug
        onThemeChanged?(slug)
    }

    /// A pick in the theme grid: tap theme set on the saved file.
    func setTheme(_ slug: String) {
        guard let deck = document?.fileURL else { return }
        runToolOnSavedDeck(["theme", "set", slug, deck.path, "--json"], actionName: "Change Theme")
    }
```

`loadDiskVersion()` gains a parameter: `func loadDiskVersion(actionName: String = "Load Disk Version")`, used in its `replaceText(..., actionName: actionName)` call; the existing callers pass nothing. `refreshThemeIfChanged()` is called at the end of `editorTextDidChange`, `documentDidRead` and `undoOrRedoDidChangeText`. If `DocumentBarView.Kind` is an enum, add `case toolFailed` in `DocumentBar.swift` (no new drawing: the bar's existing style with a message, a detail and buttons).

- [ ] **Step 8: The toolbar's Theme item**

In `DeckWindowController.swift`: `static let themeItemIdentifier = NSToolbarItem.Identifier("theme")`, `let themeButton = NSButton()`, `private(set) lazy var themePopover: ThemePopoverController = { let popover = ThemePopoverController(); popover.onPick = { [weak self] slug in self?.sessionController.setTheme(slug) }; return popover }()`. In `toolbarDefaultItemIdentifiers`: `[Self.slidesItemIdentifier, .flexibleSpace, Self.newSlideItemIdentifier, Self.themeItemIdentifier, Self.playItemIdentifier, Self.previewItemIdentifier]` (the ThemePicker board shows the theme's name between the deck's name and Play). In `toolbar(_:itemForItemIdentifier:...)`, before the `guard identifier == Self.previewItemIdentifier`:

```swift
        if identifier == Self.themeItemIdentifier {
            let item = NSToolbarItem(itemIdentifier: identifier)
            item.label = "Theme"
            item.toolTip = "The deck's theme. Click to pick another; tap theme set writes it."
            themeButton.bezelStyle = .toolbar
            themeButton.image = NSImage(systemSymbolName: "paintpalette", accessibilityDescription: "Theme")
            themeButton.imagePosition = .imageLeading
            themeButton.setAccessibilityIdentifier("theme-button")
            themeButton.target = self
            themeButton.action = #selector(showThemePopover(_:))
            refreshThemeItem(slug: sessionController.currentThemeSlug)
            item.view = themeButton
            return item
        }
```

and:

```swift
    @objc func showThemePopover(_ sender: Any?) {
        guard questionSheet == nil, window?.attachedSheet == nil else { return }
        themePopover.show(relativeTo: themeButton.bounds, of: themeButton, selected: sessionController.currentThemeSlug)
    }

    /// The item's title is the theme's name from tap's catalog, the slug
    /// while the catalog loads, and "Base" for a deck that names none (what
    /// tap renders it with).
    func refreshThemeItem(slug: String?) {
        guard let slug else { return themeButton.title = "Base" }
        themeButton.title = AppEnvironment.shared.themeImages.catalog?.name(forSlug: slug) ?? slug
    }
```

In `init`, after the popover wiring of D4: `sessionController.onThemeChanged = { [weak self] slug in self?.refreshThemeItem(slug: slug) }` and an observer of `ThemeImageLoader.didLoadCatalogNotification` that calls `refreshThemeItem(slug: sessionController.currentThemeSlug)` (removed in `windowWillClose`). The View menu needs no item: the Theme item is a picker, not a command; the Deck tab's row (Task 7) is its keyboard path.

- [ ] **Step 9: Build, then hand the hosted tests to CI**

Run: `make -C desktop build && make -C desktop test-build 2>&1 | tail -3 && make -C desktop core-test`
Expected: `** BUILD SUCCEEDED **`, `** TEST BUILD SUCCEEDED **`, the core tests green. The controller's CI run confirms: `ThemeGridTests` (4) and `ThemeSetTests` (3) pass on the macOS job, and every D2 to D5 hosted test still passes (the toolbar gained an item; `WindowLayoutTests` measure the split, not the toolbar).

- [ ] **Step 10: Mutations and commit**

Mutations, each a patch in `mutations-b/`, the ones that could lose an edit first: in `runToolOnSavedDeck`, skip `saveNow` and run tap at once (`Test: TapTests/ThemeSetTests/testChangeTheDeckSTheme`; expected: fails on "the buffer was saved first", tap wrote the file without the typed text, and the load then drops it); in `runToolOnSavedDeck`, drop the `!hasDiskConflict` guard (`Test: TapTests/ThemeSetTests/testThemeSetIsRefusedWhileADiskConflictShows`; expected: fails on "tap was not run"); in `runToolOnSavedDeck`, call `loadDiskVersion` on `.failed` too (`Test: TapTests/ThemeSetTests/testTapsErrorShowsOnTheBar`; expected: the bar never shows); in `loadDiskVersion`, ignore `actionName` (expected: `testChangeTheDeckSTheme` fails on `undoActionName`); in `ThemeImageLoader.renderMissing`, render every theme regardless of `images` (expected: `testThemePickerListsTapSThemes` fails on the second count, 42); in `renderMissing`, order the slugs alphabetically (expected: fails on `shows[0]`); in `ThemeGridViewController.rebuild`, put the dark themes first (expected: fails on `cells.map(\.slug)`); in `refreshDownload`, never hide the label (`Test: TapTests/ThemeGridTests/testTheEngineDownloadShowsUnderTheGrid`; expected: fails on `isHidden`); in `cellPressed`, drop `selectedSlug = sender.slug` (`Test: .../testAPickReportsTheSlugAndMarksTheCell`; expected: fails on `selectedSlug`); in `refreshThemeItem`, show the slug always (expected: `testChangeTheDeckSTheme` fails on "Blueprint"); in `TapTool.makeRun`, log the environment's keys (into `survivors-b/` with the reason: the log line holds names, not values, and no test reads the log for it; the final check's grep guards the value).

```bash
git add desktop/Tap desktop/TapTests desktop/TapDesktopCore
git commit -m "feat(desktop): the theme grid from tap's renders, the toolbar's Theme item, and tap theme set as one undo step"
```

---

### Task 7: The New Deck sheet, File > New Deck…, the welcome window's button, and the Deck tab's theme row

**Files:**
- Create: `desktop/Tap/Documents/NewDeckSheet.swift`
- Modify: `desktop/Tap/App/AppDelegate.swift` (`newDeck(_:)`, `createDeck(from:sheet:)`, `openDeckFromTool`)
- Modify: `desktop/Tap/App/MainMenu.swift` (New Deck… gets its action)
- Modify: `desktop/Tap/Welcome/WelcomeWindowController.swift` (the button's action and enabled state)
- Modify: `desktop/Tap/Preview/DeckFormViewController.swift` (the theme row; waits for a mockup)
- Test: `desktop/TapTests/NewDeckTests.swift`

**Interfaces:**
- Consumes: `QuestionSheet(kind:title:body:path:decline:accept:escape:returnAnswer:detail:)` (D4), `ThemeGridViewController` (Task 6), `NewDeckResult` (Task 4), `GeneralSettings.defaultTheme`, `lastNewDeckFolder` (Task 5), `TapTool.run`, `NSDocumentController.shared.openDocument(withContentsOf:display:completionHandler:)`, `AppDelegate.deck(owning:)`, `WelcomeWindowController.shared`.
- Produces: `NewDeckSheet` (`titleField`, `locationPopup`, `grid`, `createButton`, `cancelButton`, `errorLabel`, `request`, `NewDeckRequest(title:theme:location:)` with `arguments`, `chooseFolder` seam, `showError(_:)`); `AppDelegate.newDeck(_:)`, `createDeck(from:sheet:completion:)`; `WelcomeWindowController.newDeckButton` enabled with the action. Task 14's manifest claims `testNewDeck`.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/NewDeckTests.swift`:

```swift
import XCTest
@testable import Tap
@testable import TapDesktopCore

final class NewDeckTests: HostedTestCase {
    var appDelegate: AppDelegate { NSApp.delegate as! AppDelegate }

    /// Waits for the sheet the New Deck command puts on `window`.
    func newDeckSheet(on window: NSWindow?) async throws -> NewDeckSheet {
        try await waitUntil(timeout: 10, "the New Deck sheet") { window?.attachedSheet is NewDeckSheet }
        return try XCTUnwrap(window?.attachedSheet as? NewDeckSheet)
    }

    func testNewDeck() async throws {
        // With no deck open, File > New Deck goes on the welcome window.
        appDelegate.showWelcomeIfNoDecks()
        let welcome = WelcomeWindowController.shared
        XCTAssertTrue(welcome.newDeckButton.isEnabled)
        let location = try Fixtures.temporaryFolder()
        AppEnvironment.shared.generalSettings.lastNewDeckFolder = location
        AppEnvironment.shared.generalSettings.defaultTheme = "terminal"
        welcome.newDeckButton.performClick(nil)
        let sheet = try await newDeckSheet(on: welcome.window)
        XCTAssertEqual(sheet.locationPopup.titleOfSelectedItem, location.lastPathComponent, "the last used folder is preselected")
        XCTAssertEqual(sheet.request.location, location)
        try await waitUntil(timeout: 20, "the grid") { !sheet.grid.cells.isEmpty }
        XCTAssertEqual(sheet.grid.selectedSlug, "terminal", "the General default is preselected")
        XCTAssertFalse(sheet.grid.showsFooter, "no deck to set a theme on")
        XCTAssertEqual(sheet.locationHint.stringValue, "Creates a folder named after the title, with the deck and images/")

        sheet.titleField.stringValue = "Debugging Production at 3am"
        sheet.grid.cell(for: "blueprint")?.performClick(nil)
        XCTAssertEqual(sheet.request.arguments, ["new", "--yes", "--title", "Debugging Production at 3am", "--theme", "blueprint", "--folder", location.path, "--json"])
        sheet.createButton.performClick(nil)

        let expected = location.appendingPathComponent("debugging-production-at-3am/debugging-production-at-3am.md")
        try await waitUntil(timeout: 30, "the new deck to open") {
            NSDocumentController.shared.documents.contains { FilePaths.same($0.fileURL, expected) }
        }
        let document = try XCTUnwrap(NSDocumentController.shared.documents.first { FilePaths.same($0.fileURL, expected) } as? DeckDocument)
        defer { document.close() }
        XCTAssertNil(welcome.window?.attachedSheet)
        XCTAssertFalse(welcome.window?.isVisible ?? false)
        var isFolder: ObjCBool = false
        XCTAssertTrue(FileManager.default.fileExists(atPath: location.appendingPathComponent("debugging-production-at-3am/images").path, isDirectory: &isFolder) && isFolder.boolValue,
                      "tap made images/")
        let text = try String(contentsOf: expected, encoding: .utf8)
        XCTAssertTrue(text.contains("theme: blueprint"))
        XCTAssertTrue(text.contains("title: Debugging Production at 3am"))
        XCTAssertEqual(AppEnvironment.shared.generalSettings.lastNewDeckFolder, location, "remembered")
        // tap records an approval for the deck it made.
        let listed = try await TapApproval.run(["approval", "list", "--json"], configHome: configHome)
        XCTAssertTrue(listed.contains(Fixtures.realPath(of: expected)), "tap approval list: \(listed)")
        _ = try await waitForRunningTap(document)
    }

    func testTheSheetGoesOnTheKeyDeckWindowAndReportsTapsError() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        appDelegate.newDeck(nil)
        let sheet = try await newDeckSheet(on: window)
        let location = try Fixtures.temporaryFolder()
        sheet.chooseFolder = { $0(location) }
        sheet.locationPopup.selectItem(withTitle: "Other…")
        sheet.locationPopup.performClick(nil)
        XCTAssertEqual(sheet.request.location, location)
        // A location that is gone by the time Create runs: tap's error, in the sheet, the sheet stays.
        try FileManager.default.removeItem(at: location)
        sheet.titleField.stringValue = "Gone"
        sheet.createButton.performClick(nil)
        try await waitUntil(timeout: 20, "tap's error") { !sheet.errorLabel.isHidden }
        XCTAssertTrue(sheet.errorLabel.stringValue.contains("does not exist"), sheet.errorLabel.stringValue)
        XCTAssertTrue(window.attachedSheet === sheet, "the sheet stays for another try")
        XCTAssertTrue(sheet.createButton.isEnabled)
        sheet.cancelButton.performClick(nil)
        try await waitUntil(timeout: 5, "the sheet to close") { window.attachedSheet == nil }
    }

    func testAnEmptyTitleCannotCreate() async throws {
        appDelegate.showWelcomeIfNoDecks()
        appDelegate.newDeck(nil)
        let sheet = try await newDeckSheet(on: WelcomeWindowController.shared.window)
        sheet.titleField.stringValue = ""
        sheet.titleChanged(sheet.titleField)
        XCTAssertFalse(sheet.createButton.isEnabled)
        sheet.titleField.stringValue = "A"
        sheet.titleChanged(sheet.titleField)
        XCTAssertTrue(sheet.createButton.isEnabled)
        sheet.cancelButton.performClick(nil)
    }
}
```

- [ ] **Step 2: Build to verify they do not compile**

Run: `make -C desktop test-build 2>&1 | tail -3`
Expected: the build fails on `NewDeckSheet` and `AppDelegate.newDeck`.

- [ ] **Step 3: The sheet (the NewDeck board)**

The NewDeck board draws the sheet over the welcome window: the heading "New Deck", a card with a "Title" row (a text field) and a "Save in" row (a folder popup showing the folder's name, with the hint under the label), a "Theme" heading over the grid, "Scroll for all 21 themes" under it, and Cancel and Create at the bottom right. The board's hint names the slug ("Creates the folder debugging-production-at-3am with the deck and images/"); the app does not compute tap's slug, so the hint reads "Creates a folder named after the title, with the deck and images/" (open question 1). `desktop/Tap/Documents/NewDeckSheet.swift`:

```swift
import AppKit

/// What the New Deck sheet asks tap for.
struct NewDeckRequest: Equatable {
    var title: String
    var theme: String?
    var location: URL

    /// tap new makes the folder, the deck and images/, and approves the deck.
    var arguments: [String] {
        var arguments = ["new", "--yes", "--title", title]
        if let theme { arguments += ["--theme", theme] }
        arguments += ["--folder", location.path, "--json"]
        return arguments
    }
}

/// File > New Deck: a title, where to save, and the theme grid. Create
/// runs tap new; tap's error, if any, shows in the sheet and the sheet
/// stays for another try.
final class NewDeckSheet: QuestionSheet {
    let titleField = NSTextField(string: "")
    let locationPopup = NSPopUpButton(frame: .zero, pullsDown: false)
    let locationHint = NSTextField(labelWithString: "Creates a folder named after the title, with the deck and images/")
    let grid = ThemeGridViewController()
    let errorLabel = NSTextField(wrappingLabelWithString: "")
    /// Opens a folder chooser (an NSOpenPanel in production; a test answers at once).
    var chooseFolder: (@escaping (URL?) -> Void) -> Void = { completion in
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.canCreateDirectories = true
        panel.prompt = "Choose"
        panel.begin { response in completion(response == .OK ? panel.url : nil) }
    }
    private var locations: [URL] = []
    private var chosenLocation: URL

    var createButton: NSButton { acceptButton }
    var cancelButton: NSButton { declineButton }

    var request: NewDeckRequest {
        NewDeckRequest(title: titleField.stringValue.trimmingCharacters(in: .whitespaces), theme: grid.selectedSlug, location: chosenLocation)
    }

    init(lastFolder: URL?, defaultTheme: String?) {
        let home = FileManager.default.homeDirectoryForCurrentUser
        let documents = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first ?? home.appendingPathComponent("Documents")
        let desktop = FileManager.default.urls(for: .desktopDirectory, in: .userDomainMask).first ?? home.appendingPathComponent("Desktop")
        var locations = [documents, desktop]
        if let lastFolder, !locations.contains(where: { FilePaths.same($0, lastFolder) }) { locations.insert(lastFolder, at: 0) }
        chosenLocation = lastFolder ?? documents
        self.locations = locations
        let form = NSView()
        super.init(kind: "new-deck", title: "New Deck", body: "", path: nil, decline: "Cancel", accept: "Create", escape: .decline, returnAnswer: .accept, detail: form)
        bodyLabel.isHidden = true
        titleField.placeholderString = "My Presentation"
        titleField.target = self
        titleField.action = #selector(titleChanged(_:))
        titleField.delegate = self
        titleField.setAccessibilityIdentifier("new-deck-title")
        for location in locations { locationPopup.addItem(withTitle: location.lastPathComponent) }
        locationPopup.menu?.addItem(.separator())
        locationPopup.addItem(withTitle: "Other…")
        locationPopup.selectItem(at: locations.firstIndex { FilePaths.same($0, chosenLocation) } ?? 0)
        locationPopup.target = self
        locationPopup.action = #selector(locationChanged(_:))
        locationPopup.setAccessibilityIdentifier("new-deck-location")
        locationHint.font = .systemFont(ofSize: 11)
        locationHint.textColor = .secondaryLabelColor
        grid.showsFooter = false
        grid.selectedSlug = defaultTheme
        errorLabel.textColor = .systemRed
        errorLabel.font = .systemFont(ofSize: 12)
        errorLabel.isHidden = true
        errorLabel.setAccessibilityIdentifier("new-deck-error")

        let titleRow = labeledRow("Title", titleField)
        let locationRow = labeledRow("Save in", locationPopup, hint: locationHint)
        let card = NSStackView(views: [titleRow, locationRow])
        card.orientation = .vertical
        card.alignment = .leading
        card.spacing = 8
        let themeHeading = NSTextField(labelWithString: "Theme")
        themeHeading.font = .systemFont(ofSize: 11, weight: .semibold)
        themeHeading.textColor = .secondaryLabelColor
        let scrollHint = NSTextField(labelWithString: "Scroll for all themes")
        scrollHint.font = .systemFont(ofSize: 11.5)
        scrollHint.textColor = .secondaryLabelColor
        let stack = NSStackView(views: [card, themeHeading, grid.view, scrollHint, errorLabel])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.translatesAutoresizingMaskIntoConstraints = false
        form.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: form.topAnchor), stack.bottomAnchor.constraint(equalTo: form.bottomAnchor),
            stack.leadingAnchor.constraint(equalTo: form.leadingAnchor), stack.trailingAnchor.constraint(equalTo: form.trailingAnchor),
            grid.view.widthAnchor.constraint(equalTo: stack.widthAnchor), titleField.widthAnchor.constraint(equalToConstant: 300),
            errorLabel.widthAnchor.constraint(equalTo: stack.widthAnchor),
        ])
        setContentSize(contentView?.fittingSize ?? frame.size)
        titleChanged(titleField)
    }

    private func labeledRow(_ title: String, _ control: NSView, hint: NSTextField? = nil) -> NSView {
        let label = NSTextField(labelWithString: title)
        label.font = .systemFont(ofSize: 13)
        var left: [NSView] = [label]
        if let hint { left.append(hint) }
        let labels = NSStackView(views: left)
        labels.orientation = .vertical
        labels.alignment = .leading
        labels.spacing = 2
        let row = NSStackView(views: [labels, NSView(), control])
        row.orientation = .horizontal
        row.alignment = .centerY
        row.spacing = 12
        return row
    }

    @objc func titleChanged(_ sender: Any?) {
        createButton.isEnabled = !request.title.isEmpty
    }

    @objc private func locationChanged(_ sender: Any?) {
        let index = locationPopup.indexOfSelectedItem
        if index < locations.count {
            chosenLocation = locations[index]
            return
        }
        chooseFolder { [weak self] chosen in
            guard let self else { return }
            if let chosen {
                if !self.locations.contains(where: { FilePaths.same($0, chosen) }) {
                    self.locations.insert(chosen, at: 0)
                    self.locationPopup.insertItem(withTitle: chosen.lastPathComponent, at: 0)
                }
                self.chosenLocation = chosen
            }
            self.locationPopup.selectItem(at: self.locations.firstIndex { FilePaths.same($0, self.chosenLocation) } ?? 0)
        }
    }

    /// tap's message, in the sheet; Create is enabled again for another try.
    func showError(_ message: String) {
        errorLabel.stringValue = message
        errorLabel.isHidden = false
        createButton.isEnabled = true
    }

    func beginCreating() {
        errorLabel.isHidden = true
        createButton.isEnabled = false
    }
}

extension NewDeckSheet: NSTextFieldDelegate {
    func controlTextDidChange(_ notification: Notification) { titleChanged(notification.object) }
}
```

The sheet's accept button ends the sheet with `.OK` (D4's `acceptPressed`); `AppDelegate.createDeck` runs tap and, on a failure, puts the sheet up again with the error (a sheet that ended cannot stay up), or, simpler and what the test asserts, the sheet does not end on Create: override `acceptPressed` behaviour by giving `acceptButton` a new target in `init`: `acceptButton.target = self; acceptButton.action = #selector(createPressed(_:))` with `@objc private func createPressed(_ sender: Any?) { onCreate?(request) }` and `var onCreate: ((NewDeckRequest) -> Void)?`; the delegate ends the sheet itself on success (`sheetParent?.endSheet(sheet, returnCode: .OK)`) and calls `showError` on failure. Return still creates (the accept button holds `"\r"`).

- [ ] **Step 4: The command, in the app delegate, the menu and the welcome window**

In `MainMenu.fileMenu`, the New Deck item becomes `menu.addItem(item("New Deck…", action: #selector(AppDelegate.newDeck(_:)), key: "n"))`. In `WelcomeWindowController.init`, replace the disabled state and the comment with `newDeckButton.target = nil; newDeckButton.action = #selector(AppDelegate.newDeck(_:)); newDeckButton.isEnabled = true` (nil-targeted, so the responder chain reaches the app delegate). In `AppDelegate.swift`:

```swift
    /// File > New Deck and the welcome window's button: the sheet goes on
    /// the key deck window, or on the welcome window when no deck is open
    /// (shown first if it is not). Create runs tap new; the deck opens.
    @objc func newDeck(_ sender: Any?) {
        let host: NSWindow?
        if let deck = Self.deck(owning: NSApp.keyWindow) {
            host = deck.window
        } else {
            showWelcomeIfNoDecks()
            host = WelcomeWindowController.shared.window
        }
        guard let host, host.attachedSheet == nil else { return }
        let settings = AppEnvironment.shared.generalSettings
        let sheet = NewDeckSheet(lastFolder: settings.lastNewDeckFolder, defaultTheme: settings.defaultTheme)
        sheet.onCreate = { [weak self, weak sheet, weak host] request in
            guard let self, let sheet else { return }
            sheet.beginCreating()
            self.createDeck(from: request) { result in
                switch result {
                case .success(let deck):
                    host?.endSheet(sheet, returnCode: .OK)
                    AppEnvironment.shared.generalSettings.lastNewDeckFolder = request.location
                    NSDocumentController.shared.openDocument(withContentsOf: deck, display: true) { _, _, _ in }
                case .failure(let error):
                    sheet.showError((error as? ToolError).map(Self.message(for:)) ?? error.localizedDescription)
                }
            }
        }
        host.beginSheet(sheet) { _ in }
    }

    /// Runs tap new for the sheet's request and reports the deck it wrote.
    func createDeck(from request: NewDeckRequest, completion: @escaping (Result<URL, Error>) -> Void) {
        Task { @MainActor in
            let exit = await TapTool.run(request.arguments, timeout: 60)
            guard let outcome = exit.outcome else { return completion(.failure(ToolError.noResult(status: exit.status))) }
            do {
                let result = try outcome.result(NewDeckResult.self)
                completion(.success(URL(fileURLWithPath: result.deck)))
            } catch {
                completion(.failure(error))
            }
        }
    }

    static func message(for error: ToolError) -> String {
        switch error {
        case .failed(_, let message): return message
        case .noResult(let status): return "tap did not answer (exit \(status))"
        case .cancelled: return "cancelled"
        }
    }
```

Menu validation: `newDeck` is always enabled (`AppDelegate.validateMenuItem` returns true for it, as today for everything but Go to Slide). Opening the deck closes the welcome window, as any open does (D2).

- [ ] **Step 5: The Deck tab's theme row: waits for the person's mockup sign-off (the controller records it in the ledger)**

The spec says the Deck tab uses the same grid; the D5 DeckTabFields board drew a popup for the theme. Proposed, for the mockup: the row keeps its label and shows the theme's name with a "Choose…" button that opens `ThemePopoverController` anchored to it; a pick calls `sessionController.setTheme(slug)`. In `DeckFormViewController.makeControl`, the `case "string" where !key.values.isEmpty` branch gains `if path == ["theme"] { return themeRowControl(path: path) }` once signed off, where `themeRowControl` is an `NSButton` titled with the current theme's name whose action shows the popover; `refreshValues` sets the button's title from the frontmatter's value. Until sign-off the popup stays and `testDeckSettingsLiveInTheInspector` (D5) keeps passing.

- [ ] **Step 6: Build, then hand the hosted tests to CI**

Run: `make -C desktop build && make -C desktop test-build 2>&1 | tail -3`
Expected: both succeed. The controller's CI run confirms: `NewDeckTests` (3) pass; D2's `WelcomeTests.testWelcomeWindow` still passes (it reads the button's title, now enabled); D2's `UntitledLaunchTests` still pass (`applicationShouldOpenUntitledFile` is unchanged: Cmd+N is a sheet, not an untitled document).

- [ ] **Step 7: Mutations and commit**

Mutations, each a patch in `mutations-b/`: in `NewDeckRequest.arguments`, drop `--folder` and pass `--output` with the location (`Test: TapTests/NewDeckTests/testNewDeck`; expected: fails on `arguments`, and tap would refuse); in `newDeck`, skip `lastNewDeckFolder = request.location` (expected: fails on "remembered"); in `newDeck`, end the sheet on failure too (`Test: .../testTheSheetGoesOnTheKeyDeckWindowAndReportsTapsError`; expected: fails on "the sheet stays"); in `titleChanged`, enable Create always (`Test: .../testAnEmptyTitleCannotCreate`; expected: fails); in `NewDeckSheet.init`, leave `grid.showsFooter` true (expected: `testNewDeck` fails on `showsFooter`); in `newDeck`, put the sheet on the welcome window even with a deck open (expected: `testTheSheetGoesOnTheKeyDeckWindowAndReportsTapsError` times out on the deck window's sheet).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the New Deck sheet over tap new --folder"
```

---

### Task 8: Images: paste and drop through `tap image add`, Generate Image, and Regenerate

**Files:**
- Modify: `desktop/Tap/Editor/EditorTextView.swift` (`paste(_:)`, `imagePasteboard`, `draggingEntered`, `performDragOperation`, `imageFileURLs(on:)`, the delegate method)
- Modify: `desktop/Tap/Documents/DeckSessionController.swift` (`insertAtCaret`, `insertImages`, `generateImage`, `regenerateImage`, `aiImagesOnCurrentSlide`, `editor(_:insertImages:)`)
- Create: `desktop/Tap/Images/GenerateImageSheet.swift`
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (`insertImage(_:)`, `generateImage(_:)`, `regenerateImage(_:)`, `showFormSheet`, validation)
- Modify: `desktop/Tap/App/MainMenu.swift`, `desktop/Tap/Slides/SlideContextMenu.swift` (the actions; Regenerate items wait for a mockup)
- Create: `desktop/TapTests/Fixtures/ai-image/talk.md`, `desktop/TapTests/Fixtures/ai-image/images/generated-00000000.png`
- Test: `desktop/TapTests/ImageInsertTests.swift`, `desktop/TapTests/GenerateImageTests.swift`

**Interfaces:**
- Consumes: `AddedImageResult`, `GeneratedImageResult`, `AIImageReference` (Task 4), `runToolOnSavedDeck`, `showToolError`, `TapTool.run` (Task 6), `QuestionSheet`, `EditorTextView.replaceText`, `selectedRange()`, `boxes`, `currentBoxIndex`, `Slide.startLine`/`endLine` (D2), `FakeToolScripts.write` (Task 6).
- Produces: `EditorTextView.imagePasteboard`, `EditorTextView.imageFileURLs(on:)`, `EditorTextViewDelegate.editor(_:insertImages:)`; `DeckSessionController.insertAtCaret(_:actionName:)`, `insertImages(_:)`, `generateImage(prompt:aspect:matchTheme:)`, `regenerateImage(path:)`, `aiImagesOnCurrentSlide`, `pastedImageFolder`; `GenerateImageSheet` (`promptView`, `matchThemeSwitch`, `aspectControl`, `generateButton`, `request`, `showError`); `DeckWindowController.insertImage(_:)`, `generateImage(_:)`, `regenerateImage(_:)`, `showFormSheet(_:)`, `openPanelForImages` seam. Task 9 reuses `insertAtCaret` and `showFormSheet`.

- [ ] **Step 1: The fixture**

`desktop/TapTests/Fixtures/ai-image/talk.md`:

```markdown
---
title: AI Image
---

# One

---

# Two

<!-- ai-prompt: a fox at dusk -->
![](images/generated-00000000.png)

---

# Three
```

and `ai-image/images/generated-00000000.png`, the same one-pixel PNG as `diagram.png` (`cp desktop/TapTests/Fixtures/diagram.png desktop/TapTests/Fixtures/ai-image/images/generated-00000000.png`).

- [ ] **Step 2: Write the failing tests**

`desktop/TapTests/ImageInsertTests.swift`:

```swift
import XCTest
@testable import Tap
@testable import TapDesktopCore

final class ImageInsertTests: HostedTestCase {
    let diagram = Fixtures.repositoryRoot.appendingPathComponent("desktop/TapTests/Fixtures/diagram.png")

    func openOps() async throws -> (DeckDocument, DeckSessionController, URL) {
        let deck = try Fixtures.copyDeck("ops.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        return (document, controller, deck)
    }

    func testPasteAnImage() async throws {
        let (document, controller, deck) = try await openOps()
        controller.jumpToSlide(number: 3)
        let caret = controller.editor.selectedRange().location
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.image.\(UUID().uuidString)"))
        controller.editor.imagePasteboard = pasteboard
        pasteboard.clearContents()
        pasteboard.writeObjects([diagram as NSURL])

        controller.editor.paste(nil)
        try await waitUntil(timeout: 20, "the markdown") { controller.editor.string.contains("images/diagram.png") }
        let imagesFolder = deck.deletingLastPathComponent().appendingPathComponent("images")
        XCTAssertTrue(FileManager.default.fileExists(atPath: imagesFolder.appendingPathComponent("diagram.png").path), "tap copied it next to the deck")
        let text = controller.editor.string as NSString
        XCTAssertEqual(text.range(of: "![diagram](images/diagram.png)").location, caret, "inserted at the caret, tap's markdown as printed")
        XCTAssertEqual(controller.currentSlideNumber, 3)
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Insert Image")
        XCTAssertTrue(document.isDocumentEdited, "the buffer changed; tap did not touch the deck file")

        // A second paste of the same file: tap names it diagram-2.png.
        controller.editor.paste(nil)
        try await waitUntil(timeout: 20, "the second markdown") { controller.editor.string.contains("images/diagram-2.png") }
        XCTAssertTrue(FileManager.default.fileExists(atPath: imagesFolder.appendingPathComponent("diagram-2.png").path))

        // Not an image: tap refuses, the app shows tap's words and inserts nothing.
        let notes = try Fixtures.temporaryFolder().appendingPathComponent("notes.txt")
        try "not an image".write(to: notes, atomically: true, encoding: .utf8)
        let before = controller.editor.string
        controller.insertImages([notes])
        try await waitUntil(timeout: 20, "tap's refusal") { controller.editorViewController.barStack.arrangedSubviews.contains { ($0 as? DocumentBarView)?.kind == .toolFailed } }
        XCTAssertEqual(controller.editor.string, before)

        controller.editor.undoManager?.undo()
        controller.editor.undoManager?.undo()
        XCTAssertFalse(controller.editor.string.contains("images/diagram"), "two undo steps take both inserts back")
    }

    func testPastedImageDataBecomesAFile() async throws {
        let (_, controller, deck) = try await openOps()
        controller.jumpToSlide(number: 2)
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.imagedata.\(UUID().uuidString)"))
        controller.editor.imagePasteboard = pasteboard
        pasteboard.clearContents()
        let image = try XCTUnwrap(NSImage(contentsOf: diagram))
        pasteboard.writeObjects([image])

        controller.editor.paste(nil)
        try await waitUntil(timeout: 20, "the markdown") { controller.editor.string.contains("images/pasted-image.png") }
        XCTAssertTrue(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().appendingPathComponent("images/pasted-image.png").path))
    }

    func testPlainTextPasteIsStillText() async throws {
        let (_, controller, _) = try await openOps()
        controller.jumpToSlide(number: 2)
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.text.\(UUID().uuidString)"))
        controller.editor.imagePasteboard = pasteboard
        pasteboard.clearContents()
        pasteboard.setString("plain words", forType: .string)
        controller.editor.paste(nil)
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertTrue(controller.editor.string.contains("plain words"), "NSTextView's own paste")
    }

    func testDroppedImageFilesAreRecognised() async throws {
        let (_, controller, _) = try await openOps()
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.drop.\(UUID().uuidString)"))
        pasteboard.clearContents()
        pasteboard.writeObjects([diagram as NSURL, URL(fileURLWithPath: "/tmp/notes.txt") as NSURL])
        XCTAssertEqual(EditorTextView.imageFileURLs(on: pasteboard), [diagram], "only files with an extension tap accepts")
        let info = FakeDraggingInfo(pasteboard: pasteboard, location: .zero)
        XCTAssertEqual(controller.editor.draggingEntered(info), .copy)
    }

    func testPasteIntoAnUnsavedDeckIsRefused() async throws {
        let (_, controller, deck) = try await openOps()
        try FileManager.default.removeItem(at: deck)
        try await waitUntil(timeout: 10, "the deck to read as gone") { controller.document?.fileURL == nil || !FileManager.default.fileExists(atPath: deck.path) }
        controller.insertImages([diagram])
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertFalse(controller.editor.string.contains("images/diagram.png"))
        XCTAssertTrue(controller.session.log.text.contains("Insert Image needs a saved deck"), controller.session.log.text)
    }
}
```

`FakeDraggingInfo(pasteboard:location:)` is D3's test double (`Support/FakeDraggingInfo.swift`); if its initializer differs, use its own.

`desktop/TapTests/GenerateImageTests.swift`:

```swift
import XCTest
@testable import Tap
@testable import TapDesktopCore

final class GenerateImageTests: HostedTestCase {
    /// A scripted tap image generate: appends tap's pair to the deck file
    /// (the end of the last slide) and prints tap's result. It records
    /// whether GEMINI_API_KEY was set, never its value.
    func fakeGenerate(recordingTo record: URL) throws -> URL {
        try FakeToolScripts.write("""
          "image generate")
            deck="$3"
            printf '\\n<!-- ai-prompt: a fox at dusk -->\\n![](images/generated-1a2b3c4d.png)\\n' >> "$deck"
            printf '{"ok": true, "deck": "%s", "slide": 7, "image": "images/generated-1a2b3c4d.png", "prompt": "a fox at dusk", "markdown": "<!-- ai-prompt: a fox at dusk -->\\\\n![](images/generated-1a2b3c4d.png)"}\\n' "$deck"
            exit 0 ;;
          "image regenerate")
            deck="$3"
            sed -i '' 's/generated-00000000/generated-ffffffff/' "$deck"
            printf '{"ok": true, "deck": "%s", "slide": 2, "image": "images/generated-ffffffff.png", "prompt": "a fox at dusk", "markdown": "m", "replaced": "images/generated-00000000.png"}\\n' "$deck"
            exit 0 ;;
        """, recordingTo: record)
    }

    func testGenerateAnImageWithAI() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try fakeGenerate(recordingTo: record)
        AppEnvironment.shared.geminiKeyStore = MemoryGeminiKeyStore(key: "placeholder-not-a-secret")
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        controller.jumpToSlide(number: 7)

        window.generateImage(nil)
        try await waitUntil(timeout: 5, "the sheet") { window.window?.attachedSheet is GenerateImageSheet }
        let sheet = try XCTUnwrap(window.window?.attachedSheet as? GenerateImageSheet)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Generate Image for Slide 7")
        XCTAssertFalse(sheet.generateButton.isEnabled, "no prompt yet")
        sheet.promptView.string = "a fox at dusk"
        sheet.textDidChange(Notification(name: NSText.didChangeNotification, object: sheet.promptView))
        XCTAssertTrue(sheet.generateButton.isEnabled)
        XCTAssertEqual(sheet.aspectControl.label(forSegment: sheet.aspectControl.selectedSegment), "16:9")
        XCTAssertEqual(sheet.matchThemeSwitch.state, .on)
        XCTAssertEqual(sheet.request.arguments(deck: deck, slide: 7), ["image", "generate", deck.path, "--slide", "7", "--prompt", "a fox at dusk", "--aspect", "16:9", "--match-theme", "--json"])
        sheet.generateButton.performClick(nil)

        try await waitUntil(timeout: 20, "tap's edit to land") { controller.editor.string.contains("generated-1a2b3c4d.png") }
        XCTAssertNil(window.window?.attachedSheet)
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertTrue(recorded.contains("arguments: image generate \(deck.path) --slide 7 --prompt a fox at dusk --aspect 16:9 --match-theme --json"), recorded)
        XCTAssertTrue(recorded.contains("gemini: set"), "the Keychain's key reached tap as GEMINI_API_KEY")
        XCTAssertFalse(recorded.contains("placeholder-not-a-secret"), "the fake records that a key was set, never the value")
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Generate Image")
        XCTAssertEqual(controller.currentSlideNumber, 7)
        XCTAssertFalse(document.isDocumentEdited)
    }

    func testRegenerateAnAIImage() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try fakeGenerate(recordingTo: record)
        let deck = try Fixtures.copyDeck("ai-image")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 3)
        let controller = try XCTUnwrap(document.sessionController)
        controller.jumpToSlide(number: 2)
        XCTAssertEqual(controller.aiImagesOnCurrentSlide.map(\.imagePath), ["images/generated-00000000.png"])
        controller.jumpToSlide(number: 1)
        XCTAssertEqual(controller.aiImagesOnCurrentSlide, [], "slide 1 has none")
        controller.jumpToSlide(number: 2)

        controller.regenerateImage(path: "images/generated-00000000.png")
        try await waitUntil(timeout: 20, "the replacement") { controller.editor.string.contains("generated-ffffffff.png") }
        XCTAssertFalse(controller.editor.string.contains("generated-00000000.png"), "replaced in place")
        XCTAssertTrue(try String(contentsOf: record, encoding: .utf8).contains("arguments: image regenerate \(deck.path) --slide 2 --image images/generated-00000000.png --json"))
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Regenerate Image")
    }

    func testNoKeyShowsTapsMessageWithASettingsButton() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.write("""
          "image generate")
            printf '{"ok": false, "error": {"code": "no_api_key", "message": "cannot start image generation: GEMINI_API_KEY is not set"}}\\n'
            exit 1 ;;
        """, recordingTo: record)
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let window = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        window.generateImage(nil)
        try await waitUntil(timeout: 5, "the sheet") { window.window?.attachedSheet is GenerateImageSheet }
        let sheet = try XCTUnwrap(window.window?.attachedSheet as? GenerateImageSheet)
        sheet.promptView.string = "x"
        sheet.textDidChange(Notification(name: NSText.didChangeNotification, object: sheet.promptView))
        sheet.generateButton.performClick(nil)
        try await waitUntil(timeout: 20, "tap's error") { !sheet.errorLabel.isHidden }
        XCTAssertTrue(sheet.errorLabel.stringValue.contains("GEMINI_API_KEY is not set"))
        XCTAssertFalse(sheet.settingsButton.isHidden, "the way to the Image Generation pane")
        XCTAssertTrue(try String(contentsOf: record, encoding: .utf8).contains("gemini: \n"), "no key was set")
        sheet.cancelButton.performClick(nil)
    }
}
```

- [ ] **Step 3: Build to verify they do not compile**

Run: `make -C desktop test-build 2>&1 | tail -3`
Expected: the build fails on `imagePasteboard`, `insertImages`, `GenerateImageSheet`.

- [ ] **Step 4: Paste and drop in the editor**

In `EditorTextViewDelegate`, add `func editor(_ editor: EditorTextView, insertImages files: [URL])` with an empty default. In `EditorTextView`:

```swift
    /// What Paste reads: the general pasteboard, unless a test replaces it
    /// so a run never touches the person's clipboard.
    var imagePasteboard: NSPasteboard = .general

    /// The file URLs on `pasteboard` whose extension tap image add accepts,
    /// in the pasteboard's order. Other files are not images to tap.
    static let imageExtensions: Set<String> = ["png", "jpg", "jpeg", "gif", "webp", "svg", "avif"]

    static func imageFileURLs(on pasteboard: NSPasteboard) -> [URL] {
        let urls = pasteboard.readObjects(forClasses: [NSURL.self], options: [.urlReadingFileURLsOnly: true]) as? [URL] ?? []
        return urls.filter { imageExtensions.contains($0.pathExtension.lowercased()) }
    }

    /// Paste: image files and image data go to tap image add through the
    /// delegate; everything else is NSTextView's own paste.
    override func paste(_ sender: Any?) {
        let files = Self.imageFileURLs(on: imagePasteboard)
        if !files.isEmpty {
            editorDelegate?.editor(self, insertImages: files)
            return
        }
        if let image = imagePasteboard.readObjects(forClasses: [NSImage.self], options: nil)?.first as? NSImage,
           imagePasteboard.availableType(from: [.string]) == nil,
           let file = Self.writePastedImage(image) {
            editorDelegate?.editor(self, insertImages: [file])
            return
        }
        super.paste(sender)
    }

    /// Image data (a screenshot) as a PNG file for tap to copy: tap keeps
    /// the name, so a second paste becomes pasted-image-2.png.
    static func writePastedImage(_ image: NSImage) -> URL? {
        guard let tiff = image.tiffRepresentation, let bitmap = NSBitmapImageRep(data: tiff), let png = bitmap.representation(using: .png, properties: [:]) else { return nil }
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-pasted-\(UUID().uuidString)")
        let file = folder.appendingPathComponent("pasted-image.png")
        do {
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
            try png.write(to: file)
        } catch {
            return nil
        }
        return file
    }
```

In `draggingEntered`, before `guard let payload = slidePayload(sender)`: `if !Self.imageFileURLs(on: sender.draggingPasteboard).isEmpty { return .copy }`, and the same in `draggingUpdated`. In `performDragOperation`, before the slide payload guard:

```swift
        let images = Self.imageFileURLs(on: sender.draggingPasteboard)
        if !images.isEmpty {
            let point = convert(sender.draggingLocation, from: nil)
            setSelectedRange(NSRange(location: min(characterIndexForInsertion(at: point), (string as NSString).length), length: 0))
            editorDelegate?.editor(self, insertImages: images)
            return true
        }
```

`prepareForDragOperation` already returns true for every drop (D3).

- [ ] **Step 5: The session controller's image paths**

In `DeckSessionController.swift`:

```swift
    /// Inserts text at the caret as one undo step named `actionName`, the
    /// path for what tap printed (an image's markdown, a component's
    /// snippet). The caret is clamped out of the frontmatter already.
    func insertAtCaret(_ text: String, actionName: String) {
        let caret = editor.selectedRange()
        editor.replaceText(in: NSRange(location: caret.location, length: 0), with: text, actionName: actionName)
        editor.setSelectedRange(NSRange(location: caret.location + (text as NSString).length, length: 0))
    }

    /// Pasted or dropped image files: tap image add copies each into
    /// images/ next to the deck (its own name rules, -2 on a clash) and
    /// prints the markdown, which goes in at the caret, one undo step per
    /// image. tap does not touch the deck file, so nothing is saved or
    /// reloaded. A deck with no file yet has no images/ to copy into.
    func insertImages(_ files: [URL]) {
        guard let deck = document?.fileURL, FileManager.default.fileExists(atPath: deck.path) else {
            session.log.append("Insert Image needs a saved deck: tap image add copies next to the deck file", source: .app)
            NSSound.beep()
            return
        }
        Task { @MainActor [weak self] in
            for file in files {
                guard let self else { return }
                let exit = await TapTool.run(["image", "add", file.path, deck.path, "--json"], in: deck.deletingLastPathComponent(), log: self.session.log)
                switch exit.outcome {
                case .ok?:
                    guard let added = try? exit.outcome?.result(AddedImageResult.self) else { continue }
                    self.insertAtCaret(added.markdown + "\n", actionName: "Insert Image")
                case .failed(_, let message)?:
                    self.showToolError(actionName: "Insert Image", message: message)
                case nil:
                    self.showToolError(actionName: "Insert Image", message: "tap did not answer (exit \(exit.status)); see the Tap Log")
                }
            }
        }
    }

    func editor(_ editor: EditorTextView, insertImages files: [URL]) {
        insertImages(files)
    }

    /// Slide > Generate Image: tap image generate on the saved file adds
    /// the image and its ai-prompt comment to the caret's slide.
    func generateImage(prompt: String, aspect: String?, matchTheme: Bool) {
        guard let deck = document?.fileURL, let slide = currentSlideNumber else { return }
        var arguments = ["image", "generate", deck.path, "--slide", String(slide), "--prompt", prompt]
        if let aspect { arguments += ["--aspect", aspect] }
        if matchTheme { arguments.append("--match-theme") }
        arguments.append("--json")
        runToolOnSavedDeck(arguments, actionName: "Generate Image")
    }

    /// The AI images of the caret's slide, from the buffer's text within
    /// tap's line range for that slide, for the Regenerate menu items.
    var aiImagesOnCurrentSlide: [AIImageReference] {
        guard let index = editor.currentBoxIndex, editor.boxes.indices.contains(index) else { return [] }
        return AIImageReference.find(in: editor.string, slideRange: editor.boxes[index].range)
    }

    /// Regenerate on one of the slide's AI images: tap replaces it in
    /// place and deletes the old file.
    func regenerateImage(path: String) {
        guard let deck = document?.fileURL, let slide = currentSlideNumber else { return }
        runToolOnSavedDeck(["image", "regenerate", deck.path, "--slide", String(slide), "--image", path, "--json"], actionName: "Regenerate Image")
    }
```

`editor.boxes[index].range` is D2's `SlideBox.range` (the box's character range); if the box holds line numbers instead, convert with the editor's line-to-range helper D2 uses for drawing.

- [ ] **Step 6: The Generate Image sheet (the GenerateImage board)**

The board draws a sheet titled "Generate Image for Slide 5": a "Describe the image" text area, a "Style" row with a "Match theme" switch and the hint "Uses tap theme show --prompt for the Terminal style.", an "Aspect" row with 16:9, 1:1 and 4:3 (16:9 chosen), the line "Runs tap image generate. The image is saved to images/ and added to the end of slide 5 with its prompt.", and Cancel and Generate. `desktop/Tap/Images/GenerateImageSheet.swift`:

```swift
import AppKit

struct GenerateImageRequest: Equatable {
    var prompt: String
    var aspect: String?
    var matchTheme: Bool

    func arguments(deck: URL, slide: Int) -> [String] {
        var arguments = ["image", "generate", deck.path, "--slide", String(slide), "--prompt", prompt]
        if let aspect { arguments += ["--aspect", aspect] }
        if matchTheme { arguments.append("--match-theme") }
        return arguments + ["--json"]
    }
}

/// Slide > Generate Image: the prompt, Match theme and the aspect, then
/// tap image generate on the saved deck. tap's error shows in the sheet,
/// with a way to Settings for a missing key.
final class GenerateImageSheet: QuestionSheet, NSTextViewDelegate {
    let promptView = NSTextView()
    let matchThemeSwitch = NSSwitch()
    let aspectControl = NSSegmentedControl(labels: ["16:9", "1:1", "4:3"], trackingMode: .selectOne, target: nil, action: nil)
    let errorLabel = NSTextField(wrappingLabelWithString: "")
    let settingsButton = NSButton(title: "Settings…", target: nil, action: #selector(AppDelegate.showSettings(_:)))
    var onGenerate: ((GenerateImageRequest) -> Void)?
    var generateButton: NSButton { acceptButton }
    var cancelButton: NSButton { declineButton }

    var request: GenerateImageRequest {
        GenerateImageRequest(prompt: promptView.string.trimmingCharacters(in: .whitespacesAndNewlines),
                             aspect: aspectControl.label(forSegment: aspectControl.selectedSegment), matchTheme: matchThemeSwitch.state == .on)
    }

    init(slide: Int, themeName: String) {
        let form = NSView()
        super.init(kind: "generate-image", title: "Generate Image for Slide \(slide)", body: "", path: nil, decline: "Cancel", accept: "Generate", escape: .decline, returnAnswer: .accept, detail: form)
        bodyLabel.isHidden = true
        acceptButton.target = self
        acceptButton.action = #selector(generatePressed(_:))
        let scroll = NSScrollView()
        scroll.documentView = promptView
        scroll.hasVerticalScroller = true
        scroll.borderType = .bezelBorder
        promptView.isRichText = false
        promptView.font = .systemFont(ofSize: 13)
        promptView.delegate = self
        promptView.setAccessibilityIdentifier("generate-image-prompt")
        scroll.heightAnchor.constraint(equalToConstant: 72).isActive = true
        matchThemeSwitch.state = .on
        matchThemeSwitch.setAccessibilityIdentifier("generate-image-match-theme")
        aspectControl.selectedSegment = 0
        aspectControl.setAccessibilityIdentifier("generate-image-aspect")
        errorLabel.textColor = .systemRed
        errorLabel.font = .systemFont(ofSize: 12)
        errorLabel.isHidden = true
        settingsButton.isHidden = true
        settingsButton.bezelStyle = .rounded
        let describe = NSTextField(labelWithString: "Describe the image")
        describe.font = .systemFont(ofSize: 11, weight: .semibold)
        describe.textColor = .secondaryLabelColor
        let styleHint = NSTextField(labelWithString: "Uses tap theme show --prompt for the \(themeName) style.")
        styleHint.font = .systemFont(ofSize: 11)
        styleHint.textColor = .secondaryLabelColor
        let note = NSTextField(wrappingLabelWithString: "Runs tap image generate. The image is saved to images/ and added to the end of slide \(slide) with its prompt.")
        note.font = .systemFont(ofSize: 11.5)
        note.textColor = .secondaryLabelColor
        let stack = NSStackView(views: [describe, scroll, row("Style", "Match theme", matchThemeSwitch, hint: styleHint), row("Aspect", nil, aspectControl), note, errorLabel, settingsButton])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.translatesAutoresizingMaskIntoConstraints = false
        form.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: form.topAnchor), stack.bottomAnchor.constraint(equalTo: form.bottomAnchor),
            stack.leadingAnchor.constraint(equalTo: form.leadingAnchor), stack.trailingAnchor.constraint(equalTo: form.trailingAnchor),
            scroll.widthAnchor.constraint(equalTo: stack.widthAnchor), note.widthAnchor.constraint(equalTo: stack.widthAnchor),
            errorLabel.widthAnchor.constraint(equalTo: stack.widthAnchor),
        ])
        setContentSize(contentView?.fittingSize ?? frame.size)
        generateButton.isEnabled = false
    }

    private func row(_ title: String, _ controlTitle: String?, _ control: NSView, hint: NSTextField? = nil) -> NSView {
        let label = NSTextField(labelWithString: title)
        label.font = .systemFont(ofSize: 11, weight: .semibold)
        label.textColor = .secondaryLabelColor
        var right: [NSView] = []
        if let controlTitle { right.append(NSTextField(labelWithString: controlTitle)) }
        right.append(control)
        let controls = NSStackView(views: right)
        controls.spacing = 8
        var lines: [NSView] = [NSStackView(views: [label, NSView(), controls])]
        (lines[0] as? NSStackView)?.orientation = .horizontal
        if let hint { lines.append(hint) }
        let stack = NSStackView(views: lines)
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 2
        return stack
    }

    func textDidChange(_ notification: Notification) {
        generateButton.isEnabled = !request.prompt.isEmpty
        errorLabel.isHidden = true
        settingsButton.isHidden = true
    }

    @objc private func generatePressed(_ sender: Any?) {
        generateButton.isEnabled = false
        onGenerate?(request)
    }

    /// tap's message; the Settings button for a missing key.
    func showError(_ message: String, code: String) {
        errorLabel.stringValue = message
        errorLabel.isHidden = false
        settingsButton.isHidden = code != "no_api_key"
        generateButton.isEnabled = true
    }
}
```

`AppDelegate.showSettings(_:)` arrives in Task 12; until then the selector is declared there as a stub that does nothing (`@objc func showSettings(_ sender: Any?) {}`), replaced in Task 12.

- [ ] **Step 7: The window's actions and the menus**

In `DeckWindowController.swift`:

```swift
    /// A form sheet (Generate Image, New Component, Export) on this window:
    /// refused with a beep while another sheet is up, so no two sheets
    /// queue on the window.
    func showFormSheet(_ sheet: NSWindow) {
        guard let window, window.attachedSheet == nil, questionSheet == nil else {
            NSSound.beep()
            return
        }
        window.beginSheet(sheet) { _ in }
    }

    /// Slide > Insert Image and the context menu's: a file chooser, then tap image add.
    var openPanelForImages: (@escaping ([URL]) -> Void) -> Void = { completion in
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = true
        panel.canChooseDirectories = false
        panel.allowedContentTypes = [.png, .jpeg, .gif, .webP, .svg]
        panel.begin { response in completion(response == .OK ? panel.urls : []) }
    }

    @objc func insertImage(_ sender: Any?) {
        openPanelForImages { [weak self] files in
            guard !files.isEmpty else { return }
            self?.sessionController.insertImages(files)
        }
    }

    @objc func generateImage(_ sender: Any?) {
        guard let slide = sessionController.currentSlideNumber, let deck = sessionController.document?.fileURL else { return NSSound.beep() }
        let themeName = sessionController.currentThemeSlug.map { AppEnvironment.shared.themeImages.catalog?.name(forSlug: $0) ?? $0 } ?? "Base"
        let sheet = GenerateImageSheet(slide: slide, themeName: themeName)
        sheet.onGenerate = { [weak self, weak sheet] request in
            guard let self, let sheet else { return }
            self.sessionController.runToolOnSavedDeck(request.arguments(deck: deck, slide: slide), actionName: "Generate Image") { [weak self, weak sheet] outcome in
                guard let sheet else { return }
                switch outcome {
                case .ok?: self?.window?.endSheet(sheet, returnCode: .OK)
                case .failed(let code, let message)?: sheet.showError(message, code: code)
                case nil: sheet.showError("tap did not answer; see the Tap Log", code: "failed")
                }
            }
        }
        showFormSheet(sheet)
    }

    /// The context menu's Regenerate Image… items name the image in `representedObject`.
    @objc func regenerateImage(_ sender: Any?) {
        guard let path = (sender as? NSMenuItem)?.representedObject as? String else { return NSSound.beep() }
        sessionController.regenerateImage(path: path)
    }
```

`runToolOnSavedDeck`'s `showToolError` on a failure would also put a bar up under the sheet; give `runToolOnSavedDeck` a `showsErrorBar: Bool = true` parameter and pass `false` from the sheet's path, so the sheet is the one place the error shows. In `validateMenuItem`: `insertImage` and `generateImage` need `sessionController.currentSlideNumber != nil && sessionController.document?.fileURL != nil`. In `MainMenu.slideMenu`, the three items get their actions: `#selector(DeckWindowController.insertImage(_:))`, `#selector(DeckWindowController.generateImage(_:))`, and New Component… stays `nil` until Task 9. In `SlideContextMenu.build`, `add("Generate Image…", #selector(DeckWindowController.generateImage(_:)))` and `add("Insert Image…", #selector(DeckWindowController.insertImage(_:)), key: showsTextShortcuts ? "i" : "", modifiers: [.command, .shift])`.

- [ ] **Step 8: The Regenerate entry point: waits for the person's mockup sign-off (the controller records it in the ledger)**

Proposed, for the mockup: in `SlideContextMenu.build`, after Generate Image…, one item per AI image of the box's slide, "Regenerate Image…" (or "Regenerate <file name>…" when the slide has more than one), with the image's path as `representedObject` and `regenerateImage(_:)` as the action; `build` gains an `aiImages: [AIImageReference]` parameter the two callers fill from `sessionController.aiImagesOnCurrentSlide` (the editor's header menu) and from the panel's slide (the thumbnail menu; the panel knows the slide number and the session controller finds the pairs in that slide's range). The run path (`regenerateImage(path:)`) and its test are built in Step 5 regardless.

- [ ] **Step 9: Build, then hand the hosted tests to CI**

Run: `make -C desktop build && make -C desktop test-build 2>&1 | tail -3`
Expected: both succeed. The controller's CI run confirms: `ImageInsertTests` (5) and `GenerateImageTests` (3) pass; D3's `DragAndDropTests` and `SlideDragUITests` still pass (a slide drop's pasteboard holds no file URLs, so `imageFileURLs` is empty there); D3's `SlideContextMenuTests` still pass (the two items keep their titles).

- [ ] **Step 10: Mutations and commit**

Mutations, each a patch in `mutations-b/`, the ones that could lose text or run tap on an unsaved deck first: in `insertAtCaret`, replace the selection's whole range instead of an empty range at its start (`Test: TapTests/ImageInsertTests/testPasteAnImage`; expected: fails on the caret location when text is selected: add `controller.editor.setSelectedRange(NSRange(location: caret, length: 3))` before the first paste in the test so this mutation has teeth, and assert the three characters survive); in `insertImages`, run tap with no deck path (expected: tap resolves the deck from the cwd; the test's copy is alone in its folder, so tap finds it: into `survivors-b/` with that reason, or pass `in:` as `/` in the mutation so it fails); in `insertImages`, drop the `fileExists` guard (`Test: .../testPasteIntoAnUnsavedDeckIsRefused`; expected: fails on the log line); in `paste`, send image data to the delegate even when the pasteboard has a string (`Test: .../testPlainTextPasteIsStillText`; into `survivors-b/` unless the test's pasteboard also carries an image; the RTF-with-image case is noted, not tested); in `imageFileURLs`, drop the extension filter (`Test: .../testDroppedImageFilesAreRecognised`; expected: fails on the notes file); in `generateImage`, drop `--match-theme` (`Test: TapTests/GenerateImageTests/testGenerateAnImageWithAI`; expected: fails on `arguments`); in `GenerateImageSheet.showError`, hide the Settings button always (`Test: .../testNoKeyShowsTapsMessageWithASettingsButton`; expected: fails); in `aiImagesOnCurrentSlide`, search the whole text (`Test: .../testRegenerateAnAIImage`; expected: slide 1 lists the fox); in `AppEnvironment.tapEnvironment`, skip `GeminiKeySource.apply` (expected: `testGenerateAnImageWithAI` fails on "gemini: set").

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): images through tap image add, generate and regenerate"
```

---

### Task 9: Components: New Component, a Cmd-click that opens the file, and the build error on the box

**Files:**
- Create: `desktop/Tap/Components/NewComponentSheet.swift`
- Modify: `desktop/Tap/Documents/DeckSessionController.swift` (`createComponent`, `openInEditor`, `openComponentLink(at:)`)
- Modify: `desktop/Tap/Editor/EditorTextView.swift` (the Cmd-click in `mouseDown`, `componentLink(at:)`, the delegate method)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (`newComponent(_:)`, validation)
- Modify: `desktop/Tap/App/MainMenu.swift` (New Component… gets its action)
- Create: `desktop/TapTests/Fixtures/broken-component/talk.md`, `desktop/TapTests/Fixtures/broken-component/slides/Broken.jsx`
- Test: `desktop/TapTests/ComponentTests.swift`; modify `desktop/TapTests/SlideMenuTests.swift`

**Interfaces:**
- Consumes: `ComponentScaffold`, `ComponentLink` (Task 4), `insertAtCaret`, `showFormSheet`, `TapTool.run` (Tasks 6, 8), `EditorTextView.boxes`, `header(forBoxAt:)`, `BoxHeader.errors` (D2, D5).
- Produces: `NewComponentSheet` (`nameField`, `kindControl`, `typeScriptSwitch`, `createButton`, `request`, `NewComponentRequest(name:inline:typeScript:)` with `arguments(deck:)`, `showError`); `DeckSessionController.createComponent(_:completion:)`, `openInEditor: (URL) -> Void`, `openComponentLink(at:) -> Bool`; `EditorTextViewDelegate.editor(_:openComponentLinkAt:)`; `DeckWindowController.newComponent(_:)`.

- [ ] **Step 1: The fixture and the failing tests**

`desktop/TapTests/Fixtures/broken-component/talk.md`:

```markdown
---
title: Broken Component
---

# One

---

<!--
layout: ./slides/Broken.jsx
-->

# Two
```

`desktop/TapTests/Fixtures/broken-component/slides/Broken.jsx`:

```jsx
export default function Broken() {
  return <div>never closed
}
```

`desktop/TapTests/ComponentTests.swift`:

```swift
import XCTest
@testable import Tap
@testable import TapDesktopCore

final class ComponentTests: HostedTestCase {
    func testCreateAComponent() async throws {
        let deck = try Fixtures.copyDeck("ops.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        var opened: [URL] = []
        controller.openInEditor = { opened.append($0) }
        controller.jumpToSlide(number: 4)
        let caret = controller.editor.selectedRange().location

        window.newComponent(nil)
        try await waitUntil(timeout: 5, "the sheet") { window.window?.attachedSheet is NewComponentSheet }
        let sheet = try XCTUnwrap(window.window?.attachedSheet as? NewComponentSheet)
        XCTAssertFalse(sheet.createButton.isEnabled, "no name yet")
        sheet.nameField.stringValue = "Counter"
        sheet.nameChanged(sheet.nameField)
        XCTAssertTrue(sheet.createButton.isEnabled)
        sheet.nameField.stringValue = "counter"
        sheet.nameChanged(sheet.nameField)
        XCTAssertFalse(sheet.createButton.isEnabled, "tap wants PascalCase; the sheet says so before tap has to")
        sheet.nameField.stringValue = "Counter"
        sheet.nameChanged(sheet.nameField)
        XCTAssertEqual(sheet.request.arguments(deck: deck), ["component", "new", "Counter", deck.path, "--json"])
        sheet.createButton.performClick(nil)

        try await waitUntil(timeout: 20, "tap's snippet") { controller.editor.string.contains("layout: ./slides/Counter.jsx") }
        XCTAssertNil(window.window?.attachedSheet)
        let file = deck.deletingLastPathComponent().appendingPathComponent("slides/Counter.jsx")
        XCTAssertTrue(FileManager.default.fileExists(atPath: file.path), "tap wrote the component")
        XCTAssertEqual(opened.map(\.path), [file.path], "opened in the default code editor")
        let text = controller.editor.string as NSString
        XCTAssertEqual(text.range(of: "<!--\nlayout: ./slides/Counter.jsx\n-->").location, caret, "tap's snippet, at the caret")
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "New Component")
        XCTAssertEqual(controller.currentSlideNumber, 4)

        // Inline and TypeScript: the flags, and tap's snippet for a block component.
        window.newComponent(nil)
        try await waitUntil(timeout: 5, "the second sheet") { window.window?.attachedSheet is NewComponentSheet }
        let second = try XCTUnwrap(window.window?.attachedSheet as? NewComponentSheet)
        second.nameField.stringValue = "LatencyDrop"
        second.nameChanged(second.nameField)
        second.kindControl.selectedSegment = 1
        second.typeScriptSwitch.state = .on
        XCTAssertEqual(second.request.arguments(deck: deck), ["component", "new", "LatencyDrop", deck.path, "--inline", "--ts", "--json"])
        second.createButton.performClick(nil)
        try await waitUntil(timeout: 20, "the inline snippet") { controller.editor.string.contains("```component ./components/LatencyDrop.tsx") }
        XCTAssertTrue(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().appendingPathComponent("components/LatencyDrop.tsx").path))
        XCTAssertEqual(opened.count, 2)

        // A name tap refuses (the file exists now): tap's message in the sheet.
        window.newComponent(nil)
        try await waitUntil(timeout: 5, "the third sheet") { window.window?.attachedSheet is NewComponentSheet }
        let third = try XCTUnwrap(window.window?.attachedSheet as? NewComponentSheet)
        third.nameField.stringValue = "Counter"
        third.nameChanged(third.nameField)
        third.createButton.performClick(nil)
        try await waitUntil(timeout: 20, "tap's error") { !third.errorLabel.isHidden }
        XCTAssertTrue(third.errorLabel.stringValue.contains("already exists"), third.errorLabel.stringValue)
        third.cancelButton.performClick(nil)
    }

    func testOpenAComponent() async throws {
        let deck = try Fixtures.copyDeck("stepped")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: try XCTUnwrap(document.sessionController).editor.boxes.count)
        let controller = try XCTUnwrap(document.sessionController)
        var opened: [URL] = []
        controller.openInEditor = { opened.append($0) }
        let text = controller.editor.string as NSString
        let pathRange = text.range(of: "./slides/RollingDeploy.jsx")
        XCTAssertNotEqual(pathRange.location, NSNotFound, "the fixture names its component")

        // A Cmd-click on the path opens the file; a plain click moves the caret.
        XCTAssertTrue(controller.openComponentLink(at: pathRange.location + 5))
        XCTAssertEqual(opened.map(\.path), [deck.deletingLastPathComponent().appendingPathComponent("slides/RollingDeploy.jsx").path])
        XCTAssertFalse(controller.openComponentLink(at: text.range(of: "# ").location), "not on a path")
        XCTAssertEqual(opened.count, 1)
        let point = try XCTUnwrap(controller.editor.layoutManager == nil ? controller.editor.pointForCharacter(at: pathRange.location + 5) : nil)
        let event = try XCTUnwrap(NSEvent.mouseEvent(with: .leftMouseDown, location: controller.editor.convert(point, to: nil), modifierFlags: [.command], timestamp: 0,
                                                     windowNumber: controller.editor.window?.windowNumber ?? 0, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
        controller.editor.mouseDown(with: event)
        XCTAssertEqual(opened.count, 2, "the Cmd-click reached the same path")
    }

    func testComponentErrors() async throws {
        let deck = try Fixtures.copyDeck("broken-component")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 2)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 10, "tap's component error on slide 2") {
            controller.editor.boxes.count == 2 && controller.editor.header(forBoxAt: 1).errors.contains { $0.contains("failed to build") }
        }
        let errors = controller.editor.header(forBoxAt: 1).errors
        XCTAssertTrue(errors.contains { $0.contains("Broken.jsx") }, "tap names the file: \(errors)")
        XCTAssertEqual(controller.editor.header(forBoxAt: 0).errors, [], "slide 1 is fine")
        XCTAssertGreaterThan(EditorTextView.errorLineCount(for: controller.editor.boxes[1].slide), 0, "the box makes room for the line")
    }
}
```

`pointForCharacter(at:)` is a TextKit 2 helper: add to `EditorTextView` `func pointForCharacter(at index: Int) -> NSPoint` returning the center of the glyph's frame through `textLayoutManager?.textLayoutFragment(for:)` and its `textLineFragments`, offset by `textContainerOrigin`; D2's `boxRect(forBoxAt:)` does the same walk, so reuse its line-frame lookup.

In `SlideMenuTests.testSlideMenu`, replace the two `XCTAssertNil(... action, "disabled until ...")` lines with:

```swift
        XCTAssertEqual(menu.items.first { $0.title == "Generate Image…" }?.action, #selector(DeckWindowController.generateImage(_:)))
        XCTAssertEqual(menu.items.first { $0.title == "New Component…" }?.action, #selector(DeckWindowController.newComponent(_:)))
        let insert = try item(menu, #selector(DeckWindowController.insertImage(_:)))
        XCTAssertEqual(insert.keyEquivalent, "i")
        XCTAssertEqual(insert.keyEquivalentModifierMask, [.command, .shift])
```

- [ ] **Step 2: Build to verify they do not compile**

Run: `make -C desktop test-build 2>&1 | tail -3`
Expected: fails on `NewComponentSheet`, `openInEditor`, `newComponent`.

- [ ] **Step 3: The sheet (the NewComponent board)**

The board draws "New Component": a "Name" field with the placeholder "PascalCase", a "Kind" control with "Whole slide" and "Inline block", a "TypeScript" switch, the line "Runs tap component new. The app inserts the snippet into slide 5 and opens components/RetryTimeline.jsx in your code editor.", and Cancel and Create. `desktop/Tap/Components/NewComponentSheet.swift`:

```swift
import AppKit

struct NewComponentRequest: Equatable {
    var name: String
    var inline: Bool
    var typeScript: Bool

    /// tap's own rule for the name; the sheet checks it before tap has to.
    static let namePattern = try! NSRegularExpression(pattern: "^[A-Z][A-Za-z0-9]*$")
    var isValidName: Bool { Self.namePattern.firstMatch(in: name, range: NSRange(location: 0, length: (name as NSString).length)) != nil }

    var fileName: String { "\(inline ? "components" : "slides")/\(name).\(typeScript ? "tsx" : "jsx")" }

    func arguments(deck: URL) -> [String] {
        var arguments = ["component", "new", name, deck.path]
        if inline { arguments.append("--inline") }
        if typeScript { arguments.append("--ts") }
        return arguments + ["--json"]
    }
}

/// Slide > New Component: the name, the kind and TypeScript, then tap
/// component new; the snippet goes in at the caret and the file opens in
/// the default code editor.
final class NewComponentSheet: QuestionSheet, NSTextFieldDelegate {
    let nameField = NSTextField(string: "")
    let kindControl = NSSegmentedControl(labels: ["Whole slide", "Inline block"], trackingMode: .selectOne, target: nil, action: nil)
    let typeScriptSwitch = NSSwitch()
    let errorLabel = NSTextField(wrappingLabelWithString: "")
    private let note = NSTextField(wrappingLabelWithString: "")
    private let slide: Int
    var onCreate: ((NewComponentRequest) -> Void)?
    var createButton: NSButton { acceptButton }
    var cancelButton: NSButton { declineButton }

    var request: NewComponentRequest {
        NewComponentRequest(name: nameField.stringValue.trimmingCharacters(in: .whitespaces), inline: kindControl.selectedSegment == 1, typeScript: typeScriptSwitch.state == .on)
    }

    init(slide: Int) {
        self.slide = slide
        let form = NSView()
        super.init(kind: "new-component", title: "New Component", body: "", path: nil, decline: "Cancel", accept: "Create", escape: .decline, returnAnswer: .accept, detail: form)
        bodyLabel.isHidden = true
        acceptButton.target = self
        acceptButton.action = #selector(createPressed(_:))
        nameField.placeholderString = "PascalCase"
        nameField.delegate = self
        nameField.setAccessibilityIdentifier("new-component-name")
        kindControl.selectedSegment = 0
        kindControl.target = self
        kindControl.action = #selector(nameChanged(_:))
        kindControl.setAccessibilityIdentifier("new-component-kind")
        typeScriptSwitch.target = self
        typeScriptSwitch.action = #selector(nameChanged(_:))
        typeScriptSwitch.setAccessibilityIdentifier("new-component-typescript")
        note.font = .systemFont(ofSize: 11.5)
        note.textColor = .secondaryLabelColor
        errorLabel.textColor = .systemRed
        errorLabel.font = .systemFont(ofSize: 12)
        errorLabel.isHidden = true
        let stack = NSStackView(views: [row("Name", nameField), row("Kind", kindControl), row("TypeScript", typeScriptSwitch), note, errorLabel])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.translatesAutoresizingMaskIntoConstraints = false
        form.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: form.topAnchor), stack.bottomAnchor.constraint(equalTo: form.bottomAnchor),
            stack.leadingAnchor.constraint(equalTo: form.leadingAnchor), stack.trailingAnchor.constraint(equalTo: form.trailingAnchor),
            nameField.widthAnchor.constraint(equalToConstant: 260), note.widthAnchor.constraint(equalTo: stack.widthAnchor), errorLabel.widthAnchor.constraint(equalTo: stack.widthAnchor),
        ])
        setContentSize(contentView?.fittingSize ?? frame.size)
        nameChanged(nameField)
    }

    private func row(_ title: String, _ control: NSView) -> NSView {
        let label = NSTextField(labelWithString: title)
        label.font = .systemFont(ofSize: 13)
        let row = NSStackView(views: [label, NSView(), control])
        row.orientation = .horizontal
        row.spacing = 12
        return row
    }

    @objc func nameChanged(_ sender: Any?) {
        let request = request
        createButton.isEnabled = request.isValidName
        note.stringValue = request.isValidName
            ? "Runs tap component new. The app inserts the snippet into slide \(slide) and opens \(request.fileName) in your code editor."
            : "Runs tap component new. The name is a PascalCase identifier, for example RollingDeploy."
        errorLabel.isHidden = true
    }

    func controlTextDidChange(_ notification: Notification) { nameChanged(notification.object) }

    @objc private func createPressed(_ sender: Any?) {
        createButton.isEnabled = false
        onCreate?(request)
    }

    func showError(_ message: String) {
        errorLabel.stringValue = message
        errorLabel.isHidden = false
        createButton.isEnabled = true
    }
}
```

- [ ] **Step 4: The session controller, the editor and the window**

In `DeckSessionController.swift`:

```swift
    /// Opens a file in whatever the person's default is for it (their code
    /// editor for a .jsx). A test records the URL instead.
    var openInEditor: (URL) -> Void = { url in NSWorkspace.shared.open(url) }

    /// New Component: tap component new writes the file (never the deck);
    /// its snippet goes in at the caret as one undo step, and the file
    /// opens in the default code editor.
    func createComponent(_ request: NewComponentRequest, completion: @escaping (ToolOutcome?) -> Void) {
        guard let deck = document?.fileURL else { return completion(nil) }
        Task { @MainActor [weak self] in
            guard let self else { return }
            let exit = await TapTool.run(request.arguments(deck: deck), in: deck.deletingLastPathComponent(), log: self.session.log)
            if case .ok? = exit.outcome, let scaffold = try? exit.outcome?.result(ComponentScaffold.self) {
                self.insertAtCaret(scaffold.snippet, actionName: "New Component")
                if let first = scaffold.files.first { self.openInEditor(URL(fileURLWithPath: first)) }
            }
            completion(exit.outcome)
        }
    }

    /// A Cmd-click on a component's path in the editor: the file, resolved
    /// against the deck's folder, opens in the default code editor. False
    /// when the character is not on a path.
    func openComponentLink(at characterIndex: Int) -> Bool {
        let text = editor.string as NSString
        guard characterIndex < text.length else { return false }
        let lineRange = text.lineRange(for: NSRange(location: characterIndex, length: 0))
        let line = text.substring(with: lineRange).trimmingCharacters(in: .newlines)
        guard let path = ComponentLink.find(in: line, at: characterIndex - lineRange.location), let deck = document?.fileURL else { return false }
        openInEditor(deck.deletingLastPathComponent().appendingPathComponent(path).standardizedFileURL)
        return true
    }

    func editor(_ editor: EditorTextView, openComponentLinkAt characterIndex: Int) -> Bool {
        openComponentLink(at: characterIndex)
    }
```

In `EditorTextViewDelegate`, add `func editor(_ editor: EditorTextView, openComponentLinkAt characterIndex: Int) -> Bool` with a `false` default. In `EditorTextView.mouseDown`, first thing:

```swift
        // A Cmd-click on a component's path opens the file; anywhere else it is a plain click.
        if event.modifierFlags.contains(.command), !event.modifierFlags.contains(.control) {
            let index = characterIndexForInsertion(at: point)
            if index < (string as NSString).length, editorDelegate?.editor(self, openComponentLinkAt: index) == true { return }
        }
```

In `DeckWindowController.swift`:

```swift
    @objc func newComponent(_ sender: Any?) {
        guard let slide = sessionController.currentSlideNumber, sessionController.document?.fileURL != nil else { return NSSound.beep() }
        let sheet = NewComponentSheet(slide: slide)
        sheet.onCreate = { [weak self, weak sheet] request in
            guard let self, let sheet else { return }
            self.sessionController.createComponent(request) { [weak self, weak sheet] outcome in
                guard let sheet else { return }
                switch outcome {
                case .ok?: self?.window?.endSheet(sheet, returnCode: .OK)
                case .failed(_, let message)?: sheet.showError(message)
                case nil: sheet.showError("tap did not answer; see the Tap Log")
                }
            }
        }
        showFormSheet(sheet)
    }
```

with the same validation as `generateImage`. In `MainMenu.slideMenu`, New Component… gets `#selector(DeckWindowController.newComponent(_:))`. The context menu (MenusSlide board) has no New Component item; none is added.

- [ ] **Step 5: Build, then hand the hosted tests to CI**

Run: `make -C desktop build && make -C desktop test-build 2>&1 | tail -3`
Expected: both succeed. The controller's CI run confirms: `ComponentTests` (3) and the extended `testSlideMenu` pass; D3's `EditorHeaderDragTests` still pass (a header drag has no Cmd).

- [ ] **Step 6: Mutations and commit**

Mutations, each a patch in `mutations-b/`: in `createComponent`, insert `scaffold.files.first` instead of `scaffold.snippet` (`Test: TapTests/ComponentTests/testCreateAComponent`; expected: fails on the snippet's range); in `createComponent`, skip `openInEditor` (expected: fails on `opened`); in `NewComponentRequest.arguments`, drop `--inline` (expected: fails on the second request's arguments); in `NewComponentRequest.isValidName`, accept any non-empty name (expected: fails on "tap wants PascalCase"); in `openComponentLink`, resolve against the home folder (`Test: .../testOpenAComponent`; expected: fails on the opened path); in `mouseDown`, drop the Cmd branch (expected: fails on `opened.count == 2`); in `openComponentLink`, return true and open nothing when not on a path (expected: fails on the "not on a path" assertion).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): New Component through tap component new, and a Cmd-click that opens a component"
```

---

### Task 10: Export PDF: the sheet, real progress, the engine download, warnings and Cancel

**Files:**
- Create: `desktop/Tap/Export/ExportController.swift`, `desktop/Tap/Export/ExportSheet.swift`
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (`exportController`, `exportPDF(_:)`, `exportWebsite(_:)`, `exportImages(_:)`, `canStartATalk`, `windowWillClose`)
- Modify: `desktop/Tap/App/MainMenu.swift` (File > Export submenu, the MenusFile board)
- Modify: `desktop/TapTests/Support/FakeToolScripts.swift` (`exportPDF`)
- Modify: `.github/workflows/ci.yml` (the Chromium cache for the Desktop Tests job)
- Test: `desktop/TapTests/ExportPDFTests.swift`

**Interfaces:**
- Consumes: `ToolRun`, `ProgressLine`, `ToolOutcome` (Task 3), `PDFExportResult`, `BrokenSlide` (Task 4), `TapTool.makeRun` (Task 6), `saveNow(completion:)` (Task 6), `DeckWindowController.revealInFinder` (D4's seam), `showFormSheet` (Task 8).
- Produces: `ExportKind` (`.pdf(content:)`, `.website`, `.images`), `ExportRequest` (`kind`, `output`, `arguments(deck:)`, `defaultOutput(for:deck:)`), `ExportController` (`isRunning`, `start(_:)`, `cancel()`, `stop()`, `onFinished`, `state`), `ExportSheet` (`State`, `contentControl`, `outputField`, `chooseButton`, `progressBar`, `statusLabel`, `warningsLabel`, `exportButton`, `cancelButton`, `revealButton`, `previewButton`, `request`, `apply(_:)`, `chooseOutput` seam), `DeckWindowController.exportPDF(_:)`, `exportWebsite(_:)`, `exportImages(_:)`, `exportController`. Task 11 fills in the website and images kinds.

- [ ] **Step 1: The scripted exports and the failing tests**

In `FakeToolScripts.swift`, add:

```swift
    /// `tap export pdf`: `download` lines first when `downloadLines` is set,
    /// a render line per slide with `secondsPerSlide` between them, the
    /// warnings for `broken`, then the done line and the file. On SIGINT it
    /// prints tap's interrupted done line and exits 130, as tap does.
    static func exportPDF(slides: Int, secondsPerSlide: Double = 0, downloadLines: Int = 0, broken: [(slide: Int, message: String)] = [], recordingTo record: URL) throws -> URL {
        let download = (0..<downloadLines).map { index in
            #"echo '{"phase":"download","bytes":\#((index + 1) * 50_000_000),"totalBytes":\#(downloadLines * 50_000_000)}' >&2; sleep 0.2"#
        }.joined(separator: "; ")
        let brokenJSON = broken.map { #"{"slide":\#($0.slide),"message":"\#($0.message)"}"# }.joined(separator: ",")
        let warnings = broken.map { #"echo 'warning: slide \#($0.slide) shows an error card: \#($0.message)' >&2"# }.joined(separator: "; ")
        return try write("""
          "export pdf")
            trap 'echo "{\\"phase\\":\\"done\\",\\"ok\\":false,\\"error\\":{\\"code\\":\\"interrupted\\",\\"message\\":\\"interrupted\\"}}" >&2; exit 130' INT
            out=""; while [ $# -gt 0 ]; do case "$1" in --output|-o) out="$2"; shift ;; esac; shift; done
            \(download.isEmpty ? ":" : download)
            i=1; while [ $i -le \(slides) ]; do echo "{\\"phase\\":\\"render\\",\\"done\\":$i,\\"total\\":\(slides)}" >&2; sleep \(secondsPerSlide); i=$((i + 1)); done
            \(warnings.isEmpty ? ":" : warnings)
            printf 'not a real pdf' > "$out"
            echo "{\\"phase\\":\\"done\\",\\"ok\\":true,\\"output\\":\\"$out\\",\\"pages\\":\(slides),\\"bytes\\":14,\\"brokenSlides\\":[\(brokenJSON)]}" >&2
            exit 0 ;;
        """, recordingTo: record)
    }
```

`desktop/TapTests/ExportPDFTests.swift`:

```swift
import XCTest
@testable import Tap
@testable import TapDesktopCore

final class ExportPDFTests: HostedTestCase {
    func openSevenSlides() async throws -> (DeckDocument, DeckWindowController, URL) {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        return (document, try XCTUnwrap(document.windowControllers.first as? DeckWindowController), deck)
    }

    func exportSheet(_ window: DeckWindowController) async throws -> ExportSheet {
        try await waitUntil(timeout: 5, "the export sheet") { window.window?.attachedSheet is ExportSheet }
        return try XCTUnwrap(window.window?.attachedSheet as? ExportSheet)
    }

    /// The real bundled tap, the real export engine (CI caches its download): the PDF matches the CLI's.
    func testExportAPDF() async throws {
        let (document, window, deck) = try await openSevenSlides()
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.insertText("edited before the export ", replacementRange: NSRange(location: controller.editor.hiddenLength, length: 0))
        var revealed: [URL] = []
        window.revealInFinder = { revealed.append($0) }

        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Export PDF")
        XCTAssertEqual(sheet.contentControl.label(forSegment: sheet.contentControl.selectedSegment), "Slides")
        XCTAssertEqual(sheet.outputField.stringValue, deck.deletingPathExtension().appendingPathExtension("pdf").path, "next to the deck by default")
        sheet.contentControl.selectedSegment = 2
        XCTAssertEqual(sheet.request.arguments(deck: deck), ["export", "pdf", deck.path, "--output", sheet.outputField.stringValue, "--content", "both", "--progress", "json"])
        sheet.exportButton.performClick(nil)

        var sawRender = false
        try await waitUntil(timeout: 240, "the export to finish") {
            if case .running(let status) = sheet.state, status.hasPrefix("Rendering") { sawRender = true }
            return window.window?.attachedSheet == nil
        }
        XCTAssertTrue(sawRender, "real progress showed")
        XCTAssertEqual(revealed.map(\.path), [sheet.outputField.stringValue], "the file is revealed in Finder")
        let attributes = try FileManager.default.attributesOfItem(atPath: sheet.outputField.stringValue)
        XCTAssertGreaterThan((attributes[.size] as? Int) ?? 0, 1000, "a real PDF")
        XCTAssertTrue(try String(contentsOf: deck, encoding: .utf8).contains("edited before the export"), "the buffer was saved first")
        XCTAssertFalse(window.exportController.isRunning)
    }

    func testFirstPDFExport() async throws {
        let (_, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 3, downloadLines: 4, recordingTo: record)
        window.revealInFinder = { _ in }
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 10, "the download state") {
            if case .running(let status) = sheet.state { return status.hasPrefix("Downloading the export engine") }
            return false
        }
        XCTAssertEqual(sheet.statusLabel.stringValue, "Downloading the export engine")
        XCTAssertTrue(sheet.detailLabel.stringValue.hasPrefix("This happens once."), sheet.detailLabel.stringValue)
        XCTAssertTrue(sheet.progressBar.doubleValue > 0 && !sheet.progressBar.isIndeterminate, "bytes of total")
        XCTAssertTrue(sheet.bytesLabel.stringValue.contains(" of "), sheet.bytesLabel.stringValue)
        try await waitUntil(timeout: 20, "the sheet to finish") { window.window?.attachedSheet == nil }
    }

    func testASlideFailsDuringExport() async throws {
        let (_, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 3, broken: [(2, "component Throws.jsx threw")], recordingTo: record)
        var revealed: [URL] = []
        window.revealInFinder = { revealed.append($0) }
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 20, "the done state") { if case .done = sheet.state { return true } else { return false } }
        guard case .done(let summary) = sheet.state else { return XCTFail("not done") }
        XCTAssertEqual(summary.warnings, ["Slide 2 shows an error card: component Throws.jsx threw"], "tap's brokenSlides, as warnings")
        XCTAssertEqual(revealed.count, 1, "the file is still revealed: tap wrote it")
        XCTAssertTrue(window.window?.attachedSheet === sheet, "a warning keeps the sheet up so the person reads it")
        XCTAssertEqual(sheet.warningsLabel.stringValue, "Slide 2 shows an error card: component Throws.jsx threw")
        sheet.doneButton.performClick(nil)
        try await waitUntil(timeout: 5, "the sheet to close") { window.window?.attachedSheet == nil }
    }

    func testCancelAnExport() async throws {
        let (_, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 40, secondsPerSlide: 0.5, recordingTo: record)
        var revealed: [URL] = []
        window.revealInFinder = { revealed.append($0) }
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 10, "rendering") { if case .running(let status) = sheet.state { return status.hasPrefix("Rendering slide") } else { return false } }
        XCTAssertFalse(window.canStartATalk, "no talk while the export reads the file")
        sheet.cancelButton.performClick(nil)
        XCTAssertEqual(sheet.statusLabel.stringValue, "Cancelling…")
        try await waitUntil(timeout: 10, "the sheet to close") { window.window?.attachedSheet == nil }
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertTrue(recorded.contains("arguments: export pdf"))
        XCTAssertEqual(revealed, [], "nothing is revealed for a cancelled export")
        XCTAssertFalse(window.exportController.isRunning)
        XCTAssertTrue(window.canStartATalk)
        try await waitUntil(timeout: 5, "tap's exit in the log") { window.sessionController.session.log.text.contains("export cancelled (exit 130)") }
    }

    func testClosingTheDeckStopsItsExport() async throws {
        let (document, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 40, secondsPerSlide: 0.5, recordingTo: record)
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 10, "rendering") { if case .running = sheet.state { return true } else { return false } }
        let identifier = try XCTUnwrap(window.exportController.processIdentifier)
        document.close()
        try await waitUntil(timeout: 10, "the export process to be gone") { kill(identifier, 0) != 0 }
    }

    func testTapsFailureShowsInTheSheet() async throws {
        let (_, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.write("""
          "export pdf")
            echo '{"phase":"done","ok":false,"error":{"code":"browser","message":"failed to start browser: no network"}}' >&2
            exit 2 ;;
        """, recordingTo: record)
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 10, "the failure") { if case .failed = sheet.state { return true } else { return false } }
        XCTAssertEqual(sheet.statusLabel.stringValue, "Export failed.")
        XCTAssertEqual(sheet.detailLabel.stringValue, "failed to start browser: no network", "tap's own message")
        XCTAssertTrue(sheet.exportButton.isEnabled, "another try")
        sheet.cancelButton.performClick(nil)
    }
}
```

- [ ] **Step 2: Build to verify they do not compile**

Run: `make -C desktop test-build 2>&1 | tail -3`
Expected: fails on `ExportSheet`, `exportPDF`, `exportController`.

- [ ] **Step 3: `ExportController`**

`desktop/Tap/Export/ExportController.swift`:

```swift
import AppKit

enum ExportKind: Equatable {
    /// `content` is slides, notes or both, tap's --content values.
    case pdf(content: String)
    case website
    case images

    var title: String {
        switch self {
        case .pdf: return "Export PDF"
        case .website: return "Export Website"
        case .images: return "Export Slide Images"
        }
    }

    /// The default output next to the deck: <deck>.pdf, <folder>/dist, <folder>/<deck>-slides.
    func defaultOutput(for deck: URL) -> URL {
        switch self {
        case .pdf: return deck.deletingPathExtension().appendingPathExtension("pdf")
        case .website: return deck.deletingLastPathComponent().appendingPathComponent("dist")
        case .images: return deck.deletingLastPathComponent().appendingPathComponent(deck.deletingPathExtension().lastPathComponent + "-slides")
        }
    }
}

struct ExportRequest: Equatable {
    var kind: ExportKind
    var output: String

    func arguments(deck: URL) -> [String] {
        switch kind {
        case .pdf(let content): return ["export", "pdf", deck.path, "--output", output, "--content", content, "--progress", "json"]
        case .website: return ["build", deck.path, "--output", output, "--progress", "json"]
        case .images: return ["export", "images", deck.path, "--all", "--output", output, "--progress", "json"]
        }
    }
}

/// What a finished export shows: the output, a summary line, and tap's
/// warnings (a slide that shows an error card, a slide that failed).
struct ExportSummary: Equatable {
    var output: URL
    var summary: String
    var warnings: [String]
}

/// One export at a time for a deck window: saves the buffer, runs tap
/// with --progress json, turns the lines into the sheet's states, cancels
/// with SIGINT, and stops with the window. tap does the export; this
/// object reads what tap says.
@MainActor
final class ExportController {
    enum State: Equatable {
        case idle
        case running(String)
        case done(ExportSummary)
        case failed(String)
    }

    private(set) var state: State = .idle {
        didSet { onStateChange?(state) }
    }
    var onStateChange: ((State) -> Void)?
    /// Bytes of total while the export engine downloads.
    private(set) var download: (bytes: Int64, totalBytes: Int64)?
    var onDownload: (((bytes: Int64, totalBytes: Int64)?) -> Void)?
    private var run: ToolRun?
    private weak var sessionController: DeckSessionController?
    private var startedAt: Date?

    init(sessionController: DeckSessionController) {
        self.sessionController = sessionController
    }

    var isRunning: Bool { run?.isRunning ?? false }
    var processIdentifier: Int32? { run.map(\.processIdentifier) }

    /// Saves, then runs. A refused save is a failure with the document's reason.
    func start(_ request: ExportRequest) {
        guard !isRunning, let sessionController, let deck = sessionController.document?.fileURL else { return }
        state = .running("Saving…")
        sessionController.saveNow { [weak self] error in
            guard let self else { return }
            if let error {
                self.state = .failed("The deck could not be saved: \(error.localizedDescription)")
                return
            }
            Task { @MainActor [weak self] in
                guard let self, let sessionController = self.sessionController else { return }
                let run = await TapTool.makeRun(request.arguments(deck: deck), in: deck.deletingLastPathComponent(), timeout: 1800, log: sessionController.session.log)
                self.run = run
                self.startedAt = Date()
                run.onProgress = { [weak self] line in self?.handle(line, request: request) }
                run.onExit = { [weak self] exit in self?.finished(exit, request: request) }
                do {
                    try run.start()
                    self.state = .running("Preparing…")
                } catch {
                    self.state = .failed("tap could not be started: \(error.localizedDescription)")
                }
            }
        }
    }

    func cancel() {
        guard let run, run.isRunning else { return }
        state = .running("Cancelling…")
        run.cancel()
    }

    /// The window is closing: no export outlives its deck.
    func stop() {
        run?.cancel()
    }

    private func handle(_ line: ProgressLine, request: ExportRequest) {
        switch line {
        case .download(let bytes, let totalBytes):
            download = (bytes, totalBytes)
            onDownload?(download)
            state = .running("Downloading the export engine")
        case .step(let phase, let done, let total):
            if download != nil {
                download = nil
                onDownload?(nil)
            }
            state = .running(Self.status(for: phase, done: done, total: total, kind: request.kind))
        case .finished:
            break
        }
    }

    static func status(for phase: String, done: Int, total: Int, kind: ExportKind) -> String {
        switch phase {
        case "render": return "Rendering slide \(done) of \(total)"
        case "load": return "Loading the deck"
        case "parse": return "Parsing the deck"
        case "bundle": return "Bundling components"
        case "write": return "Writing files"
        default: return "\(phase) \(done) of \(total)"
        }
    }

    private func finished(_ exit: ToolRun.Exit, request: ExportRequest) {
        run = nil
        download = nil
        onDownload?(nil)
        let log = sessionController?.session.log
        if exit.cancelled {
            log?.append("export cancelled (exit \(exit.status))", source: .app)
            state = .idle
            return
        }
        switch exit.outcome {
        case .ok?:
            state = .done(summary(from: exit, request: request))
        case .failed(let code, let message)?:
            // tap export images exits 1 with broken_slides after writing the
            // other files; that is warnings with a partial result, not a failure.
            if code == "broken_slides", case .images = request.kind {
                state = .done(ExportSummary(output: URL(fileURLWithPath: request.output), summary: "Some slides could not be rendered.", warnings: [message]))
            } else {
                state = .failed(message)
            }
        case nil:
            state = .failed(exit.timedOut ? "tap took longer than 30 minutes" : "tap did not answer (exit \(exit.status)); see the Tap Log")
        }
    }

    private func summary(from exit: ToolRun.Exit, request: ExportRequest) -> ExportSummary {
        let seconds = startedAt.map { String(format: "%.1f s", Date().timeIntervalSince($0)) } ?? ""
        let formatter = ByteCountFormatter()
        formatter.countStyle = .file
        switch request.kind {
        case .pdf:
            let result = try? exit.outcome?.result(PDFExportResult.self)
            let warnings = (result?.brokenSlides ?? []).map { "Slide \($0.slide) shows an error card: \($0.message)" }
            let summary = result.map { "\($0.pages) pages, \(formatter.string(fromByteCount: $0.bytes)) in \(seconds)." } ?? "Exported."
            return ExportSummary(output: URL(fileURLWithPath: result?.output ?? request.output), summary: summary, warnings: warnings)
        case .website:
            let result = try? exit.outcome?.result(BuildResult.self)
            let slides = sessionController?.editor.boxes.count ?? 0
            let summary = result.map { "\(slides) slides, \($0.files) files, \(formatter.string(fromByteCount: $0.bytes)) in \(seconds). Live code does not run in a static site." } ?? "Exported."
            return ExportSummary(output: URL(fileURLWithPath: result?.output ?? request.output), summary: summary, warnings: [])
        case .images:
            let result = try? exit.outcome?.result(ImagesExportResult.self)
            return ExportSummary(output: URL(fileURLWithPath: request.output), summary: "\(result?.files.count ?? 0) images in \(seconds).", warnings: [])
        }
    }
}
```

- [ ] **Step 4: `ExportSheet` (the ExportPDF board, and the done state of the ExportWebsite board)**

The ExportPDF board draws "Export PDF": a "Content" control with Slides, Notes and Both, a "Save as" row with the file name, and, during the first run, "Downloading the export engine" with the line "This happens once. tap pdf renders with Chromium, so the PDF matches the CLI exactly.", a progress bar and "64 MB of 150 MB", with Cancel and Export. The ExportWebsite board draws the done state: "Website exported", "14 slides, 38 files, 4.1 MB in 2.3 s. Live code does not run in a static site.", the folder's path, and "Show in Finder" and "Preview". The render phase reuses the download layout with "Rendering slide 7 of 14" and a bar of done over total; the finished-with-warnings state's drawing waits for a mockup (Step 7), its data path is built here. `desktop/Tap/Export/ExportSheet.swift`:

```swift
import AppKit

/// File > Export: the options, then the run's progress, then what tap
/// made. One sheet per export; Cancel sends SIGINT through the controller.
final class ExportSheet: NSWindow {
    typealias State = ExportController.State

    let kind: ExportKind
    let titleLabel = NSTextField(labelWithString: "")
    let contentControl = NSSegmentedControl(labels: ["Slides", "Notes", "Both"], trackingMode: .selectOne, target: nil, action: nil)
    let outputField = NSTextField(string: "")
    let chooseButton = NSButton(title: "Choose…", target: nil, action: nil)
    let statusLabel = NSTextField(labelWithString: "")
    let detailLabel = NSTextField(wrappingLabelWithString: "")
    let bytesLabel = NSTextField(labelWithString: "")
    let progressBar = NSProgressIndicator()
    let warningsLabel = NSTextField(wrappingLabelWithString: "")
    let pathLabel = NSTextField(labelWithString: "")
    let cancelButton = NSButton(title: "Cancel", target: nil, action: nil)
    let exportButton = NSButton(title: "Export", target: nil, action: nil)
    let revealButton = NSButton(title: "Show in Finder", target: nil, action: nil)
    let previewButton = NSButton(title: "Preview", target: nil, action: nil)
    let doneButton = NSButton(title: "Done", target: nil, action: nil)
    private(set) var state: State = .idle
    private let optionsStack: NSStackView
    private let progressStack: NSStackView
    private let doneStack: NSStackView
    var onExport: ((ExportRequest) -> Void)?
    var onCancel: (() -> Void)?
    var onReveal: ((URL) -> Void)?
    var onPreview: ((URL) -> Void)?
    /// A save or folder panel in production; a test answers at once.
    var chooseOutput: (ExportKind, String, @escaping (String?) -> Void) -> Void = { kind, current, completion in
        if case .pdf = kind {
            let panel = NSSavePanel()
            panel.allowedContentTypes = [.pdf]
            panel.nameFieldStringValue = (current as NSString).lastPathComponent
            panel.directoryURL = URL(fileURLWithPath: (current as NSString).deletingLastPathComponent)
            panel.begin { response in completion(response == .OK ? panel.url?.path : nil) }
        } else {
            let panel = NSOpenPanel()
            panel.canChooseDirectories = true
            panel.canChooseFiles = false
            panel.canCreateDirectories = true
            panel.prompt = "Export Here"
            panel.begin { response in completion(response == .OK ? panel.url?.path : nil) }
        }
    }

    var request: ExportRequest {
        let contents = ["slides", "notes", "both"]
        let kind: ExportKind
        switch self.kind {
        case .pdf: kind = .pdf(content: contents[max(0, contentControl.selectedSegment)])
        default: kind = self.kind
        }
        return ExportRequest(kind: kind, output: outputField.stringValue)
    }

    init(kind: ExportKind, deck: URL) {
        self.kind = kind
        optionsStack = NSStackView()
        progressStack = NSStackView()
        doneStack = NSStackView()
        super.init(contentRect: NSRect(x: 0, y: 0, width: 520, height: 260), styleMask: [.titled], backing: .buffered, defer: false)
        titleLabel.stringValue = kind.title
        titleLabel.font = .systemFont(ofSize: 15, weight: .bold)
        outputField.stringValue = kind.defaultOutput(for: deck).path
        outputField.setAccessibilityIdentifier("export-output")
        contentControl.selectedSegment = 0
        contentControl.setAccessibilityIdentifier("export-content")
        chooseButton.bezelStyle = .rounded
        chooseButton.target = self
        chooseButton.action = #selector(choosePressed(_:))
        for button in [cancelButton, exportButton, revealButton, previewButton, doneButton] { button.bezelStyle = .rounded; button.target = self }
        cancelButton.action = #selector(cancelPressed(_:))
        cancelButton.keyEquivalent = "\u{1b}"
        exportButton.action = #selector(exportPressed(_:))
        exportButton.keyEquivalent = "\r"
        exportButton.setAccessibilityIdentifier("export-run")
        revealButton.action = #selector(revealPressed(_:))
        revealButton.setAccessibilityIdentifier("export-reveal")
        previewButton.action = #selector(previewPressed(_:))
        previewButton.setAccessibilityIdentifier("export-preview")
        doneButton.action = #selector(donePressed(_:))
        doneButton.keyEquivalent = "\r"
        statusLabel.font = .systemFont(ofSize: 13, weight: .semibold)
        statusLabel.setAccessibilityIdentifier("export-status")
        detailLabel.font = .systemFont(ofSize: 12)
        detailLabel.textColor = .secondaryLabelColor
        bytesLabel.font = .systemFont(ofSize: 11.5)
        bytesLabel.textColor = .secondaryLabelColor
        progressBar.style = .bar
        progressBar.minValue = 0
        progressBar.maxValue = 1
        progressBar.setAccessibilityIdentifier("export-progress")
        warningsLabel.font = .systemFont(ofSize: 12)
        warningsLabel.textColor = .systemOrange
        warningsLabel.setAccessibilityIdentifier("export-warnings")
        pathLabel.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        pathLabel.lineBreakMode = .byTruncatingMiddle

        let outputRow = NSStackView(views: [NSTextField(labelWithString: kind.isPDF ? "Save as" : "Export to"), outputField, chooseButton])
        outputRow.spacing = 8
        outputField.widthAnchor.constraint(equalToConstant: 300).isActive = true
        var options: [NSView] = []
        if kind.isPDF { options.append(NSStackView(views: [NSTextField(labelWithString: "Content"), NSView(), contentControl])) }
        options.append(outputRow)
        for view in options { optionsStack.addArrangedSubview(view) }
        optionsStack.orientation = .vertical
        optionsStack.alignment = .leading
        optionsStack.spacing = 10
        for view in [statusLabel, detailLabel, progressBar, bytesLabel] { progressStack.addArrangedSubview(view) }
        progressStack.orientation = .vertical
        progressStack.alignment = .leading
        progressStack.spacing = 6
        for view in [pathLabel, warningsLabel] { doneStack.addArrangedSubview(view) }
        doneStack.orientation = .vertical
        doneStack.alignment = .leading
        doneStack.spacing = 6
        let buttons = NSStackView(views: [NSView(), revealButton, previewButton, doneButton, cancelButton, exportButton])
        buttons.spacing = 8
        let stack = NSStackView(views: [titleLabel, optionsStack, progressStack, doneStack, buttons])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 24, left: 24, bottom: 24, right: 24)
        stack.widthAnchor.constraint(equalToConstant: 520).isActive = true
        for view in [optionsStack, progressStack, doneStack, buttons, detailLabel, warningsLabel, progressBar, pathLabel] as [NSView] {
            view.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -48).isActive = true
        }
        contentView = stack
        isReleasedWhenClosed = false
        setAccessibilityIdentifier("export-sheet")
        apply(.idle)
        setContentSize(stack.fittingSize)
    }

    /// The sheet's face for each state.
    func apply(_ state: State) {
        self.state = state
        switch state {
        case .idle:
            optionsStack.isHidden = false
            progressStack.isHidden = true
            doneStack.isHidden = true
            exportButton.isHidden = false
            exportButton.isEnabled = true
            cancelButton.isHidden = false
            cancelButton.title = "Cancel"
            revealButton.isHidden = true
            previewButton.isHidden = true
            doneButton.isHidden = true
        case .running(let status):
            optionsStack.isHidden = true
            progressStack.isHidden = false
            doneStack.isHidden = true
            exportButton.isHidden = true
            cancelButton.isHidden = false
            cancelButton.isEnabled = status != "Cancelling…"
            revealButton.isHidden = true
            previewButton.isHidden = true
            doneButton.isHidden = true
            statusLabel.stringValue = status
            if status.hasPrefix("Downloading") {
                detailLabel.stringValue = "This happens once. tap renders with Chromium, so the export matches the CLI exactly."
            } else {
                detailLabel.stringValue = ""
                bytesLabel.stringValue = ""
            }
            if let (done, total) = Self.counts(in: status), total > 0 {
                progressBar.isIndeterminate = false
                progressBar.doubleValue = Double(done) / Double(total)
            } else if !status.hasPrefix("Downloading") {
                progressBar.isIndeterminate = true
                progressBar.startAnimation(nil)
            }
        case .done(let summary):
            optionsStack.isHidden = true
            progressStack.isHidden = false
            doneStack.isHidden = false
            statusLabel.stringValue = Self.doneTitle(for: kind)
            detailLabel.stringValue = summary.summary
            bytesLabel.stringValue = ""
            progressBar.isHidden = true
            pathLabel.stringValue = (summary.output.path as NSString).abbreviatingWithTildeInPath
            warningsLabel.stringValue = summary.warnings.joined(separator: "\n")
            warningsLabel.isHidden = summary.warnings.isEmpty
            exportButton.isHidden = true
            cancelButton.isHidden = true
            revealButton.isHidden = false
            previewButton.isHidden = !kind.isWebsite
            doneButton.isHidden = false
        case .failed(let message):
            optionsStack.isHidden = false
            progressStack.isHidden = false
            doneStack.isHidden = true
            progressBar.isHidden = true
            statusLabel.stringValue = "Export failed."
            detailLabel.stringValue = message
            bytesLabel.stringValue = ""
            exportButton.isHidden = false
            exportButton.isEnabled = true
            cancelButton.isHidden = false
            revealButton.isHidden = true
            previewButton.isHidden = true
            doneButton.isHidden = true
        }
        if case .running = state { progressBar.isHidden = false }
        setContentSize((contentView as? NSStackView)?.fittingSize ?? frame.size)
    }

    func showDownload(_ download: (bytes: Int64, totalBytes: Int64)?) {
        guard let download else { return bytesLabel.stringValue = "" }
        let formatter = ByteCountFormatter()
        formatter.countStyle = .file
        bytesLabel.stringValue = "\(formatter.string(fromByteCount: download.bytes)) of \(formatter.string(fromByteCount: download.totalBytes))"
        progressBar.isIndeterminate = false
        progressBar.doubleValue = download.totalBytes > 0 ? Double(download.bytes) / Double(download.totalBytes) : 0
    }

    /// "Rendering slide 7 of 14" is 7 of 14.
    static func counts(in status: String) -> (Int, Int)? {
        let words = status.split(separator: " ")
        guard words.count >= 3, words[words.count - 2] == "of", let total = Int(words[words.count - 1]), let done = Int(words[words.count - 3]) else { return nil }
        return (done, total)
    }

    static func doneTitle(for kind: ExportKind) -> String {
        switch kind {
        case .pdf: return "PDF exported"
        case .website: return "Website exported"
        case .images: return "Slide images exported"
        }
    }

    @objc private func exportPressed(_ sender: Any?) { onExport?(request) }
    @objc private func cancelPressed(_ sender: Any?) {
        if case .running = state { onCancel?() } else { sheetParent?.endSheet(self, returnCode: .cancel) }
    }
    @objc private func revealPressed(_ sender: Any?) { if case .done(let summary) = state { onReveal?(summary.output) } }
    @objc private func previewPressed(_ sender: Any?) { if case .done(let summary) = state { onPreview?(summary.output) } }
    @objc private func donePressed(_ sender: Any?) { sheetParent?.endSheet(self, returnCode: .OK) }
    @objc private func choosePressed(_ sender: Any?) {
        chooseOutput(kind, outputField.stringValue) { [weak self] chosen in
            if let chosen { self?.outputField.stringValue = chosen }
        }
    }
}

extension ExportKind {
    var isPDF: Bool { if case .pdf = self { return true } else { return false } }
    var isWebsite: Bool { self == .website }
}
```

- [ ] **Step 5: The window's export actions, the menu, and Play**

In `DeckWindowController.swift`: `private(set) lazy var exportController = ExportController(sessionController: sessionController)`, and:

```swift
    @objc func exportPDF(_ sender: Any?) { beginExport(.pdf(content: "slides")) }
    @objc func exportWebsite(_ sender: Any?) { beginExport(.website) }
    @objc func exportImages(_ sender: Any?) { beginExport(.images) }

    /// File > Export: the sheet for `kind`; its Export button starts the
    /// run, and the sheet follows the controller's states. A PDF with no
    /// warnings closes the sheet and reveals the file; anything with
    /// warnings, and a website or images export, stays for the person.
    func beginExport(_ kind: ExportKind) {
        guard let deck = sessionController.document?.fileURL, !exportController.isRunning else { return NSSound.beep() }
        let sheet = ExportSheet(kind: kind, deck: deck)
        sheet.onExport = { [weak self] request in self?.exportController.start(request) }
        sheet.onCancel = { [weak self] in self?.exportController.cancel() }
        sheet.onReveal = { [weak self] url in self?.revealInFinder(url) }
        sheet.onPreview = { [weak self] folder in self?.previewWebsite(at: folder) }
        exportController.onStateChange = { [weak self, weak sheet] state in
            guard let self, let sheet else { return }
            sheet.apply(state)
            self.refreshPresentingControls()
            switch state {
            case .done(let summary) where kind.isPDF && summary.warnings.isEmpty:
                self.revealInFinder(summary.output)
                self.window?.endSheet(sheet, returnCode: .OK)
            case .done(let summary) where kind.isPDF:
                self.revealInFinder(summary.output)
            case .idle where sheet.sheetParent != nil:
                // A cancelled run: the sheet goes, nothing is shown of the partial file.
                self.window?.endSheet(sheet, returnCode: .cancel)
            default:
                break
            }
        }
        exportController.onDownload = { [weak sheet] download in sheet?.showDownload(download) }
        showFormSheet(sheet)
    }

    func previewWebsite(at folder: URL) {
        // Task 11 starts tap serve here.
    }
```

`canStartATalk` gains `&& !exportController.isRunning`; `windowWillClose` calls `exportController.stop()` and `previewServer?.stop()` (Task 11). `validateMenuItem` returns `sessionController.document?.fileURL != nil && !exportController.isRunning` for the three export selectors. In `MainMenu.fileMenu`, after `Revert to Saved`, as the MenusFile board draws (an Export submenu with PDF… ⌥⌘E, Slide Images…, Website…):

```swift
        menu.addItem(.separator())
        let export = item("Export", action: nil)
        let exportMenu = NSMenu(title: "Export")
        exportMenu.addItem(item("PDF…", action: #selector(DeckWindowController.exportPDF(_:)), key: "e", modifiers: [.command, .option]))
        exportMenu.addItem(item("Slide Images…", action: #selector(DeckWindowController.exportImages(_:))))
        exportMenu.addItem(item("Website…", action: #selector(DeckWindowController.exportWebsite(_:))))
        export.submenu = exportMenu
        menu.addItem(export)
```

(The board's "Deck Settings… ⌥⌘2" under Export is D5's View > Deck; not duplicated.)

- [ ] **Step 6: CI: the export engine for the real PDF run**

`testExportAPDF` runs the bundled tap's real `tap export pdf`, which downloads Chromium on a fresh runner. In `.github/workflows/ci.yml`, in the `test-desktop` job before "Run hosted Tap.app tests", add the same cache the Go job has, with macOS's paths:

```yaml
      # The hosted export test drives the bundled tap's real tap export pdf,
      # which fetches Chromium through playwright-go on a fresh runner: cache
      # its driver and browser, keyed on the playwright-go version in go.mod.
      - name: Determine playwright-go version
        if: steps.check-desktop.outputs.exists == 'true'
        id: playwright-go
        run: |
          version=$(grep -m1 'github.com/mxschmitt/playwright-go ' go.mod | awk '{print $2}')
          echo "version=$version" >> "$GITHUB_OUTPUT"

      - name: Cache Playwright driver and browser
        if: steps.check-desktop.outputs.exists == 'true'
        uses: actions/cache@v6
        with:
          path: |
            ~/Library/Caches/ms-playwright-go
            ~/Library/Caches/ms-playwright
          key: playwright-go-${{ runner.os }}-${{ steps.playwright-go.outputs.version }}
```

`testExportAPDF` sets its own allowance: `override var executionTimeAllowance: TimeInterval` is not per test, so the test's wait is 240 s and the Makefile's default 300 s allowance covers a cold download; if a cold run takes longer on the runner, the test's class sets `executionTimeAllowance = 600` in `setUp` (the Makefile's maximum).

- [ ] **Step 7: The warnings drawing: waits for the person's mockup sign-off (the controller records it in the ledger)**

The finished PDF export with warnings, and the images export's partial result, have no board. Proposed, for the mockup: the ExportWebsite board's done state (the title, the summary line, the path, the buttons) with the warnings as a list of lines in orange under the path, one per slide, and a "Done" button beside "Show in Finder". Until sign-off the sheet shows exactly that with plain labels, which the tests read; the drawing changes with the board, and nothing built before it does.

- [ ] **Step 8: Build, then hand the hosted tests to CI**

Run: `make -C desktop build && make -C desktop test-build 2>&1 | tail -3`
Expected: both succeed. The controller's CI run confirms: `ExportPDFTests` (6) pass, `testExportAPDF` included on the first run after the cache step (the cold download is inside its 240 s wait; if it is not, the run's log says how long the download took, for the allowance in Step 6); D2's `MenuTests.testMenuBar` and `testFileMenuHasRevertTo` still pass (the Export submenu sits after Revert to Saved; AppKit still installs Revert To).

- [ ] **Step 9: Mutations and commit**

Mutations, each a patch in `mutations-c/`, the ones that could reveal a partial file or skip the save first: in `ExportController.start`, run tap without `saveNow` (`Test: TapTests/ExportPDFTests/testExportAPDF`; expected: fails on "the buffer was saved first"); in `finished`, treat `exit.cancelled` as `.done` (`Test: .../testCancelAnExport`; expected: fails on `revealed == []`); in `cancel`, call `stop` on the run without changing `state` (expected: fails on "Cancelling…"); in `beginExport`, end the sheet on `.done` regardless of warnings (`Test: .../testASlideFailsDuringExport`; expected: fails on "keeps the sheet up"); in `summary(from:)`, drop the `brokenSlides` mapping (expected: fails on `warnings`); in `handle`, leave `state` on the download's status for a `.step` (`Test: .../testFirstPDFExport` still passes; `testExportAPDF` fails on `sawRender`); in `canStartATalk`, drop the export condition (`Test: .../testCancelAnExport`; expected: fails on `canStartATalk`); in `windowWillClose`, skip `exportController.stop()` (`Test: .../testClosingTheDeckStopsItsExport`; expected: the process is still alive); in `ExportRequest.arguments`, drop `--progress json` (`Test: .../testTapsFailureShowsInTheSheet` still fails the same; `testExportAPDF` fails on `sawRender`); in `finished`, show a `browser` failure as done (`Test: .../testTapsFailureShowsInTheSheet`; expected: fails on `.failed`).

```bash
git add desktop/Tap desktop/TapTests .github/workflows/ci.yml
git commit -m "feat(desktop): File > Export > PDF through tap export pdf, with real progress and a SIGINT cancel"
```

---

### Task 11: Export Website with Preview through `tap serve --json`, and Export Slide Images

**Files:**
- Create: `desktop/Tap/Export/PreviewServer.swift`
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (`previewServer`, `previewWebsite(at:)`, `openURL` seam, `windowWillClose`)
- Modify: `desktop/TapTests/Support/FakeToolScripts.swift` (`exportImages`)
- Test: `desktop/TapTests/ExportWebsiteTests.swift`

**Interfaces:**
- Consumes: `ToolRun` (Task 3), `ServeReady`, `BuildResult`, `ImagesExportResult` (Task 4), `TapTool.makeRun` (Task 6), `ExportController`, `ExportSheet` (Task 10).
- Produces: `PreviewServer` (`start(folder:completion:)`, `stop()`, `isRunning`, `url`), `DeckWindowController.openURL: (URL) -> Void`, `previewServer`. Task 14's manifest claims `testExportAStaticSite` and `testExportSlideImages`.

- [ ] **Step 1: The scripted images export and the failing tests**

In `FakeToolScripts.swift`:

```swift
    /// `tap export images --all`: a render line per slide, the files, and,
    /// with `broken`, tap's `slide N: reason` lines and its broken_slides
    /// failure after the files that did land, as tap does.
    static func exportImages(slides: Int, broken: [Int] = [], recordingTo record: URL) throws -> URL {
        let brokenList = broken.map(String.init).joined(separator: " ")
        return try write("""
          "export images")
            out=""; while [ $# -gt 0 ]; do case "$1" in --output|-o) out="$2"; shift ;; esac; shift; done
            mkdir -p "$out"; files=""
            i=1; while [ $i -le \(slides) ]; do
              echo "{\\"phase\\":\\"render\\",\\"done\\":$i,\\"total\\":\(slides)}" >&2
              case " \(brokenList) " in *" $i "*) echo "slide $i: an error card" >&2 ;; *) f=$(printf '%s/slide-%03d.png' "$out" "$i"); printf 'png' > "$f"; files="$files\\"$f\\"," ;; esac
              i=$((i + 1))
            done
            files="[${files%,}]"
            if [ -n "\(brokenList)" ]; then echo '{"phase":"done","ok":false,"error":{"code":"broken_slides","message":"\(broken.count) slide(s) failed to capture"}}' >&2; exit 1; fi
            echo "{\\"phase\\":\\"done\\",\\"ok\\":true,\\"files\\":$files}" >&2
            exit 0 ;;
        """, recordingTo: record)
    }
```

`desktop/TapTests/ExportWebsiteTests.swift`:

```swift
import XCTest
@testable import Tap
@testable import TapDesktopCore

final class ExportWebsiteTests: HostedTestCase {
    func openSevenSlides() async throws -> (DeckDocument, DeckWindowController, URL) {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        return (document, try XCTUnwrap(document.windowControllers.first as? DeckWindowController), deck)
    }

    func exportSheet(_ window: DeckWindowController) async throws -> ExportSheet {
        try await waitUntil(timeout: 5, "the export sheet") { window.window?.attachedSheet is ExportSheet }
        return try XCTUnwrap(window.window?.attachedSheet as? ExportSheet)
    }

    /// The real bundled tap: tap build needs no browser, and tap serve --json gives the port.
    func testExportAStaticSite() async throws {
        let (document, window, deck) = try await openSevenSlides()
        var revealed: [URL] = []
        var opened: [URL] = []
        window.revealInFinder = { revealed.append($0) }
        window.openURL = { opened.append($0) }

        window.exportWebsite(nil)
        let sheet = try await exportSheet(window)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Export Website")
        XCTAssertEqual(sheet.outputField.stringValue, deck.deletingLastPathComponent().appendingPathComponent("dist").path)
        XCTAssertEqual(sheet.request.arguments(deck: deck), ["build", deck.path, "--output", sheet.outputField.stringValue, "--progress", "json"])
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 120, "the done state") { if case .done = sheet.state { return true } else { return false } }
        guard case .done(let summary) = sheet.state else { return XCTFail("not done") }
        XCTAssertEqual(sheet.statusLabel.stringValue, "Website exported")
        XCTAssertTrue(summary.summary.hasPrefix("7 slides, "), summary.summary)
        XCTAssertTrue(summary.summary.hasSuffix("Live code does not run in a static site."))
        XCTAssertTrue(FileManager.default.fileExists(atPath: summary.output.appendingPathComponent("index.html").path), "tap build wrote the site")
        XCTAssertTrue(window.window?.attachedSheet === sheet, "the done state stays up for Show in Finder and Preview")
        sheet.revealButton.performClick(nil)
        XCTAssertEqual(revealed, [summary.output])

        sheet.previewButton.performClick(nil)
        try await waitUntil(timeout: 20, "tap serve's ready line") { !opened.isEmpty }
        let url = try XCTUnwrap(opened.first)
        XCTAssertTrue(url.absoluteString.hasPrefix("http://localhost:"), url.absoluteString)
        XCTAssertNotEqual(url.port, 3000, "a free port, not tap serve's default")
        let (data, response) = try await URLSession.shared.data(from: url.appendingPathComponent("index.html"))
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)
        XCTAssertTrue(String(decoding: data, as: UTF8.self).contains("<html"), "the built site is what tap serve serves")
        XCTAssertTrue(window.previewServer?.isRunning ?? false)
        sheet.doneButton.performClick(nil)
        try await waitUntil(timeout: 10, "the server to stop with the sheet") { window.previewServer?.isRunning == false }
        _ = document
    }

    func testExportSlideImages() async throws {
        let (_, window, deck) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportImages(slides: 7, recordingTo: record)
        var revealed: [URL] = []
        window.revealInFinder = { revealed.append($0) }

        window.exportImages(nil)
        let sheet = try await exportSheet(window)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Export Slide Images")
        let folder = deck.deletingLastPathComponent().appendingPathComponent("seven-slides-slides")
        XCTAssertEqual(sheet.outputField.stringValue, folder.path)
        XCTAssertEqual(sheet.request.arguments(deck: deck), ["export", "images", deck.path, "--all", "--output", folder.path, "--progress", "json"])
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 20, "the done state") { if case .done = sheet.state { return true } else { return false } }
        guard case .done(let summary) = sheet.state else { return XCTFail("not done") }
        XCTAssertEqual(summary.summary.split(separator: " ").first, "7")
        XCTAssertEqual(summary.output, folder)
        XCTAssertTrue(sheet.previewButton.isHidden, "nothing to serve")
        XCTAssertTrue(try String(contentsOf: record, encoding: .utf8).contains("arguments: export images \(deck.path) --all --output \(folder.path) --progress json"))
        sheet.revealButton.performClick(nil)
        XCTAssertEqual(revealed, [folder])
        sheet.doneButton.performClick(nil)
    }

    func testABrokenSlideInAnImagesExportIsAWarningWithTheFilesThatLanded() async throws {
        let (_, window, deck) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportImages(slides: 3, broken: [2], recordingTo: record)
        window.exportImages(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 20, "the done state") { if case .done = sheet.state { return true } else { return false } }
        guard case .done(let summary) = sheet.state else { return XCTFail("not done") }
        XCTAssertEqual(summary.warnings, ["1 slide(s) failed to capture"], "tap's message")
        XCTAssertTrue(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().appendingPathComponent("seven-slides-slides/slide-001.png").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().appendingPathComponent("seven-slides-slides/slide-002.png").path))
        sheet.doneButton.performClick(nil)
    }

    func testClosingTheDeckStopsThePreviewServer() async throws {
        let (document, window, _) = try await openSevenSlides()
        window.openURL = { _ in }
        window.exportWebsite(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 120, "the done state") { if case .done = sheet.state { return true } else { return false } }
        sheet.previewButton.performClick(nil)
        try await waitUntil(timeout: 20, "the server") { window.previewServer?.isRunning == true }
        let identifier = try XCTUnwrap(window.previewServer?.processIdentifier)
        document.close()
        try await waitUntil(timeout: 10, "the server process to be gone") { kill(identifier, 0) != 0 }
    }
}
```

- [ ] **Step 2: Build to verify they do not compile**

Run: `make -C desktop test-build 2>&1 | tail -3`
Expected: fails on `openURL`, `previewServer`.

- [ ] **Step 3: `PreviewServer` and the window's Preview**

`desktop/Tap/Export/PreviewServer.swift`:

```swift
import Foundation

/// `tap serve <folder> --port 0 --json` for the export sheet's Preview: tap
/// picks a free port and prints one ready line; the server runs until
/// the sheet or the deck closes, when SIGINT stops it.
@MainActor
final class PreviewServer {
    private var run: ToolRun?
    private(set) var url: URL?
    private let log: TapLog?

    init(log: TapLog?) {
        self.log = log
    }

    var isRunning: Bool { run?.isRunning ?? false }
    var processIdentifier: Int32? { run.map(\.processIdentifier) }

    /// Starts serving and reports the URL from tap's ready line, or nil
    /// when tap did not start (its error is in the log).
    func start(folder: URL, completion: @escaping (URL?) -> Void) {
        stop()
        Task { @MainActor [weak self] in
            guard let self else { return }
            let run = await TapTool.makeRun(["serve", folder.path, "--port", "0", "--json"], in: folder, timeout: nil, log: self.log)
            var answered = false
            run.onStandardOutputLine = { [weak self] line in
                guard !answered, let outcome = ToolOutcome.decode(Data(line.utf8)) else { return }
                answered = true
                let ready = try? outcome.result(ServeReady.self)
                self?.url = ready.flatMap { URL(string: $0.url) }
                completion(self?.url)
            }
            run.onExit = { [weak self] exit in
                self?.run = nil
                self?.url = nil
                if !answered {
                    answered = true
                    self?.log?.append("tap serve did not start (exit \(exit.status))", source: .app)
                    completion(nil)
                }
            }
            self.run = run
            do { try run.start() } catch {
                self.run = nil
                if !answered { answered = true; completion(nil) }
            }
        }
    }

    func stop() {
        run?.cancel()
    }
}
```

In `DeckWindowController.swift`: `private(set) var previewServer: PreviewServer?`, `var openURL: (URL) -> Void = { NSWorkspace.shared.open($0) }`, and `previewWebsite(at:)` becomes:

```swift
    /// The export sheet's Preview: tap serve on the built folder, the site
    /// opened in the default browser. The server stops with the sheet.
    func previewWebsite(at folder: URL) {
        let server = previewServer ?? PreviewServer(log: sessionController.session.log)
        previewServer = server
        server.start(folder: folder) { [weak self] url in
            guard let url else { return NSSound.beep() }
            self?.openURL(url)
        }
    }
```

In `beginExport`'s `window.beginSheet(sheet) { _ in }` (inside `showFormSheet`), the export sheet needs its own completion to stop the server: give `showFormSheet` an optional `completion: (() -> Void)? = nil` and pass `{ [weak self] in self?.previewServer?.stop() }` from `beginExport`. `windowWillClose` adds `previewServer?.stop()`.

- [ ] **Step 4: Build, then hand the hosted tests to CI**

Run: `make -C desktop build && make -C desktop test-build 2>&1 | tail -3`
Expected: both succeed. The controller's CI run confirms: `ExportWebsiteTests` (4) pass (the real `tap build` and `tap serve` run on the runner; `tap build` bundles nothing for `seven-slides.md`).

- [ ] **Step 5: Mutations and commit**

Mutations, each a patch in `mutations-c/`: in `PreviewServer.start`, pass `--port 3000` (`Test: TapTests/ExportWebsiteTests/testExportAStaticSite`; expected: fails on `url.port != 3000`, or on the ready line when 3000 is busy); in `start`, drop `--json` (expected: times out on the ready line); in `beginExport`, skip the sheet completion's `previewServer?.stop()` (expected: fails on "the server to stop with the sheet"); in `windowWillClose`, skip `previewServer?.stop()` (`Test: .../testClosingTheDeckStopsThePreviewServer`; expected: the process is still alive); in `ExportRequest.arguments` for `.images`, drop `--all` (`Test: .../testExportSlideImages`; expected: fails on `arguments`); in `ExportController.finished`, treat every `broken_slides` failure as `.failed` (`Test: .../testABrokenSlideInAnImagesExportIsAWarningWithTheFilesThatLanded`; expected: fails on `.done`); in `ExportKind.defaultOutput` for `.website`, return the working directory's `dist` (expected: `testExportAStaticSite` fails on `outputField`).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): Export Website with Preview through tap serve --json, and Export Slide Images"
```

---

### Task 12: The Settings window: General and Live Code

**Files:**
- Create: `desktop/Tap/Settings/SettingsWindowController.swift`, `desktop/Tap/Settings/GeneralSettingsViewController.swift`, `desktop/Tap/Settings/LiveCodeSettingsViewController.swift`
- Create: `desktop/Tap/Editor/EditorTypography.swift`
- Modify: `desktop/Tap/Editor/EditorTextView.swift` (the fonts and line height from `EditorTypography`, `applyTypography`)
- Modify: `desktop/Tap/App/AppDelegate.swift` (`showSettings(_:)`, the autosave delay), `desktop/Tap/App/MainMenu.swift` (Tap > Settings…, Check for Updates…)
- Test: `desktop/TapTests/SettingsTests.swift`

**Interfaces:**
- Consumes: `GeneralSettings` (Task 5), `ApprovalList`, `ApprovalRecord` (Task 4), `TapTool.run` (Task 6), `ThemeImageLoader.catalog` (Task 6), `NSDocumentController.shared.autosavingDelay`, `DeckWindowController.revealInFinder`'s pattern.
- Produces: `SettingsWindowController.shared` with `show(pane:)`, `Pane` (`.general`, `.liveCode`, `.imageGeneration`, `.commandLine`), `tabViewController`, `general`, `liveCode`, `imageGeneration`, `commandLine`; `GeneralSettingsViewController` (`fontSizePopup`, `lineSpacingControl`, `defaultThemePopup`, `autosavePopup`); `LiveCodeSettingsViewController` (`table`, `records`, `revokeButton`, `revealButton`, `reload()`, `revealInFinder` seam, `introLabel`); `EditorTypography.current`, `EditorTextView.applyTypography()`, `EditorTypography.didChangeNotification`; `AppDelegate.showSettings(_:)`. Task 13 adds the two other panes to the same window.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/SettingsTests.swift`:

```swift
import XCTest
@testable import Tap
@testable import TapDesktopCore

final class SettingsTests: HostedTestCase {
    var appDelegate: AppDelegate { NSApp.delegate as! AppDelegate }
    var settings: SettingsWindowController { SettingsWindowController.shared }

    override func tearDown() async throws {
        settings.window?.orderOut(nil)
        try await super.tearDown()
    }

    func testSettingsWindow() async throws {
        let tapMenu = try XCTUnwrap(NSApp.mainMenu?.items.first { $0.title == "Tap" }?.submenu)
        let item = try XCTUnwrap(tapMenu.items.first { $0.title == "Settings…" })
        XCTAssertEqual(item.keyEquivalent, ",")
        XCTAssertEqual(item.action, #selector(AppDelegate.showSettings(_:)))
        appDelegate.showSettings(nil)
        try await waitUntil(timeout: 5, "the window") { self.settings.window?.isVisible ?? false }
        XCTAssertEqual(settings.tabViewController.tabViewItems.map(\.label), ["General", "Live Code", "Image Generation", "Command Line"])
        XCTAssertEqual(settings.tabViewController.tabStyle, .toolbar)

        // Live Code lists the approvals tap keeps, through tap approval list.
        let deck = try Fixtures.copyDeck("live-code.md")
        try approveLiveCode(for: deck)
        settings.show(pane: .liveCode)
        settings.liveCode.reload()
        try await waitUntil(timeout: 20, "tap approval list") { self.settings.liveCode.records.contains { $0.deck == Fixtures.realPath(of: deck) } }
        let row = try XCTUnwrap(settings.liveCode.records.firstIndex { $0.deck == Fixtures.realPath(of: deck) })
        XCTAssertEqual(settings.liveCode.records[row].deckName, "live-code.md")
        XCTAssertEqual(settings.liveCode.records[row].driverSummary, "shell, sqlite")
        XCTAssertEqual(settings.liveCode.table.numberOfRows, settings.liveCode.records.count)
        XCTAssertTrue(settings.liveCode.introLabel.stringValue.contains("~/.config/tap/settings.yaml"))
        XCTAssertFalse(settings.liveCode.revokeButton.isEnabled, "nothing selected")
        settings.liveCode.table.selectRowIndexes([row], byExtendingSelection: false)
        XCTAssertTrue(settings.liveCode.revokeButton.isEnabled)
    }

    func testSharedSettings() async throws {
        // An approval tap wrote (through the D5 helper, in tap's own shape) shows; a Revoke here is what tap sees.
        let deck = try Fixtures.copyDeck("live-code.md")
        try approveLiveCode(for: deck)
        appDelegate.showSettings(nil)
        settings.show(pane: .liveCode)
        settings.liveCode.reload()
        try await waitUntil(timeout: 20, "the approval") { self.settings.liveCode.records.contains { $0.deck == Fixtures.realPath(of: deck) } }
        var revealed: [URL] = []
        settings.liveCode.revealInFinder = { revealed.append($0) }
        let row = try XCTUnwrap(settings.liveCode.records.firstIndex { $0.deck == Fixtures.realPath(of: deck) })
        settings.liveCode.table.selectRowIndexes([row], byExtendingSelection: false)
        settings.liveCode.revealButton.performClick(nil)
        XCTAssertEqual(revealed.map { Fixtures.realPath(of: $0) }, [Fixtures.realPath(of: deck)])

        settings.liveCode.revokeButton.performClick(nil)
        try await waitUntil(timeout: 20, "the revoke to land") { !self.settings.liveCode.records.contains { $0.deck == Fixtures.realPath(of: deck) } }
        let listed = try await TapApproval.run(["approval", "list", "--json"], configHome: configHome)
        XCTAssertFalse(listed.contains(Fixtures.realPath(of: deck)), "tap approval list agrees: \(listed)")
        XCTAssertFalse(storedApprovals().contains(Fixtures.realPath(of: deck)), "the file tap dev and tap present read")
    }

    func testGeneralSettings() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let editor = try XCTUnwrap(document.sessionController?.editor)
        XCTAssertEqual(editor.font?.pointSize, 13)
        appDelegate.showSettings(nil)
        settings.show(pane: .general)
        let general = settings.general
        XCTAssertEqual(general.fontSizePopup.titleOfSelectedItem, "13 pt")
        XCTAssertEqual(general.lineSpacingControl.label(forSegment: general.lineSpacingControl.selectedSegment), "Normal")
        XCTAssertEqual(general.autosavePopup.titleOfSelectedItem, "1 second")
        XCTAssertEqual(general.defaultThemePopup.titleOfSelectedItem, "tap's default")
        try await waitUntil(timeout: 20, "the catalog") { AppEnvironment.shared.themeImages.catalog != nil }
        XCTAssertTrue(general.defaultThemePopup.itemTitles.contains("Terminal"), "every theme from tap: \(general.defaultThemePopup.itemTitles)")

        general.fontSizePopup.selectItem(withTitle: "16 pt")
        general.fontSizePopup.performClick(nil)
        general.lineSpacingControl.selectedSegment = 2
        general.lineSpacingControl.performClick(nil)
        general.defaultThemePopup.selectItem(withTitle: "Terminal")
        general.defaultThemePopup.performClick(nil)
        general.autosavePopup.selectItem(withTitle: "5 seconds")
        general.autosavePopup.performClick(nil)

        let stored = AppEnvironment.shared.generalSettings
        XCTAssertEqual(stored.fontSize, 16)
        XCTAssertEqual(stored.lineSpacing, .roomy)
        XCTAssertEqual(stored.defaultTheme, "terminal", "the slug, for tap new --theme")
        XCTAssertEqual(stored.autosaveDelay, 5)
        XCTAssertEqual(NSDocumentController.shared.autosavingDelay, 5)
        XCTAssertEqual(editor.font?.pointSize, 16, "the open editor follows at once")
        XCTAssertEqual(EditorTypography.current.lineHeight, GeneralSettings.LineSpacing.roomy.lineHeight(forFontSize: 16))
        let style = editor.textStorage?.attribute(.paragraphStyle, at: editor.hiddenLength, effectiveRange: nil) as? NSParagraphStyle
        XCTAssertEqual(style?.minimumLineHeight, EditorTypography.current.lineHeight, "the text was restyled")
        general.fontSizePopup.selectItem(withTitle: "13 pt")
        general.fontSizePopup.performClick(nil)
        general.lineSpacingControl.selectedSegment = 1
        general.lineSpacingControl.performClick(nil)
        general.autosavePopup.selectItem(withTitle: "1 second")
        general.autosavePopup.performClick(nil)
    }
}
```

- [ ] **Step 2: Build to verify they do not compile**

Run: `make -C desktop test-build 2>&1 | tail -3`
Expected: fails on `SettingsWindowController`, `showSettings`, `EditorTypography`.

- [ ] **Step 3: The editor's typography from the settings**

`desktop/Tap/Editor/EditorTypography.swift`:

```swift
import AppKit

/// The editor's font size and line height, from General settings, shared
/// by every editor. A change restyles every open editor.
struct EditorTypography: Equatable {
    static let didChangeNotification = Notification.Name("TapEditorTypographyDidChange")
    let fontSize: CGFloat
    let lineHeight: CGFloat

    var font: NSFont { .monospacedSystemFont(ofSize: fontSize, weight: .regular) }
    var boldFont: NSFont { .monospacedSystemFont(ofSize: fontSize, weight: .bold) }
    var smallFont: NSFont { .monospacedSystemFont(ofSize: max(8, (fontSize * 0.7).rounded()), weight: .regular) }

    static func from(_ settings: GeneralSettings) -> EditorTypography {
        EditorTypography(fontSize: settings.fontSize, lineHeight: settings.lineSpacing.lineHeight(forFontSize: settings.fontSize))
    }

    @MainActor private(set) static var current = EditorTypography(fontSize: 13, lineHeight: 21)

    /// Reads the settings and tells every editor when they changed.
    @MainActor static func refresh(from settings: GeneralSettings) {
        let next = from(settings)
        guard next != current else { return }
        current = next
        NotificationCenter.default.post(name: didChangeNotification, object: nil)
    }
}
```

In `EditorTextView`, the four statics become `static var font: NSFont { EditorTypography.current.font }`, `static var boldFont`, `static var smallFont`, `static var lineHeight: CGFloat { EditorTypography.current.lineHeight }`; `paragraphStyles` is cleared in `applyTypography()`. In `make()`, after `view.textStorage?.delegate = view`, observe the notification: `NotificationCenter.default.addObserver(view, selector: #selector(applyTypography), name: EditorTypography.didChangeNotification, object: nil)`, and add:

```swift
    /// The settings' font size or line spacing changed: every paragraph is
    /// styled again with the new font and line height.
    @objc func applyTypography() {
        Self.paragraphStyles = [:]
        font = Self.font
        typingAttributes = [.font: Self.font, .foregroundColor: NSColor.labelColor, .paragraphStyle: Self.paragraphStyle(for: .boxMiddle)]
        restyle(NSRange(location: 0, length: (string as NSString).length))
        needsDisplay = true
    }
```

(`restyle` is D2's private method; make it `fileprivate` or call the same `restyle` through a small internal wrapper.) `AppEnvironment.warmUp` calls `EditorTypography.refresh(from: generalSettings)` once and observes `GeneralSettings.didChangeNotification` to call it again and set `NSDocumentController.shared.autosavingDelay = generalSettings.autosaveDelay`; `AppDelegate.applicationDidFinishLaunching` sets the delay from the settings instead of the literal `1`. `HostedTestCase.setUp` calls `EditorTypography.refresh(from:)` after replacing `generalSettings`, so a test's fresh suite is what the editor reads.

- [ ] **Step 4: The window and the two panes (the SettingsGeneral and SettingsLiveCode boards)**

The SettingsGeneral board draws a preferences window with a toolbar of four panes (General, Live Code, Image Generation, Command Line) and, in General, an "Editor" card with "Font size" (a popup, "13 pt") and "Line spacing" (Tight, Normal, Roomy), a "New decks" card with "Default theme" (a popup with the theme's name and the hint "Preselected in New Deck. Passed to tap new --theme.") and a "Saving" card with "Autosave after typing stops" (a popup, "1 second"). The SettingsLiveCode board draws the line "Decks you allowed to run code on this Mac. tap keeps this list in ~/.config/tap/settings.yaml, so tap dev and tap present in Terminal use the same approvals.", a table with Deck (the name over the folder), Drivers (monospaced, "shell, kubectl (custom)") and Allowed (the date), and Revoke and Show in Finder under it. `desktop/Tap/Settings/SettingsWindowController.swift`:

```swift
import AppKit

/// Tap > Settings: one window, four panes as tabs in the toolbar style.
final class SettingsWindowController: NSWindowController {
    static let shared = SettingsWindowController()

    enum Pane: Int {
        case general = 0, liveCode, imageGeneration, commandLine
    }

    let tabViewController = NSTabViewController()
    let general = GeneralSettingsViewController()
    let liveCode = LiveCodeSettingsViewController()
    let imageGeneration = ImageGenerationSettingsViewController()
    let commandLine = CommandLineSettingsViewController()

    private init() {
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 760, height: 520), styleMask: [.titled, .closable], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        window.center()
        super.init(window: window)
        tabViewController.tabStyle = .toolbar
        for (pane, symbol) in [(general, "gearshape"), (liveCode, "checkmark.shield"), (imageGeneration, "sparkles"), (commandLine, "terminal")] as [(NSViewController, String)] {
            let item = NSTabViewItem(viewController: pane)
            item.label = pane.title ?? ""
            item.image = NSImage(systemSymbolName: symbol, accessibilityDescription: pane.title)
            tabViewController.addTabViewItem(item)
        }
        window.contentViewController = tabViewController
        window.setAccessibilityIdentifier("settings-window")
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func show(pane: Pane) {
        tabViewController.selectedTabViewItemIndex = pane.rawValue
        window?.title = tabViewController.tabViewItems[pane.rawValue].label
        showWindow(nil)
    }
}
```

`ImageGenerationSettingsViewController` and `CommandLineSettingsViewController` are Task 13's; here they are two empty `NSViewController`s with their titles, replaced there. `desktop/Tap/Settings/GeneralSettingsViewController.swift`:

```swift
import AppKit

/// General: the editor's font size and line spacing, the theme New Deck
/// preselects, and the autosave delay, in GeneralSettings.
final class GeneralSettingsViewController: NSViewController {
    let fontSizePopup = NSPopUpButton(frame: .zero, pullsDown: false)
    let lineSpacingControl = NSSegmentedControl(labels: ["Tight", "Normal", "Roomy"], trackingMode: .selectOne, target: nil, action: nil)
    let defaultThemePopup = NSPopUpButton(frame: .zero, pullsDown: false)
    let autosavePopup = NSPopUpButton(frame: .zero, pullsDown: false)
    static let noDefaultTheme = "tap's default"
    private var catalogObserver: NSObjectProtocol?

    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        super.init(nibName: nil, bundle: nil)
        title = "General"
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        for size in GeneralSettings.fontSizes { fontSizePopup.addItem(withTitle: "\(Int(size)) pt") }
        fontSizePopup.target = self
        fontSizePopup.action = #selector(changed(_:))
        fontSizePopup.setAccessibilityIdentifier("settings-font-size")
        lineSpacingControl.target = self
        lineSpacingControl.action = #selector(changed(_:))
        lineSpacingControl.setAccessibilityIdentifier("settings-line-spacing")
        defaultThemePopup.target = self
        defaultThemePopup.action = #selector(changed(_:))
        defaultThemePopup.setAccessibilityIdentifier("settings-default-theme")
        for delay in GeneralSettings.autosaveDelays { autosavePopup.addItem(withTitle: Self.title(forDelay: delay)) }
        autosavePopup.target = self
        autosavePopup.action = #selector(changed(_:))
        autosavePopup.setAccessibilityIdentifier("settings-autosave")
        let editorCard = SettingsCard(title: "Editor", rows: [("Font size", nil, fontSizePopup), ("Line spacing", nil, lineSpacingControl)])
        let decksCard = SettingsCard(title: "New decks", rows: [("Default theme", "Preselected in New Deck. Passed to tap new --theme.", defaultThemePopup)])
        let savingCard = SettingsCard(title: "Saving", rows: [("Autosave after typing stops", nil, autosavePopup)])
        let stack = NSStackView(views: [editorCard, decksCard, savingCard])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 14
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 28, bottom: 20, right: 28)
        for card in [editorCard, decksCard, savingCard] { card.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true }
        stack.widthAnchor.constraint(equalToConstant: 760).isActive = true
        view = stack
        rebuildThemes()
        catalogObserver = NotificationCenter.default.addObserver(forName: ThemeImageLoader.didLoadCatalogNotification, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.rebuildThemes() }
        }
        AppEnvironment.shared.themeImages.loadAll()
        refresh()
    }

    deinit {
        if let catalogObserver { NotificationCenter.default.removeObserver(catalogObserver) }
    }

    static func title(forDelay delay: TimeInterval) -> String {
        delay == 1 ? "1 second" : delay < 1 ? "\(delay) seconds" : "\(Int(delay)) seconds"
    }

    private func rebuildThemes() {
        defaultThemePopup.removeAllItems()
        defaultThemePopup.addItem(withTitle: Self.noDefaultTheme)
        for theme in AppEnvironment.shared.themeImages.catalog?.themes ?? [] {
            defaultThemePopup.addItem(withTitle: theme.name)
            defaultThemePopup.lastItem?.representedObject = theme.slug
        }
        refresh()
    }

    /// The controls from the settings.
    func refresh() {
        let settings = AppEnvironment.shared.generalSettings
        fontSizePopup.selectItem(withTitle: "\(Int(settings.fontSize)) pt")
        lineSpacingControl.selectedSegment = GeneralSettings.LineSpacing.allCases.firstIndex(of: settings.lineSpacing) ?? 1
        autosavePopup.selectItem(withTitle: Self.title(forDelay: settings.autosaveDelay))
        if let slug = settings.defaultTheme, let index = defaultThemePopup.itemArray.firstIndex(where: { $0.representedObject as? String == slug }) {
            defaultThemePopup.selectItem(at: index)
        } else {
            defaultThemePopup.selectItem(at: 0)
        }
    }

    @objc private func changed(_ sender: Any?) {
        let settings = AppEnvironment.shared.generalSettings
        switch sender as AnyObject? {
        case let popup as NSPopUpButton where popup === fontSizePopup:
            settings.fontSize = GeneralSettings.fontSizes[max(0, popup.indexOfSelectedItem)]
        case let control as NSSegmentedControl where control === lineSpacingControl:
            settings.lineSpacing = GeneralSettings.LineSpacing.allCases[max(0, control.selectedSegment)]
        case let popup as NSPopUpButton where popup === defaultThemePopup:
            settings.defaultTheme = popup.selectedItem?.representedObject as? String
        case let popup as NSPopUpButton where popup === autosavePopup:
            settings.autosaveDelay = GeneralSettings.autosaveDelays[max(0, popup.indexOfSelectedItem)]
        default:
            break
        }
    }
}

/// A titled card of label-left, control-right rows, as the Settings
/// boards draw them (the Deck tab's cards of D5 use the same shape).
final class SettingsCard: NSBox {
    init(title: String, rows: [(String, String?, NSView)]) {
        super.init(frame: .zero)
        self.title = title
        titlePosition = .aboveTop
        titleFont = .systemFont(ofSize: 11, weight: .semibold)
        let stack = NSStackView(views: rows.map { label, hint, control in
            let name = NSTextField(labelWithString: label)
            name.font = .systemFont(ofSize: 13)
            var left: [NSView] = [name]
            if let hint {
                let hintLabel = NSTextField(labelWithString: hint)
                hintLabel.font = .systemFont(ofSize: 11)
                hintLabel.textColor = .secondaryLabelColor
                left.append(hintLabel)
            }
            let labels = NSStackView(views: left)
            labels.orientation = .vertical
            labels.alignment = .leading
            labels.spacing = 2
            let row = NSStackView(views: [labels, NSView(), control])
            row.orientation = .horizontal
            row.alignment = .centerY
            row.spacing = 12
            return row
        })
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 8
        stack.edgeInsets = NSEdgeInsets(top: 8, left: 12, bottom: 8, right: 12)
        for row in stack.arrangedSubviews { row.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -24).isActive = true }
        contentView = stack
        setAccessibilityIdentifier("settings-card-\(title)")
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }
}
```

`desktop/Tap/Settings/LiveCodeSettingsViewController.swift`:

```swift
import AppKit

/// Live Code: the decks tap allows to run code, from tap approval list;
/// Revoke runs tap approval revoke. The list is tap's file, so the CLI
/// and the app see the same approvals. Commands are shown as tap prints
/// them, secrets masked by tap; nothing is expanded here.
final class LiveCodeSettingsViewController: NSViewController, NSTableViewDataSource, NSTableViewDelegate {
    let introLabel = NSTextField(wrappingLabelWithString: "Decks you allowed to run code on this Mac. tap keeps this list in ~/.config/tap/settings.yaml, so tap dev and tap present in Terminal use the same approvals.")
    let table = NSTableView()
    let revokeButton = NSButton(title: "Revoke", target: nil, action: nil)
    let revealButton = NSButton(title: "Show in Finder", target: nil, action: nil)
    private(set) var records: [ApprovalRecord] = []
    var revealInFinder: (URL) -> Void = { url in NSWorkspace.shared.activateFileViewerSelecting([url]) }
    private static let dateFormatter: RelativeDateTimeFormatter = {
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .short
        return formatter
    }()

    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        super.init(nibName: nil, bundle: nil)
        title = "Live Code"
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        introLabel.font = .systemFont(ofSize: 12.5)
        introLabel.textColor = .secondaryLabelColor
        for (identifier, title, width) in [("deck", "Deck", 300.0), ("drivers", "Drivers", 220.0), ("allowed", "Allowed", 90.0)] {
            let column = NSTableColumn(identifier: NSUserInterfaceItemIdentifier(identifier))
            column.title = title
            column.width = width
            table.addTableColumn(column)
        }
        table.dataSource = self
        table.delegate = self
        table.rowHeight = 42
        table.style = .inset
        table.setAccessibilityIdentifier("approvals-table")
        let scroll = NSScrollView()
        scroll.documentView = table
        scroll.hasVerticalScroller = true
        scroll.heightAnchor.constraint(equalToConstant: 300).isActive = true
        for button in [revokeButton, revealButton] {
            button.bezelStyle = .rounded
            button.target = self
            button.isEnabled = false
        }
        revokeButton.action = #selector(revokePressed(_:))
        revokeButton.setAccessibilityIdentifier("approvals-revoke")
        revealButton.action = #selector(revealPressed(_:))
        revealButton.setAccessibilityIdentifier("approvals-reveal")
        let buttons = NSStackView(views: [revokeButton, revealButton])
        buttons.spacing = 8
        let stack = NSStackView(views: [introLabel, scroll, buttons])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 14
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 28, bottom: 20, right: 28)
        stack.widthAnchor.constraint(equalToConstant: 760).isActive = true
        introLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true
        scroll.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true
        view = stack
    }

    override func viewWillAppear() {
        super.viewWillAppear()
        reload()
    }

    /// tap approval list --json, again.
    func reload() {
        Task { @MainActor [weak self] in
            let exit = await TapTool.run(["approval", "list", "--json"], timeout: 30)
            guard let self, let list = try? exit.outcome?.result(ApprovalList.self) else { return }
            self.records = list.approvals
            self.table.reloadData()
            self.selectionChanged()
        }
    }

    private var selectedRecord: ApprovalRecord? {
        let row = table.selectedRow
        return records.indices.contains(row) ? records[row] : nil
    }

    private func selectionChanged() {
        revokeButton.isEnabled = selectedRecord != nil
        revealButton.isEnabled = selectedRecord.map { FileManager.default.fileExists(atPath: $0.deck) } ?? false
    }

    @objc private func revokePressed(_ sender: Any?) {
        guard let record = selectedRecord else { return }
        revokeButton.isEnabled = false
        Task { @MainActor [weak self] in
            _ = await TapTool.run(["approval", "revoke", record.deck, "--json"], timeout: 30)
            self?.reload()
        }
    }

    @objc private func revealPressed(_ sender: Any?) {
        guard let record = selectedRecord else { return }
        revealInFinder(URL(fileURLWithPath: record.deck))
    }

    func numberOfRows(in tableView: NSTableView) -> Int { records.count }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        let record = records[row]
        switch tableColumn?.identifier.rawValue {
        case "deck":
            let name = NSTextField(labelWithString: record.deckName)
            name.font = .systemFont(ofSize: 13, weight: .semibold)
            let folder = NSTextField(labelWithString: (record.folderPath as NSString).abbreviatingWithTildeInPath)
            folder.font = .systemFont(ofSize: 11)
            folder.textColor = .secondaryLabelColor
            let stack = NSStackView(views: [name, folder])
            stack.orientation = .vertical
            stack.alignment = .leading
            stack.spacing = 1
            return stack
        case "drivers":
            let label = NSTextField(labelWithString: record.driverSummary)
            label.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
            label.toolTip = record.commands.map { commands in commands.map { "\($0.key): \($0.value.joined(separator: " "))" }.sorted().joined(separator: "\n") }
            return label
        default:
            let label = NSTextField(labelWithString: record.approvedAtDate.map { Self.dateFormatter.localizedString(for: $0, relativeTo: Date()) } ?? record.approvedAt)
            label.textColor = .secondaryLabelColor
            return label
        }
    }

    func tableViewSelectionDidChange(_ notification: Notification) { selectionChanged() }
}
```

In `AppDelegate`: `@objc func showSettings(_ sender: Any?) { SettingsWindowController.shared.show(pane: .general) }` replacing Task 8's stub. In `MainMenu.tapMenu`, after About Tap and a separator, as the MenusFile board draws: `menu.addItem(item("Check for Updates…", action: nil))` (D7's; no action, so disabled) and `menu.addItem(item("Settings…", action: #selector(AppDelegate.showSettings(_:)), key: ","))`; the Install Command Line Tool… item is Task 13's.

- [ ] **Step 5: Build, then hand the hosted tests to CI**

Run: `make -C desktop build && make -C desktop test-build 2>&1 | tail -3 && make -C desktop bench-build | tail -3`
Expected: all succeed; the benchmarks still compile (the typing benchmark measures the editor, whose fonts now come from `EditorTypography.current`, the same values by default). The controller's CI run confirms: `SettingsTests` (3) pass; the Desktop Benchmarks job's typing numbers are no worse than before (a static var read per style, not per keystroke).

- [ ] **Step 6: Mutations and commit**

Mutations, each a patch in `mutations-d/`: in `LiveCodeSettingsViewController.revokePressed`, run `approval list` instead of `revoke` (`Test: TapTests/SettingsTests/testSharedSettings`; expected: fails on "the revoke to land"); in `reload`, decode nothing and keep `records` (`Test: .../testSettingsWindow`; expected: times out on the record); in `driverSummary`'s use, show `drivers.joined(", ")` without `(custom)` (into `survivors-d/`: the fixture declares built-in drivers only, no custom command; Task 4's core test covers the suffix); in `GeneralSettingsViewController.changed`, write the theme's name instead of its slug (`Test: .../testGeneralSettings`; expected: fails on "the slug"); in `AppEnvironment`'s settings observer, skip `NSDocumentController.shared.autosavingDelay` (expected: fails on `autosavingDelay`); in `applyTypography`, skip `restyle` (expected: fails on `minimumLineHeight`); in `EditorTypography.refresh`, never post the notification (expected: fails on `editor.font?.pointSize`); in `SettingsWindowController.init`, drop the Live Code tab (expected: `testSettingsWindow` fails on the labels).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the Settings window with General and Live Code"
```

---

### Task 13: Image Generation with the Keychain, and Command Line with Install

**Files:**
- Create: `desktop/Tap/Settings/ImageGenerationSettingsViewController.swift`, `desktop/Tap/Settings/CommandLineSettingsViewController.swift`, `desktop/Tap/Settings/CommandLineInstaller.swift`
- Modify: `desktop/Tap/App/AppDelegate.swift` (`installCommandLineTool(_:)`), `desktop/Tap/App/MainMenu.swift` (Tap > Install Command Line Tool…), `desktop/Tap/App/AppEnvironment.swift` (`commandLineInstaller`)
- Test: `desktop/TapTests/ImageGenerationSettingsTests.swift`, `desktop/TapTests/CommandLineSettingsTests.swift`

**Interfaces:**
- Consumes: `GeminiKeyStore`, `GeminiKeySource`, `CommandLineTool` (Task 5), `AppEnvironment.geminiKeySource()`, `tapEnvironment()`, `readVersion(of:)`, `bundledTapVersion`, `tapExecutableURL`, `SettingsWindowController`, `SettingsCard` (Task 12), `QuestionSheet`.
- Produces: `ImageGenerationSettingsViewController` (`keyField`, `sourceLabel`, `refresh()`, `keyChanged`); `CommandLineInstaller(linkDirectory:bundledTap:)` with `existingFile()`, `install() throws -> URL`, `InstallError`; `CommandLineSettingsViewController` (`bundledLabel`, `bundledPathLabel`, `otherLabel`, `otherPathLabel`, `otherNoteLabel`, `installButton`, `installHint`, `refresh()`, `install()`, `confirmInstall` seam, `otherTap: (path: String, version: String?)?`); `AppEnvironment.commandLineInstaller`; `AppDelegate.installCommandLineTool(_:)`.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/ImageGenerationSettingsTests.swift`:

```swift
import XCTest
@testable import Tap
@testable import TapDesktopCore

final class ImageGenerationSettingsTests: HostedTestCase {
    var settings: SettingsWindowController { SettingsWindowController.shared }

    override func tearDown() async throws {
        settings.window?.orderOut(nil)
        try await super.tearDown()
    }

    func testGeminiKey() async throws {
        let store = MemoryGeminiKeyStore()
        AppEnvironment.shared.geminiKeyStore = store
        (NSApp.delegate as! AppDelegate).showSettings(nil)
        settings.show(pane: .imageGeneration)
        let pane = settings.imageGeneration
        try await waitUntil(timeout: 5, "the pane") { !pane.sourceLabel.stringValue.isEmpty }
        XCTAssertTrue(pane.keyField is NSSecureTextField, "bullets only, never the key")
        XCTAssertEqual(pane.keyField.stringValue, "")
        XCTAssertEqual(pane.sourceLabel.stringValue, "Stored in your Keychain. GEMINI_API_KEY from your shell takes precedence.")
        XCTAssertTrue(pane.keyField.isEnabled)

        pane.keyField.stringValue = "placeholder-not-a-secret"
        pane.keyChanged(pane.keyField)
        XCTAssertEqual(store.writes.count, 1)
        XCTAssertEqual(store.key, "placeholder-not-a-secret", "written to the store, which is the Keychain in the app")
        let environment = await AppEnvironment.shared.tapEnvironment()
        XCTAssertEqual(environment["GEMINI_API_KEY"], "placeholder-not-a-secret", "passed to tap as GEMINI_API_KEY")
        let source = await AppEnvironment.shared.geminiKeySource()
        XCTAssertEqual(source, .keychain)

        // The shell's key wins, and the pane says so instead of editing a key nothing reads.
        AppEnvironment.shared.extraEnvironment["GEMINI_API_KEY"] = "shell-placeholder"
        await pane.refresh()
        XCTAssertFalse(pane.keyField.isEnabled)
        XCTAssertEqual(pane.sourceLabel.stringValue, "Your shell sets GEMINI_API_KEY, so tap uses that key; the Keychain's is not used.")
        let shellEnvironment = await AppEnvironment.shared.tapEnvironment()
        XCTAssertEqual(shellEnvironment["GEMINI_API_KEY"], "shell-placeholder")
        AppEnvironment.shared.extraEnvironment["GEMINI_API_KEY"] = ""

        // Clearing the field removes the key.
        await pane.refresh()
        pane.keyField.stringValue = ""
        pane.keyChanged(pane.keyField)
        XCTAssertNil(store.key)
        let cleared = await AppEnvironment.shared.tapEnvironment()
        XCTAssertNil(cleared["GEMINI_API_KEY"])

        // The key appears nowhere a person or a log could read it.
        XCTAssertFalse(pane.view.accessibilityChildren()?.description.contains("placeholder-not-a-secret") ?? true)
        for document in NSDocumentController.shared.documents {
            XCTAssertFalse(((document as? DeckDocument)?.sessionController?.session.log.text ?? "").contains("placeholder-not-a-secret"))
        }
    }
}
```

`desktop/TapTests/CommandLineSettingsTests.swift`:

```swift
import XCTest
@testable import Tap
@testable import TapDesktopCore

final class CommandLineSettingsTests: HostedTestCase {
    var settings: SettingsWindowController { SettingsWindowController.shared }
    var linkDirectory: URL!
    var pathDirectory: URL!

    override func setUp() async throws {
        try await super.setUp()
        linkDirectory = try Fixtures.temporaryFolder().appendingPathComponent("local-bin")
        pathDirectory = try Fixtures.temporaryFolder()
        AppEnvironment.shared.commandLineInstaller = CommandLineInstaller(linkDirectory: linkDirectory, bundledTap: AppEnvironment.shared.tapExecutableURL)
        AppEnvironment.shared.extraEnvironment["PATH"] = "\(linkDirectory.path):\(pathDirectory.path):/usr/bin:/bin"
    }

    override func tearDown() async throws {
        settings.window?.orderOut(nil)
        try await super.tearDown()
    }

    /// A tap of someone else's on PATH: a script that answers --version.
    func installOtherTap(version: String) throws -> URL {
        let other = pathDirectory.appendingPathComponent("tap")
        try "#!/bin/sh\necho 'tap version \(version)'\n".write(to: other, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: other.path)
        return other
    }

    func showPane() async throws -> CommandLineSettingsViewController {
        (NSApp.delegate as! AppDelegate).showSettings(nil)
        settings.show(pane: .commandLine)
        let pane = settings.commandLine
        await pane.refresh()
        return pane
    }

    func testInstallTheTapCommand() async throws {
        let pane = try await showPane()
        XCTAssertTrue(pane.bundledLabel.stringValue.hasPrefix("tap "), pane.bundledLabel.stringValue)
        XCTAssertEqual(pane.bundledPathLabel.stringValue, AppEnvironment.shared.tapExecutableURL.path)
        XCTAssertEqual(pane.otherLabel.stringValue, "No other tap is on your PATH.")
        XCTAssertEqual(pane.installButton.title, "Install in \((linkDirectory.path as NSString).abbreviatingWithTildeInPath)…")
        var confirmations = 0
        pane.confirmInstall = { completion in confirmations += 1; completion(true) }
        pane.installButton.performClick(nil)
        try await waitUntil(timeout: 10, "the link") { FileManager.default.fileExists(atPath: self.linkDirectory.appendingPathComponent("tap").path) }
        XCTAssertEqual(confirmations, 1, "after I confirm")
        let destination = try FileManager.default.destinationOfSymbolicLink(atPath: linkDirectory.appendingPathComponent("tap").path)
        XCTAssertEqual(destination, AppEnvironment.shared.tapExecutableURL.path)
        try await waitUntil(timeout: 5, "the pane to notice") { pane.installButton.title == "Installed" }
        XCTAssertFalse(pane.installButton.isEnabled)

        // A declined confirmation installs nothing; a file that is not ours is never touched.
        try FileManager.default.removeItem(at: linkDirectory.appendingPathComponent("tap"))
        pane.confirmInstall = { completion in completion(false) }
        await pane.refresh()
        pane.installButton.performClick(nil)
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertFalse(FileManager.default.fileExists(atPath: linkDirectory.appendingPathComponent("tap").path))
        try "#!/bin/sh\necho someone else's\n".write(to: linkDirectory.appendingPathComponent("tap"), atomically: true, encoding: .utf8)
        pane.confirmInstall = { completion in completion(true) }
        await pane.refresh()
        XCTAssertFalse(pane.installButton.isEnabled, "refused before any click")
        XCTAssertEqual(pane.installHint.stringValue, "\((linkDirectory.path as NSString).abbreviatingWithTildeInPath)/tap is a tap that Tap did not install. Tap never replaces or deletes it.")
        XCTAssertThrowsError(try AppEnvironment.shared.commandLineInstaller.install())
        XCTAssertEqual(try String(contentsOf: linkDirectory.appendingPathComponent("tap"), encoding: .utf8), "#!/bin/sh\necho someone else's\n", "untouched")
    }

    func testAnotherTapIsAlreadyInstalled() async throws {
        let other = try installOtherTap(version: "2.0.0")
        let pane = try await showPane()
        try await waitUntil(timeout: 10, "the other tap's version") { pane.otherLabel.stringValue == "tap 2.0.0" }
        XCTAssertEqual(pane.otherPathLabel.stringValue, other.path)
        XCTAssertEqual(pane.otherNoteLabel.stringValue, "Tap never replaces or deletes it.")
        XCTAssertEqual(pane.installHint.stringValue, "Asks first. \((linkDirectory.path as NSString).abbreviatingWithTildeInPath) comes before \(pathDirectory.path) on your PATH.")
        XCTAssertTrue(pane.installButton.isEnabled)
        // The other tap stays exactly as it was after an install.
        pane.confirmInstall = { completion in completion(true) }
        pane.installButton.performClick(nil)
        try await waitUntil(timeout: 10, "the link") { FileManager.default.fileExists(atPath: self.linkDirectory.appendingPathComponent("tap").path) }
        XCTAssertEqual(try String(contentsOf: other, encoding: .utf8), "#!/bin/sh\necho 'tap version 2.0.0'\n")
        // With the other tap first on PATH, the hint says the link would not be used.
        AppEnvironment.shared.extraEnvironment["PATH"] = "\(pathDirectory.path):\(linkDirectory.path):/usr/bin:/bin"
        await pane.refresh()
        XCTAssertEqual(pane.installHint.stringValue, "\(pathDirectory.path) comes before \((linkDirectory.path as NSString).abbreviatingWithTildeInPath) on your PATH, so Terminal would still run tap 2.0.0.")
    }

    func testTheMenuItemOpensThePane() throws {
        let tapMenu = try XCTUnwrap(NSApp.mainMenu?.items.first { $0.title == "Tap" }?.submenu)
        let item = try XCTUnwrap(tapMenu.items.first { $0.title == "Install Command Line Tool…" })
        XCTAssertEqual(item.action, #selector(AppDelegate.installCommandLineTool(_:)))
        (NSApp.delegate as! AppDelegate).installCommandLineTool(nil)
        XCTAssertEqual(settings.tabViewController.selectedTabViewItemIndex, SettingsWindowController.Pane.commandLine.rawValue)
    }
}
```

- [ ] **Step 2: Build to verify they do not compile**

Run: `make -C desktop test-build 2>&1 | tail -3`
Expected: fails on `CommandLineInstaller`, `commandLineInstaller`, the two panes' members.

- [ ] **Step 3: The Image Generation pane (the SettingsImage board)**

The board draws a "Gemini" card with an "API key" secure field and the hint "Stored in your Keychain. GEMINI_API_KEY from your shell takes precedence.", and a "Model" row. The field shows bullets alone (the board's "•••• 4f2c" tail is not built: pre-flight 6), and the Model row is not built (tap names no model in any command; open question 8). `desktop/Tap/Settings/ImageGenerationSettingsViewController.swift`:

```swift
import AppKit

/// Image Generation: the Gemini key, in the Keychain, passed to tap as
/// GEMINI_API_KEY. The field is the one place the key is shown, as
/// bullets. With a key in the login shell the field is off: that key wins.
final class ImageGenerationSettingsViewController: NSViewController, NSTextFieldDelegate {
    let keyField: NSTextField = NSSecureTextField(string: "")
    let sourceLabel = NSTextField(wrappingLabelWithString: "")

    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        super.init(nibName: nil, bundle: nil)
        title = "Image Generation"
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        keyField.placeholderString = "Paste your key"
        keyField.delegate = self
        keyField.target = self
        keyField.action = #selector(keyChanged(_:))
        keyField.setAccessibilityIdentifier("gemini-key")
        keyField.widthAnchor.constraint(equalToConstant: 300).isActive = true
        sourceLabel.font = .systemFont(ofSize: 11)
        sourceLabel.textColor = .secondaryLabelColor
        sourceLabel.setAccessibilityIdentifier("gemini-key-source")
        let card = SettingsCard(title: "Gemini", rows: [("API key", nil, keyField)])
        let stack = NSStackView(views: [card, sourceLabel])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 8
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 28, bottom: 20, right: 28)
        stack.widthAnchor.constraint(equalToConstant: 760).isActive = true
        card.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true
        sourceLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true
        view = stack
    }

    override func viewWillAppear() {
        super.viewWillAppear()
        Task { @MainActor [weak self] in await self?.refresh() }
    }

    /// The field from the store and the label from where tap's key comes from.
    func refresh() async {
        let source = await AppEnvironment.shared.geminiKeySource()
        keyField.stringValue = (try? AppEnvironment.shared.geminiKeyStore.read()) ?? ""
        switch source {
        case .shell:
            keyField.isEnabled = false
            sourceLabel.stringValue = "Your shell sets GEMINI_API_KEY, so tap uses that key; the Keychain's is not used."
        case .keychain, .none:
            keyField.isEnabled = true
            sourceLabel.stringValue = "Stored in your Keychain. GEMINI_API_KEY from your shell takes precedence."
        }
    }

    /// The field's value goes to the store; an empty field removes the key.
    @objc func keyChanged(_ sender: Any?) {
        let typed = keyField.stringValue.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            try AppEnvironment.shared.geminiKeyStore.write(typed.isEmpty ? nil : typed)
        } catch {
            sourceLabel.stringValue = "The Keychain refused the key: \(error)"
        }
    }

    func controlTextDidEndEditing(_ notification: Notification) { keyChanged(notification.object) }
}
```

- [ ] **Step 4: The installer and the Command Line pane (the SettingsCLI board)**

The board draws a "Bundled" card ("tap 2.1.0" over "Tap.app/Contents/Resources/tap"), an "On your PATH" card ("tap 2.0.0", "Installed by Homebrew. Tap never replaces or deletes it.", "/opt/homebrew/bin/tap"), a button "Install in ~/.local/bin…" and the hint "Asks first. ~/.local/bin comes before /opt/homebrew/bin on your PATH.". `desktop/Tap/Settings/CommandLineInstaller.swift`:

```swift
import Foundation

/// Links the bundled tap into a folder on PATH (~/.local/bin). It makes
/// the folder if needed, replaces only a link of its own, and refuses a
/// file it did not put there. A test points it at a folder of its own.
final class CommandLineInstaller {
    enum InstallError: Error, Equatable {
        case refused(String)
    }

    let linkDirectory: URL
    let bundledTap: URL

    init(linkDirectory: URL = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".local/bin"), bundledTap: URL) {
        self.linkDirectory = linkDirectory
        self.bundledTap = bundledTap
    }

    var linkURL: URL { linkDirectory.appendingPathComponent("tap") }

    /// What is at ~/.local/bin/tap now.
    func existingFile() -> CommandLineTool.ExistingFile {
        let attributes = try? FileManager.default.attributesOfItem(atPath: linkURL.path)
        guard attributes != nil else { return .none }
        let destination = try? FileManager.default.destinationOfSymbolicLink(atPath: linkURL.path)
        return CommandLineTool.isBundledLink(destination: destination) ? .bundledLink : .other
    }

    /// Whether the link is in place and points at this app's tap.
    var isInstalled: Bool {
        (try? FileManager.default.destinationOfSymbolicLink(atPath: linkURL.path)) == bundledTap.path
    }

    @discardableResult
    func install() throws -> URL {
        switch CommandLineTool.installDecision(for: existingFile()) {
        case .refuse(let reason):
            throw InstallError.refused(reason)
        case .replaceOwnLink:
            try FileManager.default.removeItem(at: linkURL)
        case .link:
            break
        }
        try FileManager.default.createDirectory(at: linkDirectory, withIntermediateDirectories: true)
        try FileManager.default.createSymbolicLink(at: linkURL, withDestinationURL: bundledTap)
        return linkURL
    }
}
```

`AppEnvironment` gains `lazy var commandLineInstaller = CommandLineInstaller(bundledTap: tapExecutableURL)`. `desktop/Tap/Settings/CommandLineSettingsViewController.swift`:

```swift
import AppKit

/// Command Line: the bundled tap, any other tap on the login shell's
/// PATH with its version, and Install, which asks first and links the
/// bundled tap into ~/.local/bin. Another tap is never replaced or deleted.
final class CommandLineSettingsViewController: NSViewController {
    let bundledLabel = NSTextField(labelWithString: "")
    let bundledPathLabel = NSTextField(labelWithString: "")
    let otherLabel = NSTextField(labelWithString: "")
    let otherPathLabel = NSTextField(labelWithString: "")
    let otherNoteLabel = NSTextField(labelWithString: "")
    let installButton = NSButton(title: "Install…", target: nil, action: nil)
    let installHint = NSTextField(wrappingLabelWithString: "")
    private(set) var otherTap: (path: String, version: String?)?
    /// Asks before linking: a sheet in production (Step 6); a test answers at once.
    var confirmInstall: (@escaping (Bool) -> Void) -> Void = { completion in completion(false) }

    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        super.init(nibName: nil, bundle: nil)
        title = "Command Line"
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        for label in [bundledPathLabel, otherPathLabel] {
            label.font = .monospacedSystemFont(ofSize: 11, weight: .regular)
            label.textColor = .secondaryLabelColor
            label.lineBreakMode = .byTruncatingMiddle
        }
        for label in [bundledLabel, otherLabel] { label.font = .systemFont(ofSize: 13, weight: .semibold) }
        otherNoteLabel.font = .systemFont(ofSize: 11)
        otherNoteLabel.textColor = .secondaryLabelColor
        installButton.bezelStyle = .rounded
        installButton.target = self
        installButton.action = #selector(installPressed(_:))
        installButton.setAccessibilityIdentifier("cli-install")
        installHint.font = .systemFont(ofSize: 11)
        installHint.textColor = .secondaryLabelColor
        let bundledStack = NSStackView(views: [bundledLabel, bundledPathLabel])
        bundledStack.orientation = .vertical
        bundledStack.alignment = .leading
        let otherStack = NSStackView(views: [otherLabel, otherNoteLabel, otherPathLabel])
        otherStack.orientation = .vertical
        otherStack.alignment = .leading
        let bundledCard = SettingsCard(title: "Bundled", rows: [("", nil, bundledStack)])
        let otherCard = SettingsCard(title: "On your PATH", rows: [("", nil, otherStack)])
        let stack = NSStackView(views: [bundledCard, otherCard, installButton, installHint])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 28, bottom: 20, right: 28)
        stack.widthAnchor.constraint(equalToConstant: 760).isActive = true
        for card in [bundledCard, otherCard] { card.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true }
        installHint.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true
        view = stack
    }

    override func viewWillAppear() {
        super.viewWillAppear()
        Task { @MainActor [weak self] in await self?.refresh() }
    }

    func refresh() async {
        let environment = AppEnvironment.shared
        let installer = environment.commandLineInstaller
        bundledLabel.stringValue = "tap \(environment.bundledTapVersion ?? (await AppEnvironment.readVersion(of: environment.tapExecutableURL)) ?? "unknown")"
        bundledPathLabel.stringValue = environment.tapExecutableURL.path
        let path = (await environment.tapEnvironment())["PATH"] ?? ""
        let others = CommandLineTool.locate(named: "tap", onPath: path, fileExists: { FileManager.default.isExecutableFile(atPath: $0) })
            .filter { !CommandLineTool.isBundledLink(destination: try? FileManager.default.destinationOfSymbolicLink(atPath: $0)) }
        if let first = others.first {
            let version = await AppEnvironment.readVersion(of: URL(fileURLWithPath: first))
            otherTap = (first, version)
            otherLabel.stringValue = version.map { "tap \($0)" } ?? "tap (version unknown)"
            otherPathLabel.stringValue = first
            otherNoteLabel.stringValue = "Tap never replaces or deletes it."
        } else {
            otherTap = nil
            otherLabel.stringValue = "No other tap is on your PATH."
            otherPathLabel.stringValue = ""
            otherNoteLabel.stringValue = ""
        }
        let directory = (installer.linkDirectory.path as NSString).abbreviatingWithTildeInPath
        switch installer.existingFile() {
        case .other:
            installButton.title = "Install in \(directory)…"
            installButton.isEnabled = false
            installHint.stringValue = "\(directory)/tap is a tap that Tap did not install. Tap never replaces or deletes it."
        case .bundledLink where installer.isInstalled:
            installButton.title = "Installed"
            installButton.isEnabled = false
            installHint.stringValue = pathHint(directory: directory, path: path)
        case .bundledLink, .none:
            installButton.title = "Install in \(directory)…"
            installButton.isEnabled = true
            installHint.stringValue = pathHint(directory: directory, path: path)
        }
    }

    private func pathHint(directory: String, path: String) -> String {
        let linkDirectory = AppEnvironment.shared.commandLineInstaller.linkDirectory.path
        switch CommandLineTool.directoryComesFirst(linkDirectory, beforeDirectoryOf: otherTap?.path, onPath: path) {
        case true?:
            guard let other = otherTap else { return "Asks first. \(directory) is on your PATH." }
            return "Asks first. \(directory) comes before \((other.path as NSString).deletingLastPathComponent) on your PATH."
        case false?:
            let other = otherTap.map { "\(($0.path as NSString).deletingLastPathComponent) comes before \(directory) on your PATH, so Terminal would still run tap \($0.version ?? "")" } ?? ""
            return other.trimmingCharacters(in: .whitespaces) + "."
        case nil:
            return "Asks first. \(directory) is not on your PATH: add it in your shell's profile, or Terminal will not find the command."
        }
    }

    @objc private func installPressed(_ sender: Any?) {
        confirmInstall { [weak self] confirmed in
            guard confirmed, let self else { return }
            do {
                try AppEnvironment.shared.commandLineInstaller.install()
            } catch {
                self.installHint.stringValue = "Install failed: \(error)"
            }
            Task { @MainActor [weak self] in await self?.refresh() }
        }
    }
}
```

In `AppDelegate`: `@objc func installCommandLineTool(_ sender: Any?) { SettingsWindowController.shared.show(pane: .commandLine) }`. In `MainMenu.tapMenu`, after Settings…: `menu.addItem(item("Install Command Line Tool…", action: #selector(AppDelegate.installCommandLineTool(_:))))`.

- [ ] **Step 5: Build, then hand the hosted tests to CI**

Run: `make -C desktop build && make -C desktop test-build 2>&1 | tail -3 && make -C desktop check-release-hooks`
Expected: all succeed. The controller's CI run confirms: `ImageGenerationSettingsTests` (1) and `CommandLineSettingsTests` (3) pass; `testGenerateAnImageWithAI` (Task 8) still passes with the key from the store.

- [ ] **Step 6: The Install confirmation: waits for the person's mockup sign-off (the controller records it in the ledger)**

The board says "Asks first" and draws no sheet. Proposed, for the mockup: a `QuestionSheet` on the Settings window in the D4 style: the title "Install the tap command?", the body "Tap links its bundled tap into ~/.local/bin/tap, so Terminal runs the same tap as the app. Nothing else on your Mac changes.", the path line with the link's path, Cancel and Install (Return is Install: the action is harmless and undoable by deleting the link). Once signed off, `CommandLineSettingsViewController.confirmInstall`'s production default becomes that sheet through `view.window?.beginSheet`; until then the button's production default declines (`completion(false)`), so the pane shows the state and the tests drive the seam, and the menu item still opens the pane.

- [ ] **Step 7: Mutations and commit**

Mutations, each a patch in `mutations-d/`, the ones that could touch a file that is not ours or leak the key first: in `CommandLineInstaller.install`, treat `.other` as `.replaceOwnLink` (`Test: TapTests/CommandLineSettingsTests/testInstallTheTapCommand`; expected: fails on "untouched"); in `install`, skip `confirmInstall` and link at once (expected: fails on `confirmations == 1`, and the declined case links); in `CommandLineSettingsViewController.refresh`, count the bundled link as another tap (`Test: .../testAnotherTapIsAlreadyInstalled`; expected: after the install the other label names the link); in `ImageGenerationSettingsViewController.keyChanged`, write the field's value even when empty (`Test: TapTests/ImageGenerationSettingsTests/testGeminiKey`; expected: fails on `store.key == nil`); in `refresh`, keep the field enabled with a shell key (expected: fails on `isEnabled`); in `ImageGenerationSettingsViewController.loadView`, use an `NSTextField` (expected: fails on `is NSSecureTextField`); in `pathHint`, drop the "comes before" case (expected: `testAnotherTapIsAlreadyInstalled` fails on the hint); in `TapTool.makeRun`, append `GEMINI_API_KEY=<value>` to the log line (expected: `testGeminiKey` fails on the log scan only if a deck is open and a tool ran: make `testGeminiKey` open `plain.md` and run `tap theme list` through the loader first, so the log has a tool line to scan, and keep this mutation).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the Gemini key in the Keychain, and Install Command Line Tool"
```

---

### Task 14: The scenario manifest, the theme UI test, and the README

**Files:**
- Modify: `desktop/scenarios.txt`, `desktop/README.md`
- Create: `desktop/TapUITests/ThemeUITests.swift`

**Interfaces:**
- Consumes: `check-scenarios.sh` (D5's, reading Swift and Go tests), `UITestCase.launch(withDeck:)`, `preview(in:)` (D2, D4), the accessibility identifiers this plan set: `theme-button`, `theme-grid`, `theme-cell-<slug>`, `export-sheet`, `settings-window`, `gemini-key`, `cli-install`, `new-deck-title`.
- Produces: the 22 `D6` rows; `ThemeUITests.testTryAThemeWithoutSavingIt`; the README's D6 paragraphs.

- [ ] **Step 1: Claim the scenarios**

Append to `desktop/scenarios.txt`:

```text
D6 | 08-creating-decks.feature | New deck
D6 | 08-creating-decks.feature | Theme picker lists tap's themes
D6 | 08-creating-decks.feature | Change the deck's theme
D6 | 08-creating-decks.feature | Try a theme without saving it
D6 | 09-images-and-components.feature | Paste an image
D6 | 09-images-and-components.feature | Generate an image with AI
D6 | 09-images-and-components.feature | Regenerate an AI image
D6 | 09-images-and-components.feature | Create a component
D6 | 09-images-and-components.feature | Open a component
D6 | 09-images-and-components.feature | Component errors
D6 | 10-export.feature | Export a PDF
D6 | 10-export.feature | First PDF export
D6 | 10-export.feature | Export a static site
D6 | 10-export.feature | Export slide images
D6 | 10-export.feature | A slide fails during export
D6 | 10-export.feature | Cancel an export
D6 | 11-settings-and-cli.feature | Settings window
D6 | 11-settings-and-cli.feature | Gemini key
D6 | 11-settings-and-cli.feature | Install the tap command
D6 | 11-settings-and-cli.feature | Another tap is already installed
D6 | 11-settings-and-cli.feature | Shared settings
D6 | 11-settings-and-cli.feature | General settings
```

Run: `make -C desktop check-scenarios`
Expected: `every claimed scenario has a test` once Step 2's UI test exists; before it, the one failure names "Try a theme without saving it".

- [ ] **Step 2: The UI test**

`desktop/TapUITests/ThemeUITests.swift`:

```swift
import XCTest

/// T in the preview is the page's own key: it cycles the theme on screen
/// and writes nothing. The toolbar's Theme item, which does write, keeps
/// its title. Runs on CI; locally it would open a window (the person's rule).
final class ThemeUITests: UITestCase {
    func testTryAThemeWithoutSavingIt() throws {
        let deck = try copyFixture("seven-slides.md")
        let application = try launch(withDeck: deck)
        let preview = self.preview(in: application)
        XCTAssertTrue(preview.waitForExistence(timeout: 30), "the preview")
        let themeButton = application.buttons["theme-button"]
        XCTAssertTrue(themeButton.waitForExistence(timeout: 10), "the toolbar's Theme item")
        Thread.sleep(forTimeInterval: 3)
        let fileBefore = try String(contentsOf: deck, encoding: .utf8)
        let titleBefore = themeButton.title

        preview.click()
        Thread.sleep(forTimeInterval: 1)
        let before = preview.screenshot()
        application.typeKey("t", modifierFlags: [])
        Thread.sleep(forTimeInterval: 2)
        let after = preview.screenshot()
        XCTAssertNotEqual(after.pngRepresentation, before.pngRepresentation, "the preview cycled themes")
        Thread.sleep(forTimeInterval: 2)
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), fileBefore, "the file does not change")
        XCTAssertEqual(themeButton.title, titleBefore, "the deck's theme did not change: only the page's did")

        // The grid opens from the item, and closing it without a pick writes nothing either.
        themeButton.click()
        XCTAssertTrue(application.otherElements["theme-grid"].waitForExistence(timeout: 10), "the theme grid popover")
        application.typeKey(.escape, modifierFlags: [])
        Thread.sleep(forTimeInterval: 1)
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), fileBefore)
    }
}
```

- [ ] **Step 3: The README**

In `desktop/README.md`, after the live code paragraph, add:

```markdown
The D6 tests run the bundled tap's own commands where they need no
network: `tap theme list`, `tap theme set`, `tap image add`, `tap component
new`, `tap build` and `tap serve --json` are real; `tap export pdf` is real
in `testExportAPDF` (CI caches the export engine's download, the same cache
the Go tests use), and scripted in the tests of the download state, the
warnings and Cancel; `tap theme show --image` and `tap image generate` are
scripted (`TapTests/Support/FakeToolScripts.swift`), since the first needs
the engine per theme and the second the Gemini API. A scripted tap stands
in for the one-shot commands only (`AppEnvironment.toolExecutableURL`);
the deck's `tap dev --app` stays real. No test touches the person's
Keychain (`MemoryGeminiKeyStore`), `~/.local/bin`
(`CommandLineInstaller(linkDirectory:)`), clipboard or defaults.

What only a person can check: the theme grid's first open on a Mac that
has never exported (the engine download under the grid, then every render
landing), a real Gemini key in Settings > Image Generation and Generate
Image on a slide of theirs (with Match theme and each aspect), Regenerate
on the result, a PDF and a website export of a real deck of theirs with
Preview in their browser, Install Command Line Tool followed by `tap
--version` in a new Terminal window, and a Homebrew tap on their PATH
shown in Settings > Command Line and left alone by Install.
```

- [ ] **Step 4: Run every local check**

Run: `make -C desktop check-scenarios && make -C desktop core-test && go test ./internal/cli ./internal/deckedit && make -C desktop build && make -C desktop test-build 2>&1 | tail -3 && make -C desktop bench-build | tail -3 && make -C desktop check-release-hooks`
Expected: `every claimed scenario has a test`, every core and Go test green, `** BUILD SUCCEEDED **`, `** TEST BUILD SUCCEEDED **` twice, `no test-only hook in the release build`. The controller's CI run confirms `ThemeUITests` green on the runner.

- [ ] **Step 5: Commit**

```bash
git add desktop/scenarios.txt desktop/README.md desktop/TapUITests/ThemeUITests.swift
git commit -m "test(desktop): claim the D6 scenarios, and try a theme without saving it"
```

---

## Final check

- [ ] Run: `go test ./internal/cli ./internal/deckedit -run 'TestNewFolder|TestServe|TestThemeShow|TestImageGenerate|TestImageRegenerate|TestProgress' -v`
  Expected: every pass, and `go vet ./...` clean.
- [ ] Run: `make -C desktop core-test`
  Expected: every `TapDesktopCore` test passes, the new `ToolRunTests`, `ToolResultsTests`, `ThemeCatalogTests`, `DeckReferencesTests`, `GeneralSettingsTests`, `GeminiKeyStoreTests` and `CommandLineToolTests` included.
- [ ] Run: `make -C desktop check-scenarios`
  Expected: `every claimed scenario has a test`, the 22 D6 rows included.
- [ ] Run: `make -C desktop build`, `make -C desktop test-build 2>&1 | tail -3`, `make -C desktop bench-build | tail -3`, `make -C desktop check-release-hooks`
  Expected: `** BUILD SUCCEEDED **`, `** TEST BUILD SUCCEEDED **` twice, `no test-only hook in the release build`, nothing launched.
- [ ] The controller pushes and reads CI's Desktop Tests, Desktop UI Tests, Desktop Benchmarks and Go Tests jobs: every hosted test green, the D2 to D5 tests included, `ThemeUITests` green on the runner, the typing benchmark no worse than before. The mutation branches `mutations/d6-batch-a` to `d` hold only the `mutations-<batch>/` patches, each with a killing test and a diff under `desktop/Tap`, `desktop/TapDesktopCore/Sources` or `internal/` (never a test, a fixture or a fake); the `survivors-<batch>/` patches are read by the review, not run. A mutation that survives where its task said it would be killed is a review finding.
- [ ] Run: `grep -rn "GEMINI_API_KEY" desktop/Tap`
  Expected: only `AppEnvironment.tapEnvironment()` and `geminiKeySource()` (through `GeminiKeySource`), and the two label strings in `ImageGenerationSettingsViewController`. No log line, no `print`, no accessibility value.
- [ ] Run: `grep -rn "geminiKeyStore.read\|keyStore.read" desktop/Tap`
  Expected: only `ImageGenerationSettingsViewController.refresh` and `AppEnvironment.geminiKeySource`; `tapEnvironment` reads through `GeminiKeySource.apply`.
- [ ] Run: `grep -rn "evaluateJavaScript\|callAsyncJavaScript" desktop/Tap`
  Expected: only D4's `WKWebView+BoundedEvaluation.swift`; this plan adds none.
- [ ] Run: `grep -rn "runModal\|NSAlert" desktop/Tap`
  Expected: no output.
- [ ] Run: `grep -rn "NSApp.activate\|activate(ignoringOtherApps" desktop/Tap` and `grep -rn "orderFrontRegardless\|makeKeyAndOrderFront\|makeKey\b" desktop/Tap`
  Expected: no output for the first; exactly D4's list for the second, unchanged.
- [ ] Run: `grep -rn "Process()" desktop/Tap`
  Expected: only `TapProcess.swift` (in Core), `LayoutCatalogLoader.run`, `DeckSchemaLoader.run`, `AppEnvironment.readVersion`; every tap subcommand of this plan goes through `ToolRun` and `TapTool`.
- [ ] Run: `grep -rn "textStorage?.replaceCharacters\|textStorage!.replaceCharacters" desktop/Tap` and `grep -rn "updateChangeCount" desktop/Tap`
  Expected: only D2's own lines in `EditorTextView.swift`; only `DeckSessionController.refreshEditedState`.
- [ ] Run: `grep -rnE "completion\?\([^)]" desktop/Tap` and `grep -rn "unowned" desktop/Tap desktop/TapDesktopCore/Sources`
  Expected: no output for both.
- [ ] Run: `grep -rn "$(printf '\342\200\224')" desktop/ internal/cli internal/deckedit docs/superpowers/plans/2026-09-26-desktop-creating-export-settings.md --include=*.swift --include=*.go --include=*.md --include=*.sh --include=*.txt`
  Expected: no output.
- [ ] Every part of the D6 outline maps to a task:

  | D6 outline item | Task |
  |---|---|
  | New Deck sheet with the theme grid from `tap theme show --image` | 1, 2, 4, 6, 7 |
  | The theme picker through `tap theme set` | 6 (the toolbar), 7 (the Deck tab, after sign-off) |
  | Images and components through `tap image` and `tap component new` | 2, 8, 9 |
  | File > Export through `tap export` and `tap build` with `--progress json` | 3, 10, 11 |
  | The Settings window with the Keychain | 5, 12, 13 |
  | Install tap Command | 5, 13 |
  | Every scenario of 08 to 11, and 12's Slide menu items | 9, 14 |

## Pre-flight: conflicts found, rulings and what each costs if wrong

1. **`tap new` writes a file; the spec and the sheet want a folder with `images/`.** The folder's name is tap's slug of the title (`tui.FilenameFromTitle`), a rule Swift must not copy. Ruling: `tap new --folder <location>` (Task 1), tap making the folder, the deck and `images/`; the sheet's hint names no slug (open question 1). Cost if wrong: one Go flag and a sentence of copy.
2. **`tap serve` prints a banner for a person.** Ruling: `tap serve --json` prints one ready line and logs no requests (Task 1); the app runs it with `--port 0`. The bind stays `0.0.0.0` as the CLI does today (open question 5). Cost if wrong: one flag.
3. **The first theme render downloads Chromium silently.** Ruling: `tap theme show --image --progress json` (Task 2), the grid showing the download under itself. Cost if wrong: a flag nobody uses.
4. **The approved GenerateImage board draws Aspect and Match theme, which tap lacks.** Ruling: `--aspect` and `--match-theme` on `generate` and `regenerate` (Task 2), tap prepending its own `--prompt` brief while the `ai-prompt` comment keeps the person's words. Cost if wrong: two flags, or the sheet loses two controls (a board change).
5. **The theme grid over the real tap on CI would render 21 themes in Chromium per test.** Ruling: the grid's hosted tests script `tap theme show` (the render is tap's, proven in Go) and keep `tap theme list` real; one real export (`testExportAPDF`) carries the real engine on CI with a cache step. Cost if wrong: minutes of CI per run.
6. **The SettingsImage board shows the key's last four characters.** The secrets rule says an expanded secret is never shown outside its own secure field; a suffix is part of the secret. Ruling: bullets only. Cost if wrong: a board change the person makes.
7. **The SettingsImage board's Model row.** tap names its Gemini model in no command or flag, so the app has nothing to show that it did not hard-code. Ruling: not built (open question 8). Cost if wrong: a `tap image models` command later, or a static label.
8. **Where the sheets go.** D5 put every question sheet through `showQuestionSheet` with a queue; this plan's form sheets are not questions from tap. Ruling: `showFormSheet` refuses while any sheet is up (a beep), and `showNextDeckQuestionIfIdle` already waits for `attachedSheet == nil`, so a tap question arriving mid-export waits. Play is off while an export runs. Cost if wrong: one guard.
9. **A PDF export with warnings.** The scenario says "the export finishes and lists slide 2 as a warning"; the ExportPDF board draws no done state. Ruling: a clean PDF closes the sheet and reveals the file (the scenario's "then reveals the file in Finder"); one with warnings reveals the file and keeps the sheet up with the list, in the ExportWebsite board's done layout, until the person clicks Done; the drawing waits for a mockup. Cost if wrong: the drawing.
10. **`tap export images --all` exits 1 with `broken_slides` after writing the other files.** Ruling: for images that failure is a done state with a warning and the files that landed, since tap wrote them; every other failure is a failure. Cost if wrong: one branch.
11. **Where pasted image data goes.** tap copies a file and keeps its name; a screenshot has none. Ruling: `pasted-image.png` in a temporary folder, so tap's own `-2` rule names the second one. Cost if wrong: a name.
12. **The Regenerate entry point.** The scenario says "when I choose Regenerate on it" and no board draws it. Ruling: a context menu item per AI image of the box's slide, found with the same pattern tap reads (`AIImageReference`, for locating only), waiting for a mockup; the run path is built and tested regardless. Cost if wrong: where the item lives.
13. **`--match-theme` is on by default in the sheet, as the board draws it.** Cost if wrong: a switch's default.
14. **The Deck tab's theme row.** D5 built a popup and said D6 replaces the control. Ruling: a button opening the same popover, after sign-off; the row keeps D5's label and edit path otherwise. Cost if wrong: nothing before sign-off.
15. **The Install confirmation and `/usr/local/bin`.** The feature file names `~/.local/bin` only; the design spec adds `/usr/local/bin` after a password prompt. Ruling: `~/.local/bin` in D6, the confirmation a `QuestionSheet` after sign-off; `/usr/local/bin` needs a privileged helper and is left out (open question 6). "Never replaces or deletes" is a pure function (`installDecision`) with the refusing case first in the mutations. Cost if wrong: a helper tool later.
16. **The other tap on PATH.** Ruling: the first executable `tap` on the login shell's PATH that is not our own link, its version from `tap --version` with the loader's deadline; the hint says which comes first. Cost if wrong: copy.
17. **The editor's typography as app-wide state.** D2's fonts were `static let`s used in paragraph styles and drawing math; per-instance fonts would touch every draw call. Ruling: `EditorTypography.current`, refreshed from the settings, every editor restyling on the notification. Cost if wrong: a benchmark number.
18. **Settings in AppKit, not SwiftUI.** The spec allows SwiftUI for sheets and settings; every D2 to D5 sheet is AppKit and the hosted tests read AppKit controls (`NSSwitch`, `NSPopUpButton`). Ruling: AppKit, one `NSTabViewController` in the toolbar style the boards draw. Cost if wrong: a rewrite of four panes.
19. **The consent answer in Settings.** The scenario says the consent and the approvals share `settings.yaml` through tap; no tap command reads or sets `present.record`. Ruling: the Live Code pane shows the approvals; the consent is not shown (open question 9); `testSharedSettings` proves the shared file through `tap approval list`. Cost if wrong: a `tap present consent` command later.

## Open questions

Product decisions this plan makes that the spec leaves open. Each line is the default the plan implements and what it costs if the person wants it otherwise.

1. **The New Deck sheet's folder hint.** Default: "Creates a folder named after the title, with the deck and images/", since the slug is tap's. Alternative: a `tap new --folder --dry-run --json` that prints the paths tap would make, so the hint names the folder as the board does (about ten Go lines). Cost if wrong: one flag and the hint's text.
2. **`--match-theme` and `--aspect` as tap flags** (pre-flight 4). Default: tap flags, on by default in the sheet as the board draws. Alternative: drop the two controls from the sheet (a board change). Cost if wrong: two flags.
3. **The real PDF export on CI** (pre-flight 5). Default: `testExportAPDF` runs the real engine with a cache step, so a broken `tap export pdf` under the app is seen. Alternative: a scripted fake and tap's Go tests alone. Cost if wrong: minutes of CI and a cache step.
4. **A clean PDF export closes its sheet** (pre-flight 9); a website or images export stays for Show in Finder and Preview. Alternative: every export stays up with a Done button. Cost if wrong: one branch.
5. **`tap serve` keeps binding `0.0.0.0`** (pre-flight 2), as the CLI does today, so a phone on the LAN can see the preview. Alternative: `--json` binds loopback, or a `--lan` flag as `tap dev` has. Cost if wrong: one line in `listenOnAvailablePort`'s caller.
6. **Install targets `~/.local/bin` only** (pre-flight 15). Alternative: `/usr/local/bin` through an authorization prompt. Cost if wrong: a privileged helper and its signing (D7).
7. **The Install confirmation is a `QuestionSheet`** (Task 13 Step 6), after sign-off. Alternative: no confirmation (the spec says "after you confirm", so this is not recommended). Cost if wrong: copy.
8. **The Model row is not built** (pre-flight 7). Alternative: a `tap image models --json` command, or a static label with the model the Gemini client uses. Cost if wrong: a command or a label.
9. **The recording consent is not shown in Settings** (pre-flight 19). Alternative: a `tap present consent [--set yes|no] --json` command and a row in Live Code or General. Cost if wrong: a command and a row.
10. **The theme renders are kept only for the app's life**, tap's cache making the next launch instant. Alternative: the app caches the PNGs itself under Application Support. Cost if wrong: a second cache of the same files.
11. **Line spacing factors** (tight 1.38, normal 1.62, roomy 1.92 of the font size, normal at 13 pt being D2's 21). Cost if wrong: three numbers.
12. **The pasted image's name** `pasted-image.png` (pre-flight 11). Cost if wrong: a name.
13. **Regenerate lives in the box's context menu** (pre-flight 12), after sign-off. Alternative: a "Regenerate" affordance in the editor's gutter next to the `ai-prompt` line. Cost if wrong: where the item lives.
14. **The States board's "Live code is off ... Review Code…" bar** (D5's open question 16) is not in `08` to `12` and not built here; it needs a tap stdin command to reopen a declined question. Default: its own pull request after D6. Cost if wrong: a bar kind and a tap command.
15. **The person's runs.** `make -C desktop uitest` (`ThemeUITests` on the runner's screen) and the README's manual pass are CI's and the person's, never an agent's local run.

## What this plan found missing in the spec and in tap

- `tap new` writes one file and has no notion of a deck folder; the design spec says it "creates a folder with the deck and `images/`" (Task 1 adds `--folder`).
- `tap serve` has no machine-readable ready line, and binds every interface (Task 1 adds `--json`; the bind is open question 5).
- `tap theme show --image` reports nothing while it downloads Chromium the first time (Task 2 adds `--progress json`).
- `tap image generate` has no aspect and no theme style, though `gemini.Client.GenerateImageWithAspectRatio` exists and the approved board draws both (Task 2).
- No tap command names the Gemini model (the board's Model row, open question 8).
- No tap command reads or sets the recording consent the scenario says the app shares with the CLI (open question 9).
- The feature file's "the app creates a folder with the deck and an images/ folder" assigns the folder to the app, and the design spec to tap; the plan takes the spec's reading, since the slug is tap's.
- `10-export.feature`'s "lists slide 2 as a warning" has no drawing on the approved canvas (pre-flight 9, a mockup).
- `09-images-and-components.feature`'s "Regenerate on it" names no place in the app (pre-flight 12, a mockup).
- `11-settings-and-cli.feature`'s "after I confirm" has no drawing (pre-flight 15, a mockup).
- The SettingsImage board's key suffix conflicts with the secrets rule (pre-flight 6).
- `tap export images --all` reports a broken slide as a failure after writing the other files, unlike `tap export pdf`, which reports it as a warning and exits 0 (pre-flight 10; a tap consistency item for later).

## Steps that wait for a mockup

Each step below names its screen, builds nothing until the controller records the person's sign-off in the ledger, and follows the signed-off board exactly once it does. Every other step follows the approved "Tap Desktop Mockups" canvas (NewDeck, ThemePicker, GenerateImage, NewComponent, ExportPDF, ExportWebsite, SettingsGeneral, SettingsLiveCode, SettingsImage, SettingsCLI, MenusFile, MenusSlide, Welcome). The plan makes no mockup; the person or a later session does.

| Step | Screen needing a mockup | What it builds |
|---|---|---|
| Task 7, Step 5 | The Deck tab's theme row opening the theme grid | A "Choose…" button in the Deck tab's Theme row that opens `ThemePopoverController` |
| Task 8, Step 8 | The Regenerate entry point | "Regenerate Image…" items in a box's context menu, one per AI image of the slide |
| Task 10, Step 7 | An export finished with warnings (the PDF's broken slides, the images export's partial result) | The warnings list and the Done button in the sheet's done state |
| Task 13, Step 6 | The Install confirmation ("Asks first") | A `QuestionSheet` on the Settings window: title, body, the link's path, Cancel and Install |

Two deviations from approved boards, for the person to confirm at the same time: the Image Generation pane shows bullets only and no Model row (pre-flight 6 and 7), and the New Deck sheet's hint names no slug (open question 1).

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-26-desktop-creating-export-settings.md`. Execute with superpowers:subagent-driven-development, one fresh subagent per task, as the roadmap requires, on `feat/desktop-creating-export-settings` cut from `main` after D5's pull request 43 has merged, in a worktree at `/Users/codemonkey/projects/tap-d6`. Batches for the batch-and-trust-CI pace: A = Tasks 1 to 4 (Go and the core package, no Xcode project needed: `tap new --folder`, `tap serve --json`, the theme and image flags, `ToolRun`, the decoders); B = 5 to 8 (the stores and PATH logic, the theme grid and toolbar item, the New Deck sheet, images); C = 9 to 11 (components, Export PDF, Export Website and Images); D = 12 to 14 (Settings, the manifest and the UI test). One combined review per batch, the mutations of its tasks on `mutations/d6-batch-<letter>`. Every task's steps build locally and hand the hosted runs to the controller's CI; every task's review runs the mutations its last step lists, the ones that could touch a file that is not ours, leak the key, lose an edit or run tap on an unsaved deck first. The four steps under "Steps that wait for a mockup" are built only once the ledger holds the person's sign-off; every other step builds first.

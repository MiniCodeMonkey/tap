# Tap Desktop live code, the Deck tab and fix-its (D5): implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A deck with live code asks once, in a native sheet on the deck window whose default button is Don't Allow, and nothing runs until the person allows it; the page's Run buttons then work through tap's own `{slide, block}` reference; a block whose driver the deck does not declare shows tap's message on its box with an "Allow shell in This Deck" fix-it that edits the frontmatter as one undo step; and the Deck tab is a form over the frontmatter built from `tap deck schema --json`, editing it through the editor's frontmatter path, one undo step per change.

**Architecture:** tap owns every rule (P2, merged): it asks the live code `approval` question as a stdout event after its ready line, in `tap dev --app` and `tap present --app` alike, stores a yes in `settings.yaml`, fixes its live code policy for the life of the process, and refuses `/api/execute` for anything but a declared, approved block of the loaded deck. The app answers questions and edits text. `TapDesktopCore` learns the approval payload (`ApprovalDriver`, `ApprovalBlock`), the slide list's per-block `problem` (a one-field tap change in this plan), a `Frontmatter` type that reads the deck's frontmatter as lines and produces one-line edits for it, `DeckSchema` for `tap deck schema --json`, and `TapSession.restart(reason:)`. In the app, `DeckSessionController` gains a question queue for its `tap dev` session (D4's `PresentationController` already has one for talks), `DeckWindowController` shows both through the one `showQuestionSheet` path as an `ApprovalSheet`, the editor draws a block's problem on its box with a fix-it pill and a context menu item, the fix-it edits the frontmatter through `EditorTextView.replaceText` and saves, and a save or disk load that adds a declared driver restarts `tap dev`, since only a fresh tap start asks about a new driver. The Deck tab is `DeckFormViewController`, an AppKit form generated from the schema, that reads the buffer and writes each change as one `replaceText`.

**Tech Stack:** Swift 6.3 compiler in Swift 5 language mode, AppKit (`NSWindow.beginSheet`, `NSStackView`, `NSPopUpButton`, `NSButton`, `NSTextField`, `NSScrollView`), the TextKit 2 editor of D2, XCTest and XCUITest, XcodeGen, Go 1.25 for the one tap change and two Go tests, the bundled `tap` (`tap dev --app`, `tap present --app`, `tap deck schema --json`, `tap approval list --json`, `tap approval revoke`).

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-design.md` (milestone 5; the sections "The protocol between the app and tap", "Editor", "Preview and Deck pane", "Live code approval", "Security summary", "Menus and accessibility" and "Testing"), `docs/superpowers/specs/2026-09-22-tap-desktop-prerequisites-design.md` parts 2, 3.2 and 6 as checked against `internal/` on `main` (see "P2, P3 and P6 as built" below; the code wins), the D5 outline and the contracts in `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md`, and the feature files in `docs/superpowers/specs/tap-desktop-features/` (`06-live-code-and-trust.feature` whole, plus "Deck settings live in the inspector" from `02-slide-structure.feature`; `04`, `11`, `12` and `13` have no D5 scenario: `11-settings-and-cli.feature`'s Live Code tab is D6's Settings window). The mockups are the "Tap Desktop Mockups" canvas (Approval, ApprovalNewDriver, DeckSettings, DriverError). Where the mockups and the spec differ, the spec wins.

**Branch:** `feat/desktop-live-code`, branched from `main` after D4 (`feat/desktop-presenting`) has merged, in a worktree at `/Users/codemonkey/projects/tap-d5`. If D4 has not merged when this plan starts, branch from `feat/desktop-presenting` at 88692c5 or later and rebase onto `main` once it has. One pull request. The starting code is D4's `desktop/`: `TapDesktopCore` (`TapProtocol`, `TapSession`, `TapClient`, `SlideDocument`, `TextDiff`, `BoxHeader`, `LayoutCatalog`), `DeckSessionController`, `DeckWindowController`, `PresentationController`, `QuestionSheet`, `InspectorViewController`, `EditorTextView`, `EditorViewController`, `DocumentBarView`, `MainMenu`, `AppEnvironment`, `LayoutCatalogLoader`, `HostedTestCase`, `PresentingTestCase`, `Fixtures`, `FakeTapScripts`, `TapSlideList`, `UITestCase`, `scenarios.txt`, `check-scenarios.sh`, `run-mutations.sh`.

**Prerequisites on `main`, checked 2026-09-25 against b5054e7 (`internal/cli`, `internal/server`, `internal/slidelist`, `internal/config`, `internal/usersettings`, `frontend/src`):** P2 whole (`approval.go`, `approval_command.go`, `live_code_warnings.go`, `config/drivers.go`, `server/api.go`); P3's `tap deck schema --json` (`deck_schema.go`, `config/schema.go`); P6's `approval` question in app mode (`app_questions.go`, `dev.go`). No tap pull request is a dependency. The one tap change this plan needs, the slide list's per-block `problem`, is Task 1 of this plan, on this branch.

## P2, P3 and P6 as built, checked against `main` (the code wins)

| Topic | The documents say | The code does |
|---|---|---|
| Where the question is asked | "In `--app` mode, the question is an event and the answer arrives over stdin" (P2 2.4); the D4 plan expected it only from `tap present --app` | Both `tap dev --app` and `tap present --app` ask it: `runDevServer`'s app branch (`dev.go`, the `Startup` closure) runs `liveCodeApproval` with `appApprovalAsker` and `Interactive: true` after the ready line, after the recording consent in present mode, then `srv.SetLiveCodePolicy(policy)` and `hub.BroadcastReload()`. So the deck's own `tap dev` asks at every open of an unapproved deck, within milliseconds of ready, and the page reloads once after the answer. An unanswered question leaves live code off and tap running normally: today (D4) every hosted test that opens a fixture with drivers gets the question and never answers it. |
| The question's payload | `question` has `id`, `kind`, `payload` (P6) | `approvalRequest` (`approval.go`): `{"deck": "<resolved absolute path>", "drivers": [{"name", "command" (custom drivers only, the expanded command line), "slides": [n...], "blocks": n}], "approvedBefore": [names] (omitted when empty), "blocks": [{"driver", "code", "slide", "block"}]}`. `drivers` holds only the drivers a yes would approve: every declared one the first time, only the new ones when the deck was approved before, in `Config.DeclaredDrivers()` order. `blocks` holds the blocks of those drivers, in slide order, and never a block whose driver is undeclared (`runnableBlocks` skips a block with a `Problem`). D4's `QuestionPayload` decodes only `deck`; Task 2 adds the rest. |
| When tap asks | "when stdin is a TTY and the deck needs approval" | In app mode `Interactive` is always true. A deck needs approval when at least one live block uses a declared driver (`runnableBlocks` non-empty). A deck with live blocks and no declared drivers, or no live blocks, asks nothing. A declared driver with no blocks still appears in the request ("no blocks yet" in the CLI). |
| The deck's key | "keyed by the deck's absolute path" | `usersettings.ResolveDeck`: absolute, symlinks resolved, on-disk case restored. A hosted test's copy under `FileManager.default.temporaryDirectory` (`/var/folders/...`) is stored as `/private/var/folders/...`. Tests compare with `realpath(3)`. A deck that cannot be resolved fails closed with no question. |
| The policy after a reload | not stated | Fixed for the process: "A driver added by a reload is not in it, so its blocks show 'Not approved' until the next start asks" (`dev.go`). The app restarts `tap dev` when a save or a disk load adds a declared driver (Task 10); nothing else re-asks. Editing a block's code never asks again, as the spec says, because the policy is by driver name. |
| The undeclared driver message | "This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter" (P2 2.3); the feature file says "Add sqlite under drivers in the deck settings to run this block" | `config.UndeclaredDriverMessage`: with a `drivers` map, `This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter.`; with none, `This deck does not declare the sqlite driver. Add this to the frontmatter:\n\ndrivers:\n  sqlite: {}` (every used driver listed, sorted). The page shows it in the block (`problem` on the transformed block); `tap dev` prints `warning: <file>:<line>: <message>` on stderr at startup (`undeclaredDriverWarnings`), which is the Tap Log. The app shows tap's words, never the feature file's. |
| The problem in the slide list | 3.3 lists `block`, `driver`, `language`, `live`, `line` | `slidelist.CodeBlock` has no `problem`; the transformer's `TransformedCodeBlock.Problem` never reaches `PUT /api/app/source`. Task 1 adds `problem` (`omitempty`) to the slide list, copied from the transformed block, so the editor can mark the box. |
| `/api/execute` | accepts `{"slide", "block"}`; a body with `code` gets 400; an undeclared driver and an unapproved deck get their own codes | Also needs `"revision"` equal to the loaded deck's, or 409 `stale_revision` (`api.go`); a `code` key gets 400 with `codeInBodyMessage`; a block with a `Problem` gets 422 with the message; a driver the policy does not allow gets 403 `notApprovedMessage`; an unknown slide or block 404. The route is behind the app token and the same-origin JSON guard; the page holds the token as its cookie from the launch code, so its own `fetch('/api/execute')` passes. |
| What the page shows | "Run buttons show Not approved" | `LiveCodeBlock.tsx`: an enabled `button.run-button` ("Run") for a driver in `/api/presentation`'s `liveCode.drivers`; a disabled `button.run-button.not-approved` ("Not approved") for a declared driver the policy does not allow; a `pre.live-code-problem` with tap's message and no button for an undeclared driver; the result in `.result-container .result-content`, a `table.result-table` for row data. |
| `tap approval list --json` | prints the approvals | `{"ok": true, "approvals": [{"deck", "drivers": [...], "approvedAt"}]}`; `tap approval revoke <deck>` takes a file or a deck folder and resolves it the same way. |
| `tap deck schema --json` | "keys, types, and allowed values" | `{"ok": true, "keys": [{"name", "type", "default", "values" (omitted when none), "description", "keys" (nested, omitted when none)}]}`. Types are `string`, `boolean`, `integer`, `list`, `object` (fixed nested keys: `themeColors`, `recording`), `map` (named entries, each with the nested keys: `drivers`, and `drivers.<name>.connections`). `default` is `null`, a string, a boolean or an integer. The theme's values are the theme slugs. |
| `tap new` | "records an approval for the deck it creates" | `approveNewDeck` approves the declared drivers of the deck it wrote. Whether its starter has live code is `tap new`'s own business; the test asserts the implication only. |
| The startup order in present mode | consent, then approval (D4's table) | Unchanged. D4 declined the approval with a log line; Task 8 replaces that `default` case. The talk's windows wait for every startup question, so the sheet is never covered (D4, `showWindowsIfReady`). |

## Global Constraints

- Everything in D2's, D3's and D4's Global Constraints still holds: macOS 14 or later, AppKit core, ad-hoc signing, the bundled `tap`, P6's protocol exactly as built, spelled-out identifiers, present-tense comments with no ticket references, no em dashes anywhere (`--`, a comma or a new sentence instead), `make frontend` before the first Xcode build, never modify the prototype repository, every build through `make`.
- **THE PERSON'S RULE (2026-09-25): nothing runs locally that opens windows on their screen.** An implementer builds (`make -C desktop project`, `make -C desktop build`, `make -C desktop test-build`) and runs `make -C desktop core-test` (`swift test` in `TapDesktopCore`) and `go test ./internal/...`. Hosted tests, UI tests and benchmarks run on CI: every "Run" step below that names a hosted test says what the controller's CI run confirms, never `make -C desktop test ONLY=...`. Mutations are patch files for the mutation runner (`desktop/scripts/run-mutations.sh` on a branch `mutations/<name>`): each file's first line is `Test: TapTests/<Class>/<test>`, the rest a `git diff`. The implementer writes the patches into `.superpowers/sdd/<plan>/mutations-<batch>/NN-<name>.patch` and lists them in the report; the controller pushes them. A core or Go mutation is applied and run locally instead (`make -C desktop core-test`, `go test`), then reverted exactly.
- **Every wait in a test is bounded.** Every hosted wait goes through `waitUntil(timeout:)`, `waitForPreview`, `waitForBoxes` or a `pageValue` read with its own limit; no bare `await` on a page, a cookie store, a document open or a process. A test that needs longer than the Makefile's 300 s allowance does not exist in this plan.
- **Sheets, never modal alerts.** The approval question, from `tap dev` and from `tap present`, is `ApprovalSheet` on the deck window through D4's `showQuestionSheet`, never `NSAlert`, never app-modal, never inside the slide page. Its Return key is Don't Allow (`returnAnswer: .decline`), Escape is Don't Allow, and Allow has no key at all.
- **No production code steals focus beyond D4's list.** This plan adds no focus move: the deck's own approval sheet goes through `DeckWindowController.showQuestionSheet`, whose `makeKeyAndOrderFront` is entry (4) of D4's list. The final check's grep expects D4's list unchanged.
- **The app never injects script into a page.** No `evaluateJavaScript` or `callAsyncJavaScript` in `desktop/Tap` beyond D2's two (`PreviewViewController.pageText` and `pageValue`). This plan's page reads and the Run click live in `desktop/TapTests/Support/PreviewViewController+LiveCode.swift`, a `TapTests` extension that never ships. The app learns what the page shows only through the `tapReady` handler and tap's answers.
- **The edited flag is derived from content.** `refreshEditedState` stays the only caller of `updateChangeCount`. Every frontmatter edit (the fix-it, the Deck tab) goes through `EditorTextView.replaceText(in:with:actionName:)`, which fires `didChangeText` and so `editorTextDidChange`.
- **The Deck tab and the fix-it edit the frontmatter through the frontmatter clamp, never the text storage.** `replaceText` is the one entry: it sets `isApplyingProgrammaticEdit` so `shouldChangeText` lets an edit inside the hidden range through, and makes it one undo step. No `textStorage?.replaceCharacters` and no `insertText` for the frontmatter anywhere in this plan.
- **Only a declared, approved block of the deck runs, and only when a person clicks Run.** The app sends `{"type":"answer","value":true}` only from the Allow button's completion; nothing else in the app answers `true` to an approval. The app never calls `/api/execute` in production code. Every task lists its mutations with the ones that could run code the person did not approve, or lose an edit, first.
- `weak self` in every closure that outlives a call, no `unowned`. No work with side effects inside `completion?(...)`; every completion in this plan is non-optional or the work sits outside the call.
- **XCTest rules.** No `await` inside an `XCTAssert` autoclosure: hoist into a `let`. No test depends on a key window: a sheet's buttons are pressed with `performClick(nil)`, Return with `contentView?.performKeyEquivalent(with:)` on a synthesized event, Escape with the sheet's own `keyDown(with:)`; a window check reads `attachedSheet`, `isVisible`, `sheetParent`. Every test that must see the approval question sets `approvesLiveCodeOnOpen = false` before `openDeck`; every other test opens its fixture pre-approved (Task 6), so no D2, D3 or D4 test changes its behaviour.
- **State captured before an async step is rechecked after it.** A question's `id` is valid for one process: `DeckSessionController.questionGeneration` rises on every leave from `.running`, the sheet's completion compares it, and `answer(id:value:)` drops an id no pending question has, so an Allow meant for a tap that has since restarted never reaches the new one (whose ids start at `q1` again).
- Every scenario this plan claims has a test named `test` plus the scenario name in UpperCamelCase as `check-scenarios.sh` builds it (`tr -c '[:alnum:]' ' '` then capitalize each word), so "Don't allow" is `testDonTAllow`. The two CLI-only scenarios are Go tests named the same way with `Test` (Task 1 teaches the check to read `internal/**/*_test.go`). The claims go into `desktop/scenarios.txt` as `D5 | <file> | <scenario>` rows, and `make -C desktop check-scenarios` must pass.
- The fixtures for live code tests are new, under `desktop/TapTests/Fixtures/`: `live-code.md` (shell and sqlite declared; two shell blocks and one sqlite block), `undeclared-driver.md` (sqlite declared; a shell block on slide 6), `no-drivers.md` (no `drivers` key; a sqlite block on slide 4), `custom-driver.md` (sqlite and a custom `fortune` driver running `/bin/cat`; an undeclared shell block on slide 4). D3's `ops.md` has no live code and asks nothing. Every sqlite block runs on sqlite's in-memory default (`internal/driver/sqlite.go`), so no database file is needed.

## Review Focus

Five conditions the spec implies that no scenario names, most likely to bite first. Each has its test pinned to the task that owns the code.

1. **tap restarts while the approval sheet is up.** tap's question ids start at `q1` in every process; the old sheet's Allow must never answer the new process's `q1`. The sheet must go with the process that asked, and the new process's question must get a fresh sheet. Task 10, `testATapRestartRenewsTheApprovalQuestion`; the generation guard in Task 6.
2. **The deck window closes with the sheet up.** Nothing may be stored, tap must exit with its stdin, and the next open must ask again. Task 6, `testClosingTheDeckWithTheSheetUpAsksAgainNextTime`.
3. **A frontmatter this plan's parser did not expect: Windows line endings, `drivers: {shell: {}}` in flow style, comments and blank lines inside a block, a tab.** An edit must keep the file's line endings and never corrupt a line it did not mean to touch, and tap must still parse the result. Task 3 and Task 4 core tests (`testKeepsCarriageReturnLineEndings`, `testAFlowMapGainsAPair`, `testCommentsAndBlankLinesStayWhereTheyAre`); Task 9's fix-it test round-trips the result through tap (`errors == []`).
4. **A Deck tab refresh while the person is typing in one of its fields.** Every text change refreshes the form; the field being edited must keep what was typed. Task 11, `testARefreshNeverClobbersTheFieldBeingEdited`.
5. **Play while the deck's approval sheet is up.** A second question sheet would queue behind the first on the same window and the two slots would clobber each other; the person has one question to answer first. Task 8, `testPlayWaitsForTheDecksApprovalAnswer`.

## Scenarios this plan claims

| Feature file | Scenario | Test | Task |
|---|---|---|---|
| 06-live-code-and-trust | A deck without live code | `testADeckWithoutLiveCode` | 6 |
| 06-live-code-and-trust | First open of a deck with live code, in the app | `testFirstOpenOfADeckWithLiveCodeInTheApp` | 6 |
| 06-live-code-and-trust | Allow | `testAllow` | 6 |
| 06-live-code-and-trust | Don't allow | `testDonTAllow` | 6 |
| 06-live-code-and-trust | First open of a deck with live code, in the CLI | `TestFirstOpenOfADeckWithLiveCodeInTheCLI` (Go) | 1 |
| 06-live-code-and-trust | Run a block | `testRunABlock` | 7 |
| 06-live-code-and-trust | A page cannot run code the deck does not show | `testAPageCannotRunCodeTheDeckDoesNotShow` | 7 |
| 06-live-code-and-trust | Approve or revoke later | `testApproveOrRevokeLater` | 7 |
| 06-live-code-and-trust | A moved deck | `testAMovedDeck` | 7 |
| 06-live-code-and-trust | Non-interactive runs | `TestNonInteractiveRuns` (Go) | 1 |
| 06-live-code-and-trust | A deck declares its drivers | `testADeckDeclaresItsDrivers` | 7 |
| 06-live-code-and-trust | A block uses an undeclared driver | `testABlockUsesAnUndeclaredDriver` | 9 |
| 06-live-code-and-trust | A deck with live code must list its drivers | `testADeckWithLiveCodeMustListItsDrivers` | 9 |
| 06-live-code-and-trust | A new driver asks again | `testANewDriverAsksAgain` | 10 |
| 06-live-code-and-trust | Secrets in driver settings | `testSecretsInDriverSettings` | 7 (the environment), 12 (the hint) |
| 06-live-code-and-trust | The safe button is the default | `testTheSafeButtonIsTheDefault` | 5 |
| 02-slide-structure | Deck settings live in the inspector | `testDeckSettingsLiveInTheInspector` | 11 |

17 scenarios. Every scenario in `06-live-code-and-trust.feature` is claimed. "Approve or revoke later" is claimed through `tap approval list` and `tap approval revoke`, the scenario's "or"; D6's Settings > Live Code extends the same test. Left for later milestones: `11-settings-and-cli.feature` whole (D6).

## The live code flow, end to end

| Step | What happens | Where |
|---|---|---|
| Open | `tap dev --app` starts, prints ready, and if the deck has a live block on a declared driver the deck is not approved for, emits `question` (`approval`) within milliseconds. `DeckSessionController.handle` queues it and `DeckWindowController.presentDeckQuestion` puts `ApprovalSheet` on the deck window through `showQuestionSheet(source: .deck)`. The preview loads behind it and shows "Not approved" buttons. | Task 6 |
| Allow | The sheet's Allow sends `{"type":"answer","id":"q1","value":true}`. tap stores the approval under the deck's resolved path, sets its policy, and broadcasts `reload`; the page reloads with Run buttons. A Run click sends `{slide, block, revision}` and the block's output appears in the page. | Task 6, Task 7 |
| Don't Allow | The answer is `false`; tap stores nothing, live code stays off, the deck previews and presents normally, and the next open asks again. | Task 6 |
| Play | `tap present --app` asks the same question after the consent when the deck is still unapproved; the sheet is the same, from `presentQuestion`'s `approval` case, before any talk window shows. A question arriving mid-talk uses D4's step-aside path. | Task 8 |
| A block with an undeclared driver | tap's answer to `PUT /api/app/source` now carries the block's `problem`; the editor draws it on the box as an error line with a fix-it pill "Allow shell in This Deck", also in the box's context menu and the Slide menu. The fix-it adds `shell: {}` under `drivers` through `Frontmatter.addingDriver` and `replaceText` (one undo step), then saves. | Task 1, Task 9 |
| A new driver | The save (or a silent disk load after a git pull) leaves the file declaring a driver tap did not start with; `DeckSessionController.fileTextChanged` restarts `tap dev` (`TapSession.restart`), which asks about the new driver only ("This deck now also wants to run shell"). A tap that exits on its own while a sheet is up ends the sheet, and the restarted one asks afresh. | Task 10 |
| The Deck tab | `tap deck schema --json` is loaded once per app; the Deck segment enables when it has. The form reads the buffer's frontmatter (`Frontmatter`) and writes each change as one `replaceText` with the frontmatter's own line endings; the drivers group lists the declared drivers with the `${NAME}` hint; keys tap does not know are listed under Other keys. | Task 11, Task 12 |

## File structure

| Path | Responsibility |
|---|---|
| `internal/slidelist/slidelist.go` | Modify: `CodeBlock.Problem` (`json:"problem,omitempty"`), copied from the transformed block |
| `internal/cli/approval_scenarios_test.go` | Create: the two CLI-only scenarios of `06-live-code-and-trust.feature`, named as the manifest claims them |
| `desktop/scripts/check-scenarios.sh`, `check-scenarios-test.sh` | Modify: a claimed scenario is also satisfied by `func Test<Name>(` in `internal/**/*_test.go` |
| `desktop/TapDesktopCore/Sources/TapDesktopCore/TapProtocol.swift` | Modify: `ApprovalDriver`, `ApprovalBlock`; `QuestionPayload.drivers`, `.approvedBefore`, `.blocks`; `CodeBlock.problem` |
| `.../TapDesktopCore/TapSession.swift` | Modify: `restart(reason:)` |
| `.../TapDesktopCore/Frontmatter.swift` | The frontmatter as lines: `Entry`, `range`, `entries`, `lineEnding`, `entry(at:)`, `value(at:)`, `declaredDrivers`, `setting(path:to:)`, `addingDriver`, `rawBlock(at:)`, `settingRawBlock(at:to:)`, `scalar(forString:)`, `unquoted` |
| `.../TapDesktopCore/DeckSchema.swift` | `SchemaKey` and `DeckSchema.decode` for `tap deck schema --json` |
| `.../TapDesktopCore/BoxHeader.swift` | Modify: block problems in `errors`, `FixIt`, `init(slide:declaredDrivers:)` |
| `desktop/Tap/Presenting/QuestionSheet.swift` | Modify: `ReturnAnswer`, `detail:`, `keyDown` for Escape; `ApprovalSheet`, `ApprovalBlockRow` |
| `desktop/Tap/Documents/DeckSessionController.swift` | Modify: the `tap dev` question queue (`pendingQuestions`, `onQuestion`, `answer`, `questionGeneration`, `onQuestionsDropped`), `allowDriver`, `saveNow`, `fileTextChanged` and `driversTapStartedWith`, the Deck form wiring, the context menu's fix-it |
| `desktop/Tap/Windows/DeckWindowController.swift` | Modify: `presentDeckQuestion`, `QuestionSource` and `questionSheetSource`, the `approval` case for talks, `allowDriverInThisDeck`, `showPreviewTab`, `showDeckTab`, Play refused while a question sheet is up |
| `desktop/Tap/Editor/EditorTextView.swift` | Modify: the fix-it pill drawn in the header, `fixItRect(forBoxAt:)`, the hit test in `mouseDown`, `applyFixItForBoxAt` in the delegate |
| `desktop/Tap/Preview/InspectorViewController.swift` | Modify: `Tab`, `showTab`, `embedDeck`, `setDeckTabAvailable`, the segmented control's action |
| `desktop/Tap/Preview/DeckFormViewController.swift` | The Deck tab form |
| `desktop/Tap/Preview/DeckSchemaLoader.swift` | Runs `tap deck schema --json` once, like `LayoutCatalogLoader` |
| `desktop/Tap/App/AppEnvironment.swift` | Modify: `deckSchema`, loaded in `warmUp` |
| `desktop/Tap/App/MainMenu.swift` | Modify: Slide > Allow Driver in This Deck; View > Show Preview Tab, Show Deck Tab |
| `desktop/TapTests/Support/HostedTestCase.swift` | Modify: `approvesLiveCodeOnOpen`, `approveLiveCode(for:drivers:)`, `settingsFile` |
| `desktop/TapTests/Support/Fixtures.swift` | Modify: `realPath(of:)` |
| `desktop/TapTests/Support/FakeTapScripts.swift` | Modify: `presenting(... exitsOnAnswer:)` |
| `desktop/TapTests/Support/PreviewViewController+LiveCode.swift` | Test-only page reads: the Run buttons, a click, the result |
| `desktop/TapTests/Support/TapClient+Tests.swift` | Test-only `execute(json:)` |
| `desktop/TapTests/Support/TapApproval.swift` | Runs the bundled `tap approval ...` under the test's config home |
| `desktop/TapTests/Fixtures/live-code.md`, `undeclared-driver.md`, `no-drivers.md`, `custom-driver.md` | The live code fixtures |
| `desktop/TapTests/*.swift` | The hosted tests: `ApprovalSheetTests`, `LiveCodeApprovalTests`, `RunBlockTests`, `TalkApprovalTests`, `FixItTests`, `NewDriverTests`, `DeckTabTests`, `DeckTabDriversTests` |
| `desktop/TapUITests/LiveCodeUITests.swift`, `Support/UITestCase.swift` | Return on the real sheet; fixtures pre-approved for the other UI tests |
| `desktop/scenarios.txt`, `desktop/README.md` | Modify: the 17 D5 rows; the live code tests and the manual pass |

---

### Task 1: The block's problem in the slide list, and the two CLI scenarios as Go tests

**Files:**
- Modify: `internal/slidelist/slidelist.go` (`CodeBlock`, `Build`)
- Create: `internal/slidelist/problem_test.go`
- Create: `internal/cli/approval_scenarios_test.go`
- Modify: `desktop/scripts/check-scenarios.sh`, `desktop/scripts/check-scenarios-test.sh`

**Interfaces:**
- Consumes: `transformer.TransformedCodeBlock.Problem` (set in `transformer.go` from `config.UndeclaredDriverMessage`, the one reason today), `slidelist.Build(source, baseDir)`, the test helpers in `internal/cli/approval_test.go` (`approvalFixture`, `approvalInputFor`, `terminalAsker`, `fakeAsker`), `liveCodeApproval`, `usersettings.Load`.
- Produces: `slidelist.CodeBlock.Problem string` (`json:"problem,omitempty"`), which Task 2's `CodeBlock.problem` decodes; `TestFirstOpenOfADeckWithLiveCodeInTheCLI` and `TestNonInteractiveRuns`; a `check-scenarios.sh` that accepts `func Test<Name>(` in `internal/`.

- [ ] **Step 1: Write the failing slide list test**

`internal/slidelist/problem_test.go`:

```go
package slidelist

import (
	"strings"
	"testing"
)

// The problem the page shows in a live block and /api/execute refuses it
// with rides along in the slide list, so the app can mark the block's
// box; a block that can run has none.
func TestBuildCarriesEachBlocksProblem(t *testing.T) {
	source := "---\ndrivers:\n  sqlite: {}\n---\n\n# Query\n\n```sql {driver: sqlite}\nSELECT 1;\n```\n\n---\n\n# Shell\n\n```bash {driver: shell}\necho six\n```\n"
	result, err := Build([]byte(source), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Slides[0].CodeBlocks[0].Problem; got != "" {
		t.Errorf("a declared driver's block has a problem: %q", got)
	}
	want := `This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter.`
	if got := result.Slides[1].CodeBlocks[0].Problem; got != want {
		t.Errorf("problem = %q, want %q", got, want)
	}

	noDrivers := "# Query\n\n```sql {driver: sqlite}\nSELECT 1;\n```\n"
	result, err = Build([]byte(noDrivers), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got := result.Slides[0].CodeBlocks[0].Problem
	if !strings.HasPrefix(got, "This deck does not declare the sqlite driver. Add this to the frontmatter:") || !strings.Contains(got, "\n  sqlite: {}") {
		t.Errorf("problem without a drivers map = %q", got)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/slidelist -run TestBuildCarriesEachBlocksProblem`
Expected: FAIL, `result.Slides[1].CodeBlocks[0].Problem` is not a field (a compile error is this step's failure).

- [ ] **Step 3: Add `Problem` to the slide list**

In `internal/slidelist/slidelist.go`, add to `CodeBlock` after `Line`:

```go
	// Problem says why a live block cannot run, in the words the page shows
	// in the block and /api/execute refuses it with: today, a driver the
	// deck does not declare (config.UndeclaredDriverMessage). Empty for a
	// block that can run, and for a block with no driver.
	Problem string `json:"problem,omitempty"`
```

In `Build`, inside the `for blockIndex, block := range parsed.CodeBlocks` loop, after the `codeBlock.Line` assignment and before `slide.CodeBlocks = append(...)`:

```go
			// The transformer makes one transformed block per parsed block, in order.
			if blockIndex < len(rendered.CodeBlocks) {
				codeBlock.Problem = rendered.CodeBlocks[blockIndex].Problem
			}
```

- [ ] **Step 4: Run the slide list tests**

Run: `go test ./internal/slidelist`
Expected: PASS, the new test included. Then `go test ./internal/cli -run 'TestSlideList|TestAppSource'` to see the JSON round trips still pass (the field is `omitempty`, so decks without a problem print exactly what they did).

- [ ] **Step 5: Write the two scenario tests**

`internal/cli/approval_scenarios_test.go`:

```go
package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// The two scenarios of docs/superpowers/specs/tap-desktop-features/
// 06-live-code-and-trust.feature that belong to the CLI alone, named the
// way desktop/scenarios.txt claims them (desktop/scripts/check-scenarios.sh
// reads Go tests too). The behaviour itself is covered in approval_test.go;
// these pin the scenarios' own wording.

func TestFirstOpenOfADeckWithLiveCodeInTheCLI(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	input, out := approvalInputFor(t, cfg, pres, nil)
	// The person answers no on the terminal.
	input.Asker = terminalAsker{in: strings.NewReader("n\n"), out: out}

	policy, err := liveCodeApproval(input)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"This deck can run code on this computer:",
		"shell      2 blocks on slides 1, 2",
		"Allow this deck to run code? Type s to show the code. [y/N/s]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the terminal prompt lacks %q:\n%s", want, text)
		}
	}
	if len(policy.Drivers) != 0 || policy.AllowAll {
		t.Errorf("a no leaves live code off, got %+v", policy)
	}
	if _, err := os.Stat(input.SettingsPath); !os.IsNotExist(err) {
		t.Error("a no stored something")
	}
}

func TestNonInteractiveRuns(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, out := approvalInputFor(t, cfg, pres, asker)
	input.Interactive = false

	policy, err := liveCodeApproval(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Drivers) != 0 || policy.AllowAll || len(asker.requests) != 0 {
		t.Errorf("without a terminal live code is off and nothing is asked, got %+v after %d questions", policy, len(asker.requests))
	}
	if !strings.Contains(out.String(), "Live code is off: this deck is not approved to run shell.") {
		t.Errorf("output = %q", out.String())
	}

	input.AllowCode = true
	policy, err = liveCodeApproval(input)
	if err != nil || !policy.AllowAll {
		t.Errorf("--allow-code turns live code on for the run, got %+v, %v", policy, err)
	}
	if _, err := os.Stat(input.SettingsPath); !os.IsNotExist(err) {
		t.Error("--allow-code stored an approval")
	}
	settings, _ := usersettings.Load(input.SettingsPath)
	if len(settings.Approvals) != 0 {
		t.Errorf("approvals stored: %+v", settings.Approvals)
	}
}
```

`approvalInputFor` takes an `approvalAsker`; passing `nil` and setting `input.Asker` after is fine, since the field is only read by `liveCodeApproval`.

- [ ] **Step 6: Run them**

Run: `go test ./internal/cli -run 'TestFirstOpenOfADeckWithLiveCodeInTheCLI|TestNonInteractiveRuns' -v`
Expected: both PASS. The prompt's driver line is `fmt.Sprintf("  %-10s %s", ...)`, so "shell" is padded to ten characters before "2 blocks on slides 1, 2".

- [ ] **Step 7: Teach the scenario check to read Go tests**

In `desktop/scripts/check-scenarios.sh`, replace the `tests=$(find ...)` line and the final `if ! grep -lq ... ; then` block with:

```sh
swift_tests=$(find "$root/desktop" -name '*.swift' -path '*Tests*' -o -name '*.swift' -path '*Benchmarks*')
go_tests=$(find "$root/internal" -name '*_test.go' 2>/dev/null)

# A scenario is covered by a Swift test named testName, or by a Go test
# named TestName, in the tap packages: the CLI-only scenarios of a feature
# file are tap's to prove.
has_test() {
	grep -q "func test$1(" $swift_tests 2>/dev/null && return 0
	[ -n "$go_tests" ] && grep -q "func Test$1(" $go_tests 2>/dev/null && return 0
	return 1
}
```

and, in the loop, replace the two-grep `if` with:

```sh
	if ! has_test "$name"; then
		echo "$milestone claims \"$scenario\" ($file), but no test is named test$name or Test$name"
		status=1
	fi
```

The `has_test` function must be defined before the `while` loop. In `desktop/scripts/check-scenarios-test.sh`, after the existing "a missing test should fail the check" block, add:

```sh
# A Go test named after the scenario satisfies it too.
mkdir -p "$root/internal/cli"
cat > "$root/internal/cli/scenario_test.go" <<'GO'
func TestOpenADeck(t *testing.T) {}
GO
"$script" "$root" >/dev/null || { echo "a Go test should satisfy a claimed scenario"; exit 1; }
rm "$root/internal/cli/scenario_test.go"
```

Run: `make -C desktop check-scenarios`
Expected: `every claimed scenario has a test` (the self-test first, then the check over the real tree; nothing is claimed for D5 yet).

- [ ] **Step 8: Mutate and commit**

Mutations, each applied and run locally, then reverted exactly: in `Build`, drop the `codeBlock.Problem = ...` line (expected: `TestBuildCarriesEachBlocksProblem` fails on the shell block); in `Build`, copy `Problem` from `rendered.CodeBlocks[0]` for every block (expected: it fails on the sqlite block, which gains a problem); in `has_test`, drop the Go grep (expected: `check-scenarios-test.sh` fails on "a Go test should satisfy"); in `terminalAsker.askApproval`, return `true` for `"n"` (expected: `TestFirstOpenOfADeckWithLiveCodeInTheCLI` fails on the policy); in `liveCodeApproval`, skip the `!input.Interactive` branch (expected: `TestNonInteractiveRuns` fails on `asker.requests`).

```bash
git add internal/slidelist internal/cli/approval_scenarios_test.go desktop/scripts
git commit -m "feat(slidelist): carry each live block's problem, and name the CLI live code scenarios as Go tests"
```

---

### Task 2: The approval payload, the block's problem, and a session restart

**Files:**
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/TapProtocol.swift`
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/TapSession.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/TapProtocolTests.swift`, `TapSessionTests.swift`

**Interfaces:**
- Consumes: D4's `QuestionPayload(deck:settingsPath:directory:segments:)`, `CodeBlock(block:language:driver:live:line:)`, `TapSession.start()`, `changeDeck(to:)` (its `startsAfterStop` pattern), `FakeTap.ready(recordingTo:)`, `waitUntil`.
- Produces: `ApprovalDriver(name:command:slides:blocks:)`, `ApprovalBlock(driver:code:slide:block:)`; `QuestionPayload.drivers: [ApprovalDriver]?`, `.approvedBefore: [String]?`, `.blocks: [ApprovalBlock]?`, `.isForNewDrivers: Bool`, `.approvalSummary: String` ("2 shell, 1 sqlite"); `CodeBlock.problem: String?` (`init` gains `problem: String? = nil`); `TapSession.restart(reason:)`.

- [ ] **Step 1: Write the failing protocol tests**

In `TapProtocolTests.swift`, add:

```swift
    func testDecodesTheApprovalRequest() {
        let line = #"{"type":"question","id":"q1","kind":"approval","payload":{"deck":"/private/tmp/t/talk.md","drivers":[{"name":"fortune","command":"/bin/cat","slides":[3],"blocks":1},{"name":"shell","slides":[2,5],"blocks":2}],"approvedBefore":["sqlite"],"blocks":[{"driver":"shell","code":"echo hi","slide":2,"block":1},{"driver":"fortune","code":"hello","slide":3,"block":1}]}}"#
        let expected = QuestionPayload(
            deck: "/private/tmp/t/talk.md",
            drivers: [ApprovalDriver(name: "fortune", command: "/bin/cat", slides: [3], blocks: 1),
                      ApprovalDriver(name: "shell", command: nil, slides: [2, 5], blocks: 2)],
            approvedBefore: ["sqlite"],
            blocks: [ApprovalBlock(driver: "shell", code: "echo hi", slide: 2, block: 1),
                     ApprovalBlock(driver: "fortune", code: "hello", slide: 3, block: 1)])
        XCTAssertEqual(TapEvent.decode(line: line), .question(id: "q1", kind: "approval", payload: expected))
        XCTAssertTrue(expected.isForNewDrivers)
        XCTAssertEqual(expected.approvalSummary, "1 fortune, 2 shell")
        let first = QuestionPayload(deck: "/a.md", drivers: [ApprovalDriver(name: "shell", slides: [2, 5], blocks: 2), ApprovalDriver(name: "sqlite", slides: [4], blocks: 1)])
        XCTAssertFalse(first.isForNewDrivers, "no approvedBefore means the first time")
        XCTAssertEqual(first.approvalSummary, "2 shell, 1 sqlite")
        XCTAssertEqual(QuestionPayload().approvalSummary, "")
    }

    func testDecodesABlocksProblem() throws {
        let data = Data(#"{"ok":true,"slides":[{"number":1,"startLine":1,"endLine":3,"layout":"default","title":"","fragments":0,"steps":0,"skip":false,"errors":[],"codeBlocks":[{"block":1,"language":"bash","driver":"shell","live":true,"line":2,"problem":"This deck does not declare the shell driver. Add \"shell: {}\" under drivers in the frontmatter."},{"block":2,"language":"sql","driver":"sqlite","live":true,"line":3}]}],"errors":[]}"#.utf8)
        let list = try SlideList.decodeResponse(data)
        XCTAssertEqual(list.slides[0].codeBlocks[0].problem, #"This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter."#)
        XCTAssertNil(list.slides[0].codeBlocks[1].problem, "a block that can run has none")
        XCTAssertEqual(list.slides[0].codeBlocks[1], CodeBlock(block: 2, language: "sql", driver: "sqlite", live: true, line: 3))
    }
```

- [ ] **Step 2: Write the failing session test**

In `TapSessionTests.swift`, add:

```swift
    func testRestartStopsAndStartsAgainWithoutCountingACrash() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = session(try FakeTap.ready(recordingTo: record))
        var states: [TapSession.State] = []
        tap.onStateChange = { states.append($0) }
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        tap.restart(reason: "the deck now declares shell")
        try await waitUntil(timeout: 10, "a second running state") {
            states.filter { if case .running = $0 { return true } else { return false } }.count == 2
        }
        XCTAssertFalse(states.contains { if case .restarting = $0 { return true } else { return false } }, "a requested exit is not a crash")
        XCTAssertTrue(states.contains(.stopped), "the first tap stopped before the second started")
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertEqual(recorded.components(separatedBy: "arguments: dev --app").count - 1, 2, "tap ran twice")
        XCTAssertTrue(tap.log.text.contains("restarting tap: the deck now declares shell"))
        tap.stop()
        try await waitUntil { tap.state == .stopped }
    }

    func testRestartWithNoProcessIsAStart() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = session(try FakeTap.ready(recordingTo: record))
        XCTAssertEqual(tap.state, .stopped)
        tap.restart(reason: "a start in disguise")
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        tap.stop()
        try await waitUntil { tap.state == .stopped }
    }
```

- [ ] **Step 3: Run the core tests to verify they fail**

Run: `make -C desktop core-test`
Expected: the package does not compile (`ApprovalDriver`, `problem`, `restart(reason:)` are undefined). That is the failure for this step.

- [ ] **Step 4: Extend `TapProtocol.swift`**

Add `problem` to `CodeBlock`:

```swift
    /// Why a live block cannot run, in tap's words (today: the deck does
    /// not declare its driver, config.UndeclaredDriverMessage). nil for a
    /// block that can run. The page shows the same text in the block.
    public let problem: String?

    public init(block: Int, language: String, driver: String, live: Bool, line: Int, problem: String? = nil) {
        self.block = block
        self.language = language
        self.driver = driver
        self.live = live
        self.line = line
        self.problem = problem
    }
```

(replacing the existing `init`). Add before `QuestionPayload`:

```swift
/// One driver of tap's approval request (internal/cli/approval.go,
/// approvalDriver): what a yes would allow. `command` is what a custom
/// driver runs, with its arguments and variables expanded; nil for a
/// built-in driver. `slides` are the slides with a block that uses it,
/// `blocks` how many.
public struct ApprovalDriver: Codable, Equatable, Sendable {
    public let name: String
    public let command: String?
    public let slides: [Int]
    public let blocks: Int

    public init(name: String, command: String? = nil, slides: [Int] = [], blocks: Int = 0) {
        self.name = name
        self.command = command
        self.slides = slides
        self.blocks = blocks
    }
}

/// One live code block of an approval request, so the sheet can show its
/// code before the person allows it. `slide` and `block` are 1-based,
/// `block` counting the slide's live blocks, as /api/execute names them.
public struct ApprovalBlock: Codable, Equatable, Sendable {
    public let driver: String
    public let code: String
    public let slide: Int
    public let block: Int

    public init(driver: String, code: String, slide: Int, block: Int) {
        self.driver = driver
        self.code = code
        self.slide = slide
        self.block = block
    }
}
```

Replace `QuestionPayload` with:

```swift
/// What a `question` event carries. Each kind uses a few of the fields:
/// `approval` the deck, the drivers a yes would allow, the ones an earlier
/// yes allowed, and the blocks to read; `record-consent` the settings file
/// the answer is saved to; `keep-recording` the run's folder and how many
/// segments it has. See internal/cli/approval.go, app_questions.go and
/// app_session.go.
public struct QuestionPayload: Codable, Equatable, Sendable {
    public let deck: String?
    public let settingsPath: String?
    public let directory: String?
    public let segments: Int?
    public let drivers: [ApprovalDriver]?
    public let approvedBefore: [String]?
    public let blocks: [ApprovalBlock]?

    public init(deck: String? = nil, settingsPath: String? = nil, directory: String? = nil, segments: Int? = nil,
                drivers: [ApprovalDriver]? = nil, approvedBefore: [String]? = nil, blocks: [ApprovalBlock]? = nil) {
        self.deck = deck
        self.settingsPath = settingsPath
        self.directory = directory
        self.segments = segments
        self.drivers = drivers
        self.approvedBefore = approvedBefore
        self.blocks = blocks
    }

    /// True when the deck was approved before and tap asks only about
    /// the drivers it has since gained ("This deck now also wants to run shell").
    public var isForNewDrivers: Bool { !(approvedBefore ?? []).isEmpty }

    /// "2 shell, 1 sqlite": the block counts by driver, in tap's order.
    public var approvalSummary: String {
        (drivers ?? []).map { "\($0.blocks) \($0.name)" }.joined(separator: ", ")
    }
}
```

Every D4 call site (`QuestionPayload(deck:)`, `QuestionPayload(settingsPath:)`, `QuestionPayload(directory:segments:)`, `QuestionPayload()`) still compiles: the new parameters default to nil, and `Codable` synthesis decodes an absent key as nil.

- [ ] **Step 5: Add `restart(reason:)` to `TapSession`**

After `changeDeck(to:)`:

```swift
    /// Stops tap and starts it again on the same deck, for a change only a
    /// fresh start reads: the deck now declares a driver, and tap's live
    /// code policy is fixed at startup (internal/cli/dev.go), so only a new
    /// process asks about it. The exit is a requested one, never counted by
    /// the restart policy. With no process running it is a plain start.
    public func restart(reason: String) {
        log.append("restarting tap: \(reason)", source: .app)
        guard let process else {
            start()
            return
        }
        startsAfterStop = true
        process.stop()
    }
```

`processExited(status:requested:)` already starts again when `startsAfterStop` is set, exactly as `changeDeck` relies on.

- [ ] **Step 6: Run the core tests**

Run: `make -C desktop core-test`
Expected: every test passes, the four new ones included. `FakeTap.ready` reads stdin until it closes, so `process.stop()` (which closes stdin) ends it, and the record file holds two `arguments:` lines.

- [ ] **Step 7: Mutate and commit**

Mutations, each applied and run with `make -C desktop core-test`, then reverted exactly: in `restart`, drop `startsAfterStop = true` (expected: `testRestartStopsAndStartsAgainWithoutCountingACrash` times out on the second running state); in `restart`, call `unexpectedExit()` instead of `process.stop()` (expected: it fails on `.restarting`); in `restart`, drop the `guard let process` branch's `start()` (expected: `testRestartWithNoProcessIsAStart` times out); in `approvalSummary`, join with `"; "` (expected: `testDecodesTheApprovalRequest` fails on "2 shell, 1 sqlite"); in `isForNewDrivers`, return `drivers != nil` (expected: it fails on `first.isForNewDrivers`); in `CodeBlock`, name the coding key `"reason"` (expected: `testDecodesABlocksProblem` fails on nil).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): decode the approval request and each block's problem, and restart a session on request"
```

---

### Task 3: The frontmatter as lines: parsing and the declared drivers

**Files:**
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/Frontmatter.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/FrontmatterTests.swift`

**Interfaces:**
- Consumes: nothing beyond Foundation.
- Produces: `Frontmatter(text:)`; `Frontmatter.Entry` (`key`, `value: String?`, `valueRange: NSRange?`, `children: [Entry]`, `range: NSRange`, `indent: Int`, `unquotedValue`); `Frontmatter.range: NSRange?`, `.entries`, `.lineEnding`, `.hasFrontmatter`, `.closingLocation: Int?`, `entry(at:)`, `value(at:)`, `declaredDrivers`, `declares(driver:)`, `text(of:)`; `Frontmatter.unquoted(_:)`. Task 4 adds the edits on top of these.

- [ ] **Step 1: Write the failing tests**

`desktop/TapDesktopCore/Tests/TapDesktopCoreTests/FrontmatterTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class FrontmatterTests: XCTestCase {
    let deck = """
    ---
    title: Debugging Production at 3am
    theme: "terminal"
    slideNumbers: false
    # the live code drivers
    drivers:
      sqlite:
        connections:
          incident:
            path: ./incident.db

      shell: {}
    recording:
      output: recordings
    ---

    # One
    """

    func testReadsTheBlockAndItsEntries() throws {
        let frontmatter = Frontmatter(text: deck)
        let text = deck as NSString
        XCTAssertEqual(frontmatter.range, NSRange(location: 0, length: text.range(of: "---\n\n# One").location + 4), "the block runs through the closing line's newline")
        XCTAssertEqual(frontmatter.closingLocation, text.range(of: "---\n\n# One").location)
        XCTAssertEqual(frontmatter.lineEnding, "\n")
        XCTAssertEqual(frontmatter.entries.map(\.key), ["title", "theme", "slideNumbers", "drivers", "recording"])
        XCTAssertEqual(frontmatter.value(at: ["title"]), "Debugging Production at 3am")
        XCTAssertEqual(frontmatter.value(at: ["theme"]), "\"terminal\"")
        XCTAssertEqual(frontmatter.entry(at: ["theme"])?.unquotedValue, "terminal")
        XCTAssertEqual(frontmatter.value(at: ["slideNumbers"]), "false")
        let drivers = try XCTUnwrap(frontmatter.entry(at: ["drivers"]))
        XCTAssertNil(drivers.value, "a block opener has no scalar")
        XCTAssertEqual(drivers.children.map(\.key), ["sqlite", "shell"])
        XCTAssertEqual(drivers.children.map(\.indent), [2, 2])
        XCTAssertEqual(frontmatter.value(at: ["drivers", "shell"]), "{}")
        XCTAssertEqual(frontmatter.value(at: ["drivers", "sqlite", "connections", "incident", "path"]), "./incident.db")
        XCTAssertEqual(frontmatter.text(of: drivers), "drivers:\n  sqlite:\n    connections:\n      incident:\n        path: ./incident.db\n\n  shell: {}\n",
                       "an entry's text runs to its last child, blank lines inside included")
        XCTAssertEqual(frontmatter.value(at: ["recording", "output"]), "recordings")
        XCTAssertNil(frontmatter.value(at: ["author"]))
        let title = try XCTUnwrap(frontmatter.entry(at: ["title"]))
        XCTAssertEqual(text.substring(with: try XCTUnwrap(title.valueRange)), "Debugging Production at 3am")
        XCTAssertEqual(text.substring(with: title.range), "title: Debugging Production at 3am\n")
    }

    func testTheDeclaredDriversComeFromTheDriversMap() {
        XCTAssertEqual(Frontmatter(text: deck).declaredDrivers, ["sqlite", "shell"])
        XCTAssertTrue(Frontmatter(text: deck).declares(driver: "shell"))
        XCTAssertFalse(Frontmatter(text: deck).declares(driver: "mysql"))
        XCTAssertEqual(Frontmatter(text: "---\ntitle: x\n---\n# One\n").declaredDrivers, [])
        XCTAssertEqual(Frontmatter(text: "# One\n").declaredDrivers, [])
        XCTAssertEqual(Frontmatter(text: "---\ndrivers: {shell: {}, sqlite: {connections: {a: {path: x}}}}\n---\n").declaredDrivers, ["shell", "sqlite"],
                       "a flow map declares its keys too")
        XCTAssertEqual(Frontmatter(text: "---\ndrivers: {}\n---\n").declaredDrivers, [])
        XCTAssertEqual(Frontmatter(text: "---\ndrivers:\n---\n").declaredDrivers, [])
    }

    func testADeckWithoutFrontmatter() {
        let frontmatter = Frontmatter(text: "# One\n\n---\n\n# Two\n")
        XCTAssertNil(frontmatter.range, "a separator later in the deck is not a frontmatter")
        XCTAssertFalse(frontmatter.hasFrontmatter)
        XCTAssertEqual(frontmatter.entries, [])
        XCTAssertNil(Frontmatter(text: "").range)
        XCTAssertNil(Frontmatter(text: "---\ntitle: x\n").range, "a block that never closes is not one")
        XCTAssertNil(Frontmatter(text: "\n---\ntitle: x\n---\n").range, "the first line must be the opener, as tap reads it")
    }

    func testKeepsCarriageReturnLineEndings() {
        let windows = "---\r\ntitle: x\r\ndrivers:\r\n  shell: {}\r\n---\r\n\r\n# One\r\n"
        let frontmatter = Frontmatter(text: windows)
        XCTAssertEqual(frontmatter.lineEnding, "\r\n")
        XCTAssertEqual(frontmatter.value(at: ["title"]), "x")
        XCTAssertEqual(frontmatter.declaredDrivers, ["shell"])
        XCTAssertEqual(frontmatter.range?.length, (windows as NSString).range(of: "\r\n\r\n# One").location + 2)
    }

    func testCommentsBlankLinesAndOddSpacingAreNotEntries() {
        let text = "---\n# a comment\n\ntitle:   spaced   \nkey-with-dash: 1\ndotted.key: 2\ntabbed:\tvalue\nlist:\n  - one\n  - two\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.entries.map(\.key), ["title", "key-with-dash", "dotted.key", "tabbed", "list"])
        XCTAssertEqual(frontmatter.value(at: ["title"]), "spaced")
        XCTAssertEqual(frontmatter.value(at: ["tabbed"]), "value")
        XCTAssertNil(frontmatter.value(at: ["list"]), "a list opener has no scalar")
        XCTAssertEqual(frontmatter.entry(at: ["list"])?.children, [], "list items are not entries")
        XCTAssertEqual(frontmatter.text(of: frontmatter.entry(at: ["list"])!), "list:\n  - one\n  - two\n")
    }

    func testUnquoting() {
        XCTAssertEqual(Frontmatter.unquoted("plain"), "plain")
        XCTAssertEqual(Frontmatter.unquoted("\"a \\\"b\\\" c\""), "a \"b\" c")
        XCTAssertEqual(Frontmatter.unquoted("\"back\\\\slash\""), "back\\slash")
        XCTAssertEqual(Frontmatter.unquoted("'it''s'"), "it's")
        XCTAssertEqual(Frontmatter.unquoted("\"\""), "")
        XCTAssertEqual(Frontmatter.unquoted("\"unterminated"), "\"unterminated")
    }
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `make -C desktop core-test`
Expected: the package does not compile (`Frontmatter` is undefined). That is the failure for this step.

- [ ] **Step 3: Write `Frontmatter.swift`**

```swift
import Foundation

/// The deck's frontmatter as text: the YAML block between the "---" lines
/// at the very start of the deck, read as lines rather than parsed as
/// YAML. tap's frontmatter is a map of scalars, one-level maps
/// (themeColors, recording) and the drivers map, and what the app
/// rewrites is one line at a time, so the rest of the block stays byte
/// for byte as the person wrote it. tap reads the same block
/// (internal/config/config.go, parseFrontmatter): the first line, trimmed,
/// must be "---", and the block ends at the next line that is.
///
/// An entry is a line "key: value" or "key:" at some indent; the lines
/// after it that are indented deeper, or blank, or comments, belong to
/// it, and the entries among them at the smallest indent are its
/// children. Anything else (list items, continuation lines) is kept
/// inside its entry's range and never read.
public struct Frontmatter: Equatable, Sendable {
    public struct Entry: Equatable, Sendable {
        public let key: String
        /// The text after "key:", trimmed; nil for "key:" alone, which
        /// opens a block (or is YAML's null).
        public let value: String?
        /// Where `value` sits in the whole text.
        public let valueRange: NSRange?
        public let children: [Entry]
        /// The entry's lines, children included, in the whole text. It
        /// ends with the line ending of its last line, so an insertion at
        /// its end starts a new line right below it.
        public let range: NSRange
        /// The indent of the entry's own line, in spaces.
        public let indent: Int

        public init(key: String, value: String?, valueRange: NSRange?, children: [Entry], range: NSRange, indent: Int) {
            self.key = key
            self.value = value
            self.valueRange = valueRange
            self.children = children
            self.range = range
            self.indent = indent
        }

        /// The value with YAML's quotes removed.
        public var unquotedValue: String? { value.map(Frontmatter.unquoted) }
    }

    /// The whole block from location 0 through the closing line's ending;
    /// nil when the deck has no frontmatter.
    public let range: NSRange?
    public let entries: [Entry]
    /// The line ending the deck uses, so an edit writes the same.
    public let lineEnding: String
    /// Where the closing "---" line starts: a new top-level key goes in front of it.
    public let closingLocation: Int?
    private let source: String

    public var hasFrontmatter: Bool { range != nil }

    private struct Line {
        let range: NSRange
        let content: String
    }

    public init(text: String) {
        source = text
        lineEnding = text.contains("\r\n") ? "\r\n" : "\n"
        let nsText = text as NSString
        var lines: [Line] = []
        var location = 0
        var closingIndex: Int?
        // Read lines only as far as the closing "---": the deck's body is never looked at.
        while location < nsText.length {
            var start = 0, end = 0, contentsEnd = 0
            nsText.getLineStart(&start, end: &end, contentsEnd: &contentsEnd, for: NSRange(location: location, length: 0))
            let content = nsText.substring(with: NSRange(location: start, length: contentsEnd - start))
            lines.append(Line(range: NSRange(location: start, length: end - start), content: content))
            if lines.count == 1, content.trimmingCharacters(in: .whitespaces) != "---" { break }
            if lines.count > 1, content.trimmingCharacters(in: .whitespaces) == "---" {
                closingIndex = lines.count - 1
                break
            }
            guard end > location else { break }
            location = end
        }
        guard let closingIndex else {
            range = nil
            closingLocation = nil
            entries = []
            return
        }
        range = NSRange(location: 0, length: NSMaxRange(lines[closingIndex].range))
        closingLocation = lines[closingIndex].range.location
        entries = Self.parse(lines: Array(lines[1..<closingIndex]))
    }

    private static let keyPattern = try! NSRegularExpression(pattern: #"^( *)([A-Za-z0-9_.-]+):(?:[ \t]+(.*?))?[ \t]*$"#)

    private struct KeyLine {
        let indent: Int
        let key: String
        let value: String?
        let valueRange: NSRange?
    }

    private static func keyLine(_ line: Line) -> KeyLine? {
        let content = line.content as NSString
        guard let match = keyPattern.firstMatch(in: line.content, range: NSRange(location: 0, length: content.length)) else { return nil }
        let indent = match.range(at: 1).length
        let key = content.substring(with: match.range(at: 2))
        var value: String?
        var valueRange: NSRange?
        let valueMatch = match.range(at: 3)
        if valueMatch.location != NSNotFound, valueMatch.length > 0 {
            let text = content.substring(with: valueMatch)
            // A trailing comment is not a value; a value that starts one is nothing.
            if !text.hasPrefix("#") {
                value = text
                valueRange = NSRange(location: line.range.location + valueMatch.location, length: valueMatch.length)
            }
        }
        return KeyLine(indent: indent, key: key, value: value, valueRange: valueRange)
    }

    private static func isBlankOrComment(_ line: Line) -> Bool {
        let trimmed = line.content.trimmingCharacters(in: .whitespaces)
        return trimmed.isEmpty || trimmed.hasPrefix("#")
    }

    private static func parse(lines: [Line]) -> [Entry] {
        var entries: [Entry] = []
        var index = 0
        while index < lines.count {
            guard let opener = keyLine(lines[index]) else {
                index += 1
                continue
            }
            // The entry runs until the next key at an indent no deeper than its own.
            var end = index + 1
            while end < lines.count {
                if let next = keyLine(lines[end]), next.indent <= opener.indent { break }
                end += 1
            }
            // Trailing blank and comment lines belong to the gap, not to the
            // entry, so an insertion after the entry lands right below it.
            var last = end - 1
            while last > index, isBlankOrComment(lines[last]) { last -= 1 }
            let children = last > index ? parse(lines: Array(lines[(index + 1)...last])) : []
            let range = NSRange(location: lines[index].range.location, length: NSMaxRange(lines[last].range) - lines[index].range.location)
            entries.append(Entry(key: opener.key, value: opener.value, valueRange: opener.valueRange, children: children, range: range, indent: opener.indent))
            index = end
        }
        return entries
    }

    public func entry(at path: [String]) -> Entry? {
        var siblings = entries
        var found: Entry?
        for name in path {
            guard let next = siblings.first(where: { $0.key == name }) else { return nil }
            found = next
            siblings = next.children
        }
        return found
    }

    public func value(at path: [String]) -> String? {
        entry(at: path)?.value
    }

    /// An entry's lines, as written.
    public func text(of entry: Entry) -> String {
        (source as NSString).substring(with: entry.range)
    }

    /// The names under `drivers`, in the file's order: what tap's
    /// `Config.DeclaredDrivers()` holds for this text, block or flow style.
    public var declaredDrivers: [String] {
        guard let drivers = entry(at: ["drivers"]) else { return [] }
        if !drivers.children.isEmpty { return drivers.children.map(\.key) }
        if let value = drivers.value { return Self.flowMapKeys(value) }
        return []
    }

    public func declares(driver name: String) -> Bool {
        declaredDrivers.contains(name)
    }

    /// The keys of a flow map, "{shell: {}, sqlite: {path: x}}": the text
    /// before each top-level colon, split on the commas outside any braces.
    static func flowMapKeys(_ flow: String) -> [String] {
        let trimmed = flow.trimmingCharacters(in: .whitespaces)
        guard trimmed.hasPrefix("{"), trimmed.hasSuffix("}") else { return [] }
        var parts: [String] = []
        var current = ""
        var depth = 0
        for character in trimmed.dropFirst().dropLast() {
            switch character {
            case "{", "[":
                depth += 1
                current.append(character)
            case "}", "]":
                depth -= 1
                current.append(character)
            case "," where depth == 0:
                parts.append(current)
                current = ""
            default:
                current.append(character)
            }
        }
        parts.append(current)
        return parts.compactMap { part in
            let name = part.split(separator: ":", maxSplits: 1, omittingEmptySubsequences: false).first.map { $0.trimmingCharacters(in: .whitespaces) } ?? ""
            return name.isEmpty ? nil : name
        }
    }

    /// `value` with YAML's double or single quotes removed and their
    /// escapes undone; anything else as it is.
    public static func unquoted(_ value: String) -> String {
        if value.count >= 2, value.hasPrefix("\""), value.hasSuffix("\"") {
            var result = ""
            var escaped = false
            for character in value.dropFirst().dropLast() {
                if escaped {
                    switch character {
                    case "n": result.append("\n")
                    case "t": result.append("\t")
                    default: result.append(character)
                    }
                    escaped = false
                } else if character == "\\" {
                    escaped = true
                } else {
                    result.append(character)
                }
            }
            return result
        }
        if value.count >= 2, value.hasPrefix("'"), value.hasSuffix("'") {
            return String(value.dropFirst().dropLast()).replacingOccurrences(of: "''", with: "'")
        }
        return value
    }
}
```

- [ ] **Step 4: Run the core tests**

Run: `make -C desktop core-test`
Expected: every test passes, the six new ones included. If `testReadsTheBlockAndItsEntries` fails on `text(of:)`, check the trailing-blank rule: the blank line between `path: ./incident.db` and `shell: {}` is inside `drivers` (it is followed by a child), and the entry's range still ends at `shell: {}`'s line.

- [ ] **Step 5: Mutate and commit**

Mutations, each applied and run with `make -C desktop core-test`, then reverted exactly: in `init`, drop the `lines.count > 1` condition on the closing check (expected: `testReadsTheBlockAndItsEntries` fails, the opener closes itself); in `parse`, use `next.indent < opener.indent` (expected: it fails on `drivers.children`, `recording` becomes a child of `drivers`); in `parse`, drop the trailing-blank trim (survives here: no gap follows an entry in this task's decks; Task 4's `testAddsAChildAtTheEndOfItsParentsBlock` kills it, since the new child would land after the blank line and the comment); in `declaredDrivers`, ignore the flow case (expected: `testTheDeclaredDriversComeFromTheDriversMap` fails on the flow map); in `flowMapKeys`, split on every comma (expected: the nested `connections` case yields a wrong key); in `keyLine`, keep a value that starts with `#` (expected: `testCommentsBlankLinesAndOddSpacingAreNotEntries` is unaffected; `testReadsTheBlockAndItsEntries` is unaffected too: add `commented: # nothing` to that test's deck if this mutation must be killed, expecting `value(at: ["commented"]) == nil`); in `init`, detect `"\r\n"` as `"\n"` (expected: `testKeepsCarriageReturnLineEndings` fails).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): read the deck's frontmatter as lines, with its declared drivers"
```

---

### Task 4: Frontmatter edits, and the deck schema

**Files:**
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/Frontmatter.swift`
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/DeckSchema.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/FrontmatterTests.swift`, `DeckSchemaTests.swift`

**Interfaces:**
- Consumes: Task 3's `Frontmatter`, D2's `TextReplacement(range:replacement:)`, `TapErrorPayload`.
- Produces: `Frontmatter.setting(path:to:) -> TextReplacement?` (nil value removes), `addingDriver(_:) -> TextReplacement?`, `rawBlock(at:) -> String?`, `settingRawBlock(at:to:) -> TextReplacement?`, `Frontmatter.scalar(forString:) -> String`, `Frontmatter.applying(_:to:) -> String` (a test convenience); `SchemaKey(name:type:defaultValue:values:description:keys:)` with `label`, `isScalar`; `DeckSchema.decode(_:) -> [SchemaKey]`, `DeckSchema.key(at:in:)`.

- [ ] **Step 1: Write the failing edit tests**

Add to `FrontmatterTests.swift`:

```swift
    /// `replacement` applied to `text`, for reading the result.
    func applied(_ replacement: TextReplacement?, to text: String) throws -> String {
        Frontmatter.applying(try XCTUnwrap(replacement), to: text)
    }

    func testSetsAScalarInPlace() throws {
        let text = "---\ntitle: Old\ntheme: base\n---\n\n# One\n"
        let result = try applied(Frontmatter(text: text).setting(path: ["theme"], to: "midnight"), to: text)
        XCTAssertEqual(result, "---\ntitle: Old\ntheme: midnight\n---\n\n# One\n")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["title"], to: "\"New: title\""), to: text),
                       "---\ntitle: \"New: title\"\ntheme: base\n---\n\n# One\n")
    }

    func testAddsAMissingKeyBeforeTheClosingLine() throws {
        let text = "---\ntitle: Old\n---\n# One\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["author"], to: "Me"), to: text), "---\ntitle: Old\nauthor: Me\n---\n# One\n")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["recording", "output"], to: "out"), to: text),
                       "---\ntitle: Old\nrecording:\n  output: out\n---\n# One\n", "a missing parent is made")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["drivers", "sqlite", "timeout"], to: "5"), to: text),
                       "---\ntitle: Old\ndrivers:\n  sqlite:\n    timeout: 5\n---\n# One\n")
    }

    func testAddsAChildAtTheEndOfItsParentsBlock() throws {
        let text = "---\nrecording:\n  output: out\n\n# a comment between\ntheme: base\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["recording", "audio"], to: "none"), to: text),
                       "---\nrecording:\n  output: out\n  audio: none\n\n# a comment between\ntheme: base\n---\n",
                       "right below the last child, before the gap")
        let deeper = "---\ndrivers:\n  sqlite:\n      timeout: 5\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: deeper).setting(path: ["drivers", "sqlite", "command"], to: "x"), to: deeper),
                       "---\ndrivers:\n  sqlite:\n      timeout: 5\n      command: x\n---\n", "the siblings' indent, whatever it is")
    }

    func testMakesTheFrontmatterWhenThereIsNone() throws {
        let text = "# One\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["title"], to: "T"), to: text), "---\ntitle: T\n---\n\n# One\n")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["drivers", "shell"], to: "{}"), to: text), "---\ndrivers:\n  shell: {}\n---\n\n# One\n")
        XCTAssertNil(Frontmatter(text: text).setting(path: ["title"], to: nil), "removing from nothing is nothing")
    }

    func testRemovesAKeyWithEverythingUnderIt() throws {
        let text = "---\ntitle: T\ndrivers:\n  sqlite:\n    timeout: 5\n  shell: {}\ntheme: base\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["drivers", "sqlite"], to: nil), to: text),
                       "---\ntitle: T\ndrivers:\n  shell: {}\ntheme: base\n---\n")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["drivers"], to: nil), to: text), "---\ntitle: T\ntheme: base\n---\n")
        XCTAssertNil(Frontmatter(text: text).setting(path: ["author"], to: nil))
    }

    func testABlockBecomesAScalarAndBack() throws {
        let text = "---\nrecording:\n  output: out\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["recording"], to: "{}"), to: text), "---\nrecording: {}\n---\n")
        let bare = "---\ndrivers:\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: bare).setting(path: ["drivers", "shell"], to: "{}"), to: bare), "---\ndrivers:\n  shell: {}\n---\n",
                       "a bare key opens into a block")
    }

    func testAFlowMapGainsAPair() throws {
        let empty = "---\ndrivers: {}\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: empty).setting(path: ["drivers", "shell"], to: "{}"), to: empty), "---\ndrivers:\n  shell: {}\n---\n",
                       "an empty flow map opens into a block")
        let full = "---\ndrivers: {shell: {}}\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: full).setting(path: ["drivers", "sqlite"], to: "{}"), to: full), "---\ndrivers: {shell: {}, sqlite: {}}\n---\n",
                       "a flow map with pairs keeps its shape")
        XCTAssertNil(Frontmatter(text: full).setting(path: ["drivers", "shell", "timeout"], to: "5"), "two levels into a flow map is not rewritten")
        XCTAssertNil(Frontmatter(text: full).setting(path: ["drivers", "shell"], to: "{timeout: 5}"), "a pair inside a flow map is not rewritten")
    }

    func testAddingADriver() throws {
        let text = "---\ntitle: T\ndrivers:\n  sqlite: {}\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).addingDriver("shell"), to: text), "---\ntitle: T\ndrivers:\n  sqlite: {}\n  shell: {}\n---\n")
        XCTAssertNil(Frontmatter(text: text).addingDriver("sqlite"), "already declared")
        let none = "---\ntitle: T\n---\n\n# One\n"
        XCTAssertEqual(try applied(Frontmatter(text: none).addingDriver("sqlite"), to: none), "---\ntitle: T\ndrivers:\n  sqlite: {}\n---\n\n# One\n")
        XCTAssertEqual(try applied(Frontmatter(text: "# One\n").addingDriver("shell"), to: "# One\n"), "---\ndrivers:\n  shell: {}\n---\n\n# One\n")
    }

    func testCommentsAndBlankLinesStayWhereTheyAre() throws {
        let text = "---\r\n# who\r\ntitle: T\r\n\r\ndrivers:\r\n  # the database\r\n  sqlite: {}\r\n---\r\n\r\n# One\r\n"
        let result = try applied(Frontmatter(text: text).addingDriver("shell"), to: text)
        XCTAssertEqual(result, "---\r\n# who\r\ntitle: T\r\n\r\ndrivers:\r\n  # the database\r\n  sqlite: {}\r\n  shell: {}\r\n---\r\n\r\n# One\r\n",
                       "the file's own line ending, and nothing else moved")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["author"], to: "Me"), to: text),
                       "---\r\n# who\r\ntitle: T\r\n\r\ndrivers:\r\n  # the database\r\n  sqlite: {}\r\nauthor: Me\r\n---\r\n\r\n# One\r\n")
    }

    func testRawBlocks() throws {
        let text = "---\ndrivers:\n  sqlite:\n    connections:\n      a:\n        path: x\n  shell: {}\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.rawBlock(at: ["drivers", "sqlite", "connections"]), "    connections:\n      a:\n        path: x\n")
        XCTAssertEqual(try applied(frontmatter.settingRawBlock(at: ["drivers", "sqlite", "connections"], to: "    connections:\n      b:\n        path: y\n"), to: text),
                       "---\ndrivers:\n  sqlite:\n    connections:\n      b:\n        path: y\n  shell: {}\n---\n")
        XCTAssertNil(frontmatter.rawBlock(at: ["drivers", "mysql"]))
    }

    func testScalarsAreQuotedOnlyWhenYAMLWouldReadThemOtherwise() {
        XCTAssertEqual(Frontmatter.scalar(forString: "Debugging Production at 3am"), "Debugging Production at 3am")
        XCTAssertEqual(Frontmatter.scalar(forString: "2026-01-25"), "2026-01-25")
        XCTAssertEqual(Frontmatter.scalar(forString: "true"), "\"true\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "42"), "\"42\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "3.5"), "\"3.5\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "a: b"), "\"a: b\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "a # b"), "\"a # b\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "#hash"), "\"#hash\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "- dash"), "\"- dash\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "say \"hi\""), "\"say \\\"hi\\\"\"")
        XCTAssertEqual(Frontmatter.scalar(forString: " padded"), "\" padded\"")
        XCTAssertEqual(Frontmatter.scalar(forString: ""), "\"\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "null"), "\"null\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "no"), "\"no\"")
        XCTAssertEqual(Frontmatter.unquoted(Frontmatter.scalar(forString: "round \\ trip: \"x\"")), "round \\ trip: \"x\"")
    }
```

`DeckSchemaTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class DeckSchemaTests: XCTestCase {
    let json = #"""
    {"ok":true,"keys":[
      {"name":"title","type":"string","default":null,"description":"The deck's title."},
      {"name":"theme","type":"string","default":"base","values":["base","midnight"],"description":"The built-in theme."},
      {"name":"slideNumbers","type":"boolean","default":true,"description":"Numbers."},
      {"name":"themeColors","type":"object","default":null,"description":"Colors.","keys":[{"name":"background","type":"string","default":null,"description":"bg"}]},
      {"name":"drivers","type":"map","default":null,"description":"Drivers.","keys":[
        {"name":"command","type":"string","default":null,"description":"cmd"},
        {"name":"args","type":"list","default":null,"description":"args"},
        {"name":"timeout","type":"integer","default":30,"description":"Seconds."},
        {"name":"connections","type":"map","default":null,"description":"conns","keys":[{"name":"password","type":"string","default":null,"description":"pw"}]}]}
    ]}
    """#

    func testDecodesTheSchema() throws {
        let keys = try DeckSchema.decode(Data(json.utf8))
        XCTAssertEqual(keys.map(\.name), ["title", "theme", "slideNumbers", "themeColors", "drivers"])
        XCTAssertEqual(keys[0], SchemaKey(name: "title", type: "string", defaultValue: nil, values: [], description: "The deck's title.", keys: []))
        XCTAssertEqual(keys[1].defaultValue, "base")
        XCTAssertEqual(keys[1].values, ["base", "midnight"])
        XCTAssertEqual(keys[2].defaultValue, "true")
        XCTAssertEqual(keys[3].keys.map(\.name), ["background"])
        XCTAssertEqual(keys[4].keys[2].defaultValue, "30")
        XCTAssertEqual(keys[4].keys[3].keys.map(\.name), ["password"])
        XCTAssertTrue(keys[0].isScalar)
        XCTAssertTrue(keys[4].keys[1].isScalar, "a list is edited as one line")
        XCTAssertFalse(keys[3].isScalar)
        XCTAssertFalse(keys[4].isScalar)
    }

    func testLabelsReadAsWords() {
        XCTAssertEqual(SchemaKey(name: "aspectRatio", type: "string").label, "Aspect ratio")
        XCTAssertEqual(SchemaKey(name: "title", type: "string").label, "Title")
        XCTAssertEqual(SchemaKey(name: "themeColors", type: "object").label, "Theme colors")
        XCTAssertEqual(SchemaKey(name: "codeBg", type: "string").label, "Code bg")
    }

    func testFindsAKeyByPathThroughMaps() throws {
        let keys = try DeckSchema.decode(Data(json.utf8))
        XCTAssertEqual(DeckSchema.key(at: ["theme"], in: keys)?.name, "theme")
        XCTAssertEqual(DeckSchema.key(at: ["themeColors", "background"], in: keys)?.name, "background")
        XCTAssertEqual(DeckSchema.key(at: ["drivers", "sqlite", "timeout"], in: keys)?.name, "timeout", "a map's entry name is any name")
        XCTAssertEqual(DeckSchema.key(at: ["drivers", "sqlite", "connections", "incident", "password"], in: keys)?.name, "password")
        XCTAssertNil(DeckSchema.key(at: ["drivers", "sqlite", "nope"], in: keys))
        XCTAssertNil(DeckSchema.key(at: [], in: keys))
    }

    func testRefusesAnErrorAnswer() {
        XCTAssertThrowsError(try DeckSchema.decode(Data(#"{"ok":false,"error":{"code":"internal","message":"no"}}"#.utf8)))
    }
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `make -C desktop core-test`
Expected: the package does not compile (`setting`, `DeckSchema` are undefined).

- [ ] **Step 3: Add the edits to `Frontmatter.swift`**

Inside `Frontmatter`, after `text(of:)`:

```swift
    /// The one edit that gives the key at `path` the raw scalar `value`
    /// (nil removes the key with everything under it), making the
    /// frontmatter, the key's parents and the key as needed: a missing
    /// top-level key goes before the closing line, a missing child right
    /// below its parent's last child at that child's indent (or two spaces
    /// deeper than the parent), a "key:" with nothing under it opens into
    /// a block, "key: {}" too, and a flow map with pairs gains one more.
    /// nil when the edit changes nothing (removing what is not there), or
    /// when it would need to rewrite inside a flow map, which this type
    /// does not do (the Deck tab then offers the raw text).
    public func setting(path: [String], to value: String?) -> TextReplacement? {
        guard let key = path.last else { return nil }
        guard range != nil, let closingLocation else {
            guard let value else { return nil }
            var lines = ["---"]
            for (depth, name) in path.dropLast().enumerated() { lines.append(Self.spaces(depth * 2) + name + ":") }
            lines.append(Self.spaces((path.count - 1) * 2) + key + ": " + value)
            lines.append("---")
            lines.append("")
            return TextReplacement(range: NSRange(location: 0, length: 0), replacement: lines.joined(separator: lineEnding) + lineEnding)
        }
        var parent: Entry?
        var siblings = entries
        var remaining = path[...]
        while let name = remaining.first, let found = siblings.first(where: { $0.key == name }) {
            if remaining.count == 1 {
                guard let value else { return TextReplacement(range: found.range, replacement: "") }
                if let valueRange = found.valueRange, found.children.isEmpty {
                    return TextReplacement(range: valueRange, replacement: value)
                }
                return TextReplacement(range: found.range, replacement: Self.spaces(found.indent) + key + ": " + value + lineEnding)
            }
            parent = found
            siblings = found.children
            remaining = remaining.dropFirst()
        }
        guard let value else { return nil }
        let baseIndent: Int
        let insertion: Int
        if let parent {
            if let flow = parent.value {
                let trimmed = flow.trimmingCharacters(in: .whitespaces)
                guard trimmed.hasPrefix("{"), trimmed.hasSuffix("}"), let valueRange = parent.valueRange, remaining.count == 1,
                      !value.trimmingCharacters(in: .whitespaces).hasPrefix("{") || value.trimmingCharacters(in: .whitespaces) == "{}" else { return nil }
                let inner = trimmed.dropFirst().dropLast().trimmingCharacters(in: .whitespaces)
                guard !inner.isEmpty else {
                    let block = Self.spaces(parent.indent) + parent.key + ":" + lineEnding + Self.spaces(parent.indent + 2) + key + ": " + value + lineEnding
                    return TextReplacement(range: parent.range, replacement: block)
                }
                return TextReplacement(range: valueRange, replacement: "{" + inner + ", " + key + ": " + value + "}")
            }
            baseIndent = parent.children.first?.indent ?? parent.indent + 2
            insertion = NSMaxRange(parent.range)
        } else {
            baseIndent = 0
            insertion = closingLocation
        }
        var lines: [String] = []
        let newParents = remaining.dropLast()
        for (depth, name) in newParents.enumerated() { lines.append(Self.spaces(baseIndent + depth * 2) + name + ":") }
        lines.append(Self.spaces(baseIndent + newParents.count * 2) + key + ": " + value)
        return TextReplacement(range: NSRange(location: insertion, length: 0), replacement: lines.joined(separator: lineEnding) + lineEnding)
    }

    /// The fix-it: "<name>: {}" under drivers. nil when it is declared already.
    public func addingDriver(_ name: String) -> TextReplacement? {
        guard !declares(driver: name) else { return nil }
        return setting(path: ["drivers", name], to: "{}")
    }

    /// An entry's lines as written, for the Deck tab's raw text field.
    public func rawBlock(at path: [String]) -> String? {
        entry(at: path).map(text(of:))
    }

    /// Replaces an entry's lines with `raw`, which the caller has already
    /// indented and terminated. nil when there is no such entry.
    public func settingRawBlock(at path: [String], to raw: String) -> TextReplacement? {
        entry(at: path).map { TextReplacement(range: $0.range, replacement: raw) }
    }

    /// `replacement` applied to `text`.
    public static func applying(_ replacement: TextReplacement, to text: String) -> String {
        (text as NSString).replacingCharacters(in: replacement.range, with: replacement.replacement)
    }

    private static func spaces(_ count: Int) -> String { String(repeating: " ", count: count) }

    private static let yamlWords: Set<String> = ["true", "false", "null", "~", "yes", "no", "on", "off"]
    private static let numberPattern = try! NSRegularExpression(pattern: #"^[-+]?(\d[\d_]*(\.\d*)?|\.\d+)([eE][-+]?\d+)?$"#)
    private static let unsafeLeading: Set<Character> = ["-", "?", ":", ",", "[", "]", "{", "}", "#", "&", "*", "!", "|", ">", "'", "\"", "%", "@", "`"]

    /// `string` as a YAML scalar: as it is when YAML reads it back as the
    /// same string, double-quoted with escapes otherwise (a number, a
    /// boolean or null in any case, a leading character YAML gives a
    /// meaning to, ": " or " #" inside, leading or trailing whitespace, a
    /// line break, or nothing at all).
    public static func scalar(forString string: String) -> String {
        let needsQuotes = string.isEmpty
            || string != string.trimmingCharacters(in: .whitespacesAndNewlines)
            || string.contains("\n")
            || yamlWords.contains(string.lowercased())
            || numberPattern.firstMatch(in: string, range: NSRange(location: 0, length: (string as NSString).length)) != nil
            || string.first.map { unsafeLeading.contains($0) } == true
            || string.contains(": ")
            || string.contains(" #")
            || string.hasSuffix(":")
        guard needsQuotes else { return string }
        var quoted = "\""
        for character in string {
            switch character {
            case "\\": quoted += "\\\\"
            case "\"": quoted += "\\\""
            case "\n": quoted += "\\n"
            case "\t": quoted += "\\t"
            default: quoted.append(character)
            }
        }
        return quoted + "\""
    }
```

- [ ] **Step 4: Write `DeckSchema.swift`**

```swift
import Foundation

/// One frontmatter key tap understands, from `tap deck schema --json`
/// (internal/config/schema.go). `type` is "string", "boolean", "integer",
/// "list", "object" (fixed nested `keys`) or "map" (entries named by the
/// deck, each with the nested `keys`). `defaultValue` is the default as
/// text, nil for none.
public struct SchemaKey: Equatable, Sendable {
    public let name: String
    public let type: String
    public let defaultValue: String?
    public let values: [String]
    public let description: String
    public let keys: [SchemaKey]

    public init(name: String, type: String, defaultValue: String? = nil, values: [String] = [], description: String = "", keys: [SchemaKey] = []) {
        self.name = name
        self.type = type
        self.defaultValue = defaultValue
        self.values = values
        self.description = description
        self.keys = keys
    }

    /// A key edited on one line: everything but an object or a map.
    public var isScalar: Bool { type != "object" && type != "map" }

    /// "aspectRatio" reads "Aspect ratio".
    public var label: String {
        var words = ""
        for (index, character) in name.enumerated() {
            if character.isUppercase, index > 0 { words += " " }
            words += index == 0 ? character.uppercased() : character.lowercased()
        }
        return words
    }
}

public enum DeckSchema {
    private struct Envelope: Decodable {
        let ok: Bool
        let keys: [Key]?
        let error: TapErrorPayload?
    }

    private struct Key: Decodable {
        let name: String
        let type: String
        let `default`: Default?
        let values: [String]?
        let description: String?
        let keys: [Key]?

        var schemaKey: SchemaKey {
            SchemaKey(name: name, type: type, defaultValue: `default`?.text, values: values ?? [],
                      description: description ?? "", keys: (keys ?? []).map(\.schemaKey))
        }
    }

    /// `default` is null, a string, a boolean or a number.
    private enum Default: Decodable {
        case text(String)

        var text: String {
            switch self {
            case .text(let value): return value
            }
        }

        init(from decoder: Decoder) throws {
            let container = try decoder.singleValueContainer()
            if let value = try? container.decode(Bool.self) { self = .text(value ? "true" : "false"); return }
            if let value = try? container.decode(Int.self) { self = .text(String(value)); return }
            if let value = try? container.decode(Double.self) { self = .text(String(value)); return }
            self = .text(try container.decode(String.self))
        }
    }

    public static func decode(_ data: Data) throws -> [SchemaKey] {
        let envelope = try JSONDecoder().decode(Envelope.self, from: data)
        guard envelope.ok, let keys = envelope.keys else {
            throw envelope.error ?? TapErrorPayload(code: "invalid_response", message: "tap printed no schema")
        }
        return keys.map(\.schemaKey)
    }

    /// The key a frontmatter path names: a map's entry name matches any
    /// name and continues into the map's nested keys.
    public static func key(at path: [String], in keys: [SchemaKey]) -> SchemaKey? {
        var siblings = keys
        var found: SchemaKey?
        var index = 0
        while index < path.count {
            guard let next = siblings.first(where: { $0.name == path[index] }) else { return nil }
            found = next
            siblings = next.keys
            index += 1
            if next.type == "map" {
                // The entry's own name, which the deck picks.
                guard index < path.count else { return found }
                index += 1
            }
        }
        return found
    }
}
```

A `Default` with a null JSON value decodes as absent, because `Key.default` is optional and `decodeIfPresent` treats `null` as nil.

- [ ] **Step 5: Run the core tests**

Run: `make -C desktop core-test`
Expected: every test passes. `testABlockBecomesAScalarAndBack`'s "bare key opens into a block" goes through the `parent.value == nil` path with `baseIndent = parent.indent + 2` and `insertion = NSMaxRange(parent.range)`, the end of the `drivers:` line.

- [ ] **Step 6: Mutate and commit**

Mutations, each applied and run with `make -C desktop core-test`, then reverted exactly, the ones that could lose or corrupt text first: in `setting`, for a removal return the value's range instead of the entry's (expected: `testRemovesAKeyWithEverythingUnderIt` fails, the children stay); in `setting`, insert a missing child at `parent.range.location` (expected: `testAddsAChildAtTheEndOfItsParentsBlock` fails); in `setting`, use `"\n"` instead of `lineEnding` (expected: `testCommentsAndBlankLinesStayWhereTheyAre` fails); in `setting`, drop the `remaining.count == 1` guard for flow maps (expected: `testAFlowMapGainsAPair` fails on the two-level case); in `scalar(forString:)`, drop the `yamlWords` check (expected: `testScalarsAreQuotedOnlyWhenYAMLWouldReadThemOtherwise` fails on "true"); in `scalar(forString:)`, drop the `\"` escape (expected: it fails on `say "hi"`); in `addingDriver`, drop the `declares` guard (expected: `testAddingADriver` fails on "already declared"); in `DeckSchema.key(at:in:)`, drop the map skip (expected: `testFindsAKeyByPathThroughMaps` fails on `timeout`); in `Default`, decode a Bool as `"yes"` (expected: `testDecodesTheSchema` fails on `"true"`); in `label`, drop the space (expected: `testLabelsReadAsWords` fails).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): one-line frontmatter edits, the fix-it's edit, and the deck schema"
```

---

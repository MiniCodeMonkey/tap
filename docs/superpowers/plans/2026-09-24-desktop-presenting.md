# Tap Desktop presenting (D4): implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Play and Rehearse run `tap present --app` as a second process beside the deck's `tap dev --app`, put tap's audience and presenter pages in native windows that cover the chosen displays, hold a display sleep assertion for exactly as long as the talk runs, show tap's recording consent and keep-recording questions as sheets on the deck window, and put the editor's cursor on the last slide presented when the talk ends.

**Architecture:** `TapDesktopCore` learns the rest of P6's protocol (the `recording`, `tunnel` and `slide` events, question payloads, the `answer`, `tunnel` and `recording` commands) and lets `TapSession` run `present` as well as `dev`, with a `quit` that asks tap to shut down cleanly instead of closing its stdin. A pure `DisplayArrangement` decides which screen is the audience and which the presenter, remembered per pair of displays. In the app target, one `PresentationController` per deck owns the present session, two borderless `PresentationWindow`s that each cover a screen (each holds a `WKWebView` with tap's page, full screen enabled, on the persistent data store), the `SleepAssertion`, the position from tap's `slide` events and the recording state from its `recording` events. The deck window gets a Play toolbar button with the Present popover, the Present menu, and the sheets. The presenter window gets the toolbar that slides in at the top edge (REC, Reload Slides, Swap Displays, Stop) and the REC dot. tap does all the presenting, recording and tunnelling; the app only opens windows, forwards answers, and edits text.

**Tech Stack:** Swift 6.3 compiler in Swift 5 language mode, AppKit (`NSWindow` borderless at a level above the menu bar, `NSPopover`, `NSWindow.beginSheet`, `NSTrackingArea`, `NSEvent.addLocalMonitorForEvents`), WebKit (`WKWebView`, `WKPreferences.isElementFullscreenEnabled`, `WKHTTPCookieStore`), IOKit (`IOPMAssertionCreateWithName`), CoreGraphics (`CGWindowListCopyWindowInfo` in tests, `CGDisplayIsBuiltin`), XCTest and XCUITest, XcodeGen, the bundled `tap` (`tap present --app [--no-record] [--presenter-password x] <deck>`).

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-design.md` (milestone 4; the sections "Processes", "The protocol between the app and tap", "Presenting", "Menus and accessibility" and "Testing"), `docs/superpowers/specs/2026-09-22-tap-desktop-prerequisites-design.md` part 6 as checked against `internal/cli` on `main` (see "P6 as built" below; the code wins), the D4 outline and "Decisions for the desktop app" in `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md`, and the feature files in `docs/superpowers/specs/tap-desktop-features/` (`05-presenting.feature` whole, plus "Presenting shortcuts" from `12-menus-and-shortcuts.feature`; `13-performance.feature` has no D4 scenario). The mockups are the "Tap Desktop Mockups" canvas (Present, Consent, FocusHint, PresenterWindow, PresenterIdle, Rehearse, PhoneRemote, KeepRecording). Where the mockups and the spec differ, the spec wins.

**Branch:** `feat/desktop-presenting`, branched from `main` after D3 (`feat/desktop-sidebar`, pull request 31) has merged, in a worktree at `/Users/codemonkey/projects/tap-d4`. If D3 has not merged when this plan starts, branch from `feat/desktop-sidebar` at d46a213 or later and rebase onto `main` once it has. One pull request. The starting code is D3's `desktop/`: `TapDesktopCore` (`TapProtocol`, `TapProcess`, `TapSession`, `TapClient`, `RestartPolicy`, `TapLog`), `DeckSessionController`, `DeckWindowController`, `MainMenu`, `AppEnvironment`, `PreviewViewController`, `WeakScriptMessageHandler`, `DocumentBarView`, `TapLogWindowController`, `HostedTestCase`, `Fixtures`, `FakeTapScripts`, `FakeTap`, `TestScripts`, `UITestCase`.

**Prerequisites on `main`, checked 2026-09-24 (`internal/cli`, `internal/server`, `frontend/src`):** `tap present --app` (`present.go`, P6) with the ready line, the events and the commands listed below; the audience page takes its start slide from the URL hash (`frontend/src/lib/stores/presentation.ts`, `#<1-based slide>`), and so does the presenter page (`keyboard.ts` copies the hash when the S key opens `/presenter`); the hub relays a client's `slide` messages only when its WebSocket upgrade carried the presenter cookie (`internal/server/websocket.go`, `checkPresenterAuth`), and emits a `slide` event on every relay (`dev.go`, `hub.SetOnSlideChange`); the presenter route accepts the cookie or `?key=` (`routes.go`, `handlePresenter`). No tap prerequisite task is needed.

## P6 as built, checked against `main` (the code wins)

The prerequisites document (part 6) and the code differ in these places. Every task below follows the code.

| Topic | The document says | The code does (`internal/cli`, `internal/server`) |
|---|---|---|
| The ready line | `{"type": "ready", "port", "token", "launch"}` | Also `"presenter"`: the presenter secret the app trades for the hub's presenter cookie (`app_events.go`, `appReadyEvent`; `dev.go` line 223). D2's `TapReady.presenter` and `TapClient.authorizePresenter()` already use it. |
| `--tunnel` for `tap present` | "the app passes them as `--presenter-password` and `--tunnel`" (05, Advanced remote options) | `present.go` has `--presenter-password`, `--no-record`, `--port`, `--lan`, `--allow-code` and `--app`, and **no `--tunnel`**. Only `tap dev` has `--tunnel`. The app starts the tunnel with the stdin command `{"type":"tunnel","start":true}` after the ready line (`app_session.go`, `tunnel(start)`). |
| Question payloads | `question` has `id`, `kind`, `payload` | `record-consent` carries `{"settingsPath": "..."}` (`app_questions.go`, `recordConsentPayload`); `keep-recording` carries `{"directory": "...", "segments": n}` (`app_session.go`, `keepRecordingPayload`); `approval` carries the approval request (D5). |
| Answer values | `{"type": "answer", "id", "value": true}` | `value` must be the JSON literal `true` or `false`; anything else is an `invalid_answer` error event, an unknown id is `unknown_question` (`app_questions.go`, `answer`). |
| The keep-recording question | "asks `keep-recording` first when a run has a recording" | Only when the run recorded and left slide 1 (`present.Started() && present.LeftFirstSlide()`), and tap waits **3 seconds** for the answer (`appKeepRecordingTimeout`), then keeps the recording and exits. Silence keeps. Closing stdin also keeps, with no question. See open question 1. |
| Recording events | `state` is `recording`, `paused`, `stopped`; `segment`, `elapsed`, `disk` | As documented. `elapsed` is the current segment's whole seconds; an event goes out only when the state, segment or disk changes, plus one at start, so the app counts seconds up itself between events (`app_recording.go`). `disk` is `ok`, `low` or `full`. A blocked recording (no Screen Recording permission, for example) is an `error` event with code `recording_blocked`, and the state stays `stopped`. |
| Tunnel events | `state`, `url`, `qr` (PNG, base64) | `state` is `starting`, then `running` with `url` and `qr` (a 512 px PNG), or `stopped`. No cloudflared is an `error` event with code `tunnel_unavailable` and the install hint as the message; a failed start is `tunnel_failed` (`app_session.go`, `tunnel`). |
| The `slide` event | `slide`, `step` | `slide` is 1-based (the hub's index plus one), `step` as the hub has it (`app_session.go`, `slideReporter`). It fires only for a relayed slide message, which needs the sending page to hold the presenter cookie. |
| Error codes | "a fatal or reportable error" | The reportable codes are in `app_events.go`: `invalid_command`, `unknown_command`, `unknown_question`, `invalid_answer`, `busy`, `not_presenting`, `not_editing`, `reload_failed`, `tunnel_unavailable`, `tunnel_failed`, `recording_failed`, `recording_blocked`, `command_stuck`, `startup_stuck`, `reporter_stuck`, `shutdown_stuck`. |
| `saved` in present mode | "The app saved the buffer to disk" | `tap present --app` answers `saved` with a `not_editing` error: it has no buffer and reads the file on `reload`. The app sends `reload` after saving, never `saved`, to the present process. |
| Startup order | not stated | After the ready line, tap asks `record-consent` (unless `--no-record`, or `present.record` is already set in `settings.yaml`), then the live code `approval` question when the deck declares drivers (D5 answers it; this plan declines it with a log line), then broadcasts `reload` to the pages. The questions arrive within milliseconds of the ready line. |
| Settings file | `~/.config/tap/settings.yaml` | `$XDG_CONFIG_HOME/tap/settings.yaml` when `XDG_CONFIG_HOME` is set (`usersettings.Path()`), which `HostedTestCase` sets to a fresh folder per test. The consent lives at `present.record`. |
| Pages and the token | "the page gets it as a cookie in exchange for a one-time launch code" | `/`, `/presenter`, `/api/presentation`, `/assets/`, `/components/`, `/local/` and `/ws` need no app token at all (`app_auth.go`, `audienceRoutes`). The launch code is still spent on the audience page's first load, as D2 does. What the presenter page and the hub need is the **presenter cookie** (`tap_presenter_key`), and cookies ignore ports, so the app sets the present process's cookie into the shared `WKWebsiteDataStore` before loading the pages. The launch code lives two minutes (`LaunchCodeLifetime`). |

## Global Constraints

- Everything in D2's and D3's Global Constraints still holds: macOS 14 or later, AppKit core, ad-hoc signing, the bundled `tap`, P6's protocol exactly as built, spelled-out identifiers, present-tense comments, no em dashes anywhere (`--`, a comma or a new sentence instead), `make frontend` before the first Xcode build, never modify the prototype repository.
- **The app never injects script into a page.** No `evaluateJavaScript` or `callAsyncJavaScript` in production code. `PreviewViewController.pageText` and `pageValue` stay test-only, and this plan adds one more test-only surface, `PresentationPageController.pageText()`, documented the same way. The app drives a talk's pages through the URL hash they load with and through tap's hub; it learns about them through the `tapReady` handler and tap's stdout events.
- **Sheets, never modal alerts.** Every question tap asks (`record-consent`, `keep-recording`) and the Focus hint are sheets on the deck window (`NSWindow.beginSheet`), never `NSAlert.runModal` and never app-modal. A refused action does nothing or beeps.
- **No production code steals focus, except where presenting must.** The three places, each in answer to the person's own click in this app: `PresentationController.showWindows` orders the audience and presenter windows front (`orderFrontRegardless`, and `makeKeyAndOrderFront` on the window the speaker's keys go to), because covering the projector is what Play means; `PresentationController.toggleFrontWindow` and `bringPresenterWindowForward` do the same for Option-Tab and the S key; and `DeckWindowController.presentQuestion` brings the deck window forward while a sheet is on it, because the sheet is the one thing the person must answer. No `NSApp.activate` anywhere. (D2 ledger, Task 21.)
- **The sleep assertion is held for exactly the talk.** `SleepAssertion.acquire` runs once tap present is ready and the windows exist; `release` runs in `takeDownWindows`, which every ending goes through: Stop, a failed start, tap present giving up, the deck window closing (`DeckSessionController.stop`) and the app quitting (`AppDelegate.applicationWillTerminate`). Each has a test that reaches it.
- **`tap present` is a second process.** The deck's `tap dev --app` keeps running the preview and the thumbnails through the whole talk; nothing in this plan stops, restarts or talks to it differently. The present process gets its own `TapSession`, its own `TapLog` (listed in Window > Tap Log as "<deck>, talk") and its own restart policy.
- **The edited flag is derived from content.** `refreshEditedState` stays the only caller of `updateChangeCount`. Play and Rehearse save through `NSDocument.save(to:ofType:for:completionHandler:)`, which already reports back to the session controller.
- `weak self` in every closure that outlives a call, no `unowned`. Never put work with side effects inside `completion?(...)`: an optional call skips its arguments when the closure is nil (D3 ledger lesson). Every completion in this plan is non-optional or the work sits outside the call.
- **XCTest rules.** No `await` inside an `XCTAssert` autoclosure: hoist the value into a `let`. No test depends on a key window: tests drive actions and seams directly (`windowController.playClicked(modifiers:)`, `presentation.handleKey(_:)`, `toolbar.pointerReachedTopEdge()`), set the first responder themselves, and never read `NSApp.keyWindow`. A window check filters on `isVisible` or reads the window server (`CGWindowListCopyWindowInfo` with `kCGWindowIsOnscreen`). Hosted tests run locally one at a time (`make -C desktop test ONLY=TapTests/<Class>/<test>`), the bundle runs on CI. `make -C desktop uitest` is the person's; agents compile it with `build-for-testing`. `make -C desktop bench` gains nothing in D4 (no 13-performance scenario is D4's); the person's bench run is unchanged.
- **One display is what the tests have.** CI and most local runs have a single screen. Every hosted test either uses that one screen as both displays, or hands the controller two "screens" that are the left and right halves of the real screen (`PresentingTestCase.halfScreens()`), so the arrangement, swap and memory logic runs against real windows the window server can see. A real second display, the WebKit element full screen the page's F key asks for, real screen recording and a real Cloudflare tunnel are the person's manual pass (Task 14's README list) and the UI tests; no automated test records the person's screen or starts a tunnel.
- **Mutation testing is how this branch finds tests that cannot fail.** Every task lists the mutations its review runs, the ones that can leave a talk stuck, a window covering a screen, or the sleep assertion held first. A test that survives its mutation is not done.
- Every scenario this plan claims has a test named `test` plus the scenario name in UpperCamelCase: "Start presenting with two displays" is `testStartPresentingWithTwoDisplays`. The claims go into `desktop/scenarios.txt` as `D4 | <file> | <scenario>` rows, and `make -C desktop check-scenarios` must pass.
- The fixture for presenting tests is D3's `desktop/TapTests/Fixtures/ops.md`: seven titled slides (One to Seven), no drivers, so `tap present --app` asks no live code approval question. `seven-slides.md` declares a `sqlite` driver and would, which is D5's business.

## Review Focus

Five conditions the spec implies that no scenario names, most likely to bite first. Each has its test pinned to the task that owns the code.

1. **Play while the deck has a disk conflict, or no file.** The save is refused (`DeckDocument.save(to:...)` answers `.userCancelled` during a conflict) and a deleted deck has no path for `tap present` to read. The talk must not start, no window may open, and the deck window says why. Task 13, `testATalkThatCannotBeSavedDoesNotStart`; Task 13, `testPlayIsDisabledWhileADeckIsPresentingOrHasNoFile`.
2. **The deck window closes mid-talk.** The windows must go, tap present must exit and the sleep assertion must be released, with nothing left covering a screen. Task 4, `testTheSleepAssertionIsReleasedWhenTheDeckWindowCloses`.
3. **The projector is unplugged mid-talk.** The audience window's screen is gone; it must land on the remaining screen behind the presenter window rather than stay off screen, and Swap must keep working. Task 5, `testTheAudienceWindowFallsBackWhenTheProjectorGoes`.
4. **Stop before tap present is ready.** The person clicks Play and Stop at once, or tap is slow. No window may open later, the process must be quit, and the state must return to idle. Task 4, `testStopWhileStartingOpensNoWindow`.
5. **tap present dies mid-talk.** The audience must not be left on a dead page: the app restarts tap (D2's policy), reloads both pages at the last slide, keeps the assertion, and if tap keeps dying, ends the talk and says so. Task 13, `testATalkSurvivesATapPresentRestart` and `testATalkEndsWhenTapPresentKeepsDying`.

## Scenarios this plan claims

| Feature file | Scenario | Test | Task |
|---|---|---|---|
| 05-presenting | Start presenting with two displays | `testStartPresentingWithTwoDisplays` | 5 |
| 05-presenting | Save before presenting | `testSaveBeforePresenting` | 4 |
| 05-presenting | Nothing interrupts the talk | `testNothingInterruptsTheTalk` | 12 |
| 05-presenting | Start from the first slide | `testStartFromTheFirstSlide` | 6 |
| 05-presenting | One display | `testOneDisplay` | 7 |
| 05-presenting | Rehearse | `testRehearse` | 5 |
| 05-presenting | Swap displays | `testSwapDisplays` | 5 |
| 05-presenting | Every tap dev presenter feature works | `testEveryTapDevPresenterFeatureWorks` | 7 |
| 05-presenting | Stop presenting | `testStopPresenting` | 4 |
| 05-presenting | Presenter controls | `testPresenterControls` | 8 |
| 05-presenting | The Mac stays awake | `testTheMacStaysAwake` | 4 (assertion), 8 (cursor) |
| 05-presenting | Phone remote | `testPhoneRemote` | 11 |
| 05-presenting | Advanced remote options | `testAdvancedRemoteOptions` | 11 |
| 05-presenting | First talk asks about recording | `testFirstTalkAsksAboutRecording` | 9 |
| 05-presenting | Recording follows tap present | `testRecordingFollowsTapPresent` | 9 |
| 05-presenting | Keep the recording | `testKeepTheRecording` | 10 |
| 05-presenting | Edit while presenting | `testEditWhilePresenting` | 8 |
| 05-presenting | Remember the display assignment | `testRememberTheDisplayAssignment` | 5 |
| 12-menus-and-shortcuts | Presenting shortcuts | `testPresentingShortcuts` | 7 |

19 scenarios. Every scenario in `05-presenting.feature` is claimed. Left for later milestones: "Deck settings live in the inspector" (D5), "Open a folder" (D6). `13-performance.feature` has no D4 scenario.

## Process lifetime: `tap present --app` beside `tap dev --app`

The whole plan hangs on this state machine, in `PresentationController` (Task 4):

| Step | What happens | Where |
|---|---|---|
| Play or Rehearse | `state = .starting`. The buffer is saved to the deck file if it differs from it (`DeckSessionController.saveForPresenting`), because `tap present` reads the file. A refused save (disk conflict) or a deck with no file ends here as `.failed(message)` with a bar on the deck window; no window opens. | Task 4, Task 13 |
| Start | A new `TapSession(deckURL:configuration:command: .present(record:presenterPassword:))` starts `tap present --app [--no-record] [--presenter-password x] <deck>` with the login shell environment. Its `TapLog` is "<deck>, talk". The deck's `tap dev` session is untouched. | Task 1, Task 4 |
| Ready | The ready line arrives (D2's 20 s `readyTimeout` still kills a silent tap). The app trades `ready.presenter` for the hub's presenter cookie (`TapClient.authorizePresenter`), sets that cookie into the shared `WKWebsiteDataStore`, acquires the sleep assertion, creates the windows on the arranged screens and loads `/?launch=<code>#<startSlide>` in the audience window and `/presenter#<startSlide>` in the presenter window (Rehearse: the presenter window only). If wanted, it sends `{"type":"tunnel","start":true}`. | Task 4, Task 11 |
| Shown | The windows are ordered front when the first page reports `tapReady`, or after 3 s if no page ever does (a fake tap with no server, a page that cannot load), and only once no question is pending. `state = .presenting`. Until then, a `record-consent` question (which arrives within milliseconds of ready on the first talk) shows its sheet on the deck window with nothing covering it. | Task 4, Task 9 |
| Presenting | tap's `slide` events keep `lastSlide`; `recording` events keep the toolbar's REC state; `tunnel` events drive the phone remote panel; `question` events become sheets (the presentation windows on the deck window's screen are hidden while a sheet is up, and come back after the answer). Typing in the editor goes to `tap dev` only; the toolbar counts the edits `tap present` has not read. Reload Slides saves the file and sends `{"type":"reload"}`. | Tasks 8, 9, 10, 11 |
| tap present exits unexpectedly | D2's `RestartPolicy`: `.restarting` keeps the windows (the audience keeps the last render), the next ready reloads both pages at `lastSlide` on the new port, the assertion stays held. At the third exit in 30 s the session is `.failed`: the windows close, the assertion is released, `state = .failed(message)` and the deck window shows "The talk stopped" with Show Tap Log. tap's recording, if any, is finished by tap's own exit path. | Task 13 |
| Stop | Escape in the audience window, Present > Stop, the toolbar's Stop, or the deck window closing: `state = .stopping`, the windows close and the assertion is released at once, then `session.quit()` sends `{"type":"quit"}`. tap may answer with a `keep-recording` question (sheet on the deck window; tap waits 3 s, then keeps). When the process exits, `state = .idle` and the editor cursor moves to `lastSlide`. A tap that ignores `quit` for 15 s (2 s if it never got ready) gets its stdin closed, then SIGTERM, then SIGKILL (D2's `TapProcess.stop`). | Task 4, Task 10 |
| App quit | `AppDelegate.applicationWillTerminate` stops every deck's presentation: windows down, assertion released, `quit` sent. The app does not wait for keep-recording on quit; tap keeps the recording (its rule for a closed stdin). | Task 4 |
| Play or Rehearse while tap dev is down | Allowed. The present process is independent; the preview overlay keeps showing tap dev's state. Play while a talk is running (this deck or another) is disabled; Stop, Reload Slides, Swap Displays and Phone Remote are enabled only while this deck presents. | Task 7, Task 12, Task 13 |

## File structure

| Path | Responsibility |
|---|---|
| `desktop/TapDesktopCore/Sources/TapDesktopCore/TapProtocol.swift` | Modify: `QuestionPayload`, `RecordingEvent`, `TunnelEvent`; `TapEvent.question(id:kind:payload:)`, `.recording`, `.tunnel`, `.slide`; `TapCommand.answer`, `.tunnel`, `.recording`; `RecordingAction` |
| `.../TapDesktopCore/TapSession.swift` | Modify: `Command` (`.dev`, `.present`), `quit(timeout:)`, the log title and line for a talk |
| `.../TapDesktopCore/TapClient.swift` | Modify: `audienceLaunchURL(slide:)`, `presenterURL(slide:)` |
| `.../TapDesktopCore/Presenting.swift` | `ScreenInfo`, `DisplayAssignmentStore`, `DisplayArrangement`, `PresentationMode`, `PresentationOptions`, `RecordingStatus`, `FocusHintState` |
| `desktop/Tap/Presenting/SleepAssertion.swift` | The IOKit display sleep assertion, acquired and released by the controller |
| `desktop/Tap/Presenting/PresentationPageController.swift` | One of tap's pages in a `WKWebView`: full screen enabled, persistent store, the `tapReady` handler, navigation policy, the S key's popup |
| `desktop/Tap/Presenting/PresentationWindow.swift` | A borderless window that covers one screen, above the menu bar; the presenter one also holds the toolbar and the REC dot |
| `desktop/Tap/Presenting/PresentationController.swift` | The talk: the present session, the windows, the arrangement, the assertion, `lastSlide`, questions, recording, the tunnel, the key monitor, the edits counter |
| `desktop/Tap/Presenting/PresentPopoverController.swift` | The Present popover: display arrangement, Swap Displays, start from, record, phone remote, Advanced, Rehearse, Start Presenting |
| `desktop/Tap/Presenting/DisplayArrangementView.swift` | The two labelled screen boxes the popover draws |
| `desktop/Tap/Presenting/PresenterToolbar.swift` | REC, edits label, Reload Slides, Swap Displays, Stop; slides in at the top edge; the REC dot |
| `desktop/Tap/Presenting/QuestionSheet.swift` | The sheet for consent, keep-recording and the Focus hint |
| `desktop/Tap/Presenting/RemotePanel.swift` | The phone remote panel with tap's QR code and URL |
| `desktop/Tap/Preview/WeakScriptMessageHandler.swift` | Unchanged; shared with the presentation pages |
| `desktop/Tap/Windows/DeckWindowController.swift` | Modify: the Play toolbar item, `play`, `rehearse`, `stopPresenting`, `reloadSlides`, `swapDisplays`, `togglePhoneRemote`, the sheets, validation, the failure bar |
| `desktop/Tap/Documents/DeckSessionController.swift` | Modify: `presentation`, `saveForPresenting`, the stop path, the edits counter hook |
| `desktop/Tap/Documents/DocumentBar.swift` | Modify: the `talkFailed` bar kind |
| `desktop/Tap/App/MainMenu.swift` | Modify: the Present menu |
| `desktop/Tap/App/AppDelegate.swift` | Modify: `deck(owning:)` for presentation windows, `applicationWillTerminate`, `stopAllPresentations` |
| `desktop/Tap/App/AppEnvironment.swift` | Modify: `displayAssignments`, `presentExecutableURL`, `presentSessionConfiguration()`, `focusHint`, `presentingCount`, `isPresenting`, `updatesMayInterrupt` |
| `desktop/Tap/TapLog/TapLogWindowController.swift` | Modify: lists the talk's log beside the deck's |
| `desktop/TapTests/Support/PresentingTestCase.swift` | The consent file, the half screens, the window server order, start and stop helpers, the sleep assertion check |
| `desktop/TapTests/Support/FakeTapScripts.swift` | Modify: `presenting(...)`, a scripted `tap present --app` |
| `desktop/TapTests/Support/HostedTestCase.swift` | Modify: fresh `displayAssignments`, `focusHint`, `presentExecutableURL` per test |
| `desktop/TapTests/*.swift` | The hosted tests: `PresentingTests`, `PresentingDisplayTests`, `PresentPopoverTests`, `PresentMenuTests`, `PresenterToolbarTests`, `RecordingTests`, `KeepRecordingTests`, `PhoneRemoteTests`, `FocusHintTests`, `PresentingFailureTests` |
| `desktop/TapUITests/PresentingUITests.swift` | Play through the popover, Escape stops; Rehearse by shortcut; local only |
| `desktop/scenarios.txt`, `desktop/README.md` | Modify: the 19 D4 rows; the presenting tests and the manual pass |

---

### Task 1: The rest of P6's protocol, and a session that runs `present` and quits cleanly

**Files:**
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/TapProtocol.swift`
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/TapSession.swift`
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/TapClient.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/TapProtocolTests.swift`, `TapSessionTests.swift`, `TapClientTests.swift`

**Interfaces:**
- Consumes: D2's `TapEvent`, `TapCommand`, `TapReady`, `TapSession`, `TapProcess.send(_:)`, `TapProcess.stop(graceSeconds:)`, `RestartPolicy`, `FakeTap.ready(recordingTo:)`, `TestScripts.make(_:)`, `waitUntil`.
- Produces: `QuestionPayload(deck:settingsPath:directory:segments:)`; `RecordingEvent(state:segment:elapsed:disk:)`; `TunnelEvent(state:url:qr:)`; `TapEvent.question(id:kind:payload:)`, `.recording(RecordingEvent)`, `.tunnel(TunnelEvent)`, `.slide(slide:step:)`; `RecordingAction` (`.newSegment`, `.stop`); `TapCommand.answer(id:value:)`, `.tunnel(start:)`, `.recording(action:)`; `TapSession.Command` (`.dev`, `.present(record:presenterPassword:)`) with `arguments(deck:)`, `logLine(deck:)`, `logTitle(deck:)`; `TapSession.init(deckURL:configuration:command:)`, `let command`, `quit(timeout:)`, `private(set) var quitRequested`; `TapClient.audienceLaunchURL(slide:)`, `presenterURL(slide:)`.

- [ ] **Step 1: Write the failing protocol tests**

In `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/TapProtocolTests.swift`, replace `testDecodesQuestionsErrorsAndOtherEvents` and `testEncodesCommandsAsJSONLines` with these, and add the third:

```swift
    func testDecodesQuestionsErrorsAndOtherEvents() {
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"question","id":"q1","kind":"approval","payload":{"deck":"/a.md"}}"#),
                       .question(id: "q1", kind: "approval", payload: QuestionPayload(deck: "/a.md")))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"question","id":"q2","kind":"record-consent","payload":{"settingsPath":"/c/tap/settings.yaml"}}"#),
                       .question(id: "q2", kind: "record-consent", payload: QuestionPayload(settingsPath: "/c/tap/settings.yaml")))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"question","id":"q3","kind":"keep-recording","payload":{"directory":"/r/talk-1","segments":2}}"#),
                       .question(id: "q3", kind: "keep-recording", payload: QuestionPayload(directory: "/r/talk-1", segments: 2)))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"question","id":"q4","kind":"approval"}"#),
                       .question(id: "q4", kind: "approval", payload: QuestionPayload()), "a missing payload decodes as an empty one")
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"error","code":"deck_not_found","message":"deck not found: a.md"}"#),
                       .error(TapErrorPayload(code: "deck_not_found", message: "deck not found: a.md")))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"connected"}"#), .other(type: "connected"))
    }

    func testDecodesTheTalkEvents() {
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"recording","state":"recording","segment":2,"elapsed":75,"disk":"ok"}"#),
                       .recording(RecordingEvent(state: "recording", segment: 2, elapsed: 75, disk: "ok")))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"tunnel","state":"running","url":"https://x.trycloudflare.com","qr":"aGk="}"#),
                       .tunnel(TunnelEvent(state: "running", url: "https://x.trycloudflare.com", qr: "aGk=")))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"tunnel","state":"stopped"}"#), .tunnel(TunnelEvent(state: "stopped", url: nil, qr: nil)))
        XCTAssertEqual(TapEvent.decode(line: #"{"type":"slide","slide":3,"step":1}"#), .slide(slide: 3, step: 1))
        XCTAssertNil(TapEvent.decode(line: #"{"type":"slide","step":1}"#), "a slide event without its slide is not an event")
    }

    func testEncodesCommandsAsJSONLines() {
        XCTAssertEqual(TapCommand.saved.line, #"{"type":"saved"}"#)
        XCTAssertEqual(TapCommand.reload.line, #"{"type":"reload"}"#)
        XCTAssertEqual(TapCommand.quit.line, #"{"type":"quit"}"#)
        XCTAssertEqual(TapCommand.answer(id: "q1", value: true).line, #"{"type":"answer","id":"q1","value":true}"#)
        XCTAssertEqual(TapCommand.answer(id: "q\"2", value: false).line, #"{"type":"answer","id":"q\"2","value":false}"#, "the id is JSON-escaped")
        XCTAssertEqual(TapCommand.tunnel(start: true).line, #"{"type":"tunnel","start":true}"#)
        XCTAssertEqual(TapCommand.tunnel(start: false).line, #"{"type":"tunnel","start":false}"#)
        XCTAssertEqual(TapCommand.recording(action: .newSegment).line, #"{"type":"recording","action":"new-segment"}"#)
        XCTAssertEqual(TapCommand.recording(action: .stop).line, #"{"type":"recording","action":"stop"}"#)
    }
```

- [ ] **Step 2: Write the failing session and client tests**

Add to `TapSessionTests.swift`:

```swift
    func testATalkRunsTapPresentWithItsFlagsAndHasItsOwnLogTitle() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = TapSession(deckURL: deckURL, configuration: TapSession.Configuration(
            executableURL: try FakeTap.ready(recordingTo: record), environment: { ["PATH": "/usr/bin:/bin"] }),
            command: .present(record: false, presenterPassword: "secret"))
        XCTAssertEqual(tap.command.arguments(deck: deckURL), ["present", "--app", "--no-record", "--presenter-password", "secret", deckURL.path])
        XCTAssertEqual(TapSession.Command.present(record: true, presenterPassword: nil).arguments(deck: deckURL), ["present", "--app", deckURL.path])
        XCTAssertEqual(TapSession.Command.present(record: true, presenterPassword: "").arguments(deck: deckURL), ["present", "--app", deckURL.path], "an empty password is no password")
        XCTAssertEqual(TapSession.Command.dev.arguments(deck: deckURL), ["dev", "--app", deckURL.path])
        XCTAssertEqual(tap.log.title, "talk, talk")
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertTrue(recorded.contains("arguments: present --app --no-record --presenter-password secret \(deckURL.path)"))
        XCTAssertTrue(tap.log.text.contains("tap present --app --no-record --presenter-password secret talk.md"))
        tap.stop()
        try await waitUntil { tap.state == .stopped }
    }

    func testQuitSendsTheQuitCommandAndNeverRestarts() async throws {
        // A tap that exits on quit, as tap present does once its recording is finished.
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let script = try TestScripts.make("""
        echo '{"type":"ready","port":4242,"token":"token","launch":"launch","presenter":"p"}'
        while IFS= read -r line; do
          echo "stdin: $line" >> "\(record.path)"
          case "$line" in *'"type":"quit"'*) exit 0 ;; esac
        done
        exit 0
        """)
        let tap = session(script)
        var states: [TapSession.State] = []
        tap.onStateChange = { states.append($0) }
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        tap.quit()
        XCTAssertTrue(tap.quitRequested)
        try await waitUntil { tap.state == .stopped }
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertTrue(recorded.contains(#"stdin: {"type":"quit"}"#))
        XCTAssertFalse(states.contains { if case .restarting = $0 { return true } else { return false } }, "an exit after quit is not a crash")
        XCTAssertTrue(tap.log.text.contains("tap quit"))
    }

    func testQuitClosesStdinWhenTapIgnoresTheCommand() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = session(try FakeTap.ready(recordingTo: record))
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        // FakeTap.ready reads stdin until it closes and never acts on quit.
        tap.quit(timeout: 0.3)
        try await waitUntil(timeout: 5) { tap.state == .stopped }
        XCTAssertTrue(tap.log.text.contains("did not quit within 0 seconds"))
    }

    func testQuitBeforeTheProcessExistsStopsAtOnce() {
        let tap = session(URL(fileURLWithPath: "/usr/bin/false"))
        tap.quit()
        XCTAssertEqual(tap.state, .stopped)
    }
```

Add to `TapClientTests.swift` (it already has a `stubbedClient()` helper from D2; use its `ready`):

```swift
    func testTheTalkPagesCarryTheStartSlideInTheirHash() {
        let client = TapClient(ready: TapReady(port: 4242, token: "t", launch: "launch-code", presenter: "p"))
        XCTAssertEqual(client.audienceLaunchURL(slide: 3).absoluteString, "http://127.0.0.1:4242/?launch=launch-code#3")
        XCTAssertEqual(client.presenterURL(slide: 3).absoluteString, "http://127.0.0.1:4242/presenter#3")
        XCTAssertEqual(client.audienceLaunchURL(slide: 1).fragment, "1")
    }
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `make -C desktop core-test`
Expected: the package does not compile (`QuestionPayload`, `.recording`, `TapSession.Command`, `quit`, `audienceLaunchURL` are undefined). That is the failure for this step.

- [ ] **Step 4: Extend `TapProtocol.swift`**

Add these types before `TapEvent`:

```swift
/// What a `question` event carries. Each kind uses a few of the fields:
/// `approval` the deck (and its drivers, which D5 reads), `record-consent`
/// the settings file the answer is saved to, `keep-recording` the run's
/// folder and how many segments it has. See internal/cli/app_questions.go
/// and app_session.go.
public struct QuestionPayload: Codable, Equatable, Sendable {
    public let deck: String?
    public let settingsPath: String?
    public let directory: String?
    public let segments: Int?

    public init(deck: String? = nil, settingsPath: String? = nil, directory: String? = nil, segments: Int? = nil) {
        self.deck = deck
        self.settingsPath = settingsPath
        self.directory = directory
        self.segments = segments
    }
}

/// The recording state of a tap present run, as internal/cli/app_recording.go
/// reports it: `state` is "recording", "paused" or "stopped", `segment` the
/// current or last segment from 1 (0 before the first), `elapsed` the
/// current segment's whole seconds, `disk` "ok", "low" or "full".
public struct RecordingEvent: Equatable, Sendable {
    public let state: String
    public let segment: Int
    public let elapsed: Int
    public let disk: String

    public init(state: String, segment: Int, elapsed: Int, disk: String) {
        self.state = state
        self.segment = segment
        self.elapsed = elapsed
        self.disk = disk
    }
}

/// The tunnel state: "starting", "running" with the public URL and a QR
/// code of the presenter view (PNG, base64), or "stopped".
public struct TunnelEvent: Equatable, Sendable {
    public let state: String
    public let url: String?
    public let qr: String?

    public init(state: String, url: String?, qr: String?) {
        self.state = state
        self.url = url
        self.qr = qr
    }
}
```

Replace the `TapEvent` enum's cases, its `Envelope` and its `decode` with:

```swift
/// One JSON line from tap's standard output.
public enum TapEvent: Equatable, Sendable {
    case ready(TapReady)
    case fileChanged(path: String, slideList: SlideList?)
    case question(id: String, kind: String, payload: QuestionPayload)
    case recording(RecordingEvent)
    case tunnel(TunnelEvent)
    /// The audience position: a 1-based slide and its step.
    case slide(slide: Int, step: Int)
    case error(TapErrorPayload)
    case other(type: String)

    private struct Envelope: Decodable {
        let type: String
        let port: Int?
        let token: String?
        let launch: String?
        let presenter: String?
        let path: String?
        let slides: [Slide]?
        let errors: [String]?
        let id: String?
        let kind: String?
        let payload: QuestionPayload?
        let state: String?
        let segment: Int?
        let elapsed: Int?
        let disk: String?
        let url: String?
        let qr: String?
        let slide: Int?
        let step: Int?
        let code: String?
        let message: String?
    }

    /// Returns nil for a line that is not a JSON object with a known shape.
    public static func decode(line: String) -> TapEvent? {
        guard let envelope = try? JSONDecoder().decode(Envelope.self, from: Data(line.utf8)) else { return nil }
        switch envelope.type {
        case "ready":
            guard let port = envelope.port, let token = envelope.token, let launch = envelope.launch else { return nil }
            // presenter is missing only from a tap older than app mode's
            // presenter secret. TapClient refuses to authorize on an empty
            // one rather than opening a socket that cannot drive the deck.
            return .ready(TapReady(port: port, token: token, launch: launch, presenter: envelope.presenter ?? ""))
        case "file-changed":
            guard let path = envelope.path else { return nil }
            let list = envelope.slides.map { SlideList(slides: $0, errors: envelope.errors ?? []) }
            return .fileChanged(path: path, slideList: list)
        case "question":
            guard let id = envelope.id, let kind = envelope.kind else { return nil }
            return .question(id: id, kind: kind, payload: envelope.payload ?? QuestionPayload())
        case "recording":
            return .recording(RecordingEvent(state: envelope.state ?? "stopped", segment: envelope.segment ?? 0,
                                             elapsed: envelope.elapsed ?? 0, disk: envelope.disk ?? "ok"))
        case "tunnel":
            return .tunnel(TunnelEvent(state: envelope.state ?? "stopped", url: envelope.url, qr: envelope.qr))
        case "slide":
            guard let slide = envelope.slide else { return nil }
            return .slide(slide: slide, step: envelope.step ?? 0)
        case "error":
            return .error(TapErrorPayload(code: envelope.code ?? "unknown", message: envelope.message ?? ""))
        default:
            return .other(type: envelope.type)
        }
    }
}
```

Note: the `approval` question's payload has fields this struct does not name (`drivers`); `JSONDecoder` ignores unknown keys, and a payload that is not an object makes the whole line undecodable, which `decode` reports as nil and `TapSession` logs as a plain line. That is the same handling D2 gives every malformed line.

Replace `TapCommand` with:

```swift
/// The recording command's action, as `c` does in tap present.
public enum RecordingAction: String, Sendable {
    case newSegment = "new-segment"
    case stop
}

/// A command the app writes to tap's standard input, one JSON line each.
public enum TapCommand: Equatable, Sendable {
    /// The app saved its buffer to the deck file (tap dev only).
    case saved
    /// Render the deck again and reload every page.
    case reload
    /// Shut down. tap present asks keep-recording first when the run recorded.
    case quit
    /// The answer to a question event with this id.
    case answer(id: String, value: Bool)
    /// Start or stop the tunnel, as `u` does.
    case tunnel(start: Bool)
    /// Start a new segment or stop recording, as `c` does.
    case recording(action: RecordingAction)

    /// The command as one JSON line, without the newline.
    public var line: String {
        switch self {
        case .saved: return #"{"type":"saved"}"#
        case .reload: return #"{"type":"reload"}"#
        case .quit: return #"{"type":"quit"}"#
        case .answer(let id, let value):
            return #"{"type":"answer","id":"# + Self.jsonString(id) + #","value":"# + (value ? "true" : "false") + "}"
        case .tunnel(let start): return #"{"type":"tunnel","start":"# + (start ? "true" : "false") + "}"
        case .recording(let action): return #"{"type":"recording","action":""# + action.rawValue + #""}"#
        }
    }

    /// `text` as a JSON string literal, quotes included.
    private static func jsonString(_ text: String) -> String {
        let data = (try? JSONEncoder().encode([text])) ?? Data("[\"\"]".utf8)
        let array = String(decoding: data, as: UTF8.self)
        return String(array.dropFirst().dropLast())
    }
}
```

- [ ] **Step 5: Give `TapSession` a command and a clean quit**

In `TapSession.swift`, add inside the class, after `State`:

```swift
    /// Which tap process this session runs. `dev` is the deck's writing
    /// process; `present` is a talk, started beside it.
    public enum Command: Equatable, Sendable {
        case dev
        /// `record` false adds `--no-record` (Rehearse, or Play with the
        /// record checkbox off); `presenterPassword` is the person's own,
        /// otherwise tap generates one and prints it on the ready line.
        case present(record: Bool, presenterPassword: String?)

        public func arguments(deck: URL) -> [String] {
            switch self {
            case .dev:
                return ["dev", "--app", deck.path]
            case .present(let record, let presenterPassword):
                var arguments = ["present", "--app"]
                if !record { arguments.append("--no-record") }
                if let presenterPassword, !presenterPassword.isEmpty {
                    arguments += ["--presenter-password", presenterPassword]
                }
                arguments.append(deck.path)
                return arguments
            }
        }

        /// The log line for a start: the arguments with the deck's name in
        /// place of its path.
        public func logLine(deck: URL) -> String {
            "tap " + (arguments(deck: deck).dropLast() + [deck.lastPathComponent]).joined(separator: " ")
        }

        /// The Tap Log title: the deck's name, and ", talk" for a talk.
        public func logTitle(deck: URL) -> String {
            let name = deck.deletingPathExtension().lastPathComponent
            if case .present = self { return name + ", talk" }
            return name
        }
    }
```

Add stored properties after `restartPolicy`:

```swift
    public let command: Command
    /// True from `quit()` until the next `start()`: the exit that follows
    /// is tap answering the quit command, never a crash.
    public private(set) var quitRequested = false
    private var quitWork: DispatchWorkItem?
```

Change the initializer to:

```swift
    public init(deckURL: URL, configuration: Configuration, command: Command = .dev) {
        self.deckURL = deckURL
        self.configuration = configuration
        self.command = command
        policy = configuration.policy
        log = TapLog(title: command.logTitle(deck: deckURL))
    }
```

In `start()`, add `quitRequested = false` and `quitWork?.cancel()` as the first two lines. In `stop()`, add `quitWork?.cancel()` after `readyWork?.cancel()`. Add after `tryAgain()`:

```swift
    /// Asks tap to shut down with the `quit` command, which lets tap present
    /// ask keep-recording and finish its recording, and closes standard
    /// input after `timeout` if tap is still running then (D2's stop, with
    /// its SIGTERM and SIGKILL escalation). The exit that follows counts as
    /// requested: nothing restarts.
    public func quit(timeout: TimeInterval = 15) {
        restartWork?.cancel()
        readyWork?.cancel()
        quitWork?.cancel()
        startsAfterStop = false
        launchGeneration += 1
        quitRequested = true
        guard let process else {
            state = .stopped
            return
        }
        process.send(.quit)
        let deadline = DispatchWorkItem { [weak self, weak process] in
            MainActor.assumeIsolated {
                guard let self, let process, self.process === process else { return }
                self.log.append("tap did not quit within \(Int(timeout)) seconds", source: .app)
                process.stop()
            }
        }
        quitWork = deadline
        DispatchQueue.main.asyncAfter(deadline: .now() + timeout, execute: deadline)
    }
```

In `changeDeck(to:)`, keep as is. In `launch(environment:)`, change the arguments and the log line:

```swift
            arguments: command.arguments(deck: deckURL),
```

and

```swift
        log.append(command.logLine(deck: deckURL), source: .app)
```

In `receive(_:)`, change the question case to:

```swift
        case .question(_, let kind, _):
            log.append("tap asks a \(kind) question", source: .event)
```

and add, before `case .error`:

```swift
        case .recording(let recording):
            log.append("recording \(recording.state), segment \(recording.segment), disk \(recording.disk)", source: .event)
        case .tunnel(let tunnel):
            log.append("tunnel \(tunnel.state)" + (tunnel.url.map { " \($0)" } ?? ""), source: .event)
        case .slide(let slide, let step):
            log.append("audience on slide \(slide), step \(step)", source: .event)
```

In `processExited(status:requested:)`, replace the `if requested { ... }` block with:

```swift
        if requested || quitRequested {
            quitWork?.cancel()
            log.append(quitRequested ? "tap quit" : "tap stopped", source: .app)
            quitRequested = false
            state = .stopped
            if startsAfterStop {
                startsAfterStop = false
                start()
            }
            return
        }
```

- [ ] **Step 6: Add the talk URLs to `TapClient`**

After `presenterLaunchURL` in `TapClient.swift`:

```swift
    /// The audience page for a talk, starting on `slide` (1-based): the
    /// launch code as the preview uses it, and the slide in the fragment,
    /// which the page reads on load (`initializeFromURL` in
    /// frontend/src/lib/stores/presentation.ts). The fragment survives
    /// tap's redirect, which names no fragment of its own.
    public func audienceLaunchURL(slide: Int) -> URL { url(path: "/?launch=\(ready.launch)#\(slide)") }

    /// The presenter page for a talk, starting on `slide`. It carries no
    /// key: the presenter cookie is in the web views' data store by the
    /// time this loads (see `PresentationController.installPresenterCookie`).
    public func presenterURL(slide: Int) -> URL { url(path: "/presenter#\(slide)") }
```

- [ ] **Step 7: Run the tests**

Run: `make -C desktop core-test`
Expected: every test passes, including D2's `testStartsTapDevAppAndReadsTheReadyLine` (its log line is unchanged: `tap dev --app talk.md`) and the six new ones.

- [ ] **Step 8: Mutate and commit**

Mutations, each reverted: in `quit`, drop `quitRequested = true` (expected: `testQuitSendsTheQuitCommandAndNeverRestarts` fails, the exit restarts); in `quit`, drop `process.send(.quit)` (expected: the same test fails on the record file); in `quit`, drop the deadline work item (expected: `testQuitClosesStdinWhenTapIgnoresTheCommand` times out); in `Command.arguments`, always append `--no-record` (expected: `testATalkRunsTapPresentWithItsFlagsAndHasItsOwnLogTitle` fails on the `record: true` case); in `decode`, return `.other(type: "slide")` for a slide event (expected: `testDecodesTheTalkEvents` fails).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): decode the talk events, encode the answers, run tap present and quit it cleanly"
```

---

### Task 2: Displays, options, the recording status and the Focus hint state

**Files:**
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/Presenting.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/PresentingCoreTests.swift`

**Interfaces:**
- Consumes: `TapSession.Command` (Task 1), `RecordingEvent` (Task 1), `UserDefaults`.
- Produces: `ScreenInfo(name:frame:isBuiltIn:)`; `DisplayAssignmentStore(defaults:)` with `audienceName(for:)`, `setAudienceName(_:for:)`, `static key(for:)`; `DisplayArrangement(audience:presenter:)` with `isSingleDisplay`, `static resolve(screens:store:)`, `swapped()`; `PresentationMode` (`.play`, `.rehearse`); `PresentationOptions(mode:startSlide:record:phoneRemote:tunnel:presenterPassword:)` with `command`, `wantsTunnel`; `RecordingStatus` with `state`, `segment`, `elapsed`, `disk`, `blockedReason`, `label`, `isRecording`, `apply(_:)`, `tick()`, `static clock(_:)`; `FocusHintState(defaults:)` with `hasBeenShown`, `markShown()`.

- [ ] **Step 1: Write the failing tests**

`desktop/TapDesktopCore/Tests/TapDesktopCoreTests/PresentingCoreTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class PresentingCoreTests: XCTestCase {
    let builtIn = ScreenInfo(name: "Built-in Retina Display", frame: CGRect(x: 0, y: 0, width: 1728, height: 1117), isBuiltIn: true)
    let projector = ScreenInfo(name: "LG UltraFine", frame: CGRect(x: 1728, y: 0, width: 3840, height: 2160), isBuiltIn: false)

    func freshDefaults() throws -> UserDefaults {
        try XCTUnwrap(UserDefaults(suiteName: "PresentingCoreTests.\(UUID().uuidString)"))
    }

    func testTheExternalDisplayIsTheAudienceByDefaultAndTheBuiltInThePresenter() throws {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        let arrangement = try XCTUnwrap(DisplayArrangement.resolve(screens: [builtIn, projector], store: store))
        XCTAssertEqual(arrangement.audience, projector)
        XCTAssertEqual(arrangement.presenter, builtIn)
        XCTAssertFalse(arrangement.isSingleDisplay)
        let reversed = try XCTUnwrap(DisplayArrangement.resolve(screens: [projector, builtIn], store: store))
        XCTAssertEqual(reversed.audience, projector, "the order the system lists screens in does not matter")
    }

    func testOneDisplayIsBothAndNoDisplayIsNothing() throws {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        let one = try XCTUnwrap(DisplayArrangement.resolve(screens: [builtIn], store: store))
        XCTAssertEqual(one.audience, builtIn)
        XCTAssertEqual(one.presenter, builtIn)
        XCTAssertTrue(one.isSingleDisplay)
        XCTAssertNil(DisplayArrangement.resolve(screens: [], store: store))
    }

    func testTwoExternalDisplaysUseTheFirstAsTheAudience() throws {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        let second = ScreenInfo(name: "Dell", frame: CGRect(x: -1920, y: 0, width: 1920, height: 1080), isBuiltIn: false)
        let arrangement = try XCTUnwrap(DisplayArrangement.resolve(screens: [projector, second], store: store))
        XCTAssertEqual(arrangement.audience, projector)
        XCTAssertEqual(arrangement.presenter, second)
    }

    func testASwapIsRememberedForThePairOfDisplaysWhicheverOrderTheyComeIn() throws {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        let arrangement = try XCTUnwrap(DisplayArrangement.resolve(screens: [builtIn, projector], store: store))
        let swapped = arrangement.swapped()
        XCTAssertEqual(swapped.audience, builtIn)
        XCTAssertEqual(swapped.presenter, projector)
        store.setAudienceName(swapped.audience.name, for: [builtIn, projector])
        XCTAssertEqual(DisplayArrangement.resolve(screens: [projector, builtIn], store: store)?.audience, builtIn)
        XCTAssertEqual(DisplayAssignmentStore.key(for: [projector, builtIn]), DisplayAssignmentStore.key(for: [builtIn, projector]))
        // A different pair has no memory of it.
        let other = ScreenInfo(name: "Epson", frame: projector.frame, isBuiltIn: false)
        XCTAssertEqual(DisplayArrangement.resolve(screens: [builtIn, other], store: store)?.audience, other)
        // A remembered name that is no longer connected is ignored.
        store.setAudienceName("Gone", for: [builtIn, projector])
        XCTAssertEqual(DisplayArrangement.resolve(screens: [builtIn, projector], store: store)?.audience, projector)
    }

    func testOptionsBecomeTheTapPresentCommand() {
        let play = PresentationOptions(mode: .play, startSlide: 3)
        XCTAssertEqual(play.command, .present(record: true, presenterPassword: nil))
        XCTAssertFalse(play.wantsTunnel)
        let quiet = PresentationOptions(mode: .play, startSlide: 1, record: false, phoneRemote: true)
        XCTAssertEqual(quiet.command, .present(record: false, presenterPassword: nil))
        XCTAssertTrue(quiet.wantsTunnel)
        let rehearse = PresentationOptions(mode: .rehearse, startSlide: 5, record: true, tunnel: true, presenterPassword: "secret")
        XCTAssertEqual(rehearse.command, .present(record: false, presenterPassword: "secret"), "a rehearsal never records")
        XCTAssertTrue(rehearse.wantsTunnel)
    }

    func testTheRecordingLabelFollowsTapAndCountsUpBetweenEvents() {
        var status = RecordingStatus()
        XCTAssertEqual(status.label, "NOT RECORDING")
        XCTAssertFalse(status.isRecording)
        status.apply(RecordingEvent(state: "recording", segment: 1, elapsed: 724, disk: "ok"))
        XCTAssertEqual(status.label, "REC 12:04")
        XCTAssertTrue(status.isRecording)
        status.tick()
        XCTAssertEqual(status.label, "REC 12:05")
        status.apply(RecordingEvent(state: "paused", segment: 1, elapsed: 725, disk: "ok"))
        XCTAssertEqual(status.label, "REC PAUSED")
        status.tick()
        XCTAssertEqual(status.elapsed, 725, "a pause does not count up")
        status.apply(RecordingEvent(state: "stopped", segment: 1, elapsed: 0, disk: "full"))
        XCTAssertEqual(status.label, "NOT RECORDING")
        XCTAssertEqual(status.disk, "full")
        status.blockedReason = "Screen Recording permission is off"
        XCTAssertEqual(status.label, "NOT RECORDING")
        XCTAssertEqual(RecordingStatus.clock(0), "0:00")
        XCTAssertEqual(RecordingStatus.clock(59), "0:59")
        XCTAssertEqual(RecordingStatus.clock(3661), "1:01:01")
    }

    func testTheFocusHintShowsOnce() throws {
        let hint = FocusHintState(defaults: try freshDefaults())
        XCTAssertFalse(hint.hasBeenShown)
        hint.markShown()
        XCTAssertTrue(hint.hasBeenShown)
    }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `make -C desktop core-test`
Expected: the package does not compile (`ScreenInfo` and the others are undefined).

- [ ] **Step 3: Write `Presenting.swift`**

```swift
import Foundation

/// A display, as the talk's windows see it: its name, its frame in screen
/// coordinates and whether it is the Mac's own screen. The app builds one
/// per `NSScreen`; tests build them by hand.
public struct ScreenInfo: Equatable, Sendable {
    public let name: String
    public let frame: CGRect
    public let isBuiltIn: Bool

    public init(name: String, frame: CGRect, isBuiltIn: Bool) {
        self.name = name
        self.frame = frame
        self.isBuiltIn = isBuiltIn
    }
}

/// Which display was the audience the last time this set of displays was
/// connected, across decks. Keyed by the displays' names, sorted, so the
/// order the system lists them in does not matter.
// UserDefaults is thread-safe, so a struct that only holds a reference to it is safe to share.
public struct DisplayAssignmentStore: @unchecked Sendable {
    private let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public static func key(for screens: [ScreenInfo]) -> String {
        "DisplayAssignment:" + screens.map(\.name).sorted().joined(separator: "|")
    }

    public func audienceName(for screens: [ScreenInfo]) -> String? {
        defaults.string(forKey: Self.key(for: screens))
    }

    public func setAudienceName(_ name: String, for screens: [ScreenInfo]) {
        defaults.set(name, forKey: Self.key(for: screens))
    }
}

/// Which screen shows the audience page and which the presenter page. With
/// one display both are the same screen.
public struct DisplayArrangement: Equatable, Sendable {
    public let audience: ScreenInfo
    public let presenter: ScreenInfo

    public init(audience: ScreenInfo, presenter: ScreenInfo) {
        self.audience = audience
        self.presenter = presenter
    }

    public var isSingleDisplay: Bool { audience == presenter }

    /// The arrangement for `screens`: the first external display is the
    /// audience and the built-in one the presenter, unless the store
    /// remembers the audience on another connected display. With no
    /// built-in display, the first listed is the audience. nil with no
    /// screens at all.
    public static func resolve(screens: [ScreenInfo], store: DisplayAssignmentStore) -> DisplayArrangement? {
        guard let first = screens.first else { return nil }
        guard screens.count > 1 else { return DisplayArrangement(audience: first, presenter: first) }
        let presenter = screens.first(where: \.isBuiltIn) ?? screens[1]
        let audience = screens.first { $0 != presenter } ?? first
        var arrangement = DisplayArrangement(audience: audience, presenter: presenter)
        if let remembered = store.audienceName(for: screens), remembered == presenter.name {
            arrangement = arrangement.swapped()
        }
        return arrangement
    }

    public func swapped() -> DisplayArrangement {
        DisplayArrangement(audience: presenter, presenter: audience)
    }
}

public enum PresentationMode: Equatable, Sendable {
    /// Play: the audience page on the projector, the presenter page on the laptop, tap's recording rule.
    case play
    /// Rehearse: the presenter page alone, never recorded.
    case rehearse
}

/// What the Present popover collects, and Rehearse assumes.
public struct PresentationOptions: Equatable, Sendable {
    public var mode: PresentationMode
    /// The slide the talk opens on, 1-based.
    public var startSlide: Int
    /// False passes --no-record for this run. True leaves recording to tap's
    /// own consent, stored in settings.yaml.
    public var record: Bool
    /// Phone remote: the tunnel, with the QR code panel.
    public var phoneRemote: Bool
    /// Advanced: the tunnel on its own.
    public var tunnel: Bool
    /// Advanced: the person's own presenter password, passed to tap.
    public var presenterPassword: String?

    public init(mode: PresentationMode, startSlide: Int, record: Bool = true, phoneRemote: Bool = false,
                tunnel: Bool = false, presenterPassword: String? = nil) {
        self.mode = mode
        self.startSlide = startSlide
        self.record = record
        self.phoneRemote = phoneRemote
        self.tunnel = tunnel
        self.presenterPassword = presenterPassword
    }

    public var command: TapSession.Command {
        .present(record: mode == .play && record, presenterPassword: presenterPassword)
    }

    public var wantsTunnel: Bool { phoneRemote || tunnel }
}

/// The recording state the presenter toolbar shows, kept from tap's
/// recording events and ticked once a second in between, since tap sends
/// an event only when the state, segment or disk level changes.
public struct RecordingStatus: Equatable, Sendable {
    public var state = "stopped"
    public var segment = 0
    public var elapsed = 0
    public var disk = "ok"
    /// Why tap cannot record at all this run (a `recording_blocked` error), if it said so.
    public var blockedReason: String?

    public init() {}

    public var isRecording: Bool { state == "recording" }

    public var label: String {
        switch state {
        case "recording": return "REC " + Self.clock(elapsed)
        case "paused": return "REC PAUSED"
        default: return "NOT RECORDING"
        }
    }

    public mutating func apply(_ event: RecordingEvent) {
        state = event.state
        segment = event.segment
        elapsed = event.elapsed
        disk = event.disk
    }

    /// One second passed.
    public mutating func tick() {
        if isRecording { elapsed += 1 }
    }

    /// `seconds` as m:ss, or h:mm:ss from an hour.
    public static func clock(_ seconds: Int) -> String {
        let hours = seconds / 3600
        let minutes = (seconds % 3600) / 60
        let rest = seconds % 60
        if hours > 0 { return String(format: "%d:%02d:%02d", hours, minutes, rest) }
        return String(format: "%d:%02d", minutes, rest)
    }
}

/// Whether the Focus hint has been shown: it appears before the first talk
/// on this Mac and never again.
public struct FocusHintState: @unchecked Sendable {
    private let defaults: UserDefaults
    static let key = "FocusHintShown"

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public var hasBeenShown: Bool { defaults.bool(forKey: Self.key) }

    public func markShown() {
        defaults.set(true, forKey: Self.key)
    }
}
```

- [ ] **Step 4: Run the tests**

Run: `make -C desktop core-test`
Expected: the seven new tests pass; the package is green.

- [ ] **Step 5: Mutate and commit**

Mutations, each reverted: in `resolve`, ignore the store (expected: `testASwapIsRememberedForThePairOfDisplaysWhicheverOrderTheyComeIn` fails on the swapped audience); in `resolve`, honour a remembered name that is not connected (expected: the same test fails on "Gone"); in `key(for:)`, drop `.sorted()` (expected: the key equality assertion fails); in `PresentationOptions.command`, drop `mode == .play &&` (expected: `testOptionsBecomeTheTapPresentCommand` fails on the rehearsal); in `tick`, drop the `isRecording` guard (expected: the paused count fails); in `label`, return "REC" for paused (expected: `testTheRecordingLabelFollowsTapAndCountsUpBetweenEvents` fails).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): display arrangement, presentation options, recording status and the Focus hint state"
```

---

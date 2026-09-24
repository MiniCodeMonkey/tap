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
        XCTAssertTrue(tap.log.text.contains("did not quit within 0.3 seconds"))
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
                self.log.append("tap did not quit within \(String(format: "%.1f", timeout)) seconds", source: .app)
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

### Task 3: The sleep assertion, a page of tap's in a web view, and a window that covers a screen

**Files:**
- Create: `desktop/Tap/Presenting/SleepAssertion.swift`
- Create: `desktop/Tap/Presenting/PresentationPageController.swift`
- Create: `desktop/Tap/Presenting/PresentationWindow.swift`
- Create: `desktop/TapTests/Support/WindowServer.swift`
- Test: `desktop/TapTests/PresentationWindowTests.swift`

**Interfaces:**
- Consumes: D2's `ReadyPayload`, `WeakScriptMessageHandler`, `PreviewViewController.isExternalWebLink(url:navigationType:)`; D3's `DeckWindowController` (only as a weak reference type).
- Produces: `SleepAssertion` with `static let reason`, `isHeld`, `acquire()`, `release()`; `PresentationPageController(accessibilityIdentifier:)` with `webView`, `onReady`, `onLoadFailed`, `onPresenterPopup`, `openExternally`, `lastReady`, `pageLoadCount`, `lastLoadedURL`, `load(_:allowedPort:)`, `popupRequested(for:navigationType:)`, `pageReportedReady(_:)`, `pageText()` (test-only); `PresentationWindow(role:screenFrame:)` with `Role` (`.audience`, `.presenter`), `page`, `container`, `deckWindowController`, `static coveringLevel`, `cover(_:)`; the test helper `onScreenWindowNumbers()`.

- [ ] **Step 1: Write the window server helper and the failing tests**

`desktop/TapTests/Support/WindowServer.swift`:

```swift
import AppKit

/// The window numbers of this process's windows that the window server
/// has on screen, front to back. This is the window server's own truth,
/// not AppKit's bookkeeping, so it holds in a host with no key window and
/// on the CI runner alike.
func onScreenWindowNumbers() -> [Int] {
    let pid = Int(ProcessInfo.processInfo.processIdentifier)
    let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
    return windows.compactMap { window in
        guard window[kCGWindowOwnerPID as String] as? Int == pid,
              window[kCGWindowIsOnscreen as String] as? Bool == true else { return nil }
        return window[kCGWindowNumber as String] as? Int
    }
}

/// Runs `pmset -g assertions` and reports whether an assertion with `name`
/// is listed: the kernel's own view of what the app holds.
func powerAssertionIsListed(named name: String) -> Bool {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/pmset")
    process.arguments = ["-g", "assertions"]
    let output = Pipe()
    process.standardOutput = output
    process.standardError = FileHandle.nullDevice
    guard (try? process.run()) != nil else { return false }
    let data = output.fileHandleForReading.readDataToEndOfFile()
    process.waitUntilExit()
    return String(decoding: data, as: UTF8.self).contains(name)
}
```

`desktop/TapTests/PresentationWindowTests.swift`:

```swift
import XCTest
import WebKit
@testable import Tap

final class PresentationWindowTests: HostedTestCase {
    func testTheSleepAssertionIsHeldOnlyBetweenAcquireAndRelease() {
        let assertion = SleepAssertion()
        XCTAssertFalse(assertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
        assertion.acquire()
        XCTAssertTrue(assertion.isHeld)
        XCTAssertTrue(powerAssertionIsListed(named: SleepAssertion.reason), "the kernel lists the assertion")
        assertion.acquire()
        XCTAssertTrue(assertion.isHeld, "a second acquire changes nothing")
        assertion.release()
        XCTAssertFalse(assertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
        assertion.release()
        XCTAssertFalse(assertion.isHeld)
    }

    func testAWindowCoversItsScreenAboveTheMenuBar() async throws {
        let screen = NSScreen.screens[0].frame
        let window = PresentationWindow(role: .audience, screenFrame: screen)
        defer { window.close() }
        XCTAssertEqual(window.frame, screen)
        XCTAssertEqual(window.level, PresentationWindow.coveringLevel)
        XCTAssertGreaterThan(window.level.rawValue, NSWindow.Level.mainMenu.rawValue)
        XCTAssertTrue(window.styleMask.contains(.borderless))
        XCTAssertTrue(window.canBecomeKey)
        XCTAssertTrue(window.collectionBehavior.contains(.canJoinAllSpaces))
        XCTAssertFalse(window.isVisible)
        window.orderFrontRegardless()
        XCTAssertTrue(window.isVisible)
        try await waitUntil(timeout: 5, "the window server to show the window") { onScreenWindowNumbers().contains(window.windowNumber) }
        let half = CGRect(x: screen.minX, y: screen.minY, width: screen.width / 2, height: screen.height)
        window.cover(half)
        XCTAssertEqual(window.frame, half)
        XCTAssertEqual(window.page.webView.frame.size, half.size, "the page fills the window")
        window.orderOut(nil)
        try await waitUntil(timeout: 5, "the window to leave the screen") { !onScreenWindowNumbers().contains(window.windowNumber) }
    }

    func testThePageHasEverythingTapDevsBrowserWouldGiveIt() {
        let page = PresentationPageController(accessibilityIdentifier: "audience-page")
        _ = page.view
        XCTAssertTrue(page.webView.configuration.preferences.isElementFullscreenEnabled, "the F key's full screen works")
        XCTAssertTrue(page.webView.configuration.websiteDataStore.isPersistent, "the presenter layout and notes size persist")
        XCTAssertTrue(page.webView.configuration.websiteDataStore === WKWebsiteDataStore.default(), "the one store every talk shares")
        XCTAssertEqual(page.webView.accessibilityIdentifier(), "audience-page")
    }

    func testTheSKeysPopupBringsThePresenterWindowForwardAndLinksGoToTheBrowser() {
        let page = PresentationPageController(accessibilityIdentifier: "audience-page")
        _ = page.view
        page.load(URL(string: "about:blank")!, allowedPort: 4242)
        var popups = 0
        var opened: [URL] = []
        page.onPresenterPopup = { popups += 1 }
        page.openExternally = { opened.append($0) }
        page.popupRequested(for: URL(string: "http://127.0.0.1:4242/presenter#3"), navigationType: .other)
        XCTAssertEqual(popups, 1)
        page.popupRequested(for: URL(string: "http://127.0.0.1:9999/presenter"), navigationType: .other)
        XCTAssertEqual(popups, 1, "another port is not this talk's presenter view")
        page.popupRequested(for: URL(string: "https://example.com/"), navigationType: .linkActivated)
        XCTAssertEqual(opened, [URL(string: "https://example.com/")!])
        page.popupRequested(for: URL(string: "https://example.com/redirect"), navigationType: .other)
        XCTAssertEqual(opened.count, 1, "a popup the page opened on its own goes nowhere")
        XCTAssertEqual(page.pageLoadCount, 1)
        XCTAssertEqual(page.lastLoadedURL?.absoluteString, "about:blank")
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/PresentationWindowTests/testTheSleepAssertionIsHeldOnlyBetweenAcquireAndRelease`
Expected: the test target does not compile (`SleepAssertion`, `PresentationWindow`, `PresentationPageController` are undefined).

- [ ] **Step 3: Write `SleepAssertion.swift`**

```swift
import Foundation
import IOKit.pwr_mgt

/// Keeps the display awake for exactly as long as a talk runs: the same
/// kind of assertion Keynote holds during a slideshow. Every way a talk
/// ends releases it (see `PresentationController.takeDownWindows`), and a
/// process that dies takes its assertions with it.
final class SleepAssertion {
    static let reason = "Tap is presenting"
    /// The assertion type, spelled as IOPMLib.h spells
    /// kIOPMAssertionTypePreventUserIdleDisplaySleep, which is a macro of a
    /// macro that Swift does not import.
    static let type = "PreventUserIdleDisplaySleep"

    private var identifier: IOPMAssertionID = 0
    private(set) var isHeld = false

    func acquire() {
        guard !isHeld else { return }
        let result = IOPMAssertionCreateWithName(Self.type as CFString, IOPMAssertionLevel(kIOPMAssertionLevelOn),
                                                 Self.reason as CFString, &identifier)
        // kIOReturnSuccess is KERN_SUCCESS, which is 0.
        isHeld = result == 0
    }

    func release() {
        guard isHeld else { return }
        IOPMAssertionRelease(identifier)
        identifier = 0
        isHeld = false
    }

    deinit {
        release()
    }
}
```

- [ ] **Step 4: Write `PresentationPageController.swift`**

```swift
import AppKit
import WebKit

/// One of tap's own pages for a talk, the audience page or the presenter
/// page, in a `WKWebView` with what tap dev's browser gives it: element
/// full screen for the F key, the persistent data store so the presenter
/// layout and notes size survive between launches, and the window the S
/// key opens answered by bringing the presenter window forward. The app
/// drives the page only through the URL it loads (its start slide is in
/// the fragment) and through tap's hub, and reads it only through the
/// tapReady handler.
final class PresentationPageController: NSViewController, WKNavigationDelegate, WKUIDelegate, WKScriptMessageHandler {
    let webView: WKWebView
    var onReady: ((ReadyPayload) -> Void)?
    var onLoadFailed: ((Error) -> Void)?
    /// The page asked for a window at /presenter: the S key.
    var onPresenterPopup: (() -> Void)?
    /// Opens a URL outside the app. A test replaces it to see what the app tried to open.
    var openExternally: (URL) -> Void = { url in NSWorkspace.shared.open(url) }
    private(set) var lastReady: ReadyPayload?
    private(set) var pageLoadCount = 0
    private(set) var lastLoadedURL: URL?
    private var allowedPort: Int?

    init(accessibilityIdentifier: String) {
        let configuration = WKWebViewConfiguration()
        configuration.userContentController = WKUserContentController()
        configuration.websiteDataStore = .default()
        configuration.preferences.isElementFullscreenEnabled = true
        webView = WKWebView(frame: .zero, configuration: configuration)
        super.init(nibName: nil, bundle: nil)
        configuration.userContentController.add(WeakScriptMessageHandler(self), name: "tapReady")
        webView.navigationDelegate = self
        webView.uiDelegate = self
        webView.setAccessibilityIdentifier(accessibilityIdentifier)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        view = webView
    }

    /// Loads one of tap's pages. `allowedPort` is the talk's tap, the only
    /// origin the page may navigate within.
    func load(_ url: URL, allowedPort: Int) {
        self.allowedPort = allowedPort
        pageLoadCount += 1
        lastLoadedURL = url
        lastReady = nil
        webView.load(URLRequest(url: url))
    }

    /// The page's visible text. Tests read it; the app never runs script in the page.
    func pageText() async -> String {
        (try? await webView.evaluateJavaScript("document.body.innerText") as? String) ?? ""
    }

    // MARK: WebKit

    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction) async -> WKNavigationActionPolicy {
        guard let url = navigationAction.request.url else { return .cancel }
        if url.host == "127.0.0.1", url.port == allowedPort { return .allow }
        if url.scheme == "about" || url.scheme == "blob" || url.scheme == "data" { return .allow }
        if PreviewViewController.isExternalWebLink(url: url, navigationType: navigationAction.navigationType) { openExternally(url) }
        return .cancel
    }

    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
                 for navigationAction: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? {
        popupRequested(for: navigationAction.request.url, navigationType: navigationAction.navigationType)
        return nil
    }

    /// Answers a `window.open` from the page: this talk's presenter view
    /// goes to the presenter window, a clicked web link to the browser,
    /// anything else nowhere. Internal so a test can drive it without a page.
    func popupRequested(for url: URL?, navigationType: WKNavigationType) {
        guard let url else { return }
        if url.host == "127.0.0.1", url.port == allowedPort, url.path.hasPrefix("/presenter") {
            onPresenterPopup?()
        } else if PreviewViewController.isExternalWebLink(url: url, navigationType: navigationType) {
            openExternally(url)
        }
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        onLoadFailed?(error)
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        onLoadFailed?(error)
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        guard message.name == "tapReady", let body = message.body as? [String: Any],
              let slide = (body["slide"] as? NSNumber)?.intValue else { return }
        pageReportedReady(ReadyPayload(revision: body["revision"] as? String ?? "",
                                       slide: slide,
                                       step: (body["step"] as? NSNumber)?.intValue ?? 0,
                                       settled: (body["settled"] as? NSNumber)?.boolValue ?? true))
    }

    /// Records a ready the page reported and passes it on. Internal so a
    /// test can deliver one through the same path the handler uses.
    func pageReportedReady(_ payload: ReadyPayload) {
        lastReady = payload
        onReady?(payload)
    }
}
```

- [ ] **Step 5: Write `PresentationWindow.swift`**

```swift
import AppKit

/// A window that covers one whole screen for a talk: borderless, above the
/// menu bar and the Dock, with no system full screen Space. It appears at
/// once on whichever display it is given, two of them can share one
/// display (the audience with the presenter behind it on a single screen),
/// and a swap is a frame change. Escape and Option-Tab are the
/// controller's key monitor's.
final class PresentationWindow: NSWindow {
    enum Role: Equatable {
        case audience
        case presenter
    }

    let role: Role
    let page: PresentationPageController
    /// Holds the page and, in the presenter window, the toolbar and the REC dot over it.
    let container = NSView()
    /// The deck this window presents, for menu actions that reach this window first.
    weak var deckWindowController: DeckWindowController?
    /// One above the menu bar, which is above the Dock.
    static let coveringLevel = NSWindow.Level(rawValue: NSWindow.Level.mainMenu.rawValue + 1)

    init(role: Role, screenFrame: CGRect) {
        self.role = role
        page = PresentationPageController(accessibilityIdentifier: role == .audience ? "audience-page" : "presenter-page")
        super.init(contentRect: screenFrame, styleMask: [.borderless], backing: .buffered, defer: false)
        level = Self.coveringLevel
        collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .ignoresCycle, .stationary]
        isReleasedWhenClosed = false
        hasShadow = false
        animationBehavior = .none
        backgroundColor = .black
        isOpaque = true
        acceptsMouseMovedEvents = true
        setAccessibilityIdentifier(role == .audience ? "audience-window" : "presenter-window")
        page.view.translatesAutoresizingMaskIntoConstraints = false
        container.addSubview(page.view)
        NSLayoutConstraint.activate([
            page.view.topAnchor.constraint(equalTo: container.topAnchor),
            page.view.leadingAnchor.constraint(equalTo: container.leadingAnchor),
            page.view.trailingAnchor.constraint(equalTo: container.trailingAnchor),
            page.view.bottomAnchor.constraint(equalTo: container.bottomAnchor),
        ])
        contentView = container
        container.layoutSubtreeIfNeeded()
    }

    // A borderless window takes keys and becomes main only when it says so.
    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { true }

    /// Puts the window over `frame`, a screen's frame.
    func cover(_ frame: CGRect) {
        setFrame(frame, display: true)
        container.layoutSubtreeIfNeeded()
    }

    /// Menu actions this window cannot answer go to the deck's window
    /// controller: Stop, Reload Slides, Swap Displays and Tap Log work
    /// while a presentation window is key.
    override func supplementalTarget(forAction action: Selector, sender: Any?) -> Any? {
        if let deckWindowController, deckWindowController.responds(to: action) { return deckWindowController }
        return super.supplementalTarget(forAction: action, sender: sender)
    }
}
```

- [ ] **Step 6: Run the tests one at a time**

Run: `make -C desktop project` (the new folder must enter the generated project), then:

```bash
make -C desktop test ONLY=TapTests/PresentationWindowTests/testTheSleepAssertionIsHeldOnlyBetweenAcquireAndRelease
make -C desktop test ONLY=TapTests/PresentationWindowTests/testAWindowCoversItsScreenAboveTheMenuBar
make -C desktop test ONLY=TapTests/PresentationWindowTests/testThePageHasEverythingTapDevsBrowserWouldGiveIt
make -C desktop test ONLY=TapTests/PresentationWindowTests/testTheSKeysPopupBringsThePresenterWindowForwardAndLinksGoToTheBrowser
```

Expected: all four pass. The window test covers the screen for under a second.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted, the assertion ones first: in `release`, drop `IOPMAssertionRelease(identifier)` (expected: `testTheSleepAssertionIsHeldOnlyBetweenAcquireAndRelease` fails on the second `pmset` check); in `acquire`, never set `isHeld` (expected: it fails on `isHeld`); in `PresentationWindow.init`, use `NSWindow.Level.normal` (expected: the level assertions fail); drop `canBecomeKey` (expected: `canBecomeKey` fails); in `popupRequested`, drop the port check (expected: the popup count is 2); in the page's init, drop `isElementFullscreenEnabled = true` (expected: the page test fails).

```bash
git add desktop/Tap/Presenting desktop/TapTests
git commit -m "feat(desktop): the sleep assertion, a talk page in a web view and a window that covers a screen"
```

---

### Task 4: The talk: start, ready, windows, stop, and every way the sleep assertion is released

**Files:**
- Create: `desktop/Tap/Presenting/PresentationController.swift`
- Modify: `desktop/Tap/Documents/DeckSessionController.swift` (`presentation`, `saveForPresenting`, `stop`)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (`init`: hand the controller its deck window controller)
- Modify: `desktop/Tap/App/AppEnvironment.swift` (`displayAssignments`, `presentExecutableURL`, `presentSessionConfiguration()`)
- Modify: `desktop/Tap/App/AppDelegate.swift` (`deck(owning:)`, `applicationWillTerminate`, `stopAllPresentations`)
- Modify: `desktop/Tap/TapLog/TapLogWindowController.swift` (`reload` lists the talk's log)
- Modify: `desktop/TapTests/Support/HostedTestCase.swift` (fresh seams per test)
- Modify: `desktop/TapTests/Support/FakeTapScripts.swift` (`readyAndWaiting`, `silent`)
- Create: `desktop/TapTests/Support/PresentingTestCase.swift`
- Test: `desktop/TapTests/PresentingTests.swift`

**Interfaces:**
- Consumes: Task 1's `TapSession(deckURL:configuration:command:)`, `quit(timeout:)`, `TapEvent` cases; Task 2's `DisplayArrangement`, `DisplayAssignmentStore`, `PresentationOptions`, `RecordingStatus`, `ScreenInfo`; Task 3's `SleepAssertion`, `PresentationWindow`, `PresentationPageController`; D2's `TapClient.authorizePresenter()`, `presenterCookie`, `presenterCookieName`, `openSocket()`; D3's `DeckSessionController.jumpToSlide(number:)`, `isContentEdited`, `DeckDocument.save(to:ofType:for:completionHandler:)`.
- Produces: `PresentationController` with `State` (`.idle`, `.starting`, `.presenting`, `.stopping`, `.failed(String)`), `state`, `options`, `session`, `client`, `audienceWindow`, `presenterWindow`, `frontWindow`, `arrangement`, `currentArrangement`, `sleepAssertion`, `lastSlide`, `recording`, `windowsShown`, `pendingQuestion`, `isActive`, `canStart`, `screens`, `displayAssignments`, `deckWindowController`, `onStateChange`, `onEvent`, `onStopped`, `onRecordingChange`, `onQuestion`, `onFailed`, `start(_:)`, `stop()`, `answer(id:value:)`, `toggleFrontWindow()`, `bringPresenterWindowForward()`, `handle(_:)`, `static installPresenterCookie(_:)`, `static showWindowsFallbackInterval`; `DeckSessionController.presentation`, `saveForPresenting(completion:)`; `AppEnvironment.displayAssignments`, `presentExecutableURL`, `presentSessionConfiguration()`; `AppDelegate.stopAllPresentations()`; `PresentingTestCase` with `writeRecordingConsent(_:)`, `removeRecordingConsent()`, `settingsFile`, `oneScreen()`, `halfScreens()`, `openDeckForPresenting(_:)`, `startPresenting(_:_:)`, `stopPresenting(_:)`, `presenterCookieInTheSharedStore()`; `FakeTapScripts.readyAndWaiting()`, `silent()`.

- [ ] **Step 1: Write the test support**

Add to `desktop/TapTests/Support/FakeTapScripts.swift`, inside the enum:

```swift
    /// Prints a ready line, then reads stdin until it closes, like a tap
    /// that is up but has no server: the talk's pages cannot load.
    static func readyAndWaiting() throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        try """
        #!/bin/sh
        echo '{"type":"ready","port":1,"token":"token","launch":"launch","presenter":"presenter"}'
        while IFS= read -r line; do :; done
        exit 0
        """.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }

    /// Never prints a ready line and ignores stdin closing, so only a
    /// signal ends it.
    static func silent() throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        try "#!/bin/sh\nwhile true; do sleep 0.1; done\n".write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }
```

In `desktop/TapTests/Support/HostedTestCase.swift`, add to `setUp` after the `slidePasteboard` line:

```swift
        AppEnvironment.shared.displayAssignments = DisplayAssignmentStore(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.displays.\(UUID().uuidString)")))
        AppEnvironment.shared.presentExecutableURL = nil
```

`desktop/TapTests/Support/PresentingTestCase.swift`:

```swift
import XCTest
import WebKit
@testable import Tap

/// A hosted test that runs a talk with the bundled tap present, on the one
/// screen the machine has. The consent question is answered ahead of time
/// in the test's own settings folder, so tap asks nothing and records
/// nothing; a test that wants the question removes the answer.
@MainActor
class PresentingTestCase: HostedTestCase {
    override func setUp() async throws {
        try await super.setUp()
        try writeRecordingConsent(false)
    }

    override func tearDown() async throws {
        for document in NSDocumentController.shared.documents {
            (document as? DeckDocument)?.sessionController?.presentation.stop()
        }
        try await super.tearDown()
        try await waitUntil(timeout: 20, "every talk window to go away") {
            !NSApp.windows.contains { $0.isVisible && $0 is PresentationWindow }
        }
    }

    var settingsFile: URL { configHome.appendingPathComponent("tap/settings.yaml") }

    /// The CLI's own answer to the consent question, at present.record.
    func writeRecordingConsent(_ record: Bool) throws {
        try FileManager.default.createDirectory(at: settingsFile.deletingLastPathComponent(), withIntermediateDirectories: true)
        try "present:\n  record: \(record)\n".write(to: settingsFile, atomically: true, encoding: .utf8)
    }

    func removeRecordingConsent() throws {
        if FileManager.default.fileExists(atPath: settingsFile.path) { try FileManager.default.removeItem(at: settingsFile) }
    }

    /// The real screen, as the only display.
    func oneScreen() -> [ScreenInfo] {
        [ScreenInfo(name: "Only Display", frame: NSScreen.screens[0].frame, isBuiltIn: true)]
    }

    /// The real screen split in two: the left half stands for the laptop,
    /// the right half for the projector. Both halves are on screen, so the
    /// window server sees every window the arrangement places.
    func halfScreens() -> [ScreenInfo] {
        let frame = NSScreen.screens[0].frame
        let left = CGRect(x: frame.minX, y: frame.minY, width: (frame.width / 2).rounded(.down), height: frame.height)
        let right = CGRect(x: left.maxX, y: frame.minY, width: frame.width - left.width, height: frame.height)
        return [ScreenInfo(name: "Built-in Display", frame: left, isBuiltIn: true),
                ScreenInfo(name: "Projector", frame: right, isBuiltIn: false)]
    }

    /// Opens a copy of the deck, waits for its preview and boxes, and points
    /// its talk at the one real screen.
    func openDeckForPresenting(_ name: String = "ops.md", slides: Int = 7) async throws -> (DeckDocument, DeckSessionController) {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck(name))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: slides)
        let screens = oneScreen()
        controller.presentation.screens = { screens }
        return (document, controller)
    }

    func startPresenting(_ controller: DeckSessionController, _ options: PresentationOptions, timeout: TimeInterval = 40) async throws {
        controller.presentation.start(options)
        try await waitUntil(timeout: timeout, "the talk to be presenting (state \(controller.presentation.state))") {
            controller.presentation.state == .presenting
        }
    }

    func stopPresenting(_ controller: DeckSessionController) async throws {
        controller.presentation.stop()
        try await waitUntil(timeout: 30, "the talk to end") { controller.presentation.state == .idle }
    }

    /// The presenter cookie in the web views' shared store, if any.
    func presenterCookieInTheSharedStore() async -> String? {
        let cookies: [HTTPCookie] = await withCheckedContinuation { continuation in
            WKWebsiteDataStore.default().httpCookieStore.getAllCookies { continuation.resume(returning: $0) }
        }
        return cookies.first { $0.name == TapClient.presenterCookieName && $0.domain == "127.0.0.1" }?.value
    }

    func isRunning(_ pid: Int32) -> Bool {
        kill(pid, 0) == 0
    }
}
```

- [ ] **Step 2: Write the failing tests**

`desktop/TapTests/PresentingTests.swift`:

```swift
import XCTest
@testable import Tap

final class PresentingTests: PresentingTestCase {
    func testSaveBeforePresenting() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let end = (controller.editor.string as NSString).range(of: "# One").upperBound
        controller.editor.setSelectedRange(NSRange(location: end, length: 0))
        controller.editor.insertText(" edited", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertTrue(document.isDocumentEdited)

        controller.presentation.start(PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertEqual(controller.presentation.state, .starting)
        // The process exists only once the save has completed.
        try await waitUntil(timeout: 10, "tap present to be started") { controller.presentation.session != nil }
        let onDisk = try String(contentsOf: deck, encoding: .utf8)
        XCTAssertTrue(onDisk.contains("# One edited"), "the buffer reached the file before tap present started")
        XCTAssertFalse(document.isDocumentEdited)
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
    }

    func testWindowsCoverTheScreenAndTheAudienceIsInFront() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 2))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let screen = NSScreen.screens[0].frame
        XCTAssertEqual(audience.frame, screen)
        XCTAssertEqual(presenter.frame, screen)
        XCTAssertTrue(audience.isVisible)
        XCTAssertTrue(presenter.isVisible)
        XCTAssertTrue(audience.deckWindowController === controller.editor.window?.windowController as? DeckWindowController)
        try await waitUntil(timeout: 5, "both windows on screen") {
            let order = onScreenWindowNumbers()
            return order.contains(audience.windowNumber) && order.contains(presenter.windowNumber)
        }
        let order = onScreenWindowNumbers()
        let audienceIndex = try XCTUnwrap(order.firstIndex(of: audience.windowNumber))
        let presenterIndex = try XCTUnwrap(order.firstIndex(of: presenter.windowNumber))
        XCTAssertLessThan(audienceIndex, presenterIndex, "on one display the audience is in front")
        XCTAssertTrue(presentation.frontWindow === audience)

        XCTAssertEqual(audience.page.lastLoadedURL?.fragment, "2")
        XCTAssertEqual(presenter.page.lastLoadedURL?.fragment, "2")
        XCTAssertEqual(audience.page.lastLoadedURL?.query, "launch=\(try XCTUnwrap(presentation.client).ready.launch)")
        try await waitUntil(timeout: 20, "the audience page on slide 2") { audience.page.lastReady?.slide == 2 }
        try await waitUntil(timeout: 20, "the presenter page ready") { presenter.page.lastReady != nil }
        let cookie = await presenterCookieInTheSharedStore()
        XCTAssertNotNil(cookie)
        XCTAssertEqual(cookie, presentation.client?.presenterCookie, "both pages hold the hub's presenter cookie")

        presentation.toggleFrontWindow()
        try await waitUntil(timeout: 5, "the presenter window in front") {
            let now = onScreenWindowNumbers()
            guard let a = now.firstIndex(of: audience.windowNumber), let p = now.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
        XCTAssertTrue(presentation.frontWindow === presenter)
        presentation.toggleFrontWindow()
        XCTAssertTrue(presentation.frontWindow === audience)
    }

    func testTheMacStaysAwake() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertTrue(presentation.sleepAssertion.isHeld)
        XCTAssertTrue(powerAssertionIsListed(named: SleepAssertion.reason), "the kernel holds the display awake")
        try await stopPresenting(controller)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
    }

    func testTheSleepAssertionIsReleasedWhenTheDeckWindowCloses() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let pid = try XCTUnwrap(presentation.session?.processIdentifier)
        let audience = try XCTUnwrap(presentation.audienceWindow)
        document.close()
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
        XCTAssertFalse(audience.isVisible)
        try await waitUntil(timeout: 20, "tap present to exit") { !self.isRunning(pid) }
        try await waitUntil(timeout: 5, "no talk window on screen") { !onScreenWindowNumbers().contains(audience.windowNumber) }
    }

    func testTheSleepAssertionIsReleasedWhenTapPresentDies() async throws {
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.readyAndWaiting()
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 10, "the assertion at ready") { presentation.sleepAssertion.isHeld }
        // Three deaths in thirty seconds: the app stops restarting (D2's policy).
        for _ in 0..<3 {
            try await waitUntil(timeout: 10, "a running tap present or the end") {
                if case .failed = presentation.state { return true }
                return presentation.session?.processIdentifier != nil
            }
            guard let pid = presentation.session?.processIdentifier else { break }
            kill(pid, SIGKILL)
            // The restart's process has a new identifier; the failure has none.
            try await waitUntil(timeout: 10, "the killed process to be gone") { presentation.session?.processIdentifier != pid }
        }
        try await waitUntil(timeout: 10, "the talk to fail") { if case .failed = presentation.state { return true } else { return false } }
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertNil(presentation.presenterWindow)
    }

    func testTheSleepAssertionIsReleasedWhenTheAppQuits() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let pid = try XCTUnwrap(presentation.session?.processIdentifier)
        let delegate = try XCTUnwrap(NSApp.delegate as? AppDelegate)
        delegate.applicationWillTerminate(Notification(name: NSApplication.willTerminateNotification))
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertEqual(presentation.state, .stopping)
        try await waitUntil(timeout: 20, "tap present to exit") { !self.isRunning(pid) }
        try await waitUntil(timeout: 5, "the talk to be idle") { presentation.state == .idle }
    }

    func testStopPresenting() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        controller.jumpToSlide(number: 2)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 2))
        XCTAssertEqual(presentation.lastSlide, 2, "the start slide until tap says otherwise")
        // Drive the deck through tap's hub, as the pages do, and hear tap's slide event.
        let client = try XCTUnwrap(presentation.client)
        XCTAssertNotNil(client.presenterCookie)
        let socket = client.openSocket()
        socket.resume()
        try await Task.sleep(nanoseconds: 300_000_000)
        socket.send(SlideMessage(slideIndex: 3, fragment: -1, step: 0))
        try await waitUntil(timeout: 10, "tap's slide event for slide 4") { presentation.lastSlide == 4 }
        socket.close()
        let pid = try XCTUnwrap(presentation.session?.processIdentifier)
        let devPid = controller.session.processIdentifier

        presentation.stop()
        XCTAssertEqual(presentation.state, .stopping)
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertNil(presentation.presenterWindow)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        try await waitUntil(timeout: 30, "the talk to end") { presentation.state == .idle }
        XCTAssertEqual(controller.currentSlideNumber, 4, "the cursor is on the last slide presented")
        try await waitUntil(timeout: 10, "tap present to exit") { !self.isRunning(pid) }
        XCTAssertFalse(NSApp.windows.contains { $0.isVisible && $0 is PresentationWindow })
        XCTAssertEqual(controller.session.processIdentifier, devPid, "tap dev is untouched")
        if case .running = controller.session.state {} else { XCTFail("tap dev keeps running the preview") }
    }

    func testStopWhileStartingOpensNoWindow() async throws {
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.silent()
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 10, "the process") { presentation.session?.processIdentifier != nil }
        let pid = try XCTUnwrap(presentation.session?.processIdentifier)
        presentation.stop()
        try await waitUntil(timeout: 20, "the talk to be idle") { presentation.state == .idle }
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        try await Task.sleep(nanoseconds: 3_500_000_000)
        XCTAssertNil(presentation.audienceWindow, "no window opens after the fallback either")
        XCTAssertEqual(presentation.state, .idle)
        XCTAssertFalse(isRunning(pid))
    }

    func testTheTalksLogIsListedInTheTapLogWindow() async throws {
        let (_, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let logWindow = TapLogWindowController.shared
        logWindow.reload()
        XCTAssertEqual(logWindow.picker.segmentCount, 2)
        XCTAssertEqual(logWindow.picker.label(forSegment: 1), "ops, talk")
        try await stopPresenting(controller)
        logWindow.reload()
        XCTAssertEqual(logWindow.picker.segmentCount, 1)
    }
}
```

- [ ] **Step 3: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/PresentingTests/testTheMacStaysAwake`
Expected: the test target does not compile (`presentation`, `PresentationController` are undefined).

- [ ] **Step 4: Write `PresentationController.swift`**

```swift
import AppKit
import WebKit

/// One deck's talk: the `tap present --app` process beside the deck's own
/// `tap dev --app`, the audience and presenter windows, the display sleep
/// assertion, and the slide the audience is on. tap dev keeps running the
/// preview the whole time; nothing here touches it.
///
/// The state moves idle, starting (the save, the process, the ready line,
/// the windows loading), presenting (the windows are on screen), stopping
/// (the windows are down and tap is quitting, which may take a
/// keep-recording answer), and back to idle; or to failed, with a message
/// for the person, when tap present cannot start or stops restarting.
@MainActor
final class PresentationController {
    enum State: Equatable {
        case idle
        case starting
        case presenting
        case stopping
        case failed(String)
    }

    struct PendingQuestion: Equatable {
        let id: String
        let kind: String
        let payload: QuestionPayload
    }

    private(set) var state: State = .idle {
        didSet { if state != oldValue { onStateChange?(state) } }
    }
    private(set) var options: PresentationOptions?
    private(set) var session: TapSession?
    private(set) var client: TapClient?
    private(set) var audienceWindow: PresentationWindow?
    private(set) var presenterWindow: PresentationWindow?
    /// The window last ordered in front of the other. On one display that
    /// is what Option-Tab switches.
    private(set) weak var frontWindow: PresentationWindow?
    private(set) var arrangement: DisplayArrangement?
    let sleepAssertion = SleepAssertion()
    /// The slide the audience is on, 1-based: the start slide until tap's
    /// first slide event, then the last event's slide.
    private(set) var lastSlide = 1
    private(set) var recording = RecordingStatus()
    /// True once the windows have been ordered front for this talk.
    private(set) var windowsShown = false
    /// A question tap asked that has not been answered. The windows are not
    /// shown while one is open at startup, so the sheet on the deck window
    /// has nothing over it.
    private(set) var pendingQuestion: PendingQuestion?
    /// Set once a page reported ready or failed to load, or the fallback
    /// fired: the windows may be shown as soon as no question is pending.
    private var pagesReported = false
    private var showWindowsFallback: DispatchWorkItem?

    var onStateChange: ((State) -> Void)?
    var onEvent: ((TapEvent) -> Void)?
    /// The talk ended, by Stop or a failure; this is the last slide the audience saw.
    var onStopped: ((_ lastSlide: Int) -> Void)?
    var onRecordingChange: ((RecordingStatus) -> Void)?
    var onQuestion: ((PendingQuestion) -> Void)?
    /// tap present could not start or stopped restarting.
    var onFailed: ((String) -> Void)?

    /// The deck file tap present reads: nil while the deck has no file.
    let deckURL: () -> URL?
    /// Writes the buffer to the deck file, then calls back. tap present
    /// reads the file, so this runs before the process starts. A test can
    /// wrap it.
    var saveDeck: (@escaping (Error?) -> Void) -> Void
    let sessionConfiguration: () -> TapSession.Configuration
    var displayAssignments: DisplayAssignmentStore
    /// The displays. Production reads NSScreen; tests hand in frames of their own.
    var screens: () -> [ScreenInfo] = { NSScreen.screens.map(ScreenInfo.init(screen:)) }
    /// The deck's window controller, which the talk's windows forward menu actions to.
    weak var deckWindowController: DeckWindowController?
    /// How long the windows wait for the first page to report before they
    /// are shown anyway: a page that never reports (no server, a load
    /// failure) must not keep the talk from starting.
    static let showWindowsFallbackInterval: TimeInterval = 3
    /// How long quit waits for tap that got ready (its own quit deadline is
    /// 8 s, plus 3 s for a keep-recording answer), and for one that never did.
    static let quitTimeout: TimeInterval = 15
    static let quitTimeoutBeforeReady: TimeInterval = 2

    init(deckURL: @escaping () -> URL?,
         saveDeck: @escaping (@escaping (Error?) -> Void) -> Void,
         sessionConfiguration: @escaping () -> TapSession.Configuration,
         displayAssignments: DisplayAssignmentStore) {
        self.deckURL = deckURL
        self.saveDeck = saveDeck
        self.sessionConfiguration = sessionConfiguration
        self.displayAssignments = displayAssignments
    }

    /// True from Play until the talk is idle or failed again.
    var isActive: Bool {
        switch state {
        case .starting, .presenting, .stopping: return true
        case .idle, .failed: return false
        }
    }

    /// Play and Rehearse need a deck file and no talk in progress.
    var canStart: Bool {
        deckURL() != nil && !isActive
    }

    /// The arrangement the next talk would use, for the popover.
    var currentArrangement: DisplayArrangement? {
        arrangement ?? DisplayArrangement.resolve(screens: screens(), store: displayAssignments)
    }

    // MARK: Start

    func start(_ options: PresentationOptions) {
        guard canStart, let deck = deckURL() else { return }
        self.options = options
        lastSlide = options.startSlide
        recording = RecordingStatus()
        pendingQuestion = nil
        pagesReported = false
        windowsShown = false
        state = .starting
        saveDeck { [weak self] error in
            guard let self, self.state == .starting else { return }
            if let error {
                self.fail("The deck could not be saved: \(error.localizedDescription)")
                return
            }
            self.launch(deck: deck, options: options)
        }
    }

    private func launch(deck: URL, options: PresentationOptions) {
        let session = TapSession(deckURL: deck, configuration: sessionConfiguration(), command: options.command)
        session.onStateChange = { [weak self] sessionState in self?.sessionStateChanged(sessionState) }
        session.onEvent = { [weak self] event in self?.handle(event) }
        self.session = session
        session.start()
    }

    private func sessionStateChanged(_ sessionState: TapSession.State) {
        switch sessionState {
        case .running(let ready):
            tapIsReady(ready)
        case .failed(let lastOutput):
            endBecauseTapFailed(lastOutput: lastOutput)
        case .stopped:
            if state == .stopping { finishStopping() }
        case .starting, .restarting:
            break
        }
    }

    /// tap present printed its ready line, at the start or after a restart.
    /// The presenter secret is traded for the hub's cookie first, so both
    /// pages' own WebSocket connections are relayed: the speaker's keys in
    /// either window move the other, and tap hears every position for its
    /// slide events and chapters.
    private func tapIsReady(_ ready: TapReady) {
        guard state == .starting || state == .presenting else { return }
        let client = TapClient(ready: ready)
        self.client = client
        Task { @MainActor [weak self] in
            var cookie: String?
            do {
                cookie = try await client.authorizePresenter()
            } catch {
                self?.session?.log.append("tap refused the presenter secret: \(error)", source: .app)
            }
            if let cookie { await Self.installPresenterCookie(cookie) }
            guard let self, self.client === client, self.state == .starting || self.state == .presenting else { return }
            self.openWindows(client: client)
        }
    }

    /// Puts the hub's presenter cookie into the web views' shared data
    /// store. Cookies ignore ports, so the one value for 127.0.0.1 is the
    /// present process's while a talk runs; the preview is driven by the
    /// app's own socket, which carries its cookie in a header, and never
    /// needs this one.
    static func installPresenterCookie(_ value: String) async {
        guard let cookie = HTTPCookie(properties: [.name: TapClient.presenterCookieName, .value: value,
                                                    .domain: "127.0.0.1", .path: "/"]) else { return }
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            WKWebsiteDataStore.default().httpCookieStore.setCookie(cookie) { continuation.resume() }
        }
    }

    /// Creates the windows on the arranged screens, or reuses them after a
    /// restart, and loads the pages at `lastSlide`. The assertion is held
    /// from here: tap is up and the windows exist.
    private func openWindows(client: TapClient) {
        guard let options else { return }
        guard let arrangement = DisplayArrangement.resolve(screens: screens(), store: displayAssignments) else {
            fail("No display is connected.")
            return
        }
        self.arrangement = arrangement
        sleepAssertion.acquire()
        if options.mode == .play {
            let audience = audienceWindow ?? makeWindow(role: .audience, frame: arrangement.audience.frame)
            audienceWindow = audience
            audience.cover(arrangement.audience.frame)
            audience.page.load(client.audienceLaunchURL(slide: lastSlide), allowedPort: client.ready.port)
        }
        let presenter = presenterWindow ?? makeWindow(role: .presenter, frame: arrangement.presenter.frame)
        presenterWindow = presenter
        presenter.cover(arrangement.presenter.frame)
        presenter.page.load(client.presenterURL(slide: lastSlide), allowedPort: client.ready.port)
        if !windowsShown { armShowWindowsFallback() }
    }

    private func makeWindow(role: PresentationWindow.Role, frame: CGRect) -> PresentationWindow {
        let window = PresentationWindow(role: role, screenFrame: frame)
        window.deckWindowController = deckWindowController
        window.page.onReady = { [weak self] _ in self?.pageReported() }
        window.page.onLoadFailed = { [weak self] _ in self?.pageReported() }
        window.page.onPresenterPopup = { [weak self] in self?.bringPresenterWindowForward() }
        return window
    }

    private func armShowWindowsFallback() {
        showWindowsFallback?.cancel()
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated { self?.pageReported() }
        }
        showWindowsFallback = work
        DispatchQueue.main.asyncAfter(deadline: .now() + Self.showWindowsFallbackInterval, execute: work)
    }

    /// A page reported ready or failed to load, or the fallback fired.
    private func pageReported() {
        pagesReported = true
        showWindowsIfReady()
    }

    private func showWindowsIfReady() {
        guard state == .starting, !windowsShown, pagesReported, pendingQuestion == nil else { return }
        showWindows()
    }

    /// Orders the talk's windows front. This is the one place presenting
    /// takes the screen: the person clicked Play in this app, and a window
    /// covering the projector is what the click means. On two displays each
    /// window is alone on its screen and the presenter window is key, since
    /// that is where the speaker's keys go. On one display the audience
    /// covers the screen and Option-Tab brings the presenter window over it.
    private func showWindows() {
        showWindowsFallback?.cancel()
        showWindowsFallback = nil
        guard let arrangement else { return }
        windowsShown = true
        if let audienceWindow, let presenterWindow, arrangement.isSingleDisplay {
            presenterWindow.orderFrontRegardless()
            audienceWindow.makeKeyAndOrderFront(nil)
            audienceWindow.orderFrontRegardless()
            frontWindow = audienceWindow
        } else {
            audienceWindow?.orderFrontRegardless()
            presenterWindow?.makeKeyAndOrderFront(nil)
            presenterWindow?.orderFrontRegardless()
            frontWindow = presenterWindow
        }
        state = .presenting
    }

    /// Option-Tab on one display: the other window comes over this one.
    func toggleFrontWindow() {
        guard let audienceWindow, let presenterWindow else { return }
        let next = frontWindow === audienceWindow ? presenterWindow : audienceWindow
        next.makeKeyAndOrderFront(nil)
        next.orderFrontRegardless()
        frontWindow = next
    }

    /// The S key in the audience page.
    func bringPresenterWindowForward() {
        guard let presenterWindow else { return }
        presenterWindow.makeKeyAndOrderFront(nil)
        presenterWindow.orderFrontRegardless()
        frontWindow = presenterWindow
    }

    // MARK: Events

    /// tap's stdout events. Internal so a test can deliver one through the
    /// same path the session uses.
    func handle(_ event: TapEvent) {
        switch event {
        case .slide(let slide, _):
            lastSlide = slide
        case .recording(let recordingEvent):
            recording.apply(recordingEvent)
            onRecordingChange?(recording)
        case .error(let payload) where payload.code == "recording_blocked":
            recording.blockedReason = payload.message
            onRecordingChange?(recording)
        case .question(let id, let kind, let payload):
            let question = PendingQuestion(id: id, kind: kind, payload: payload)
            pendingQuestion = question
            onQuestion?(question)
        default:
            break
        }
        onEvent?(event)
    }

    /// Answers the pending question with `id`, and lets the windows show
    /// if they were waiting on it.
    func answer(id: String, value: Bool) {
        guard pendingQuestion?.id == id else { return }
        pendingQuestion = nil
        session?.send(.answer(id: id, value: value))
        showWindowsIfReady()
    }

    // MARK: Stop

    /// Ends the talk: the windows go and the assertion is released at once,
    /// then tap is asked to quit, which may bring a keep-recording question
    /// before it exits.
    func stop() {
        guard state == .starting || state == .presenting else { return }
        state = .stopping
        takeDownWindows()
        guard let session else {
            finishStopping()
            return
        }
        let ready: Bool = { if case .running = session.state { return true } else { return false } }()
        session.quit(timeout: ready ? Self.quitTimeout : Self.quitTimeoutBeforeReady)
    }

    /// Every ending goes through here: Stop, a failed start, tap giving up,
    /// the deck window closing and the app quitting.
    private func takeDownWindows() {
        showWindowsFallback?.cancel()
        showWindowsFallback = nil
        for window in [audienceWindow, presenterWindow].compactMap({ $0 }) {
            window.orderOut(nil)
            window.close()
        }
        audienceWindow = nil
        presenterWindow = nil
        frontWindow = nil
        windowsShown = false
        sleepAssertion.release()
    }

    private func finishStopping() {
        session = nil
        client = nil
        pendingQuestion = nil
        state = .idle
        onStopped?(lastSlide)
    }

    /// tap present exited three times in thirty seconds, or never got ready.
    private func endBecauseTapFailed(lastOutput: [String]) {
        let summary = session?.restartPolicy.exitSummary ?? "tap present exited"
        let detail = lastOutput.last.map { "\(summary). Last output: \($0)" } ?? summary
        fail(detail)
        onStopped?(lastSlide)
    }

    private func fail(_ message: String) {
        takeDownWindows()
        session?.stop()
        session = nil
        client = nil
        pendingQuestion = nil
        state = .failed(message)
        onFailed?(message)
    }
}

extension ScreenInfo {
    /// A real display. The built-in flag comes from CoreGraphics, the name
    /// from the system ("Built-in Retina Display", "LG UltraFine").
    init(screen: NSScreen) {
        let number = screen.deviceDescription[NSDeviceDescriptionKey(rawValue: "NSScreenNumber")] as? NSNumber
        let displayID = CGDirectDisplayID(number?.uint32Value ?? 0)
        self.init(name: screen.localizedName, frame: screen.frame, isBuiltIn: CGDisplayIsBuiltin(displayID) != 0)
    }
}
```

- [ ] **Step 5: Wire the controller into the deck**

In `DeckSessionController.swift`, add after `var exchangePresenterSecret`:

```swift
    /// The deck's talk. Created on first use; `stop()` ends it with the deck.
    private(set) lazy var presentation: PresentationController = {
        let controller = PresentationController(
            deckURL: { [weak self] in self?.document?.fileURL },
            saveDeck: { [weak self] completion in
                guard let self else { return completion(nil) }
                self.saveForPresenting(completion: completion)
            },
            sessionConfiguration: { AppEnvironment.shared.presentSessionConfiguration() },
            displayAssignments: AppEnvironment.shared.displayAssignments)
        controller.onStopped = { [weak self] lastSlide in self?.jumpToSlide(number: lastSlide) }
        return controller
    }()

    /// Writes the buffer to the deck file before a talk, because tap
    /// present reads the file. A buffer that already equals the file needs
    /// no write. A save the document refuses (a disk conflict is showing)
    /// comes back as its error, and the talk does not start.
    func saveForPresenting(completion: @escaping (Error?) -> Void) {
        guard let document, let url = document.fileURL else { return completion(CocoaError(.fileNoSuchFile)) }
        guard isContentEdited else { return completion(nil) }
        document.save(to: url, ofType: document.fileType ?? "net.daringfireball.markdown", for: .saveOperation, completionHandler: completion)
    }
```

In `stop()`, add `presentation.stop()` right after `stopped = true`.

In `DeckWindowController.init`, after `shouldCascadeWindows = true`, add:

```swift
        sessionController.presentation.deckWindowController = self
```

In `AppEnvironment.swift`, add after `slidePasteboard`:

```swift
    /// Which display is the audience for each pair of displays, across
    /// decks. A test replaces this with a store on a fresh UserDefaults suite.
    var displayAssignments = DisplayAssignmentStore()
    /// A tap for talks alone, for tests that script tap present while the
    /// deck's real tap dev keeps running. nil runs the bundled tap.
    var presentExecutableURL: URL?
```

and after `sessionConfiguration()`:

```swift
    /// The session configuration for a talk: the same tap and environment as
    /// tap dev, unless a test named another executable for talks.
    func presentSessionConfiguration() -> TapSession.Configuration {
        TapSession.Configuration(executableURL: presentExecutableURL ?? tapExecutableURL, environment: { [weak self] in
            await self?.tapEnvironment() ?? ProcessInfo.processInfo.environment
        })
    }
```

In `AppDelegate.swift`, change `deck(owning:)` to:

```swift
    static func deck(owning window: NSWindow?) -> DeckWindowController? {
        if let deck = window?.windowController as? DeckWindowController { return deck }
        if let presentation = window as? PresentationWindow { return presentation.deckWindowController }
        return (window?.windowController as? PreviewWindowController)?.deckWindowController
    }
```

and add after `applicationShouldHandleReopen`:

```swift
    func applicationWillTerminate(_ notification: Notification) {
        Self.stopAllPresentations()
    }

    /// Ends every deck's talk: windows down, sleep assertions released, tap
    /// present told to quit. tap keeps a recording when its stdin closes
    /// without an answer, so quitting the app never loses one.
    static func stopAllPresentations() {
        for document in NSDocumentController.shared.documents {
            (document as? DeckDocument)?.sessionController?.presentation.stop()
        }
    }
```

In `TapLogWindowController.reload()`, replace the `logs = ...` line with:

```swift
        logs = NSDocumentController.shared.documents.flatMap { document -> [TapLog] in
            guard let controller = (document as? DeckDocument)?.sessionController else { return [] }
            return [controller.session.log] + (controller.presentation.session.map { [$0.log] } ?? [])
        }
```

- [ ] **Step 6: Run the tests one at a time**

Run: `make -C desktop project`, then:

```bash
make -C desktop test ONLY=TapTests/PresentingTests/testSaveBeforePresenting
make -C desktop test ONLY=TapTests/PresentingTests/testWindowsCoverTheScreenAndTheAudienceIsInFront
make -C desktop test ONLY=TapTests/PresentingTests/testTheMacStaysAwake
make -C desktop test ONLY=TapTests/PresentingTests/testTheSleepAssertionIsReleasedWhenTheDeckWindowCloses
make -C desktop test ONLY=TapTests/PresentingTests/testTheSleepAssertionIsReleasedWhenTapPresentDies
make -C desktop test ONLY=TapTests/PresentingTests/testTheSleepAssertionIsReleasedWhenTheAppQuits
make -C desktop test ONLY=TapTests/PresentingTests/testStopPresenting
make -C desktop test ONLY=TapTests/PresentingTests/testStopWhileStartingOpensNoWindow
make -C desktop test ONLY=TapTests/PresentingTests/testTheTalksLogIsListedInTheTapLogWindow
```

Expected: all nine pass. Each covers the screen for a few seconds. D2's and D3's suites are unchanged; run `make -C desktop test ONLY=TapTests/RestartTests` and `ONLY=TapTests/TapLogTests` to confirm the session and log changes broke nothing.

If `testStopPresenting` never sees `lastSlide == 4`: the hub relays only from a connection that carried the presenter cookie, so check `client.presenterCookie` is non-nil and that `TapClient.socketRequest()` still sets the `Cookie` header (D2). If the audience page never reports ready in `testWindowsCoverTheScreenAndTheAudienceIsInFront`, read `HostedTestCase.pageStateScript` through `audience.page.webView` the way `waitForPreview` does before changing anything.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted, the ones that can leave a screen covered or the assertion held first: in `takeDownWindows`, drop `sleepAssertion.release()` (expected: `testTheMacStaysAwake`, both close and quit tests and the crash test fail on `isHeld`); in `DeckSessionController.stop`, drop `presentation.stop()` (expected: the close test fails, the window stays visible); in `AppDelegate.applicationWillTerminate`, drop the call (expected: the quit test fails); in `endBecauseTapFailed`, skip `fail` (expected: the crash test times out on `.failed` with the assertion held); in `stop`, drop `takeDownWindows()` (expected: `testStopPresenting` fails on `audienceWindow`); in `start`, skip `saveDeck` and call `launch` directly (expected: `testSaveBeforePresenting` fails on the file); in `handle`, drop the `.slide` case (expected: `testStopPresenting` times out on `lastSlide`); in `showWindows`, order the presenter front on one display (expected: the window-order assertion fails); in `tapIsReady`, skip `installPresenterCookie` (expected: the cookie assertion fails); in `showWindowsIfReady`, drop `pendingQuestion == nil` (survives here; Task 9's consent test kills it).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): start and stop a talk with tap present, its windows and the sleep assertion"
```

---

### Task 5: Two displays: the arrangement, Swap Displays, the memory, Rehearse and an unplugged projector

**Files:**
- Modify: `desktop/Tap/Presenting/PresentationController.swift` (`swapDisplays`, `screensChanged`, the screen observer)
- Test: `desktop/TapTests/PresentingDisplayTests.swift`

**Interfaces:**
- Consumes: Task 4's controller, `PresentingTestCase.halfScreens()`, `oneScreen()`; Task 2's `DisplayArrangement.swapped()`, `DisplayAssignmentStore`.
- Produces: `PresentationController.swapDisplays()`, `screensChanged()`; `NSApplication.didChangeScreenParametersNotification` observed while a talk is active.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/PresentingDisplayTests.swift`:

```swift
import XCTest
@testable import Tap

/// Two displays on one screen: the left half is the laptop, the right half
/// the projector. The window server sees every window either way.
final class PresentingDisplayTests: PresentingTestCase {
    func testStartPresentingWithTwoDisplays() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        controller.presentation.screens = { screens }
        controller.jumpToSlide(number: 3)
        XCTAssertEqual(controller.currentSlideNumber, 3)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 3))

        let talk = try XCTUnwrap(presentation.session)
        XCTAssertEqual(talk.command, .present(record: true, presenterPassword: nil))
        XCTAssertTrue(talk.log.text.contains("tap present --app ops.md"))
        XCTAssertNotEqual(talk.processIdentifier, controller.session.processIdentifier, "a second process")
        if case .running = controller.session.state {} else { XCTFail("the preview keeps running from tap dev") }

        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.frame, screens[1].frame, "the audience covers the projector")
        XCTAssertEqual(presenter.frame, screens[0].frame, "the presenter view is on the built-in display")
        XCTAssertTrue(presentation.frontWindow === presenter, "the speaker's keys go to the presenter window")
        try await waitUntil(timeout: 5, "both windows on screen") {
            let order = onScreenWindowNumbers()
            return order.contains(audience.windowNumber) && order.contains(presenter.windowNumber)
        }
        try await waitUntil(timeout: 20, "the audience page on slide 3") { audience.page.lastReady?.slide == 3 }
        try await waitUntil(timeout: 20, "the presenter page on slide 3") { presenter.page.lastReady?.slide == 3 }
    }

    func testSwapDisplays() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        let presentation = controller.presentation
        presentation.screens = { screens }
        XCTAssertEqual(presentation.currentArrangement?.audience.name, "Projector")
        // Before the talk: the popover's Swap Displays.
        presentation.swapDisplays()
        XCTAssertEqual(presentation.currentArrangement?.audience.name, "Built-in Display")
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.frame, screens[0].frame, "the presenter and audience displays swapped before the start")
        XCTAssertEqual(presenter.frame, screens[1].frame)
        // During the talk: the toolbar's Swap Displays.
        presentation.swapDisplays()
        XCTAssertEqual(audience.frame, screens[1].frame)
        XCTAssertEqual(presenter.frame, screens[0].frame)
        try await waitUntil(timeout: 5, "both windows still on screen") {
            let order = onScreenWindowNumbers()
            return order.contains(audience.windowNumber) && order.contains(presenter.windowNumber)
        }
    }

    func testRememberTheDisplayAssignment() async throws {
        let (_, first) = try await openDeckForPresenting()
        let screens = halfScreens()
        first.presentation.screens = { screens }
        first.presentation.swapDisplays()
        try await startPresenting(first, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertEqual(first.presentation.audienceWindow?.frame, screens[0].frame)
        try await stopPresenting(first)
        XCTAssertEqual(AppEnvironment.shared.displayAssignments.audienceName(for: screens), "Built-in Display")

        // Another deck, the same pair of displays, in either order.
        let (_, second) = try await openDeckForPresenting()
        second.presentation.screens = { screens.reversed() }
        try await startPresenting(second, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertEqual(second.presentation.audienceWindow?.frame, screens[0].frame, "the same assignment, for every deck")
        XCTAssertEqual(second.presentation.presenterWindow?.frame, screens[1].frame)
    }

    func testRehearse() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        let presentation = controller.presentation
        presentation.screens = { screens }
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 4))
        let talk = try XCTUnwrap(presentation.session)
        XCTAssertEqual(talk.command, .present(record: false, presenterPassword: nil))
        XCTAssertTrue(talk.log.text.contains("tap present --app --no-record ops.md"))
        XCTAssertNil(presentation.audienceWindow, "only the presenter view")
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(presenter.frame, screens[0].frame, "full screen on the laptop")
        XCTAssertTrue(presentation.frontWindow === presenter)
        try await waitUntil(timeout: 20, "the presenter page, with its timer, on slide 4") { presenter.page.lastReady?.slide == 4 }
        XCTAssertFalse(presentation.recording.isRecording)
    }

    func testTheAudienceWindowFallsBackWhenTheProjectorGoes() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let two = halfScreens()
        let one = oneScreen()
        let presentation = controller.presentation
        presentation.screens = { two }
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)

        presentation.screens = { one }
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertEqual(audience.frame, one[0].frame, "the audience lands on the remaining screen")
        XCTAssertEqual(presenter.frame, one[0].frame)
        XCTAssertEqual(presentation.arrangement?.isSingleDisplay, true)
        try await waitUntil(timeout: 5, "the presenter window over the audience") {
            let order = onScreenWindowNumbers()
            guard let a = order.firstIndex(of: audience.windowNumber), let p = order.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
        XCTAssertTrue(presentation.frontWindow === presenter)
        XCTAssertTrue(presentation.sleepAssertion.isHeld)

        // The projector is back.
        presentation.screens = { two }
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertEqual(audience.frame, two[1].frame)
        XCTAssertEqual(presenter.frame, two[0].frame)
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/PresentingDisplayTests/testSwapDisplays`
Expected: the test target does not compile (`swapDisplays` is undefined).

- [ ] **Step 3: Swap, remember and follow the screens**

In `PresentationController.swift`, add a stored property after `showWindowsFallback`:

```swift
    private var screenObserver: NSObjectProtocol?
```

At the end of `init`, add:

```swift
        screenObserver = NotificationCenter.default.addObserver(forName: NSApplication.didChangeScreenParametersNotification, object: nil, queue: nil) { [weak self] _ in
            MainActor.assumeIsolated { self?.screensChanged() }
        }
```

Add after `bringPresenterWindowForward()`:

```swift
    // MARK: Displays

    /// Exchanges the audience and presenter displays, before the talk (the
    /// popover's Swap Displays) or during it (the toolbar's), and remembers
    /// the choice for this pair of displays, across decks. Nothing to swap
    /// on one display.
    func swapDisplays() {
        let screens = self.screens()
        guard let current = arrangement ?? DisplayArrangement.resolve(screens: screens, store: displayAssignments),
              !current.isSingleDisplay else { return }
        let swapped = current.swapped()
        displayAssignments.setAudienceName(swapped.audience.name, for: screens)
        guard isActive else { return }
        arrangement = swapped
        audienceWindow?.cover(swapped.audience.frame)
        presenterWindow?.cover(swapped.presenter.frame)
    }

    /// The displays changed while a talk runs: a projector unplugged or
    /// plugged back in. The windows follow the new arrangement; with one
    /// display left, the audience goes behind the presenter window, since
    /// the speaker is at the laptop.
    func screensChanged() {
        guard isActive, let resolved = DisplayArrangement.resolve(screens: screens(), store: displayAssignments) else { return }
        arrangement = resolved
        audienceWindow?.cover(resolved.audience.frame)
        presenterWindow?.cover(resolved.presenter.frame)
        if resolved.isSingleDisplay, let audienceWindow, let presenterWindow, windowsShown {
            presenterWindow.makeKeyAndOrderFront(nil)
            presenterWindow.orderFrontRegardless()
            audienceWindow.order(.below, relativeTo: presenterWindow.windowNumber)
            frontWindow = presenterWindow
        }
    }
```

The observer is removed in a `deinit`:

```swift
    deinit {
        if let screenObserver { NotificationCenter.default.removeObserver(screenObserver) }
    }
```

- [ ] **Step 4: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/PresentingDisplayTests/testStartPresentingWithTwoDisplays
make -C desktop test ONLY=TapTests/PresentingDisplayTests/testSwapDisplays
make -C desktop test ONLY=TapTests/PresentingDisplayTests/testRememberTheDisplayAssignment
make -C desktop test ONLY=TapTests/PresentingDisplayTests/testRehearse
make -C desktop test ONLY=TapTests/PresentingDisplayTests/testTheAudienceWindowFallsBackWhenTheProjectorGoes
```

Expected: all five pass.

- [ ] **Step 5: Mutate and commit**

Mutations, each reverted, the one that can leave a window off screen first: in `screensChanged`, drop the `cover` calls (expected: the fallback test fails on the frames); in `swapDisplays`, drop `setAudienceName` (expected: `testRememberTheDisplayAssignment` fails on the second deck); in `swapDisplays`, drop the `isActive` guard's early return so windows are covered while idle (survives, it is harmless; no claim); in `openWindows`, use `arrangement.presenter.frame` for the audience (expected: `testStartPresentingWithTwoDisplays` fails); in `PresentationOptions.command`, hard-code `record: true` (expected: `testRehearse` fails on the command).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): arrange a talk over two displays, swap them, remember the pair, and rehearse on one"
```

---

### Task 6: The Play toolbar button and the Present popover

**Files:**
- Create: `desktop/Tap/Presenting/DisplayArrangementView.swift`
- Create: `desktop/Tap/Presenting/PresentPopoverController.swift`
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (the Play toolbar item, `play`, `playClicked(modifiers:)`, `rehearse`, `startPresenting(_:)`, `refreshPresentingControls`, `presentPopover`)
- Test: `desktop/TapTests/PresentPopoverTests.swift`

**Interfaces:**
- Consumes: Task 4's `PresentationController` (`currentArrangement`, `start`, `canStart`, `onStateChange`, `swapDisplays`), Task 2's `PresentationOptions`, `DisplayArrangement`; D3's `HoverButton` pattern and `LayoutGalleryController`'s popover pattern.
- Produces: `DisplayArrangementView` with `presenterBox`, `audienceBox` (each a `ScreenBox` with `roleLabel`, `nameLabel`), `update(arrangement:)`; `PresentPopoverController` with `Context(arrangement:cursorSlide:)`, `isShown`, `arrangementView`, `singleDisplayLabel`, `swapButton`, `startFromControl`, `recordCheckbox`, `phoneRemoteCheckbox`, `advancedButton`, `advancedStack`, `passwordField`, `tunnelCheckbox`, `rehearseButton`, `startButton`, `onSwap`, `onStart`, `onRehearse`, `show(context:relativeTo:of:)`, `update(context:)`, `close()`, `options(mode:)`; `DeckWindowController.playItemIdentifier`, `playButton`, `presentPopover`, `play(_:)`, `playClicked(modifiers:)`, `rehearse(_:)`, `startPresenting(_:)`, `refreshPresentingControls()`, `popoverContext()`.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/PresentPopoverTests.swift`:

```swift
import XCTest
@testable import Tap

final class PresentPopoverTests: PresentingTestCase {
    func windowController(_ controller: DeckSessionController) throws -> DeckWindowController {
        try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
    }

    func testStartFromTheFirstSlide() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        controller.jumpToSlide(number: 3)
        deckWindow.playClicked(modifiers: [.shift])
        XCTAssertFalse(deckWindow.presentPopover.isShown, "Shift-click starts at once")
        XCTAssertEqual(controller.presentation.options?.mode, .play)
        XCTAssertEqual(controller.presentation.options?.startSlide, 1)
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        let audience = try XCTUnwrap(controller.presentation.audienceWindow)
        try await waitUntil(timeout: 20, "the audience page on slide 1") { audience.page.lastReady?.slide == 1 }
        XCTAssertEqual(controller.currentSlideNumber, 3, "the cursor stays where it was")
    }

    func testThePopoverCollectsTheOptions() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let screens = halfScreens()
        controller.presentation.screens = { screens }
        controller.jumpToSlide(number: 3)
        deckWindow.playClicked(modifiers: [])
        let popover = deckWindow.presentPopover
        XCTAssertTrue(popover.isShown)
        XCTAssertEqual(popover.startFromControl.label(forSegment: 0), "Slide 3")
        XCTAssertEqual(popover.startFromControl.label(forSegment: 1), "Slide 1")
        XCTAssertEqual(popover.startFromControl.selectedSegment, 0)
        XCTAssertEqual(popover.arrangementView.audienceBox.nameLabel.stringValue, "Projector")
        XCTAssertEqual(popover.arrangementView.presenterBox.nameLabel.stringValue, "Built-in Display")
        XCTAssertFalse(popover.arrangementView.isHidden)
        XCTAssertTrue(popover.singleDisplayLabel.isHidden)
        XCTAssertTrue(popover.swapButton.isEnabled)
        XCTAssertEqual(popover.recordCheckbox.state, .on)
        XCTAssertEqual(popover.phoneRemoteCheckbox.state, .off)
        XCTAssertTrue(popover.advancedStack.isHidden)

        popover.swapButton.performClick(nil)
        XCTAssertEqual(popover.arrangementView.audienceBox.nameLabel.stringValue, "Built-in Display", "Swap Displays swaps before the start")
        XCTAssertEqual(controller.presentation.currentArrangement?.audience.name, "Built-in Display")

        popover.advancedButton.performClick(nil)
        XCTAssertFalse(popover.advancedStack.isHidden)
        popover.recordCheckbox.state = .off
        popover.passwordField.stringValue = "secret"
        popover.tunnelCheckbox.state = .on
        let advanced = popover.options(mode: .play)
        XCTAssertEqual(advanced, PresentationOptions(mode: .play, startSlide: 3, record: false, phoneRemote: false, tunnel: true, presenterPassword: "secret"))
        popover.tunnelCheckbox.state = .off
        popover.phoneRemoteCheckbox.performClick(nil)
        XCTAssertEqual(popover.phoneRemoteCheckbox.state, .on)
        XCTAssertEqual(popover.tunnelCheckbox.state, .on, "the phone remote is the tunnel")
        XCTAssertFalse(popover.tunnelCheckbox.isEnabled)
        popover.phoneRemoteCheckbox.performClick(nil)
        XCTAssertTrue(popover.tunnelCheckbox.isEnabled)
        popover.tunnelCheckbox.state = .off
        popover.passwordField.stringValue = ""
        popover.startFromControl.selectedSegment = 1
        XCTAssertEqual(popover.options(mode: .play), PresentationOptions(mode: .play, startSlide: 1, record: false))

        popover.startButton.performClick(nil)
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(controller.presentation.options, PresentationOptions(mode: .play, startSlide: 1, record: false))
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        XCTAssertEqual(controller.presentation.session?.command, .present(record: false, presenterPassword: nil))
        XCTAssertEqual(controller.presentation.audienceWindow?.frame, screens[0].frame, "the swap held")
    }

    func testThePopoverWithOneDisplaySaysSo() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        deckWindow.playClicked(modifiers: [])
        let popover = deckWindow.presentPopover
        XCTAssertTrue(popover.isShown)
        XCTAssertTrue(popover.arrangementView.isHidden)
        XCTAssertFalse(popover.singleDisplayLabel.isHidden)
        XCTAssertFalse(popover.swapButton.isEnabled)
        popover.rehearseButton.performClick(nil)
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(controller.presentation.options?.mode, .rehearse)
        XCTAssertEqual(controller.presentation.options?.startSlide, 1)
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
    }

    func testThePlayButtonFollowsTheTalk() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let identifiers = try XCTUnwrap(deckWindow.window?.toolbar?.items.map(\.itemIdentifier))
        XCTAssertTrue(identifiers.contains(DeckWindowController.playItemIdentifier))
        XCTAssertEqual(deckWindow.playButton.accessibilityIdentifier(), "play-button")
        XCTAssertTrue(deckWindow.playButton.isEnabled)
        deckWindow.rehearse(nil)
        XCTAssertFalse(deckWindow.playButton.isEnabled, "no second talk while one runs")
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
        XCTAssertTrue(deckWindow.playButton.isEnabled)
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/PresentPopoverTests/testThePopoverWithOneDisplaySaysSo`
Expected: the test target does not compile (`playClicked`, `presentPopover` are undefined).

- [ ] **Step 3: Write `DisplayArrangementView.swift`**

```swift
import AppKit

/// The popover's picture of the displays: two boxes, each the shape of its
/// screen, labelled with its role and its name.
final class DisplayArrangementView: NSView {
    final class ScreenBox: NSView {
        let roleLabel = NSTextField(labelWithString: "")
        let nameLabel = NSTextField(labelWithString: "")
        private var aspect: NSLayoutConstraint?

        init(role: String, accessibilityIdentifier: String) {
            super.init(frame: .zero)
            wantsLayer = true
            layer?.cornerRadius = 8
            layer?.borderWidth = 1
            layer?.borderColor = NSColor.separatorColor.cgColor
            layer?.backgroundColor = NSColor.controlBackgroundColor.cgColor
            roleLabel.stringValue = role
            roleLabel.font = .systemFont(ofSize: 11, weight: .semibold)
            nameLabel.font = .systemFont(ofSize: 11)
            nameLabel.textColor = .secondaryLabelColor
            nameLabel.lineBreakMode = .byTruncatingTail
            let stack = NSStackView(views: [roleLabel, nameLabel])
            stack.orientation = .vertical
            stack.alignment = .centerX
            stack.spacing = 2
            stack.translatesAutoresizingMaskIntoConstraints = false
            addSubview(stack)
            NSLayoutConstraint.activate([
                stack.centerXAnchor.constraint(equalTo: centerXAnchor),
                stack.centerYAnchor.constraint(equalTo: centerYAnchor),
                stack.leadingAnchor.constraint(greaterThanOrEqualTo: leadingAnchor, constant: 6),
                widthAnchor.constraint(equalToConstant: 150),
            ])
            setAccessibilityElement(true)
            setAccessibilityRole(.group)
            setAccessibilityIdentifier(accessibilityIdentifier)
        }

        required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

        func show(_ screen: ScreenInfo) {
            nameLabel.stringValue = screen.name
            setAccessibilityLabel("\(roleLabel.stringValue), \(screen.name)")
            aspect?.isActive = false
            let ratio = screen.frame.width > 0 ? max(screen.frame.height / screen.frame.width, 0.3) : 0.625
            aspect = heightAnchor.constraint(equalTo: widthAnchor, multiplier: ratio)
            aspect?.isActive = true
        }
    }

    let presenterBox = ScreenBox(role: "Presenter view", accessibilityIdentifier: "presenter-screen")
    let audienceBox = ScreenBox(role: "Audience, full screen", accessibilityIdentifier: "audience-screen")

    override init(frame: NSRect) {
        super.init(frame: frame)
        let row = NSStackView(views: [presenterBox, audienceBox])
        row.orientation = .horizontal
        row.alignment = .bottom
        row.spacing = 12
        row.translatesAutoresizingMaskIntoConstraints = false
        addSubview(row)
        NSLayoutConstraint.activate([
            row.topAnchor.constraint(equalTo: topAnchor),
            row.leadingAnchor.constraint(equalTo: leadingAnchor),
            row.trailingAnchor.constraint(equalTo: trailingAnchor),
            row.bottomAnchor.constraint(equalTo: bottomAnchor),
        ])
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func update(arrangement: DisplayArrangement) {
        presenterBox.show(arrangement.presenter)
        audienceBox.show(arrangement.audience)
    }
}
```

- [ ] **Step 4: Write `PresentPopoverController.swift`**

```swift
import AppKit

/// The Present popover: which display is which, Swap Displays, where to
/// start, recording, the phone remote, and the Advanced options, with
/// Rehearse and Start Presenting. It collects `PresentationOptions`; the
/// deck's window controller starts the talk.
@MainActor
final class PresentPopoverController: NSObject, NSPopoverDelegate {
    struct Context {
        let arrangement: DisplayArrangement?
        let cursorSlide: Int
    }

    /// Kept here rather than read from the popover, whose `isShown` can
    /// depend on whether the app is active (D3's gallery found the same).
    private(set) var isShown = false
    let arrangementView = DisplayArrangementView(frame: .zero)
    let singleDisplayLabel = NSTextField(wrappingLabelWithString: "One display: the audience fills the screen, and Option-Tab shows the presenter view.")
    let swapButton = NSButton(title: "Swap Displays", target: nil, action: nil)
    let startFromControl = NSSegmentedControl(labels: ["Slide 1", "Slide 1"], trackingMode: .selectOne, target: nil, action: nil)
    let recordCheckbox = NSButton(checkboxWithTitle: "Record the talk", target: nil, action: nil)
    let recordHint = NSTextField(wrappingLabelWithString: "Records from the start until you stop, when recording is on for you.")
    let phoneRemoteCheckbox = NSButton(checkboxWithTitle: "Phone remote", target: nil, action: nil)
    let advancedButton = NSButton(title: "Advanced", target: nil, action: nil)
    let advancedStack = NSStackView()
    let passwordField = NSTextField()
    let tunnelCheckbox = NSButton(checkboxWithTitle: "Public tunnel", target: nil, action: nil)
    let tunnelHint = NSTextField(labelWithString: "Needs cloudflared.")
    let rehearseButton = NSButton(title: "Rehearse", target: nil, action: nil)
    let startButton = NSButton(title: "Start Presenting", target: nil, action: nil)
    var onSwap: (() -> Void)?
    var onStart: ((PresentationOptions) -> Void)?
    var onRehearse: ((PresentationOptions) -> Void)?
    private let popover = NSPopover()
    private var context = Context(arrangement: nil, cursorSlide: 1)

    override init() {
        super.init()
        singleDisplayLabel.font = .systemFont(ofSize: 12)
        singleDisplayLabel.textColor = .secondaryLabelColor
        swapButton.bezelStyle = .rounded
        swapButton.target = self
        swapButton.action = #selector(swapPressed(_:))
        startFromControl.selectedSegment = 0
        recordCheckbox.state = .on
        recordHint.font = .systemFont(ofSize: 11)
        recordHint.textColor = .secondaryLabelColor
        phoneRemoteCheckbox.target = self
        phoneRemoteCheckbox.action = #selector(phoneRemoteChanged(_:))
        // A disclosure button draws its triangle and no title; the label beside it says Advanced.
        advancedButton.bezelStyle = .disclosure
        advancedButton.setButtonType(.pushOnPushOff)
        advancedButton.title = ""
        advancedButton.target = self
        advancedButton.action = #selector(advancedPressed(_:))
        advancedButton.setAccessibilityLabel("Advanced")
        let advancedRow = NSStackView(views: [advancedButton, NSTextField(labelWithString: "Advanced")])
        advancedRow.orientation = .horizontal
        advancedRow.spacing = 4
        passwordField.placeholderString = "None"
        passwordField.setAccessibilityIdentifier("presenter-password")
        tunnelHint.font = .systemFont(ofSize: 11)
        tunnelHint.textColor = .secondaryLabelColor
        rehearseButton.bezelStyle = .rounded
        rehearseButton.target = self
        rehearseButton.action = #selector(rehearsePressed(_:))
        startButton.bezelStyle = .rounded
        startButton.keyEquivalent = "\r"
        startButton.target = self
        startButton.action = #selector(startPressed(_:))
        startButton.setAccessibilityIdentifier("start-presenting")

        let passwordRow = NSStackView(views: [NSTextField(labelWithString: "Presenter password"), passwordField])
        passwordRow.orientation = .horizontal
        passwordField.widthAnchor.constraint(equalToConstant: 160).isActive = true
        let tunnelRow = NSStackView(views: [tunnelCheckbox, tunnelHint])
        tunnelRow.orientation = .horizontal
        advancedStack.orientation = .vertical
        advancedStack.alignment = .leading
        advancedStack.spacing = 6
        advancedStack.addArrangedSubview(passwordRow)
        advancedStack.addArrangedSubview(tunnelRow)
        advancedStack.isHidden = true

        let startRow = NSStackView(views: [NSTextField(labelWithString: "Start from"), startFromControl])
        startRow.orientation = .horizontal
        let buttons = NSStackView(views: [NSView(), rehearseButton, startButton])
        buttons.orientation = .horizontal
        let stack = NSStackView(views: [arrangementView, singleDisplayLabel, swapButton, startRow, recordCheckbox, recordHint,
                                        phoneRemoteCheckbox, advancedRow, advancedStack, buttons])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.edgeInsets = NSEdgeInsets(top: 16, left: 16, bottom: 16, right: 16)
        stack.widthAnchor.constraint(equalToConstant: 360).isActive = true
        buttons.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -32).isActive = true
        singleDisplayLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -32).isActive = true
        recordHint.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -32).isActive = true
        let content = NSViewController()
        content.view = stack
        popover.contentViewController = content
        popover.behavior = .transient
        popover.delegate = self
        stack.setAccessibilityIdentifier("present-popover")
    }

    func show(context: Context, relativeTo rect: NSRect, of view: NSView) {
        update(context: context)
        popover.show(relativeTo: rect, of: view, preferredEdge: .maxY)
        isShown = true
    }

    /// The displays or the cursor changed, or a swap happened.
    func update(context: Context) {
        self.context = context
        startFromControl.setLabel("Slide \(context.cursorSlide)", forSegment: 0)
        startFromControl.setLabel("Slide 1", forSegment: 1)
        if let arrangement = context.arrangement, !arrangement.isSingleDisplay {
            arrangementView.update(arrangement: arrangement)
            arrangementView.isHidden = false
            singleDisplayLabel.isHidden = true
            swapButton.isEnabled = true
        } else {
            arrangementView.isHidden = true
            singleDisplayLabel.isHidden = false
            swapButton.isEnabled = false
        }
    }

    func close() {
        isShown = false
        popover.performClose(nil)
    }

    func popoverDidClose(_ notification: Notification) {
        isShown = false
    }

    /// What the controls say now.
    func options(mode: PresentationMode) -> PresentationOptions {
        let password = passwordField.stringValue.trimmingCharacters(in: .whitespaces)
        return PresentationOptions(mode: mode,
                                   startSlide: startFromControl.selectedSegment == 1 ? 1 : context.cursorSlide,
                                   record: recordCheckbox.state == .on,
                                   phoneRemote: phoneRemoteCheckbox.state == .on,
                                   tunnel: tunnelCheckbox.state == .on,
                                   presenterPassword: password.isEmpty ? nil : password)
    }

    @objc private func swapPressed(_ sender: Any?) {
        onSwap?()
    }

    @objc private func advancedPressed(_ sender: Any?) {
        advancedStack.isHidden = advancedButton.state == .off
    }

    /// The phone remote is the tunnel with a QR code, so the tunnel switch follows it.
    @objc private func phoneRemoteChanged(_ sender: Any?) {
        let on = phoneRemoteCheckbox.state == .on
        if on { tunnelCheckbox.state = .on }
        tunnelCheckbox.isEnabled = !on
    }

    @objc private func startPressed(_ sender: Any?) {
        let options = options(mode: .play)
        close()
        onStart?(options)
    }

    @objc private func rehearsePressed(_ sender: Any?) {
        let options = options(mode: .rehearse)
        close()
        onRehearse?(options)
    }
}
```

- [ ] **Step 5: The Play button and the actions in the window controller**

In `DeckWindowController.swift`, add after `static let newSlideItemIdentifier`:

```swift
    static let playItemIdentifier = NSToolbarItem.Identifier("play")
```

After `let newSlideButton = NewSlideButton()`:

```swift
    /// The toolbar's Play button: a click opens the Present popover, a Shift-click starts from slide 1.
    let playButton = NSButton()
    private(set) lazy var presentPopover: PresentPopoverController = {
        let popover = PresentPopoverController()
        popover.onSwap = { [weak self] in
            guard let self else { return }
            self.sessionController.presentation.swapDisplays()
            self.presentPopover.update(context: self.popoverContext())
        }
        popover.onStart = { [weak self] options in self?.startPresenting(options) }
        popover.onRehearse = { [weak self] options in self?.startPresenting(options) }
        return popover
    }()
```

In `init`, after `sessionController.presentation.deckWindowController = self` (Task 4), add:

```swift
        sessionController.presentation.onStateChange = { [weak self] _ in self?.refreshPresentingControls() }
```

Add after `showLayoutGallery(_:)`:

```swift
    // MARK: Presenting

    /// The Play button and Present > Play. A Shift-click starts from slide 1 at once.
    @objc func play(_ sender: Any?) {
        playClicked(modifiers: NSApp.currentEvent?.modifierFlags ?? [])
    }

    func playClicked(modifiers: NSEvent.ModifierFlags) {
        guard sessionController.presentation.canStart else { return }
        if modifiers.contains(.shift) {
            startPresenting(PresentationOptions(mode: .play, startSlide: 1))
            return
        }
        let anchor: NSView = playButton.window == nil ? (window?.contentView ?? playButton) : playButton
        presentPopover.show(context: popoverContext(), relativeTo: anchor.bounds, of: anchor)
    }

    /// Present > Rehearse: the presenter view alone, from the cursor's slide.
    @objc func rehearse(_ sender: Any?) {
        guard sessionController.presentation.canStart else { return }
        startPresenting(PresentationOptions(mode: .rehearse, startSlide: sessionController.currentSlideNumber ?? 1))
    }

    /// Every start comes here: the popover's buttons, the Shift-click and Rehearse.
    func startPresenting(_ options: PresentationOptions) {
        sessionController.presentation.start(options)
        refreshPresentingControls()
    }

    func popoverContext() -> PresentPopoverController.Context {
        PresentPopoverController.Context(arrangement: sessionController.presentation.currentArrangement,
                                         cursorSlide: sessionController.currentSlideNumber ?? 1)
    }

    /// The toolbar's Play button follows the talk: off while one runs.
    func refreshPresentingControls() {
        playButton.isEnabled = sessionController.presentation.canStart
    }
```

In `toolbarDefaultItemIdentifiers`, return `[Self.slidesItemIdentifier, .flexibleSpace, Self.newSlideItemIdentifier, Self.playItemIdentifier, Self.previewItemIdentifier]`. In `toolbar(_:itemForItemIdentifier:willBeInsertedIntoToolbar:)`, add before the `guard identifier == Self.previewItemIdentifier` line:

```swift
        if identifier == Self.playItemIdentifier {
            let item = NSToolbarItem(itemIdentifier: identifier)
            item.label = "Play"
            item.toolTip = "Present from this slide. Shift-click to start from slide 1."
            playButton.image = NSImage(systemSymbolName: "play.fill", accessibilityDescription: "Play")
            playButton.bezelStyle = .toolbar
            playButton.setAccessibilityIdentifier("play-button")
            playButton.target = self
            playButton.action = #selector(play(_:))
            playButton.isEnabled = sessionController.presentation.canStart
            item.view = playButton
            return item
        }
```

- [ ] **Step 6: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/PresentPopoverTests/testStartFromTheFirstSlide
make -C desktop test ONLY=TapTests/PresentPopoverTests/testThePopoverCollectsTheOptions
make -C desktop test ONLY=TapTests/PresentPopoverTests/testThePopoverWithOneDisplaySaysSo
make -C desktop test ONLY=TapTests/PresentPopoverTests/testThePlayButtonFollowsTheTalk
```

Expected: all four pass. Also run `make -C desktop test ONLY=TapTests/MenuTests` and `ONLY=TapTests/WindowLayoutTests`: the toolbar gained an item and the menu is unchanged so far.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted: in `playClicked`, drop the `.shift` branch (expected: `testStartFromTheFirstSlide` fails, the popover shows); in `options(mode:)`, ignore `startFromControl` (expected: `testThePopoverCollectsTheOptions` fails on `startSlide: 1`); in `phoneRemoteChanged`, drop `tunnelCheckbox.state = .on` (expected: it fails on the tunnel state); in `update(context:)`, never hide `arrangementView` (expected: `testThePopoverWithOneDisplaySaysSo` fails); in `refreshPresentingControls`, always enable the button (expected: `testThePlayButtonFollowsTheTalk` fails); in `onSwap`, drop `swapDisplays()` (expected: the options test fails on `currentArrangement`).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the Play button and the Present popover with displays, start slide, recording and remote options"
```

---

### Task 7: The Present menu, Escape and Option-Tab, the S key, and every tap dev key

**Files:**
- Modify: `desktop/Tap/App/MainMenu.swift` (the Present menu)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (`stopPresenting`, `swapDisplays`, validation)
- Modify: `desktop/Tap/Presenting/PresentationController.swift` (the key monitor, `handleKey`)
- Test: `desktop/TapTests/PresentMenuTests.swift`

**Interfaces:**
- Consumes: Task 4's controller (`stop`, `toggleFrontWindow`, `bringPresenterWindowForward`, `isActive`, `canStart`, `currentArrangement`), Task 6's `play`, `rehearse`; Task 3's `PresentationWindow.role`, `page.popupRequested`.
- Produces: Present menu items Play (Cmd+Option+P), Rehearse (Cmd+Option+Shift+P), Stop (Cmd+.), Swap Displays; `DeckWindowController.stopPresenting(_:)`, `swapDisplays(_:)`, validation for the five; `PresentationController.handleKey(_:)` and the local monitor installed while the windows show.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/PresentMenuTests.swift`:

```swift
import XCTest
import WebKit
@testable import Tap

final class PresentMenuTests: PresentingTestCase {
    func presentMenu() throws -> NSMenu {
        try XCTUnwrap(NSApp.mainMenu?.items.first { $0.submenu?.title == "Present" }?.submenu)
    }

    func item(_ menu: NSMenu, action: Selector) throws -> NSMenuItem {
        try XCTUnwrap(menu.items.first { $0.action == action }, "no item with \(action)")
    }

    func keyEvent(_ type: NSEvent.EventType, characters: String, keyCode: UInt16, modifiers: NSEvent.ModifierFlags = [], in window: NSWindow) throws -> NSEvent {
        try XCTUnwrap(NSEvent.keyEvent(with: type, location: .zero, modifierFlags: modifiers, timestamp: ProcessInfo.processInfo.systemUptime,
                                       windowNumber: window.windowNumber, context: nil, characters: characters,
                                       charactersIgnoringModifiers: characters, isARepeat: false, keyCode: keyCode))
    }

    func testPresentingShortcuts() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let menu = try presentMenu()
        let play = try item(menu, action: #selector(DeckWindowController.play(_:)))
        XCTAssertEqual(play.title, "Play")
        XCTAssertEqual(play.keyEquivalent, "p")
        XCTAssertEqual(play.keyEquivalentModifierMask, [.command, .option])
        let rehearse = try item(menu, action: #selector(DeckWindowController.rehearse(_:)))
        XCTAssertEqual(rehearse.keyEquivalent, "p")
        XCTAssertEqual(rehearse.keyEquivalentModifierMask, [.command, .option, .shift])
        let stop = try item(menu, action: #selector(DeckWindowController.stopPresenting(_:)))
        XCTAssertEqual(stop.keyEquivalent, ".")
        XCTAssertEqual(stop.keyEquivalentModifierMask, [.command])
        let swap = try item(menu, action: #selector(DeckWindowController.swapDisplays(_:)))

        XCTAssertTrue(deckWindow.validateMenuItem(play))
        XCTAssertTrue(deckWindow.validateMenuItem(rehearse))
        XCTAssertFalse(deckWindow.validateMenuItem(stop))
        XCTAssertFalse(deckWindow.validateMenuItem(swap), "one display: nothing to swap")

        // Cmd+Option+Shift+P starts rehearsing; Cmd+Option+P opens the popover to start presenting.
        deckWindow.rehearse(nil)
        XCTAssertEqual(controller.presentation.options?.mode, .rehearse)
        XCTAssertFalse(deckWindow.validateMenuItem(play))
        XCTAssertFalse(deckWindow.validateMenuItem(rehearse))
        XCTAssertTrue(deckWindow.validateMenuItem(stop))
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
        deckWindow.stopPresenting(nil)
        try await waitUntil(timeout: 30, "the end") { controller.presentation.state == .idle }
        deckWindow.play(nil)
        XCTAssertTrue(deckWindow.presentPopover.isShown)
        deckWindow.presentPopover.close()
    }

    func testOneDisplay() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.frame, NSScreen.screens[0].frame, "the audience page fills the screen")
        XCTAssertTrue(presentation.frontWindow === audience)
        let optionTab = try keyEvent(.keyDown, characters: "\t", keyCode: 48, modifiers: [.option], in: audience)
        XCTAssertNil(presentation.handleKey(optionTab), "the app takes Option-Tab")
        XCTAssertTrue(presentation.frontWindow === presenter)
        try await waitUntil(timeout: 5, "the presenter window in front") {
            let order = onScreenWindowNumbers()
            guard let a = order.firstIndex(of: audience.windowNumber), let p = order.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
        let again = try keyEvent(.keyDown, characters: "\t", keyCode: 48, modifiers: [.option], in: presenter)
        XCTAssertNil(presentation.handleKey(again))
        XCTAssertTrue(presentation.frontWindow === audience)
        let plainTab = try keyEvent(.keyDown, characters: "\t", keyCode: 48, in: audience)
        XCTAssertNotNil(presentation.handleKey(plainTab), "a plain Tab goes to the page")
        let elsewhere = try keyEvent(.keyDown, characters: "\t", keyCode: 48, modifiers: [.option], in: try XCTUnwrap(controller.editor.window))
        XCTAssertNotNil(presentation.handleKey(elsewhere), "Option-Tab in the deck window is not the app's")
    }

    func testEscapeInTheAudienceWindowStopsTheTalk() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let escapeInPresenter = try keyEvent(.keyDown, characters: "\u{1b}", keyCode: 53, in: presenter)
        XCTAssertNotNil(presentation.handleKey(escapeInPresenter), "Escape in the presenter window is the page's while the audience window exists")
        XCTAssertEqual(presentation.state, .presenting)
        let escape = try keyEvent(.keyDown, characters: "\u{1b}", keyCode: 53, in: audience)
        XCTAssertNil(presentation.handleKey(escape))
        XCTAssertEqual(presentation.state, .stopping)
        try await waitUntil(timeout: 30, "the end") { presentation.state == .idle }

        // A rehearsal has no audience window: Escape in the presenter window stops it.
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let rehearsal = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertNil(presentation.handleKey(try keyEvent(.keyDown, characters: "\u{1b}", keyCode: 53, in: rehearsal)))
        XCTAssertEqual(presentation.state, .stopping)
    }

    func testEveryTapDevPresenterFeatureWorks() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 2))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        for page in [audience.page, presenter.page] {
            XCTAssertTrue(page.webView.configuration.preferences.isElementFullscreenEnabled, "F goes full screen")
            XCTAssertTrue(page.webView.configuration.websiteDataStore.isPersistent, "the presenter layout and notes size persist between launches")
        }
        try await waitUntil(timeout: 20, "the audience page on slide 2") { audience.page.lastReady?.slide == 2 }
        try await waitUntil(timeout: 20, "the presenter page on slide 2") { presenter.page.lastReady?.slide == 2 }

        // The arrow keys, and every other key, go to tap's page unchanged: the
        // page moves, the hub relays it (the page holds the presenter cookie),
        // and tap reports the new position.
        let rightArrow = String(Character(Unicode.Scalar(UInt16(NSRightArrowFunctionKey))!))
        audience.page.webView.keyDown(with: try keyEvent(.keyDown, characters: rightArrow, keyCode: 124, in: audience))
        audience.page.webView.keyUp(with: try keyEvent(.keyUp, characters: rightArrow, keyCode: 124, in: audience))
        try await waitUntil(timeout: 10, "tap's slide event for slide 3") { presentation.lastSlide == 3 }
        try await waitUntil(timeout: 10, "the presenter page following") { presenter.page.lastReady?.slide == 3 }

        // S opens the presenter view in the presenter window, never a browser popup.
        let port = try XCTUnwrap(presentation.client).ready.port
        audience.page.popupRequested(for: URL(string: "http://127.0.0.1:\(port)/presenter#3"), navigationType: .other)
        XCTAssertTrue(presentation.frontWindow === presenter)
        try await waitUntil(timeout: 5, "the presenter window in front") {
            let order = onScreenWindowNumbers()
            guard let a = order.firstIndex(of: audience.windowNumber), let p = order.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/PresentMenuTests/testEscapeInTheAudienceWindowStopsTheTalk`
Expected: the test target does not compile (`handleKey`, `stopPresenting` are undefined).

- [ ] **Step 3: The Present menu and its actions**

In `MainMenu.swift`, replace `presentMenu()` with:

```swift
    static func presentMenu() -> NSMenu {
        let menu = NSMenu(title: "Present")
        menu.addItem(item("Play", action: #selector(DeckWindowController.play(_:)), key: "p", modifiers: [.command, .option]))
        menu.addItem(item("Rehearse", action: #selector(DeckWindowController.rehearse(_:)), key: "p", modifiers: [.command, .option, .shift]))
        menu.addItem(.separator())
        menu.addItem(item("Stop", action: #selector(DeckWindowController.stopPresenting(_:)), key: "."))
        menu.addItem(item("Swap Displays", action: #selector(DeckWindowController.swapDisplays(_:))))
        return menu
    }
```

In `DeckWindowController.swift`, add after `refreshPresentingControls()`:

```swift
    /// Present > Stop, the toolbar's Stop, and Escape in the audience window.
    @objc func stopPresenting(_ sender: Any?) {
        sessionController.presentation.stop()
    }

    /// Present > Swap Displays and the toolbar's: during the talk the
    /// windows change places; before it the popover's arrangement does.
    @objc func swapDisplays(_ sender: Any?) {
        sessionController.presentation.swapDisplays()
        if presentPopover.isShown { presentPopover.update(context: popoverContext()) }
    }
```

In `validateMenuItem(_:)`, add before `let count = ...`:

```swift
        let presentation = sessionController.presentation
        if [#selector(play(_:)), #selector(rehearse(_:))].contains(menuItem.action) { return presentation.canStart }
        if menuItem.action == #selector(stopPresenting(_:)) { return presentation.isActive }
        if menuItem.action == #selector(swapDisplays(_:)) { return presentation.currentArrangement?.isSingleDisplay == false }
```

- [ ] **Step 4: The key monitor**

In `PresentationController.swift`, add a stored property after `screenObserver`:

```swift
    private var keyMonitor: Any?
```

Add after `bringPresenterWindowForward()`:

```swift
    // MARK: Keys

    /// Escape in the audience window ends the talk (in the presenter window
    /// too, when there is no audience window: a rehearsal), and Option-Tab
    /// brings the other window over this one. Every other key goes to
    /// tap's page unchanged. Internal so a test can drive it with an event
    /// of its own; the monitor calls it for every key down while the
    /// windows show.
    func handleKey(_ event: NSEvent) -> NSEvent? {
        guard isActive, let window = event.window as? PresentationWindow,
              window === audienceWindow || window === presenterWindow else { return event }
        if event.keyCode == 53, window.role == .audience || audienceWindow == nil {
            stop()
            return nil
        }
        if event.keyCode == 48, event.modifierFlags.contains(.option) {
            toggleFrontWindow()
            return nil
        }
        return event
    }

    private func installKeyMonitor() {
        guard keyMonitor == nil else { return }
        keyMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            guard let self else { return event }
            return self.handleKey(event)
        }
    }

    private func removeKeyMonitor() {
        if let keyMonitor { NSEvent.removeMonitor(keyMonitor) }
        keyMonitor = nil
    }
```

In `showWindows()`, add `installKeyMonitor()` right after `windowsShown = true`. In `takeDownWindows()`, add `removeKeyMonitor()` as its first line.

- [ ] **Step 5: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/PresentMenuTests/testPresentingShortcuts
make -C desktop test ONLY=TapTests/PresentMenuTests/testOneDisplay
make -C desktop test ONLY=TapTests/PresentMenuTests/testEscapeInTheAudienceWindowStopsTheTalk
make -C desktop test ONLY=TapTests/PresentMenuTests/testEveryTapDevPresenterFeatureWorks
make -C desktop test ONLY=TapTests/MenuTests
```

Expected: all pass. If `testEveryTapDevPresenterFeatureWorks` never sees `lastSlide == 3`: first check the presenter cookie is in the store (`presenterCookieInTheSharedStore()`); if it is, the key did not reach the web content. Then make the audience window's first responder the web view (`audience.makeFirstResponder(audience.page.webView)`) and send the events through `audience.sendEvent(_:)` instead of `keyDown(with:)`. If neither moves the page, the UI test in Task 14 is what proves the keys, and this test keeps the cookie, the configuration and the S key assertions and drives the position through the app's socket as `testStopPresenting` does; record it in the ledger.

- [ ] **Step 6: Mutate and commit**

Mutations, each reverted, the ones that can leave a screen covered first: in `handleKey`, drop the Escape branch (expected: `testEscapeInTheAudienceWindowStopsTheTalk` fails); in `handleKey`, stop on Escape in any presentation window (expected: it fails on the presenter window's Escape); in `takeDownWindows`, drop `removeKeyMonitor()` (survives here: the guard on `isActive` makes a stale monitor inert; note it, not a claim); in `handleKey`, drop the `.option` check (expected: `testOneDisplay` fails on the plain Tab); in `validateMenuItem`, return true for Stop always (expected: `testPresentingShortcuts` fails); in `presentMenu`, give Stop the key `"s"` (expected: it fails on the key equivalent).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the Present menu, Escape and Option-Tab in the talk windows, and the S key"
```

---

### Task 8: The presenter toolbar, the REC dot, the edits the audience has not seen, Reload Slides and the idle cursor

**Files:**
- Create: `desktop/Tap/Presenting/PresenterToolbar.swift`
- Modify: `desktop/Tap/Presenting/PresentationWindow.swift` (the toolbar and the dot in the presenter window, the top-edge tracking)
- Modify: `desktop/Tap/Presenting/PresentationController.swift` (`editsNotShown`, `presentedText`, `deckTextChanged`, `reloadSlides`, the cursor timer, `refreshPresenterToolbar`)
- Modify: `desktop/Tap/Documents/DeckSessionController.swift` (`applySlideList` reports the text; `saveForPresenting` records it)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (`reloadSlides(_:)`, the toolbar's actions)
- Modify: `desktop/Tap/App/MainMenu.swift` (Reload Slides)
- Modify: `desktop/TapTests/PresentingTests.swift` (`testTheMacStaysAwake` gains the cursor)
- Test: `desktop/TapTests/PresenterToolbarTests.swift`

**Interfaces:**
- Consumes: Task 4's controller and `saveDeck`, Task 3's `PresentationWindow.container`, Task 2's `RecordingStatus`, `PresentationMode`; D3's `DeckSessionController.applySlideList` and `lastAppliedText`.
- Produces: `PresenterToolbar` with `titleLabel`, `recordButton`, `editsLabel`, `reloadButton`, `swapButton`, `stopButton`, `isShown`, `hideDelay`, `onRecord`, `onReload`, `onSwap`, `onStop`, `pointerReachedTopEdge()`, `pointerLeft()`, `update(recording:editsNotShown:mode:)`; `RecordingDot`; `PresentationWindow.toolbar`, `recordingDot`, `onMouseMoved`; `PresentationController.editsNotShown`, `presentedText`, `deckTextChanged(_:)`, `reloadSlides()`, `cursorHideDelay`, `hideCursor`, `isCursorHideArmed`, `noteMouseMoved()`, `toggleRecording()`; `DeckWindowController.reloadSlides(_:)`; the Present menu item Reload Slides (Cmd+R).

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/PresenterToolbarTests.swift`:

```swift
import XCTest
@testable import Tap

final class PresenterToolbarTests: PresentingTestCase {
    func testPresenterControls() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let toolbar = try XCTUnwrap(presenter.toolbar)
        let dot = try XCTUnwrap(presenter.recordingDot)
        XCTAssertNil(presentation.audienceWindow?.toolbar, "the audience window has no toolbar")
        XCTAssertTrue(toolbar.isHidden, "the toolbar is out of sight until the pointer reaches the top edge")
        XCTAssertFalse(toolbar.isShown)
        XCTAssertTrue(dot.isHidden, "no REC dot while nothing records")

        toolbar.pointerReachedTopEdge()
        XCTAssertTrue(toolbar.isShown)
        XCTAssertFalse(toolbar.isHidden)
        XCTAssertEqual(toolbar.recordButton.title, "NOT RECORDING")
        XCTAssertEqual(toolbar.reloadButton.title, "Reload Slides")
        XCTAssertEqual(toolbar.swapButton.title, "Swap Displays")
        XCTAssertEqual(toolbar.stopButton.title, "Stop")
        XCTAssertTrue(toolbar.editsLabel.isHidden)
        XCTAssertEqual(toolbar.frame.maxY, presenter.container.bounds.maxY, "it sits along the top edge")

        toolbar.hideDelay = 0.1
        toolbar.pointerLeft()
        try await waitUntil(timeout: 2, "the toolbar to slide away") { toolbar.isHidden }
        toolbar.pointerReachedTopEdge()
        toolbar.pointerLeft()
        toolbar.pointerReachedTopEdge()
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertFalse(toolbar.isHidden, "coming back cancels the hide")

        // The REC dot stays visible in a corner at all times while recording.
        presentation.handle(.recording(RecordingEvent(state: "recording", segment: 1, elapsed: 5, disk: "ok")))
        XCTAssertFalse(dot.isHidden)
        XCTAssertEqual(toolbar.recordButton.title, "REC 0:05")
        XCTAssertEqual(dot.frame.maxY, presenter.container.bounds.maxY - 16, accuracy: 1)
        XCTAssertEqual(dot.frame.maxX, presenter.container.bounds.maxX - 18, accuracy: 1)
        toolbar.pointerLeft()
        try await waitUntil(timeout: 2, "the toolbar away again") { toolbar.isHidden }
        XCTAssertFalse(dot.isHidden, "the dot does not go with the toolbar")
        presentation.handle(.recording(RecordingEvent(state: "stopped", segment: 1, elapsed: 0, disk: "ok")))
        XCTAssertTrue(dot.isHidden)

        toolbar.pointerReachedTopEdge()
        toolbar.stopButton.performClick(nil)
        XCTAssertEqual(presentation.state, .stopping)
    }

    func testARehearsalHasNoRecordingOrSwapControls() async throws {
        let (_, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let toolbar = try XCTUnwrap(controller.presentation.presenterWindow?.toolbar)
        toolbar.pointerReachedTopEdge()
        XCTAssertEqual(toolbar.titleLabel.stringValue, "Rehearsal")
        XCTAssertTrue(toolbar.recordButton.isHidden)
        XCTAssertTrue(toolbar.swapButton.isHidden)
        XCTAssertFalse(toolbar.reloadButton.isHidden)
        XCTAssertFalse(toolbar.stopButton.isHidden)
    }

    func testEditWhilePresenting() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        controller.jumpToSlide(number: 1)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let toolbar = try XCTUnwrap(presentation.presenterWindow?.toolbar)
        try await waitUntil(timeout: 20, "the audience page on slide 1") { audience.page.lastReady?.slide == 1 }
        var text = await audience.page.pageText()
        XCTAssertTrue(text.contains("One"))
        XCTAssertEqual(presentation.editsNotShown, 0)

        // An edit reaches tap dev, never tap present.
        let end = (controller.editor.string as NSString).range(of: "# One").upperBound
        controller.editor.setSelectedRange(NSRange(location: end, length: 0))
        controller.editor.insertText(" edited", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 15, "tap dev's answer") { controller.lastAppliedText?.contains("# One edited") == true }
        try await Task.sleep(nanoseconds: 500_000_000)
        text = await audience.page.pageText()
        XCTAssertFalse(text.contains("One edited"), "the audience does not see the change: tap present does not watch files")
        XCTAssertEqual(presentation.editsNotShown, 1)
        toolbar.pointerReachedTopEdge()
        XCTAssertFalse(toolbar.editsLabel.isHidden)
        XCTAssertEqual(toolbar.editsLabel.stringValue, "1 edit not shown")

        // Another typing pause counts again; undoing back to the presented text counts nothing.
        controller.editor.insertText("!", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 15, "the second answer") { controller.lastAppliedText?.contains("# One edited!") == true }
        XCTAssertEqual(presentation.editsNotShown, 2)
        XCTAssertEqual(toolbar.editsLabel.stringValue, "2 edits not shown")

        // Present > Reload Slides saves and reloads, as r does in tap present.
        deckWindow.reloadSlides(nil)
        // waitUntil takes a synchronous condition, and reading the page is async.
        let deadline = Date().addingTimeInterval(20)
        var shown = await audience.page.pageText()
        while !shown.contains("One edited!") {
            if Date() > deadline {
                XCTFail("the audience never showed the edit: \(shown)")
                throw CancellationError()
            }
            try await Task.sleep(nanoseconds: 200_000_000)
            shown = await audience.page.pageText()
        }
        XCTAssertEqual(presentation.editsNotShown, 0)
        XCTAssertTrue(toolbar.editsLabel.isHidden)
        XCTAssertTrue(try String(contentsOf: deck, encoding: .utf8).contains("# One edited!"), "Reload Slides saved the buffer first")
        XCTAssertFalse(document.isDocumentEdited)
    }
}
```

In `desktop/TapTests/PresentingTests.swift`, inside `testTheMacStaysAwake`, add before `try await stopPresenting(controller)`:

```swift
        // The cursor hides once the pointer has rested on a talk window.
        presentation.cursorHideDelay = 0.1
        var hides = 0
        presentation.hideCursor = { hides += 1 }
        presentation.noteMouseMoved()
        XCTAssertTrue(presentation.isCursorHideArmed)
        try await waitUntil(timeout: 2, "the cursor to hide") { hides == 1 }
        XCTAssertFalse(presentation.isCursorHideArmed)
        presentation.noteMouseMoved()
        presentation.noteMouseMoved()
        try await waitUntil(timeout: 2, "the cursor to hide once more") { hides == 2 }
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/PresenterToolbarTests/testARehearsalHasNoRecordingOrSwapControls`
Expected: the test target does not compile (`toolbar` is undefined on `PresentationWindow`).

- [ ] **Step 3: Write `PresenterToolbar.swift`**

```swift
import AppKit

/// The app's toolbar over the presenter page: REC, the edits the audience
/// has not seen, Reload Slides, Swap Displays and Stop. It is out of sight
/// while the speaker talks and slides in when the pointer reaches the top
/// edge, then slides away once the pointer has left it.
final class PresenterToolbar: NSView {
    static let height: CGFloat = 52
    let titleLabel = NSTextField(labelWithString: "")
    let recordButton = NSButton(title: "NOT RECORDING", target: nil, action: nil)
    let editsLabel = NSTextField(labelWithString: "")
    let reloadButton = NSButton(title: "Reload Slides", target: nil, action: nil)
    let swapButton = NSButton(title: "Swap Displays", target: nil, action: nil)
    let stopButton = NSButton(title: "Stop", target: nil, action: nil)
    /// How long the pointer must be away before the toolbar slides off. Tests shorten it.
    var hideDelay: TimeInterval = 1.5
    private(set) var isShown = false
    var onRecord: (() -> Void)?
    var onReload: (() -> Void)?
    var onSwap: (() -> Void)?
    var onStop: (() -> Void)?
    private var hideWork: DispatchWorkItem?

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
        layer?.backgroundColor = NSColor(white: 0.12, alpha: 0.92).cgColor
        setAccessibilityIdentifier("presenter-toolbar")
        titleLabel.font = .systemFont(ofSize: 13, weight: .semibold)
        titleLabel.textColor = .white
        editsLabel.font = .systemFont(ofSize: 12)
        editsLabel.textColor = .secondaryLabelColor
        for button in [recordButton, reloadButton, swapButton, stopButton] {
            button.bezelStyle = .rounded
            button.target = self
        }
        recordButton.action = #selector(recordPressed(_:))
        reloadButton.action = #selector(reloadPressed(_:))
        swapButton.action = #selector(swapPressed(_:))
        stopButton.action = #selector(stopPressed(_:))
        stopButton.hasDestructiveAction = true
        recordButton.setAccessibilityIdentifier("record-button")
        reloadButton.setAccessibilityIdentifier("reload-slides-button")
        swapButton.setAccessibilityIdentifier("swap-displays-button")
        stopButton.setAccessibilityIdentifier("stop-button")
        let row = NSStackView(views: [titleLabel, recordButton, NSView(), editsLabel, reloadButton, swapButton, stopButton])
        row.orientation = .horizontal
        row.spacing = 10
        row.edgeInsets = NSEdgeInsets(top: 0, left: 16, bottom: 0, right: 16)
        row.translatesAutoresizingMaskIntoConstraints = false
        addSubview(row)
        NSLayoutConstraint.activate([
            row.topAnchor.constraint(equalTo: topAnchor),
            row.leadingAnchor.constraint(equalTo: leadingAnchor),
            row.trailingAnchor.constraint(equalTo: trailingAnchor),
            row.bottomAnchor.constraint(equalTo: bottomAnchor),
        ])
        isHidden = true
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// The pointer touched the top edge: the toolbar comes in and stays while the pointer is on it.
    func pointerReachedTopEdge() {
        hideWork?.cancel()
        hideWork = nil
        isShown = true
        isHidden = false
    }

    /// The pointer left the toolbar: it goes after `hideDelay`, unless the pointer comes back first.
    func pointerLeft() {
        guard isShown else { return }
        hideWork?.cancel()
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated {
                guard let self else { return }
                self.isShown = false
                self.isHidden = true
            }
        }
        hideWork = work
        DispatchQueue.main.asyncAfter(deadline: .now() + hideDelay, execute: work)
    }

    /// What the talk is now. A rehearsal has no recording and nothing to swap.
    func update(recording: RecordingStatus, editsNotShown: Int, mode: PresentationMode) {
        titleLabel.stringValue = mode == .rehearse ? "Rehearsal" : "Presenting"
        recordButton.title = recording.label
        recordButton.isHidden = mode == .rehearse
        swapButton.isHidden = mode == .rehearse
        editsLabel.isHidden = editsNotShown == 0
        editsLabel.stringValue = editsNotShown == 1 ? "1 edit not shown" : "\(editsNotShown) edits not shown"
    }

    @objc private func recordPressed(_ sender: Any?) { onRecord?() }
    @objc private func reloadPressed(_ sender: Any?) { onReload?() }
    @objc private func swapPressed(_ sender: Any?) { onSwap?() }
    @objc private func stopPressed(_ sender: Any?) { onStop?() }
}

/// The small red dot with REC that stays in the top-right corner of the
/// presenter window for as long as tap records.
final class RecordingDot: NSView {
    private let label = NSTextField(labelWithString: "REC")

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
        layer?.cornerRadius = 12
        layer?.backgroundColor = NSColor(white: 0, alpha: 0.55).cgColor
        setAccessibilityIdentifier("recording-dot")
        let dot = NSView()
        dot.wantsLayer = true
        dot.layer?.cornerRadius = 4
        dot.layer?.backgroundColor = NSColor.systemRed.cgColor
        dot.translatesAutoresizingMaskIntoConstraints = false
        label.font = .systemFont(ofSize: 12, weight: .bold)
        label.textColor = NSColor(red: 1, green: 0.41, blue: 0.38, alpha: 1)
        let row = NSStackView(views: [dot, label])
        row.orientation = .horizontal
        row.spacing = 6
        row.edgeInsets = NSEdgeInsets(top: 4, left: 10, bottom: 4, right: 10)
        row.translatesAutoresizingMaskIntoConstraints = false
        addSubview(row)
        NSLayoutConstraint.activate([
            dot.widthAnchor.constraint(equalToConstant: 8),
            dot.heightAnchor.constraint(equalToConstant: 8),
            row.topAnchor.constraint(equalTo: topAnchor),
            row.leadingAnchor.constraint(equalTo: leadingAnchor),
            row.trailingAnchor.constraint(equalTo: trailingAnchor),
            row.bottomAnchor.constraint(equalTo: bottomAnchor),
        ])
        isHidden = true
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }
}
```

- [ ] **Step 4: The toolbar in the presenter window**

In `PresentationWindow.swift`, replace `let container = NSView()` with:

```swift
    /// Holds the page and, in the presenter window, the toolbar and the REC dot over it.
    let container = PresentationContentView()
    /// The presenter window's toolbar; nil in the audience window.
    private(set) var toolbar: PresenterToolbar?
    /// The presenter window's REC dot; nil in the audience window.
    private(set) var recordingDot: RecordingDot?
    /// The pointer moved over this window.
    var onMouseMoved: (() -> Void)?
```

At the end of `init`, before `container.layoutSubtreeIfNeeded()`, add:

```swift
        if role == .presenter {
            let toolbar = PresenterToolbar(frame: .zero)
            let dot = RecordingDot(frame: .zero)
            for view in [toolbar, dot] as [NSView] {
                view.translatesAutoresizingMaskIntoConstraints = false
                container.addSubview(view)
            }
            NSLayoutConstraint.activate([
                toolbar.topAnchor.constraint(equalTo: container.topAnchor),
                toolbar.leadingAnchor.constraint(equalTo: container.leadingAnchor),
                toolbar.trailingAnchor.constraint(equalTo: container.trailingAnchor),
                toolbar.heightAnchor.constraint(equalToConstant: PresenterToolbar.height),
                dot.topAnchor.constraint(equalTo: container.topAnchor, constant: 16),
                dot.trailingAnchor.constraint(equalTo: container.trailingAnchor, constant: -18),
            ])
            self.toolbar = toolbar
            recordingDot = dot
        }
        container.onMouseMoved = { [weak self] point in
            guard let self else { return }
            self.onMouseMoved?()
            guard let toolbar = self.toolbar else { return }
            if point.y >= self.container.bounds.height - 2 {
                toolbar.pointerReachedTopEdge()
            } else if point.y < self.container.bounds.height - PresenterToolbar.height {
                toolbar.pointerLeft()
            }
        }
```

Add at the end of the file:

```swift
/// The talk window's content view: it tracks the pointer everywhere in the
/// window, so the toolbar can slide in at the top edge and the cursor can
/// hide when idle.
final class PresentationContentView: NSView {
    var onMouseMoved: ((NSPoint) -> Void)?
    private var trackingArea: NSTrackingArea?

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let trackingArea { removeTrackingArea(trackingArea) }
        let area = NSTrackingArea(rect: bounds, options: [.mouseMoved, .activeAlways, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        trackingArea = area
    }

    override func mouseMoved(with event: NSEvent) {
        onMouseMoved?(convert(event.locationInWindow, from: nil))
    }
}
```

- [ ] **Step 5: The controller's side**

In `PresentationController.swift`, add stored properties after `pagesReported`:

```swift
    /// How many typing pauses tap dev has answered for since tap present
    /// last read the file, so the toolbar can say what the audience has not
    /// seen. Zero again when the text equals what was presented.
    private(set) var editsNotShown = 0
    /// The deck text tap present last read: at the start and after Reload Slides.
    var presentedText: String?
    /// How long the pointer rests on a talk window before the cursor hides.
    var cursorHideDelay: TimeInterval = 3
    /// Hides the cursor until the mouse moves. A test replaces it.
    var hideCursor: () -> Void = { NSCursor.setHiddenUntilMouseMoves(true) }
    private var cursorHideWork: DispatchWorkItem?
    var isCursorHideArmed: Bool { cursorHideWork != nil }
```

In `start(_:)`, add `editsNotShown = 0` after `recording = RecordingStatus()`. In `makeWindow`, add after the `onPresenterPopup` line:

```swift
        window.onMouseMoved = { [weak self] in self?.noteMouseMoved() }
        if let toolbar = window.toolbar {
            toolbar.onRecord = { [weak self] in self?.toggleRecording() }
            toolbar.onReload = { [weak self] in self?.reloadSlides() }
            toolbar.onSwap = { [weak self] in self?.swapDisplays() }
            toolbar.onStop = { [weak self] in self?.stop() }
        }
```

In `openWindows`, add `refreshPresenterToolbar()` as the last line. In `handle(_:)`, add `refreshPresenterToolbar()` after each `onRecordingChange?(recording)`. In `takeDownWindows`, add `cursorHideWork?.cancel()` and `cursorHideWork = nil` before `sleepAssertion.release()`. Add after the `// MARK: Keys` section:

```swift
    // MARK: The presenter toolbar

    func refreshPresenterToolbar() {
        guard let presenterWindow, let options else { return }
        presenterWindow.toolbar?.update(recording: recording, editsNotShown: editsNotShown, mode: options.mode)
        presenterWindow.recordingDot?.isHidden = !recording.isRecording
    }

    /// tap dev answered for `text`: an edit the audience has not seen, unless
    /// the text is back to what tap present read.
    func deckTextChanged(_ text: String) {
        guard isActive else { return }
        if text == presentedText {
            editsNotShown = 0
        } else {
            editsNotShown += 1
        }
        refreshPresenterToolbar()
    }

    /// Reload Slides: the buffer goes to the file, then tap present reads
    /// it again, as r does.
    func reloadSlides() {
        guard state == .presenting else { return }
        saveDeck { [weak self] error in
            guard let self, self.state == .presenting else { return }
            if let error {
                self.session?.log.append("Reload Slides could not save the deck: \(error.localizedDescription)", source: .app)
                return
            }
            self.session?.send(.reload)
            self.editsNotShown = 0
            self.refreshPresenterToolbar()
        }
    }

    /// REC: stop a recording, or start a new segment, as c does.
    func toggleRecording() {
        guard isActive else { return }
        session?.send(.recording(action: recording.isRecording ? .stop : .newSegment))
    }

    /// The pointer moved over a talk window: the cursor hides again after it rests.
    func noteMouseMoved() {
        guard isActive else { return }
        cursorHideWork?.cancel()
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated {
                guard let self else { return }
                self.cursorHideWork = nil
                self.hideCursor()
            }
        }
        cursorHideWork = work
        DispatchQueue.main.asyncAfter(deadline: .now() + cursorHideDelay, execute: work)
    }
```

In `DeckSessionController.swift`, replace `saveForPresenting` with this version, which records the text that reaches the file whether or not a write is needed:

```swift
    func saveForPresenting(completion: @escaping (Error?) -> Void) {
        guard let document, let url = document.fileURL else { return completion(CocoaError(.fileNoSuchFile)) }
        presentation.presentedText = editor.string
        guard isContentEdited else { return completion(nil) }
        document.save(to: url, ofType: document.fileType ?? "net.daringfireball.markdown", for: .saveOperation, completionHandler: completion)
    }
```

In `applySlideList`, add after `lastAppliedText = sentText`:

```swift
        presentation.deckTextChanged(sentText)
```

In `DeckWindowController.swift`, add after `swapDisplays(_:)`:

```swift
    /// Present > Reload Slides and the toolbar's.
    @objc func reloadSlides(_ sender: Any?) {
        sessionController.presentation.reloadSlides()
    }
```

and in `validateMenuItem`, add `if menuItem.action == #selector(reloadSlides(_:)) { return presentation.state == .presenting }` next to the Stop line. In `MainMenu.presentMenu()`, add after the Stop item:

```swift
        menu.addItem(item("Reload Slides", action: #selector(DeckWindowController.reloadSlides(_:)), key: "r"))
```

- [ ] **Step 6: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/PresenterToolbarTests/testPresenterControls
make -C desktop test ONLY=TapTests/PresenterToolbarTests/testARehearsalHasNoRecordingOrSwapControls
make -C desktop test ONLY=TapTests/PresenterToolbarTests/testEditWhilePresenting
make -C desktop test ONLY=TapTests/PresentingTests/testTheMacStaysAwake
make -C desktop test ONLY=TapTests/PresentMenuTests/testPresentingShortcuts
```

Expected: all pass. If `testEditWhilePresenting` never shows the edit after Reload Slides, check `tap present --app` logged `reload` in the talk's log (`presentation.session?.log.text`) and that the file holds the edit; a `reload_failed` error event names tap's reason.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted: in `reloadSlides`, skip `saveDeck` and send `.reload` at once (expected: `testEditWhilePresenting` fails: the file lacks the edit, so the audience never shows it); in `deckTextChanged`, drop the `text == presentedText` branch (survives: the test never undoes; add an undo step to the test if wanted, or leave as a written property); in `update(recording:editsNotShown:mode:)`, never hide `recordButton` (expected: the rehearsal test fails); in `pointerLeft`, drop the delayed hide (expected: `testPresenterControls` times out); in `refreshPresenterToolbar`, drop the dot line (expected: it fails on `dot.isHidden`); in `noteMouseMoved`, drop `cursorHideWork?.cancel()` (expected: `testTheMacStaysAwake` counts 3 hides, not 2).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the presenter toolbar, the REC dot, the edits counter, Reload Slides and the idle cursor"
```

---

### Task 9: Sheets for tap's questions: the recording consent, and the recording state as tap reports it

**Files:**
- Create: `desktop/Tap/Presenting/QuestionSheet.swift`
- Modify: `desktop/Tap/Presenting/PresentationController.swift` (the windows step aside for a sheet, the recording timer)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (`presentQuestion`, `showQuestionSheet`, `questionSheet`)
- Modify: `desktop/TapTests/Support/FakeTapScripts.swift` (`presenting(...)`, `onePixelPNG`)
- Test: `desktop/TapTests/RecordingTests.swift`

**Interfaces:**
- Consumes: Task 4's `PendingQuestion`, `onQuestion`, `answer(id:value:)`, `windowsShown`, `frontWindow`; Task 8's `refreshPresenterToolbar`, `PresenterToolbar.recordButton`, `RecordingDot`; Task 1's `QuestionPayload`, `RecordingEvent`.
- Produces: `QuestionSheet(kind:title:body:path:decline:accept:)` with `kind`, `titleLabel`, `bodyLabel`, `pathLabel`, `declineButton`, `acceptButton`, `button(titled:)`, `static consent(settingsPath:)`; `PresentationController.hideWindowsSharingScreen(with:)`, `restoreHiddenWindows()`, `windowsHiddenForQuestion`; `DeckWindowController.questionSheet`, `presentQuestion(_:)`, `showQuestionSheet(_:completion:)`; `FakeTapScripts.presenting(events:quit:recordingTo:)`, `QuitBehavior` (`.exit`, `.askToKeep(directory:segments:)`, `.askToKeepThenExit(after:directory:segments:)`), `tunnelUnavailable`, `onePixelPNG`.

- [ ] **Step 1: The scripted tap present**

Add to `desktop/TapTests/Support/FakeTapScripts.swift`, inside the enum:

```swift
    /// What the scripted tap present does with a quit command.
    enum QuitBehavior {
        /// Exits at once, as a run with no recording does.
        case exit
        /// Asks keep-recording and exits on the answer.
        case askToKeep(directory: URL, segments: Int)
        /// Asks keep-recording and exits after `seconds` whatever comes, as
        /// tap does once its three-second wait is over.
        case askToKeepThenExit(after: TimeInterval, directory: URL, segments: Int)
    }

    /// A 1 by 1 PNG, base64: the smallest QR code a fake can send.
    static let onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

    /// A scripted `tap present --app`: it records its arguments and every
    /// stdin line in `record`, prints a ready line (with no server behind
    /// it), then `events` one per line, and answers commands the way tap
    /// does: a tunnel start with a running tunnel (or, with
    /// `tunnelUnavailable`, the error tap sends without cloudflared), a
    /// tunnel stop with a stopped tunnel, a recording stop with a stopped
    /// recording, a new segment with segment 2 recording, and quit as
    /// `quit` says.
    static func presenting(events: [String], quit: QuitBehavior = .exit, tunnelUnavailable: Bool = false, recordingTo record: URL) throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        let eventLines = events.map { "echo '\($0)'" }.joined(separator: "\n")
        let tunnelRunning = tunnelUnavailable
            ? #"echo '{"type":"error","code":"tunnel_unavailable","message":"the tunnel needs cloudflared: brew install cloudflared"}'"#
            : #"echo '{"type":"tunnel","state":"starting"}'; echo '{"type":"tunnel","state":"running","url":"https://stark-lake-1234.trycloudflare.com","qr":"\#(onePixelPNG)"}'"#
        let onQuit: String
        switch quit {
        case .exit:
            onQuit = "exit 0"
        case .askToKeep(let directory, let segments):
            onQuit = #"echo '{"type":"question","id":"q1","kind":"keep-recording","payload":{"directory":"\#(directory.path)","segments":\#(segments)}}'"#
        case .askToKeepThenExit(let seconds, let directory, let segments):
            onQuit = #"echo '{"type":"question","id":"q1","kind":"keep-recording","payload":{"directory":"\#(directory.path)","segments":\#(segments)}}'; sleep \#(seconds); exit 0"#
        }
        try """
        #!/bin/sh
        echo "arguments: $@" >> "\(record.path)"
        echo '{"type":"ready","port":1,"token":"token","launch":"launch","presenter":"presenter"}'
        \(eventLines)
        while IFS= read -r line; do
          echo "stdin: $line" >> "\(record.path)"
          case "$line" in
            *'"type":"quit"'*) \(onQuit) ;;
            *'"type":"answer"'*) exit 0 ;;
            *'"type":"tunnel","start":true'*) \(tunnelRunning) ;;
            *'"type":"tunnel","start":false'*) echo '{"type":"tunnel","state":"stopped"}' ;;
            *'"action":"stop"'*) echo '{"type":"recording","state":"stopped","segment":1,"elapsed":0,"disk":"ok"}' ;;
            *'"action":"new-segment"'*) echo '{"type":"recording","state":"recording","segment":2,"elapsed":0,"disk":"ok"}' ;;
          esac
        done
        exit 0
        """.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }
```

The raw string `#"..."#` form keeps the JSON's quotes literal; `\#(...)` interpolates inside it.

- [ ] **Step 2: Write the failing tests**

`desktop/TapTests/RecordingTests.swift`:

```swift
import XCTest
@testable import Tap

final class RecordingTests: PresentingTestCase {
    func windowController(_ controller: DeckSessionController) throws -> DeckWindowController {
        try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
    }

    func testFirstTalkAsksAboutRecording() async throws {
        try removeRecordingConsent()
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "tap's consent question") { presentation.pendingQuestion?.kind == "record-consent" }
        let sheet = try XCTUnwrap(deckWindow.questionSheet)
        XCTAssertEqual(sheet.kind, "record-consent")
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet, "a sheet on the deck window")
        XCTAssertEqual(sheet.titleLabel.stringValue, "Record every talk automatically?")
        XCTAssertTrue(sheet.bodyLabel.stringValue.contains("follows the projector"))
        XCTAssertEqual(sheet.pathLabel.stringValue, settingsFile.path, "tap says where the answer is saved")
        XCTAssertEqual(sheet.declineButton.title, "Don't Record")
        XCTAssertEqual(sheet.acceptButton.title, "Record Automatically")
        XCTAssertEqual(presentation.state, .starting, "the talk waits for the answer")
        XCTAssertFalse(presentation.windowsShown, "nothing covers the sheet")
        XCTAssertFalse(presentation.audienceWindow?.isVisible ?? false)

        try XCTUnwrap(sheet.button(titled: "Don't Record")).performClick(nil)
        XCTAssertNil(deckWindow.questionSheet)
        XCTAssertNil(deckWindow.window?.attachedSheet)
        XCTAssertNil(presentation.pendingQuestion)
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
        try await waitUntil(timeout: 10, "tap to save the answer") {
            (try? String(contentsOf: self.settingsFile, encoding: .utf8))?.contains("record: false") == true
        }
        XCTAssertFalse(presentation.recording.isRecording)
        XCTAssertEqual(presentation.presenterWindow?.toolbar?.recordButton.title, "NOT RECORDING")

        // The answer is the CLI's too: the next talk asks nothing.
        try await stopPresenting(controller)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertNil(deckWindow.questionSheet)
    }

    func testAQuestionDuringTheTalkHidesTheWindowsOnItsScreen() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        presentation.handle(.question(id: "q9", kind: "record-consent", payload: QuestionPayload(settingsPath: "/tmp/settings.yaml")))
        let sheet = try XCTUnwrap(deckWindow.questionSheet)
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet)
        XCTAssertFalse(audience.isVisible, "the talk windows on the deck window's screen step aside")
        XCTAssertFalse(presenter.isVisible)
        XCTAssertEqual(presentation.windowsHiddenForQuestion.count, 2)
        try await waitUntil(timeout: 5, "the talk windows off screen") {
            let order = onScreenWindowNumbers()
            return !order.contains(audience.windowNumber) && !order.contains(presenter.windowNumber)
        }
        try XCTUnwrap(sheet.button(titled: "Record Automatically")).performClick(nil)
        XCTAssertNil(presentation.pendingQuestion)
        XCTAssertTrue(audience.isVisible)
        XCTAssertTrue(presenter.isVisible)
        XCTAssertTrue(presentation.frontWindow === audience)
        try await waitUntil(timeout: 5, "the talk windows back") {
            let order = onScreenWindowNumbers()
            return order.contains(audience.windowNumber) && order.contains(presenter.windowNumber)
        }
        XCTAssertEqual(presentation.state, .presenting)
    }

    func testRecordingFollowsTapPresent() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(
            events: [#"{"type":"recording","state":"recording","segment":1,"elapsed":724,"disk":"ok"}"#], recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let toolbar = try XCTUnwrap(presentation.presenterWindow?.toolbar)
        let dot = try XCTUnwrap(presentation.presenterWindow?.recordingDot)
        try await waitUntil(timeout: 10, "tap's recording event") { presentation.recording.isRecording }
        XCTAssertEqual(presentation.recording.segment, 1)
        XCTAssertTrue(toolbar.recordButton.title.hasPrefix("REC 12:0"), "REC 12:04 as tap reports it, counting up: \(toolbar.recordButton.title)")
        XCTAssertFalse(dot.isHidden)
        let shown = presentation.recording.elapsed
        try await waitUntil(timeout: 3, "the clock to count up between events") { presentation.recording.elapsed > shown }
        XCTAssertEqual(toolbar.recordButton.title, "REC " + RecordingStatus.clock(presentation.recording.elapsed))

        // REC stops the recording, and REC again starts a new segment, through tap.
        toolbar.recordButton.performClick(nil)
        try await waitUntil(timeout: 5, "the stop command") {
            (try? String(contentsOf: record, encoding: .utf8))?.contains(#"stdin: {"type":"recording","action":"stop"}"#) == true
        }
        try await waitUntil(timeout: 5, "tap's stopped event") { !presentation.recording.isRecording }
        XCTAssertEqual(toolbar.recordButton.title, "NOT RECORDING")
        XCTAssertTrue(dot.isHidden)
        toolbar.recordButton.performClick(nil)
        try await waitUntil(timeout: 5, "the new segment") { presentation.recording.segment == 2 && presentation.recording.isRecording }
        XCTAssertEqual(toolbar.recordButton.title, "REC 0:00")

        // A blocked recording stays NOT RECORDING, with tap's reason kept.
        presentation.handle(.recording(RecordingEvent(state: "stopped", segment: 2, elapsed: 0, disk: "ok")))
        presentation.handle(.error(TapErrorPayload(code: "recording_blocked", message: "Screen Recording is off for Tap in System Settings")))
        XCTAssertEqual(presentation.recording.blockedReason, "Screen Recording is off for Tap in System Settings")
        XCTAssertEqual(toolbar.recordButton.title, "NOT RECORDING")
    }
}
```

- [ ] **Step 3: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/RecordingTests/testFirstTalkAsksAboutRecording`
Expected: the test target does not compile (`questionSheet` is undefined).

- [ ] **Step 4: Write `QuestionSheet.swift`**

```swift
import AppKit

/// A question tap asked, or the Focus hint, as a sheet on the deck window:
/// a title, a body, an optional path line and two buttons. A sheet belongs
/// to its window; the rest of the app keeps running and nothing is
/// app-modal.
final class QuestionSheet: NSWindow {
    let kind: String
    let titleLabel = NSTextField(labelWithString: "")
    let bodyLabel = NSTextField(wrappingLabelWithString: "")
    let pathLabel = NSTextField(labelWithString: "")
    let declineButton = NSButton(title: "", target: nil, action: nil)
    let acceptButton = NSButton(title: "", target: nil, action: nil)

    init(kind: String, title: String, body: String, path: String?, decline: String, accept: String) {
        self.kind = kind
        super.init(contentRect: NSRect(x: 0, y: 0, width: 460, height: 200), styleMask: [.titled], backing: .buffered, defer: false)
        titleLabel.stringValue = title
        titleLabel.font = .systemFont(ofSize: 15, weight: .bold)
        bodyLabel.stringValue = body
        bodyLabel.font = .systemFont(ofSize: 12.5)
        bodyLabel.textColor = .secondaryLabelColor
        pathLabel.stringValue = path ?? ""
        pathLabel.isHidden = path == nil
        pathLabel.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        pathLabel.lineBreakMode = .byTruncatingMiddle
        pathLabel.setAccessibilityIdentifier("question-path")
        declineButton.title = decline
        declineButton.bezelStyle = .rounded
        declineButton.keyEquivalent = "\u{1b}"
        declineButton.target = self
        declineButton.action = #selector(declinePressed(_:))
        acceptButton.title = accept
        acceptButton.bezelStyle = .rounded
        acceptButton.keyEquivalent = "\r"
        acceptButton.target = self
        acceptButton.action = #selector(acceptPressed(_:))
        let buttons = NSStackView(views: [NSView(), declineButton, acceptButton])
        buttons.orientation = .horizontal
        buttons.spacing = 8
        let stack = NSStackView(views: [titleLabel, bodyLabel, pathLabel, buttons])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 24, left: 24, bottom: 24, right: 24)
        stack.widthAnchor.constraint(equalToConstant: 460).isActive = true
        bodyLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -48).isActive = true
        pathLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -48).isActive = true
        buttons.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -48).isActive = true
        contentView = stack
        setContentSize(stack.fittingSize)
        isReleasedWhenClosed = false
        setAccessibilityIdentifier("question-\(kind)")
    }

    func button(titled title: String) -> NSButton? {
        [declineButton, acceptButton].first { $0.title == title }
    }

    @objc private func declinePressed(_ sender: Any?) {
        sheetParent?.endSheet(self, returnCode: .cancel)
    }

    @objc private func acceptPressed(_ sender: Any?) {
        sheetParent?.endSheet(self, returnCode: .OK)
    }

    /// tap's record-consent question. `settingsPath` is where tap saves the answer.
    static func consent(settingsPath: String?) -> QuestionSheet {
        QuestionSheet(kind: "record-consent",
                      title: "Record every talk automatically?",
                      body: "Tap records the projector screen and your microphone from the start of each talk until you stop, and follows the projector if the cable is swapped. You choose whether to keep each recording at the end. The answer is saved for you, not the deck; tap present in Terminal uses it too.",
                      path: settingsPath,
                      decline: "Don't Record",
                      accept: "Record Automatically")
    }
}
```

- [ ] **Step 5: The sheet on the deck window, and the windows stepping aside**

In `DeckWindowController.swift`, add after `presentPopover`:

```swift
    /// The sheet for tap's question, while it is up.
    private(set) var questionSheet: QuestionSheet?
```

In `init`, after the `onStateChange` line (Task 6), add:

```swift
        sessionController.presentation.onQuestion = { [weak self] question in self?.presentQuestion(question) }
```

Add after `reloadSlides(_:)`:

```swift
    // MARK: tap's questions

    /// tap asked something. Consent and keep-recording become sheets on
    /// this window; the live code approval is D5's and is declined until
    /// then, which runs no code.
    func presentQuestion(_ question: PresentationController.PendingQuestion) {
        let presentation = sessionController.presentation
        switch question.kind {
        case "record-consent":
            showQuestionSheet(QuestionSheet.consent(settingsPath: question.payload.settingsPath)) { record in
                presentation.answer(id: question.id, value: record)
            }
        default:
            presentation.session?.log.append("the \(question.kind) question is not answered by this version of the app; declined", source: .app)
            presentation.answer(id: question.id, value: false)
        }
    }

    /// Puts `sheet` on this window and calls back with the answer. The talk's
    /// windows on this window's screen step aside until then, and this
    /// window comes forward: the sheet is the one thing the person must
    /// answer, so this is the one focus move outside the talk windows.
    func showQuestionSheet(_ sheet: QuestionSheet, completion: @escaping (Bool) -> Void) {
        guard let window else {
            completion(sheet.kind == "keep-recording")
            return
        }
        let presentation = sessionController.presentation
        questionSheet = sheet
        presentation.hideWindowsSharingScreen(with: window)
        window.makeKeyAndOrderFront(nil)
        window.beginSheet(sheet) { [weak self] response in
            self?.questionSheet = nil
            completion(response == .OK)
            presentation.restoreHiddenWindows()
        }
    }
```

In `PresentationController.swift`, add a stored property after `windowsShown`:

```swift
    /// The talk windows a question sheet sent off screen, to come back after the answer.
    private(set) var windowsHiddenForQuestion: [PresentationWindow] = []
    private var recordingTimer: Timer?
```

Add after `answer(id:value:)`:

```swift
    /// A sheet is going on `other`: the talk windows on its screen go off
    /// screen so the sheet has nothing over it. Nothing happens before the
    /// windows have been shown; then they simply wait.
    func hideWindowsSharingScreen(with other: NSWindow) {
        guard windowsShown, let screenFrame = other.screen?.frame else { return }
        for window in [audienceWindow, presenterWindow].compactMap({ $0 }) where window.isVisible && window.frame.intersects(screenFrame) {
            window.orderOut(nil)
            windowsHiddenForQuestion.append(window)
        }
    }

    /// The sheet is gone: the windows it sent away come back, the front one in front.
    func restoreHiddenWindows() {
        let hidden = windowsHiddenForQuestion
        windowsHiddenForQuestion = []
        guard isActive else { return }
        for window in hidden where window === audienceWindow || window === presenterWindow {
            window.orderFrontRegardless()
        }
        if let frontWindow, hidden.contains(where: { $0 === frontWindow }) {
            frontWindow.makeKeyAndOrderFront(nil)
            frontWindow.orderFrontRegardless()
        }
    }

    /// Counts the recording's seconds up between tap's events.
    private func startRecordingTimer() {
        recordingTimer?.invalidate()
        recordingTimer = Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated {
                guard let self, self.recording.isRecording else { return }
                self.recording.tick()
                self.refreshPresenterToolbar()
            }
        }
    }
```

In `openWindows`, add `startRecordingTimer()` after `sleepAssertion.acquire()`. In `takeDownWindows`, add as its first lines:

```swift
        recordingTimer?.invalidate()
        recordingTimer = nil
        windowsHiddenForQuestion = []
```

- [ ] **Step 6: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/RecordingTests/testFirstTalkAsksAboutRecording
make -C desktop test ONLY=TapTests/RecordingTests/testAQuestionDuringTheTalkHidesTheWindowsOnItsScreen
make -C desktop test ONLY=TapTests/RecordingTests/testRecordingFollowsTapPresent
```

Expected: all three pass. `testFirstTalkAsksAboutRecording` runs the real tap: with no `settings.yaml`, tap asks within milliseconds of ready, before any page has loaded, so the windows are never shown before the sheet. tap saves `present: record: false` under the test's `XDG_CONFIG_HOME`; the person's own settings are never touched.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted, the one that can leave a screen covered first: in `showWindowsIfReady`, drop `pendingQuestion == nil` (expected: `testFirstTalkAsksAboutRecording` fails on `windowsShown`, the windows cover the sheet); in `restoreHiddenWindows`, drop the `orderFrontRegardless` loop (expected: the mid-talk test fails on `isVisible`); in `hideWindowsSharingScreen`, drop `orderOut` (expected: it fails on `isVisible` being true); in `presentQuestion`, answer consent `true` regardless (expected: the consent test fails on `record: false`); in `startRecordingTimer`, never tick (expected: `testRecordingFollowsTapPresent` fails on the count); in `toggleRecording`, always send `.stop` (expected: it fails on segment 2); in `showQuestionSheet`, answer without a sheet (expected: the consent test fails on `attachedSheet`).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): tap's questions as sheets on the deck window, the recording consent, and the recording state"
```

---

### Task 10: Keep the recording

**Files:**
- Modify: `desktop/Tap/Presenting/QuestionSheet.swift` (`keepRecording`)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (the keep-recording case, `revealInFinder`, `folderSize`, the sheet ending when tap has already kept)
- Test: `desktop/TapTests/KeepRecordingTests.swift`

**Interfaces:**
- Consumes: Task 9's `QuestionSheet`, `showQuestionSheet`, `presentQuestion`, `FakeTapScripts.presenting(events:quit:recordingTo:)`; Task 4's `stop`, `state`, `onStateChange`.
- Produces: `QuestionSheet.keepRecording(directory:segments:size:)`; `DeckWindowController.revealInFinder`, `static folderSize(at:)`.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/KeepRecordingTests.swift`:

```swift
import XCTest
@testable import Tap

final class KeepRecordingTests: PresentingTestCase {
    var revealed: [URL] = []

    /// A run folder with two segments and a chapters file, 3 kB in all.
    func recordingFolder() throws -> URL {
        let folder = try Fixtures.temporaryFolder().appendingPathComponent("conference-talk-2026-09-24-1932")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        try Data(count: 2048).write(to: folder.appendingPathComponent("segment-1.mov"))
        try Data(count: 1024).write(to: folder.appendingPathComponent("segment-2.mov"))
        try "00:00 Intro\n".write(to: folder.appendingPathComponent("chapters.txt"), atomically: true, encoding: .utf8)
        return folder
    }

    func startTalk(quit: FakeTapScripts.QuitBehavior, record: URL) async throws -> (DeckSessionController, DeckWindowController) {
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], quit: quit, recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        deckWindow.revealInFinder = { [weak self] url in self?.revealed.append(url) }
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        return (controller, deckWindow)
    }

    func testKeepTheRecording() async throws {
        let folder = try recordingFolder()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let (controller, deckWindow) = try await startTalk(quit: .askToKeep(directory: folder, segments: 2), record: record)
        let presentation = controller.presentation
        deckWindow.stopPresenting(nil)
        XCTAssertEqual(presentation.state, .stopping)
        XCTAssertNil(presentation.audienceWindow, "the windows are already down")
        try await waitUntil(timeout: 10, "tap's keep-recording question") { deckWindow.questionSheet?.kind == "keep-recording" }
        let sheet = try XCTUnwrap(deckWindow.questionSheet)
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Keep this recording?")
        XCTAssertEqual(sheet.bodyLabel.stringValue, "2 segments, 3 KB on disk.")
        XCTAssertEqual(sheet.pathLabel.stringValue, folder.path)
        XCTAssertEqual(sheet.declineButton.title, "Delete")
        XCTAssertEqual(sheet.acceptButton.title, "Keep and Show in Finder")

        try XCTUnwrap(sheet.button(titled: "Keep and Show in Finder")).performClick(nil)
        XCTAssertEqual(revealed, [folder], "a kept run is revealed in Finder")
        try await waitUntil(timeout: 5, "the answer to reach tap") {
            (try? String(contentsOf: record, encoding: .utf8))?.contains(#"stdin: {"type":"answer","id":"q1","value":true}"#) == true
        }
        try await waitUntil(timeout: 10, "the talk to end") { presentation.state == .idle }
        XCTAssertNil(deckWindow.questionSheet)
    }

    func testDeletingTheRecordingAnswersNo() async throws {
        let folder = try recordingFolder()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let (controller, deckWindow) = try await startTalk(quit: .askToKeep(directory: folder, segments: 2), record: record)
        deckWindow.stopPresenting(nil)
        try await waitUntil(timeout: 10, "the question") { deckWindow.questionSheet?.kind == "keep-recording" }
        try XCTUnwrap(deckWindow.questionSheet?.button(titled: "Delete")).performClick(nil)
        XCTAssertEqual(revealed, [], "nothing to show for a deleted run")
        try await waitUntil(timeout: 5, "the answer to reach tap") {
            (try? String(contentsOf: record, encoding: .utf8))?.contains(#"stdin: {"type":"answer","id":"q1","value":false}"#) == true
        }
        try await waitUntil(timeout: 10, "the talk to end") { controller.presentation.state == .idle }
    }

    func testATapThatKeepsWithoutWaitingRevealsTheRun() async throws {
        let folder = try recordingFolder()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let (controller, deckWindow) = try await startTalk(quit: .askToKeepThenExit(after: 0.5, directory: folder, segments: 1), record: record)
        deckWindow.stopPresenting(nil)
        try await waitUntil(timeout: 10, "the question") { deckWindow.questionSheet?.kind == "keep-recording" }
        // tap's own three-second wait is over: it kept the recording and exited.
        try await waitUntil(timeout: 10, "the talk to end") { controller.presentation.state == .idle }
        XCTAssertNil(deckWindow.questionSheet, "the sheet goes with the process that asked")
        XCTAssertNil(deckWindow.window?.attachedSheet)
        XCTAssertEqual(revealed, [folder], "kept, so shown")
        XCTAssertTrue(controller.session.log.text.contains("tap kept the recording"))
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/KeepRecordingTests/testKeepTheRecording`
Expected: the test target does not compile (`revealInFinder` is undefined).

- [ ] **Step 3: The keep-recording sheet and its ending**

In `QuestionSheet.swift`, add after `consent(settingsPath:)`:

```swift
    /// tap's keep-recording question, asked when Stop ends a run that
    /// recorded. `size` is the run folder's size, formatted.
    static func keepRecording(directory: String, segments: Int, size: String) -> QuestionSheet {
        QuestionSheet(kind: "keep-recording",
                      title: "Keep this recording?",
                      body: "\(segments) segment\(segments == 1 ? "" : "s"), \(size) on disk.",
                      path: directory,
                      decline: "Delete",
                      accept: "Keep and Show in Finder")
    }
```

In `DeckWindowController.swift`, add after `questionSheet`:

```swift
    /// Reveals a kept recording. Production opens Finder on it; a test records the URL.
    var revealInFinder: (URL) -> Void = { url in NSWorkspace.shared.activateFileViewerSelecting([url]) }
```

In `presentQuestion(_:)`, add a case before `default`:

```swift
        case "keep-recording":
            let directory = question.payload.directory ?? ""
            let sheet = QuestionSheet.keepRecording(directory: directory, segments: question.payload.segments ?? 0,
                                                    size: Self.folderSize(at: URL(fileURLWithPath: directory)))
            showQuestionSheet(sheet) { [weak self] keep in
                // tap keeps the recording on a yes and on no answer at all; the app
                // never touches the folder itself.
                presentation.answer(id: question.id, value: keep)
                if keep, !directory.isEmpty { self?.revealInFinder(URL(fileURLWithPath: directory)) }
            }
```

Add after `showQuestionSheet`:

```swift
    /// The size of every file under `folder`, formatted for a person.
    static func folderSize(at folder: URL) -> String {
        var bytes: Int64 = 0
        if let files = FileManager.default.enumerator(at: folder, includingPropertiesForKeys: [.fileSizeKey]) {
            for case let file as URL in files {
                bytes += Int64((try? file.resourceValues(forKeys: [.fileSizeKey]))?.fileSize ?? 0)
            }
        }
        return ByteCountFormatter.string(fromByteCount: bytes, countStyle: .file)
    }

    /// tap exited while its keep-recording sheet was still up: its
    /// three-second wait ran out and it kept the recording. The sheet ends
    /// as a yes, so the run is revealed like any kept run.
    func talkEnded() {
        guard let sheet = questionSheet, sheet.kind == "keep-recording", let window else { return }
        sessionController.session.log.append("tap kept the recording before an answer came", source: .app)
        window.endSheet(sheet, returnCode: .OK)
    }
```

In `init`, replace the `onStateChange` line (Task 6) with:

```swift
        sessionController.presentation.onStateChange = { [weak self] state in
            self?.refreshPresentingControls()
            if state == .idle { self?.talkEnded() }
        }
```

- [ ] **Step 4: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/KeepRecordingTests/testKeepTheRecording
make -C desktop test ONLY=TapTests/KeepRecordingTests/testDeletingTheRecordingAnswersNo
make -C desktop test ONLY=TapTests/KeepRecordingTests/testATapThatKeepsWithoutWaitingRevealsTheRun
```

Expected: all three pass. `ByteCountFormatter` prints 3,072 bytes as "3 KB".

- [ ] **Step 5: Mutate and commit**

Mutations, each reverted: in the keep-recording case, reveal on `false` too (expected: `testDeletingTheRecordingAnswersNo` fails); answer `true` regardless (expected: it fails on the record file); in `talkEnded`, drop `endSheet` (expected: the no-wait test fails on `questionSheet`); in `keepRecording`, drop the segments count from the body (expected: `testKeepTheRecording` fails on the body).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the keep-recording sheet, revealing a kept run, and a tap that keeps without waiting"
```

---

### Task 11: The phone remote and the tunnel

**Files:**
- Create: `desktop/Tap/Presenting/RemotePanel.swift`
- Modify: `desktop/Tap/Presenting/PresentationController.swift` (`tunnel`, `tunnelError`, `setTunnel(on:)`, the tunnel start after ready)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (`remotePanel`, `togglePhoneRemote`, `refreshRemotePanel`)
- Modify: `desktop/Tap/App/MainMenu.swift` (Phone Remote)
- Test: `desktop/TapTests/PhoneRemoteTests.swift`

**Interfaces:**
- Consumes: Task 1's `TunnelEvent`, `TapCommand.tunnel(start:)`; Task 2's `PresentationOptions.wantsTunnel`; Task 9's `FakeTapScripts.presenting`; Task 3's `PresentationWindow.coveringLevel`.
- Produces: `RemotePanel` with `qrImageView`, `urlLabel`, `noteLabel`, `messageLabel`, `turnOffButton`, `onTurnOff`, `show(tunnel:error:ownPassword:on:)`, `hide()`; `PresentationController.tunnel`, `tunnelError`, `onTunnelChange`, `setTunnel(on:)`; `DeckWindowController.remotePanel`, `togglePhoneRemote(_:)`, `refreshRemotePanel()`; the Present menu item Phone Remote.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/PhoneRemoteTests.swift`:

```swift
import XCTest
@testable import Tap

final class PhoneRemoteTests: PresentingTestCase {
    func recorded(_ record: URL) -> String {
        (try? String(contentsOf: record, encoding: .utf8)) ?? ""
    }

    func testPhoneRemote() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, phoneRemote: true))
        try await waitUntil(timeout: 5, "the tunnel command") { self.recorded(record).contains(#"stdin: {"type":"tunnel","start":true}"#) }
        try await waitUntil(timeout: 5, "tap's running tunnel") { presentation.tunnel?.state == "running" }
        XCTAssertEqual(presentation.tunnel?.url, "https://stark-lake-1234.trycloudflare.com")
        let panel = deckWindow.remotePanel
        XCTAssertTrue(panel.isVisible, "the QR code that tap generates is on screen")
        XCTAssertEqual(panel.urlLabel.stringValue, "https://stark-lake-1234.trycloudflare.com")
        XCTAssertEqual(panel.qrImageView.image?.size, NSSize(width: 1, height: 1), "tap's PNG, decoded")
        XCTAssertTrue(panel.noteLabel.stringValue.contains("tap made a presenter password for this talk"))
        XCTAssertTrue(panel.messageLabel.isHidden)
        XCTAssertGreaterThan(panel.level.rawValue, PresentationWindow.coveringLevel.rawValue, "over the presenter window")
        try await waitUntil(timeout: 5, "the panel on screen") { onScreenWindowNumbers().contains(panel.windowNumber) }

        panel.turnOffButton.performClick(nil)
        try await waitUntil(timeout: 5, "the tunnel stop") { self.recorded(record).contains(#"stdin: {"type":"tunnel","start":false}"#) }
        try await waitUntil(timeout: 5, "tap's stopped tunnel") { presentation.tunnel?.state == "stopped" }
        XCTAssertFalse(panel.isVisible)

        // Present > Phone Remote turns it back on.
        deckWindow.togglePhoneRemote(nil)
        try await waitUntil(timeout: 5, "the tunnel again") { presentation.tunnel?.state == "running" && panel.isVisible }
        try await stopPresenting(controller)
        XCTAssertFalse(panel.isVisible, "the panel goes with the talk")
    }

    func testAdvancedRemoteOptions() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, tunnel: true, presenterPassword: "secret"))
        XCTAssertEqual(presentation.session?.command, .present(record: true, presenterPassword: "secret"))
        try await waitUntil(timeout: 5, "the arguments") { self.recorded(record).contains("--presenter-password secret") }
        XCTAssertFalse(recorded(record).contains("--tunnel"), "tap present has no --tunnel flag; the tunnel is a command")
        try await waitUntil(timeout: 5, "the tunnel command") { self.recorded(record).contains(#"stdin: {"type":"tunnel","start":true}"#) }
        try await waitUntil(timeout: 5, "the running tunnel") { presentation.tunnel?.state == "running" }
        XCTAssertTrue(deckWindow.remotePanel.noteLabel.stringValue.contains("your presenter password"), "the person's own password, not a generated one")
        try await stopPresenting(controller)
    }

    func testWithoutCloudflaredThePanelSaysSo() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], tunnelUnavailable: true, recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, phoneRemote: true))
        try await waitUntil(timeout: 5, "tap's error") { presentation.tunnelError != nil }
        let panel = deckWindow.remotePanel
        XCTAssertTrue(panel.isVisible)
        XCTAssertFalse(panel.messageLabel.isHidden)
        XCTAssertTrue(panel.messageLabel.stringValue.contains("brew install cloudflared"))
        XCTAssertNil(panel.qrImageView.image)
        try await stopPresenting(controller)
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/PhoneRemoteTests/testPhoneRemote`
Expected: the test target does not compile (`tunnel`, `remotePanel` are undefined).

- [ ] **Step 3: Write `RemotePanel.swift`**

```swift
import AppKit

/// The phone remote: the QR code and URL tap made for the tunnel, over the
/// presenter window, with Turn Off Remote. The QR image comes from tap.
final class RemotePanel: NSPanel {
    let qrImageView = NSImageView()
    let urlLabel = NSTextField(labelWithString: "")
    let noteLabel = NSTextField(wrappingLabelWithString: "")
    let messageLabel = NSTextField(wrappingLabelWithString: "")
    let turnOffButton = NSButton(title: "Turn Off Remote", target: nil, action: nil)
    var onTurnOff: (() -> Void)?

    init() {
        super.init(contentRect: NSRect(x: 0, y: 0, width: 360, height: 460), styleMask: [.titled, .nonactivatingPanel], backing: .buffered, defer: false)
        title = "Phone remote"
        level = NSWindow.Level(rawValue: PresentationWindow.coveringLevel.rawValue + 1)
        collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        isReleasedWhenClosed = false
        isFloatingPanel = true
        setAccessibilityIdentifier("remote-panel")
        let heading = NSTextField(labelWithString: "Scan to control the talk from your phone")
        heading.font = .systemFont(ofSize: 15, weight: .bold)
        qrImageView.imageScaling = .scaleProportionallyUpOrDown
        qrImageView.setAccessibilityIdentifier("remote-qr")
        urlLabel.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        urlLabel.isSelectable = true
        urlLabel.lineBreakMode = .byTruncatingMiddle
        noteLabel.font = .systemFont(ofSize: 12)
        noteLabel.textColor = .secondaryLabelColor
        messageLabel.font = .systemFont(ofSize: 12)
        messageLabel.textColor = .systemRed
        turnOffButton.bezelStyle = .rounded
        turnOffButton.target = self
        turnOffButton.action = #selector(turnOffPressed(_:))
        let stack = NSStackView(views: [heading, qrImageView, urlLabel, noteLabel, messageLabel, turnOffButton])
        stack.orientation = .vertical
        stack.alignment = .centerX
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 20, bottom: 20, right: 20)
        stack.widthAnchor.constraint(equalToConstant: 360).isActive = true
        qrImageView.widthAnchor.constraint(equalToConstant: 256).isActive = true
        qrImageView.heightAnchor.constraint(equalToConstant: 256).isActive = true
        for label in [urlLabel, noteLabel, messageLabel] {
            label.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -40).isActive = true
        }
        contentView = stack
        setContentSize(stack.fittingSize)
    }

    /// Shows tap's tunnel, or the reason there is none, centred on `frame`.
    func show(tunnel: TunnelEvent?, error: String?, ownPassword: Bool, on frame: CGRect) {
        if let tunnel, tunnel.state == "running" {
            urlLabel.stringValue = tunnel.url ?? ""
            qrImageView.image = tunnel.qr.flatMap { Data(base64Encoded: $0) }.flatMap { NSImage(data: $0) }
            messageLabel.isHidden = true
        } else {
            urlLabel.stringValue = tunnel?.state == "starting" ? "Starting the tunnel…" : ""
            qrImageView.image = nil
            messageLabel.stringValue = error ?? ""
            messageLabel.isHidden = error == nil
        }
        noteLabel.stringValue = ownPassword
            ? "Only a phone with your presenter password can control the talk."
            : "tap made a presenter password for this talk, so only this code works."
        setContentSize(contentView?.fittingSize ?? frame.size)
        setFrameOrigin(NSPoint(x: frame.midX - self.frame.width / 2, y: frame.midY - self.frame.height / 2))
        orderFrontRegardless()
    }

    func hide() {
        orderOut(nil)
    }

    @objc private func turnOffPressed(_ sender: Any?) {
        onTurnOff?()
    }
}
```

- [ ] **Step 4: The tunnel in the controller and the panel in the window controller**

In `PresentationController.swift`, add stored properties after `recordingTimer`:

```swift
    /// tap's last tunnel event, and the last tunnel error, for the remote panel.
    private(set) var tunnel: TunnelEvent?
    private(set) var tunnelError: String?
    var onTunnelChange: (() -> Void)?
```

In `start(_:)`, add `tunnel = nil` and `tunnelError = nil` after `editsNotShown = 0`. In `openWindows`, add after `refreshPresenterToolbar()`:

```swift
        // A restart's tap has no tunnel; ask again whenever one is wanted.
        if options.wantsTunnel { setTunnel(on: true) }
```

In `handle(_:)`, add cases before `default`:

```swift
        case .tunnel(let tunnelEvent):
            tunnel = tunnelEvent
            tunnelError = nil
            onTunnelChange?()
        case .error(let payload) where payload.code == "tunnel_unavailable" || payload.code == "tunnel_failed":
            tunnelError = payload.message
            onTunnelChange?()
```

(The `recording_blocked` case stays above these.) Add after `toggleRecording()`:

```swift
    /// Starts or stops tap's tunnel, as u does.
    func setTunnel(on: Bool) {
        guard isActive else { return }
        session?.send(.tunnel(start: on))
    }
```

In `takeDownWindows`, add `tunnel = nil` and `tunnelError = nil` after `windowsHiddenForQuestion = []`, and call `onTunnelChange?()` at the end of the method.

In `DeckWindowController.swift`, add after `revealInFinder`:

```swift
    private(set) lazy var remotePanel: RemotePanel = {
        let panel = RemotePanel()
        panel.onTurnOff = { [weak self] in self?.sessionController.presentation.setTunnel(on: false) }
        return panel
    }()
```

In `init`, after the `onQuestion` line, add:

```swift
        sessionController.presentation.onTunnelChange = { [weak self] in self?.refreshRemotePanel() }
```

Add after `talkEnded()`:

```swift
    // MARK: The phone remote

    /// Present > Phone Remote: the tunnel on or off.
    @objc func togglePhoneRemote(_ sender: Any?) {
        let presentation = sessionController.presentation
        presentation.setTunnel(on: presentation.tunnel?.state != "running")
    }

    /// The panel follows tap's tunnel: shown with the QR code while the
    /// tunnel runs or starts, shown with the reason when tap could not
    /// start it, gone when it stops or the talk ends.
    func refreshRemotePanel() {
        let presentation = sessionController.presentation
        let running = presentation.tunnel?.state == "running" || presentation.tunnel?.state == "starting"
        guard presentation.isActive, running || presentation.tunnelError != nil else {
            remotePanel.hide()
            return
        }
        let screenFrame = presentation.presenterWindow?.frame ?? window?.screen?.frame ?? NSScreen.screens[0].frame
        remotePanel.show(tunnel: presentation.tunnel, error: presentation.tunnelError,
                         ownPassword: presentation.options?.presenterPassword != nil, on: screenFrame)
    }
```

In `validateMenuItem`, add next to the Stop line:

```swift
        if menuItem.action == #selector(togglePhoneRemote(_:)) {
            menuItem.state = presentation.tunnel?.state == "running" ? .on : .off
            return presentation.isActive
        }
```

In `MainMenu.presentMenu()`, add after Swap Displays:

```swift
        menu.addItem(item("Phone Remote", action: #selector(DeckWindowController.togglePhoneRemote(_:))))
```

- [ ] **Step 5: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/PhoneRemoteTests/testPhoneRemote
make -C desktop test ONLY=TapTests/PhoneRemoteTests/testAdvancedRemoteOptions
make -C desktop test ONLY=TapTests/PhoneRemoteTests/testWithoutCloudflaredThePanelSaysSo
make -C desktop test ONLY=TapTests/PresentMenuTests/testPresentingShortcuts
```

Expected: all pass. Every tunnel test uses the scripted tap; no real tunnel is ever started.

- [ ] **Step 6: Mutate and commit**

Mutations, each reverted: in `openWindows`, drop the `wantsTunnel` line (expected: `testPhoneRemote` times out on the tunnel command); in `refreshRemotePanel`, never hide the panel (expected: it fails after Turn Off); in `handle`, drop the tunnel error case (expected: `testWithoutCloudflaredThePanelSaysSo` times out); in `RemotePanel.show`, drop the base64 decode (expected: the image size assertion fails); in `togglePhoneRemote`, always start (expected: `testPhoneRemote` fails on the second toggle's expectation only if extended; leave as noted); in `takeDownWindows`, drop `onTunnelChange?()` (expected: `testPhoneRemote` fails on the panel after the stop).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the phone remote panel with tap's QR code, the tunnel command, and the Advanced options"
```

---

### Task 12: Nothing interrupts the talk: the Focus hint, one talk at a time, and no updates during it

**Files:**
- Modify: `desktop/Tap/App/AppEnvironment.swift` (`focusHint`, `presentingCount`, `isPresenting`, `updatesMayInterrupt`, `presentingDidChangeNotification`)
- Modify: `desktop/Tap/Presenting/PresentationController.swift` (counts itself in and out)
- Modify: `desktop/Tap/Presenting/QuestionSheet.swift` (`focusHint`)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (the hint before the first talk, `openFocusSettings`, validation across decks)
- Modify: `desktop/TapTests/Support/HostedTestCase.swift` (a fresh, already-shown hint per test)
- Test: `desktop/TapTests/FocusHintTests.swift`

**Interfaces:**
- Consumes: Task 2's `FocusHintState`; Task 9's `QuestionSheet`, `showQuestionSheet`; Task 6's `startPresenting`, `refreshPresentingControls`.
- Produces: `AppEnvironment.focusHint`, `presentingCount`, `isPresenting`, `updatesMayInterrupt`, `noteTalkStarted()`, `noteTalkEnded()`, `static presentingDidChangeNotification`; `QuestionSheet.focusHint()`; `DeckWindowController.openFocusSettings`, `focusHintSheet`.

- [ ] **Step 1: Write the failing test**

In `desktop/TapTests/Support/HostedTestCase.swift`, add to `setUp` after the `presentExecutableURL` line:

```swift
        // The Focus hint shows before the first talk on a Mac; every test but the hint's own has seen it.
        AppEnvironment.shared.focusHint = FocusHintState(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.focus.\(UUID().uuidString)")))
        AppEnvironment.shared.focusHint.markShown()
```

`desktop/TapTests/FocusHintTests.swift`:

```swift
import XCTest
@testable import Tap

final class FocusHintTests: PresentingTestCase {
    func testNothingInterruptsTheTalk() async throws {
        AppEnvironment.shared.focusHint = FocusHintState(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.focus.first.\(UUID().uuidString)")))
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        var settingsOpened = 0
        deckWindow.openFocusSettings = { settingsOpened += 1 }

        // The first time I present, the app suggests a Focus mode.
        deckWindow.startPresenting(PresentationOptions(mode: .play, startSlide: 1))
        let sheet = try XCTUnwrap(deckWindow.focusHintSheet)
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Turn on a Focus before your talk?")
        XCTAssertTrue(sheet.bodyLabel.stringValue.contains("macOS does not let apps turn on a Focus for you"))
        XCTAssertEqual(presentation.state, .idle, "the talk waits for the hint")
        try XCTUnwrap(sheet.button(titled: "Open Focus Settings")).performClick(nil)
        XCTAssertEqual(settingsOpened, 1, "a button that opens its setting")
        XCTAssertEqual(presentation.state, .idle, "the person sets the Focus, then presses Play again")
        XCTAssertTrue(AppEnvironment.shared.focusHint.hasBeenShown)

        // Never again: Play starts at once.
        deckWindow.startPresenting(PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertNil(deckWindow.focusHintSheet)
        XCTAssertEqual(presentation.state, .starting)
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }

        // While I am presenting, nothing else may interrupt: no update prompt, no second talk.
        XCTAssertTrue(AppEnvironment.shared.isPresenting)
        XCTAssertFalse(AppEnvironment.shared.updatesMayInterrupt, "D7's Sparkle checks this before any prompt or restart")
        let (_, other) = try await openDeckForPresenting()
        let otherWindow = try XCTUnwrap(other.editor.window?.windowController as? DeckWindowController)
        let play = try XCTUnwrap(NSApp.mainMenu?.items.first { $0.submenu?.title == "Present" }?.submenu?.items.first { $0.action == #selector(DeckWindowController.play(_:)) })
        XCTAssertFalse(otherWindow.validateMenuItem(play), "one talk at a time")
        XCTAssertFalse(otherWindow.playButton.isEnabled)
        try await stopPresenting(controller)
        XCTAssertFalse(AppEnvironment.shared.isPresenting)
        XCTAssertTrue(AppEnvironment.shared.updatesMayInterrupt)
        XCTAssertTrue(otherWindow.validateMenuItem(play))
        XCTAssertTrue(otherWindow.playButton.isEnabled)
    }

    func testNotNowStartsTheTalk() async throws {
        AppEnvironment.shared.focusHint = FocusHintState(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.focus.notnow.\(UUID().uuidString)")))
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        deckWindow.startPresenting(PresentationOptions(mode: .rehearse, startSlide: 2))
        let sheet = try XCTUnwrap(deckWindow.focusHintSheet)
        try XCTUnwrap(sheet.button(titled: "Not Now")).performClick(nil)
        XCTAssertNil(deckWindow.focusHintSheet)
        XCTAssertEqual(controller.presentation.state, .starting)
        XCTAssertEqual(controller.presentation.options?.startSlide, 2)
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/FocusHintTests/testNotNowStartsTheTalk`
Expected: the test target does not compile (`focusHint`, `focusHintSheet` are undefined).

- [ ] **Step 3: The environment's count, the hint and the validation**

In `AppEnvironment.swift`, add after `presentExecutableURL`:

```swift
    /// Whether the Focus hint has been shown on this Mac. A test replaces it.
    var focusHint = FocusHintState()
    /// How many talks are running across every deck. Play is off while one
    /// runs, and D7's updater reads `updatesMayInterrupt` before any prompt
    /// or restart.
    private(set) var presentingCount = 0
    static let presentingDidChangeNotification = Notification.Name("TapPresentingDidChange")

    var isPresenting: Bool { presentingCount > 0 }
    var updatesMayInterrupt: Bool { !isPresenting }

    func noteTalkStarted() {
        presentingCount += 1
        NotificationCenter.default.post(name: Self.presentingDidChangeNotification, object: self)
    }

    func noteTalkEnded() {
        presentingCount = max(0, presentingCount - 1)
        NotificationCenter.default.post(name: Self.presentingDidChangeNotification, object: self)
    }
```

In `PresentationController.swift`, add a stored property after `tunnelError`:

```swift
    /// True while this talk is counted in `AppEnvironment.presentingCount`.
    private var countedAsPresenting = false
```

In `launch(deck:options:)`, add before `session.start()`:

```swift
        countedAsPresenting = true
        AppEnvironment.shared.noteTalkStarted()
```

Add a private method and call it as the first line of both `finishStopping()` and `fail(_:)`:

```swift
    private func uncount() {
        guard countedAsPresenting else { return }
        countedAsPresenting = false
        AppEnvironment.shared.noteTalkEnded()
    }
```

In `canStart`, use `deckURL() != nil && !isActive && !AppEnvironment.shared.isPresenting`.

In `QuestionSheet.swift`, add after `keepRecording`:

```swift
    /// The hint before the first talk. Its accept button opens the setting.
    static func focusHint() -> QuestionSheet {
        QuestionSheet(kind: "focus-hint",
                      title: "Turn on a Focus before your talk?",
                      body: "Notifications can appear on the projector. macOS does not let apps turn on a Focus for you.",
                      path: nil,
                      decline: "Not Now",
                      accept: "Open Focus Settings")
    }
```

In `DeckWindowController.swift`, add after `remotePanel`:

```swift
    private(set) var focusHintSheet: QuestionSheet?
    /// Opens the Focus pane of System Settings. A test replaces it.
    var openFocusSettings: () -> Void = {
        if let url = URL(string: "x-apple.systempreferences:com.apple.Focus-Settings-extension") { NSWorkspace.shared.open(url) }
    }
    private var presentingObserver: NSObjectProtocol?
```

Replace `startPresenting(_:)` with:

```swift
    /// Every start comes here: the popover's buttons, the Shift-click and
    /// Rehearse. The first time on this Mac, the Focus hint comes first:
    /// Not Now starts the talk, Open Focus Settings opens the setting and
    /// leaves the person to press Play again once the Focus is on, since a
    /// talk would cover System Settings.
    func startPresenting(_ options: PresentationOptions) {
        let hint = AppEnvironment.shared.focusHint
        guard hint.hasBeenShown, focusHintSheet == nil else {
            guard focusHintSheet == nil else { return }
            hint.markShown()
            guard let window else {
                // No window to hang a sheet on: the hint is skipped, never a reason not to present.
                sessionController.presentation.start(options)
                return
            }
            let sheet = QuestionSheet.focusHint()
            focusHintSheet = sheet
            window.beginSheet(sheet) { [weak self] response in
                guard let self else { return }
                self.focusHintSheet = nil
                if response == .OK {
                    self.openFocusSettings()
                } else {
                    self.sessionController.presentation.start(options)
                    self.refreshPresentingControls()
                }
            }
            return
        }
        sessionController.presentation.start(options)
        refreshPresentingControls()
    }
```

In `init`, after the `onTunnelChange` line, add:

```swift
        presentingObserver = NotificationCenter.default.addObserver(forName: AppEnvironment.presentingDidChangeNotification, object: nil, queue: nil) { [weak self] _ in
            MainActor.assumeIsolated { self?.refreshPresentingControls() }
        }
```

In `windowWillClose`, add `if let presentingObserver { NotificationCenter.default.removeObserver(presentingObserver) }` and `presentingObserver = nil`.

- [ ] **Step 4: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/FocusHintTests/testNothingInterruptsTheTalk
make -C desktop test ONLY=TapTests/FocusHintTests/testNotNowStartsTheTalk
make -C desktop test ONLY=TapTests/PresentPopoverTests/testThePlayButtonFollowsTheTalk
```

Expected: all pass.

- [ ] **Step 5: Mutate and commit**

Mutations, each reverted: in `startPresenting`, start the talk on `.OK` too (expected: `testNothingInterruptsTheTalk` fails on `.idle` after Open Focus Settings); drop `hint.markShown()` (expected: it fails on the second start showing a sheet); in `canStart`, drop the `isPresenting` check (expected: it fails on the other deck's Play); in `uncount`, never call `noteTalkEnded` (expected: it fails on `isPresenting` after the stop); in `launch`, drop `noteTalkStarted` (expected: it fails on `isPresenting`).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the Focus hint before the first talk, one talk at a time, and no updates while presenting"
```

---

### Task 13: When the talk cannot run: a refused save, a tap that fails to start, a tap that dies mid-talk

**Files:**
- Modify: `desktop/Tap/Documents/DocumentBar.swift` (`talkFailed`)
- Modify: `desktop/Tap/Presenting/PresentationController.swift` (`lastErrorMessage`, `lastTalkLog`, the failure message)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (the failure bar)
- Modify: `desktop/Tap/TapLog/TapLogWindowController.swift` (the failed talk's log stays listed)
- Modify: `desktop/TapTests/Support/FakeTapScripts.swift` (`failing(code:message:)`)
- Test: `desktop/TapTests/PresentingFailureTests.swift`

**Interfaces:**
- Consumes: Task 4's `onFailed`, `endBecauseTapFailed`, `fail`, `openWindows` reusing the windows on a restart; D3's `DocumentBarView`, `EditorViewController.showBar`, `hideBar`, `bar(_:)`, `DeckSessionController.diskChanged()`, `hasDiskConflict`, `DeckDocument.fileWasDeleted()`.
- Produces: `DocumentBarView.Kind.talkFailed`; `PresentationController.lastErrorMessage`, `lastTalkLog`; `DeckWindowController.showTalkFailed(_:)`; `FakeTapScripts.failing(code:message:)`.

- [ ] **Step 1: Write the failing tests**

Add to `desktop/TapTests/Support/FakeTapScripts.swift`:

```swift
    /// Prints one error event and exits with status 1, as tap does when it cannot start.
    static func failing(code: String, message: String) throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        try """
        #!/bin/sh
        echo '{"type":"error","code":"\(code)","message":"\(message)"}'
        exit 1
        """.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }
```

`desktop/TapTests/PresentingFailureTests.swift`:

```swift
import XCTest
@testable import Tap

final class PresentingFailureTests: PresentingTestCase {
    func windowController(_ controller: DeckSessionController) throws -> DeckWindowController {
        try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
    }

    func testATalkThatCannotStartShowsABar() async throws {
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.failing(code: "deck_not_found", message: "deck not found: talk.md")
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "the talk to fail") { if case .failed = presentation.state { return true } else { return false } }
        guard case .failed(let message) = presentation.state else { return XCTFail() }
        XCTAssertTrue(message.contains("deck not found: talk.md"), "tap's own reason: \(message)")
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(AppEnvironment.shared.isPresenting)
        let bar = try XCTUnwrap(controller.editorViewController.bar(.talkFailed))
        XCTAssertEqual(bar.message, "The talk could not run.")
        XCTAssertNotNil(bar.button(titled: "Show Tap Log"))
        XCTAssertTrue(presentation.canStart, "Play is back")
        XCTAssertTrue(deckWindow.playButton.isEnabled)
        TapLogWindowController.shared.reload()
        XCTAssertEqual(TapLogWindowController.shared.picker.segmentCount, 2, "the failed talk's log stays readable")
        try XCTUnwrap(bar.button(titled: "Dismiss")).performClick(nil)
        XCTAssertNil(controller.editorViewController.bar(.talkFailed))
    }

    func testATalkThatCannotBeSavedDoesNotStart() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let presentation = controller.presentation
        // An unsaved edit, and another program's change on disk: the app refuses to save over it.
        let end = (controller.editor.string as NSString).range(of: "# One").upperBound
        controller.editor.setSelectedRange(NSRange(location: end, length: 0))
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        try "# Theirs\n".write(to: deck, atomically: true, encoding: .utf8)
        controller.diskChanged()
        XCTAssertTrue(controller.hasDiskConflict)
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 10, "the refusal") { if case .failed = presentation.state { return true } else { return false } }
        guard case .failed(let message) = presentation.state else { return XCTFail() }
        XCTAssertTrue(message.hasPrefix("The deck could not be saved"), message)
        XCTAssertNil(presentation.session, "tap present never started")
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertNotNil(controller.editorViewController.bar(.talkFailed))
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), "# Theirs\n", "the other program's file is untouched")
    }

    func testPlayIsDisabledWhileADeckIsPresentingOrHasNoFile() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        XCTAssertTrue(controller.presentation.canStart)
        document.fileWasDeleted()
        XCTAssertFalse(controller.presentation.canStart, "tap present needs a file to read")
        deckWindow.refreshPresentingControls()
        XCTAssertFalse(deckWindow.playButton.isEnabled)
        deckWindow.playClicked(modifiers: [.shift])
        XCTAssertEqual(controller.presentation.state, .idle)
    }

    func testATalkSurvivesATapPresentRestart() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 2))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let client = try XCTUnwrap(presentation.client)
        let socket = client.openSocket()
        socket.resume()
        try await Task.sleep(nanoseconds: 300_000_000)
        socket.send(SlideMessage(slideIndex: 2, fragment: -1, step: 0))
        try await waitUntil(timeout: 10, "slide 3") { presentation.lastSlide == 3 }
        socket.close()
        let firstPid = try XCTUnwrap(presentation.session?.processIdentifier)

        kill(firstPid, SIGKILL)
        try await waitUntil(timeout: 30, "tap present back on a new process") {
            if let pid = presentation.session?.processIdentifier, pid != firstPid, case .running = presentation.session?.state { return true }
            return false
        }
        XCTAssertEqual(presentation.state, .presenting, "the talk never ended")
        XCTAssertTrue(presentation.sleepAssertion.isHeld)
        XCTAssertTrue(presentation.audienceWindow === audience, "the same windows")
        XCTAssertTrue(presentation.presenterWindow === presenter)
        try await waitUntil(timeout: 20, "the pages reloaded at the last slide") {
            audience.page.pageLoadCount == 2 && presenter.page.pageLoadCount == 2
        }
        XCTAssertEqual(audience.page.lastLoadedURL?.fragment, "3")
        XCTAssertEqual(audience.page.lastLoadedURL?.port, presentation.client?.ready.port, "the new process's port")
        try await waitUntil(timeout: 20, "the audience page on slide 3 again") { audience.page.lastReady?.slide == 3 }
        XCTAssertTrue(audience.isVisible)
    }

    func testATalkEndsWhenTapPresentKeepsDying() async throws {
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.readyAndWaiting()
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        // The talk is on (the windows show after the fallback, since the fake has no pages) when tap starts dying.
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        for _ in 0..<3 {
            try await waitUntil(timeout: 10, "a running tap present or the end") {
                if case .failed = presentation.state { return true }
                return presentation.session?.processIdentifier != nil
            }
            guard let pid = presentation.session?.processIdentifier else { break }
            kill(pid, SIGKILL)
            try await waitUntil(timeout: 10, "the killed process to be gone") { presentation.session?.processIdentifier != pid }
        }
        try await waitUntil(timeout: 30, "the talk to fail") { if case .failed = presentation.state { return true } else { return false } }
        guard case .failed(let message) = presentation.state else { return XCTFail() }
        XCTAssertTrue(message.contains("tap exited 3 times in 30 seconds"), message)
        XCTAssertTrue(presentation.failedAfterShowing)
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertNil(presentation.presenterWindow)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(NSApp.windows.contains { $0.isVisible && $0 is PresentationWindow })
        let bar = try XCTUnwrap(controller.editorViewController.bar(.talkFailed))
        XCTAssertTrue(bar.message.contains("The talk stopped"))
        XCTAssertEqual(controller.currentSlideNumber, 1, "the cursor is on the last slide presented")
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/PresentingFailureTests/testATalkThatCannotStartShowsABar`
Expected: the test target does not compile (`.talkFailed` is undefined).

- [ ] **Step 3: The failure bar and tap's own reason**

In `DocumentBar.swift`, add `talkFailed` to `Kind`:

```swift
        case changedOnDisk, deleted, deckErrors, environmentNotice, talkFailed
```

In `PresentationController.swift`, add stored properties after `countedAsPresenting`:

```swift
    /// tap's last error event, which names why it could not start.
    private(set) var lastErrorMessage: String?
    /// The log of a talk that failed, so Window > Tap Log still shows it.
    private(set) var lastTalkLog: TapLog?
    /// True when the failure came after the windows had been shown: the
    /// talk stopped, rather than never ran.
    private(set) var failedAfterShowing = false
```

In `start(_:)`, add `lastErrorMessage = nil`, `lastTalkLog = nil` and `failedAfterShowing = false` after `tunnelError = nil`. In `handle(_:)`, add before `default`:

```swift
        case .error(let payload):
            lastErrorMessage = payload.message
```

(This case comes after the two `where` cases on `.error`, so a recording or tunnel error is recorded by its own case and never reaches it.) Replace `endBecauseTapFailed` with:

```swift
    /// tap present exited three times in thirty seconds, or never got
    /// ready. Its own error event is the reason when it sent one; its last
    /// stderr line otherwise.
    private func endBecauseTapFailed(lastOutput: [String]) {
        let summary = session?.restartPolicy.exitSummary ?? "tap present exited"
        let reason = lastErrorMessage ?? lastOutput.last
        failedAfterShowing = windowsShown
        fail(reason.map { "\(summary). \($0)" } ?? summary)
        onStopped?(lastSlide)
    }
```

In `fail(_:)`, add `lastTalkLog = session?.log` before `session?.stop()`.

In `TapLogWindowController.reload()`, change the talk's log expression to `(controller.presentation.session?.log ?? controller.presentation.lastTalkLog).map { [$0] } ?? []`.

In `DeckWindowController.swift`, in `init`, after the `presentingObserver` block, add:

```swift
        sessionController.presentation.onFailed = { [weak self] message in self?.showTalkFailed(message) }
```

and in the `onStateChange` closure (Task 10), add `if state == .starting { self?.sessionController.editorViewController.hideBar(.talkFailed) }`. Add after `refreshRemotePanel()`:

```swift
    /// The talk could not start, or tap present stopped restarting. A bar
    /// on the deck window says why, with the talk's log a click away.
    func showTalkFailed(_ message: String) {
        let stopped = sessionController.presentation.failedAfterShowing
        let bar = DocumentBarView(
            kind: .talkFailed,
            message: stopped ? "The talk stopped." : "The talk could not run.",
            detail: message,
            buttons: [("Show Tap Log", { [weak self] in
                guard let self else { return }
                let presentation = self.sessionController.presentation
                TapLogWindowController.shared.show(log: presentation.session?.log ?? presentation.lastTalkLog ?? self.sessionController.session.log)
            }), ("Dismiss", { [weak self] in
                self?.sessionController.editorViewController.hideBar(.talkFailed)
            })])
        sessionController.editorViewController.showBar(bar)
    }
```

- [ ] **Step 4: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/PresentingFailureTests/testATalkThatCannotStartShowsABar
make -C desktop test ONLY=TapTests/PresentingFailureTests/testATalkThatCannotBeSavedDoesNotStart
make -C desktop test ONLY=TapTests/PresentingFailureTests/testPlayIsDisabledWhileADeckIsPresentingOrHasNoFile
make -C desktop test ONLY=TapTests/PresentingFailureTests/testATalkSurvivesATapPresentRestart
make -C desktop test ONLY=TapTests/PresentingFailureTests/testATalkEndsWhenTapPresentKeepsDying
make -C desktop test ONLY=TapTests/PresentingTests/testTheTalksLogIsListedInTheTapLogWindow
```

Expected: all pass. `testATalkSurvivesATapPresentRestart` takes a few seconds: D2's policy waits 0.5 s before the first restart, and tap present needs its ready line again.

- [ ] **Step 5: Mutate and commit**

Mutations, each reverted, the ones that can leave a screen covered or the assertion held first: in `endBecauseTapFailed`, skip `fail` (expected: `testATalkEndsWhenTapPresentKeepsDying` times out with the assertion held); in `openWindows`, create fresh windows on every ready instead of reusing (expected: the restart test fails on `===`); in `tapIsReady`, refuse `.presenting` (expected: the restart test never reloads); in `fail`, drop `takeDownWindows()` (expected: the dying test fails on `audienceWindow`); in `handle`, drop the `.error` case (expected: `testATalkThatCannotStartShowsABar` fails on the message); in `showTalkFailed`, drop the Dismiss button (expected: it fails on the unwrap); in `canStart`, drop the `deckURL() != nil` check (expected: `testPlayIsDisabledWhileADeckIsPresentingOrHasNoFile` fails).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): a talk that cannot start or stops restarting says why, and one that loses tap present comes back"
```

---

### Task 14: The scenario manifest, the UI tests, the config seam for them, and the README

**Files:**
- Modify: `desktop/scenarios.txt`, `desktop/README.md`
- Modify: `desktop/Tap/App/AppEnvironment.swift` (`-TapConfigHome`)
- Create: `desktop/TapUITests/PresentingUITests.swift`

**Interfaces:**
- Consumes: `desktop/scripts/check-scenarios.sh` (D2), the accessibility identifiers this plan set: `play-button`, `present-popover`, `start-presenting`, `audience-window`, `presenter-window`, `audience-page`, `presenter-page`, `presenter-toolbar`, `stop-button`, `record-button`, `reload-slides-button`, `swap-displays-button`, `recording-dot`, `question-record-consent`, `question-keep-recording`, `question-focus-hint`, `remote-panel`, `remote-qr`; D2's `-TapOpenOnLaunch`.
- Produces: the 19 `D4` rows; UI tests the person runs with `make -C desktop uitest`; `-TapConfigHome <folder>`, which the app puts into every tap process's environment as `XDG_CONFIG_HOME`, so a UI test's tap never reads or writes the person's settings.

- [ ] **Step 1: Claim the scenarios**

Append to `desktop/scenarios.txt`:

```text
D4 | 05-presenting.feature | Start presenting with two displays
D4 | 05-presenting.feature | Save before presenting
D4 | 05-presenting.feature | Nothing interrupts the talk
D4 | 05-presenting.feature | Start from the first slide
D4 | 05-presenting.feature | One display
D4 | 05-presenting.feature | Rehearse
D4 | 05-presenting.feature | Swap displays
D4 | 05-presenting.feature | Every tap dev presenter feature works
D4 | 05-presenting.feature | Stop presenting
D4 | 05-presenting.feature | Presenter controls
D4 | 05-presenting.feature | The Mac stays awake
D4 | 05-presenting.feature | Phone remote
D4 | 05-presenting.feature | Advanced remote options
D4 | 05-presenting.feature | First talk asks about recording
D4 | 05-presenting.feature | Recording follows tap present
D4 | 05-presenting.feature | Keep the recording
D4 | 05-presenting.feature | Edit while presenting
D4 | 05-presenting.feature | Remember the display assignment
D4 | 12-menus-and-shortcuts.feature | Presenting shortcuts
```

Run: `make -C desktop check-scenarios`
Expected: `every claimed scenario has a test`.

- [ ] **Step 2: The config seam**

In `AppEnvironment.init`, add after the `tapExecutableURL` assignment:

```swift
        // UI tests pass -TapConfigHome <folder>, so the tap they drive reads
        // and writes a settings file of their own, never the person's.
        if let configHome = UserDefaults.standard.string(forKey: "TapConfigHome") {
            extraEnvironment["XDG_CONFIG_HOME"] = configHome
        }
```

- [ ] **Step 3: Write the UI tests (compile only)**

`desktop/TapUITests/PresentingUITests.swift`:

```swift
import XCTest

/// A real talk on the real screen, driven with the real keyboard and
/// pointer. Local only: it covers the screen, and it needs Xcode's
/// permission to control the computer.
final class PresentingUITests: UITestCase {
    /// A settings folder with the recording question answered no, so the
    /// talk asks nothing and records nothing.
    func configHome() throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-ui-config-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: folder.appendingPathComponent("tap"), withIntermediateDirectories: true)
        try "present:\n  record: false\n".write(to: folder.appendingPathComponent("tap/settings.yaml"), atomically: true, encoding: .utf8)
        return folder
    }

    func launchForPresenting() throws -> XCUIApplication {
        let deck = try copyFixture("ops.md")
        let application = XCUIApplication()
        application.launchArguments = ["-TapOpenOnLaunch", deck.path, "-ApplePersistenceIgnoreState", "YES",
                                       "-TapConfigHome", try configHome().path, "-FocusHintShown", "YES"]
        application.launch()
        return application
    }

    func testPlayThroughThePopoverAndEscapeStops() throws {
        let application = try launchForPresenting()
        let play = application.buttons["play-button"]
        XCTAssertTrue(play.waitForExistence(timeout: 30))
        Thread.sleep(forTimeInterval: 2)
        play.click()
        let start = application.buttons["start-presenting"]
        XCTAssertTrue(start.waitForExistence(timeout: 5), "the Present popover")
        start.click()
        let audience = application.windows["audience-window"]
        XCTAssertTrue(audience.waitForExistence(timeout: 30), "the audience window covers the screen")
        Thread.sleep(forTimeInterval: 3)
        // The arrow keys go to tap's page.
        application.typeKey(.rightArrow, modifierFlags: [])
        Thread.sleep(forTimeInterval: 1)
        // Option-Tab shows the presenter window on one display.
        application.typeKey("\t", modifierFlags: [.option])
        Thread.sleep(forTimeInterval: 1)
        application.typeKey("\t", modifierFlags: [.option])
        Thread.sleep(forTimeInterval: 1)
        application.typeKey(.escape, modifierFlags: [])
        let gone = NSPredicate(format: "exists == false")
        expectation(for: gone, evaluatedWith: audience)
        waitForExpectations(timeout: 20)
        XCTAssertTrue(application.textViews["editor"].exists, "back to the editor")
    }

    func testRehearseByShortcutAndStopFromTheToolbar() throws {
        let application = try launchForPresenting()
        let editor = application.textViews["editor"]
        XCTAssertTrue(editor.waitForExistence(timeout: 30))
        Thread.sleep(forTimeInterval: 2)
        application.typeKey("p", modifierFlags: [.command, .option, .shift])
        let presenter = application.windows["presenter-window"]
        XCTAssertTrue(presenter.waitForExistence(timeout: 30), "the presenter view, full screen")
        XCTAssertFalse(application.windows["audience-window"].exists)
        Thread.sleep(forTimeInterval: 2)
        // The toolbar slides in when the pointer reaches the top edge.
        presenter.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.0)).hover()
        let stop = application.buttons["stop-button"]
        XCTAssertTrue(stop.waitForExistence(timeout: 5))
        stop.click()
        let gone = NSPredicate(format: "exists == false")
        expectation(for: gone, evaluatedWith: presenter)
        waitForExpectations(timeout: 20)
    }
}
```

`-FocusHintShown YES` sets the `FocusHintShown` default in the launched app's standard defaults, which `FocusHintState()` reads, so the hint does not stand between the test and the talk.

Run: `cd desktop && xcodebuild -project Tap.xcodeproj -derivedDataPath build/DerivedData -scheme Tap -destination 'platform=macOS' CODE_SIGN_IDENTITY=- CODE_SIGNING_REQUIRED=NO build-for-testing`
Expected: `** TEST BUILD SUCCEEDED **`, nothing launched. The XCUI element types (`windows`, `buttons`) follow the accessibility roles the views expose; the person's first run settles them, and any that need changing go in the ledger.

- [ ] **Step 4: The README**

Add to `desktop/README.md` under Test, after the thumbnail paragraph:

```markdown
The presenting tests run a real `tap present --app` beside the deck's
`tap dev --app` and cover the screen with the talk's windows for a few
seconds each. They answer tap's recording question ahead of time in the
test's own settings folder, so nothing is ever recorded, and every tunnel
test drives a scripted tap, so no tunnel is ever started. Two displays are
stood in for by the two halves of the one screen
(`PresentingTestCase.halfScreens()`).

What only a person can check, with a projector plugged in: the audience
window on the projector and the presenter window on the laptop, Swap
Displays moving them, the F key's element full screen in either page, a
real recording with Screen Recording permission (REC in the toolbar, the
keep-recording sheet at Stop, the run in Finder), and Phone remote with
cloudflared installed (the QR code from tap, a phone driving the deck).
`make -C desktop uitest` runs the two presenting UI tests on the real screen.
```

- [ ] **Step 5: Commit**

```bash
git add desktop/scenarios.txt desktop/README.md desktop/Tap/App/AppEnvironment.swift desktop/TapUITests
git commit -m "test(desktop): claim the D4 scenarios, add the presenting UI tests and the README's manual pass"
```

---

## Final check

- [ ] Run: `make -C desktop core-test`
  Expected: every `TapDesktopCore` test passes, including the new `PresentingCoreTests` and the extended `TapProtocolTests`, `TapSessionTests` and `TapClientTests`.
- [ ] Run: `make -C desktop check-scenarios`
  Expected: `every claimed scenario has a test`.
- [ ] Push the branch and read CI's `Desktop Tests` job: every hosted test green on the runner, the D2 and D3 tests included. The hosted bundle is not run locally in full (branch rule). The presenting tests cover the runner's screen; nothing there minds.
- [ ] Run: `cd desktop && xcodebuild -project Tap.xcodeproj -derivedDataPath build/DerivedData -scheme Tap -destination 'platform=macOS' CODE_SIGN_IDENTITY=- CODE_SIGNING_REQUIRED=NO build-for-testing`, and the same with `-scheme TapBenchmarks`
  Expected: the UI tests compile and the benchmarks still do. `make -C desktop uitest` and `make -C desktop bench` are the person's runs.
- [ ] Run: `grep -rn "$(printf '\342\200\224')" desktop/ docs/superpowers/plans/2026-09-24-desktop-presenting.md --include=*.swift --include=*.md --include=*.yml --include=*.sh --include=*.txt`
  Expected: no output.
- [ ] Run: `grep -rn "evaluateJavaScript\|callAsyncJavaScript" desktop/Tap`
  Expected: only `PreviewViewController.pageText`, `pageValue` and `PresentationPageController.pageText`, the test-only surfaces this plan and D2 document.
- [ ] Run: `grep -rn "runModal\|NSAlert" desktop/Tap`
  Expected: no output. Every question is a sheet.
- [ ] Run: `grep -rn "NSApp.activate\|activate(ignoringOtherApps" desktop/Tap`
  Expected: no output.
- [ ] Run: `grep -rn "orderFrontRegardless\|makeKeyAndOrderFront" desktop/Tap`
  Expected: only `PresentationController.showWindows`, `toggleFrontWindow`, `bringPresenterWindowForward`, `restoreHiddenWindows`, `screensChanged`, `RemotePanel.show`, `DeckWindowController.showQuestionSheet`, and D2's and D3's own lines (`showPreviewInWindow`, `bringDeckWindowForward`). Each is in answer to the person's own click in this app.
- [ ] Run: `grep -rn "sleepAssertion.release\|sleepAssertion.acquire" desktop/Tap`
  Expected: `acquire` only in `openWindows`; `release` only in `takeDownWindows`.
- [ ] Run: `grep -rn "completion?(" desktop/Tap`
  Expected: no output (D3 ledger lesson).
- [ ] Run: `grep -rn "unowned" desktop/Tap desktop/TapDesktopCore/Sources`
  Expected: no output.
- [ ] Run: `grep -rn "updateChangeCount" desktop/Tap`
  Expected: only `DeckSessionController.refreshEditedState`.
- [ ] Run: `pgrep -fl "tap present"` after the hosted tests
  Expected: no output; every talk's process is gone.
- [ ] Open the app by hand: `open "desktop/$(make -s -C desktop app-path)"`, then open `examples/basic.md`. Check: Play opens the popover; Start Presenting covers the screen with the audience page on the cursor's slide; Option-Tab shows the presenter view with the timer; the pointer at the top edge slides the toolbar in; Escape ends the talk and the cursor is on the last slide; Cmd+Option+Shift+P rehearses; Window > Tap Log lists "basic, talk" during a talk; `pgrep -fl "tap present"` prints nothing afterwards.
- [ ] Every part of the D4 outline maps to a task:

  | D4 outline item | Task |
  |---|---|
  | Play and Rehearse through `tap present --app` | 1, 4, 5 |
  | The popover | 6 |
  | Display arrangement and swap | 2, 5 |
  | Full screen audience window, presenter window | 3, 4, 7 |
  | The presenter toolbar | 8 |
  | The consent and keep-recording sheets from stdout questions | 9, 10 |
  | The sleep assertion | 3, 4 |
  | Phone remote, Advanced (05) | 11 |
  | Nothing interrupts the talk (05) | 12 |
  | Presenting shortcuts (12) | 7 |
  | The scenario manifest, the UI tests, the README | 14 |

## Pre-flight: conflicts found, rulings and what each costs if wrong

1. **`--tunnel` for `tap present` (05, Advanced remote options) does not exist.** Ruling: the code wins; the app sends `{"type":"tunnel","start":true}` after the ready line, on Play with Phone remote or Public tunnel on, and again after a restart. `testAdvancedRemoteOptions` asserts no `--tunnel` in the arguments. Cost if wrong: if P6 later adds the flag, one line in `Command.arguments` and the `setTunnel` call go.
2. **Full screen: cover windows, not system full screen Spaces.** The spec says the audience page "goes full screen on the projector" and the presenter view is "full screen". `toggleFullScreen` would make a Space per window, take about a second of animation, put two full screen windows of one app on one display into two Spaces that Option-Tab cannot switch, and be untestable on the runner. Ruling: a borderless window at `NSWindow.Level.mainMenu + 1` covering the screen's frame, with `.canJoinAllSpaces`; a swap is a frame change; Option-Tab is a window order change. The page's F key still asks WebKit for element full screen, which WebKit shows in its own full screen window; that is tap's page's business and the person's manual check. Cost if wrong: `PresentationWindow.cover` becomes `setFrame` plus `toggleFullScreen`, and `toggleFrontWindow` becomes a Space switch. Open question 2.
3. **Cmd+Option+P opens the popover (the spec's Play) rather than starting at once (12's "starts presenting").** The spec wins: the popover's default button is Start Presenting, so Cmd+Option+P then Return starts; Cmd+Option+Shift+P rehearses at once, since a rehearsal has no options to collect; Shift-click Play starts at once from slide 1 (the spec). Cost if wrong: one line in `playClicked`. Open question 3.
4. **Sheets on the deck window while the talk windows cover its screen.** The requirement is the deck window. Ruling: the windows are not shown until any startup question is answered (the consent arrives within milliseconds of ready, before a page can load, so no flicker), and a question during the talk sends the talk windows on the deck window's screen off screen until the answer, then brings them back with the front window in front; the deck window comes forward for the sheet, the one focus move outside the talk windows. On two displays the audience on the projector stays. Cost if wrong: `hideWindowsSharingScreen` and `restoreHiddenWindows` go, and `showQuestionSheet` takes the presenter window as the sheet's parent. Open question 4.
5. **The keep-recording answer has three seconds.** tap keeps the recording and exits when no answer comes in `appKeepRecordingTimeout`; the sheet stays up until the person answers or tap exits. Ruling: when tap exits first, the sheet ends as a yes and the run is revealed, since tap kept it; Delete after that does nothing, and the deck's log says tap kept it. The app never deletes a folder itself. Cost if wrong: the person who takes more than three seconds to press Delete keeps a run they wanted gone, and finds it in Finder. Open question 1 asks for the tap change that removes the race.
6. **Escape in the audience window ends the talk, whatever the page does with Escape.** The page uses Escape to close its help overlay and leave overview; the spec makes Escape Stop. Ruling: Escape in the audience window stops, in the presenter window it goes to the page unless there is no audience window (a rehearsal), where it stops. Cost if wrong: one branch in `handleKey`. Open question 5.
7. **Both pages hold the presenter cookie.** With a presenter password set (app mode always has one), the hub relays only from connections that carry the cookie; without it, the speaker's arrow keys in the audience window would move that page alone, the presenter view would not follow, and tap would emit no `slide` events or chapters. Ruling: the app trades the secret for the cookie and sets it into the shared `WKWebsiteDataStore` before loading either page; cookies ignore ports, so it stands for the present process while the talk runs and the preview, driven by the app's own socket, is unaffected. Cost if wrong: none that the tests would not show at once (`testEveryTapDevPresenterFeatureWorks`, `testStopPresenting`).
8. **"Record the talk" in the popover.** tap decides whether to record from `present.record` in `settings.yaml`, and the app decides nothing about recording. Ruling: the checkbox on (the default) passes no flag and leaves the decision to tap; off passes `--no-record` for this run, which is tap's own way to skip one talk. Cost if wrong: one checkbox.
9. **What "how many edits are not shown yet" counts.** Ruling: typing pauses tap dev has answered for since tap present last read the file, and zero again when the text is back to what was presented. Reload Slides saves and sends `reload`. Cost if wrong: a label's number.
10. **The Focus hint's Open Focus Settings does not start the talk.** A talk would cover System Settings. Ruling: the hint is marked shown either way; Not Now starts, Open Focus Settings opens the pane and leaves the person to press Play again. Cost if wrong: one branch in `startPresenting`. Open question 6.
11. **The live code approval question during a talk (D5).** tap asks it after the consent when the deck declares drivers. Ruling: declined with a log line until D5, which runs no code. Cost if wrong: none; D5 replaces the `default` case in `presentQuestion`.
12. **tap present dies mid-talk.** The spec's restart rule is written for tap dev. Ruling: the same policy; the windows stay up with the last render, the next ready reloads both pages at the last slide on the new port, the assertion stays held, and after three exits in thirty seconds the talk ends with a bar. tap's recording, if any, is a new run after the restart; the old one is whatever tap's exit path left. Cost if wrong: a speaker sees a reload instead of a frozen page; if a frozen page is preferred, `openWindows` skips the reload on a restart.
13. **App quit during a talk keeps the recording without asking.** `applicationWillTerminate` stops every talk, and tap keeps a recording when its stdin closes with no answer. Ruling: no sheet on quit. Cost if wrong: a run the person wanted deleted is on disk.
14. **One talk at a time, app-wide.** Two talks would fight over the displays. Ruling: `canStart` reads `AppEnvironment.isPresenting`. Cost if wrong: one check.
15. **The consent sheet's body and the Focus hint's body.** The mockups' copy, with the settings path from tap's payload in place of the mockup's fixed line. The keep-recording body has the segments and the folder's size; the mockup's minutes and start time are not in tap's payload. Cost if wrong: copy.
16. **`-FocusHintShown` and `-TapConfigHome` for UI tests.** The hint and the person's real settings would otherwise stand between a UI test and the talk. Cost if wrong: two lines.
17. **Window > Tap Log lists the talk's log** ("<deck>, talk") beside the deck's, and keeps a failed talk's log until the next talk. The spec says the log shows each process's output. Cost if wrong: one `flatMap`.
18. **The recording clock counts up in the app between events.** tap sends a recording event only on a change, with `elapsed` as of that moment. Cost if wrong: a label a second out.

## Open questions

Each has the default this plan implements. Change the plan before running it if an answer differs.

1. **The keep-recording timeout in tap (3 s).** `appKeepRecordingTimeout` in `internal/cli/app_session.go` assumes an app answers within seconds; a person reading a sheet does not. Default: the plan lives with it (pre-flight 5). Recommended: a small P6 follow-up on its own branch that waits for the answer while stdin stays open, with a long ceiling (60 s), since `quit` is only sent by the app and closing stdin already means "keep". Cost if wrong: the Delete button silently loses the race after three seconds, and the person finds a run they wanted gone.
2. **Cover windows versus system full screen.** Default: cover windows (pre-flight 2). Cost if wrong: a Space per window, an animated entry, and Option-Tab replaced by Space switching; about a day of rework in `PresentationWindow` and `PresentationController`, and the window-order tests change shape.
3. **Cmd+Option+P: the popover or an immediate start.** Default: the popover (pre-flight 3). Cost if wrong: one line, and the 12 scenario's wording.
4. **Where a mid-talk question's sheet goes.** Default: the deck window, with the talk windows on its screen stepping aside (pre-flight 4). Alternative: the presenter window as the sheet's parent. Cost if wrong: the hide and restore code goes, and one parent changes.
5. **Escape ends the talk at once.** Default: yes, in the audience window (pre-flight 6). Alternative: Escape twice, or Cmd+. only, to survive a stray key on stage. Cost if wrong: a stray Escape ends a talk; the keep-recording sheet and Play bring it back in seconds.
6. **The Focus hint and Open Focus Settings.** Default: opening the setting does not start the talk (pre-flight 10). Cost if wrong: one branch.
7. **The consent sheet before the windows show, versus a talk that starts under it.** Default: the windows wait for the answer. Cost if wrong: the `pendingQuestion == nil` guard in `showWindowsIfReady` goes.
8. **The person's runs.** `make -C desktop uitest` (two presenting UI tests on the real screen) and the manual pass in the README (a projector, the F key, a real recording, cloudflared) are compiled and written by the agents and run by the person, as in D2 and D3. `make -C desktop bench` gains nothing in D4.

## What this plan found missing in the spec and in P6

- P6 has no `--tunnel` for `tap present`; the app uses the stdin command (pre-flight 1).
- P6's keep-recording wait is three seconds, sized for a machine, not a person (open question 1).
- No event says "the startup questions are done"; the app infers it from the first page's ready and a pending question. D5's approval question, which comes after the consent, will arrive with the windows already up and use the stepping-aside path.
- The prerequisites document does not say that the pages need the presenter cookie to be relayed, or that cookies ignore ports (pre-flight 7); the code's comments do (`app_events.go`, `app_auth.go`).
- The spec does not say what happens when tap present dies mid-talk, when the projector is unplugged, when two decks try to present, or what Escape does in the presenter window; pre-flight 12, the Review Focus item 3, pre-flight 14 and pre-flight 6 decide.
- The design's "SwiftUI is used only for sheets and settings" is not taken up here: the sheets are AppKit stack views, as every D2 and D3 view is, so the hosted tests can drive their buttons directly. D6's Settings window can still be SwiftUI.

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-24-desktop-presenting.md`. Execute with superpowers:subagent-driven-development, one fresh subagent per task, as the roadmap requires. Tasks 1 and 2 are the core package and need no Xcode project; Task 3 onward touch the app target and its hosted tests, one at a time locally, the bundle on CI. Every task's review runs the mutations its last step lists, the ones that can leave a talk stuck, a window covering a screen or the sleep assertion held first.

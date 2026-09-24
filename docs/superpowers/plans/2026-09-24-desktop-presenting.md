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
            let pid = try XCTUnwrap(presentation.session?.processIdentifier)
            kill(pid, SIGKILL)
            try await waitUntil(timeout: 10, "the next process or the end") {
                presentation.session?.processIdentifier.map { $0 != pid } ?? true || presentation.state != .starting && presentation.state != .presenting
            }
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

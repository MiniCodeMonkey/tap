# Tap Desktop presenting (D4): implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Play and Rehearse run `tap present --app` as a second process beside the deck's `tap dev --app`, put tap's audience and presenter pages in native windows that each enter their own macOS full screen Space on the chosen display, hold a display sleep assertion for exactly as long as the talk runs, show tap's recording consent and keep-recording questions as sheets on the deck window, and put the editor's cursor on the last slide presented when the talk ends.

**Architecture:** `TapDesktopCore` learns the rest of P6's protocol (the `recording`, `tunnel` and `slide` events, question payloads, the `answer`, `tunnel` and `recording` commands) and lets `TapSession` run `present` as well as `dev`, with a `quit` that asks tap to shut down cleanly instead of closing its stdin, and a per-deck port so the talk pages keep one origin across launches. A pure `DisplayArrangement` decides which screen is the audience and which the presenter, remembered per pair of displays. In the app target, one `PresentationController` per deck owns the present session, two `PresentationWindow`s that each go to system full screen on one display (each holds a `WKWebView` with tap's page, element full screen enabled, on the persistent data store), the `SleepAssertion`, the position from tap's `slide` events and the recording state from its `recording` events. A talk that is stopping stays alive in `AppEnvironment` until its process exits, whatever happens to its deck window. The deck window gets a Play toolbar button with the Present popover, the Present menu, and the sheets. The presenter window gets the toolbar that slides up from the bottom edge (REC, Reload Slides, Swap Displays, Stop; the top edge is where full screen drops the menu bar) and the REC dot. tap does all the presenting, recording and tunnelling; the app only opens windows, forwards answers, and edits text.

**Tech Stack:** Swift 6.3 compiler in Swift 5 language mode, AppKit (`NSWindow.toggleFullScreen` with `.fullScreenPrimary`, the full screen delegate notifications, `NSPopover`, `NSWindow.beginSheet`, `NSTrackingArea`, `NSEvent.addLocalMonitorForEvents`), WebKit (`WKWebView`, `WKPreferences.isElementFullscreenEnabled`, `WKHTTPCookieStore`), IOKit (`IOPMAssertionCreateWithName`), CoreGraphics (`CGWindowListCopyWindowInfo` in tests, `CGDisplayIsBuiltin`), XCTest and XCUITest, XcodeGen, the bundled `tap` (`tap present --app [--no-record] [--presenter-password x] [--port n] <deck>`).

**Depends on:** tap's keep-recording wait, pull request 35 (`fix/app-keep-recording-wait`), **merged to `main` as a569901 on 2026-09-25**: `appKeepRecordingTimeout` is 60 seconds, so `tap present --app` waits for the keep-recording answer while the app's stdin is open, up to 60 seconds, and a closed stdin still resolves at once. This plan is written against that behaviour; the D4 branch is cut from a `main` at or after a569901. The roadmap's D4 row names the dependency.

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-design.md` (milestone 4; the sections "Processes", "The protocol between the app and tap", "Presenting", "Menus and accessibility" and "Testing"), `docs/superpowers/specs/2026-09-22-tap-desktop-prerequisites-design.md` part 6 as checked against `internal/cli` on `main` (see "P6 as built" below; the code wins), the D4 outline and "Decisions for the desktop app" in `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md`, and the feature files in `docs/superpowers/specs/tap-desktop-features/` (`05-presenting.feature` whole, plus "Presenting shortcuts" from `12-menus-and-shortcuts.feature`; `13-performance.feature` has no D4 scenario). The mockups are the "Tap Desktop Mockups" canvas (Present, Consent, FocusHint, PresenterWindow, PresenterIdle, Rehearse, PhoneRemote, KeepRecording). Where the mockups and the spec differ, the spec wins.

**Branch:** `feat/desktop-presenting`, branched from `main` after D3 (`feat/desktop-sidebar`, pull request 31) has merged, in a worktree at `/Users/codemonkey/projects/tap-d4`. If D3 has not merged when this plan starts, branch from `feat/desktop-sidebar` at d46a213 or later and rebase onto `main` once it has. One pull request. The starting code is D3's `desktop/`: `TapDesktopCore` (`TapProtocol`, `TapProcess`, `TapSession`, `TapClient`, `RestartPolicy`, `TapLog`), `DeckSessionController`, `DeckWindowController`, `MainMenu`, `AppEnvironment`, `PreviewViewController`, `WeakScriptMessageHandler`, `DocumentBarView`, `TapLogWindowController`, `HostedTestCase`, `Fixtures`, `FakeTapScripts`, `FakeTap`, `TestScripts`, `UITestCase`.

**Prerequisites on `main`, checked 2026-09-24 (`internal/cli`, `internal/server`, `frontend/src`):** `tap present --app` (`present.go`, P6) with the ready line, the events and the commands listed below; `--port <n>` (`present.go` line 101), which in app mode binds exactly that port and fails with an `error` event (code `failed`, message "port <n> is already in use (another tap dev may be running); pass --port <other>", since present runs the dev server, which names itself) and a non-zero exit when the port is taken (`port.go`, `startOnAvailablePort` with `explicit`), while no `--port` binds a free port that the ready line reports (`dev.go` line 186); the audience page takes its start slide from the URL hash (`frontend/src/lib/stores/presentation.ts`, `#<1-based slide>`), and so does the presenter page (`keyboard.ts` copies the hash when the S key opens `/presenter`); the hub relays a client's `slide` messages only when its WebSocket upgrade carried the presenter cookie (`internal/server/websocket.go`, `checkPresenterAuth`), and emits a `slide` event on every relay (`dev.go`, `hub.SetOnSlideChange`); the presenter route accepts the cookie or `?key=` and redirects without the key, which keeps the URL fragment (`routes.go`, `handlePresenter`, line 137); the page's key handler listens on `window` (`keyboard.ts` line 323). The one tap prerequisite is the keep-recording wait named under "Depends on".

## P6 as built, checked against `main` (the code wins)

The prerequisites document (part 6) and the code differ in these places. Every task below follows the code.

| Topic | The document says | The code does (`internal/cli`, `internal/server`) |
|---|---|---|
| The ready line | `{"type": "ready", "port", "token", "launch"}` | Also `"presenter"`: the presenter secret the app trades for the hub's presenter cookie (`app_events.go`, `appReadyEvent`; `dev.go` line 223). D2's `TapReady.presenter` and `TapClient.authorizePresenter()` already use it. |
| `--tunnel` for `tap present` | "the app passes them as `--presenter-password` and `--tunnel`" (05, Advanced remote options) | `present.go` has `--presenter-password`, `--no-record`, `--port`, `--lan`, `--allow-code` and `--app`, and **no `--tunnel`**. Only `tap dev` has `--tunnel`. The app starts the tunnel with the stdin command `{"type":"tunnel","start":true}` after the ready line (`app_session.go`, `tunnel(start)`). |
| Question payloads | `question` has `id`, `kind`, `payload` | `record-consent` carries `{"settingsPath": "..."}` (`app_questions.go`, `recordConsentPayload`); `keep-recording` carries `{"directory": "...", "segments": n}` (`app_session.go`, `keepRecordingPayload`); `approval` carries the approval request (D5). |
| Answer values | `{"type": "answer", "id", "value": true}` | `value` must be the JSON literal `true` or `false`; anything else is an `invalid_answer` error event, an unknown id is `unknown_question` (`app_questions.go`, `answer`). |
| The keep-recording question | "asks `keep-recording` first when a run has a recording" | Only when the run recorded and left slide 1 (`present.Started() && present.LeftFirstSlide()`). Since pull request 35 (a569901) tap waits **while the app's stdin stays open, up to 60 seconds** (`appKeepRecordingTimeout`), then keeps the recording and exits. Silence past the ceiling keeps. Closing stdin also keeps, with no question. The app's quit deadline (Task 4, `quitTimeoutWithRecording`) is longer than the ceiling, and a tap that keeps and exits before an answer still ends the sheet as Keep (Task 10). |
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
- **The app never injects script into a page.** No `evaluateJavaScript` or `callAsyncJavaScript` in production code. `PreviewViewController.pageText` and `pageValue` stay test-only as D2 left them; this plan's two test helpers, `PresentationPageController.pageText()` and `pressKey(_:)`, live in a `TapTests` extension (`Support/PresentationPageController+Tests.swift`), so they do not ship in the binary at all. The app drives a talk's pages through the URL they load with (the start slide in the hash, the presenter key in the query) and through tap's hub; it learns about them through the `tapReady` handler and tap's stdout events.
- **Sheets, never modal alerts.** Every question tap asks (`record-consent`, `keep-recording`) and the Focus hint are sheets on the deck window (`NSWindow.beginSheet`), never `NSAlert.runModal` and never app-modal. A refused action does nothing or beeps. Escape on a sheet never chooses a destructive answer: the keep-recording sheet's Delete has no key equivalent (C2 in the review).
- **No production code steals focus, except where presenting must.** This is the one list; the final check's grep is checked against it. Each entry answers the person's own click in this app. (1) `PresentationWindow.present(on:)` and its `settle()` order the window front before an entry, `PresentationWindow.attach(to:)` orders the presenter window over the audience window as its child, and `PresentationController.showWindows`, through `placeNext`'s completion, makes the front window key, because taking the projector is what Play means. (2) `PresentationController.toggleFrontWindow`, `bringPresenterWindowForward` and `showPresenterOverAudience` (Option-Tab and the S key: a child window over the audience on one display, a Space switch on two) and `returnToTalk` (after a sheet) make a talk window key. (3) `PresentationController.screensChanged` and `moveWindows`'s completions make the presenter window key when the displays change (the projector going or returning, a swap), so the speaker is not left looking at the audience page. (4) `DeckWindowController.showQuestionSheet` brings the deck window forward while a sheet is on it, because the sheet is the one thing the person must answer. (5) `RemotePanel.show` orders the phone remote panel front over the presenter window. (6) `DeckWindowController.revealInFinder` activates Finder on a kept run, and `openFocusSettings` opens System Settings, both on a button the person pressed. No `NSApp.activate` anywhere. (D2 ledger, Task 21.) The final check's grep expects exactly these.
- **System full screen, and nothing left in it.** On two displays each talk window enters its own macOS full screen Space with `toggleFullScreen` (`.fullScreenPrimary`), never a borderless window at a level above the menu bar: Cmd-Tab to a demo app, the menu bar at the top edge and the page's F key (WebKit's element full screen) then work as they do in a browser. On one display the audience window has the Space and the presenter window is its child inside that Space, shown over it by Option-Tab and the S key and hidden by Option-Tab, with no Space animation (the person's decision 5). Every way a talk ends goes through `PresentationController.takeDownWindows`, which takes the windows down one at a time, the front one first (an attached child goes with its parent), each leaving full screen before it closes (`PresentationWindow.takeDown`, which closes anyway after `exitTimeout` if the exit never completes), and releases the sleep assertion at once: Stop, Escape, a failed start, tap present giving up, a taken port, the deck window closing (`DeckSessionController.stop`) and the app quitting (`AppDelegate.applicationWillTerminate`, where the process ends before the exits complete and the Spaces go with it). Each has a test that reaches it and asserts that no `PresentationWindow` is left with `.fullScreen` in its `styleMask`, and every task's mutation list runs the mutations that could leave a window in full screen or the assertion held first. Whether a hosted test host can enter full screen at all is what Task 3's spike finds out (the person's decision 6): a probe decides once per process from what it observes, the full screen and Space assertions skip with `XCTSkip` and the probe's reason where it cannot, the state machine stays tested through seams, and real full screen is the person's UI tests. No test may leave CI red because the host cannot enter full screen.
- **A stopping talk outlives its deck window.** `DeckSessionController.stop` hands a talk that is still ending to `AppEnvironment.endingTalks`, and the talk removes itself when its process has exited (or the talk has failed) and its windows are down. Closing a deck mid-talk therefore still quits tap present with the app's deadline and escalation, still counts the talk out, and still frees Play in every other deck (C1 in the review).
- **`tap present` is a second process, on the deck's own port.** The deck's `tap dev --app` keeps running the preview and the thumbnails through the whole talk; nothing in this plan stops, restarts or talks to it differently. The present process gets its own `TapSession`, its own `TapLog` (listed in Window > Tap Log as "<deck>, talk") and its own restart policy. The app remembers one port per deck (`DeckPortStore`) and passes it with `--port`, so the talk pages keep one origin across launches and the presenter layout and notes size, which the page keeps in `localStorage`, survive a relaunch. A taken port falls back to a fresh one; that talk's presenter layout starts from the defaults, with a line in the talk's log and nothing on screen.
- **The edited flag is derived from content.** `refreshEditedState` stays the only caller of `updateChangeCount`. Play and Rehearse save through `NSDocument.save(to:ofType:for:completionHandler:)`, which already reports back to the session controller.
- `weak self` in every closure that outlives a call, no `unowned`. Never put work with side effects inside `completion?(...)`: an optional call skips its arguments when the closure is nil (D3 ledger lesson). Every completion in this plan is non-optional or the work sits outside the call.
- **XCTest rules.** No `await` inside an `XCTAssert` autoclosure: hoist the value into a `let`. No test depends on a key window: tests drive actions and seams directly (`windowController.playButtonClicked(modifiers:)`, `presentation.handleKey(_:)`, `container.mouseMoved(with:)`), set the first responder themselves, and never read `NSApp.keyWindow`. A window check reads observable state: `PresentationWindow.fullScreenState`, `styleMask.contains(.fullScreen)`, `settledFrame`, `isVisible`, `isAttached`, `isClosed`, or the window server (`CGWindowListCopyWindowInfo` with `kCGWindowIsOnscreen`, which lists the windows of the active Space only, so a Space switch is visible to it). A test that asserts full screen or which Space is active calls `try await requireFullScreen()` (or `requireSecondSpace()` for two Spaces on one screen) first, which skips it with the probe's reason on a host that cannot; every other assertion holds with plain windows too, so the rest of the test runs everywhere. Hosted tests run locally one at a time (`make -C desktop test ONLY=TapTests/<Class>/<test>`), the bundle runs on CI; the one exception is Task 4's whole-class run, which exists to catch a talk count that leaks between tests. **Hosted run budget:** at most four `make -C desktop test` runs per task are required, and each task's run step marks which; the rest are optional and are skipped when the branch is behind. Never run a hosted test while the person's `make -C desktop uitest` or `bench` runs: every presenting test takes the screen.
- **Every build step goes through `make`.** `make -C desktop test`, `uitest`, `bench` and the new `test-build` (Task 14) all depend on `project`, which runs xcodegen, because `Tap.xcodeproj` is git-ignored and a stale project silently lacks new files (D3's Critical came from exactly this). No task runs `xcodebuild` by hand. `make -C desktop uitest` and `bench` are the person's runs; agents compile the UI tests with `make -C desktop test-build`. `make -C desktop bench` gains nothing in D4 (no 13-performance scenario is D4's).
- **One display is what the tests have.** CI and most local runs have a single screen. Every hosted test either uses that one screen as both displays, or hands the controller two "screens" that are the left and right halves of the real screen (`PresentingTestCase.halfScreens()`). With system full screen a window fills the whole display it is on whatever frame it was given, so on one display the two windows of a "two display" test become two full screen Spaces of the one display (which the probe's second case says the host can do), while a real one-display talk is one Space with the presenter view as a child window inside it. The tests therefore assert what the controller asked for (`PresentationWindow.targetFrame`, `settledFrame`) and what the window server reports (`fullScreenState`, `styleMask`, `isAttached`, the active Space's windows), never a window's frame. What only the person can check, on a Mac with a projector: the audience Space on the projector and the presenter Space on the laptop, Swap Displays moving them across displays, the projector unplugged and plugged back in, the page's F key, Cmd-Tab to a demo app and back, "Displays have separate Spaces" turned off, real screen recording and a real Cloudflare tunnel (Task 14's README list and the UI tests). No automated test records the person's screen or starts a tunnel. If a host cannot enter full screen at all, Task 3's spike says so before any presenting code is written, the probe turns the full screen assertions into skips with that reason, the talk windows in the tests are plain windows over their frames (`PresentationController.fullScreenAllowed`), and CI stays green; the person's UI tests are what prove real full screen there.
- **Mutation testing is how this branch finds tests that cannot fail.** Every task lists the mutations its review runs, the ones that can leave a talk stuck, a window covering a screen, or the sleep assertion held first. A test that survives its mutation is not done.
- Every scenario this plan claims has a test named `test` plus the scenario name in UpperCamelCase: "Start presenting with two displays" is `testStartPresentingWithTwoDisplays`. The claims go into `desktop/scenarios.txt` as `D4 | <file> | <scenario>` rows, and `make -C desktop check-scenarios` must pass.
- The fixture for presenting tests is D3's `desktop/TapTests/Fixtures/ops.md`: seven titled slides (One to Seven), no drivers, so `tap present --app` asks no live code approval question. `seven-slides.md` declares a `sqlite` driver and would, which is D5's business.

## Review Focus

Five conditions the spec implies that no scenario names, most likely to bite first. Each has its test pinned to the task that owns the code.

1. **Play while the deck has a disk conflict, or no file.** The save is refused (`DeckDocument.save(to:...)` answers `.userCancelled` during a conflict) and a deleted deck has no path for `tap present` to read. The talk must not start, no window may open, and the deck window says why. Task 13, `testATalkThatCannotBeSavedDoesNotStart`; Task 13, `testPlayIsDisabledWhileADeckHasNoFile` and `testPlayIsDisabledWhileAnotherDeckPresents`.
2. **The deck window closes mid-talk.** The windows must leave full screen and close, tap present must exit within the app's deadline, the sleep assertion must be released, and the talk must be counted out so another deck can present. Task 4, `testClosingTheDeckMidTalkStillEndsTheProcessAndFreesPlay`.
3. **The projector is unplugged mid-talk.** macOS moves the audience Space to the remaining display; the speaker must end up looking at the presenter window, the assertion must stay held, and plugging the projector back in must put the audience back on it. Task 5, `testTheAudienceWindowFallsBackWhenTheProjectorGoes`.
4. **Stop while the presenter exchange is in flight.** The person clicks Play and Stop at once. `authorizePresenter` is awaiting; when it returns, no window may be created or enter full screen, the process must be quit, the assertion must not be held, and the state must return to idle. Task 4, `testStopWhileTheExchangeIsInFlightOpensNoWindow` (the exchange is a closure seam the test holds open).
5. **tap present dies mid-talk.** The audience must not be left on a dead page: the app restarts tap (D2's policy), reloads both pages at the last slide, keeps the assertion, and if tap keeps dying, ends the talk with nothing left in full screen and says so. Task 13, `testATalkSurvivesATapPresentRestart` and `testATalkEndsWhenTapPresentKeepsDying`.

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
| Play or Rehearse | `state = .starting`, and the talk is counted in `AppEnvironment.presentingCount` from here, so no other deck can start while the save runs. The buffer is saved to the deck file if it differs from it (`DeckSessionController.saveForPresenting`), because `tap present` reads the file. A refused save (disk conflict) or a deck with no file ends here as `.failed(message)` with a bar on the deck window; no window opens and the cursor does not move. | Task 4, Task 12, Task 13 |
| Start | A new `TapSession(deckURL:configuration:command: .present(record:presenterPassword:port:))` starts `tap present --app [--no-record] [--presenter-password x] [--port n] <deck>` with the login shell environment, `n` being the port `DeckPortStore` remembers for this deck, or its suggested port for the deck's first talk. Its `TapLog` is "<deck>, talk"; the log line hides the password. The deck's `tap dev` session is untouched. | Task 1, Task 2, Task 4 |
| Port taken | tap reports `{"type":"error","code":"failed","message":"port n is already in use (another tap dev may be running); pass --port <other>"}` (present runs the dev server, which names itself "tap dev") and exits, at the start or on a mid-talk restart when something grabbed the port in between. The controller stops that session before D2's policy can restart it with the same port, starts again with no `--port`, and logs "port n was taken; this talk runs on a new port, and the presenter layout starts fresh". The person sees the presenter view's default layout and notes size for that one talk, and nothing else; the next talk remembers the new port. A deck's first talk asks for `DeckPortStore.suggestedPort`, one of 20000 to 29999 derived from the deck's path, outside the ephemeral range that outgoing connections and tap dev draw from. | Task 2, Task 4 |
| Ready | The ready line arrives (D2's 20 s `readyTimeout` still kills a silent tap); the reported port is remembered for the deck. The app trades `ready.presenter` for the hub's presenter cookie through a closure seam (`authorizePresenter`), sets that cookie into the shared `WKWebsiteDataStore` for the audience page, and, if the talk is still starting, acquires the sleep assertion, creates the windows off screen and loads `/?launch=<code>#<startSlide>` in the audience window and `/presenter?key=<presenter secret>#<startSlide>` in the presenter window (the server sets the cookie and redirects, keeping the hash, so the presenter page never depends on the injected cookie). Rehearse: the presenter window only. If wanted, it sends `{"type":"tunnel","start":true}`. | Task 4, Task 11 |
| Shown | The windows go up when the first page reports `tapReady`, or after 3 s if no page ever does (a fake tap with no server, a page that cannot load), and only once no question is pending. On two displays each window is ordered front on its display and enters its own Space, one window at a time (AppKit runs one transition at a time), the presenter last so its Space is the active one and it is key. On one display only the audience window goes up, to full screen; the presenter window stays hidden until Option-Tab or the S key attach it over the audience window as a child in the same Space, with no animation. `state = .presenting` as the first window is asked up; `windowsAreSettled` becomes true once every placed window is where it was asked to be (in full screen, or a plain window over its frame where the host or the Spaces setting refuses full screen, with a log line). Until then, a `record-consent` question (which arrives within milliseconds of ready on the first talk) shows its sheet on the deck window with nothing covering it. After the consent answer tap's startup ends with a `reload` broadcast, so both pages reload once just as they appear; that flash is tap's, not a bug. | Task 3, Task 4, Task 9 |
| Presenting | tap's `slide` events keep `lastSlide`; `recording` events keep the toolbar's REC state; `tunnel` events drive the phone remote panel; `question` events queue up and become sheets one at a time (the deck window comes forward, which switches to its Space; after the answer the front talk window is made key, which switches back). Typing in the editor goes to `tap dev` only; the toolbar counts the edits `tap present` has not read. Reload Slides saves the file and sends `{"type":"reload"}`. | Tasks 8, 9, 10, 11 |
| tap present exits unexpectedly | D2's `RestartPolicy`: `.restarting` keeps the windows in full screen (the audience keeps the last render), the next ready reloads both pages at `lastSlide` on the new port, the assertion stays held. At the third exit in 30 s the session is `.failed`: the windows leave full screen and close, the assertion is released, `state = .failed(message)`, the talk is counted out and the deck window shows "The talk stopped" with Show Tap Log. tap's recording, if any, is finished by tap's own exit path. | Task 13 |
| Stop | Escape in the audience window, Present > Stop, the toolbar's Stop, or the deck window closing: `state = .stopping`, the windows go down one at a time, the front one first (an attached presenter window goes with the audience window), each leaving full screen before it closes and closing anyway after `PresentationWindow.exitTimeout` if its exit never completes, the assertion is released at once, then `session.quit()` sends `{"type":"quit"}`. tap may answer with a `keep-recording` question (a sheet on the deck window; tap waits while stdin is open, up to 60 s, then keeps). When the process exits, `state = .idle`, the talk is counted out and the editor cursor moves to `lastSlide`. A tap that ignores `quit` gets its stdin closed, then SIGTERM, then SIGKILL (D2's `TapProcess.stop`) after 15 s (2 s if it never got ready); a keep-recording question extends that deadline to 75 s, past tap's 60 s ceiling. A deck window that closes during any of this hands the talk to `AppEnvironment.endingTalks`, which keeps it alive until the process is gone, and answers any question tap still asks with tap's own default (keep a recording, decline anything else), since no window is left to show a sheet on. | Task 4, Task 10 |
| App quit | `AppDelegate.applicationWillTerminate` stops every deck's presentation: windows down, assertion released, `quit` sent. The app does not wait for keep-recording on quit; tap keeps the recording (its rule for a closed stdin, unchanged by the wait). | Task 4 |
| Play or Rehearse while tap dev is down | Allowed. The present process is independent; the preview overlay keeps showing tap dev's state. Play while a talk is running (this deck or another) is disabled; Stop, Reload Slides, Swap Displays and Phone Remote are enabled only while this deck presents. | Task 7, Task 12, Task 13 |

## File structure

| Path | Responsibility |
|---|---|
| `desktop/TapDesktopCore/Sources/TapDesktopCore/TapProtocol.swift` | Modify: `QuestionPayload`, `RecordingEvent`, `TunnelEvent`; `TapEvent.question(id:kind:payload:)`, `.recording`, `.tunnel`, `.slide`; `TapCommand.answer`, `.tunnel`, `.recording`; `RecordingAction` |
| `.../TapDesktopCore/TapSession.swift` | Modify: `Command` (`.dev`, `.present(record:presenterPassword:port:)`), `quit(timeout:)`, `extendQuit(timeout:)`, the log title and line for a talk (the password hidden) |
| `.../TapDesktopCore/TapClient.swift` | Modify: `audienceLaunchURL(slide:)`, `presenterURL(slide:)` (with the presenter key) |
| `.../TapDesktopCore/Presenting.swift` | `ScreenInfo`, `DisplayAssignmentStore`, `DisplayArrangement`, `DeckPortStore`, `PresentationMode`, `PresentationOptions`, `PresentationSettings`, `PresentationSettingsStore`, `RecordingStatus`, `FocusHintState` |
| `desktop/Tap/Presenting/SleepAssertion.swift` | The IOKit display sleep assertion, acquired and released by the controller |
| `desktop/Tap/Presenting/PresentationPageController.swift` | One of tap's pages in a `WKWebView`: element full screen enabled, persistent store, the `tapReady` handler, navigation policy, the S key's popup, the test-only `pageText()` and `pressKey(_:)` |
| `desktop/Tap/Presenting/PresentationWindow.swift` | A window that enters its own system full screen Space on one display, with the windowed, entering, fullScreen and exiting states, `present(on:completion:)`, `takeDown()` and the exit deadline; the presenter one also holds the toolbar and the REC dot |
| `desktop/Tap/Presenting/PresentationController.swift` | The talk: the present session, the deck's port and the fallback, the windows and their placement one at a time, the arrangement, the assertion, `lastSlide`, the question queue, recording, the tunnel, the key monitor, the edits counter, the talk count |
| `desktop/Tap/Presenting/PresentPopoverController.swift` | The Present popover: display arrangement, Swap Displays, start from, record, phone remote, Advanced, Rehearse, Start Presenting; its controls are the last settings, saved to `PresentationSettingsStore` on every start |
| `desktop/Tap/Presenting/DisplayArrangementView.swift` | The two labelled screen boxes the popover draws |
| `desktop/Tap/Presenting/PresenterToolbar.swift` | REC, edits label, Reload Slides, Swap Displays, Stop; slides up from the bottom edge; the REC dot |
| `desktop/Tap/Presenting/QuestionSheet.swift` | The sheet for consent, keep-recording and the Focus hint |
| `desktop/Tap/Presenting/RemotePanel.swift` | The phone remote panel with tap's QR code and URL |
| `desktop/Tap/Preview/WeakScriptMessageHandler.swift` | Unchanged; shared with the presentation pages |
| `desktop/Tap/Windows/DeckWindowController.swift` | Modify: the Play toolbar item (opens the popover), `play` (Cmd+Option+P starts with the last settings), `playWithOptions`, `rehearse`, `stopPresenting`, `reloadSlides`, `swapDisplays`, `togglePhoneRemote`, the sheets, validation, the failure bar, the remote panel closed with the window |
| `desktop/Tap/Documents/DeckSessionController.swift` | Modify: `presentation`, `saveForPresenting`, the stop path (hands a stopping talk to `AppEnvironment.endingTalks`), the edits counter hook |
| `desktop/Tap/Documents/DocumentBar.swift` | Modify: the `talkFailed` bar kind |
| `desktop/Tap/App/MainMenu.swift` | Modify: the Present menu |
| `desktop/Tap/App/AppDelegate.swift` | Modify: `deck(owning:)` for presentation windows, `applicationWillTerminate`, `stopAllPresentations` |
| `desktop/Tap/App/AppEnvironment.swift` | Modify: `displayAssignments`, `deckPorts`, `presentationSettings`, `presentExecutableURL`, `presentSessionConfiguration()`, `endingTalks`, `focusHint`, `presentingCount`, `isPresenting`, `updatesMayInterrupt`, `-TapConfigHome` |
| `desktop/Tap/TapLog/TapLogWindowController.swift` | Modify: lists the talk's log beside the deck's |
| `desktop/TapTests/Support/PresentingTestCase.swift` | The consent file, the half screens, the window server order, start and stop helpers, the sleep assertion check, the full screen check |
| `desktop/TapTests/Support/FakeTapScripts.swift` | Modify: `presenting(...)`, a scripted `tap present --app` |
| `desktop/TapTests/Support/HostedTestCase.swift` | Modify: fresh `displayAssignments`, `deckPorts`, `presentationSettings`, `focusHint`, `presentExecutableURL` per test |
| `desktop/TapTests/*.swift` | The hosted tests: `PresentingTests`, `PresentingDisplayTests`, `PresentPopoverTests`, `PresentMenuTests`, `PresenterToolbarTests`, `RecordingTests`, `KeepRecordingTests`, `PhoneRemoteTests`, `FocusHintTests`, `PresentingFailureTests` |
| `desktop/TapUITests/PresentingUITests.swift` | Play through the popover, Escape stops; Rehearse by shortcut; local only |
| `desktop/scenarios.txt`, `desktop/README.md` | Modify: the 19 D4 rows; the presenting tests and the manual pass |
| `desktop/Makefile` | Modify: `test-build` and `bench-build`, the `build-for-testing` targets that depend on `project` |

---

### Task 1: The rest of P6's protocol, and a session that runs `present` and quits cleanly

**Files:**
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/TapProtocol.swift`
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/TapSession.swift`
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/TapClient.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/TapProtocolTests.swift`, `TapSessionTests.swift`, `TapClientTests.swift`

**Interfaces:**
- Consumes: D2's `TapEvent`, `TapCommand`, `TapReady`, `TapSession`, `TapProcess.send(_:)`, `TapProcess.stop(graceSeconds:)`, `RestartPolicy`, `FakeTap.ready(recordingTo:)`, `TestScripts.make(_:)`, `waitUntil`.
- Produces: `QuestionPayload(deck:settingsPath:directory:segments:)`; `RecordingEvent(state:segment:elapsed:disk:)`; `TunnelEvent(state:url:qr:)`; `TapEvent.question(id:kind:payload:)`, `.recording(RecordingEvent)`, `.tunnel(TunnelEvent)`, `.slide(slide:step:)`; `RecordingAction` (`.newSegment`, `.stop`); `TapCommand.answer(id:value:)`, `.tunnel(start:)`, `.recording(action:)`; `TapSession.Command` (`.dev`, `.present(record:presenterPassword:port:)`) with `arguments(deck:)`, `logLine(deck:)`, `logTitle(deck:)`, `port`; `TapSession.init(deckURL:configuration:command:)`, `let command`, `quit(timeout:)`, `extendQuit(timeout:)`, `private(set) var quitRequested`; `TapClient.audienceLaunchURL(slide:)`, `presenterURL(slide:)`.

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
            command: .present(record: false, presenterPassword: "secret", port: 4242))
        XCTAssertEqual(tap.command.arguments(deck: deckURL), ["present", "--app", "--no-record", "--presenter-password", "secret", "--port", "4242", deckURL.path])
        XCTAssertEqual(tap.command.port, 4242)
        XCTAssertEqual(TapSession.Command.present(record: true, presenterPassword: nil, port: nil).arguments(deck: deckURL), ["present", "--app", deckURL.path])
        XCTAssertEqual(TapSession.Command.present(record: true, presenterPassword: "", port: nil).arguments(deck: deckURL), ["present", "--app", deckURL.path], "an empty password is no password")
        XCTAssertEqual(TapSession.Command.dev.arguments(deck: deckURL), ["dev", "--app", deckURL.path])
        XCTAssertNil(TapSession.Command.dev.port)
        XCTAssertEqual(tap.log.title, "talk, talk")
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertTrue(recorded.contains("arguments: present --app --no-record --presenter-password secret --port 4242 \(deckURL.path)"))
        XCTAssertTrue(tap.log.text.contains("tap present --app --no-record --presenter-password *** --port 4242 talk.md"), "the log hides the password")
        XCTAssertFalse(tap.log.text.contains("secret"))
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

    func testExtendQuitMovesTheDeadline() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = session(try FakeTap.ready(recordingTo: record))
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        tap.quit(timeout: 0.3)
        // A keep-recording question would move the deadline past tap's wait; here the test moves it.
        tap.extendQuit(timeout: 1.5)
        try await Task.sleep(nanoseconds: 800_000_000)
        if case .running = tap.state {} else { XCTFail("the first deadline no longer applies") }
        try await waitUntil(timeout: 5) { tap.state == .stopped }
        XCTAssertTrue(tap.log.text.contains("did not quit within 1.5 seconds"))
        XCTAssertFalse(tap.log.text.contains("within 0.3 seconds"))
    }

    func testQuitBeforeTheProcessExistsStopsAtOnce() async throws {
        // The environment closure is still running when quit arrives: no process is ever launched.
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = TapSession(deckURL: deckURL, configuration: TapSession.Configuration(
            executableURL: try FakeTap.ready(recordingTo: record),
            environment: { try? await Task.sleep(nanoseconds: 500_000_000); return ["PATH": "/usr/bin:/bin"] }))
        tap.start()
        XCTAssertEqual(tap.state, .starting)
        tap.quit()
        XCTAssertEqual(tap.state, .stopped)
        try await Task.sleep(nanoseconds: 800_000_000)
        XCTAssertEqual(tap.state, .stopped, "the environment arriving after quit launches nothing")
        XCTAssertNil(tap.processIdentifier)
        XCTAssertFalse(FileManager.default.fileExists(atPath: record.path), "the fake never ran")
    }
```

Add to `TapClientTests.swift` (it already has a `stubbedClient()` helper from D2; use its `ready`):

```swift
    func testTheTalkPagesCarryTheStartSlideInTheirHash() {
        let client = TapClient(ready: TapReady(port: 4242, token: "t", launch: "launch-code", presenter: "p"))
        XCTAssertEqual(client.audienceLaunchURL(slide: 3).absoluteString, "http://127.0.0.1:4242/?launch=launch-code#3")
        XCTAssertEqual(client.presenterURL(slide: 3).absoluteString, "http://127.0.0.1:4242/presenter?key=p#3",
                       "the presenter page carries its key: the server sets the cookie and redirects, keeping the hash")
        XCTAssertEqual(client.audienceLaunchURL(slide: 1).fragment, "1")
        XCTAssertEqual(client.presenterURL(slide: 1).fragment, "1")
    }

    func testThePresenterKeyIsPercentEncoded() throws {
        // The person's own password can hold anything; each of these would cut, split or space the key if it went in raw.
        let password = "a b#c&d+e%"
        let client = TapClient(ready: TapReady(port: 4242, token: "t", launch: "launch-code", presenter: password))
        for url in [client.presenterLaunchURL, client.presenterURL(slide: 2)] {
            let components = try XCTUnwrap(URLComponents(url: url, resolvingAgainstBaseURL: false))
            XCTAssertEqual(components.path, "/presenter")
            XCTAssertEqual(components.queryItems?.count, 1)
            XCTAssertEqual(components.queryItems?.first?.name, "key")
            XCTAssertEqual(components.queryItems?.first?.value, password, "decodes back to the same string: \(url)")
            XCTAssertFalse(components.percentEncodedQuery?.contains("+") ?? true, "a plus is encoded, or Go reads it as a space")
        }
        XCTAssertEqual(client.presenterURL(slide: 2).fragment, "2", "the hash survives the encoding")
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
        /// otherwise tap generates one and prints it on the ready line;
        /// `port` is the deck's remembered port, which tap binds exactly
        /// (or fails), and nil lets tap pick a free one.
        case present(record: Bool, presenterPassword: String?, port: Int?)

        public func arguments(deck: URL) -> [String] {
            switch self {
            case .dev:
                return ["dev", "--app", deck.path]
            case .present(let record, let presenterPassword, let port):
                var arguments = ["present", "--app"]
                if !record { arguments.append("--no-record") }
                if let presenterPassword, !presenterPassword.isEmpty {
                    arguments += ["--presenter-password", presenterPassword]
                }
                if let port { arguments += ["--port", String(port)] }
                arguments.append(deck.path)
                return arguments
            }
        }

        /// The port a present command asks for, nil for none and for dev.
        public var port: Int? {
            if case .present(_, _, let port) = self { return port }
            return nil
        }

        /// The log line for a start: the arguments with the deck's name in
        /// place of its path and the presenter password hidden, since the
        /// Tap Log is copied into bug reports.
        public func logLine(deck: URL) -> String {
            var shown = arguments(deck: deck).dropLast() + [deck.lastPathComponent]
            if let index = shown.firstIndex(of: "--presenter-password"), shown.indices.contains(index + 1) {
                shown[index + 1] = "***"
            }
            return "tap " + shown.joined(separator: " ")
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

    /// Moves the quit deadline to `timeout` from now, for a tap that has
    /// asked keep-recording and is waiting for a person: the deadline
    /// must outlast tap's own wait for the answer. Nothing happens unless
    /// a quit is in progress.
    public func extendQuit(timeout: TimeInterval) {
        guard quitRequested, let process else { return }
        quitWork?.cancel()
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

- [ ] **Step 6: Add the talk URLs to `TapClient`, with the key encoded**

In `TapClient.swift`, replace D2's `url(scheme:path:)` and `presenterLaunchURL` with these, and add the two talk URLs after them:

```swift
    /// Builds a URL against the one running tap. `scheme` is the only thing
    /// that changes between the HTTP endpoints and the WebSocket. Query
    /// values are percent-encoded, a plus included: the person's own
    /// presenter password may hold `#`, `&`, `+` or `%`, and Go's
    /// Query().Get would cut, split or space an unencoded one.
    private func url(scheme: String = "http", path: String = "", query: [String: String] = [:], fragment: String? = nil) -> URL {
        var components = URLComponents()
        components.scheme = scheme
        components.host = "127.0.0.1"
        components.port = ready.port
        components.path = path
        if !query.isEmpty {
            components.percentEncodedQuery = query.sorted { $0.key < $1.key }
                .map { "\($0.key)=\(Self.encodedQueryValue($0.value))" }
                .joined(separator: "&")
        }
        components.fragment = fragment
        return components.url!
    }

    /// `value` percent-encoded for a query: everything a query allows
    /// except the characters that mean something there (`&`, `=`, `+`, `#`).
    static func encodedQueryValue(_ value: String) -> String {
        var allowed = CharacterSet.urlQueryAllowed
        allowed.remove(charactersIn: "&=+#")
        return value.addingPercentEncoding(withAllowedCharacters: allowed) ?? ""
    }

    /// The presenter view, carrying the ready line's presenter secret. tap
    /// answers with the presenter cookie and a redirect to the same path
    /// without the key, so the secret leaves the address bar at once.
    public var presenterLaunchURL: URL { url(path: "/presenter", query: ["key": ready.presenter]) }

    /// The audience page for a talk, starting on `slide` (1-based): the
    /// launch code as the preview uses it, and the slide in the fragment,
    /// which the page reads on load (`initializeFromURL` in
    /// frontend/src/lib/stores/presentation.ts). The fragment survives
    /// tap's redirect, which names no fragment of its own.
    public func audienceLaunchURL(slide: Int) -> URL { url(path: "/", query: ["launch": ready.launch], fragment: String(slide)) }

    /// The presenter page for a talk, starting on `slide`. It carries the
    /// presenter secret as `key`: the server sets the presenter cookie and
    /// redirects to /presenter without the key, and the redirect keeps the
    /// fragment (internal/server/routes.go, handlePresenter). The page
    /// therefore never depends on a cookie the app injected first.
    public func presenterURL(slide: Int) -> URL { url(path: "/presenter", query: ["key": ready.presenter], fragment: String(slide)) }
```

`baseURL`, `previewURL` and `previewLaunchURL` keep their shapes (`url()`, `url(path: "/")`, `url(path: "/", query: ["launch": ready.launch])`); D2's `TapClientTests` pin the first two, and the launch code tap generates is URL-safe, so the third is unchanged for it.

- [ ] **Step 7: Run the tests**

Run: `make -C desktop core-test`
Expected: every test passes, including D2's `testStartsTapDevAppAndReadsTheReadyLine` (its log line is unchanged: `tap dev --app talk.md`) and the seven new ones.

- [ ] **Step 8: Mutate and commit**

Mutations, each reverted: in `quit`, drop `quitRequested = true` (expected: `testQuitSendsTheQuitCommandAndNeverRestarts` fails, the exit restarts); in `quit`, drop `process.send(.quit)` (expected: the same test fails on the record file); in `quit`, drop the deadline work item (expected: `testQuitClosesStdinWhenTapIgnoresTheCommand` times out); in `extendQuit`, drop `quitWork?.cancel()` (expected: `testExtendQuitMovesTheDeadline` fails on the 0.3 s log line); in `quit`, drop `launchGeneration += 1` (expected: `testQuitBeforeTheProcessExistsStopsAtOnce` fails, the fake runs; the `state == .starting` guard in `start` is D2's second guard for the same case and survives on its own); in `encodedQueryValue`, keep `+` in the allowed set (expected: `testThePresenterKeyIsPercentEncoded` fails on the plus); in `url(scheme:path:query:fragment:)`, put the value in raw (expected: it fails on the decoded value); in `Command.arguments`, always append `--no-record` (expected: `testATalkRunsTapPresentWithItsFlagsAndHasItsOwnLogTitle` fails on the `record: true` case); in `Command.arguments`, drop the `--port` pair (expected: the same test fails on the arguments); in `logLine`, drop the redaction (expected: it fails on "secret"); in `presenterURL`, drop the key (expected: `testTheTalkPagesCarryTheStartSlideInTheirHash` fails); in `decode`, return `.other(type: "slide")` for a slide event (expected: `testDecodesTheTalkEvents` fails).

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
- Produces: `ScreenInfo(name:frame:isBuiltIn:)`; `DisplayAssignmentStore(defaults:)` with `audienceName(for:)`, `setAudienceName(_:for:)`, `static key(for:)`; `DisplayArrangement(audience:presenter:)` with `isSingleDisplay`, `static resolve(screens:store:)`, `swapped()`; `DeckPortStore(defaults:)` with `port(for:)`, `setPort(_:for:)`, `static key(for:)`, `static suggestedPort(for:)`; `PresentationMode` (`.play`, `.rehearse`); `PresentationOptions(mode:startSlide:record:phoneRemote:tunnel:presenterPassword:)` with `command(port:)`, `wantsTunnel`; `PresentationSettings(startFromSlideOne:record:phoneRemote:tunnel:)` with `options(mode:cursorSlide:presenterPassword:)`; `PresentationSettingsStore(defaults:)` with `settings`, `defaults`; `RecordingStatus` with `state`, `segment`, `elapsed`, `disk`, `blockedReason`, `label`, `isRecording`, `apply(_:)`, `tick()`, `static clock(_:)`; `FocusHintState(defaults:)` with `hasBeenShown`, `markShown()`.

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
        XCTAssertEqual(play.command(port: nil), .present(record: true, presenterPassword: nil, port: nil))
        XCTAssertEqual(play.command(port: 4242), .present(record: true, presenterPassword: nil, port: 4242), "the deck's remembered port travels with the command")
        XCTAssertFalse(play.wantsTunnel)
        let quiet = PresentationOptions(mode: .play, startSlide: 1, record: false, phoneRemote: true)
        XCTAssertEqual(quiet.command(port: nil), .present(record: false, presenterPassword: nil, port: nil))
        XCTAssertTrue(quiet.wantsTunnel)
        let rehearse = PresentationOptions(mode: .rehearse, startSlide: 5, record: true, tunnel: true, presenterPassword: "secret")
        XCTAssertEqual(rehearse.command(port: nil), .present(record: false, presenterPassword: "secret", port: nil), "a rehearsal never records")
        XCTAssertTrue(rehearse.wantsTunnel)
    }

    func testThePortIsRememberedPerDeck() throws {
        let store = DeckPortStore(defaults: try freshDefaults())
        let deck = URL(fileURLWithPath: "/talks/ops.md")
        XCTAssertNil(store.port(for: deck))
        store.setPort(4242, for: deck)
        XCTAssertEqual(store.port(for: deck), 4242)
        XCTAssertEqual(store.port(for: URL(fileURLWithPath: "/talks/../talks/ops.md")), 4242, "the same file, however spelled")
        XCTAssertNil(store.port(for: URL(fileURLWithPath: "/talks/other.md")), "another deck has its own")
        store.setPort(4243, for: deck)
        XCTAssertEqual(store.port(for: deck), 4243, "a fallback port replaces the taken one")
        // A deck's first talk asks for a port of its own, outside the ephemeral range every outgoing connection and tap dev draw from.
        let suggested = DeckPortStore.suggestedPort(for: deck)
        XCTAssertTrue((20000...29999).contains(suggested))
        XCTAssertEqual(DeckPortStore.suggestedPort(for: URL(fileURLWithPath: "/talks/../talks/ops.md")), suggested, "the same file, the same port, in every launch")
        XCTAssertNotEqual(DeckPortStore.suggestedPort(for: URL(fileURLWithPath: "/talks/other.md")), suggested)
    }

    func testTheLastSettingsBecomeTheNextOptions() throws {
        let store = PresentationSettingsStore(defaults: try freshDefaults())
        XCTAssertEqual(store.settings, PresentationSettings(), "the defaults: from the cursor, recording on, no remote, no tunnel")
        let chosen = PresentationSettings(startFromSlideOne: true, record: false, phoneRemote: true, tunnel: true)
        store.settings = chosen
        XCTAssertEqual(PresentationSettingsStore(defaults: store.defaults).settings, chosen, "kept across launches")
        XCTAssertEqual(chosen.options(mode: .play, cursorSlide: 4, presenterPassword: nil),
                       PresentationOptions(mode: .play, startSlide: 1, record: false, phoneRemote: true, tunnel: true, presenterPassword: nil))
        XCTAssertEqual(PresentationSettings().options(mode: .play, cursorSlide: 4, presenterPassword: "secret"),
                       PresentationOptions(mode: .play, startSlide: 4, record: true, presenterPassword: "secret"),
                       "the password comes from the popover's field for this launch; it is never kept")
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

    /// The tap present command for these options, on `port` (the deck's
    /// remembered port, or nil for a free one).
    public func command(port: Int?) -> TapSession.Command {
        .present(record: mode == .play && record, presenterPassword: presenterPassword, port: port)
    }

    public var wantsTunnel: Bool { phoneRemote || tunnel }
}

/// The Present popover's settings, the ones Cmd+Option+P starts with: kept
/// across launches. The presenter password is not among them; it lives in
/// the popover's field for one launch of the app.
public struct PresentationSettings: Equatable, Sendable {
    public var startFromSlideOne = false
    public var record = true
    public var phoneRemote = false
    public var tunnel = false

    public init(startFromSlideOne: Bool = false, record: Bool = true, phoneRemote: Bool = false, tunnel: Bool = false) {
        self.startFromSlideOne = startFromSlideOne
        self.record = record
        self.phoneRemote = phoneRemote
        self.tunnel = tunnel
    }

    /// The options for a start with these settings, from `cursorSlide`
    /// unless the settings say slide 1.
    public func options(mode: PresentationMode, cursorSlide: Int, presenterPassword: String?) -> PresentationOptions {
        PresentationOptions(mode: mode, startSlide: startFromSlideOne ? 1 : cursorSlide, record: record,
                            phoneRemote: phoneRemote, tunnel: tunnel, presenterPassword: presenterPassword)
    }
}

// UserDefaults is thread-safe, so a struct that only holds a reference to it is safe to share.
public struct PresentationSettingsStore: @unchecked Sendable {
    public let defaults: UserDefaults
    static let key = "PresentationSettings"

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public var settings: PresentationSettings {
        get {
            guard let stored = defaults.dictionary(forKey: Self.key) else { return PresentationSettings() }
            return PresentationSettings(startFromSlideOne: stored["startFromSlideOne"] as? Bool ?? false,
                                        record: stored["record"] as? Bool ?? true,
                                        phoneRemote: stored["phoneRemote"] as? Bool ?? false,
                                        tunnel: stored["tunnel"] as? Bool ?? false)
        }
        nonmutating set {
            defaults.set(["startFromSlideOne": newValue.startFromSlideOne, "record": newValue.record,
                          "phoneRemote": newValue.phoneRemote, "tunnel": newValue.tunnel], forKey: Self.key)
        }
    }
}

/// The port each deck's talks run on. WebKit keys a page's localStorage,
/// where the presenter page keeps its layout and notes size, by origin,
/// and the origin includes the port; one port per deck keeps that state
/// across launches. Keyed by the deck's standardized path.
// UserDefaults is thread-safe, so a struct that only holds a reference to it is safe to share.
public struct DeckPortStore: @unchecked Sendable {
    private let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public static func key(for deck: URL) -> String {
        "PresentPort:" + deck.standardizedFileURL.path
    }

    public func port(for deck: URL) -> Int? {
        let port = defaults.integer(forKey: Self.key(for: deck))
        return port > 0 ? port : nil
    }

    public func setPort(_ port: Int, for deck: URL) {
        defaults.set(port, forKey: Self.key(for: deck))
    }

    /// The port a deck's first talk asks for: one of 20000 to 29999,
    /// derived from the deck's path with FNV-1a (Swift's own hasher is
    /// seeded per process), so it is the same in every launch and outside
    /// the ephemeral range that outgoing connections and `tap dev --app`
    /// draw from, which a port tap picked itself would sit in.
    public static func suggestedPort(for deck: URL) -> Int {
        var hash: UInt64 = 0xcbf29ce484222325
        for byte in deck.standardizedFileURL.path.utf8 {
            hash ^= UInt64(byte)
            hash = hash &* 0x100000001b3
        }
        return 20000 + Int(hash % 10000)
    }
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
Expected: the nine new tests pass; the package is green.

- [ ] **Step 5: Mutate and commit**

Mutations, each reverted: in `resolve`, ignore the store (expected: `testASwapIsRememberedForThePairOfDisplaysWhicheverOrderTheyComeIn` fails on the swapped audience); in `resolve`, honour a remembered name that is not connected (expected: the same test fails on "Gone"); in `key(for:)`, drop `.sorted()` (expected: the key equality assertion fails); in `PresentationOptions.command(port:)`, drop `mode == .play &&` (expected: `testOptionsBecomeTheTapPresentCommand` fails on the rehearsal); in `command(port:)`, pass `port: nil` always (expected: it fails on 4242); in `DeckPortStore.key(for:)`, drop `standardizedFileURL` (expected: `testThePortIsRememberedPerDeck` fails on the respelled path); in `suggestedPort`, use `hashValue` (expected: the same test fails on the respelled path in some runs, since the hash is seeded per process; note the flake as the reason for FNV); in `PresentationSettings.options`, ignore `startFromSlideOne` (expected: `testTheLastSettingsBecomeTheNextOptions` fails on `startSlide: 1`); in `tick`, drop the `isRecording` guard (expected: the paused count fails); in `label`, return "REC" for paused (expected: `testTheRecordingLabelFollowsTapAndCountsUpBetweenEvents` fails).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): display arrangement, the deck's port, presentation options and settings, recording status and the Focus hint state"
```

---

### Task 3: The full screen spike, the sleep assertion, a page of tap's in a web view, and a window that enters its own full screen Space

**Files:**
- Create: `desktop/TapTests/FullScreenSpikeTests.swift` (Step 0)
- Create: `desktop/TapTests/Support/FullScreenProbe.swift`
- Create: `desktop/Tap/Presenting/SleepAssertion.swift`
- Create: `desktop/Tap/Presenting/PresentationPageController.swift`
- Create: `desktop/TapTests/Support/PresentationPageController+Tests.swift` (the test-only `pageText()` and `pressKey(_:)`)
- Create: `desktop/Tap/Presenting/PresentationWindow.swift`
- Create: `desktop/TapTests/Support/WindowServer.swift`
- Modify: `desktop/Tap/App/AppEnvironment.swift` (`presentationDataStore`)
- Modify: `desktop/TapTests/Support/HostedTestCase.swift` (a fresh data store per test)
- Test: `desktop/TapTests/PresentationWindowTests.swift`

**Interfaces:**
- Consumes: D2's `ReadyPayload`, `WeakScriptMessageHandler`, `PreviewViewController.isExternalWebLink(url:navigationType:)`; D3's `DeckWindowController` (only as a weak reference type).
- Produces: `SleepAssertion` with `static let reason`, `isHeld`, `acquire()`, `release()`; `AppEnvironment.presentationDataStore`; `PresentationPageController(accessibilityIdentifier:dataStore:)` with `webView`, `onReady`, `onLoadFailed`, `onPresenterPopup`, `openExternally`, `lastReady`, `pageLoadCount`, `lastLoadedURL`, `load(_:allowedPort:)`, `popupRequested(for:navigationType:)`, `pageReportedReady(_:)`; the test-target extension's `pageText()` and `pressKey(_:)`; `PresentationWindow(role:screenFrame:)` with `Role` (`.audience`, `.presenter`), `FullScreenState` (`.windowed`, `.entering`, `.fullScreen`, `.exiting`), `page`, `container`, `deckWindowController`, `fullScreenState`, `targetFrame`, `settledFrame`, `isClosed`, `isAttached`, `onFullScreenChange`, `onClosed`, `requestFullScreenToggle` (test seam), `present(on:fullScreen:completion:)`, `takeDown(completion:)`, `attach(to:)`, `detach()`, `static enterTimeout`, `static exitTimeout`; `FullScreenProbe` with `Result`, `run()`; `XCTestCase.requireFullScreen()`, `requireSecondSpace()`; the test helpers `onScreenWindowNumbers()`, `powerAssertionIsListed(named:)`, `fullScreenPresentationWindows()`.

The person's decision 6 (2026-09-25): whether a hosted test host can enter system full screen is found out by a spike before any presenting code, locally and on CI, and recorded in the ledger. Where a host cannot, the full screen and Space assertions skip with `XCTSkip` and a reason, decided by one helper from the probe's observed result (never an environment variable), and the state machine stays tested through seams. Real full screen is the person's UI tests. No test may leave CI red because the host cannot enter full screen.

- [ ] **Step 0: The spike, run locally and on CI, and its result in the ledger**

`desktop/TapTests/FullScreenSpikeTests.swift`:

```swift
import XCTest
@testable import Tap

/// What this host does when a plain window asks for system full screen.
/// The test asserts nothing about full screen: it records what happened,
/// one line per event, so a local run and a CI run can be compared in
/// the ledger. (a) is one window entering and leaving; (b) is a second
/// window on the same screen entering while the first is in full screen,
/// which the two-display tests on one screen rely on.
final class FullScreenSpikeTests: HostedTestCase {
    func spikeWindow(_ title: String) -> NSWindow {
        let window = NSWindow(contentRect: NSScreen.screens[0].frame, styleMask: [.titled, .fullSizeContentView], backing: .buffered, defer: false)
        window.title = title
        window.collectionBehavior = [.fullScreenPrimary, .fullScreenDisallowsTiling]
        window.isReleasedWhenClosed = false
        window.backgroundColor = .black
        return window
    }

    func testFullScreenSpike() async throws {
        var events: [String] = []
        let names: [Notification.Name] = [NSWindow.willEnterFullScreenNotification, NSWindow.didEnterFullScreenNotification,
                                          NSWindow.willExitFullScreenNotification, NSWindow.didExitFullScreenNotification]
        let a = spikeWindow("spike A")
        let b = spikeWindow("spike B")
        var observers: [NSObjectProtocol] = []
        for window in [a, b] {
            for name in names {
                observers.append(NotificationCenter.default.addObserver(forName: name, object: window, queue: .main) { _ in
                    events.append("\(window.title): \(name.rawValue)")
                })
            }
        }
        defer { observers.forEach { NotificationCenter.default.removeObserver($0) } }
        let started = Date()
        func note(_ line: String) { events.append(String(format: "%5.1fs ", Date().timeIntervalSince(started)) + line) }

        note("active app: \(NSApp.isActive), screens: \(NSScreen.screens.count), separate Spaces: \(NSScreen.screensHaveSeparateSpaces)")
        a.orderFrontRegardless()
        a.toggleFullScreen(nil)
        try? await waitUntil(timeout: 8, "A in full screen") { a.styleMask.contains(.fullScreen) }
        note("A styleMask.fullScreen: \(a.styleMask.contains(.fullScreen)), on screen: \(onScreenWindowNumbers().contains(a.windowNumber))")

        b.orderFrontRegardless()
        b.toggleFullScreen(nil)
        try? await waitUntil(timeout: 8, "B in full screen") { b.styleMask.contains(.fullScreen) }
        note("B styleMask.fullScreen: \(b.styleMask.contains(.fullScreen)), on screen: \(onScreenWindowNumbers().contains(b.windowNumber)), A on screen: \(onScreenWindowNumbers().contains(a.windowNumber))")

        a.makeKeyAndOrderFront(nil)
        try? await waitUntil(timeout: 4, "A's Space active") { onScreenWindowNumbers().contains(a.windowNumber) }
        note("after makeKey A: A on screen: \(onScreenWindowNumbers().contains(a.windowNumber)), B on screen: \(onScreenWindowNumbers().contains(b.windowNumber))")

        for window in [b, a] where window.styleMask.contains(.fullScreen) {
            window.toggleFullScreen(nil)
            try? await waitUntil(timeout: 6, "\(window.title) out of full screen") { !window.styleMask.contains(.fullScreen) }
            note("\(window.title) left full screen: \(!window.styleMask.contains(.fullScreen))")
        }
        a.close()
        b.close()
        try? await waitUntil(timeout: 4, "the spike windows gone") { !onScreenWindowNumbers().contains(a.windowNumber) && !onScreenWindowNumbers().contains(b.windowNumber) }
        note("closed; any spike window still on screen: \(onScreenWindowNumbers().contains(a.windowNumber) || onScreenWindowNumbers().contains(b.windowNumber))")

        // Never a failure: the record is the result.
        let report = events.joined(separator: "\n")
        print("FullScreenSpike:\n\(report)")
        XCTContext.runActivity(named: "FullScreenSpike") { activity in
            let attachment = XCTAttachment(string: report)
            attachment.lifetime = .keepAlways
            activity.add(attachment)
        }
    }
}
```

Run locally: `make -C desktop project && make -C desktop test ONLY=TapTests/FullScreenSpikeTests 2>&1 | sed -n '/FullScreenSpike:/,/closed;/p'`.
Run on CI: push the spike alone on a scratch branch (`spike/full-screen`, off this branch's base), wait for the `Desktop Tests` job, and read the same lines from the job log (the hosted step prints test output). Delete the scratch branch afterwards.
Record in the ledger, for both runs: whether A entered ((a)), whether B entered while A was in full screen and whether A left the screen when B did ((b)), whether `makeKeyAndOrderFront` brought A back ((c)), whether both left full screen, and the time each step took. The spike file stays in the target: it asserts nothing, so it never goes red, and it is the reference for what the probe below measures. If (a) fails on either host, everything from Step 1 on still runs: the probe below turns the full screen assertions into skips there, and the state machine is tested through seams. If (b) fails, the two-display tests on one screen skip their Space assertions; the product's one-display talk does not depend on (b) since the person's decision 5 (the presenter view is a window inside the audience's Space, not a second Space).

- [ ] **Step 1: Write the probe, the window server helper and the failing tests**

`desktop/TapTests/Support/FullScreenProbe.swift`:

```swift
import AppKit
import XCTest

/// Whether this host can put a window into system full screen, found out
/// once per process by trying, the way the spike did: a probe window
/// enters and leaves, then a second one enters on the same screen while
/// the first is in. The result is observed, never configured: nothing in
/// the environment can claim a capability the host does not have.
@MainActor
enum FullScreenProbe {
    struct Result {
        /// One window can enter and leave system full screen.
        let available: Bool
        /// A second window on the same screen gets its own Space while the first is in full screen.
        let secondSpace: Bool
        let reason: String
    }

    private static var cached: Result?

    static func run() async -> Result {
        if let cached { return cached }
        func probeWindow() -> NSWindow {
            let window = NSWindow(contentRect: NSScreen.screens[0].frame, styleMask: [.titled, .fullSizeContentView], backing: .buffered, defer: false)
            window.collectionBehavior = [.fullScreenPrimary, .fullScreenDisallowsTiling]
            window.isReleasedWhenClosed = false
            window.backgroundColor = .black
            return window
        }
        func wait(_ seconds: TimeInterval, until condition: @escaping @MainActor () -> Bool) async -> Bool {
            let deadline = Date().addingTimeInterval(seconds)
            while Date() < deadline {
                if condition() { return true }
                try? await Task.sleep(nanoseconds: 50_000_000)
            }
            return condition()
        }
        let a = probeWindow()
        a.orderFrontRegardless()
        a.toggleFullScreen(nil)
        let entered = await wait(8) { a.styleMask.contains(.fullScreen) }
        var second = false
        if entered {
            let b = probeWindow()
            b.orderFrontRegardless()
            b.toggleFullScreen(nil)
            second = await wait(8) { b.styleMask.contains(.fullScreen) && !onScreenWindowNumbers().contains(a.windowNumber) }
            if b.styleMask.contains(.fullScreen) {
                b.toggleFullScreen(nil)
                _ = await wait(6) { !b.styleMask.contains(.fullScreen) }
            }
            b.close()
            a.toggleFullScreen(nil)
            _ = await wait(6) { !a.styleMask.contains(.fullScreen) }
        }
        a.close()
        let result = Result(available: entered, secondSpace: second,
                            reason: entered
                                ? (second ? "full screen and a second Space work" : "one window enters full screen; a second window on the same screen does not get its own Space")
                                : "a window asked for system full screen never entered it within 8 s (active app: \(NSApp.isActive))")
        cached = result
        return result
    }
}

extension XCTestCase {
    /// Skips the test unless this host can enter system full screen. Call
    /// it before any assertion on `fullScreenState == .fullScreen`,
    /// `styleMask.contains(.fullScreen)` or which Space is active.
    @MainActor
    func requireFullScreen() async throws {
        let result = await FullScreenProbe.run()
        if !result.available {
            throw XCTSkip("this host cannot enter system full screen (\(result.reason)); the state machine is covered by the seam tests, real full screen by the person's UI tests")
        }
    }

    /// Skips the test unless a second window on the same screen can have a
    /// Space of its own: what the two-display tests on one screen need.
    @MainActor
    func requireSecondSpace() async throws {
        try await requireFullScreen()
        let result = await FullScreenProbe.run()
        if !result.secondSpace {
            throw XCTSkip("this host gives one full screen Space per screen (\(result.reason)); two displays on one screen cannot be stood in for here, and a real second display is the person's manual pass")
        }
    }
}
```

`desktop/TapTests/Support/WindowServer.swift`:

```swift
import AppKit
@testable import Tap

/// The window numbers of this process's windows that the window server
/// has on screen, front to back. This is the window server's own truth,
/// not AppKit's bookkeeping, so it holds in a host with no key window and
/// on the CI runner alike. A window in a full screen Space that is not
/// the active one is not on screen, so a Space switch shows up here.
func onScreenWindowNumbers() -> [Int] {
    let pid = Int(ProcessInfo.processInfo.processIdentifier)
    let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
    return windows.compactMap { window in
        guard window[kCGWindowOwnerPID as String] as? Int == pid,
              window[kCGWindowIsOnscreen as String] as? Bool == true else { return nil }
        return window[kCGWindowNumber as String] as? Int
    }
}

/// Runs `pmset -g assertions` and reports whether this process holds an
/// assertion with `name`: the kernel's own view. The line is matched on
/// this process's pid, so the person's own Tap presenting, or their
/// `make uitest`, cannot make the check pass or fail.
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
    let ownPid = "pid \(ProcessInfo.processInfo.processIdentifier)("
    return String(decoding: data, as: UTF8.self).split(separator: "\n").contains { $0.contains(ownPid) && $0.contains(name) }
}

/// Every talk window that still exists and is in full screen. Empty is
/// what every ending must leave behind, on a host with full screen or without.
@MainActor
func fullScreenPresentationWindows() -> [PresentationWindow] {
    NSApp.windows.compactMap { $0 as? PresentationWindow }.filter { !$0.isClosed && $0.styleMask.contains(.fullScreen) }
}
```

`desktop/TapTests/Support/PresentationPageController+Tests.swift` (the test target only; the app never runs script in a page, and these two helpers do not ship in it):

```swift
import Foundation
@testable import Tap

extension PresentationPageController {
    /// The page's visible text.
    func pageText() async -> String {
        (try? await webView.evaluateJavaScript("document.body.innerText") as? String) ?? ""
    }

    /// Presses `key` (a KeyboardEvent key name, "ArrowRight" or "o") in
    /// the page, the way the page's own handler on `window` sees it, so a
    /// key test does not depend on which window the host has as key.
    func pressKey(_ key: String) async {
        let encoded = String(decoding: (try? JSONEncoder().encode([key])) ?? Data("[\"\"]".utf8), as: UTF8.self)
        let script = "window.dispatchEvent(new KeyboardEvent('keydown', {key: \(encoded)[0], bubbles: true, cancelable: true})); true"
        _ = try? await webView.evaluateJavaScript(script)
    }
}
```

In `desktop/TapTests/Support/HostedTestCase.swift`, add to `setUp` after the `slidePasteboard` line (the tests run inside the real Tap.app, and a talk page's localStorage and presenter cookie must not land in the person's own store):

```swift
        AppEnvironment.shared.presentationDataStore = WKWebsiteDataStore(forIdentifier: UUID())
```

and `import WebKit` at the top.

`desktop/TapTests/PresentationWindowTests.swift`:

```swift
import XCTest
import WebKit
@testable import Tap

final class PresentationWindowTests: HostedTestCase {
    override func tearDown() async throws {
        for window in NSApp.windows.compactMap({ $0 as? PresentationWindow }) where !window.isClosed { window.takeDown() }
        try await waitUntil(timeout: 10, "every talk window closed") { fullScreenPresentationWindows().isEmpty }
        try await super.tearDown()
    }

    func testTheSleepAssertionIsHeldOnlyBetweenAcquireAndRelease() {
        let assertion = SleepAssertion()
        XCTAssertFalse(assertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
        assertion.acquire()
        XCTAssertTrue(assertion.isHeld)
        XCTAssertTrue(powerAssertionIsListed(named: SleepAssertion.reason), "the kernel lists the assertion for this process")
        assertion.acquire()
        XCTAssertTrue(assertion.isHeld, "a second acquire changes nothing")
        assertion.release()
        XCTAssertFalse(assertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
        assertion.release()
        XCTAssertFalse(assertion.isHeld)
    }

    func testAWindowEntersItsOwnFullScreenSpaceAndLeavesItOnTakeDown() async throws {
        try await requireFullScreen()
        let screen = NSScreen.screens[0].frame
        let window = PresentationWindow(role: .audience, screenFrame: screen)
        var states: [PresentationWindow.FullScreenState] = []
        window.onFullScreenChange = { states.append($0) }
        XCTAssertEqual(window.fullScreenState, .windowed)
        XCTAssertEqual(window.level, .normal, "no covering level: a full screen Space is what covers the display")
        XCTAssertTrue(window.collectionBehavior.contains(.fullScreenPrimary))
        XCTAssertFalse(window.collectionBehavior.contains(.canJoinAllSpaces), "the window lives in its own Space, so Cmd-Tab to another app leaves it")
        XCTAssertTrue(window.canBecomeKey)
        XCTAssertFalse(window.isVisible)

        var settled = 0
        window.present(on: screen) { settled += 1 }
        XCTAssertEqual(window.targetFrame, screen)
        try await waitUntil(timeout: 10, "the window in full screen (state \(window.fullScreenState))") { window.fullScreenState == .fullScreen }
        XCTAssertTrue(window.styleMask.contains(.fullScreen), "AppKit's own flag, after the enter notification")
        XCTAssertEqual(window.settledFrame, screen)
        XCTAssertEqual(settled, 1)
        XCTAssertEqual(states, [.entering, .fullScreen])
        XCTAssertEqual(window.page.webView.frame.size, window.contentView?.bounds.size, "the page fills the window")
        try await waitUntil(timeout: 5, "the window server to show the window") { onScreenWindowNumbers().contains(window.windowNumber) }

        var closed = false
        window.takeDown { closed = true }
        try await waitUntil(timeout: 10, "the window closed (state \(window.fullScreenState))") { window.isClosed }
        XCTAssertTrue(closed)
        XCTAssertEqual(states, [.entering, .fullScreen, .exiting, .windowed], "it left full screen before it closed")
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        try await waitUntil(timeout: 5, "the window to leave the screen") { !onScreenWindowNumbers().contains(window.windowNumber) }
    }

    func testATakeDownDuringTheEntryStillCloses() async throws {
        try await requireFullScreen()
        let window = PresentationWindow(role: .presenter, screenFrame: NSScreen.screens[0].frame)
        window.present(on: NSScreen.screens[0].frame)
        XCTAssertEqual(window.fullScreenState, .entering)
        window.takeDown()
        try await waitUntil(timeout: 15, "the window closed (state \(window.fullScreenState))") { window.isClosed }
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty, "nothing is left in full screen")
    }

    func testATakeDownClosesEvenWhenTheExitNeverCompletes() async throws {
        try await requireFullScreen()
        let window = PresentationWindow(role: .audience, screenFrame: NSScreen.screens[0].frame)
        window.present(on: NSScreen.screens[0].frame)
        try await waitUntil(timeout: 10, "full screen") { window.fullScreenState == .fullScreen }
        // An AppKit that never answers the exit request: the deadline closes the window anyway.
        window.requestFullScreenToggle = {}
        let asked = Date()
        window.takeDown()
        try await waitUntil(timeout: PresentationWindow.exitTimeout + 5, "the deadline to close the window") { window.isClosed }
        XCTAssertGreaterThanOrEqual(Date().timeIntervalSince(asked), PresentationWindow.exitTimeout - 0.5)
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        try await waitUntil(timeout: 5, "the window off the screen") { !onScreenWindowNumbers().contains(window.windowNumber) }
    }

    func testAMoveToAnotherFrameLeavesAndReentersFullScreen() async throws {
        try await requireFullScreen()
        let screen = NSScreen.screens[0].frame
        let half = CGRect(x: screen.minX, y: screen.minY, width: screen.width / 2, height: screen.height)
        let window = PresentationWindow(role: .audience, screenFrame: screen)
        var states: [PresentationWindow.FullScreenState] = []
        window.onFullScreenChange = { states.append($0) }
        window.present(on: screen)
        try await waitUntil(timeout: 10, "full screen") { window.fullScreenState == .fullScreen }
        var moved = false
        // On one display the half frame is on the same display, but it is not the frame the window settled on, so the window goes out and in again.
        window.present(on: half) { moved = true }
        XCTAssertEqual(window.targetFrame, half)
        try await waitUntil(timeout: 15, "the move to settle (state \(window.fullScreenState))") { moved }
        XCTAssertEqual(window.fullScreenState, .fullScreen)
        XCTAssertEqual(window.settledFrame, half, "what it settled on is what it was asked for, not the display's frame")
        XCTAssertEqual(states, [.entering, .fullScreen, .exiting, .windowed, .entering, .fullScreen])
        var again = false
        window.present(on: half) { again = true }
        XCTAssertTrue(again, "already there: nothing to do, and no exit")
        XCTAssertEqual(states.count, 6)
    }

    /// The state machine without AppKit: the toggle is a seam that records
    /// the asks, and the test plays the delegate's notifications itself.
    /// This is what covers the machine on a host that cannot enter full
    /// screen, and it runs everywhere.
    func testTheStateMachineThroughTheSeams() async throws {
        let screen = NSScreen.screens[0].frame
        let other = CGRect(x: screen.minX, y: screen.minY, width: screen.width / 2, height: screen.height)
        let window = PresentationWindow(role: .audience, screenFrame: screen)
        var toggles = 0
        window.requestFullScreenToggle = { toggles += 1 }
        var states: [PresentationWindow.FullScreenState] = []
        window.onFullScreenChange = { states.append($0) }
        func enter() { window.windowDidEnterFullScreen(Notification(name: NSWindow.didEnterFullScreenNotification, object: window)) }
        func exit() { window.windowDidExitFullScreen(Notification(name: NSWindow.didExitFullScreenNotification, object: window)) }

        var settled = 0
        window.present(on: screen) { settled += 1 }
        XCTAssertEqual(toggles, 1)
        XCTAssertEqual(window.fullScreenState, .entering)
        enter()
        XCTAssertEqual(window.fullScreenState, .fullScreen)
        XCTAssertEqual(window.settledFrame, screen)
        XCTAssertEqual(settled, 1)

        // A move: exit, then enter on the new frame.
        window.present(on: other) { settled += 1 }
        XCTAssertEqual(toggles, 2)
        XCTAssertEqual(window.fullScreenState, .exiting)
        exit()
        XCTAssertEqual(toggles, 3, "out, then in again")
        XCTAssertEqual(window.fullScreenState, .entering)
        XCTAssertEqual(window.frame.size, other.size)
        enter()
        XCTAssertEqual(settled, 2)
        XCTAssertEqual(window.settledFrame, other)

        // The same frame again: nothing.
        window.present(on: other) { settled += 1 }
        XCTAssertEqual(toggles, 3)
        XCTAssertEqual(settled, 3)

        // A failed entry leaves a plain window; a failed exit on take-down closes anyway.
        window.present(on: screen) { settled += 1 }
        exit()
        XCTAssertEqual(window.fullScreenState, .entering)
        window.windowDidFailToEnterFullScreen(window)
        XCTAssertEqual(window.fullScreenState, .windowed)
        XCTAssertEqual(settled, 4, "a refused entry still settles, as a plain window")
        window.present(on: screen) { settled += 1 }
        enter()
        var closed = false
        window.takeDown { closed = true }
        XCTAssertEqual(window.fullScreenState, .exiting)
        window.windowDidFailToExitFullScreen(window)
        XCTAssertTrue(window.isClosed, "closed in whatever state it was in")
        XCTAssertTrue(closed)
        XCTAssertEqual(states, [.entering, .fullScreen, .exiting, .windowed, .entering, .fullScreen,
                                .exiting, .windowed, .entering, .windowed, .entering, .fullScreen, .exiting, .fullScreen])
        XCTAssertEqual(toggles, 7)

        // A plain window: present orders it front and settles at once; take-down closes at once.
        let plain = PresentationWindow(role: .presenter, screenFrame: screen)
        plain.requestFullScreenToggle = { XCTFail("a plain placement never asks for full screen") }
        var plainSettled = false
        plain.present(on: screen, fullScreen: false) { plainSettled = true }
        XCTAssertTrue(plainSettled)
        XCTAssertTrue(plain.isVisible)
        XCTAssertEqual(plain.fullScreenState, .windowed)
        plain.takeDown()
        XCTAssertTrue(plain.isClosed)
    }

    func testAChildWindowRidesInItsParentsSpace() async throws {
        let screen = NSScreen.screens[0].frame
        let audience = PresentationWindow(role: .audience, screenFrame: screen)
        let presenter = PresentationWindow(role: .presenter, screenFrame: screen)
        audience.present(on: screen, fullScreen: false)
        XCTAssertFalse(presenter.isVisible)
        presenter.attach(to: audience)
        XCTAssertTrue(presenter.isAttached)
        XCTAssertTrue(presenter.isVisible, "attaching shows it over the parent")
        XCTAssertTrue(audience.childWindows?.contains(presenter) == true)
        XCTAssertEqual(presenter.frame, audience.frame, "it covers the parent")
        XCTAssertTrue(presenter.collectionBehavior.contains(.fullScreenAuxiliary), "a child may live in a full screen Space")
        XCTAssertFalse(presenter.collectionBehavior.contains(.fullScreenPrimary))
        try await waitUntil(timeout: 5, "both on screen, the child in front") {
            let order = onScreenWindowNumbers()
            guard let parent = order.firstIndex(of: audience.windowNumber), let child = order.firstIndex(of: presenter.windowNumber) else { return false }
            return child < parent
        }
        presenter.detach()
        XCTAssertFalse(presenter.isAttached)
        XCTAssertFalse(presenter.isVisible)
        XCTAssertFalse(audience.childWindows?.contains(presenter) == true)
        XCTAssertTrue(presenter.collectionBehavior.contains(.fullScreenPrimary), "a detached window can have a Space of its own again")
        presenter.attach(to: audience)
        audience.takeDown()
        XCTAssertTrue(audience.isClosed)
        XCTAssertFalse(presenter.isAttached, "a parent going down lets its child go first")
        XCTAssertTrue(presenter.isClosed)
    }

    func testAMenuCannotTakeATalkWindowOutOfFullScreen() {
        let window = PresentationWindow(role: .audience, screenFrame: NSScreen.screens[0].frame)
        let item = NSMenuItem(title: "Enter Full Screen", action: #selector(NSWindow.toggleFullScreen(_:)), keyEquivalent: "")
        XCTAssertFalse(window.validateUserInterfaceItem(item), "View > Enter Full Screen does not reach a talk window; the controller owns its full screen")
    }

    func testThePageHasEverythingTapDevsBrowserWouldGiveIt() {
        let page = PresentationPageController(accessibilityIdentifier: "audience-page", dataStore: AppEnvironment.shared.presentationDataStore)
        _ = page.view
        XCTAssertTrue(page.webView.configuration.preferences.isElementFullscreenEnabled, "the F key's full screen works")
        XCTAssertTrue(page.webView.configuration.websiteDataStore.isPersistent, "localStorage survives the process")
        XCTAssertTrue(page.webView.configuration.websiteDataStore === AppEnvironment.shared.presentationDataStore, "the one store every talk shares")
        XCTAssertFalse(page.webView.configuration.websiteDataStore === WKWebsiteDataStore.default(), "in a test, never the person's own store")
        XCTAssertEqual(page.webView.accessibilityIdentifier(), "audience-page")
    }

    func testTheSKeysPopupBringsThePresenterWindowForwardAndLinksGoToTheBrowser() {
        let page = PresentationPageController(accessibilityIdentifier: "audience-page", dataStore: AppEnvironment.shared.presentationDataStore)
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
Expected: the test target does not compile (`SleepAssertion`, `PresentationWindow`, `PresentationPageController`, `presentationDataStore` are undefined).

- [ ] **Step 3: Write `SleepAssertion.swift` and the data store seam**

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

In `AppEnvironment.swift`, add after `slidePasteboard` (with `import WebKit` at the top):

```swift
    /// The data store every talk page uses: persistent, so the presenter
    /// layout and notes size (the page's localStorage) survive the process.
    /// A test replaces it with a store of its own, so a run never touches
    /// the person's.
    var presentationDataStore: WKWebsiteDataStore = .default()
```

- [ ] **Step 4: Write `PresentationPageController.swift`**

```swift
import AppKit
import WebKit

/// One of tap's own pages for a talk, the audience page or the presenter
/// page, in a `WKWebView` with what tap dev's browser gives it: element
/// full screen for the F key, a persistent data store so the presenter
/// layout and notes size (the page's localStorage, keyed by origin, which
/// is why the deck keeps one port) survive between launches, and the
/// window the S key opens answered by bringing the presenter window
/// forward. The app drives the page only through the URL it loads (its
/// start slide is in the fragment, the presenter key in the query) and
/// through tap's hub, and reads it only through the tapReady handler.
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

    init(accessibilityIdentifier: String, dataStore: WKWebsiteDataStore) {
        let configuration = WKWebViewConfiguration()
        configuration.userContentController = WKUserContentController()
        configuration.websiteDataStore = dataStore
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

/// A window for one of a talk's pages. It enters its own macOS full screen
/// Space on the display it is given (system full screen, as a browser's
/// is), so Cmd-Tab to another app, the menu bar at the top edge and the
/// page's own F key work as they do in a browser. It is windowed,
/// entering, in full screen or exiting. A move to another display goes
/// exit, move, enter. A take-down goes exit, close, and closes anyway
/// after `exitTimeout`, so no ending can leave a window in full screen.
/// On one display the presenter window is instead a child of the audience
/// window, riding in its Space (`attach(to:)`). Escape and Option-Tab are
/// the controller's key monitor's.
final class PresentationWindow: NSWindow, NSWindowDelegate {
    enum Role: Equatable {
        case audience
        case presenter
    }

    enum FullScreenState: Equatable {
        case windowed
        case entering
        case fullScreen
        case exiting
    }

    let role: Role
    let page: PresentationPageController
    /// Holds the page and, in the presenter window, the toolbar and the REC dot over it.
    let container = NSView()
    /// The deck this window presents, for menu actions that reach this window first.
    weak var deckWindowController: DeckWindowController?
    private(set) var fullScreenState: FullScreenState = .windowed
    /// The screen frame this window is meant to fill: what the controller
    /// asked for, set before any transition, so a test can read it while
    /// the window server is still animating.
    private(set) var targetFrame: CGRect
    /// The frame the window last settled on, in full screen or as a plain
    /// window. A request for the same frame is already satisfied: in full
    /// screen the window's own frame is the whole display, so it cannot
    /// tell a half-screen "display" from the display itself.
    private(set) var settledFrame: CGRect?
    private(set) var isClosed = false
    /// True while this window is a child of another (the presenter view over the audience on one display).
    private(set) var isAttached = false
    var onFullScreenChange: ((FullScreenState) -> Void)?
    var onClosed: (() -> Void)?
    /// Asks AppKit to toggle full screen. A test replaces it with a no-op
    /// to stand for a transition that never completes, or with a recorder.
    var requestFullScreenToggle: (() -> Void)?
    /// How long an entry may take before the window is treated as
    /// windowed, and how long an exit may take before a take-down closes
    /// the window regardless.
    static let enterTimeout: TimeInterval = 5
    static let exitTimeout: TimeInterval = 3

    private var settled: (() -> Void)?
    private var closedCompletion: (() -> Void)?
    private var pendingFrame: CGRect?
    private var wantsFullScreen = true
    private var closeWhenSettled = false
    private var transitionDeadline: DispatchWorkItem?

    init(role: Role, screenFrame: CGRect) {
        self.role = role
        targetFrame = screenFrame
        page = PresentationPageController(accessibilityIdentifier: role == .audience ? "audience-page" : "presenter-page",
                                          dataStore: AppEnvironment.shared.presentationDataStore)
        // A titled window with its chrome hidden: AppKit puts a titled
        // window in a full screen Space and hides the title bar there.
        super.init(contentRect: screenFrame, styleMask: [.titled, .fullSizeContentView], backing: .buffered, defer: false)
        delegate = self
        titleVisibility = .hidden
        titlebarAppearsTransparent = true
        for kind in [.closeButton, .miniaturizeButton, .zoomButton] as [NSWindow.ButtonType] {
            standardWindowButton(kind)?.isHidden = true
        }
        isMovable = false
        level = .normal
        collectionBehavior = [.fullScreenPrimary, .fullScreenDisallowsTiling, .ignoresCycle]
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

    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { true }

    /// View > Enter Full Screen (Ctrl+Cmd+F) never reaches a talk window:
    /// the controller owns its full screen, and a window taken out from
    /// under it would stay a plain window.
    override func validateUserInterfaceItem(_ item: NSValidatedUserInterfaceItem) -> Bool {
        if item.action == #selector(NSWindow.toggleFullScreen(_:)) { return false }
        return super.validateUserInterfaceItem(item)
    }

    // MARK: Full screen

    /// Fills `frame`, a screen's frame, in that screen's own full screen
    /// Space, and calls `completion` once the window is settled there (or
    /// has failed to enter and stays a plain window over the frame). From
    /// full screen on another frame the window exits, moves and enters
    /// again; the frame it settled on last is already satisfied. During a
    /// transition the frame waits for it to end. With `fullScreen` false
    /// the window is a plain window over the frame: what a two-display
    /// talk gets while "Displays have separate Spaces" is off, since one
    /// full screen Space would black out the other display, and what a
    /// test host without full screen gets.
    func present(on frame: CGRect, fullScreen: Bool = true, completion: @escaping () -> Void = {}) {
        guard !isClosed else { return completion() }
        targetFrame = frame
        wantsFullScreen = fullScreen
        settled = completion
        switch fullScreenState {
        case .windowed:
            setFrame(frame, display: true)
            orderFrontRegardless()
            if wantsFullScreen {
                enterFullScreen()
            } else {
                settledFrame = frame
                settle()
            }
        case .fullScreen:
            if wantsFullScreen, settledFrame == frame {
                settle()
            } else {
                pendingFrame = frame
                exitFullScreen()
            }
        case .entering, .exiting:
            pendingFrame = frame
        }
    }

    /// Leaves full screen and closes, then calls `completion`. A
    /// transition in flight finishes first; one that never finishes is cut
    /// short by the deadline, and the window is closed in whatever state
    /// it is in, which drops its Space. A window with children lets them
    /// go first.
    func takeDown(completion: @escaping () -> Void = {}) {
        guard !isClosed else { return completion() }
        for child in childWindows?.compactMap({ $0 as? PresentationWindow }) ?? [] {
            child.detach()
            child.takeDown()
        }
        settled = nil
        pendingFrame = nil
        closedCompletion = completion
        closeWhenSettled = true
        switch fullScreenState {
        case .windowed:
            finishClose()
        case .fullScreen:
            exitFullScreen()
        case .entering, .exiting:
            armDeadline(Self.exitTimeout)
        }
    }

    /// Makes this window a child of `parent`, covering it, in its Space:
    /// the presenter view over the audience on one display. A child may
    /// not have a Space of its own, so the collection behaviour changes
    /// with it and changes back on `detach`.
    func attach(to parent: NSWindow) {
        guard !isClosed, !isAttached, fullScreenState == .windowed else { return }
        isAttached = true
        collectionBehavior = [.fullScreenAuxiliary, .ignoresCycle]
        setFrame(parent.frame, display: true)
        parent.addChildWindow(self, ordered: .above)
    }

    /// Takes the window out of its parent and off the screen.
    func detach() {
        guard isAttached else { return }
        isAttached = false
        parent?.removeChildWindow(self)
        orderOut(nil)
        collectionBehavior = [.fullScreenPrimary, .fullScreenDisallowsTiling, .ignoresCycle]
    }

    private func enterFullScreen() {
        fullScreenState = .entering
        onFullScreenChange?(.entering)
        armDeadline(Self.enterTimeout)
        toggle()
    }

    private func exitFullScreen() {
        fullScreenState = .exiting
        onFullScreenChange?(.exiting)
        armDeadline(Self.exitTimeout)
        toggle()
    }

    private func toggle() {
        if let requestFullScreenToggle {
            requestFullScreenToggle()
        } else {
            toggleFullScreen(nil)
        }
    }

    private func armDeadline(_ seconds: TimeInterval) {
        transitionDeadline?.cancel()
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated { self?.transitionTimedOut() }
        }
        transitionDeadline = work
        DispatchQueue.main.asyncAfter(deadline: .now() + seconds, execute: work)
    }

    private func clearDeadline() {
        transitionDeadline?.cancel()
        transitionDeadline = nil
    }

    /// AppKit never finished the transition. The state follows the style
    /// mask, a take-down closes the window as it is, and a presentation
    /// goes on with the window as it is.
    private func transitionTimedOut() {
        transitionDeadline = nil
        fullScreenState = styleMask.contains(.fullScreen) ? .fullScreen : .windowed
        onFullScreenChange?(fullScreenState)
        if closeWhenSettled {
            finishClose()
            return
        }
        pendingFrame = nil
        settledFrame = targetFrame
        let completion = settled
        settled = nil
        completion?()
    }

    /// A transition finished. A queued close runs, then a queued move,
    /// then the caller's completion.
    private func settle() {
        if closeWhenSettled {
            if fullScreenState == .fullScreen {
                exitFullScreen()
            } else {
                finishClose()
            }
            return
        }
        if let frame = pendingFrame {
            pendingFrame = nil
            if fullScreenState == .fullScreen, !wantsFullScreen || settledFrame != frame {
                targetFrame = frame
                pendingFrame = frame
                exitFullScreen()
                return
            }
            if fullScreenState == .windowed {
                targetFrame = frame
                setFrame(frame, display: true)
                orderFrontRegardless()
                if wantsFullScreen {
                    enterFullScreen()
                    return
                }
            }
        }
        settledFrame = targetFrame
        let completion = settled
        settled = nil
        completion?()
    }

    private func finishClose() {
        clearDeadline()
        close()
    }

    // MARK: NSWindowDelegate

    func windowDidEnterFullScreen(_ notification: Notification) {
        clearDeadline()
        fullScreenState = .fullScreen
        onFullScreenChange?(.fullScreen)
        settle()
    }

    func windowDidFailToEnterFullScreen(_ window: NSWindow) {
        clearDeadline()
        fullScreenState = .windowed
        onFullScreenChange?(.windowed)
        pendingFrame = nil
        settle()
    }

    func windowDidExitFullScreen(_ notification: Notification) {
        clearDeadline()
        fullScreenState = .windowed
        onFullScreenChange?(.windowed)
        settle()
    }

    func windowDidFailToExitFullScreen(_ window: NSWindow) {
        clearDeadline()
        fullScreenState = .fullScreen
        onFullScreenChange?(.fullScreen)
        pendingFrame = nil
        if closeWhenSettled {
            finishClose()
            return
        }
        settle()
    }

    /// In full screen the menu bar and the Dock stay out of the way until
    /// the pointer asks for them, as in a browser's full screen.
    func window(_ window: NSWindow, willUseFullScreenPresentationOptions proposedOptions: NSApplication.PresentationOptions) -> NSApplication.PresentationOptions {
        [.fullScreen, .autoHideMenuBar, .autoHideDock]
    }

    func windowWillClose(_ notification: Notification) {
        isClosed = true
        clearDeadline()
        if isAttached {
            isAttached = false
            parent?.removeChildWindow(self)
        }
        let completion = closedCompletion
        closedCompletion = nil
        onClosed?()
        completion?()
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

`windowWillEnterFullScreen` and `windowWillExitFullScreen` are not needed: the state is set when the toggle is asked for, so a `takeDown` that arrives between the request and AppKit's notification already sees `.entering`. The delegate methods are internal, so the seam test can play them.

- [ ] **Step 6: Run the tests one at a time**

Run: `make -C desktop project` (the new folder must enter the generated project), then the required runs:

```bash
make -C desktop test ONLY=TapTests/PresentationWindowTests/testTheStateMachineThroughTheSeams
make -C desktop test ONLY=TapTests/PresentationWindowTests/testAWindowEntersItsOwnFullScreenSpaceAndLeavesItOnTakeDown
make -C desktop test ONLY=TapTests/PresentationWindowTests/testAChildWindowRidesInItsParentsSpace
make -C desktop test ONLY=TapTests/PresentationWindowTests/testAMoveToAnotherFrameLeavesAndReentersFullScreen
```

Optional, when the branch is ahead: `testTheSleepAssertionIsHeldOnlyBetweenAcquireAndRelease`, `testATakeDownDuringTheEntryStillCloses`, `testATakeDownClosesEvenWhenTheExitNeverCompletes`, `testAMenuCannotTakeATalkWindowOutOfFullScreen`, `testThePageHasEverythingTapDevsBrowserWouldGiveIt`, `testTheSKeysPopupBringsThePresenterWindowForwardAndLinksGoToTheBrowser`. Expected: the seam test and the child test pass everywhere; the full screen tests pass on a host the spike found able, and report `skipped` with the probe's reason on one it found unable (the ledger says which this machine is). A skipped full screen test on a host the spike found able is a defect in the probe or the window, not a pass.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted, the ones that can leave a window in full screen or the assertion held first: in `takeDown`, close at once from `.fullScreen` without exiting (expected: `testTheStateMachineThroughTheSeams` fails on `.exiting`, and the entry test on `states` where it runs); in `armDeadline`, never schedule the work item (expected: `testATakeDownClosesEvenWhenTheExitNeverCompletes` times out where it runs); in `transitionTimedOut`, skip `finishClose` (expected: the same); in `present(on:)`, compare `screen?.frame` instead of `settledFrame` (expected: the seam test fails on `toggles == 3` after the repeated frame); in `takeDown`, skip the children (expected: the child test fails on `presenter.isClosed`); in `attach`, keep `.fullScreenPrimary` (expected: the child test fails on the collection behaviour); in `validateUserInterfaceItem`, return `super` for `toggleFullScreen` (expected: the menu test fails); in `release`, drop `IOPMAssertionRelease(identifier)` (expected: `testTheSleepAssertionIsHeldOnlyBetweenAcquireAndRelease` fails on the second `pmset` check); in `acquire`, never set `isHeld` (expected: it fails on `isHeld`); in `init`, drop `.fullScreenPrimary` (expected: the entry test fails on `collectionBehavior` where it runs, and the seam test on nothing: note it, the probe-able host is where this shows); in the page's init, use `.default()` instead of `dataStore` (expected: the page test fails on the store); in `popupRequested`, drop the port check (expected: the popup count is 2); in the page's init, drop `isElementFullscreenEnabled = true` (expected: the page test fails).

```bash
git add desktop/Tap/Presenting desktop/Tap/App/AppEnvironment.swift desktop/TapTests
git commit -m "feat(desktop): the full screen spike and probe, the sleep assertion, a talk page in a web view and a window that enters its own full screen Space"
```

---

### Task 4: The talk: start, the deck's port, ready, the windows in their Spaces, stop, and every way the sleep assertion is released

**Files:**
- Create: `desktop/Tap/Presenting/PresentationController.swift`
- Modify: `desktop/Tap/Documents/DeckSessionController.swift` (`presentation`, `saveForPresenting`, `stop`)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (`init`: hand the controller its deck window controller)
- Modify: `desktop/Tap/App/AppEnvironment.swift` (`displayAssignments`, `deckPorts`, `presentationSettings`, `presentExecutableURL`, `presentSessionConfiguration()`, the talk count, `endingTalks`)
- Modify: `desktop/Tap/App/AppDelegate.swift` (`deck(owning:)`, `applicationWillTerminate`, `stopAllPresentations`)
- Modify: `desktop/Tap/TapLog/TapLogWindowController.swift` (`reload` lists the talk's log)
- Modify: `desktop/TapTests/Support/HostedTestCase.swift` (fresh seams per test)
- Modify: `desktop/TapTests/Support/FakeTapScripts.swift` (`readyAndWaiting`)
- Create: `desktop/TapTests/Support/PresentingTestCase.swift`
- Test: `desktop/TapTests/PresentingTests.swift`

**Interfaces:**
- Consumes: Task 1's `TapSession(deckURL:configuration:command:)`, `quit(timeout:)`, `extendQuit(timeout:)`, `command.port`, `TapEvent` cases; Task 2's `DisplayArrangement`, `DisplayAssignmentStore`, `DeckPortStore` (`port(for:)`, `setPort(_:for:)`, `static suggestedPort(for:)`), `PresentationOptions.command(port:)`, `PresentationSettingsStore`, `RecordingStatus`, `ScreenInfo`; Task 3's `SleepAssertion`, `PresentationWindow` (`present(on:fullScreen:completion:)`, `takeDown(completion:)`, `attach(to:)`, `detach()`, `fullScreenState`, `settledFrame`, `isAttached`, `onClosed`), `PresentationPageController`, `FullScreenProbe`, `requireFullScreen()`, `fullScreenPresentationWindows()`, `AppEnvironment.presentationDataStore`; D2's `TapClient.authorizePresenter()`, `presenterCookie`, `presenterCookieName`, `openSocket()`; D3's `DeckSessionController.jumpToSlide(number:)`, `isContentEdited`, `DeckDocument.save(to:ofType:for:completionHandler:)`.
- Produces: `PresentationController` with `State` (`.idle`, `.starting`, `.presenting`, `.stopping`, `.failed(String)`), `PendingQuestion`, `state`, `options`, `session`, `client`, `audienceWindow`, `presenterWindow`, `frontWindow`, `presenterIsShownOverAudience`, `arrangement`, `currentArrangement`, `sleepAssertion`, `lastSlide`, `lastTalkLog`, `recording`, `windowsShown`, `windowsAreSettled`, `windowsGoingDown`, `pendingQuestions`, `pendingQuestion`, `isActive`, `canStart`, `usesFullScreen`, `screens`, `screensHaveSeparateSpaces`, `fullScreenAllowed`, `authorizePresenter`, `displayAssignments`, `deckPorts`, `deckWindowController`, `onStateChange`, `onEvent`, `onStopped`, `onRecordingChange`, `onQuestion`, `onFailed`, `start(_:)`, `stop()`, `answer(id:value:)`, `toggleFrontWindow()`, `bringPresenterWindowForward()`, `handle(_:)`, `static installPresenterCookie(_:into:)`, `static saveFailureMessage(for:)`, `static showWindowsFallbackInterval`, `static quitTimeout`, `static quitTimeoutBeforeReady`, `static quitTimeoutWithRecording`; `DeckSessionController.presentation`, `presentationIfCreated`, `saveForPresenting(completion:)`; `AppEnvironment.displayAssignments`, `deckPorts`, `presentationSettings`, `presentExecutableURL`, `presentSessionConfiguration()`, `presentingCount`, `isPresenting`, `updatesMayInterrupt`, `noteTalkStarted()`, `noteTalkEnded()`, `endingTalks`, `retainEndingTalk(_:)`, `releaseEndingTalk(_:)`, `static presentingDidChangeNotification`; `AppDelegate.stopAllPresentations()`; `PresentingTestCase` with `fullScreenAvailable`, `writeRecordingConsent(_:)`, `removeRecordingConsent()`, `settingsFile`, `oneScreen()`, `halfScreens()`, `openDeckForPresenting(_:slides:)`, `startPresenting(_:_:)`, `stopPresenting(_:)`, `presenterCookieInTheTalkStore()`, `isRunning(_:)`, `WeakTalk`; `FakeTapScripts.readyAndWaiting()`.

The person's decision 5 (2026-09-25): on one display the audience view is full screen and the presenter view is a child window over it, inside the same Space, shown and hidden by Option-Tab and the S key with no Space animation; on two displays each window has its own Space. So a one-display talk never asks AppKit for a second Space, and a mid-talk sheet is the one Space switch a one-display talk can see (the deck window's Space, and back).

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
```

In `desktop/TapTests/Support/HostedTestCase.swift`, add to `setUp` after the `presentationDataStore` line (Task 3):

```swift
        AppEnvironment.shared.displayAssignments = DisplayAssignmentStore(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.displays.\(UUID().uuidString)")))
        AppEnvironment.shared.deckPorts = DeckPortStore(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.ports.\(UUID().uuidString)")))
        AppEnvironment.shared.presentationSettings = PresentationSettingsStore(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.present.\(UUID().uuidString)")))
        AppEnvironment.shared.presentExecutableURL = nil
```

`desktop/TapTests/Support/PresentingTestCase.swift`:

```swift
import XCTest
import WebKit
@testable import Tap

/// A weak handle on a talk, for a test that lets go of the deck that
/// owned it and wants to see who keeps it alive.
final class WeakTalk {
    weak var presentation: PresentationController?
    init(_ presentation: PresentationController) { self.presentation = presentation }
}

/// A hosted test that runs a talk with the bundled tap present, on the one
/// screen the machine has. The consent question is answered ahead of time
/// in the test's own settings folder, so tap asks nothing and records
/// nothing; a test that wants the question removes the answer. On a host
/// the probe found unable to enter full screen, the talk windows are
/// plain windows over their frames and the tests that assert full screen
/// skip; everything else runs.
@MainActor
class PresentingTestCase: HostedTestCase {
    /// The probe's verdict for this process (Task 3), never an environment variable.
    private(set) var fullScreenAvailable = false

    override func setUp() async throws {
        try await super.setUp()
        try writeRecordingConsent(false)
        fullScreenAvailable = await FullScreenProbe.run().available
    }

    override func tearDown() async throws {
        for document in NSDocumentController.shared.documents {
            (document as? DeckDocument)?.sessionController?.presentationIfCreated?.stop()
        }
        try await super.tearDown()
        // A talk that is stopping outlives its deck; the next test starts
        // once its process is gone, its count is out and no window is left
        // in full screen.
        try await waitUntil(timeout: 40, "every talk to end (\(AppEnvironment.shared.presentingCount) counted, \(AppEnvironment.shared.endingTalks.count) ending)") {
            !AppEnvironment.shared.isPresenting && AppEnvironment.shared.endingTalks.isEmpty
        }
        try await waitUntil(timeout: 20, "every talk window to go away") {
            fullScreenPresentationWindows().isEmpty && !NSApp.windows.contains { ($0 as? PresentationWindow).map { !$0.isClosed } ?? false }
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
    /// the right half for the projector. A full screen window fills the
    /// whole display whichever half it was given, so what a test checks
    /// is the frame the controller asked for (`targetFrame`) and settled
    /// on (`settledFrame`), never the window's own frame. Two Spaces on
    /// one screen need `requireSecondSpace()`.
    func halfScreens() -> [ScreenInfo] {
        let frame = NSScreen.screens[0].frame
        let left = CGRect(x: frame.minX, y: frame.minY, width: (frame.width / 2).rounded(.down), height: frame.height)
        let right = CGRect(x: left.maxX, y: frame.minY, width: frame.width - left.width, height: frame.height)
        return [ScreenInfo(name: "Built-in Display", frame: left, isBuiltIn: true),
                ScreenInfo(name: "Projector", frame: right, isBuiltIn: false)]
    }

    /// Opens a copy of the deck, waits for its preview and boxes, points
    /// its talk at the one real screen, and lets the talk use full screen
    /// only where the probe found it works.
    func openDeckForPresenting(_ name: String = "ops.md", slides: Int = 7) async throws -> (DeckDocument, DeckSessionController) {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck(name))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: slides)
        let screens = oneScreen()
        controller.presentation.screens = { screens }
        let available = fullScreenAvailable
        controller.presentation.fullScreenAllowed = { available }
        return (document, controller)
    }

    /// Starts a talk and waits until it is presenting and every window is
    /// where it was asked to be (in full screen, or a plain window on a
    /// host without it, or after a refused entry, which the talk's log says).
    func startPresenting(_ controller: DeckSessionController, _ options: PresentationOptions, timeout: TimeInterval = 40) async throws {
        controller.presentation.start(options)
        try await waitUntil(timeout: timeout, "the talk to be presenting (state \(controller.presentation.state))") {
            controller.presentation.state == .presenting
        }
        try await waitUntil(timeout: 20, "the talk windows to settle") { controller.presentation.windowsAreSettled }
    }

    func stopPresenting(_ controller: DeckSessionController) async throws {
        controller.presentation.stop()
        try await waitUntil(timeout: 30, "the talk to end") { controller.presentation.state == .idle }
        try await waitUntil(timeout: 10, "the talk windows to close") { controller.presentation.windowsGoingDown.isEmpty }
    }

    /// The presenter cookie in the talk pages' data store, if any.
    func presenterCookieInTheTalkStore() async -> String? {
        let cookies: [HTTPCookie] = await withCheckedContinuation { continuation in
            AppEnvironment.shared.presentationDataStore.httpCookieStore.getAllCookies { continuation.resume(returning: $0) }
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
        XCTAssertTrue(AppEnvironment.shared.isPresenting, "counted from the start, so no other deck can start during the save")
        // The process exists only once the save has completed.
        try await waitUntil(timeout: 10, "tap present to be started") { controller.presentation.session != nil }
        let onDisk = try String(contentsOf: deck, encoding: .utf8)
        XCTAssertTrue(onDisk.contains("# One edited"), "the buffer reached the file before tap present started")
        XCTAssertFalse(document.isDocumentEdited)
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
    }

    func testStopThenPlayDuringTheSaveStartsOneTalk() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        // A save that takes its time, so Stop and Play can land inside it.
        let realSave = presentation.saveDeck
        var completions: [(Error?) -> Void] = []
        presentation.saveDeck = { completion in completions.append(completion) }
        presentation.start(PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertEqual(completions.count, 1)
        presentation.stop()
        XCTAssertEqual(presentation.state, .idle, "nothing to quit yet")
        presentation.start(PresentationOptions(mode: .rehearse, startSlide: 2))
        XCTAssertEqual(completions.count, 2)
        completions[0](nil)
        XCTAssertNil(presentation.session, "the old save's completion belongs to a start that was stopped")
        completions[1](nil)
        let session = try XCTUnwrap(presentation.session, "the new save's completion launches")
        XCTAssertEqual(presentation.options?.startSlide, 2)
        presentation.saveDeck = realSave
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
        XCTAssertTrue(presentation.session === session, "one process, not two")
    }

    func testOneDisplayIsOneSpaceWithThePresenterViewOverIt() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 2))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let screen = NSScreen.screens[0].frame
        XCTAssertEqual(audience.targetFrame, screen)
        XCTAssertEqual(audience.settledFrame, screen)
        XCTAssertEqual(presenter.fullScreenState, .windowed, "the presenter view never has a Space of its own on one display")
        XCTAssertFalse(presenter.isVisible, "hidden until Option-Tab or the S key")
        XCTAssertFalse(presenter.isAttached)
        XCTAssertFalse(presentation.presenterIsShownOverAudience)
        XCTAssertTrue(audience.isVisible)
        XCTAssertTrue(audience.deckWindowController === controller.editor.window?.windowController as? DeckWindowController)
        XCTAssertTrue(presentation.frontWindow === audience, "on one display the audience is in front")
        try await waitUntil(timeout: 5, "the audience on screen, the presenter not") {
            let order = onScreenWindowNumbers()
            return order.contains(audience.windowNumber) && !order.contains(presenter.windowNumber)
        }

        XCTAssertEqual(audience.page.lastLoadedURL?.fragment, "2")
        XCTAssertEqual(presenter.page.lastLoadedURL?.fragment, "2")
        let client = try XCTUnwrap(presentation.client)
        XCTAssertEqual(audience.page.lastLoadedURL?.query, "launch=\(client.ready.launch)")
        XCTAssertEqual(presenter.page.lastLoadedURL?.query, "key=\(client.ready.presenter)", "the presenter page brings its own key")
        try await waitUntil(timeout: 20, "the audience page on slide 2") { audience.page.lastReady?.slide == 2 }
        try await waitUntil(timeout: 20, "the presenter page ready, though hidden") { presenter.page.lastReady != nil }
        let cookie = await presenterCookieInTheTalkStore()
        XCTAssertNotNil(cookie)
        XCTAssertEqual(cookie, client.presenterCookie, "the audience page holds the hub's presenter cookie too")

        // Option-Tab: the presenter view comes over the audience, in the same Space, no animation.
        presentation.toggleFrontWindow()
        XCTAssertTrue(presentation.presenterIsShownOverAudience)
        XCTAssertTrue(presenter.isAttached)
        XCTAssertTrue(audience.childWindows?.contains(presenter) == true)
        XCTAssertTrue(presenter.isVisible)
        XCTAssertEqual(presenter.frame, audience.frame, "it covers the audience view")
        XCTAssertTrue(presentation.frontWindow === presenter)
        try await waitUntil(timeout: 5, "both on screen, the presenter in front") {
            let order = onScreenWindowNumbers()
            guard let a = order.firstIndex(of: audience.windowNumber), let p = order.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
        XCTAssertTrue(audience.isVisible, "the audience view is still there underneath")
        presentation.toggleFrontWindow()
        XCTAssertFalse(presentation.presenterIsShownOverAudience)
        XCTAssertFalse(presenter.isVisible)
        XCTAssertTrue(presentation.frontWindow === audience)
        try await waitUntil(timeout: 5, "the presenter off the screen again") { !onScreenWindowNumbers().contains(presenter.windowNumber) }
    }

    func testTheAudienceWindowIsInItsOwnFullScreenSpace() async throws {
        try await requireFullScreen()
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        XCTAssertTrue(presentation.usesFullScreen)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.fullScreenState, .fullScreen, "the audience view has the display's full screen Space")
        XCTAssertTrue(audience.styleMask.contains(.fullScreen))
        XCTAssertEqual(fullScreenPresentationWindows().count, 1, "one Space on one display")
        presentation.toggleFrontWindow()
        XCTAssertEqual(presenter.fullScreenState, .windowed)
        XCTAssertTrue(presenter.collectionBehavior.contains(.fullScreenAuxiliary), "the child rides in the audience's Space")
        try await waitUntil(timeout: 5, "the presenter over the audience in the full screen Space") {
            let order = onScreenWindowNumbers()
            return order.contains(presenter.windowNumber) && order.contains(audience.windowNumber)
        }
        XCTAssertEqual(fullScreenPresentationWindows().count, 1)
    }

    func testTheMacStaysAwake() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertTrue(presentation.sleepAssertion.isHeld)
        XCTAssertTrue(powerAssertionIsListed(named: SleepAssertion.reason), "the kernel holds the display awake for this process")
        try await stopPresenting(controller)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
    }

    /// Starts a talk and closes its deck, letting go of every reference of
    /// its own, so what keeps the talk alive afterwards is the app's doing.
    func startTalkAndCloseTheDeck() async throws -> (talk: WeakTalk, pid: Int32) {
        let (document, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let pid = try XCTUnwrap(controller.presentation.session?.processIdentifier)
        let talk = WeakTalk(controller.presentation)
        document.close()
        return (talk, pid)
    }

    func testClosingTheDeckMidTalkStillEndsTheProcessAndFreesPlay() async throws {
        let (talk, pid) = try await startTalkAndCloseTheDeck()
        let presentation = try XCTUnwrap(talk.presentation, "the ending talk is kept alive until its process exits")
        XCTAssertEqual(presentation.state, .stopping)
        XCTAssertEqual(AppEnvironment.shared.endingTalks.count, 1)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertTrue(AppEnvironment.shared.isPresenting, "still counted while tap present runs")
        try await waitUntil(timeout: 20, "tap present to exit") { !self.isRunning(pid) }
        try await waitUntil(timeout: 10, "the talk to be counted out") { !AppEnvironment.shared.isPresenting }
        XCTAssertTrue(AppEnvironment.shared.endingTalks.isEmpty, "the talk let go of itself once its process was gone")
        try await waitUntil(timeout: 10, "no talk window left") { fullScreenPresentationWindows().isEmpty }

        // Another deck can present at once.
        let (_, other) = try await openDeckForPresenting()
        XCTAssertTrue(other.presentation.canStart)
        XCTAssertTrue(AppEnvironment.shared.updatesMayInterrupt)
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
        XCTAssertFalse(AppEnvironment.shared.isPresenting)
        try await waitUntil(timeout: 10, "no window left in full screen") { fullScreenPresentationWindows().isEmpty && presentation.windowsGoingDown.isEmpty }
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
        try await waitUntil(timeout: 10, "no window left in full screen") { fullScreenPresentationWindows().isEmpty && presentation.windowsGoingDown.isEmpty }
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
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        presentation.toggleFrontWindow()
        XCTAssertTrue(presenter.isAttached)

        presentation.stop()
        XCTAssertEqual(presentation.state, .stopping)
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertNil(presentation.presenterWindow)
        XCTAssertFalse(presenter.isAttached, "the child goes first")
        XCTAssertTrue(presenter.isClosed, "a plain window closes at once")
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        try await waitUntil(timeout: 30, "the talk to end") { presentation.state == .idle }
        XCTAssertEqual(controller.currentSlideNumber, 4, "the cursor is on the last slide presented")
        XCTAssertTrue(presentation.lastTalkLog?.text.contains("tap quit") == true, "the talk's log is kept after the talk")
        try await waitUntil(timeout: 10, "tap present to exit") { !self.isRunning(pid) }
        try await waitUntil(timeout: 10, "the windows closed") { presentation.windowsGoingDown.isEmpty && audience.isClosed }
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty, "nothing is left in full screen")
        XCTAssertEqual(controller.session.processIdentifier, devPid, "tap dev is untouched")
        if case .running = controller.session.state {} else { XCTFail("tap dev keeps running the preview") }
    }

    func testStopLeavesFullScreenBeforeClosing() async throws {
        try await requireFullScreen()
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        var history: [PresentationWindow.FullScreenState] = []
        var closedAfter: [PresentationWindow.FullScreenState] = []
        audience.onFullScreenChange = { history.append($0) }
        audience.onClosed = { closedAfter = history }
        presentation.stop()
        try await waitUntil(timeout: 10, "the audience window closed") { audience.isClosed }
        XCTAssertEqual(closedAfter.suffix(2), [.exiting, .windowed], "it left full screen, then closed")
        try await waitUntil(timeout: 30, "the talk to end") { presentation.state == .idle }
    }

    func testStopWhileTheExchangeIsInFlightOpensNoWindow() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        var gate: CheckedContinuation<Void, Never>?
        // The exchange waits for the test to let it through, then runs for real.
        // The gate is set inside the continuation's closure, so a resume can never miss it.
        presentation.authorizePresenter = { client in
            await withCheckedContinuation { continuation in gate = continuation }
            return try await client.authorizePresenter()
        }
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "the exchange to begin") { gate != nil }
        let pid = try XCTUnwrap(presentation.session?.processIdentifier)
        presentation.stop()
        XCTAssertEqual(presentation.state, .stopping)
        gate?.resume()
        try await waitUntil(timeout: 20, "the talk to be idle") { presentation.state == .idle }
        try await Task.sleep(nanoseconds: 3_500_000_000)
        XCTAssertNil(presentation.audienceWindow, "no window opens after the exchange returns, nor after the fallback")
        XCTAssertNil(presentation.presenterWindow)
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertEqual(presentation.state, .idle)
        XCTAssertFalse(isRunning(pid))
        let cookie = await presenterCookieInTheTalkStore()
        XCTAssertNil(cookie, "a late exchange installs nothing for a talk that has ended")
    }

    func testTheTalkKeepsItsPortAcrossTalks() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let presentation = controller.presentation
        XCTAssertNil(AppEnvironment.shared.deckPorts.port(for: deck), "nothing remembered before the first talk")
        let suggested = DeckPortStore.suggestedPort(for: deck)
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let first = try XCTUnwrap(presentation.session)
        let port = try XCTUnwrap(presentation.client).ready.port
        if first.command.port == suggested {
            XCTAssertEqual(port, suggested, "the first talk asks for the deck's own port, outside the ephemeral range")
        } else {
            XCTAssertNil(first.command.port, "the suggested port was taken on this machine, so the fallback ran")
            XCTAssertTrue(first.log.text.contains("port \(suggested) was taken"))
        }
        XCTAssertEqual(AppEnvironment.shared.deckPorts.port(for: deck), port, "the port tap runs on is remembered for the deck")
        try await stopPresenting(controller)

        // The next talk asks for the same port, so the presenter page keeps its origin, and with it its layout and notes size.
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let second = try XCTUnwrap(presentation.session)
        XCTAssertEqual(second.command.port, port)
        XCTAssertTrue(second.log.text.contains("--port \(port)"))
        XCTAssertEqual(presentation.client?.ready.port, port)
        XCTAssertEqual(presentation.presenterWindow?.page.lastLoadedURL?.port, port)
    }

    func testATakenPortFallsBackToANewOne() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let presentation = controller.presentation
        // The deck's own tap dev holds a port; remember that one for the talk.
        let taken = try await waitForRunningTap(document).port
        AppEnvironment.shared.deckPorts.setPort(taken, for: deck)
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1), timeout: 60)
        let talk = try XCTUnwrap(presentation.session)
        XCTAssertNil(talk.command.port, "the second attempt asks for no port")
        let port = try XCTUnwrap(presentation.client).ready.port
        XCTAssertNotEqual(port, taken)
        XCTAssertEqual(AppEnvironment.shared.deckPorts.port(for: deck), port, "the new port replaces the taken one")
        XCTAssertTrue(talk.log.text.contains("port \(taken) was taken"), "the talk's log says why the layout starts fresh")
        XCTAssertFalse(talk.log.text.contains("restart"), "D2's policy never restarted the failed attempt on the same port")
        XCTAssertTrue(presentation.sleepAssertion.isHeld)
    }

    func testATakenPortOnARestartFallsBackToo() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let port = try XCTUnwrap(presentation.client).ready.port
        let firstPid = try XCTUnwrap(presentation.session?.processIdentifier)
        // Something else grabs the deck's port the moment tap present dies: a listener of the test's own.
        kill(firstPid, SIGKILL)
        try await waitUntil(timeout: 10, "the process gone") { !self.isRunning(firstPid) }
        let squatter = try TestListener(port: port)
        defer { squatter.close() }
        try await waitUntil(timeout: 40, "tap present back on another port") {
            if let ready = presentation.client?.ready, ready.port != port, case .running = presentation.session?.state { return true }
            return false
        }
        XCTAssertEqual(presentation.state, .presenting, "the talk never ended")
        XCTAssertNil(presentation.session?.command.port, "the fallback ran mid-talk")
        XCTAssertEqual(AppEnvironment.shared.deckPorts.port(for: deck), presentation.client?.ready.port)
        XCTAssertTrue(presentation.session?.log.text.contains("port \(port) was taken") == true)
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

/// A TCP listener on one port, so a test can make a port taken.
final class TestListener {
    private let socket: Int32

    init(port: Int) throws {
        socket = Darwin.socket(AF_INET, SOCK_STREAM, 0)
        var yes: Int32 = 1
        setsockopt(socket, SOL_SOCKET, SO_REUSEADDR, &yes, socklen_t(MemoryLayout<Int32>.size))
        var address = sockaddr_in()
        address.sin_len = UInt8(MemoryLayout<sockaddr_in>.size)
        address.sin_family = sa_family_t(AF_INET)
        address.sin_port = in_port_t(port).bigEndian
        address.sin_addr.s_addr = INADDR_ANY
        let bound = withUnsafePointer(to: &address) { pointer in
            pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) { bind(socket, $0, socklen_t(MemoryLayout<sockaddr_in>.size)) }
        }
        guard bound == 0, listen(socket, 1) == 0 else {
            Darwin.close(socket)
            throw CocoaError(.fileWriteUnknown)
        }
    }

    func close() {
        Darwin.close(socket)
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
/// On two displays each window has its own full screen Space. On one
/// display the audience window has the Space and the presenter window is
/// its child, shown over it by Option-Tab and the S key and hidden again
/// by Option-Tab, with no Space switch.
///
/// The state moves idle, starting (the save, the process, the ready line,
/// the windows loading), presenting (the windows are up), stopping (the
/// windows are down and tap is quitting, which may take a keep-recording
/// answer), and back to idle; or to failed, with a message for the
/// person, when tap present cannot start or stops restarting. A talk is
/// counted in `AppEnvironment.presentingCount` from start to idle or
/// failed, and a stopping talk whose deck has closed is kept alive by
/// `AppEnvironment.endingTalks` until its process has exited.
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
    /// The window the speaker's keys go to: the one whose Space is active
    /// on two displays, the one on top on one display.
    private(set) weak var frontWindow: PresentationWindow?
    /// The displays this talk runs on; nil between talks, so the popover
    /// always reads the displays that are connected now.
    private(set) var arrangement: DisplayArrangement?
    let sleepAssertion = SleepAssertion()
    /// The slide the audience is on, 1-based: the start slide until tap's
    /// first slide event, then the last event's slide.
    private(set) var lastSlide = 1
    /// The log of the last talk, kept after its session is gone, so Window
    /// > Tap Log still shows it and the deck window can append to it.
    private(set) var lastTalkLog: TapLog?
    private(set) var recording = RecordingStatus()
    /// True once the windows have been asked to show for this talk.
    private(set) var windowsShown = false
    /// True once this talk's windows were shown at all, so an ending moves
    /// the editor's cursor only for a talk the person actually saw.
    private var windowsWereShown = false
    /// tap's questions in the order they came; the first is the one on
    /// screen. tap asks one at a time at startup, but a later question can
    /// arrive on top of a sheet that is still up (D5's approval).
    private(set) var pendingQuestions: [PendingQuestion] = []
    var pendingQuestion: PendingQuestion? { pendingQuestions.first }
    /// Set once a page reported ready or failed to load, or the fallback
    /// fired: the windows may be shown as soon as no question is pending.
    private var pagesReported = false
    private var showWindowsFallback: DispatchWorkItem?
    /// Windows taking themselves down, one at a time, the front one first:
    /// two exits at once fail the same way two entries do. Each is kept
    /// until it has closed.
    private(set) var windowsGoingDown: [PresentationWindow] = []
    private var takingDown = false
    /// The windows still to be put on their displays, one at a time.
    private var placements: [(window: PresentationWindow, frame: CGRect, fullScreen: Bool)] = []
    private var placing = false
    private var placementCompletion: (() -> Void)?
    /// The remembered port tap said is taken; the next attempt asks for none.
    private var takenPort: Int?
    private var portFallbackPending = false
    /// Counts starts, so a save that completes for a start that was
    /// stopped in the meantime launches nothing.
    private var startGeneration = 0
    /// True while this talk is counted in `AppEnvironment.presentingCount`.
    /// Read in deinit, which is not on the main actor, as a last guard.
    nonisolated(unsafe) private var countedAsPresenting = false

    var onStateChange: ((State) -> Void)?
    var onEvent: ((TapEvent) -> Void)?
    /// The talk ended, by Stop or a failure, after its windows had shown;
    /// this is the last slide the audience saw.
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
    var deckPorts: DeckPortStore
    /// The displays. Production reads NSScreen; tests hand in frames of their own.
    var screens: () -> [ScreenInfo] = { NSScreen.screens.map(ScreenInfo.init(screen:)) }
    /// Whether each display has its own Spaces (System Settings > Desktop &
    /// Dock). Without it, one full screen Space blacks out every other
    /// display, so a two-display talk cannot use full screen at all.
    var screensHaveSeparateSpaces: () -> Bool = { NSScreen.screensHaveSeparateSpaces }
    /// Whether this host may use system full screen at all. Production
    /// always may; a hosted test on a host the probe found unable says no,
    /// and the talk runs as plain windows over the displays.
    var fullScreenAllowed: () -> Bool = { true }
    /// Trades the presenter secret for the hub's cookie. A test replaces it
    /// to hold the exchange open while it presses Stop.
    var authorizePresenter: (TapClient) async throws -> String = { try await $0.authorizePresenter() }
    /// The deck's window controller, which the talk's windows forward menu actions to.
    weak var deckWindowController: DeckWindowController?
    /// How long the windows wait for the first page to report before they
    /// are shown anyway: a page that never reports (no server, a load
    /// failure) must not keep the talk from starting.
    static let showWindowsFallbackInterval: TimeInterval = 3
    /// How long quit waits for a tap that got ready (its own quit deadline
    /// is 8 s), and for one that never did.
    static let quitTimeout: TimeInterval = 15
    static let quitTimeoutBeforeReady: TimeInterval = 2
    /// Once tap has asked keep-recording it waits for the answer while
    /// stdin is open, up to 60 s. The deadline must outlast that wait, or
    /// closing stdin would answer the person's question for them.
    static let quitTimeoutWithRecording: TimeInterval = 75

    init(deckURL: @escaping () -> URL?,
         saveDeck: @escaping (@escaping (Error?) -> Void) -> Void,
         sessionConfiguration: @escaping () -> TapSession.Configuration,
         displayAssignments: DisplayAssignmentStore,
         deckPorts: DeckPortStore) {
        self.deckURL = deckURL
        self.saveDeck = saveDeck
        self.sessionConfiguration = sessionConfiguration
        self.displayAssignments = displayAssignments
        self.deckPorts = deckPorts
    }

    deinit {
        // A talk that is freed while counted (which retainEndingTalk is
        // there to prevent) must not keep Play off in every deck forever.
        if countedAsPresenting {
            countedAsPresenting = false
            MainActor.assumeIsolated { AppEnvironment.shared.noteTalkEnded() }
        }
    }

    /// True from Play until the talk is idle or failed again.
    var isActive: Bool {
        switch state {
        case .starting, .presenting, .stopping: return true
        case .idle, .failed: return false
        }
    }

    /// Play and Rehearse need a deck file and no talk in progress, in this
    /// deck or any other.
    var canStart: Bool {
        deckURL() != nil && !isActive && !AppEnvironment.shared.isPresenting
    }

    /// The arrangement the next talk would use, for the popover; the
    /// running talk's while one runs.
    var currentArrangement: DisplayArrangement? {
        arrangement ?? DisplayArrangement.resolve(screens: screens(), store: displayAssignments)
    }

    /// Whether the audience window (and on two displays the presenter
    /// window) goes to full screen: when the host allows it, and on two
    /// displays only when each display has its own Spaces. Reads the
    /// displays connected now, not a running talk's.
    var usesFullScreen: Bool {
        guard fullScreenAllowed() else { return false }
        let connected = DisplayArrangement.resolve(screens: screens(), store: displayAssignments)
        return (connected?.isSingleDisplay ?? true) || screensHaveSeparateSpaces()
    }

    /// True once every window is where it was asked to be.
    var windowsAreSettled: Bool { placements.isEmpty && !placing }

    /// On one display: the presenter view is over the audience view.
    var presenterIsShownOverAudience: Bool {
        guard let presenterWindow else { return false }
        return presenterWindow.isAttached && presenterWindow.isVisible
    }

    // MARK: Start

    func start(_ options: PresentationOptions) {
        guard canStart, let deck = deckURL() else { return }
        self.options = options
        lastSlide = options.startSlide
        recording = RecordingStatus()
        pendingQuestions = []
        pagesReported = false
        windowsShown = false
        windowsWereShown = false
        takenPort = nil
        portFallbackPending = false
        startGeneration += 1
        let generation = startGeneration
        state = .starting
        countIn()
        saveDeck { [weak self] error in
            guard let self, self.state == .starting, self.startGeneration == generation else { return }
            if let error {
                self.fail(Self.saveFailureMessage(for: error))
                return
            }
            self.launch(deck: deck, options: options, port: self.deckPorts.port(for: deck) ?? DeckPortStore.suggestedPort(for: deck))
        }
    }

    /// What a refused save means to the person. The document answers
    /// userCancelled while a disk conflict is showing, which says nothing
    /// on its own.
    static func saveFailureMessage(for error: Error) -> String {
        if (error as? CocoaError)?.code == .userCancelled {
            return "The deck could not be saved: resolve the change on disk first."
        }
        return "The deck could not be saved: \(error.localizedDescription)"
    }

    private func launch(deck: URL, options: PresentationOptions, port: Int?) {
        let session = TapSession(deckURL: deck, configuration: sessionConfiguration(), command: options.command(port: port))
        // A replaced session (the port fallback) may still report; only the current one is heard.
        session.onStateChange = { [weak self, weak session] sessionState in
            guard let self, let session, session === self.session else { return }
            self.sessionStateChanged(sessionState)
        }
        session.onEvent = { [weak self, weak session] event in
            guard let self, let session, session === self.session else { return }
            self.handle(event)
        }
        self.session = session
        lastTalkLog = session.log
        session.start()
    }

    private func sessionStateChanged(_ sessionState: TapSession.State) {
        switch sessionState {
        case .running(let ready):
            tapIsReady(ready)
        case .failed(let lastOutput):
            endBecauseTapFailed(lastOutput: lastOutput)
        case .stopped:
            if state == .stopping {
                finishStopping()
            } else if portFallbackPending {
                relaunchOnAFreePort()
            }
        case .restarting:
            // The port attempt exited before its error line was read: stop
            // the restart, which lands in .stopped and relaunches.
            if portFallbackPending { session?.stop() }
        case .starting:
            break
        }
    }

    /// tap said the remembered port is taken: its error event, code
    /// "failed", "port n is already in use (another tap dev may be
    /// running); pass --port <other>" (present runs the dev server, which
    /// names itself). At the start, or on a restart mid-talk when
    /// something grabbed the port in between, the session is stopped
    /// before D2's policy can restart it on the same port, and a new one
    /// starts with no port. That talk's presenter layout starts fresh,
    /// since the page's origin changed; the next talk remembers the new port.
    private func portIsTaken() {
        guard state == .starting || state == .presenting, let session, let port = session.command.port, !portFallbackPending else { return }
        portFallbackPending = true
        takenPort = port
        session.stop()
    }

    private func relaunchOnAFreePort() {
        portFallbackPending = false
        guard state == .starting || state == .presenting, let options, let deck = deckURL() else { return }
        launch(deck: deck, options: options, port: nil)
        if let takenPort {
            session?.log.append("port \(takenPort) was taken; this talk runs on a new port, and the presenter layout starts fresh", source: .app)
        }
    }

    /// tap present printed its ready line, at the start or after a restart.
    /// The port is remembered for the deck. The presenter secret is traded
    /// for the hub's cookie first, so the audience page's own WebSocket
    /// connection is relayed: the speaker's keys in the audience window
    /// move the presenter view, and tap hears every position for its
    /// slide events and chapters. The presenter page brings its own key.
    /// A late exchange for a talk that has ended installs nothing.
    private func tapIsReady(_ ready: TapReady) {
        guard state == .starting || state == .presenting, let session else { return }
        deckPorts.setPort(ready.port, for: session.deckURL)
        let client = TapClient(ready: ready)
        self.client = client
        Task { @MainActor [weak self] in
            var cookie: String?
            do {
                cookie = try await self?.authorizePresenter(client)
            } catch {
                self?.session?.log.append("tap refused the presenter secret: \(error)", source: .app)
            }
            guard let self, self.client === client, self.state == .starting || self.state == .presenting else { return }
            if let cookie { await Self.installPresenterCookie(cookie, into: AppEnvironment.shared.presentationDataStore) }
            guard self.client === client, self.state == .starting || self.state == .presenting else { return }
            self.openWindows(client: client)
        }
    }

    /// Puts the hub's presenter cookie into the talk pages' data store.
    /// Cookies ignore ports, so the one value for 127.0.0.1 is the present
    /// process's while a talk runs; the preview is driven by the app's own
    /// socket, which carries its cookie in a header, and never needs this one.
    static func installPresenterCookie(_ value: String, into store: WKWebsiteDataStore) async {
        guard let cookie = HTTPCookie(properties: [.name: TapClient.presenterCookieName, .value: value,
                                                    .domain: "127.0.0.1", .path: "/"]) else { return }
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            store.httpCookieStore.setCookie(cookie) { continuation.resume() }
        }
    }

    /// Creates the windows off screen, or reuses them after a restart, and
    /// loads the pages at `lastSlide`. The assertion is held from here:
    /// tap is up and the windows exist.
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
            audience.page.load(client.audienceLaunchURL(slide: lastSlide), allowedPort: client.ready.port)
        }
        let presenter = presenterWindow ?? makeWindow(role: .presenter, frame: arrangement.presenter.frame)
        presenterWindow = presenter
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
        guard state == .starting, !windowsShown, pagesReported, pendingQuestions.isEmpty else { return }
        showWindows()
    }

    /// Puts the talk's windows up. This is the one place presenting takes
    /// the screen: the person clicked Play, and taking the projector is
    /// what the click means. On two displays each window enters its own
    /// Space, one at a time (AppKit runs one transition at a time), the
    /// presenter last so its Space is active and it is key, since that is
    /// where the speaker's keys go. On one display only the audience
    /// window goes up (to full screen); the presenter window waits as its
    /// child-to-be, shown by Option-Tab or the S key.
    private func showWindows() {
        showWindowsFallback?.cancel()
        showWindowsFallback = nil
        guard let arrangement, let presenterWindow else { return }
        windowsShown = true
        windowsWereShown = true
        state = .presenting
        let fullScreen = usesFullScreen
        if !fullScreen, !fullScreenAllowed() {
            session?.log.append("this host does not allow system full screen; the talk windows are plain windows over their displays", source: .app)
        } else if !fullScreen {
            session?.log.append("\"Displays have separate Spaces\" is off in System Settings > Desktop & Dock, so the talk windows are plain windows over their displays rather than full screen Spaces", source: .app)
        }
        var order: [(window: PresentationWindow, frame: CGRect, fullScreen: Bool)] = []
        let front: PresentationWindow
        if let audienceWindow, arrangement.isSingleDisplay {
            order = [(audienceWindow, arrangement.audience.frame, fullScreen)]
            front = audienceWindow
        } else {
            if let audienceWindow { order.append((audienceWindow, arrangement.audience.frame, fullScreen)) }
            order.append((presenterWindow, arrangement.presenter.frame, fullScreen))
            front = presenterWindow
        }
        frontWindow = front
        place(order) { [weak self, weak front] in
            guard let self, let front, front === self.frontWindow, self.windowsShown else { return }
            front.makeKeyAndOrderFront(nil)
        }
    }

    /// Puts each window on its display in turn, then calls `completion`.
    /// A second window asked to enter full screen while another is still
    /// animating fails to, so the queue waits for each to settle.
    private func place(_ windows: [(window: PresentationWindow, frame: CGRect, fullScreen: Bool)], completion: @escaping () -> Void = {}) {
        placements += windows
        placementCompletion = completion
        placeNext()
    }

    private func placeNext() {
        guard !placing else { return }
        guard let next = placements.first else {
            let completion = placementCompletion
            placementCompletion = nil
            completion?()
            return
        }
        placements.removeFirst()
        placing = true
        next.window.present(on: next.frame, fullScreen: next.fullScreen) { [weak self] in
            guard let self else { return }
            self.placing = false
            if next.fullScreen, next.window.fullScreenState != .fullScreen, !next.window.isClosed {
                let name = next.window.role == .audience ? "audience" : "presenter"
                self.session?.log.append("the \(name) window could not enter full screen and stays a plain window over its display", source: .app)
            }
            self.placeNext()
        }
    }

    /// Option-Tab on one display: the presenter view comes over the
    /// audience view as its child, in the same Space, or goes away again.
    /// No Space switch, no animation.
    func toggleFrontWindow() {
        guard let audienceWindow, let presenterWindow, windowsShown, arrangement?.isSingleDisplay == true else { return }
        if presenterIsShownOverAudience {
            presenterWindow.detach()
            frontWindow = audienceWindow
            audienceWindow.makeKey()
        } else {
            showPresenterOverAudience()
        }
    }

    /// The S key in the audience page: the presenter view comes forward.
    /// On one display it comes over the audience view; on two its Space
    /// becomes the active one, which is what making it key does.
    func bringPresenterWindowForward() {
        guard let presenterWindow, windowsShown else { return }
        if arrangement?.isSingleDisplay == true, audienceWindow != nil {
            showPresenterOverAudience()
        } else {
            presenterWindow.makeKeyAndOrderFront(nil)
            frontWindow = presenterWindow
        }
    }

    private func showPresenterOverAudience() {
        guard let audienceWindow, let presenterWindow, !presenterIsShownOverAudience else { return }
        presenterWindow.attach(to: audienceWindow)
        presenterWindow.makeKey()
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
        case .error(let payload) where payload.code == "failed" && payload.message.hasPrefix("port ") && payload.message.contains("already in use"):
            portIsTaken()
        case .question(let id, let kind, let payload):
            let question = PendingQuestion(id: id, kind: kind, payload: payload)
            pendingQuestions.append(question)
            if kind == "keep-recording", state == .stopping {
                session?.extendQuit(timeout: Self.quitTimeoutWithRecording)
            }
            if pendingQuestions.count == 1 { onQuestion?(question) }
        default:
            break
        }
        onEvent?(event)
    }

    /// Answers the question with `id`, puts up the next queued one, and
    /// lets the windows show if they were waiting on it.
    func answer(id: String, value: Bool) {
        guard let index = pendingQuestions.firstIndex(where: { $0.id == id }) else { return }
        pendingQuestions.remove(at: index)
        session?.send(.answer(id: id, value: value))
        if index == 0, let next = pendingQuestions.first { onQuestion?(next) }
        showWindowsIfReady()
    }

    // MARK: Stop

    /// Ends the talk: the windows leave full screen and close and the
    /// assertion is released at once, then tap is asked to quit, which may
    /// bring a keep-recording question before it exits.
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
    /// the deck window closing and the app quitting. The windows go down
    /// one at a time, the front one first (an attached presenter window
    /// closes at once as its parent's child); each leaves full screen and
    /// closes on its own clock. The assertion goes now.
    private func takeDownWindows() {
        showWindowsFallback?.cancel()
        showWindowsFallback = nil
        placements = []
        placing = false
        placementCompletion = nil
        var order: [PresentationWindow] = []
        if let frontWindow, !frontWindow.isAttached { order.append(frontWindow) }
        for window in [audienceWindow, presenterWindow].compactMap({ $0 }) where !order.contains(where: { $0 === window }) && !window.isAttached {
            order.append(window)
        }
        windowsGoingDown += order
        audienceWindow = nil
        presenterWindow = nil
        frontWindow = nil
        windowsShown = false
        sleepAssertion.release()
        takeDownNext()
    }

    private func takeDownNext() {
        guard !takingDown, let window = windowsGoingDown.first(where: { !$0.isClosed }) else {
            windowsGoingDown.removeAll { $0.isClosed }
            return
        }
        takingDown = true
        window.takeDown { [weak self] in
            guard let self else { return }
            self.takingDown = false
            self.windowsGoingDown.removeAll { $0.isClosed }
            self.takeDownNext()
        }
    }

    private func finishStopping() {
        session = nil
        client = nil
        pendingQuestions = []
        arrangement = nil
        state = .idle
        countOut()
        AppEnvironment.shared.releaseEndingTalk(self)
        if windowsWereShown { onStopped?(lastSlide) }
    }

    /// tap present exited three times in thirty seconds, or never got ready.
    private func endBecauseTapFailed(lastOutput: [String]) {
        let summary = session?.restartPolicy.exitSummary ?? "tap present exited"
        let detail = lastOutput.last.map { "\(summary). Last output: \($0)" } ?? summary
        let showed = windowsWereShown
        fail(detail)
        if showed { onStopped?(lastSlide) }
    }

    private func fail(_ message: String) {
        takeDownWindows()
        session?.stop()
        session = nil
        client = nil
        pendingQuestions = []
        arrangement = nil
        state = .failed(message)
        countOut()
        AppEnvironment.shared.releaseEndingTalk(self)
        onFailed?(message)
    }

    private func countIn() {
        guard !countedAsPresenting else { return }
        countedAsPresenting = true
        AppEnvironment.shared.noteTalkStarted()
    }

    private func countOut() {
        guard countedAsPresenting else { return }
        countedAsPresenting = false
        AppEnvironment.shared.noteTalkEnded()
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

The attached presenter window is not queued: `PresentationWindow.takeDown` on its parent detaches and closes it first, so the child is gone before the parent's exit begins.

- [ ] **Step 5: Wire the controller into the deck and the environment**

In `DeckSessionController.swift`, add after `var exchangePresenterSecret`:

```swift
    /// The deck's talk, created on first use. `stop()` ends it with the
    /// deck, and hands it to the environment if it is still stopping.
    private var createdPresentation: PresentationController?
    var presentation: PresentationController {
        if let createdPresentation { return createdPresentation }
        let controller = PresentationController(
            deckURL: { [weak self] in self?.document?.fileURL },
            saveDeck: { [weak self] completion in
                guard let self else { return completion(nil) }
                self.saveForPresenting(completion: completion)
            },
            sessionConfiguration: { AppEnvironment.shared.presentSessionConfiguration() },
            displayAssignments: AppEnvironment.shared.displayAssignments,
            deckPorts: AppEnvironment.shared.deckPorts)
        controller.onStopped = { [weak self] lastSlide in self?.jumpToSlide(number: lastSlide) }
        createdPresentation = controller
        return controller
    }
    /// The talk, if this deck ever started one; nil costs nothing to check.
    var presentationIfCreated: PresentationController? { createdPresentation }

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

In `stop()`, add right after `stopped = true`:

```swift
        if let presentation = createdPresentation {
            presentation.stop()
            if presentation.isActive {
                // The deck is going, but the talk's process is not gone yet:
                // the environment keeps the talk until it is, so the quit
                // deadline, the escalation and the talk count all still run.
                // Nobody is left to answer a question, so tap's own defaults
                // answer it: a recording is kept, anything else declined.
                presentation.onQuestion = { [weak presentation] question in
                    let keep = question.kind == "keep-recording"
                    presentation?.session?.log.append("the deck window has closed; the \(question.kind) question is answered \(keep ? "keep" : "no") for it", source: .app)
                    presentation?.answer(id: question.id, value: keep)
                }
                AppEnvironment.shared.retainEndingTalk(presentation)
            }
        }
```

In `DeckWindowController.init`, after `shouldCascadeWindows = true`, add:

```swift
        sessionController.presentation.deckWindowController = self
```

In `AppEnvironment.swift`, add after `presentationDataStore` (Task 3):

```swift
    /// Which display is the audience for each pair of displays, across
    /// decks. A test replaces this with a store on a fresh UserDefaults suite.
    var displayAssignments = DisplayAssignmentStore()
    /// The port each deck's talks run on, so the talk pages keep one origin.
    var deckPorts = DeckPortStore()
    /// The Present popover's last settings, which Cmd+Option+P starts with.
    var presentationSettings = PresentationSettingsStore()
    /// A tap for talks alone, for tests that script tap present while the
    /// deck's real tap dev keeps running. nil runs the bundled tap.
    var presentExecutableURL: URL?
    /// How many talks are running across every deck, from Play to idle or
    /// failed. Play is off while one runs, and D7's updater reads
    /// `updatesMayInterrupt` before any prompt or restart.
    private(set) var presentingCount = 0
    static let presentingDidChangeNotification = Notification.Name("TapPresentingDidChange")
    /// Talks whose deck closed while they were still stopping, kept alive
    /// until their process has exited or the talk has failed.
    private(set) var endingTalks: [PresentationController] = []

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

    func retainEndingTalk(_ talk: PresentationController) {
        guard !endingTalks.contains(where: { $0 === talk }) else { return }
        endingTalks.append(talk)
    }

    func releaseEndingTalk(_ talk: PresentationController) {
        endingTalks.removeAll { $0 === talk }
    }
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
    /// without an answer, so quitting the app never loses one. The process
    /// ends before the windows' exits complete; the Spaces go with it.
    static func stopAllPresentations() {
        for document in NSDocumentController.shared.documents {
            (document as? DeckDocument)?.sessionController?.presentationIfCreated?.stop()
        }
    }
```

In `TapLogWindowController.reload()`, replace the `logs = ...` line with:

```swift
        logs = NSDocumentController.shared.documents.flatMap { document -> [TapLog] in
            guard let controller = (document as? DeckDocument)?.sessionController else { return [] }
            let talk = controller.presentationIfCreated
            return [controller.session.log] + ((talk?.session?.log ?? (talk?.isActive == true ? talk?.lastTalkLog : nil)).map { [$0] } ?? [])
        }
```

(A finished talk's log is listed again by Task 13, which keeps a failed talk's log readable.)

- [ ] **Step 6: Run the tests**

Run: `make -C desktop project`, then the required runs:

```bash
make -C desktop test ONLY=TapTests/PresentingTests/testTheMacStaysAwake
make -C desktop test ONLY=TapTests/PresentingTests
```

The second run is the whole class in one process, the one exception to running hosted tests one at a time: a talk count that leaks from one test to the next (a stopping talk freed before its process exits) shows up only when a second test starts a talk in the same process, and CI would otherwise be the first to see it. Record the class run's wall time in the ledger; the CI job's timeout (Task 14) is sized from it. Expected: every test in the class passes, or skips with the probe's reason where it asserts full screen (`testTheAudienceWindowIsInItsOwnFullScreenSpace`, `testStopLeavesFullScreenBeforeClosing`) on a host the spike found unable. Optional: `make -C desktop test ONLY=TapTests/RestartTests` and `ONLY=TapTests/TapLogTests`, to confirm the session and log changes broke nothing.

If `testStopPresenting` never sees `lastSlide == 4`: the hub relays only from a connection that carried the presenter cookie, so check `client.presenterCookie` is non-nil and that `TapClient.socketRequest()` still sets the `Cookie` header (D2). If the audience page never reports ready in `testOneDisplayIsOneSpaceWithThePresenterViewOverIt`, read `HostedTestCase.pageStateScript` through `audience.page.webView` the way `waitForPreview` does before changing anything. If `testATakenPortFallsBackToANewOne` times out on `.presenting`, read the talk's log: tap's error line must say "port <n> is already in use"; if tap instead started on the port (the probe found it free), tap dev's port was released between the two starts, which does not happen while the deck is open.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted, the ones that can leave a window in full screen, the assertion held, or Play off everywhere first: in `takeDownWindows`, drop `sleepAssertion.release()` (expected: `testTheMacStaysAwake`, the close, quit and dying tests fail on `isHeld`); in `takeDownNext`, never call `takeDown` (expected: `testStopPresenting` fails on `audience.isClosed`); in `takeDownNext`, take every window down in the same turn (expected: `testStopLeavesFullScreenBeforeClosing` is unaffected on one display, where only one window is in full screen; Task 5's two-display exit test kills it); in `DeckSessionController.stop`, drop `retainEndingTalk` (expected: `testClosingTheDeckMidTalkStillEndsTheProcessAndFreesPlay` fails on `endingTalks.count`, and with the locals gone the talk is never counted out); in `DeckSessionController.stop`, keep the old `onQuestion` (expected: Task 10's `testAClosedDeckKeepsItsRecordingWithoutAsking` times out); in `finishStopping`, drop `countOut()` (expected: the close test times out on `isPresenting`, and the class run fails on every later test's `canStart`); in `DeckSessionController.stop`, drop `presentation.stop()` (expected: the close test fails on `state`); in `AppDelegate.applicationWillTerminate`, drop the call (expected: the quit test fails); in `endBecauseTapFailed`, skip `fail` (expected: the dying test times out on `.failed` with the assertion held); in `tapIsReady`'s task, drop the first guard (expected: `testStopWhileTheExchangeIsInFlightOpensNoWindow` fails on the cookie, then on `audienceWindow`); in `start`, drop the `startGeneration == generation` check (expected: `testStopThenPlayDuringTheSaveStartsOneTalk` fails on `session` after the old completion); in `start`, skip `saveDeck` and call `launch` directly (expected: `testSaveBeforePresenting` fails on the file); in `start`, move `countIn()` into `launch` (expected: `testSaveBeforePresenting` fails on `isPresenting`); in `handle`, drop the `.slide` case (expected: `testStopPresenting` times out on `lastSlide`); in `toggleFrontWindow`, make the presenter key without attaching (expected: `testOneDisplayIsOneSpaceWithThePresenterViewOverIt` fails on `isAttached` and `childWindows`); in `showWindows`, place the presenter on one display too (expected: the same test fails on `presenter.isVisible`); in `tapIsReady`, drop `deckPorts.setPort` (expected: `testTheTalkKeepsItsPortAcrossTalks` fails on the remembered port); in `start`, pass `deckPorts.port(for:)` alone (expected: it fails on the suggested port); in `handle`, drop the port-taken case (expected: `testATakenPortFallsBackToANewOne` times out, tap restarting on the same port); in `portIsTaken`, require `.starting` (expected: `testATakenPortOnARestartFallsBackToo` times out); in `tapIsReady`, skip `installPresenterCookie` (expected: the cookie assertion fails); in `presenterURL`, drop the key (expected: the presenter page's query assertion fails); in `showWindowsIfReady`, drop `pendingQuestions.isEmpty` (survives here; Task 9's consent test kills it).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): start and stop a talk with tap present on the deck's port, its windows in their Spaces and the sleep assertion"
```

---

### Task 5: Two displays: the arrangement, Swap Displays, the memory, Rehearse, an unplugged projector, and displays without separate Spaces

**Files:**
- Modify: `desktop/Tap/Presenting/PresentationController.swift` (`swapDisplays`, `screensChanged`, `moveWindows`, the screen observer)
- Test: `desktop/TapTests/PresentingDisplayTests.swift`

**Interfaces:**
- Consumes: Task 4's controller (`place`, `arrangement`, `frontWindow`, `windowsAreSettled`, `usesFullScreen`, `screensHaveSeparateSpaces`, `fullScreenAllowed`, `presenterIsShownOverAudience`), `PresentingTestCase.halfScreens()`, `oneScreen()`, `requireSecondSpace()`; Task 3's `PresentationWindow.present(on:fullScreen:completion:)`, `attach(to:)`, `detach()`, `targetFrame`, `settledFrame`, `fullScreenState`, `isAttached`; Task 2's `DisplayArrangement.swapped()`, `DisplayAssignmentStore`.
- Produces: `PresentationController.swapDisplays()`, `screensChanged()`, `onScreensChanged`; `NSApplication.didChangeScreenParametersNotification` observed while a talk's windows exist.

Two displays on one screen (`halfScreens()`) are two full screen Spaces of that screen, which the probe's (b) case says whether the host can do; the tests that assert two Spaces call `requireSecondSpace()` first. The presenter view's moves between one display (a child of the audience window) and two (a Space of its own) are what the projector test exercises.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/PresentingDisplayTests.swift`:

```swift
import XCTest
@testable import Tap

/// Two displays on one screen: the left half is the laptop, the right half
/// the projector. A full screen window fills the whole display it is on,
/// so on one screen both windows end up as two Spaces of that display;
/// what these tests check is the frame each window was asked for and the
/// full screen state the window server reports, which is what a real
/// projector would also show.
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
        XCTAssertEqual(presentation.client?.ready.port, AppEnvironment.shared.deckPorts.port(for: try XCTUnwrap(controller.document?.fileURL)))
        XCTAssertTrue(talk.log.text.contains("tap present --app"))
        XCTAssertNotEqual(talk.processIdentifier, controller.session.processIdentifier, "a second process")
        if case .running = controller.session.state {} else { XCTFail("the preview keeps running from tap dev") }

        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.targetFrame, screens[1].frame, "the audience goes to the projector")
        XCTAssertEqual(presenter.targetFrame, screens[0].frame, "the presenter view goes to the built-in display")
        XCTAssertFalse(presenter.isAttached, "on two displays the presenter view is a window of its own")
        XCTAssertTrue(presenter.isVisible)
        XCTAssertTrue(presentation.frontWindow === presenter, "the speaker's keys go to the presenter window")
        try await waitUntil(timeout: 20, "the audience page on slide 3") { audience.page.lastReady?.slide == 3 }
        try await waitUntil(timeout: 20, "the presenter page on slide 3") { presenter.page.lastReady?.slide == 3 }
    }

    func testTwoDisplaysAreTwoSpacesThatLeaveOneAtATime() async throws {
        try await requireSecondSpace()
        let (_, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        let presentation = controller.presentation
        presentation.screens = { screens }
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.fullScreenState, .fullScreen)
        XCTAssertEqual(presenter.fullScreenState, .fullScreen)
        XCTAssertEqual(fullScreenPresentationWindows().count, 2)
        try await waitUntil(timeout: 5, "the presenter's Space active, the audience's not") {
            let order = onScreenWindowNumbers()
            return order.contains(presenter.windowNumber) && !order.contains(audience.windowNumber)
        }

        // Stop: the front window leaves its Space and closes, then the other; never both at once.
        var events: [String] = []
        for window in [audience, presenter] {
            let name = window.role == .audience ? "audience" : "presenter"
            window.onFullScreenChange = { events.append("\(name) \($0)") }
            window.onClosed = { events.append("\(name) closed") }
        }
        presentation.stop()
        try await waitUntil(timeout: 20, "both windows closed") { audience.isClosed && presenter.isClosed }
        XCTAssertEqual(events, ["presenter exiting", "presenter windowed", "presenter closed", "audience exiting", "audience windowed", "audience closed"],
                       "each leaves full screen before it closes, and the second waits for the first")
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        try await waitUntil(timeout: 30, "the talk to end") { presentation.state == .idle }
    }

    func testSwapDisplays() async throws {
        try await requireSecondSpace()
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
        XCTAssertEqual(audience.targetFrame, screens[0].frame, "the presenter and audience displays swapped before the start")
        XCTAssertEqual(presenter.targetFrame, screens[1].frame)
        // During the talk: the toolbar's Swap Displays. Each window leaves its Space, moves and enters again.
        presentation.swapDisplays()
        XCTAssertEqual(audience.targetFrame, screens[1].frame)
        XCTAssertEqual(presenter.targetFrame, screens[0].frame)
        XCTAssertFalse(presentation.windowsAreSettled)
        try await waitUntil(timeout: 30, "the windows back in full screen on their new displays") {
            presentation.windowsAreSettled && audience.fullScreenState == .fullScreen && presenter.fullScreenState == .fullScreen
        }
        XCTAssertTrue(presentation.sleepAssertion.isHeld, "a swap is not an ending")
        XCTAssertEqual(fullScreenPresentationWindows().count, 2)
    }

    func testSwapDisplaysAfterATalkReadsTheDisplaysConnectedNow() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        let screens = halfScreens()
        presentation.screens = { screens }
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        try await stopPresenting(controller)
        XCTAssertNil(presentation.arrangement, "a finished talk's arrangement is not kept")
        // The projector is gone, another is plugged in: the popover shows what is connected now, and Swap swaps that.
        let other = [screens[0], ScreenInfo(name: "Epson", frame: screens[1].frame, isBuiltIn: false)]
        presentation.screens = { other }
        XCTAssertEqual(presentation.currentArrangement?.audience.name, "Epson")
        presentation.swapDisplays()
        XCTAssertEqual(presentation.currentArrangement?.audience.name, "Built-in Display")
        XCTAssertEqual(AppEnvironment.shared.displayAssignments.audienceName(for: other), "Built-in Display")
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertEqual(presentation.audienceWindow?.targetFrame, other[0].frame, "the next talk uses the swapped displays")
    }

    func testRememberTheDisplayAssignment() async throws {
        let (_, first) = try await openDeckForPresenting()
        let screens = halfScreens()
        first.presentation.screens = { screens }
        first.presentation.swapDisplays()
        try await startPresenting(first, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertEqual(first.presentation.audienceWindow?.targetFrame, screens[0].frame)
        try await stopPresenting(first)
        XCTAssertEqual(AppEnvironment.shared.displayAssignments.audienceName(for: screens), "Built-in Display")

        // Another deck, the same pair of displays, in either order.
        let (_, second) = try await openDeckForPresenting()
        second.presentation.screens = { screens.reversed() }
        try await startPresenting(second, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertEqual(second.presentation.audienceWindow?.targetFrame, screens[0].frame, "the same assignment, for every deck")
        XCTAssertEqual(second.presentation.presenterWindow?.targetFrame, screens[1].frame)
    }

    func testRehearse() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        let presentation = controller.presentation
        presentation.screens = { screens }
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 4))
        let talk = try XCTUnwrap(presentation.session)
        XCTAssertEqual(talk.command, .present(record: false, presenterPassword: nil, port: nil))
        XCTAssertTrue(talk.log.text.contains("tap present --app --no-record ops.md"))
        XCTAssertNil(presentation.audienceWindow, "only the presenter view")
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(presenter.targetFrame, screens[0].frame, "on the laptop's display")
        XCTAssertTrue(presenter.isVisible)
        XCTAssertFalse(presenter.isAttached, "nothing to ride on: it has the display")
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

        // The projector is unplugged: macOS has moved its Space to the remaining
        // display already. The presenter view leaves its own Space and comes
        // over the audience view as its child, shown, since the speaker is at the laptop.
        presentation.screens = { one }
        presentation.screensChanged()
        XCTAssertEqual(presentation.arrangement?.isSingleDisplay, true)
        XCTAssertEqual(audience.targetFrame, one[0].frame, "the audience is asked for the remaining screen")
        XCTAssertEqual(presenter.targetFrame, one[0].frame)
        try await waitUntil(timeout: 30, "the windows settled and the presenter over the audience") {
            presentation.windowsAreSettled && presentation.presenterIsShownOverAudience
        }
        XCTAssertTrue(presenter.isAttached)
        XCTAssertEqual(presenter.fullScreenState, .windowed, "a child has no Space of its own")
        XCTAssertTrue(presentation.frontWindow === presenter)
        try await waitUntil(timeout: 5, "both on screen, the presenter in front") {
            let order = onScreenWindowNumbers()
            guard let a = order.firstIndex(of: audience.windowNumber), let p = order.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
        XCTAssertTrue(presentation.sleepAssertion.isHeld)
        XCTAssertLessThanOrEqual(fullScreenPresentationWindows().count, 1, "at most the audience window is in full screen")
        presentation.toggleFrontWindow()
        XCTAssertFalse(presentation.presenterIsShownOverAudience, "Option-Tab works as on one display now (Task 7 wires the key)")

        // The projector is back: the presenter view gets its own display and Space again, the audience goes to the projector.
        presentation.screens = { two }
        presentation.screensChanged()
        XCTAssertEqual(audience.targetFrame, two[1].frame)
        XCTAssertEqual(presenter.targetFrame, two[0].frame)
        try await waitUntil(timeout: 30, "the windows back on their displays") {
            presentation.windowsAreSettled && !presenter.isAttached && presenter.isVisible
        }
        XCTAssertTrue(presentation.frontWindow === presenter)
        presentation.toggleFrontWindow()
        XCTAssertFalse(presenter.isAttached, "nothing to toggle with a display each")
    }

    func testTheScreenObserverReachesTheTalkOnlyWhileItsWindowsExist() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        var changes = 0
        presentation.onScreensChanged = { changes += 1 }
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertEqual(changes, 0, "no talk, no observer")
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertEqual(changes, 1, "AppKit's notification reaches the talk")
        try await stopPresenting(controller)
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertEqual(changes, 1, "the observer went with the windows")
    }

    func testWithoutSeparateSpacesTheTalkWindowsStayPlainWindows() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        let presentation = controller.presentation
        presentation.screens = { screens }
        // "Displays have separate Spaces" is off: one full screen Space would black out the other display.
        presentation.screensHaveSeparateSpaces = { false }
        XCTAssertFalse(presentation.usesFullScreen)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.fullScreenState, .windowed)
        XCTAssertEqual(presenter.fullScreenState, .windowed)
        XCTAssertTrue(audience.isVisible)
        XCTAssertTrue(presenter.isVisible)
        XCTAssertEqual(audience.targetFrame, screens[1].frame)
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        XCTAssertTrue(presentation.session?.log.text.contains("Displays have separate Spaces") == true, "the talk's log says why")
        try await stopPresenting(controller)
        // One display never needs the setting. usesFullScreen reads the displays connected now, not a talk's.
        presentation.screens = { oneScreen() }
        XCTAssertEqual(presentation.usesFullScreen, fullScreenAvailable, "true wherever the host allows full screen at all")
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/PresentingDisplayTests/testSwapDisplays`
Expected: the test target does not compile (`swapDisplays` is undefined).

- [ ] **Step 3: Swap, remember, follow the screens, and honour the Spaces setting**

In `PresentationController.swift`, add stored properties after `placementCompletion`:

```swift
    /// Installed while the talk's windows exist, removed with them, so a
    /// finished talk never hears about displays and nothing is read in deinit.
    private var screenObserver: NSObjectProtocol?
    /// The screens changed while the windows exist. A test counts the calls.
    var onScreensChanged: (() -> Void)?
```

In `openWindows`, add after `sleepAssertion.acquire()`:

```swift
        if screenObserver == nil {
            screenObserver = NotificationCenter.default.addObserver(forName: NSApplication.didChangeScreenParametersNotification, object: nil, queue: nil) { [weak self] _ in
                MainActor.assumeIsolated { self?.screensChanged() }
            }
        }
```

In `takeDownWindows`, add as its first lines:

```swift
        if let screenObserver { NotificationCenter.default.removeObserver(screenObserver) }
        screenObserver = nil
```

Add after `bringPresenterWindowForward()`:

```swift
    // MARK: Displays

    /// Exchanges the audience and presenter displays, before the talk (the
    /// popover's Swap Displays) or during it (the toolbar's), and remembers
    /// the choice for this pair of displays, across decks. During a talk
    /// each window leaves its Space, moves and enters the other display's.
    /// Nothing to swap on one display.
    func swapDisplays() {
        let screens = self.screens()
        guard let current = arrangement ?? DisplayArrangement.resolve(screens: screens, store: displayAssignments),
              !current.isSingleDisplay else { return }
        let swapped = current.swapped()
        displayAssignments.setAudienceName(swapped.audience.name, for: screens)
        guard isActive, windowsShown else { return }
        arrangement = swapped
        moveWindows(to: swapped)
    }

    /// The displays changed while a talk runs: a projector unplugged or
    /// plugged back in. macOS has already moved a vanished display's Space
    /// to a remaining one; the windows are asked for the new arrangement
    /// (a window already on its frame does nothing). With one display
    /// left, the presenter window leaves its Space and becomes the
    /// audience window's child, shown, since the speaker is at the laptop;
    /// with the projector back it detaches and gets its Space again.
    func screensChanged() {
        onScreensChanged?()
        guard isActive, windowsShown, let resolved = DisplayArrangement.resolve(screens: screens(), store: displayAssignments) else { return }
        arrangement = resolved
        moveWindows(to: resolved)
    }

    /// Puts the windows on `arrangement`'s displays. A rehearsal has only
    /// the presenter window, which goes where the arrangement puts it. A
    /// talk on one display places the audience window and shows the
    /// presenter window over it as its child; on two, each window gets
    /// its display, the presenter last so its Space is the active one.
    private func moveWindows(to arrangement: DisplayArrangement) {
        let fullScreen = usesFullScreen
        guard let presenterWindow else { return }
        guard let audienceWindow else {
            place([(presenterWindow, arrangement.presenter.frame, fullScreen)]) { [weak self, weak presenterWindow] in
                guard let self, let presenterWindow, self.windowsShown else { return }
                presenterWindow.makeKeyAndOrderFront(nil)
            }
            return
        }
        if arrangement.isSingleDisplay {
            // The presenter window leaves its own Space first (a child may not have one), then rides over the audience.
            presenterWindow.detach()
            place([(presenterWindow, arrangement.presenter.frame, false), (audienceWindow, arrangement.audience.frame, fullScreen)]) { [weak self] in
                guard let self, self.windowsShown, let presenterWindow = self.presenterWindow else { return }
                presenterWindow.orderOut(nil)
                self.frontWindow = self.audienceWindow
                self.showPresenterOverAudience()
            }
        } else {
            presenterWindow.detach()
            place([(audienceWindow, arrangement.audience.frame, fullScreen), (presenterWindow, arrangement.presenter.frame, fullScreen)]) { [weak self] in
                guard let self, self.windowsShown, let presenterWindow = self.presenterWindow else { return }
                self.frontWindow = presenterWindow
                presenterWindow.makeKeyAndOrderFront(nil)
            }
        }
    }
```

- [ ] **Step 4: Run the tests**

Required:

```bash
make -C desktop test ONLY=TapTests/PresentingDisplayTests/testTwoDisplaysAreTwoSpacesThatLeaveOneAtATime
make -C desktop test ONLY=TapTests/PresentingDisplayTests/testTheAudienceWindowFallsBackWhenTheProjectorGoes
make -C desktop test ONLY=TapTests/PresentingDisplayTests/testWithoutSeparateSpacesTheTalkWindowsStayPlainWindows
make -C desktop test ONLY=TapTests/PresentingDisplayTests/testTheScreenObserverReachesTheTalkOnlyWhileItsWindowsExist
```

Optional: `testStartPresentingWithTwoDisplays`, `testSwapDisplays`, `testSwapDisplaysAfterATalkReadsTheDisplaysConnectedNow`, `testRememberTheDisplayAssignment`, `testRehearse`. Expected: all pass, or `testTwoDisplaysAreTwoSpacesThatLeaveOneAtATime` and `testSwapDisplays` skip with the probe's reason on a host whose spike found no second Space. A swap on one screen takes both windows out of full screen and back in, about four seconds of animation; the projector test moves the presenter window out of its Space and back, about the same.

- [ ] **Step 5: Mutate and commit**

Mutations, each reverted, the ones that can leave a window off its display or in full screen first: in `takeDownNext`, take every window down in the same turn (expected: `testTwoDisplaysAreTwoSpacesThatLeaveOneAtATime` fails on `events`, the audience's exit starting before the presenter has closed); in `moveWindows`, drop the `place` call (expected: `testSwapDisplays` fails on `windowsAreSettled`, which never becomes false, and the fallback test on `targetFrame`); in `moveWindows`'s single-display completion, drop `showPresenterOverAudience()` (expected: the fallback test times out on `presenterIsShownOverAudience`); in `moveWindows`, skip `presenterWindow.detach()` before the two-display placement (expected: the fallback test fails on `!presenter.isAttached` after the projector returns, since a child cannot enter a Space); in `takeDownWindows`, drop the observer removal (expected: `testTheScreenObserverReachesTheTalkOnlyWhileItsWindowsExist` fails on the third count); in `swapDisplays`, drop `setAudienceName` (expected: `testRememberTheDisplayAssignment` fails on the second deck); in `finishStopping`, keep `arrangement` (expected: `testSwapDisplaysAfterATalkReadsTheDisplaysConnectedNow` fails on "Epson"); in `openWindows`, use `arrangement.presenter.frame` for the audience (expected: `testStartPresentingWithTwoDisplays` fails on `targetFrame`); in `usesFullScreen`, ignore `screensHaveSeparateSpaces` (expected: the separate Spaces test fails on `fullScreenState`); in `usesFullScreen`, read `currentArrangement` instead of the displays connected now (survives: after the stop `arrangement` is nil and both read the same; the property is for the popover between talks, and `testThePopoverSaysWhenDisplaysShareOneSpace` in Task 6 reads it that way); in `PresentationOptions.command(port:)`, hard-code `record: true` (expected: `testRehearse` fails on the command).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): arrange a talk over two displays, swap them, remember the pair, rehearse on one, and follow an unplugged projector"
```

---

### Task 6: The Play toolbar button and the Present popover

**Files:**
- Create: `desktop/Tap/Presenting/DisplayArrangementView.swift`
- Create: `desktop/Tap/Presenting/PresentPopoverController.swift`
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (the Play toolbar item, `play`, `playWithOptions`, `playButtonClicked(modifiers:)`, `rehearse`, `startPresenting(_:)`, `refreshPresentingControls`, `presentPopover`)
- Test: `desktop/TapTests/PresentPopoverTests.swift`

**Interfaces:**
- Consumes: Task 4's `PresentationController` (`currentArrangement`, `start`, `canStart`, `onStateChange`, `swapDisplays`), Task 5's `usesFullScreen`, Task 2's `PresentationOptions`, `PresentationSettings`, `PresentationSettingsStore`, `DisplayArrangement`; D3's `HoverButton` pattern and `LayoutGalleryController`'s popover pattern.
- Produces: `DisplayArrangementView` with `presenterBox`, `audienceBox` (each a `ScreenBox` with `roleLabel`, `nameLabel`), `update(arrangement:)`; `PresentPopoverController` with `Context(arrangement:cursorSlide:usesFullScreen:)`, `isShown`, `arrangementView`, `singleDisplayLabel`, `separateSpacesLabel`, `swapButton`, `startFromControl`, `recordCheckbox`, `phoneRemoteCheckbox`, `advancedButton`, `advancedStack`, `passwordField`, `tunnelCheckbox`, `rehearseButton`, `startButton`, `onSwap`, `onStart`, `onRehearse`, `show(context:relativeTo:of:)`, `update(context:)`, `close()`, `options(mode:)`, `settings`, `loadSettings(_:)`; `DeckWindowController.playItemIdentifier`, `playButton`, `presentPopover`, `play(_:)` (Cmd+Option+P: starts at once with the last settings), `playWithOptions(_:)` (the popover), `playButtonClicked(modifiers:)`, `rehearse(_:)`, `startPresenting(_:)`, `refreshPresentingControls()`, `popoverContext()`.

The person's decision (2026-09-24): Cmd+Option+P starts the talk at once with the last settings; clicking the Play button opens the popover; Shift-click starts from slide 1. The last settings are the popover's controls, loaded from `AppEnvironment.presentationSettings` when the popover is made and saved on every start, so they survive a relaunch; the presenter password stays in the field for one launch and is never saved.

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
        deckWindow.playButtonClicked(modifiers: [.shift])
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
        deckWindow.playButtonClicked(modifiers: [])
        let popover = deckWindow.presentPopover
        XCTAssertTrue(popover.isShown, "a click on Play opens the popover")
        XCTAssertTrue(popover.separateSpacesLabel.isHidden)
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
        XCTAssertEqual(AppEnvironment.shared.presentationSettings.settings,
                       PresentationSettings(startFromSlideOne: true, record: false, phoneRemote: false, tunnel: false),
                       "a start saves its settings for Cmd+Option+P and the next launch")
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        XCTAssertEqual(controller.presentation.session?.command, .present(record: false, presenterPassword: nil, port: nil))
        XCTAssertEqual(controller.presentation.audienceWindow?.targetFrame, screens[0].frame, "the swap held")
    }

    func testCmdOptionPStartsAtOnceWithTheLastSettings() async throws {
        AppEnvironment.shared.presentationSettings.settings = PresentationSettings(startFromSlideOne: false, record: false, phoneRemote: false, tunnel: true)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        controller.jumpToSlide(number: 4)
        deckWindow.presentPopover.passwordField.stringValue = "secret"
        // Present > Play, Cmd+Option+P: no popover, the last settings, the cursor's slide.
        deckWindow.play(nil)
        XCTAssertFalse(deckWindow.presentPopover.isShown)
        XCTAssertEqual(controller.presentation.options,
                       PresentationOptions(mode: .play, startSlide: 4, record: false, phoneRemote: false, tunnel: true, presenterPassword: "secret"))
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        XCTAssertEqual(controller.presentation.session?.command, .present(record: false, presenterPassword: "secret", port: nil))
        try await stopPresenting(controller)
        // Present > Play with Options opens the popover, with the same settings showing.
        deckWindow.playWithOptions(nil)
        XCTAssertTrue(deckWindow.presentPopover.isShown)
        XCTAssertEqual(deckWindow.presentPopover.recordCheckbox.state, .off)
        XCTAssertEqual(deckWindow.presentPopover.tunnelCheckbox.state, .on)
        deckWindow.presentPopover.close()

        // The settings are app-wide: a second deck's Cmd+Option+P starts with what the first deck last chose, never with its own stale controls.
        let (_, other) = try await openDeckForPresenting()
        let otherWindow = try windowController(other)
        otherWindow.playWithOptions(nil)
        otherWindow.presentPopover.recordCheckbox.state = .on
        otherWindow.presentPopover.close()
        deckWindow.presentPopover.recordCheckbox.state = .off
        deckWindow.presentPopover.tunnelCheckbox.state = .off
        deckWindow.play(nil)
        XCTAssertEqual(controller.presentation.options?.record, false)
        XCTAssertEqual(controller.presentation.options?.tunnel, false, "the first deck saved its own controls")
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
        other.jumpToSlide(number: 3)
        otherWindow.play(nil)
        XCTAssertEqual(other.presentation.options?.record, false, "the newest saved settings, not this popover's stale controls")
        XCTAssertEqual(other.presentation.options?.startSlide, 3, "this deck's cursor, read now")
        try await waitUntil(timeout: 40, "the other talk") { other.presentation.state == .presenting }
    }

    func testThePopoverWithOneDisplaySaysSo() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        deckWindow.playButtonClicked(modifiers: [])
        let popover = deckWindow.presentPopover
        XCTAssertTrue(popover.isShown)
        XCTAssertTrue(popover.arrangementView.isHidden)
        XCTAssertFalse(popover.singleDisplayLabel.isHidden)
        XCTAssertTrue(popover.separateSpacesLabel.isHidden, "one display never needs the setting")
        XCTAssertFalse(popover.swapButton.isEnabled)
        popover.rehearseButton.performClick(nil)
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(controller.presentation.options?.mode, .rehearse)
        XCTAssertEqual(controller.presentation.options?.startSlide, 1)
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
    }

    func testThePopoverSaysWhenDisplaysShareOneSpace() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let screens = halfScreens()
        controller.presentation.screens = { screens }
        controller.presentation.screensHaveSeparateSpaces = { false }
        deckWindow.playButtonClicked(modifiers: [])
        let popover = deckWindow.presentPopover
        XCTAssertFalse(popover.separateSpacesLabel.isHidden)
        XCTAssertTrue(popover.separateSpacesLabel.stringValue.contains("Displays have separate Spaces"))
        XCTAssertTrue(popover.startButton.isEnabled, "the talk still runs, as plain windows")
        popover.close()
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
        /// False when two displays share one Space, so the talk windows cannot be full screen.
        let usesFullScreen: Bool
    }

    /// Kept here rather than read from the popover, whose `isShown` can
    /// depend on whether the app is active (D3's gallery found the same).
    private(set) var isShown = false
    let arrangementView = DisplayArrangementView(frame: .zero)
    let singleDisplayLabel = NSTextField(wrappingLabelWithString: "One display: the audience fills the screen, and Option-Tab shows the presenter view.")
    let separateSpacesLabel = NSTextField(wrappingLabelWithString: "Turn on \"Displays have separate Spaces\" in System Settings > Desktop & Dock so each display gets its own full screen window. Until then the talk windows are plain windows over their displays.")
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
    private var context = Context(arrangement: nil, cursorSlide: 1, usesFullScreen: true)

    override init() {
        super.init()
        singleDisplayLabel.font = .systemFont(ofSize: 12)
        singleDisplayLabel.textColor = .secondaryLabelColor
        separateSpacesLabel.font = .systemFont(ofSize: 12)
        separateSpacesLabel.textColor = .systemOrange
        separateSpacesLabel.isHidden = true
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
        let stack = NSStackView(views: [arrangementView, singleDisplayLabel, separateSpacesLabel, swapButton, startRow, recordCheckbox, recordHint,
                                        phoneRemoteCheckbox, advancedRow, advancedStack, buttons])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.edgeInsets = NSEdgeInsets(top: 16, left: 16, bottom: 16, right: 16)
        stack.widthAnchor.constraint(equalToConstant: 360).isActive = true
        buttons.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -32).isActive = true
        singleDisplayLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -32).isActive = true
        separateSpacesLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -32).isActive = true
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
            separateSpacesLabel.isHidden = context.usesFullScreen
            swapButton.isEnabled = true
        } else {
            arrangementView.isHidden = true
            singleDisplayLabel.isHidden = false
            separateSpacesLabel.isHidden = true
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
        settings.options(mode: mode, cursorSlide: context.cursorSlide, presenterPassword: presenterPassword)
    }

    /// The controls as settings: what a start saves, and what Cmd+Option+P starts with.
    var settings: PresentationSettings {
        PresentationSettings(startFromSlideOne: startFromControl.selectedSegment == 1,
                             record: recordCheckbox.state == .on,
                             phoneRemote: phoneRemoteCheckbox.state == .on,
                             tunnel: tunnelCheckbox.state == .on)
    }

    /// The field's password, nil when empty. It is never saved.
    var presenterPassword: String? {
        let password = passwordField.stringValue.trimmingCharacters(in: .whitespaces)
        return password.isEmpty ? nil : password
    }

    /// Puts saved settings into the controls, once, when the popover is made.
    func loadSettings(_ settings: PresentationSettings) {
        startFromControl.selectedSegment = settings.startFromSlideOne ? 1 : 0
        recordCheckbox.state = settings.record ? .on : .off
        phoneRemoteCheckbox.state = settings.phoneRemote ? .on : .off
        tunnelCheckbox.state = settings.tunnel || settings.phoneRemote ? .on : .off
        tunnelCheckbox.isEnabled = !settings.phoneRemote
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
    /// The popover, whose controls are the last settings: loaded once from
    /// the environment, saved on every start.
    private(set) lazy var presentPopover: PresentPopoverController = {
        let popover = PresentPopoverController()
        popover.loadSettings(AppEnvironment.shared.presentationSettings.settings)
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

    /// Present > Play, Cmd+Option+P: the talk starts at once with the last
    /// settings (the popover's controls), from the cursor's slide unless
    /// those settings say slide 1.
    @objc func play(_ sender: Any?) {
        guard sessionController.presentation.canStart else { return }
        startPresenting(freshPopover().options(mode: .play))
    }

    /// Present > Play with Options: the popover, anchored on the Play button.
    @objc func playWithOptions(_ sender: Any?) {
        guard sessionController.presentation.canStart else { return }
        let anchor: NSView = playButton.window == nil ? (window?.contentView ?? playButton) : playButton
        freshPopover().show(context: popoverContext(), relativeTo: anchor.bounds, of: anchor)
    }

    /// The toolbar's Play button: the popover, or with Shift a start from slide 1 at once.
    @objc func playButtonPressed(_ sender: Any?) {
        playButtonClicked(modifiers: NSApp.currentEvent?.modifierFlags ?? [])
    }

    func playButtonClicked(modifiers: NSEvent.ModifierFlags) {
        guard sessionController.presentation.canStart else { return }
        if modifiers.contains(.shift) {
            var options = freshPopover().options(mode: .play)
            options.startSlide = 1
            startPresenting(options)
            return
        }
        playWithOptions(nil)
    }

    /// The popover with the settings as they are now and the cursor and
    /// displays as they are now: the settings are app-wide and another
    /// deck may have saved newer ones, and the cursor moved since the
    /// popover was last shown. The password field is left alone.
    private func freshPopover() -> PresentPopoverController {
        presentPopover.loadSettings(AppEnvironment.shared.presentationSettings.settings)
        presentPopover.update(context: popoverContext())
        return presentPopover
    }

    /// Present > Rehearse: the presenter view alone, from the cursor's slide.
    @objc func rehearse(_ sender: Any?) {
        guard sessionController.presentation.canStart else { return }
        startPresenting(PresentationOptions(mode: .rehearse, startSlide: sessionController.currentSlideNumber ?? 1))
    }

    /// Every start comes here: the popover's buttons, Play, the Shift-click
    /// and Rehearse. The popover's settings are saved, so the next
    /// Cmd+Option+P and the next launch start the same way.
    func startPresenting(_ options: PresentationOptions) {
        AppEnvironment.shared.presentationSettings.settings = presentPopover.settings
        sessionController.presentation.start(options)
        refreshPresentingControls()
    }

    func popoverContext() -> PresentPopoverController.Context {
        let presentation = sessionController.presentation
        return PresentPopoverController.Context(arrangement: presentation.currentArrangement,
                                                cursorSlide: sessionController.currentSlideNumber ?? 1,
                                                usesFullScreen: presentation.usesFullScreen)
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
            item.toolTip = "Present: choose displays and options. Shift-click to start from slide 1. Cmd+Option+P starts at once."
            playButton.image = NSImage(systemSymbolName: "play.fill", accessibilityDescription: "Play")
            playButton.bezelStyle = .toolbar
            playButton.setAccessibilityIdentifier("play-button")
            playButton.target = self
            playButton.action = #selector(playButtonPressed(_:))
            playButton.isEnabled = sessionController.presentation.canStart
            item.view = playButton
            return item
        }
```

- [ ] **Step 6: Run the tests one at a time**

Required:

```bash
make -C desktop test ONLY=TapTests/PresentPopoverTests/testThePopoverCollectsTheOptions
make -C desktop test ONLY=TapTests/PresentPopoverTests/testCmdOptionPStartsAtOnceWithTheLastSettings
make -C desktop test ONLY=TapTests/PresentPopoverTests/testThePopoverSaysWhenDisplaysShareOneSpace
make -C desktop test ONLY=TapTests/MenuTests
```

Optional: `testStartFromTheFirstSlide`, `testThePopoverWithOneDisplaySaysSo`, `testThePlayButtonFollowsTheTalk`, `ONLY=TapTests/WindowLayoutTests` (the toolbar gained an item; the menu is unchanged until Task 7). Expected: all pass.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted: in `playButtonClicked`, drop the `.shift` branch (expected: `testStartFromTheFirstSlide` fails, the popover shows); in `play(_:)`, show the popover instead of starting (expected: `testCmdOptionPStartsAtOnceWithTheLastSettings` fails on `isShown`); in `startPresenting`, drop the settings save (expected: `testThePopoverCollectsTheOptions` fails on the saved settings); in `presentPopover`'s init, drop `loadSettings` (expected: the Cmd+Option+P test fails on `record: false`); in `freshPopover`, drop `update(context:)` (expected: `testCmdOptionPStartsAtOnceWithTheLastSettings` fails on `startSlide: 4`, the context's default cursor being 1); in `freshPopover`, drop `loadSettings` (expected: it fails on the second deck's `record`); in `options(mode:)`, ignore `startFromSlideOne` (expected: `testThePopoverCollectsTheOptions` fails on `startSlide: 1`); in `phoneRemoteChanged`, drop `tunnelCheckbox.state = .on` (expected: it fails on the tunnel state); in `update(context:)`, never hide `arrangementView` (expected: `testThePopoverWithOneDisplaySaysSo` fails); in `update(context:)`, always hide `separateSpacesLabel` (expected: `testThePopoverSaysWhenDisplaysShareOneSpace` fails); in `refreshPresentingControls`, always enable the button (expected: `testThePlayButtonFollowsTheTalk` fails); in `onSwap`, drop `swapDisplays()` (expected: the options test fails on `currentArrangement`).

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
- Consumes: Task 4's controller (`stop`, `toggleFrontWindow`, `bringPresenterWindowForward`, `isActive`, `canStart`, `currentArrangement`, `arrangement`), Task 6's `play`, `playWithOptions`, `rehearse`; Task 3's `PresentationWindow.role`, `page.popupRequested`, `page.pressKey`.
- Produces: Present menu items Play (Cmd+Option+P, starts at once), Play with Options… (no shortcut, the popover), Rehearse (Cmd+Option+Shift+P), Stop (Cmd+.), Swap Displays; `DeckWindowController.stopPresenting(_:)`, `swapDisplays(_:)`, validation for the six; `PresentationController.handleKey(_:)` and the local monitor installed while the windows show.

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
        let withOptions = try item(menu, action: #selector(DeckWindowController.playWithOptions(_:)))
        XCTAssertEqual(withOptions.title, "Play with Options…")
        XCTAssertEqual(withOptions.keyEquivalent, "", "the popover has no shortcut; the Play button opens it")
        let rehearse = try item(menu, action: #selector(DeckWindowController.rehearse(_:)))
        XCTAssertEqual(rehearse.keyEquivalent, "p")
        XCTAssertEqual(rehearse.keyEquivalentModifierMask, [.command, .option, .shift])
        let stop = try item(menu, action: #selector(DeckWindowController.stopPresenting(_:)))
        XCTAssertEqual(stop.keyEquivalent, ".")
        XCTAssertEqual(stop.keyEquivalentModifierMask, [.command])
        let swap = try item(menu, action: #selector(DeckWindowController.swapDisplays(_:)))

        XCTAssertTrue(deckWindow.validateMenuItem(play))
        XCTAssertTrue(deckWindow.validateMenuItem(withOptions))
        XCTAssertTrue(deckWindow.validateMenuItem(rehearse))
        XCTAssertFalse(deckWindow.validateMenuItem(stop))
        XCTAssertFalse(deckWindow.validateMenuItem(swap), "one display: nothing to swap")

        // Cmd+Option+Shift+P starts rehearsing at once.
        deckWindow.rehearse(nil)
        XCTAssertEqual(controller.presentation.options?.mode, .rehearse)
        XCTAssertFalse(deckWindow.validateMenuItem(play))
        XCTAssertFalse(deckWindow.validateMenuItem(withOptions))
        XCTAssertFalse(deckWindow.validateMenuItem(rehearse))
        XCTAssertTrue(deckWindow.validateMenuItem(stop))
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
        deckWindow.stopPresenting(nil)
        try await waitUntil(timeout: 30, "the end") { controller.presentation.state == .idle }

        // Cmd+Option+P starts presenting at once, with the last settings and no popover.
        controller.jumpToSlide(number: 2)
        deckWindow.play(nil)
        XCTAssertFalse(deckWindow.presentPopover.isShown)
        XCTAssertEqual(controller.presentation.options?.mode, .play)
        XCTAssertEqual(controller.presentation.options?.startSlide, 2)
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
    }

    func testOneDisplay() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.targetFrame, NSScreen.screens[0].frame, "the audience page fills the screen")
        XCTAssertTrue(presentation.frontWindow === audience)
        XCTAssertFalse(presenter.isVisible)
        try await waitUntil(timeout: 5, "the audience on screen") { onScreenWindowNumbers().contains(audience.windowNumber) }
        // Option-Tab: the presenter view comes over the audience view, in the same Space, with no animation.
        let optionTab = try keyEvent(.keyDown, characters: "\t", keyCode: 48, modifiers: [.option], in: audience)
        XCTAssertNil(presentation.handleKey(optionTab), "the app takes Option-Tab")
        XCTAssertTrue(presentation.frontWindow === presenter)
        XCTAssertTrue(presentation.presenterIsShownOverAudience)
        XCTAssertTrue(presenter.isAttached)
        try await waitUntil(timeout: 5, "the presenter over the audience") {
            let order = onScreenWindowNumbers()
            guard let a = order.firstIndex(of: audience.windowNumber), let p = order.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
        let again = try keyEvent(.keyDown, characters: "\t", keyCode: 48, modifiers: [.option], in: presenter)
        XCTAssertNil(presentation.handleKey(again))
        XCTAssertTrue(presentation.frontWindow === audience)
        XCTAssertFalse(presentation.presenterIsShownOverAudience)
        try await waitUntil(timeout: 5, "the presenter off the screen again") { !onScreenWindowNumbers().contains(presenter.windowNumber) }
        let plainTab = try keyEvent(.keyDown, characters: "\t", keyCode: 48, in: audience)
        XCTAssertNotNil(presentation.handleKey(plainTab), "a plain Tab goes to the page")
        let elsewhere = try keyEvent(.keyDown, characters: "\t", keyCode: 48, modifiers: [.option], in: try XCTUnwrap(controller.editor.window))
        XCTAssertNotNil(presentation.handleKey(elsewhere), "Option-Tab in the deck window is not the app's")
    }

    func testOptionTabIsThePagesOnTwoDisplays() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        let screens = halfScreens()
        presentation.screens = { screens }
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let optionTab = try keyEvent(.keyDown, characters: "\t", keyCode: 48, modifiers: [.option], in: presenter)
        XCTAssertNotNil(presentation.handleKey(optionTab), "with a display each there is nothing to switch, so the page gets the key")
        XCTAssertTrue(presentation.frontWindow === presenter)
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
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        try await waitUntil(timeout: 30, "the end") { presentation.state == .idle }
        try await waitUntil(timeout: 10, "nothing left in full screen") { fullScreenPresentationWindows().isEmpty && presentation.windowsGoingDown.isEmpty }

        // A rehearsal has no audience window: Escape in the presenter window stops it.
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let rehearsal = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertNil(presentation.handleKey(try keyEvent(.keyDown, characters: "\u{1b}", keyCode: 53, in: rehearsal)))
        XCTAssertEqual(presentation.state, .stopping)
        try await waitUntil(timeout: 30, "the end") { presentation.state == .idle }
        try await waitUntil(timeout: 10, "nothing left in full screen") { fullScreenPresentationWindows().isEmpty }
    }

    /// Every key the scenario lists, as the page's handler would see it.
    static let pageKeys: [(characters: String, keyCode: UInt16)] = [
        (String(Character(Unicode.Scalar(UInt16(NSLeftArrowFunctionKey))!)), 123),
        (String(Character(Unicode.Scalar(UInt16(NSRightArrowFunctionKey))!)), 124),
        (String(Character(Unicode.Scalar(UInt16(NSDownArrowFunctionKey))!)), 125),
        (String(Character(Unicode.Scalar(UInt16(NSUpArrowFunctionKey))!)), 126),
        (" ", 49),
        (String(Character(Unicode.Scalar(UInt16(NSHomeFunctionKey))!)), 115),
        (String(Character(Unicode.Scalar(UInt16(NSEndFunctionKey))!)), 119),
        ("o", 31), ("t", 17), ("f", 3), ("?", 44), ("r", 15), ("v", 9),
        ("1", 18), ("2", 19), ("3", 20), ("4", 21), ("5", 23), ("-", 27), ("=", 24),
    ]

    func testEveryTapDevKeyGoesToThePageUnchanged() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        for window in [audience, presenter] {
            for key in Self.pageKeys {
                for modifiers in [[], [.shift]] as [NSEvent.ModifierFlags] {
                    let event = try keyEvent(.keyDown, characters: key.characters, keyCode: key.keyCode, modifiers: modifiers, in: window)
                    XCTAssertTrue(presentation.handleKey(event) === event, "\(key.characters) with \(modifiers) in the \(window.role) window is the page's")
                }
            }
        }
        XCTAssertEqual(presentation.state, .presenting, "none of them ended the talk")
    }

    func testEveryTapDevPresenterFeatureWorks() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 2))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        for page in [audience.page, presenter.page] {
            XCTAssertTrue(page.webView.configuration.preferences.isElementFullscreenEnabled, "F goes full screen")
            XCTAssertTrue(page.webView.configuration.websiteDataStore.isPersistent, "the page's localStorage outlives the process")
        }
        XCTAssertEqual(presenter.page.lastLoadedURL?.port, AppEnvironment.shared.deckPorts.port(for: try XCTUnwrap(controller.document?.fileURL)),
                       "the deck's port: the presenter layout and notes size persist between launches (Task 4 proves the port is reused)")
        try await waitUntil(timeout: 20, "the audience page on slide 2") { audience.page.lastReady?.slide == 2 }
        try await waitUntil(timeout: 20, "the presenter page on slide 2") { presenter.page.lastReady?.slide == 2 }

        // The arrow keys, and every other key, go to tap's page unchanged
        // (testEveryTapDevKeyGoesToThePageUnchanged): the page moves, the
        // hub relays it (the page holds the presenter cookie), and tap
        // reports the new position. The key is pressed in the page itself,
        // so the proof does not depend on which window the host has as key.
        await audience.page.pressKey("ArrowRight")
        try await waitUntil(timeout: 10, "tap's slide event for slide 3") { presentation.lastSlide == 3 }
        try await waitUntil(timeout: 10, "the presenter page following") { presenter.page.lastReady?.slide == 3 }

        // S opens the presenter view in the presenter window, never a browser popup: on one display it comes over the audience view.
        let port = try XCTUnwrap(presentation.client).ready.port
        audience.page.popupRequested(for: URL(string: "http://127.0.0.1:\(port)/presenter#3"), navigationType: .other)
        XCTAssertTrue(presentation.frontWindow === presenter)
        XCTAssertTrue(presentation.presenterIsShownOverAudience)
        try await waitUntil(timeout: 5, "the presenter over the audience") {
            let order = onScreenWindowNumbers()
            guard let a = order.firstIndex(of: audience.windowNumber), let p = order.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
        audience.page.popupRequested(for: URL(string: "http://127.0.0.1:\(port)/presenter#3"), navigationType: .other)
        XCTAssertTrue(presentation.presenterIsShownOverAudience, "S again keeps it there; Option-Tab is what hides it")
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
        menu.addItem(item("Play with Options…", action: #selector(DeckWindowController.playWithOptions(_:))))
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
        if [#selector(play(_:)), #selector(playWithOptions(_:)), #selector(rehearse(_:))].contains(menuItem.action) { return presentation.canStart }
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
    /// too, when there is no audience window: a rehearsal), and on one
    /// display Option-Tab switches to the other window's Space. Every
    /// other key goes to tap's page unchanged, and so does Option-Tab when
    /// each window has a display of its own. Internal so a test can drive
    /// it with an event of its own; the monitor calls it for every key
    /// down while the windows show.
    func handleKey(_ event: NSEvent) -> NSEvent? {
        guard isActive, let window = event.window as? PresentationWindow,
              window === audienceWindow || window === presenterWindow else { return event }
        if event.keyCode == 53, window.role == .audience || audienceWindow == nil {
            stop()
            return nil
        }
        if event.keyCode == 48, event.modifierFlags.contains(.option), arrangement?.isSingleDisplay == true, audienceWindow != nil {
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

Required:

```bash
make -C desktop test ONLY=TapTests/PresentMenuTests/testPresentingShortcuts
make -C desktop test ONLY=TapTests/PresentMenuTests/testEscapeInTheAudienceWindowStopsTheTalk
make -C desktop test ONLY=TapTests/PresentMenuTests/testEveryTapDevPresenterFeatureWorks
make -C desktop test ONLY=TapTests/PresentMenuTests/testEveryTapDevKeyGoesToThePageUnchanged
```

Optional: `testOneDisplay`, `testOptionTabIsThePagesOnTwoDisplays`, `ONLY=TapTests/MenuTests`. Expected: all pass. If `testEveryTapDevPresenterFeatureWorks` never sees `lastSlide == 3`: first check the presenter cookie is in the store (`presenterCookieInTheTalkStore()`); if it is, read the audience page's `window.__tapReadyState` through `HostedTestCase.pageStateScript` to see whether the page's key handler is installed (`keyboard.ts` listens on `window` for `keydown` and reads `event.key`), and check the dispatched event's `key` is exactly `"ArrowRight"`. The key press is the page's own event, so which window the host has as key does not matter.

- [ ] **Step 6: Mutate and commit**

Mutations, each reverted, the ones that can leave a window in full screen first: in `handleKey`, drop the Escape branch (expected: `testEscapeInTheAudienceWindowStopsTheTalk` fails); in `handleKey`, stop on Escape in any presentation window (expected: it fails on the presenter window's Escape, and `testEveryTapDevKeyGoesToThePageUnchanged` is unaffected since Escape is not in its list); in `takeDownWindows`, drop `removeKeyMonitor()` (survives here: the guard on `isActive` makes a stale monitor inert; note it, not a claim); in `handleKey`, drop the `.option` check (expected: `testOneDisplay` fails on the plain Tab); in `handleKey`, drop the `isSingleDisplay` check (expected: `testOptionTabIsThePagesOnTwoDisplays` fails); in `handleKey`, take "o" as the app's (expected: `testEveryTapDevKeyGoesToThePageUnchanged` fails); in `validateMenuItem`, return true for Stop always (expected: `testPresentingShortcuts` fails); in `presentMenu`, give Stop the key `"s"` (expected: it fails on the key equivalent); in `presentMenu`, give Play with Options the Cmd+Option+P key (expected: it fails on the empty key equivalent).

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
- Produces: `PresenterToolbar` with `titleLabel`, `recordButton`, `editsLabel`, `reloadButton`, `swapButton`, `stopButton`, `isShown`, `hideDelay`, `onRecord`, `onReload`, `onSwap`, `onStop`, `pointerReachedBottomEdge()`, `pointerLeft()`, `update(recording:editsNotShown:mode:)`; `RecordingDot`; `PresentationWindow.toolbar` (pinned to the bottom edge), `recordingDot`, `onMouseMoved`; `PresentationController.editsNotShown`, `presentedText`, `deckTextChanged(_:)`, `reloadSlides()`, `cursorHideDelay`, `hideCursor`, `isCursorHideArmed`, `noteMouseMoved()`, `toggleRecording()`; `DeckWindowController.reloadSlides(_:)`; the Present menu item Reload Slides (Cmd+R).

- [ ] **Step 0: The mockup and the person's sign-off**

The toolbar is the one piece of UI this plan adds that the mockups do not show as built: system full screen (decision 1) drops the menu bar over the top edge, so the toolbar slides up from the bottom edge instead (decision 7). Before any of this task's UI code is written, the coordinating agent (not the implementer) shows the person a mockup of the presenter window in full screen with the toolbar revealed along the bottom edge (REC and the title at the left, the edits label, Reload Slides, Swap Displays and Stop at the right, 52 pt tall, the dark translucent band, the REC dot in the top-right corner) and gets a sign-off, per the person's rule for UI changes. The ledger records the sign-off (date, and any change the person asked for, which then goes into Steps 3 and 4 before they run). The implementer does not start Step 3 until the ledger has that line.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/PresenterToolbarTests.swift`:

```swift
import XCTest
@testable import Tap

final class PresenterToolbarTests: PresentingTestCase {
    /// A pointer move to `point`, in window coordinates, as AppKit's tracking area would deliver it.
    func mouseMove(to point: NSPoint, in window: NSWindow) throws -> NSEvent {
        try XCTUnwrap(NSEvent.mouseEvent(with: .mouseMoved, location: point, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                         windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 0, pressure: 0))
    }

    func testPresenterControls() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let toolbar = try XCTUnwrap(presenter.toolbar)
        let dot = try XCTUnwrap(presenter.recordingDot)
        XCTAssertNil(presentation.audienceWindow?.toolbar, "the audience window has no toolbar")
        XCTAssertTrue(toolbar.isHidden, "the toolbar is out of sight until the pointer reaches the bottom edge")
        XCTAssertFalse(toolbar.isShown)
        XCTAssertTrue(dot.isHidden, "no REC dot while nothing records")
        XCTAssertTrue(presenter.container.trackingAreas.contains { $0.options.contains(.mouseMoved) && $0.options.contains(.activeAlways) },
                      "the content view tracks the pointer everywhere in the window")

        // The pointer reaches the bottom edge: the toolbar slides up, and the idle cursor timer is armed.
        let height = presenter.container.bounds.height
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: 1), in: presenter))
        XCTAssertTrue(toolbar.isShown)
        XCTAssertFalse(toolbar.isHidden)
        XCTAssertTrue(presentation.isCursorHideArmed, "a move over a talk window arms the cursor hide")
        XCTAssertEqual(toolbar.recordButton.title, "NOT RECORDING")
        XCTAssertEqual(toolbar.reloadButton.title, "Reload Slides")
        XCTAssertEqual(toolbar.swapButton.title, "Swap Displays")
        XCTAssertEqual(toolbar.stopButton.title, "Stop")
        XCTAssertTrue(toolbar.editsLabel.isHidden)
        XCTAssertEqual(toolbar.frame.minY, 0, "it sits along the bottom edge, away from the menu bar full screen drops over the top")
        XCTAssertEqual(toolbar.frame.height, PresenterToolbar.height)

        // The pointer moves up into the page: the toolbar slides away after its delay. The top edge is the menu bar's, not the toolbar's.
        toolbar.hideDelay = 0.1
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: height / 2), in: presenter))
        try await waitUntil(timeout: 2, "the toolbar to slide away") { toolbar.isHidden }
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: height - 1), in: presenter))
        XCTAssertTrue(toolbar.isHidden, "the top edge does nothing")
        // A move within the toolbar's own band keeps it.
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: 1), in: presenter))
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: PresenterToolbar.height / 2), in: presenter))
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertFalse(toolbar.isHidden, "the pointer on the toolbar itself does not send it away")
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: height / 2), in: presenter))
        try await waitUntil(timeout: 2, "the toolbar away") { toolbar.isHidden }
        toolbar.pointerReachedBottomEdge()
        toolbar.pointerLeft()
        toolbar.pointerReachedBottomEdge()
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

        toolbar.pointerReachedBottomEdge()
        toolbar.stopButton.performClick(nil)
        XCTAssertEqual(presentation.state, .stopping)
    }

    func testARehearsalHasNoRecordingOrSwapControls() async throws {
        let (_, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let toolbar = try XCTUnwrap(controller.presentation.presenterWindow?.toolbar)
        toolbar.pointerReachedBottomEdge()
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
        toolbar.pointerReachedBottomEdge()
        XCTAssertFalse(toolbar.editsLabel.isHidden)
        XCTAssertEqual(toolbar.editsLabel.stringValue, "1 edit not shown")

        // Another typing pause counts again; undoing back to the presented text counts nothing.
        controller.editor.insertText("!", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 15, "the second answer") { controller.lastAppliedText?.contains("# One edited!") == true }
        XCTAssertEqual(presentation.editsNotShown, 2)
        XCTAssertEqual(toolbar.editsLabel.stringValue, "2 edits not shown")
        // tap dev answering again for the same text (a restart, a component change) is not an edit.
        presentation.deckTextChanged(try XCTUnwrap(controller.lastAppliedText))
        XCTAssertEqual(presentation.editsNotShown, 2, "the same text counts once")

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
        // Two moves, one hide: the first move's work item was cancelled, not merely outrun.
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(hides, 2)
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/PresenterToolbarTests/testARehearsalHasNoRecordingOrSwapControls`
Expected: the test target does not compile (`toolbar` is undefined on `PresentationWindow`).

- [ ] **Step 3: Write `PresenterToolbar.swift`**

```swift
import AppKit

/// The app's toolbar over the presenter page: REC, the edits the audience
/// has not seen, Reload Slides, Swap Displays and Stop. It is out of sight
/// while the speaker talks and slides up when the pointer reaches the
/// bottom edge, then slides away once the pointer has left it. The bottom,
/// not the top: in system full screen the menu bar drops over the top
/// edge when the pointer rests there, and would cover a toolbar (the
/// person's decision 7, 2026-09-25).
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

    /// The pointer touched the bottom edge: the toolbar comes up and stays while the pointer is on it.
    func pointerReachedBottomEdge() {
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
                toolbar.bottomAnchor.constraint(equalTo: container.bottomAnchor),
                toolbar.leadingAnchor.constraint(equalTo: container.leadingAnchor),
                toolbar.trailingAnchor.constraint(equalTo: container.trailingAnchor),
                toolbar.heightAnchor.constraint(equalToConstant: PresenterToolbar.height),
                dot.topAnchor.constraint(equalTo: container.topAnchor, constant: 16),
                dot.trailingAnchor.constraint(equalTo: container.trailingAnchor, constant: -18),
            ])
            self.toolbar = toolbar
            recordingDot = dot
        }
        // The bottom edge: the top is where full screen drops the menu bar.
        container.onMouseMoved = { [weak self] point in
            guard let self else { return }
            self.onMouseMoved?()
            guard let toolbar = self.toolbar else { return }
            if point.y <= 2 {
                toolbar.pointerReachedBottomEdge()
            } else if point.y > PresenterToolbar.height {
                toolbar.pointerLeft()
            }
        }
```

Add at the end of the file:

```swift
/// The talk window's content view: it tracks the pointer everywhere in the
/// window, so the toolbar can slide up at the bottom edge and the cursor can
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
    /// The text tap dev last answered for, so an answer for the same text is not counted twice.
    private var lastCountedText: String?
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
    /// the text is back to what tap present read. tap dev also answers
    /// after a restart and after a component change with the text
    /// unchanged; those are not edits, so a text counts once.
    func deckTextChanged(_ text: String) {
        guard isActive else { return }
        if text == presentedText {
            editsNotShown = 0
        } else if text != lastCountedText {
            editsNotShown += 1
        }
        lastCountedText = text
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
        let text = editor.string
        guard isContentEdited else {
            presentation.presentedText = text
            return completion(nil)
        }
        // The text counts as presented only once it is on disk: a refused save leaves the old value.
        document.save(to: url, ofType: document.fileType ?? "net.daringfireball.markdown", for: .saveOperation) { [weak self] error in
            if error == nil { self?.presentation.presentedText = text }
            completion(error)
        }
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

Required:

```bash
make -C desktop test ONLY=TapTests/PresenterToolbarTests/testPresenterControls
make -C desktop test ONLY=TapTests/PresenterToolbarTests/testEditWhilePresenting
make -C desktop test ONLY=TapTests/PresentingTests/testTheMacStaysAwake
```

Optional: `testARehearsalHasNoRecordingOrSwapControls`, `ONLY=TapTests/PresentMenuTests/testPresentingShortcuts`. Expected: all pass. In system full screen the macOS menu bar drops down when the pointer rests at the very top of the screen; the toolbar lives at the bottom edge so the two never meet, and the person's manual check is that the bottom-edge reveal feels right on the laptop's own screen. If `testEditWhilePresenting` never shows the edit after Reload Slides, check `tap present --app` logged `reload` in the talk's log (`presentation.session?.log.text`) and that the file holds the edit; a `reload_failed` error event names tap's reason.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted: in `PresentationWindow.init`, invert the `point.y <=` comparison (expected: `testPresenterControls` fails on `toolbar.isShown` after the bottom-edge move, and on "the top edge does nothing"); in `PresentationWindow.init`, drop the `container.onMouseMoved = ...` wiring (expected: it fails on `isShown` and on `isCursorHideArmed`); in `PresentationContentView.updateTrackingAreas`, drop `addTrackingArea` (expected: it fails on `trackingAreas`); in `PresentationWindow.init`, call `pointerLeft()` for every move above the bottom edge (expected: it fails on "the pointer on the toolbar itself"); in `PresentationWindow.init`, pin the toolbar to the top anchor (expected: it fails on `frame.minY`); in `reloadSlides`, skip `saveDeck` and send `.reload` at once (expected: `testEditWhilePresenting` fails: the file lacks the edit, so the audience never shows it); in `deckTextChanged`, drop the `text != lastCountedText` check (expected: `testEditWhilePresenting` fails on "the same text counts once"); in `saveForPresenting`, set `presentedText` before the save (survives: no test refuses a Reload Slides save; leave as a written property); in `deckTextChanged`, drop the `text == presentedText` branch (survives: the test never undoes; leave as a written property); in `update(recording:editsNotShown:mode:)`, never hide `recordButton` (expected: the rehearsal test fails); in `pointerLeft`, drop the delayed hide (expected: `testPresenterControls` times out); in `refreshPresenterToolbar`, drop the dot line (expected: it fails on `dot.isHidden`); in `noteMouseMoved`, drop `cursorHideWork?.cancel()` (expected: `testTheMacStaysAwake` counts 3 hides, not 2, at the 0.3 s check).

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
- Consumes: Task 4's `PendingQuestion`, `pendingQuestions`, `onQuestion`, `answer(id:value:)`, `windowsShown`, `frontWindow`; Task 8's `refreshPresenterToolbar`, `PresenterToolbar.recordButton`, `RecordingDot`; Task 1's `QuestionPayload`, `RecordingEvent`.
- Produces: `QuestionSheet(kind:title:body:path:decline:accept:escape:)` with `EscapeAnswer` (`.decline`, `.nothing`), `kind`, `titleLabel`, `bodyLabel`, `pathLabel`, `declineButton`, `acceptButton`, `button(titled:)`, `static consent(settingsPath:)`; `PresentationController.returnToTalk()`; `DeckWindowController.questionSheet`, `presentQuestion(_:)`, `showQuestionSheet(_:completion:)`, `endQuestionSheet(as:)`; `FakeTapScripts.presenting(events:quit:tunnelFailed:tunnelUnavailable:recordingTo:)`, `QuitBehavior` (`.exit`, `.askToKeep(directory:segments:)`, `.askToKeepThenExit(after:directory:segments:)`), `onePixelPNG`.

With system full screen a sheet needs no window juggling: the deck window lives on the desktop Space, so bringing it forward for the sheet switches Spaces away from the talk, and making the front talk window key afterwards switches back. On two displays the audience Space on the projector is untouched either way.

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
        /// tap does once its 60 s wait is over (and at once when its stdin
        /// closes). The fake's wait is short so the test is not.
        case askToKeepThenExit(after: TimeInterval, directory: URL, segments: Int)
    }

    /// A 1 by 1 PNG, base64: the smallest QR code a fake can send.
    static let onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

    /// A scripted `tap present --app`: it records its arguments and every
    /// stdin line in `record`, prints a ready line (with no server behind
    /// it), then `events` one per line, and answers commands the way tap
    /// does: a tunnel start with a running tunnel (or, with
    /// `tunnelUnavailable`, the error tap sends without cloudflared; or,
    /// with `tunnelFailed`, the `tunnel_failed` error followed by the
    /// stopped tunnel event tap sends after a failed start), a tunnel stop
    /// with a stopped tunnel, a recording stop with a stopped recording, a
    /// new segment with segment 2 recording, and quit as `quit` says.
    static func presenting(events: [String], quit: QuitBehavior = .exit, tunnelFailed: Bool = false, tunnelUnavailable: Bool = false,
                           recordingTo record: URL) throws -> URL {
        let url = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        let eventLines = events.map { "echo '\($0)'" }.joined(separator: "\n")
        let tunnelRunning: String
        if tunnelUnavailable {
            tunnelRunning = #"echo '{"type":"error","code":"tunnel_unavailable","message":"the tunnel needs cloudflared: brew install cloudflared"}'"#
        } else if tunnelFailed {
            tunnelRunning = #"echo '{"type":"tunnel","state":"starting"}'; echo '{"type":"error","code":"tunnel_failed","message":"cloudflared exited: connection refused"}'; echo '{"type":"tunnel","state":"stopped"}'"#
        } else {
            tunnelRunning = #"echo '{"type":"tunnel","state":"starting"}'; echo '{"type":"tunnel","state":"running","url":"https://stark-lake-1234.trycloudflare.com","qr":"\#(onePixelPNG)"}'"#
        }
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
        XCTAssertEqual(sheet.titleLabel.stringValue, "Record automatically every time you present?", "the spec's words (05-presenting)")
        XCTAssertTrue(sheet.bodyLabel.stringValue.contains("follows the projector"))
        XCTAssertEqual(sheet.pathLabel.stringValue, settingsFile.path, "tap says where the answer is saved")
        XCTAssertEqual(sheet.declineButton.title, "Don't Record")
        XCTAssertEqual(sheet.acceptButton.title, "Record Automatically")
        XCTAssertEqual(presentation.state, .starting, "the talk waits for the answer")
        // Past the page load and the show-windows fallback: the sheet still has nothing over it.
        try await Task.sleep(nanoseconds: UInt64((PresentationController.showWindowsFallbackInterval + 0.5) * 1_000_000_000))
        XCTAssertEqual(presentation.state, .starting)
        XCTAssertFalse(presentation.windowsShown, "nothing covers the sheet")
        XCTAssertFalse(presentation.audienceWindow?.isVisible ?? false)
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        if let audience = presentation.audienceWindow {
            XCTAssertFalse(onScreenWindowNumbers().contains(audience.windowNumber))
        }

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

    func testAQuestionDuringTheTalkBringsTheDeckWindowForward() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let deck = try XCTUnwrap(deckWindow.window)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        presentation.handle(.question(id: "q9", kind: "record-consent", payload: QuestionPayload(settingsPath: "/tmp/settings.yaml")))
        let sheet = try XCTUnwrap(deckWindow.questionSheet)
        XCTAssertTrue(deck.attachedSheet === sheet)
        XCTAssertNil(presentation.windowsGoingDown.first, "the talk windows stay where they are")
        // In full screen the deck window's Space becomes the active one and the audience's leaves the screen;
        // on a host without full screen the deck window simply comes over the audience's plain window.
        try await waitUntil(timeout: 5, "the deck window in front of the talk") {
            let order = onScreenWindowNumbers()
            guard let deckIndex = order.firstIndex(of: deck.windowNumber) else { return false }
            return order.firstIndex(of: audience.windowNumber).map { deckIndex < $0 } ?? true
        }

        // A second question arriving on top waits its turn.
        presentation.handle(.question(id: "q10", kind: "approval", payload: QuestionPayload(deck: "/tmp/ops.md")))
        XCTAssertEqual(presentation.pendingQuestions.count, 2)
        XCTAssertTrue(deckWindow.questionSheet === sheet, "the first sheet is still the one up")

        try XCTUnwrap(sheet.button(titled: "Record Automatically")).performClick(nil)
        XCTAssertNil(presentation.pendingQuestions.first { $0.id == "q9" })
        XCTAssertNil(deckWindow.questionSheet, "the approval is declined with a log line until D5, so no second sheet")
        try await waitUntil(timeout: 5, "the approval answered") { presentation.pendingQuestions.isEmpty }
        XCTAssertTrue(presentation.frontWindow === audience)
        try await waitUntil(timeout: 5, "the talk in front again") {
            let order = onScreenWindowNumbers()
            guard let audienceIndex = order.firstIndex(of: audience.windowNumber) else { return false }
            return order.firstIndex(of: deck.windowNumber).map { audienceIndex < $0 } ?? true
        }
        XCTAssertEqual(presentation.state, .presenting)
    }

    func testStopDuringTheConsentSheetEndsTheSheetToo() async throws {
        try removeRecordingConsent()
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "tap's consent question") { deckWindow.questionSheet?.kind == "record-consent" }
        let stale = try XCTUnwrap(deckWindow.questionSheet)
        // Present > Stop is enabled while the talk is starting.
        deckWindow.stopPresenting(nil)
        try await waitUntil(timeout: 30, "the talk to end") { presentation.state == .idle }
        XCTAssertNil(deckWindow.questionSheet, "a talk's sheets end with it")
        XCTAssertNil(deckWindow.window?.attachedSheet)
        XCTAssertTrue(presentation.pendingQuestions.isEmpty)
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)

        // The next talk asks again, with a sheet of its own, and nothing stale clears it.
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "the new consent question") { deckWindow.questionSheet != nil && deckWindow.questionSheet !== stale }
        let sheet = try XCTUnwrap(deckWindow.questionSheet)
        try XCTUnwrap(sheet.button(titled: "Don't Record")).performClick(nil)
        XCTAssertNil(deckWindow.questionSheet)
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
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
        XCTAssertTrue(toolbar.recordButton.title.hasPrefix("REC 0:0"), "a fresh segment; the 1 s timer may already have ticked: \(toolbar.recordButton.title)")

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
/// app-modal. Return is the accept button. Escape is the decline button
/// only where declining is harmless; a destructive decline has no key,
/// so a stray Escape after a talk (Escape is how a talk ends) can never
/// choose it.
final class QuestionSheet: NSWindow {
    enum EscapeAnswer {
        /// Escape presses the decline button.
        case decline
        /// Escape does nothing; the person clicks.
        case nothing
    }

    let kind: String
    let titleLabel = NSTextField(labelWithString: "")
    let bodyLabel = NSTextField(wrappingLabelWithString: "")
    let pathLabel = NSTextField(labelWithString: "")
    let declineButton = NSButton(title: "", target: nil, action: nil)
    let acceptButton = NSButton(title: "", target: nil, action: nil)

    init(kind: String, title: String, body: String, path: String?, decline: String, accept: String, escape: EscapeAnswer = .decline) {
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
        declineButton.keyEquivalent = escape == .decline ? "\u{1b}" : ""
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
                      title: "Record automatically every time you present?",
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

In `init`, replace the `onStateChange` line (Task 6) with these two:

```swift
        sessionController.presentation.onStateChange = { [weak self] state in
            self?.refreshPresentingControls()
            switch state {
            case .idle, .failed: self?.talkEnded()
            case .starting, .presenting, .stopping: break
            }
        }
        sessionController.presentation.onQuestion = { [weak self] question in self?.presentQuestion(question) }
```

Add after `reloadSlides(_:)`:

```swift
    /// The talk is idle or failed: no sheet of its outlives it. A sheet
    /// ended this way answers nothing (the talk's questions are gone with
    /// it); Task 10 gives the keep-recording sheet its own ending.
    func talkEnded() {
        endQuestionSheet(as: .abort)
    }
```

and then:

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

    /// Puts `sheet` on this window and calls back with the answer. This
    /// window comes forward, which switches to its Space and leaves the
    /// talk's Spaces where they are: the sheet is the one thing the person
    /// must answer, so this is the one focus move outside the talk windows.
    /// After the answer the talk's front window is made key again, which
    /// switches back.
    func showQuestionSheet(_ sheet: QuestionSheet, completion: @escaping (Bool) -> Void) {
        guard let window else {
            completion(sheet.kind == "keep-recording")
            return
        }
        let presentation = sessionController.presentation
        questionSheet = sheet
        window.makeKeyAndOrderFront(nil)
        window.beginSheet(sheet) { [weak self] response in
            // Only the sheet that completed clears the slot: a stale sheet
            // ended late must not clear a newer one.
            if self?.questionSheet === sheet { self?.questionSheet = nil }
            completion(response == .OK)
            presentation.returnToTalk()
        }
    }

    /// Ends the sheet that is up, if any, as `response`. The talk ending
    /// calls this so no sheet outlives the talk that asked.
    func endQuestionSheet(as response: NSApplication.ModalResponse) {
        guard let sheet = questionSheet, let window else { return }
        window.endSheet(sheet, returnCode: response)
    }
```

In `PresentationController.swift`, add a stored property after `windowsShown`:

```swift
    private var recordingTimer: Timer?
```

Add after `answer(id:value:)`:

```swift
    /// A sheet on the deck window is gone: the talk's front window is made
    /// key again, which brings its Space back.
    func returnToTalk() {
        guard isActive, windowsShown, let frontWindow else { return }
        frontWindow.makeKeyAndOrderFront(nil)
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

In `openWindows`, add `startRecordingTimer()` after `sleepAssertion.acquire()`. In `takeDownWindows`, add after the screen observer lines:

```swift
        recordingTimer?.invalidate()
        recordingTimer = nil
```

- [ ] **Step 6: Run the tests one at a time**

Required:

```bash
make -C desktop test ONLY=TapTests/RecordingTests/testFirstTalkAsksAboutRecording
make -C desktop test ONLY=TapTests/RecordingTests/testAQuestionDuringTheTalkBringsTheDeckWindowForward
make -C desktop test ONLY=TapTests/RecordingTests/testStopDuringTheConsentSheetEndsTheSheetToo
make -C desktop test ONLY=TapTests/RecordingTests/testRecordingFollowsTapPresent
```

Expected: all four pass. `testFirstTalkAsksAboutRecording` runs the real tap: with no `settings.yaml`, tap asks within milliseconds of ready, before any page has loaded, so the windows are never shown before the sheet; the 3.5 s wait proves they stay unshown past the fallback. After the answer tap's startup ends with `hub.BroadcastReload()`, so both pages reload once just as the windows enter full screen: a brief flash that is tap's own, not a bug to chase. tap saves `present: record: false` under the test's `XDG_CONFIG_HOME`; the person's own settings are never touched. In `testStopDuringTheConsentSheetEndsTheSheetToo`, tap's consent question is cancelled by its own quit path (`app_session.go`, `end` cancels the startup context, and `appQuestions.ask` returns unanswered), so tap exits cleanly without an answer.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted, the one that can leave a window over the sheet first: in `showWindowsIfReady`, drop `pendingQuestions.isEmpty` (expected: `testFirstTalkAsksAboutRecording` fails on `windowsShown` after the 3.5 s wait); in `returnToTalk`, drop `makeKeyAndOrderFront` (expected: the mid-talk test times out on "the talk's Space active again"); in `showQuestionSheet`, drop `window.makeKeyAndOrderFront(nil)` (expected: it times out on "the deck window's Space active"); in `handle`, call `onQuestion` for every question rather than the first (expected: it fails on `questionSheet === sheet`); in `showQuestionSheet`'s completion, clear `questionSheet` unconditionally (survives: `endSheet` runs the completion synchronously, so the stale sheet is cleared before the new one is set; the guard is for a sheet AppKit ends late, which no test reaches; leave it as a written property); in `presentQuestion`, answer consent `true` regardless (expected: the consent test fails on `record: false`); in `QuestionSheet.consent`, use the mockup's title (expected: the consent test fails on the spec's words); in `startRecordingTimer`, never tick (expected: `testRecordingFollowsTapPresent` fails on the count); in `toggleRecording`, always send `.stop` (expected: it fails on segment 2); in `showQuestionSheet`, answer without a sheet (expected: the consent test fails on `attachedSheet`).

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
- Consumes: Task 9's `QuestionSheet` (with `escape: .nothing`), `showQuestionSheet`, `endQuestionSheet(as:)`, `talkEnded`, `presentQuestion`, `FakeTapScripts.presenting(events:quit:recordingTo:)`; Task 4's `stop`, `state`, `onStateChange`, `quitTimeoutWithRecording`.
- Produces: `QuestionSheet.keepRecording(directory:segments:size:)` whose Delete has no key equivalent and `hasDestructiveAction`; `DeckWindowController.revealInFinder`, `static folderSize(at:)`, `talkEnded()` ending a keep-recording sheet as Keep.

A second Escape after a talk is common (Escape ended it), and tap's own wait makes that second press land on this sheet. Escape here does nothing; only a click can delete (C2 in the review). tap waits for the answer while stdin is open, up to 60 s (pull request 35); the app's quit deadline is extended to 75 s when the question arrives (Task 4), so closing stdin never answers for the person.

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
        let size = ByteCountFormatter.string(fromByteCount: 3072, countStyle: .file)
        XCTAssertEqual(sheet.bodyLabel.stringValue, "2 segments, \(size) on disk.")
        XCTAssertEqual(sheet.pathLabel.stringValue, folder.path)
        XCTAssertEqual(sheet.declineButton.title, "Delete")
        XCTAssertEqual(sheet.declineButton.keyEquivalent, "", "no key can delete")
        XCTAssertTrue(sheet.declineButton.hasDestructiveAction)
        XCTAssertEqual(sheet.acceptButton.title, "Keep and Show in Finder")
        XCTAssertTrue(presentation.session?.log.text.contains("tap asks a keep-recording question") == true)

        // A second Escape, the one that ended the talk a moment ago, lands on the sheet and does nothing.
        let escape = try XCTUnwrap(NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                                    windowNumber: sheet.windowNumber, context: nil, characters: "\u{1b}",
                                                    charactersIgnoringModifiers: "\u{1b}", isARepeat: false, keyCode: 53))
        XCTAssertFalse(sheet.performKeyEquivalent(with: escape))
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet, "the sheet is still up")
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertFalse((try? String(contentsOf: record, encoding: .utf8))?.contains(#""value":false"#) ?? false, "nothing was deleted")
        XCTAssertEqual(presentation.state, .stopping, "tap is still waiting for the answer")

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
        // tap's own wait (60 s in tap, 0.5 s in the fake) is over: it kept the recording and exited.
        try await waitUntil(timeout: 10, "the talk to end") { controller.presentation.state == .idle }
        XCTAssertNil(deckWindow.questionSheet, "the sheet goes with the process that asked")
        XCTAssertNil(deckWindow.window?.attachedSheet)
        XCTAssertEqual(revealed, [folder], "kept, so shown")
        XCTAssertTrue(controller.presentation.lastTalkLog?.text.contains("tap kept the recording") == true, "in the talk's log, not tap dev's")
    }

    func testAClosedDeckKeepsItsRecordingWithoutAsking() async throws {
        let folder = try recordingFolder()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], quit: .askToKeep(directory: folder, segments: 2), recordingTo: record)
        let (document, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let talk = WeakTalk(controller.presentation)
        // The deck closes mid-talk: nobody is left to answer, so the app answers for tap's own default at once.
        document.close()
        try await waitUntil(timeout: 2, "the keep answer to reach tap") {
            (try? String(contentsOf: record, encoding: .utf8))?.contains(#"stdin: {"type":"answer","id":"q1","value":true}"#) == true
        }
        try await waitUntil(timeout: 5, "the talk counted out") { !AppEnvironment.shared.isPresenting }
        XCTAssertTrue(AppEnvironment.shared.endingTalks.isEmpty)
        XCTAssertEqual(revealed, [], "no window to reveal from")
        XCTAssertTrue(talk.presentation?.lastTalkLog?.text.contains("the deck window has closed; the keep-recording question is answered keep") ?? true)
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
    /// recorded. `size` is the run folder's size, formatted. Delete is
    /// destructive: no key reaches it, and a stray Escape after the talk
    /// does nothing here.
    static func keepRecording(directory: String, segments: Int, size: String) -> QuestionSheet {
        let sheet = QuestionSheet(kind: "keep-recording",
                                  title: "Keep this recording?",
                                  body: "\(segments) segment\(segments == 1 ? "" : "s"), \(size) on disk.",
                                  path: directory,
                                  decline: "Delete",
                                  accept: "Keep and Show in Finder",
                                  escape: .nothing)
        sheet.declineButton.hasDestructiveAction = true
        return sheet
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

    /// The talk is idle or failed: no sheet of its outlives it. A
    /// keep-recording sheet still up means tap exited before an answer
    /// came: its 60 s wait ran out (or the app's stdin closed) and it kept
    /// the recording, so the sheet ends as a yes and the run is revealed
    /// like any kept run. Any other sheet ends answering nothing.
    func talkEnded() {
        guard let sheet = questionSheet else { return }
        if sheet.kind == "keep-recording" {
            // The talk's own log, which outlives its session; never tap dev's.
            sessionController.presentationIfCreated?.lastTalkLog?.append("tap kept the recording before an answer came", source: .app)
            endQuestionSheet(as: .OK)
        } else {
            endQuestionSheet(as: .abort)
        }
    }
```

This replaces Task 9's `talkEnded()`; the `onStateChange` closure that calls it is unchanged.

- [ ] **Step 4: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/KeepRecordingTests/testKeepTheRecording
make -C desktop test ONLY=TapTests/KeepRecordingTests/testDeletingTheRecordingAnswersNo
make -C desktop test ONLY=TapTests/KeepRecordingTests/testATapThatKeepsWithoutWaitingRevealsTheRun
make -C desktop test ONLY=TapTests/KeepRecordingTests/testAClosedDeckKeepsItsRecordingWithoutAsking
```

Expected: all three pass. `ByteCountFormatter` prints 3,072 bytes as "3 KB".

- [ ] **Step 5: Mutate and commit**

Mutations, each reverted, the one that can delete a recording first: in `keepRecording`, pass `escape: .decline` (expected: `testKeepTheRecording` fails on the key equivalent, and on `"value":false` reaching tap); in `keepRecording`, drop `hasDestructiveAction` (expected: it fails on that assertion); in the keep-recording case, reveal on `false` too (expected: `testDeletingTheRecordingAnswersNo` fails); answer `true` regardless (expected: it fails on the record file); in `talkEnded`, end every sheet as `.abort` (expected: the no-wait test fails on `revealed`); in `talkEnded`, drop the call for keep-recording (expected: the no-wait test fails on `questionSheet`); in `handle`, drop `extendQuit` on the keep-recording question (survives here, since the fakes answer within 15 s; it is the person's manual check with a real recording, timed: Stop, wait 20 s, then Delete must still delete); in `keepRecording`, drop the segments count from the body (expected: `testKeepTheRecording` fails on the body).

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
- Consumes: Task 1's `TunnelEvent`, `TapCommand.tunnel(start:)`; Task 2's `PresentationOptions.wantsTunnel`; Task 9's `FakeTapScripts.presenting(events:quit:tunnelFailed:tunnelUnavailable:recordingTo:)`; Task 3's `PresentationWindow`.
- Produces: `RemotePanel` with `qrImageView`, `urlLabel`, `noteLabel`, `messageLabel`, `turnOffButton`, `onTurnOff`, `show(tunnel:error:ownPassword:on:)`, `hide()`; `PresentationController.tunnel`, `tunnelError`, `onTunnelChange`, `setTunnel(on:)`; `DeckWindowController.remotePanel` (made with the window, closed with it), `togglePhoneRemote(_:)`, `refreshRemotePanel()`; the Present menu item Phone Remote.

The panel is a floating panel with `.fullScreenAuxiliary`, so it shows over the presenter window's full screen Space.

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
        XCTAssertEqual(panel.level, .floating)
        XCTAssertTrue(panel.collectionBehavior.contains(.fullScreenAuxiliary), "shows over the presenter window's full screen Space")
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
        XCTAssertEqual(presentation.session?.command, .present(record: true, presenterPassword: "secret", port: nil))
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

    func testAFailedTunnelStartKeepsItsReason() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], tunnelFailed: true, recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, phoneRemote: true))
        // tap sends tunnel_failed and then a stopped tunnel event; the reason must survive the second.
        try await waitUntil(timeout: 5, "tap's stopped tunnel") { presentation.tunnel?.state == "stopped" }
        XCTAssertEqual(presentation.tunnelError, "cloudflared exited: connection refused")
        let panel = deckWindow.remotePanel
        XCTAssertTrue(panel.isVisible, "the panel stays with the reason")
        XCTAssertFalse(panel.messageLabel.isHidden)
        XCTAssertTrue(panel.messageLabel.stringValue.contains("connection refused"))
        // Trying again sends a second start; the fake answers starting, failed and stopped in one write, so the
        // observable proof is the second command in the record file and the reason still standing afterwards.
        deckWindow.togglePhoneRemote(nil)
        try await waitUntil(timeout: 5, "a second tunnel start") {
            (try? String(contentsOf: record, encoding: .utf8))?.components(separatedBy: #"stdin: {"type":"tunnel","start":true}"#).count == 3
        }
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(presentation.tunnelError, "cloudflared exited: connection refused", "the second failure's reason stands")
        XCTAssertTrue(panel.isVisible)
        try await stopPresenting(controller)
        XCTAssertFalse(panel.isVisible)
    }

    func testThePanelClosesWithTheDeckWindow() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], recordingTo: record)
        let (document, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, phoneRemote: true))
        let panel = deckWindow.remotePanel
        try await waitUntil(timeout: 5, "the panel") { panel.isVisible }
        document.close()
        XCTAssertFalse(panel.isVisible, "no panel outlives its deck window")
        try await waitUntil(timeout: 5, "the panel off screen") { !onScreenWindowNumbers().contains(panel.windowNumber) }
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
        // A floating panel that may show over a full screen Space, and moves
        // to whichever Space is active when it is ordered front.
        level = .floating
        collectionBehavior = [.fullScreenAuxiliary, .moveToActiveSpace]
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
            // A failed start is tunnel_failed followed by a stopped event
            // (app_session.go, tunnel); only a new start clears the reason.
            if tunnelEvent.state == "starting" || tunnelEvent.state == "running" { tunnelError = nil }
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

In `takeDownWindows`, add `tunnel = nil` and `tunnelError = nil` after the recording timer lines, and call `onTunnelChange?()` at the end of the method.

In `DeckWindowController.swift`, add after `revealInFinder`:

```swift
    /// The phone remote panel, made with the window (a panel that is never
    /// shown costs nothing) and closed with it, so none outlives its deck.
    let remotePanel = RemotePanel()
```

In `init`, after the `onQuestion` line, add:

```swift
        remotePanel.onTurnOff = { [weak self] in self?.sessionController.presentation.setTunnel(on: false) }
        sessionController.presentation.onTunnelChange = { [weak self] in self?.refreshRemotePanel() }
```

In `windowWillClose`, add after the preview window lines:

```swift
        remotePanel.orderOut(nil)
        remotePanel.close()
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
        let screenFrame = presentation.presenterWindow?.targetFrame ?? window?.screen?.frame ?? NSScreen.screens[0].frame
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

Required:

```bash
make -C desktop test ONLY=TapTests/PhoneRemoteTests/testPhoneRemote
make -C desktop test ONLY=TapTests/PhoneRemoteTests/testAFailedTunnelStartKeepsItsReason
make -C desktop test ONLY=TapTests/PhoneRemoteTests/testThePanelClosesWithTheDeckWindow
make -C desktop test ONLY=TapTests/PhoneRemoteTests/testWithoutCloudflaredThePanelSaysSo
```

Optional: `testAdvancedRemoteOptions`, `ONLY=TapTests/PresentMenuTests/testPresentingShortcuts`. Expected: all pass. Every tunnel test uses the scripted tap; no real tunnel is ever started.

- [ ] **Step 6: Mutate and commit**

Mutations, each reverted: in `openWindows`, drop the `wantsTunnel` line (expected: `testPhoneRemote` times out on the tunnel command); in `refreshRemotePanel`, never hide the panel (expected: it fails after Turn Off); in `handle`, clear `tunnelError` on every tunnel event (expected: `testAFailedTunnelStartKeepsItsReason` fails on `tunnelError`); in `handle`, drop the tunnel error case (expected: `testWithoutCloudflaredThePanelSaysSo` times out); in `windowWillClose`, drop the panel lines (expected: `testThePanelClosesWithTheDeckWindow` fails on `isVisible`); in `RemotePanel.show`, drop the base64 decode (expected: the image size assertion fails); in `togglePhoneRemote`, always start (expected: `testPhoneRemote` fails on the second toggle's expectation only if extended; leave as noted); in `takeDownWindows`, drop `onTunnelChange?()` (expected: `testPhoneRemote` fails on the panel after the stop).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the phone remote panel with tap's QR code, the tunnel command, and the Advanced options"
```

---

### Task 12: Nothing interrupts the talk: the Focus hint, one talk at a time, and no updates during it

**Files:**
- Modify: `desktop/Tap/App/AppEnvironment.swift` (`focusHint`)
- Modify: `desktop/Tap/Presenting/QuestionSheet.swift` (`focusHint`)
- Modify: `desktop/Tap/Windows/DeckWindowController.swift` (the hint before the first talk, `openFocusSettings`, the Play button following every deck's talk)
- Modify: `desktop/TapTests/Support/HostedTestCase.swift` (a fresh, already-shown hint per test)
- Test: `desktop/TapTests/FocusHintTests.swift`

**Interfaces:**
- Consumes: Task 2's `FocusHintState`; Task 4's `AppEnvironment.presentingCount`, `isPresenting`, `updatesMayInterrupt`, `presentingDidChangeNotification` and the count from `start` to idle or failed; Task 9's `QuestionSheet`, `showQuestionSheet`; Task 6's `startPresenting`, `refreshPresentingControls`.
- Produces: `AppEnvironment.focusHint`; `QuestionSheet.focusHint()`; `DeckWindowController.openFocusSettings`, `focusHintSheet`, the `presentingDidChangeNotification` observer that refreshes Play in every deck.

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
        // The second deck opens on the desktop Space, under nothing; its
        // window is ordered front by openDeck, which switches Spaces away
        // from the talk. Its preview's ready still relies on pull request
        // 27 (a hidden page reports ready) if the window server has not
        // switched yet when the page paints.
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

- [ ] **Step 3: The hint, and Play following every deck's talk**

The talk count itself is Task 4's (`AppEnvironment.presentingCount`, counted in `PresentationController.start` and out in `finishStopping`, `fail` and, as a last guard, `deinit`; `canStart` already reads `isPresenting`). This task adds the hint and the observer that refreshes Play in every deck window when the count changes.

In `AppEnvironment.swift`, add after `presentExecutableURL`:

```swift
    /// Whether the Focus hint has been shown on this Mac. A test replaces it.
    var focusHint = FocusHintState()
```

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

Mutations, each reverted: in `startPresenting`, start the talk on `.OK` too (expected: `testNothingInterruptsTheTalk` fails on `.idle` after Open Focus Settings); drop `hint.markShown()` (expected: it fails on the second start showing a sheet); in `canStart`, drop the `isPresenting` check (expected: it fails on the other deck's Play); in `countOut`, never call `noteTalkEnded` (expected: it fails on `isPresenting` after the stop, and Task 4's class run fails on every later test); in `init`, drop the `presentingDidChangeNotification` observer (expected: it fails on the other deck's `playButton.isEnabled`, since only the notification refreshes a deck that did not start the talk).

Add to the branch ledger: "`AppEnvironment.updatesMayInterrupt` is a flag until D7 wires Sparkle. D7 must read it before any update prompt or restart; `testNothingInterruptsTheTalk` pins the flag, not Sparkle."

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the Focus hint before the first talk, and Play following every deck's talk"
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
        controller.jumpToSlide(number: 1)
        presentation.start(PresentationOptions(mode: .play, startSlide: 3))
        try await waitUntil(timeout: 30, "the talk to fail") { if case .failed = presentation.state { return true } else { return false } }
        guard case .failed(let message) = presentation.state else { return XCTFail() }
        XCTAssertTrue(message.contains("deck not found: talk.md"), "tap's own reason: \(message)")
        XCTAssertEqual(controller.currentSlideNumber, 1, "no window ever showed, so the cursor did not move to the start slide")
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
        controller.jumpToSlide(number: 4)
        presentation.start(PresentationOptions(mode: .play, startSlide: 4))
        try await waitUntil(timeout: 10, "the refusal") { if case .failed = presentation.state { return true } else { return false } }
        guard case .failed(let message) = presentation.state else { return XCTFail() }
        XCTAssertEqual(message, "The deck could not be saved: resolve the change on disk first.", "what to do, not CocoaError's text")
        XCTAssertNil(presentation.session, "tap present never started")
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertFalse(AppEnvironment.shared.isPresenting, "counted out again")
        XCTAssertNotNil(controller.editorViewController.bar(.talkFailed))
        XCTAssertEqual(controller.currentSlideNumber, 4, "no talk ran, so the cursor did not move")
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), "# Theirs\n", "the other program's file is untouched")
    }

    func testPlayIsDisabledWhileADeckHasNoFile() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        XCTAssertTrue(controller.presentation.canStart)
        document.fileWasDeleted()
        XCTAssertFalse(controller.presentation.canStart, "tap present needs a file to read")
        deckWindow.refreshPresentingControls()
        XCTAssertFalse(deckWindow.playButton.isEnabled)
        deckWindow.playButtonClicked(modifiers: [.shift])
        deckWindow.play(nil)
        XCTAssertEqual(controller.presentation.state, .idle)
    }

    func testPlayIsDisabledWhileAnotherDeckPresents() async throws {
        let (_, first) = try await openDeckForPresenting()
        try await startPresenting(first, PresentationOptions(mode: .rehearse, startSlide: 1))
        let (_, second) = try await openDeckForPresenting()
        let secondWindow = try windowController(second)
        XCTAssertFalse(second.presentation.canStart, "one talk at a time, app-wide")
        XCTAssertFalse(secondWindow.playButton.isEnabled)
        secondWindow.play(nil)
        XCTAssertEqual(second.presentation.state, .idle)
        try await stopPresenting(first)
        XCTAssertTrue(second.presentation.canStart)
        XCTAssertTrue(secondWindow.playButton.isEnabled, "the notification reached the other deck")
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
        controller.jumpToSlide(number: 1)
        // The talk is on (the windows show after the fallback, since the fake has no pages) when tap starts dying.
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 3))
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
        XCTAssertFalse(AppEnvironment.shared.isPresenting)
        try await waitUntil(timeout: 10, "nothing left in full screen") { fullScreenPresentationWindows().isEmpty && presentation.windowsGoingDown.isEmpty }
        let bar = try XCTUnwrap(controller.editorViewController.bar(.talkFailed))
        XCTAssertTrue(bar.message.contains("The talk stopped"))
        XCTAssertEqual(controller.currentSlideNumber, 3, "the cursor is on the last slide presented, not where it was")
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
    /// True when the failure came after the windows had been shown: the
    /// talk stopped, rather than never ran.
    private(set) var failedAfterShowing = false
```

In `start(_:)`, add `lastErrorMessage = nil` and `failedAfterShowing = false` after `tunnelError = nil`. (`lastTalkLog` is Task 4's: set at launch, it outlives the session for the Tap Log window and the deck window's own lines.) In `handle(_:)`, add before `default`:

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
        let showed = windowsWereShown
        failedAfterShowing = showed
        fail(reason.map { "\(summary). \($0)" } ?? summary)
        if showed { onStopped?(lastSlide) }
    }
```

Add to `PresentationController`, after `lastTalkLog`:

```swift
    /// A failed talk's log, for Window > Tap Log: the reason is in it. A
    /// talk that ended well is not listed, so the picker goes back to the deck's log.
    var lastTalkLogAfterFailure: TapLog? {
        if case .failed = state { return lastTalkLog }
        return nil
    }
```

In `TapLogWindowController.reload()`, change the talk's log expression to `(talk?.session?.log ?? (talk?.isActive == true ? talk?.lastTalkLog : talk?.lastTalkLogAfterFailure)).map { [$0] } ?? []`.

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

Required:

```bash
make -C desktop test ONLY=TapTests/PresentingFailureTests/testATalkThatCannotStartShowsABar
make -C desktop test ONLY=TapTests/PresentingFailureTests/testATalkThatCannotBeSavedDoesNotStart
make -C desktop test ONLY=TapTests/PresentingFailureTests/testATalkSurvivesATapPresentRestart
make -C desktop test ONLY=TapTests/PresentingFailureTests/testATalkEndsWhenTapPresentKeepsDying
```

Optional: `testPlayIsDisabledWhileADeckHasNoFile`, `testPlayIsDisabledWhileAnotherDeckPresents`, `ONLY=TapTests/PresentingTests/testTheTalksLogIsListedInTheTapLogWindow`. Expected: all pass. `testATalkSurvivesATapPresentRestart` takes a few seconds: D2's policy waits 0.5 s before the first restart, and tap present needs its ready line again; the restarted tap asks for the deck's remembered port, which the dead process has freed.

- [ ] **Step 5: Mutate and commit**

Mutations, each reverted, the ones that can leave a window in full screen or the assertion held first: in `endBecauseTapFailed`, skip `fail` (expected: `testATalkEndsWhenTapPresentKeepsDying` times out with the assertion held); in `fail`, drop `takeDownWindows()` (expected: the dying test fails on `audienceWindow` and on `fullScreenPresentationWindows()`); in `openWindows`, create fresh windows on every ready instead of reusing (expected: the restart test fails on `===`); in `tapIsReady`, refuse `.presenting` (expected: the restart test never reloads); in `endBecauseTapFailed`, call `onStopped` whether or not the windows showed (expected: `testATalkThatCannotStartShowsABar` fails on the cursor, which would move to 3); in `saveFailureMessage`, return the localized description for `.userCancelled` (expected: the save test fails on the message); in `handle`, drop the `.error` case (expected: `testATalkThatCannotStartShowsABar` fails on the message); in `showTalkFailed`, drop the Dismiss button (expected: it fails on the unwrap); in `canStart`, drop the `deckURL() != nil` check (expected: `testPlayIsDisabledWhileADeckHasNoFile` fails); in `canStart`, drop the `isPresenting` check (expected: `testPlayIsDisabledWhileAnotherDeckPresents` fails).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): a talk that cannot start or stops restarting says why, and one that loses tap present comes back"
```

---

### Task 14: The scenario manifest, the UI tests, the config seam for them, and the README

**Files:**
- Modify: `desktop/scenarios.txt`, `desktop/README.md`
- Modify: `desktop/Makefile` (`test-build`, `bench-build`)
- Modify: `.github/workflows/ci.yml` (the desktop job's timeout)
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

- [ ] **Step 2: The build targets and the config seam**

In `desktop/Makefile`, add `test-build` and `bench-build` to `.PHONY` and these targets after `bench`:

```make
# Compiles the hosted and UI tests without running them; the person runs uitest.
test-build: project
	$(XCODEBUILD) -scheme Tap -destination 'platform=macOS' build-for-testing

bench-build: project
	$(XCODEBUILD) -scheme TapBenchmarks -destination 'platform=macOS' build-for-testing
```

Both depend on `project`, so xcodegen runs first and a new test file cannot be left out of a stale `Tap.xcodeproj`.

In `.github/workflows/ci.yml`, in the `test-desktop` job (`name: Desktop Tests`, `runs-on: macos-15`), change `timeout-minutes: 30` to `timeout-minutes: 60`. Why 60: the hosted step takes about 3.5 minutes today; D4 adds about 65 hosted tests, each of which opens a deck, starts a real or scripted tap present, runs up to four full screen transitions of about a second each, and tears down, with the fakes that ignore `quit` waiting out the 15 s deadline, so 15 to 25 s each, or 16 to 27 minutes more, before a cold DerivedData build of a few minutes. Task 4's class run put its wall time in the ledger; if `PresentingTests` (thirteen tests) took more than 5 minutes there, split the presenting classes into a second `test-desktop-presenting` job with the same steps and `-only-testing:TapTests/Presenting*` rather than raising the number again.

In `AppEnvironment.init`, add after the `loginShellLoader = ...` line (the `tapExecutableURL` assignment sits in two branches; this line has one place):

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
        // Option-Tab shows the presenter window over the audience window on one display, and hides it again.
        application.typeKey("\t", modifierFlags: [.option])
        XCTAssertTrue(application.windows["presenter-window"].waitForExistence(timeout: 5))
        Thread.sleep(forTimeInterval: 1)
        application.typeKey("\t", modifierFlags: [.option])
        Thread.sleep(forTimeInterval: 1)
        XCTAssertFalse(application.windows["presenter-window"].exists)
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
        // The toolbar slides up when the pointer reaches the bottom edge.
        presenter.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 1.0)).hover()
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

Run: `make -C desktop test-build 2>&1 | tee /tmp/tap-test-build.log | tail -3`, then `grep -c "PresentingUITests.swift" /tmp/tap-test-build.log`
Expected: `** TEST BUILD SUCCEEDED **`, nothing launched, and the grep count is at least 1: the generated project compiled the new file (D3's Critical was a stale project that built "successfully" without one). The XCUI element types (`windows`, `buttons`) follow the accessibility roles the views expose; a full screen window is still a window to XCUI, and the person's first run settles them, with any that need changing going in the ledger.

- [ ] **Step 4: The README**

Add to `desktop/README.md` under Test, after the thumbnail paragraph:

```markdown
The presenting tests run a real `tap present --app` beside the deck's
`tap dev --app` and put the audience window into system full screen for a
few seconds each, with AppKit's own animation, where the host can: a probe
(`FullScreenProbe`, once per process) tries it first, and on a host that
cannot the full screen assertions skip with the probe's reason while the
talk runs as plain windows and everything else is checked. The
`FullScreenSpikeTests` record what this host does, for the ledger. The
tests answer tap's recording question ahead of time in the test's own
settings folder, so nothing is ever recorded, and every tunnel test
drives a scripted tap, so no tunnel is ever started. Two displays are
stood in for by the two halves of the one screen
(`PresentingTestCase.halfScreens()`); on one screen the two windows
become two full screen Spaces of that screen, and the tests check the
frame each window was asked for and its full screen state, never its
frame. `make -C desktop test-build` compiles the UI tests without running
them.

What only a person can check, with a projector plugged in: the audience
Space on the projector and the presenter Space on the laptop, Swap
Displays moving them across displays, the projector unplugged mid-talk
(the presenter view comes over the audience view) and plugged back in
(the audience goes back to the projector, the presenter view gets its
Space again), Cmd-Tab to another app and back, the F key's element full
screen in either page, the toolbar sliding up from the bottom edge and the
menu bar dropping over the top edge without covering it, "Displays have
separate Spaces" turned off (the popover's note, plain windows instead of
Spaces), a real recording with Screen Recording permission (REC in the
toolbar, the keep-recording sheet at Stop, Delete still deleting after a
20 s pause, the run in Finder), a second talk on the same deck keeping the
presenter layout and notes size (the deck's port), and Phone remote with
cloudflared installed (the QR code from tap, a phone driving the deck).
`make -C desktop uitest` runs the two presenting UI tests on the real
screen; they are what proves real full screen on a machine whose hosted
tests skip it.
```

- [ ] **Step 5: Commit**

```bash
git add desktop/scenarios.txt desktop/README.md desktop/Makefile desktop/Tap/App/AppEnvironment.swift desktop/TapUITests .github/workflows/ci.yml
git commit -m "test(desktop): claim the D4 scenarios, add the presenting UI tests, the test-build target, the README's manual pass and a longer CI timeout"
```

---

## Final check

- [ ] Run: `make -C desktop core-test`
  Expected: every `TapDesktopCore` test passes, including the new `PresentingCoreTests` and the extended `TapProtocolTests`, `TapSessionTests` and `TapClientTests`.
- [ ] Run: `make -C desktop check-scenarios`
  Expected: `every claimed scenario has a test`.
- [ ] Push the branch and read CI's `Desktop Tests` job: every hosted test green on the runner, the D2 and D3 tests included. The hosted bundle is not run locally in full (branch rule). The presenting tests cover the runner's screen; nothing there minds.
- [ ] Run: `make -C desktop test-build 2>&1 | tee /tmp/tap-test-build.log | tail -3 && grep -c PresentingUITests.swift /tmp/tap-test-build.log`, then `make -C desktop bench-build | tail -3`
  Expected: `** TEST BUILD SUCCEEDED **` twice, the grep count at least 1 (the regenerated project compiled the UI test file), nothing launched. `make -C desktop uitest` and `make -C desktop bench` are the person's runs. No step on this branch runs `xcodebuild` by hand.
- [ ] Run: `grep -rn "toggleFullScreen\|canJoinAllSpaces\|NSWindow.Level.mainMenu" desktop/Tap`
  Expected: `toggleFullScreen` in `PresentationWindow.toggle()`, in `PresentationWindow.validateUserInterfaceItem` (which refuses it) and in D3's View > Enter Full Screen item in `MainMenu.swift`, and nowhere else; no `canJoinAllSpaces` in `desktop/Tap/Presenting` (the remote panel is `.fullScreenAuxiliary` only); no covering level anywhere. System full screen is the one way a talk window takes a display.
- [ ] Run: `grep -rn "$(printf '\342\200\224')" desktop/ docs/superpowers/plans/2026-09-24-desktop-presenting.md --include=*.swift --include=*.md --include=*.yml --include=*.sh --include=*.txt`
  Expected: no output.
- [ ] Run: `grep -rn "evaluateJavaScript\|callAsyncJavaScript" desktop/Tap`
  Expected: only D2's two, `PreviewViewController.pageText` and `pageValue`. This plan's `pageText()` and `pressKey(_:)` live in `desktop/TapTests/Support/PresentationPageController+Tests.swift` and never reach the binary; `grep -rn "evaluateJavaScript" desktop/TapTests/Support/PresentationPageController+Tests.swift` finds them there.
- [ ] Run: `grep -rn "runModal\|NSAlert" desktop/Tap`
  Expected: no output. Every question is a sheet.
- [ ] Run: `grep -rn "NSApp.activate\|activate(ignoringOtherApps" desktop/Tap`
  Expected: no output.
- [ ] Run: `grep -rn "orderFrontRegardless\|makeKeyAndOrderFront" desktop/Tap`
  Expected: exactly the Global Constraints list ("No production code steals focus"): `PresentationWindow.present(on:)` and `settle()` (`orderFrontRegardless` before an entry), `PresentationWindow.attach(to:)` (`addChildWindow`, which orders the child front), `PresentationController.showWindows` through `placeNext`'s completion, `toggleFrontWindow`, `bringPresenterWindowForward`, `showPresenterOverAudience`, `returnToTalk`, `moveWindows`'s two completions, `RemotePanel.show`, `DeckWindowController.showQuestionSheet`, and D2's and D3's own lines (`showPreviewInWindow`, `bringDeckWindowForward`). Add `makeKey\b` to the grep: `toggleFrontWindow` and `showPresenterOverAudience` use it. `activateFileViewerSelecting` (`revealInFinder`) and the Focus Settings URL (`openFocusSettings`) are the two that leave the app, both on a button the person pressed. Anything else is a defect.
- [ ] Run: `grep -rn "sleepAssertion.release\|sleepAssertion.acquire" desktop/Tap`
  Expected: `acquire` only in `openWindows`; `release` only in `takeDownWindows`.
- [ ] Run: `grep -rnE "completion\?\([^)]" desktop/Tap`
  Expected: no output (D3 ledger lesson: an optional call skips its arguments when the closure is nil). The bare `completion?()` calls in `PresentationWindow` and `PresentationController.placeNext` carry no arguments and no side effects inside the call, so the pattern leaves them out on purpose.
- [ ] Run: `grep -rn "unowned" desktop/Tap desktop/TapDesktopCore/Sources`
  Expected: no output.
- [ ] Run: `grep -rn "updateChangeCount" desktop/Tap`
  Expected: only `DeckSessionController.refreshEditedState`.
- [ ] Run: `pgrep -fl "tap present"` after the hosted tests
  Expected: no output; every talk's process is gone.
- [ ] Open the app by hand: `open "desktop/$(make -s -C desktop app-path)"`, then open `examples/basic.md`. Check: the Play button opens the popover; Start Presenting puts the audience page in its own full screen Space on the cursor's slide, with the menu bar and Dock out of the way; Option-Tab shows the presenter view over it with no animation, with the timer, and Option-Tab hides it again; Cmd-Tab to Terminal and back leaves the talk where it was; the pointer at the bottom edge slides the toolbar up, and the pointer at the top edge drops the menu bar and nothing else; Escape ends the talk, no window is left in full screen and the cursor is on the last slide; Cmd+Option+P starts again at once with the same settings; Cmd+Option+Shift+P rehearses; Window > Tap Log lists "basic, talk" during a talk and, on the second talk, `--port <n>` with the same `n`; `pgrep -fl "tap present"` prints nothing afterwards; `pmset -g assertions | grep "Tap is presenting"` prints nothing afterwards.
- [ ] Every part of the D4 outline maps to a task:

  | D4 outline item | Task |
  |---|---|
  | Play and Rehearse through `tap present --app` | 1, 4, 5 |
  | The popover | 6 |
  | Display arrangement and swap | 2, 5 |
  | Full screen audience window, presenter window (system full screen Spaces) | 3, 4, 5, 7 |
  | The presenter toolbar | 8 |
  | The consent and keep-recording sheets from stdout questions | 9, 10 |
  | The sleep assertion | 3, 4 |
  | Phone remote, Advanced (05) | 11 |
  | Nothing interrupts the talk (05) | 12 |
  | Presenting shortcuts (12) | 7 |
  | The scenario manifest, the UI tests, the README | 14 |

## Pre-flight: conflicts found, rulings and what each costs if wrong

The person's decisions of 2026-09-24 (1 to 4) and 2026-09-25 (5 to 8) settle the items marked; the rest are the plan's own rulings, collected again as product decisions under "Open questions".

1. **`--tunnel` for `tap present` (05, Advanced remote options) does not exist.** Ruling: the code wins; the app sends `{"type":"tunnel","start":true}` after the ready line, on Play with Phone remote or Public tunnel on, and again after a restart. `testAdvancedRemoteOptions` asserts no `--tunnel` in the arguments. Cost if wrong: if P6 later adds the flag, one line in `Command.arguments` and the `setTunnel` call go.
2. **System full screen, not cover windows (decision 1), one Space per window on two displays and one Space with the presenter view as a child window on one (decision 5).** Cmd-Tab to a demo app, the menu bar at the top edge and the page's F key work as in a browser. What it costs: about a second of animation on entry and exit, a swap or an unplugged-then-replugged projector going exit, move, enter. On one display Option-Tab and the S key attach the presenter window over the audience window inside its Space, with no animation, so a laptop mirrored to a projector never shows a Space switch. Every ending exits full screen before closing, one window at a time, and closes anyway after `PresentationWindow.exitTimeout`. With "Displays have separate Spaces" off, a two-display talk cannot use full screen at all (one Space blacks out the other display), so it runs as plain windows with a note in the popover (open question 2).
3. **Cmd+Option+P starts at once with the last settings; the Play button opens the popover (decision 4); the settings persist app-wide across launches, the password never stored (decision 8).** The spec's design section and feature 12 are edited on this branch to say so. Present > Play with Options… is the menu item for the popover, so every toolbar action still has a menu item. The settings are read fresh on every start, and the popover's context (the cursor, the displays) is refreshed with them, so a second deck's Cmd+Option+P starts with the newest saved choice from its own cursor. Cost if wrong: one line in `play(_:)`.
4. **Sheets on the deck window while the talk windows are up.** The requirement is the deck window. Ruling: the windows do not go up until any startup question is answered (the consent arrives within milliseconds of ready, before a page can load); a question during the talk brings the deck window forward, which on two displays switches to its Space and on one display switches away from the audience's Space (the one Space switch a one-display talk can see), and the answer makes the talk's front window key, which switches back. Cost if wrong: `returnToTalk` goes, and `showQuestionSheet` takes the presenter window as the sheet's parent.
5. **The keep-recording answer has 60 seconds (decision 3, tap pull request 35).** tap waits while stdin is open, up to 60 s, then keeps and exits; a closed stdin keeps at once. The app extends its quit deadline to 75 s when the question arrives, so it never closes stdin under the person; a tap that keeps and exits first ends the sheet as Keep and reveals the run. Escape on the sheet does nothing (Critical 2 in the first review). A deck that closed mid-talk has no window for the sheet, so the app answers keep at once, with a log line (Important 6 in the re-review). Cost if wrong: a person who takes more than a minute keeps a run they wanted gone, and finds it in Finder.
6. **Escape in the audience window ends the talk, whatever the page does with Escape.** The page uses Escape to close its help overlay and leave overview; the spec makes Escape Stop. Ruling: Escape in the audience window stops, in the presenter window it goes to the page unless there is no audience window (a rehearsal), where it stops. Cost if wrong: one branch in `handleKey`.
7. **Both pages hold the presenter cookie, and the presenter page brings its own key, percent-encoded.** With a presenter password set (app mode always has one), the hub relays only from connections that carry the cookie; without it, the speaker's arrow keys in the audience window would move that page alone, the presenter view would not follow, and tap would emit no `slide` events or chapters. Ruling: the app trades the secret for the cookie and sets it into the talk pages' data store for the audience page, and loads the presenter page as `/presenter?key=<secret>#<slide>`, which the server answers by setting the cookie and redirecting with the hash kept, so a 403 never lands in a full screen window. The key is built with `URLComponents` and a plus encoded, since the person's own password may hold `#`, `&`, `+` or `%`. The password still goes to tap in argv (`--presenter-password`), which `ps` can read; only a tap change (reading it from stdin or the environment) removes that, and the Tap Log hides it. Cost if wrong: none that the tests would not show at once.
8. **One port per deck (decision 2), a suggested first port, and the fallback mid-talk too.** `tap present --app` without `--port` binds a new free port every run, and WebKit keys the presenter page's `localStorage` (its layout and notes size) by origin, port included, so the spec's "persist between launches" needs one origin per deck. Ruling: a deck's first talk asks for `DeckPortStore.suggestedPort`, one of 20000 to 29999 from the deck's path (a port tap picked itself would sit in the ephemeral range and be taken now and then); the port tap reports is remembered and passed with `--port` from then on; a taken port (tap's `failed` error, "port n is already in use (another tap dev may be running)"), at the start or on a restart mid-talk, makes the app stop that attempt before D2's policy restarts it and start again with no port, remembering the new one. How the fallback shows: the presenter view's default layout and notes size for that one talk, a line in the talk's log, nothing on screen. Cost if wrong: if the person would rather be told, one bar on the deck window.
9. **"Record the talk" in the popover.** tap decides whether to record from `present.record` in `settings.yaml`, and the app decides nothing about recording. Ruling: the checkbox on (the default) passes no flag and leaves the decision to tap; off passes `--no-record` for this run, which is tap's own way to skip one talk. Cost if wrong: one checkbox.
10. **What "how many edits are not shown yet" counts.** Ruling: distinct texts tap dev has answered for since tap present last read the file (an answer for the same text again, after a tap dev restart or a component change, counts nothing), and zero again when the text is back to what was presented. Reload Slides saves and sends `reload`; the text counts as presented only once it is on disk. Cost if wrong: a label's number.
11. **The Focus hint's Open Focus Settings does not start the talk.** A talk would cover System Settings. Ruling: the hint is marked shown either way; Not Now starts, Open Focus Settings opens the pane and leaves the person to press Play again. Cost if wrong: one branch in `startPresenting`.
12. **The live code approval question during a talk (D5).** tap asks it after the consent when the deck declares drivers. Ruling: declined with a log line until D5, which runs no code; questions queue, so an approval arriving over a consent sheet waits its turn. Cost if wrong: none; D5 replaces the `default` case in `presentQuestion`.
13. **tap present dies mid-talk.** The spec's restart rule is written for tap dev. Ruling: the same policy; the windows stay where they are with the last render, the next ready reloads both pages at the last slide on the deck's port (or a fresh one if the deck's was taken in between), the assertion stays held, and after three exits in thirty seconds the talk ends with a bar. tap's recording, if any, is a new run after the restart; the old one is whatever tap's exit path left. Cost if wrong: a speaker sees a reload instead of a frozen page; if a frozen page is preferred, `openWindows` skips the reload on a restart.
14. **App quit during a talk keeps the recording without asking.** `applicationWillTerminate` stops every talk, and tap keeps a recording when its stdin closes with no answer (unchanged by pull request 35). The process ends before the windows' exits complete; the Spaces go with it, which is why the design spec's sentence on leaving full screen names quit as the exception. Ruling: no sheet on quit. Cost if wrong: a run the person wanted deleted is on disk.
15. **One talk at a time, app-wide, and a stopping talk outlives its deck.** Two talks would fight over the displays. Ruling: `canStart` reads `AppEnvironment.isPresenting`, counted from `start` (so a second deck cannot slip in during the save; a Stop then Play during the save starts one talk, not two) to idle or failed; a deck that closes mid-talk hands the talk to `AppEnvironment.endingTalks` until its process has exited (Critical 1 in the first review). Cost if wrong: one check and one array.
16. **The sheet copy.** The consent title is the spec's, "Record automatically every time you present?"; the bodies follow the mockups, with the settings path from tap's payload in place of the mockup's fixed line. The keep-recording body has the segments and the folder's size; the mockup's minutes and start time are not in tap's payload. Cost if wrong: copy.
17. **`-FocusHintShown` and `-TapConfigHome` for UI tests.** The hint and the person's real settings would otherwise stand between a UI test and the talk. Cost if wrong: two lines.
18. **Window > Tap Log lists the talk's log** ("<deck>, talk") beside the deck's while it runs, and keeps a failed talk's log until the next talk. The log line hides the presenter password. The deck window's own lines about a talk ("tap kept the recording") go to the talk's log, not tap dev's. Cost if wrong: one `flatMap`.
19. **The recording clock counts up in the app between events.** tap sends a recording event only on a change, with `elapsed` as of that moment. Cost if wrong: a label a second out.
20. **The full screen spike, the probe and the skips (decision 6).** Whether a hosted test host can enter system full screen, and whether a second window on the same screen gets a Space of its own, is found out by Task 3's Step 0 spike on the local machine and on CI before any presenting code, and recorded in the ledger. `FullScreenProbe` decides once per process from what it observes (never an environment variable); `requireFullScreen()` and `requireSecondSpace()` skip a test with the reason; the talk windows in the tests are plain windows where the probe says no (`fullScreenAllowed`); the state machine is covered through `requestFullScreenToggle` and the delegate methods; the person's UI tests prove real full screen. Cost if wrong: a skipped test on a host that could have run it, which the ledger's spike record shows.
21. **The presenter toolbar slides up from the bottom edge (decision 7), after a mockup and sign-off.** In system full screen the pointer at the top edge drops the menu bar and the window's hidden title bar over the content, where a top toolbar would sit. Ruling: the toolbar is pinned to the bottom, `point.y <= 2` reveals it, a move above its band hides it; the REC dot stays top-right. Task 8's Step 0 shows the person a mockup and the ledger records the sign-off before the UI code is written. Cost if wrong: two constraints and two comparisons.
22. **View > Enter Full Screen never reaches a talk window.** `PresentationWindow.validateUserInterfaceItem` refuses `toggleFullScreen(_:)`, since a window taken out of full screen from under the controller would stay a plain window. Cost if wrong: one override.
23. **The talk pages' data store is a seam.** Production uses `WKWebsiteDataStore.default()` (the presenter layout must persist); the hosted tests, which run inside the real Tap.app, use a store of their own (`WKWebsiteDataStore(forIdentifier:)`, macOS 14) so a test's localStorage and presenter cookie never land in the person's. Cost if wrong: one property.

## Open questions

Decided and closed: system full screen (1), a fixed port per deck (2), the 60 s keep-recording wait (3, merged as tap pull request 35), Cmd+Option+P starting at once (4), the one-display presenter view as a child window (5), the full screen spike and the skips (6), the toolbar at the bottom edge (7), the popover's settings kept app-wide with the password never stored (8). What remains open is the list below: product decisions this plan makes that the spec leaves open. Each line is the default the plan implements and what it costs if the person wants it otherwise, so the list can be scanned and any line changed before the plan runs.

1. **Play with Options… is the popover's menu item, with no shortcut.** Default: as named. Cost if wrong: a title, or a key.
2. **Two displays without "Displays have separate Spaces" run as plain windows, with a note in the popover.** Default: the talk still starts; the note names the setting. Alternative: refuse to start until the setting is on. Cost if wrong: one guard in `startPresenting` and the note's wording.
3. **A mid-talk question brings the deck window forward and the answer brings the talk back.** Default: as pre-flight 4. Alternative: the presenter window as the sheet's parent, which on one display would keep the audience's Space active. Cost if wrong: one parent change.
4. **Escape in the presenter window goes to the page, except in a rehearsal.** Default: as pre-flight 6. Cost if wrong: one branch.
5. **Escape on the keep-recording sheet does nothing.** Default: no key reaches Delete, and Escape is not Keep either, so a stray key neither deletes nor opens Finder. Alternative: Escape as Keep. Cost if wrong: `escape: .decline` becomes a third case.
6. **"Record the talk" on leaves recording to tap's consent; off passes `--no-record`.** With `present.record: false` saved, the box still reads on while nothing records. Default: as pre-flight 9. Cost if wrong: the box reads tap's setting, which the app would have to parse from `settings.yaml`.
7. **The edits counter counts distinct texts since the last reload.** Default: as pre-flight 10. Cost if wrong: a number.
8. **Open Focus Settings does not start the talk, and the hint shows before a first Rehearse too.** The spec says "the first time I present". Default: before the first talk of either kind. Cost if wrong: one `mode` check.
9. **Live code approval is declined until D5, so live code stays off in D4 talks.** Default: as pre-flight 12. Cost if wrong: none.
10. **tap present dying mid-talk restarts and reloads at the last slide, and ends the talk after three exits.** Default: as pre-flight 13. Cost if wrong: a frozen page instead of a reload is one skipped call.
11. **App quit keeps any recording without asking.** Default: as pre-flight 14. Cost if wrong: a sheet on quit, and `applicationShouldTerminate` waiting for it.
12. **One talk at a time, app-wide.** Default: as pre-flight 15. Cost if wrong: two talks fighting over the displays; the count becomes per deck.
13. **The sheet copy follows the mockups, with the spec's consent title, and the keep sheet's buttons are "Delete" and "Keep and Show in Finder".** Default: as pre-flight 16. Cost if wrong: copy.
14. **A tap that keeps and exits before an answer ends the sheet as Keep and opens Finder.** Default: as pre-flight 5. Alternative: the sheet stays with "tap kept the recording" and one OK button. Cost if wrong: one branch in `talkEnded`.
15. **A deck closed mid-talk answers keep-recording with keep, at once.** Default: tap's own default, since no window is left for a sheet. Alternative: leave the question to tap's 60 s ceiling, which keeps Play off in every deck for that minute. Cost if wrong: one closure.
16. **The shortcuts and timings: Stop Cmd+., Reload Slides Cmd+R, a Phone Remote menu item, Option-Tab, a 3 s cursor hide, a 1.5 s toolbar hide.** Default: as named. Cost if wrong: constants.
17. **Rehearse from the menu starts at the cursor's slide; the popover offers the cursor's slide or slide 1.** Default: as named. Cost if wrong: one argument.
18. **The display memory is keyed by display names, so two monitors of the same model share one key.** Default: names, since that is what the person sees in the popover. Cost if wrong: the key takes the display ID as well.
19. **A taken port falls back silently, with a log line; a deck's first port comes from its path.** Default: as pre-flight 8. Alternative: a bar on the deck window saying the presenter layout starts fresh. Cost if wrong: one bar, or one constant range.
20. **There is no "starting" indicator: a slow or hung tap present shows nothing for up to about 60 s (3 ready timeouts).** Default: the Play button is off and nothing else. Cost if wrong: a spinner in the toolbar item.
21. **AppKit sheets rather than SwiftUI.** The spec allows SwiftUI "only for sheets and settings" but does not require it. Default: AppKit stack views, so the hosted tests can drive their buttons. Cost if wrong: three views rewritten; D6's Settings window can still be SwiftUI.
22. **Presenting keeps the persistent `.default()` data store, shared with the preview.** In hosted tests a store of the test's own. Default: shared in production. Cost if wrong: a store per deck, keyed like the port.
23. **The person's runs.** `make -C desktop uitest` (two presenting UI tests on the real screen) and the manual pass in the README (a projector, the F key, Cmd-Tab, the Spaces setting, the bottom-edge toolbar, a real recording, the port across talks, cloudflared) are compiled and written by the agents and run by the person, as in D2 and D3, plus the spike's CI run from a scratch branch and the ledger line for the toolbar mockup's sign-off. `make -C desktop bench` gains nothing in D4. This is process, not a spec question.

## What this plan found missing in the spec and in P6

- P6 has no `--tunnel` for `tap present`; the app uses the stdin command (pre-flight 1).
- P6's keep-recording wait was three seconds, sized for a machine, not a person; tap pull request 35 (a569901) made it 60 seconds while the app is connected, which this plan depends on (pre-flight 5).
- P6 binds a new port per run, and the spec's "the presenter layout and notes size persist between launches" needs one origin per deck; the app's remembered `--port` supplies it (pre-flight 8).
- P6 takes the presenter password in argv only, where `ps` can read it; the Tap Log hides it, and a tap change (stdin or the environment) would be needed to hide it from the process list (pre-flight 7).
- No event says "the startup questions are done"; the app infers it from the first page's ready and the question queue. D5's approval question, which comes after the consent, queues behind it.
- The prerequisites document does not say that the pages need the presenter cookie to be relayed, or that cookies ignore ports, or that `/presenter?key=` keeps the fragment (pre-flight 7); the code's comments do (`app_events.go`, `app_auth.go`, `routes.go`).
- The spec says "goes full screen" without saying system full screen, and "Option-Tab switches to the presenter window" without saying how; decisions 1 and 5 make it system full screen with the one-display presenter view as a child window, and the design spec's Presenting section on this branch says so. The spec's "slides in when the pointer reaches the top edge" met the menu bar that full screen drops there; decision 7 moves the toolbar to the bottom edge, and the spec says that too. The spec does not mention "Displays have separate Spaces", which system full screen needs on two displays (open question 2). Feature 05's "the app starts `tap present --app talk.md`" now describes only the shape of the command; every talk carries `--port <n>`.
- The spec does not say what happens when tap present dies mid-talk, when the projector is unplugged, when two decks try to present, what Escape does in the presenter window, what a second Escape does at the keep-recording sheet, or what happens to a question whose deck window has closed; pre-flights 13, 2, 15, 6, 5 and 5 decide.
- The design's "SwiftUI is used only for sheets and settings" is not taken up here: the sheets are AppKit stack views, as every D2 and D3 view is, so the hosted tests can drive their buttons directly. D6's Settings window can still be SwiftUI.

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-24-desktop-presenting.md`, revised on 2026-09-25 after its two reviews (`d4-plan-review.md` and `d4-plan-rereview.md`, untracked) and the person's eight decisions. Execute with superpowers:subagent-driven-development, one fresh subagent per task, as the roadmap requires, on a branch cut from `main` at or after a569901 (tap pull request 35). Task 3's Step 0 spike runs first, locally and on CI, and its result goes in the ledger before Task 3's Step 1. Task 8's Step 0 needs the person's sign-off on the toolbar mockup in the ledger before its UI code. Tasks 1 and 2 are the core package and need no Xcode project; Task 3 onward touch the app target and its hosted tests, one at a time locally (Task 4's class run excepted), the bundle on CI. Every task's review runs the mutations its last step lists, the ones that can leave a window in full screen, a talk stuck or the sleep assertion held first.

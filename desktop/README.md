# Tap Desktop

The native macOS app for writing and presenting tap decks. The design is in
`docs/superpowers/specs/2026-09-22-tap-desktop-design.md`.

## Build

You need Xcode 26, Go, Node and XcodeGen (`brew install xcodegen`).

```sh
make frontend             # once, from the repo root: the app bundles tap
make -C desktop build     # generates Tap.xcodeproj and builds Tap.app
open "desktop/$(make -s -C desktop app-path)"
```

`desktop/project.yml` is the project. `Tap.xcodeproj` is generated and not
committed; run `make -C desktop project` after changing `project.yml` or
adding files.

## Test

```sh
make -C desktop core-test   # the TapDesktopCore package, no Xcode project
make -C desktop test        # tests hosted in Tap.app, with the bundled tap
make -C desktop uitest      # UI tests: takes over the screen, so it runs on CI, not here
make -C desktop test-build  # compiles the hosted and UI tests without running them
make -C desktop bench       # the 200-slide typing and preview benchmarks
make -C desktop check-scenarios
```

The slide operation tests round-trip every result through the bundled
`tap slide list --json` (`TapTests/Support/TapSlideList.swift`), so the
slide order and the separators are checked by tap, never by Swift. The
thumbnail tests need the deck window on screen: the hidden renderer only
paints while the window is visible. Thumbnails are cached under
`~/Library/Application Support/Tap/Thumbnails`; the tests use a temporary
folder instead.

A header drag in the editor leaves a text selection elsewhere untouched:
`SlideDragUITests.testDraggingABoxHeaderLeavesTheSelectedTextInPlace`
checks it with a real drag. `NSTextView`'s end-of-move handling deletes
only the ranges of a text drag it started itself, and a header drag
starts none, so no override is needed.

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

What only a person can check, with a projector plugged in as a second
display (two displays are otherwise covered only by seam tests): the
audience Space on the projector, Swap Displays moving the windows across
displays, Cmd-Tab to a demo app during a talk, the page's own F-key full
screen, a real recording made with Screen Recording permission, the
phone remote with cloudflared installed, and the "Displays have separate
Spaces" setting turned off. The UI tests and benchmarks run on CI (the
Desktop UI Tests and Desktop Benchmarks jobs), never on the person's own
machine, since they take over the screen.

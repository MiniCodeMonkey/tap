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
make -C desktop uitest      # UI tests, local only
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

One behavior no automated test reaches: drag a slide's header in the
editor while some text elsewhere in the editor is selected. The drag must
move the slide and leave that selection untouched.
`EditorTextView.draggingSession(_:endedAt:operation:)` skips
`NSTextView`'s own end-of-move deletion for slide drags; check it by hand
after any change near `EditorTextView`'s drag handling.

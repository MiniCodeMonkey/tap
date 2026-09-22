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

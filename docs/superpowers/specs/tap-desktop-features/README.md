# Tap Desktop: feature specification

These feature files describe every behavior of Tap Desktop as scenarios. They are the functional inventory for the app and the acceptance criteria for its implementation. The design doc is `../2026-09-22-tap-desktop-design.md`.

## The rule these files enforce

The desktop app never implements deck behavior itself. It is a window, an editor, and an input layer on top of the `tap` binary it bundles.

- Anything that parses, renders, validates, runs, presents, records, or exports a deck is done by `tap`. The app calls the bundled binary or its `--app` server.
- Anything tap already does that changes files (new deck, theme, AI images, components) is done by calling tap, never by a second implementation in Swift.
- Editing the text of a deck belongs to the app. This includes structural edits the CLI does not need, such as moving, duplicating, and deleting slides. The app does them on its buffer, using the slide ranges that tap reports, so slide boundaries are never computed in Swift.
- The app also owns what is native: windows and tabs, menus, drag and drop, undo, the Keychain, display selection, and permission prompts.
- When a scenario needs something tap does not offer yet, the step marks it `NEW`.
- Presenting from the app is `tap present`, and writing is `tap dev`. The app adds editing, not a second presentation engine.

## Conventions in these files

- Slide numbers are 1-based, as the user sees them.
- `Then tap …` steps describe what the bundled tap does. `Then the app …` steps describe native behavior.
- Every question raised while writing these files has been answered and turned into scenarios.

## Files

| File | Area |
|---|---|
| `01-documents.feature` | Opening, saving, autosave, tabs, external changes |
| `02-slide-structure.feature` | Slide boxes, deck settings, separators, the current slide |
| `03-slide-operations.feature` | Insert, move, duplicate, delete, multi-select, undo |
| `04-preview.feature` | Following the cursor, pinning, steps |
| `05-presenting.feature` | Displays, start and stop, presenter view, remote, recording |
| `06-live-code-and-trust.feature` | Live code approval, drivers, secrets |
| `07-tap-process.feature` | Starting tap, crashes, environment, security |
| `08-creating-decks.feature` | New deck, templates, themes |
| `09-images-and-components.feature` | Pasting images, AI images, components |
| `10-export.feature` | PDF, static build, screenshots |
| `11-settings-and-cli.feature` | Settings window, Install tap command |
| `12-menus-and-shortcuts.feature` | Menu bar, context menus, keyboard |
| `13-performance.feature` | Performance targets |

`cli-review.md` covers the consistency review of the existing `tap` commands and the new ones these files need.

## Baseline facts from the current code

- `tap present` does not exist yet. Recording by segment and following the projector display exist as library code with no command.
- Live code execution is built but not connected: no production code calls `Server.SetRegistry`, so `/api/execute` returns 500 "Driver registry not configured".
- Live code blocks are read-only. The Run button sends the block's original source.
- Several `tap dev` features exist only as TUI keys, with no flag or API: recording (`c`), toggling the tunnel mid-session (`u`), AI image generation (`i`), and the theme picker that writes the deck (`t`).
- `tap add` always appends a slide at the end of the file.
- The frontend has no single "slide finished rendering" signal. `tap pdf` combines several checks.

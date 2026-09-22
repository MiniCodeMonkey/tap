# Tap Desktop: design

Date: 2026-09-22
Status: draft, rewritten after the feature review, awaiting approval

Related documents:

- Behavior, scenario by scenario: `tap-desktop-features/` (13 feature files and a README).
- The tap work this app depends on: `2026-09-22-tap-desktop-prerequisites-design.md`.
- Mockups: the "Tap Desktop Mockups" canvas (17 screens).

## Summary

Tap Desktop is a native macOS app for writing and presenting tap decks. It is a markdown editor in the spirit of Clearly, with one difference: it knows what a slide is. Each slide is a box inside one scrolling editor. A sidebar of thumbnails reorders slides by dragging. A preview follows the cursor. Play presents the talk.

The `.md` file on disk is the only source of truth. The app never writes a private format, so git diffs stay clean and the CLI works on the same file.

## The rule: tap does the work, the app edits

- `tap` does anything that parses, renders, validates, runs, presents, records, or exports a deck. The app calls the tap binary inside its bundle, either as a subcommand or through the `--app` server.
- `tap` also does every file change it already knows how to make: a new deck, the theme, AI images, and components. The app never has a second copy of that logic in Swift.
- The app owns editing the text, including structural edits the CLI does not need: moving, duplicating, and deleting slides. It edits its own buffer with the slide ranges that tap reports, so Swift never works out where a slide begins or ends.
- The app owns what is native: windows and tabs, menus, drag and drop, undo, the Keychain, display selection, and native sheets for the questions tap asks.
- Writing uses `tap dev`, and presenting uses `tap present`. The app adds editing. It does not add a second presentation engine.

## Goals

- Edit a deck slide by slide without hunting for `---` lines.
- Move, duplicate, delete, and skip slides, including several at once, as single undo steps.
- See the slide under the cursor rendered exactly as tap renders it.
- Present and rehearse from the app with everything `tap present` does, including recording.
- Reach every CLI feature from the app.
- Stay fast on a 200-slide deck with a component on every slide.

## Non-goals

- Windows or Linux.
- Mac App Store distribution. The sandbox blocks the bundled helper binary and live code drivers.
- A WYSIWYG slide canvas. The user edits markdown, and the preview is read-only.
- Live merging of external edits.
- Editing `.jsx` component files inside the app.
- Vim keybindings.
- Crash reporting or analytics of any kind.

## Comparison with existing tools

- **iA Presenter** is the closest match: text first, slide blocks, and presenting from the app. It is closed, uses its own markdown dialect, and has no live code.
- **Slidev** has a web editor panel and an overview, but editing is secondary to a Node dev server.
- **Marp for VS Code** has a side preview, but you still scroll one long file.
- **Deckset** is a native viewer for a file you edit somewhere else. It has no editor.
- **Clearly and Typora** are good markdown editors with no notion of slides.

## Platform and repo

- macOS 14 or later.
- An AppKit core: the editor, sidebar, windows, and presenting. SwiftUI is used only for sheets and settings.
- Documents use `NSDocument`: native tabs, recent files, autosave in place, Revert To, and file coordination that sees external writes.
- The app registers for `.md` as an Alternate editor. It appears in Open With without becoming the default.
- The app lives in `desktop/` in this repo. It bundles the `tap` binary built from the same commit at `Contents/Resources/tap`. The app and the CLI share one version number, one release job, and one changelog.

## Processes

Each open deck has one writing process, so a crash affects one deck and tap needs no multi-deck support:

```
tap dev --app <file>
```

Play or Rehearse starts a second process for the talk. The writing process keeps running for the preview:

```
tap present --app <file>                 # Play
tap present --app --no-record <file>     # Rehearse
```

In `--app` mode, tap:

- binds to `127.0.0.1` on a free port, never `0.0.0.0`;
- opens no browser and shows no TUI;
- prints a JSON ready line to stdout with the port, the token, and a one-time launch code;
- requires the token on every request and on the WebSocket upgrade;
- uses stdin and stdout as the control channel: questions it would ask in the terminal (approval, recording consent, keep recording) and state changes go out as JSON events on stdout, and the app's answers and commands (reload, tunnel, recording, quit) come in as JSON lines on stdin. Only the parent process can reach these pipes, so script on a slide page can never answer a question or stop a recording.

The app starts every tap process with the login shell environment. An app launched from Finder does not inherit it, so the app runs `$SHELL -l -i -c env` once at launch, with a timeout. If that fails, the app falls back to the default environment and shows a notice.

A tap process started with `--app` exits when its stdin closes, so a crashed or killed app never leaves tap processes behind. The prototype left 10 orphaned `tap dev` processes before this rule.

When a tap process exits unexpectedly, the app restarts it with backoff, and the editor keeps working. The preview shows "Restarting preview" over the last good render. After 3 exits in 30 seconds, the app stops retrying and shows tap's last stderr lines with "Try Again". Window > Tap Log shows each process's output, and the About window shows the bundled tap version.

## The protocol between the app and tap

- `PUT /api/app/source` sends the unsaved buffer after a typing pause of about 100 ms. The response lists the slides: line range, layout, title, step count, skip flag, errors, and each code block with its driver. tap renders from the buffer until the next save.
- The existing WebSocket carries rendered updates to the preview, and a new `file-changed` message so the preview reloads. The app itself learns about file changes, questions, recording, and the tunnel from the stdout events.
- The app drives slide and step positions through the existing WebSocket `slide` message. It never injects JavaScript into the page.
- Commands that need no server run as subcommands: `tap new`, `tap theme list`, `tap theme show`, `tap theme set`, `tap image …`, `tap component new`, `tap slide add --print`, `tap deck schema`, `tap export …`, and `tap build`.
- When a tap command writes the deck, the app saves the buffer first, runs the command on the file, and loads the result as one undo step.

## Editor

- `NSTextView` on TextKit 2, with its own neutral look that follows system light and dark mode. The deck theme appears only in the preview and the thumbnails.
- The frontmatter is hidden, so slide 1 is the first box. The Deck tab edits the frontmatter. Hiding uses the TextKit 2 layout fragment enumeration, and the app keeps the caret and every selection out of the hidden range. Without that clamp, Up Arrow and a keystroke edited the frontmatter in the prototype. Select All, Find, Replace, and Undo must be tested against the hidden range.
- Each slide is a rounded box. The header shows the number, layout, title, a step count badge, and a live-code badge with the driver. The `---` line stays in the text and is drawn as a faint divider.
- Highlighting covers markdown plus tap's own syntax: `::slot` markers, `<!-- pause -->`, and directive comments. Speaker notes stay inline, dimmed and in italics. The editor never folds or hides text.
- Parse and render errors mark the box in red and show the message on the line, like Xcode issues. Where the app can fix the problem, it offers a fix-it, for example "Allow shell in This Deck".
- Before tap answers a keystroke, the app shifts the old box ranges by the size of the edit, so boxes never jump. tap's answer then replaces the shifted ranges.
- Cmd+Shift+O opens an outline to jump to a slide. The native find bar searches the whole deck. Spellcheck is off inside code blocks and directives.
- A pasted or dropped image goes to `tap image add`, which copies it into `images/` and keeps its name, adding -2 or -3 when the name is taken.
- A component path in a slide opens its `.jsx` file in the default code editor. When a saved `.jsx` file changes, the preview reloads.

## Window layout

- The main area is a 50/50 split: the editor on the left, and on the right a large pane with Preview and Deck tabs. The split divider can be dragged.
- The slide panel (thumbnails) has two states. **Floating:** a glass panel over the left edge of the editor, shown and hidden with a toolbar button or a key. **Pinned:** the pin in its header docks it as a normal `NSSplitViewController` sidebar, and the editor and the right pane share the rest of the width.
- On first launch the panel is pinned, so people find it. Each window then remembers its own state.
- The app owns the divider. It restores 50/50 after the panel is pinned, unpinned, or collapsed, and after the window resizes, unless the user dragged the divider.
- The editor's content scrolls under the unified toolbar with the standard macOS 26 scroll edge effect.
- The pinned state is a stock sidebar. The floating state is a small custom overlay, because AppKit has no stock "overlay until pinned" sidebar.

## Slide panel and slide operations

- The slide panel is an `NSCollectionView` of thumbnails in the Keynote style. It is virtualized, so only on-screen thumbnails have cells.
- The slide panel is linked to the cursor. Clicking a thumbnail moves the cursor there. Moving the cursor selects the thumbnail. Shift-click and Cmd-click select several slides, and the cursor stays in the slide clicked last.
- A slide's text holds its own directives, slots, and notes, so a slide moves as one line range. A move cuts the range, inserts it at the target boundary, and fixes the separators so exactly one `---` sits between slides. A `---` inside a fenced block is never a separator, because the ranges come from tap's parser.
- Moving works by dragging thumbnails or box headers, with Cmd+Option+Up and Down, or from the menus. Moves, duplicates, deletes, and skips act on the whole selection as one undo step, and the selected slides keep their order. The drop indicators are AppKit's blue line with a ring, and the drag image carries a count badge.
- A dropped slide never goes above the frontmatter.
- Dragging to another deck's sidebar copies the slides, and holding Cmd moves them.
- New Slide inserts the last layout used. Holding the button, or using Slide > New Slide, opens a gallery of tap's 12 layouts. The templates come from `tap slide add --layout <x> --print`.
- Skip Slide writes the new `skip: true` directive, and the slide appears dimmed.

## Preview and Deck pane

The right half of the window has two tabs: Preview and Deck.

- **Preview** shows the audience view of the slide under the cursor, with all steps revealed and full error cards. The audience-safe error form is used only while presenting. A step control moves through fragments, and a pin keeps one slide in view. T cycles themes for a quick look without writing the file. View > Preview in Window detaches the preview into its own window.
- **Deck** is a form for the frontmatter. `tap deck schema --json` supplies the keys, types, and allowed values, so Swift hard-codes no keys. Keys the form does not know are listed under "Other keys". Driver settings show a hint to use `${NAME}` for secrets.

## Performance

- The preview is a live render, never a screenshot. After a typing pause of about 100 ms, the app sends the buffer, tap parses it (about 2 ms for 100 slides) and sends an `update` message, and the page replaces its data in place and re-renders only the changed slide. Measured in the native prototype on an M4 Max: from the PUT until the new text is in a real `WKWebView` takes 1.7 ms on a 9-slide deck, and 13.8 ms on a 200-slide deck with a component on every slide (about 43 ms to a painted frame). tap's parse, build, and transform is about 10 ms of that. From a keystroke to the new text is 104 ms and 116 ms, most of it the 100 ms typing pause. A `.jsx` edit takes about 400 ms, because tap rebuilds the bundle. If updates ever need to be faster, look first at tap's pipeline on large decks, then at the page's full `/api/presentation` fetch on every `update`.
- While presenting, the talk windows show only their current slides.
- No browser is launched for previews. `tap export` uses headless Chromium, but only for export.
- Thumbnails are static images from one hidden `WKWebView`. The current slide goes to the front of the queue after each update, so its thumbnail follows within about 25 ms. It is not a snapshot of the live preview, which can be caught in the middle of a transition or a theme animation.
- The hidden `WKWebView` lives inside a visible window's view hierarchy, behind other content. In an off-screen window, WebKit suspends the page and nothing finishes rendering. It loads the page with `?print=true`, so it never joins the WebSocket hub and never moves the preview, and it scales the 1920x1080 print layout down. It renders slides in a queue: visible thumbnails first, then nearby ones, then the rest at idle. The queue pauses while you type. Before each snapshot, it waits for tap's single ready signal, the same one `tap export` uses.
- A cold pass over every thumbnail took 0.3 s for 9 slides and 4.9 s for 200 slides in the prototype, about 24 ms per slide, most of it waiting for the slide to render. `takeSnapshot` itself took under 2 ms.
- Thumbnails are cached on disk, keyed by a hash of the slide text, theme, component bundle, and canvas size. After a theme change, the old thumbnails stay visible with an "updating" mark until the new ones replace them.
- The parser alone is cheap: `BenchmarkParse200Slides` takes 3.3 ms. The full pipeline, with component builds and the transformer, takes about 10 ms on the 200-slide deck.
- Targets on a generated 200-slide deck with a component on every slide: typing under 16 ms, preview updates under 200 ms, and reopening loads every thumbnail from the cache. The prototype measured typing with slide boxes at 1.6 ms median, 6 ms p95, and 10 ms worst, to the frame commit. Drawing the boxes cost about 0.4 ms.

## Presenting

- Play (Cmd+Option+P) opens a popover with the display arrangement, "Swap Displays", where to start, recording, phone remote, and an Advanced section for the presenter password and the tunnel. Shift-click starts from slide 1. Rehearse (Cmd+Option+Shift+P) shows only the presenter view, with the timer.
- Before starting, the app saves the buffer, because `tap present` reads the file.
- The audience page goes full screen on the projector, and the presenter page opens on the laptop. Both are tap's own pages in `WKWebView`s. The web views have full screen enabled, handle the popup that the `S` key opens, and use a persistent data store, so every `tap dev` key and presenter setting works unchanged.
- The app's presenter toolbar has REC, Reload Slides, Swap Displays, and Stop. In full screen, it slides in when the pointer reaches the top edge. A REC dot stays visible while recording.
- `tap present` does not watch files. Edits made during the talk wait for Reload Slides, and the toolbar shows how many edits are not shown yet.
- Recording is `tap present`'s: segments, following the projector, and the disk guard. The consent question and "Keep this recording?" come from tap as events and appear as sheets. The answers are stored in `~/.config/tap/settings.yaml`, which the CLI shares.
- The app remembers display assignments per pair of displays, across decks. It holds a sleep assertion, shows no Sparkle prompts or restarts during a talk, and the first time you present, it suggests a Focus mode. After the talk, the cursor moves to the last slide you presented.

## Live code approval

tap owns this rule, so the CLI and the app behave the same way.

- A deck with live code must declare each driver in its existing `drivers:` frontmatter map. A driver with no settings is declared as `shell: {}`. A block with an undeclared driver does not run. Its message names the exact entry to add, and the app offers a fix-it.
- Nothing runs until the user approves the deck. The first open of an unapproved deck with live code asks once. The app asks with a native sheet whose default button is Don't Allow. The CLI asks in the terminal before the TUI starts. The prompt lists the declared drivers, and for a custom driver it shows the command. Each block can be expanded to read.
- Approvals are stored in `~/.config/tap/settings.yaml`, keyed by the deck's absolute path, with the drivers that were approved. Edits to code never ask again. A new driver asks again. Moving the deck asks again. Decks made with New Deck are approved automatically.
- Don't Allow leaves the deck fully usable, with Run buttons that show "Not approved". The question returns at the next open.
- Run buttons send only `{slide, block}`. tap runs the code that the deck file contains, so script on the page cannot run anything else. Components and raw HTML need no approval.
- Headless runs keep live code off unless `--allow-code` is passed for that run.
- Settings > Live Code, `tap approval list`, and `tap approval revoke` show and revoke approvals.
- The approval prompt never appears inside the slide page, where page script could answer it.

## Documents and external changes

- The welcome window shows New Deck, Open, and recent decks with thumbnails. Closing the last deck window shows it again. Open accepts a folder and uses tap's deck resolution rule. Opening an open deck again brings its window to the front.
- The app autosaves in place after the delay set in Settings, which is 1 second by default.
- An external change with no unsaved edits loads silently. The reload is applied as a diff and is one undo step, so Cmd+Z returns to the text before the change, and the cursor stays on its slide. With unsaved edits, a bar offers "Load Disk Version" and "Keep Mine". If the file is deleted, the app keeps the buffer as unsaved and offers Save As. A rename is followed, as in other `NSDocument` apps.

## Creating decks and themes

- New Deck is a sheet with the title, a grid of every theme, and a location that defaults to the last used folder. The theme cells are real renders from `tap theme show <slug> --image`, cached and grouped light and dark. The sheet runs `tap new`, which also approves the new deck, and creates a folder with the deck and `images/`.
- The toolbar theme picker and the Deck tab use the same grid. Picking a theme runs `tap theme set`.
- Generate Image and Regenerate run `tap image generate` and `tap image regenerate`. New Component runs `tap component new`, inserts the snippet that tap prints, and opens the new file in the default code editor.

## Export

- File > Export > PDF runs `tap export pdf --progress json`. The first export shows a one-time sheet while the export engine downloads.
- File > Export > Website runs `tap build --progress json` and offers Preview through `tap serve`.
- File > Export > Images runs `tap export images --all`.
- Cancelling sends SIGINT. Slides that fail are listed as warnings, as the CLI does.

## Settings

- **General:** the editor font size and line spacing, the default theme for new decks, and the autosave delay.
- **Live Code:** tap's approvals, with Revoke and Show in Finder.
- **Image Generation:** the Gemini key, stored in the Keychain and passed to tap as `GEMINI_API_KEY`. A value from the login shell takes precedence.
- **Command Line:** the bundled tap version and any other `tap` on PATH. Install links the bundled binary into `~/.local/bin`, or into `/usr/local/bin` after a password prompt, only where it comes first on PATH and only after you confirm. It never replaces or deletes a binary the app did not install.

## Menus and accessibility

- The menus are Tap, File, Edit, Slide, View, Present, Window, and Help. Every toolbar and context menu action also has a menu item. The Slide, Present, and View menus are drawn on the canvas.
- Presentation keys go to the preview when it has focus. In the editor, those keys type text.
- Boxes, thumbnails, and drop indicators have VoiceOver labels, and every slide operation works from the keyboard alone.

## Security summary

- The `--app` server binds to loopback only. It requires a per-launch 32-byte token on every request and on the WebSocket upgrade. The app sends it as `Authorization: Bearer`, which forces a CORS preflight that the server never approves. The pages get it as a cookie in exchange for a one-time launch code, so the token never appears in a URL. The server rejects foreign `Origin` headers and bodies that are not JSON.
- Questions and control commands travel only over the child process's stdin and stdout, never over HTTP.
- The same Origin and Content-Type guard for plain `tap dev` is PR #14.
- Live code runs only for approved decks and declared drivers, and only code that is written in the deck.
- The app exposes no general bridge from the page to Swift. It has one narrow message handler for a fixed set of events. Links to other sites open in the default browser.
- Driver secrets use `${ENV}` from the login shell, not literal values in the deck.

## Distribution

- Free, under the repo's license.
- Developer ID signing and notarization.
- A DMG on each GitHub release, a Sparkle appcast in the same release, and a `tap-desktop` Homebrew cask that the release job updates.
- No crash reports or analytics.

## Milestones

Each milestone ends with a working app. The tap prerequisites come first. They are specified in their own document.

1. The tap prerequisites for writing: `--app` mode, the slide list with ranges, the `file-changed` message, and the render-ready signal.
2. The app shell: documents, the editor with boxes, the Preview tab, the welcome window, and the tap process lifecycle.
3. The sidebar, thumbnails and cache, and slide operations, including multi-select and the layout gallery.
4. Presenting and rehearsing through `tap present --app`, with the recording sheets.
5. Live code approval in the app, the Deck tab, and the fix-its.
6. Creating decks, themes, images, components, export, and Settings.
7. Signing, notarization, Sparkle, the Homebrew cask, and the release job.

## Testing

- The feature files are the acceptance criteria. Each scenario maps to one XCUITest or Go test with the same name, and a CI check fails when a scenario has no matching test.
- Go tests cover every tap prerequisite, as described in its own spec.
- XCTest covers the slide operations on the buffer, with a round trip that parses the result with tap and checks the slide order and the separators.
- Performance tests use the generated 200-slide deck.
- To verify during milestone 3: how reliably a React component reaches its final state for a thumbnail snapshot.

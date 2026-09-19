# Changelog

All notable changes to Tap are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [2.0.0-beta.1] - 2026-09-19

### Breaking changes

- **`|||` column separator removed** - Multi-column and multi-slot layouts now use `::slotname` marker lines instead (e.g. `::right`, `::sidebar`, `::caption`). Content before the first marker is the `default` slot. See `docs/reference/slide-directives.md` and `docs/reference/layouts-reference.md`.
- **21 new themes replace the old 13** - Every built-in theme was redesigned from scratch: `base` plus 20 named themes (`terminal`, `product`, `swiss`, `newsprint`, `zine`, `poster`, `blueprint`, `riso`, `retro-computing`, `paperback`, `keynote`, `editorial`, `observatory`, `arcade`, `isometric`, `ink`, `lab-notebook`, `bauhaus`, `sketch`, `transit`). An old theme name (e.g. `paper`, `noir`, `aurora`, `phosphor`, `signal`, `carbon`, `spectrum`, `mono`, `flux`) is unknown to the new set and falls back to `base` with a warning rather than failing the build. See `docs/guide/themes.md`.
- **Frontend rewritten in React** - The Svelte frontend (`frontend/src-svelte/`) is gone; the app is React 19 now. This only matters if you were embedding or patching the frontend directly.
- **`codeTheme` and `transitionDuration` frontmatter keys removed** - Neither ever had an effect. A deck that still sets them keeps working; the keys are ignored.

### Added

- **Slot markers** - `::name` marker lines for layouts with more than one content area (`two-column`, `three-column`, `big-stat`, `quote`, `sidebar`, `split-media`), replacing the `|||` separator.
- **Fence line highlighting** - A fenced code block's info string, e.g. ` ```go {3-4} `, highlights those lines without a separate directive.
- **Speaker notes anywhere in a slide** - A trailing `<!-- notes: ... -->` HTML comment works anywhere in a slide, not only inside the directive block.
- **21 CSS-only themes** - `base` plus 20 themes, each with a `light` or `dark` polarity and a one-line pitch describing the talk it fits. Press `t` while presenting to cycle themes live, or force one with `?theme=<slug>`. `tap new --theme <slug>` scaffolds a deck with a theme chosen up front.
- **Deck-supplied React components** - A deck can supply React components from its own folder, as a whole slide (`layout: ./slides/RollingDeploy.jsx`) or as an inline ```` ```component ./charts/LatencyDrop.jsx ```` fence with JSON props. Tap bundles them itself with esbuild, with no Node install and nothing fetched from the network. Components share tap's React and Motion, read the slide's presenter step, render a final state in print mode, and read the active theme's tokens with `useTheme()`. npm packages installed next to the deck resolve normally. See `docs/guide/custom-components.md` and `docs/reference/components-reference.md`.
- **`tap pdf` renders deck components** - PDF export builds and registers component bundles through the same shared setup `tap screenshot` uses, and exports each component in its final state. A component that fails to build stops the export with exit status 1; a slide that shows an error card is still written, with a `warning: slide <n> shows an error card` line on standard error.
- **`tap screenshot --wait <ms>`** - Keeps a stepped capture live and waits that long after the page is ready (network idle, fonts, running animations finished) before the shot, for an animation that runs on a timer or longer than those waits. Without it, a stepped capture is settled: the requested step with animations and timers finished, rather than caught partway.
- **`tap screenshot`** - Renders one slide, or every slide with `--all`, to a PNG through the same headless browser `tap pdf` uses. `--step` and `--fragment` render an exact presenter state. It exits with status 1 when the slide shows an error card, so an LLM or a script can check a slide it just wrote without opening a browser.
- **`tap add component <Name>`** - Scaffolds a working deck component from a template, with `--inline` for a block component, `--ts` for TypeScript plus `tap-env.d.ts` and `tap-shims.d.ts`, and `--deck` to place it next to a specific deck. Prints the markdown snippet to paste.
- **`tap theme list` and `tap theme show`** - Print a theme's name, polarity, pitch, tokens (colors, fonts, motion, spacing) and illustration style, as a table, as JSON with `--json`, or as a ready style brief for an image model with `--prompt`. `--deck <file>` reads the theme from a deck's frontmatter.
- **Theme tokens for components and custom CSS** - Every theme now also defines `--space-unit`, `--radius`, and `--stroke-width` alongside the existing `--bg`, `--fg`, `--muted`, `--accent`, `--accent-text`, `--surface`, `--font-display`, `--font-body`, `--font-mono`, `--ease`, and `--dur`.
- **`steps:` slide directive** - Sets how many clicker presses a slide consumes, for step-driven components and map slides.
- **Short-title scaling** - Every rendered `h1`, `h2`, and `h3` carries a `data-length` attribute (`short`, `medium`, or `long`), so a theme can scale a short heading up without breaking a long one.
- **The asciinema player is bundled** - The player used to load from a CDN; it now ships in the binary and loads as a lazy chunk, so a terminal recording plays with no network.
- **Hub state retention** - The dev server's websocket hub remembers the live slide, fragment, and step state for 10 minutes after the last client disconnects, so reloading a viewer mid-talk lands back where the talk is. Set `TAP_HUB_STATE_RETENTION` (a Go duration such as `0s`, `30s`, `5m`) to change it.
- **`?step=` and `?fragment=` URL parameters** - Load a slide directly at a given presenter step or revealed fragment, clamped to that slide's own limits. Used by `tap screenshot`, together with `?capture=true`, which renders the live viewer but never opens the websocket, so a stepped capture never shows the connection badge.
- **`textOn(fill, theme)` in the `tap` helper module** - Returns whichever of `theme.bg` and `theme.fg` contrasts more with `fill`, for text painted on an accent fill. `theme.accent` is a fill color; text and thin lines on the slide background use `theme.accentText`.
- **Component slides get the theme's padding and chrome** - A `layout: ./slides/Thing.jsx` slide is now inset like that theme's `default` layout, so a component fills the same safe area every other slide has. Per-theme measurements are in `docs/reference/theme-porting.md`.

### Changed

- **`printMode` means the settled state of the current step** - It used to pair only with `step` forced to the slide's total, so a component could treat it as "show the last step". A stepped `tap screenshot --step k` capture now also sets `printMode`, with `step = k`, so a component must derive what to show from `step` and use `printMode` only to skip animation. The `Step` helper follows the same rule in every mode, which means a `<Step from={a} to={b}>` whose `to` is below the slide's total is now hidden in a PDF and in previews; content that must appear in a PDF belongs in an `at` Step or outside a `Step`.
- **A component's mount animation plays** - The `AnimatePresence` around slide transitions used to propagate `initial: false` into a deck component's tree, so a component's own `initial` to `animate` transition was skipped on the first slide a page loaded on. Tap now resets the presence context around every deck component.
- **`useTheme()` has real values on the first painted frame** - Tokens are read in a layout effect rather than a plain effect.
- **Dev and serve ports** - A busy explicit `--port` now fails with a clear message and exit status 1; a busy default port falls forward to the next free one (up to 20) and the real URL is printed. `tap serve` binds before printing its startup message. The temporary servers behind `tap pdf` and `tap screenshot` bind `127.0.0.1`.
- **Sync on reconnect** - A page's load-time URL hash is weighed against the hub's state only on the first state message of that page load; a reconnect's message wins outright, so a reconnecting presenter is no longer pulled back past the audience. The retained state carries no theme, and a relayed slide index that is negative or past the last slide is rejected.
- **Validation** - A `steps:` directive that is negative or not an integer is ignored with a warning naming the slide; a literal `layout: component` with no path is reported as an unknown layout; a `component` fence path may contain spaces; a bad props JSON reports the real line in the deck file; a build error with no source position no longer prints `:0:0`.
- **Component imports** - Code and stylesheets may be imported from outside the deck folder, but a data or asset file (JSON, text, image, font) must resolve inside it or under `node_modules`, symlinks followed. JSX always compiles with the automatic React runtime regardless of a `tsconfig.json` near the deck. A failed bundle import is retried when the slide is entered again.
- **Escape blurs a focused input** - An input or textarea on a slide used to swallow every key, including Escape, leaving no way back to the clicker.
- **Errors and warnings go to standard error** - Every command's error and warning output used to land on standard output, mixed in with the command's real result. Both now go to standard error, so a script reading, for example, `tap screenshot`'s written paths sees only those paths.
- **A fixed 1920px canvas** - Every slide renders on a 1920px-wide canvas (height from `aspectRatio`) that scales as one unit to fit the window, so a deck looks identical on any screen. The letterbox area around it is always black, whatever the theme.
- **Text sizes reworked across every theme** - Body text is at least 40px, code at least 36px, table text at least 40px, and no text goes under 24px; `h1` and `h2` under 60px is reported as a warning by the theme check suite. Several themes were rebalanced to sit in the recommended ranges.

### Fixed

- **`tap --version` reports the release version** - Release builds set a version flag on a variable that did not exist, so every release printed `0.1.0`. A build without the flag now prints `dev`.
- **Unquoted hex colors in directives** - A value like `background: #1a1a2e` no longer needs quotes in the directive block.
- **Fragment count** - The reported fragment count no longer includes a dead first press that revealed nothing.
- **Presenter view mirrors fragment and step state** - The presenter's current-slide panel now shows the same revealed fragments and step as the audience view, not just the slide index.
- **Static builds no longer retry the hot-reload websocket** - A `tap build` deck has no server, so the viewer and presenter now skip opening (and reconnecting) the websocket once static mode is detected.
- **Returning to slide 1 now syncs** - A websocket slide message carrying index 0 used to be dropped on the way to other clients, so navigating back to the first slide left the presenter (or viewer) stuck on the previous one.
- **Navigating back to a slide a peer just pushed on you now syncs** - A local navigation that happened to match this client's own last broadcast, but not what any peer actually last received, is no longer silently skipped.
- **Live code, map, and highlighted code blocks pair with the right block again** - A layout that renders its slots in a different order than the source (or a fence indented in a list item, or a `~~~` fence) used to make the wrong block become a live driver widget or get deleted as a map block.
- **Presenter view now loads the deck's theme** - The presenter's current and next slide panels used to set `data-theme` without ever loading that theme's CSS, so anything other than `base` rendered unstyled there; both panels now load and apply the deck's theme (or `?theme=`) the same way the audience view does.
- **A presenter opened mid-talk now lands on the current slide** - Opening the presenter view partway through a talk used to start it at slide 1 (or wherever its URL hash pointed); it now picks up the viewer's current slide and fragment state instead.
- **Jumping to a slide from the overview now syncs** - Selecting a slide from the O-key overview never broadcast the new state to other connected clients, unlike every other navigation path.
- **A huge fence highlight range no longer hangs the tab** - A spec such as `` ```php {1-999999999} `` used to expand into a billion line numbers; it's now clamped to the code block's actual line count.
- **A URL hash now wins over the hub's live state, and print mode never connects at all** - A normal page load with a hash naming a different slide than the hub's current one used to jump to the hub's slide anyway; the hash now wins, and the hub's fragment/step/scroll state is only taken when the hash names the *same* slide as the hub. A page loaded with no hash still picks up the hub's live position, as before. `?print=true` (PDF export) never opens the websocket at all, so an export can no longer have a live viewer's slide applied out from under its screenshots.

## [0.3.0] - 2026-03-27

### Added

- **Screen Wake Lock in presenter view** - Prevent sleep/screensaver while presenting using the Screen Wake Lock API.
- **Image border control** - Add `{border=none}` attribute to disable theme borders on individual images.
- **PDF export shortcut** - Press `e` in the dev server to export to PDF.
- **Slide overview redesign** - Full-screen grid layout with proper slide rendering in overview mode.

### Fixed

- **Code focus vertical centering** - Vertically center content when focused code block is small.

## [0.2.0] - 2026-03-26

### Added

- **Asciinema terminal recording support** - Embed terminal recordings directly in slides with full playback support.
- **5 new themes** - Carbon, Flux, Mono, Signal, and Spectrum themes with distinct visual styles.

### Changed

- **Presenter view layout** - Slides now display on top with notes below for a more natural workflow.

### Fixed

- **Asciinema player** - Hide player control bar by default for cleaner slide appearance.
- **Theme list alignment** - Fix list padding and bullet design in Spectrum theme.
- **Centered layout alignment** - Force left-align on lists and blockquotes inside centered layouts.
- **Code block alignment** - Prevent text-align center on code blocks across all themes.
- **Static builds** - Use real Vite frontend in static builds for proper theme rendering.
- **Post theme** - Fix code block colors in Post theme.
- **PDF export** - Disable slide transitions in print mode to prevent ghosting.

## [0.1.0] - 2026-02-04

### Added

- **Animated map slides** - New `map` slide type with animated transitions between locations. Supports custom markers, zoom levels, and route animations.
- **Scroll reveal directive** - Add `<!-- scroll -->` to slides with long content for smooth scroll-based progressive reveal.
- **Fragment auto-fragmentation** - List items can now automatically animate in sequence using the `<!-- fragments -->` directive.
- **PDF export improvements** - Added print mode for fragments and PDF metadata support (title, author, subject).
- **Interactive file picker** - When no slide file is provided, tap now shows an interactive file picker.
- **Bidirectional presenter sync** - Complete two-way synchronization between presenter and audience views.
- **Image preloading** - Images are preloaded on page load to prevent transition flashes.
- **Theme gallery** - New documentation page showcasing all available themes with screenshots.
- **Local font embedding** - Font infrastructure for embedding fonts locally in themes.

### Changed

- **8 curated themes** - Aurora, Bauhaus, Editorial, Ink, Noir, Paper, Phosphor, and Poster themes with improved typography, spacing, and visual effects.
- **Better syntax highlighting** - Refined code block syntax highlighting per theme.
- **Mermaid integration** - Improved Mermaid diagram theme integration and foreignObject text clipping fix.

### Fixed

- **Slide transitions** - Enabled smooth slide transitions between slides.
- **PDF export** - Resolved PDF export failures and added image support.
- **Presenter view** - Speaker notes now display correctly in presenter view.
- **Image sizing** - Markdown image sizing attributes now apply correctly.
- **Code block parsing** - Slide delimiters inside code blocks are now ignored.
- **Scroll state** - Prevented scroll state from incorrectly triggering on forward navigation.
- **Dev server bugs** - Multiple dev server stability improvements.

## [0.0.1] - 2026-01-25

### Added

- **Mermaid diagram support** - Render flowcharts, sequence diagrams, ER diagrams, and more directly in slides using mermaid code blocks. Diagrams automatically match your presentation theme.
- **AI image generation** - Generate images from text prompts using Google Gemini. Press `i` in the dev server to open the image generator, describe what you want, and the image is created and inserted into your slide.

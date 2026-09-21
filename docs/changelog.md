---
title: Changelog
---

# Changelog

All notable changes to Tap are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [2.0.0-rc.1] - 2026-09-21

### Added

- **Record a talk from `tap dev`** - Press `C` to record the screen and microphone on macOS, and press it again to stop. Recording uses `screencapture`, which is built into macOS, so there is nothing to install. With more than one display attached, a picker opens first, and `t` runs a five second test capture that opens in QuickTime Player. A chapter list of slide timings is written beside the movie and updated on every slide change, so a crash mid-talk still leaves one. A `recording:` frontmatter block sets the output folder, the microphone (`audio: none` records silently), the display, time warnings and a hard stop (`warnAfter`, `stopAfter`), and whether to show clicks or write chapters. `tap dev` checks the Screen Recording permission at startup, so a missing grant shows up during setup rather than on stage. See the Talk Recording guide.

- **Five presenter layouts, switchable mid-talk** - **Standard** is the familiar big current slide with the next slide and notes beside it. **Notes first** gives the notes most of the width, for a talk you read from. **Duo** shows the current and next slides at equal size with the notes below, for demos and builds. **Slide only** is a confidence monitor. **Notes only** fills the screen with the script. A button in the presenter header opens the list; `V` cycles layouts, and `1` to `5` pick one while the list is open. Duo and Slide only are offered only above 768px, and on a phone the list opens as a sheet at the bottom of the screen.

- **Speaker notes that scale to fit their panel** - A new **Fit to panel** notes size picks a font size per slide so the slide's notes fill the panel without scrolling, from 1rem up to 6rem, re-measured when the slide changes or the window resizes. In this mode `-` and `=` scale the fitted size down to half and back. Notes too long to fit at 1rem stay at 1rem and scroll. Manual stays the default.

- **`presenterLayout:` frontmatter** - A deck can suggest the layout the presenter view opens in. A device that has picked a layout keeps its own choice. The order is `?layout=` on the URL, then the browser's saved choice, then the deck's key, then Standard. `?layout=` and `?notesSize=` are one-off overrides and are never saved.

### Changed

- **A phone no longer scrolls the presenter view** - Each layout now fills the screen exactly, and only the notes scroll, and only when the size is set manually. The old phone layout capped the notes at 40% of the screen and let the whole page scroll, which lost your place mid-sentence.

- **The dev terminal's QR code opens the presenter view** - Scanning it on a phone lands on the speaker notes and controls, and a new "Slides" link in the presenter header gets to the deck itself. A presenter password is carried in the scanned URL, so the view opens straight away.

- **The QR code shows in a shorter window** - The height it needs is measured against the real screen rather than assuming a tall terminal.

### Fixed

- **The screen keeps staying awake on a phone** - The wake lock was asked for once, when the view opened, and Safari often refuses a request made while the page is still settling, so a phone showing notes dimmed and locked anyway. A refusal is now retried, the first touch or key press asks again, and a lock the platform takes back is re-taken at once. This applies to both views, over `https` (`tap dev --tunnel` gives you one) or on `localhost`.

- **Recording a talk no longer reloads the deck on every slide change** - The chapter list is rewritten on each slide change, and the recordings folder sits inside the deck folder by default, so `tap dev` rebuilt the deck and reloaded every window, flashing the page and resetting the presenter timer. The dev watcher now ignores the recordings folder and Finder's `.DS_Store` files.

## [2.0.0-beta.7] - 2026-09-20

### Fixed

- **The QR code in the dev terminal scans again** - A code taller than 15 lines had every other row dropped to make it fit, which leaves something that still looks like a QR code and cannot be scanned. Codes are now drawn two module rows per line with half blocks, so a tunnel URL takes 21 lines instead of 41 and keeps every row, and they are drawn dark-on-light rather than inverted, which some scanners refuse. A code is left out entirely when the window is too short for it, rather than mangled to fit.

## [2.0.0-beta.6] - 2026-09-20

### Added

- **`tap dev --tunnel` puts the deck on a public https URL** - Through a Cloudflare Quick Tunnel, which needs no Cloudflare account, no login and no configuration: `cloudflared` dials out and Cloudflare hands back a random `*.trycloudflare.com` address that lasts as long as the server. Press `u` in the dev terminal to start or stop one mid-session; the URL appears with a QR code to point a phone at. The tunnel's hostname is added to the Host allow-list while it runs and removed when it stops, so nothing else becomes reachable. `cloudflared` has to be installed (`brew install cloudflared`), and `--tunnel` says so plainly when it is not. Anyone with the link can watch the deck, so the terminal says so.

- **The screen stays awake in the audience view** - A deck is watched, not touched, so a phone propped up as a prompter used to dim and lock partway through a slide. The audience view now holds a screen wake lock, re-taking it whenever the page becomes visible again, the way the presenter view already did. Both views share one implementation. The Screen Wake Lock API needs a secure context, so this applies over `https` or on `localhost`, and not over plain `http` to a LAN address.

## [2.0.0-beta.5] - 2026-09-20

### Added

- **The mouse pointer hides itself while presenting** - In fullscreen, the pointer disappears after 2.5 still seconds, so it does not sit on a slide on a TV or a projector, and comes back on the next movement. In a window it is never touched.

- **A swipe says where it landed** - A small pill fades in at the bottom of the screen with the new slide number, and says "First slide" or "Last slide" when a swipe hits either end. A deck with no transition changes instantly, so without this a swipe looks like nothing happened.

- **Touch navigation in the audience view** - On a phone or a tablet, swipe left for the next fragment, step or slide, and swipe right to go back. A two-finger tap toggles the slide overview, where a tap picks a slide and a tap outside closes the grid. A swipe must travel at least 50 px, be more horizontal than vertical, and finish within 800 ms, so scrolling a slide, pinching to zoom and tapping a link are all left alone.

### Changed

- **The slide overview renders only the thumbnails near the viewport** - It used to mount every slide at once, so a long deck of component-driven slides could exhaust a phone's memory and reload the page. A thumbnail now mounts as it comes within 300 px of the viewport and unmounts once it leaves. Where `IntersectionObserver` is missing, every thumbnail renders as before.

- **Bigger thumbnails on a touch device** - The overview grid is one column up to 600 px wide and two above it, rather than the three to five a mouse gets, so a thumbnail is big enough to read and to hit with a finger.

### Fixed

- **The page no longer drifts vertically during a horizontal swipe** - Once a drag is clearly sideways, swipe navigation claims it, so the browser stops scrolling the page under the finger. A vertical drag and a pinch are still the browser's, so a scrollable slide scrolls and zoom keeps working.

- **The overview no longer closes itself the moment a two-finger tap opens it** - The synthetic click a browser fires after a touch sequence landed on the backdrop the gesture had just opened. That one click is now swallowed.

## [2.0.0-beta.4] - 2026-09-19

### Changed

- **The terminal theme's window tab shows the deck title** - The tmux tab in the status bar reads `3:Quarterly Review*`, taken from the deck's `title`, or `3:zsh*` when the deck has none. A long title ends in an ellipsis before the slide counter. It used to read `3:tap*`.

## [2.0.0-beta.3] - 2026-09-19

### Added

- **`tap dev` shows its version** - Next to the title in the terminal interface, and on a `Version:` line in `--headless` mode, so a server left running from an older build is easy to spot.

## [2.0.0-beta.2] - 2026-09-19

### Added

- **`?` lists the keyboard shortcuts** - In the audience view and the presenter view, `?` opens a list of that view's shortcuts. `?`, `Esc`, or a click outside closes it. The `tap dev` help line mentions it.
- **`tap new --yes`** - Writes the starter deck from `--title`, `--theme`, and `--output` with no wizard, and prints only the written path. `tap new` does the same when standard input is not a terminal. An existing file is kept unless `--force` is given.
- **`slideNumbers: false`** - Frontmatter key that hides the slide number every theme draws. Decorations around the number stay: transit keeps an empty station marker, blueprint keeps the drawing row of its title block, and terminal keeps its window tab.
- **Status and second-accent theme tokens** - Every theme now defines `--status-ok`, `--status-warn`, and `--status-error`, each readable as a fill at 3:1 or better against `--bg`, plus `--accent-2` (a real second accent where the theme has one, otherwise equal to `--accent-text`). `useTheme()` returns them as `statusOk`, `statusWarn`, `statusError`, and `accent2`, so it has 18 keys rather than 14. `tap theme show --json` lists them under `tokens.colors` and `--prompt` names them as colors to use only where meaning requires them. Use them for a health state or a passed or failed check; something merely inactive still belongs in `--muted`.
- **`tap dev --allow-origin <value>`** - Repeatable. Adds an origin (`scheme://host:port`) allowed to connect to the websocket hub, **or** a host (`host:port`) allowed in a request's `Host` header. Needed for a contributor's separate Vite dev server, and for reaching `tap dev` through a custom DNS name or a tunnel such as ngrok or Tailscale. `localhost`, loopback, private and link-local addresses, `.local` names, the machine's own hostname, and the documented proxy workflow all work without it.
- **Audience-safe errors** - A fullscreen viewer, or one opened with `?present=true`, shows a failing component as the slide's own fallback content plus a small muted `component error` chip, instead of a card with the raw message. For a whole-slide component the fallback is the slide's slots in the default layout; for an inline one the rest of the slide renders as normal. Fullscreen is followed live, so entering or leaving it switches forms at once. The presenter view, a normal window, `?debug=true`, and every print or capture pass still show the full card. The hidden `.deck-error-card` element stays in the DOM either way, carrying `data-message` and `data-source`, so `tap screenshot` still exits 1.
- **`?debug=true` and `?present=true` URL parameters** - Force the full error card on any window, and force the audience-safe form without going fullscreen.
- **Windows catch up after a restart** - The hub's `connected` message carries a revision of the presentation and its component bundles. A window that reconnects and sees a different revision than it first saw reloads itself, so restarting `tap dev`, or editing the deck while a window was asleep, no longer leaves a stale slide on the projector.
- **Blockquote length classes** - Blockquotes carry `data-length="short|medium|long"` (up to 80 runes, 81 to 180, above 180) alongside the heading classes. Thirteen themes step long quotes down.
- **`scripts/prepare-changelog.sh`** - The release workflow's changelog logic, runnable on its own so a contributor can test it against a copy.

### Changed

- **Presenter view layout** - The current slide takes a 60% column. The next slide sits above the speaker notes in the other column, and the notes fill the remaining height. A- / A+ buttons and the `-` / `=` keys change the notes font size, which the browser remembers. On a phone the notes come before the next slide.
- **The presenter view accepts PageUp and PageDown**, like the audience view.
- **The terminal theme's status bar tab reads `3:tap*`** instead of the slide's layout name.
- **Imported assets over 100 KB are emitted as files** - An image or font imported by a component is inlined as a data URL only under 100 KB. At 100 KB or more it is emitted as its own file named `asset-<hash>.<ext>`, served by `tap dev` from `/components/` and written into `dist/components/` by `tap build`, and referenced by URL. The original file name is not used. An asset referenced from a component's CSS with `url()` resolves relative to the emitted CSS file, so it works in dev and in a static build under any sub path. The previous behavior inlined every asset at any size.
- **`--presenter-password` gates control, not just the view** - A correct `?key=` on `/presenter` answers 302 to the same path with `key` removed, so the password leaves the address bar, and sets an HttpOnly cookie holding a random per-process session token, never the password. A valid cookie opens `/presenter` without `?key=`, so the `s` key opens a working presenter window. `/qr` is gated the same way. A window without the cookie still receives all sync and reload traffic, but the hub drops its navigation messages, so an audience member clicking around moves only their own screen and the presenter drives the room. The password may contain any characters and is URL-encoded wherever tap prints a presenter URL or builds a QR code. With no password set, nothing changes.
- **Print mode stops more on its own** - Deck components are wrapped in Motion's `reducedMotion="always"`, which makes transform and layout animations instant, and one global rule sets `animation: none; transition: none` on everything under `[data-print='true']`. Motion animations of opacity, color, and background color are not affected, and neither are your own timers, so a component must still honor `usePrintMode()`. Because CSS animations never run rather than being fast-forwarded, an element that reaches its final look only through `animation-fill-mode: forwards` snaps back to its base style: make the base style the settled state and animate from the start state instead.
- **A component that does not load in 8 seconds is an error** - `component did not load within 8 seconds: <source>`; re-entering the slide retries. No timeout in print, capture, or preview.
- **`?step=` and `?fragment=` leave the URL** - Both are load-time parameters, so they are removed from the address bar on the first navigation rather than pinning a shared link to the step you started on. Every other query parameter stays.
- **Component build errors name their slides** - `error: slides/Broken.jsx:7:1: Unexpected "return" (used on slides 2, 5)`, or `(used on slide 2)` for one.
- **An entry path outside the deck folder fails early** - `component files must live inside the deck folder: <path> (imports from outside are allowed, entry files are not)`, checked before esbuild runs. Imports from outside the folder are still allowed for code and stylesheets.
- **`tap add component --deck <path>` accepts a directory** - A directory argument is used as the deck folder itself, rather than being read as a file whose folder is taken.
- **`tap screenshot` and `tap pdf` stop cleanly on Ctrl-C** - Both finish their cleanup, print `interrupted` on standard error, and exit with status 130 on SIGINT or SIGTERM, whether the signal reaches the process directly or the terminal signals the whole process group. A second Ctrl-C during the cleanup exits at once.
- **Spinners write to standard error** - `tap build` and `tap pdf` draw their progress spinner on standard error, and nothing at all when standard error is not a terminal, so standard output holds only result lines. `tap pdf` stops the spinner before printing warnings.
- **Bundler warnings get their own box in the dev TUI** - Shown during a live reload and cleared on the next clean rebuild.
- **`useTheme()` follows `themeColors`** - Frontmatter overrides now also set the standard token names on the canvas frame, and `useTheme()` re-reads when they change. Setting `accent` sets both `--accent` and `--accent-text`.
- **The frontend build emits `.woff2` only** - The `.woff` fallbacks every `@fontsource` package ships are stripped from the CSS and deleted, since every browser tap supports has read `.woff2` for years.
- **golangci-lint v2** - `.golangci.yml` uses the v2 schema. staticcheck's `QF1012` is excluded deliberately.

### Fixed

- **`tap new` escapes the title and author**, so a title with a double quote no longer breaks the frontmatter.
- **Quoted or special-character alt text on an image attribute block** - `![The "quoted" screenshot](img.jpg){width=300px}` rendered as literal `<img ...>` text on the slide instead of the image, because the hand-built `<img>` tag's `alt` and `src` values were not HTML-escaped. A hostile `width` value is now rejected with a warning instead of being written into the `style` attribute unescaped.
- **Static builds under a URL sub path** - A build's asset references are relative, so the same `dist/` folder now works from a domain root, a sub path, and a GitHub Pages project site with no flag and no rewriting.
- **Entrance animations that end hidden** - The theme check suite gained a live check, per theme, that an element which animates in never settles hidden. A theme whose keyframes ended at `opacity: 0` passed every print-mode check and still showed the audience a blank slide.
- **Error card messages are reported** - `tap pdf` prints `warning: slide <n> shows an error card: <message>` and `tap screenshot` fails with `slide <n> shows an error card: <message>`, instead of naming only the slide. Both card elements carry the message in `data-message` and the file in `data-source`.
- **Doubly escaped image alt text** - An alt text containing `&` was escaped twice, so `AT&T` rendered as `AT&amp;T`, and an alt text that already contained `&amp;` was mangled. Size units are also matched case-insensitively now, so `{width=300PX}` works; `calc()` and anything that is not a plain CSS length or percentage is still dropped with a warning.
- **Status colors that were hard to tell apart** - The three status tokens in all 21 themes are stepped apart by luminance as well as hue, so any two differ by a contrast ratio of at least 1.35. Picking them by hue alone left red and green at nearly the same lightness, which a colorblind viewer or a washed-out projector flattens into one color. The theme check suite enforces both this and the 3:1 floor against the background.
- **A component asset's original file name baked into its URL** - An emitted asset is now named `asset-<hash>.<ext>`, so nothing about a local folder layout reaches a published deck, and an asset referenced from a component's CSS resolves relative to the emitted CSS file rather than the page.
- **`scripts/prepare-changelog.sh` dropped a reused section's last line** - Re-running the release for a version whose section was the last thing in the changelog produced notes missing their final entry.
- **The app answered on every unmatched path** - `tap dev` served the application for any unknown nested path. It now serves the app only at `/`, `/index.html`, `/presenter`, and `/presenter.html`, redirects `/presenter/` to `/presenter` with a 301, and returns 404 for anything else.

### Security

- **Host allow-list against DNS rebinding** - Requests to `/ws`, `/api/`, `/presenter`, `/qr`, `/local/`, and `/components/` must carry a `Host` that is `localhost`, a loopback, private, or link-local address, a `.local` name, the machine's own hostname, or a value given with `--allow-origin`. Anything else gets 403 `Forbidden: host not allowed; use --allow-origin to allow it`. Without it, a hostile page could point a name it controls at `127.0.0.1` and, once the browser re-resolved it, read the deck and drive the hub from off the machine. The websocket now needs both checks: the origin's host must equal the request's `Host` and that host must pass the allow-list, unless the origin is in `--allow-origin`.
- **The presenter cookie holds a token, not the password** - It used to be built from the password itself, and sanitizing it for use as a cookie value could also corrupt it. It is now a random per-process session token, the password is stripped from the URL by a redirect after a successful `?key=`, and `/qr` is gated behind the same check.
- **Image attribute escaping and validation** - Alt text and URL of an image with an attribute block are HTML-escaped, and a size value must be a CSS length or percentage or it is dropped with a warning naming the slide (issue #7).

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

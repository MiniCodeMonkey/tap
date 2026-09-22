---
title: Components Reference
---

# Components Reference

The exact contract for a deck-supplied React component: what tap passes in,
what the `tap` helper module exports, how a component is bundled, how
`steps` is resolved, and what every error looks like. For a walkthrough
that builds one from scratch, see
[Custom Components](/guide/custom-components).

## What counts as a component path

A value is treated as a component file path, rather than a built-in layout
name, when both of these hold:

- it starts with `./` or `../`
- it ends in `.jsx`, `.tsx`, `.js`, or `.ts`

The path is resolved relative to the deck's markdown file. A `.js` file is
parsed with the JSX loader, so JSX in a plain `.js` file builds. A `.ts`
file is parsed as TypeScript without JSX; put JSX in `.tsx`.

The entry file must live inside the deck's own folder. A path that resolves
outside it, symlinks followed, is rejected with:

```
error: component files must live inside the deck folder: ../shared/Thing.jsx (imports from outside are allowed, entry files are not) (used on slide 1)
```

This is checked before esbuild runs, so a deck with an out-of-folder entry
fails immediately rather than partway through a build. A build error with
no source position omits it rather than printing a misleading `:0:0`.

Files that the entry file *imports* may live outside the deck folder when they
are code or a stylesheet (`.js`, `.jsx`, `.mjs`, `.cjs`, `.ts`, `.tsx`,
`.css`); a data or asset file (JSON, text, an image, a font) must live inside
the deck folder.

## Authoring forms

### Whole slide

The `layout:` directive names the component. The component replaces the
built-in layout for that slide and decides which of the slide's slots to
render.

```markdown
<!--
layout: ./slides/RollingDeploy.jsx
-->

# Zero-downtime deploys

Four app servers behind one load balancer.

::caption
One restarts at a time.
```

The step count comes from the component's own `export const steps`, so the
directive does not repeat it. See [Steps](#steps).

A component layout accepts any slot name. Tap does not validate slot names
against a fixed list for it, because only the component knows which ones it
uses.

`component` is not a layout name you can write yourself. A literal
`layout: component` with no path is reported as an unknown layout:

```
error: deck.md: slide 1: unknown layout "component" (valid layouts: big-stat, blank, code-focus, cover, default, quote, section, sidebar, split-media, three-column, title, two-column)
```

### Inline

A fenced block with the info string `component <path>` places a component
inside a normal slide, at the fence's position. The fence body is optional
JSON props.

````markdown
```component ./charts/LatencyDrop.jsx
{ "before": 412, "after": 88 }
```
````

The path may contain spaces: the whole rest of the info string is the path,
so ```` ```component ./my slides/Chart.jsx ```` works.

The body must be a JSON **object** or empty. Anything else is a parse
error that names the slide and the real line in the deck file:

```
Error: failed to parse presentation: deck.md slide 2: line 15: invalid component props JSON: invalid character 'n' looking for beginning of object key string
Error: failed to parse presentation: deck.md slide 2: line 15: component props must be a JSON object
```

The line number is the fence's position in the deck file, counted through
frontmatter, slide splits, slot markers, and pause markers. When an
upstream rewrite makes an exact line impossible to reconstruct, the message
names the fence's path instead of a line.

The parser renders the fence as a placeholder in the slot's HTML:

```html
<div class="deck-component" data-component-index="0"></div>
```

Because the placeholder sits where the fence was, an inline component
follows fragments and slot placement like any other block.

## Component props

A component file has a default export that is a React component. Tap passes
one object:

```ts
interface DeckComponentProps {
  /** Slot HTML by slot name. Whole-slide form only; {} for an inline component. */
  slots: Record<string, string>;
  /** The JSON object from the fence body. {} for the whole-slide form. */
  props: Record<string, unknown>;
  /** The slide's own JSON: index, layout, notes, tag, badge, and the rest. */
  slide: Slide;
  /** The current presenter step, 0 through `steps`. */
  step: number;
  /** The slide's total step count. */
  steps: number;
  /** True only while this slide is the one shown to the viewer. */
  active: boolean;
  /** True for PDF export, `?print=true`, and thumbnails: render the final state, no animation. */
  printMode: boolean;
}
```

A bundle with no default export, or a default export that is not a
function, fails at load time with `<url> has no default export`.

## The `tap` helper module

Import from the bare specifier `tap`. The host provides it; it is never
bundled into the component.

```js
import { useStep, Step, usePrintMode, useActive, useTheme, Slot } from 'tap';
```

| Export | Signature | What it gives you |
|--------|-----------|-------------------|
| `useStep` | `useStep(): { step: number; steps: number }` | The current step and the slide's total. Same values as the `step` and `steps` props. |
| `usePrintMode` | `usePrintMode(): boolean` | True for PDF export, `?print=true`, and thumbnails. |
| `useActive` | `useActive(): boolean` | True only while this slide is on screen. False in every thumbnail. |
| `useTheme` | `useTheme(): ThemeTokens` | The active theme's tokens, read live from CSS custom properties. |
| `Step` | `<Step at={n}>` or `<Step from={a} to={b}>` | Reveals its children once the slide has advanced far enough. |
| `Slot` | `<Slot html={slots.name} className?="..." />` | Renders one slot's HTML, with fragment visibility applied. |
| `textOn` | `textOn(fill: string, theme: ThemeTokens): string` | Picks `theme.bg` or `theme.fg`, whichever contrasts more with `fill`, for text painted on that fill. |

### `Step`

`<Step at={n}>` renders its children from step `n` onward.
`<Step from={a} to={b}>` renders them for steps `a` through `b` inclusive.
`to` is optional and only meaningful with `from`.

`Step` compares against the **current** `step` in every mode, print mode
included. There is no special print-mode branch.

For `<Step at={n}>` that changes nothing: print mode and previews set
`step` to the slide's total, so any `at` at or below the total renders.

For `<Step from={a} to={b}>` it matters. A range whose `to` is below the
slide's total is **hidden** in a PDF, under `?print=true`, and in every
preview, because `step` there is the total and the range has passed.

::: warning Content that must appear in the PDF
Put it in an `<Step at={n}>`, or outside a `Step` entirely. A
`<Step from={1} to={3}>` on a five-step slide is correct on stage and
absent from the handout.
:::

### `Slot`

`Slot` renders the slot's HTML and toggles `fragment-visible` and
`fragment-hidden` on every `[data-fragment-index]` element inside it, so
fragments written in the markdown keep working inside a component. A slot
name that the slide does not define renders nothing.

### `useTheme`

Returns an object with **exactly these 18 keys**, each read from the
matching CSS custom property on the nearest `[data-theme]` ancestor:

| Key | CSS custom property |
|-----|---------------------|
| `bg` | `--bg` |
| `fg` | `--fg` |
| `muted` | `--muted` |
| `accent` | `--accent` |
| `accentText` | `--accent-text` |
| `accent2` | `--accent-2` |
| `surface` | `--surface` |
| `statusOk` | `--status-ok` |
| `statusWarn` | `--status-warn` |
| `statusError` | `--status-error` |
| `fontDisplay` | `--font-display` |
| `fontBody` | `--font-body` |
| `fontMono` | `--font-mono` |
| `ease` | `--ease` |
| `dur` | `--dur` |
| `spaceUnit` | `--space-unit` |
| `radius` | `--radius` |
| `strokeWidth` | `--stroke-width` |

Every value is a string, exactly as the browser computes it. The tokens are
read in a layout effect, so the first painted frame already has real
values; there is no flash of unstyled output to design around. A token the
current theme does not define reads as an empty string.

The values update when the slide's `data-theme` attribute changes, so a
component follows the `t` key and `?theme=<slug>` live.

Run `tap theme show <slug>` to see the same token values on the command
line.

`useTheme()` also sees a deck's frontmatter `themeColors` overrides, and
re-reads when they change.

#### Reading other theme variables

The 18 keys are the portable set that every theme defines. A theme may
define more; a component can read any of them directly, and should give a
fallback so it still works in the themes that do not:

```jsx
<div style={{ color: 'var(--brand-ink, var(--fg))' }}>ok</div>
```

`tap theme show <slug> --json` lists every raw variable under
`tokens.other`.

## The color rule

`theme.accent` is a **fill** color. It is the one color a theme uses to
paint a shape, and several themes choose a value that is unreadable as
text on their own background: `zine`'s accent is `#ffe600` on an `#e8e5dd`
page.

| What you are painting | Use |
|-----------------------|-----|
| A filled shape, bar, or box on the slide | `theme.accent` |
| A second filled shape that must read apart from the first | `theme.accent2` |
| Text or a thin line sitting on `theme.bg` | `theme.accentText`, or `currentColor` |
| Text painted on top of an accent fill | `textOn(theme.accent, theme)` |
| Healthy, warning, or failed **state** | `theme.statusOk`, `theme.statusWarn`, `theme.statusError` |
| Merely inactive or out of focus, with no verdict | `theme.muted`, a dashed outline, lower opacity |

`theme.accentText` is the accent in a form the theme guarantees is
readable as text on its background. For `zine` that is `#0d0d0d`, not the
yellow.

### Status colors

Every theme defines `--status-ok`, `--status-warn`, and `--status-error`.
Each is readable **as a fill**, at a contrast of at least 3:1 against
`--bg`, and any two of the three differ from each other by a contrast ratio
of at least 1.35, so they stay apart by lightness and not only by hue. Use
`textOn(theme.statusError, theme)` for a label painted on top of one.

::: warning Never signal status by color alone
Roughly one viewer in twelve will not read your red against your green, and
a washed-out projector flattens the difference for everybody. Pair a status
color with a label, an icon, or a shape: a cross and a check, `failed` and
`ok`, a filled square and a hollow one. The luminance separation is there
so the pairing still reads when the color does not.
:::

Use them only where the color carries meaning: a health state, a passed or
failed check, a threshold crossed. A component that paints its whole
palette in status colors leaves the audience nothing to read them against.
Something that is merely inactive or out of focus is not a verdict, so it
stays `theme.muted` with a dashed outline and lower opacity, the way the
shipped example's restarting server box does.

`theme.accent2` is a real second accent in themes that have one, and equal
to `theme.accentText` in the rest, so a component can always reach for a
second fill without checking.

### `textOn`

```ts
textOn(fill: string, theme: ThemeTokens): string
```

Returns whichever of `theme.bg` and `theme.fg` has the higher WCAG
contrast ratio against `fill`, for text painted on top of a `fill`
background. Which one reads differs per theme: a bright accent wants dark
text, a saturated dark accent wants light text, so this is measured rather
than hardcoded.

`fill` accepts `#rgb`, `#rrggbb`, `#rrggbbaa`, `rgb(...)`, and `rgba(...)`.
A translucent fill is composited over `theme.bg` before its luminance is
measured, so a token like base's `--surface: rgba(0, 0, 0, 0.02)` is
correctly treated as the near-white it renders as, not as near-black.
Anything it cannot parse, including an empty string, returns `theme.fg`.

```jsx
import { textOn, useTheme } from 'tap';

const theme = useTheme();

<div style={{ background: theme.accent, color: textOn(theme.accent, theme) }}>
	v2
</div>
```


## Steps

`steps` is the number of clicker presses the slide consumes. Tap resolves
it in this order:

1. The slide's `steps:` directive, when present. It always wins, including
   `steps: 0`.
2. A static `export const steps = <integer>` in the component file.
3. Otherwise 0, except that a `map` fence on the same slide sets a floor
   of 1.

::: warning Do not set both
The directive overrides the file's `export const steps` silently, with no
warning. A deck that sets `steps: 5` in the directive and then has its
component change to four steps keeps consuming five presses, and the fifth
does nothing. Pick one place: the export, so the count lives with the code
that defines it, or the directive when the slide has no component export to
read. The shipped example uses the export alone.
:::

When a slide has several inline components, the slide's step count is the
largest `steps` among them (unless the directive set it).

The export is read statically. Tap never runs the component to find out.
The file is parsed with esbuild and the pattern is matched against the
printed output, so comments cannot hide or fake a value. Two limits follow:

- It must be exactly `export const steps = N` with a literal integer. A
  computed value, an expression, a re-export, or several names in one
  declaration are all ignored, and the count falls back to 0.
- A lookalike assignment inside a real string or template literal still
  matches. Avoid writing the text `export const steps = 3` inside a string.

A `steps:` directive that is negative or not an integer is ignored, with a
warning naming the slide:

```
error: deck.md: slide 4: steps: directive ignored (must be a non-negative integer)
```

A `export const steps` whose value is negative is ignored the same way, and
the count falls back to 0.

## Print mode

`printMode` does **not** mean "show the last step". It means:

> Render the settled state of the **current** `step`, with no animation and
> no timers running.

Which step that is depends on the caller:

| Caller | `step` | `printMode` |
|--------|--------|-------------|
| `tap export pdf` | `steps` | `true` |
| `?print=true` | `steps` | `true` |
| Overview grid, presenter next-slide panel | `steps` | `true` |
| `tap export images` with no `--step`/`--fragment` | `steps` | `true` |
| `tap export images --step k` | `k` | `true` |
| `tap export images --step k --wait <ms>` | `k` | `false` (live) |

`--wait` starts its clock once the page is ready (network idle, fonts
loaded, running animations finished), not at navigation, so it suits an
animation that starts on a timer or runs longer than those waits.

::: warning Never treat print mode as "the final state"
A component that branches on `printMode` to jump to its last step produces
a **wrong** stepped capture: `tap export images --step 2` would show step 5.
Branch on `printMode` only to skip animation and timers, and always derive
what to show from `step`.
:::

```jsx
// Wrong: ignores the requested step.
const shown = printMode ? SERVERS.length : step;

// Right: printMode only decides whether to animate.
const shown = step;
const duration = printMode ? 0 : 0.3;
```

### What tap enforces in print mode

Tap does some of the work for you, but not all of it, so a component must
still honor `usePrintMode()`.

Tap **does**:

- wrap deck components in Motion's `reducedMotion="always"`, which makes
  transform and layout animations instant: `x`, `y`, `scale`, `rotate`,
  `skew`, `width`, `height`, `top`, `left`, `right`, `bottom`, and
  `layoutId` projection
- apply `animation: none !important; transition: none !important` to
  everything under `[data-print='true']`, one global rule in the app
  stylesheet covering the whole slide, components included, which stops raw
  CSS keyframe animations and CSS transitions

Tap does **not** stop:

- Motion animations of `opacity`, `color`, or `backgroundColor`
- anything your own code drives: `setTimeout`, `setInterval`,
  `requestAnimationFrame`, a state machine, a canvas loop

So a fade that Motion drives will still fade in a PDF unless you set
`transition={{ duration: printMode ? 0 : 0.4 }}` yourself, and a timer will
still tick unless you gate it. Treat the enforcement as a safety net for
the transform cases, not as a reason to skip the check.

::: warning `animation-fill-mode: forwards` does not survive print mode
Because the rule is `animation: none`, a CSS animation never runs at all in
print mode, so an element that only reaches its final appearance through
`forwards` snaps back to its base style: invisible, off-position, or
whatever the keyframes started from.

Write it the other way round. Make the **base** style the settled state,
and animate **from** the start state:

```css
/* Fragile: nothing runs in print, so the element stays at opacity 0. */
.badge { opacity: 0; animation: fade-in 400ms forwards; }

/* Sound: the base style is already the end state. */
.badge { opacity: 1; animation: fade-in 400ms; }
@keyframes fade-in { from { opacity: 0; } }
```
:::

The `Step` helper follows the same rule: it compares `at`/`from` against
the live `step` in every mode. True print and previews already force `step`
to the slide's total, so their behavior is unchanged, but a settled capture
at step 2 correctly hides a `<Step at={5}>`.

## Mount animations

Tap resets Motion's presence context around every deck component, both the
whole-slide and the inline form. Without that, the `<AnimatePresence
initial={false}>` that wraps slide transitions would propagate
`initial: false` into the component's tree and skip its own mount
animation, so a reload, a deep link, or `tap export images --step k` would
land on the animation's end state.

With the reset, a component's own `initial` to `animate` transition runs
even on the very first slide a page loads on, and a component may nest its
own `AnimatePresence` normally for its own enter and exit transitions. The
slide-level transition is unaffected.

### What a component receives per mode

| Mode | `step` | `printMode` | `active` |
|------|--------|-------------|----------|
| Live viewer | current | `false` | `true` |
| `?print=true`, `tap export pdf` | `steps` | `true` | `true` |
| Preview (overview grid, presenter next-slide panel) | `steps` | `true` | `false` |
| `?capture=true` (`tap export images --step k`) | the requested `k` | `true` | `true` |
| `?capture=true&live=true` (`tap export images --wait`) | the requested `k` | `false` | `true` |
| Presenter current-slide panel | current | `false` | `true` |

Fragments follow the same shape: print mode reveals them all, a capture
shows the `--fragment` index it was given.

## Thumbnails

The overview grid and the presenter's next-slide panel render a component
like any other slide content, with `step` forced to `steps`,
`printMode: true`, and `active: false`. A component that starts an
animation or a timer on mount would otherwise run once per thumbnail, all
at the same time, so read `useActive()` or `usePrintMode()` before starting
anything that runs on its own.

To keep a component out of thumbnails, export:

```js
export const preview = false;
```

Tap then shows a small placeholder card with the file name instead. The
export is read with the same static scan as `steps`, so it must be exactly
`export const preview = false`.

## Bundling

Each distinct component path becomes one bundle, built with esbuild from
Go. No Node install is needed.

- Output format: ESM, target ES2022, automatic JSX runtime, TypeScript
  supported.
- JSX always compiles with the automatic React runtime. A `tsconfig.json`
  in or above the deck folder cannot change that: its `jsx` and
  `jsxImportSource` settings are overridden, since `react-jsxdev` would
  look for `react/jsx-dev-runtime`, `react` would need a React global tap
  never defines, and a different `jsxImportSource` would look for a
  library that is not there.
- `tap dev` builds unminified with a linked source map.
- `tap build` builds minified without source maps, and any bundle error
  fails the build.

### Host modules

These bare specifiers are **not** bundled. They resolve to a shim that
reads the host's own instance off `window.__TAP_HOST__`, so the deck
component and tap share one React and one Motion:

- `react`
- `react/jsx-runtime`
- `react-dom`
- `react-dom/client`
- `motion`
- `motion/react`
- `tap`

### npm packages

Any other bare import resolves from a `node_modules` folder next to the
component, or in one of its ancestors. Install packages next to the deck:

```bash
cd my-talk
npm install d3-shape
```

A package that cannot be resolved is a build error that names it:

```
error: charts/LatencyDrop.jsx:2:18: package "d3-shape" not found; run `npm install d3-shape` next to the deck
```

Importing a subpath of a host package that tap does not shim says so
instead of suggesting an install, and lists the entry points that do exist:

```
error: slides/Dev.jsx:1:23: tap provides "react/jsx-dev-runtime" directly; it does not need to be installed. Only these entry points exist: react, react/jsx-runtime, react-dom, react-dom/client, motion, motion/react, tap
```

**Maps.** The built-in ```` ```map ```` fence cannot be used from inside a
component, and it fetches tiles over the network. For a map in a
component, draw it from vector data instead:

```bash
npm install d3-geo topojson-client world-atlas
```

`d3-geo` supplies the projection and path generator and a topojson atlas
supplies the shapes. Both bundle into the component, so nothing is fetched
at run time.

### Data, CSS, and assets

`import data from './rollout.json'` is supported. The JSON is parsed at
build time and bundled into the component, so there is no fetch and no
loading state.

**Where a data or asset file may live.** Code and stylesheets (`.js`,
`.jsx`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.css`) may be imported from outside
the deck folder. A data or asset file (JSON, text, an image, a font) must
resolve inside the deck folder, or under a `node_modules` folder. Symlinks
are followed, so an in-deck symlink pointing outside does not get around
it. A file that does not is a build error, and no bundle is produced:

```
error: slides/Chart.jsx: imports a data file from outside the deck folder: /elsewhere/data.json (copy it into the deck folder)
```

`import './deploy.css'` is supported. esbuild emits a CSS file next to the
bundle, and tap adds one `<link rel="stylesheet" data-deck-component>` for
it, once per URL.

Imported `.png`, `.jpg`, `.jpeg`, `.gif`, `.svg`, `.webp`, and `.woff2`
files are handled by size:

| Size | What happens |
|------|--------------|
| Under 100 KB | Inlined into the bundle as a data URL |
| 100 KB or more | Emitted as its own file and referenced by URL |

An emitted file is named `asset-<hash>.<ext>`; the original file name is
not used, so nothing about your folder layout leaks into a published deck.
It is served by `tap dev` from `/components/`, and written by `tap build`
into `dist/components/` alongside the bundle. An asset referenced from a
component's CSS with `url()` resolves relative to the emitted CSS file, so
it works in `tap dev` and in a static build served from any sub path. Either way the
component just uses the imported value as a `src`; the difference only
shows in the output size.

A large photograph still belongs in the markdown as a normal image rather
than in a component's imports, since the markdown path gives you sizing
attributes and the theme's image styling.

### URLs and output

| Context | Where a bundle lives |
|---------|----------------------|
| `tap dev` | Served at `/components/<name>-<hash>.js` (and `.css`, `.js.map`) |
| `tap build` | Written to `dist/components/<name>-<hash>.js`, referenced relatively |

`<name>` is the entry file's base name without its extension, with anything
outside `A-Za-z0-9_-` replaced. `<hash>` is the first 12 hex characters of
the SHA-256 of the JavaScript bytes followed by the CSS bytes, so the URL
changes whenever the output changes.

### Watching

`tap dev` watches the deck's folder tree and rebuilds a component when it
or anything it imports changes. Two rules matter:

- Directories named `node_modules` or `dist`, any directory whose name
  starts with `.`, and a configured `--output` directory are not watched.
  The rule never applies to the deck's own folder, so a deck that happens
  to live in a folder named `dist` is still watched.
- A file imported from outside the deck's folder tree is watched too: tap
  adds the directories of every input esbuild reported.

## Errors

### Build errors

Every build error prints one line to standard error, in this format:

```
error: <file>:<line>:<column>: <message> (used on slides <n>, <m>)
```

The suffix names every slide that uses the file, so you know where to look:
`(used on slide 2)` for one, `(used on slides 2, 5)` for several.

```
error: slides/Broken.jsx:7:1: Top-level return cannot be used inside an ECMAScript module (used on slides 1, 3)
```

`<file>` is relative to the deck's folder when the file lives inside it,
and absolute otherwise. In `tap build` the errors print and the build exits
with status 1.

### Where the error card shows

A component that fails to build, fails to load, or throws while rendering
is caught by its own error boundary, so one broken inline component never
takes the rest of the slide with it. A slide that fails to render for any
other reason is caught by the slide error boundary.

There are three forms, and which one you get depends on who is looking.

| Form | What is shown |
|------|---------------|
| **Full card** | The source path and the message, in place of the component |
| **Audience-safe** | The fallback content plus a small muted `component error` chip in the top right corner. For a whole-slide component the fallback is the slide's slots in the `default` layout; for an inline one the rest of the slide renders as normal and the chip marks the gap |
| **Silent fallback** | The fallback content, with no card and no chip |

| Context | Form |
|---------|------|
| A normal viewer window, not fullscreen | Full card |
| The presenter view (`/presenter`) | Full card |
| `?debug=true` | Full card |
| `?print=true`, `?capture=true`, `tap export pdf`, `tap export images` | Full card |
| A **fullscreen** viewer | Audience-safe |
| `?present=true` | Audience-safe |
| A static `tap build` output | Silent fallback |

The audience-safe form exists because a live talk otherwise shows a
throwing component's raw error to the whole room. Go fullscreen, or open
the audience window with `?present=true`, and a failure degrades to the
slide's own content with a chip only you will notice. `?debug=true` forces
the full card back on any window, which is what you want while authoring.

Fullscreen is followed **live**: entering or leaving fullscreen while an
error is on screen switches between the two forms straight away, with no
reload.

Even in the audience-safe form the `.deck-error-card` element stays in the
DOM, hidden, carrying the message in `data-message` and the file in
`data-source`. `tap export images` still finds it, still exits 1, and now
reports the message:

```
Error: slide 2 shows an error card: component blew up on purpose
```

`tap export pdf` prints the same message as a warning and still writes the PDF:

```
warning: slide 2 shows an error card: component blew up on purpose
```

A static build's silent fallback is deliberate: a shipped deck stays usable
rather than showing a stack trace. A static build is told apart at run time
by the `#presentation-data` element its `index.html` embeds;
`import.meta.env.DEV` is not the signal, because the embedded frontend is
always a production Vite build.

### The load timeout

A component that has not loaded within **8 seconds** becomes a load error:

```
component did not load within 8 seconds: ./slides/RollingDeploy.jsx
```

Re-entering the slide retries. There is no timeout in print mode, in a
capture, or in a preview, since those have no user waiting on a slide.

### The card itself

The error card is a `<div class="deck-error-card" data-source="...">` for a
component, and `.slide-error` for a slide-level failure. `tap export images`
looks for `.slide-error, .deck-error-card` and exits with status 1 when it
finds one, which is what makes an automated self-check work.

A render error is not retried on every step press: the boundary resets when
the bundle URL or the source path changes, not when the step changes.

A failed **import** is different. When a bundle fails to load, for example
a 404 while `tap dev` is restarting, the rejected import is evicted from
the cache rather than remembered, so leaving the slide and entering it
again retries the import. A reload is no longer needed to recover.

### tap export pdf

`tap export pdf` builds and registers component bundles through the same shared
setup `tap export images` uses, so a PDF renders every component. Each one is
exported in its final state: `printMode = true` and `step = steps`, the
same as a thumbnail.

Failures split by kind:

- **A component that fails to build** stops the export before any browser
  work. `tap export pdf` prints the same
  `error: <file>:<line>:<column>: <message>` line `tap export images` does,
  writes no PDF, and exits with status 1.
- **A slide that shows an error card at export time** (a component that
  throws while rendering, or any slide that fails to render) is still
  written to the PDF, card and all. `tap export pdf` prints
  `warning: slide <n> shows an error card` to standard error, one line per
  affected slide, and exits 0. The rest of the deck exports normally, so a
  handout is never lost to one broken slide.

## The slide canvas

A whole-slide component gets the theme's normal slide padding and chrome,
the same as the `default` layout gets.

Tap wraps the component's own markup in `<div class="deck-component-root">`.
There is no `layout-component` class; the wrapper is what the theme styles.
`layouts.css` gives it a shared baseline:

- `position: relative`, so a child with `position: absolute; inset: 0`
  resolves against this box rather than some distant ancestor
- `width: 100%` and `height: 100%`
- `display: flex; flex-direction: column`
- `border: var(--slide-padding) solid transparent` for the padding. A
  transparent border rather than real padding, because an absolutely
  positioned child's containing block is the padding edge, so real padding
  would be painted over.

What a component author should do:

- **Fill the wrapper; do not pad it.** Give your root `height: 100%` with a
  column flex layout, or fill it absolutely with
  `position: absolute; inset: 0`. Both work. Never add an outer margin or
  padding of your own: it would sit inside the theme's padding.
- **Render the title through `Slot`** so it gets the theme's own heading
  treatment rather than yours.
- **`tag` and `badge` directives still render**, drawn by the theme outside
  your root.
- **Allow for per-theme chrome.** Some themes put a title block, a page
  number, a running head, or a HUD band on the slide, so the usable area
  differs per theme.

The measured safe area for every theme is in
[Creating Themes](/reference/theme-porting#component-slides), under
"Component slides". Do not design to those numbers; check the themes you
care about:

```bash
tap export images deck.md --slide 3 --theme terminal --output terminal.png
tap export images deck.md --slide 3 --theme zine --output zine.png
```

## Auto-playing components

A component may run its own timeline rather than waiting for a press, as
long as every timer is gated on `useActive()` being true and
`usePrintMode()` being false, and cleared on cleanup. The presenter's
next-slide panel and the overview grid mount the same component, so an
ungated `setInterval`, `repeat: Infinity`, or recursive
`requestAnimationFrame` runs once per thumbnail, all at the same time.

Export `steps = 1` for an auto-playing component:

| Press | Result |
|-------|--------|
| (slide appears) | Step 0. The timeline plays on its own. |
| First press | Step 1. Render the final state at once, no animation. |
| Second press | The next slide. |

With no `steps` export the slide has zero steps, so the first press leaves
the slide mid-animation. Fold print mode and "already pressed" into one
condition so there is a single code path for the settled state:

```jsx
const done = printMode || step >= 1;

useEffect(() => {
	if (done || !active) return;
	const id = setInterval(tick, 700);
	return () => clearInterval(id);
}, [done, active]);
```

Check both halves with `tap export images --step 0` and `--step 1`.

## Motion gotchas

Tap embeds **Motion 12** (`motion` `^12.23.24`, currently resolving to
12.43.0). Everything here was reproduced against that version with
`tap export images --wait`, and rechecked after the mount-animation fix.

### `animate()`'s `delay` runs `onUpdate` with the start value

This one is real and easy to hit. The imperative `animate(from, to, ...)`
calls `onUpdate(from)` on **every frame during its delay**, not just after
it. Two staggered animations writing to the same piece of state therefore
fight: the delayed one keeps resetting the display to its own `from` while
the first is still running.

```jsx
// Broken: the delayed call holds the counter at 0 for two seconds,
// overwriting the first animation the whole time.
animate(0, 100, { duration: 0.8, onUpdate: (v) => setCount(Math.round(v)) });
animate(0, 999, { duration: 0.8, delay: 2, onUpdate: (v) => setCount(Math.round(v)) });
```

Measured at 1.2 seconds, the counter above reads `0`; it should read `100`.

**Workaround: start the delayed animation from a `setTimeout` instead of
using Motion's `delay`.**

```jsx
animate(0, 100, { duration: 0.8, onUpdate: (v) => setCount(Math.round(v)) });
const id = setTimeout(() => {
	animate(0, 999, { duration: 0.8, onUpdate: (v) => setCount(Math.round(v)) });
}, 2000);
// clear id on cleanup
```

With the timeout, the same probe reads `100` at 1.2 seconds. The rule
generalizes: if several `animate()` calls share one setter, do not give any
of them a Motion `delay`.

### Reported and not reproduced on this version

These were reported on an earlier state of the branch and do **not**
reproduce on Motion 12.43.0 with the current frontend. They are listed so
nobody re-adds a workaround for them:

| Report | Result |
|--------|--------|
| An SVG motion element that mounts with a keyframe array, or with `repeat`, renders its last keyframe and never runs | Not reproduced. Both animate normally from mount. This was the presence-context bug, fixed by the mount-animation reset. |
| Keyframe arrays on SVG `cx` and `cy` jump to the last keyframe | Not reproduced. Both interpolate smoothly. |
| A transition with `duration: 0` plus a `delay` applies at once and ignores the delay | Not reproduced. The delay is honored: a `duration: 0, delay: 2` opacity stays at 0 through 1.5 seconds and flips to 1 after 2. |

If you hit one of these after a Motion upgrade, reproduce it with a scratch
component and `tap export images --wait` before working around it.

## Slide JSON

For reference when reading a built `index.html`, the transformer adds:

```jsonc
{
  "layout": "component",
  "component": {
    "source": "./slides/RollingDeploy.jsx",
    "url": "components/RollingDeploy-179a23f87880.js",
    "css": "components/RollingDeploy-179a23f87880.css",  // only when the component imports CSS
    "error": "..."                                        // only when the bundle failed
  },
  "components": [
    {
      "index": 0,
      "source": "./charts/LatencyDrop.jsx",
      "url": "components/LatencyDrop-51eed2583b00.js",
      "props": { "before": 412, "after": 88 }
    }
  ],
  "steps": 4
}
```

## Type declarations

`tap component new --ts` writes two files next to the deck, each only when
it does not already exist:

- `tap-env.d.ts`: declares the `tap` module, every helper it exports
  (`useStep`, `usePrintMode`, `useActive`, `useTheme`, `Step`, `Slot`,
  `textOn`), `ThemeTokens`, and `DeckComponentProps`. Edit it freely.
- `tap-shims.d.ts`: stand-in declarations for `motion/react`,
  `react/jsx-runtime`, and a permissive JSX namespace, so `tsc` passes with
  nothing installed. It is skipped when `node_modules/@types/react` exists
  next to the deck or above it. Delete the whole file once you install real
  React types.

## Security

A deck component is code that the deck's author chose to run. It has the
same trust as raw HTML written into the deck. Tap never fetches component
code from the network: every bundle is built from files on disk, and a
static build contains the bundled code. Only build decks you trust.

## See also

- [Custom Components](/guide/custom-components) - the tutorial
- [CLI Commands](/reference/cli-commands) - `tap export images`, `tap component new`, `tap theme`
- [Themes](/guide/themes) - the token list and each theme's illustration style
- [Creating Themes](/reference/theme-porting#component-slides) - the per-theme safe area for a component slide
- [Layouts Reference](/reference/layouts-reference) - the built-in layouts and their slots
- [Keyboard Shortcuts](/reference/keyboard-shortcuts#url-parameters) - `?step=`, `?fragment=`, `?print=true`, `?capture=true`

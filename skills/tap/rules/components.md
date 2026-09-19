# Deck Components

A deck can supply its own React components from files next to the markdown.
Use one when a slide needs a picture that moves or a diagram that builds up
over several clicker presses. Everything else belongs in markdown.

Tap bundles the file itself, with esbuild, from Go. No Node install, no
build step, no network.

## Start from the scaffold, not a blank file

```bash
tap add component RollingDeploy --deck deck.md   # slides/RollingDeploy.jsx
tap add component LatencyDrop --inline --deck deck.md   # components/LatencyDrop.jsx
```

Flags: `--inline` for a block inside a normal slide, `--ts` for `.tsx` plus
`tap-env.d.ts` and `tap-shims.d.ts`, `--deck <file>` to resolve folders
relative to that deck. `<Name>` must be PascalCase. The command refuses to
overwrite an existing file and prints the markdown snippet to paste.

## Two authoring forms

Whole slide, through the `layout:` directive. A layout value that starts
with `./` or `../` and ends in `.jsx`, `.tsx`, `.js`, or `.ts` is a
component path, relative to the deck file:

```markdown
<!--
layout: ./slides/RollingDeploy.jsx
-->

# Zero-downtime deploys

Four app servers behind one load balancer.

::caption
One restarts at a time.
```

The step count comes from the component's `export const steps`; do not
also set a `steps:` directive (see Steps below).

Inline, through a fence. The body is an optional JSON object passed as
`props`:

````markdown
```component ./charts/LatencyDrop.jsx
{ "before": 412, "after": 88 }
```
````

The entry file must live inside the deck's folder. Code and stylesheets it
imports may live outside; a data or asset file (JSON, text, an image, a
font) must live inside the deck folder.

## The contract

The default export is a React component. Tap passes one object:

```ts
{
  slots: Record<string, string>;   // slot HTML; {} for an inline component
  props: Record<string, unknown>;  // the fence body's JSON; {} whole-slide
  slide: Slide;
  step: number;                    // 0..steps
  steps: number;
  active: boolean;                 // false in every thumbnail
  printMode: boolean;              // true: final state, no animation
}
```

Import helpers from the bare specifier `tap`:

| Export | Use |
|--------|-----|
| `useStep()` | `{ step, steps }` |
| `usePrintMode()` | `true` for PDF, `?print=true`, and thumbnails |
| `useActive()` | `true` only while the slide is on screen |
| `useTheme()` | The active theme's tokens, as an object |
| `<Step at={n}>`, `<Step from={a} to={b}>` | Reveal on a step |
| `<Slot html={slots.default} />` | Render a slot's markdown, fragments included |
| `textOn(fill, theme)` | `theme.bg` or `theme.fg`, whichever contrasts more with `fill` |

Only these bare imports are allowed besides packages installed in a
`node_modules` next to the deck: `react`, `react/jsx-runtime`, `react-dom`,
`react-dom/client`, `motion`, `motion/react`, `tap`. Never install React or
Motion next to the deck; they come from tap so the component shares one
instance.

Relative imports of `.json` and `.css` work too:

```jsx
import data from './rollout.json';   // parsed at build time, bundled in
import './deploy.css';               // emitted next to the bundle
```

**Maps:** the built-in ```` ```map ```` fence cannot be used from a
component, and it needs network tiles. Draw a map from vector data
instead: `npm install d3-geo topojson-client world-atlas` next to the deck,
then project the atlas yourself. Everything bundles in, nothing is
fetched at run time.

## The color rule

Get this wrong and the slide is unreadable in half the themes.

**`theme.accent` is a FILL color.** Several themes pick an accent that
cannot be read as text on their own background: `zine`'s accent is
`#ffe600` on an `#e8e5dd` page.

| Painting | Use |
|----------|-----|
| A filled shape, bar, or box on the slide | `theme.accent` |
| Text or a thin line on `theme.bg` | `theme.accentText`, or `currentColor` |
| Text on top of an accent fill | `textOn(theme.accent, theme)` |
| A failed, down, or inactive state | `theme.muted` + dashed outline + lower opacity |

`textOn(fill, theme)` returns whichever of `theme.bg` and `theme.fg` has
the higher contrast ratio against `fill`. It accepts `#rgb`, `#rrggbb`,
`#rrggbbaa`, `rgb(...)`, and `rgba(...)`, and returns `theme.fg` for
anything else, including the empty string a token holds before the
component's root has mounted.

```jsx
import { textOn, useTheme } from 'tap';

const theme = useTheme();

<div style={{ background: theme.accent, color: textOn(theme.accent, theme) }}>v2</div>
```

Three more rules that follow:

- `useTheme()` has **exactly 14 keys** and **no second accent**.
- To use a theme's extra variable, read the CSS variable directly, always
  with a fallback: `var(--accent-2, var(--accent-text))`. Any theme
  variable can be read this way.
- There is **no token for a failed or down state**. Build one from
  `muted`, `dashed`, and opacity.

## The slide canvas

A whole-slide component gets the theme's normal slide padding and chrome,
the same as the `default` layout. Tap wraps the component in
`.deck-component-root`, which already supplies that padding and is a
full-height column flex container.

- Give your root `height: 100%` with a column flex layout, or fill it with
  `position: absolute; inset: 0`. Both work.
- **Never add outer padding or margin of your own.** It sits inside the
  theme's padding.
- **Render the title through `Slot`**, so it matches every other slide.
- `tag` and `badge` directives still render, drawn by the theme outside
  your root.
- Themes put different chrome on a slide (title blocks, page numbers, HUD
  bands), so the usable area differs. Per-theme safe areas are in
  `docs/reference/theme-porting.md` under "Component slides"; do not design
  to the numbers, check with `tap screenshot --theme <slug>`.

## Working example

This is the shipped example, `examples/components/slides/RollingDeploy.jsx`,
in pieces. Read the whole file before writing your own.

Imports, the step count, and the constants:

```jsx
import { Slot, Step, textOn, useActive, usePrintMode, useStep, useTheme } from 'tap';
import { motion } from 'motion/react';
import { useLayoutEffect, useState } from 'react';

export const steps = 5;

const SERVER_COUNT = 4;
const DRAIN_MS = 450;
const RESTART_MS = 950;
```

Everything visible is a pure function of `step`, so jumping straight to a
step with `?step=` or `tap screenshot --step` looks the same as clicking
there:

```jsx
/** The server at `index`'s version and state for the given step and roll phase. */
function serverState(index, step, phase) {
	const rollingIndex = step - 1;
	if (index < rollingIndex) return { version: 'v2', state: 'serving' };
	if (index > rollingIndex) return { version: 'v1', state: 'serving' };
	if (phase === 'serving') return { version: 'v2', state: 'serving' };
	return { version: 'v1', state: phase };
}
```

The color rule in miniature. Accent as a background, `accentText` as the
border, `textOn` for the text on that fill, and `muted` plus a dashed
outline for the down state:

```jsx
/** Box styling for one server, by its state and theme tokens. */
function boxStyle(state, isUpgraded, theme, textOnAccent) {
	const base = { flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', borderRadius: theme.radius, fontFamily: theme.fontMono, textAlign: 'center' };
	if (isUpgraded) return { ...base, background: theme.accent, border: `calc(${theme.strokeWidth} * 2) solid ${theme.accentText}`, color: textOnAccent };
	if (state !== 'serving') return { ...base, background: 'transparent', border: `${theme.strokeWidth} dashed ${theme.muted}`, color: theme.muted };
	return { ...base, background: theme.surface, border: `${theme.strokeWidth} solid ${theme.muted}`, color: theme.fg };
}
```

Hooks rather than props, so a helper deeper in the tree can read the same
values without threading props down:

```jsx
export default function RollingDeploy({ slots }) {
	const { step, steps: totalSteps } = useStep();
	const active = useActive();
	const printMode = usePrintMode();
	const theme = useTheme();
	const rollingIndex = step - 1;
	const [phase, setPhase] = useState('serving');
	const textOnAccent = textOn(theme.accent, theme);
```

A timeline inside one step: a **layout** effect so a press never paints a
frame of the previous step, gated on `active` and `printMode`, and cleaned
up:

```jsx
	// A layout effect, not a plain effect: it resets `phase` to 'draining'
	// before the browser paints the new step, so a press never paints one
	// frame of the previous step's 'serving' box for the server that is
	// about to drain.
	useLayoutEffect(() => {
		if (!active || printMode || rollingIndex < 0 || rollingIndex >= SERVER_COUNT) {
			setPhase('serving');
			return;
		}
		setPhase('draining');
		const toRestart = setTimeout(() => setPhase('restarting'), DRAIN_MS);
		const toServing = setTimeout(() => setPhase('serving'), RESTART_MS);
		return () => {
			clearTimeout(toRestart);
			clearTimeout(toServing);
		};
	}, [step, active, printMode]);
```

The root fills the wrapper and adds no padding of its own; the title comes
through `Slot`:

```jsx
	return (
		<div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', gap: theme.spaceUnit, fontFamily: theme.fontBody, color: theme.fg }}>
			<Slot html={slots.default} />

			<div style={{ display: 'flex', flexDirection: 'column', gap: `calc(${theme.spaceUnit} / 2)`, flex: 1, minHeight: 0 }}>
```

A reveal tied to the export rather than a repeated literal:

```jsx
				{/* Only visible once the clicker reaches the slide's last step
				    (and always in print mode, which shows the final state). */}
				<Step at={totalSteps}>
					<div style={{ fontSize: '1.5rem', color: theme.accentText, textAlign: 'center' }}>Rollout complete. Every server now serves v2.</div>
				</Step>
			</div>

			<div style={{ fontSize: '2.25rem' }}>
				<Slot html={slots.caption} />
			</div>
		</div>
	);
}
```

## Steps

Resolution order: the slide's `steps:` directive, then a static
`export const steps = N` in the file, then 0.

**Do not set both.** The directive overrides the export silently, with no
warning, so a component whose step count later changes keeps consuming the
directive's number. Prefer the export, so the count lives next to the code
that defines it. Tap reads the export with a
pattern match over the parsed source and never runs the component, so it
must be exactly `export const steps = <integer>`. A computed value, an
expression, or several names in one declaration are ignored. Never write
the text `export const steps = 3` inside a string literal: a lookalike
still matches.

With several inline components on one slide, the slide's steps are the
largest of theirs unless the directive sets it.

## Auto-playing components

A component may run its own timeline instead of waiting for a press.

```jsx
export const steps = 1;

const done = printMode || step >= 1;

useEffect(() => {
	if (done || !active) return;
	const id = setInterval(tick, 700);
	return () => clearInterval(id);
}, [done, active]);
```

Rules, all mandatory:

- Gate every timer on `useActive()` true and `usePrintMode()` false.
- Clear every timer in the effect's cleanup.
- Render the final state at once when print mode or the settled step is
  reached. Folding both into one `done` flag keeps it to one code path.
- **Export `steps = 1`.** Step 0 plays on its own; the first press jumps to
  step 1, which shows the final state at once; the next press leaves the
  slide. With no `steps` export the first press leaves the slide
  mid-animation with no way to show the conclusion.
- **Never loop forever without the `active` gate.** The presenter view and
  the overview mount the same component, so `repeat: Infinity`,
  `setInterval`, and recursive `requestAnimationFrame` all run once per
  thumbnail, simultaneously.

Prove both halves: `tap screenshot deck.md --slide N --step 0` and
`--step 1`.

## Print mode

**`printMode = true` means "render the settled state of the CURRENT
`step`, no animation, no timers". It does NOT mean "show the last step".**

For `tap pdf`, `?print=true`, and previews the current step is `steps`, so
they do show the final state. But `tap screenshot --step 2` also sets
`printMode = true`, with `step = 2`. A component that branches on
`printMode` to jump to its last step produces a wrong stepped capture.

```jsx
const shown = printMode ? LAST : step;   // wrong
const shown = step;                       // right
const duration = printMode ? 0 : 0.3;     // printMode decides animation only
```

`Step` compares against the current `step` in every mode. `<Step at={n}>`
is unaffected in print, since `step` is the total there. `<Step from={a}
to={b}>` whose `to` is below the total is **hidden** in a PDF and in
previews. Content that must appear in the PDF belongs in an `at` Step, or
outside a `Step`.

What a component receives:

| Mode | `step` | `printMode` | `active` |
|------|--------|-------------|----------|
| Live viewer | current | `false` | `true` |
| `?print=true`, `tap pdf` | `steps` | `true` | `true` |
| Preview (overview, presenter next panel) | `steps` | `true` | `false` |
| `tap screenshot --step k` | `k` | `true` | `true` |
| `tap screenshot --step k --wait` | `k` | `false` | `true` |
| Presenter current panel | current | `false` | `true` |

## Thumbnails

The overview grid, the presenter's next-slide panel, PDF export, and
`?print=true` render with `printMode: true`, `step` forced to `steps`, and
`active: false`. Always:

- Pass `transition={{ duration: printMode ? 0 : 0.3 }}` and
  `initial={printMode ? false : {...}}` to Motion.
- Check `useActive()` or `usePrintMode()` before starting a timer or an
  animation on mount. Thumbnails mount every slide at once.

`export const preview = false` replaces the component with a small
placeholder card in thumbnails only.

## Theme tokens

`useTheme()` returns strings read from the slide's CSS custom properties,
updated live when the theme changes: `bg`, `fg`, `muted`, `accent`,
`accentText`, `surface`, `fontDisplay`, `fontBody`, `fontMono`, `ease`,
`dur`, `spaceUnit`, `radius`, `strokeWidth`.

The tokens are read in a layout effect, so the first painted frame already
has real values. A token the theme does not define reads as an empty
string; `textOn` tolerates that and falls back to `theme.fg`.

Read the same values, and the theme's illustration style, on the command
line:

```bash
tap theme show blueprint
tap theme show blueprint --json
tap theme show blueprint --prompt     # style brief for an image model
tap theme show --deck deck.md --prompt
```

Use `--prompt` in front of an image request so generated illustrations
match the deck's theme.

## Mount animations

A component's own `initial` to `animate` transition runs even on the first
slide a page loads on: tap resets Motion's presence context around every
deck component. A component may nest its own `AnimatePresence`.

## Working in a browser or with a dev server

- The hub remembers the last presenter state for 10 minutes and **beats
  `?step=N`** when the hash names the same slide. Use
  `http://localhost:<port>/?capture=true&step=2#3`, or start the server
  with `TAP_HUB_STATE_RETENTION=0s`.
- **Any file change under the deck folder reloads the page** and remounts
  components, restarting their animations. Write screenshots and scratch
  files outside the deck folder, or into a dot folder or one named `dist`,
  which the watcher skips.
- Give each `tap dev` its own `--port` when several run. The default port
  falls forward to the next free one, so nothing breaks if you forget, but
  you will not know which server you are looking at.
- Check end states with `--step k`, moments with `--wait`.

## Motion gotchas

Tap embeds Motion 12 (currently 12.43.0).

- **`animate()`'s `delay` calls `onUpdate` with the start value on every
  frame of the delay.** Two staggered `animate()` calls writing to one
  setter fight: the delayed one pins the value to its own `from` while the
  first is still running. Start the delayed one from a `setTimeout`
  instead of passing Motion a `delay`.
- Reported and **not** reproducible on this version, so do not work around
  them: SVG motion elements mounting with a keyframe array or `repeat`
  (they animate correctly since the presence-context fix); keyframe arrays
  on SVG `cx`/`cy`; `duration: 0` with a `delay` (the delay is honored).

## Check your own work with tap screenshot

Never claim a component works without rendering it.

```bash
tap screenshot deck.md --slide 2 --out check.png            # final state
tap screenshot deck.md --slide 2 --step 2 --out step-2.png  # one step
tap screenshot deck.md --all --out shots/                   # every slide
```

With neither `--step` nor `--fragment`, the slide renders its final state
in print mode. With either, it renders that exact step, **settled**:
animations and timers finished rather than caught partway.

`--wait <ms>` (0 to 60000) keeps the capture live and waits that long after
the page is **ready** (network idle, fonts, running animations finished),
not after navigation. A short mount animation is over before the wait
starts, so `--wait` suits timer-driven or long animations.

**The exit status is the check.** `tap screenshot` exits 1, with a message
on standard error, when the deck is missing, a slide, step, or fragment is
out of range, the theme is unknown, a component fails to build, the
browser cannot start, or the rendered slide shows a slide or component
error card. On success it prints only the paths it wrote.

`--all` does not stop at the first broken slide: it captures what it can,
then prints one `slide N: <reason>` line per broken slide to standard
error and exits 1. It cannot be combined with `--slide`, `--step`, or
`--fragment`.

```bash
tap screenshot deck.md --slide 2 --out check.png || echo "slide 2 is broken"
```

`tap build deck.md` also exits 1 on any component build error, so a build
is a second, cheaper check.

## Validation

- `layout: component` with no path is an unknown-layout error. The
  `component` layout is only reachable through a real file path.
- A `steps:` directive that is negative or not an integer is ignored with a
  warning naming the slide.
- A `component` fence path may contain spaces.
- A bad props JSON names the slide and the real line in the deck file.
- Code and stylesheets may be imported from outside the deck folder. A data
  or asset file (JSON, text, image, font) must resolve inside it or under
  `node_modules`; symlinks are followed.
- JSX always compiles with the automatic React runtime, whatever a nearby
  `tsconfig.json` says.

## Errors

Build errors print one line each to standard error:

```
error: slides/RollingDeploy.jsx:12:8: Expected ")" but found "}"
```

A build error with no source position omits it rather than printing
`:0:0`. A failed bundle **import** is retried the next time the slide is
entered, so a 404 during a `tap dev` restart clears itself without a
reload.

A missing npm package names itself and the command to run:

```
error: charts/LatencyDrop.jsx:2:18: package "d3-shape" not found; run `npm install d3-shape` next to the deck
```

The affected slide shows an error card in `tap dev`, `tap pdf`, and
`tap screenshot` alike, and the rest of the deck keeps working. `tap build`
fails outright on a build error. Only a static `tap build` output falls
back instead of showing a card: a whole-slide component renders the slide's
slots with the `default` layout, an inline one leaves the slot's raw
content, so a shipped deck stays usable on stage.

`tap pdf` builds components too, and exports each in its final state
(`printMode = true`, `step = steps`). A build error stops it with the same
`error:` line and exit status 1. A slide that shows an error card is still
written to the PDF; `tap pdf` prints `warning: slide <n> shows an error
card` to standard error and exits 0.

## Rules of thumb

- Markdown holds the words. A component that renders a bullet list is the
  wrong tool.
- One component per idea, small enough to read in one screen.
- Use theme tokens, never hard-coded colors or fonts.
- Give every step something to change. A four-step component that looks the
  same at step 2 and step 3 wastes a press.

Full reference: `docs/reference/components-reference.md`. Tutorial:
`docs/guide/custom-components.md`.

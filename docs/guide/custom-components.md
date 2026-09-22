---
title: Custom Components
---

# Custom Components

Markdown holds the words. When a slide needs a picture that moves, a deck
can supply its own React component from a file next to the deck. Tap
bundles it, gives it the slide's step count and the theme's tokens, and
renders it inside the slide like any layout.

There are two forms:

- **Whole slide**: the component replaces the layout and decides what the
  slide looks like.
- **Inline**: the component renders one block inside an otherwise normal
  markdown slide, with JSON props.

This guide walks through both, line by line, using the deck that ships in
[`examples/components/`](https://github.com/MiniCodeMonkey/tap/tree/main/examples/components).
Every block below is the real file, so you can read along with it open:

```
examples/components/
  deck.md
  slides/RollingDeploy.jsx    whole slide, five steps
  charts/LatencyDrop.jsx      inline, JSON props
```

The visualization is a rolling deploy: four app servers behind a load
balancer, one draining and restarting on the new version per step while
the other three keep serving.

## Scaffold a component

`tap component new` writes a working starting point rather than an empty
file:

```bash
tap component new RollingDeploy deck.md
```

It prints the file it wrote and the markdown snippet to paste into the
deck:

```
slides/RollingDeploy.jsx

<!--
layout: ./slides/RollingDeploy.jsx
-->

# Title
```

Useful flags:

| Flag | Effect |
|------|--------|
| `--inline` | Writes `components/<Name>.jsx` and prints a ```` ```component ```` fence instead |
| `--ts` | Writes a `.tsx` file, plus `tap-env.d.ts` and `tap-shims.d.ts` next to the deck |

`[deck]` is a deck file or a deck folder that the component's folders are
resolved relative to; the default is the current directory.

`<Name>` must be PascalCase. The command refuses to overwrite an existing
file.

## The whole-slide component

Set the `layout:` directive to a path that starts with `./` and ends in
`.jsx`, `.tsx`, `.js`, or `.ts`. That is what tells tap it is a component
and not a layout name. From `examples/components/deck.md`:

```markdown
<!--
layout: ./slides/RollingDeploy.jsx
-->

# The Rollout

::caption
Each server drains, restarts on the new version, and rejoins before the next one starts.
```

Note what is **not** there: a `steps:` directive. The count lives in the
component file, and setting both would let them drift apart silently.

### Imports and the step count

```jsx
import { Slot, Step, textOn, useActive, usePrintMode, useStep, useTheme } from 'tap';
import { motion } from 'motion/react';
import { useLayoutEffect, useState } from 'react';

export const steps = 5;

const SERVER_COUNT = 4;
const DRAIN_MS = 450;
const RESTART_MS = 950;
```

`export const steps = 5` tells tap the slide consumes five clicker
presses. Tap reads it statically, without running the file, so it must be
exactly that shape: `export const steps = <integer>`.

::: warning Do not also set a `steps:` directive
The directive overrides this export **silently**. If the component later
grows a sixth step, a deck that pinned `steps: 5` keeps consuming five, and
the sixth press does nothing. Keep the count in the file, next to the code
that defines it.
:::

Note what is imported. `tap` and `motion/react` are **host modules**: they
resolve to tap's own instances rather than being bundled, so the component
shares one React and one Motion with the app. `react` itself is a host
module too, which is why `useEffect` and `useState` are available.

### Deriving state from the step

Steps are a number, not a sequence of events. Everything visible is a pure
function of `step`, so jumping straight to step 4 with `?step=4` or
`tap export images --step 4` looks exactly like clicking there.

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

`phase` is the short-lived animation state within one step, covered
[below](#running-a-timeline-inside-one-step).

### The color rule

This is the part most hand-written components get wrong.

**`theme.accent` is a fill color.** Text and thin lines that sit directly
on `theme.bg` must use `theme.accentText` or `currentColor`, never
`theme.accent`. Several themes have a pale accent that is unreadable as
text on their background: `zine`'s accent is `#ffe600` on a `#e8e5dd`
page, which disappears. Its `accentText` is `#0d0d0d`, which does not.

**Text painted on top of an accent fill** is the other direction, and
there is no single right answer: whether `bg` or `fg` reads better on the
accent depends on the theme. The `tap` module measures it for you:

```ts
textOn(fill: string, theme: ThemeTokens): string
```

It returns whichever of `theme.bg` and `theme.fg` has the higher contrast
ratio against `fill`. It accepts `#rgb`, `#rrggbb`, `#rrggbbaa`,
`rgb(...)`, and `rgba(...)`, and returns `theme.fg` for anything else,
including the empty string every token holds before the component's root
has mounted. So `textOn(theme.accent, theme)` is safe to call on the first
render.

```jsx
/** Box styling for one server, by its state and theme tokens. */
function boxStyle(state, isUpgraded, theme, textOnAccent) {
	const base = { flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', borderRadius: theme.radius, fontFamily: theme.fontMono, textAlign: 'center' };
	if (isUpgraded) return { ...base, background: theme.accent, border: `calc(${theme.strokeWidth} * 2) solid ${theme.accentText}`, color: textOnAccent };
	if (state !== 'serving') return { ...base, background: 'transparent', border: `${theme.strokeWidth} dashed ${theme.muted}`, color: theme.muted };
	return { ...base, background: theme.surface, border: `${theme.strokeWidth} solid ${theme.muted}`, color: theme.fg };
}
```

Read `boxStyle` as the rule in miniature. The upgraded box uses
`theme.accent` as a **background** and `theme.accentText` as its border;
its text color is whatever `textOn` picked. The restarting box uses no
accent at all.

For a **state**, reach for the status tokens instead:
`theme.statusOk`, `theme.statusWarn`, `theme.statusError`. Each is a fill,
readable at 3:1 or better against the background in every theme, and the
three are stepped apart in lightness as well as hue. Spend them only where
the color means something, and use `textOn(theme.statusError, theme)` for a
label on top of one.

**Never signal status by color alone.** Pair the color with a label, an
icon, or a shape, so the meaning survives a colorblind viewer and a
washed-out projector.

A server that is merely restarting is not a verdict, which is why the
example keeps `theme.muted` with a dashed outline and lower opacity for
that case. Save `statusError` for something that actually failed.

### The component body

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

The step, the active flag, and print mode all arrive as props too. Reading
them through `useStep()`, `useActive()`, and `usePrintMode()` is the
idiomatic form, because a helper deeper in the component tree can call the
same hook instead of having props threaded down to it.

`textOn` is computed once here and passed into `boxStyle`, which keeps the
styling helper a pure function of its arguments.

### Running a timeline inside one step

Each step is a small animation: the server drains, then restarts, then
serves again. Two timers drive it, and every one of them is gated.

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

Four things make this safe:

- **It is a `useLayoutEffect`, not a `useEffect`.** The phase resets before
  the browser paints the new step, so a press never shows one frame of the
  previous step's state for the server that is about to drain.
- **`!active || printMode` bails out.** A preview render (the overview
  grid, the presenter's next-slide panel) mounts every slide at once. An
  ungated timer would fire once per thumbnail, simultaneously.
- **The cleanup clears both timers**, so changing step mid-animation
  cannot leave a stale timer to land later.
- **Print mode never starts it**, and `serverState` returns the settled
  result for whatever `step` is current, so that step renders in one
  frame.

### Filling the slide

```jsx
	return (
		<div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', gap: theme.spaceUnit, fontFamily: theme.fontBody, color: theme.fg }}>
			<Slot html={slots.default} />

			<div style={{ display: 'flex', flexDirection: 'column', gap: `calc(${theme.spaceUnit} / 2)`, flex: 1, minHeight: 0 }}>
```

The root fills its container and lays out as a column. It adds **no outer
padding of its own**: the theme already supplied the slide's padding and
chrome. See [The slide canvas](#the-slide-canvas) below.

The title comes through `<Slot html={slots.default} />` rather than being
drawn by the component, so it inherits the theme's own heading treatment.

### The load balancer and the connectors

```jsx
				<div
					style={{
						textAlign: 'center',
						padding: `calc(${theme.spaceUnit} / 2) ${theme.spaceUnit}`,
						border: `${theme.strokeWidth} solid ${theme.accentText}`,
						borderRadius: theme.radius,
						fontFamily: theme.fontMono,
						fontSize: '1.75rem',
						color: theme.accentText
					}}
				>
					load balancer
				</div>
```

The bar sits directly on the slide background, so its border and its text
are `theme.accentText`, not `theme.accent`.

```jsx
				{/* One connector per server: solid with a traveling request dot
				    while serving, dashed while draining or restarting. */}
				<div style={{ display: 'flex', height: `calc(${theme.spaceUnit} * 2)` }}>
					{Array.from({ length: SERVER_COUNT }, (_, index) => {
						const connected = serverState(index, step, phase).state === 'serving';
						return (
							<div key={index} style={{ flex: 1, position: 'relative', display: 'flex', justifyContent: 'center' }}>
								<div style={{ width: 0, height: '100%', borderLeft: `${theme.strokeWidth} ${connected ? 'solid' : 'dashed'} ${theme.muted}`, opacity: connected ? 1 : 0.5 }} />
								{connected && !printMode && active && (
									<motion.div
										aria-hidden="true"
										style={{ position: 'absolute', top: 0, width: '0.6rem', height: '0.6rem', borderRadius: '50%', background: theme.accentText }}
										animate={{ top: ['0%', '90%'], opacity: [0, 1, 0] }}
										transition={{ duration: 0.9, repeat: Infinity, ease: 'linear' }}
									/>
								)}
							</div>
						);
					})}
				</div>
```

The traveling request dot is gated on `!printMode && active` for the same
reason the timers are, and it is `theme.accentText` because a thin line on
the background is text-like, not a fill.

### The servers

```jsx
				<div style={{ display: 'flex', gap: theme.spaceUnit, flex: 1, minHeight: 0 }}>
					{Array.from({ length: SERVER_COUNT }, (_, index) => {
						const { version, state } = serverState(index, step, phase);
						const isUpgraded = version === 'v2' && state === 'serving';
						const isRestarting = state === 'restarting';
						const pulsing = isRestarting && !printMode && active;

						return (
							<motion.div
								key={index}
								data-testid={`server-${index}`}
								data-version={version}
								data-state={state}
								style={boxStyle(state, isUpgraded, theme, textOnAccent)}
								animate={pulsing ? { opacity: [0.35, 0.85, 0.35] } : { opacity: 1 }}
								transition={pulsing ? { duration: 0.9, repeat: Infinity } : { duration: 0.2 }}
							>
								{isRestarting ? (
									<div style={{ fontSize: '1.5rem' }}>restarting&hellip;</div>
								) : (
									<>
										<div style={{ fontSize: isUpgraded ? '3.25rem' : '2rem', fontWeight: isUpgraded ? 'bold' : 'normal' }}>{version}</div>
										<div style={{ fontSize: '1.5rem', marginTop: `calc(${theme.spaceUnit} / 4)` }}>server {index} &middot; {state}</div>
									</>
								)}
							</motion.div>
						);
					})}
				</div>
```

`data-testid`, `data-version`, and `data-state` are there for the
end-to-end tests. They cost nothing and make a component checkable without
comparing pixels.

### Revealing on the last step

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

`<Step at={totalSteps}>` shows its children from that step onward, and
`totalSteps` came from `useStep()`, so the reveal follows the export rather
than repeating the number 5.

`Step` compares against the current `step` in every mode, print mode
included. For `<Step at={n}>` that changes nothing, since print mode and
previews set `step` to the slide's total. For `<Step from={a} to={b}>`
it does: a range whose `to` is below the total is **hidden** in a PDF, in
`?print=true`, and in every preview.

::: warning Content that must appear in the PDF
Put it in an `at` Step, or outside a `Step` entirely. A
`<Step from={1} to={3}>` on a five-step slide is right on stage and
missing from the handout.
:::

The caption slot renders last, again through `Slot`.

## The inline component

An inline component is a fenced block whose info string is `component`
followed by the path. The fence body is an optional JSON object, passed to
the component as `props`. From `deck.md`:

````markdown
## What It Bought Us

```component ./charts/LatencyDrop.jsx
{ "before": 412, "after": 88, "unit": "ms", "label": "p95 latency" }
```

<!-- pause -->

- No error budget spent on the deploy itself
- No page from an on-call engineer
````

The component renders where the fence was, so it takes part in fragments
and slot placement like any other block. Note the order: the chart sits
**before** the first `<!-- pause -->`, so it is visible and animating as
the slide arrives. Behind a pause it would still mount with the slide,
animate while hidden, and then be revealed already finished.

```jsx
import { useActive, usePrintMode, useTheme } from 'tap';
import { motion } from 'motion/react';

const MAX_BAR_HEIGHT = 220;

export default function LatencyDrop({ props }) {
	const active = useActive();
	const printMode = usePrintMode();
	const theme = useTheme();

	const before = props.before ?? 0;
	const after = props.after ?? 0;
	const unit = props.unit ?? 'ms';
	const label = props.label ?? 'latency';
	const scale = MAX_BAR_HEIGHT / Math.max(before, after, 1);
	const shown = active || printMode;

	const bars = [
		{ key: 'before', value: before, color: theme.muted },
		{ key: 'after', value: after, color: theme.accent }
	];
```

```jsx
		<div style={{ fontFamily: theme.fontBody, color: theme.fg }}>
			<div style={{ fontSize: '1.5rem', color: theme.muted, marginBottom: `calc(${theme.spaceUnit} / 2)` }}>{label}</div>
			<div style={{ display: 'flex', alignItems: 'flex-end', gap: theme.spaceUnit }}>
				{bars.map((bar) => (
					<div key={bar.key} style={{ textAlign: 'center', fontFamily: theme.fontMono }}>
						<motion.div
							initial={printMode ? false : { height: 0 }}
							animate={{ height: shown ? bar.value * scale : 0 }}
							transition={{ duration: printMode ? 0 : 0.5 }}
							style={{ width: '5.5rem', background: bar.color, borderRadius: theme.radius, border: `${theme.strokeWidth} solid currentColor` }}
						/>
						<div style={{ fontSize: '1.75rem', marginTop: `calc(${theme.spaceUnit} / 4)` }}>{bar.value}{unit}</div>
						<div style={{ fontSize: '1.3rem', color: theme.muted }}>{bar.key}</div>
					</div>
				))}
			</div>
		</div>
	);
}
```

Two things to copy from this one:

- **`shown = active || printMode`** is what keeps the bars from growing in
  every overview thumbnail at once, while still giving print mode the
  final heights.
- **The bars are fills** (`theme.muted` and `theme.accent`); every label
  sits on the slide background and uses `currentColor`, inherited from the
  root's `theme.fg`. That is the color rule again, in three lines.

If several inline components on one slide export `steps`, the slide's step
count is the largest of them, unless the `steps:` directive sets it.

## Auto-playing components

A component may run its own timeline instead of waiting for a press, as
long as it is gated. The rule is the same as the rolling deploy's timers:
start only when `useActive()` is true and `usePrintMode()` is false, and
clear every timer on cleanup.

**Export `steps = 1` when you do this.** It gives the audience a way out:

| Press | What happens |
|-------|--------------|
| (slide appears) | Step 0. The timeline plays on its own. |
| First press | Step 1. The final state renders at once, animation skipped. |
| Second press | The next slide. |

Without a `steps` export the slide has zero steps, so the first press
leaves the slide mid-animation with no way to show its conclusion.

```jsx
import { Slot, useActive, usePrintMode, useTheme } from 'tap';
import { useEffect, useState } from 'react';
import data from '../data.json';

export const steps = 1;

const FRAMES = data.regions.length;

export default function Timeline({ slots, step }) {
	const active = useActive();
	const printMode = usePrintMode();
	const theme = useTheme();
	const done = printMode || step >= 1;
	const [frame, setFrame] = useState(0);

	useEffect(() => {
		if (done || !active) return;
		setFrame(0);
		const id = setInterval(() => setFrame((f) => Math.min(f + 1, FRAMES)), 700);
		return () => clearInterval(id);
	}, [done, active]);

	const shown = done ? FRAMES : frame;

	return (
		<div style={{ height: '100%', display: 'flex', flexDirection: 'column', gap: theme.spaceUnit, color: theme.fg }}>
			<Slot html={slots.default} />
			<div style={{ display: 'flex', gap: theme.spaceUnit, flex: 1 }}>
				{data.regions.map((name, index) => (
					<div
						key={name}
						style={{
							flex: 1,
							display: 'flex',
							alignItems: 'center',
							justifyContent: 'center',
							fontFamily: theme.fontMono,
							borderRadius: theme.radius,
							background: index < shown ? theme.accent : 'transparent',
							border: `${theme.strokeWidth} ${index < shown ? 'solid' : 'dashed'} ${theme.muted}`,
							color: index < shown ? theme.bg : theme.muted,
							opacity: index < shown ? 1 : 0.6
						}}
					>
						{name}
					</div>
				))}
			</div>
		</div>
	);
}
```

`done` folds print mode and "the audience pressed once" into a single
condition, so there is one code path for the final state.

Check both halves without a browser:

```bash
tap export images deck.md --slide 1 --step 0 --output playing.png
tap export images deck.md --slide 1 --step 1 --output settled.png
```

::: warning Never loop forever without the gate
`repeat: Infinity`, `setInterval`, and a recursive
`requestAnimationFrame` all keep running in the presenter's next-slide
panel and in every overview thumbnail, because those mount the same
component. Always gate on `useActive()`.
:::

## The slide canvas

A whole-slide component gets the theme's normal slide padding and chrome,
the same as the `default` layout. Tap wraps the component in
`.deck-component-root`, which already supplies that padding and is itself
a full-height column flex container.

So:

- **Fill the wrapper, do not pad it.** Give your root `height: 100%` with
  a column flex layout, or fill it absolutely the way the shipped example
  does with `position: absolute; inset: 0`. Either works;
  `.deck-component-root` is positioned so an inset-0 child resolves
  against it. Do not add an outer margin or padding of your own: you would
  be padding inside the theme's padding.
- **Render the title through `Slot`**, so it looks like every other slide
  in the deck rather than like your component.
- **`tag` and `badge` directives still render**, drawn by the theme
  outside your root.
- **Some themes put fixed chrome on the slide**: a title block, a page
  number, a running head. The usable area differs per theme.

The measured safe area for every theme is in
[Creating Themes](/reference/theme-porting#component-slides), under
"Component slides". Rather than designing to those numbers, check the
themes you actually care about:

```bash
tap export images deck.md --slide 3 --theme terminal --output terminal.png
tap export images deck.md --slide 3 --theme blueprint --output blueprint.png
tap export images deck.md --slide 3 --theme zine --output zine.png
```

## Theme tokens

`useTheme()` reads the active theme's CSS custom properties from the
slide's `[data-theme]` root, and re-reads them when the theme changes. That
means a component follows the `t` key and `?theme=<slug>` without any work.

It returns **exactly these 18 keys**, and no more:

| Token | CSS property | Typical use |
|-------|--------------|-------------|
| `bg` | `--bg` | Slide background |
| `fg` | `--fg` | Body text |
| `muted` | `--muted` | Secondary text, inactive and down states |
| `accent` | `--accent` | The one highlight color, **as a fill** |
| `accentText` | `--accent-text` | The accent as text or a thin line on `bg` |
| `accent2` | `--accent-2` | A second fill; equals `accentText` in themes with no second accent |
| `surface` | `--surface` | Panels and cards |
| `statusOk` | `--status-ok` | A healthy or passing state, as a fill |
| `statusWarn` | `--status-warn` | A warning state, as a fill |
| `statusError` | `--status-error` | A failed state, as a fill |
| `fontDisplay` | `--font-display` | Headings |
| `fontBody` | `--font-body` | Body text |
| `fontMono` | `--font-mono` | Code and labels |
| `ease` | `--ease` | Animation easing |
| `dur` | `--dur` | Animation duration |
| `spaceUnit` | `--space-unit` | Gaps and padding |
| `radius` | `--radius` | Corner radius |
| `strokeWidth` | `--stroke-width` | Border and line weight |

Three things that follow from that list:

- **`accent2` is always safe to use.** It is a real second accent in the
  themes that have one, and equal to `accentText` in the rest, so a
  component that needs two fills never has to check.
- **The status colors carry meaning, so spend them sparingly.** Each is
  readable as a fill at 3:1 or better against the background, and any two
  differ in lightness by at least a 1.35 contrast ratio, in every theme.
  Use them for a health state, a passed or failed check, a threshold
  crossed. Something merely inactive is not a verdict: that stays `muted`
  with a dashed outline and lower opacity, the way the rolling deploy's
  restarting box does. **Never signal status by color alone:** pair it with
  a label, an icon, or a shape, so it still reads for a colorblind viewer
  and on a washed-out projector.
- **`useTheme()` is the portable set, not the whole set.** A theme may
  define more variables; read one directly with a fallback,
  `var(--brand-ink, var(--fg))`, so it still works elsewhere. Frontmatter
  `themeColors` overrides are picked up automatically.

To see the same values on the command line:

```bash
tap theme show blueprint
tap theme show blueprint --json
```

## Print mode

This is the rule that catches people out, so it is worth stating flatly.

> `printMode = true` means **render the settled state of the current
> `step`**, with no animation and no timers. It does **not** mean "show the
> last step".

For PDF export, `?print=true`, and thumbnails the current step happens to
be `steps`, so they do show the final state, as they always did. But
`tap export images --step 2` also passes `printMode = true`, with `step = 2`,
so that capture shows step 2 settled.

```jsx
// Wrong. tap export images --step 2 would show step 5.
const shown = printMode ? SERVERS.length : step;

// Right. printMode only decides whether to animate.
const shown = step;
const duration = printMode ? 0 : 0.3;
```

The `Step` helper follows the same rule, comparing against the live `step`
in every mode, so `<Step at={totalSteps}>` correctly stays hidden in a
step 2 capture.

### Tap helps, but not enough to skip the check

In print mode tap wraps components in Motion's `reducedMotion="always"`,
which makes transform and layout animations instant (`x`, `y`, `scale`,
`rotate`, `skew`, `width`, `height`, `top`, `left`, `right`, `bottom`,
`layoutId`), and applies `animation: none; transition: none` inside the
component root, which stops CSS keyframes and CSS transitions.

It does **not** stop Motion animations of `opacity`, `color`, or
`backgroundColor`, and it cannot stop your own timers. A Motion fade still
fades in a PDF unless you write
<code v-pre>transition={{ duration: printMode ? 0 : 0.4 }}</code> yourself.

Because CSS animations are switched off rather than fast-forwarded, an
element that only reaches its final look through
`animation-fill-mode: forwards` snaps back to its base style in print mode.
Make the base style the settled state and animate **from** the start state
instead:

```css
.badge { opacity: 1; animation: fade-in 400ms; }
@keyframes fade-in { from { opacity: 0; } }
```

## Thumbnails

The overview grid (`o`), the presenter's next-slide panel, PDF export, and
`?print=true` all render with `printMode: true`, `step` forced to the
slide's total, and `active: false`.

Two habits keep that honest:

- Give Motion <code v-pre>transition={{ duration: printMode ? 0 : 0.3 }}</code> and
  `initial={printMode ? false : {...}}`.
- Never start a timer or an animation on mount without checking
  `useActive()` or `usePrintMode()` first.

A component that has no sensible thumbnail can opt out:

```js
export const preview = false;
```

Tap then shows a small placeholder card with the file name in previews. The
slide itself still renders the component normally.

## Mount animations

A component's own `initial` to `animate` transition runs even on the very
first slide a page loads on. Tap resets Motion's presence context around
every deck component, so the `AnimatePresence` that drives slide
transitions cannot suppress it. A reload, a deep link, or
`tap export images --step k --wait 300` all show the animation actually
running rather than its end state.

A component may nest its own `AnimatePresence` for its own enter and exit
transitions; the slide-level one is untouched.

## npm packages next to the deck

Any bare import that is not one of tap's host modules (`react`,
`react/jsx-runtime`, `react-dom`, `react-dom/client`, `motion`,
`motion/react`, `tap`) resolves from a `node_modules` folder next to the
deck, or in an ancestor:

```bash
cd my-talk
npm install d3-shape
```

```jsx
import { line } from 'd3-shape';
```

React and Motion come from tap itself and must not be installed: sharing
one React instance is what lets a component use tap's hooks at all. A
package that cannot be resolved fails the build with a message that names
it and the `npm install` line to run.

### Maps

The built-in ```` ```map ```` fence cannot be used from inside a component,
and it needs network tiles, which a conference room may not give you. For a
map inside a component, draw it yourself from vector data:

```bash
npm install d3-geo topojson-client world-atlas
```

`d3-geo` gives you the projection and path generator, and a topojson atlas
gives you the shapes. Both resolve from `node_modules` next to the deck,
both are bundled into the component, and nothing is fetched at run time.

## Data and CSS imports

A component can import JSON from its own folder, which is the tidiest way
to keep a chart's numbers out of the JSX:

```jsx
import data from './rollout.json';
```

The JSON is parsed at build time and bundled in, so there is no fetch and
no loading state.

A component can import a stylesheet the same way:

```jsx
import './deploy.css';
```

Tap emits the CSS next to the bundle and adds one stylesheet link for it.
Scope your selectors, since the stylesheet applies to the whole document.

An imported image or font under 100 KB is inlined as a data URL. One of
100 KB or more is emitted as its own file named `asset-<hash>.<ext>` next
to the bundle and referenced by URL, served by `tap dev` from
`/components/` and written into `dist/components/` by `tap build`. Either
way your component just uses the imported value as a `src`, and an asset
referenced from your CSS with `url()` resolves relative to the emitted CSS
file, so it survives a static build under any sub path.

A large photograph still belongs in the markdown as a normal image, where
you get sizing attributes and the theme's image styling.

## When something breaks

A build error prints one line per problem to standard error:

```
error: slides/RollingDeploy.jsx:12:8: Expected ")" but found "}"
```

The affected slide shows an **error card** with the file and the message,
in `tap dev`, `tap export pdf`, and `tap export images` alike. Other slides keep
working, so you can navigate past it. A PDF of a deck with a broken slide
has the error card on that page rather than a blank one.

Only a static `tap build` output falls back instead of showing a card: a
whole-slide component that fails renders the slide's slots with the
`default` layout, and an inline one leaves the slot's raw content. That
keeps a shipped deck usable on stage.

`tap build` itself exits with status 1 on any component build error, so a
broken component never reaches that fallback by accident.

`tap export pdf` builds components the same way, so a PDF has each one in its
final state. A component that fails to **build** stops the export with the
same `error:` line and exit status 1. A slide that shows an error card at
export time is still written to the PDF, and `tap export pdf` prints
`warning: slide <n> shows an error card` to standard error and exits 0, so
one broken slide never costs you the handout.

## Working on components with an LLM or in a browser

Four things will waste your time if nobody tells you about them.

**The hub remembers the last presenter state for 10 minutes**, and it beats
`?step=N` when the URL hash names the same slide. So opening
`http://localhost:<port>/?step=2#3` after you have clicked around on slide
3 shows the state you left, not step 2. Two ways out:

```bash
# ask for a capture-mode page, which never takes hub state
open 'http://localhost:<port>/?capture=true&step=2#3'

# or start the server with no retention at all
TAP_HUB_STATE_RETENTION=0s tap dev deck.md --port 4000
```

**A file change under the deck folder updates the page in place.** A slide whose content did not change keeps its component mounted, with its state and its step. A component whose file changed gets a new bundle and mounts again, which restarts its animations. A change to the custom theme file reloads the whole page. Write screenshots and scratch files **outside** the deck folder, or into a dot folder or a folder named `dist`: the watcher skips both. It also skips the recordings folder and Finder's `.DS_Store` files.

```bash
tap export images deck.md --slide 3 --output ../shots/s3.png   # outside the deck
tap export images deck.md --slide 3 --output .shots/s3.png     # skipped by the watcher
```

**Give each `tap dev` its own `--port`** when you run several. The default
port now falls forward to the next free one, so nothing breaks if you
forget, but you will not know which server you are looking at.

**Check end states with `--step`, moments with `--wait`.**

```bash
tap export images deck.md --slide 3 --step 2              # step 2, settled
tap export images deck.md --slide 3 --step 2 --wait 400   # 400ms past readiness
```

## Check a slide without opening a browser

`tap export images` renders one slide state to a PNG through the same headless
browser `tap export pdf` uses. It is built for checking a component you just
wrote:

```bash
tap export images deck.md --slide 3 --output check.png
tap export images deck.md --slide 3 --step 2 --output step-2.png
tap export images deck.md --slide 3 --theme keynote --output keynote.png
tap export images deck.md --all --output shots/
```

With neither `--step` nor `--fragment`, the slide renders its final state
in print mode. With either, it renders that exact presenter state, settled:
the step you asked for, with animations and timers finished rather than
caught partway through.

`--wait <ms>` (0 to 60000) skips the settling and keeps the page live,
then waits that long before the shot.

The clock starts once the page is **ready**, not at navigation: network
idle, fonts loaded, and any animation already running finished. A short
mount animation is therefore over before the wait even begins. `--wait`
suits an animation that starts on a timer, or one that runs longer than
those readiness waits.

The command exits with status 1, with a message on standard error, when
the slide shows an error card or a component fails to build, so a script
can tell a good render from a broken one without looking at the image:

```bash
tap export images deck.md --slide 3 --output check.png || echo "slide 3 is broken"
```

## Next steps

- [Components Reference](/reference/components-reference) - the full
  contract, bundling rules, and error formats
- [CLI Commands](/reference/cli-commands) - `tap export images`,
  `tap component new`, `tap theme`
- [Themes](/guide/themes) - tokens and each theme's illustration style
- [Creating Themes](/reference/theme-porting#component-slides) - the
  per-theme safe area for a component slide

---
title: Keyboard Shortcuts
---

# Keyboard Shortcuts

Every key tap listens for, in the audience view, the presenter view, and
the `tap dev` terminal.

Keys are ignored while an input, textarea, select, or `contenteditable`
element has focus.

Press **?** in the audience view or the presenter view to show the
shortcuts for that view without leaving the deck.

## Audience View

The main presentation window, at `http://localhost:3000`.

### Navigation

| Shortcut | Action |
|----------|--------|
| **Right**, **Down**, **Space**, **Enter**, **PageDown** | Next fragment or step, then next slide |
| **Left**, **Up**, **Backspace**, **PageUp** | Previous fragment or step, then previous slide |
| **Home** | First slide |
| **End** | Last slide |

::: tip Fragments and steps
On a slide with fragments or a step-driven component, an advance key moves
through those first and only then moves to the next slide. A slide with two
`<!-- pause -->` markers takes two presses before it advances.
:::

### View controls

| Shortcut | Action |
|----------|--------|
| **S** | Open the presenter view in a new window |
| **O** | Toggle the slide overview |
| **T** | Cycle to the next theme |
| **F** | Toggle fullscreen |
| **?** | Show or hide the list of shortcuts |
| **Esc** | Blur a focused input, or close the overview or the list of shortcuts, or exit fullscreen |

Navigation keys are ignored while an input, textarea, select, or
`contenteditable` element inside a slide has focus, so a form on a slide
can be typed into. **Escape** blurs that element and gives the clicker back;
it does not also close the overview or leave fullscreen on that same press.

In the overview, arrow keys move the selection and **Enter** jumps to the
selected slide. Other shortcuts are ignored while the overview is open.

While the list of shortcuts is open, only **?** and **Esc** work, and both
close it. A click outside the list also closes it.

**T** cycles through every built-in theme without touching the deck's
frontmatter. To force one theme for a link or a recording, add
`?theme=<slug>` to the URL instead.

### The screen staying awake

Both the audience view and the presenter view ask the device to keep the
screen on while a deck is open, and ask again each time the page becomes
visible, since a browser drops the lock on a tab switch or a lock screen.

The Screen Wake Lock API is only available in a secure context, so this
works over `https` and on `localhost`. A phone opening a `tap dev` server
by LAN address over plain `http` has no wake lock available, and its screen
dims on its own schedule.

### The mouse pointer

While the deck is fullscreen, the pointer hides after 2.5 seconds without
movement and reappears on the next move, click or scroll. Presenting on a
TV or a projector therefore needs nothing switched on beforehand, and
pointing at something on a slide still works.

Outside fullscreen the pointer is left alone, so it never vanishes while
you are working on a deck in a window.

### Touch gestures

On a phone or a tablet, where there is no keyboard, the audience view
answers to two gestures.

| Gesture | Action |
|---------|--------|
| Swipe left | Next fragment or step, then next slide |
| Swipe right | Previous fragment or step, then previous slide |
| Two-finger tap | Toggle the slide overview |

A swipe must travel at least 50 px, be more horizontal than vertical, and
finish within 800 ms, so scrolling a long slide and tapping a link both
still work. In the overview, tap a slide to jump to it, or tap outside the
grid to close it.

A swipe raises a small pill at the bottom of the screen with the new slide
number, or "First slide" / "Last slide" when there is nowhere further to
go. A deck with no transition changes instantly, so without it a swipe can
look like nothing happened.

Only a drag that is clearly horizontal is taken from the browser, and only
once it is clearly horizontal, so the page cannot scroll under the finger
mid-swipe. Vertical panning and pinch-zoom stay with the browser. That is
also why a pinch does not open the overview: the browser owns that
gesture.

## Presenter View

The window at `http://localhost:3000/presenter`, or opened with **S**.

| Shortcut | Action |
|----------|--------|
| **Right**, **Down**, **Space**, **Enter**, **PageDown** | Next fragment or step, then next slide |
| **Left**, **Up**, **Backspace**, **PageUp** | Previous fragment or step, then previous slide |
| **Home** | First slide |
| **End** | Last slide |
| **R** | Reset the timer |
| **-** | Make the speaker notes smaller |
| **=** | Make the speaker notes larger |
| **?** | Show or hide the list of shortcuts |
| **Esc** | Close the list of shortcuts |

The speaker notes size is remembered in this browser.

Navigation in the presenter view is broadcast to every connected audience
view, and the other way round.

## Dev Server Terminal

The `tap dev` terminal interface, not the browser.

| Shortcut | Action |
|----------|--------|
| **A** | Open the slide builder and append a new slide |
| **O** | Open the audience view in your browser |
| **P** | Open the presenter view in your browser |
| **R** | Trigger a manual reload |
| **T** | Open the theme picker |
| **E** | Export the deck to PDF |
| **U** | Start or stop a public tunnel, and show its URL and QR code |
| **I** | Open the AI image generator (needs `GEMINI_API_KEY`) |
| **Q**, **Ctrl+C** | Stop the dev server |

**O** means different things in the two places: in this terminal it opens
the browser, in the browser it opens the slide overview. The help line at
the bottom of the terminal reminds you that **?** in the browser lists the
browser's own shortcuts.

In the theme picker, **Up**/**K** and **Down**/**J** move the selection,
**Enter** applies the theme to every connected browser and writes it into
the deck's frontmatter, and **Esc** or **Q** closes the picker without
changing anything.

## URL Parameters

Not keyboard shortcuts, but the same job from a link or a script.

| Parameter | Effect |
|-----------|--------|
| `?theme=<slug>` | Render with that theme instead of the deck's own |
| `?present=true` | Audience mode: a failing component or slide degrades to its fallback content plus a small `component error` chip, instead of a full error card |
| `?debug=true` | Force the full error card on any window, including a fullscreen or `?present=true` one |
| `?print=true` | Print mode: every fragment and step at its final state, no animation, no websocket |
| `?capture=true` | Capture mode: no websocket, no connection badge, and the requested step or fragment rendered settled |
| `?live=true` | With `capture=true`, keeps animations and timers running instead of settling |
| `?step=<k>` | Load the slide at presenter step `k`, clamped to the slide's own maximum |
| `?fragment=<k>` | Load the slide with fragments revealed through index `k`, clamped |
| `#<n>` | The URL hash selects the slide, by one-based slide number (`#5`) |

`?step=` and `?fragment=` are read once at load and never again, so normal
clicker navigation is unaffected. They are also removed from the address
bar on the first navigation, so a link you share after clicking around does
not pin the reader to the step you started on. Every other query parameter
stays. `tap screenshot --step` and `--fragment`
use them, together with `?capture=true`.

`?capture=true` exists for a stepped or fragment screenshot. Such a capture
renders the requested presenter state rather than the deck's final state,
but it runs against a temporary server with no websocket route, so a
connect attempt would retry forever and bake a "Reconnecting" badge into
the image. Capture mode drops the websocket and the badge.

By default a capture is also **settled**: the requested step or fragment is
shown with animations and timers finished, so the image is that state's
resting appearance rather than a frame caught partway toward it. Unlike
print mode, settling does not move the step or the fragment; it only stops
things animating toward it.

`?live=true`, which `tap screenshot --wait` adds, turns the settling off
and leaves the page genuinely live. The wait it pairs with starts once the
page is ready, not at navigation, so it catches an animation that runs on
a timer or longer than the readiness waits.

`tap screenshot` sets both for you. Print mode never needs either, since it
already skips the websocket and shows the final state.

The keyboard reference for these states is in
[Print mode](/reference/components-reference#print-mode).

A **fullscreen** window behaves like `?present=true` for error display,
without needing the parameter. See
[Where the error card shows](/reference/components-reference#where-the-error-card-shows).

## See Also

- [Presenter Mode](/guide/presenter-mode) - Full presenter mode guide with cross-device setup
- [Animations & Transitions](/guide/animations-transitions) - Configure fragments and transitions
- [Themes](/guide/themes) - The theme list and how to pick one

---
title: Presenter Mode
---

# Presenter Mode

Tap's presenter mode gives you a powerful dual-window setup: your audience sees the presentation while you see speaker notes, a timer, and a preview of upcoming slides.

## How Presenter Mode Works

Presenter mode creates two synchronized views of your presentation:

1. **Audience View** (`/`) - The full-screen presentation your audience sees
2. **Presenter View** (`/presenter`) - Your private control panel with notes, timer, and navigation

Both views stay perfectly in sync. Advance the slide in either window and the other follows automatically, including the revealed fragments and the current step, so the presenter's current-slide panel always shows exactly what the audience sees.

### Opening a window mid-talk

When a new browser window connects while a talk is already underway, it either picks up the live slide or keeps its own, depending on the URL:

- **No slide number in the URL** (e.g. opening `/presenter` fresh) - it lands on whatever slide, fragment, and step the talk is currently on.
- **A deep link to the same slide** (e.g. reloading on `/#12` while the talk is on slide 12) - it takes the live fragment, step, and scroll position, so a reload keeps the fragments you've already revealed.
- **A deep link to a different slide** (e.g. sharing `/#12` while the talk has moved on to slide 20) - the link wins. That window stays on slide 12 in its initial state and won't jump to slide 20 until you actually navigate it.

The hash is weighed against the hub's state **only on the first state
message after a page load**. A dropped connection that comes back sends
another one, and every later message wins outright, so a presenter whose
laptop reconnects mid-talk snaps to where the audience actually is instead
of being pulled back to the hash it happened to load on.

The hub does not remember a theme as part of that state, so a window that
reconnects keeps whatever theme it was set to. It also refuses a slide
index that is negative or past the last slide, so a stale or malformed
message cannot send a viewer off the end of the deck.

The dev server remembers the live slide, fragment, and step for 10 minutes
after the last window closes, so reloading the only open window mid-talk
lands back where you were rather than on slide 1. (Set
`TAP_HUB_STATE_RETENTION` to a Go duration such as `0s` or `30s` on the
`tap dev` process to change it; it exists for tests, not for presenting.)

A PDF export (`?print=true`) never takes part in this at all: it's a static snapshot of one slide and never connects to the live talk, so exporting while a viewer is open can't pull a live slide into the screenshots.

### Starting Presenter Mode

```bash
# Start the dev server
tap dev presentation.md

# Open presenter view in a new tab or window
# Navigate to http://localhost:3000/presenter
```

::: tip Keyboard Shortcut
Press **S** during the presentation to open the presenter view in a new window.
:::

## Presenter View Features

The presenter view includes everything you need to deliver a polished presentation:

### Layouts

Not every talk wants the same screen. Pick one of five layouts from the
button in the presenter header, which shows the current layout and a small
diagram of it:

| Layout | Id | What it shows |
|--------|----|---------------|
| **Standard** | `standard` | A big current slide on the left, the next slide and the notes on the right. This is the default. |
| **Notes first** | `notes-first` | The notes take most of the width; both slides shrink to small cues. For a talk you read from. |
| **Duo** | `duo` | The current and next slides at equal size, with the notes on a strip below. For demos and builds. |
| **Slide only** | `slide-only` | The current slide alone, as a confidence monitor. No notes, no look-ahead. |
| **Notes only** | `notes-only` | The notes fill the screen. The projector already has the slide. |

Press **V** to cycle to the next layout without opening the menu. While the
menu is open, **1** to **5** jump straight to a layout, and **Esc** closes
the menu. Digits do nothing while the menu is closed.

On a screen narrower than 768px, Duo and Slide only are dropped, because two
slides side by side do not fit a phone. The menu opens as a sheet at the
bottom of the screen rather than under the header, within reach of a thumb.
If your saved layout is one of the two wide ones, a narrow screen shows
Notes first instead and switches back when the window is wide again.

Your choice is kept per device, in this browser, across every deck. A deck
can suggest a starting layout with the `presenterLayout` frontmatter key:

```yaml
---
title: Quarterly Results
presenterLayout: notes-first
---
```

That is only a suggestion. Once you have picked a layout on a device, that
device keeps your choice and ignores the deck's. For a one-off, add
`?layout=notes-only` to the presenter URL; it wins over both and is never
saved.

### Speaker Notes

Your notes appear in the right column of the presenter view, under the next
slide preview, and use all the height the preview leaves. The current slide
takes the wider left column. Add notes to any slide using the `notes` directive:

```markdown
# Quarterly Results

Revenue grew 23% year over year.

<!--
notes: |
  - Mention the new product launch in Q2
  - Highlight the APAC expansion
  - Transition to next quarter goals
-->
```

Notes are shown as plain text with their line breaks and blank lines
preserved, so write them as short lines or a small list. Markdown is not
rendered there: `**bold**` shows as `**bold**`. A notes comment anywhere in
the slide works too, and is the better form for long free text. See
[Slide Directives](/reference/slide-directives#notes).

To change the notes font size, press **-** or **=**, or use the **A-** and
**A+** buttons in the notes panel title. The presenter view remembers the
size in this browser.

The notes size control in the layout menu offers two ways to size them:

- **Manual** - one size for the whole deck, the one **-** and **=** set.
  This is the default.
- **Fit to panel** - each slide's notes are sized to fill the panel without
  scrolling, up to 6rem. A slide with two lines of notes gets much larger
  text than one with twenty. In this mode **-** and **=** scale that result
  down to as little as half and back up again, so you can ask for everything
  a notch smaller without giving up the fitting. Notes too long to fit even
  at 1rem stay at 1rem and scroll.

Add `?notesSize=fit` to the presenter URL to turn fitting on for one
session without saving it.

### Timer

The presenter view includes a timer that starts when you begin presenting:

- **Elapsed time** - How long you've been presenting
- **Current time** - The current wall clock time

::: tip Reset Timer
Press **R** in presenter view to reset the timer to zero.
:::

### Next Slide Preview

A preview of the upcoming slide appears in the presenter view, helping you:

- Prepare smooth transitions
- Remember what's coming next
- Avoid surprises during your talk

### Current Slide Preview

Your current slide is displayed in the presenter view so you can see exactly what your audience sees without turning around.

## Cross-Device Presenter Mode

One of Tap's most powerful features is the ability to control your presentation from a separate device.

### Using an iPad or Phone as a Controller

1. Start `tap dev --lan` on your laptop
2. Connect your iPad/phone to the same network
3. Scan the QR code in the terminal, or open the Network URL it shows
4. Your device becomes a wireless presentation remote

This setup lets you:

- Walk around freely while presenting
- See your notes without looking at your laptop
- Advance slides with the presenter view's on-screen buttons

On a touch device, both views answer to gestures instead of keys: swipe
left for the next fragment, step or slide, swipe right to go back, and
two-finger tap for the slide overview. A swipe shows the new slide number
for a moment, so you can tell the gesture landed even on a deck with no
transition.

### When the Phone Is Not on the Same Network

A LAN address only works when both devices sit on the same network, and a
conference network often stops them talking to each other at all. Start the
server with a tunnel instead:

```bash
tap dev presentation.md --tunnel
```

```
Audience:  http://localhost:3000
Presenter: http://localhost:3000/presenter
Tunnel:    https://plain-shoes-arrive-lately.trycloudflare.com
```

That address works from anywhere, on any network. It is a Cloudflare Quick
Tunnel, so it needs no Cloudflare account, no login and no configuration,
and the random name lasts only as long as the server. It does need the
`cloudflared` binary (`brew install cloudflared`), and `--tunnel` says so
if it is missing.

Press `u` in the dev terminal to start or stop a tunnel without
restarting; the URL appears with a QR code to point a phone at.

**Anyone with the link can watch the deck** while the tunnel is up. Stop it
with `u`, or stop the server, when you are done.

### Why a Tunnel Keeps the Screen On

Both views ask the device to keep the screen awake, so a phone propped up
as a prompter does not dim partway through a slide. That request needs a
*secure context*, which means `https` or `localhost`.

A phone opening a LAN address over plain `http` therefore has no wake lock
available and its screen dims on its own schedule. Through a tunnel the
page is `https`, and the screen stays on. This is the practical reason to
use `--tunnel` even when both devices are on the same network.

### QR Code for Easy Access

The dev server's terminal shows a QR code only when there is a URL worth
scanning:

- With `--lan`, the terminal shows a `Network:` presenter URL and a QR
  code for it.
- With `--tunnel`, it shows the tunnel's QR code instead.
- With neither, it shows no QR code, since only this machine can connect.

```bash
tap dev presentation.md --lan

# Output includes:
#   Audience:  http://localhost:3000
#   Presenter: http://localhost:3000/presenter
#   Network:   http://192.168.1.100:3000/presenter
#   [QR CODE]
```

Scan the QR code with your phone or tablet to instantly open the presenter view.

## On Stage

Three things make a live talk safer. None of them need a flag on the deck.

**Put the audience window in fullscreen, or open it with
`?present=true`.** Both switch error display to its audience-safe form: a
whole-slide component that throws is replaced by the slide's own content,
its slots in the default layout, with a small muted `component error` chip
in the corner; an inline one leaves the rest of the slide untouched and the
chip marks the gap. Either way the room sees a slide, not a stack trace.
Fullscreen is followed live, so entering or leaving it while an error is on
screen switches forms straight away. Your presenter view keeps the full message, so you can
read what broke while the audience sees a merely plainer slide. Add
`?debug=true` to any window to force the full card back while you are
still authoring.

**A window catches up by itself after a restart.** The hub tells each
window the deck's revision when it connects. A window that reconnects and
finds a different revision than it first saw reloads itself, so restarting
`tap dev`, or editing the deck while a window was asleep, no longer leaves
a stale slide on the projector.

**With a presenter password set, only authenticated windows drive the
others.** See below.

## Password Protection

For sensitive presentations, you can protect the presenter view with a password:

```bash
tap dev presentation.md --presenter-password 'secret 123'
```

The password is a `tap dev` flag, not a frontmatter key, so it never ends
up in the deck file. It may contain any characters; tap URL-encodes it
wherever it prints a presenter URL or builds the QR code.

Authenticate once by opening `/presenter?key=<password>`. Tap answers with
a redirect to `/presenter` with `key` removed, so the password does not sit
in the address bar or in your history, and sets an HttpOnly cookie holding
a **random session token**, never the password itself. After that the
presenter view opens without `?key=`, which is what makes the **S** key
work: the window it opens inherits the cookie.

| Request | Response |
|---------|----------|
| No key, no cookie | 403 `Forbidden: presenter password required. Use ?key=<password>` |
| Wrong key | 403 `Forbidden: incorrect presenter password` |
| Correct key | 302 to `/presenter`, cookie set |
| Valid cookie, no key | 200 |

`/qr` is gated the same way, so the QR code cannot be used to hand out
presenter access to anyone who can reach the server. It also needs
`--lan`: without it, its network URLs would not work, and the endpoint
answers 404.

When password protection is enabled:

- The audience view (`/`) remains publicly accessible
- The presenter view (`/presenter`) requires the password
- Notes and upcoming slides stay private
- **Only an authenticated window can drive the others.** A window without
  the cookie still receives every sync and reload message, so it follows
  along, but its own navigation messages are dropped by the hub. An
  audience member clicking around moves only their own screen.

With no password set, nothing changes: any connected window can navigate
every other one, which is what you want for a rehearsal across two
laptops.

::: warning
The password is passed on the command line, so it lands in your shell
history. It protects the presenter view on a shared network; it is not a
substitute for not serving confidential material.
:::

## Presenter Mode Keyboard Shortcuts

In the presenter view:

| Shortcut | Action |
|----------|--------|
| **Right**, **Down**, **Space**, **Enter**, **PageDown** | Next fragment or step, then next slide |
| **Left**, **Up**, **Backspace**, **PageUp** | Previous fragment or step, then previous slide |
| **Home** / **End** | First / last slide |
| **R** | Reset timer |
| **-** / **=** | Smaller / larger speaker notes, or the fitting scale |
| **V** | Next presenter layout |
| **1** to **5** | Pick a layout, while the layout menu is open |
| **?** | Show or hide the list of shortcuts |
| **Esc** | Close the layout menu, or the list of shortcuts |

In the audience view:

| Shortcut | Action |
|----------|--------|
| **S** | Open the presenter view in a new window |
| **O** | Toggle the slide overview |
| **T** | Cycle themes |
| **F** | Toggle fullscreen |
| **?** | Show or hide the list of shortcuts |
| **Esc** | Close the overview or the list of shortcuts, or exit fullscreen |

The presenter view also has on-screen previous and next buttons, which is
what makes it usable from a phone or tablet.

## Best Practices

### Before Your Talk

1. **Test presenter mode** on the actual display setup
2. **Check network connectivity** if using cross-device control
3. **Set a password** for confidential content
4. **Write notes** for complex or data-heavy slides

### During Your Talk

1. **Use the timer** to pace yourself
2. **Glance at the next slide preview** before transitions
3. **Keep notes concise** - bullet points work better than paragraphs

### Tip: Dual Monitor Setup

The ideal setup uses two displays:

1. **External display** - Shows audience view in fullscreen (press **F**)
2. **Laptop screen** - Shows presenter view with notes

This mirrors the classic conference room setup while giving you modern features like cross-device sync.

## Next Steps

- [Keyboard Shortcuts](/reference/keyboard-shortcuts) - Complete shortcut reference
- [Writing Slides](/guide/writing-slides) - Learn about speaker notes syntax
- [Building & Export](/guide/building-export) - Export with or without notes

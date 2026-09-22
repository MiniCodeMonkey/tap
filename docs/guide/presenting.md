---
title: Presenting
---

# Presenting

`tap present` is for giving the talk, not writing it. It serves the deck
exactly like `tap dev`, but it does not reload while you are on stage, and
it can record the whole run without you touching a key.

```
tap present [deck.md] [--port N] [--no-record]
```

With no file it shows the same picker as `tap dev`.

## How it differs from `tap dev`

- **No file watching.** Saving the deck, a component, or a theme does not
  reload the slides. An editor autosave cannot bump the audience back to
  slide 1 mid-talk. Press **r** to reload by hand when you do want the
  latest version.
- **The audience view opens at launch**, not the presenter view. Press
  **p** to open the presenter view when you need it.
- **The editing keys do nothing.** `a` (add slide), `i` (image), `t`
  (theme) and `e` (PDF export) write to the deck or a file next to it, and
  none of that belongs in a talk, so they are unbound.

The header reads `● PRESENTING`, and the keys that remain are:

| Key | Action |
|---|---|
| `o` | Open the audience view again |
| `p` | Open the presenter view |
| `r` | Reload the deck from disk |
| `u` | Start or stop the public tunnel |
| `c` | Stop the recording, or start a new segment |
| `q` | Quit, with a keep prompt when a recording exists |
| `ctrl+c` | Quit and keep the recording |

## Recording every talk automatically

The first time you run `tap present` on a machine where recording is
possible, Tap asks once, before the TUI starts:

```
Record automatically every time you run tap present? (y/n)
```

The answer is saved as `present.record: true` or `false` in
`settings.yaml`, in `$XDG_CONFIG_HOME/tap/` or `~/.config/tap/` when that
is unset. The setting lives with your machine, never with the deck, so a
deck someone else cloned from you cannot turn recording on for them. To
change your answer, edit or delete that file; deleting it makes the next
run ask again.

Without a terminal attached, Tap does not ask and does not record.
`--no-record` skips recording for one run without touching the saved
answer.

If you said no, or the setting has never been asked, pressing **c** still
records: the answer only controls whether recording starts on its own.

Off macOS, recording is not available, so `tap present` never asks and
never records.

## What gets recorded

When recording is on, it starts as soon as the server is up, so anything
said over the title slide is captured. A run is stored as one folder,
named from the deck's slugified title and the local start time:

```
recordings/my-talk-2026-09-21-1932/
  01.mov
  02.mov
  03.mov
  chapters.txt
```

Each file is a segment; a new one starts whenever the recorded display
changes (see below), or when you press **c**. Tap does not join segments
into one file.

`chapters.txt` covers the whole run, one section per segment headed by the
file name:

```
01.mov
0:00 Title
0:41 Agenda
0:44 Title
02.mov
0:00 Title
0:47 Talk starts
0:47 Agenda
3:12 Live demo
```

`Talk starts` marks the last time the deck left the title slide. Clicking
through the deck while you get set up, then going back to the title,
doesn't count: a later departure replaces the mark, so the entry always
reflects when the talk actually began.

The deck's `recording` frontmatter block still applies: `output`, `audio`,
`showClicks`, and `chapters` all work the same as in `tap dev`. See the
[Talk Recording](/guide/recording) guide for that reference.
`recording.display` is ignored, because `tap present` picks the display
itself. There is no time cap or 90 minute warning in `tap present`, since
a meetup evening can run for hours before the talk starts; `tap dev` keeps
both.

## Following the projector

Tap records the projector, or any other external display, when one is
connected, and the laptop screen otherwise. It checks the display list
every 3 seconds.

- **Plugging in or swapping the projector** starts a new segment on the
  newly chosen display.
- **Unplugging the projector**, once one has been seen in this run, pauses
  recording: nothing is captured until any external display comes back.
  This keeps the host, another speaker, or your presenter notes off the
  video during the gap.
- **Replugging** any external display resumes recording in a new segment.
- **A practice run with no projector ever connected** records the laptop
  screen for the whole run.
- **Mirroring** works normally: whatever picture is shared to the
  projector is what gets recorded.

While paused, the header shows `PAUSED, waiting for projector` in place of
the recording state.

## When disk space runs low

Below 5 GB free, the TUI shows a warning and the audience and presenter
views show a small badge: `Disk almost full, recording stops at 1 GB`.
Below 1 GB free, Tap stops the recording, finalizes the file, and the
badge changes to `Recording stopped: disk full`. Press **c** to start a
new segment once you have freed space. This guard runs in both `tap dev`
and `tap present`.

## Quitting

Pressing **q** asks:

```
Keep this recording? (Y/n)
```

Enter, `y`, or `ctrl+c` keep the recording. Only `n` deletes it. `esc`
cancels and goes back to the TUI with the recording still running.

A run that never left the first slide is not a talk, so Tap deletes it
without asking and prints:

```
Deleted the recording: the deck never left the first slide.
```

Any other kept run prints its folder path, as `tap dev` does. Inside a
git repository, keeping a recording may ask whether to add `recordings/`
to `.gitignore`, the same prompt described in the
[Talk Recording](/guide/recording) guide.

---
title: Talk Recording
---

# Talk Recording

Press **C** in `tap dev` to record your screen and microphone. Press it
again to stop. Recording writes a chapter list of slide timings beside the
video, so a video description is most of the way written by the time you
stop talking.

## macOS Only

Recording uses `screencapture`, the capture binary already built into
macOS, so there is nothing to install. Tap does not bundle `ffmpeg` and does
not ask you to.

On Linux and Windows, **C** does nothing and no recording controls appear.
If a deck's frontmatter has a `recording` block, `tap dev` prints a startup
warning that recording is macOS only and the block is ignored; the deck
itself still runs normally.

## Screen Recording Permission

Recording needs the Screen Recording permission, and it is granted to the
terminal application that runs `tap`, not to Tap itself. Open **System
Settings > Privacy & Security > Screen Recording**, enable your terminal
(Terminal, iTerm, Ghostty, whichever you use), and **restart the terminal**
for the grant to take effect.

`tap dev` checks this when it starts the terminal interface on macOS, by
taking a tiny test screenshot, and says nothing when it succeeds. If the
permission is missing, or was revoked, the check fails immediately and the
terminal prints a warning naming the exact fix above. This check runs
whether or not the deck asks for recording, so a missing grant surfaces
while you are still setting up rather than when you press **C** on stage.
The same grant covers audio capture too, so macOS never raises a separate
microphone prompt on top of it.

## Choosing a Display

With one display attached, pressing **C** starts recording immediately;
there is nothing to choose. With more than one display attached, such as
your laptop plus a projector, **C** opens a picker first:

```
  Record

  Display
  > Built-in Retina Display   3456 x 2234   main
    DELL U2718Q               3840 x 2160

  Audio
    MacBook Pro Microphone (system default input)
    Change it in System Settings or the menu bar.

  enter start    t test 5s    esc cancel
```

Use the arrow keys or `j`/`k` to move between displays, then **enter** to
start recording the highlighted one, or **esc** to cancel.

Press **t** to run a five second test capture of the current selection
without leaving the picker. It opens in QuickTime Player when it finishes,
so you can watch the right screen came through and listen for your
microphone before you are on stage. Test again after switching displays or
plugging in the projector; the picker stays open so you can.

## The Microphone

The picker shows the name of the system's current default input, read
only. Tap cannot offer a dropdown to change it: `screencapture` selects a
microphone by its CoreAudio device UID, and no pure-Go binary can read
those UIDs back out of macOS. Building that lookup would mean adding cgo to
a project that cross-compiles cleanly today, for a picker that would only
ever show a UID nobody recognizes anyway.

To record from a different microphone, change the system default input
first, in **System Settings > Sound** or from the microphone icon in the
menu bar, then open the picker. It will show the new default.

If you already know your microphone's CoreAudio UID, `recording.audio`
accepts it directly:

```yaml
recording:
  audio: BuiltInMicrophoneDevice
```

`recording.audio` also accepts `default` (the same as leaving it out) and
`none`, which records a silent video, useful for a demo where the audio
would just be typing.

## Where Recordings Land

By default, recordings are written to a `recordings/` directory next to
the deck, alongside a matching chapter file with the same name and a
`.txt` extension. Nothing creates that directory until you actually press
**C**: `tap dev` does not touch the filesystem just because a deck's
frontmatter has a `recording` block. Set `recording.output` to write
somewhere else:

```yaml
recording:
  output: /Users/me/Movies/talks
```

A relative path is resolved against the deck's own directory.

The first time a recording finishes inside a git repository with no
existing `.gitignore` entry covering the output directory, `tap dev` asks
whether to add one:

```
  Recording saved

  A recording is large, and this deck is in a git repository.

  Add recordings/ to .gitignore? (y/n)
```

The entry offered is derived from wherever `recording.output` actually
points, so it is correct even when recordings do not land in the default
`recordings/` directory. Answering `y` appends the entry once; the prompt
does not appear again once it is there.

After `tap dev` exits, the terminal prints the last recording's path and
its chapter file:

```
  Last recording: recordings/quarterly-results-2026-09-20-1432.mov
  Chapters: recordings/quarterly-results-2026-09-20-1432.txt
```

## The Chapter List

While recording, Tap writes one line to the chapter file every time the
slide changes, in the format YouTube's chapter parser expects: a timestamp,
then the slide's title.

- The first line is always `0:00`, whatever slide is on screen when
  recording starts.
- Moving to a new slide adds an entry.
- Moving back to a slide you have already shown adds another entry: the
  chapter list follows what actually happened, not just forward progress.
- Landing on the same slide again immediately, such as an accidental
  double key press, adds nothing.
- Revealing a fragment or a step on the same slide adds nothing; only a
  change of slide creates a chapter.

A slide's title comes from its first heading. A slide with no heading, such
as an image-only or component-only slide, is labeled `Slide N`.

A worked example, ready to paste into a YouTube description:

```
0:00 Introduction
0:42 The problem
2:15 What this adds
5:03 Live demo
9:47 Questions
1:04:12 Live demo
```

## Long Recordings

Tap warns in the TUI if a recording is still running after 90 minutes, and
stops it automatically at 3 hours, so a recording nobody remembered to stop
does not fill the disk overnight. Both are configurable as durations such
as `90m` or `3h`:

```yaml
recording:
  warnAfter: 45m
  stopAfter: 2h
```

Set `stopAfter: off` to disable the automatic stop entirely. There is no
way to disable the warning; it only ever writes a line to the event log.

## Stopping and Quitting

Pressing **q** while a recording is running asks first, since
`screencapture` cannot resume a file it has already closed:

```
  Recording in progress

  my-talk-2026-09-20-1432.mov  14:32

  Stop recording and quit? (y/n)
```

`y` stops the recording, finalizes the file, and quits. `n` or **esc**
returns to the TUI with the recording untouched.

`Ctrl+C` does not ask. It still finalizes the file: `tap dev` stops the
recorder as part of its normal shutdown, so an interrupted recording is
still a playable video, just without the chance to say no.

## Frontmatter Reference

Every key under `recording` is optional; an omitted key keeps its default.

```yaml
---
recording:
  # Directory recordings are written to, relative to the deck unless given
  # as an absolute path. Default: recordings
  output: recordings
  # "default" for the system input, "none" for a silent recording, or a
  # CoreAudio device UID. Default: default
  audio: default
  # How long a recording runs before the TUI warns about it. Default: 90m
  warnAfter: 90m
  # How long a recording runs before it stops itself. "off" disables the
  # cap. Default: 3h
  stopAfter: 3h
  # Preselects a display in the picker by its screencapture index (1 is
  # the main display). 0 means no preference. Default: 0
  display: 0
  # Draws mouse clicks in the recording. Default: false
  showClicks: false
  # Writes the sidecar chapter list; set to false to turn it off.
  # Default: true
  chapters: true
---
```

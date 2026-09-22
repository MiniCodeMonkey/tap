# tap CLI consistency review

This review covers every current `tap` command, plus the new commands that Tap Desktop needs.

Decisions so far: the CLI moves to the noun-grouped tree, with no aliases for the old names. Structural slide edits (move, duplicate, delete) stay out of the CLI and belong to the app.

## Current command tree

```
tap [--verbose] [--version]
├── new            [--title] [--theme/-t] [--output/-o] [--yes/-y] [--force]
├── dev [file]     [--port/-p] [--presenter-password] [--headless] [--tunnel] [--allow-origin]
├── serve [dir]    [--port/-p]
├── build <file>   [--output/-o]
├── pdf <file>     [--output/-o] [--content slides|notes|both]
├── screenshot <file> [--slide] [--step] [--fragment] [--theme] [--out] [--width] [--all] [--wait]
├── add [file]                      (interactive slide wizard, appends at the end)
│   └── component <Name> [--inline] [--ts] [--deck]
└── theme
    ├── list       [--json]
    └── show [slug] [--json] [--prompt] [--deck]
```

## Findings

### 1. The deck is named four different ways

- `dev [file]` takes an optional file and opens an interactive picker when it is missing.
- `add [file]` takes an optional file and guesses one with different rules (`presentation.md`, `slides.md`, `talk.md`, `deck.md`, then any other `.md` file).
- `build`, `pdf` and `screenshot` require `<file>`.
- `add component` and `theme show` take `--deck`.

**Proposal:** every command that acts on a deck takes an optional positional `[deck]`, which is a file or a folder. One shared rule resolves it: use the argument, else the only deck in the current folder, else the picker when stdin is a TTY, else an error that lists the candidates.

### 2. Output flags differ

- `build`, `pdf` and `new` use `--output/-o`.
- `screenshot` uses `--out`, with no short form.

**Proposal:** use `--output/-o` everywhere. `--out` is removed.

### 3. Short flags differ

- `new` has `--theme/-t`, but `screenshot --theme` has no short form.
- `--port/-p` is consistent.

**Proposal:** a flag with the same name has the same short form on every command.

### 4. "add" means two unrelated things

- `tap add` runs a wizard that appends a slide to a deck.
- `tap add component` scaffolds a new source file and never touches the deck.
- Deck creation uses the verb `new` instead.

**Proposal:** group commands by noun, with the same verbs in every group. See "Proposed tree" below.

### 5. Theme switching has two meanings

- `t` then Enter in the `tap dev` TUI writes `theme:` into the deck file.
- `T` in the browser only changes the theme for that viewer and saves nothing.

**Proposal:** name the two actions differently. "Preview a theme" (the `T` key) never writes the file. "Set the theme" is an explicit command, `tap theme set <slug> [deck]`, which the TUI and the app both call.

### 6. Features that only the TUI can reach

Recording (`c`), toggling the tunnel mid-session (`u`), AI images (`i`), setting the theme (`t`), and the `.gitignore` prompt have no command or API. The app cannot call a TUI key.

**Proposal:** each one becomes a command or a dev server endpoint that the TUI calls too, so there is one implementation.

- Deck edits become commands: `tap theme set`, `tap image generate`, and `tap image regenerate`.
- Session actions that need the running server become endpoints: recording start and stop, and tunnel start and stop.

### 7. `--json` is rare

Only `theme list` and `theme show` offer `--json`. Scripts, agents and the app all need machine-readable output.

**Proposal:** every command that prints data or a result supports `--json`, with one shape: `{"ok": true, ...}` or `{"ok": false, "error": {"code", "message"}}`. The exit code is 0 on success, 1 on a user error, and 2 on an internal error. Exit 130 on interrupt stays as it is today.

### 8. Indexing is mixed

`screenshot --slide` is 1-based. `--fragment` defaults to -1 and appears to be 0-based. `--step` defaults to 0. I have not checked what step 0 means compared with fragment 0.

**Proposal:** everything a user types is 1-based (`--slide 3`, `--step 2`). "Final state" is the default when a flag is omitted. JSON output uses the same numbers.

### 9. Dead or misleading surface

- The global `--verbose` flag is read nowhere.
- The TUI's `r` key ("Manual reload") only logs an event.
- The `tap dev` help text and the README advertise live code execution, but the driver registry is never connected, so `/api/execute` returns 500 today.

**Proposal:** connect the registry, and either make `--verbose` and `r` work or remove them.

### 10. `tap present` is missing

The recording code for segments, follow-display and consent exists, but there is no command for it.

**Proposal:** ship `tap present [deck]` as the CLI form of the app's Play button, before the app does.

## Proposed tree

Singular nouns group the commands. Every group uses the same verbs: `new`, `list`, `show`, `set`, `add`, `generate`, `revoke`.

```
tap
├── new [deck]                       create a deck          (was: new)
├── dev [deck]                       write with live reload (unchanged)
├── present [deck]                   present, record        (NEW)
├── build [deck]                     static site            (was: build <file>)
├── serve [dir]                      serve a build          (unchanged)
├── export
│   ├── pdf [deck]                   (was: tap pdf)
│   └── images [deck]                (was: tap screenshot)
├── slide
│   ├── list [deck] --json           ranges, layouts, titles, steps, code blocks (NEW)
│   └── add [deck] --layout [--print] (was: tap add; the wizard when interactive)
├── deck
│   └── schema --json                frontmatter keys, types, allowed values (NEW)
├── approval
│   ├── list
│   └── revoke <deck>                (NEW)
├── theme
│   ├── list --json
│   ├── show [slug] --json --prompt
│   └── set <slug> [deck]                                   (NEW)
├── image
│   ├── add <file> [deck] --slide                           (NEW, copies into images/)
│   ├── generate [deck] --slide --prompt                    (NEW, was TUI i)
│   └── regenerate [deck] --slide --image                   (NEW)
└── component
    └── new <Name> [deck] --inline --ts                     (was: tap add component)
```

Global conventions:

- `[deck]` is optional everywhere and uses the one resolution rule.
- `--output/-o`, `--json`, and `--yes/-y` mean the same thing on every command.
- Slide numbers are 1-based.
- Old names are removed in the next major version, with a clear error that names the new command.

## Open questions

Decided: `pdf` and `screenshot` move under `tap export`. Every group noun is singular: `slide`, `theme`, `image`, `component`, `deck`, `approval`, `export`.


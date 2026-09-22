# tap prerequisites for Tap Desktop: design

Date: 2026-09-22
Status: draft, awaiting approval

This spec covers the work in tap that Tap Desktop depends on. Every item is useful to CLI users on its own, and none of it is desktop-only code. The desktop design is `2026-09-22-tap-desktop-design.md`, and the behavior the items must support is in `tap-desktop-features/`.

The work splits into six parts. Each part is one implementation plan and one or more pull requests. They are listed in the recommended order.

| Part | What | Depends on |
|---|---|---|
| 1 | CLI command tree and conventions | nothing |
| 2 | Live code: connection, execution by reference, approvals | 1 (command names) |
| 3 | Deck features: `skip`, schema, slide list | 1 |
| 4 | Commands that the TUI keys use today | 1 |
| 5 | Render-ready signal, in-place updates, and progress output | nothing |
| 6 | `--app` mode for `tap dev` and `tap present` | 2, 3, 5, and `tap present` |

`tap present` is being built separately and is assumed to exist before part 6 starts.

---

## Part 1: CLI command tree and conventions

The full review is `tap-desktop-features/cli-review.md`. This part applies its decisions.

### Command tree

```
tap
├── new [deck]
├── dev [deck]
├── present [deck]
├── build [deck]
├── serve [dir]
├── export
│   ├── pdf [deck]
│   └── images [deck]
├── slide
│   ├── list [deck]
│   └── add [deck]
├── deck
│   └── schema
├── theme
│   ├── list
│   ├── show [slug]
│   └── set <slug> [deck]
├── image
│   ├── add <file> [deck]
│   ├── generate [deck]
│   └── regenerate [deck]
├── component
│   └── new <Name> [deck]
└── approval
    ├── list
    └── revoke <deck>
```

### Renames, with no aliases

| Old | New |
|---|---|
| `tap pdf` | `tap export pdf` |
| `tap screenshot` | `tap export images` |
| `tap add` | `tap slide add` |
| `tap add component` | `tap component new` |
| `screenshot --out` | `--output/-o` |
| `--deck` on `add component` and `theme show` | the positional `[deck]` |

An old command name exits with code 1 and one line that names the new command, for example `tap pdf was renamed: use tap export pdf`. It does nothing else. This message is the only trace of the old names.

**Timing:** land this part before 2.0.0 is final, so the rename ships in 2.0 and does not force a 3.0.

### Conventions for every command

- **Deck argument.** Every command that acts on a deck takes an optional positional `[deck]`, a file or a folder. One shared resolver picks the deck in this order:
  1. the argument;
  2. the only `.md` deck in the folder;
  3. an interactive picker when stdin is a TTY;
  4. otherwise, exit 1 with the list of candidates.

  It replaces both the `tap dev` picker and the `tap add` guessing rules.
- **Flags.** A flag has the same name and short form on every command: `--output/-o`, `--theme/-t`, `--port/-p`, `--yes/-y`, and `--json`.
- **Numbers.** Everything a user types is 1-based: `--slide 3`, `--step 2`, `--fragment 1`. Leaving a flag out means the final state.
- **JSON output.** Every command that prints data or a result accepts `--json` and prints one object: `{"ok": true, ...}` or `{"ok": false, "error": {"code": "...", "message": "..."}}`.
- **Exit codes.** 0 means success, 1 a user error, 2 an internal error, and 130 an interrupt.
- **Dead surface.** Remove the unused global `--verbose` flag. Make the TUI `r` key reload the deck in `tap dev`, as it already does in `tap present`.

### Tests

- A table test runs every command with `--help`, and checks that the flag names and short forms match the conventions.
- One test per old name checks the rename message and exit code 1.
- Resolver tests cover each branch, including the non-TTY error that lists candidates.

---

## Part 2: Live code

### 2.1 Connect the driver registry

No production code calls `Server.SetRegistry`, so `/api/execute` returns 500 "Driver registry not configured" in every real run. `tap dev` and `tap present` build the registry from the built-in drivers and the deck's `drivers:` map, and set it on the server. `tap build` output stays static, with no execution.

### 2.2 Execute by reference

- `POST /api/execute` accepts `{"slide": 4, "block": 1}`, both 1-based, where `block` counts live code blocks within the slide. tap looks up the block in the current deck and runs its code with its driver and connection.
- A body that contains `code` is rejected with 400.
- The frontend Run button sends the reference. The presenter page does the same.
- The cross-site guard from PR #14 still applies.

### 2.3 Required driver declaration

- A deck with live code must declare every driver it uses as a key in the frontmatter `drivers:` map. A driver without settings is declared as `shell: {}`.
- A block whose driver is not declared never runs. The page shows a message in the block, and `tap dev` and `tap present` print the same message with the file and line: `This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter.` When the deck has no `drivers:` key at all, the message shows the complete block to paste.
- `tap new` writes a `drivers:` map whenever its starter has live code.

### 2.4 Approvals

- Approvals are stored in the user settings file, `~/.config/tap/settings.yaml` (the same file and loader as the `present.record` consent):

  ```yaml
  approvals:
    - deck: /Users/me/talks/3am/conference-talk.md
      drivers: [shell, sqlite]
      approvedAt: 2026-09-22T19:32:00Z
  ```

- A deck needs approval when it has at least one live code block. It is approved when its absolute path is listed and every declared driver is in the approved list.
- **When tap asks:** at startup of `tap dev` and `tap present`, before the TUI starts, when stdin is a TTY and the deck needs approval. The prompt lists the declared drivers, the command of any custom driver, and the number of blocks and their slides. It offers to print each block. The answer is `y` or `n`, and `n` is the default.
- **A new driver:** when an approved deck declares a driver that is not in its approved list, tap asks again and names only the new driver.
- **Declined or not asked:** live code is off. Run buttons show "Not approved", and `/api/execute` answers 403 with that reason. Nothing is stored for a "no", so the next start asks again.
- **Non-interactive runs** (no TTY, `--headless`, `tap export`) never ask. Live code is off unless `--allow-code` is given. `--allow-code` allows it for that run and stores nothing.
- `tap new` records an approval for the deck it creates.
- `tap approval list [--json]` prints the approvals. `tap approval revoke <deck>` removes one.
- In `--app` mode, the question is an event and the answer arrives over stdin (part 6). The page never shows the question.

### 2.5 Environment variables in driver settings

- String values in `drivers:` settings expand `${NAME}` from the environment when a driver runs. `$${` writes a literal `${`.
- An unset variable makes the block fail with a message that names the variable. It never expands to an empty string.
- Only driver settings expand. Other frontmatter keys stay literal.

### Tests

- A registry integration test runs a real `tap dev` and executes a shell block by reference.
- `/api/execute` rejects a `code` body, an unknown slide or block, an undeclared driver, and an unapproved deck, each with its own status code and message.
- Approval tests cover the TTY prompt, a declined answer, a new driver, a moved deck, `--allow-code`, and non-TTY runs.
- Expansion tests cover a set variable, an unset variable, and the `$${` escape.

---

## Part 3: Deck features

### 3.1 `skip` directive

- The per-slide directive `skip: true` leaves the slide out of presenting (in the audience and presenter views, and in slide counts), and out of `tap export` and `tap build`.
- `tap dev` still renders a skipped slide when you go to it directly, marked as skipped, so writing and checking it still works.

### 3.2 `tap deck schema --json`

This command prints every frontmatter key tap understands, from the same source as `config.go`: name, type, default, allowed values, description, and nested keys (for `drivers`, `recording` and `themeColors`). The app builds the Deck form from it, and editor integrations can use it too. A test fails when a key is added to the config struct but not to the schema.

### 3.3 `tap slide list [deck] --json`

This command prints each slide with the fields the app needs. The same Go function also serves `PUT /api/app/source` in part 6:

```json
{"ok": true, "slides": [
  {"number": 3, "startLine": 18, "endLine": 31, "layout": "default", "title": "What We Knew",
   "fragments": 1, "steps": 0, "skip": false, "errors": [],
   "codeBlocks": [{"block": 1, "driver": "sqlite", "live": true, "line": 24}]}
]}
```

Line numbers are 1-based and cover the slide's text, including its directive comment, without leading or trailing blank lines. Blank lines next to a `---` separator belong to no slide. The separator lines and the frontmatter are excluded.

Code blocks must be detected with every attribute form tap accepts. The prototype got an empty list for the example deck's ```` ```sql {driver: sqlite, connection: incident} {2-3} ```` fence, so that exact fence is a required test case.

`fragments` and `steps` are the two separate counters the frontend already walks: `fragments` counts `<!-- pause -->` reveals, and `steps` is the transformer's `countSteps` result, which includes `export const steps` from a whole-slide component and the largest count among inline components, unless a `steps:` directive overrides it. Both numbers therefore come from the transformer after the component bundles are built, not from the parser alone. When a saved `.jsx` file changes its `steps` export, tap rebuilds the bundle and sends a new slide list.

### Tests

- Parser tests cover `skip` in presenting, export, build, and slide counts.
- A schema completeness test compares the schema with the config struct.
- Golden tests for `tap slide list` cover the example decks, including fenced blocks that contain `---`.

---

## Part 4: Commands behind the TUI keys

Each TUI key that changes files becomes a command. The TUI calls the same Go function, so each behavior has one implementation.

| Command | Replaces | Behavior |
|---|---|---|
| `tap theme set <slug> [deck]` | TUI `t` + Enter | Writes `theme:` in the frontmatter through the existing `config.UpdateThemeInFile`. An unknown slug is exit 1 with the list of themes. |
| `tap theme show <slug> --image [-o file]` | new | Renders a title slide in that theme to a PNG, for theme pickers. Output is cached by slug and tap version. |
| `tap image add <file> [deck] [--slide n]` | new | Copies the file into `images/` beside the deck. It keeps the name and adds `-2`, `-3` on a clash. It prints the markdown to insert, and with `--slide`, inserts it at the end of that slide. |
| `tap image generate [deck] --slide n --prompt "..."` | TUI `i`, add | The existing generator, unchanged: `images/generated-<hash>.<ext>` and the `ai-prompt` comment. |
| `tap image regenerate [deck] --slide n --image path [--prompt "..."]` | TUI `i`, regenerate | Replaces the pair in place and deletes the old file, as the TUI does. |
| `tap slide add [deck] --layout <x> [--print]` | `tap add` | Without flags on a TTY, runs the existing wizard. With `--layout`, it appends that layout's template. With `--print`, it prints the template and writes nothing, for the app's layout gallery. It covers all 12 layouts, not only the 7 the wizard has today. |
| `tap component new <Name> [deck] [--inline] [--ts]` | `tap add component` | Unchanged behavior under the new name. With `--json`, it prints the snippet as a field. |

The TUI keys stay, and they call these functions.

### Tests

- One test per command against a temporary deck, plus a test that the TUI key path and the command produce identical files.

---

## Part 5: Render-ready signal and progress

### 5.1 One ready signal

This is a hard prerequisite: without it, the prototype's thumbnails were missing component charts. The frontend sets `window.__tapReady = {revision, slide, step}` and dispatches a `tap:ready` event once the slide has settled. When the page runs inside the app, it also calls `window.webkit.messageHandlers.tapReady.postMessage({revision, slide, step})` if that handler exists. The app registers the handler and never injects script. The thumbnail renderer loads `?print=true` and has no WebSocket, so this handler is how it learns that a slide is ready. The slide has settled when:

- fonts are loaded;
- images are complete;
- maps have reported ready;
- components have settled;
- no error card is still pending;
- transitions and theme animations have finished.

It resets when the slide or step changes. `tap export pdf`, `tap export images`, and the app's thumbnail renderer all wait for this one signal. The checklist that `internal/pdf/exporter.go` puts together today moves into the frontend, which is the only code that knows when rendering has finished.

### 5.2 `--progress json`

`tap export pdf`, `tap export images`, and `tap build` accept `--progress json`. It prints one JSON line per step to stderr, for example `{"phase": "render", "done": 7, "total": 14}`, and one final line with the result. A first-time Chromium download reports `{"phase": "download", "bytes": ..., "totalBytes": ...}`.

### 5.3 Update in place instead of reloading

Today every change makes the server send `reload`, and the page calls `window.location.reload()`. A spike on 2026-09-22 (Apple M4 Max, Playwright WebKit and Chromium, 20 runs each) measured this path:

| Path, write or PUT until the new text is visible | WebKit median / p95 | Chromium median / p95 |
|---|---|---|
| Full reload through the file watcher (today) | 154 / 157 ms | 144 / 146 ms |
| In-place update through the file watcher | 104 / 105 ms | 103 / 106 ms |
| In-place update from `PUT /api/app/source`, no watcher | 2 / 3 ms | 2.4 / 3 ms |
| Edited `.jsx` component, in-place path (5 runs) | 411 ms | 411 ms |

Findings:

- The watcher's 100 ms debounce is almost all of the server-side time. Parse and transform take 0.3 ms for the example deck and 5 ms for 200 slides. Sending the buffer over HTTP skips the debounce, and is the biggest win.
- Today's full reload did not lose the slide or step, because the hub replays the last position to a reconnecting page. It showed no visible flash in 20 runs. The in-place update is still better, because it keeps component state and does not reload fonts and bundles.
- Component bundles already swap without a reload: bundle file names are content-hashed, and `DeckComponent` keys its lazy import on the URL. A new URL in the deck data is enough.
- Not measured: a real `WKWebView` (only Playwright's WebKit build), and the full-reload time for a `.jsx` edit.

- The server sends a new `update` message with the new revision and the numbers of the slides that changed.
- The page fetches `/api/presentation` and replaces the deck data in its store without reloading. It keeps the current slide, fragment, and step, clamped to the new counts. Only slides whose content changed re-render, because slides are keyed by a hash of their content.
- A changed component bundle needs no extra code: the new content-hashed URL in the deck data makes `DeckComponent` import it.
- `reload` stays for the cases that need a fresh page, such as a changed theme CSS file or a new tap version.
- `tap dev` in a browser gets the same benefit: saving no longer flashes the page or loses the position.

### Tests

- Frontend tests check that an `update` keeps the slide and step, re-renders only changed slides, and clamps the step when a slide loses steps.
- Frontend tests check that the signal fires for each kind of blocker, and resets on navigation.
- The export tests run against the signal and show no change in the output.
- Progress lines are checked against a small schema.

---

## Part 6: `--app` mode

`tap dev --app <file>` and `tap present --app [--no-record] <file>` run the normal server with these differences.

### Startup and channels

- It binds to `127.0.0.1` on a free port. It opens no browser and shows no TUI.
- It generates a 32-byte random token.
- It exits when stdin reaches end of file, so a crashed or killed app never leaves tap running.
- **stdout** carries only JSON lines. The first line is `{"type": "ready", "port": 49152, "token": "…"}`. Later lines are events.
- **stdin** carries JSON lines with commands and answers from the app.
- **stderr** carries human-readable logs, which the app shows in Window > Tap Log.

The stdio pipes are the control channel. Only the parent process can read or write them, so script on a slide page cannot answer a question or stop a recording. HTTP carries only the page, the buffer, and page traffic.

### HTTP additions, all behind the token

- Every request needs `Authorization: Bearer <token>`, and the WebSocket upgrade needs the token too. The page gets it from a cookie that tap sets when the app loads the first URL with a one-time `?launch=` code. That code is printed in the ready line and expires on first use, so the token itself never appears in a URL.
- `PUT /api/app/source` takes the buffer and answers with the `tap slide list` structure. tap renders from the buffer until the next `{"type": "saved"}` command.
- Every request body has a size limit, enforced with `http.MaxBytesReader` before the body is read. The limit is 8 MB for `PUT /api/app/source` and 64 KB for the other routes. A larger body gets 413.
- The PR #14 guard (a same-origin `Origin` header and a JSON body) applies to every mutating route.

### Events on stdout

| `type` | When | Fields |
|---|---|---|
| `ready` | startup | `port`, `token`, `launch` |
| `file-changed` | the watcher sees a change that tap did not make (dev only) | `path` |
| `question` | tap needs an answer | `id`, `kind` (`approval`, `record-consent`, `keep-recording`), `payload` |
| `recording` | the recording state changes (present only) | `state` (`recording`, `paused`, `stopped`), `segment`, `elapsed`, `disk` |
| `tunnel` | the tunnel state changes | `state`, `url`, `qr` (PNG, base64) |
| `slide` | the audience position changes (present only) | `slide`, `step` |
| `error` | a fatal or reportable error | `code`, `message` |

The same message also goes on the WebSocket as `file-changed`, because the preview page reloads from it.

### Commands on stdin

| `type` | Effect |
|---|---|
| `answer` | Answers a question: `{"type": "answer", "id": "…", "value": true}` |
| `saved` | The app saved the buffer to disk. tap drops the buffer and reads the file. |
| `reload` | Reloads the deck, as `r` does |
| `tunnel` | `{"start": true}` or `{"start": false}`, as `u` does |
| `recording` | `{"action": "new-segment"}` or `{"action": "stop"}`, as `c` does |
| `quit` | Shuts down cleanly, and asks `keep-recording` first when a run has a recording |

### `tap present --app` specifics

- It does not open the audience view in a browser. The app opens its own windows.
- It sends the consent question and the keep-recording question as events, and never prompts on the terminal.
- It follows the projector and guards disk space exactly as `tap present` does. The app does not decide anything about recording.

### Tests

- A process-level test starts `tap dev --app` on a fixture deck, reads the ready line, and covers the token, the launch code, the buffer round trip, and `file-changed`. It also sends `reload` and `quit`.
- Tests show that a page on the same origin cannot answer a question or change recording state, because no HTTP route exists for either.
- `tap present --app` tests cover every question kind, and recording events with a fake recorder.

---

## Out of scope

- Any Swift code.
- Editing the deck's structure from the CLI (moving, duplicating, or deleting slides). This belongs to the app.
- Editable code blocks during a talk.

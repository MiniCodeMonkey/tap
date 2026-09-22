# Deck features: `skip`, `tap deck schema` and `tap slide list`: implementation plan

## Controller rulings on the open questions (2026-09-22)

- The contract additions `Result.Errors` and `CodeBlock.Language` are accepted. The roadmap lists them.
- The plan's defaults stand for questions 2 to 6.
- Slide numbering: skipped slides keep their deck numbers everywhere tap numbers slides (URLs, the WebSocket, `slide list`, `--slide`). Only what the audience sees counts presented slides. P4 and P6 use this numbering.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the per-slide `skip: true` directive (honored by presenting, slide counts, `tap build` and `tap export`), `tap deck schema --json` (every frontmatter key from one Go source), and `tap slide list [deck] --json` (line ranges and the slide facts an editor needs), and fix the two parser bugs the slide list exposes.

**Architecture:** The parser records `skip`, each slide's file line range, and keeps broken slides when asked (`ParseKeepingErrors`). A new package `internal/slidelist` turns a deck's source into the slide list by running the real pipeline (parse, build components, transform, validate layouts), so counts come from the same code that renders. `config.Schema()` describes the `Config` struct, and a reflection test fails when the two drift. Skipped slides stay in the deck everywhere tap numbers slides (URLs, the WebSocket, `tap slide list`, `export images --slide`), so numbers always match the markdown. The frontend passes over them when navigating and counting. `tap build` and `tap export pdf` render a copy of the deck without them.

**Tech Stack:** Go 1.24, cobra, goldmark 1.7, esbuild (through `internal/components`), React 19 with Zustand, vitest.

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-prerequisites-design.md`, "Part 3: Deck features", and the `PUT /api/app/source` line of Part 6, which reuses the slide list. The contracts are in `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md`, "Contracts between plans", section "From P3". Both files are on the `docs/tap-desktop` branch.

## Global Constraints

- Branch `feat/deck-features`, from an up-to-date `main` after P1 (`2026-09-22-cli-command-tree-and-driver-registry.md`) has merged, in a worktree at `/Users/codemonkey/projects/tap-deck-features`. One pull request.
- This plan is written against the code as it is after P1: `resolveDeck`, `firstArg`, `userError`, `internalError`, the `code*` constants in `exit.go`, `printJSONOK`, `runTap`, `expectedCommands` in `conventions_test.go`, `slideCmd` in `slide.go`, `exportCmd`, `runExportPDF`, `runExportImages`, `screenshotOutput`. Do not redefine any of them.
- Contract names (roadmap, "From P3"), exactly: package `internal/slidelist`, `func Build(source []byte, baseDir string) (Result, error)`, `Result.Slides []Slide`, `Slide` with `Number`, `StartLine`, `EndLine`, `Layout`, `Title`, `Fragments`, `Steps`, `Skip`, `Errors`, `CodeBlocks []CodeBlock`, `CodeBlock` with `Block`, `Driver`, `Live`, `Line`, the JSON names from spec section 3.3, the line range rule as a doc comment on `Slide`, and `func config.Schema() []SchemaKey`.
- Line numbers are 1-based. A range covers the slide's text, including its directive comment, without leading or trailing blank lines. Blank lines next to a `---` separator belong to no slide. The separator lines and the frontmatter are excluded.
- Every user-typed number is 1-based. `block` in the slide list is 1-based.
- `--json` prints one object: `{"ok": true, ...}` or `{"ok": false, "error": {"code": "...", "message": "..."}}`, through `printJSONOK` and `execute`.
- A `--json` result struct declares its fields in output order. When `fieldalignment` complains, keep the order and add `//nolint:govet // fieldalignment: field order is the JSON output order`.
- Spell identifiers out in full (`options`, not `opts`; `index`, not `idx`, in new code). Match the surrounding code otherwise.
- Code comments describe the present. No ticket numbers, no "before the fix" wording.
- No em dashes in code, comments, docs or the changelog. Use `--`, a comma, or a new sentence.
- Frontend tests live next to their sources. Run them with `cd frontend && npm test -- --run`. Also run `npm run check` and `npm run lint` in `frontend`.
- P5 runs in parallel in another worktree and changes the frontend and the PDF exporter. This plan does not touch `internal/pdf`. If P5 merges first, expect small rebase conflicts in `frontend/src/App.tsx`, `PresenterApp.tsx` or `lib/stores/presentation.ts`. Keep both sides.

## Open questions

These are the gaps the spec leaves. Each has a plan decision so the work can go ahead. The user should confirm them before execution.

1. **Contract additions.** The slide list needs two fields the roadmap contract does not name: `Result.Errors []string` (problems with the deck as a whole, such as bad frontmatter, which belong to no slide) and `CodeBlock.Language` (feature 02 draws "sql, live" in the box header). The plan adds both. The roadmap's "From P3" section should list them.
2. **`tap export images --slide N` on a skipped slide.** The plan renders it, because the person asked for that slide by number, and prints a note on stderr. `--all` leaves skipped slides out. Alternative: exit 1.
3. **`tap present` and a direct jump.** Next and Previous pass over skipped slides in every view. A direct jump (URL hash, overview click, `Home`/`End` excepted) still opens a skipped slide, also in `tap present`, where it shows without the "Skipped" marker because the audience could see it. Alternative: refuse direct jumps to skipped slides in present mode.
4. **Validation of skipped slides in `tap build`.** `tap build` still fails on a layout error in a skipped slide. Component bundles that only skipped slides use are still written to `dist/components/`. Leaving skipped slides out of validation and bundling is possible but costs a second parse pass in `runBuild`.
5. **Frontmatter `fragments`.** `Config.Fragments` is parsed, `DefaultConfig` sets it to `true`, the docs say the default is `false`, and nothing reads it. The schema reports it as the struct has it (default `true`). Whether to wire it up or drop it is out of scope.
6. **`line` for an empty, info-less fence.** goldmark 1.7 gives no position for a fence with no info string and no content lines, so its `line` is 0. Every other fence gets its real line.

## Decisions this plan makes

1. **Numbering.** A skipped slide keeps its number. URLs (`#4`), the WebSocket `slide` message, `tap slide list`, and `tap export images --slide` all use deck numbers that count skipped slides. Only what the audience sees (the slide number a theme draws, the progress bar, the presenter counter) counts presented slides.
2. **Where skip is applied.** The frontend applies it for `tap dev` and `tap present`. `tap build` and `tap export pdf` serve a copy of the deck without skipped slides (`transformer.WithoutSkippedSlides`), renumbered, so the exporter needs no change. `tap export pdf` maps broken-slide warnings back to deck numbers.
3. **The marker.** A skipped slide opened directly shows a "Skipped" pill, has `data-skipped="true"`, and hides the theme's slide number. The marker is left out in print mode, in a settled capture, and during `tap present`.
4. **Starting slide.** With no URL hash, the viewer opens on the first slide that is not skipped. `Home` and `End` go to the first and last presented slides.
5. **Two parser fixes.** A fence with both a driver group and a highlight group (```` ```sql {driver: sqlite, connection: incident} {2-3} ````) lost its driver, because only the last `{...}` group was read. The slide splitter only knew backtick fences at column 0, so a `---` inside a `~~~` fence or an indented fence split the slide. Both are real and fixed here (Tasks 2 and 3).
6. **Broken slides stay in the list.** `slidelist.Build` never fails for a problem in the deck. A slide that fails to parse keeps its range and directives and carries its error. A frontmatter problem goes in `Result.Errors`, and the slides are still listed with the default config.
7. **Titles.** `Title` is the text of the slide's first ATX heading with bold and code markers removed, or `""`. The helper moves from `internal/recorder` to `internal/parser` so both use one implementation.
8. **Component slides.** A slide whose layout is a component file reports the file path as its `layout`.
9. **Schema types.** `type` is one of `string`, `boolean`, `integer`, `list`, `object` (fixed nested keys) and `map` (entries under names the deck picks). The completeness test checks both the key paths and these types against the struct.

## Out of scope

- `PUT /api/app/source` itself, and sending a new slide list when a `.jsx` file's `steps` export changes. P6 does both, by calling `slidelist.Build` from the watcher and the handler.
- Editing slide structure from the CLI.
- Changing the PDF exporter (`internal/pdf`).

## Spec coverage

| Spec requirement | Task |
|---|---|
| 3.1 `skip: true` directive | 1 |
| 3.1 left out of presenting, audience and presenter views | 13, 14, 15 |
| 3.1 left out of slide counts | 13, 14, 15 |
| 3.1 left out of `tap export` | 12 |
| 3.1 left out of `tap build` | 11 |
| 3.1 `tap dev` renders a skipped slide opened directly, marked | 13, 14 |
| 3.2 `tap deck schema --json` from the `config.go` source, with name, type, default, values, description, nested keys | 6, 10 |
| 3.2 completeness test against the config struct | 6 |
| 3.3 `tap slide list [deck] --json` with every field | 7, 9 |
| 3.3 the same Go function serves `PUT /api/app/source` | 7 (`slidelist.Build`) |
| 3.3 line range rule | 4, 7 |
| 3.3 code blocks in every attribute form, the sqlite fence case | 2, 5, 7 |
| 3.3 `fragments` and `steps` from the transformer after bundles are built | 7 |
| Tests: parser tests for skip in presenting, export, build and counts | 1, 11, 12, 13, 14, 15 |
| Tests: schema completeness | 6 |
| Tests: golden slide lists over the example decks, with fences that contain `---` | 8 |
| Docs and changelog | 16 |

## File structure

| File | Status | Responsibility |
|---|---|---|
| `internal/parser/parser.go` | modify | `SlideDirectives.Skip`, `Slide.StartLine`/`EndLine`, `ParseKeepingErrors`, `parseSlide`, splitter on `fenceTracker` |
| `internal/parser/fences.go` | new | `fenceTracker`: CommonMark fence rules, one line at a time |
| `internal/parser/highlight_lines.go` | modify | `splitCodeFenceInfo` returns every `{...}` group, `parseFenceMeta` |
| `internal/parser/codeblocks.go` | modify | `buildCodeBlock` uses `parseFenceMeta`; `FenceLines` |
| `internal/parser/title.go` | new | `SlideTitle`, moved from `internal/recorder` |
| `internal/recorder/chapters.go` | modify | `SlideTitle` calls `parser.SlideTitle` |
| `internal/transformer/transformer.go` | modify | `TransformedSlide.Skip`, `WithoutSkippedSlides` |
| `internal/config/config.go` | modify | `FromSource`, ordered value lists, `DefaultDriverTimeoutSeconds`, `DefaultRecordingOutput` |
| `internal/config/schema.go` | new | `SchemaKey`, `Schema()` |
| `internal/slidelist/slidelist.go` | new | `Result`, `Slide`, `CodeBlock`, `Build` |
| `internal/slidelist/testdata/` | new | `fences.md` fixture and `golden/*.json` |
| `internal/cli/slide_list.go` | new | `tap slide list` |
| `internal/cli/deck_schema.go` | new | `tap deck` and `tap deck schema` |
| `internal/cli/export_pdf.go`, `export_images.go` | modify | skip in exports |
| `internal/cli/dev.go` | modify | `config.DefaultRecordingOutput` |
| `internal/builder/builder.go` | modify | skip in `tap build` |
| `frontend/src/lib/utils/skip.ts` | new | pure helpers over a slide array |
| `frontend/src/lib/stores/presentation.ts` | modify | navigation passes over skipped slides, presented selectors, `goToFirstSlide`, `goToLastSlide` |
| `frontend/src/lib/components/Slide.tsx`, `ProgressBar.tsx`, `SwipeFeedback.tsx`, `SlideOverview.tsx` | modify | presented numbers, the marker |
| `frontend/src/App.tsx`, `PresenterApp.tsx`, `lib/utils/keyboard.ts` | modify | presented counts, `Home`/`End`, the next-slide preview |
| `frontend/src/lib/types.ts` | modify | `Slide.skip` |
| `frontend/src/lib/styles/ui-components.css` | modify | the marker and the dimmed thumbnail |
| Docs | modify | `README.md`, `docs/reference/cli-commands.md`, `docs/reference/slide-directives.md`, `docs/reference/frontmatter-options.md`, `docs/guide/code-blocks.md`, `skills/tap/rules/cli.md`, `skills/tap/rules/slide-directives.md`, `CHANGELOG.md`, `docs/changelog.md` |

---

### Task 1: The `skip` directive in the parser and the transformer

**Files:**
- Modify: `internal/parser/parser.go` (`SlideDirectives`, `directiveFields`)
- Modify: `internal/parser/parser_test.go`
- Modify: `internal/transformer/transformer.go`
- Modify: `internal/transformer/transformer_test.go`

**Interfaces:**
- Produces:
  - `parser.SlideDirectives.Skip bool`
  - `transformer.TransformedSlide.Skip bool` with JSON name `skip`, omitted when false
  - `func transformer.WithoutSkippedSlides(presentation *TransformedPresentation) (kept *TransformedPresentation, deckNumbers []int)`

- [ ] **Step 1: Write the failing parser tests**

Append to `internal/parser/parser_test.go`:

```go
func TestParse_SkipDirective(t *testing.T) {
	content := []byte("# One\n\n---\n\n<!--\nlayout: section\nskip: true\n-->\n\n# Two\n\n---\n\n<!-- skip: false -->\n\n# Three")

	pres, err := New().Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if len(pres.Slides) != 3 {
		t.Fatalf("got %d slides, want 3: a skipped slide stays in the parsed deck", len(pres.Slides))
	}
	for index, want := range []bool{false, true, false} {
		if got := pres.Slides[index].Directives.Skip; got != want {
			t.Errorf("slide %d: Skip = %v, want %v", index+1, got, want)
		}
	}
	if pres.Slides[1].Directives.Layout != "section" {
		t.Errorf("slide 2 layout = %q, want section: skip sits beside other directives", pres.Slides[1].Directives.Layout)
	}
	if strings.Contains(pres.Slides[1].Content, "skip") {
		t.Errorf("slide 2 content = %q, want the directive comment removed", pres.Slides[1].Content)
	}
}

func TestParse_SkipDirectiveIgnoresANonBoolean(t *testing.T) {
	pres, err := New().Parse([]byte("<!-- skip: maybe -->\n\n# Slide"))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if pres.Slides[0].Directives.Skip {
		t.Error("Skip = true for skip: maybe, want false")
	}
}
```

- [ ] **Step 2: Run the parser tests to verify they fail**

Run: `go test ./internal/parser -run 'TestParse_SkipDirective' -v`
Expected: compile failure, `pres.Slides[index].Directives.Skip undefined`.

- [ ] **Step 3: Add the directive**

In `internal/parser/parser.go`, add a field at the end of `SlideDirectives`:

```go
	// Skip is true when the slide's "skip" directive is true. Presenting,
	// slide counts, tap build and tap export leave the slide out. tap dev
	// still shows it when someone goes to it directly.
	Skip bool
```

Add an entry to `directiveFields`, after the `scroll-speed` entry:

```go
	{"skip", func(y map[string]interface{}, d *SlideDirectives) {
		if v, ok := y["skip"].(bool); ok {
			d.Skip = v
		}
	}},
```

`directiveKeyNames` in `notes.go` is built from `directiveFields`, so `skip:` is also recognized in a comment that mixes directives and notes.

- [ ] **Step 4: Run the parser tests to verify they pass**

Run: `go test ./internal/parser -v -run 'TestParse_SkipDirective|TestParse_SlideDirectives|TestParse_Directives'`
Expected: PASS.

- [ ] **Step 5: Write the failing transformer tests**

Append to `internal/transformer/transformer_test.go`:

```go
func TestTransform_CarriesSkip(t *testing.T) {
	pres := &parser.Presentation{Slides: []parser.Slide{
		{Index: 0, HTML: "<p>one</p>"},
		{Index: 1, HTML: "<p>two</p>", Directives: parser.SlideDirectives{Skip: true}},
	}}

	result := New(config.DefaultConfig()).Transform(pres)
	if result.Slides[0].Skip || !result.Slides[1].Skip {
		t.Fatalf("Skip = (%v, %v), want (false, true)", result.Slides[0].Skip, result.Slides[1].Skip)
	}

	kept, err := json.Marshal(result.Slides[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(kept), `"skip"`) {
		t.Errorf("slide JSON %s has a skip field, want it left out when false", kept)
	}
	skipped, err := json.Marshal(result.Slides[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(skipped), `"skip":true`) {
		t.Errorf("slide JSON %s, want \"skip\":true", skipped)
	}
}

func TestWithoutSkippedSlides(t *testing.T) {
	presentation := &TransformedPresentation{Slides: []TransformedSlide{
		{Index: 0, HTML: "one"},
		{Index: 1, HTML: "two", Skip: true},
		{Index: 2, HTML: "three"},
		{Index: 3, HTML: "four", Skip: true},
	}}

	kept, deckNumbers := WithoutSkippedSlides(presentation)

	if len(kept.Slides) != 2 || kept.Slides[0].HTML != "one" || kept.Slides[1].HTML != "three" {
		t.Fatalf("kept slides = %+v, want one and three", kept.Slides)
	}
	if kept.Slides[0].Index != 0 || kept.Slides[1].Index != 1 {
		t.Errorf("kept indexes = (%d, %d), want (0, 1)", kept.Slides[0].Index, kept.Slides[1].Index)
	}
	if len(deckNumbers) != 2 || deckNumbers[0] != 1 || deckNumbers[1] != 3 {
		t.Errorf("deckNumbers = %v, want [1 3]", deckNumbers)
	}
	if len(presentation.Slides) != 4 || presentation.Slides[2].Index != 2 {
		t.Error("WithoutSkippedSlides changed the presentation it was given")
	}
}
```

- [ ] **Step 6: Run the transformer tests to verify they fail**

Run: `go test ./internal/transformer -run 'TestTransform_CarriesSkip|TestWithoutSkippedSlides' -v`
Expected: compile failure, `unknown field Skip in struct literal`.

- [ ] **Step 7: Add `Skip` and `WithoutSkippedSlides` to the transformer**

In `internal/transformer/transformer.go`, add to `TransformedSlide`, after `Scroll`:

```go
	// Skip is true for a slide whose skip directive is true. The frontend
	// passes over it when presenting and leaves it out of slide counts.
	Skip bool `json:"skip,omitempty"`
```

In `transformSlide`, add `Skip: slide.Directives.Skip,` to the `TransformedSlide` literal, after `StepsInvalid`.

Add below `Transform`:

```go
// WithoutSkippedSlides returns a copy of presentation without the slides
// whose skip directive is true. The kept slides' Index values are
// renumbered from 0, so the copy is a complete deck of its own, the way
// tap build and tap export pdf render it. The copy shares each slide's
// maps and slices with presentation. deckNumbers holds the 1-based number
// each kept slide has in the full deck, in order.
func WithoutSkippedSlides(presentation *TransformedPresentation) (kept *TransformedPresentation, deckNumbers []int) {
	kept = &TransformedPresentation{
		Config: presentation.Config,
		Slides: make([]TransformedSlide, 0, len(presentation.Slides)),
	}
	deckNumbers = make([]int, 0, len(presentation.Slides))
	for index, slide := range presentation.Slides {
		if slide.Skip {
			continue
		}
		slide.Index = len(kept.Slides)
		kept.Slides = append(kept.Slides, slide)
		deckNumbers = append(deckNumbers, index+1)
	}
	return kept, deckNumbers
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test ./internal/parser ./internal/transformer`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/parser/parser.go internal/parser/parser_test.go internal/transformer/transformer.go internal/transformer/transformer_test.go
git commit -m "feat(parser): add the skip slide directive"
```

---

### Task 2: Read every `{...}` group of a fence info string

The example deck's fence ```` ```sql {driver: sqlite, connection: incident} {2-3} ```` had no driver: `metaPattern` matches only the last `{...}` group, which is the highlight spec, so the driver group was dropped and the block was not live. This task reads every group.

**Files:**
- Modify: `internal/parser/highlight_lines.go` (`splitCodeFenceInfo`, the renderer)
- Modify: `internal/parser/codeblocks.go` (`buildCodeBlock`)
- Modify: `internal/parser/parser.go` (the `metaPattern` comment)
- Create: `internal/parser/fence_info_test.go`

**Interfaces:**
- Produces:
  - `func splitCodeFenceInfo(infoString string) (language string, groups []string)` (the signature changes; it has no callers outside this package)
  - `func parseFenceMeta(infoString string) CodeBlockMeta`

- [ ] **Step 1: Write the failing tests**

`internal/parser/fence_info_test.go`:

```go
package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_FenceAttributeForms(t *testing.T) {
	tests := []struct {
		name         string
		info         string
		wantLanguage string
		wantMeta     CodeBlockMeta
	}{
		{"driver, then highlight", "sql {driver: sqlite, connection: incident} {2-3}", "sql", CodeBlockMeta{Driver: "sqlite", Connection: "incident", HighlightLines: "2-3"}},
		{"highlight, then driver", "sql {2-3} {driver: sqlite}", "sql", CodeBlockMeta{Driver: "sqlite", HighlightLines: "2-3"}},
		{"groups with no space between", "sql {driver: sqlite}{2}", "sql", CodeBlockMeta{Driver: "sqlite", HighlightLines: "2"}},
		{"driver only", "sql {driver: sqlite, connection: demo}", "sql", CodeBlockMeta{Driver: "sqlite", Connection: "demo"}},
		{"key=value form", "bash {driver=shell}", "bash", CodeBlockMeta{Driver: "shell"}},
		{"highlight only", "js {1, 3-5}", "js", CodeBlockMeta{HighlightLines: "1,3-5"}},
		{"no attributes", "go", "go", CodeBlockMeta{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pres, err := New().Parse([]byte("```" + tt.info + "\nSELECT 1;\n```"))
			if err != nil {
				t.Fatalf("Parse() returned error: %v", err)
			}
			block := pres.Slides[0].CodeBlocks[0]
			if block.Language != tt.wantLanguage {
				t.Errorf("Language = %q, want %q", block.Language, tt.wantLanguage)
			}
			if block.Meta != tt.wantMeta {
				t.Errorf("Meta = %+v, want %+v", block.Meta, tt.wantMeta)
			}
		})
	}
}

func TestParse_FenceWithDriverKeepsItsHighlightAttribute(t *testing.T) {
	pres, err := New().Parse([]byte("```sql {driver: sqlite, connection: incident} {2-3}\nSELECT 1;\nSELECT 2;\n```"))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if !strings.Contains(pres.Slides[0].HTML, `data-highlight-lines="2-3"`) {
		t.Errorf("HTML = %s, want data-highlight-lines=\"2-3\"", pres.Slides[0].HTML)
	}
}

func TestParse_ConferenceTalkQueryIsLive(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "examples", "conference-talk.md"))
	if err != nil {
		t.Fatal(err)
	}
	pres, err := New().Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	block := pres.Slides[3].CodeBlocks[0]
	if block.Meta.Driver != "sqlite" || block.Meta.Connection != "incident" || block.Meta.HighlightLines != "2-3" {
		t.Errorf("slide 4 block meta = %+v, want sqlite, incident, 2-3", block.Meta)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/parser -run 'TestParse_FenceAttributeForms|TestParse_FenceWithDriver|TestParse_ConferenceTalkQueryIsLive' -v`
Expected: FAIL. "driver, then highlight" gets an empty `Driver`, and so does the conference talk test.

- [ ] **Step 3: Replace `splitCodeFenceInfo` and add `parseFenceMeta`**

In `internal/parser/highlight_lines.go`, replace `splitCodeFenceInfo` with:

```go
// splitCodeFenceInfo splits a fence info string into its language part
// and the content of each "{...}" group that ends it, in order. So
// "sql {driver: sqlite} {2-3}" gives "sql" and ["driver: sqlite", "2-3"].
func splitCodeFenceInfo(infoString string) (language string, groups []string) {
	remaining := strings.TrimSpace(infoString)
	for {
		match := metaPattern.FindStringSubmatch(remaining)
		if match == nil {
			break
		}
		groups = append([]string{match[1]}, groups...)
		remaining = strings.TrimSpace(remaining[:len(remaining)-len(match[0])])
	}
	return remaining, groups
}

// parseFenceMeta reads every "{...}" group of a fence info string. A group
// that is a line-highlight spec sets HighlightLines. Any other group sets
// Driver and Connection. The groups can come in either order.
func parseFenceMeta(infoString string) CodeBlockMeta {
	_, groups := splitCodeFenceInfo(infoString)
	var meta CodeBlockMeta
	for _, group := range groups {
		groupMeta := parseCodeBlockMeta(group)
		if groupMeta.HighlightLines != "" {
			meta.HighlightLines = groupMeta.HighlightLines
		}
		if groupMeta.Driver != "" {
			meta.Driver = groupMeta.Driver
		}
		if groupMeta.Connection != "" {
			meta.Connection = groupMeta.Connection
		}
	}
	return meta
}
```

In `renderFencedCodeBlock`, replace the `if n.Info != nil { ... }` block that writes `data-highlight-lines` with:

```go
		if n.Info != nil {
			if meta := parseFenceMeta(string(n.Info.Segment.Value(source))); meta.HighlightLines != "" {
				_, _ = w.WriteString(" data-highlight-lines=\"")
				_, _ = w.WriteString(meta.HighlightLines)
				_, _ = w.WriteString("\"")
			}
		}
```

In the doc comment of `fencedCodeBlockRenderer`, change "when its trailing "{...}" is a line-highlight spec" to "when one of its "{...}" groups is a line-highlight spec".

- [ ] **Step 4: Use `parseFenceMeta` in `buildCodeBlock`**

In `internal/parser/codeblocks.go`, replace the `if fcb.Info != nil { ... }` block at the end of `buildCodeBlock` with:

```go
	if fcb.Info != nil {
		block.Meta = parseFenceMeta(string(fcb.Info.Segment.Value(source)))
	}
```

In `internal/parser/parser.go`, change the comment above `metaPattern` to:

```go
// metaPattern matches one {key: value, ...} group at the end of an info
// string. splitCodeFenceInfo applies it repeatedly, so an info string can
// carry several groups, such as sql {driver: mysql} {2-3}.
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/parser`
Expected: PASS, including the older `TestParseCodeBlockMeta_*` tests.

- [ ] **Step 6: Commit**

```bash
git add internal/parser/highlight_lines.go internal/parser/codeblocks.go internal/parser/parser.go internal/parser/fence_info_test.go
git commit -m "fix(parser): keep the driver of a fence that also highlights lines"
```

---

### Task 3: Split slides with CommonMark's fence rules

The slide splitter tracks only backtick fences that start at column 0. A `---` inside a `~~~` fence, or inside a fence indented by one to three spaces, split the slide, while goldmark rendered it as code.

**Files:**
- Create: `internal/parser/fences.go`
- Create: `internal/parser/fences_test.go`
- Modify: `internal/parser/parser.go` (`SplitSlidesPreservingCodeBlocks`, `splitSlidesPreservingCodeBlocksWithLines`)

**Interfaces:**
- Produces: `type fenceTracker struct` with `func (tracker *fenceTracker) advance(line string) bool`. Task 5 uses it.

- [ ] **Step 1: Write the failing tests**

`internal/parser/fences_test.go`:

```go
package parser

import "testing"

func TestFenceTracker(t *testing.T) {
	lines := []string{
		"text",       // plain text
		"```go",      // opens a backtick fence
		"---",        // content
		"~~~",        // a tilde run does not close a backtick fence
		"```",        // closes
		"after",      // plain text
		"~~~~",       // opens a tilde fence of four
		"```",        // content
		"~~~",        // too short to close
		"~~~~~",      // closes
		"``` a`b",    // not a fence: a backtick info string cannot hold a backtick
		"   ```",     // opens, indented by three spaces
		"x",          // content
		"    ```",    // indented by four, so content
		"```",        // closes
	}
	want := []bool{false, true, true, true, true, false, true, true, true, true, false, true, true, true, true}

	var tracker fenceTracker
	for index, line := range lines {
		if got := tracker.advance(line); got != want[index] {
			t.Errorf("line %d %q: advance() = %v, want %v", index+1, line, got, want[index])
		}
	}
}

func TestSplitSlidesKeepsSeparatorsInsideFences(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"backtick fence", "# A\n\n```yaml\n---\nkey: value\n```\n\n---\n\n# B", 2},
		{"tilde fence", "# A\n\n~~~yaml\n---\n~~~\n\n---\n\n# B", 2},
		{"four backticks around three", "# A\n\n````markdown\n```\n---\n```\n````\n\n---\n\n# B", 2},
		{"indented fence", "# A\n\n  ```\n---\n  ```\n\n---\n\n# B", 2},
		{"a separator right after a fence still splits", "```\ncode\n```\n---\n# B", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(SplitSlidesPreservingCodeBlocks(tt.text)); got != tt.want {
				t.Errorf("SplitSlidesPreservingCodeBlocks() gave %d slides, want %d", got, tt.want)
			}
			if got := len(splitSlidesPreservingCodeBlocksWithLines(tt.text)); got != tt.want {
				t.Errorf("splitSlidesPreservingCodeBlocksWithLines() gave %d slides, want %d", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/parser -run 'TestFenceTracker|TestSplitSlidesKeepsSeparatorsInsideFences' -v`
Expected: compile failure, `undefined: fenceTracker`.

- [ ] **Step 3: Write `internal/parser/fences.go`**

```go
package parser

import "strings"

// fenceTracker follows fenced code blocks through markdown one line at a
// time, with CommonMark's rules. A fence opens with a run of at least
// three backticks or three tildes, indented by at most three spaces. A
// backtick fence's info string cannot hold a backtick. The block closes
// at a line that is a run of the same character, at least as long as the
// opening run, indented by at most three spaces, with only spaces after
// it. The zero value is outside any fence.
type fenceTracker struct {
	character byte
	length    int
}

// advance reads the next line and reports whether it belongs to a fenced
// code block, counting the opening and the closing fence lines.
func (tracker *fenceTracker) advance(line string) bool {
	character, run, rest, isRun := fenceRun(line)
	if tracker.length == 0 {
		if !isRun || (character == '`' && strings.Contains(rest, "`")) {
			return false
		}
		tracker.character, tracker.length = character, run
		return true
	}
	if isRun && character == tracker.character && run >= tracker.length && strings.TrimSpace(rest) == "" {
		tracker.character, tracker.length = 0, 0
	}
	return true
}

// fenceRun reports whether line starts, after at most three spaces, with
// a run of at least three backticks or three tildes. It returns the run's
// character and length, and the rest of the line after the run.
func fenceRun(line string) (character byte, run int, rest string, isRun bool) {
	indent := 0
	for indent < len(line) && line[indent] == ' ' {
		indent++
	}
	if indent > 3 || indent == len(line) {
		return 0, 0, "", false
	}
	character = line[indent]
	if character != '`' && character != '~' {
		return 0, 0, "", false
	}
	run = 1
	for indent+run < len(line) && line[indent+run] == character {
		run++
	}
	if run < 3 {
		return 0, 0, "", false
	}
	return character, run, line[indent+run:], true
}
```

- [ ] **Step 4: Use the tracker in both splitters**

In `SplitSlidesPreservingCodeBlocks`, delete `insideCodeBlock`, `codeBlockFenceLength` and the whole `if backtickCount >= 3 { ... }` block. Declare `var fences fenceTracker` before the loop. The loop starts:

```go
	for i, line := range lines {
		insideFence := fences.advance(line)

		// A "---" line is a slide delimiter only outside a fenced code block
		if !insideFence && slideDelimiter.MatchString(line) {
```

The rest of the loop body stays.

Make the same change in `splitSlidesPreservingCodeBlocksWithLines`: delete `insideCodeBlock`, `codeBlockFenceLength` and the backtick block, declare `var fences fenceTracker`, and start the loop with `insideFence := fences.advance(line)` and `if !insideFence && slideDelimiter.MatchString(line) {`.

Change the doc comment of `SplitSlidesPreservingCodeBlocks` to:

```go
// SplitSlidesPreservingCodeBlocks splits text on "---" delimiters while
// preserving code blocks. A "---" inside a fenced code block (backticks or
// tildes, see fenceTracker) is not a slide delimiter.
```

`countLeadingBackticks` stays, because `notes.go` and `slots.go` still use it.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/parser ./internal/tui`
Expected: PASS. `internal/tui` calls `SplitSlidesPreservingCodeBlocks` for image generation.

- [ ] **Step 6: Commit**

```bash
git add internal/parser/fences.go internal/parser/fences_test.go internal/parser/parser.go
git commit -m "fix(parser): keep a --- inside a tilde or indented fence in its slide"
```

---

### Task 4: Slide line ranges, and parsing that keeps broken slides

**Files:**
- Modify: `internal/parser/parser.go` (`Slide`, `Parse`, new `ParseKeepingErrors` and `parseSlide`)
- Modify: `internal/parser/parser_test.go`

**Interfaces:**
- Consumes: `splitSlidesPreservingCodeBlocksWithLines` (Task 3)
- Produces:
  - `parser.Slide.StartLine int`, `parser.Slide.EndLine int` (1-based, inclusive, deck file lines)
  - `func (p *Parser) ParseKeepingErrors(content []byte) (presentation *Presentation, slideErrors map[int]error)`
  - `Parse` keeps its signature and its error text (`slide N: ...`)

- [ ] **Step 1: Write the failing tests**

Append to `internal/parser/parser_test.go`, and add `"os"` and `"path/filepath"` to its imports:

```go
func TestParse_SlideLineRangesOfTheConferenceTalk(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "examples", "conference-talk.md"))
	if err != nil {
		t.Fatal(err)
	}
	pres, err := New().Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	want := [][2]int{{10, 14}, {18, 20}, {24, 32}, {36, 46}, {50, 68}, {72, 80}, {84, 86}, {90, 94}, {98, 106}}
	if len(pres.Slides) != len(want) {
		t.Fatalf("got %d slides, want %d", len(pres.Slides), len(want))
	}
	for index, slide := range pres.Slides {
		if slide.StartLine != want[index][0] || slide.EndLine != want[index][1] {
			t.Errorf("slide %d: lines %d-%d, want %d-%d", index+1, slide.StartLine, slide.EndLine, want[index][0], want[index][1])
		}
	}
}

func TestParse_SlideLineRanges(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    [][2]int
	}{
		{"no frontmatter", "# One\n\n---\n\n\n# Two\nmore\n", [][2]int{{1, 1}, {6, 7}}},
		{"windows line endings", "---\r\ntitle: T\r\n---\r\n\r\n# One\r\n---\r\n# Two\r\n", [][2]int{{5, 5}, {7, 7}}},
		{"an empty chunk is no slide", "# One\n---\n\n---\n# Two", [][2]int{{1, 1}, {5, 5}}},
		{"a separator inside a fence", "# One\n\n```yaml\n---\n```\n\n---\n# Two", [][2]int{{1, 5}, {8, 8}}},
		{"blank lines before the frontmatter", "\n\n---\ntitle: T\n---\n# One", [][2]int{{6, 6}}},
		{"a directive comment is part of the slide", "<!--\nlayout: title\n-->\n\n# One\n", [][2]int{{1, 5}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pres, err := New().Parse([]byte(tt.content))
			if err != nil {
				t.Fatalf("Parse() returned error: %v", err)
			}
			if len(pres.Slides) != len(tt.want) {
				t.Fatalf("got %d slides, want %d", len(pres.Slides), len(tt.want))
			}
			for index, slide := range pres.Slides {
				if slide.StartLine != tt.want[index][0] || slide.EndLine != tt.want[index][1] {
					t.Errorf("slide %d: lines %d-%d, want %d-%d", index+1, slide.StartLine, slide.EndLine, tt.want[index][0], tt.want[index][1])
				}
			}
		})
	}
}

func TestParseKeepingErrorsKeepsTheBrokenSlide(t *testing.T) {
	content := []byte("# One\n\n---\n\n## Two\n\n```component ./Chart.jsx\n{not json}\n```\n\n---\n\n# Three")

	pres, slideErrors := New().ParseKeepingErrors(content)
	if len(pres.Slides) != 3 {
		t.Fatalf("got %d slides, want 3", len(pres.Slides))
	}
	if len(slideErrors) != 1 || slideErrors[1] == nil {
		t.Fatalf("slideErrors = %v, want one error for index 1", slideErrors)
	}
	if !strings.Contains(slideErrors[1].Error(), "invalid component props JSON") {
		t.Errorf("error = %v, want it to name the props JSON", slideErrors[1])
	}
	broken := pres.Slides[1]
	if broken.Index != 1 || broken.StartLine != 5 || broken.EndLine != 9 {
		t.Errorf("broken slide = index %d, lines %d-%d, want index 1, lines 5-9", broken.Index, broken.StartLine, broken.EndLine)
	}
	if !strings.Contains(pres.Slides[2].HTML, "Three") {
		t.Errorf("slide 3 HTML = %q, want the slides after a broken one parsed", pres.Slides[2].HTML)
	}

	if _, err := New().Parse(content); err == nil || !strings.HasPrefix(err.Error(), "slide 2: ") {
		t.Errorf("Parse() error = %v, want one that starts with \"slide 2: \"", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/parser -run 'TestParse_SlideLineRanges|TestParseKeepingErrors' -v`
Expected: compile failure, `slide.StartLine undefined` and `undefined: ParseKeepingErrors`.

- [ ] **Step 3: Add the range fields to `Slide`**

In `internal/parser/parser.go`, add at the end of `Slide`:

```go
	// StartLine and EndLine are the 1-based, inclusive lines of the deck
	// file the slide covers: its text, including its directive comment,
	// without leading or trailing blank lines. Blank lines next to a "---"
	// separator belong to no slide, and neither do the separator lines
	// and the frontmatter.
	StartLine int
	EndLine   int
```

- [ ] **Step 4: Split `Parse` into `ParseKeepingErrors` and `parseSlide`**

Replace the whole `Parse` function with the three functions below. `parseSlide` is the old loop body. The comments inside it are the old ones, kept as they were.

```go
// Parse parses markdown content and returns a Presentation with slides.
// Slides are split on "---" delimiters. Frontmatter (if present) is
// skipped. A slide that fails to parse fails the whole parse, naming the
// first such slide.
func (p *Parser) Parse(content []byte) (*Presentation, error) {
	presentation, slideErrors := p.ParseKeepingErrors(content)
	for index := range presentation.Slides {
		if err, failed := slideErrors[index]; failed {
			return nil, fmt.Errorf("slide %d: %w", index+1, err)
		}
	}
	return presentation, nil
}

// ParseKeepingErrors parses content the way Parse does, but a slide that
// fails to parse stays in the presentation with its Index, line range,
// Directives and Content, and no HTML. slideErrors maps the 0-based index
// of each such slide to its error. An editor uses it to show every slide
// of a deck while one of them is broken.
func (p *Parser) ParseKeepingErrors(content []byte) (presentation *Presentation, slideErrors map[int]error) {
	// Normalize CRLF line endings to LF so a Windows-saved deck parses the
	// same way as one saved with Unix line endings, and no stray "\r"
	// characters end up in slide content, HTML, or notes. This never
	// changes a line's number: each "\r\n" becomes exactly one "\n".
	text := strings.ReplaceAll(string(content), "\r\n", "\n")

	// Skip frontmatter if present, and remember how many file lines it
	// took up, so a slide's line numbers can be translated back to real
	// file lines.
	text, frontmatterLineOffset := skipFrontmatterWithLineOffset(text)

	// Split content on --- delimiter, preserving code blocks, and keep
	// each slide's starting line for the same reason.
	chunks := splitSlidesPreservingCodeBlocksWithLines(text)

	presentation = &Presentation{Slides: make([]Slide, 0, len(chunks))}
	slideErrors = map[int]error{}
	for _, chunk := range chunks {
		// Skip empty slides
		if strings.TrimSpace(chunk.Content) == "" {
			continue
		}
		index := len(presentation.Slides)
		slide, err := p.parseSlide(chunk, frontmatterLineOffset, index)
		if err != nil {
			slideErrors[index] = err
		}
		presentation.Slides = append(presentation.Slides, slide)
	}
	return presentation, slideErrors
}

// parseSlide parses one non-empty chunk into the slide with the given
// 0-based index. frontmatterLineOffset is the number of file lines before
// the text the chunk came from. When the slide fails to parse, the
// returned slide still has its Index, line range, Directives and Content.
func (p *Parser) parseSlide(chunk slideChunk, frontmatterLineOffset int, index int) (Slide, error) {
	// Trim whitespace from slide content
	slideContent := strings.TrimSpace(chunk.Content)
	startLine := frontmatterLineOffset + chunk.StartLine + leadingWhitespaceLines(chunk.Content)
	slide := Slide{
		Index:     index,
		StartLine: startLine,
		EndLine:   startLine + strings.Count(slideContent, "\n"),
	}

	// Parse directives from HTML comments at slide start
	directives, contentAfterDirectives := parseDirectives(slideContent)
	// parseDirectives only ever removes a prefix, so contentAfterDirectives
	// is always a suffix of slideContent; the newlines in what was
	// removed are exactly the lines the directive comment took up.
	slideFileLine := startLine + strings.Count(slideContent[:len(slideContent)-len(contentAfterDirectives)], "\n")

	// Remove any further notes comments from the rest of the slide
	// (e.g. a trailing "<!-- notes: ... -->" after the content), and
	// join their text onto directive notes, if any, in document order.
	// This can remove lines from the middle of the slide, which this
	// package does not track, so a slide with a notes comment loses
	// exact component fence line numbers.
	var extraNotes []string
	beforeNotes := contentAfterDirectives
	contentAfterDirectives, extraNotes = extractNotesComments(contentAfterDirectives)
	lineNumbersExact := len(extraNotes) == 0 && contentAfterDirectives == beforeNotes
	if len(extraNotes) > 0 {
		joined := strings.Join(extraNotes, "\n\n")
		if directives.Notes != "" {
			directives.Notes = directives.Notes + "\n\n" + joined
		} else {
			directives.Notes = joined
		}
	}
	slide.Directives = directives

	// Pre-process images with attributes (e.g., {width=50%}) to HTML.
	// Every replacement stays on the single line the image markdown
	// was on, so this never shifts line numbers.
	contentAfterDirectives = transformImageAttributes(contentAfterDirectives, index+1)

	// Pre-process asciinema code blocks to move info string meta into
	// body. A block with metadata pairs turns one line into several,
	// which does shift the lines after it.
	if asciinemaInfoPattern.MatchString(contentAfterDirectives) {
		lineNumbersExact = false
	}
	contentAfterDirectives = transformAsciinemaBlocks(contentAfterDirectives)
	slide.Content = contentAfterDirectives

	sections, err := splitSlots(contentAfterDirectives)
	if err != nil {
		return slide, err
	}

	slots := make(map[string]string, len(sections))
	slotOrder := make([]string, 0, len(sections))
	fragmentCount := 0
	codeBlockCount := 0
	componentCount := 0
	autoFragment := directives.Fragments && !hasPauseMarkers(contentAfterDirectives)
	var fullHTML strings.Builder
	var codeBlocks []CodeBlock
	var components []Component
	for _, section := range sections {
		sectionFileLine := slideFileLine + (section.StartLine - 1)
		slotHTML, nextFragmentIndex, nextCodeBlockIndex, nextComponentIndex, blocks, sectionComponents, err := p.renderSlot(section.Content, fragmentCount, codeBlockCount, componentCount, sectionFileLine, lineNumbersExact)
		if err != nil {
			return slide, err
		}
		fragmentCount = nextFragmentIndex
		codeBlockCount = nextCodeBlockIndex
		componentCount = nextComponentIndex
		codeBlocks = append(codeBlocks, blocks...)
		components = append(components, sectionComponents...)
		if autoFragment {
			slotHTML, fragmentCount = autoFragmentListItems(slotHTML, fragmentCount)
		}
		slots[section.Name] = slotHTML
		slotOrder = append(slotOrder, section.Name)
		fullHTML.WriteString(slotHTML)
	}

	slide.HTML = fullHTML.String()
	slide.Slots = slots
	slide.SlotOrder = slotOrder
	slide.FragmentCount = fragmentCount
	slide.CodeBlocks = codeBlocks
	slide.Components = components
	return slide, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/parser`
Expected: PASS, including every existing test that checks a `slide N:` error.

Run: `go test ./internal/... -short`
Expected: PASS. Nothing outside the parser changes behavior.

- [ ] **Step 6: Commit**

```bash
git add internal/parser/parser.go internal/parser/parser_test.go
git commit -m "feat(parser): record each slide's line range and keep slides that fail to parse"
```

---

### Task 5: Fence lines and slide titles

**Files:**
- Modify: `internal/parser/codeblocks.go` (`FenceLines`, `openingFenceLine`)
- Create: `internal/parser/title.go`
- Create: `internal/parser/title_test.go`
- Modify: `internal/parser/fence_info_test.go`
- Modify: `internal/recorder/chapters.go`

**Interfaces:**
- Consumes: `fenceTracker` (Task 3), `componentFencePattern`, `fencedCodeBlockLine`, `codeBlockScanner` (existing)
- Produces:
  - `func parser.FenceLines(markdown string) []int`: the 1-based line, within `markdown`, of each non-component fence's opening line, in document order. For a slide's text this lines up with `Slide.CodeBlocks`. 0 means unknown (an empty fence with no info string).
  - `func parser.SlideTitle(content string) string`: the first ATX heading's text, or `""`.
  - `recorder.SlideTitle(content string, index int) string` keeps its signature and behavior.

- [ ] **Step 1: Write the failing tests**

Append to `internal/parser/fence_info_test.go`:

```go
func TestFenceLinesMatchTheSlideCodeBlocks(t *testing.T) {
	markdown := "<!--\nlayout: code-focus\n-->\n\n" +
		"```sql {driver: sqlite}\nSELECT 1;\n```\n\n" +
		"```component ./Chart.jsx\n{}\n```\n\n" +
		"- item\n\n  ```bash\n  ls\n  ```\n\n" +
		"```\nplain\n```"

	got := FenceLines(markdown)
	want := []int{5, 15, 19}
	if len(got) != len(want) {
		t.Fatalf("FenceLines() = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("FenceLines()[%d] = %d, want %d", index, got[index], want[index])
		}
	}

	pres, err := New().Parse([]byte(markdown))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if len(pres.Slides[0].CodeBlocks) != len(want) {
		t.Errorf("the slide has %d code blocks, FenceLines found %d", len(pres.Slides[0].CodeBlocks), len(want))
	}
}
```

`internal/parser/title_test.go`:

```go
package parser

import "testing"

func TestSlideTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"first heading", "Intro text\n\n## What We Knew\n\n# Later", "What We Knew"},
		{"bold and code markers are removed", "## The **naive** `lookup`", "The naive lookup"},
		{"a heading in a backtick fence is code", "```bash\n# not a title\n```\n\n# Real", "Real"},
		{"a heading in a tilde fence is code", "~~~\n# not a title\n~~~\n\n# Real", "Real"},
		{"no heading", "![](diagram.png)", ""},
		{"closing hashes are dropped", "# Title #", "Title"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SlideTitle(tt.content); got != tt.want {
				t.Errorf("SlideTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/parser -run 'TestFenceLines|TestSlideTitle' -v`
Expected: compile failure, `undefined: FenceLines` and `undefined: SlideTitle`.

- [ ] **Step 3: Add `FenceLines`**

Append to `internal/parser/codeblocks.go`:

```go
// FenceLines returns the 1-based line, within markdown, of each fenced
// code block's opening fence, in document order, leaving out ```component
// fences. Given a slide's text as it stands in the deck file, the result
// lines up with that slide's CodeBlocks: FenceLines(text)[i] is the line
// of CodeBlocks[i]. A directive or notes comment holds no fence, so the
// raw text and the parsed slide see the same fences. The line is 0 when
// goldmark gives no position, which happens only for an empty fence with
// no info string.
func FenceLines(markdown string) []int {
	source := []byte(markdown)
	doc := codeBlockScanner.Parser().Parse(text.NewReader(source))

	var lines []int
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		fence, isFence := node.(*ast.FencedCodeBlock)
		if !entering || !isFence {
			return ast.WalkContinue, nil
		}
		info := ""
		if fence.Info != nil {
			info = strings.TrimSpace(string(fence.Info.Segment.Value(source)))
		}
		if componentFencePattern.MatchString(info) {
			return ast.WalkContinue, nil
		}
		lines = append(lines, openingFenceLine(source, fence))
		return ast.WalkContinue, nil
	})
	return lines
}

// openingFenceLine returns the 1-based line of a fence's opening line. A
// fence with an info string starts on the info string's line. A fence
// without one starts on the line before its first content line. An empty
// fence with no info string has no position, so it gives 0.
func openingFenceLine(source []byte, fence *ast.FencedCodeBlock) int {
	if fence.Info != nil {
		return fencedCodeBlockLine(source, fence)
	}
	if fence.Lines().Len() > 0 {
		return fencedCodeBlockLine(source, fence) - 1
	}
	return 0
}
```

`text`, `ast` and `strings` are already imported in `codeblocks.go`.

- [ ] **Step 4: Move the title helper into the parser**

Create `internal/parser/title.go`:

```go
package parser

import (
	"regexp"
	"strings"
)

// titleHeadingPattern matches one ATX heading line and captures its text.
var titleHeadingPattern = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*\s*$`)

// titleEmphasisPattern matches paired bold and code markers around their
// text. Underscores are left alone on purpose: a heading on these slides
// is far more likely to hold get_user_by_id or __init__ than _italics_,
// and mangling an identifier is worse than keeping a stray marker.
var titleEmphasisPattern = regexp.MustCompile("\\*\\*(.+?)\\*\\*|`(.+?)`")

// SlideTitle returns the text of a slide's first ATX heading, with bold
// and code markers removed, or "" when the slide has no heading. Fenced
// code is skipped, because a comment inside a code block is not a heading
// however much it looks like one.
func SlideTitle(content string) string {
	var fences fenceTracker
	for _, line := range strings.Split(content, "\n") {
		if fences.advance(line) {
			continue
		}
		match := titleHeadingPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if title := strings.TrimSpace(stripTitleEmphasis(match[1])); title != "" {
			return title
		}
	}
	return ""
}

// stripTitleEmphasis removes paired markers and keeps the text they
// wrapped.
func stripTitleEmphasis(heading string) string {
	return titleEmphasisPattern.ReplaceAllStringFunc(heading, func(match string) string {
		groups := titleEmphasisPattern.FindStringSubmatch(match)
		for _, group := range groups[1:] {
			if group != "" {
				return group
			}
		}
		return match
	})
}
```

Before writing `stripTitleEmphasis`, read `stripEmphasis` in `internal/recorder/chapters.go` and keep the same fallback it has after the loop. If it differs from `return match`, copy it.

In `internal/recorder/chapters.go`, replace `SlideTitle` with:

```go
// SlideTitle names a slide for the chapter list: the text of its first
// heading (see parser.SlideTitle), or its number when it has none, which
// is the case for an image-only or component-only slide.
func SlideTitle(content string, index int) string {
	if title := parser.SlideTitle(content); title != "" {
		return title
	}
	return fmt.Sprintf("Slide %d", index+1)
}
```

Add `"github.com/MiniCodeMonkey/tap/internal/parser"` to its imports. Then:

Run: `grep -n "headingPattern\|emphasisPattern\|stripEmphasis" internal/recorder/*.go`
Expected: hits only for the declarations. Delete `headingPattern`, `emphasisPattern` and `stripEmphasis` from `chapters.go`, and remove any import that is now unused (`regexp` is the likely one; the compiler will say).

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/parser ./internal/recorder ./internal/cli -short`
Expected: PASS. `TestSlideTitle*` in `internal/recorder/chapters_test.go` still passes unchanged.

- [ ] **Step 6: Commit**

```bash
git add internal/parser/codeblocks.go internal/parser/title.go internal/parser/title_test.go internal/parser/fence_info_test.go internal/recorder/chapters.go
git commit -m "feat(parser): find each fence's line and a slide's title"
```

---

### Task 6: `config.Schema()` and its completeness test

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/schema.go`
- Create: `internal/config/schema_test.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/server/api_test.go`
- Modify: `internal/cli/dev.go` (one line)

**Interfaces:**
- Produces:
  - `type config.SchemaKey struct { Name string; Type string; Default any; Values []string; Description string; Keys []SchemaKey }`, JSON names `name`, `type`, `default`, `values` (omitted when empty), `description`, `keys` (omitted when empty)
  - `func config.Schema() []SchemaKey`
  - `func config.FromSource(source []byte) (*Config, error)`
  - `const config.DefaultDriverTimeoutSeconds = 30`, `const config.DefaultRecordingOutput = "recordings"`

- [ ] **Step 1: Write the failing tests**

`internal/config/schema_test.go`:

```go
package config

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// configKeyPaths records every frontmatter key path the Config struct
// reads, with the schema type its Go type must have. A map whose values
// are structs has entries under names the deck picks, written "<name>".
// A map of plain values has a fixed set of keys, listed in fixedMapKeys.
func configKeyPaths(t *testing.T, structType reflect.Type, prefix string, paths map[string]string) {
	t.Helper()
	for index := 0; index < structType.NumField(); index++ {
		field := structType.Field(index)
		name := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		path := prefix + name
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		switch fieldType.Kind() {
		case reflect.String:
			paths[path] = "string"
		case reflect.Bool:
			paths[path] = "boolean"
		case reflect.Int:
			paths[path] = "integer"
		case reflect.Slice:
			paths[path] = "list"
		case reflect.Struct:
			paths[path] = "object"
			configKeyPaths(t, fieldType, path+".", paths)
		case reflect.Map:
			if fieldType.Elem().Kind() == reflect.Struct {
				paths[path] = "map"
				configKeyPaths(t, fieldType.Elem(), path+".<name>.", paths)
				continue
			}
			keys, known := fixedMapKeys()[path]
			if !known {
				paths[path] = "a map with no fixed keys: add it to fixedMapKeys and Schema"
				continue
			}
			paths[path] = "object"
			for _, key := range keys {
				paths[path+"."+key] = "string"
			}
		default:
			t.Errorf("%s has Go kind %s, which the schema has no type for", path, fieldType.Kind())
		}
	}
}

// fixedMapKeys lists the keys of each Config map whose values are plain.
func fixedMapKeys() map[string][]string {
	names := make([]string, 0, len(themeColorKeys))
	for _, key := range themeColorKeys {
		names = append(names, key.name)
	}
	return map[string][]string{"themeColors": names}
}

// schemaKeyPaths records every key path and type in keys.
func schemaKeyPaths(keys []SchemaKey, prefix string, paths map[string]string) {
	for _, key := range keys {
		path := prefix + key.Name
		paths[path] = key.Type
		switch key.Type {
		case "object":
			schemaKeyPaths(key.Keys, path+".", paths)
		case "map":
			schemaKeyPaths(key.Keys, path+".<name>.", paths)
		}
	}
}

func TestSchemaCoversEveryConfigKey(t *testing.T) {
	want := map[string]string{}
	configKeyPaths(t, reflect.TypeOf(Config{}), "", want)
	got := map[string]string{}
	schemaKeyPaths(Schema(), "", got)

	var problems []string
	for path, wantType := range want {
		gotType, found := got[path]
		switch {
		case !found:
			problems = append(problems, "missing from Schema(): "+path)
		case gotType != wantType:
			problems = append(problems, "wrong type for "+path+": schema says "+gotType+", the struct needs "+wantType)
		}
	}
	for path := range got {
		if _, found := want[path]; !found {
			problems = append(problems, "in Schema() but not in the Config struct: "+path)
		}
	}
	sort.Strings(problems)
	for _, problem := range problems {
		t.Error(problem)
	}
}

// findSchemaKey returns the key at a dotted path such as
// "recording.warnAfter" or "drivers.<name>.timeout".
func findSchemaKey(t *testing.T, path string) SchemaKey {
	t.Helper()
	keys := Schema()
	parts := strings.Split(path, ".")
	for index := 0; index < len(parts); index++ {
		if parts[index] == "<name>" {
			continue
		}
		found := false
		for _, key := range keys {
			if key.Name == parts[index] {
				if index == len(parts)-1 {
					return key
				}
				keys = key.Keys
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no schema key at %s", path)
		}
	}
	t.Fatalf("no schema key at %s", path)
	return SchemaKey{}
}

func TestSchemaValuesMatchValidation(t *testing.T) {
	if got, want := findSchemaKey(t, "theme").Values, themes.Slugs(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("theme values = %v, want themes.Slugs() %v", got, want)
	}

	tests := []struct {
		path    string
		set     func(cfg *Config, value string)
		invalid string
	}{
		{"aspectRatio", func(cfg *Config, value string) { cfg.AspectRatio = value }, "5:4"},
		{"transition", func(cfg *Config, value string) { cfg.Transition = value }, "spin"},
		{"presenterLayout", func(cfg *Config, value string) { cfg.PresenterLayout = value }, "wide"},
	}
	for _, tt := range tests {
		values := findSchemaKey(t, tt.path).Values
		if len(values) == 0 {
			t.Errorf("%s lists no values", tt.path)
		}
		for _, value := range values {
			cfg := DefaultConfig()
			tt.set(cfg, value)
			if err := cfg.Validate(); err != nil {
				t.Errorf("%s: schema value %q fails Validate: %v", tt.path, value, err)
			}
		}
		cfg := DefaultConfig()
		tt.set(cfg, tt.invalid)
		if cfg.Validate() == nil {
			t.Errorf("%s: %q passes Validate, so the schema's list is not what Validate checks", tt.path, tt.invalid)
		}
	}
}

func TestSchemaDefaultsMatchTheConfigDefaults(t *testing.T) {
	defaults := DefaultConfig()
	for path, want := range map[string]any{
		"theme":                   defaults.Theme,
		"aspectRatio":             defaults.AspectRatio,
		"transition":              defaults.Transition,
		"fragments":               defaults.Fragments,
		"slideNumbers":            true,
		"drivers.<name>.timeout":  DefaultDriverTimeoutSeconds,
		"recording.output":        DefaultRecordingOutput,
		"recording.chapters":      true,
		"recording.showClicks":    false,
		"recording.display":       0,
	} {
		if got := findSchemaKey(t, path).Default; got != want {
			t.Errorf("%s default = %#v, want %#v", path, got, want)
		}
	}

	for path, want := range map[string]time.Duration{
		"recording.warnAfter": defaultWarnAfter,
		"recording.stopAfter": defaultStopAfter,
	} {
		text, isText := findSchemaKey(t, path).Default.(string)
		parsed, err := time.ParseDuration(text)
		if !isText || err != nil || parsed != want {
			t.Errorf("%s default = %#v, want a duration equal to %s", path, findSchemaKey(t, path).Default, want)
		}
	}
}

func TestEverySchemaKeyHasADescription(t *testing.T) {
	var check func(keys []SchemaKey, prefix string)
	check = func(keys []SchemaKey, prefix string) {
		for _, key := range keys {
			if key.Name == "" || strings.TrimSpace(key.Description) == "" {
				t.Errorf("%s%s has no name or no description", prefix, key.Name)
			}
			check(key.Keys, prefix+key.Name+".")
		}
	}
	check(Schema(), "")
}
```

Append to `internal/config/config_test.go`:

```go
func TestFromSource(t *testing.T) {
	cfg, err := FromSource([]byte("---\ntitle: Talk\ntheme: swiss\n---\n\n# One\n"))
	if err != nil {
		t.Fatalf("FromSource() error = %v", err)
	}
	if cfg.Title != "Talk" || cfg.Theme != "swiss" || cfg.AspectRatio != "16:9" {
		t.Errorf("cfg = %+v, want the frontmatter over the defaults", cfg)
	}

	cfg, err = FromSource([]byte("# No frontmatter\n"))
	if err != nil || cfg.Theme != "base" {
		t.Errorf("FromSource() without frontmatter = (%+v, %v), want the defaults", cfg, err)
	}

	if _, err := FromSource([]byte("---\ntitle: Talk\n")); err == nil {
		t.Error("FromSource() with unclosed frontmatter should fail")
	}
	if _, err := FromSource([]byte("")); err == nil {
		t.Error("FromSource() of an empty deck should fail, as Load does")
	}
}
```

Append to `internal/server/api_test.go` (it already imports `config` and `time`; add `time` if missing):

```go
func TestDefaultExecuteTimeoutMatchesTheSchemaDefault(t *testing.T) {
	if want := time.Duration(config.DefaultDriverTimeoutSeconds) * time.Second; DefaultExecuteTimeout != want {
		t.Errorf("DefaultExecuteTimeout = %s, want %s from config.DefaultDriverTimeoutSeconds", DefaultExecuteTimeout, want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config ./internal/server -run 'TestSchema|TestEverySchemaKey|TestFromSource|TestDefaultExecuteTimeout' -v`
Expected: compile failure, `undefined: Schema`, `undefined: FromSource`, `undefined: themeColorKeys`.

- [ ] **Step 3: Refactor `config.go`**

1. Add below the `Config` struct:

```go
// DefaultDriverTimeoutSeconds is how long a live code run may take when
// the deck's driver settings give no timeout.
const DefaultDriverTimeoutSeconds = 30

// DefaultRecordingOutput is where recordings go, relative to the deck,
// when recording.output is not set.
const DefaultRecordingOutput = "recordings"
```

2. Replace `validAspectRatios`, `validPresenterLayouts`, `validTransitions` and `validThemeColorKeys` with ordered lists and sets built from them. Keep the error messages in `Validate` exactly as they are.

```go
// aspectRatioValues lists the allowed aspectRatio values, in the order
// the schema shows them.
var aspectRatioValues = []string{"16:9", "4:3", "16:10"}

// validAspectRatios contains the allowed aspect ratio values.
var validAspectRatios = valueSet(aspectRatioValues)

// presenterLayoutValues lists the allowed presenterLayout values.
var presenterLayoutValues = []string{"standard", "notes-first", "duo", "slide-only", "notes-only"}

// validPresenterLayouts contains the allowed presenterLayout values.
var validPresenterLayouts = valueSet(presenterLayoutValues)

// transitionValues lists the allowed transition values.
var transitionValues = []string{"none", "fade", "slide", "push", "zoom"}

// validTransitions contains the allowed transition values.
var validTransitions = valueSet(transitionValues)

// themeColorKeys lists the allowed themeColors keys and the CSS custom
// property each one sets.
var themeColorKeys = []struct {
	name     string
	property string
}{
	{"background", "--color-bg"},
	{"text", "--color-text"},
	{"muted", "--color-muted"},
	{"accent", "--color-accent"},
	{"codeBg", "--color-code-bg"},
}

// validThemeColorKeys contains the allowed themeColors keys.
var validThemeColorKeys = func() map[string]bool {
	keys := make(map[string]bool, len(themeColorKeys))
	for _, key := range themeColorKeys {
		keys[key.name] = true
	}
	return keys
}()

// valueSet turns a list of allowed values into a set for lookups.
func valueSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
```

3. Replace `Load` with `Load`, `FromSource` and `parseFrontmatter`. `Load` keeps its behavior: a deck with no frontmatter gets the defaults and loads no `.env`.

```go
// Load reads a markdown file and parses its YAML frontmatter into a
// Config (see FromSource). When the deck has frontmatter, it also loads
// the .env file in the deck's folder and resolves environment variables
// in the driver settings.
func Load(path string) (*Config, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	cfg, found, err := parseFrontmatter(source)
	if err != nil || !found {
		return cfg, err
	}

	// Load .env file from presentation directory
	if err := LoadEnv(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("failed to load .env file: %w", err)
	}

	// Resolve environment variables in sensitive fields
	cfg.ResolveEnvVars()

	return cfg, nil
}

// FromSource parses the YAML frontmatter at the start of a deck's markdown
// into a Config, with Load's rules: the first line must be "---", and a
// deck without frontmatter gets DefaultConfig. Unlike Load, it reads no
// .env file and resolves no environment variables, so it has no side
// effects.
func FromSource(source []byte) (*Config, error) {
	cfg, _, err := parseFrontmatter(source)
	return cfg, err
}

// parseFrontmatter parses the frontmatter of source over DefaultConfig.
// found is false when source has no frontmatter.
func parseFrontmatter(source []byte) (cfg *Config, found bool, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(source))

	// Check for frontmatter start delimiter
	if !scanner.Scan() {
		return nil, false, fmt.Errorf("empty file")
	}

	firstLine := strings.TrimSpace(scanner.Text())
	if firstLine != "---" {
		// No frontmatter, return default config
		return DefaultConfig(), false, nil
	}

	// Read frontmatter content until closing delimiter
	var frontmatter strings.Builder
	foundEnd := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			foundEnd = true
			break
		}
		frontmatter.WriteString(line)
		frontmatter.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return nil, false, fmt.Errorf("error reading file: %w", err)
	}

	if !foundEnd {
		return nil, false, fmt.Errorf("frontmatter not closed: missing closing ---")
	}

	// Parse YAML frontmatter
	cfg = DefaultConfig()
	if err := yaml.Unmarshal([]byte(frontmatter.String()), cfg); err != nil {
		return nil, false, fmt.Errorf("failed to parse frontmatter: %w", err)
	}
	return cfg, true, nil
}
```

Add `"bytes"` to the imports.

- [ ] **Step 4: Write `internal/config/schema.go`**

```go
package config

import "github.com/MiniCodeMonkey/tap/internal/themes"

// SchemaKey describes one frontmatter key tap understands, for tools that
// build a form or completions from it (tap deck schema --json).
//
// Type is "string", "boolean", "integer", "list" (of strings), "object"
// (a fixed set of nested keys) or "map" (entries under names the deck
// picks, each with the nested keys). Default is the value tap uses when
// the key is left out, or nil when there is none. Values lists the
// allowed values when only some values are allowed.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type SchemaKey struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Default     any         `json:"default"`
	Values      []string    `json:"values,omitempty"`
	Description string      `json:"description"`
	Keys        []SchemaKey `json:"keys,omitempty"`
}

// Schema returns every frontmatter key tap understands, in the order the
// docs list them. The allowed values and the defaults come from the same
// lists and constants that Validate and DefaultConfig use, and
// TestSchemaCoversEveryConfigKey fails when a Config field has no key
// here.
func Schema() []SchemaKey {
	defaults := DefaultConfig()
	return []SchemaKey{
		{Name: "title", Type: "string", Description: "The deck's title, used in the PDF's metadata, in recording file names, and by themes that print it."},
		{Name: "author", Type: "string", Description: "The speaker's name, stored in the PDF's metadata."},
		{Name: "date", Type: "string", Description: "The date of the talk, as free text."},
		{Name: "theme", Type: "string", Default: defaults.Theme, Values: themes.Slugs(), Description: "The built-in theme. An unknown name falls back to base with a warning."},
		{Name: "customTheme", Type: "string", Description: "A CSS file, relative to the deck, loaded after the theme."},
		{Name: "themeColors", Type: "object", Description: "CSS colors that replace the theme's own.", Keys: themeColorSchemaKeys()},
		{Name: "aspectRatio", Type: "string", Default: defaults.AspectRatio, Values: aspectRatioValues, Description: "The shape of every slide."},
		{Name: "transition", Type: "string", Default: defaults.Transition, Values: transitionValues, Description: "The animation between slides. A slide's transition directive overrides it."},
		{Name: "fragments", Type: "boolean", Default: defaults.Fragments, Description: "Reveal list items one at a time on every slide. A slide's fragments directive overrides it."},
		{Name: "slideNumbers", Type: "boolean", Default: true, Description: "Whether the theme draws a slide number on every slide."},
		{Name: "presenterLayout", Type: "string", Values: presenterLayoutValues, Description: "The layout the presenter view opens in, unless the device has chosen one."},
		{Name: "drivers", Type: "map", Description: "Live code drivers by name, with their settings.", Keys: driverSchemaKeys()},
		{Name: "recording", Type: "object", Description: "Screen recording settings for tap dev and tap present.", Keys: recordingSchemaKeys()},
	}
}

// themeColorSchemaKeys describes the themeColors keys.
func themeColorSchemaKeys() []SchemaKey {
	keys := make([]SchemaKey, 0, len(themeColorKeys))
	for _, key := range themeColorKeys {
		keys = append(keys, SchemaKey{Name: key.name, Type: "string", Description: "A CSS color for " + key.property + "."})
	}
	return keys
}

// driverSchemaKeys describes the settings of one entry under drivers.
func driverSchemaKeys() []SchemaKey {
	return []SchemaKey{
		{Name: "command", Type: "string", Description: "The program a custom driver runs. The code goes to its standard input."},
		{Name: "args", Type: "list", Description: "Arguments passed to the command."},
		{Name: "timeout", Type: "integer", Default: DefaultDriverTimeoutSeconds, Description: "Seconds before a run is stopped."},
		{Name: "connections", Type: "map", Description: "Named connections, which a code block picks with connection: <name>.", Keys: []SchemaKey{
			{Name: "host", Type: "string", Description: "The database host. ${NAME} reads it from the environment or .env."},
			{Name: "user", Type: "string", Description: "The user name. ${NAME} reads it from the environment or .env."},
			{Name: "password", Type: "string", Description: "The password. Write ${NAME} to read it from the environment or .env instead of the deck."},
			{Name: "database", Type: "string", Description: "The database name. ${NAME} reads it from the environment or .env."},
			{Name: "path", Type: "string", Description: "A database file, relative to the deck, for sqlite. ${NAME} reads it from the environment or .env."},
			{Name: "port", Type: "integer", Description: "The database port."},
		}},
	}
}

// recordingSchemaKeys describes the keys under recording.
func recordingSchemaKeys() []SchemaKey {
	return []SchemaKey{
		{Name: "output", Type: "string", Default: DefaultRecordingOutput, Description: "The folder recordings are written to, relative to the deck."},
		{Name: "audio", Type: "string", Default: "default", Description: "The microphone: default, none for a silent recording, or a CoreAudio device UID."},
		{Name: "warnAfter", Type: "string", Default: "90m", Description: "How long a recording runs before tap warns about it, as a duration such as 90m."},
		{Name: "stopAfter", Type: "string", Default: "3h", Description: "How long a recording runs before it stops itself, as a duration such as 3h, or off."},
		{Name: "display", Type: "integer", Default: 0, Description: "The display the record picker preselects. 0 is the main display."},
		{Name: "showClicks", Type: "boolean", Default: false, Description: "Draw mouse clicks in the recording."},
		{Name: "chapters", Type: "boolean", Default: true, Description: "Write a chapter list of slide timings next to each recording."},
	}
}
```

- [ ] **Step 5: Use `DefaultRecordingOutput` in `tap dev`**

In `internal/cli/dev.go`, change `recordOutputDir := filepath.Join(baseDir, "recordings")` to:

```go
	recordOutputDir := filepath.Join(baseDir, config.DefaultRecordingOutput)
```

Run: `grep -rn '"recordings"' internal --include='*.go' | grep -v _test`
Expected: only the constant in `internal/config/config.go`. If another default hides elsewhere (a recorder default, for example), replace it with the constant too.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/config ./internal/server ./internal/cli -short`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config internal/server/api_test.go internal/cli/dev.go
git commit -m "feat(config): describe every frontmatter key in one schema"
```

---

### Task 7: `internal/slidelist`

**Files:**
- Create: `internal/slidelist/slidelist.go`
- Create: `internal/slidelist/slidelist_test.go`

**Interfaces:**
- Consumes: `config.FromSource` (Task 6); `parser.ParseKeepingErrors`, `Slide.StartLine`/`EndLine` (Task 4); `parser.FenceLines`, `parser.SlideTitle` (Task 5); `SlideDirectives.Skip` (Task 1); `components.Resolve`, `transformer.NewWithBaseDir`, `layouts.Validate` (existing)
- Produces (the P3 contract, used by Task 9 and by P6's `PUT /api/app/source`):
  - `type Result struct { Slides []Slide; Errors []string }`
  - `type Slide struct { Number, StartLine, EndLine int; Layout, Title string; Fragments, Steps int; Skip bool; Errors []string; CodeBlocks []CodeBlock }`
  - `type CodeBlock struct { Block int; Language, Driver string; Live bool; Line int }`
  - `func Build(source []byte, baseDir string) (Result, error)`

- [ ] **Step 1: Write the failing tests**

`internal/slidelist/slidelist_test.go`:

```go
package slidelist

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func buildFile(t *testing.T, path string) Result {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Build(source, filepath.Dir(path))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return result
}

func buildSource(t *testing.T, source string) Result {
	t.Helper()
	result, err := Build([]byte(source), t.TempDir())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return result
}

func TestBuildTheConferenceTalk(t *testing.T) {
	result := buildFile(t, filepath.Join("..", "..", "examples", "conference-talk.md"))
	if len(result.Slides) != 9 || len(result.Errors) != 0 {
		t.Fatalf("got %d slides and errors %v, want 9 slides and no errors", len(result.Slides), result.Errors)
	}

	third := result.Slides[2]
	if third.Number != 3 || third.StartLine != 24 || third.EndLine != 32 || third.Title != "What We Knew" || third.Fragments != 1 || third.Steps != 0 || third.Skip {
		t.Errorf("slide 3 = %+v", third)
	}

	fourth := result.Slides[3]
	if fourth.Layout != "code-focus" {
		t.Errorf("slide 4 layout = %q, want code-focus", fourth.Layout)
	}
	want := CodeBlock{Block: 1, Language: "sql", Driver: "sqlite", Live: true, Line: 40}
	if len(fourth.CodeBlocks) != 1 || fourth.CodeBlocks[0] != want {
		t.Errorf("slide 4 code blocks = %+v, want [%+v]", fourth.CodeBlocks, want)
	}

	if result.Slides[0].Layout != "title" || result.Slides[0].Title != "Debugging Production at 3am" {
		t.Errorf("slide 1 = %+v", result.Slides[0])
	}
}

func TestBuildMarksSkippedSlides(t *testing.T) {
	result := buildSource(t, "# One\n\n---\n\n<!-- skip: true -->\n\n# Two\n")
	if len(result.Slides) != 2 {
		t.Fatalf("got %d slides, want 2: a skipped slide is still listed", len(result.Slides))
	}
	skipped := result.Slides[1]
	if !skipped.Skip || skipped.Number != 2 || skipped.StartLine != 5 || skipped.EndLine != 7 {
		t.Errorf("slide 2 = %+v, want skipped, number 2, lines 5-7", skipped)
	}
	if result.Slides[0].Skip {
		t.Error("slide 1 is marked skipped")
	}
}

func TestBuildKeepsBrokenSlides(t *testing.T) {
	result := buildSource(t, "<!-- layout: sectoin -->\n\n# One\n\n---\n\n```component ./Chart.jsx\n{not json}\n```\n\n---\n\n# Three\n")
	if len(result.Slides) != 3 {
		t.Fatalf("got %d slides, want 3", len(result.Slides))
	}
	if errors := strings.Join(result.Slides[0].Errors, "\n"); !strings.Contains(errors, `unknown layout "sectoin"`) {
		t.Errorf("slide 1 errors = %q, want the unknown layout", errors)
	}
	if errors := strings.Join(result.Slides[1].Errors, "\n"); !strings.Contains(errors, "invalid component props JSON") {
		t.Errorf("slide 2 errors = %q, want the props JSON error", errors)
	}
	if len(result.Slides[2].Errors) != 0 {
		t.Errorf("slide 3 errors = %v, want none", result.Slides[2].Errors)
	}
}

func TestBuildReportsFrontmatterErrorsAndStillListsSlides(t *testing.T) {
	result := buildSource(t, "---\naspectRatio: \"5:4\"\n---\n\n# One\n")
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0], "invalid aspectRatio") {
		t.Errorf("Errors = %v, want the aspectRatio error", result.Errors)
	}
	if len(result.Slides) != 1 || result.Slides[0].StartLine != 5 {
		t.Errorf("Slides = %+v, want slide 1 at line 5", result.Slides)
	}
}

func TestBuildCountsAComponentsSteps(t *testing.T) {
	dir := t.TempDir()
	component := "export const steps = 3;\n\nexport default function Steps() {\n  return null;\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "Steps.jsx"), []byte(component), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Build([]byte("<!-- layout: ./Steps.jsx -->\n\n# Built\n"), dir)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	slide := result.Slides[0]
	if slide.Steps != 3 || slide.Layout != "./Steps.jsx" || len(slide.Errors) != 0 {
		t.Errorf("slide = %+v, want 3 steps from the component, its path as the layout, no errors", slide)
	}
}

func TestBuildJSONShape(t *testing.T) {
	encoded, err := json.Marshal(buildSource(t, "<!-- layout: section -->\n# One\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"slides":[{"number":1,"startLine":1,"endLine":2,"layout":"section","title":"One","fragments":0,"steps":0,"skip":false,"errors":[],"codeBlocks":[]}],"errors":[]}`
	if string(encoded) != want {
		t.Errorf("JSON =\n%s\nwant\n%s", encoded, want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/slidelist -v`
Expected: compile failure, `undefined: Build`.

- [ ] **Step 3: Write `internal/slidelist/slidelist.go`**

```go
// Package slidelist describes each slide of a deck with what an editor
// needs to draw it: its line range, layout, title, reveal counts, skip
// flag, errors and code blocks. tap slide list prints it, and the source
// endpoint of tap dev --app answers with it.
package slidelist

import (
	"fmt"
	"strings"

	"github.com/MiniCodeMonkey/tap/internal/components"
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/layouts"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// Result is the slide list of one deck.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type Result struct {
	Slides []Slide `json:"slides"`
	// Errors are problems with the deck as a whole, such as frontmatter
	// that does not parse. A problem with one slide is in its own Errors.
	Errors []string `json:"errors"`
}

// Slide describes one slide of a deck.
//
// StartLine and EndLine are 1-based line numbers in the deck file, and
// both are inclusive. The range covers the slide's text, including its
// directive comment, without leading or trailing blank lines. Blank lines
// next to a "---" separator belong to no slide. The separator lines and
// the frontmatter belong to no slide either. A "---" inside a fenced code
// block is text, not a separator.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type Slide struct {
	// Number is the slide's 1-based position in the deck. Skipped slides
	// count, so the number matches URLs and every other tap command.
	Number    int `json:"number"`
	StartLine int `json:"startLine"`
	EndLine   int `json:"endLine"`
	// Layout is the layout the slide renders with: its layout directive,
	// or the layout tap picks from its content. A slide that is a
	// component has the component's path.
	Layout string `json:"layout"`
	// Title is the text of the slide's first heading, or "" when it has
	// none.
	Title string `json:"title"`
	// Fragments counts the slide's fragment reveals: its pause markers, or
	// its list items when its fragments directive is on.
	Fragments int `json:"fragments"`
	// Steps is the slide's step count: its steps directive, or else the
	// largest "export const steps" among its components.
	Steps int `json:"steps"`
	// Skip is true when the slide's skip directive is true.
	Skip bool `json:"skip"`
	// Errors lists the slide's problems: a parse error, a layout or slot
	// warning, a component that failed to build.
	Errors     []string    `json:"errors"`
	CodeBlocks []CodeBlock `json:"codeBlocks"`
}

// CodeBlock describes one fenced code block of a slide. A ```component
// fence is not a code block.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type CodeBlock struct {
	// Block is the block's 1-based position among the slide's code blocks.
	Block    int    `json:"block"`
	Language string `json:"language"`
	Driver   string `json:"driver"`
	// Live is true when the block names a driver, so tap dev can run it.
	Live bool `json:"live"`
	// Line is the 1-based deck file line of the block's opening fence. It
	// is 0 only for an empty fence with no info string.
	Line int `json:"line"`
}

// Build returns the slide list of a deck. source is the deck's markdown,
// and baseDir is its folder, where component files resolve. Build bundles
// the deck's components, because a component's steps export sets its
// slide's step count. A problem in the deck goes in the Result, never in
// the error, so the list stays complete while a slide is broken.
func Build(source []byte, baseDir string) (Result, error) {
	result := Result{Slides: []Slide{}, Errors: []string{}}

	cfg, err := config.FromSource(source)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("frontmatter: %v", err))
		cfg = config.DefaultConfig()
	} else if err := cfg.Validate(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("frontmatter: %v", err))
	}

	presentation, slideErrors := parser.New().ParseKeepingErrors(source)
	resolved := components.Resolve(presentation, baseDir, components.Options{
		DeckDirectory:   baseDir,
		AssetPublicPath: "/components/",
	})
	deckTransformer := transformer.NewWithBaseDir(cfg, baseDir)
	deckTransformer.SetComponents(resolved)
	transformed := deckTransformer.Transform(presentation)

	warningsBySlide := map[int][]string{}
	for _, warning := range layouts.Validate(transformed) {
		warningsBySlide[warning.SlideNumber] = append(warningsBySlide[warning.SlideNumber], warning.Message)
	}

	lines := strings.Split(strings.ReplaceAll(string(source), "\r\n", "\n"), "\n")
	for index, parsed := range presentation.Slides {
		rendered := transformed.Slides[index]
		number := index + 1
		slide := Slide{
			Number:     number,
			StartLine:  parsed.StartLine,
			EndLine:    parsed.EndLine,
			Layout:     rendered.Layout,
			Title:      parser.SlideTitle(parsed.Content),
			Fragments:  rendered.FragmentCount,
			Steps:      rendered.Steps,
			Skip:       parsed.Directives.Skip,
			Errors:     []string{},
			CodeBlocks: []CodeBlock{},
		}
		if rendered.Component != nil {
			slide.Layout = rendered.Component.Source
		}

		if err, failed := slideErrors[index]; failed {
			slide.Errors = append(slide.Errors, err.Error())
		}
		slide.Errors = append(slide.Errors, warningsBySlide[number]...)
		for _, component := range rendered.Components {
			if component.Error != "" {
				slide.Errors = append(slide.Errors, fmt.Sprintf("component %q failed to build: %s", component.Source, component.Error))
			}
		}

		fenceLines := parser.FenceLines(slideText(lines, parsed.StartLine, parsed.EndLine))
		for blockIndex, block := range parsed.CodeBlocks {
			codeBlock := CodeBlock{
				Block:    blockIndex + 1,
				Language: block.Language,
				Driver:   block.Meta.Driver,
				Live:     block.Meta.Driver != "",
			}
			if blockIndex < len(fenceLines) && fenceLines[blockIndex] > 0 {
				codeBlock.Line = parsed.StartLine + fenceLines[blockIndex] - 1
			}
			slide.CodeBlocks = append(slide.CodeBlocks, codeBlock)
		}

		result.Slides = append(result.Slides, slide)
	}
	return result, nil
}

// slideText returns lines startLine to endLine, 1-based and inclusive,
// joined back into one string.
func slideText(lines []string, startLine, endLine int) string {
	if startLine < 1 || endLine > len(lines) || startLine > endLine {
		return ""
	}
	return strings.Join(lines[startLine-1:endLine], "\n")
}
```

Check the field names of `components.Options` against `internal/cli/components.go` (`DeckDirectory`, `Minify`, `SourceMaps`, `AssetPublicPath`). `Minify` and `SourceMaps` stay false: the slide list needs only each bundle's steps export.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/slidelist -v`
Expected: PASS. If `TestBuildTheConferenceTalk` fails only on a layout name that tap's own detection picks (for example slide 1 not being `title`), run `go test ./internal/slidelist -run TestBuildTheConferenceTalk -v`, check the detected layout in `tap dev`, and fix the test's expectation, not the code. The line numbers and the code block must match exactly.

Run: `go vet ./internal/slidelist`
Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add internal/slidelist
git commit -m "feat(slidelist): describe each slide with its line range, counts and code blocks"
```

---

### Task 8: Golden slide lists for the example decks

**Files:**
- Create: `internal/slidelist/golden_test.go`
- Create: `internal/slidelist/testdata/fences.md`
- Create: `internal/slidelist/testdata/golden/*.json` (generated)

**Interfaces:**
- Consumes: `Build` (Task 7)

- [ ] **Step 1: Write the fixture**

`internal/slidelist/testdata/fences.md`, exactly (the line numbers in step 2 depend on it):

`````markdown
---
title: Fences
---

# Fences keep their separators

```yaml
---
name: inside a backtick fence
---
```

---

~~~markdown
---
inside a tilde fence
~~~

---

````markdown
```yaml
---
```
````

---

- A list item with a fence

  ```yaml
  ---
  ```

---

<!-- skip: true -->

## A skipped slide

```sql {driver: sqlite, connection: demo} {2}
SELECT 1;
SELECT 2;
```
`````

The file ends with a newline after the last fence.

- [ ] **Step 2: Write the tests**

`internal/slidelist/golden_test.go`:

```go
package slidelist

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/golden")

// goldenDecks maps each golden file name to the deck it pins.
var goldenDecks = map[string]string{
	"basic":           "../../examples/basic.md",
	"code-demo":       "../../examples/code-demo.md",
	"conference-talk": "../../examples/conference-talk.md",
	"launch-day":      "../../examples/launch-day.md",
	"map-demo":        "../../examples/map-demo.md",
	"sql-demo":        "../../examples/sql-demo.md",
	"theme-tour":      "../../examples/theme-tour.md",
	"components":      "../../examples/components/deck.md",
	"fences":          "testdata/fences.md",
}

func TestGoldenSlideLists(t *testing.T) {
	for name, path := range goldenDecks {
		t.Run(name, func(t *testing.T) {
			got, err := json.MarshalIndent(buildFile(t, path), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')

			goldenPath := filepath.Join("testdata", "golden", name+".json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("%v: run go test ./internal/slidelist -run TestGoldenSlideLists -update", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("slide list of %s differs from %s. If the change is intended, run with -update and review the diff.\ngot:\n%s", path, goldenPath, got)
			}
		})
	}
}

func TestEveryExampleDeckHasAGoldenFile(t *testing.T) {
	decks, err := filepath.Glob("../../examples/*.md")
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, path := range goldenDecks {
		known[path] = true
	}
	for _, deck := range decks {
		if !known[filepath.ToSlash(deck)] {
			t.Errorf("%s has no entry in goldenDecks", deck)
		}
	}
}

func TestTheFencesFixture(t *testing.T) {
	result := buildFile(t, filepath.Join("testdata", "fences.md"))

	wantRanges := [][2]int{{5, 11}, {15, 18}, {22, 26}, {30, 34}, {38, 45}}
	wantBlocks := []CodeBlock{
		{Block: 1, Language: "yaml", Line: 7},
		{Block: 1, Language: "markdown", Line: 15},
		{Block: 1, Language: "markdown", Line: 22},
		{Block: 1, Language: "yaml", Line: 32},
		{Block: 1, Language: "sql", Driver: "sqlite", Live: true, Line: 42},
	}
	if len(result.Slides) != len(wantRanges) {
		t.Fatalf("got %d slides, want %d: a --- inside a fence must not split a slide", len(result.Slides), len(wantRanges))
	}
	for index, slide := range result.Slides {
		if slide.StartLine != wantRanges[index][0] || slide.EndLine != wantRanges[index][1] {
			t.Errorf("slide %d: lines %d-%d, want %d-%d", index+1, slide.StartLine, slide.EndLine, wantRanges[index][0], wantRanges[index][1])
		}
		if len(slide.CodeBlocks) != 1 || slide.CodeBlocks[0] != wantBlocks[index] {
			t.Errorf("slide %d code blocks = %+v, want [%+v]", index+1, slide.CodeBlocks, wantBlocks[index])
		}
		if len(slide.Errors) != 0 {
			t.Errorf("slide %d errors = %v", index+1, slide.Errors)
		}
	}
	if last := result.Slides[4]; !last.Skip || last.Title != "A skipped slide" {
		t.Errorf("slide 5 = %+v, want skipped, titled", last)
	}
	for index := 0; index < 4; index++ {
		if result.Slides[index].Skip {
			t.Errorf("slide %d is marked skipped", index+1)
		}
	}
	if strings.Join(result.Errors, "") != "" {
		t.Errorf("deck errors = %v", result.Errors)
	}
}
```

- [ ] **Step 3: Run the fixture test**

Run: `go test ./internal/slidelist -run 'TestTheFencesFixture|TestEveryExampleDeckHasAGoldenFile' -v`
Expected: PASS. If `TestEveryExampleDeckHasAGoldenFile` names a deck that is missing, add it to `goldenDecks`.

- [ ] **Step 4: Generate the golden files and review them**

Run: `go test ./internal/slidelist -run TestGoldenSlideLists -update`
Then: `go test ./internal/slidelist -v`
Expected: PASS.

Review before committing. For each file in `internal/slidelist/testdata/golden/`:
- no `errors` entries, and no absolute paths anywhere (`grep -rn '/Users/\|/home/' internal/slidelist/testdata/golden` prints nothing);
- `conference-talk.json` slide 4 has `"driver": "sqlite"`, `"live": true`, `"line": 40`;
- `sql-demo.json` has a `live` block on every slide that has a sql fence;
- `components.json` has a nonzero `steps` for the `./slides/RollingDeploy.jsx` slide, and that path as its layout;
- `map-demo.json` has `steps` of at least 1 on each map slide.

If a value is wrong, fix the code, not the golden file.

- [ ] **Step 5: Commit**

```bash
git add internal/slidelist
git commit -m "test(slidelist): pin the slide lists of the example decks"
```

---

### Task 9: `tap slide list [deck] --json`

**Files:**
- Create: `internal/cli/slide_list.go`
- Create: `internal/cli/slide_list_test.go`
- Modify: `internal/cli/conventions_test.go` (`expectedCommands`)

**Interfaces:**
- Consumes: `slideCmd` (P1, `slide.go`); `resolveDeck`, `firstArg`, `userError`, `internalError`, `codeDeckNotFound`, `codeInternal`, `printJSONOK`, `runTap` (P1); `slidelist.Build` (Task 7)
- Produces: `var slideListCmd`, `func runSlideList(cmd *cobra.Command, args []string) error`, `func printSlideList(w io.Writer, result slidelist.Result) error`

- [ ] **Step 1: Write the failing tests**

`internal/cli/slide_list_test.go`:

```go
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/slidelist"
)

func TestSlideListCommandShape(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"slide", "list"})
	if err != nil || command.Name() != "list" || command.Parent().Name() != "slide" {
		t.Fatalf("tap slide list not found: %v", err)
	}
	if command.Use != "list [deck]" {
		t.Errorf("Use = %q, want %q", command.Use, "list [deck]")
	}
	if command.Flags().Lookup("json") == nil {
		t.Error("missing --json")
	}
}

func TestSlideListJSON(t *testing.T) {
	deck := filepath.Join("..", "..", "examples", "conference-talk.md")
	exitCode, stdout, stderr := runTap(t, "slide", "list", deck, "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if !strings.HasPrefix(stdout, "{\n  \"ok\": true,\n  \"slides\": [") {
		t.Errorf("stdout starts %q, want ok first, then slides", stdout[:min(len(stdout), 40)])
	}

	var output struct {
		OK     bool              `json:"ok"`
		Slides []slidelist.Slide `json:"slides"`
		Errors []string          `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !output.OK || len(output.Slides) != 9 || output.Errors == nil {
		t.Fatalf("output = %+v", output)
	}
	block := output.Slides[3].CodeBlocks[0]
	if block.Driver != "sqlite" || !block.Live || block.Line != 40 {
		t.Errorf("slide 4 block = %+v", block)
	}
}

func TestSlideListTable(t *testing.T) {
	dir := t.TempDir()
	deck := filepath.Join(dir, "talk.md")
	if err := os.WriteFile(deck, []byte("# One\n\n---\n\n<!-- skip: true -->\n\n# Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	exitCode, stdout, stderr := runTap(t, "slide", "list", deck)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "#") {
		t.Fatalf("stdout =\n%s\nwant a header and two rows", stdout)
	}
	if !strings.Contains(lines[2], "5-7") || !strings.Contains(lines[2], "Two") || !strings.Contains(lines[2], "skipped") {
		t.Errorf("row 2 = %q, want lines 5-7, the title and \"skipped\"", lines[2])
	}
}

func TestSlideListMissingDeck(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "slide", "list", filepath.Join(t.TempDir(), "missing.md"), "--json")
	if exitCode != exitUserError || !strings.Contains(stdout, `"code": "deck_not_found"`) {
		t.Errorf("exit %d, stdout %q", exitCode, stdout)
	}
}
```

In `internal/cli/conventions_test.go`, add `"tap slide list",` to `expectedCommands`, right after `"tap slide add",`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestSlideList|TestCommandTree' -short`
Expected: FAIL, `tap slide list not found`, and `TestCommandTree` lists the missing command.

- [ ] **Step 3: Write `internal/cli/slide_list.go`**

```go
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/slidelist"
)

var slideListJSON bool

var slideListCmd = &cobra.Command{
	Use:   "list [deck]",
	Short: "List a deck's slides with their line ranges",
	Long: `List every slide of a deck: its number, the lines it covers in the
file, its layout, title, step and fragment counts, whether it is skipped,
its errors, and its code blocks.

Line numbers are 1-based. A slide's range covers its text, including its
directive comment, without the blank lines around it. The "---" separator
lines and the frontmatter belong to no slide. Slide numbers count skipped
slides, so they match the numbers every other tap command uses.

Examples:
  tap slide list                 # The deck in this folder
  tap slide list talk.md
  tap slide list talk.md --json  # For editors and scripts`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSlideList,
}

func init() {
	slideCmd.AddCommand(slideListCmd)
	slideListCmd.Flags().BoolVar(&slideListJSON, "json", false, "print the slide list as JSON")
}

func runSlideList(cmd *cobra.Command, args []string) error {
	file, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(file)
	if err != nil {
		return internalError(codeInternal, fmt.Errorf("failed to resolve file path: %w", err))
	}
	source, err := os.ReadFile(absolute)
	if err != nil {
		return userError(codeDeckNotFound, fmt.Errorf("failed to read %s: %w", file, err))
	}

	result, err := slidelist.Build(source, filepath.Dir(absolute))
	if err != nil {
		return internalError(codeInternal, err)
	}
	if slideListJSON {
		return printJSONOK(cmd.OutOrStdout(), result)
	}
	return printSlideList(cmd.OutOrStdout(), result)
}

// printSlideList writes the slide list as a table, one slide per row. The
// deck's own errors come first, and each slide's errors come after the
// table.
func printSlideList(w io.Writer, result slidelist.Result) error {
	for _, message := range result.Errors {
		fmt.Fprintf(w, "error: %s\n", message)
	}

	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "#\tLINES\tLAYOUT\tTITLE\tSTEPS\tFRAGMENTS\tNOTES")
	for _, slide := range result.Slides {
		fmt.Fprintf(table, "%d\t%d-%d\t%s\t%s\t%d\t%d\t%s\n",
			slide.Number, slide.StartLine, slide.EndLine, slide.Layout, slide.Title, slide.Steps, slide.Fragments, slideNotes(slide))
	}
	if err := table.Flush(); err != nil {
		return err
	}

	for _, slide := range result.Slides {
		for _, message := range slide.Errors {
			fmt.Fprintf(w, "slide %d: %s\n", slide.Number, message)
		}
	}
	return nil
}

// slideNotes sums up a slide for the NOTES column: whether it is skipped,
// the driver of each live code block, and how many errors it has.
func slideNotes(slide slidelist.Slide) string {
	var notes []string
	if slide.Skip {
		notes = append(notes, "skipped")
	}
	for _, block := range slide.CodeBlocks {
		if block.Live {
			notes = append(notes, "live: "+block.Driver)
		}
	}
	switch count := len(slide.Errors); {
	case count == 1:
		notes = append(notes, "1 error")
	case count > 1:
		notes = append(notes, fmt.Sprintf("%d errors", count))
	}
	return strings.Join(notes, ", ")
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestSlideList|TestCommandTree|TestEveryCommandFollows' -short -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/slide_list.go internal/cli/slide_list_test.go internal/cli/conventions_test.go
git commit -m "feat(cli): add tap slide list"
```

---

### Task 10: `tap deck schema --json`

**Files:**
- Create: `internal/cli/deck_schema.go`
- Create: `internal/cli/deck_schema_test.go`
- Modify: `internal/cli/conventions_test.go` (`expectedCommands`)

**Interfaces:**
- Consumes: `config.Schema`, `config.SchemaKey` (Task 6); `printJSONOK`, `runTap` (P1)
- Produces: `var deckCmd`, `var deckSchemaCmd`, `func runDeckSchema(cmd *cobra.Command, args []string) error`. The `--json` result is `{"ok": true, "keys": [<SchemaKey>...]}`.

- [ ] **Step 1: Write the failing tests**

`internal/cli/deck_schema_test.go`:

```go
package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
)

func TestDeckSchemaJSON(t *testing.T) {
	exitCode, stdout, stderr := runTap(t, "deck", "schema", "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	var output struct {
		OK   bool               `json:"ok"`
		Keys []config.SchemaKey `json:"keys"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !output.OK || len(output.Keys) != len(config.Schema()) {
		t.Fatalf("output = %+v, want ok and every schema key", output)
	}

	byName := map[string]config.SchemaKey{}
	for _, key := range output.Keys {
		byName[key.Name] = key
	}
	if theme := byName["theme"]; theme.Default != "base" || !strings.Contains(strings.Join(theme.Values, ","), "terminal") {
		t.Errorf("theme = %+v, want default base and terminal among the values", theme)
	}
	if drivers := byName["drivers"]; drivers.Type != "map" || len(drivers.Keys) == 0 {
		t.Errorf("drivers = %+v, want a map with nested keys", drivers)
	}
}

func TestDeckSchemaTable(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "deck", "schema")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d", exitCode)
	}
	for _, want := range []string{"KEY", "aspectRatio", "recording.warnAfter", "drivers.<name>.connections.<name>.password", "themeColors.accent"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestDeckSchemaTakesNoArguments(t *testing.T) {
	if exitCode, _, _ := runTap(t, "deck", "schema", "talk.md"); exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
}
```

In `internal/cli/conventions_test.go`, add `"tap deck",` and `"tap deck schema",` to `expectedCommands`, right after `"tap component new",`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestDeckSchema|TestCommandTree' -short`
Expected: FAIL, `unknown command "deck"`.

- [ ] **Step 3: Write `internal/cli/deck_schema.go`**

```go
package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/config"
)

var deckSchemaJSON bool

// deckCmd groups the commands that describe what a deck file can hold.
var deckCmd = &cobra.Command{
	Use:   "deck",
	Short: "Describe what a deck file can hold",
}

var deckSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "List every frontmatter key tap understands",
	Long: `List every frontmatter key tap understands, with its type, its
default, its allowed values, and what it does. Nested keys are shown with
dots, and "<name>" stands for a name the deck picks, such as a driver's.

Editors and tools can build a form or completions from --json.

Examples:
  tap deck schema
  tap deck schema --json`,
	Args: cobra.NoArgs,
	RunE: runDeckSchema,
}

func init() {
	rootCmd.AddCommand(deckCmd)
	deckCmd.AddCommand(deckSchemaCmd)
	deckSchemaCmd.Flags().BoolVar(&deckSchemaJSON, "json", false, "print the schema as JSON")
}

func runDeckSchema(cmd *cobra.Command, args []string) error {
	keys := config.Schema()
	if deckSchemaJSON {
		return printJSONOK(cmd.OutOrStdout(), struct {
			Keys []config.SchemaKey `json:"keys"`
		}{Keys: keys})
	}

	table := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "KEY\tTYPE\tDEFAULT\tVALUES\tDESCRIPTION")
	writeSchemaRows(table, keys, "")
	return table.Flush()
}

// writeSchemaRows writes one table row per key, then the rows of its
// nested keys, with the key path joined by dots.
func writeSchemaRows(w io.Writer, keys []config.SchemaKey, prefix string) {
	for _, key := range keys {
		path := prefix + key.Name
		defaultText := ""
		if key.Default != nil {
			defaultText = fmt.Sprint(key.Default)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", path, key.Type, defaultText, strings.Join(key.Values, ", "), key.Description)
		switch key.Type {
		case "object":
			writeSchemaRows(w, key.Keys, path+".")
		case "map":
			writeSchemaRows(w, key.Keys, path+".<name>.")
		}
	}
}
```

If a cobra `Args` error does not exit 1 under P1's `execute` (it should, as a plain error), wrap it: `Args: func(cmd *cobra.Command, args []string) error { if err := cobra.NoArgs(cmd, args); err != nil { return userError(codeUsage, err) }; return nil }`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestDeckSchema|TestCommandTree|TestEveryCommandFollows' -short -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/deck_schema.go internal/cli/deck_schema_test.go internal/cli/conventions_test.go
git commit -m "feat(cli): add tap deck schema"
```

---

### Task 11: Leave skipped slides out of `tap build`

**Files:**
- Modify: `internal/builder/builder.go` (`Build`)
- Modify: `internal/builder/builder_test.go`
- Create: `internal/cli/build_skip_test.go`

**Interfaces:**
- Consumes: `transformer.WithoutSkippedSlides` (Task 1)

- [ ] **Step 1: Write the failing tests**

Append to `internal/builder/builder_test.go`:

```go
func TestBuild_LeavesOutSkippedSlides(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "dist")
	pres := &parser.Presentation{Slides: []parser.Slide{
		{Index: 0, HTML: "<p>Kept first</p>"},
		{Index: 1, HTML: "<p>Left out of the build</p>", Directives: parser.SlideDirectives{Skip: true}},
		{Index: 2, HTML: "<p>Kept second</p>"},
	}}

	if _, err := NewWithOutput(outputDir).Build(config.DefaultConfig(), pres); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(content)
	if strings.Contains(html, "Left out of the build") {
		t.Error("index.html holds the skipped slide's text")
	}

	startMarker := `<script id="presentation-data" type="application/json">`
	start := strings.Index(html, startMarker)
	if start == -1 {
		t.Fatal("presentation data script tag not found")
	}
	start += len(startMarker)
	end := strings.Index(html[start:], "</script>")
	var data struct {
		Slides []transformer.TransformedSlide `json:"slides"`
	}
	if err := json.Unmarshal([]byte(html[start:start+end]), &data); err != nil {
		t.Fatalf("embedded JSON is invalid: %v", err)
	}
	if len(data.Slides) != 2 || data.Slides[1].Index != 1 || !strings.Contains(data.Slides[1].HTML, "Kept second") {
		t.Errorf("embedded slides = %+v, want two slides indexed 0 and 1", data.Slides)
	}
}
```

`internal/cli/build_skip_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildLeavesOutSkippedSlides(t *testing.T) {
	dir := t.TempDir()
	deck := filepath.Join(dir, "talk.md")
	content := "# Kept\n\n---\n\n<!-- skip: true -->\n\n# Left out of the build\n\n---\n\n# Also kept\n"
	if err := os.WriteFile(deck, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "dist")

	exitCode, stdout, stderr := runTap(t, "build", deck, "-o", output, "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stdout %q, stderr %q", exitCode, stdout, stderr)
	}
	built, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(built), "Left out of the build") {
		t.Error("the built deck holds the skipped slide")
	}
	if !strings.Contains(string(built), "Also kept") {
		t.Error("the built deck lost a slide that is not skipped")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/builder -run TestBuild_LeavesOutSkippedSlides -v && go test ./internal/cli -run TestBuildLeavesOutSkippedSlides -v`
Expected: FAIL, the skipped text is in `index.html`.

- [ ] **Step 3: Filter in `Builder.Build`**

In `internal/builder/builder.go`, replace `transformed := trans.Transform(pres)` with:

```go
	// A slide whose skip directive is true is left out of the built deck
	// entirely, not just hidden, so its content is not published.
	transformed, _ := transformer.WithoutSkippedSlides(trans.Transform(pres))
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/builder ./internal/cli -run 'TestBuild' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/builder/builder.go internal/builder/builder_test.go internal/cli/build_skip_test.go
git commit -m "feat(build): leave skipped slides out of tap build"
```

---

### Task 12: Leave skipped slides out of `tap export`

**Files:**
- Modify: `internal/cli/export_pdf.go` (`runExportPDF`, new `renumberBrokenSlides`)
- Modify: `internal/cli/export_pdf_test.go`
- Modify: `internal/cli/export_images.go` (`runExportImages`, `captureAllSlides`)
- Modify: `internal/cli/export_images_test.go`

**Interfaces:**
- Consumes: `transformer.WithoutSkippedSlides` (Task 1); `prepareDeck` (existing, returns the presentation as its second value)
- Produces:
  - `func renumberBrokenSlides(broken []pdf.BrokenSlide, deckNumbers []int) []pdf.BrokenSlide`
  - `captureAllSlides(ctx context.Context, capture captureFunc, serverURL string, slideNumbers []int, width, height int, theme, outputDir string) ([]string, []brokenSlide, error)`: `slideNumbers` replaces `slideCount`

- [ ] **Step 1: Write the failing tests**

Append to `internal/cli/export_pdf_test.go`:

```go
func TestRenumberBrokenSlidesUsesDeckNumbers(t *testing.T) {
	broken := []pdf.BrokenSlide{{SlideNumber: 1, Message: "a"}, {SlideNumber: 2, Message: "b"}}
	got := renumberBrokenSlides(broken, []int{1, 3})
	if got[0].SlideNumber != 1 || got[1].SlideNumber != 3 || got[1].Message != "b" {
		t.Errorf("renumberBrokenSlides() = %+v, want pages 1 and 2 as deck slides 1 and 3", got)
	}
	if broken[1].SlideNumber != 2 {
		t.Error("renumberBrokenSlides() changed its input")
	}
}
```

Add the `internal/pdf` import to that test file if it is missing.

In `internal/cli/export_images_test.go`, change the three `captureAllSlides` calls: the `4` argument becomes `[]int{1, 2, 3, 4}` (two calls) and the `2` argument becomes `[]int{1, 2}`. Then append:

```go
func TestCaptureAllSlidesCapturesOnlyTheGivenNumbers(t *testing.T) {
	tempDir := t.TempDir()
	var calls []int
	fakeCapture := func(ctx context.Context, serverURL string, options pdf.CaptureOptions, outputPath string) error {
		calls = append(calls, options.SlideNumber)
		return os.WriteFile(outputPath, []byte("fake png"), 0o644)
	}

	written, broken, err := captureAllSlides(context.Background(), fakeCapture, "http://localhost:0", []int{1, 3}, 1920, 1080, "", tempDir)
	if err != nil || len(broken) != 0 {
		t.Fatalf("captureAllSlides() = (%v, %v, %v)", written, broken, err)
	}
	if len(calls) != 2 || calls[0] != 1 || calls[1] != 3 {
		t.Errorf("captured slides %v, want [1 3]", calls)
	}
	want := []string{filepath.Join(tempDir, "slide-001.png"), filepath.Join(tempDir, "slide-003.png")}
	if strings.Join(written, ",") != strings.Join(want, ",") {
		t.Errorf("written = %v, want %v: files keep the deck's slide numbers", written, want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestRenumberBrokenSlides|TestCaptureAllSlides' -short`
Expected: compile failure, `undefined: renumberBrokenSlides` and a type mismatch for the `[]int` argument.

- [ ] **Step 3: `tap export pdf` renders the deck without skipped slides**

In `runExportPDF`, change `srv, _, warnings, componentBuildErrs, componentBuildWarnings, err := prepareDeck(...)` so the second value is kept as `pres`. Right after the deferred `srv.Shutdown`, add:

```go
	// The PDF holds only the slides a talk shows. The temporary server
	// serves a copy of the deck without skipped slides, and deckNumbers
	// turns a page number back into the slide's number in the deck.
	presented, deckNumbers := transformer.WithoutSkippedSlides(pres)
	if len(presented.Slides) == 0 {
		return userError(codeInvalidDeck, errors.New("every slide has skip: true, so there is nothing to export"))
	}
	srv.SetPresentation(presented)
```

Right after the successful `exporter.Export` call (after its error handling, before the broken-slide warnings are printed), add:

```go
	result.BrokenSlides = renumberBrokenSlides(result.BrokenSlides, deckNumbers)
```

Add below `runExportPDF`:

```go
// renumberBrokenSlides turns the page numbers in broken, which count only
// the slides the PDF holds, into the deck's own slide numbers.
// deckNumbers[page-1] is the deck number of that page.
func renumberBrokenSlides(broken []pdf.BrokenSlide, deckNumbers []int) []pdf.BrokenSlide {
	renumbered := make([]pdf.BrokenSlide, len(broken))
	for index, slide := range broken {
		renumbered[index] = slide
		if slide.SlideNumber >= 1 && slide.SlideNumber <= len(deckNumbers) {
			renumbered[index].SlideNumber = deckNumbers[slide.SlideNumber-1]
		}
	}
	return renumbered
}
```

Add `"errors"` and `"github.com/MiniCodeMonkey/tap/internal/transformer"` to the imports if they are missing.

- [ ] **Step 4: `tap export images` leaves skipped slides out of `--all`**

In `export_images.go`, change `captureAllSlides` to take the slide numbers:

```go
// captureAllSlides writes one PNG per slide in slideNumbers, at its final
// state, into outputDir, named by the slide's deck number: slide-001.png
// and so on. The caller passes the deck numbers of every slide that is
// not skipped. It tries every slide even after one fails: a broken slide
// (a capture error, or a rendered error card) is collected and does not
// stop the rest from being written. The only fatal errors are one that
// stops the loop before it can try any slide at all (failing to create
// outputDir), and ctx being cancelled (Ctrl-C or SIGTERM), which is
// checked between slides so the loop stops there instead of starting one
// more capture.
//
// Returns the paths successfully written, in slide order, and the slides
// that failed, also in slide order. Printing is the caller's job, which
// keeps this loop cheap to test against a fake captureFunc.
func captureAllSlides(ctx context.Context, capture captureFunc, serverURL string, slideNumbers []int, width, height int, theme, outputDir string) ([]string, []brokenSlide, error) {
```

In its loop, replace `for i := 0; i < slideCount; i++ {` and `slideNumber := i + 1` with `for _, slideNumber := range slideNumbers {`. The rest of the loop body stays.

In `runExportImages`, in the `--all` branch, before the `captureAllSlides` call:

```go
		_, deckNumbers := transformer.WithoutSkippedSlides(pres)
		if len(deckNumbers) == 0 {
			return userError(codeInvalidDeck, errors.New("every slide has skip: true, so there is nothing to export"))
		}
```

and pass `deckNumbers` where the call passed `total`.

For a single `--slide`, right after `slide = &pres.Slides[screenshotSlide-1]` and its `validateStepAndFragment` check, add:

```go
		if slide.Skip {
			fmt.Fprintf(cmd.ErrOrStderr(), "note: slide %d has skip: true, so presenting and exports leave it out. It is rendered because you asked for it by number.\n", screenshotSlide)
		}
```

`--slide` keeps counting every slide, skipped ones too, so a number means the same slide as in `tap slide list`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cli -short`
Expected: PASS.

Run the browser tests once, since the export paths changed:

Run: `go test ./internal/cli -run 'TestExportImages|TestExportPDF' -v`
Expected: PASS, or SKIP when no browser is installed.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/export_pdf.go internal/cli/export_pdf_test.go internal/cli/export_images.go internal/cli/export_images_test.go
git commit -m "feat(export): leave skipped slides out of tap export"
```

---

### Task 13: Frontend navigation passes over skipped slides

**Files:**
- Modify: `frontend/src/lib/types.ts` (`Slide`)
- Create: `frontend/src/lib/utils/skip.ts`
- Create: `frontend/src/lib/utils/skip.test.ts`
- Modify: `frontend/src/lib/stores/presentation.ts`
- Modify: `frontend/src/lib/stores/presentation.test.ts`

**Interfaces:**
- Consumes: the slide JSON's `skip` field (Task 1)
- Produces:
  - In `lib/utils/skip.ts`: `isSkipped(slide)`, `presentedSlideCount(slides)`, `presentedSlidesThrough(slides, index)`, `presentedSlideNumber(slides, index): number | null`, `nextPresentedIndex(slides, fromIndex): number | null`, `previousPresentedIndex(slides, fromIndex): number | null`, `firstPresentedIndex(slides): number | null`, `lastPresentedIndex(slides): number | null`
  - In `lib/stores/presentation.ts`: `selectPresentedSlideCount`, `selectPresentedSlideNumber`, `goToFirstSlide(): boolean`, `goToLastSlide(): boolean`. `selectTotalSlides` keeps returning every slide.

- [ ] **Step 1: Write the failing tests**

`frontend/src/lib/utils/skip.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import type { Slide } from '$lib/types';
import {
	firstPresentedIndex,
	isSkipped,
	lastPresentedIndex,
	nextPresentedIndex,
	presentedSlideCount,
	presentedSlideNumber,
	presentedSlidesThrough,
	previousPresentedIndex
} from './skip';

function makeSlides(skipped: boolean[]): Slide[] {
	return skipped.map((skip, index) => ({
		index,
		layout: 'default',
		html: '',
		slots: {},
		slotOrder: [],
		fragmentCount: 0,
		steps: 0,
		skip
	}));
}

describe('skip helpers', () => {
	const slides = makeSlides([true, false, true, false, false, true]);

	it('knows which slides are skipped', () => {
		expect(slides.map((slide) => isSkipped(slide))).toEqual([true, false, true, false, false, true]);
		expect(isSkipped(null)).toBe(false);
		expect(isSkipped(makeSlides([false])[0])).toBe(false);
	});

	it('counts the slides a talk shows', () => {
		expect(presentedSlideCount(slides)).toBe(3);
		expect(presentedSlideCount([])).toBe(0);
	});

	it('numbers a slide among the slides a talk shows', () => {
		expect(presentedSlideNumber(slides, 1)).toBe(1);
		expect(presentedSlideNumber(slides, 3)).toBe(2);
		expect(presentedSlideNumber(slides, 4)).toBe(3);
		expect(presentedSlideNumber(slides, 2)).toBeNull();
		expect(presentedSlideNumber(slides, 9)).toBeNull();
	});

	it('counts the presented slides up to a skipped one', () => {
		expect(presentedSlidesThrough(slides, 2)).toBe(1);
		expect(presentedSlidesThrough(slides, 5)).toBe(3);
	});

	it('finds the next and previous presented slides', () => {
		expect(nextPresentedIndex(slides, 1)).toBe(3);
		expect(nextPresentedIndex(slides, 4)).toBeNull();
		expect(previousPresentedIndex(slides, 3)).toBe(1);
		expect(previousPresentedIndex(slides, 1)).toBeNull();
		expect(firstPresentedIndex(slides)).toBe(1);
		expect(lastPresentedIndex(slides)).toBe(4);
		expect(firstPresentedIndex(makeSlides([true, true]))).toBeNull();
	});
});
```

In `frontend/src/lib/stores/presentation.test.ts`:

1. Add `skip?: boolean;` to `SlideOptions`, and `skip: options.skip` to the object `makeSlide` returns.
2. Add `selectPresentedSlideCount`, `selectPresentedSlideNumber`, `goToFirstSlide` and `goToLastSlide` to the import list.
3. Append inside the outer `describe('presentation store', ...)`:

```ts
	describe('skipped slides', () => {
		it('nextSlide passes over a skipped slide', () => {
			loadPresentation(createTestPresentation(4, [{}, { skip: true }, {}, {}]));
			expect(nextSlide()).toBe(true);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(2);
		});

		it('prevSlide passes over a skipped slide and lands on its final state', () => {
			loadPresentation(createTestPresentation(3, [{ steps: 2 }, { skip: true }, {}]));
			goToSlide(2);
			expect(prevSlide()).toBe(true);
			const state = usePresentationStore.getState();
			expect(state.currentSlideIndex).toBe(0);
			expect(state.currentStep).toBe(2);
		});

		it('nextSlide stays put when only skipped slides follow', () => {
			loadPresentation(createTestPresentation(2, [{}, { skip: true }]));
			expect(nextSlide()).toBe(false);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('goToSlide still opens a skipped slide', () => {
			loadPresentation(createTestPresentation(3, [{}, { skip: true }, {}]));
			expect(goToSlide(1)).toBe(true);
			expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
		});

		it('moves on from a skipped slide that was opened directly', () => {
			loadPresentation(createTestPresentation(3, [{}, { skip: true }, {}]));
			goToSlide(1);
			nextSlide();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(2);
		});

		it('starts on the first slide that is not skipped when the URL names none', () => {
			loadPresentation(createTestPresentation(3, [{ skip: true }, {}, {}]));
			expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
		});

		it('starts on a skipped slide that the URL hash names', () => {
			mockWindow.location.hash = '#1';
			loadPresentation(createTestPresentation(3, [{ skip: true }, {}, {}]));
			expect(usePresentationStore.getState().currentSlideIndex).toBe(0);
		});

		it('goToFirstSlide and goToLastSlide pass over skipped slides', () => {
			loadPresentation(createTestPresentation(4, [{ skip: true }, {}, {}, { skip: true }]));
			goToLastSlide();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(2);
			goToFirstSlide();
			expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
		});

		it('counts only the slides that are not skipped', () => {
			loadPresentation(createTestPresentation(3, [{}, { skip: true }, {}]));
			const state = () => usePresentationStore.getState();
			expect(selectPresentedSlideCount(state())).toBe(2);
			expect(selectTotalSlides(state())).toBe(3);
			goToSlide(2);
			expect(selectPresentedSlideNumber(state())).toBe(2);
			goToSlide(1);
			expect(selectPresentedSlideNumber(state())).toBeNull();
		});
	});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npm test -- --run src/lib/utils/skip.test.ts src/lib/stores/presentation.test.ts`
Expected: FAIL, `Failed to resolve import "./skip"`, and the store imports are undefined.

- [ ] **Step 3: Add the type and the helpers**

In `frontend/src/lib/types.ts`, add to `interface Slide`, after `steps`:

```ts
	/**
	 * True when the slide's skip directive is set. Presenting passes over it
	 * and slide counts leave it out, but it keeps its place and its number
	 * in the deck.
	 */
	skip?: boolean;
```

`frontend/src/lib/utils/skip.ts`:

```ts
/**
 * Helpers for slides with `skip: true`. A skipped slide stays in the deck,
 * so slide numbers in URLs and messages keep matching the markdown, but
 * presenting passes over it and slide counts leave it out.
 */

import type { Slide } from '$lib/types';

/** Whether a slide is left out of presenting. */
export function isSkipped(slide: Slide | null | undefined): boolean {
	return slide?.skip === true;
}

/** How many slides a talk shows: every slide that is not skipped. */
export function presentedSlideCount(slides: readonly Slide[]): number {
	return slides.filter((slide) => !isSkipped(slide)).length;
}

/**
 * How many presented slides come at or before index. For a skipped slide
 * this is the number of the presented slide before it.
 */
export function presentedSlidesThrough(slides: readonly Slide[], index: number): number {
	let count = 0;
	for (let position = 0; position <= index && position < slides.length; position++) {
		if (!isSkipped(slides[position])) {
			count++;
		}
	}
	return count;
}

/**
 * The 1-based number the audience sees for the slide at index: its place
 * among the slides that are not skipped. Null for a skipped slide and for
 * an index outside the deck.
 */
export function presentedSlideNumber(slides: readonly Slide[], index: number): number | null {
	const slide = slides[index];
	if (!slide || isSkipped(slide)) {
		return null;
	}
	return presentedSlidesThrough(slides, index);
}

/** The index of the first slide after fromIndex that is not skipped, or null. */
export function nextPresentedIndex(slides: readonly Slide[], fromIndex: number): number | null {
	for (let index = fromIndex + 1; index < slides.length; index++) {
		if (!isSkipped(slides[index])) {
			return index;
		}
	}
	return null;
}

/** The index of the last slide before fromIndex that is not skipped, or null. */
export function previousPresentedIndex(slides: readonly Slide[], fromIndex: number): number | null {
	for (let index = Math.min(fromIndex, slides.length) - 1; index >= 0; index--) {
		if (!isSkipped(slides[index])) {
			return index;
		}
	}
	return null;
}

/** The index of the first slide that is not skipped, or null. */
export function firstPresentedIndex(slides: readonly Slide[]): number | null {
	return nextPresentedIndex(slides, -1);
}

/** The index of the last slide that is not skipped, or null. */
export function lastPresentedIndex(slides: readonly Slide[]): number | null {
	return previousPresentedIndex(slides, slides.length);
}
```

- [ ] **Step 4: Change the store**

In `frontend/src/lib/stores/presentation.ts`:

1. Import the helpers:

```ts
import {
	firstPresentedIndex,
	lastPresentedIndex,
	nextPresentedIndex,
	presentedSlideCount,
	presentedSlideNumber,
	previousPresentedIndex
} from '$lib/utils/skip';
```

2. Add below `selectTotalSlides`:

```ts
/**
 * Number of slides a talk shows, leaving out skipped slides. This is the
 * count the audience and the presenter see.
 */
export const selectPresentedSlideCount = (state: PresentationState): number => {
	return presentedSlideCount(state.presentation?.slides ?? []);
};

/**
 * The current slide's 1-based number among the slides a talk shows, or
 * null when the current slide is skipped.
 */
export const selectPresentedSlideNumber = (state: PresentationState): number | null => {
	return presentedSlideNumber(state.presentation?.slides ?? [], state.currentSlideIndex);
};
```

3. In `nextSlide`, replace the block from `const total = state.presentation?.slides.length ?? 0;` to the `return true; }` that follows it with:

```ts
	const newSlideIndex = nextPresentedIndex(state.presentation?.slides ?? [], state.currentSlideIndex);
	if (newSlideIndex !== null) {
		usePresentationStore.setState({
			currentSlideIndex: newSlideIndex,
			currentStep: 0,
			currentFragmentIndex: -1,
			scrollRevealed: false
		});
		updateURLHash(newSlideIndex);
		return true;
	}
```

In its doc comment, change step 4 to "Next slide - advance to the next slide that is not skipped, resetting step/scroll/fragment state."

4. In `prevSlide`, replace `if (state.currentSlideIndex > 0) {` and the `const newSlideIndex = state.currentSlideIndex - 1;` line under it with:

```ts
	const newSlideIndex = previousPresentedIndex(state.presentation?.slides ?? [], state.currentSlideIndex);
	if (newSlideIndex !== null) {
```

The rest of that block stays. In the doc comment, change step 4 to "Previous slide - go to the previous slide that is not skipped, landing on its final state".

5. Add below `goToSlide`:

```ts
/** Go to the first slide a talk shows, passing over skipped slides. */
export function goToFirstSlide(): boolean {
	const index = firstPresentedIndex(usePresentationStore.getState().presentation?.slides ?? []);
	return index === null ? false : goToSlide(index);
}

/** Go to the last slide a talk shows, passing over skipped slides. */
export function goToLastSlide(): boolean {
	const index = lastPresentedIndex(usePresentationStore.getState().presentation?.slides ?? []);
	return index === null ? false : goToSlide(index);
}
```

Add to `goToSlide`'s doc comment: "A skipped slide opens too: going to a slide directly is how tap dev shows one while writing it."

6. Replace `clampHashSlideIndex` with:

```ts
/**
 * The slide a page load opens on: the one the URL hash names, clamped to
 * the deck, which may be a skipped slide; or, with no hash, the first
 * slide that is not skipped.
 */
function initialSlideIndex(hashIndex: number | null, slides: readonly Slide[]): number {
	if (hashIndex === null) {
		return firstPresentedIndex(slides) ?? 0;
	}
	return slides.length > 0 ? Math.min(hashIndex, slides.length - 1) : hashIndex;
}
```

In `initializeFromURL`, replace `const slideIndex = clampHashSlideIndex(hashIndex, total);` with `const slideIndex = initialSlideIndex(hashIndex, state.presentation?.slides ?? []);`. In `loadPresentation`, replace `const slideIndex = clampHashSlideIndex(hashIndex, data.slides.length);` with `const slideIndex = initialSlideIndex(hashIndex, data.slides);`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd frontend && npm test -- --run src/lib/utils/skip.test.ts src/lib/stores`
Expected: PASS, including the existing store and websocket tests.

Run: `cd frontend && npm run check && npm run lint`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/lib/utils/skip.ts frontend/src/lib/utils/skip.test.ts frontend/src/lib/stores/presentation.ts frontend/src/lib/stores/presentation.test.ts
git commit -m "feat(frontend): pass over skipped slides when navigating"
```

---

### Task 14: Presented numbers and the "Skipped" marker in the audience view

**Files:**
- Modify: `frontend/src/lib/components/Slide.tsx`, `Slide.test.tsx`
- Modify: `frontend/src/lib/components/ProgressBar.tsx`, `ProgressBar.test.tsx`
- Modify: `frontend/src/lib/components/SwipeFeedback.tsx`
- Create: `frontend/src/lib/components/SwipeFeedback.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/lib/utils/keyboard.ts`, `keyboard.test.ts`
- Modify: `frontend/src/lib/styles/ui-components.css`

**Interfaces:**
- Consumes: the helpers and selectors of Task 13; `useConnectionStore` (`presentMode`) from `$lib/stores/websocket`
- Produces: a skipped slide's root has `data-skipped="true"` and `data-slide-numbers="off"`, and, when the marker shows, a child `.slide-skipped-marker` with the text "Skipped". A presented slide's `data-index` is its presented number.

- [ ] **Step 1: Write the failing tests**

In `frontend/src/lib/components/Slide.test.tsx`, import `useConnectionStore` from `$lib/stores/websocket`, and append inside `describe('Slide', ...)`:

```tsx
	describe('skipped slides', () => {
		afterEach(() => {
			resetPresentation();
			useConnectionStore.setState({ presentMode: false });
		});

		const slides = [makeSlide({ index: 0 }), makeSlide({ index: 1, skip: true }), makeSlide({ index: 2 })];

		it('numbers a slide among the slides that are not skipped', () => {
			loadPresentation({ config: {}, slides });
			const { container } = render(
				<Slide slide={slides[2]} active printMode={false} fragmentIndex={-1} step={0} total={2} />
			);
			expect(container.querySelector('.slide')?.getAttribute('data-index')).toBe('2');
		});

		it('marks a skipped slide opened directly and hides its number', () => {
			loadPresentation({ config: {}, slides });
			const { container } = render(
				<Slide slide={slides[1]} active printMode={false} fragmentIndex={-1} step={0} total={2} />
			);
			const root = container.querySelector('.slide');
			expect(root?.getAttribute('data-skipped')).toBe('true');
			expect(root?.getAttribute('data-slide-numbers')).toBe('off');
			expect(container.querySelector('.slide-skipped-marker')?.textContent).toBe('Skipped');
		});

		it('leaves the marker out in print mode', () => {
			loadPresentation({ config: {}, slides });
			const { container } = render(<Slide slide={slides[1]} active printMode fragmentIndex={0} step={0} total={2} />);
			expect(container.querySelector('.slide-skipped-marker')).toBeNull();
		});

		it('leaves the marker out during tap present', () => {
			useConnectionStore.setState({ presentMode: true });
			loadPresentation({ config: {}, slides });
			const { container } = render(
				<Slide slide={slides[1]} active printMode={false} fragmentIndex={-1} step={0} total={2} />
			);
			expect(container.querySelector('.slide-skipped-marker')).toBeNull();
		});
	});
```

In `frontend/src/lib/components/ProgressBar.test.tsx`, give `makePresentation` a second parameter and set `skip`:

```tsx
function makePresentation(slideCount: number, skipped: number[] = []): Presentation {
	return {
		config: {},
		slides: Array.from({ length: slideCount }, (_, index) => ({
			index,
			layout: 'default',
			html: '',
			slots: {},
			slotOrder: [],
			fragmentCount: 0,
			steps: 0,
			skip: skipped.includes(index)
		}))
	};
}
```

and append inside `describe('ProgressBar', ...)`:

```tsx
	it('counts only the slides that are not skipped', () => {
		usePresentationStore.setState({ presentation: makePresentation(5, [1]), currentSlideIndex: 2 });

		const { container } = render(<ProgressBar />);

		const fill = container.querySelector('.progress-bar-fill') as HTMLElement;
		expect(fill.style.width).toBe('50%');
		expect(container.querySelector('[role="progressbar"]')?.getAttribute('aria-valuemax')).toBe('4');
	});
```

Create `frontend/src/lib/components/SwipeFeedback.test.tsx`:

```tsx
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { SwipeFeedback } from './SwipeFeedback';
import { resetPresentation, usePresentationStore } from '$lib/stores/presentation';
import type { Presentation } from '$lib/types';

const presentation: Presentation = {
	config: {},
	slides: [0, 1, 2].map((index) => ({
		index,
		layout: 'default',
		html: '',
		slots: {},
		slotOrder: [],
		fragmentCount: 0,
		steps: 0,
		skip: index === 1
	}))
};

afterEach(() => {
	cleanup();
	resetPresentation();
});

describe('SwipeFeedback', () => {
	it('shows the position among the slides that are not skipped', () => {
		usePresentationStore.setState({ presentation, currentSlideIndex: 2 });
		const { container } = render(<SwipeFeedback direction="next" moved nonce={1} />);
		expect(container.querySelector('.swipe-feedback-position')?.textContent).toBe('2 / 2');
	});
});
```

In `frontend/src/lib/utils/keyboard.test.ts`, add `goToFirstSlide: vi.fn(() => true)` and `goToLastSlide: vi.fn(() => true)` to the mocked `$lib/stores/presentation` module. Change the two navigation key tests:

```ts
		it('should go to the first presented slide on Home', () => {
			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 'Home' });
			const preventDefaultSpy = vi.spyOn(event, 'preventDefault');

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(presentationStore.goToFirstSlide).toHaveBeenCalled();
			expect(preventDefaultSpy).toHaveBeenCalled();
		});

		it('should go to the last presented slide on End', () => {
			cleanup = setupKeyboardNavigation();

			const event = new KeyboardEvent('keydown', { key: 'End' });
			const preventDefaultSpy = vi.spyOn(event, 'preventDefault');

			if (keydownHandler) {
				keydownHandler(event);
			}

			expect(presentationStore.goToLastSlide).toHaveBeenCalled();
			expect(preventDefaultSpy).toHaveBeenCalled();
		});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npm test -- --run src/lib/components/Slide.test.tsx src/lib/components/ProgressBar.test.tsx src/lib/components/SwipeFeedback.test.tsx src/lib/utils/keyboard.test.ts`
Expected: FAIL. `data-index` is `3`, there is no marker, the progress bar shows 60%, the swipe pill says `3 / 3`, and Home calls `goToSlide`.

- [ ] **Step 3: Change `Slide.tsx`**

Add the imports:

```tsx
import { useConnectionStore } from '../stores/websocket';
import { isSkipped, presentedSlideNumber } from '../utils/skip';
```

Next to `slideNumbersOff`, add:

```tsx
	// A skipped slide only shows when someone goes to it directly, which is
	// how tap dev lets its author check it. It carries a marker then,
	// except in print and capture passes, and during tap present, where the
	// audience could see it. It never shows a slide number, and every other
	// slide shows its number among the slides a talk shows.
	const skipped = isSkipped(slide);
	const presentMode = useConnectionStore((state) => state.presentMode);
	const showSkippedMarker = skipped && !printMode && !settleComponents && !presentMode;
	const presentedNumber = usePresentationStore((state) =>
		presentedSlideNumber(state.presentation?.slides ?? [], slide.index)
	);
```

On the root `div`, change the attributes to:

```tsx
				data-index={presentedNumber ?? slide.index + 1}
				data-total={total}
				data-slide-numbers={slideNumbersOff || skipped ? 'off' : undefined}
				data-skipped={skipped ? 'true' : undefined}
```

and add, right after the `slide-badge` line:

```tsx
				{showSkippedMarker ? <div className="slide-skipped-marker">Skipped</div> : null}
```

Update the `total` prop's comment in `SlideProps` to "Number of slides a talk shows, leaving out skipped slides, shown as data-total on the slide root."

- [ ] **Step 4: Change the progress bar, the swipe pill, the viewer and the keyboard**

`ProgressBar.tsx`:

```tsx
import { usePresentationStore, selectPresentedSlideCount } from '$lib/stores/presentation';
import { presentedSlidesThrough } from '$lib/utils/skip';
```

```tsx
	const position = usePresentationStore((state) =>
		presentedSlidesThrough(state.presentation?.slides ?? [], state.currentSlideIndex)
	);
	const total = usePresentationStore(selectPresentedSlideCount);

	if (!show || total <= 0) {
		return null;
	}

	// Skipped slides are left out of both numbers. On the first slide this
	// shows 1/total progress, and on the last one 100%.
	const progressPercent = (position / total) * 100;
```

and use `position` in place of `currentIndex + 1` in the three ARIA attributes. Delete the now unused `currentIndex`.

`SwipeFeedback.tsx`: replace `selectTotalSlides` with `selectPresentedSlideCount` and `selectPresentedSlideNumber` in the import, and:

```tsx
	const presentedNumber = usePresentationStore(selectPresentedSlideNumber);
	const total = usePresentationStore(selectPresentedSlideCount);
```

In the position text, replace `currentIndex + 1` with `presentedNumber ?? currentIndex + 1`. Delete `currentIndex` if nothing else reads it.

`App.tsx`: replace the `selectTotalSlides` import with `selectPresentedSlideCount`, and `const totalSlides = usePresentationStore(selectTotalSlides);` with `const totalSlides = usePresentationStore(selectPresentedSlideCount);`.

`keyboard.ts`: import `goToFirstSlide` and `goToLastSlide`. The Home branch calls `goToFirstSlide();` in place of `goToSlide(0);`. The End branch becomes:

```ts
	// End - go to the last slide a talk shows
	if (key === 'End') {
		event.preventDefault();
		goToLastSlide();
		currentOptions.onNavigate?.();
		return;
	}
```

Change the Home comment to "Home - go to the first slide a talk shows". Remove `goToSlide`, `selectTotalSlides` and `usePresentationStore` from the imports only if nothing else in the file uses them (`npm run lint` reports unused imports).

- [ ] **Step 5: Style the marker**

Append to `frontend/src/lib/styles/ui-components.css`:

```css
/* A skipped slide opened directly in tap dev. See Slide.tsx. */
.slide-skipped-marker {
	position: absolute;
	top: 32px;
	right: 32px;
	z-index: 20;
	padding: 10px 22px;
	border-radius: 999px;
	background: rgba(0, 0, 0, 0.72);
	color: #fff;
	font: 600 28px/1 Inter, system-ui, sans-serif;
	letter-spacing: 0.08em;
	text-transform: uppercase;
	pointer-events: none;
}
```

`.slide` is already `position: relative` (`app.css`).

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd frontend && npm test -- --run`
Expected: PASS, the whole suite.

Run: `cd frontend && npm run check && npm run lint`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): count and number only presented slides, and mark a skipped one"
```

---

### Task 15: Skipped slides in the presenter view and the overview

**Files:**
- Modify: `frontend/src/PresenterApp.tsx`, `PresenterApp.test.tsx`
- Modify: `frontend/src/lib/components/SlideOverview.tsx`, `SlideOverview.test.tsx`
- Modify: `frontend/src/lib/styles/ui-components.css`

**Interfaces:**
- Consumes: `selectPresentedSlideCount`, `selectPresentedSlideNumber`, `goToFirstSlide`, `goToLastSlide` (Task 13); `isSkipped`, `nextPresentedIndex`, `presentedSlideCount`, `presentedSlideNumber` (Task 13)

- [ ] **Step 1: Write the failing tests**

Append inside `describe('PresenterApp', ...)` in `frontend/src/PresenterApp.test.tsx`:

```tsx
	it('previews the next presented slide and counts only presented slides', async () => {
		const withSkip: Presentation = {
			config: { title: 'Test Deck' },
			slides: ['one', 'two', 'three'].map((word, index) => ({
				index,
				layout: 'default',
				html: `<p>Slide ${word}</p>`,
				slots: { default: `<p>Slide ${word}</p>` },
				slotOrder: ['default'],
				fragmentCount: 0,
				steps: 0,
				skip: index === 1
			}))
		};
		// An earlier test's navigation can leave a hash in the URL, which
		// would open a skipped slide directly.
		window.history.replaceState(null, '', '/presenter');
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({ ok: true, statusText: 'OK', json: () => Promise.resolve(withSkip) } as Response)
			)
		);

		const { container } = render(<PresenterApp />);

		await waitFor(() => {
			expect(container.querySelector('.presenter-next-slide-panel')?.textContent).toContain('Slide three');
		});
		expect(container.querySelector('.presenter-slide-counter .total')?.textContent).toBe('2');
		expect(container.querySelector('.presenter-slide-counter .current')?.textContent).toBe('1');

		act(() => {
			usePresentationStore.setState({ currentSlideIndex: 1 });
		});
		expect(container.querySelector('.presenter-slide-counter .current')?.textContent).toBe('Skipped');
	});
```

Append inside `describe('SlideOverview', ...)` in `frontend/src/lib/components/SlideOverview.test.tsx`:

```tsx
	it('dims a skipped slide and numbers the others among the presented slides', () => {
		const slides = makeSlides(3).map((slide, index) => ({ ...slide, skip: index === 1 }));
		const { container } = render(<SlideOverview slides={slides} isOpen />);

		const thumbnails = container.querySelectorAll('.thumbnail');
		expect(thumbnails[1].classList.contains('skipped')).toBe(true);
		expect(thumbnails[0].classList.contains('skipped')).toBe(false);
		expect(thumbnails[1].querySelector('.thumbnail-number')?.textContent).toBe('Skipped');
		expect(thumbnails[2].querySelector('.thumbnail-number')?.textContent).toBe('2');
		expect(thumbnails[1].getAttribute('aria-label')).toBe('Slide 2, skipped');
	});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npm test -- --run src/PresenterApp.test.tsx src/lib/components/SlideOverview.test.tsx`
Expected: FAIL. The next panel shows "Slide two", the total is 3, and the thumbnails have no `skipped` class.

- [ ] **Step 3: Change `PresenterApp.tsx`**

1. Import `selectPresentedSlideCount`, `selectPresentedSlideNumber`, `goToFirstSlide` and `goToLastSlide` from `$lib/stores/presentation`, and `nextPresentedIndex` from `$lib/utils/skip`. Drop `selectTotalSlides` and `goToSlide` from the import if nothing else uses them.
2. Replace `const totalSlides = usePresentationStore(selectTotalSlides);` with:

```tsx
	const totalSlides = usePresentationStore(selectPresentedSlideCount);
	const presentedNumber = usePresentationStore(selectPresentedSlideNumber);
```

3. Replace the `nextSlideData` expression with:

```tsx
	// The next slide the talk shows, passing over skipped slides.
	const nextSlideIndex = presentation ? nextPresentedIndex(presentation.slides, currentSlideIndex) : null;
	const nextSlideData = presentation && nextSlideIndex !== null ? presentation.slides[nextSlideIndex] : null;
```

4. In the key handler, the `Home` case calls `goToFirstSlide();` in place of `goToSlide(0);`, and the `End` case becomes:

```tsx
				case 'End':
					event.preventDefault();
					goToLastSlide();
					broadcastPresentationState();
					break;
```

5. In the header counter, replace `{currentSlideIndex + 1}` with `{presentedNumber ?? 'Skipped'}`.
6. On the Next button, replace the `disabled` expression with `disabled={nextSlideData === null && currentFragmentIndex >= fragmentCount - 1}`.

- [ ] **Step 4: Change `SlideOverview.tsx`**

Import `isSkipped`, `presentedSlideCount` and `presentedSlideNumber` from `$lib/utils/skip`. Before `return (`, add:

```tsx
	const presentedCount = presentedSlideCount(slides);
```

Change the thumbnail button and its number:

```tsx
					{slides.map((slide, index) => {
						const skipped = isSkipped(slide);
						const number = presentedSlideNumber(slides, index);
						return (
							<button
								key={slide.index}
								className={`thumbnail${index === currentIndex ? ' current' : ''}${index === focusedIndex ? ' focused' : ''}${skipped ? ' skipped' : ''}`}
								onClick={() => selectSlide(index)}
								role="option"
								aria-selected={index === currentIndex}
								aria-label={skipped ? `Slide ${index + 1}, skipped` : `Slide ${number}`}
							>
								<LazyThumbnail>
									<SlideCanvas aspectRatio={aspectRatio} theme={theme}>
										<Slide
											slide={slide}
											active={false}
											printMode
											preview
											fragmentIndex={slide.fragmentCount}
											step={slide.steps}
											total={presentedCount}
										/>
									</SlideCanvas>
								</LazyThumbnail>
								<div className="thumbnail-number">{number ?? 'Skipped'}</div>
							</button>
						);
					})}
```

Clicking a skipped thumbnail still goes to it, like any direct navigation.

Append to `frontend/src/lib/styles/ui-components.css`:

```css
/* A skipped slide in the overview grid is dimmed, with "Skipped" for its number. */
.slide-overview .thumbnail.skipped > :not(.thumbnail-number) {
	opacity: 0.45;
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd frontend && npm test -- --run`
Expected: PASS, the whole suite.

Run: `cd frontend && npm run check && npm run lint`
Expected: no errors.

- [ ] **Step 6: Check it by hand**

Run: `make build`, then, in a scratch folder, write a three-slide deck whose second slide starts with `<!-- skip: true -->`, and run `tap dev` on it.
- Arrow keys go from slide 1 to slide 3. The theme's slide numbers read 1 and 2, and the progress bar reaches the end on slide 3.
- Open `http://127.0.0.1:<port>/#2`: slide 2 shows with the "Skipped" pill.
- Press `O`: slide 2 is dimmed and labeled "Skipped".
- Open the presenter view (`S`): the counter reads `1 / 2`, and the next panel shows slide 3.
- Run `tap present` on the same deck: slide 2 never comes up with the arrow keys.

- [ ] **Step 7: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): skip slides in the presenter view and the overview"
```

---

### Task 16: Docs, skill and changelog

**Files:**
- Modify: `README.md`
- Modify: `docs/reference/cli-commands.md`, `docs/reference/slide-directives.md`, `docs/reference/frontmatter-options.md`
- Modify: `docs/guide/code-blocks.md`
- Modify: `skills/tap/rules/cli.md`, `skills/tap/rules/slide-directives.md`
- Modify: `CHANGELOG.md`, `docs/changelog.md`

Do not edit `docs/superpowers/**` or released changelog sections.

- [ ] **Step 1: The `skip` directive**

In `docs/reference/slide-directives.md`, add a `### skip` section after `### steps`, in the same shape as the others:

````markdown
### skip

Leaves the slide out of the talk without deleting it.

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Default | `false` |

```markdown
<!-- skip: true -->

# A slide for the long version of this talk
```

A skipped slide is left out of presenting: the arrow keys pass over it in
the audience view and the presenter view, and slide numbers, the progress
bar and the presenter's counter leave it out. `tap build` and
`tap export pdf` leave it out of their output, and `tap export images --all`
writes no image for it.

It keeps its place and its number in the deck, so `#4` in the URL and
`tap slide list` still count it. In `tap dev`, opening it directly (with the
URL, or from the overview) shows it with a "Skipped" marker, so you can
still write and check it. `tap export images --slide 4` renders it too,
because you asked for it by number.
````

Add a row to the "Quick Reference" table: `` | `skip` | boolean | `false` | Leave the slide out of presenting and exports | ``.

In `skills/tap/rules/slide-directives.md`, add a `### skip` section after `### steps` with the example and two sentences: "A skipped slide stays in the file and keeps its number, but presenting, slide counts, `tap build` and `tap export` leave it out. `tap dev` still shows it, marked, when you open it directly."

- [ ] **Step 2: The new commands**

In `docs/reference/cli-commands.md`, add two sections in the P1 order: `## tap slide list` after `## tap slide add`, and `## tap deck schema` after `## tap component new`. Each has Usage, Flags, Output and Examples, in the style of the file's other sections.

`tap slide list [deck]`:
- Flags: `--json`.
- Output: a table with `#`, `LINES`, `LAYOUT`, `TITLE`, `STEPS`, `FRAGMENTS` and `NOTES` (skipped, live drivers, error count). The deck's own errors print first, each slide's errors after the table.
- `--json` result: `{"ok": true, "slides": [...], "errors": [...]}`. Show one slide from the conference talk example, taken from `internal/slidelist/testdata/golden/conference-talk.json` (slide 4 with its sqlite block). List each field: `number`, `startLine`, `endLine`, `layout`, `title`, `fragments`, `steps`, `skip`, `errors`, `codeBlocks` (`block`, `language`, `driver`, `live`, `line`), and top-level `errors`.
- The line rule, word for word from the `Slide` doc comment.
- Slide numbers count skipped slides.

`tap deck schema`:
- Flags: `--json`.
- Output: one row per key with `KEY`, `TYPE`, `DEFAULT`, `VALUES`, `DESCRIPTION`. Nested keys use dots, and `<name>` stands for a name the deck picks.
- `--json` result: `{"ok": true, "keys": [...]}`, each key with `name`, `type`, `default`, `values`, `description`, `keys`. List the six types.

Add both commands to the same places in `skills/tap/rules/cli.md`, shorter, with one example each and the `--json` shape.

In `README.md`, add both commands to the "Command Reference" table, and add a line about `skip: true` to the "Slide Directives" section.

In `docs/reference/frontmatter-options.md`, add one sentence under "Overview": "`tap deck schema` lists every key on this page with its type, default and allowed values, and `tap deck schema --json` prints them for editors and tools."

- [ ] **Step 3: Driver and highlight on one fence**

In `docs/guide/code-blocks.md`, at the end of "Line Highlighting", add:

````markdown
### With Live Code

A live code block can highlight lines too. Give the driver and the lines as
two groups, in either order:

```markdown
```sql {driver: sqlite, connection: demo} {2-3}
SELECT region, count(*) AS errors
FROM request_log
WHERE status >= 500
```
```
````

Write the outer fence with four backticks so the inner one shows.

- [ ] **Step 4: The changelog**

In `CHANGELOG.md` and `docs/changelog.md`, under `## [Unreleased]`, add these. Match each file's existing style.

Under `### Added`:

```markdown
- **`skip: true` leaves a slide out of the talk** - A skipped slide stays in the file and keeps its number, but the arrow keys pass over it in the audience and presenter views, slide numbers and the progress bar leave it out, and `tap build` and `tap export` leave it out of their output. `tap dev` still shows it with a "Skipped" marker when you open it directly.
- **`tap slide list [deck]`** - Lists each slide with the lines it covers in the file, its layout, title, step and fragment counts, whether it is skipped, its errors, and its code blocks with their drivers. `--json` prints the same for editors and scripts.
- **`tap deck schema`** - Lists every frontmatter key tap understands, with its type, default, allowed values and description. `--json` prints it for editors and tools.
```

Under `### Fixed`:

```markdown
- **A live code block that also highlights lines runs again** - A fence such as `sql {driver: sqlite, connection: demo} {2-3}` lost its driver, because only the last `{...}` group was read, so the block had no Run button. Both groups now count, in either order.
- **A `---` inside a `~~~` fence or an indented fence stays in its slide** - It used to split the slide in two, while the fence still rendered as code.
```

- [ ] **Step 5: Check the docs build**

Run: `cd docs && npm run build`
Expected: the build succeeds with no broken links. Skip this if `docs/node_modules` is missing, and say so in the PR.

Run: `grep -rn $'\xe2\x80\x94' README.md docs/reference docs/guide/code-blocks.md skills/tap/rules CHANGELOG.md docs/changelog.md internal frontend/src/lib/utils/skip.ts`
Expected: no hits in lines this plan added. (Older lines with an em dash are not this plan's to change.)

- [ ] **Step 6: Commit**

```bash
git add README.md docs skills CHANGELOG.md
git commit -m "docs: describe skip, tap slide list and tap deck schema"
```

---

## Final check

- [ ] Run `go test ./...` (not `-short`, so the subprocess and browser tests run).
- [ ] Run `go vet ./...` and `make lint`.
- [ ] Run `cd frontend && npm test -- --run && npm run check && npm run lint`.
- [ ] Run `make build`, then by hand, from the repository root:
  - `./tap slide list examples/conference-talk.md` and `--json`: 9 slides, slide 4 lines 36-46 with `live: sqlite`.
  - `./tap deck schema` and `--json | head -40`.
  - `./tap dev examples/conference-talk.md`: slide 4 has a Run button now, and lines 2 and 3 of the query are highlighted.
  - The skip checks from Task 15, step 6, plus `./tap build` and `./tap export pdf` on that deck: the PDF has 2 pages, and the built `index.html` has no text from the skipped slide.
- [ ] Open the PR against `main`. Its description lists the open questions at the top of this plan that the user has not answered yet.

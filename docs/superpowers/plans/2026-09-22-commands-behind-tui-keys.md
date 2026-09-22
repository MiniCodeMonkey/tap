# Commands behind the TUI keys: implementation plan

## Controller rulings on the open questions (2026-09-22)

These override the plan text where they differ.

- Question 1: `--slide` uses the numbering of P3's `slidelist.Build`, so the app, `slide list` and `--slide` always agree. The branch starts from main after P1 and P3 have merged, and the commands find the slide with `slidelist.Build`, not with the `i` key's own counting. The `i` key moves to the same function.
- Question 2: `tap image add` replaces spaces, parentheses and other characters that need escaping in a markdown link with `-` in the copied file's name, then applies the `-2`, `-3` rule. The link is a plain relative path.
- Question 4: `tap slide add --print --json` with no `--layout` prints every layout: `{"ok": true, "layouts": [{"name": "...", "template": "..."}]}`, in the wizard's order. The app's layout gallery reads this, so Swift hard-codes no layout names.
- The plan's defaults stand for questions 3, 5, 6 and 7.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every TUI key that changes a deck's files a command (`tap theme set`, `tap theme show --image`, `tap image add|generate|regenerate`, `tap slide add --layout|--print`), with one Go implementation that the command and the TUI key both call.

**Architecture:** A new package `internal/deckedit` holds every change to a deck file on disk: setting the theme, appending a slide, inserting markdown into a slide, copying an image in, and placing a generated AI image. The slide templates for all 12 layouts move into `internal/layouts`, next to the slot list they must match. The TUI (`internal/tui`) and the new commands (`internal/cli`) both call these packages, and a parity test drives the real TUI key path and the command against two copies of a deck and checks that the files come out byte for byte the same. Image generation goes through a `deckedit.ImageGenerator` interface, so no test calls the Gemini API.

**Tech Stack:** Go 1.24, cobra, Bubble Tea, Playwright (through `internal/pdf`, for `theme show --image`), goldmark (through `internal/parser`, in the template tests).

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-prerequisites-design.md`, "Part 4: Commands behind the TUI keys". The desktop scenarios that call these commands are in `docs/superpowers/specs/tap-desktop-features/03-slide-operations.feature`, `08-creating-decks.feature` and `09-images-and-components.feature`. The contracts this plan consumes are in `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md` ("Contracts between plans", "From P1"). All three are on the `docs/tap-desktop` branch.

## Global Constraints

- This plan runs after P1 (`2026-09-22-cli-command-tree-and-driver-registry.md`) has merged. It uses P1's names exactly: `resolveDeck(arg string) (string, error)`, `firstArg`, `userError`, `internalError`, `reportedError`, `errInterrupted`, the `code*` constants in `internal/cli/exit.go`, `printJSONOK(w io.Writer, payload any) error`, `runTap(t, args...)`, `resetAllFlags`, `writeDecks`, `withWorkingDirectory`, `slideOutOfRangeError`, `unknownThemeError`, `findTheme`, `prepareDeck`, `stdinIsTerminal`, and `expectedCommands` in `internal/cli/conventions_test.go`.
- Branch: `feat/tui-key-commands`, from `main` after P1 merges.
- Commands this plan adds: `tap theme set <slug> [deck]`, `tap image add <file> [deck]`, `tap image generate [deck]`, `tap image regenerate [deck]`. It adds flags to `tap theme show` (`--image`, `--output/-o`) and `tap slide add` (`--layout`, `--print`, `--json`). Every new command goes in `expectedCommands`, kept sorted.
- Flags follow P1: `--output/-o`, `--json` with no short form, `--slide` with no short form. Numbers a user types are 1-based: `--slide 1` is the first slide.
- `--json` prints one object through `printJSONOK`: `{"ok": true, ...}`. Errors go through P1's `execute`, which prints `{"ok": false, "error": {"code", "message"}}`.
- Exit codes: 0 success, 1 user error, 2 internal error, 130 interrupt.
- One implementation per behavior. A TUI key and its command call the same function in `internal/deckedit` or `internal/layouts`. No file-changing logic stays in `internal/tui` or `internal/cli`.
- No test calls the real Gemini API. Tests replace `deckedit.NewImageGenerator` with a fake.
- A `--json` result struct declares its fields in output order. If the `fieldalignment` linter complains, keep the order and add `//nolint:govet // fieldalignment: field order is the JSON output order` above the struct.
- Spell identifiers out in full (`options`, not `opts`; `index`, not `idx`). Code comments describe the present, with no ticket numbers and no "before the fix" wording.
- No em dashes in docs, changelog, comments or help text. Use `--`, a comma, or a new sentence.

## Open questions

Each item names the choice this plan makes, so the work can go ahead. The reviewer confirms or changes them before execution.

1. **Slide numbering for `--slide`.** The plan counts slides the way the TUI `i` key does today: the non-empty chunks between `---` separators after the frontmatter (`deckedit.SlideBodies`). P3's `slidelist.Build` must count the same way, or the app's slide numbers and `--slide` disagree on decks with empty slides. If P3 lands first and counts differently, switch `slideIndexFromFlag` (Task 6) to P3's numbering.
2. **Image names with spaces or parentheses.** `tap image add` keeps the file name as the spec says, and writes the link destination in angle brackets, `![my diagram](<images/my diagram.png>)`, which CommonMark and goldmark accept. `tap build` finds images through the rendered `src`, where goldmark percent-encodes the space. The plan does not check that `tap build` copies such an image. The alternative is to replace spaces and parentheses with `-` in the copied name.
3. **What `--print` prints.** The slide body only, with no `---` separator in front, because the app inserts it at the cursor and adds separators itself. `tap slide add --layout x` (without `--print`) appends `\n---\n\n` and then the same body.
4. **The layout list for the app's gallery.** Part 4 adds no `tap layout list`. The app can take the 12 names from the unknown-layout error (`tap slide add --layout x --print --json` lists them) or hard-code them. A `layout list --json` command would be a small follow-up.
5. **`theme show --image` render.** A 1280x720 PNG of one title slide: `# <theme name>` and the theme's pitch as the subtitle, in print mode. The cache lives at `os.UserCacheDir()/tap/themes/<tap version>/<slug>.png` (on macOS, `~/Library/Caches/tap/themes/...`). A `dev` build never reads the cache and re-renders every time, because its version does not change when the frontend does.
6. **Alt text from `tap image add`.** The file name without its extension, with `[` and `]` removed.
7. **Two small TUI fixes that come with the extraction.** Regenerating an image whose new bytes hash to the same file name no longer deletes the new file. A regenerate prompt containing `$` is written as typed (the old code ran it through regexp expansion). Both change behavior only in those cases.

## File structure

| File | Status | Responsibility |
|---|---|---|
| `internal/deckedit/slides.go` | new | `SlideSeparator`, `SlideBodies`, `InsertIntoSlide`, `InsertIntoFile`, `AppendSlide`, `SlideRangeError` |
| `internal/deckedit/aiimage.go` | new | `AIImage`, `ParseAIImages`, `AIImageMarkdown`, `InsertAIImage`, `ReplaceAIImage`, `GenerateImageFilename`, `GetExtensionFromContentType` (moved from `internal/tui/imagegen.go`) |
| `internal/deckedit/theme.go` | new | `SetTheme`, `ErrUnknownTheme` |
| `internal/deckedit/images.go` | new | `ImagesDir`, `EnsureImagesDir`, `AddImage`, `ImageMarkdown`, `ErrNotAnImage` |
| `internal/deckedit/generate.go` | new | `ImageGenerator`, `NewImageGenerator`, `Placement`, `PlacedImage`, `SaveImage`, `PlaceGeneratedImage` |
| `internal/layouts/templates.go` | new | `Field`, `Template`, `Templates`, `RenderSlide`, `Names` for all 12 layouts |
| `internal/tui/add.go` | modify | The wizard lists all 12 layouts from `layouts.Templates()` and appends through `deckedit.AppendSlide` |
| `internal/tui/imagegen.go` | modify | Delegates slide parsing and placement to `deckedit`; `PlaceImage` |
| `internal/tui/dev.go` | modify | `t` calls `deckedit.SetTheme`; image placement calls `PlaceImage` |
| `internal/cli/exit.go` | modify | New error codes |
| `internal/cli/theme_set.go` | new | `tap theme set` |
| `internal/cli/theme_image.go` | new | `tap theme show --image`, the cache, the renderer |
| `internal/cli/theme.go` | modify | `--image` and `--output/-o` on `theme show` |
| `internal/cli/slide.go` | modify | `--layout`, `--print`, `--json` on `slide add` |
| `internal/cli/image.go` | new | `tap image`, `image add`, `image generate`, `image regenerate`, `slideIndexFromFlag` |
| `internal/cli/edit_helpers_test.go` | new | `writeDeckFile`, `folderSnapshot`, `twoDeckCopies`, key helpers, the fake image generator |
| `internal/cli/tui_parity_test.go` | new | TUI key path versus command, identical files |
| `internal/cli/conventions_test.go` | modify | `expectedCommands` |
| Docs | modify | `docs/reference/cli-commands.md`, `docs/guide/ai-images.md`, `docs/guide/themes.md`, `docs/guide/layouts.md`, `skills/tap/rules/cli.md`, `skills/tap/rules/ai-images.md`, `skills/tap/rules/themes.md`, `CHANGELOG.md`, `docs/changelog.md` |

---

### Task 1: Start the branch, confirm P1, add the error codes

**Files:**
- Modify: `internal/cli/exit.go`
- Modify: `internal/cli/component_test.go`

**Interfaces:**
- Consumes: P1's `runTap`, `componentSnippet(name, extension string, inline bool) string`, the `component new --json` output
- Produces: error code constants `codeUnknownLayout = "unknown_layout"`, `codeFileNotFound = "file_not_found"`, `codeNotAnImage = "not_an_image"`, `codeNoAPIKey = "no_api_key"`, `codeImageGeneration = "image_generation"`, `codeImageNotFound = "image_not_found"`

- [ ] **Step 1: Create the branch and confirm P1 is on main**

```bash
git switch main && git pull
git switch -c feat/tui-key-commands
grep -n "func resolveDeck\b\|func printJSONOK\|var expectedCommands\|Snippet string" internal/cli/*.go
```

Expected: four hits, in `resolve.go`, `jsonout.go`, `conventions_test.go` and `component.go`. If any is missing, P1 has not merged. Stop and say so.

- [ ] **Step 2: Confirm `component new --json` prints the snippet**

Part 4 lists `--json` on `tap component new`. P1 decision 10 already added the `snippet` field. Check that a test covers the JSON output:

```bash
grep -n "\"--json\"" internal/cli/component_test.go
```

If there is a hit whose test checks `snippet`, skip to step 4. Otherwise add this test to `internal/cli/component_test.go` (add `"encoding/json"` to its imports if it is missing):

```go
func TestComponentNewJSONPrintsTheSnippet(t *testing.T) {
	deckDir := t.TempDir()
	exitCode, stdout, stderr := runTap(t, "component", "new", "Counter", deckDir, "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	var output struct {
		OK      bool     `json:"ok"`
		Files   []string `json:"files"`
		Snippet string   `json:"snippet"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !output.OK || len(output.Files) != 1 {
		t.Errorf("output = %+v, want ok and one file", output)
	}
	if output.Snippet != componentSnippet("Counter", "jsx", false) {
		t.Errorf("snippet = %q, want %q", output.Snippet, componentSnippet("Counter", "jsx", false))
	}
}
```

- [ ] **Step 3: Run it**

Run: `go test ./internal/cli -run TestComponentNewJSONPrintsTheSnippet -short -v`
Expected: PASS. P1 already prints the field, so no code changes. If it fails, the failure is in P1's `runComponentNew`; fix it there to print `printJSONOK(cmd.OutOrStdout(), result)`.

- [ ] **Step 4: Add the error codes**

In `internal/cli/exit.go`, add these lines at the end of the error code `const` block, before its closing parenthesis:

```go
	codeUnknownLayout   = "unknown_layout"
	codeFileNotFound    = "file_not_found"
	codeNotAnImage      = "not_an_image"
	codeNoAPIKey        = "no_api_key"
	codeImageGeneration = "image_generation"
	codeImageNotFound   = "image_not_found"
```

Run `gofmt -w internal/cli/exit.go` so the block stays aligned.

- [ ] **Step 5: Build and commit**

Run: `go build ./... && go vet ./internal/cli`
Expected: no output.

```bash
git add internal/cli/exit.go internal/cli/component_test.go
git commit -m "feat(cli): add error codes for the image, layout and theme commands"
```

---

### Task 2: `internal/deckedit` for slides and AI image text

This task moves the slide and AI image text functions out of `internal/tui/imagegen.go` into a package that the commands can call. Behavior does not change. The existing tests move with the functions and prove it.

**Files:**
- Create: `internal/deckedit/slides.go`
- Create: `internal/deckedit/slides_test.go`
- Create: `internal/deckedit/aiimage.go`
- Create: `internal/deckedit/aiimage_test.go` (from tests moved out of `internal/tui/imagegen_test.go`)
- Modify: `internal/tui/imagegen.go`
- Modify: `internal/tui/imagegen_test.go`

**Interfaces:**
- Produces, in `internal/deckedit`:
  - `const SlideSeparator = "\n---\n\n"`
  - `type SlideRangeError struct { Index, Total int }` with `Error() string`
  - `func SlideBodies(content string) []string`: each slide, trimmed, frontmatter and empty chunks skipped. Index 0 is the first slide.
  - `func InsertIntoSlide(content string, slideIndex int, markdown string) (string, error)`: zero-based `slideIndex`; appends `markdown` at the end of that slide.
  - `func InsertIntoFile(deckPath string, slideIndex int, markdown string) error`
  - `func AppendSlide(deckPath, slideMarkdown string) error`: writes `SlideSeparator + slideMarkdown` at the end of the file.
  - `type AIImage struct { Prompt, ImagePath string }`
  - `func ParseAIImages(content string) []AIImage`
  - `func AIImageMarkdown(prompt, imagePath string) string`
  - `func InsertAIImage(content string, slideIndex int, prompt, imagePath string) (string, error)`
  - `func ReplaceAIImage(content, oldPrompt, oldImagePath, newPrompt, newImagePath string) (string, error)`
  - `func GenerateImageFilename(imageData []byte, contentType string) string`
  - `func GetExtensionFromContentType(contentType string) string`
- Changes in `internal/tui`: `type AIImageInfo = deckedit.AIImage` (a type alias, so existing code and tests keep compiling).

- [ ] **Step 1: Write the failing tests for the new slide functions**

`internal/deckedit/slides_test.go`:

```go
package deckedit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlideBodiesSkipsFrontmatterAndEmptySlides(t *testing.T) {
	content := "---\ntitle: Demo\n---\n\n# One\n\n---\n\n---\n\n# Two\n"
	got := SlideBodies(content)
	want := []string{"# One", "# Two"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("SlideBodies() = %q, want %q", got, want)
	}
}

func TestSlideBodiesKeepsSeparatorsInsideCodeBlocks(t *testing.T) {
	content := "# One\n\n```yaml\n---\nkey: value\n```\n\n---\n\n# Two\n"
	if got := SlideBodies(content); len(got) != 2 {
		t.Errorf("SlideBodies() found %d slides, want 2: %q", len(got), got)
	}
}

func TestInsertIntoSlideAddsAtTheEndOfThatSlide(t *testing.T) {
	content := "---\ntitle: Demo\n---\n\n# One\n\nText\n\n---\n\n# Two\n"
	got, err := InsertIntoSlide(content, 0, "![a](images/a.png)")
	if err != nil {
		t.Fatalf("InsertIntoSlide() error = %v", err)
	}
	if !strings.HasPrefix(got, "---\ntitle: Demo\n---\n") {
		t.Errorf("frontmatter lost:\n%s", got)
	}
	if !strings.Contains(got, "Text\n\n![a](images/a.png)\n") {
		t.Errorf("markdown not at the end of slide 1:\n%s", got)
	}
	if strings.Index(got, "![a]") > strings.Index(got, "# Two") {
		t.Errorf("markdown landed after slide 2:\n%s", got)
	}
}

func TestInsertIntoSlideOutOfRange(t *testing.T) {
	_, err := InsertIntoSlide("# One\n", 1, "x")
	var rangeError *SlideRangeError
	if !errors.As(err, &rangeError) {
		t.Fatalf("error = %v, want a *SlideRangeError", err)
	}
	if rangeError.Index != 1 || rangeError.Total != 1 {
		t.Errorf("rangeError = %+v, want Index 1, Total 1", rangeError)
	}
}

func TestInsertIntoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(path, []byte("# One\n\n---\n\n# Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InsertIntoFile(path, 1, "![a](images/a.png)"); err != nil {
		t.Fatalf("InsertIntoFile() error = %v", err)
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "# Two\n\n![a](images/a.png)\n") {
		t.Errorf("file = %q", content)
	}
}

func TestAppendSlideAddsTheSeparatorAndTheSlide(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(path, []byte("# One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendSlide(path, "## Two\n"); err != nil {
		t.Fatalf("AppendSlide() error = %v", err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != "# One\n\n---\n\n## Two\n" {
		t.Errorf("file = %q", content)
	}
}

func TestAppendSlideToAMissingFileFails(t *testing.T) {
	if err := AppendSlide(filepath.Join(t.TempDir(), "missing.md"), "## Two\n"); err == nil {
		t.Error("AppendSlide() on a missing file should fail")
	}
}
```

- [ ] **Step 2: Move the pure-function tests out of `internal/tui/imagegen_test.go`**

Create `internal/deckedit/aiimage_test.go` with only a header:

```go
package deckedit

import (
	"strings"
	"testing"
)
```

Move the tests with this script. It cuts each named top-level test out of the source file and appends it to the target:

```bash
python3 - internal/tui/imagegen_test.go internal/deckedit/aiimage_test.go \
  'TestParseAIImages' \
  'TestGenerateImageFilename\w*' \
  'TestGetExtensionFromContentType' \
  'TestInsertImageIntoSlide_\w+' \
  'TestReplaceImageInContent' <<'EOF'
import re
import sys

source_path, target_path, patterns = sys.argv[1], sys.argv[2], sys.argv[3:]
with open(source_path) as source_file:
    text = source_file.read()

# A top-level test ends at a closing brace in column 0 that is followed by
# the next top-level declaration, a comment, or the end of the file.
moved = []
for pattern in patterns:
    expression = re.compile(
        r"\nfunc (" + pattern + r")\(t \*testing\.T\) \{\n.*?\n\}\n"
        r"(?=\n*(?:func |// |type |var |const )|\n*\Z)",
        re.S,
    )
    matches = list(expression.finditer(text))
    if not matches:
        sys.exit(f"no test matches {pattern}")
    print(pattern, "->", [match.group(1) for match in matches])
    moved.extend(match.group(0) for match in matches)
    for match in reversed(matches):
        text = text[: match.start()] + text[match.end():]

if target_path != "-":
    with open(target_path, "a") as target_file:
        target_file.write("".join(moved))
with open(source_path, "w") as source_file:
    source_file.write(text)
EOF
```

Expected output: one line per pattern listing the tests it moved (`TestGenerateImageFilename`, `TestGenerateImageFilename_DifferentDataDifferentHash`, `TestGenerateImageFilename_SameDataSameHash`, the nine `TestInsertImageIntoSlide_...` tests, and so on).

Rename the calls in the moved tests to the new exported names:

```bash
perl -pi -e 's/\binsertImageIntoSlide\(/InsertAIImage(/g; s/\breplaceImageInContent\(/ReplaceAIImage(/g; s/\bparseAIImages\(/ParseAIImages(/g; s/\bAIImageInfo\b/AIImage/g' internal/deckedit/aiimage_test.go
gofmt -w internal/deckedit/aiimage_test.go internal/tui/imagegen_test.go
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/deckedit/ -short`
Expected: compile failure, `undefined: SlideBodies`, `undefined: InsertAIImage`, and others. If `go vet ./internal/deckedit/` also names a missing standard library import in `aiimage_test.go` (for example `fmt`), add it to that file's import block.

- [ ] **Step 4: Write `internal/deckedit/slides.go`**

```go
// Package deckedit changes deck files on disk. The tap commands and the
// terminal UI keys both call it, so each change to a deck has one
// implementation.
package deckedit

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/MiniCodeMonkey/tap/internal/parser"
)

// SlideSeparator starts a new slide at the end of a deck.
const SlideSeparator = "\n---\n\n"

// frontmatterPattern matches YAML frontmatter at the start of a deck.
var frontmatterPattern = regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)

// SlideRangeError reports a slide index outside the deck. Index is
// zero-based; Total is the number of slides.
type SlideRangeError struct {
	Index int
	Total int
}

func (e *SlideRangeError) Error() string {
	return fmt.Sprintf("invalid slide index: %d (have %d slides)", e.Index, e.Total)
}

// splitDeck splits content into its frontmatter and the chunks between
// slide separators. slideParts holds, for each slide in order, the index
// of its chunk in parts. Empty chunks are not slides.
func splitDeck(content string) (frontmatter string, parts []string, slideParts []int) {
	frontmatter = frontmatterPattern.FindString(content)
	parts = parser.SplitSlidesPreservingCodeBlocks(content[len(frontmatter):])
	for index, part := range parts {
		if strings.TrimSpace(part) != "" {
			slideParts = append(slideParts, index)
		}
	}
	return frontmatter, parts, slideParts
}

// SlideBodies returns the text of each slide in content, in order, with
// surrounding whitespace trimmed. The frontmatter and empty chunks between
// separators are not slides. Index 0 is the first slide; every slideIndex
// in this package counts the same way.
func SlideBodies(content string) []string {
	_, parts, slideParts := splitDeck(content)
	bodies := make([]string, len(slideParts))
	for index, partIndex := range slideParts {
		bodies[index] = strings.TrimSpace(parts[partIndex])
	}
	return bodies
}

// InsertIntoSlide returns content with markdown added at the end of the
// slide at slideIndex, after one blank line.
func InsertIntoSlide(content string, slideIndex int, markdown string) (string, error) {
	frontmatter, parts, slideParts := splitDeck(content)
	if slideIndex < 0 || slideIndex >= len(slideParts) {
		return "", &SlideRangeError{Index: slideIndex, Total: len(slideParts)}
	}

	partIndex := slideParts[slideIndex]
	parts[partIndex] = strings.TrimRight(parts[partIndex], " \t\n") + "\n\n" + markdown + "\n"

	var result strings.Builder
	result.WriteString(frontmatter)
	for index, part := range parts {
		result.WriteString(part)
		if index < len(parts)-1 {
			result.WriteString("---\n")
		}
	}
	return result.String(), nil
}

// InsertIntoFile adds markdown at the end of the slide at slideIndex in
// the deck file.
func InsertIntoFile(deckPath string, slideIndex int, markdown string) error {
	content, err := os.ReadFile(deckPath)
	if err != nil {
		return fmt.Errorf("failed to read markdown file: %w", err)
	}
	updated, err := InsertIntoSlide(string(content), slideIndex, markdown)
	if err != nil {
		return err
	}
	if err := os.WriteFile(deckPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}
	return nil
}

// AppendSlide adds a slide at the end of the deck file: SlideSeparator,
// then slideMarkdown.
func AppendSlide(deckPath, slideMarkdown string) error {
	file, err := os.OpenFile(deckPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	if _, err := file.WriteString(SlideSeparator + slideMarkdown); err != nil {
		_ = file.Close()
		return fmt.Errorf("failed to write to file: %w", err)
	}
	return file.Close()
}
```

- [ ] **Step 5: Write `internal/deckedit/aiimage.go`**

```go
package deckedit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
)

// aiImagePattern matches an ai-prompt comment followed, on the next line,
// by the image it produced. Group 1 is the prompt, group 2 the image path.
var aiImagePattern = regexp.MustCompile(`<!--\s*ai-prompt:\s*(.+?)\s*-->\n[ \t]*!\[\]\(([^)]+)\)`)

// AIImage is an AI-generated image in a deck: the prompt that made it and
// the path the deck links to, relative to the deck's folder.
type AIImage struct {
	Prompt    string
	ImagePath string
}

// ParseAIImages returns the AI-generated images in content, in order.
func ParseAIImages(content string) []AIImage {
	matches := aiImagePattern.FindAllStringSubmatch(content, -1)
	if matches == nil {
		return nil
	}
	images := make([]AIImage, 0, len(matches))
	for _, match := range matches {
		images = append(images, AIImage{Prompt: match[1], ImagePath: match[2]})
	}
	return images
}

// AIImageMarkdown is the text that records an AI-generated image in a
// deck: the prompt comment and the image link.
func AIImageMarkdown(prompt, imagePath string) string {
	return fmt.Sprintf("<!-- ai-prompt: %s -->\n![](%s)", prompt, imagePath)
}

// InsertAIImage returns content with an AI-generated image added at the
// end of the slide at slideIndex.
func InsertAIImage(content string, slideIndex int, prompt, imagePath string) (string, error) {
	return InsertIntoSlide(content, slideIndex, AIImageMarkdown(prompt, imagePath))
}

// ReplaceAIImage returns content with the AI-generated image that has
// oldPrompt and oldImagePath replaced in place by one with newPrompt and
// newImagePath. It fails when content has no such image.
func ReplaceAIImage(content, oldPrompt, oldImagePath, newPrompt, newImagePath string) (string, error) {
	pattern, err := regexp.Compile(fmt.Sprintf(`<!--\s*ai-prompt:\s*%s\s*-->\n[ \t]*!\[\]\(%s\)`,
		regexp.QuoteMeta(oldPrompt), regexp.QuoteMeta(oldImagePath)))
	if err != nil {
		return "", fmt.Errorf("failed to compile replacement pattern: %w", err)
	}
	if !pattern.MatchString(content) {
		return "", fmt.Errorf("could not find the existing image reference to replace")
	}
	return pattern.ReplaceAllLiteralString(content, AIImageMarkdown(newPrompt, newImagePath)), nil
}

// GenerateImageFilename names a generated image by its content:
// "generated-<first 8 hex characters of its SHA-256>.<extension>".
func GenerateImageFilename(imageData []byte, contentType string) string {
	hash := sha256.Sum256(imageData)
	return fmt.Sprintf("generated-%s.%s", hex.EncodeToString(hash[:])[:8], GetExtensionFromContentType(contentType))
}

// GetExtensionFromContentType returns the file extension for an image
// MIME type, and "png" for any type it does not know.
func GetExtensionFromContentType(contentType string) string {
	switch contentType {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	default:
		return "png"
	}
}
```

`ReplaceAllLiteralString` writes the prompt as typed. The TUI used `ReplaceAllString`, which expanded `$1` in a prompt (open question 7).

- [ ] **Step 6: Point `internal/tui/imagegen.go` at `deckedit`**

1. Add the import `"github.com/MiniCodeMonkey/tap/internal/deckedit"`.
2. Replace the `AIImageInfo` struct (and its comment) with an alias:

```go
// AIImageInfo is an AI-generated image on a slide.
type AIImageInfo = deckedit.AIImage
```

3. Delete `frontmatterRe`, `aiImageRe`, `parseAIImages`, `GenerateImageFilename`, `GetExtensionFromContentType`, `replaceImageInContent` and `insertImageIntoSlide`, with their comments. Keep `headingRe` and `extractSlideTitle`.
4. Replace `parseSlides`:

```go
// parseSlides lists the slides in markdown content, with the AI-generated
// images on each.
func parseSlides(content string) []SlideInfo {
	bodies := deckedit.SlideBodies(content)
	slides := make([]SlideInfo, 0, len(bodies))
	for index, body := range bodies {
		aiImages := deckedit.ParseAIImages(body)
		slides = append(slides, SlideInfo{
			Index:        index,
			Title:        extractSlideTitle(body),
			AIImages:     aiImages,
			HasAIImages:  len(aiImages) > 0,
			AIImageCount: len(aiImages),
		})
	}
	return slides
}
```

5. In `SaveGeneratedImage`, change `GenerateImageFilename(` to `deckedit.GenerateImageFilename(`.
6. In `InsertImageIntoMarkdown`, change `insertImageIntoSlide(` to `deckedit.InsertAIImage(`.
7. In `ReplaceImageInMarkdown`, change `replaceImageInContent(` to `deckedit.ReplaceAIImage(`.
8. Remove the imports the build now reports unused (`crypto/sha256`, `encoding/hex`, and `internal/parser`).

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go build ./... && go test ./internal/deckedit/ ./internal/tui/ -short -v 2>&1 | grep -E "^(--- FAIL|FAIL|ok)"`
Expected: `ok` for both packages, no `FAIL` lines. `grep -rn "insertImageIntoSlide\|replaceImageInContent\|parseAIImages" internal/` prints nothing.

- [ ] **Step 8: Commit**

```bash
git add internal/deckedit internal/tui/imagegen.go internal/tui/imagegen_test.go
git commit -m "refactor: move slide and AI image text functions into internal/deckedit"
```

---

### Task 3: `tap theme set`

**Files:**
- Create: `internal/deckedit/theme.go`
- Create: `internal/deckedit/theme_test.go`
- Create: `internal/cli/theme_set.go`
- Create: `internal/cli/theme_set_test.go`
- Create: `internal/cli/edit_helpers_test.go`
- Create: `internal/cli/tui_parity_test.go`
- Modify: `internal/tui/dev.go` (`handleThemePickerKey`)
- Modify: `internal/cli/conventions_test.go`

**Interfaces:**
- Consumes: `config.UpdateThemeInFile(path, newTheme string) error`, `themes.IsValid`, `themes.All`; P1's `resolveDeck`, `userError`, `printJSONOK`, `unknownThemeError`, `runTap`, `withWorkingDirectory`
- Produces:
  - `var deckedit.ErrUnknownTheme error`; `func deckedit.SetTheme(deckPath, slug string) error`
  - `var themeSetCmd`; `type themeSetResult struct { Deck string; Theme string }` (JSON `deck`, `theme`)
  - Test helpers (package `cli`): `writeDeckFile(t, dir, name, content string) string`, `folderSnapshot(t, dir string) map[string]string`, `twoDeckCopies(t, name, content string) (tuiDeck, commandDeck string)`, `runeKey(text string) tea.KeyMsg`, `pressKeys(model tea.Model, keys ...tea.KeyMsg) (tea.Model, tea.Cmd)`, `requireSameFolders(t, tuiDeck, commandDeck string)`

- [ ] **Step 1: Write the failing `deckedit` tests**

`internal/deckedit/theme_test.go`:

```go
package deckedit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSetThemeReplacesTheTheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(path, []byte("---\ntitle: Demo\ntheme: base\n---\n\n# One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetTheme(path, "terminal"); err != nil {
		t.Fatalf("SetTheme() error = %v", err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != "---\ntitle: Demo\ntheme: terminal\n---\n\n# One\n" {
		t.Errorf("file = %q", content)
	}
}

func TestSetThemeAddsFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(path, []byte("# One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetTheme(path, "terminal"); err != nil {
		t.Fatalf("SetTheme() error = %v", err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != "---\ntheme: terminal\n---\n# One\n" {
		t.Errorf("file = %q", content)
	}
}

func TestSetThemeRejectsAnUnknownThemeAndLeavesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.md")
	original := "---\ntheme: base\n---\n\n# One\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetTheme(path, "no-such-theme"); !errors.Is(err, ErrUnknownTheme) {
		t.Errorf("SetTheme() error = %v, want ErrUnknownTheme", err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != original {
		t.Errorf("file changed to %q", content)
	}
}
```

- [ ] **Step 2: Write the shared CLI test helpers**

`internal/cli/edit_helpers_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// writeDeckFile writes content to dir/name and returns the path.
func writeDeckFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// folderSnapshot maps every file under dir, by its slash-separated path
// relative to dir, to its content.
func folderSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

// twoDeckCopies writes the same deck into two new folders, one for the TUI
// key path and one for the command, and returns both deck paths.
func twoDeckCopies(t *testing.T, name, content string) (tuiDeck, commandDeck string) {
	t.Helper()
	return writeDeckFile(t, t.TempDir(), name, content), writeDeckFile(t, t.TempDir(), name, content)
}

// requireSameFolders fails the test unless the folders of the two decks
// hold the same files with the same bytes.
func requireSameFolders(t *testing.T, tuiDeck, commandDeck string) {
	t.Helper()
	tuiFiles := folderSnapshot(t, filepath.Dir(tuiDeck))
	commandFiles := folderSnapshot(t, filepath.Dir(commandDeck))
	names := func(files map[string]string) string {
		list := make([]string, 0, len(files))
		for name := range files {
			list = append(list, name)
		}
		sort.Strings(list)
		return strings.Join(list, ", ")
	}
	if names(tuiFiles) != names(commandFiles) {
		t.Fatalf("files differ:\n  TUI:     %s\n  command: %s", names(tuiFiles), names(commandFiles))
	}
	for name, tuiContent := range tuiFiles {
		if commandFiles[name] != tuiContent {
			t.Errorf("%s differs:\n--- TUI ---\n%s\n--- command ---\n%s", name, tuiContent, commandFiles[name])
		}
	}
}

// runeKey is a key press that types text.
func runeKey(text string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)}
}

// pressKeys sends each key to model in order and returns the final model
// and the command the last key returned.
func pressKeys(model tea.Model, keys ...tea.KeyMsg) (tea.Model, tea.Cmd) {
	var command tea.Cmd
	for _, key := range keys {
		model, command = model.Update(key)
	}
	return model, command
}

// repeatKey returns key count times. A negative count gives none.
func repeatKey(key tea.KeyMsg, count int) []tea.KeyMsg {
	keys := make([]tea.KeyMsg, 0, max(count, 0))
	for range max(count, 0) {
		keys = append(keys, key)
	}
	return keys
}
```

- [ ] **Step 3: Write the failing command tests**

`internal/cli/theme_set_test.go`:

```go
package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const themeSetDeck = "---\ntitle: Demo\ntheme: base\n---\n\n# One\n"

func TestThemeSetWritesTheTheme(t *testing.T) {
	deck := writeDeckFile(t, t.TempDir(), "talk.md", themeSetDeck)
	exitCode, stdout, stderr := runTap(t, "theme", "set", "terminal", deck)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if stdout != "Theme set to terminal in "+deck+"\n" {
		t.Errorf("stdout = %q", stdout)
	}
	content, _ := os.ReadFile(deck)
	if !strings.Contains(string(content), "\ntheme: terminal\n") {
		t.Errorf("deck = %q", content)
	}
}

func TestThemeSetJSON(t *testing.T) {
	deck := writeDeckFile(t, t.TempDir(), "talk.md", themeSetDeck)
	exitCode, stdout, _ := runTap(t, "theme", "set", "terminal", deck, "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d", exitCode)
	}
	var output struct {
		OK    bool   `json:"ok"`
		Deck  string `json:"deck"`
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !output.OK || output.Deck != deck || output.Theme != "terminal" {
		t.Errorf("output = %+v", output)
	}
}

func TestThemeSetUnknownThemeListsTheThemes(t *testing.T) {
	deck := writeDeckFile(t, t.TempDir(), "talk.md", themeSetDeck)
	exitCode, stdout, _ := runTap(t, "theme", "set", "no-such-theme", deck, "--json")
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
	if !strings.Contains(stdout, `"code": "unknown_theme"`) || !strings.Contains(stdout, "terminal") {
		t.Errorf("stdout = %q, want unknown_theme and the list of themes", stdout)
	}
	content, _ := os.ReadFile(deck)
	if string(content) != themeSetDeck {
		t.Errorf("deck changed to %q", content)
	}
}

func TestThemeSetUsesTheDeckInTheCurrentFolder(t *testing.T) {
	dir := t.TempDir()
	writeDeckFile(t, dir, "talk.md", themeSetDeck)
	withWorkingDirectory(t, dir, func() {
		if exitCode, _, stderr := runTap(t, "theme", "set", "terminal"); exitCode != exitOK {
			t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
		}
		content, _ := os.ReadFile("talk.md")
		if !strings.Contains(string(content), "\ntheme: terminal\n") {
			t.Errorf("deck = %q", content)
		}
	})
}
```

`internal/cli/tui_parity_test.go`:

```go
package cli

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// themeIndex is the position of slug in the TUI theme picker.
func themeIndex(t *testing.T, slug string) int {
	t.Helper()
	for index, theme := range tui.AvailableThemes {
		if theme.Name == slug {
			return index
		}
	}
	t.Fatalf("theme %q is not in the picker", slug)
	return -1
}

func TestThemeSetMatchesTheThemePickerKey(t *testing.T) {
	target := tui.AvailableThemes[len(tui.AvailableThemes)-1].Name
	if target == "base" {
		target = tui.AvailableThemes[0].Name
	}
	tuiDeck, commandDeck := twoDeckCopies(t, "talk.md", themeSetDeck)

	model := tea.Model(tui.NewDevModel(tui.DevConfig{MarkdownFile: tuiDeck, CurrentTheme: "base"}))
	model, _ = pressKeys(model, runeKey("t"))
	steps := themeIndex(t, target) - themeIndex(t, "base")
	model, _ = pressKeys(model, repeatKey(tea.KeyMsg{Type: tea.KeyDown}, steps)...)
	model, _ = pressKeys(model, repeatKey(tea.KeyMsg{Type: tea.KeyUp}, -steps)...)
	pressKeys(model, tea.KeyMsg{Type: tea.KeyEnter})

	if exitCode, _, stderr := runTap(t, "theme", "set", target, commandDeck); exitCode != exitOK {
		t.Fatalf("tap theme set exited %d: %s", exitCode, stderr)
	}
	requireSameFolders(t, tuiDeck, commandDeck)
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./internal/deckedit/ ./internal/cli/ -run 'TestSetTheme|TestThemeSet' -short`
Expected: compile failure, `undefined: SetTheme`, and `tap theme set` is an unknown command.

- [ ] **Step 5: Write `internal/deckedit/theme.go`**

```go
package deckedit

import (
	"errors"
	"fmt"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// ErrUnknownTheme means a slug names no built-in theme.
var ErrUnknownTheme = errors.New("unknown theme")

// SetTheme writes "theme: <slug>" into the deck's frontmatter, adding the
// frontmatter when the deck has none. An unknown slug leaves the file
// alone and returns an error that wraps ErrUnknownTheme.
func SetTheme(deckPath, slug string) error {
	if !themes.IsValid(slug) {
		return fmt.Errorf("%w %q", ErrUnknownTheme, slug)
	}
	return config.UpdateThemeInFile(deckPath, slug)
}
```

- [ ] **Step 6: Write `internal/cli/theme_set.go`**

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/deckedit"
	"github.com/MiniCodeMonkey/tap/internal/themes"
)

var themeSetJSON bool

// themeSetCmd writes a theme into a deck's frontmatter.
var themeSetCmd = &cobra.Command{
	Use:   "set <slug> [deck]",
	Short: "Set a deck's theme",
	Long: `Set the theme: key in a deck's frontmatter, as the t key in tap dev
does. A deck with no frontmatter gets one.

[deck] is a deck file or folder. With no deck, tap uses the deck in the
current folder. See tap theme list for the slugs.

Examples:
  tap theme set terminal
  tap theme set midnight talk.md
  tap theme set midnight talk.md --json`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runThemeSet,
}

func init() {
	themeCmd.AddCommand(themeSetCmd)
	themeSetCmd.Flags().BoolVar(&themeSetJSON, "json", false, "print the result as JSON")
}

// themeSetResult is the --json result of tap theme set.
type themeSetResult struct {
	Deck  string `json:"deck"`
	Theme string `json:"theme"`
}

func runThemeSet(cmd *cobra.Command, args []string) error {
	slug := args[0]
	if !themes.IsValid(slug) {
		return userError(codeUnknownTheme, unknownThemeError(slug))
	}
	var deckArg string
	if len(args) > 1 {
		deckArg = args[1]
	}
	deck, err := resolveDeck(deckArg)
	if err != nil {
		return err
	}
	if err := deckedit.SetTheme(deck, slug); err != nil {
		return userError(codeInvalidDeck, fmt.Errorf("cannot set the theme of %s: %w", deck, err))
	}
	if themeSetJSON {
		return printJSONOK(cmd.OutOrStdout(), themeSetResult{Deck: deck, Theme: slug})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Theme set to %s in %s\n", slug, deck)
	return nil
}
```

- [ ] **Step 7: Make the `t` key call `deckedit.SetTheme`**

In `internal/tui/dev.go`, `handleThemePickerKey`, the `"enter"` case: change `config.UpdateThemeInFile(absPath, selectedTheme)` to `deckedit.SetTheme(absPath, selectedTheme)`. Add the import `"github.com/MiniCodeMonkey/tap/internal/deckedit"`, and remove the `internal/config` import if the build reports it unused (it was the only use).

- [ ] **Step 8: Add the command to `expectedCommands`**

In `internal/cli/conventions_test.go`, add `"tap theme set",` between `"tap theme list",` and `"tap theme show",`. The list stays sorted.

- [ ] **Step 9: Run the tests to verify they pass**

Run: `go test ./internal/deckedit/ ./internal/cli/ ./internal/tui/ -run 'TestSetTheme|TestThemeSet|TestCommandTree|TestEveryCommandFollows|TestDevModel' -short -v 2>&1 | grep -E "^(--- FAIL|FAIL|ok)"`
Expected: `ok` for all three packages.

- [ ] **Step 10: Commit**

```bash
git add internal/deckedit/theme.go internal/deckedit/theme_test.go internal/cli/theme_set.go internal/cli/theme_set_test.go internal/cli/edit_helpers_test.go internal/cli/tui_parity_test.go internal/cli/conventions_test.go internal/tui/dev.go
git commit -m "feat(cli): add tap theme set, sharing the theme picker's code"
```

---

### Task 4: Templates for all 12 layouts

The wizard knows 7 layouts. This task moves its templates into `internal/layouts`, adds the other 5, and builds the wizard's list from them. The 7 existing templates keep their exact output.

**Files:**
- Create: `internal/layouts/templates.go`
- Create: `internal/layouts/templates_test.go`
- Modify: `internal/tui/add.go`
- Modify: `internal/tui/add_test.go`

**Interfaces:**
- Consumes: `deckedit.AppendSlide`, `deckedit.SlideSeparator` (Task 2)
- Produces, in `internal/layouts`:
  - `type Field struct { Name, Placeholder string; Multiline bool }`
  - `type Template struct { Name, Description string; Fields []Field }`
  - `func Templates() []Template`: all 12, in wizard order: title, section, default, two-column, code-focus, quote, big-stat, three-column, sidebar, split-media, cover, blank
  - `func RenderSlide(name string, values []string) (string, error)`: the slide body with no separator, ending in `\n`. `values[i]` fills `Fields[i]`; an empty or missing value takes the field's default.
  - `func Names() []string`: every layout name, sorted
- In `internal/tui`: `AvailableLayouts` holds all 12; `GenerateSlideMarkdown(layoutName string, values []string) string` returns `deckedit.SlideSeparator + body`.

- [ ] **Step 1: Write the failing tests**

`internal/layouts/templates_test.go`:

```go
package layouts

import (
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

func TestEveryLayoutHasATemplate(t *testing.T) {
	templates := Templates()
	if len(templates) != len(Names()) {
		t.Fatalf("%d templates for %d layouts", len(templates), len(Names()))
	}
	seen := map[string]bool{}
	for _, template := range templates {
		seen[template.Name] = true
		if template.Description == "" || len(template.Fields) == 0 {
			t.Errorf("template %q needs a description and at least one field", template.Name)
		}
	}
	for _, name := range Names() {
		if !seen[name] {
			t.Errorf("layout %q has no template", name)
		}
	}
}

func TestRenderSlideDefaults(t *testing.T) {
	want := map[string]string{
		"title":        "# Title\n",
		"section":      "## Section\n",
		"default":      "## Header\n\n- Point one\n- Point two\n",
		"two-column":   "::left\n\nLeft content\n\n::right\n\nRight content\n",
		"code-focus":   "<!--\nlayout: code-focus\n-->\n\n```\n// Your code here\n```\n",
		"quote":        "<!--\nlayout: quote\n-->\n\n> \"Your quote here\"\n",
		"big-stat":     "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n",
		"three-column": "::left\n\nLeft content\n\n::center\n\nCenter content\n\n::right\n\nRight content\n",
		"sidebar":      "<!--\nlayout: sidebar\n-->\n\n## Header\n\nMain content\n\n::sidebar\n\n- Note one\n- Note two\n",
		"split-media":  "<!--\nlayout: split-media\n-->\n\n## Header\n\nDescribe the image\n\n::media\n\nImage or video\n",
		"cover":        "<!--\nlayout: cover\n-->\n\n# Title\n",
		"blank":        "<!--\nlayout: blank\n-->\n\nContent\n",
	}
	for name, expected := range want {
		got, err := RenderSlide(name, nil)
		if err != nil || got != expected {
			t.Errorf("RenderSlide(%q, nil) = (%q, %v), want %q", name, got, err, expected)
		}
	}
}

func TestRenderSlideUsesTheValues(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"title", []string{"Hello", "World"}, "# Hello\n\nWorld\n"},
		{"two-column", []string{"Compare", "A", "B"}, "## Compare\n\n::left\n\nA\n\n::right\n\nB\n"},
		{"quote", []string{"Less is more", "Mies"}, "<!--\nlayout: quote\n-->\n\n> \"Less is more\"\n>\n> -- Mies\n"},
		{"cover", []string{"Launch", "images/hero.jpg"}, "<!--\nlayout: cover\nbackground: images/hero.jpg\n-->\n\n# Launch\n"},
		{"split-media", []string{"Dashboard", "New look", "images/shot.png"}, "<!--\nlayout: split-media\n-->\n\n## Dashboard\n\nNew look\n\n::media\n\n![](images/shot.png)\n"},
	}
	for _, tt := range tests {
		got, err := RenderSlide(tt.name, tt.values)
		if err != nil || got != tt.want {
			t.Errorf("RenderSlide(%q, %q) = (%q, %v), want %q", tt.name, tt.values, got, err, tt.want)
		}
	}
}

func TestEveryTemplateRendersAsItsOwnLayout(t *testing.T) {
	for _, template := range Templates() {
		t.Run(template.Name, func(t *testing.T) {
			body, err := RenderSlide(template.Name, nil)
			if err != nil {
				t.Fatal(err)
			}
			presentation, err := parser.New().Parse([]byte("---\ntitle: Templates\n---\n\n" + body))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			transformed := transformer.New(config.DefaultConfig()).Transform(presentation)
			if len(transformed.Slides) != 1 {
				t.Fatalf("%d slides, want 1", len(transformed.Slides))
			}
			if got := transformed.Slides[0].Layout; got != template.Name {
				t.Errorf("layout = %q, want %q", got, template.Name)
			}
			if warnings := Validate(transformed); len(warnings) > 0 {
				t.Errorf("warnings: %+v", warnings)
			}
		})
	}
}

func TestRenderSlideUnknownLayoutListsTheLayouts(t *testing.T) {
	_, err := RenderSlide("bogus", nil)
	if err == nil || !strings.Contains(err.Error(), "big-stat") || !strings.Contains(err.Error(), "split-media") {
		t.Errorf("RenderSlide(bogus) error = %v, want it to list the layouts", err)
	}
}
```

Move the two helper tests from the wizard to the new package, and delete the two `appendToFile` tests (Task 2's `TestAppendSlide...` tests replace them):

```bash
cat > internal/layouts/templates_helpers_test.go <<'EOF'
package layouts

import "testing"
EOF
python3 - internal/tui/add_test.go internal/layouts/templates_helpers_test.go 'TestGetValueOrDefault' 'TestFormatContent' <<'EOF'
import re
import sys

source_path, target_path, patterns = sys.argv[1], sys.argv[2], sys.argv[3:]
with open(source_path) as source_file:
    text = source_file.read()

# A top-level test ends at a closing brace in column 0 that is followed by
# the next top-level declaration, a comment, or the end of the file.
moved = []
for pattern in patterns:
    expression = re.compile(
        r"\nfunc (" + pattern + r")\(t \*testing\.T\) \{\n.*?\n\}\n"
        r"(?=\n*(?:func |// |type |var |const )|\n*\Z)",
        re.S,
    )
    matches = list(expression.finditer(text))
    if not matches:
        sys.exit(f"no test matches {pattern}")
    print(pattern, "->", [match.group(1) for match in matches])
    moved.extend(match.group(0) for match in matches)
    for match in reversed(matches):
        text = text[: match.start()] + text[match.end():]

if target_path != "-":
    with open(target_path, "a") as target_file:
        target_file.write("".join(moved))
with open(source_path, "w") as source_file:
    source_file.write(text)
EOF
```

Then run the same script again with `-` as the target, to delete the `appendToFile` tests from `internal/tui/add_test.go`: replace the first line of the command above with `python3 - internal/tui/add_test.go - 'TestAppendToFile\w*' <<'EOF'` and keep the script body. Run `gofmt -w internal/tui/add_test.go internal/layouts/templates_helpers_test.go`. If `go vet ./internal/layouts/` names a missing standard library import in `templates_helpers_test.go` (for example `strings`), add it.

In `internal/tui/add_test.go`, `TestAvailableLayouts`, replace the `requiredLayouts` line with:

```go
	requiredLayouts := []string{"title", "section", "default", "two-column", "three-column", "code-focus", "big-stat", "quote", "cover", "sidebar", "split-media", "blank"}
	if len(AvailableLayouts) != len(requiredLayouts) {
		t.Errorf("AvailableLayouts has %d layouts, want %d", len(AvailableLayouts), len(requiredLayouts))
	}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/layouts/ ./internal/tui/ -short`
Expected: compile failure in `internal/layouts` (`undefined: Templates`, `undefined: getValueOrDefault`), and `TestAvailableLayouts` fails in `internal/tui`.

- [ ] **Step 3: Write `internal/layouts/templates.go`**

```go
package layouts

import (
	"fmt"
	"strings"
)

// Field is one value the slide wizard asks for when it writes a new slide.
type Field struct {
	Name        string
	Placeholder string
	Multiline   bool
}

// Template describes a new slide in one layout: what the wizard asks for,
// and, through RenderSlide, the markdown written from the answers.
type Template struct {
	Name        string
	Description string
	Fields      []Field
}

// templates is every layout's template, in the order the wizard lists
// them.
var templates = []Template{
	{Name: "title", Description: "Title slide with centered heading", Fields: []Field{
		{Name: "Title", Placeholder: "My Title"},
		{Name: "Subtitle", Placeholder: "Optional subtitle"},
	}},
	{Name: "section", Description: "Section header for topic transitions", Fields: []Field{
		{Name: "Section Title", Placeholder: "Section Name"},
	}},
	{Name: "default", Description: "Standard content slide", Fields: []Field{
		{Name: "Header", Placeholder: "Slide Header"},
		{Name: "Content", Placeholder: "Bullet points or paragraphs", Multiline: true},
	}},
	{Name: "two-column", Description: "Side-by-side content columns", Fields: []Field{
		{Name: "Header", Placeholder: "Optional Header"},
		{Name: "Left Column", Placeholder: "Left side content", Multiline: true},
		{Name: "Right Column", Placeholder: "Right side content", Multiline: true},
	}},
	{Name: "code-focus", Description: "Full-width code block", Fields: []Field{
		{Name: "Language", Placeholder: "go, python, javascript..."},
		{Name: "Code", Placeholder: "Your code here", Multiline: true},
	}},
	{Name: "quote", Description: "Styled blockquote with attribution", Fields: []Field{
		{Name: "Quote", Placeholder: "The quote text"},
		{Name: "Author", Placeholder: "Author name"},
	}},
	{Name: "big-stat", Description: "Large number with description", Fields: []Field{
		{Name: "Statistic", Placeholder: "99%"},
		{Name: "Description", Placeholder: "Description of the statistic"},
	}},
	{Name: "three-column", Description: "Three columns side by side", Fields: []Field{
		{Name: "Header", Placeholder: "Optional Header"},
		{Name: "Left Column", Placeholder: "Left column content", Multiline: true},
		{Name: "Center Column", Placeholder: "Center column content", Multiline: true},
		{Name: "Right Column", Placeholder: "Right column content", Multiline: true},
	}},
	{Name: "sidebar", Description: "Main content with a sidebar", Fields: []Field{
		{Name: "Header", Placeholder: "Slide Header"},
		{Name: "Content", Placeholder: "Main content", Multiline: true},
		{Name: "Sidebar", Placeholder: "Notes or references", Multiline: true},
	}},
	{Name: "split-media", Description: "Content beside an image or video", Fields: []Field{
		{Name: "Header", Placeholder: "Slide Header"},
		{Name: "Content", Placeholder: "What the image shows", Multiline: true},
		{Name: "Media", Placeholder: "images/screenshot.png"},
	}},
	{Name: "cover", Description: "Full-screen background image with a title", Fields: []Field{
		{Name: "Title", Placeholder: "Big statement"},
		{Name: "Background", Placeholder: "images/hero.jpg"},
	}},
	{Name: "blank", Description: "No layout styling, full control", Fields: []Field{
		{Name: "Content", Placeholder: "Anything", Multiline: true},
	}},
}

// Templates returns every layout's template, in the order the wizard lists
// them.
func Templates() []Template {
	return append([]Template(nil), templates...)
}

// Names returns every layout name, sorted.
func Names() []string {
	loadRegistry()
	return append([]string(nil), layoutNames...)
}

// RenderSlide returns the markdown of a new slide in the named layout,
// with no slide separator in front of it. values fill the template's
// fields in order; an empty or missing value takes the field's default.
func RenderSlide(name string, values []string) (string, error) {
	var b strings.Builder
	switch name {
	case "title":
		b.WriteString(fmt.Sprintf("# %s\n", getValueOrDefault(values, 0, "Title")))
		if subtitle := getValueOrDefault(values, 1, ""); subtitle != "" {
			b.WriteString(fmt.Sprintf("\n%s\n", subtitle))
		}
	case "section":
		b.WriteString(fmt.Sprintf("## %s\n", getValueOrDefault(values, 0, "Section")))
	case "default":
		b.WriteString(fmt.Sprintf("## %s\n\n", getValueOrDefault(values, 0, "Header")))
		b.WriteString(formatContent(getValueOrDefault(values, 1, "- Point one\n- Point two")))
		b.WriteString("\n")
	case "two-column":
		writeOptionalHeader(&b, getValueOrDefault(values, 0, ""))
		b.WriteString("::left\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 1, "Left content")))
		b.WriteString("\n\n::right\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 2, "Right content")))
		b.WriteString("\n")
	case "code-focus":
		b.WriteString(layoutDirective("code-focus"))
		language := getValueOrDefault(values, 0, "")
		code := getValueOrDefault(values, 1, "// Your code here")
		b.WriteString(fmt.Sprintf("```%s\n%s\n```\n", language, code))
	case "quote":
		b.WriteString(layoutDirective("quote"))
		b.WriteString(fmt.Sprintf("> %q\n", getValueOrDefault(values, 0, "Your quote here")))
		if author := getValueOrDefault(values, 1, ""); author != "" {
			b.WriteString(fmt.Sprintf(">\n> -- %s\n", author))
		}
	case "big-stat":
		b.WriteString(layoutDirective("big-stat"))
		b.WriteString(fmt.Sprintf("# %s\n\n%s\n", getValueOrDefault(values, 0, "100%"), getValueOrDefault(values, 1, "Description")))
	case "three-column":
		writeOptionalHeader(&b, getValueOrDefault(values, 0, ""))
		b.WriteString("::left\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 1, "Left content")))
		b.WriteString("\n\n::center\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 2, "Center content")))
		b.WriteString("\n\n::right\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 3, "Right content")))
		b.WriteString("\n")
	case "sidebar":
		b.WriteString(layoutDirective("sidebar"))
		b.WriteString(fmt.Sprintf("## %s\n\n", getValueOrDefault(values, 0, "Header")))
		b.WriteString(formatContent(getValueOrDefault(values, 1, "Main content")))
		b.WriteString("\n\n::sidebar\n\n")
		b.WriteString(formatContent(getValueOrDefault(values, 2, "- Note one\n- Note two")))
		b.WriteString("\n")
	case "split-media":
		b.WriteString(layoutDirective("split-media"))
		b.WriteString(fmt.Sprintf("## %s\n\n", getValueOrDefault(values, 0, "Header")))
		b.WriteString(formatContent(getValueOrDefault(values, 1, "Describe the image")))
		b.WriteString("\n\n::media\n\n")
		if media := getValueOrDefault(values, 2, ""); media != "" {
			b.WriteString(fmt.Sprintf("![](%s)\n", media))
		} else {
			b.WriteString("Image or video\n")
		}
	case "cover":
		if background := getValueOrDefault(values, 1, ""); background != "" {
			b.WriteString(fmt.Sprintf("<!--\nlayout: cover\nbackground: %s\n-->\n\n", background))
		} else {
			b.WriteString(layoutDirective("cover"))
		}
		b.WriteString(fmt.Sprintf("# %s\n", getValueOrDefault(values, 0, "Title")))
	case "blank":
		b.WriteString(layoutDirective("blank"))
		b.WriteString(formatContent(getValueOrDefault(values, 0, "Content")))
		b.WriteString("\n")
	default:
		return "", fmt.Errorf("unknown layout %q (valid layouts: %s)", name, strings.Join(Names(), ", "))
	}
	return b.String(), nil
}

// layoutDirective is the comment that sets a slide's layout, followed by a
// blank line.
func layoutDirective(name string) string {
	return fmt.Sprintf("<!--\nlayout: %s\n-->\n\n", name)
}

// writeOptionalHeader writes "## header" and a blank line, or nothing when
// header is empty.
func writeOptionalHeader(b *strings.Builder, header string) {
	if header != "" {
		b.WriteString(fmt.Sprintf("## %s\n\n", header))
	}
}

// getValueOrDefault returns values[index], or defaultValue when it is
// missing or empty.
func getValueOrDefault(values []string, index int, defaultValue string) string {
	if index < len(values) && values[index] != "" {
		return values[index]
	}
	return defaultValue
}

// formatContent trims each line of content and drops lines that are only
// whitespace, keeping the line breaks.
func formatContent(content string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	for index, line := range lines {
		result.WriteString(strings.TrimSpace(line))
		if index < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
}
```

`formatContent` gives the same output as the wizard's version: a line that trims to empty wrote nothing there either. The moved `TestFormatContent` checks it.

- [ ] **Step 4: Build the wizard from the templates**

In `internal/tui/add.go`:

1. Replace the whole `AvailableLayouts` literal with:

```go
// AvailableLayouts lists the layouts the wizard offers, built from the
// templates in internal/layouts.
var AvailableLayouts = buildAvailableLayouts()

func buildAvailableLayouts() []Layout {
	templates := layouts.Templates()
	result := make([]Layout, len(templates))
	for index, template := range templates {
		fields := make([]LayoutField, len(template.Fields))
		for fieldIndex, field := range template.Fields {
			fields[fieldIndex] = LayoutField{Name: field.Name, Placeholder: field.Placeholder, Multiline: field.Multiline}
		}
		result[index] = Layout{
			Name:        template.Name,
			Description: template.Description,
			ASCII:       layoutPreviews[template.Name],
			Fields:      fields,
		}
	}
	return result
}

// layoutPreviews is the ASCII sketch the wizard shows for each layout.
var layoutPreviews = map[string]string{
	"title": `
┌─────────────────────┐
│                     │
│      # Title        │
│      subtitle       │
│                     │
└─────────────────────┘`,
	"section": `
┌─────────────────────┐
│                     │
│                     │
│    ## Section       │
│                     │
│                     │
└─────────────────────┘`,
	"default": `
┌─────────────────────┐
│ ## Header           │
│                     │
│ - Point one         │
│ - Point two         │
│ - Point three       │
└─────────────────────┘`,
	"two-column": `
┌─────────────────────┐
│ ## Header           │
│          ┃          │
│  Left    ┃   Right  │
│  column  ┃   column │
│          ┃          │
└─────────────────────┘`,
	"code-focus": `
┌─────────────────────┐
│ ┌─────────────────┐ │
│ │ func main() {   │ │
│ │   // code here  │ │
│ │ }               │ │
│ └─────────────────┘ │
└─────────────────────┘`,
	"quote": `
┌─────────────────────┐
│                     │
│  "Quote text..."    │
│                     │
│        -- Author    │
│                     │
└─────────────────────┘`,
	"big-stat": `
┌─────────────────────┐
│                     │
│        99%          │
│                     │
│    of developers    │
│    love this tool   │
└─────────────────────┘`,
	"three-column": `
┌─────────────────────┐
│ ## Header           │
│      ┃       ┃      │
│ Left ┃Center ┃Right │
│      ┃       ┃      │
│      ┃       ┃      │
└─────────────────────┘`,
	"sidebar": `
┌─────────────────────┐
│ ## Header    ┃ Side │
│              ┃      │
│ Main content ┃ - a  │
│              ┃ - b  │
│              ┃      │
└─────────────────────┘`,
	"split-media": `
┌─────────────────────┐
│ ## Header ┃ ┌─────┐ │
│           ┃ │     │ │
│ Text      ┃ │ img │ │
│           ┃ │     │ │
│           ┃ └─────┘ │
└─────────────────────┘`,
	"cover": `
┌─────────────────────┐
│░░░░░░░░░░░░░░░░░░░░░│
│░░░░░░░░░░░░░░░░░░░░░│
│░░░░   # Title   ░░░░│
│░░░░░░░░░░░░░░░░░░░░░│
│░░░░░░░░░░░░░░░░░░░░░│
└─────────────────────┘`,
	"blank": `
┌─────────────────────┐
│                     │
│                     │
│      (empty)        │
│                     │
│                     │
└─────────────────────┘`,
}
```

2. Replace `finalize`, `generateMarkdown`, `GenerateSlideMarkdown`, and delete `getValueOrDefault`, `formatContent` and `appendToFile`:

```go
func (m AddModel) finalize() (tea.Model, tea.Cmd) {
	if m.filePath != "" {
		if err := deckedit.AppendSlide(m.filePath, m.slideBody()); err != nil {
			m.err = err
			m.step = addStepDone
			return m, tea.Quit
		}
	}

	m.done = true
	m.step = addStepDone
	return m, tea.Quit
}

// fieldValues returns the trimmed text of each field, in order.
func (m AddModel) fieldValues() []string {
	values := make([]string, len(m.textInputs))
	for index, input := range m.textInputs {
		values[index] = strings.TrimSpace(input.Value())
	}
	return values
}

// slideBody is the new slide's markdown, without the separator.
func (m AddModel) slideBody() string {
	body, _ := layouts.RenderSlide(AvailableLayouts[m.layoutIndex].Name, m.fieldValues())
	return body
}

func (m AddModel) generateMarkdown() string {
	return deckedit.SlideSeparator + m.slideBody()
}

// GenerateSlideMarkdown returns the text the wizard appends for a slide in
// layoutName: the slide separator, then the layout's template filled with
// values. An unknown layout gives the separator alone.
func GenerateSlideMarkdown(layoutName string, values []string) string {
	body, _ := layouts.RenderSlide(layoutName, values)
	return deckedit.SlideSeparator + body
}
```

3. Imports of `add.go`: add `"github.com/MiniCodeMonkey/tap/internal/deckedit"` and `"github.com/MiniCodeMonkey/tap/internal/layouts"`; remove `"os"`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/layouts/ ./internal/tui/ -short -v 2>&1 | grep -E "^(--- FAIL|FAIL|ok)"`
Expected: `ok` for both. The existing `TestGenerateSlideMarkdown_*` tests pass unchanged, which shows the 7 old templates write the same text.

If `TestEveryTemplateRendersAsItsOwnLayout` fails for `title`, `section`, `default`, `two-column` or `three-column` (the five that rely on automatic layout detection), that template's content is detected as another layout. Add `b.WriteString(layoutDirective("<name>"))` as the first line of that case, update its expected string in `TestRenderSlideDefaults` and `TestRenderSlideUsesTheValues`, and update the matching `TestGenerateSlideMarkdown_*` test in `internal/tui/add_test.go`.

- [ ] **Step 6: Commit**

```bash
git add internal/layouts internal/tui/add.go internal/tui/add_test.go
git commit -m "feat(layouts): add slide templates for all 12 layouts, used by the wizard"
```

---

### Task 5: `tap slide add --layout` and `--print`

**Files:**
- Modify: `internal/cli/slide.go`
- Modify: `internal/cli/slide_test.go`
- Modify: `internal/cli/tui_parity_test.go`

**Interfaces:**
- Consumes: `layouts.RenderSlide`, `layouts.Templates` (Task 4); `deckedit.AppendSlide` (Task 2); P1's `slideAddCmd`, `resolveDeck`, `stdinIsTerminal`, `tui.RunAddWizard`
- Produces: flag variables `slideAddLayout string`, `slideAddPrint bool`, `slideAddJSON bool`; `type slideAddResult struct { Deck, Layout, Markdown string }` (JSON `deck` omitted when empty, `layout`, `markdown`)

- [ ] **Step 1: Write the failing tests**

Add to `internal/cli/slide_test.go` (add `"encoding/json"`, `"os"` and `"strings"` to its imports):

```go
func TestSlideAddLayoutAppendsTheTemplate(t *testing.T) {
	deck := writeDeckFile(t, t.TempDir(), "talk.md", "# One\n")
	exitCode, stdout, stderr := runTap(t, "slide", "add", deck, "--layout", "big-stat")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if stdout != "Added a big-stat slide to "+deck+"\n" {
		t.Errorf("stdout = %q", stdout)
	}
	content, _ := os.ReadFile(deck)
	want := "# One\n\n---\n\n<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n"
	if string(content) != want {
		t.Errorf("deck = %q, want %q", content, want)
	}
}

func TestSlideAddLayoutNeedsNoTerminal(t *testing.T) {
	original := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = original })

	deck := writeDeckFile(t, t.TempDir(), "talk.md", "# One\n")
	if exitCode, _, stderr := runTap(t, "slide", "add", deck, "--layout", "quote"); exitCode != exitOK {
		t.Errorf("exit code = %d, stderr %q", exitCode, stderr)
	}
}

func TestSlideAddPrintWritesNothing(t *testing.T) {
	dir := t.TempDir()
	deck := writeDeckFile(t, dir, "talk.md", "# One\n")
	exitCode, stdout, _ := runTap(t, "slide", "add", deck, "--layout", "sidebar", "--print")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d", exitCode)
	}
	if !strings.HasPrefix(stdout, "<!--\nlayout: sidebar\n-->") || strings.HasPrefix(stdout, "\n---") {
		t.Errorf("stdout = %q, want the template with no separator", stdout)
	}
	content, _ := os.ReadFile(deck)
	if string(content) != "# One\n" {
		t.Errorf("--print changed the deck to %q", content)
	}
}

func TestSlideAddPrintNeedsNoDeck(t *testing.T) {
	withWorkingDirectory(t, t.TempDir(), func() {
		exitCode, stdout, stderr := runTap(t, "slide", "add", "--layout", "title", "--print")
		if exitCode != exitOK || stdout != "# Title\n" {
			t.Errorf("(%d, %q, %q), want (0, \"# Title\\n\", \"\")", exitCode, stdout, stderr)
		}
	})
}

func TestSlideAddPrintJSON(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "slide", "add", "--layout", "blank", "--print", "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d", exitCode)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if output["ok"] != true || output["layout"] != "blank" || output["markdown"] != "<!--\nlayout: blank\n-->\n\nContent\n" {
		t.Errorf("output = %v", output)
	}
	if _, hasDeck := output["deck"]; hasDeck {
		t.Errorf("--print --json should have no deck field: %v", output)
	}
}

func TestSlideAddUnknownLayout(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "slide", "add", "--layout", "bogus", "--print", "--json")
	if exitCode != exitUserError || !strings.Contains(stdout, `"code": "unknown_layout"`) || !strings.Contains(stdout, "split-media") {
		t.Errorf("(%d, %q), want exit 1, unknown_layout, and the list of layouts", exitCode, stdout)
	}
}

func TestSlideAddPrintNeedsALayout(t *testing.T) {
	exitCode, _, stderr := runTap(t, "slide", "add", "--print")
	if exitCode != exitUserError || !strings.Contains(stderr, "--print needs --layout") {
		t.Errorf("(%d, %q), want exit 1 and a message about --layout", exitCode, stderr)
	}
}
```

Add to `internal/cli/tui_parity_test.go` (add `"github.com/MiniCodeMonkey/tap/internal/layouts"` to its imports):

```go
func TestSlideAddLayoutMatchesTheWizardKey(t *testing.T) {
	for index, template := range layouts.Templates() {
		t.Run(template.Name, func(t *testing.T) {
			tuiDeck, commandDeck := twoDeckCopies(t, "talk.md", "# One\n")

			model := tea.Model(tui.NewDevModel(tui.DevConfig{MarkdownFile: tuiDeck}))
			model, _ = pressKeys(model, runeKey("a"))
			model, _ = pressKeys(model, repeatKey(tea.KeyMsg{Type: tea.KeyDown}, index)...)
			pressKeys(model, tea.KeyMsg{Type: tea.KeyEnter}, tea.KeyMsg{Type: tea.KeyCtrlD})

			if exitCode, _, stderr := runTap(t, "slide", "add", commandDeck, "--layout", template.Name); exitCode != exitOK {
				t.Fatalf("tap slide add exited %d: %s", exitCode, stderr)
			}
			requireSameFolders(t, tuiDeck, commandDeck)
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestSlideAdd' -short`
Expected: FAIL, `unknown flag: --layout`.

- [ ] **Step 3: Rewrite `internal/cli/slide.go`**

Replace the whole file:

```go
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/deckedit"
	"github.com/MiniCodeMonkey/tap/internal/layouts"
	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// Flags for tap slide add.
var (
	slideAddLayout string
	slideAddPrint  bool
	slideAddJSON   bool
)

// slideCmd groups the commands that work on a deck's slides.
var slideCmd = &cobra.Command{
	Use:   "slide",
	Short: "Work with the slides in a deck",
}

// slideAddCmd appends a slide, through the wizard or from a layout's
// template.
var slideAddCmd = &cobra.Command{
	Use:   "add [deck]",
	Short: "Add a slide to a deck",
	Long: `Add a slide to the end of a deck.

Without flags, an interactive wizard asks for a layout and the content of
each section of the slide. It needs a terminal.

With --layout, tap appends that layout's template without asking. With
--print as well, it prints the template and writes nothing; the template
has no "---" separator in front of it, and no deck is needed.

Layouts: ` + strings.Join(layouts.Names(), ", ") + `.

Examples:
  tap slide add                               # The wizard, for the deck in this folder
  tap slide add talk.md --layout quote        # Append a quote slide
  tap slide add --layout big-stat --print     # Print the big-stat template
  tap slide add --layout big-stat --print --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSlideAdd,
}

func init() {
	rootCmd.AddCommand(slideCmd)
	slideCmd.AddCommand(slideAddCmd)

	slideAddCmd.Flags().StringVar(&slideAddLayout, "layout", "", "append this layout's template instead of running the wizard")
	slideAddCmd.Flags().BoolVar(&slideAddPrint, "print", false, "print the template and write nothing (needs --layout)")
	slideAddCmd.Flags().BoolVar(&slideAddJSON, "json", false, "print the result as JSON (needs --layout)")
}

// slideAddResult is the --json result of tap slide add --layout. Deck is
// empty with --print.
type slideAddResult struct {
	Deck     string `json:"deck,omitempty"`
	Layout   string `json:"layout"`
	Markdown string `json:"markdown"`
}

func runSlideAdd(cmd *cobra.Command, args []string) error {
	if slideAddLayout != "" {
		return addSlideFromTemplate(cmd, args)
	}
	if slideAddPrint {
		return userError(codeUsage, errors.New("--print needs --layout"))
	}
	if slideAddJSON {
		return userError(codeUsage, errors.New("--json needs --layout: the wizard has no JSON output"))
	}

	if !stdinIsTerminal() {
		return userError(codeNeedsTerminal, errors.New("tap slide add runs a wizard and needs a terminal; pass --layout to add a slide without it"))
	}
	file, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
	result, err := tui.RunAddWizard(file)
	if err != nil {
		return internalError(codeInternal, err)
	}
	if result.Aborted {
		return errCancelled
	}
	return nil
}

// addSlideFromTemplate prints or appends the template of --layout.
func addSlideFromTemplate(cmd *cobra.Command, args []string) error {
	body, err := layouts.RenderSlide(slideAddLayout, nil)
	if err != nil {
		return userError(codeUnknownLayout, err)
	}

	if slideAddPrint {
		if slideAddJSON {
			return printJSONOK(cmd.OutOrStdout(), slideAddResult{Layout: slideAddLayout, Markdown: body})
		}
		_, err := fmt.Fprint(cmd.OutOrStdout(), body)
		return err
	}

	deck, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
	if err := deckedit.AppendSlide(deck, body); err != nil {
		return userError(codeInvalidDeck, fmt.Errorf("cannot add a slide to %s: %w", deck, err))
	}
	if slideAddJSON {
		return printJSONOK(cmd.OutOrStdout(), slideAddResult{Deck: deck, Layout: slideAddLayout, Markdown: body})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Added a %s slide to %s\n", slideAddLayout, deck)
	return nil
}
```

P1's `TestSlideAddWithoutATerminalIsAUserError` still passes: with no `--layout`, the terminal check comes first, as before.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestSlideAdd|TestCommandTree|TestEveryCommandFollows' -short -v 2>&1 | grep -E "^(--- FAIL|FAIL|ok)"`
Expected: `ok`. `TestSlideAddLayoutMatchesTheWizardKey` runs 12 subtests.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/slide.go internal/cli/slide_test.go internal/cli/tui_parity_test.go
git commit -m "feat(cli): add --layout and --print to tap slide add for all 12 layouts"
```

---

### Task 6: `tap image add`

**Files:**
- Create: `internal/deckedit/images.go`
- Create: `internal/deckedit/images_test.go`
- Create: `internal/cli/image.go`
- Create: `internal/cli/image_test.go`
- Modify: `internal/cli/conventions_test.go`

**Interfaces:**
- Consumes: `deckedit.SlideBodies`, `deckedit.InsertIntoFile` (Task 2); P1's `resolveDeck`, `slideOutOfRangeError`, error helpers
- Produces:
  - `func deckedit.ImagesDir(deckPath string) string`; `func deckedit.EnsureImagesDir(deckPath string) (string, error)`
  - `var deckedit.ErrNotAnImage error`; `type deckedit.AddedImage struct { Path, Markdown string }`; `func deckedit.AddImage(deckPath, sourcePath string) (AddedImage, error)`; `func deckedit.ImageMarkdown(imagePath string) string`
  - `var imageCmd`, `var imageAddCmd`; `func slideIndexFromFlag(deck string, slideNumber int) (int, error)` (1-based in, zero-based out); `type addedImageResult struct { Deck, Image, Markdown string; Slide int }`

- [ ] **Step 1: Write the failing `deckedit` tests**

`internal/deckedit/images_test.go`:

```go
package deckedit

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAddImageCopiesIntoImages(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")
	source := writeFile(t, filepath.Join(t.TempDir(), "diagram.png"), "png bytes")

	added, err := AddImage(deck, source)
	if err != nil {
		t.Fatalf("AddImage() error = %v", err)
	}
	if added.Path != filepath.Join("images", "diagram.png") || added.Markdown != "![diagram](images/diagram.png)" {
		t.Errorf("added = %+v", added)
	}
	copied, err := os.ReadFile(filepath.Join(deckDir, "images", "diagram.png"))
	if err != nil || string(copied) != "png bytes" {
		t.Errorf("copied file = (%q, %v)", copied, err)
	}
}

func TestAddImageNumbersAClash(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")
	source := writeFile(t, filepath.Join(t.TempDir(), "diagram.png"), "png bytes")

	for _, want := range []string{"diagram.png", "diagram-2.png", "diagram-3.png"} {
		added, err := AddImage(deck, source)
		if err != nil {
			t.Fatalf("AddImage() error = %v", err)
		}
		if added.Path != filepath.Join("images", want) {
			t.Errorf("Path = %q, want images/%s", added.Path, want)
		}
	}
}

func TestAddImageOfAFileAlreadyInImagesDoesNotCopy(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")
	source := writeFile(t, filepath.Join(deckDir, "images", "diagram.png"), "png bytes")

	added, err := AddImage(deck, source)
	if err != nil || added.Path != filepath.Join("images", "diagram.png") {
		t.Errorf("AddImage() = (%+v, %v), want the existing file", added, err)
	}
	entries, _ := os.ReadDir(filepath.Join(deckDir, "images"))
	if len(entries) != 1 {
		t.Errorf("images/ has %d files, want 1", len(entries))
	}
}

func TestAddImageRejectsANonImage(t *testing.T) {
	deck := writeFile(t, filepath.Join(t.TempDir(), "talk.md"), "# One\n")
	source := writeFile(t, filepath.Join(t.TempDir(), "notes.txt"), "text")
	if _, err := AddImage(deck, source); !errors.Is(err, ErrNotAnImage) {
		t.Errorf("AddImage() error = %v, want ErrNotAnImage", err)
	}
}

func TestAddImageMissingSource(t *testing.T) {
	deck := writeFile(t, filepath.Join(t.TempDir(), "talk.md"), "# One\n")
	if _, err := AddImage(deck, filepath.Join(t.TempDir(), "missing.png")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("AddImage() error = %v, want fs.ErrNotExist", err)
	}
}

func TestImageMarkdown(t *testing.T) {
	tests := map[string]string{
		filepath.Join("images", "diagram.png"):    "![diagram](images/diagram.png)",
		filepath.Join("images", "my diagram.png"): "![my diagram](<images/my diagram.png>)",
		filepath.Join("images", "chart (v2).png"): "![chart (v2)](<images/chart (v2).png>)",
		filepath.Join("images", "[draft].png"):    "![draft](images/[draft].png)",
	}
	for path, want := range tests {
		if got := ImageMarkdown(path); got != want {
			t.Errorf("ImageMarkdown(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestEnsureImagesDirWhenImagesIsAFile(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")
	writeFile(t, filepath.Join(deckDir, "images"), "not a folder")
	if _, err := EnsureImagesDir(deck); err == nil {
		t.Error("EnsureImagesDir() should fail when images is a file")
	}
}
```

- [ ] **Step 2: Write the failing command tests**

`internal/cli/image_test.go`:

```go
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const imageDeck = "# One\n\n---\n\n# Two\n"

func TestImageAddCopiesAndPrintsTheMarkdown(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeDeckFile(t, deckDir, "talk.md", imageDeck)
	source := writeDeckFile(t, t.TempDir(), "diagram.png", "png bytes")

	exitCode, stdout, stderr := runTap(t, "image", "add", source, deck)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if stdout != "![diagram](images/diagram.png)\n" {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(deckDir, "images", "diagram.png")); err != nil {
		t.Errorf("image not copied: %v", err)
	}
	content, _ := os.ReadFile(deck)
	if string(content) != imageDeck {
		t.Errorf("without --slide the deck should not change, got %q", content)
	}

	_, stdout, _ = runTap(t, "image", "add", source, deck)
	if stdout != "![diagram](images/diagram-2.png)\n" {
		t.Errorf("second add stdout = %q, want diagram-2.png", stdout)
	}
}

func TestImageAddWithSlideInsertsAtTheEndOfThatSlide(t *testing.T) {
	deck := writeDeckFile(t, t.TempDir(), "talk.md", imageDeck)
	source := writeDeckFile(t, t.TempDir(), "diagram.png", "png bytes")

	exitCode, stdout, stderr := runTap(t, "image", "add", source, deck, "--slide", "1", "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	var output struct {
		OK       bool   `json:"ok"`
		Deck     string `json:"deck"`
		Image    string `json:"image"`
		Markdown string `json:"markdown"`
		Slide    int    `json:"slide"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !output.OK || output.Image != "images/diagram.png" || output.Slide != 1 || output.Markdown != "![diagram](images/diagram.png)" {
		t.Errorf("output = %+v", output)
	}
	content, _ := os.ReadFile(deck)
	text := string(content)
	if !strings.Contains(text, "# One\n\n![diagram](images/diagram.png)\n") || strings.Index(text, "![diagram]") > strings.Index(text, "# Two") {
		t.Errorf("deck = %q, want the image at the end of slide 1", text)
	}
}

func TestImageAddSlideOutOfRangeCopiesNothing(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeDeckFile(t, deckDir, "talk.md", imageDeck)
	source := writeDeckFile(t, t.TempDir(), "diagram.png", "png bytes")

	exitCode, stdout, _ := runTap(t, "image", "add", source, deck, "--slide", "3", "--json")
	if exitCode != exitUserError || !strings.Contains(stdout, `"code": "out_of_range"`) {
		t.Errorf("(%d, %q), want exit 1 and out_of_range", exitCode, stdout)
	}
	if _, err := os.Stat(filepath.Join(deckDir, "images")); !os.IsNotExist(err) {
		t.Error("images/ should not be created when the slide is out of range")
	}
}

func TestImageAddErrors(t *testing.T) {
	deck := writeDeckFile(t, t.TempDir(), "talk.md", imageDeck)
	notes := writeDeckFile(t, t.TempDir(), "notes.txt", "text")
	tests := []struct {
		name string
		args []string
		code string
	}{
		{"missing file", []string{"image", "add", filepath.Join(t.TempDir(), "missing.png"), deck, "--json"}, codeFileNotFound},
		{"not an image", []string{"image", "add", notes, deck, "--json"}, codeNotAnImage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode, stdout, _ := runTap(t, tt.args...)
			if exitCode != exitUserError || !strings.Contains(stdout, `"code": "`+tt.code+`"`) {
				t.Errorf("(%d, %q), want exit 1 and code %s", exitCode, stdout, tt.code)
			}
		})
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/deckedit/ ./internal/cli/ -run 'TestAddImage|TestImageMarkdown|TestEnsureImagesDir|TestImageAdd' -short`
Expected: compile failure, `undefined: AddImage`, and `tap image` is an unknown command.

- [ ] **Step 4: Write `internal/deckedit/images.go`**

```go
package deckedit

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotAnImage means a file does not have an image extension tap accepts.
var ErrNotAnImage = errors.New("not an image")

// imageExtensions are the file types tap image add accepts, in lower case.
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".svg": true, ".avif": true,
}

// AddedImage is an image copied into a deck's images folder. Path is
// relative to the deck's folder.
type AddedImage struct {
	Path     string
	Markdown string
}

// ImagesDir is the images folder next to a deck.
func ImagesDir(deckPath string) string {
	return filepath.Join(filepath.Dir(deckPath), "images")
}

// EnsureImagesDir creates the images folder next to a deck when it does
// not exist, and returns its path.
func EnsureImagesDir(deckPath string) (string, error) {
	imagesDir := ImagesDir(deckPath)
	info, err := os.Stat(imagesDir)
	if err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("images path exists but is not a directory: %s", imagesDir)
		}
		return imagesDir, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check images directory: %w", err)
	}
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create images directory: %w", err)
	}
	return imagesDir, nil
}

// AddImage copies the image at sourcePath into the deck's images folder.
// It keeps the file name, and adds -2, -3 and so on before the extension
// when the name is taken. A source already in the images folder is used
// where it is, without a copy.
func AddImage(deckPath, sourcePath string) (AddedImage, error) {
	if !imageExtensions[strings.ToLower(filepath.Ext(sourcePath))] {
		return AddedImage{}, fmt.Errorf("%w: %s (tap accepts png, jpg, jpeg, gif, webp, svg and avif)", ErrNotAnImage, sourcePath)
	}
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return AddedImage{}, err
	}
	if sourceInfo.IsDir() {
		return AddedImage{}, fmt.Errorf("%w: %s is a folder", ErrNotAnImage, sourcePath)
	}

	imagesDir, err := EnsureImagesDir(deckPath)
	if err != nil {
		return AddedImage{}, err
	}
	name := filepath.Base(sourcePath)
	if existing, err := os.Stat(filepath.Join(imagesDir, name)); err == nil && os.SameFile(existing, sourceInfo) {
		return newAddedImage(name), nil
	}
	name, err = copyToFreeName(sourcePath, imagesDir, name)
	if err != nil {
		return AddedImage{}, err
	}
	return newAddedImage(name), nil
}

func newAddedImage(name string) AddedImage {
	path := filepath.Join("images", name)
	return AddedImage{Path: path, Markdown: ImageMarkdown(path)}
}

// copyToFreeName copies sourcePath into directory under name, or under
// name-2, name-3 and so on when that file exists. It returns the name used.
func copyToFreeName(sourcePath, directory, name string) (string, error) {
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	for number := 1; ; number++ {
		candidate := name
		if number > 1 {
			candidate = fmt.Sprintf("%s-%d%s", stem, number, extension)
		}
		destinationPath := filepath.Join(directory, candidate)
		destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("cannot create %s: %w", destinationPath, err)
		}
		copyErr := copyInto(destination, sourcePath)
		if closeErr := destination.Close(); copyErr == nil {
			copyErr = closeErr
		}
		if copyErr != nil {
			_ = os.Remove(destinationPath)
			return "", copyErr
		}
		return candidate, nil
	}
}

func copyInto(destination io.Writer, sourcePath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	_, err = io.Copy(destination, source)
	return err
}

// ImageMarkdown is the markdown that shows the image at imagePath, a path
// relative to the deck's folder. The alt text is the file name without its
// extension. A path with a space or parenthesis goes in angle brackets, so
// the link still parses.
func ImageMarkdown(imagePath string) string {
	destination := filepath.ToSlash(imagePath)
	if strings.ContainsAny(destination, " \t()") {
		destination = "<" + destination + ">"
	}
	alt := strings.TrimSuffix(filepath.Base(imagePath), filepath.Ext(imagePath))
	alt = strings.NewReplacer("[", "", "]", "").Replace(alt)
	return fmt.Sprintf("![%s](%s)", alt, destination)
}
```

- [ ] **Step 5: Write `internal/cli/image.go`**

```go
package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/deckedit"
)

// Flags for tap image add.
var (
	imageAddSlide int
	imageAddJSON  bool
)

// imageCmd groups the commands for a deck's images.
var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Add and generate a deck's images",
}

// imageAddCmd copies an image into a deck's images folder.
var imageAddCmd = &cobra.Command{
	Use:   "add <file> [deck]",
	Short: "Copy an image into the deck's images folder",
	Long: `Copy an image into the images/ folder next to a deck and print the
markdown that shows it.

The copy keeps the file name. When the name is taken, tap adds -2, -3 and
so on before the extension. With --slide N, tap also adds the markdown at
the end of slide N.

[deck] is a deck file or folder. With no deck, tap uses the deck in the
current folder.

Examples:
  tap image add ~/Desktop/diagram.png
  tap image add diagram.png talk.md --slide 3
  tap image add diagram.png --json`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runImageAdd,
}

func init() {
	rootCmd.AddCommand(imageCmd)
	imageCmd.AddCommand(imageAddCmd)

	imageAddCmd.Flags().IntVar(&imageAddSlide, "slide", 0, "also add the image at the end of this slide, from 1")
	imageAddCmd.Flags().BoolVar(&imageAddJSON, "json", false, "print the result as JSON")
}

// addedImageResult is the --json result of tap image add. Slide is 0 when
// --slide was not given.
type addedImageResult struct {
	Deck     string `json:"deck"`
	Image    string `json:"image"`
	Markdown string `json:"markdown"`
	Slide    int    `json:"slide,omitempty"`
}

func runImageAdd(cmd *cobra.Command, args []string) error {
	source := args[0]
	var deckArg string
	if len(args) > 1 {
		deckArg = args[1]
	}
	deck, err := resolveDeck(deckArg)
	if err != nil {
		return err
	}

	hasSlide := cmd.Flags().Changed("slide")
	slideIndex := 0
	if hasSlide {
		if slideIndex, err = slideIndexFromFlag(deck, imageAddSlide); err != nil {
			return err
		}
	}

	added, err := deckedit.AddImage(deck, source)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return userError(codeFileNotFound, fmt.Errorf("file not found: %s", source))
	case errors.Is(err, deckedit.ErrNotAnImage):
		return userError(codeNotAnImage, err)
	case err != nil:
		return internalError(codeInternal, err)
	}

	if hasSlide {
		if err := deckedit.InsertIntoFile(deck, slideIndex, added.Markdown); err != nil {
			return internalError(codeInternal, err)
		}
	}

	if imageAddJSON {
		result := addedImageResult{Deck: deck, Image: filepath.ToSlash(added.Path), Markdown: added.Markdown}
		if hasSlide {
			result.Slide = imageAddSlide
		}
		return printJSONOK(cmd.OutOrStdout(), result)
	}
	fmt.Fprintln(cmd.OutOrStdout(), added.Markdown)
	return nil
}

// slideIndexFromFlag checks a 1-based --slide value against the deck and
// returns the zero-based index that internal/deckedit uses.
func slideIndexFromFlag(deck string, slideNumber int) (int, error) {
	content, err := os.ReadFile(deck)
	if err != nil {
		return 0, userError(codeDeckNotFound, fmt.Errorf("cannot read %s: %w", deck, err))
	}
	total := len(deckedit.SlideBodies(string(content)))
	if slideNumber < 1 || slideNumber > total {
		return 0, userError(codeOutOfRange, slideOutOfRangeError(slideNumber, total))
	}
	return slideNumber - 1, nil
}
```

The imports of `image.go` are `errors`, `fmt`, `io/fs`, `os`, `path/filepath`, cobra and `internal/deckedit`.

- [ ] **Step 6: Add the commands to `expectedCommands`**

In `internal/cli/conventions_test.go`, add `"tap image",` and `"tap image add",` after `"tap export pdf",` and before `"tap new",`.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/deckedit/ ./internal/cli/ -run 'TestAddImage|TestImageMarkdown|TestEnsureImagesDir|TestImageAdd|TestCommandTree|TestEveryCommandFollows' -short -v 2>&1 | grep -E "^(--- FAIL|FAIL|ok)"`
Expected: `ok` for both packages.

- [ ] **Step 8: Commit**

```bash
git add internal/deckedit/images.go internal/deckedit/images_test.go internal/cli/image.go internal/cli/image_test.go internal/cli/conventions_test.go
git commit -m "feat(cli): add tap image add"
```

---

### Task 7: One function places a generated image

The TUI `i` key saves the generated image, inserts or replaces the markdown, and deletes the old file, in four `ImageGenModel` methods called from `DevModel.Update`. This task puts those steps in one `deckedit` function and a generator seam, and makes the TUI call them. Tasks 8 and 9 call the same function.

**Files:**
- Create: `internal/deckedit/generate.go`
- Create: `internal/deckedit/generate_test.go`
- Modify: `internal/tui/imagegen.go`
- Modify: `internal/tui/imagegen_test.go`
- Modify: `internal/tui/dev.go`

**Interfaces:**
- Consumes: `EnsureImagesDir` (Task 6); `InsertAIImage`, `ReplaceAIImage`, `AIImageMarkdown`, `GenerateImageFilename` (Task 2); `gemini.ImageResult`, `gemini.NewClientFromEnv`
- Produces, in `internal/deckedit`:
  - `type ImageGenerator interface { GenerateImage(ctx context.Context, prompt string) (*gemini.ImageResult, error) }`
  - `var NewImageGenerator func() (ImageGenerator, error)`: the Gemini client by default; tests replace it.
  - `type Placement struct { DeckPath string; SlideIndex int; Prompt string; Replacing *AIImage }`
  - `type PlacedImage struct { Path, Markdown, DeletedPath string; DeleteError error }`
  - `func SaveImage(deckPath string, image gemini.ImageResult) (string, error)`
  - `func PlaceGeneratedImage(placement Placement, image gemini.ImageResult) (PlacedImage, error)`
- In `internal/tui`: `func (m *ImageGenModel) PlaceImage() (deckedit.PlacedImage, error)`. `SaveGeneratedImage`, `InsertImageIntoMarkdown`, `DeleteOldImage`, `ReplaceImageInMarkdown`, `GetImagesDir` and `EnsureImagesDir` are removed.

- [ ] **Step 1: Write the failing tests**

`internal/deckedit/generate_test.go`:

```go
package deckedit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/gemini"
)

func pngImage(content string) gemini.ImageResult {
	return gemini.ImageResult{Data: []byte(content), ContentType: "image/png"}
}

func TestPlaceGeneratedImageAddsToTheSlide(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "---\ntheme: paper\n---\n\n# One\n\n---\n\n# Two\n")

	image := pngImage("new image")
	placed, err := PlaceGeneratedImage(Placement{DeckPath: deck, SlideIndex: 1, Prompt: "a red fox"}, image)
	if err != nil {
		t.Fatalf("PlaceGeneratedImage() error = %v", err)
	}
	wantPath := filepath.Join("images", GenerateImageFilename(image.Data, image.ContentType))
	if placed.Path != wantPath || placed.Markdown != AIImageMarkdown("a red fox", wantPath) {
		t.Errorf("placed = %+v", placed)
	}
	saved, err := os.ReadFile(filepath.Join(deckDir, wantPath))
	if err != nil || string(saved) != "new image" {
		t.Errorf("saved image = (%q, %v)", saved, err)
	}
	content, _ := os.ReadFile(deck)
	if !strings.Contains(string(content), "# Two\n\n"+placed.Markdown+"\n") {
		t.Errorf("deck = %q", content)
	}
	if !strings.HasPrefix(string(content), "---\ntheme: paper\n---\n") {
		t.Errorf("frontmatter lost: %q", content)
	}
}

func TestPlaceGeneratedImageReplacesInPlaceAndDeletesTheOldFile(t *testing.T) {
	deckDir := t.TempDir()
	oldPath := writeFile(t, filepath.Join(deckDir, "images", "generated-old00000.png"), "old image")
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"),
		"# One\n\nBefore\n\n<!-- ai-prompt: a blue whale -->\n![](images/generated-old00000.png)\n\nAfter\n")

	replacing := AIImage{Prompt: "a blue whale", ImagePath: "images/generated-old00000.png"}
	placed, err := PlaceGeneratedImage(Placement{DeckPath: deck, Prompt: "a green whale", Replacing: &replacing}, pngImage("new image"))
	if err != nil {
		t.Fatalf("PlaceGeneratedImage() error = %v", err)
	}
	if placed.DeletedPath != "images/generated-old00000.png" || placed.DeleteError != nil {
		t.Errorf("placed = %+v", placed)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("the old image should be deleted")
	}
	content, _ := os.ReadFile(deck)
	want := "# One\n\nBefore\n\n" + placed.Markdown + "\n\nAfter\n"
	if string(content) != want {
		t.Errorf("deck = %q, want %q", content, want)
	}
}

func TestPlaceGeneratedImageKeepsAFileWithTheSameName(t *testing.T) {
	deckDir := t.TempDir()
	image := pngImage("same bytes")
	name := GenerateImageFilename(image.Data, image.ContentType)
	writeFile(t, filepath.Join(deckDir, "images", name), "same bytes")
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n\n<!-- ai-prompt: a cat -->\n![](images/"+name+")\n")

	replacing := AIImage{Prompt: "a cat", ImagePath: "images/" + name}
	placed, err := PlaceGeneratedImage(Placement{DeckPath: deck, Prompt: "a cat", Replacing: &replacing}, image)
	if err != nil {
		t.Fatalf("PlaceGeneratedImage() error = %v", err)
	}
	if placed.DeletedPath != "" {
		t.Errorf("DeletedPath = %q, want nothing deleted", placed.DeletedPath)
	}
	if _, err := os.Stat(filepath.Join(deckDir, "images", name)); err != nil {
		t.Errorf("the new image was deleted: %v", err)
	}
}

func TestPlaceGeneratedImageWritesADollarSignAsTyped(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n\n<!-- ai-prompt: old -->\n![](images/old.png)\n")
	replacing := AIImage{Prompt: "old", ImagePath: "images/old.png"}
	if _, err := PlaceGeneratedImage(Placement{DeckPath: deck, Prompt: "costs $1 a day", Replacing: &replacing}, pngImage("x")); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(deck)
	if !strings.Contains(string(content), "<!-- ai-prompt: costs $1 a day -->") {
		t.Errorf("deck = %q", content)
	}
}

func TestPlaceGeneratedImageSlideOutOfRange(t *testing.T) {
	deck := writeFile(t, filepath.Join(t.TempDir(), "talk.md"), "# One\n")
	if _, err := PlaceGeneratedImage(Placement{DeckPath: deck, SlideIndex: 4, Prompt: "x"}, pngImage("x")); err == nil {
		t.Error("PlaceGeneratedImage() on a missing slide should fail")
	}
}

func TestSaveImageNamesTheFileByItsContent(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")
	image := gemini.ImageResult{Data: []byte("jpeg bytes"), ContentType: "image/jpeg"}
	path, err := SaveImage(deck, image)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, filepath.Join("images", "generated-")) || !strings.HasSuffix(path, ".jpg") {
		t.Errorf("path = %q", path)
	}
	info, err := os.Stat(filepath.Join(deckDir, path))
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("saved file = (%v, %v), want mode 0644", info, err)
	}
}
```

- [ ] **Step 2: Delete the TUI tests the new tests replace**

These tests exercise the `ImageGenModel` file methods this task removes. `generate_test.go` and `images_test.go` cover the same cases. Delete them with the script, with `-` as the target:

```bash
python3 - internal/tui/imagegen_test.go - \
  'TestImageGenModel_GetImagesDir' \
  'TestImageGenModel_EnsureImagesDir_\w+' \
  'TestImageGenModel_SaveGeneratedImage\w*' \
  'TestImageGenModel_InsertImageIntoMarkdown\w*' \
  'TestDeleteOldImage\w*' \
  'TestReplaceImageInMarkdown_\w+' \
  'TestImageRegeneration_FullWorkflow_Integration' <<'EOF'
import re
import sys

source_path, target_path, patterns = sys.argv[1], sys.argv[2], sys.argv[3:]
with open(source_path) as source_file:
    text = source_file.read()

# A top-level test ends at a closing brace in column 0 that is followed by
# the next top-level declaration, a comment, or the end of the file.
moved = []
for pattern in patterns:
    expression = re.compile(
        r"\nfunc (" + pattern + r")\(t \*testing\.T\) \{\n.*?\n\}\n"
        r"(?=\n*(?:func |// |type |var |const )|\n*\Z)",
        re.S,
    )
    matches = list(expression.finditer(text))
    if not matches:
        sys.exit(f"no test matches {pattern}")
    print(pattern, "->", [match.group(1) for match in matches])
    moved.extend(match.group(0) for match in matches)
    for match in reversed(matches):
        text = text[: match.start()] + text[match.end():]

if target_path != "-":
    with open(target_path, "a") as target_file:
        target_file.write("".join(moved))
with open(source_path, "w") as source_file:
    source_file.write(text)
EOF
gofmt -w internal/tui/imagegen_test.go
```

Then check that no test still calls a removed method:

```bash
grep -n "SaveGeneratedImage\|InsertImageIntoMarkdown\|DeleteOldImage\|ReplaceImageInMarkdown\|GetImagesDir\|EnsureImagesDir" internal/tui/*_test.go
```

Expected: no output. If a remaining test calls one (for example a Done-step test that saves first), change that call to `model.PlaceImage()` and read `.Path` from the result.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/deckedit/ -run 'TestPlaceGeneratedImage|TestSaveImage' -short`
Expected: compile failure, `undefined: PlaceGeneratedImage`.

- [ ] **Step 4: Write `internal/deckedit/generate.go`**

```go
package deckedit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MiniCodeMonkey/tap/internal/gemini"
)

// ImageGenerator makes an image from a text prompt. *gemini.Client is one.
type ImageGenerator interface {
	GenerateImage(ctx context.Context, prompt string) (*gemini.ImageResult, error)
}

// NewImageGenerator returns the generator that tap image generate, tap
// image regenerate and the TUI i key use: the Gemini client, configured
// from GEMINI_API_KEY. Tests replace it with a fake, so no test calls the
// Gemini API.
var NewImageGenerator = func() (ImageGenerator, error) {
	client, err := gemini.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Placement says where a generated image goes: at the end of the slide at
// SlideIndex, or, when Replacing is set, in place of that AI image.
type Placement struct {
	DeckPath   string
	SlideIndex int
	Prompt     string
	Replacing  *AIImage
}

// PlacedImage is the result of PlaceGeneratedImage. Path is relative to
// the deck's folder. DeletedPath is the replaced image's file, removed
// from disk. DeleteError is set when that file could not be removed; the
// deck is already updated then.
type PlacedImage struct {
	Path        string
	Markdown    string
	DeletedPath string
	DeleteError error
}

// SaveImage writes a generated image into the deck's images folder, named
// by its content, and returns its path relative to the deck's folder.
func SaveImage(deckPath string, image gemini.ImageResult) (string, error) {
	imagesDir, err := EnsureImagesDir(deckPath)
	if err != nil {
		return "", fmt.Errorf("failed to ensure images directory: %w", err)
	}
	filename := GenerateImageFilename(image.Data, image.ContentType)
	if err := os.WriteFile(filepath.Join(imagesDir, filename), image.Data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write image file: %w", err)
	}
	return filepath.Join("images", filename), nil
}

// PlaceGeneratedImage saves image into the deck's images folder and
// records it in the deck with its prompt: added at the end of the slide,
// or in place of the image it replaces. A replaced image's file is
// deleted, unless the new image has the same file name.
func PlaceGeneratedImage(placement Placement, image gemini.ImageResult) (PlacedImage, error) {
	path, err := SaveImage(placement.DeckPath, image)
	if err != nil {
		return PlacedImage{}, err
	}
	placed := PlacedImage{Path: path, Markdown: AIImageMarkdown(placement.Prompt, path)}

	content, err := os.ReadFile(placement.DeckPath)
	if err != nil {
		return placed, fmt.Errorf("failed to read markdown file: %w", err)
	}
	var updated string
	if placement.Replacing == nil {
		updated, err = InsertAIImage(string(content), placement.SlideIndex, placement.Prompt, path)
	} else {
		updated, err = ReplaceAIImage(string(content), placement.Replacing.Prompt, placement.Replacing.ImagePath, placement.Prompt, path)
	}
	if err != nil {
		return placed, fmt.Errorf("failed to update markdown: %w", err)
	}
	if err := os.WriteFile(placement.DeckPath, []byte(updated), 0o644); err != nil {
		return placed, fmt.Errorf("failed to write markdown file: %w", err)
	}

	if placement.Replacing != nil && filepath.Clean(placement.Replacing.ImagePath) != filepath.Clean(path) {
		if err := deleteImage(placement.DeckPath, placement.Replacing.ImagePath); err != nil {
			placed.DeleteError = err
		} else {
			placed.DeletedPath = placement.Replacing.ImagePath
		}
	}
	return placed, nil
}

// deleteImage removes an image given by its path relative to the deck's
// folder. A file that is already gone is not an error.
func deleteImage(deckPath, imagePath string) error {
	err := os.Remove(filepath.Join(filepath.Dir(deckPath), imagePath))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete old image: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Make the TUI use it**

In `internal/tui/imagegen.go`:

1. Delete `GetImagesDir`, `EnsureImagesDir`, `SaveGeneratedImage`, `InsertImageIntoMarkdown`, `DeleteOldImage` and `ReplaceImageInMarkdown`, with their comments. Add:

```go
// PlaceImage saves the generated image and records it in the deck: at the
// end of the selected slide, or in place of the image being regenerated.
func (m *ImageGenModel) PlaceImage() (deckedit.PlacedImage, error) {
	if m.GeneratedImage == nil {
		return deckedit.PlacedImage{}, fmt.Errorf("no generated image to save")
	}
	return deckedit.PlaceGeneratedImage(deckedit.Placement{
		DeckPath:   m.MarkdownFile,
		SlideIndex: m.SelectedIndex,
		Prompt:     m.Prompt,
		Replacing:  m.SelectedImage,
	}, gemini.ImageResult{Data: m.GeneratedImage.ImageData, ContentType: m.GeneratedImage.ContentType})
}
```

2. In `generateImageCmd`, change `client, err := gemini.NewClientFromEnv()` to `client, err := deckedit.NewImageGenerator()`. The rest of the function stays.
3. Remove imports the build reports unused (`path/filepath`, `os` if nothing else uses them).

In `internal/tui/dev.go`, `Update`, replace the block that starts at `// Save the generated image` and ends before `// Send reload event` with:

```go
					placed, err := m.imageGenModel.PlaceImage()
					if err != nil {
						m.SetError(err)
						m.addEvent(DevEvent{
							Type:      "error",
							Message:   "Failed to add the generated image to the deck",
							Timestamp: time.Now(),
						})
						return m, cmd
					}
					savedPath := placed.Path
					m.imageGenModel.SavedImagePath = savedPath
					if placed.DeleteError != nil {
						m.addEvent(DevEvent{
							Type:      "error",
							Message:   "Failed to delete old image (non-fatal)",
							Timestamp: time.Now(),
						})
					}
```

The `// Send reload event` block after it keeps using `savedPath`.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go build ./... && go test ./internal/deckedit/ ./internal/tui/ -short -v 2>&1 | grep -E "^(--- FAIL|FAIL|ok)"`
Expected: `ok` for both.

- [ ] **Step 7: Commit**

```bash
git add internal/deckedit/generate.go internal/deckedit/generate_test.go internal/tui/imagegen.go internal/tui/imagegen_test.go internal/tui/dev.go
git commit -m "refactor(tui): place generated images through internal/deckedit"
```

---

### Task 8: `tap image generate`

**Files:**
- Modify: `internal/cli/image.go`
- Modify: `internal/cli/image_test.go`
- Modify: `internal/cli/edit_helpers_test.go`
- Modify: `internal/cli/tui_parity_test.go`
- Modify: `internal/cli/conventions_test.go`

**Interfaces:**
- Consumes: `deckedit.NewImageGenerator`, `deckedit.PlaceGeneratedImage`, `deckedit.Placement` (Task 7); `slideIndexFromFlag` (Task 6); `config.LoadEnv(dir string) error`; P1's `errInterrupted`
- Produces:
  - `var imageGenerateCmd`; flags `imageGenerateSlide int`, `imageGeneratePrompt string`, `imageGenerateJSON bool`
  - `type generatedImageResult struct { Deck string; Slide int; Image, Prompt, Markdown, Replaced string }` (JSON `deck`, `slide`, `image`, `prompt`, `markdown`, `replaced` omitted when empty)
  - `func generateAndPlace(ctx context.Context, placement deckedit.Placement) (deckedit.PlacedImage, error)`
  - `func imageGenerationError(err error) error`
  - Test helpers: `type fakeImageGenerator struct { prompts []string; err error }`, `func useFakeImageGenerator(t *testing.T) *fakeImageGenerator`, `func deliverCommand(model tea.Model, command tea.Cmd) tea.Model`

- [ ] **Step 1: Add the fake generator and the command runner to the helpers**

Append to `internal/cli/edit_helpers_test.go` (add `"context"`, `"github.com/MiniCodeMonkey/tap/internal/deckedit"` and `"github.com/MiniCodeMonkey/tap/internal/gemini"` to its imports):

```go
// fakeImageGenerator stands in for the Gemini API. Each image's bytes are
// derived from its prompt, so the same prompt gives the same file name.
type fakeImageGenerator struct {
	err     error
	prompts []string
}

func (f *fakeImageGenerator) GenerateImage(ctx context.Context, prompt string) (*gemini.ImageResult, error) {
	f.prompts = append(f.prompts, prompt)
	if f.err != nil {
		return nil, f.err
	}
	return &gemini.ImageResult{Data: []byte("png bytes for " + prompt), ContentType: "image/png"}, nil
}

// useFakeImageGenerator makes tap and the TUI use a fake generator for
// the rest of the test, and sets GEMINI_API_KEY so the TUI opens its
// image generator.
func useFakeImageGenerator(t *testing.T) *fakeImageGenerator {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", "test-key")
	fake := &fakeImageGenerator{}
	original := deckedit.NewImageGenerator
	deckedit.NewImageGenerator = func() (deckedit.ImageGenerator, error) { return fake, nil }
	t.Cleanup(func() { deckedit.NewImageGenerator = original })
	return fake
}

// deliverCommand runs command and sends each message it produces to
// model, flattening batches. It does not run the commands those messages
// return, so a spinner tick does not loop.
func deliverCommand(model tea.Model, command tea.Cmd) tea.Model {
	if command == nil {
		return model
	}
	message := command()
	if batch, ok := message.(tea.BatchMsg); ok {
		for _, inner := range batch {
			model = deliverCommand(model, inner)
		}
		return model
	}
	model, _ = model.Update(message)
	return model
}
```

- [ ] **Step 2: Write the failing tests**

Add to `internal/cli/image_test.go` (add `"github.com/MiniCodeMonkey/tap/internal/deckedit"` and `"github.com/MiniCodeMonkey/tap/internal/gemini"` to its imports):

```go
func TestImageGenerateAddsTheImageToTheSlide(t *testing.T) {
	fake := useFakeImageGenerator(t)
	deckDir := t.TempDir()
	deck := writeDeckFile(t, deckDir, "talk.md", imageDeck)

	exitCode, stdout, stderr := runTap(t, "image", "generate", deck, "--slide", "2", "--prompt", "a red fox", "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if len(fake.prompts) != 1 || fake.prompts[0] != "a red fox" {
		t.Errorf("generator prompts = %q", fake.prompts)
	}
	var output struct {
		OK       bool   `json:"ok"`
		Deck     string `json:"deck"`
		Slide    int    `json:"slide"`
		Image    string `json:"image"`
		Prompt   string `json:"prompt"`
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	wantImage := "images/" + deckedit.GenerateImageFilename([]byte("png bytes for a red fox"), "image/png")
	if !output.OK || output.Slide != 2 || output.Image != wantImage || output.Prompt != "a red fox" {
		t.Errorf("output = %+v, want image %s", output, wantImage)
	}
	content, _ := os.ReadFile(deck)
	if !strings.HasSuffix(strings.TrimRight(string(content), "\n"), "<!-- ai-prompt: a red fox -->\n![]("+wantImage+")") {
		t.Errorf("deck = %q", content)
	}
	if _, err := os.Stat(filepath.Join(deckDir, wantImage)); err != nil {
		t.Errorf("image not saved: %v", err)
	}
}

func TestImageGeneratePrintsTheImagePath(t *testing.T) {
	useFakeImageGenerator(t)
	deck := writeDeckFile(t, t.TempDir(), "talk.md", imageDeck)
	_, stdout, _ := runTap(t, "image", "generate", deck, "--slide", "1", "--prompt", "a red fox")
	want := "images/" + deckedit.GenerateImageFilename([]byte("png bytes for a red fox"), "image/png") + "\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestImageGenerateUsageErrors(t *testing.T) {
	useFakeImageGenerator(t)
	deck := writeDeckFile(t, t.TempDir(), "talk.md", imageDeck)
	tests := []struct {
		name string
		args []string
		code string
	}{
		{"no slide", []string{"image", "generate", deck, "--prompt", "x", "--json"}, codeUsage},
		{"no prompt", []string{"image", "generate", deck, "--slide", "1", "--json"}, codeUsage},
		{"blank prompt", []string{"image", "generate", deck, "--slide", "1", "--prompt", "  ", "--json"}, codeUsage},
		{"slide out of range", []string{"image", "generate", deck, "--slide", "9", "--prompt", "x", "--json"}, codeOutOfRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode, stdout, _ := runTap(t, tt.args...)
			if exitCode != exitUserError || !strings.Contains(stdout, `"code": "`+tt.code+`"`) {
				t.Errorf("(%d, %q), want exit 1 and code %s", exitCode, stdout, tt.code)
			}
		})
	}
}

func TestImageGenerateWithoutAnAPIKey(t *testing.T) {
	original := deckedit.NewImageGenerator
	deckedit.NewImageGenerator = func() (deckedit.ImageGenerator, error) {
		return nil, &gemini.APIError{Type: gemini.ErrorTypeAuth, Message: "API key is required"}
	}
	t.Cleanup(func() { deckedit.NewImageGenerator = original })

	deck := writeDeckFile(t, t.TempDir(), "talk.md", imageDeck)
	exitCode, stdout, _ := runTap(t, "image", "generate", deck, "--slide", "1", "--prompt", "x", "--json")
	if exitCode != exitUserError || !strings.Contains(stdout, `"code": "no_api_key"`) {
		t.Errorf("(%d, %q), want exit 1 and no_api_key", exitCode, stdout)
	}
}

func TestImageGenerationErrorExitCodes(t *testing.T) {
	tests := []struct {
		errorType    gemini.ErrorType
		wantExitCode int
	}{
		{gemini.ErrorTypeContentPolicy, exitUserError},
		{gemini.ErrorTypeRateLimit, exitUserError},
		{gemini.ErrorTypeNetwork, exitInternal},
		{gemini.ErrorTypeServer, exitInternal},
	}
	for _, tt := range tests {
		exitCode, code, _ := classify(imageGenerationError(&gemini.APIError{Type: tt.errorType, Message: "x"}))
		if exitCode != tt.wantExitCode || code != codeImageGeneration {
			t.Errorf("%s: classify() = (%d, %q), want (%d, %q)", tt.errorType, exitCode, code, tt.wantExitCode, codeImageGeneration)
		}
	}
}
```

Add to `internal/cli/tui_parity_test.go`:

```go
func TestImageGenerateMatchesTheImageKey(t *testing.T) {
	useFakeImageGenerator(t)
	tuiDeck, commandDeck := twoDeckCopies(t, "talk.md", "---\ntheme: paper\n---\n\n# One\n\n---\n\n# Two\n\nText\n")

	model := tea.Model(tui.NewDevModel(tui.DevConfig{MarkdownFile: tuiDeck}))
	model, _ = pressKeys(model, runeKey("i"), tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyEnter}, runeKey("a red fox"))
	model, submit := pressKeys(model, tea.KeyMsg{Type: tea.KeyCtrlD})
	deliverCommand(model, submit)

	if exitCode, _, stderr := runTap(t, "image", "generate", commandDeck, "--slide", "2", "--prompt", "a red fox"); exitCode != exitOK {
		t.Fatalf("tap image generate exited %d: %s", exitCode, stderr)
	}
	requireSameFolders(t, tuiDeck, commandDeck)
	if len(folderSnapshot(t, filepath.Dir(tuiDeck))) != 2 {
		t.Error("the TUI path wrote no image: the parity check compared two unchanged folders")
	}
}
```

Add `"path/filepath"` to the imports of `tui_parity_test.go`.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestImageGenerat|TestImageGenerateMatches' -short`
Expected: compile failure, `undefined: imageGenerationError`, and `tap image generate` is an unknown command.

- [ ] **Step 4: Add `tap image generate` to `internal/cli/image.go`**

Add to the imports: `"context"`, `"os/signal"`, `"strings"`, `"syscall"`, `"github.com/MiniCodeMonkey/tap/internal/config"`, `"github.com/MiniCodeMonkey/tap/internal/gemini"`.

Add the flag variables below the `tap image add` flags:

```go
// Flags for tap image generate.
var (
	imageGenerateSlide  int
	imageGeneratePrompt string
	imageGenerateJSON   bool
)
```

Add the command, and register it in `init` with its flags:

```go
// imageGenerateCmd generates an image with AI and adds it to a slide.
var imageGenerateCmd = &cobra.Command{
	Use:   "generate [deck]",
	Short: "Generate an image with AI and add it to a slide",
	Long: `Generate an image from a prompt with Google Gemini, as the i key in
tap dev does, and add it at the end of a slide.

The image is saved as images/generated-<hash>.<ext> next to the deck, and
the slide gets an ai-prompt comment with the prompt, so tap image
regenerate can make it again. GEMINI_API_KEY must be set, in the
environment or in a .env file next to the deck.

Examples:
  tap image generate --slide 3 --prompt "a lighthouse at dusk, flat vector"
  tap image generate talk.md --slide 3 --prompt "..." --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runImageGenerate,
}
```

In `init`:

```go
	imageCmd.AddCommand(imageGenerateCmd)
	imageGenerateCmd.Flags().IntVar(&imageGenerateSlide, "slide", 0, "slide to add the image to, from 1 (required)")
	imageGenerateCmd.Flags().StringVar(&imageGeneratePrompt, "prompt", "", "what the image shows (required)")
	imageGenerateCmd.Flags().BoolVar(&imageGenerateJSON, "json", false, "print the result as JSON")
```

Add the run function and the shared helpers:

```go
// generatedImageResult is the --json result of tap image generate and tap
// image regenerate. Replaced is the image that regenerate replaced.
type generatedImageResult struct {
	Deck     string `json:"deck"`
	Slide    int    `json:"slide"`
	Image    string `json:"image"`
	Prompt   string `json:"prompt"`
	Markdown string `json:"markdown"`
	Replaced string `json:"replaced,omitempty"`
}

func runImageGenerate(cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Changed("slide") {
		return userError(codeUsage, errors.New("--slide is required"))
	}
	prompt := strings.TrimSpace(imageGeneratePrompt)
	if prompt == "" {
		return userError(codeUsage, errors.New("--prompt is required"))
	}
	deck, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
	slideIndex, err := slideIndexFromFlag(deck, imageGenerateSlide)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	placed, err := generateAndPlace(ctx, deckedit.Placement{DeckPath: deck, SlideIndex: slideIndex, Prompt: prompt})
	if err != nil {
		return err
	}

	if imageGenerateJSON {
		return printJSONOK(cmd.OutOrStdout(), generatedImageResult{
			Deck:     deck,
			Slide:    imageGenerateSlide,
			Image:    filepath.ToSlash(placed.Path),
			Prompt:   prompt,
			Markdown: placed.Markdown,
		})
	}
	fmt.Fprintln(cmd.OutOrStdout(), filepath.ToSlash(placed.Path))
	return nil
}

// generateAndPlace generates an image for placement.Prompt and places it
// in the deck through deckedit.PlaceGeneratedImage, the function the TUI
// i key also uses. It reads GEMINI_API_KEY from a .env file next to the
// deck when the environment has none.
func generateAndPlace(ctx context.Context, placement deckedit.Placement) (deckedit.PlacedImage, error) {
	if err := config.LoadEnv(filepath.Dir(placement.DeckPath)); err != nil {
		return deckedit.PlacedImage{}, userError(codeInvalidDeck, fmt.Errorf("cannot read the .env file next to %s: %w", placement.DeckPath, err))
	}
	generator, err := deckedit.NewImageGenerator()
	if err != nil {
		return deckedit.PlacedImage{}, userError(codeNoAPIKey, fmt.Errorf("cannot start image generation: %w", err))
	}
	image, err := generator.GenerateImage(ctx, placement.Prompt)
	if err != nil {
		if ctx.Err() != nil {
			return deckedit.PlacedImage{}, errInterrupted
		}
		return deckedit.PlacedImage{}, imageGenerationError(err)
	}
	placed, err := deckedit.PlaceGeneratedImage(placement, *image)
	if err != nil {
		return deckedit.PlacedImage{}, internalError(codeInternal, err)
	}
	return placed, nil
}

// imageGenerationError classifies a failed generation. A network or
// server failure is a problem in the environment (exit 2); a refused
// prompt, a rate limit or a bad key is one the person can fix (exit 1).
func imageGenerationError(err error) error {
	wrapped := fmt.Errorf("image generation failed: %w", err)
	var apiError *gemini.APIError
	if errors.As(err, &apiError) && (apiError.Type == gemini.ErrorTypeNetwork || apiError.Type == gemini.ErrorTypeServer) {
		return internalError(codeImageGeneration, wrapped)
	}
	return userError(codeImageGeneration, wrapped)
}
```

- [ ] **Step 5: Add the command to `expectedCommands`**

In `internal/cli/conventions_test.go`, add `"tap image generate",` after `"tap image add",`.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestImage|TestCommandTree|TestEveryCommandFollows' -short -v 2>&1 | grep -E "^(--- FAIL|FAIL|ok)"`
Expected: `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/image.go internal/cli/image_test.go internal/cli/edit_helpers_test.go internal/cli/tui_parity_test.go internal/cli/conventions_test.go
git commit -m "feat(cli): add tap image generate, sharing the i key's code"
```

---

### Task 9: `tap image regenerate`

**Files:**
- Modify: `internal/cli/image.go`
- Modify: `internal/cli/image_test.go`
- Modify: `internal/cli/tui_parity_test.go`
- Modify: `internal/cli/conventions_test.go`

**Interfaces:**
- Consumes: `generateAndPlace`, `generatedImageResult`, `slideIndexFromFlag` (Tasks 6 and 8); `deckedit.SlideBodies`, `deckedit.ParseAIImages`, `deckedit.AIImage` (Task 2)
- Produces: `var imageRegenerateCmd`; flags `imageRegenerateSlide int`, `imageRegenerateImage string`, `imageRegeneratePrompt string`, `imageRegenerateJSON bool`; `func findAIImage(deck string, slideIndex int, imagePath string) (deckedit.AIImage, error)`

- [ ] **Step 1: Write the failing tests**

Add to `internal/cli/image_test.go`:

```go
const regenerateDeck = "# One\n\n---\n\n# Two\n\nBefore\n\n<!-- ai-prompt: a blue whale -->\n![](images/generated-old00000.png)\n\nAfter\n"

func writeRegenerateDeck(t *testing.T, dir string) string {
	t.Helper()
	writeDeckFile(t, dir, "images/generated-old00000.png", "old image")
	return writeDeckFile(t, dir, "talk.md", regenerateDeck)
}

func TestImageRegenerateReusesThePromptAndReplacesInPlace(t *testing.T) {
	fake := useFakeImageGenerator(t)
	deckDir := t.TempDir()
	deck := writeRegenerateDeck(t, deckDir)

	exitCode, stdout, stderr := runTap(t, "image", "regenerate", deck, "--slide", "2", "--image", "images/generated-old00000.png", "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if len(fake.prompts) != 1 || fake.prompts[0] != "a blue whale" {
		t.Errorf("generator prompts = %q, want the old prompt", fake.prompts)
	}
	newImage := "images/" + deckedit.GenerateImageFilename([]byte("png bytes for a blue whale"), "image/png")
	if !strings.Contains(stdout, `"image": "`+newImage+`"`) || !strings.Contains(stdout, `"replaced": "images/generated-old00000.png"`) {
		t.Errorf("stdout = %s", stdout)
	}
	content, _ := os.ReadFile(deck)
	want := "# One\n\n---\n\n# Two\n\nBefore\n\n<!-- ai-prompt: a blue whale -->\n![](" + newImage + ")\n\nAfter\n"
	if string(content) != want {
		t.Errorf("deck = %q, want %q", content, want)
	}
	if _, err := os.Stat(filepath.Join(deckDir, "images", "generated-old00000.png")); !os.IsNotExist(err) {
		t.Error("the old image should be deleted")
	}
}

func TestImageRegenerateWithANewPrompt(t *testing.T) {
	fake := useFakeImageGenerator(t)
	deck := writeRegenerateDeck(t, t.TempDir())
	exitCode, _, stderr := runTap(t, "image", "regenerate", deck, "--slide", "2", "--image", "./images/generated-old00000.png", "--prompt", "a green whale")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if len(fake.prompts) != 1 || fake.prompts[0] != "a green whale" {
		t.Errorf("generator prompts = %q", fake.prompts)
	}
	content, _ := os.ReadFile(deck)
	if !strings.Contains(string(content), "<!-- ai-prompt: a green whale -->") {
		t.Errorf("deck = %q", content)
	}
}

func TestImageRegenerateAnImageThatIsNotOnTheSlide(t *testing.T) {
	useFakeImageGenerator(t)
	deck := writeRegenerateDeck(t, t.TempDir())
	exitCode, stdout, _ := runTap(t, "image", "regenerate", deck, "--slide", "1", "--image", "images/generated-old00000.png", "--json")
	if exitCode != exitUserError || !strings.Contains(stdout, `"code": "image_not_found"`) {
		t.Errorf("(%d, %q), want exit 1 and image_not_found", exitCode, stdout)
	}
	content, _ := os.ReadFile(deck)
	if string(content) != regenerateDeck {
		t.Errorf("deck changed to %q", content)
	}
}

func TestImageRegenerateNeedsSlideAndImage(t *testing.T) {
	useFakeImageGenerator(t)
	deck := writeRegenerateDeck(t, t.TempDir())
	for _, args := range [][]string{
		{"image", "regenerate", deck, "--image", "images/generated-old00000.png", "--json"},
		{"image", "regenerate", deck, "--slide", "2", "--json"},
	} {
		exitCode, stdout, _ := runTap(t, args...)
		if exitCode != exitUserError || !strings.Contains(stdout, `"code": "usage"`) {
			t.Errorf("%v: (%d, %q), want exit 1 and usage", args, exitCode, stdout)
		}
	}
}
```

Add to `internal/cli/tui_parity_test.go`:

```go
func TestImageRegenerateMatchesTheImageKey(t *testing.T) {
	useFakeImageGenerator(t)
	tuiDeck, commandDeck := twoDeckCopies(t, "talk.md", regenerateDeck)
	writeDeckFile(t, filepath.Dir(tuiDeck), "images/generated-old00000.png", "old image")
	writeDeckFile(t, filepath.Dir(commandDeck), "images/generated-old00000.png", "old image")

	// i opens the generator, Down and Enter pick slide 2, Down and Enter
	// pick "Regenerate" (the first option is "Add new image"), and Ctrl+D
	// submits the prompt the TUI fills in from the old image.
	model := tea.Model(tui.NewDevModel(tui.DevConfig{MarkdownFile: tuiDeck}))
	model, _ = pressKeys(model, runeKey("i"), tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyEnter},
		tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyEnter})
	model, submit := pressKeys(model, tea.KeyMsg{Type: tea.KeyCtrlD})
	deliverCommand(model, submit)

	if exitCode, _, stderr := runTap(t, "image", "regenerate", commandDeck, "--slide", "2", "--image", "images/generated-old00000.png"); exitCode != exitOK {
		t.Fatalf("tap image regenerate exited %d: %s", exitCode, stderr)
	}
	requireSameFolders(t, tuiDeck, commandDeck)
	if _, err := os.Stat(filepath.Join(filepath.Dir(tuiDeck), "images", "generated-old00000.png")); !os.IsNotExist(err) {
		t.Error("the TUI path did not regenerate: the old image is still there")
	}
}
```

Add `"os"` to the imports of `tui_parity_test.go`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestImageRegenerate' -short`
Expected: FAIL, `tap image regenerate` is an unknown command.

- [ ] **Step 3: Add `tap image regenerate` to `internal/cli/image.go`**

Flag variables:

```go
// Flags for tap image regenerate.
var (
	imageRegenerateSlide  int
	imageRegenerateImage  string
	imageRegeneratePrompt string
	imageRegenerateJSON   bool
)
```

Command:

```go
// imageRegenerateCmd makes an AI image again and replaces it in place.
var imageRegenerateCmd = &cobra.Command{
	Use:   "regenerate [deck]",
	Short: "Generate an AI image again and replace it in place",
	Long: `Generate an AI image on a slide again, as the i key in tap dev does,
and replace it where it is. The old image file is deleted.

--image names the image by the path the slide links to, for example
images/generated-1a2b3c4d.png. Without --prompt, tap reuses the prompt in
the image's ai-prompt comment.

Examples:
  tap image regenerate --slide 3 --image images/generated-1a2b3c4d.png
  tap image regenerate talk.md --slide 3 --image images/generated-1a2b3c4d.png --prompt "..."`,
	Args: cobra.MaximumNArgs(1),
	RunE: runImageRegenerate,
}
```

In `init`:

```go
	imageCmd.AddCommand(imageRegenerateCmd)
	imageRegenerateCmd.Flags().IntVar(&imageRegenerateSlide, "slide", 0, "slide the image is on, from 1 (required)")
	imageRegenerateCmd.Flags().StringVar(&imageRegenerateImage, "image", "", "path of the AI image to replace, as the slide links to it (required)")
	imageRegenerateCmd.Flags().StringVar(&imageRegeneratePrompt, "prompt", "", "a new prompt (default: the image's own prompt)")
	imageRegenerateCmd.Flags().BoolVar(&imageRegenerateJSON, "json", false, "print the result as JSON")
```

Run function and lookup:

```go
func runImageRegenerate(cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Changed("slide") {
		return userError(codeUsage, errors.New("--slide is required"))
	}
	if imageRegenerateImage == "" {
		return userError(codeUsage, errors.New("--image is required"))
	}
	deck, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
	slideIndex, err := slideIndexFromFlag(deck, imageRegenerateSlide)
	if err != nil {
		return err
	}
	replacing, err := findAIImage(deck, slideIndex, imageRegenerateImage)
	if err != nil {
		return err
	}
	prompt := replacing.Prompt
	if cmd.Flags().Changed("prompt") {
		prompt = strings.TrimSpace(imageRegeneratePrompt)
		if prompt == "" {
			return userError(codeUsage, errors.New("--prompt is empty"))
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	placed, err := generateAndPlace(ctx, deckedit.Placement{DeckPath: deck, SlideIndex: slideIndex, Prompt: prompt, Replacing: &replacing})
	if err != nil {
		return err
	}
	if placed.DeleteError != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %v\n", placed.DeleteError)
	}

	if imageRegenerateJSON {
		return printJSONOK(cmd.OutOrStdout(), generatedImageResult{
			Deck:     deck,
			Slide:    imageRegenerateSlide,
			Image:    filepath.ToSlash(placed.Path),
			Prompt:   prompt,
			Markdown: placed.Markdown,
			Replaced: filepath.ToSlash(replacing.ImagePath),
		})
	}
	fmt.Fprintln(cmd.OutOrStdout(), filepath.ToSlash(placed.Path))
	return nil
}

// findAIImage returns the AI image on the slide at slideIndex whose link
// is imagePath. Paths compare after cleaning, so ./images/a.png matches
// images/a.png.
func findAIImage(deck string, slideIndex int, imagePath string) (deckedit.AIImage, error) {
	content, err := os.ReadFile(deck)
	if err != nil {
		return deckedit.AIImage{}, userError(codeDeckNotFound, fmt.Errorf("cannot read %s: %w", deck, err))
	}
	images := deckedit.ParseAIImages(deckedit.SlideBodies(string(content))[slideIndex])
	paths := make([]string, 0, len(images))
	for _, image := range images {
		if filepath.Clean(image.ImagePath) == filepath.Clean(imagePath) {
			return image, nil
		}
		paths = append(paths, image.ImagePath)
	}
	onSlide := "it has none"
	if len(paths) > 0 {
		onSlide = "its AI images are " + strings.Join(paths, ", ")
	}
	return deckedit.AIImage{}, userError(codeImageNotFound, fmt.Errorf("slide %d has no AI image %s: %s", slideIndex+1, imagePath, onSlide))
}
```

- [ ] **Step 4: Add the command to `expectedCommands`**

In `internal/cli/conventions_test.go`, add `"tap image regenerate",` after `"tap image generate",`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestImage|TestCommandTree|TestEveryCommandFollows' -short -v 2>&1 | grep -E "^(--- FAIL|FAIL|ok)"`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/image.go internal/cli/image_test.go internal/cli/tui_parity_test.go internal/cli/conventions_test.go
git commit -m "feat(cli): add tap image regenerate, sharing the i key's code"
```

---

### Task 10: `tap theme show --image`

**Files:**
- Create: `internal/cli/theme_image.go`
- Create: `internal/cli/theme_image_test.go`
- Modify: `internal/cli/theme.go`

**Interfaces:**
- Consumes: P1's `runThemeShow`, `resolveThemeShowSlug`, `findTheme`, `prepareDeck`, `themeShowJSON`, `themeShowPrompt`; `pdf.New`, `(*pdf.Exporter).EnsureBrowser`, `(*pdf.Exporter).CaptureSlide`, `pdf.CaptureOptions`; `Version` (`internal/cli/root.go`)
- Produces:
  - flags `themeShowImage bool`, `themeShowOutput string` (`--output/-o`)
  - `const themeImageWidth = 1280`, `const themeImageHeight = 720`
  - `var themeImageCacheRoot func() (string, error)` (default `os.UserCacheDir`)
  - `var renderThemeImage func(ctx context.Context, theme themes.Theme, outputPath string) error` (default `renderThemeImageWithBrowser`)
  - `func themeImageCachePath(slug string) (string, error)`: `<cache root>/tap/themes/<Version>/<slug>.png`
  - `func showThemeImage(cmd *cobra.Command, theme themes.Theme) error`
  - `func themeImageDeck(theme themes.Theme) string`
  - `type themeImageResult struct { Slug, Image string; Cached bool }` (JSON `slug`, `image`, `cached`)

- [ ] **Step 1: Write the failing tests**

`internal/cli/theme_image_test.go`:

```go
package cli

import (
	"context"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/pdf"
	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// useFakeThemeRenderer replaces the browser renderer and the cache folder
// for the rest of the test, sets a release version, and returns the
// number of renders so far and the cache root.
func useFakeThemeRenderer(t *testing.T) (renders *int, cacheRoot string) {
	t.Helper()
	count := 0
	cacheRoot = t.TempDir()
	originalRender, originalRoot, originalVersion := renderThemeImage, themeImageCacheRoot, Version
	renderThemeImage = func(ctx context.Context, theme themes.Theme, outputPath string) error {
		count++
		return os.WriteFile(outputPath, []byte("png of "+theme.Slug), 0o644)
	}
	themeImageCacheRoot = func() (string, error) { return cacheRoot, nil }
	Version = "9.9.9-test"
	t.Cleanup(func() {
		renderThemeImage, themeImageCacheRoot, Version = originalRender, originalRoot, originalVersion
	})
	return &count, cacheRoot
}

func TestThemeShowImageRendersOnceThenUsesTheCache(t *testing.T) {
	renders, cacheRoot := useFakeThemeRenderer(t)
	wantPath := filepath.Join(cacheRoot, "tap", "themes", "9.9.9-test", "terminal.png")

	exitCode, stdout, stderr := runTap(t, "theme", "show", "terminal", "--image")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if stdout != wantPath+"\n" {
		t.Errorf("stdout = %q, want %q", stdout, wantPath)
	}
	content, err := os.ReadFile(wantPath)
	if err != nil || string(content) != "png of terminal" {
		t.Errorf("cached image = (%q, %v)", content, err)
	}

	exitCode, stdout, _ = runTap(t, "theme", "show", "terminal", "--image", "--json")
	if exitCode != exitOK {
		t.Fatalf("second run exit code = %d", exitCode)
	}
	var output struct {
		OK     bool   `json:"ok"`
		Slug   string `json:"slug"`
		Image  string `json:"image"`
		Cached bool   `json:"cached"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !output.OK || output.Slug != "terminal" || output.Image != wantPath || !output.Cached {
		t.Errorf("output = %+v", output)
	}
	if *renders != 1 {
		t.Errorf("rendered %d times, want 1", *renders)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(wantPath), "*.partial*"))
	if len(leftovers) > 0 {
		t.Errorf("partial files left in the cache: %v", leftovers)
	}
}

func TestThemeShowImageOutputCopiesTheImage(t *testing.T) {
	useFakeThemeRenderer(t)
	output := filepath.Join(t.TempDir(), "preview.png")
	exitCode, stdout, _ := runTap(t, "theme", "show", "terminal", "--image", "-o", output)
	if exitCode != exitOK || stdout != output+"\n" {
		t.Errorf("(%d, %q), want (0, %q)", exitCode, stdout, output)
	}
	content, err := os.ReadFile(output)
	if err != nil || string(content) != "png of terminal" {
		t.Errorf("output file = (%q, %v)", content, err)
	}
}

func TestThemeShowImageInADevBuildAlwaysRenders(t *testing.T) {
	renders, _ := useFakeThemeRenderer(t)
	Version = "dev"
	runTap(t, "theme", "show", "terminal", "--image")
	runTap(t, "theme", "show", "terminal", "--image")
	if *renders != 2 {
		t.Errorf("rendered %d times, want 2: a dev build must not trust the cache", *renders)
	}
}

func TestThemeShowImageNewVersionRendersAgain(t *testing.T) {
	renders, _ := useFakeThemeRenderer(t)
	runTap(t, "theme", "show", "terminal", "--image")
	Version = "9.9.10-test"
	runTap(t, "theme", "show", "terminal", "--image")
	if *renders != 2 {
		t.Errorf("rendered %d times, want 2", *renders)
	}
}

func TestThemeShowImageFlagErrors(t *testing.T) {
	useFakeThemeRenderer(t)
	tests := [][]string{
		{"theme", "show", "terminal", "--image", "--prompt"},
		{"theme", "show", "terminal", "-o", "x.png"},
	}
	for _, args := range tests {
		exitCode, _, stderr := runTap(t, args...)
		if exitCode != exitUserError || stderr == "" {
			t.Errorf("%v: (%d, %q), want exit 1 and a message", args, exitCode, stderr)
		}
	}
}

func TestThemeImageDeckIsATitleSlide(t *testing.T) {
	deck := themeImageDeck(themes.Theme{Slug: "terminal", Name: "Terminal", Pitch: "Green on black"})
	for _, want := range []string{"theme: terminal\n", "\n# Terminal\n", "\nGreen on black\n"} {
		if !strings.Contains(deck, want) {
			t.Errorf("deck %q is missing %q", deck, want)
		}
	}
}

func TestRenderThemeImageWithBrowser(t *testing.T) {
	if testing.Short() {
		t.Skip("drives a real browser")
	}
	exporter, err := pdf.New()
	if err != nil {
		t.Fatal(err)
	}
	requireBrowser(t, exporter)
	_ = exporter.Close()

	theme, _ := findTheme("terminal")
	output := filepath.Join(t.TempDir(), "terminal.png")
	if err := renderThemeImageWithBrowser(context.Background(), theme, output); err != nil {
		t.Fatalf("renderThemeImageWithBrowser() error = %v", err)
	}
	file, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	config, err := png.DecodeConfig(file)
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}
	if config.Width != themeImageWidth || config.Height != themeImageHeight {
		t.Errorf("size = %dx%d, want %dx%d", config.Width, config.Height, themeImageWidth, themeImageHeight)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestThemeShowImage|TestThemeImageDeck' -short`
Expected: compile failure, `undefined: renderThemeImage`.

- [ ] **Step 3: Write `internal/cli/theme_image.go`**

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/pdf"
	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// The size of a theme preview image, 16:9.
const (
	themeImageWidth  = 1280
	themeImageHeight = 720
)

// themeImageCacheRoot is the folder the theme image cache lives under.
// Tests point it at a temporary folder.
var themeImageCacheRoot = os.UserCacheDir

// renderThemeImage writes a PNG of a title slide in theme to outputPath.
// Tests replace it, so only one test drives a browser.
var renderThemeImage = renderThemeImageWithBrowser

// themeImageResult is the --json result of tap theme show --image.
type themeImageResult struct {
	Slug   string `json:"slug"`
	Image  string `json:"image"`
	Cached bool   `json:"cached"`
}

// themeImageCachePath is where the preview of slug is cached for this tap
// version: <user cache folder>/tap/themes/<version>/<slug>.png. A new tap
// version gets a new folder, so a changed theme is never served stale.
func themeImageCachePath(slug string) (string, error) {
	root, err := themeImageCacheRoot()
	if err != nil {
		return "", internalError(codeInternal, fmt.Errorf("no cache folder for theme images: %w", err))
	}
	return filepath.Join(root, "tap", "themes", Version, slug+".png"), nil
}

// showThemeImage prints the path of a preview image of theme, rendering it
// into the cache first when the cache has none. A dev build always
// renders, because its version does not change when the frontend does.
// With --output, the image is copied there and that path is printed.
func showThemeImage(cmd *cobra.Command, theme themes.Theme) error {
	cachePath, err := themeImageCachePath(theme.Slug)
	if err != nil {
		return err
	}

	cached := false
	if Version != "dev" {
		if info, err := os.Stat(cachePath); err == nil && info.Mode().IsRegular() {
			cached = true
		}
	}
	if !cached {
		if err := renderIntoCache(theme, cachePath); err != nil {
			return err
		}
	}

	image := cachePath
	if themeShowOutput != "" {
		content, err := os.ReadFile(cachePath)
		if err != nil {
			return internalError(codeInternal, fmt.Errorf("reading the cached theme image: %w", err))
		}
		if err := os.WriteFile(themeShowOutput, content, 0o644); err != nil {
			return userError(codeFailed, fmt.Errorf("cannot write %s: %w", themeShowOutput, err))
		}
		image = themeShowOutput
	}

	if themeShowJSON {
		return printJSONOK(cmd.OutOrStdout(), themeImageResult{Slug: theme.Slug, Image: image, Cached: cached})
	}
	fmt.Fprintln(cmd.OutOrStdout(), image)
	return nil
}

// renderIntoCache renders theme into a temporary file next to cachePath
// and moves it into place, so a cancelled render never leaves a partial
// image in the cache.
func renderIntoCache(theme themes.Theme, cachePath string) error {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return internalError(codeInternal, fmt.Errorf("creating the theme image cache: %w", err))
	}
	temporary, err := os.CreateTemp(filepath.Dir(cachePath), theme.Slug+".partial*.png")
	if err != nil {
		return internalError(codeInternal, fmt.Errorf("creating the theme image cache: %w", err))
	}
	temporaryPath := temporary.Name()
	_ = temporary.Close()
	defer os.Remove(temporaryPath)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := renderThemeImage(ctx, theme, temporaryPath); err != nil {
		if ctx.Err() != nil {
			return errInterrupted
		}
		return err
	}
	if err := os.Rename(temporaryPath, cachePath); err != nil {
		return internalError(codeInternal, fmt.Errorf("saving the theme image: %w", err))
	}
	return nil
}

// themeImageDeck is a one-slide deck that shows theme on a title slide:
// the theme's name as the heading and its pitch as the subtitle.
func themeImageDeck(theme themes.Theme) string {
	return fmt.Sprintf("---\ntitle: %q\ntheme: %s\n---\n\n# %s\n\n%s\n", theme.Name, theme.Slug, theme.Name, theme.Pitch)
}

// renderThemeImageWithBrowser renders themeImageDeck in the headless
// browser, through the same temporary server and capture as tap export
// images, and writes the PNG to outputPath.
func renderThemeImageWithBrowser(ctx context.Context, theme themes.Theme, outputPath string) error {
	folder, err := os.MkdirTemp("", "tap-theme-image-*")
	if err != nil {
		return internalError(codeInternal, err)
	}
	defer os.RemoveAll(folder)

	deck := filepath.Join(folder, "theme.md")
	if err := os.WriteFile(deck, []byte(themeImageDeck(theme)), 0o644); err != nil {
		return internalError(codeInternal, err)
	}
	cfg, err := config.Load(deck)
	if err != nil {
		return internalError(codeInternal, fmt.Errorf("loading the theme deck: %w", err))
	}
	srv, _, _, buildErrors, _, err := prepareDeck(deck, cfg, folder)
	if err != nil {
		return internalError(codeInternal, fmt.Errorf("serving the theme deck: %w", err))
	}
	if len(buildErrors) > 0 || srv == nil {
		return internalError(codeInternal, fmt.Errorf("the theme deck did not build"))
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownContext)
	}()

	exporter, err := pdf.New()
	if err != nil {
		return internalError(codeBrowser, fmt.Errorf("failed to create browser exporter: %w", err))
	}
	defer func() { _ = exporter.Close() }()
	if err := exporter.EnsureBrowser(); err != nil {
		return internalError(codeBrowser, fmt.Errorf("failed to start browser: %w", err))
	}

	options := pdf.CaptureOptions{SlideNumber: 1, Width: themeImageWidth, Height: themeImageHeight, Print: true}
	if err := exporter.CaptureSlide(ctx, fmt.Sprintf("http://localhost:%d", srv.Port()), options, outputPath); err != nil {
		return internalError(codeExportFailed, fmt.Errorf("rendering the theme image: %w", err))
	}
	return nil
}
```

- [ ] **Step 4: Add the flags to `tap theme show` in `internal/cli/theme.go`**

1. Add `themeShowImage bool` and `themeShowOutput string` to the flag `var` block.
2. In `init`, after the `theme show` flags:

```go
	themeShowCmd.Flags().BoolVar(&themeShowImage, "image", false, "render a title slide in the theme to a PNG and print its path")
	themeShowCmd.Flags().StringVarP(&themeShowOutput, "output", "o", "", "copy the --image PNG to this file")
```

3. At the top of `runThemeShow`, after the `--json`/`--prompt` check:

```go
	if themeShowImage && themeShowPrompt {
		return userError(codeUsage, errors.New("--image and --prompt cannot be used together"))
	}
	if themeShowOutput != "" && !themeShowImage {
		return userError(codeUsage, errors.New("--output needs --image"))
	}
```

4. In `runThemeShow`, right after the `findTheme` check returns `theme`:

```go
	if themeShowImage {
		return showThemeImage(cmd, theme)
	}
```

5. In the `Long` text of `theme show`, add this paragraph before `Examples:`, and add the two example lines:

```
With --image, tap renders a title slide in the theme to a 1280x720 PNG
and prints its path. The image is cached per theme and tap version in the
user cache folder, under tap/themes/<version>/<slug>.png, so the second
call returns at once. -o copies it to a file.
```

```
  tap theme show terminal --image
  tap theme show terminal --image -o terminal.png
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestThemeShowImage|TestThemeImageDeck|TestTheme|TestCommandTree|TestEveryCommandFollows' -short -v 2>&1 | grep -E "^(--- FAIL|FAIL|ok)"`
Expected: `ok`.

Run: `go test ./internal/cli -run TestRenderThemeImageWithBrowser -v`
Expected: PASS where Playwright's Chromium is installed, SKIP otherwise (and FAIL in CI without a browser, as `requireBrowser` does for the other browser tests).

- [ ] **Step 6: Try it by hand**

```bash
go build -o /tmp/tap-p4 ./cmd/tap
/tmp/tap-p4 theme show terminal --image -o /tmp/terminal.png && open /tmp/terminal.png
```

Expected: a 1280x720 title slide reading "Terminal" in the terminal theme. `tap` is a dev build here, so it renders every time.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/theme_image.go internal/cli/theme_image_test.go internal/cli/theme.go
git commit -m "feat(cli): add tap theme show --image with a per-version cache"
```

---

### Task 11: Docs, skill and changelog

**Files:**
- Modify: `docs/reference/cli-commands.md`, `skills/tap/rules/cli.md`
- Modify: `docs/guide/ai-images.md`, `skills/tap/rules/ai-images.md`
- Modify: `docs/guide/themes.md`, `skills/tap/rules/themes.md`
- Modify: `docs/guide/layouts.md`
- Modify: `CHANGELOG.md`, `docs/changelog.md`

Do not edit `docs/superpowers/**` or released sections of the changelogs.

- [ ] **Step 1: Add the commands to `docs/reference/cli-commands.md` and `skills/tap/rules/cli.md`**

P1 put the commands in this order: `new`, `dev`, `present`, `build`, `serve`, `export pdf`, `export images`, `slide add`, `component new`, `theme list`, `theme show`. Add `image add`, `image generate` and `image regenerate` after `component new`, and `theme set` after `theme show`. In `slide add` and `theme show`, add the new flags. Each command section in `cli-commands.md` has Usage, Flags, Examples and `--json` result fields, as P1 wrote them. Use this content, in the style of the surrounding sections. `skills/tap/rules/cli.md` gets the same facts, shorter.

`tap slide add`, new flags:

| Flag | Meaning |
|---|---|
| `--layout <name>` | Append that layout's template without the wizard. Works without a terminal. |
| `--print` | With `--layout`, print the template and write nothing. The template has no `---` in front. No deck needed. |
| `--json` | With `--layout`: `{"ok": true, "deck": "...", "layout": "...", "markdown": "..."}`. With `--print`, `deck` is left out. |

Say that the wizard and `--layout` offer all 12 layouts, and list them.

`tap image add <file> [deck]`:

- Copies the file into `images/` next to the deck, keeping its name, and as `name-2.png`, `name-3.png` when the name is taken. Prints the markdown, for example `![diagram](images/diagram.png)`.
- `--slide N` also adds the markdown at the end of slide N.
- Accepts png, jpg, jpeg, gif, webp, svg and avif.
- `--json`: `{"ok": true, "deck", "image", "markdown", "slide"}`; `slide` only with `--slide`.
- Error codes: `file_not_found`, `not_an_image`, `out_of_range`.

`tap image generate [deck] --slide N --prompt "..."`:

- The `i` key's generator as a command. Saves `images/generated-<hash>.<ext>` and adds the image with its `<!-- ai-prompt: ... -->` comment at the end of slide N. Prints the image path.
- Needs `GEMINI_API_KEY`, in the environment or in a `.env` file next to the deck.
- `--json`: `{"ok": true, "deck", "slide", "image", "prompt", "markdown"}`.
- Error codes: `usage`, `out_of_range`, `no_api_key`, `image_generation` (exit 2 for a network or server failure, 1 otherwise).

`tap image regenerate [deck] --slide N --image <path> [--prompt "..."]`:

- Makes the AI image at `<path>` on slide N again and replaces it where it is. The old file is deleted. Without `--prompt`, it reuses the image's own prompt.
- `--json`: as `image generate`, plus `"replaced": "<old path>"`.
- Error codes: as `image generate`, plus `image_not_found`.

`tap theme show`, new flags:

| Flag | Meaning |
|---|---|
| `--image` | Render a title slide in the theme to a 1280x720 PNG and print its path. |
| `--output`, `-o` | With `--image`, copy the PNG to this file and print that path. |

Say that the image is cached per theme and tap version, in the user cache folder under `tap/themes/<version>/<slug>.png` (`~/Library/Caches/tap/themes/...` on macOS), and that `--json` gives `{"ok": true, "slug", "image", "cached"}`.

`tap theme set <slug> [deck]`:

- Writes `theme: <slug>` in the frontmatter, the same change as the `t` key in `tap dev`. A deck with no frontmatter gets one.
- An unknown slug exits 1 with code `unknown_theme` and the list of themes.
- `--json`: `{"ok": true, "deck", "theme"}`.

- [ ] **Step 2: Update the guides**

- `docs/guide/ai-images.md` and `skills/tap/rules/ai-images.md`: add a section "From the command line" after "Using the Image Generator" (docs) or "Usage" (skill), with one `tap image generate` and one `tap image regenerate` example and one sentence each. Say that they write the same files as the `i` key. In "Regenerating Images", add: "or run `tap image regenerate --slide N --image <path>`".
- `docs/guide/themes.md` and `skills/tap/rules/themes.md`: in "Setting a Theme", add `tap theme set <slug>` as the command-line way. In `docs/guide/themes.md`, "Reading a Theme From the Command Line", add one paragraph on `tap theme show <slug> --image`.
- `docs/guide/layouts.md`: add one paragraph near the top: `tap slide add --layout <name>` appends a slide in any layout, and `--print` shows its template.

- [ ] **Step 3: Add the changelog entries**

In `CHANGELOG.md` and `docs/changelog.md`, under `## [Unreleased]`, `### Added` (match each file's style):

```markdown
- **`tap theme set <slug> [deck]`** - Writes `theme:` in the deck's frontmatter, the same change the `t` key makes in `tap dev`. An unknown slug exits 1 with the list of themes.
- **`tap theme show <slug> --image`** - Renders a title slide in a theme to a PNG, for theme pickers. The image is cached per theme and tap version, so the second call returns at once. `-o` copies it to a file.
- **`tap image add <file> [deck]`** - Copies an image into `images/` next to the deck, as `name-2.png` when the name is taken, and prints the markdown that shows it. `--slide N` also adds it to the end of slide N.
- **`tap image generate` and `tap image regenerate`** - The `i` key's AI image generator as commands. `generate --slide N --prompt "..."` adds a new image to a slide. `regenerate --slide N --image <path>` makes an image again in place, with its own prompt or a new one, and deletes the old file. Both write exactly what the `i` key writes.
- **`tap slide add --layout <name>`** - Appends a slide in any of the 12 layouts without the wizard, and without a terminal. `--print` prints the template and writes nothing. The wizard now offers all 12 layouts, up from 7.
```

Under `### Fixed`:

```markdown
- **Regenerating an AI image keeps the new file** - When the new image had the same bytes as the old one, it got the same file name, and deleting the old file deleted the new one. The old file is now kept in that case. A regenerate prompt with a `$` in it is also written as typed.
```

- [ ] **Step 4: Check the docs build**

Run: `cd docs && npm run build`
Expected: the build succeeds with no broken links. Skip it if `docs/node_modules` is missing, and say so in the PR.

Run: `grep -rn "—" docs/reference/cli-commands.md docs/guide/ai-images.md docs/guide/themes.md docs/guide/layouts.md skills/tap/rules CHANGELOG.md docs/changelog.md internal/deckedit internal/layouts/templates.go internal/cli/image.go internal/cli/theme_set.go internal/cli/theme_image.go`
Expected: no hits in lines this plan added.

- [ ] **Step 5: Commit**

```bash
git add docs skills CHANGELOG.md
git commit -m "docs: describe the image, theme set, theme image and slide layout commands"
```

---

## Final check

- [ ] Run `go test ./...` (not `-short`, so the browser test in Task 10 runs).
- [ ] Run `go vet ./...` and `make lint` if the Makefile has it.
- [ ] Run `grep -rn "UpdateThemeInFile\|appendToFile\|insertImageIntoSlide\|replaceImageInContent" internal/tui internal/cli`. Expected: no output. Every file change the TUI keys make goes through `internal/deckedit` or `internal/layouts`.
- [ ] Build the binary and try each command by hand in a folder with one deck, checking `echo $?` after each:
  - `tap theme set midnight`, then `tap theme set nope` (exit 1, lists themes).
  - `tap theme show terminal --image --json` twice; the second says `"cached": true` in a release build (`go build -ldflags "-X github.com/MiniCodeMonkey/tap/internal/cli.Version=9.9.9" ./cmd/tap`).
  - `tap slide add --layout split-media`, `tap slide add --layout big-stat --print`.
  - `tap image add ~/Desktop/some.png --slide 1`, twice (the second copy is `some-2.png`).
  - With `GEMINI_API_KEY` set: `tap image generate --slide 1 --prompt "a lighthouse, flat vector"`, then `tap image regenerate --slide 1 --image <the printed path>`.
  - In `tap dev`: `t` changes the theme, `a` offers 12 layouts, `i` still generates and regenerates.
- [ ] Open the PR against `main` with the parity tests named in the description, and the open questions from the top of this plan for the reviewer.

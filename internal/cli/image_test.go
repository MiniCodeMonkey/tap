package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/deckedit"
	"github.com/MiniCodeMonkey/tap/internal/gemini"
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
	// Each case runs in its own subtest so runTap's flag reset, which
	// fires on subtest cleanup, happens between cases; otherwise a flag
	// set in one case would leak into the next.
	for _, args := range [][]string{
		{"image", "regenerate", deck, "--image", "images/generated-old00000.png", "--json"},
		{"image", "regenerate", deck, "--slide", "2", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			exitCode, stdout, _ := runTap(t, args...)
			if exitCode != exitUserError || !strings.Contains(stdout, `"code": "usage"`) {
				t.Errorf("%v: (%d, %q), want exit 1 and usage", args, exitCode, stdout)
			}
		})
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

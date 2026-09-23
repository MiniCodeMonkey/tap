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

func TestBuildFlagsANonBooleanSkipValue(t *testing.T) {
	result, err := Build([]byte("<!-- skip: yes -->\n\n# One"), t.TempDir())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	slide := result.Slides[0]
	if slide.Skip {
		t.Error("Skip = true, want false: skip: yes must not skip the slide")
	}
	if errors := strings.Join(slide.Errors, "\n"); !strings.Contains(errors, "skip") {
		t.Errorf("slide errors = %q, want a warning naming the skip directive", errors)
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

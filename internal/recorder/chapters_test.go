package recorder

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSlideTitleUsesTheFirstHeading(t *testing.T) {
	content := "<!-- layout: title -->\n\n# Why geocoding is hard\n\nSome body text.\n"

	if got := SlideTitle(content, 0); got != "Why geocoding is hard" {
		t.Errorf("SlideTitle() = %q, want the heading", got)
	}
}

func TestSlideTitleStripsMarkdownEmphasis(t *testing.T) {
	if got := SlideTitle("## The **naive** approach\n", 1); got != "The naive approach" {
		t.Errorf("SlideTitle() = %q, want the emphasis stripped", got)
	}
}

func TestSlideTitleFallsBackToTheSlideNumber(t *testing.T) {
	if got := SlideTitle("![](diagram.png)\n", 4); got != "Slide 5" {
		t.Errorf("SlideTitle() = %q, want Slide 5", got)
	}
}

func TestSlideTitleIgnoresSlotMarkersAndComments(t *testing.T) {
	content := "::left\n<!-- pause -->\n\n# Real title\n"

	if got := SlideTitle(content, 0); got != "Real title" {
		t.Errorf("SlideTitle() = %q, want Real title", got)
	}
}

func TestSlideTitleIgnoresHeadingsInsideCodeFences(t *testing.T) {
	content := "```bash\n# set the flag\necho hi\n```\n\n# The real title\n"

	if got := SlideTitle(content, 0); got != "The real title" {
		t.Errorf("SlideTitle() = %q, want the heading outside the fence", got)
	}
}

func TestSlideTitleFallsBackWhenOnlyCodeCommentsLookLikeHeadings(t *testing.T) {
	content := "```python\n# not a heading\nprint(1)\n```\n"

	if got := SlideTitle(content, 2); got != "Slide 3" {
		t.Errorf("SlideTitle() = %q, want Slide 3", got)
	}
}

func TestSlideTitleKeepsUnderscoresInIdentifiers(t *testing.T) {
	if got := SlideTitle("# Understanding get_user_by_id\n", 0); got != "Understanding get_user_by_id" {
		t.Errorf("SlideTitle() = %q, want the identifier intact", got)
	}
}

func TestSlideTitleStripsInlineCodeMarkers(t *testing.T) {
	if got := SlideTitle("# The `geocode` call\n", 0); got != "The geocode call" {
		t.Errorf("SlideTitle() = %q, want the backticks removed", got)
	}
}

func TestChaptersRenderAsAYouTubeList(t *testing.T) {
	start := time.Date(2026, 9, 20, 14, 32, 0, 0, time.UTC)
	chapters := NewChapters(start)

	chapters.Add(start, 0, "Why geocoding is hard")
	chapters.Add(start.Add(107*time.Second), 1, "The naive approach")
	chapters.Add(start.Add(3732*time.Second), 2, "What we shipped")

	want := "0:00 Why geocoding is hard\n1:47 The naive approach\n1:02:12 What we shipped\n"
	if got := chapters.Render(); got != want {
		t.Errorf("Render() =\n%q\nwant\n%q", got, want)
	}
}

func TestChaptersAlwaysStartAtZero(t *testing.T) {
	start := time.Date(2026, 9, 20, 14, 32, 0, 0, time.UTC)
	chapters := NewChapters(start)

	// The first entry arrives late, because recording started on slide 5
	// and the hub reported the current slide a moment afterwards.
	chapters.Add(start.Add(3*time.Second), 4, "Live demo")

	want := "0:00 Live demo\n"
	if got := chapters.Render(); got != want {
		t.Errorf("Render() = %q, want the first entry pinned to 0:00", got)
	}
}

func TestChaptersSkipAnImmediateRepeat(t *testing.T) {
	start := time.Now()
	chapters := NewChapters(start)

	chapters.Add(start, 0, "Intro")
	chapters.Add(start.Add(time.Second), 0, "Intro")
	chapters.Add(start.Add(2*time.Second), 1, "Next")

	if got := chapters.Render(); got != "0:00 Intro\n0:02 Next\n" {
		t.Errorf("Render() = %q, want the repeat skipped", got)
	}
}

func TestChaptersKeepARevisit(t *testing.T) {
	start := time.Now()
	chapters := NewChapters(start)

	chapters.Add(start, 0, "Intro")
	chapters.Add(start.Add(time.Second), 1, "Demo")
	chapters.Add(start.Add(2*time.Second), 0, "Intro")

	want := "0:00 Intro\n0:01 Demo\n0:02 Intro\n"
	if got := chapters.Render(); got != want {
		t.Errorf("Render() = %q, want the revisit kept", got)
	}
}

func TestChaptersNeverGoBackwards(t *testing.T) {
	start := time.Now()
	chapters := NewChapters(start)

	chapters.Add(start.Add(5*time.Second), 0, "Intro")
	chapters.Add(start.Add(2*time.Second), 1, "Out of order")

	// A timestamp earlier than the previous entry would make the file
	// invalid as a chapter list, so it is clamped rather than dropped.
	if got := chapters.Render(); got != "0:00 Intro\n0:00 Out of order\n" {
		t.Errorf("Render() = %q, want non-decreasing timestamps", got)
	}
}

func TestChaptersWriteToAFile(t *testing.T) {
	start := time.Now()
	chapters := NewChapters(start)
	chapters.Add(start, 0, "Intro")

	path := filepath.Join(t.TempDir(), "talk.txt")
	if err := chapters.WriteTo(path); err != nil {
		t.Fatalf("WriteTo() returned %v", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != "0:00 Intro\n" {
		t.Errorf("the file holds %q", written)
	}
}

func TestEmptyChaptersAreEmpty(t *testing.T) {
	if !NewChapters(time.Now()).Empty() {
		t.Error("a chapter list with no entries does not report as empty")
	}
}

func TestChaptersRenderTheMarkBeforeTheSlideItShares(t *testing.T) {
	start := time.Date(2026, 9, 21, 19, 32, 0, 0, time.UTC)
	chapters := NewChapters(start)

	chapters.Add(start, 0, "Title")
	chapters.Add(start.Add(47*time.Second), 1, "Agenda")
	chapters.SetMark(start.Add(47*time.Second), "Talk starts")

	want := "0:00 Title\n0:47 Talk starts\n0:47 Agenda\n"
	if got := chapters.Render(); got != want {
		t.Errorf("Render() =\n%q\nwant\n%q", got, want)
	}
}

func TestChaptersRenderAMarkAfterTheLastSlide(t *testing.T) {
	start := time.Date(2026, 9, 21, 19, 32, 0, 0, time.UTC)
	chapters := NewChapters(start)

	chapters.Add(start, 0, "Title")
	chapters.SetMark(start.Add(time.Minute), "Talk starts")

	want := "0:00 Title\n1:00 Talk starts\n"
	if got := chapters.Render(); got != want {
		t.Errorf("Render() =\n%q\nwant\n%q", got, want)
	}
}

func TestChaptersClearMark(t *testing.T) {
	start := time.Date(2026, 9, 21, 19, 32, 0, 0, time.UTC)
	chapters := NewChapters(start)

	chapters.Add(start, 0, "Title")
	chapters.SetMark(start.Add(time.Minute), "Talk starts")
	chapters.ClearMark()

	if got := chapters.Render(); got != "0:00 Title\n" {
		t.Errorf("Render() = %q, want only the title", got)
	}
}

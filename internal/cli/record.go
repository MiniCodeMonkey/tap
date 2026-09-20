package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
)

// testCaptureSeconds is how long the picker's test capture runs: long
// enough to hear a sentence, short enough that nobody minds waiting.
const testCaptureSeconds = 5

// recordControllerOptions is everything the controller needs from the deck
// and the dev server.
type recordControllerOptions struct {
	// DeckTitle names the output file.
	DeckTitle string
	// OutputDir is where recordings are written.
	OutputDir string
	// AudioUID selects a microphone by CoreAudio UID. Empty uses the
	// system default input.
	AudioUID string
	// CommandName overrides the recorder binary. Tests use it; production
	// leaves it empty.
	CommandName string
	// CurrentSlide reports the slide the deck is on, for the first chapter.
	CurrentSlide func() (int, bool)
	// TitleFor names a slide for the chapter list.
	TitleFor func(slideIndex int) string
	// OpenFile shows a finished test capture to the speaker.
	OpenFile func(path string) error
	// NoAudio records silently.
	NoAudio bool
	// ShowClicks draws mouse clicks.
	ShowClicks bool
	// Chapters writes the sidecar chapter list.
	Chapters bool
}

// recordController is the dev server's recording, from the TUI's side of
// the interface. It owns the session, the chapter list and the paths.
type recordController struct {
	options recordControllerOptions

	mu       sync.Mutex
	session  *recorder.Session
	chapters *recorder.Chapters
}

// newRecordController builds the controller the TUI drives.
func newRecordController(options recordControllerOptions) *recordController {
	if options.OpenFile == nil {
		options.OpenFile = openInDefaultApplication
	}
	return &recordController{options: options}
}

// Available reports whether this platform can record at all.
func (c *recordController) Available() bool { return recorder.Supported() }

// Displays lists the screens that can be recorded.
func (c *recordController) Displays() ([]recorder.Display, error) { return recorder.Displays() }

// DefaultAudioInput names the microphone a recording will use.
func (c *recordController) DefaultAudioInput() string { return recorder.DefaultAudioInput() }

// Preflight checks everything that can stop a recording.
func (c *recordController) Preflight() recorder.Report {
	return recorder.Preflight(c.options.OutputDir, c.recorderOptions("", 0, 0))
}

// Recording reports whether a recording is running.
func (c *recordController) Recording() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session != nil
}

// Elapsed is how long the running recording has been going.
func (c *recordController) Elapsed() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session == nil {
		return 0
	}
	return c.session.Elapsed()
}

// Start begins recording the given display and returns the output path.
func (c *recordController) Start(display int) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session != nil {
		return c.session.Path(), nil
	}

	startedAt := time.Now()
	path, err := recorder.OutputPath(c.options.OutputDir, c.options.DeckTitle, startedAt)
	if err != nil {
		return "", err
	}

	session, err := recorder.Start(c.recorderOptions(path, display, 0))
	if err != nil {
		return "", err
	}
	c.session = session

	if c.options.Chapters {
		c.chapters = recorder.NewChapters(startedAt)
		// Whatever is on screen when recording starts is the first
		// chapter, so a recording begun mid-deck still labels its opening.
		if slideIndex, known := c.currentSlide(); known {
			c.chapters.Add(startedAt, slideIndex, c.titleFor(slideIndex))
		}
	}

	return path, nil
}

// Stop ends the recording and writes the chapter list.
func (c *recordController) Stop() (recorder.Result, error) {
	c.mu.Lock()
	session, chapters := c.session, c.chapters
	c.session, c.chapters = nil, nil
	c.mu.Unlock()

	if session == nil {
		return recorder.Result{}, nil
	}

	result, err := session.Stop()
	if err != nil {
		return result, err
	}

	if chapters != nil && !chapters.Empty() {
		chapterPath := recorder.ChapterPath(result.Path)
		if writeErr := chapters.WriteTo(chapterPath); writeErr != nil {
			return result, fmt.Errorf("writing the chapter list: %w", writeErr)
		}
		result.ChapterPath = chapterPath
	}

	return result, nil
}

// Test records a few seconds with the current settings and opens it, so the
// speaker can see the screen and hear the microphone before it matters. It
// writes to the temp directory: a test is not a recording of the talk.
func (c *recordController) Test(display int) error {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("tap-test-capture-%d.mov", time.Now().UnixNano()))

	session, err := recorder.Start(c.recorderOptions(path, display, testCaptureSeconds))
	if err != nil {
		return err
	}

	select {
	case <-session.Done():
	case <-time.After(time.Duration(testCaptureSeconds+10) * time.Second):
		if _, stopErr := session.Stop(); stopErr != nil {
			return stopErr
		}
	}

	return c.options.OpenFile(path)
}

// NoteSlideChange records that the deck moved, for the chapter list. It is
// called for every slide change from every client, including while nothing
// is being recorded, so it does nothing unless a list is open.
func (c *recordController) NoteSlideChange(slideIndex int) {
	c.mu.Lock()
	chapters := c.chapters
	c.mu.Unlock()

	if chapters == nil {
		return
	}
	chapters.Add(time.Now(), slideIndex, c.titleFor(slideIndex))
}

// recorderOptions builds the options for one capture.
func (c *recordController) recorderOptions(path string, display, limitSeconds int) recorder.Options {
	return recorder.Options{
		Command:      c.options.CommandName,
		OutputPath:   path,
		AudioUID:     c.options.AudioUID,
		Display:      display,
		LimitSeconds: limitSeconds,
		NoAudio:      c.options.NoAudio,
		ShowClicks:   c.options.ShowClicks,
	}
}

// currentSlide reports the slide the deck is on, when the hub knows it.
func (c *recordController) currentSlide() (int, bool) {
	if c.options.CurrentSlide == nil {
		return 0, false
	}
	return c.options.CurrentSlide()
}

// titleFor names a slide for the chapter list.
func (c *recordController) titleFor(slideIndex int) string {
	if c.options.TitleFor == nil {
		return fmt.Sprintf("Slide %d", slideIndex+1)
	}
	return c.options.TitleFor(slideIndex)
}

// openInDefaultApplication shows a file to the speaker. A .mov opens in
// QuickTime Player, which is where a test capture is watched and heard.
func openInDefaultApplication(path string) error {
	return exec.Command("open", path).Start()
}

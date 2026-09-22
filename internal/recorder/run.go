package recorder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// talkStartTitle labels the last time the deck left its first slide.
const talkStartTitle = "Talk starts"

// RunOptions configures one tap present run.
type RunOptions struct {
	// Parent is the recordings directory. The run folder is created
	// inside it when the first segment starts.
	Parent    string
	DeckTitle string
	// Base carries the recorder settings every segment shares. Its
	// OutputPath and Display are filled in per segment.
	Base     Options
	Chapters bool
	// CurrentSlide seeds each segment's first chapter and the slide the
	// run starts on.
	CurrentSlide func() (int, bool)
	TitleFor     func(slideIndex int) string
	// Now is the clock. Nil means time.Now.
	Now func() time.Time
	// OnSegmentExit is called when a segment's recorder stops without
	// being asked to, for example when its display is unplugged. ran is
	// how long the segment recorded.
	OnSegmentExit func(err error, ran time.Duration)
}

// RunSummary describes a finished run.
type RunSummary struct {
	Dir                 string
	Segments            int
	Deleted             bool
	NeverLeftFirstSlide bool
	Truncated           bool
}

type segment struct {
	name     string
	session  *Session
	chapters *Chapters
}

// Run is one tap present run: a folder of numbered segments and one
// chapter list across them.
type Run struct {
	options RunOptions

	mu             sync.Mutex
	dir            string
	segments       []*segment
	current        *segment
	lastSlide      int
	slideKnown     bool
	leftFirstSlide bool
	truncated      bool
	finished       bool
	summary        RunSummary
}

// NewRun prepares a run. Nothing is written until StartSegment.
func NewRun(options RunOptions) *Run {
	if options.Now == nil {
		options.Now = time.Now
	}
	run := &Run{options: options}
	if options.CurrentSlide != nil {
		run.lastSlide, run.slideKnown = options.CurrentSlide()
		run.leftFirstSlide = run.slideKnown && run.lastSlide != 0
	}
	return run
}

// StartSegment starts a new segment on display and then stops the current
// one, so the audio gap between them is as short as possible.
// StartSegment stops the previous segment only after releasing r.mu, since
// Session.Stop can take up to killGrace and every other Run method,
// including a poller reading Recording or SegmentElapsed once a second,
// would otherwise stall behind it. r.current already points at the new
// segment by the time the previous one is stopped, so watchSegment still
// sees a requested stop rather than reporting it as unexpected.
func (r *Run) StartSegment(display int) (string, error) {
	r.mu.Lock()

	if r.finished {
		r.mu.Unlock()
		return "", errors.New("the run has finished")
	}
	if r.dir == "" {
		dir, err := RunDir(r.options.Parent, r.options.DeckTitle, r.options.Now())
		if err != nil {
			r.mu.Unlock()
			return "", err
		}
		r.dir = dir
	}

	name := fmt.Sprintf("%02d.mov", len(r.segments)+1)
	options := r.options.Base
	options.OutputPath = filepath.Join(r.dir, name)
	options.Display = display

	session, err := Start(options)
	if err != nil {
		r.mu.Unlock()
		return "", err
	}
	startedAt := r.options.Now()

	// A segment's first chapter is the slide the deck is on when it
	// starts. A freshly opened deck starts on its title slide, and the
	// title slide never broadcasts a slide change on its own, so an
	// unknown slide is seeded as slide 0 rather than left without a
	// chapter.
	next := &segment{name: name, session: session, chapters: NewChapters(startedAt)}
	slideIndex, _ := r.currentSlide()
	next.chapters.Add(startedAt, slideIndex, r.titleFor(slideIndex))

	previous := r.current
	r.segments = append(r.segments, next)
	r.current = next
	r.writeChaptersLocked()
	r.mu.Unlock()

	go r.watchSegment(next)

	if previous != nil {
		_ = r.stopAndRecordTruncated(previous)
	}
	return options.OutputPath, nil
}

// StopSegment stops the current segment, if any, without holding r.mu while
// it does: see StartSegment.
func (r *Run) StopSegment() error {
	r.mu.Lock()
	if r.current == nil {
		r.mu.Unlock()
		return nil
	}
	previous := r.current
	r.current = nil
	r.mu.Unlock()

	return r.stopAndRecordTruncated(previous)
}

// stopAndRecordTruncated stops a segment that is no longer current. It is
// called without r.mu held, so a slow Session.Stop (up to killGrace) never
// blocks another Run method; it takes the lock only briefly afterward to
// record whether the recorder had to be killed.
func (r *Run) stopAndRecordTruncated(stopping *segment) error {
	result, err := stopping.session.Stop()
	if result.Truncated {
		r.mu.Lock()
		r.truncated = true
		r.mu.Unlock()
	}
	return err
}

// watchSegment reports a segment whose recorder exits while it is still
// the current one.
func (r *Run) watchSegment(watched *segment) {
	<-watched.session.Done()

	r.mu.Lock()
	unexpected := r.current == watched
	if unexpected {
		r.current = nil
		r.truncated = true
	}
	r.mu.Unlock()

	if unexpected && r.options.OnSegmentExit != nil {
		err := watched.session.ExitError()
		if err == nil {
			err = errors.New("the recorder stopped on its own")
		}
		r.options.OnSegmentExit(err, watched.session.Elapsed())
	}
}

// Recording reports whether a segment is being written.
func (r *Run) Recording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.current != nil
}

// Started reports whether any segment has been started.
func (r *Run) Started() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.segments) > 0
}

// Segments is how many segments the run has started.
func (r *Run) Segments() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.segments)
}

// SegmentElapsed is how long the current segment has run.
func (r *Run) SegmentElapsed() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil {
		return 0
	}
	return r.current.session.Elapsed()
}

// NoteSlide records a slide change: a chapter in the current segment, and
// the talk start mark when the deck leaves its first slide. A later
// departure from the first slide moves the mark, so clicking through the
// deck during setup does not count as the start.
func (r *Run) NoteSlide(slideIndex int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.options.Now()
	leaving := slideIndex != 0 && (!r.slideKnown || r.lastSlide == 0)
	r.lastSlide, r.slideKnown = slideIndex, true
	if slideIndex != 0 {
		r.leftFirstSlide = true
	}

	if r.current == nil {
		return
	}
	if leaving {
		for _, each := range r.segments {
			each.chapters.ClearMark()
		}
		r.current.chapters.SetMark(now, talkStartTitle)
	}
	r.current.chapters.Add(now, slideIndex, r.titleFor(slideIndex))
	r.writeChaptersLocked()
}

// LeftFirstSlide reports whether the deck has shown any slide but the
// first during the run.
func (r *Run) LeftFirstSlide() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.leftFirstSlide
}

// Dir is the run folder, or "" before the first segment.
func (r *Run) Dir() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dir
}

// Finish stops the run and keeps or deletes its folder. A run that never
// left the first slide is deleted whatever keep says. Finish is
// idempotent: later calls return the first summary. Unlike StartSegment and
// StopSegment, Finish stops the last segment while still holding r.mu:
// the run is ending, so nothing else needs the lock in the meantime.
func (r *Run) Finish(keep bool) (RunSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.finished {
		return r.summary, nil
	}
	r.finished = true

	var stopErr error
	if r.current != nil {
		stopping := r.current
		r.current = nil
		result, err := stopping.session.Stop()
		if result.Truncated {
			r.truncated = true
		}
		stopErr = err
	}
	if r.dir == "" {
		return r.summary, stopErr
	}
	r.writeChaptersLocked()

	r.summary = RunSummary{
		Dir:                 r.dir,
		Segments:            len(r.segments),
		NeverLeftFirstSlide: !r.leftFirstSlide,
		Truncated:           r.truncated,
	}
	if !r.leftFirstSlide || !keep {
		if err := os.RemoveAll(r.dir); err != nil {
			return r.summary, fmt.Errorf("deleting the recording: %w", err)
		}
		r.summary.Deleted = true
	}
	return r.summary, stopErr
}

// writeChaptersLocked rewrites chapters.txt on every change, so the file
// on disk is right even if Tap is killed.
func (r *Run) writeChaptersLocked() {
	if !r.options.Chapters || r.dir == "" {
		return
	}
	var builder strings.Builder
	for _, each := range r.segments {
		builder.WriteString(each.name)
		builder.WriteString("\n")
		builder.WriteString(each.chapters.Render())
	}
	_ = os.WriteFile(filepath.Join(r.dir, "chapters.txt"), []byte(builder.String()), 0o600)
}

func (r *Run) currentSlide() (int, bool) {
	if r.options.CurrentSlide == nil {
		return 0, false
	}
	return r.options.CurrentSlide()
}

func (r *Run) titleFor(slideIndex int) string {
	if r.options.TitleFor == nil {
		return fmt.Sprintf("Slide %d", slideIndex+1)
	}
	return r.options.TitleFor(slideIndex)
}

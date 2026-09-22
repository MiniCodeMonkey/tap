package cli

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// displayPollInterval is how often a tap present run checks the displays.
const displayPollInterval = 3 * time.Second

var _ tui.PresentRecorder = (*presentRecorder)(nil)

// minSegmentRunToRespawn is how long a segment must have recorded before
// its own exit is treated as something to recover from by starting a new
// one. A segment that ends sooner than this almost certainly died for a
// reason that will recur at once (Screen Recording permission just
// revoked, for example), so restarting it would spin a hot loop of
// segments instead of surfacing the problem.
const minSegmentRunToRespawn = 2 * time.Second

// presentRecorderOptions configures a tap present run.
type presentRecorderOptions struct {
	Run          recorder.RunOptions
	OutputDir    string
	ListScreens  func() ([]recorder.Screen, error)
	FreeSpace    func(string) (uint64, error)
	OnEvent      func(eventType, message string)
	OnDiskLevel  func(recorder.DiskLevel)
	PollInterval time.Duration
	DiskInterval time.Duration
}

// presentRecorder records a tap present run: it follows the projector,
// pauses while it is away, and stops before the disk fills. The speaker's
// own c and a full disk both stop the run until the next c; the display
// poll never restarts a recording either of them stopped.
//
// State is kept in an atomic value and blocked in its own mutex, both
// readable without p.mu, because applyLocked holds p.mu across
// Run.StartSegment/StopSegment, which stop the previous segment's recorder
// synchronously and can take a while. The TUI reads State() and Blocked()
// every second from its View and must never wait behind a segment switch.
// All mutations still happen under p.mu.
type presentRecorder struct {
	options presentRecorderOptions
	run     *recorder.Run

	mu       sync.Mutex
	follower recorder.Follower
	held     bool

	state atomic.Value // tui.PresentRecordingState

	blockedMu sync.Mutex
	blocked   string
}

func newPresentRecorder(options presentRecorderOptions) *presentRecorder {
	if options.ListScreens == nil {
		options.ListScreens = recorder.Screens
	}
	if options.PollInterval == 0 {
		options.PollInterval = displayPollInterval
	}
	if options.DiskInterval == 0 {
		options.DiskInterval = diskPollInterval
	}
	present := &presentRecorder{options: options}
	present.state.Store(tui.PresentNotRecording)
	runOptions := options.Run
	runOptions.OnSegmentExit = present.noteSegmentExit
	present.run = recorder.NewRun(runOptions)
	return present
}

// Block keeps the run from ever recording, and says why in the TUI.
func (p *presentRecorder) Block(reason string) {
	p.blockedMu.Lock()
	p.blocked = reason
	p.blockedMu.Unlock()
}

// Begin starts the display and disk polls, and records at once when
// startNow is set. Without it the run waits for c, but the polls still run
// so a recording started by hand follows the projector too. It returns at
// once.
func (p *presentRecorder) Begin(ctx context.Context, startNow bool) {
	p.mu.Lock()
	if !startNow {
		p.held = true
	}
	p.mu.Unlock()
	if p.Blocked() != "" {
		return
	}

	p.recheck(startNow)
	go p.pollDisplays(ctx)
	go newDiskWatch(p.options.OutputDir, p.options.FreeSpace, p.noteDiskLevel).run(ctx, p.options.DiskInterval)
}

func (p *presentRecorder) pollDisplays(ctx context.Context) {
	ticker := time.NewTicker(p.options.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.recheck(false)
		}
	}
}

// recheck folds a fresh screen list into the display rule. force applies
// the decision even when it has not changed, for a segment whose recorder
// stopped on its own.
func (p *presentRecorder) recheck(force bool) {
	screens, err := p.options.ListScreens()
	if err != nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	decision, changed := p.follower.Decide(screens)
	if p.held || p.Blocked() != "" || (!changed && !force) {
		return
	}
	p.applyLocked(decision)
}

// applyLocked makes the run match a decision. It is called with p.mu held.
func (p *presentRecorder) applyLocked(decision recorder.FollowDecision) {
	if decision.Paused {
		if err := p.run.StopSegment(); err != nil {
			p.event("error", "Stopping the recording failed: "+err.Error())
		}
		if p.state.Load() != tui.PresentPaused {
			p.event("action", "Projector gone: recording paused until it is back")
		}
		p.state.Store(tui.PresentPaused)
		return
	}

	path, err := p.run.StartSegment(decision.Display)
	if err != nil {
		p.state.Store(tui.PresentNotRecording)
		p.event("error", "Recording failed: "+err.Error())
		return
	}
	p.state.Store(tui.PresentRecording)
	p.event("action", "Recording → "+path)
}

// noteSegmentExit is Run's report that a segment's recorder stopped on its
// own. A segment that ran for at least minSegmentRunToRespawn is treated
// as a display hiccup and recovered by starting a new segment on whatever
// the display rule says now. A segment that exited sooner almost certainly
// hit a problem that would recur immediately (a revoked permission, for
// example), so the run is held instead of respawning in a hot loop, and
// the speaker is told why recording stopped.
func (p *presentRecorder) noteSegmentExit(err error, ran time.Duration) {
	if ran >= minSegmentRunToRespawn {
		p.recheck(true)
		return
	}

	p.mu.Lock()
	p.held = true
	p.mu.Unlock()
	p.state.Store(tui.PresentNotRecording)
	p.event("error", "Recording stopped: "+err.Error())
}

func (p *presentRecorder) noteDiskLevel(level recorder.DiskLevel) {
	if level == recorder.DiskFull {
		p.mu.Lock()
		p.held = true
		_ = p.run.StopSegment()
		p.mu.Unlock()
		p.state.Store(tui.PresentNotRecording)
	}
	if p.options.OnDiskLevel != nil {
		p.options.OnDiskLevel(level)
	}
}

// NoteSlideChange is the hub's slide callback.
func (p *presentRecorder) NoteSlideChange(slideIndex int) {
	p.run.NoteSlide(slideIndex)
}

// Toggle is c: stop a recording or a pause and hold the run, or release
// the hold and start a new segment on the display the rule picks now.
func (p *presentRecorder) Toggle() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if blocked := p.Blocked(); blocked != "" {
		return errors.New(blocked)
	}
	if p.state.Load() != tui.PresentNotRecording {
		p.held = true
		p.state.Store(tui.PresentNotRecording)
		return p.run.StopSegment()
	}

	p.held = false
	screens, err := p.options.ListScreens()
	if err != nil {
		return err
	}
	decision, _ := p.follower.Decide(screens)
	if decision.Paused {
		// c is a request to record now, whatever the projector does.
		decision = recorder.FollowDecision{Display: 1}
	}
	p.applyLocked(decision)
	return nil
}

// Finish stops the run and keeps or deletes it.
func (p *presentRecorder) Finish(keep bool) (recorder.RunSummary, error) {
	p.mu.Lock()
	p.held = true
	p.mu.Unlock()
	return p.run.Finish(keep)
}

func (p *presentRecorder) State() tui.PresentRecordingState {
	return p.state.Load().(tui.PresentRecordingState)
}

func (p *presentRecorder) Elapsed() time.Duration   { return p.run.SegmentElapsed() }
func (p *presentRecorder) Started() bool            { return p.run.Started() }
func (p *presentRecorder) LeftFirstSlide() bool     { return p.run.LeftFirstSlide() }
func (p *presentRecorder) SuggestGitignore() string { return suggestGitignore(p.options.OutputDir) }
func (p *presentRecorder) AddGitignoreEntry() error { return addGitignoreEntry(p.options.OutputDir) }

func (p *presentRecorder) Blocked() string {
	p.blockedMu.Lock()
	defer p.blockedMu.Unlock()
	return p.blocked
}

func (p *presentRecorder) event(eventType, message string) {
	if p.options.OnEvent != nil {
		p.options.OnEvent(eventType, message)
	}
}

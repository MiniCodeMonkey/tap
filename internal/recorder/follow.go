package recorder

// FollowDecision is what a run should be recording right now: a display,
// or nothing because the projector it was using has gone.
type FollowDecision struct {
	Display int
	Paused  bool
}

// Follower applies the display rule across a run. Once any external screen
// has been seen, losing every external screen pauses the run instead of
// falling back to the laptop: during an HDMI swap the laptop would record
// someone else's talk and the presenter notes. A run that never sees an
// external screen records the laptop throughout.
type Follower struct {
	current      FollowDecision
	decided      bool
	externalSeen bool
}

// Decide folds a fresh screen list into the rule and reports whether the
// decision differs from the previous one. The first call always reports a
// change.
func (f *Follower) Decide(screens []Screen) (FollowDecision, bool) {
	display, external := ChooseDisplay(screens)
	if external {
		f.externalSeen = true
	}

	next := FollowDecision{Display: display}
	if !external && f.externalSeen {
		next = FollowDecision{Paused: true}
	}

	changed := !f.decided || next != f.current
	f.current, f.decided = next, true
	return next, changed
}

// Current is the last decision.
func (f *Follower) Current() FollowDecision {
	return f.current
}

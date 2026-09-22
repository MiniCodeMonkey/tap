package recorder

import "testing"

var (
	laptopOnly = []Screen{{Name: "Color LCD", Index: 1, BuiltIn: true}}
	withBeamer = []Screen{{Name: "Color LCD", Index: 1, BuiltIn: true}, {Name: "EPSON PJ", Index: 2}}
)

func TestFollowerRecordsTheLaptopForAPracticeRun(t *testing.T) {
	var follower Follower

	decision, changed := follower.Decide(laptopOnly)
	if !changed || decision != (FollowDecision{Display: 1}) {
		t.Fatalf("got %+v changed=%v, want display 1, changed", decision, changed)
	}

	decision, changed = follower.Decide(laptopOnly)
	if changed || decision != (FollowDecision{Display: 1}) {
		t.Errorf("second decide = %+v changed=%v, want display 1, unchanged", decision, changed)
	}
}

func TestFollowerPausesOnceAProjectorHasGone(t *testing.T) {
	var follower Follower

	follower.Decide(withBeamer)
	decision, changed := follower.Decide(laptopOnly)
	if !changed || decision != (FollowDecision{Paused: true}) {
		t.Fatalf("after unplug got %+v changed=%v, want paused", decision, changed)
	}

	decision, changed = follower.Decide(withBeamer)
	if !changed || decision != (FollowDecision{Display: 2}) {
		t.Errorf("after replug got %+v changed=%v, want display 2", decision, changed)
	}
}

func TestFollowerCurrentIsTheLastDecision(t *testing.T) {
	var follower Follower
	follower.Decide(withBeamer)

	if got := follower.Current(); got != (FollowDecision{Display: 2}) {
		t.Errorf("Current() = %+v, want display 2", got)
	}
}

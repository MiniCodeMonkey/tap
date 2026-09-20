package recorder

import (
	"errors"
	"testing"
)

const twoDisplayProfile = `{
  "SPDisplaysDataType": [
    {
      "spdisplays_ndrvs": [
        {
          "_name": "Color LCD",
          "_spdisplays_resolution": "2294 x 1432 @ 120.00Hz",
          "spdisplays_main": "spdisplays_yes",
          "spdisplays_online": "spdisplays_yes"
        },
        {
          "_name": "DELL U2720Q",
          "_spdisplays_resolution": "3840 x 2160 @ 60.00Hz",
          "spdisplays_online": "spdisplays_yes"
        }
      ]
    }
  ]
}`

// fakeProbe returns a displayProbe that succeeds for every index up to and
// including okThrough, and fails for everything after that.
func fakeProbe(okThrough int, failure error) displayProbe {
	return func(display int) error {
		if display <= okThrough {
			return nil
		}
		return failure
	}
}

func TestCountDisplaysCountsConsecutiveSuccesses(t *testing.T) {
	if got := countDisplays(fakeProbe(1, errors.New("refused")), maxProbedDisplays); got != 1 {
		t.Errorf("countDisplays() = %d, want 1", got)
	}
}

// This is the regression case: screencapture's real two-display refusal is
// "Invalid display specified. Must be a number from 1-2", worded nothing
// like the single-display "Only 1 display" message the old parser expected.
// Probing display 3 and having it refused, after 1 and 2 both succeeded,
// must still read as two displays.
func TestCountDisplaysHandlesTheTwoDisplayRefusalWording(t *testing.T) {
	probe := fakeProbe(2, errors.New("screencapture: Invalid display specified. Must be a number from 1-2"))
	if got := countDisplays(probe, maxProbedDisplays); got != 2 {
		t.Errorf("countDisplays() = %d, want 2", got)
	}
}

func TestCountDisplaysFallsBackToOneWhenTheFirstProbeFails(t *testing.T) {
	// The first probe should only fail this way after Preflight has already
	// confirmed screencapture can capture something, so this is treated as
	// an anomaly, not as "zero displays exist".
	probe := fakeProbe(0, errors.New("screencapture: cannot write file"))
	if got := countDisplays(probe, maxProbedDisplays); got != 1 {
		t.Errorf("countDisplays() = %d, want 1", got)
	}
}

func TestCountDisplaysStopsAtAPermissionFailureRatherThanCountingFurther(t *testing.T) {
	// Two displays already succeeded before a third probe fails for an
	// unrelated reason. That failure must not be read as "three displays",
	// nor as "zero displays": the two confirmed successes stand.
	probe := fakeProbe(2, errors.New("screencapture: cannot write file"))
	if got := countDisplays(probe, maxProbedDisplays); got != 2 {
		t.Errorf("countDisplays() = %d, want 2", got)
	}
}

func TestCountDisplaysIsBoundedByMaxDisplays(t *testing.T) {
	alwaysOK := func(int) error { return nil }
	if got := countDisplays(alwaysOK, maxProbedDisplays); got != maxProbedDisplays {
		t.Errorf("countDisplays() = %d, want the bound %d", got, maxProbedDisplays)
	}
}

func TestParseDisplayDetailsPutsTheMainDisplayFirst(t *testing.T) {
	got := parseDisplayDetails([]byte(twoDisplayProfile))

	if len(got) != 2 {
		t.Fatalf("parseDisplayDetails() returned %d displays, want 2", len(got))
	}
	if got[0].Name != "Color LCD" || !got[0].Main {
		t.Errorf("first display = %+v, want the main Color LCD", got[0])
	}
	if got[0].Resolution != "2294 x 1432" {
		t.Errorf("Resolution = %q, want the refresh rate stripped", got[0].Resolution)
	}
	if got[1].Name != "DELL U2720Q" {
		t.Errorf("second display = %q, want DELL U2720Q", got[1].Name)
	}
}

func TestParseDisplayDetailsSkipsOfflineDisplays(t *testing.T) {
	raw := `{"SPDisplaysDataType":[{"spdisplays_ndrvs":[
		{"_name":"Color LCD","spdisplays_main":"spdisplays_yes","spdisplays_online":"spdisplays_yes"},
		{"_name":"Sleeping","spdisplays_online":"spdisplays_no"}
	]}]}`

	got := parseDisplayDetails([]byte(raw))
	if len(got) != 1 {
		t.Fatalf("parseDisplayDetails() returned %d displays, want 1", len(got))
	}
}

func TestMergeDisplaysNumbersFromOne(t *testing.T) {
	got := mergeDisplays(2, parseDisplayDetails([]byte(twoDisplayProfile)))

	if len(got) != 2 {
		t.Fatalf("mergeDisplays() returned %d displays, want 2", len(got))
	}
	if got[0].Index != 1 || got[1].Index != 2 {
		t.Errorf("indexes = %d, %d, want 1, 2", got[0].Index, got[1].Index)
	}
	if got[0].Name != "Color LCD" {
		t.Errorf("display 1 = %q, want the main display", got[0].Name)
	}
}

func TestMergeDisplaysDropsNamesWhenTheCountsDisagree(t *testing.T) {
	got := mergeDisplays(3, parseDisplayDetails([]byte(twoDisplayProfile)))

	if len(got) != 3 {
		t.Fatalf("mergeDisplays() returned %d displays, want 3", len(got))
	}
	for _, display := range got {
		if display.Name != "" || display.Resolution != "" {
			t.Errorf("display %d carries a name %q, want it dropped when the sources disagree", display.Index, display.Name)
		}
	}
	if !got[0].Main {
		t.Error("display 1 is not marked main, and screencapture documents -D 1 as the main display")
	}
}

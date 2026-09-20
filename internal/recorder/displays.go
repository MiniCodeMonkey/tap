package recorder

import (
	"encoding/json"
	"strings"
)

// displayProbe asks whether screencapture accepts the given display index.
// Displays on darwin supplies the real implementation, which shells out for
// a single 1x1 still; tests supply a fake so the loop is exercised without
// ever invoking the real binary.
type displayProbe func(display int) error

// maxProbedDisplays bounds the probing loop so a pathological failure
// cannot spin forever waiting for a refusal that never comes.
const maxProbedDisplays = 16

// countDisplays finds how many displays screencapture accepts by asking for
// display 1, then 2, then 3, and so on, until one is refused. The number of
// consecutive successes is the count. This tests exactly what -D<n> accepts,
// so it keeps working no matter how screencapture words its refusal, unlike
// parsing that wording for a number.
//
// A probe can also fail for a reason that has nothing to do with the
// display index, such as a missing Screen Recording permission. That
// failure is not a display count and is not treated as one: the loop stops
// and reports however many probes already succeeded. If none had, it
// reports 1, since Displays is only ever called after Preflight has already
// confirmed screencapture can capture the main display at all, so a probe
// failing on the very first display means something else went wrong, not
// that zero displays exist.
func countDisplays(probe displayProbe, maxDisplays int) int {
	count := 0
	for display := 1; display <= maxDisplays; display++ {
		if err := probe(display); err != nil {
			break
		}
		count = display
	}
	if count == 0 {
		return 1
	}
	return count
}

// systemProfilerDisplays is the shape of the display report this code reads.
type systemProfilerDisplays struct {
	Displays []struct {
		Screens []struct {
			Name       string `json:"_name"`
			Resolution string `json:"_spdisplays_resolution"`
			Main       string `json:"spdisplays_main"`
			Online     string `json:"spdisplays_online"`
		} `json:"spdisplays_ndrvs"`
	} `json:"SPDisplaysDataType"`
}

// parseDisplayDetails reads display names and resolutions, main display
// first. Order matters: screencapture documents -D 1 as the main display,
// so putting it first is what lets an index line up with a name at all.
// Anything offline is left out, since it cannot be recorded.
func parseDisplayDetails(raw []byte) []Display {
	var report systemProfilerDisplays
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil
	}

	var main []Display
	var others []Display

	for _, group := range report.Displays {
		for _, screen := range group.Screens {
			if screen.Online == "spdisplays_no" {
				continue
			}

			display := Display{
				Name:       screen.Name,
				Resolution: trimRefreshRate(screen.Resolution),
				Main:       screen.Main == "spdisplays_yes",
			}
			if display.Main {
				main = append(main, display)
			} else {
				others = append(others, display)
			}
		}
	}

	return append(main, others...)
}

// trimRefreshRate turns "2294 x 1432 @ 120.00Hz" into "2294 x 1432". The
// refresh rate is not what anyone identifies a projector by.
func trimRefreshRate(resolution string) string {
	if index := strings.Index(resolution, "@"); index >= 0 {
		resolution = resolution[:index]
	}
	return strings.TrimSpace(resolution)
}

// mergeDisplays numbers the displays the way screencapture does, from 1.
//
// The count is authoritative because it comes from probing screencapture
// itself. The names come from a different tool, and only line up with
// those indexes if both report the same number of displays. When they
// disagree, the names are dropped: an unlabeled index is honest, a wrong
// label sends someone to record the wrong screen.
func mergeDisplays(count int, details []Display) []Display {
	displays := make([]Display, 0, count)
	named := len(details) == count

	for index := 1; index <= count; index++ {
		display := Display{Index: index, Main: index == 1}
		if named {
			display.Name = details[index-1].Name
			display.Resolution = details[index-1].Resolution
		}
		displays = append(displays, display)
	}

	return displays
}

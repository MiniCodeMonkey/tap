package recorder

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// displayCountPattern reads the display count out of screencapture's own
// refusal, which is the only source that speaks in -D indexes. It covers
// both the singular and plural wording.
var displayCountPattern = regexp.MustCompile(`Only (\d+) display`)

// parseDisplayCount reads how many displays screencapture will accept from
// the message it prints when asked for one that does not exist.
func parseDisplayCount(output string) (int, bool) {
	match := displayCountPattern.FindStringSubmatch(output)
	if match == nil {
		return 0, false
	}

	count, err := strconv.Atoi(match[1])
	if err != nil || count < 1 {
		return 0, false
	}
	return count, true
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
// The count is authoritative because it comes from screencapture itself.
// The names come from a different tool, and only line up with those indexes
// if both report the same number of displays. When they disagree, the names
// are dropped: an unlabeled index is honest, a wrong label sends someone to
// record the wrong screen.
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

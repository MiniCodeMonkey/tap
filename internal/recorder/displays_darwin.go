//go:build darwin

package recorder

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// outOfRangeDisplay is larger than any plausible display count, so asking
// for it always draws a refusal. ValidateAudioUID uses it to make sure a
// probe for an audio device never actually records anything.
const outOfRangeDisplay = "-D99"

// probeDisplay takes a 1x1 still from the given display index and reports
// whether screencapture accepted it. It costs nothing and produces nothing
// the speaker has to clean up.
func probeDisplay(display int) error {
	probe := filepath.Join(os.TempDir(), "tap-display-probe-"+strconv.Itoa(display)+".png")
	defer func() { _ = os.Remove(probe) }()

	return exec.Command(defaultCommand, "-x", "-D"+strconv.Itoa(display), "-R0,0,1,1", probe).Run()
}

// Displays lists the screens screencapture will record, in its own index
// order.
func Displays() ([]Display, error) {
	count := countDisplays(probeDisplay, maxProbedDisplays)

	details, err := exec.Command("system_profiler", "SPDisplaysDataType", "-json").Output()
	if err != nil {
		return mergeDisplays(count, nil), nil
	}

	return mergeDisplays(count, parseDisplayDetails(details)), nil
}

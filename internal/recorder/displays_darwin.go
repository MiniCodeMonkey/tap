//go:build darwin

package recorder

import (
	"os"
	"os/exec"
	"path/filepath"
)

// outOfRangeDisplay is larger than any plausible display count, so asking
// for it always draws the refusal that names the real count.
const outOfRangeDisplay = "-D99"

// Displays lists the screens screencapture will record, in its own index
// order. It takes a 1x1 still rather than a recording, so it costs nothing
// and produces nothing the speaker has to clean up.
func Displays() ([]Display, error) {
	probe := filepath.Join(os.TempDir(), "tap-display-probe.png")
	defer func() { _ = os.Remove(probe) }()

	output, _ := exec.Command(defaultCommand, "-x", outOfRangeDisplay, "-R0,0,1,1", probe).CombinedOutput()

	count, ok := parseDisplayCount(string(output))
	if !ok {
		// screencapture said something unexpected. One display is the
		// safe reading: it is the only index guaranteed to exist.
		count = 1
	}

	details, err := exec.Command("system_profiler", "SPDisplaysDataType", "-json").Output()
	if err != nil {
		return mergeDisplays(count, nil), nil
	}

	return mergeDisplays(count, parseDisplayDetails(details)), nil
}

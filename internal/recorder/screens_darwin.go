package recorder

import "os/exec"

// Screens lists the online screens. It only runs system_profiler, so it is
// cheap enough to poll every few seconds, unlike Displays, which probes
// each display with screencapture.
func Screens() ([]Screen, error) {
	raw, err := exec.Command("system_profiler", "SPDisplaysDataType", "-json").Output()
	if err != nil {
		return nil, err
	}
	return parseScreens(raw), nil
}

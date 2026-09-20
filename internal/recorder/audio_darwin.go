//go:build darwin

package recorder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// audioReport is the raw audio device report, or nil when it cannot be read.
func audioReport() []byte {
	output, err := exec.Command("system_profiler", "SPAudioDataType", "-json").Output()
	if err != nil {
		return nil
	}
	return output
}

// DefaultAudioInput names the microphone a recording will use, or "" when
// it cannot be determined.
func DefaultAudioInput() string {
	return parseDefaultAudioInput(audioReport())
}

// ValidateAudioUID reports whether screencapture accepts a CoreAudio UID.
// It pairs the id with a display index that cannot exist, so screencapture
// refuses the request either way and records nothing: it checks the audio
// device first, so an unknown id is named in the error while a good one
// falls through to the complaint about the display.
func ValidateAudioUID(uid string) error {
	if uid == "" {
		return nil
	}

	probe := filepath.Join(os.TempDir(), "tap-audio-probe.mov")
	defer func() { _ = os.Remove(probe) }()

	output, _ := exec.Command(defaultCommand, "-x", "-v", "-V1", "-G"+uid, outOfRangeDisplay, probe).CombinedOutput()
	if strings.Contains(string(output), "not found") {
		return fmt.Errorf("audio device %q not found: recording.audio takes a CoreAudio UID, not a device name", uid)
	}
	return nil
}

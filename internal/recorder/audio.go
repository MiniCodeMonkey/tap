package recorder

import "encoding/json"

// systemProfilerAudio is the shape of the audio report this code reads.
// CoreAudio device UIDs are absent from it at every detail level, which is
// why a microphone cannot be chosen from inside Tap.
type systemProfilerAudio struct {
	Groups []struct {
		Items []struct {
			Name          string `json:"_name"`
			InputChannels int    `json:"coreaudio_device_input"`
			DefaultInput  string `json:"coreaudio_default_audio_input_device"`
		} `json:"_items"`
	} `json:"SPAudioDataType"`
}

// parseDefaultAudioInput names the input screencapture's -g flag will
// record from, or "" when the report shows no default input.
func parseDefaultAudioInput(raw []byte) string {
	var report systemProfilerAudio
	if err := json.Unmarshal(raw, &report); err != nil {
		return ""
	}

	for _, group := range report.Groups {
		for _, item := range group.Items {
			if item.DefaultInput == "spaudio_yes" {
				return item.Name
			}
		}
	}
	return ""
}

// parseAudioInputCount counts the devices that can capture audio.
func parseAudioInputCount(raw []byte) int {
	var report systemProfilerAudio
	if err := json.Unmarshal(raw, &report); err != nil {
		return 0
	}

	count := 0
	for _, group := range report.Groups {
		for _, item := range group.Items {
			if item.InputChannels > 0 {
				count++
			}
		}
	}
	return count
}

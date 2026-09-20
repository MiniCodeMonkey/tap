package recorder

import "testing"

const audioProfile = `{
  "SPAudioDataType": [
    {
      "_items": [
        {"_name": "Stream Deck Microphone", "coreaudio_device_input": 1},
        {"_name": "MacBook Pro Microphone", "coreaudio_device_input": 1,
         "coreaudio_default_audio_input_device": "spaudio_yes"},
        {"_name": "MacBook Pro Speakers", "coreaudio_device_output": 2}
      ]
    }
  ]
}`

func TestParseDefaultAudioInputNamesTheDefault(t *testing.T) {
	if got := parseDefaultAudioInput([]byte(audioProfile)); got != "MacBook Pro Microphone" {
		t.Errorf("parseDefaultAudioInput() = %q, want MacBook Pro Microphone", got)
	}
}

func TestParseDefaultAudioInputWithoutADefault(t *testing.T) {
	raw := `{"SPAudioDataType":[{"_items":[{"_name":"Speakers","coreaudio_device_output":2}]}]}`

	if got := parseDefaultAudioInput([]byte(raw)); got != "" {
		t.Errorf("parseDefaultAudioInput() = %q, want an empty string", got)
	}
}

func TestParseDefaultAudioInputWithBrokenJSON(t *testing.T) {
	if got := parseDefaultAudioInput([]byte("not json")); got != "" {
		t.Errorf("parseDefaultAudioInput() = %q, want an empty string", got)
	}
}

func TestParseAudioInputCountCountsOnlyInputs(t *testing.T) {
	if got := parseAudioInputCount([]byte(audioProfile)); got != 2 {
		t.Errorf("parseAudioInputCount() = %d, want 2", got)
	}
}

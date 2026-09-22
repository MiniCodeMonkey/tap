package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

func consentFor(t *testing.T, typed string) (consentInput, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	return consentInput{
		SettingsPath: filepath.Join(t.TempDir(), "settings.yaml"),
		Supported:    true,
		Interactive:  true,
		In:           strings.NewReader(typed),
		Out:          out,
	}, out
}

func TestConsentAsksOnceAndSavesTheAnswer(t *testing.T) {
	input, out := consentFor(t, "y\n")

	wanted, err := presentRecordingWanted(input)
	if err != nil || !wanted {
		t.Fatalf("wanted = %v, %v; want true", wanted, err)
	}
	if !strings.Contains(out.String(), "Record automatically every time you run tap present? (y/n)") {
		t.Errorf("prompt missing: %q", out.String())
	}

	input.In = strings.NewReader("")
	out.Reset()
	wanted, err = presentRecordingWanted(input)
	if err != nil || !wanted || out.Len() != 0 {
		t.Errorf("second run = %v, %v, output %q; want true without asking", wanted, err, out.String())
	}
}

func TestConsentAsksAgainAfterAnUnclearAnswer(t *testing.T) {
	input, out := consentFor(t, "maybe\nn\n")

	wanted, err := presentRecordingWanted(input)
	if err != nil || wanted {
		t.Errorf("wanted = %v, %v; want false", wanted, err)
	}
	if strings.Count(out.String(), "(y/n)") != 2 {
		t.Errorf("want the prompt twice: %q", out.String())
	}
}

func TestConsentNeverAsksWithoutATerminal(t *testing.T) {
	input, out := consentFor(t, "y\n")
	input.Interactive = false

	wanted, err := presentRecordingWanted(input)
	if err != nil || wanted {
		t.Errorf("wanted = %v, %v; want false", wanted, err)
	}
	if strings.Contains(out.String(), "(y/n)") {
		t.Errorf("asked without a terminal: %q", out.String())
	}
	if settings, _ := usersettings.Load(input.SettingsPath); settings.Present.Record != nil {
		t.Errorf("saved an answer nobody gave")
	}
}

func TestConsentNoRecordSkipsThisRunOnly(t *testing.T) {
	input, _ := consentFor(t, "y\n")
	if _, err := presentRecordingWanted(input); err != nil {
		t.Fatal(err)
	}

	input.NoRecord = true
	wanted, err := presentRecordingWanted(input)
	if err != nil || wanted {
		t.Errorf("wanted = %v, %v; want false with --no-record", wanted, err)
	}
	settings, _ := usersettings.Load(input.SettingsPath)
	if settings.Present.Record == nil || !*settings.Present.Record {
		t.Errorf("--no-record changed the saved answer")
	}
}

// TestConsentIgnoresAMalformedSettingsFileAndAsksAgain covers ruling 6: a
// malformed settings.yaml must not stop the talk. Consent is treated as
// unanswered, so an interactive run asks again and saves the speaker's
// answer, overwriting the broken file.
func TestConsentIgnoresAMalformedSettingsFileAndAsksAgain(t *testing.T) {
	input, out := consentFor(t, "y\n")
	if err := os.WriteFile(input.SettingsPath, []byte("present: [this is not valid yaml"), 0o600); err != nil {
		t.Fatal(err)
	}

	wanted, err := presentRecordingWanted(input)
	if err != nil || !wanted {
		t.Fatalf("wanted = %v, %v; want true", wanted, err)
	}
	if !strings.Contains(out.String(), input.SettingsPath) {
		t.Errorf("no warning naming the malformed file: %q", out.String())
	}
	if !strings.Contains(out.String(), "(y/n)") {
		t.Errorf("a malformed settings file stopped the talk from asking: %q", out.String())
	}

	settings, loadErr := usersettings.Load(input.SettingsPath)
	if loadErr != nil || settings.Present.Record == nil || !*settings.Present.Record {
		t.Errorf("the speaker's answer was not saved over the malformed file: %+v, %v", settings, loadErr)
	}
}

// TestConsentIgnoresAMalformedSettingsFileWithoutATerminal covers the other
// half of ruling 6: a non-interactive run with a malformed settings file
// does not record, warns once, and never overwrites the file since nobody
// answered the prompt.
func TestConsentIgnoresAMalformedSettingsFileWithoutATerminal(t *testing.T) {
	input, out := consentFor(t, "y\n")
	input.Interactive = false
	broken := []byte("present: [this is not valid yaml")
	if err := os.WriteFile(input.SettingsPath, broken, 0o600); err != nil {
		t.Fatal(err)
	}

	wanted, err := presentRecordingWanted(input)
	if err != nil || wanted {
		t.Fatalf("wanted = %v, %v; want false", wanted, err)
	}
	if !strings.Contains(out.String(), input.SettingsPath) {
		t.Errorf("no warning naming the malformed file: %q", out.String())
	}

	onDisk, err := os.ReadFile(input.SettingsPath)
	if err != nil || string(onDisk) != string(broken) {
		t.Errorf("the malformed file was overwritten without an answer: %q, %v", onDisk, err)
	}
}

func TestConsentOffMacOSNeverAsks(t *testing.T) {
	input, out := consentFor(t, "y\n")
	input.Supported = false

	if wanted, _ := presentRecordingWanted(input); wanted || out.Len() != 0 {
		t.Errorf("asked or recorded on an unsupported platform")
	}
}

package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// consentInput is everything the consent check reads.
type consentInput struct {
	SettingsPath string
	In           io.Reader
	Out          io.Writer
	// Asker asks the question. Nil asks on In and Out, at the terminal.
	Asker       consentAsker
	NoRecord    bool
	Supported   bool
	Interactive bool
}

// consentAsker asks whether to record every tap present run. answered is
// false when no answer came, and then nothing is saved.
type consentAsker interface {
	askRecordConsent() (record, answered bool)
}

// terminalConsentAsker asks at the terminal.
type terminalConsentAsker struct {
	in  io.Reader
	out io.Writer
}

func (asker terminalConsentAsker) askRecordConsent() (record, answered bool) {
	reader := bufio.NewReader(asker.in)
	for {
		fmt.Fprint(asker.out, "Record automatically every time you run tap present? (y/n) ")
		line, readErr := reader.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true, true
		case "n", "no":
			return false, true
		}
		if readErr != nil {
			return false, false
		}
	}
}

// presentRecordingWanted decides whether this tap present run records from
// launch. The speaker is asked once per machine, and only at a terminal;
// the answer is saved in the user settings, never in the deck, so a deck
// from someone else cannot switch recording on.
func presentRecordingWanted(input consentInput) (bool, error) {
	if !input.Supported || input.NoRecord {
		return false, nil
	}

	settings, err := usersettings.Load(input.SettingsPath)
	if err != nil {
		// A malformed settings file must not stop the talk: treat consent
		// as unanswered instead of failing outright, so an interactive
		// run asks again and a non-interactive run just does not record.
		// The file itself is left alone unless the speaker answers the
		// prompt below, which overwrites it with a well-formed one.
		fmt.Fprintf(input.Out, "Ignoring %s, it could not be read: %v\n", input.SettingsPath, err)
		settings = usersettings.Settings{}
	}
	if settings.Present.Record != nil {
		return *settings.Present.Record, nil
	}

	if !input.Interactive {
		fmt.Fprintf(input.Out, "Not recording: set present.record in %s to record every tap present run.\n", input.SettingsPath)
		return false, nil
	}

	asker := input.Asker
	if asker == nil {
		asker = terminalConsentAsker{in: input.In, out: input.Out}
	}
	record, answered := asker.askRecordConsent()
	if !answered {
		return false, nil
	}
	return record, saveConsent(input.SettingsPath, settings, record)
}

func saveConsent(path string, settings usersettings.Settings, record bool) error {
	// Reload under the lock rather than reusing settings: the person may
	// have taken a while to answer the prompt, and another tap process
	// could have saved an approval or its own consent answer in the
	// meantime. Merging into a fresh read keeps that change instead of
	// overwriting it.
	return usersettings.WithLock(path, func() error {
		fresh, err := usersettings.Load(path)
		if err != nil {
			fresh = settings
		}
		fresh.Present.Record = &record
		return usersettings.Save(path, fresh)
	})
}

package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// The two scenarios of docs/superpowers/specs/tap-desktop-features/
// 06-live-code-and-trust.feature that belong to the CLI alone, named the
// way desktop/scenarios.txt claims them (desktop/scripts/check-scenarios.sh
// reads Go tests too). The behaviour itself is covered in approval_test.go;
// these pin the scenarios' own wording.

func TestFirstOpenOfADeckWithLiveCodeInTheCLI(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	input, out := approvalInputFor(t, cfg, pres, nil)
	// The person answers no on the terminal.
	input.Asker = terminalAsker{in: strings.NewReader("n\n"), out: out}

	policy, err := liveCodeApproval(input)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"This deck can run code on this computer:",
		"shell      2 blocks on slides 1, 2",
		"Allow this deck to run code? Type s to show the code. [y/N/s]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the terminal prompt lacks %q:\n%s", want, text)
		}
	}
	if len(policy.Drivers) != 0 || policy.AllowAll {
		t.Errorf("a no leaves live code off, got %+v", policy)
	}
	if _, err := os.Stat(input.SettingsPath); !os.IsNotExist(err) {
		t.Error("a no stored something")
	}
}

func TestNonInteractiveRuns(t *testing.T) {
	cfg, pres := approvalFixture("shell")
	asker := &fakeAsker{answer: true}
	input, out := approvalInputFor(t, cfg, pres, asker)
	input.Interactive = false

	policy, err := liveCodeApproval(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Drivers) != 0 || policy.AllowAll || len(asker.requests) != 0 {
		t.Errorf("without a terminal live code is off and nothing is asked, got %+v after %d questions", policy, len(asker.requests))
	}
	if !strings.Contains(out.String(), "Live code is off: this deck is not approved to run shell.") {
		t.Errorf("output = %q", out.String())
	}

	input.AllowCode = true
	policy, err = liveCodeApproval(input)
	if err != nil || !policy.AllowAll {
		t.Errorf("--allow-code turns live code on for the run, got %+v, %v", policy, err)
	}
	if _, err := os.Stat(input.SettingsPath); !os.IsNotExist(err) {
		t.Error("--allow-code stored an approval")
	}
	settings, _ := usersettings.Load(input.SettingsPath)
	if len(settings.Approvals) != 0 {
		t.Errorf("approvals stored: %+v", settings.Approvals)
	}
}

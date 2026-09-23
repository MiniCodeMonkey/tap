package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// descriptorProbeMarker is the text the probe deck's shell block writes on
// every descriptor it can reach. Anything tap can see that carries it came
// out of a child process, through a descriptor the child should not have
// had.
const descriptorProbeMarker = "reached-by-deck-code"

// descriptorProbeDeck writes a deck whose one live code block tries to
// write on every descriptor above standard error, in both a shape that
// breaks a JSON parser and a shape that a JSON parser accepts as an event.
// It is an ordinary deck: the block is the documented live code feature,
// run through the Run button's own route, and nothing is injected into
// tap.
func descriptorProbeDeck(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var probe strings.Builder
	for descriptor := 3; descriptor <= 9; descriptor++ {
		fmt.Fprintf(&probe, "echo '%s-not-json-on-%d' >&%d || true\n", descriptorProbeMarker, descriptor, descriptor)
		fmt.Fprintf(&probe, "echo '{\"type\":\"forged\",\"descriptor\":%d,\"note\":\"%s\"}' >&%d || true\n", descriptor, descriptorProbeMarker, descriptor)
	}
	deck := "---\ntitle: Descriptor Probe Deck\ndrivers:\n  shell: {}\n---\n\n# Probe\n\n```bash {driver: shell}\n" + probe.String() + "echo probe finished\n```\n"
	path := filepath.Join(directory, "talk.md")
	if err := os.WriteFile(path, []byte(deck), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAppModeKeepsItsOwnDescriptorsFromChildProcesses drives the one thing
// the redirect depends on staying private. --app mode duplicates standard
// output for the protocol and standard error for the log, and the whole
// construction rests on no package being able to name those duplicates. A
// child process inherits every descriptor that is not close-on-exec, so if
// they are taken without that flag the deck's own live code is handed both
// of them: one is the stream the app parses as the protocol, the other is
// the raw standard error that --app mode exists to make unreachable.
//
// The deck used here is hostile, which is the premise of the whole mode: a
// deck is something someone sends you. It runs through the approval
// question and the execute route like any other, and afterwards nothing it
// wrote may appear anywhere tap can see.
func TestAppModeKeepsItsOwnDescriptorsFromChildProcesses(t *testing.T) {
	deck := descriptorProbeDeck(t)
	process := startAppProcess(t, t.TempDir(), "dev", "--app", deck)

	question := process.next(appEventQuestion)
	process.send(fmt.Sprintf(`{"type":"answer","id":%q,"value":true}`, question["id"]))
	waitUntil(t, "the probe block runs", func() bool {
		status, body := process.execute(1, 1)
		return status == http.StatusOK && strings.Contains(body, "probe finished")
	})
	// Long enough for anything the child wrote on an inherited descriptor
	// to have been read by the test's own reader of standard output and by
	// the log drain.
	time.Sleep(time.Second)

	process.send(`{"type":"quit"}`)
	_ = process.waitForExit()

	process.assertOnlyJSONLines()
	for _, event := range process.remainingEvents() {
		if event["type"] == "forged" {
			t.Errorf("a deck's live code forged a protocol event the app accepted as real: %v", event)
		}
	}
	if log := process.stderr.String(); strings.Contains(log, descriptorProbeMarker) {
		t.Errorf("a deck's live code wrote on the app's log stream directly, past the bounded writer:\n%s", log)
	}
}

package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// fixtureWithRunner returns the --app fixture deck with a custom runner
// driver that runs command, and a fifth slide with a runner block.
func fixtureWithRunner(t *testing.T, command string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("testdata", "app", "talk.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(source), "drivers:\n  shell: {}\n", fmt.Sprintf("drivers:\n  shell: {}\n  runner:\n    command: %s\n", command), 1)
	return text + "\n---\n\n# Runner\n\n```text {driver: runner}\necho hello from the runner\n```\n"
}

// approvalQuestionFor waits for an approval question about the runner
// driver with command.
func approvalQuestionFor(process *appProcess, command string) map[string]any {
	process.t.Helper()
	return process.nextWhere(appEventQuestion, func(event map[string]any) bool {
		payload, _ := event["payload"].(map[string]any)
		drivers, _ := payload["drivers"].([]any)
		if event["kind"] != appQuestionApproval || len(drivers) != 1 {
			return false
		}
		driver, _ := drivers[0].(map[string]any)
		return driver["name"] == "runner" && driver["command"] == command
	})
}

// executeOnceRendered runs a block once the deck that has it is served,
// and returns the first answer that is not "no such block".
func executeOnceRendered(t *testing.T, process *appProcess, slide, block int) (int, string) {
	t.Helper()
	var status int
	var body string
	waitUntil(t, fmt.Sprintf("slide %d is served", slide), func() bool {
		status, body = process.execute(slide, block)
		return status != http.StatusNotFound && status != http.StatusConflict
	})
	return status, body
}

// questionDriver returns the first driver of an approval question.
func questionDriver(question map[string]any) map[string]any {
	payload, _ := question["payload"].(map[string]any)
	drivers, _ := payload["drivers"].([]any)
	if len(drivers) == 0 {
		return nil
	}
	driver, _ := drivers[0].(map[string]any)
	return driver
}

func answerApproval(process *appProcess, question map[string]any, approved bool) {
	process.send(fmt.Sprintf(`{"type":"answer","id":%q,"value":%t}`, question["id"], approved))
}

func TestAppDevAsksAgainWhenTheDeckFileGainsADriver(t *testing.T) {
	configHome := t.TempDir()
	deck := copyAppFixture(t)
	process := startAppProcess(t, configHome, "dev", "--app", deck)
	answerApproval(process, process.next(appEventQuestion), true)
	waitUntil(t, "shell runs", func() bool {
		status, _ := process.execute(2, 1)
		return status == http.StatusOK
	})

	if err := os.WriteFile(deck, []byte(fixtureWithRunner(t, "sh")), 0o644); err != nil {
		t.Fatal(err)
	}
	question := approvalQuestionFor(process, "sh")
	payload, _ := question["payload"].(map[string]any)
	if before, _ := payload["approvedBefore"].([]any); len(before) != 1 || before[0] != "shell" {
		t.Errorf("approvedBefore = %v, want shell", payload["approvedBefore"])
	}
	if driver := questionDriver(question); driver == nil || driver["previousCommand"] != nil {
		t.Errorf("driver = %v, want no previousCommand for a new driver", driver)
	}

	// Until the answer, the new driver's block is refused and shell runs.
	if status, body := executeOnceRendered(t, process, 5, 1); status != http.StatusForbidden || !strings.Contains(body, "Not approved") {
		t.Errorf("the runner block before the answer: status %d, %s; want the refusal", status, body)
	}
	if status, body := process.execute(2, 1); status != http.StatusOK || !strings.Contains(body, "hello from the fixture") {
		t.Errorf("the approved shell block while the question is open: status %d, %s", status, body)
	}

	answerApproval(process, question, true)
	waitUntil(t, "the runner block runs", func() bool {
		status, body := process.execute(5, 1)
		return status == http.StatusOK && strings.Contains(body, "hello from the runner")
	})
	deckKey, err := usersettings.ResolveDeck(deck)
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(configHome, "tap", "settings.yaml")
	if !coversDriver(t, settingsPath, deckKey, usersettings.Driver{Name: "runner", Command: []string{"sh"}}) {
		settings, _ := usersettings.Load(settingsPath)
		t.Errorf("settings = %+v; want runner stored with its command", settings)
	}
}

func TestAppDevAsksAgainWhenTheBufferChangesACommand(t *testing.T) {
	configHome := t.TempDir()
	deck := copyAppFixture(t)
	if err := os.WriteFile(deck, []byte(fixtureWithRunner(t, "sh")), 0o644); err != nil {
		t.Fatal(err)
	}
	deckKey, err := usersettings.ResolveDeck(deck)
	if err != nil {
		t.Fatal(err)
	}
	var settings usersettings.Settings
	settings.ApproveDrivers(deckKey, withDigests(t, filepath.Join(configHome, "tap", "settings.yaml"), usersettings.Driver{Name: "shell"}, usersettings.Driver{Name: "runner", Command: []string{"sh"}}), time.Now())
	if err := usersettings.Save(filepath.Join(configHome, "tap", "settings.yaml"), settings); err != nil {
		t.Fatal(err)
	}

	process := startAppProcess(t, configHome, "dev", "--app", deck)
	waitUntil(t, "the approved runner runs", func() bool {
		status, _ := process.execute(5, 1)
		return status == http.StatusOK
	})
	process.noEvent(appEventQuestion, 500*time.Millisecond)

	if status, body := process.putSource(fixtureWithRunner(t, "bash")); status != http.StatusOK {
		t.Fatalf("PUT source: %d %s", status, body)
	}
	first := approvalQuestionFor(process, "bash")
	if driver := questionDriver(first); driver["previousCommand"] != "sh" {
		t.Errorf("driver = %v, want previousCommand sh", driver)
	}
	if status, body := process.execute(5, 1); status != http.StatusForbidden || !strings.Contains(body, "Not approved") {
		t.Errorf("the runner block with a changed command: status %d, %s; want the refusal", status, body)
	}

	// A newer command withdraws the open question and asks about it.
	if status, body := process.putSource(fixtureWithRunner(t, "/bin/sh")); status != http.StatusOK {
		t.Fatalf("PUT source: %d %s", status, body)
	}
	if closed := process.next(appEventQuestionClosed); closed["id"] != first["id"] {
		t.Errorf("question-closed = %v, want %v closed", closed, first["id"])
	}
	second := approvalQuestionFor(process, "/bin/sh")
	if driver := questionDriver(second); driver["previousCommand"] != "sh" {
		t.Errorf("driver = %v, want previousCommand sh", driver)
	}

	// The approved command again needs no question at all.
	if status, body := process.putSource(fixtureWithRunner(t, "sh")); status != http.StatusOK {
		t.Fatalf("PUT source: %d %s", status, body)
	}
	if closed := process.next(appEventQuestionClosed); closed["id"] != second["id"] {
		t.Errorf("question-closed = %v, want %v closed", closed, second["id"])
	}
	process.noEvent(appEventQuestion, 500*time.Millisecond)
	waitUntil(t, "the runner block runs its approved command", func() bool {
		status, body := process.execute(5, 1)
		return status == http.StatusOK && strings.Contains(body, "hello from the runner")
	})
}

func TestAppPresentAsksAgainOnReloadAndRemembersANo(t *testing.T) {
	configHome := t.TempDir()
	deck := copyAppFixture(t)
	process := startAppProcess(t, configHome, "present", "--app", "--no-record", deck)
	answerApproval(process, process.next(appEventQuestion), true)
	waitUntil(t, "shell runs", func() bool {
		status, _ := process.execute(2, 1)
		return status == http.StatusOK
	})

	if err := os.WriteFile(deck, []byte(fixtureWithRunner(t, "sh")), 0o644); err != nil {
		t.Fatal(err)
	}
	process.send(`{"type":"reload"}`)
	question := approvalQuestionFor(process, "sh")
	if status, _ := executeOnceRendered(t, process, 5, 1); status != http.StatusForbidden {
		t.Errorf("the runner block before the answer: status %d, want 403", status)
	}

	answerApproval(process, question, false)
	process.send(`{"type":"reload"}`)
	process.noEvent(appEventQuestion, time.Second)
	if status, _ := process.execute(5, 1); status != http.StatusForbidden {
		t.Errorf("the declined runner block: status %d, want 403", status)
	}
	if status, _ := process.execute(2, 1); status != http.StatusOK {
		t.Errorf("the approved shell block after a no to runner: status %d, want 200", status)
	}
}

// TestAppDevPicksUpAnApprovalAnotherTapStored covers tap present --app
// and tap dev --app open on the same deck: an Allow given to the talk is
// stored, and tap dev's next reload finds it rather than asking again,
// whether tap dev was declined or still waiting for its own answer.
func TestAppDevPicksUpAnApprovalAnotherTapStored(t *testing.T) {
	for _, dev := range []struct {
		name   string
		answer func(process *appProcess, question map[string]any)
	}{
		{name: "declined", answer: func(process *appProcess, question map[string]any) { answerApproval(process, question, false) }},
		{name: "unanswered", answer: func(*appProcess, map[string]any) {}},
	} {
		t.Run(dev.name, func(t *testing.T) {
			configHome := t.TempDir()
			deck := copyAppFixture(t)
			if err := os.WriteFile(deck, []byte(fixtureWithRunner(t, "sh")), 0o644); err != nil {
				t.Fatal(err)
			}

			editor := startAppProcess(t, configHome, "dev", "--app", deck)
			editorQuestion := editor.next(appEventQuestion)
			dev.answer(editor, editorQuestion)
			if status, _ := executeOnceRendered(t, editor, 5, 1); status != http.StatusForbidden {
				t.Fatalf("the runner block before any approval: status %d, want 403", status)
			}

			talk := startAppProcess(t, configHome, "present", "--app", "--no-record", deck)
			answerApproval(talk, talk.next(appEventQuestion), true)
			waitUntil(t, "the talk runs the runner block", func() bool {
				status, _ := talk.execute(5, 1)
				return status == http.StatusOK
			})

			editor.send(`{"type":"reload"}`)
			waitUntil(t, "tap dev runs the runner block", func() bool {
				status, body := editor.execute(5, 1)
				return status == http.StatusOK && strings.Contains(body, "hello from the runner")
			})
			if dev.name == "unanswered" {
				if closed := editor.next(appEventQuestionClosed); closed["id"] != editorQuestion["id"] {
					t.Errorf("question-closed = %v, want tap dev's own question closed", closed)
				}
			}
			editor.noEvent(appEventQuestion, 500*time.Millisecond)
		})
	}
}

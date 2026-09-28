package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// tap new --folder makes the deck's folder for the app's New Deck sheet:
// the folder and the file are named by tap's own slug of the title, so no
// other program has to know the rule, and images/ is ready for tap image add.
func TestNewFolderCreatesTheDeckFolder(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	location := t.TempDir()

	exitCode, stdout, stderr := runTap(t, "new", "--folder", location, "--title", "Debugging Production at 3am", "--theme", "terminal", "--json")
	if exitCode != exitOK {
		t.Fatalf("exit %d, stderr %q", exitCode, stderr)
	}
	var output struct {
		OK     bool   `json:"ok"`
		Deck   string `json:"deck"`
		Folder string `json:"folder"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	wantFolder := filepath.Join(location, "debugging-production-at-3am")
	wantDeck := filepath.Join(wantFolder, "debugging-production-at-3am.md")
	if !output.OK || output.Folder != wantFolder || output.Deck != wantDeck {
		t.Errorf("output = %+v, want folder %s and deck %s", output, wantFolder, wantDeck)
	}
	content, err := os.ReadFile(wantDeck)
	if err != nil {
		t.Fatal(err)
	}
	// The starter quotes the title, as new_test.go expects for "My Presentation".
	if !strings.Contains(string(content), `title: "Debugging Production at 3am"`) || !strings.Contains(string(content), "theme: terminal") {
		t.Errorf("deck = %q", content)
	}
	images, err := os.Stat(filepath.Join(wantFolder, "images"))
	if err != nil || !images.IsDir() {
		t.Errorf("no images/ folder: %v", err)
	}

	settings, err := usersettings.Load(filepath.Join(configHome, "tap", "settings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := usersettings.ResolveDeck(wantDeck)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := settings.ApprovalFor(key); !found {
		t.Errorf("no approval for %s: %+v", wantDeck, settings.Approvals)
	}
}

func TestNewFolderAddsASuffixWhenTheFolderExists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	location := t.TempDir()
	if err := os.MkdirAll(filepath.Join(location, "my-talk"), 0o755); err != nil {
		t.Fatal(err)
	}

	exitCode, stdout, stderr := runTap(t, "new", "--folder", location, "--title", "My Talk")
	if exitCode != exitOK {
		t.Fatalf("exit %d, stderr %q", exitCode, stderr)
	}
	want := filepath.Join(location, "my-talk-2", "my-talk-2.md")
	if stdout != want+"\n" {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("deck not written: %v", err)
	}
}

func TestNewFolderDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	location := t.TempDir()

	exitCode, stdout, stderr := runTap(t, "new", "--folder", location)
	if exitCode != exitOK {
		t.Fatalf("exit %d, stderr %q", exitCode, stderr)
	}
	want := filepath.Join(location, "my-presentation", "my-presentation.md")
	if stdout != want+"\n" {
		t.Errorf("stdout = %q, want %q (tui.DefaultTitle's slug)", stdout, want)
	}
}

func TestNewFolderUsageErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	location := t.TempDir()
	cases := []struct {
		name string
		args []string
		code string
		want string
	}{
		{"with a deck argument", []string{"new", "talk.md", "--folder", location}, codeUsage, "--folder names the deck from the title"},
		{"with --output", []string{"new", "--folder", location, "--output", "talk.md"}, codeUsage, "--folder names the deck from the title"},
		{"with --force", []string{"new", "--folder", location, "--force"}, codeUsage, "--force has no meaning with --folder"},
		{"a missing location", []string{"new", "--folder", filepath.Join(location, "nowhere")}, codeFileNotFound, "does not exist"},
		{"an unknown theme", []string{"new", "--folder", location, "--theme", "nope"}, codeUnknownTheme, "unknown theme"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			exitCode, stdout, _ := runTap(t, append(testCase.args, "--json")...)
			if exitCode != exitUserError {
				t.Errorf("exit %d, want %d", exitCode, exitUserError)
			}
			var output struct {
				OK    bool      `json:"ok"`
				Error jsonError `json:"error"`
			}
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
			}
			if output.OK || output.Error.Code != testCase.code || !strings.Contains(output.Error.Message, testCase.want) {
				t.Errorf("error = %+v, want code %s with %q", output.Error, testCase.code, testCase.want)
			}
		})
	}
	if entries, _ := os.ReadDir(location); len(entries) != 0 {
		t.Errorf("a refused run wrote %v", entries)
	}
}

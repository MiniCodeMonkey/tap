package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// useSettings points the user settings at a new folder for one test and
// returns the settings file path.
func useSettings(t *testing.T, approvals ...usersettings.Approval) string {
	t.Helper()
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	path := filepath.Join(configHome, "tap", "settings.yaml")
	if len(approvals) > 0 {
		if err := usersettings.Save(path, usersettings.Settings{Approvals: approvals}); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

var listedAt = time.Date(2026, 9, 22, 19, 32, 0, 0, time.UTC)

func TestApprovalListWithNoApprovals(t *testing.T) {
	useSettings(t)
	exitCode, stdout, stderr := runTap(t, "approval", "list")
	if exitCode != exitOK || stdout != "No deck is approved to run live code.\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", exitCode, stdout, stderr)
	}
}

func TestApprovalListText(t *testing.T) {
	useSettings(t,
		usersettings.Approval{Deck: "/talks/a.md", Drivers: []string{"shell", "sqlite"}, ApprovedAt: listedAt},
		usersettings.Approval{Deck: "/talks/b.md", Drivers: []string{}, ApprovedAt: listedAt},
	)
	exitCode, stdout, _ := runTap(t, "approval", "list")
	want := "/talks/a.md\n  drivers:  shell, sqlite\n  approved: 2026-09-22T19:32:00Z\n" +
		"/talks/b.md\n  drivers:  none\n  approved: 2026-09-22T19:32:00Z\n"
	if exitCode != exitOK || stdout != want {
		t.Errorf("exit %d, stdout:\n%s\nwant:\n%s", exitCode, stdout, want)
	}
}

func TestApprovalListJSON(t *testing.T) {
	useSettings(t, usersettings.Approval{Deck: "/talks/a.md", Drivers: []string{"shell"}, ApprovedAt: listedAt})
	exitCode, stdout, _ := runTap(t, "approval", "list", "--json")
	var decoded struct {
		OK        bool `json:"ok"`
		Approvals []struct {
			Deck       string   `json:"deck"`
			Drivers    []string `json:"drivers"`
			ApprovedAt string   `json:"approvedAt"`
		} `json:"approvals"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if exitCode != exitOK || !decoded.OK || len(decoded.Approvals) != 1 {
		t.Fatalf("exit %d, decoded %+v", exitCode, decoded)
	}
	approval := decoded.Approvals[0]
	if approval.Deck != "/talks/a.md" || strings.Join(approval.Drivers, ",") != "shell" || approval.ApprovedAt != "2026-09-22T19:32:00Z" {
		t.Errorf("approval = %+v", approval)
	}
}

func TestApprovalListJSONWithNoApprovalsIsAnEmptyList(t *testing.T) {
	useSettings(t)
	_, stdout, _ := runTap(t, "approval", "list", "--json")
	if !strings.Contains(stdout, `"approvals": []`) {
		t.Errorf("stdout = %q, want an empty list, not null", stdout)
	}
}

// TestApprovalRevoke seeds the approval under the deck's resolved
// DeckKey, the way a real approval is always stored (liveCodeApproval
// keys every Approve by usersettings.ResolveDeck). t.TempDir() itself is
// not that key on macOS: TMPDIR runs through /var, a symlink to
// /private/var, so the raw path and the resolved one differ by that
// prefix. Seeding the unresolved path here would make a correct revoke,
// which resolves before it looks up the approval, unable to find it.
func TestApprovalRevoke(t *testing.T) {
	deckPath := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(deckPath, []byte("# Talk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolvedDeckPath := resolveKey(t, deckPath).String()
	settingsPath := useSettings(t,
		usersettings.Approval{Deck: resolvedDeckPath, Drivers: []string{"shell"}, ApprovedAt: listedAt},
		usersettings.Approval{Deck: "/talks/other.md", Drivers: []string{"shell"}, ApprovedAt: listedAt},
	)

	exitCode, stdout, stderr := runTap(t, "approval", "revoke", deckPath)
	if exitCode != exitOK || !strings.Contains(stdout, "Revoked: "+resolvedDeckPath) {
		t.Fatalf("exit %d, stdout %q, stderr %q", exitCode, stdout, stderr)
	}
	settings, _ := usersettings.Load(settingsPath)
	if len(settings.Approvals) != 1 || settings.Approvals[0].Deck != "/talks/other.md" {
		t.Errorf("approvals = %+v, want only the other deck", settings.Approvals)
	}
}

func TestApprovalRevokeTakesTheDeckFolder(t *testing.T) {
	folder := t.TempDir()
	deckPath := filepath.Join(folder, "talk.md")
	if err := os.WriteFile(deckPath, []byte("# Talk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	useSettings(t, usersettings.Approval{Deck: resolveKey(t, deckPath).String(), Drivers: []string{"shell"}, ApprovedAt: listedAt})

	exitCode, _, stderr := runTap(t, "approval", "revoke", folder)
	if exitCode != exitOK {
		t.Errorf("exit %d, stderr %q", exitCode, stderr)
	}
}

func TestApprovalRevokeADeckThatNoLongerExists(t *testing.T) {
	useSettings(t, usersettings.Approval{Deck: "/gone/talk.md", Drivers: []string{"shell"}, ApprovedAt: listedAt})
	exitCode, stdout, _ := runTap(t, "approval", "revoke", "/gone/talk.md", "--json")
	if exitCode != exitOK || !strings.Contains(stdout, `"deck": "/gone/talk.md"`) {
		t.Errorf("exit %d, stdout %q", exitCode, stdout)
	}
}

func TestApprovalRevokeAnUnapprovedDeck(t *testing.T) {
	useSettings(t)
	exitCode, stdout, _ := runTap(t, "approval", "revoke", "/talks/never.md", "--json")
	if exitCode != exitUserError || !strings.Contains(stdout, `"code": "not_approved"`) {
		t.Errorf("exit %d, stdout %q", exitCode, stdout)
	}
}

func TestApprovalListShowsTheApprovedCommands(t *testing.T) {
	useSettings(t, usersettings.Approval{
		Deck:       "/talks/a.md",
		Drivers:    []string{"python", "shell"},
		Commands:   map[string][]string{"python": {"python3", "-u"}},
		ApprovedAt: listedAt,
	})
	exitCode, stdout, _ := runTap(t, "approval", "list")
	want := "/talks/a.md\n  drivers:  python (runs: python3 -u), shell\n  approved: 2026-09-22T19:32:00Z\n"
	if exitCode != exitOK || stdout != want {
		t.Errorf("exit %d, stdout:\n%s\nwant:\n%s", exitCode, stdout, want)
	}

	_, stdout, _ = runTap(t, "approval", "list", "--json")
	if !strings.Contains(stdout, `"commands": {`) || !strings.Contains(stdout, `"python3"`) {
		t.Errorf("the JSON lacks the commands:\n%s", stdout)
	}
}

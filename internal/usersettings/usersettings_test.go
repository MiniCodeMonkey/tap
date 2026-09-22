package usersettings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/config-home")

	path, err := Path()
	if err != nil || path != "/tmp/config-home/tap/settings.yaml" {
		t.Errorf("Path() = %q, %v", path, err)
	}
}

func TestPathFallsBackToDotConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/Users/speaker")

	path, err := Path()
	if err != nil || path != "/Users/speaker/.config/tap/settings.yaml" {
		t.Errorf("Path() = %q, %v", path, err)
	}
}

func TestLoadAMissingFileIsEmpty(t *testing.T) {
	settings, err := Load(filepath.Join(t.TempDir(), "settings.yaml"))
	if err != nil || settings.Present.Record != nil {
		t.Errorf("Load = %+v, %v; want empty settings", settings, err)
	}
}

func TestSaveThenLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tap", "settings.yaml")
	record := true

	if err := Save(path, Settings{Present: Present{Record: &record}}); err != nil {
		t.Fatal(err)
	}
	settings, err := Load(path)
	if err != nil || settings.Present.Record == nil || !*settings.Present.Record {
		t.Errorf("Load = %+v, %v; want record: true", settings, err)
	}
}

var approvalTime = time.Date(2026, 9, 22, 19, 32, 0, 0, time.UTC)

func TestApprovedNeedsEveryDeclaredDriver(t *testing.T) {
	var settings Settings
	settings.Approve("/talks/talk.md", []string{"sqlite"}, approvalTime)

	if !settings.Approved("/talks/talk.md", []string{"sqlite"}) {
		t.Error("the approved deck and driver should be approved")
	}
	if settings.Approved("/talks/talk.md", []string{"shell", "sqlite"}) {
		t.Error("a driver added after the approval should not be approved")
	}
	if settings.Approved("/elsewhere/talk.md", []string{"sqlite"}) {
		t.Error("a moved deck should not be approved")
	}
	if !settings.Approved("/talks/./talk.md", nil) {
		t.Error("the path should be compared after cleaning")
	}
}

func TestApproveKeepsTheDriversApprovedBefore(t *testing.T) {
	var settings Settings
	settings.Approve("/talks/talk.md", []string{"sqlite"}, approvalTime)
	later := approvalTime.Add(time.Hour)
	settings.Approve("/talks/talk.md", []string{"shell"}, later)

	if len(settings.Approvals) != 1 {
		t.Fatalf("approvals = %+v, want one", settings.Approvals)
	}
	approval, found := settings.ApprovalFor("/talks/talk.md")
	if !found {
		t.Fatal("ApprovalFor() found nothing")
	}
	if strings.Join(approval.Drivers, ",") != "shell,sqlite" {
		t.Errorf("drivers = %v, want [shell sqlite]", approval.Drivers)
	}
	if !approval.ApprovedAt.Equal(later) {
		t.Errorf("approvedAt = %v, want %v", approval.ApprovedAt, later)
	}
}

func TestApproveWithNoDriversStoresAnEmptyList(t *testing.T) {
	var settings Settings
	settings.Approve("/talks/new.md", nil, approvalTime)
	approval, _ := settings.ApprovalFor("/talks/new.md")
	if approval.Drivers == nil || len(approval.Drivers) != 0 {
		t.Errorf("drivers = %#v, want an empty, non-nil list", approval.Drivers)
	}
}

func TestRevoke(t *testing.T) {
	var settings Settings
	settings.Approve("/talks/a.md", []string{"shell"}, approvalTime)
	settings.Approve("/talks/b.md", []string{"shell"}, approvalTime)

	if !settings.Revoke("/talks/a.md") {
		t.Error("Revoke() = false for an approved deck")
	}
	if settings.Revoke("/talks/a.md") {
		t.Error("Revoke() = true for a deck that is no longer approved")
	}
	if len(settings.Approvals) != 1 || settings.Approvals[0].Deck != "/talks/b.md" {
		t.Errorf("approvals = %+v, want only b.md", settings.Approvals)
	}
}

func TestApprovalsFileFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	var settings Settings
	settings.Approve("/Users/me/talks/3am/conference-talk.md", []string{"sqlite", "shell"}, approvalTime)
	if err := Save(path, settings); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"approvals:\n",
		"- deck: /Users/me/talks/3am/conference-talk.md\n",
		"drivers: [shell, sqlite]\n",
		"approvedAt: 2026-09-22T19:32:00Z\n",
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("settings file lacks %q:\n%s", want, raw)
		}
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Approved("/Users/me/talks/3am/conference-talk.md", []string{"shell", "sqlite"}) {
		t.Errorf("loaded settings lost the approval: %+v", loaded)
	}
}

func TestRevokingTheLastApprovalDropsTheKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	var settings Settings
	settings.Approve("/talks/a.md", []string{"shell"}, approvalTime)
	settings.Revoke("/talks/a.md")
	if err := Save(path, settings); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "approvals") {
		t.Errorf("settings file = %q, want no approvals key", raw)
	}
}

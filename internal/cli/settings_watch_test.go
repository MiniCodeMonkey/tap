package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// storeFromAnotherProcess saves an approval of drivers for the harness's
// deck the way another tap process does: under the settings lock, merged
// into a fresh read, with the approval key made if there is none.
func storeFromAnotherProcess(t *testing.T, harness *gateHarness, drivers ...usersettings.Driver) {
	t.Helper()
	err := usersettings.WithLock(harness.settings, func() error {
		fresh, err := usersettings.Load(harness.settings)
		if err != nil {
			fresh = usersettings.Settings{}
		}
		fresh.ApproveDrivers(harness.deckKey, withDigests(t, harness.settings, drivers...), approvalNow)
		return usersettings.Save(harness.settings, fresh)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// followSettingsForTest runs the gate's settings watcher until the test ends.
func followSettingsForTest(t *testing.T, harness *gateHarness) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		harness.gate.followSettings(ctx, 20*time.Millisecond)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func (harness *gateHarness) changeCount() int {
	harness.mu.Lock()
	defer harness.mu.Unlock()
	return harness.changes
}

func TestGateHonoursAnApprovalAnotherProcessStoresAfterADecline(t *testing.T) {
	harness := newGateHarness(t, approvedShell)
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver})
	followSettingsForTest(t, harness)
	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.asker.nextRequest(t)
	harness.asker.answer(t, false)
	waitUntil(t, "the no is recorded", func() bool {
		return strings.Contains(harness.out.String(), "Live code is off for python in this run.")
	})
	waitUntil(t, "the gate done asking", func() bool {
		harness.gate.mu.Lock()
		defer harness.gate.mu.Unlock()
		return !harness.gate.asking
	})
	changesBefore := harness.changeCount()

	// tap present, answering during a talk, stores python for this deck.
	storeFromAnotherProcess(t, harness, approvedShell, approvedPython3)

	// No reload of the deck: the settings change alone lets python run.
	waitUntil(t, "python runs", func() bool { return harness.allows("python", python3Driver) })
	waitUntil(t, "open pages told", func() bool { return harness.changeCount() > changesBefore })
	if !harness.allows("shell", shellDriver) {
		t.Error("shell stopped running")
	}
	harness.asker.noRequest(t)
}

func TestGateHonoursAnApprovalAnotherProcessStoresWhileItAsks(t *testing.T) {
	harness := newGateHarness(t, approvedShell)
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver})
	followSettingsForTest(t, harness)
	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.asker.nextRequest(t)
	if harness.allows("python", python3Driver) {
		t.Fatal("python runs before any answer")
	}

	// The question stays open here; another terminal's tap dev approves.
	storeFromAnotherProcess(t, harness, approvedShell, approvedPython3)

	waitUntil(t, "python runs", func() bool { return harness.allows("python", python3Driver) })
	// The open question is withdrawn and nothing is asked again.
	harness.asker.noRequest(t)
}

func TestGateTellsPagesOfAnApprovalStoredElsewhereWhileItAsked(t *testing.T) {
	harness := newGateHarness(t, approvedShell)
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver})
	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.asker.nextRequest(t)
	changesBefore := harness.changeCount()
	// No settings watcher here: the gate reads the settings again once
	// the question is answered, and the other process's yes wins over
	// this run's no.
	storeFromAnotherProcess(t, harness, approvedShell, approvedPython3)
	harness.asker.answer(t, false)
	waitUntil(t, "python runs", func() bool { return harness.allows("python", python3Driver) })
	waitUntil(t, "open pages told", func() bool { return harness.changeCount() > changesBefore })
}

func TestGateSettingsChangeWithNothingNewTellsNoPage(t *testing.T) {
	harness := newGateHarness(t, approvedShell)
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver})
	harness.gate.settingsChanged()
	if changes := harness.changeCount(); changes != 0 {
		t.Errorf("changes = %d, want none: the policy did not change", changes)
	}
}

func TestGateSettingsChangeBeforeStartupDecidesNothing(t *testing.T) {
	harness := newGateHarness(t)
	harness.reload(map[string]config.DriverConfig{"python": python3Driver})
	harness.gate.settingsChanged()
	harness.asker.noRequest(t)
	if changes := harness.changeCount(); changes != 0 {
		t.Errorf("changes = %d, want none before startup", changes)
	}
}

func TestWatchSettingsFileReportsEachSettledChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tap", "settings.yaml")
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		watchSettingsFile(ctx, path, 10*time.Millisecond, func() { calls.Add(1) })
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	time.Sleep(60 * time.Millisecond)
	if got := calls.Load(); got != 0 {
		t.Fatalf("calls = %d before any change, want 0", got)
	}
	// The directory does not exist yet: the first approval makes it.
	if err := usersettings.Save(path, usersettings.Settings{}); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the first save reported", func() bool { return calls.Load() == 1 })
	time.Sleep(80 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls = %d with the file holding still, want 1", got)
	}
	var settings usersettings.Settings
	settings.ApproveDrivers(usersettings.DeckKey{}, []usersettings.Driver{{Name: "shell"}}, approvalNow)
	if err := usersettings.Save(path, settings); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the second save reported", func() bool { return calls.Load() == 2 })
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the removal reported", func() bool { return calls.Load() == 3 })
}

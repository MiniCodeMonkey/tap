package usersettings

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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

func TestSavePreservesTheExistingFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	if err := os.WriteFile(path, []byte("present: {}\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	record := true
	if err := Save(path, Settings{Present: Present{Record: &record}}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("Save changed the file mode to %v, want 0640 preserved", info.Mode().Perm())
	}
}

// TestConcurrentSavesWithoutLockCanLoseAnApproval reproduces the race the
// review found: two callers each Load, then modify, then Save, with
// neither seeing the other's change first. A barrier forces both Loads to
// finish before either Save runs, so this is not a matter of luck: with
// no coordination, whichever Save finishes last always overwrites the
// first save's approval, no matter how the goroutines are scheduled.
func TestConcurrentSavesWithoutLockCanLoseAnApproval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	deckA := deckFile(t, "a.md")
	deckB := deckFile(t, "b.md")

	barrier := make(chan struct{})
	var loaded sync.WaitGroup
	loaded.Add(2)
	var wg sync.WaitGroup
	wg.Add(2)
	save := func(deck DeckKey) {
		defer wg.Done()
		settings, err := Load(path)
		if err != nil {
			t.Error(err)
			return
		}
		// Both goroutines must finish Load before either proceeds to
		// Save, so the loss below is forced, not a matter of luck.
		loaded.Done()
		<-barrier
		settings.Approve(deck, []string{"shell"}, approvalTime)
		if err := Save(path, settings); err != nil {
			t.Error(err)
		}
	}
	go save(deckA)
	go save(deckB)
	loaded.Wait()
	close(barrier)
	wg.Wait()

	settings, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.Approvals) != 1 {
		t.Fatalf("got %d approvals, want exactly 1 lost to the unlocked race: %+v", len(settings.Approvals), settings.Approvals)
	}
}

// TestWithLockKeepsBothConcurrentApprovals is the same race as
// TestConcurrentSavesWithoutLockCanLoseAnApproval, but with each
// load-modify-save sequence run through WithLock. Both approvals survive.
func TestWithLockKeepsBothConcurrentApprovals(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	deckA := deckFile(t, "a.md")
	deckB := deckFile(t, "b.md")

	var wg sync.WaitGroup
	wg.Add(2)
	save := func(deck DeckKey) {
		defer wg.Done()
		err := WithLock(path, func() error {
			settings, err := Load(path)
			if err != nil {
				return err
			}
			settings.Approve(deck, []string{"shell"}, approvalTime)
			return Save(path, settings)
		})
		if err != nil {
			t.Error(err)
		}
	}
	go save(deckA)
	go save(deckB)
	wg.Wait()

	settings, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.Approvals) != 2 {
		t.Fatalf("WithLock did not prevent the lost update: got %d approvals, want 2: %+v", len(settings.Approvals), settings.Approvals)
	}
}

// TestWithLockBreaksAStaleLockFileAndWritesUnderIt reproduces the gap the
// final re-review found: a lock file left behind by a process that was
// killed while holding it. WithLock must not treat that file as an
// impassable obstacle to wait out and then quietly step around; it must
// recognize it as abandoned, remove it, and take a fresh lock of its own
// before running fn, the same as it would with no lock file at all.
func TestWithLockBreaksAStaleLockFileAndWritesUnderIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	lockPath := path + lockSuffix
	if err := os.WriteFile(lockPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * lockStaleAge)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}

	var lockDuringFn os.FileInfo
	err := WithLock(path, func() error {
		lockDuringFn, _ = os.Stat(lockPath)
		return Save(path, Settings{})
	})
	if err != nil {
		t.Fatal(err)
	}
	if lockDuringFn == nil {
		t.Fatal("no lock file was held while fn ran; the stale lock was stepped around instead of broken and replaced")
	}
	if lockDuringFn.ModTime().Equal(old) {
		t.Error("fn ran beside the original stale lock file, not under a fresh one taken in its place")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Errorf("lock file left behind after WithLock returned: %v", err)
	}
}

// TestWithLockBreakingAStaleLockDoesNotLetTwoWritersInAtOnce attacks the
// break itself: many callers start against the same abandoned lock file
// at once, so more than one of them could decide it is stale and try to
// claim it. Only one may ever succeed at a time; if two both proceeded
// into fn believing they held the lock, this catches the overlap.
func TestWithLockBreakingAStaleLockDoesNotLetTwoWritersInAtOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	lockPath := path + lockSuffix
	if err := os.WriteFile(lockPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * lockStaleAge)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}

	const writers = 20
	var active int32
	var overlapped int32
	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			err := WithLock(path, func() error {
				if atomic.AddInt32(&active, 1) > 1 {
					atomic.StoreInt32(&overlapped, 1)
				}
				time.Sleep(5 * time.Millisecond)
				atomic.AddInt32(&active, -1)
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if atomic.LoadInt32(&overlapped) != 0 {
		t.Fatal("two writers ran inside WithLock at once while racing to break the same stale lock")
	}
}

// TestTwentyWayContentionSerializesWithNothingLost guards a property the
// final re-review confirmed by hand with a throwaway test but never
// committed: ordinary contention among many well-behaved writers, with no
// stale lock involved, still serializes correctly and drops nothing.
func TestTwentyWayContentionSerializesWithNothingLost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	const writers = 20
	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		deck := deckFile(t, fmt.Sprintf("deck-%d.md", i))
		go func(deck DeckKey) {
			defer wg.Done()
			err := WithLock(path, func() error {
				settings, err := Load(path)
				if err != nil {
					return err
				}
				settings.Approve(deck, []string{"shell"}, approvalTime)
				return Save(path, settings)
			})
			if err != nil {
				t.Error(err)
			}
		}(deck)
	}
	wg.Wait()

	settings, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.Approvals) != writers {
		t.Fatalf("got %d approvals after 20-way contention, want %d: nothing should be lost", len(settings.Approvals), writers)
	}
	if _, err := os.Stat(path + lockSuffix); !os.IsNotExist(err) {
		t.Errorf("lock file left behind after all writers finished: %v", err)
	}
}

// TestWithLockWarnsOnStderrWhenItFallsBackUnlocked covers the other half
// of the fix: whenever WithLock cannot get the lock and runs fn anyway,
// it must say so, in one sentence a person can act on, rather than doing
// it silently. A read-only settings directory forces the fallback
// immediately, without waiting out the acquire timeout.
func TestWithLockWarnsOnStderrWhenItFallsBackUnlocked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.yaml")
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	ran := false
	stderr := captureStderr(t, func() {
		err := WithLock(path, func() error {
			ran = true
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	if !ran {
		t.Fatal("WithLock did not run fn at all")
	}
	if !strings.Contains(stderr, "lock") {
		t.Errorf("stderr = %q, want a plain-sentence warning about the unlocked fallback", stderr)
	}
}

// TestWithLockNeverHangsForeverOnAFreshLockItCannotAcquire confirms the
// bounded wait the final re-review demanded stays intact: a lock file
// with a current timestamp looks like it could belong to a live writer,
// so it must not be broken as stale, but WithLock still must not wait for
// it indefinitely.
func TestWithLockNeverHangsForeverOnAFreshLockItCannotAcquire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	lockPath := path + lockSuffix
	if err := os.WriteFile(lockPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(lockPath) })

	start := time.Now()
	ran := false
	err := WithLock(path, func() error {
		ran = true
		return nil
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("WithLock did not run fn")
	}
	if elapsed > 10*time.Second {
		t.Fatalf("WithLock waited %v for a lock it could never break or acquire, want a bounded wait", elapsed)
	}
}

// TestWithLockRecoversWhenADirectorySitsAtTheLockPath confirms a
// directory occupying the lock path, old enough to look abandoned, does
// not wedge tap forever: WithLock still returns and still runs fn.
func TestWithLockRecoversWhenADirectorySitsAtTheLockPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	lockPath := path + lockSuffix
	if err := os.Mkdir(lockPath, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * lockStaleAge)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}

	ran := false
	err := WithLock(path, func() error {
		ran = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("WithLock did not run fn")
	}
}

// captureStderr redirects os.Stderr for the duration of fn and returns
// what was written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = original }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

var approvalTime = time.Date(2026, 9, 22, 19, 32, 0, 0, time.UTC)

// deckFile creates a real file named name in a fresh temporary directory
// and returns its DeckKey. Approved, Approve, ApprovalFor and Revoke only
// take a DeckKey, and ResolveDeck can only produce one for a file that
// actually exists, so every test that exercises those methods needs a
// real file rather than a fictional path such as "/talks/talk.md".
func deckFile(t *testing.T, name string) DeckKey {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	key, err := ResolveDeck(path)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestApprovedNeedsEveryDeclaredDriver(t *testing.T) {
	deck := deckFile(t, "talk.md")
	elsewhere := deckFile(t, "talk.md")

	var settings Settings
	settings.Approve(deck, []string{"sqlite"}, approvalTime)

	if !settings.Approved(deck, []string{"sqlite"}) {
		t.Error("the approved deck and driver should be approved")
	}
	if settings.Approved(deck, []string{"shell", "sqlite"}) {
		t.Error("a driver added after the approval should not be approved")
	}
	if settings.Approved(elsewhere, []string{"sqlite"}) {
		t.Error("a different deck should not be approved")
	}

	sameAgain, err := ResolveDeck(deck.String())
	if err != nil {
		t.Fatal(err)
	}
	if !settings.Approved(sameAgain, nil) {
		t.Error("resolving the same deck again should still match its approval")
	}
}

func TestApproveKeepsTheDriversApprovedBefore(t *testing.T) {
	deck := deckFile(t, "talk.md")
	var settings Settings
	settings.Approve(deck, []string{"sqlite"}, approvalTime)
	later := approvalTime.Add(time.Hour)
	settings.Approve(deck, []string{"shell"}, later)

	if len(settings.Approvals) != 1 {
		t.Fatalf("approvals = %+v, want one", settings.Approvals)
	}
	approval, found := settings.ApprovalFor(deck)
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
	deck := deckFile(t, "new.md")
	var settings Settings
	settings.Approve(deck, nil, approvalTime)
	approval, _ := settings.ApprovalFor(deck)
	if approval.Drivers == nil || len(approval.Drivers) != 0 {
		t.Errorf("drivers = %#v, want an empty, non-nil list", approval.Drivers)
	}
}

func TestRevoke(t *testing.T) {
	a := deckFile(t, "a.md")
	b := deckFile(t, "b.md")
	var settings Settings
	settings.Approve(a, []string{"shell"}, approvalTime)
	settings.Approve(b, []string{"shell"}, approvalTime)

	if !settings.Revoke(a) {
		t.Error("Revoke() = false for an approved deck")
	}
	if settings.Revoke(a) {
		t.Error("Revoke() = true for a deck that is no longer approved")
	}
	if len(settings.Approvals) != 1 || settings.Approvals[0].Deck != b.String() {
		t.Errorf("approvals = %+v, want only b", settings.Approvals)
	}
}

func TestRevokeStoredPathRemovesADeletedDecksApproval(t *testing.T) {
	// A deck can be revoked after it no longer exists on disk, when
	// ResolveDeck can no longer produce a DeckKey for it at all: the
	// caller copies the exact path from Settings.Approvals instead.
	dir := t.TempDir()
	path := filepath.Join(dir, "gone.md")
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	deck, err := ResolveDeck(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings Settings
	settings.Approve(deck, []string{"shell"}, approvalTime)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveDeck(path); err == nil {
		t.Fatal("ResolveDeck() succeeded for a deleted deck, want an error")
	}

	stored := settings.Approvals[0].Deck
	if !settings.RevokeStoredPath(stored) {
		t.Error("RevokeStoredPath() = false for an approval that is there")
	}
	if len(settings.Approvals) != 0 {
		t.Errorf("approvals = %+v, want none left", settings.Approvals)
	}
	if settings.RevokeStoredPath(stored) {
		t.Error("RevokeStoredPath() = true for an approval that is no longer there")
	}
}

func TestApprovalsFileFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	deck := deckFile(t, "conference-talk.md")
	var settings Settings
	settings.Approve(deck, []string{"sqlite", "shell"}, approvalTime)
	if err := Save(path, settings); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"approvals:\n",
		"- deck: " + deck.String() + "\n",
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
	if !loaded.Approved(deck, []string{"shell", "sqlite"}) {
		t.Errorf("loaded settings lost the approval: %+v", loaded)
	}
}

func TestRevokingTheLastApprovalDropsTheKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	deck := deckFile(t, "a.md")
	var settings Settings
	settings.Approve(deck, []string{"shell"}, approvalTime)
	settings.Revoke(deck)
	if err := Save(path, settings); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "approvals") {
		t.Errorf("settings file = %q, want no approvals key", raw)
	}
}

func TestResolveDeckMatchesASymlinkToItsRealPath(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.md")
	if err := os.WriteFile(real, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	realKey, err := ResolveDeck(real)
	if err != nil {
		t.Fatal(err)
	}
	linkKey, err := ResolveDeck(link)
	if err != nil {
		t.Fatal(err)
	}
	if realKey != linkKey {
		t.Errorf("ResolveDeck(%q) = %v, ResolveDeck(%q) = %v, want the same key", real, realKey, link, linkKey)
	}
}

func TestResolveDeckMatchesASymlinkedDirectoryComponent(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	deck := filepath.Join(realDir, "talk.md")
	if err := os.WriteFile(deck, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkedDir := filepath.Join(dir, "linked")
	if err := os.Symlink(realDir, linkedDir); err != nil {
		t.Fatal(err)
	}
	throughLink := filepath.Join(linkedDir, "talk.md")

	directKey, err := ResolveDeck(deck)
	if err != nil {
		t.Fatal(err)
	}
	throughLinkKey, err := ResolveDeck(throughLink)
	if err != nil {
		t.Fatal(err)
	}
	if directKey != throughLinkKey {
		t.Errorf("ResolveDeck(%q) = %v, ResolveDeck(%q) = %v, want the same key", deck, directKey, throughLink, throughLinkKey)
	}
}

func TestResolveDeckMatchesARelativePathAndOneWithDotDot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "talk.md")
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	absoluteKey, err := ResolveDeck(path)
	if err != nil {
		t.Fatal(err)
	}

	dotDotKey, err := ResolveDeck(filepath.Join(dir, "sub", "..", "talk.md"))
	if err != nil {
		t.Fatal(err)
	}
	if dotDotKey != absoluteKey {
		t.Errorf("a path with .. resolved to %v, want %v", dotDotKey, absoluteKey)
	}

	t.Chdir(dir)
	relativeKey, err := ResolveDeck("talk.md")
	if err != nil {
		t.Fatal(err)
	}
	if relativeKey != absoluteKey {
		t.Errorf("a relative path resolved to %v, want %v", relativeKey, absoluteKey)
	}
}

func TestResolveDeckMatchesADifferentlyCasedSpellingOfTheSameFile(t *testing.T) {
	dir := t.TempDir()
	upper := filepath.Join(dir, "Talk.md")
	if err := os.WriteFile(upper, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	lower := filepath.Join(dir, "talk.md")
	if _, err := os.Stat(lower); err != nil {
		t.Skip("this filesystem is case sensitive, so a different spelling names a different file")
	}

	upperKey, err := ResolveDeck(upper)
	if err != nil {
		t.Fatal(err)
	}
	lowerKey, err := ResolveDeck(lower)
	if err != nil {
		t.Fatal(err)
	}
	if upperKey != lowerKey {
		t.Errorf("ResolveDeck(%q) = %v, ResolveDeck(%q) = %v, want the same key", upper, upperKey, lower, lowerKey)
	}
}

// TestResolveDeckKeepsDistinctFilesDifferingOnlyByCaseSeparate is the
// fail-open direction of the case test above: on a case-sensitive
// filesystem, two files whose names differ only by case are two
// different decks, and resolving them must never collapse them into the
// same key. Getting this backwards would let one deck's approval cover
// another deck's code.
func TestResolveDeckKeepsDistinctFilesDifferingOnlyByCaseSeparate(t *testing.T) {
	dir := t.TempDir()
	upper := filepath.Join(dir, "Talk.md")
	if err := os.WriteFile(upper, []byte("upper"), 0o644); err != nil {
		t.Fatal(err)
	}
	upperInfo, err := os.Stat(upper)
	if err != nil {
		t.Fatal(err)
	}
	lower := filepath.Join(dir, "talk.md")
	if lowerInfo, err := os.Stat(lower); err == nil && os.SameFile(upperInfo, lowerInfo) {
		t.Skip("this filesystem is case insensitive: Talk.md and talk.md name the same file")
	}
	if err := os.WriteFile(lower, []byte("lower"), 0o644); err != nil {
		t.Fatal(err)
	}

	upperKey, err := ResolveDeck(upper)
	if err != nil {
		t.Fatal(err)
	}
	lowerKey, err := ResolveDeck(lower)
	if err != nil {
		t.Fatal(err)
	}
	if upperKey == lowerKey {
		t.Errorf("ResolveDeck(%q) == ResolveDeck(%q) == %v, want two distinct files to resolve to different keys", upper, lower, upperKey)
	}
}

func TestResolveDeckFailsForADeckThatDoesNotExist(t *testing.T) {
	if _, err := ResolveDeck(filepath.Join(t.TempDir(), "gone.md")); err == nil {
		t.Error("ResolveDeck() succeeded for a deck that does not exist, want an error")
	}
}

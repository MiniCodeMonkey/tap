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

	"golang.org/x/sys/unix"
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

// TestWithLockDoesNotBlockOnALockFileLeftByAKilledProcess replaces the
// old age-heuristic test for a killed holder: it used to check that a
// lock file older than lockStaleAge got renamed away and replaced. There
// is no age to guess at now. The kernel drops an flock the instant the
// holding file descriptor closes, which is exactly what happens when a
// process dies, killed or not, so a second file descriptor standing in
// for a dead process's lock (opened, locked, then closed without ever
// calling WithLock) must let the very next WithLock through immediately.
func TestWithLockDoesNotBlockOnALockFileLeftByAKilledProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	lockPath := path + lockSuffix
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	stray, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Flock(int(stray.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	// Closing this descriptor without unlocking stands in for the
	// holding process dying: the kernel releases the flock the moment
	// the descriptor closes, the same as it would on process exit.
	if err := stray.Close(); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	ran := false
	err = WithLock(path, func() error {
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
	if elapsed > 100*time.Millisecond {
		t.Errorf("WithLock took %v to notice the lock was released, want near-instant", elapsed)
	}
}

// TestWithLockBreakingAStaleLockDoesNotLetTwoWritersInAtOnce keeps its
// name and shape from before the fix -- many callers contend at once for
// a lock file that is already sitting on disk, as if left by another
// process -- but no longer marks that file's timestamp as stale: under
// the flock-based lock, an unheld lock file needs no staleness judgment
// at all, it is simply available. Only one caller may ever be inside fn
// at a time; if two both proceeded believing they held the lock, this
// catches the overlap.
func TestWithLockBreakingAStaleLockDoesNotLetTwoWritersInAtOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	lockPath := path + lockSuffix
	if err := os.WriteFile(lockPath, []byte(""), 0o600); err != nil {
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
	// The lock file itself is expected to remain on disk: removing it
	// after each release would race with a caller that already opened
	// it and is about to flock it, since deleting the name does not
	// free the lock held on the still-open file behind it. Leaving an
	// empty, permanently reusable lock file in place is what keeps the
	// flock itself the single source of truth for who holds it.
	if _, err := os.Stat(path + lockSuffix); err != nil {
		t.Errorf("lock file missing after all writers finished: %v", err)
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

// TestWithLockNeverHangsForeverOnALockItCannotAcquire confirms the
// bounded wait stays intact against a lock that is genuinely, currently
// held: a second file descriptor on the same lock file, flocked and
// kept open for the rest of the test, stands in for another live tap
// process. WithLock must still give up after lockAcquireTimeout and run
// fn unlocked rather than wait for a lock that might never be released.
func TestWithLockNeverHangsForeverOnALockItCannotAcquire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	lockPath := path + lockSuffix
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	holder, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Flock(int(holder.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Close() })

	start := time.Now()
	ran := false
	err = WithLock(path, func() error {
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
		t.Fatalf("WithLock waited %v for a lock it could never acquire, want a bounded wait", elapsed)
	}
}

// TestWithLockRecoversWhenADirectorySitsAtTheLockPath confirms a
// directory occupying the lock path does not wedge tap forever: opening
// it as a regular file fails immediately, so WithLock falls back to
// running fn unlocked rather than waiting out the acquire timeout.
func TestWithLockRecoversWhenADirectorySitsAtTheLockPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	lockPath := path + lockSuffix
	if err := os.Mkdir(lockPath, 0o755); err != nil {
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

func TestCoversADriverOnlyWithTheDigestItWasApprovedWith(t *testing.T) {
	deck := deckFile(t, "talk.md")
	key := []byte(strings.Repeat("k", 32))
	python3 := CommandDigest(key, []string{"python3", "-c"})
	var settings Settings
	settings.ApproveDrivers(deck, []Driver{
		{Name: "python", Command: []string{"${PYTHON}", "-c"}, Digest: python3},
		{Name: "shell"},
	}, approvalTime)

	if !settings.Covers(deck, Driver{Name: "python", Digest: python3}) {
		t.Error("python with the approved digest should be covered")
	}
	if settings.Covers(deck, Driver{Name: "python", Digest: CommandDigest(key, []string{"bash", "-c"})}) {
		t.Error("python with a changed command should not be covered")
	}
	if settings.Covers(deck, Driver{Name: "python", Digest: CommandDigest(key, []string{"python3", "-c", "import os"})}) {
		t.Error("python with an added argument should not be covered")
	}
	if settings.Covers(deck, Driver{Name: "python", Digest: CommandDigest([]byte(strings.Repeat("o", 32)), []string{"python3", "-c"})}) {
		t.Error("python digested with another key should not be covered")
	}
	if settings.Covers(deck, Driver{Name: "python"}) {
		t.Error("python with no command should not be covered by an approval of a command")
	}
	if !settings.Covers(deck, Driver{Name: "shell"}) {
		t.Error("the built-in shell driver should be covered")
	}
	if settings.Covers(deck, Driver{Name: "shell", Digest: CommandDigest(key, []string{"sh"})}) {
		t.Error("shell with a command should not be covered by an approval without one")
	}
}

func TestCommandDigestKeepsArgumentBoundaries(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	if CommandDigest(key, []string{"sh", "-c", "a b"}) == CommandDigest(key, []string{"sh", "-c", "a", "b"}) {
		t.Error("two different argument lists have the same digest")
	}
	if CommandDigest(key, nil) != "" {
		t.Error("no command should have no digest")
	}
	if !strings.HasPrefix(CommandDigest(key, []string{"sh"}), "hmac-sha256:") {
		t.Errorf("digest = %q, want the hmac-sha256: prefix", CommandDigest(key, []string{"sh"}))
	}
}

func TestAnApprovalWithoutDigestsCoversNoCustomCommand(t *testing.T) {
	deck := deckFile(t, "talk.md")
	path := filepath.Join(t.TempDir(), "settings.yaml")
	record := fmt.Sprintf("approvals:\n  - deck: %s\n    drivers: [python, shell]\n    approvedAt: 2026-09-22T19:32:00Z\n", deck.String())
	if err := os.WriteFile(path, []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	key := []byte(strings.Repeat("k", 32))
	if settings.Covers(deck, Driver{Name: "python", Digest: CommandDigest(key, []string{"python3", "-c"})}) {
		t.Error("a record written before commands were stored covered a custom command")
	}
	if !settings.Covers(deck, Driver{Name: "shell"}) {
		t.Error("a record written before commands were stored should still cover a built-in driver")
	}
}

func TestApproveDriversReplacesTheCommandOfAReapprovedDriver(t *testing.T) {
	deck := deckFile(t, "talk.md")
	path := filepath.Join(t.TempDir(), "settings.yaml")
	key := []byte(strings.Repeat("k", 32))
	python3 := CommandDigest(key, []string{"python3", "-c"})
	bash := CommandDigest(key, []string{"bash", "-c"})
	ruby := CommandDigest(key, []string{"ruby"})
	var settings Settings
	settings.ApproveDrivers(deck, []Driver{{Name: "python", Command: []string{"python3", "-c"}, Digest: python3}, {Name: "ruby", Command: []string{"ruby"}, Digest: ruby}}, approvalTime)
	settings.ApproveDrivers(deck, []Driver{{Name: "python", Command: []string{"bash", "-c"}, Digest: bash}}, approvalTime)
	if err := Save(path, settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Covers(deck, Driver{Name: "python", Digest: bash}) {
		t.Error("the new command was not stored")
	}
	if loaded.Covers(deck, Driver{Name: "python", Digest: python3}) {
		t.Error("the replaced command is still covered")
	}
	if !loaded.Covers(deck, Driver{Name: "ruby", Digest: ruby}) {
		t.Error("a driver approved before lost its command")
	}
	approval, _ := loaded.ApprovalFor(deck)
	if strings.Join(approval.Commands["python"], " ") != "bash -c" {
		t.Errorf("commands = %v, want the bash template for display", approval.Commands)
	}
}

func TestEnsureApprovalKeyCreatesAPrivateKeyOnce(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "tap", "settings.yaml")
	if _, err := LoadApprovalKey(settingsPath); err == nil {
		t.Fatal("LoadApprovalKey() found a key that was never made")
	}
	key, err := EnsureApprovalKey(settingsPath)
	if err != nil || len(key) != 32 {
		t.Fatalf("EnsureApprovalKey() = %d bytes, %v", len(key), err)
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(settingsPath), "approval.key"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("key file = %v, %v; want mode 0600", info, err)
	}
	again, err := EnsureApprovalKey(settingsPath)
	if err != nil || string(again) != string(key) {
		t.Error("a second EnsureApprovalKey() changed the key")
	}
	loaded, err := LoadApprovalKey(settingsPath)
	if err != nil || string(loaded) != string(key) {
		t.Errorf("LoadApprovalKey() = %v, want the key made before", err)
	}
}

func TestEnsureApprovalKeyReplacesAKeyOfTheWrongLength(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.yaml")
	if err := os.WriteFile(ApprovalKeyPath(settingsPath), []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApprovalKey(settingsPath); err == nil {
		t.Error("LoadApprovalKey() accepted a key of the wrong length")
	}
	key, err := EnsureApprovalKey(settingsPath)
	if err != nil || len(key) != 32 {
		t.Fatalf("EnsureApprovalKey() = %d bytes, %v", len(key), err)
	}
}

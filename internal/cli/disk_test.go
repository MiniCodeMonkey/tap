package cli

import (
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
)

func TestDiskWatchReportsOnlyChanges(t *testing.T) {
	free := uint64(20 << 30)
	var levels []recorder.DiskLevel
	watch := newDiskWatch(t.TempDir(), func(string) (uint64, error) { return free, nil }, func(level recorder.DiskLevel) {
		levels = append(levels, level)
	})

	watch.check()
	free = 4 << 30
	watch.check()
	watch.check()
	free = 512 << 20
	watch.check()

	want := []recorder.DiskLevel{recorder.DiskOK, recorder.DiskLow, recorder.DiskFull}
	if len(levels) != len(want) || levels[0] != want[0] || levels[1] != want[1] || levels[2] != want[2] {
		t.Errorf("levels = %v, want %v", levels, want)
	}
}

// TestDiskWatchFirstCheckAlwaysReportsEvenAtDiskOK covers the ruling that a
// fresh watch (the one Start makes for a recording after a previous
// DiskFull stop, for example) must announce DiskOK on its own first check
// rather than silently matching the DiskOK zero value and staying quiet.
func TestDiskWatchFirstCheckAlwaysReportsEvenAtDiskOK(t *testing.T) {
	var levels []recorder.DiskLevel
	watch := newDiskWatch(t.TempDir(), func(string) (uint64, error) { return 50 << 30, nil }, func(level recorder.DiskLevel) {
		levels = append(levels, level)
	})

	watch.check()

	if len(levels) != 1 || levels[0] != recorder.DiskOK {
		t.Errorf("levels = %v, want a single DiskOK report", levels)
	}
}

func TestDiskStatusName(t *testing.T) {
	if diskStatusName(recorder.DiskOK) != "" || diskStatusName(recorder.DiskLow) != "low" || diskStatusName(recorder.DiskFull) != "full" {
		t.Error("disk status names do not match the hub's")
	}
}

package cli

import (
	"context"
	"sync"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
)

// diskPollInterval is how often a running recording checks free space.
const diskPollInterval = 10 * time.Second

// diskLevelUnset is not a valid recorder.DiskLevel. A fresh diskWatch
// starts here instead of at DiskOK's zero value, so its first check always
// reports its level instead of silently matching an unset level that looks
// the same as DiskOK: without that, a watch started right after a
// DiskFull stop (tap dev's next recording, say) would find DiskOK on its
// first check, see no change from the zero value, and never tell anyone
// the disk is fine again, leaving the "disk full" status up throughout the
// new recording.
const diskLevelUnset recorder.DiskLevel = -1

// diskWatch reports each change of the recordings disk's level. The first
// check always reports its level.
type diskWatch struct {
	dir     string
	free    func(string) (uint64, error)
	onLevel func(recorder.DiskLevel)

	mu    sync.Mutex
	level recorder.DiskLevel
}

func newDiskWatch(dir string, free func(string) (uint64, error), onLevel func(recorder.DiskLevel)) *diskWatch {
	if free == nil {
		free = recorder.FreeSpace
	}
	return &diskWatch{dir: dir, free: free, onLevel: onLevel, level: diskLevelUnset}
}

// check reads free space once. An unreadable volume leaves the level as
// it was: a failed read is not evidence of a full disk.
func (w *diskWatch) check() {
	free, err := w.free(w.dir)
	if err != nil {
		return
	}
	level := recorder.DiskLevelFor(free)

	w.mu.Lock()
	changed := level != w.level
	w.level = level
	w.mu.Unlock()

	if changed && w.onLevel != nil {
		w.onLevel(level)
	}
}

// currentLevel reads the last level reported, under the mutex.
func (w *diskWatch) currentLevel() recorder.DiskLevel {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.level
}

// run checks at once and then every interval until ctx ends.
func (w *diskWatch) run(ctx context.Context, interval time.Duration) {
	w.check()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.check()
		}
	}
}

// diskStatusName is the hub's name for a level.
func diskStatusName(level recorder.DiskLevel) string {
	switch level {
	case recorder.DiskLow:
		return "low"
	case recorder.DiskFull:
		return "full"
	default:
		return ""
	}
}

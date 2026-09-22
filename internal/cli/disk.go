package cli

import (
	"context"
	"sync"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
)

// diskPollInterval is how often a running recording checks free space.
const diskPollInterval = 10 * time.Second

// diskWatch reports each change of the recordings disk's level. The first
// check reports only when the level is not DiskOK.
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
	return &diskWatch{dir: dir, free: free, onLevel: onLevel}
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

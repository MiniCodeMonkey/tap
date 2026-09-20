//go:build darwin

package recorder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Supported reports whether this platform can record.
func Supported() bool { return true }

// Preflight checks everything that can stop a recording, before the
// speaker is on stage. It is quiet on success; every finding carries a fix,
// because none of these can be repaired from inside Tap.
func Preflight(outputDir string, options Options) Report {
	return buildReport(checks{
		screenPermission: checkScreenPermission(),
		audioInputs:      parseAudioInputCount(audioReport()),
		audioUID:         ValidateAudioUID(options.AudioUID),
		outputWritable:   checkOutputWritable(outputDir),
		freeBytes:        freeSpace(outputDir),
	})
}

// checkScreenPermission takes a 1x1 still. Without the Screen Recording
// grant this fails, which is the cheapest way to find out: it returns in
// well under a second and writes a file small enough to be free.
func checkScreenPermission() error {
	probe := filepath.Join(os.TempDir(), "tap-permission-probe.png")
	defer func() { _ = os.Remove(probe) }()

	if output, err := exec.Command(defaultCommand, "-x", "-R0,0,1,1", probe).CombinedOutput(); err != nil {
		return fmt.Errorf("%s", output)
	}
	if _, err := os.Stat(probe); err != nil {
		return fmt.Errorf("the screen could not be captured")
	}
	return nil
}

// checkOutputWritable makes sure a recording has somewhere to land. Only the
// write decides the verdict; removing the probe afterward is housekeeping
// and must not fail the check.
func checkOutputWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	probe := filepath.Join(dir, ".tap-write-probe")
	defer func() { _ = os.Remove(probe) }()

	return os.WriteFile(probe, []byte("ok"), 0o600)
}

// freeSpace is free space on the volume holding dir, or 0 when it cannot
// be read.
func freeSpace(dir string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0
	}
	return stat.Bavail * uint64(stat.Bsize)
}

package recorder

// DiskLevel is how close the recordings volume is to full.
type DiskLevel int

const (
	// DiskOK leaves more than DiskWarnBelow free.
	DiskOK DiskLevel = iota
	// DiskLow is below DiskWarnBelow: warn, keep recording.
	DiskLow
	// DiskFull is below DiskStopBelow: stop recording while macOS still
	// has room to finalize the file and keep running.
	DiskFull
)

const (
	// DiskWarnBelow is about two more hours of screen recording.
	DiskWarnBelow uint64 = 5 << 30
	// DiskStopBelow is where recording stops.
	DiskStopBelow uint64 = 1 << 30
)

// DiskLevelFor maps free bytes to a level.
func DiskLevelFor(free uint64) DiskLevel {
	switch {
	case free < DiskStopBelow:
		return DiskFull
	case free < DiskWarnBelow:
		return DiskLow
	default:
		return DiskOK
	}
}

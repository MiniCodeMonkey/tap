package recorder

// lowDiskThreshold is where free space stops being comfortable. Roughly a
// 45 minute retina recording.
const lowDiskThreshold = 5 << 30

// checks is what the platform code found. Assembling the report from it is
// platform-neutral, so the wording is tested everywhere.
type checks struct {
	// screenPermission is non-nil when a still capture failed, which is
	// what a missing Screen Recording grant looks like.
	screenPermission error
	// audioUID is non-nil when a configured CoreAudio UID was rejected.
	audioUID error
	// outputWritable is non-nil when the output directory cannot be used.
	outputWritable error
	// audioInputs is how many capture devices exist.
	audioInputs int
	// freeBytes is free space on the output volume.
	freeBytes uint64
}

// buildReport turns raw check results into findings a speaker can act on.
// Only the two failures that make a recording worthless block it: no screen
// and no usable output. A missing microphone is a warning, because a silent
// recording of a demo still beats nothing.
func buildReport(found checks) Report {
	var report Report

	if found.screenPermission != nil {
		report.Findings = append(report.Findings, Finding{
			Message:  "Screen Recording permission is missing",
			Fix:      "Grant it to your terminal in System Settings > Privacy & Security > Screen Recording, then restart the terminal",
			Blocking: true,
		})
	}

	if found.outputWritable != nil {
		report.Findings = append(report.Findings, Finding{
			Message:  "The recordings directory cannot be written: " + found.outputWritable.Error(),
			Fix:      "Set recording.output in the deck frontmatter to a writable directory",
			Blocking: true,
		})
	}

	if found.audioUID != nil {
		report.Findings = append(report.Findings, Finding{
			Message:  found.audioUID.Error(),
			Fix:      "Set recording.audio to default, or to a CoreAudio UID this machine has",
			Blocking: true,
		})
	}

	if found.audioInputs == 0 {
		report.Findings = append(report.Findings, Finding{
			Message: "No microphone is attached, so the recording will be silent",
			Fix:     "Attach a microphone, or set recording.audio to none to stop asking",
		})
	}

	if found.freeBytes > 0 && found.freeBytes < lowDiskThreshold {
		report.Findings = append(report.Findings, Finding{
			Message: "Less than 5 GB of disk space free on the recording volume, which is about 45 minutes",
			Fix:     "Free some space before a long talk",
		})
	}

	return report
}

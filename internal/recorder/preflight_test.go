package recorder

import (
	"errors"
	"strings"
	"testing"
)

func findingsText(report Report) string {
	var parts []string
	for _, finding := range report.Findings {
		parts = append(parts, finding.Message+" "+finding.Fix)
	}
	return strings.Join(parts, "\n")
}

func TestBuildReportPassesACleanMachine(t *testing.T) {
	report := buildReport(checks{audioInputs: 1, freeBytes: 50 << 30})

	if len(report.Findings) != 0 {
		t.Errorf("buildReport() returned %d findings, want none: %s", len(report.Findings), findingsText(report))
	}
	if report.Blocked() {
		t.Error("a clean machine reports as blocked")
	}
}

func TestBuildReportBlocksWithoutScreenPermission(t *testing.T) {
	report := buildReport(checks{
		screenPermission: errors.New("could not create image from display"),
		audioInputs:      1,
		freeBytes:        50 << 30,
	})

	if !report.Blocked() {
		t.Fatal("a missing screen permission does not block")
	}
	if !strings.Contains(findingsText(report), "Screen Recording") {
		t.Errorf("the finding does not mention Screen Recording: %s", findingsText(report))
	}
	if !strings.Contains(findingsText(report), "restart") {
		t.Errorf("the fix does not mention restarting the terminal: %s", findingsText(report))
	}
}

func TestBuildReportBlocksOnARejectedAudioUID(t *testing.T) {
	report := buildReport(checks{audioUID: errors.New("audio device \"lav\" not found"), audioInputs: 1, freeBytes: 50 << 30})

	if !report.Blocked() {
		t.Fatal("a rejected audio UID does not block")
	}
}

func TestBuildReportWarnsWithoutAnAudioInput(t *testing.T) {
	report := buildReport(checks{audioInputs: 0, freeBytes: 50 << 30})

	if report.Blocked() {
		t.Error("a missing microphone blocks, want a warning: a silent recording is still worth having")
	}
	if len(report.Findings) != 1 {
		t.Fatalf("buildReport() returned %d findings, want 1", len(report.Findings))
	}
}

func TestBuildReportWarnsOnLowDisk(t *testing.T) {
	report := buildReport(checks{audioInputs: 1, freeBytes: 2 << 30})

	if report.Blocked() {
		t.Error("low disk blocks, want a warning")
	}
	if !strings.Contains(findingsText(report), "disk") {
		t.Errorf("the finding does not mention disk space: %s", findingsText(report))
	}
}

func TestBuildReportBlocksOnAnUnwritableOutputDirectory(t *testing.T) {
	report := buildReport(checks{audioInputs: 1, freeBytes: 50 << 30, outputWritable: errors.New("permission denied")})

	if !report.Blocked() {
		t.Fatal("an unwritable output directory does not block")
	}
}

package recorder

import (
	"reflect"
	"testing"
)

func TestBuildArgsDefaultAudio(t *testing.T) {
	got := buildArgs(Options{OutputPath: "/tmp/talk.mov", Display: 1})
	want := []string{"-v", "-g", "-C", "-x", "-D1", "/tmp/talk.mov"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildArgs() = %v, want %v", got, want)
	}
}

func TestBuildArgsNamedAudioDevice(t *testing.T) {
	got := buildArgs(Options{OutputPath: "/tmp/talk.mov", Display: 2, AudioUID: "BuiltInMicrophoneDevice"})
	want := []string{"-v", "-GBuiltInMicrophoneDevice", "-C", "-x", "-D2", "/tmp/talk.mov"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildArgs() = %v, want %v", got, want)
	}
}

func TestBuildArgsWithoutAudio(t *testing.T) {
	got := buildArgs(Options{OutputPath: "/tmp/talk.mov", Display: 1, NoAudio: true})
	want := []string{"-v", "-C", "-x", "-D1", "/tmp/talk.mov"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildArgs() = %v, want %v", got, want)
	}
}

func TestBuildArgsShowClicks(t *testing.T) {
	got := buildArgs(Options{OutputPath: "/tmp/talk.mov", Display: 1, ShowClicks: true})
	want := []string{"-v", "-g", "-C", "-x", "-k", "-D1", "/tmp/talk.mov"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildArgs() = %v, want %v", got, want)
	}
}

func TestBuildArgsLimitSeconds(t *testing.T) {
	got := buildArgs(Options{OutputPath: "/tmp/test.mov", Display: 1, LimitSeconds: 5})
	want := []string{"-v", "-g", "-C", "-x", "-V5", "-D1", "/tmp/test.mov"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildArgs() = %v, want %v", got, want)
	}
}

func TestBuildArgsTreatsDisplayZeroAsMain(t *testing.T) {
	got := buildArgs(Options{OutputPath: "/tmp/talk.mov"})

	if got[len(got)-2] != "-D1" {
		t.Errorf("display flag = %q, want -D1", got[len(got)-2])
	}
}

func TestReportBlockedOnlyForBlockingFindings(t *testing.T) {
	warning := Report{Findings: []Finding{{Message: "low disk"}}}
	if warning.Blocked() {
		t.Error("a warning-only report reports as blocked")
	}

	blocker := Report{Findings: []Finding{{Message: "no permission", Blocking: true}}}
	if !blocker.Blocked() {
		t.Error("a blocking report does not report as blocked")
	}
}

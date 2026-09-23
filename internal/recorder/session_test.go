package recorder

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeFakeRecorder writes a shell script that stands in for screencapture.
// The body decides how it behaves when signalled.
func writeFakeRecorder(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fake-recorder")
	script := "#!/bin/sh\nfor last in \"$@\"; do :; done\n" + body
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// obedientRecorder writes its output file and exits when interrupted, the
// way screencapture finalizes a .mov on SIGINT. It signals readiness by
// writing a marker file once its trap is installed, so a test can wait for
// that instead of racing the trap against a freshly launched shell.
func obedientRecorder(t *testing.T) string {
	return writeFakeRecorder(t, "trap 'printf recorded > \"$last\"; exit 0' INT\nprintf ready > \"$last.ready\"\nsleep 60 &\nwait $!\n")
}

// waitForRecorderReady blocks until the fake recorder at outputPath has
// signalled that its trap is installed and it is actually running, so
// Stop() is exercised against a live recorder rather than one still
// starting up.
func waitForRecorderReady(t *testing.T, outputPath string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(outputPath + ".ready"); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the fake recorder never signalled that it was running")
}

func TestStopFinalizesTheRecording(t *testing.T) {
	output := filepath.Join(t.TempDir(), "talk.mov")

	session, err := Start(Options{Command: obedientRecorder(t), OutputPath: output, Display: 1})
	if err != nil {
		t.Fatalf("Start() returned %v", err)
	}
	waitForRecorderReady(t, output)

	result, err := session.Stop()
	if err != nil {
		t.Fatalf("Stop() returned %v", err)
	}

	if result.Path != output {
		t.Errorf("Result.Path = %q, want %q", result.Path, output)
	}
	if result.Truncated {
		t.Error("Result.Truncated is true, want false for a clean stop")
	}
	if result.Size == 0 {
		t.Error("Result.Size is 0, so the file was never written")
	}
	if result.Duration <= 0 {
		t.Error("Result.Duration is not positive")
	}
}

func TestStopKillsARecorderThatIgnoresInterrupt(t *testing.T) {
	output := filepath.Join(t.TempDir(), "talk.mov")
	stubborn := writeFakeRecorder(t, "trap '' INT\nprintf ready > \"$last.ready\"\nsleep 60 &\nwait $!\n")

	previousGrace := KillGrace
	KillGrace = 200 * time.Millisecond
	t.Cleanup(func() { KillGrace = previousGrace })

	session, err := Start(Options{Command: stubborn, OutputPath: output, Display: 1})
	if err != nil {
		t.Fatalf("Start() returned %v", err)
	}
	waitForRecorderReady(t, output)

	result, err := session.Stop()
	if err != nil {
		t.Fatalf("Stop() returned %v", err)
	}
	if !result.Truncated {
		t.Error("Result.Truncated is false, want true after a kill")
	}
}

func TestDoneClosesWhenTheRecorderExitsOnItsOwn(t *testing.T) {
	output := filepath.Join(t.TempDir(), "talk.mov")
	quitter := writeFakeRecorder(t, "exit 3\n")

	session, err := Start(Options{Command: quitter, OutputPath: output, Display: 1})
	if err != nil {
		t.Fatalf("Start() returned %v", err)
	}

	select {
	case <-session.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Done() never closed after the recorder exited")
	}

	if session.ExitError() == nil {
		t.Error("ExitError() is nil, want the non-zero exit")
	}
}

func TestStopAfterAnUnexpectedExitStillReturns(t *testing.T) {
	output := filepath.Join(t.TempDir(), "talk.mov")
	quitter := writeFakeRecorder(t, "printf partial > \"$last\"\nexit 3\n")

	session, err := Start(Options{Command: quitter, OutputPath: output, Display: 1})
	if err != nil {
		t.Fatalf("Start() returned %v", err)
	}
	<-session.Done()

	result, err := session.Stop()
	if err != nil {
		t.Fatalf("Stop() returned %v", err)
	}
	if result.Size == 0 {
		t.Error("Result.Size is 0, want the partial file measured")
	}
}

func TestConcurrentStopsAgreeOnTheResult(t *testing.T) {
	output := filepath.Join(t.TempDir(), "talk.mov")

	session, err := Start(Options{Command: obedientRecorder(t), OutputPath: output, Display: 1})
	if err != nil {
		t.Fatalf("Start() returned %v", err)
	}
	waitForRecorderReady(t, output)

	results := make(chan Result, 2)
	for range 2 {
		go func() {
			result, stopErr := session.Stop()
			if stopErr != nil {
				t.Errorf("Stop() returned %v", stopErr)
			}
			results <- result
		}()
	}

	first, second := <-results, <-results
	if first.Path != output || second.Path != output {
		t.Errorf("concurrent Stop() returned %q and %q, want both %q", first.Path, second.Path, output)
	}
	if first.Size == 0 || second.Size == 0 {
		t.Error("a concurrent Stop() returned an empty result")
	}
}

func TestStartRejectsAMissingRecorder(t *testing.T) {
	_, err := Start(Options{Command: "/nonexistent/recorder", OutputPath: filepath.Join(t.TempDir(), "x.mov")})
	if err == nil {
		t.Fatal("Start() accepted a missing binary, want an error")
	}
}

func TestElapsedGrows(t *testing.T) {
	session, err := Start(Options{Command: obedientRecorder(t), OutputPath: filepath.Join(t.TempDir(), "talk.mov")})
	if err != nil {
		t.Fatalf("Start() returned %v", err)
	}
	t.Cleanup(func() { _, _ = session.Stop() })

	time.Sleep(20 * time.Millisecond)
	if session.Elapsed() <= 0 {
		t.Error("Elapsed() is not positive while recording")
	}
}

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
)

// recordCmd groups the commands that work on recordings tap made.
var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Work with recordings made by tap present",
	Args:  cobra.ArbitraryArgs,
	RunE:  runUnknownGroupSubcommand,
}

// recordFinishCmd turns a recording folder into an upload-ready talk.
var recordFinishCmd = &cobra.Command{
	Use:   "finish <folder>",
	Short: "Join a recording's segments and cut an upload-ready talk.mp4",
	Long: `Finish a recording folder made by tap present. It needs ffmpeg
(brew install ffmpeg) and writes these next to the segments:

  talk.mov            every segment joined without re-encoding (skipped
                      when there is only one segment)
  talk.mp4            the talk from the "Talk starts" mark, or from the
                      start when there is none, with the video copied and
                      the audio normalized to -14 LUFS for YouTube
  talk-chapters.txt   a YouTube chapter list timed to talk.mp4

The segments and chapters.txt are not changed. tap present runs this by
itself when a recording ends; run it by hand on an older recording, or
when ffmpeg was not installed at the time.

Examples:
  tap record finish recordings/my-talk-2026-09-21-1932`,
	Args: cobra.ExactArgs(1),
	RunE: runRecordFinish,
}

func init() {
	recordCmd.AddCommand(recordFinishCmd)
	rootCmd.AddCommand(recordCmd)
}

func runRecordFinish(cmd *cobra.Command, args []string) error {
	dir, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		return userError(codeFileNotFound, fmt.Errorf("%s is not a folder", args[0]))
	}

	err = recorder.Export(dir, cmd.ErrOrStderr())
	if errors.Is(err, recorder.ErrFFmpegNotInstalled) {
		return userError(codeFailed, ffmpegMissingError(dir))
	}
	if err != nil {
		return userError(codeExportFailed, err)
	}
	Success("Wrote %s and %s in %s\n", recorder.TalkName, recorder.TalkChapters, dir)
	return nil
}

// ffmpegMissingError tells the user how to finish a recording by hand.
func ffmpegMissingError(dir string) error {
	return fmt.Errorf("ffmpeg is not installed, so the raw files are kept as they are.\n\n    %s\n    tap record finish %q\n\nrun those to make talk.mp4", recorder.FFmpegInstallHint(), dir)
}

// exportLogName is the file in a recording folder that records what the
// detached export printed.
const exportLogName = "finish.log"

// startRecordingExport runs "tap record finish" on dir in a process of
// its own, so a minute or more of ffmpeg outlives tap, which the app
// quits with a deadline. It returns recorder.ErrFFmpegNotInstalled
// without starting anything when ffmpeg is missing. The child's output
// goes to finish.log in dir, and the path to that file is returned.
func startRecordingExport(dir string) (string, error) {
	if !recorder.FFmpegAvailable() {
		return "", recorder.ErrFFmpegNotInstalled
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	logPath := filepath.Join(dir, exportLogName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	defer logFile.Close()

	command := exec.Command(executable, "record", "finish", dir)
	command.Stdout = logFile
	command.Stderr = logFile
	detachProcess(command)
	if err := command.Start(); err != nil {
		return "", err
	}
	return logPath, command.Process.Release()
}

// exportEvent starts the export of a kept recording and says what came of
// it. A missing ffmpeg is not a failure: the raw files are all there.
func exportEvent(dir string) appRecordingExportEvent {
	event := appRecordingExportEvent{Type: appEventRecordingExport, Directory: dir}
	logPath, err := startRecordingExport(dir)
	switch {
	case err == nil:
		event.State, event.Log = "started", logPath
		event.Message = "Exporting talk.mp4 in the background; progress is in " + logPath
	case errors.Is(err, recorder.ErrFFmpegNotInstalled):
		event.State = "unavailable"
		event.Message = "ffmpeg is not installed, so the raw files are kept as they are. Run `" +
			recorder.FFmpegInstallHint() + "` then `tap record finish " + dir + "` to make talk.mp4."
	default:
		event.State = "failed"
		event.Message = "Could not start the export: " + err.Error()
	}
	return event
}

// printExport tells a person at the terminal what exportEvent decided.
func printExport(event appRecordingExportEvent) {
	if event.State == "started" {
		Muted("  %s\n", event.Message)
		return
	}
	Warning("  %s\n", event.Message)
}

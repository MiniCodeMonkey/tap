package recorder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Names of the files Export writes next to the segments.
const (
	JoinedName   = "talk.mov"
	TalkName     = "talk.mp4"
	TalkChapters = "talk-chapters.txt"
)

// Loudness target for YouTube.
const (
	targetLoudness = "-14"
	targetPeak     = "-1"
)

// ErrFFmpegNotInstalled is returned by Export when ffmpeg or ffprobe is
// not on PATH. Callers show FFmpegInstallHint alongside it.
var ErrFFmpegNotInstalled = errors.New("ffmpeg is not installed")

// FFmpegInstallHint returns the one line to tell the user to run, for the
// platform this is running on.
func FFmpegInstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install ffmpeg"
	case "windows":
		return "winget install --id Gyan.FFmpeg"
	default:
		return "see https://ffmpeg.org/download.html"
	}
}

// FFmpegAvailable reports whether ffmpeg and ffprobe can be found on PATH.
func FFmpegAvailable() bool {
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			return false
		}
	}
	return true
}

// loudness is the first loudnorm pass's measurement of a clip.
type loudness struct {
	Integrated string `json:"input_i"`
	TruePeak   string `json:"input_tp"`
	Range      string `json:"input_lra"`
	Threshold  string `json:"input_thresh"`
	Offset     string `json:"target_offset"`
}

// concatArgs joins the segments listed in listPath without re-encoding.
func concatArgs(listPath, output string) []string {
	return []string{"-hide_banner", "-nostats", "-loglevel", "warning", "-y",
		"-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", output}
}

// measureArgs is the loudnorm analysis pass over the audio from start on.
func measureArgs(input string, start time.Duration) []string {
	return []string{"-hide_banner", "-nostats", "-y",
		"-ss", seconds(start), "-i", input, "-vn",
		"-af", "loudnorm=I=" + targetLoudness + ":TP=" + targetPeak + ":LRA=11:print_format=json",
		"-f", "null", "-"}
}

// encodeArgs cuts input at start with the video stream copied and the
// audio normalized by a linear second loudnorm pass. start is a keyframe's
// own time, so the copy begins on that keyframe. loudnorm resamples to
// 192 kHz, so the rate is pinned back to 48 kHz. measured is nil for a
// recording without audio.
func encodeArgs(input string, start time.Duration, output string, measured *loudness) []string {
	arguments := []string{"-hide_banner", "-nostats", "-loglevel", "warning", "-y",
		"-ss", seconds(start), "-i", input, "-map", "0:v:0", "-c:v", "copy"}
	if measured == nil {
		arguments = append(arguments, "-an")
	} else {
		filter := "loudnorm=I=" + targetLoudness + ":TP=" + targetPeak + ":LRA=11" +
			":measured_I=" + measured.Integrated +
			":measured_TP=" + measured.TruePeak +
			":measured_LRA=" + measured.Range +
			":measured_thresh=" + measured.Threshold +
			":offset=" + measured.Offset + ":linear=true"
		arguments = append(arguments, "-map", "0:a:0", "-af", filter,
			"-c:a", "aac", "-b:a", "192k", "-ar", "48000")
	}
	return append(arguments, "-movflags", "+faststart", output)
}

func seconds(offset time.Duration) string {
	return strconv.FormatFloat(offset.Seconds(), 'f', 6, 64)
}

// Export finishes a recording folder: it joins the segments into
// talk.mov, cuts talk.mp4 from the talk start with loudness normalized
// for YouTube, and writes talk-chapters.txt timed to that video. The
// segments and chapters.txt are left as they are. Progress goes to log.
func Export(dir string, log io.Writer) error {
	if !FFmpegAvailable() {
		return ErrFFmpegNotInstalled
	}

	names, err := segmentNames(dir)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("no segments (01.mov, 02.mov, ...) in %s", dir)
	}

	starts := map[string]time.Duration{}
	var total time.Duration
	for _, name := range names {
		duration, err := probeDuration(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("reading the length of %s: %w", name, err)
		}
		starts[name] = total
		total += duration
	}

	source := filepath.Join(dir, names[0])
	if len(names) > 1 {
		source = filepath.Join(dir, JoinedName)
		fmt.Fprintf(log, "Joining %d segments into %s\n", len(names), JoinedName)
		if err := joinSegments(dir, names, source, log); err != nil {
			return err
		}
	}

	var joined []chapter
	if text, readErr := os.ReadFile(filepath.Join(dir, "chapters.txt")); readErr == nil {
		joined = joinChapters(parseChapterFile(string(text)), starts)
	}
	// A stream copy can only begin on a keyframe, so the cut is made at the
	// last keyframe at or before the talk start and the chapters are timed
	// from there. Cutting exactly there, rather than at the mark, keeps the
	// video from starting before the audio.
	cut, err := keyframeAtOrBefore(source, talkStart(joined))
	if err != nil {
		return fmt.Errorf("finding the keyframe before the talk start: %w", err)
	}

	output := filepath.Join(dir, TalkName)
	fmt.Fprintf(log, "Cutting %s from %s with the audio at %s LUFS\n", TalkName, formatOffset(cut), targetLoudness)
	if err := encodeTalk(source, cut, output, log); err != nil {
		return err
	}

	if len(joined) == 0 {
		fmt.Fprintln(log, "No chapters in chapters.txt, so no chapter list")
		return nil
	}
	chaptersPath := filepath.Join(dir, TalkChapters)
	if err := os.WriteFile(chaptersPath, []byte(talkChapters(joined, cut)), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", TalkChapters, err)
	}
	fmt.Fprintf(log, "Done: %s, %s\n", TalkName, TalkChapters)
	return nil
}

// keyframeAtOrBefore finds the time of the last video keyframe at or
// before at. It lists every video packet, which ffprobe does without
// decoding, because a seek by time lands somewhere else on a screen
// capture, whose decode times run ahead of its presentation times.
func keyframeAtOrBefore(path string, at time.Duration) (time.Duration, error) {
	output, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "packet=pts_time,flags", "-of", "csv=p=0", path).Output()
	if err != nil {
		return 0, err
	}
	return lastKeyframe(string(output), at), nil
}

// lastKeyframe picks the latest keyframe at or before at from ffprobe's
// "pts_time,flags" lines, or the earliest one when none is that early, or
// at itself when there is none.
func lastKeyframe(packets string, at time.Duration) time.Duration {
	before, first := time.Duration(-1), time.Duration(-1)
	for _, line := range strings.Split(packets, "\n") {
		fields := strings.Split(strings.TrimSpace(line), ",")
		if len(fields) < 2 || !strings.Contains(fields[1], "K") {
			continue
		}
		shown, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		found := time.Duration(shown * float64(time.Second))
		if first < 0 || found < first {
			first = found
		}
		if found <= at && found > before {
			before = found
		}
	}
	switch {
	case before >= 0:
		return before
	case first >= 0:
		return first
	}
	return at
}

// segmentNames lists the numbered segment files in a folder, in order.
func segmentNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if segmentNamePattern.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func joinSegments(dir string, names []string, output string, log io.Writer) error {
	list, err := os.CreateTemp("", "tap-concat-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(list.Name())
	for _, name := range names {
		path := strings.ReplaceAll(filepath.Join(dir, name), `'`, `'\''`)
		fmt.Fprintf(list, "file '%s'\n", path)
	}
	if err := list.Close(); err != nil {
		return err
	}
	return runFFmpeg(log, log, concatArgs(list.Name(), output))
}

func encodeTalk(source string, cut time.Duration, output string, log io.Writer) error {
	var measured *loudness
	hasAudio, err := probeHasAudio(source)
	if err != nil {
		return err
	}
	if hasAudio {
		var report bytes.Buffer
		if err := runFFmpeg(&report, log, measureArgs(source, cut)); err != nil {
			return fmt.Errorf("measuring loudness: %w", err)
		}
		if measured, err = parseLoudness(report.String()); err != nil {
			return err
		}
	}

	// Written under another name and moved into place, so a run that is
	// killed never leaves a half-written talk.mp4 that looks finished.
	partial := strings.TrimSuffix(output, ".mp4") + ".partial.mp4"
	if err := runFFmpeg(log, log, encodeArgs(source, cut, partial, measured)); err != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("cutting %s: %w", TalkName, err)
	}
	// The cut must begin on the keyframe itself. A seek that lands on the
	// keyframe before it starts the video ahead of the audio.
	first, err := firstVideoTime(partial)
	if err != nil || first < -landingTolerance || first > landingTolerance {
		_ = os.Remove(partial)
		return fmt.Errorf("%s does not begin on the keyframe at %s (first frame at %v, %v)", TalkName, formatOffset(cut), first, err)
	}
	return os.Rename(partial, output)
}

// landingTolerance is how far the first video frame of the cut may be from
// zero.
const landingTolerance = 10 * time.Millisecond

// firstVideoTime is the presentation time of the first video packet.
func firstVideoTime(path string) (time.Duration, error) {
	output, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "packet=pts_time", "-of", "csv=p=0",
		"-read_intervals", "%+#1", path).Output()
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(value * float64(time.Second)), nil
}

// runFFmpeg runs ffmpeg with its standard error in report and anything it
// prints on standard output in log.
func runFFmpeg(report, log io.Writer, arguments []string) error {
	command := exec.Command("ffmpeg", arguments...)
	command.Stdout = log
	command.Stderr = report
	if err := command.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	return nil
}

var loudnessBlock = regexp.MustCompile(`(?s)\{[^{}]*"input_i"[^{}]*\}`)

// parseLoudness reads the JSON block loudnorm prints at the end of its
// analysis pass.
func parseLoudness(report string) (*loudness, error) {
	block := loudnessBlock.FindString(report)
	var measured loudness
	if block == "" || json.Unmarshal([]byte(block), &measured) != nil {
		return nil, errors.New("ffmpeg did not report the loudness")
	}
	return &measured, nil
}

func probeDuration(path string) (time.Duration, error) {
	output, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(value * float64(time.Second)), nil
}

func probeHasAudio(path string) (bool, error) {
	output, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "a",
		"-show_entries", "stream=index", "-of", "csv=p=0", path).Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(output)) != "", nil
}

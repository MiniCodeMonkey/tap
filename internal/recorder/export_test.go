package recorder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const twoSegmentChapters = `01.mov
0:00 Intro
0:18 Geocodio
0:19 Intro
1:10 Talk starts
1:10 Geocodio
1:20 Slide 3
1:00:05 Slide 74
02.mov
0:00 Slide 74
0:30 Wrap up
`

func TestParseChapterFileReadsSegmentsAndTheMark(t *testing.T) {
	segments := parseChapterFile(twoSegmentChapters)

	if len(segments) != 2 || segments[0].name != "01.mov" || segments[1].name != "02.mov" {
		t.Fatalf("segments = %+v", segments)
	}
	first := segments[0].entries
	if len(first) != 7 {
		t.Fatalf("first segment has %d entries, want 7", len(first))
	}
	if first[3].slideIndex != -1 || first[3].offset != 70*time.Second {
		t.Errorf("mark = %+v, want slideIndex -1 at 1:10", first[3])
	}
	if first[6].offset != time.Hour+5*time.Second {
		t.Errorf("h:mm:ss offset = %v, want 1h0m5s", first[6].offset)
	}
}

func TestTalkChaptersShiftAcrossSegmentsAndDropEarlierEntries(t *testing.T) {
	starts := map[string]time.Duration{"01.mov": 0, "02.mov": 2 * time.Hour}
	joined := joinChapters(parseChapterFile(twoSegmentChapters), starts)

	// The keyframe is 4 seconds before the mark.
	got := talkChapters(joined, 66*time.Second)

	want := strings.Join([]string{
		"0:00 Geocodio",
		"0:14 Slide 3",
		"58:59 Slide 74",
		"1:58:54 Slide 74",
		"1:59:24 Wrap up",
		"",
	}, "\n")
	if got != want {
		t.Errorf("talkChapters =\n%s\nwant\n%s", got, want)
	}
}

func TestTalkChaptersKeepTheMarkWhenNoSlideSharesItsTime(t *testing.T) {
	joined := joinChapters(parseChapterFile("01.mov\n0:00 Intro\n0:30 Talk starts\n0:45 Next\n"),
		map[string]time.Duration{"01.mov": 0})

	got := talkChapters(joined, 28*time.Second)

	if want := "0:00 Talk starts\n0:17 Next\n"; got != want {
		t.Errorf("talkChapters = %q, want %q", got, want)
	}
}

func TestTalkChaptersWithoutAMarkStartAtTheBeginning(t *testing.T) {
	joined := joinChapters(parseChapterFile("01.mov\n0:00 A\n0:01 B\n0:02 C\n"),
		map[string]time.Duration{"01.mov": 0})

	if got, want := talkChapters(joined, 0), "0:00 A\n0:01 B\n0:02 C\n"; got != want {
		t.Errorf("talkChapters = %q, want %q", got, want)
	}
}

func TestEncodeArgsCopyVideoAndNormalizeAudioLinearly(t *testing.T) {
	measured := &loudness{Integrated: "-31.2", TruePeak: "-9.0", Range: "6.1", Threshold: "-41.5", Offset: "0.3"}

	got := strings.Join(encodeArgs("talk.mov", 708*time.Second, "talk.mp4", measured), " ")

	for _, want := range []string{
		"-ss 708.000000 -i talk.mov", "-c:v copy", "linear=true", "I=-14", "TP=-1",
		"measured_I=-31.2", "measured_thresh=-41.5", "offset=0.3", "-c:a aac -b:a 192k -ar 48000",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("encode arguments %q lack %q", got, want)
		}
	}
	if silent := strings.Join(encodeArgs("a.mov", 0, "b.mp4", nil), " "); !strings.Contains(silent, "-an") {
		t.Errorf("a recording without audio should drop the audio: %q", silent)
	}
}

func TestParseLoudnessFindsTheJSONBlockAmongLogLines(t *testing.T) {
	report := "[Parsed_loudnorm_0 @ 0x1] \n{\n\t\"input_i\" : \"-31.04\",\n\t\"input_tp\" : \"-9.60\",\n" +
		"\t\"input_lra\" : \"7.20\",\n\t\"input_thresh\" : \"-41.30\",\n\t\"target_offset\" : \"0.12\"\n}\n"

	measured, err := parseLoudness(report)

	if err != nil || measured.Integrated != "-31.04" || measured.Offset != "0.12" {
		t.Errorf("parseLoudness = %+v, %v", measured, err)
	}
}

func TestConcatArgsCopyWithoutReencoding(t *testing.T) {
	got := strings.Join(concatArgs("list.txt", "talk.mov"), " ")
	if !strings.Contains(got, "-f concat -safe 0 -i list.txt -c copy talk.mov") {
		t.Errorf("concat arguments = %q", got)
	}
}

func TestLastKeyframePicksTheLatestKeyframeAtOrBeforeTheTime(t *testing.T) {
	packets := "600.000000,K__\n604.000000,___\n608.500000,K__\n712.000000,K__\n"

	if got := lastKeyframe(packets, 708*time.Second); got != 608500*time.Millisecond {
		t.Errorf("lastKeyframe = %v, want 10m8.5s", got)
	}
	if got := lastKeyframe("", 708*time.Second); got != 708*time.Second {
		t.Errorf("without keyframes lastKeyframe = %v, want the time itself", got)
	}
}

// makeClip writes a clip of the given length with a tone and a keyframe
// every second.
func makeClip(t *testing.T, path string, seconds string) {
	t.Helper()
	command := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=d="+seconds+":s=160x120:r=10",
		"-f", "lavfi", "-i", "sine=d="+seconds+":frequency=440",
		"-c:v", "libx264", "-g", "10", "-c:a", "aac", "-shortest", path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot make a test clip: %v\n%s", err, output)
	}
}

func TestExportJoinsCutsAndWritesChapters(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("ffmpeg is not installed")
	}
	dir := t.TempDir()
	makeClip(t, filepath.Join(dir, "01.mov"), "6")
	makeClip(t, filepath.Join(dir, "02.mov"), "4")
	chapters := "01.mov\n0:00 Intro\n0:03 Talk starts\n0:03 One\n0:05 Two\n02.mov\n0:00 Two\n0:02 Three\n"
	if err := os.WriteFile(filepath.Join(dir, "chapters.txt"), []byte(chapters), 0o600); err != nil {
		t.Fatal(err)
	}

	var log bytes.Buffer
	if err := Export(dir, &log); err != nil {
		t.Fatalf("Export: %v\n%s", err, log.String())
	}

	for _, name := range []string{"talk.mov", "talk.mp4", "talk-chapters.txt", "01.mov", "02.mov", "chapters.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s is missing: %v", name, err)
		}
	}
	length, err := probeDuration(filepath.Join(dir, "talk.mp4"))
	if err != nil || length < 6*time.Second || length > 9*time.Second {
		t.Errorf("talk.mp4 is %v long (%v), want 7s to the keyframe before the mark", length, err)
	}
	text, _ := os.ReadFile(filepath.Join(dir, "talk-chapters.txt"))
	if !strings.HasPrefix(string(text), "0:00 One\n") || !strings.HasSuffix(string(text), "Three\n") {
		t.Errorf("talk-chapters.txt = %q", text)
	}
}

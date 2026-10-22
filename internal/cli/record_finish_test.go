package cli

import (
	"strings"
	"testing"
)

func TestExportEventSaysHowToFinishWhenFFmpegIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()

	event := exportEvent(dir)

	if event.State != "unavailable" || event.Log != "" {
		t.Fatalf("event = %+v, want unavailable with no log", event)
	}
	for _, want := range []string{"raw files are kept", "tap record finish " + dir} {
		if !strings.Contains(event.Message, want) {
			t.Errorf("message %q lacks %q", event.Message, want)
		}
	}
}

package server

import (
	"strings"
	"testing"

	"github.com/skip2/go-qrcode"
)

const sampleURL = "https://advert-howard-sys-blocks.trycloudflare.com"

func compactLines(t *testing.T, url string) []string {
	t.Helper()

	code, err := GenerateCompactQRCode(url)
	if err != nil {
		t.Fatalf("GenerateCompactQRCode: %v", err)
	}
	return strings.Split(strings.TrimRight(code, "\n"), "\n")
}

// A QR code that has lost rows still looks like a QR code and cannot be
// scanned, so the row count is worth pinning down: one line per two module
// rows, and nothing dropped.
func TestCompactQRKeepsEveryModuleRow(t *testing.T) {
	qr, err := qrcode.New(sampleURL, qrcode.Medium)
	if err != nil {
		t.Fatalf("qrcode.New: %v", err)
	}
	modules := len(qr.Bitmap())

	lines := compactLines(t, sampleURL)
	want := (modules + 1) / 2
	if len(lines) != want {
		t.Errorf("got %d lines for %d module rows, want %d", len(lines), modules, want)
	}
}

func TestCompactQRIsRectangular(t *testing.T) {
	lines := compactLines(t, sampleURL)
	width := len([]rune(lines[0]))

	for i, line := range lines {
		if got := len([]rune(line)); got != width {
			t.Fatalf("line %d is %d columns wide, want %d; the code is not rectangular", i, got, width)
		}
	}
}

// Drawn inverted, so the dark modules are the terminal's background and the
// code reads dark-on-light the way a scanner expects. The quiet zone is
// therefore solid blocks.
func TestCompactQRDrawsAQuietZone(t *testing.T) {
	lines := compactLines(t, sampleURL)

	if strings.Trim(lines[0], "█") != "" {
		t.Errorf("first line is %q, want nothing but full blocks (the quiet zone)", lines[0])
	}

	last := lines[len(lines)-1]
	if strings.Trim(last, "█") != "" {
		t.Errorf("last line is %q, want nothing but full blocks (the quiet zone)", last)
	}
}

func TestCompactQRHasDarkModules(t *testing.T) {
	code, err := GenerateCompactQRCode(sampleURL)
	if err != nil {
		t.Fatalf("GenerateCompactQRCode: %v", err)
	}

	if !strings.ContainsAny(code, " ▀▄") {
		t.Error("the code has no dark modules at all, so it encodes nothing")
	}
}

func TestCompactQRIsHalfTheHeightOfTheFullOne(t *testing.T) {
	compact := compactLines(t, sampleURL)

	full, err := GenerateASCIIQRCode(sampleURL)
	if err != nil {
		t.Fatalf("GenerateASCIIQRCode: %v", err)
	}
	fullLines := strings.Split(strings.TrimRight(full, "\n"), "\n")

	if len(compact) > len(fullLines)/2+1 {
		t.Errorf("compact code is %d lines against the full code's %d; it is not saving the height it promises",
			len(compact), len(fullLines))
	}
}

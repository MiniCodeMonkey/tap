package pdf

import "testing"

func TestBuildSlideURL(t *testing.T) {
	step := 2
	fragment := 1

	tests := []struct {
		name string
		opts CaptureOptions
		want string
	}{
		{
			name: "print mode, final state",
			opts: CaptureOptions{SlideNumber: 5, Print: true},
			want: "http://localhost:3360?print=true#5",
		},
		{
			name: "step only",
			opts: CaptureOptions{SlideNumber: 3, Step: &step},
			want: "http://localhost:3360?capture=true&step=2#3",
		},
		{
			name: "fragment only",
			opts: CaptureOptions{SlideNumber: 3, Fragment: &fragment},
			want: "http://localhost:3360?capture=true&fragment=1#3",
		},
		{
			name: "step and fragment",
			opts: CaptureOptions{SlideNumber: 3, Step: &step, Fragment: &fragment},
			want: "http://localhost:3360?capture=true&fragment=1&step=2#3",
		},
		{
			name: "no state at all",
			opts: CaptureOptions{SlideNumber: 1},
			want: "http://localhost:3360#1",
		},
		{
			name: "theme override with print mode",
			opts: CaptureOptions{SlideNumber: 1, Print: true, Theme: "bauhaus"},
			want: "http://localhost:3360?print=true&theme=bauhaus#1",
		},
		{
			name: "theme override with step",
			opts: CaptureOptions{SlideNumber: 1, Step: &step, Theme: "bauhaus"},
			want: "http://localhost:3360?capture=true&step=2&theme=bauhaus#1",
		},
		{
			name: "print mode never sets capture even with theme",
			opts: CaptureOptions{SlideNumber: 1, Print: true},
			want: "http://localhost:3360?print=true#1",
		},
		{
			name: "wait alone sets capture and live, with no step or fragment",
			opts: CaptureOptions{SlideNumber: 3, WaitMS: 400},
			want: "http://localhost:3360?capture=true&live=true#3",
		},
		{
			name: "step and wait: live wins over settled",
			opts: CaptureOptions{SlideNumber: 3, Step: &step, WaitMS: 400},
			want: "http://localhost:3360?capture=true&live=true&step=2#3",
		},
		{
			name: "wait of zero is the same as no wait at all",
			opts: CaptureOptions{SlideNumber: 3, Step: &step, WaitMS: 0},
			want: "http://localhost:3360?capture=true&step=2#3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSlideURL("http://localhost:3360", tt.opts)
			if got != tt.want {
				t.Errorf("buildSlideURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

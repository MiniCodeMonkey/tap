package recorder

import "strconv"

// defaultCommand is the macOS binary that does the actual recording.
const defaultCommand = "screencapture"

// buildArgs turns Options into a screencapture argument list.
//
// The flags, in order: -v records video, -g or -G<uid> attaches audio, -C
// includes the cursor for live demos, -x suppresses the capture sounds so
// the room does not hear them and the file does not contain them, -k draws
// clicks, -V<n> caps the length, and -D<n> selects the display.
func buildArgs(options Options) []string {
	args := []string{"-v"}

	switch {
	case options.NoAudio:
	case options.AudioUID != "":
		args = append(args, "-G"+options.AudioUID)
	default:
		args = append(args, "-g")
	}

	args = append(args, "-C", "-x")

	if options.ShowClicks {
		args = append(args, "-k")
	}
	if options.LimitSeconds > 0 {
		args = append(args, "-V"+strconv.Itoa(options.LimitSeconds))
	}

	display := options.Display
	if display < 1 {
		display = 1
	}
	args = append(args, "-D"+strconv.Itoa(display), options.OutputPath)

	return args
}

// command is the binary to run, with the default filled in.
func (o Options) command() string {
	if o.Command != "" {
		return o.Command
	}
	return defaultCommand
}

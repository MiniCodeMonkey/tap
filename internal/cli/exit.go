package cli

import (
	"errors"
)

// Exit codes. Every tap command exits with one of these.
const (
	exitOK          = 0
	exitUserError   = 1
	exitInternal    = 2
	exitInterrupted = 130
)

// Error codes for the "code" field of a --json error. Parts of tap that
// add commands add their own codes here.
const (
	codeFailed         = "failed"
	codeUsage          = "usage"
	codeDeckNotFound   = "deck_not_found"
	codeNoDeck         = "no_deck"
	codeAmbiguousDeck  = "ambiguous_deck"
	codeInvalidDeck    = "invalid_deck"
	codeUnknownTheme   = "unknown_theme"
	codeOutOfRange     = "out_of_range"
	codeExists         = "exists"
	codeComponentBuild = "component_build"
	codeBrokenSlides   = "broken_slides"
	codeBrowser        = "browser"
	codeExportFailed   = "export_failed"
	codeRenamed        = "renamed"
	codeNeedsTerminal  = "needs_terminal"
	codeInterrupted    = "interrupted"
	codeCancelled      = "cancelled"
	codeInternal       = "internal"
)

// errCancelled means the person closed an interactive prompt, such as the
// deck picker, without choosing. It exits 130 and prints nothing.
var errCancelled = errors.New("cancelled")

// errInterrupted marks a failure caused by Ctrl-C (SIGINT) or SIGTERM
// during a capture or export: the command prints "interrupted" to standard
// error instead of the usual "Error: ..." line and exits with status 130,
// the conventional exit code for a process killed by SIGINT.
var errInterrupted = errors.New("interrupted")

// commandError carries the exit code and the --json error code of a
// failed command. reported is true when the command already printed its
// own diagnostics to standard error, so execute prints nothing more there.
type commandError struct {
	err      error
	code     string
	exitCode int
	reported bool
}

func (e *commandError) Error() string { return e.err.Error() }
func (e *commandError) Unwrap() error { return e.err }

// userError is a failure the person can fix: a missing deck, a bad flag,
// an invalid frontmatter key. It exits 1.
func userError(code string, err error) error {
	return &commandError{err: err, code: code, exitCode: exitUserError}
}

// internalError is a failure in tap or its environment: the browser does
// not start, a temporary server cannot bind. It exits 2.
func internalError(code string, err error) error {
	return &commandError{err: err, code: code, exitCode: exitInternal}
}

// reportedError is a user error whose details the command has already
// printed to standard error, one line per problem. It exits 1.
func reportedError(code string, err error) error {
	return &commandError{err: err, code: code, exitCode: exitUserError, reported: true}
}

// classify returns the exit code, the --json error code, and whether the
// details were already printed, for any error a command returns.
func classify(err error) (exitCode int, code string, reported bool) {
	var commandErr *commandError
	switch {
	case errors.Is(err, errInterrupted):
		return exitInterrupted, codeInterrupted, false
	case errors.Is(err, errCancelled):
		return exitInterrupted, codeCancelled, true
	case errors.As(err, &commandErr):
		return commandErr.exitCode, commandErr.code, commandErr.reported
	default:
		return exitUserError, codeFailed, false
	}
}

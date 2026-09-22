package cli

import (
	"fmt"

	"github.com/fatih/color"
)

// Color helpers for CLI output
var (
	// successColor is green for success messages
	successColor = color.New(color.FgGreen)
	// errorColor is red for error messages
	errorColor = color.New(color.FgRed)
	// infoColor is blue for informational messages
	infoColor = color.New(color.FgBlue)
	// warningColor is yellow for warning messages
	warningColor = color.New(color.FgYellow)
)

// Success prints a success message in green
func Success(format string, a ...any) {
	successColor.Printf(format, a...)
}

// Successln prints a success message in green with a newline
func Successln(a ...any) {
	successColor.Println(a...)
}

// Error prints an error message in red to standard error. Errors are
// diagnostics, not command output, so they must never land on standard
// output where a script or an LLM reading a command's result (for example
// tap export images's written paths) would see them mixed in.
func Error(format string, a ...any) {
	errorColor.Fprintf(color.Error, format, a...)
}

// Errorln prints an error message in red to standard error with a newline.
// See Error for why standard error, not standard output.
func Errorln(a ...any) {
	errorColor.Fprintln(color.Error, a...)
}

// Info prints an informational message in blue
func Info(format string, a ...any) {
	infoColor.Printf(format, a...)
}

// Infoln prints an informational message in blue with a newline
func Infoln(a ...any) {
	infoColor.Println(a...)
}

// Warning prints a warning message in yellow to standard error. Warnings
// are diagnostics, not command output; see Error for why that means
// standard error, not standard output.
func Warning(format string, a ...any) {
	warningColor.Fprintf(color.Error, format, a...)
}

// Warningln prints a warning message in yellow to standard error with a
// newline. See Warning for why standard error, not standard output.
func Warningln(a ...any) {
	warningColor.Fprintln(color.Error, a...)
}

// SuccessSprint returns a success message formatted in green
func SuccessSprint(a ...any) string {
	return successColor.Sprint(a...)
}

// ErrorSprint returns an error message formatted in red
func ErrorSprint(a ...any) string {
	return errorColor.Sprint(a...)
}

// InfoSprint returns an informational message formatted in blue
func InfoSprint(a ...any) string {
	return infoColor.Sprint(a...)
}

// WarningSprint returns a warning message formatted in yellow
func WarningSprint(a ...any) string {
	return warningColor.Sprint(a...)
}

// SuccessSprintln returns a success message formatted in green with newline
func SuccessSprintln(a ...any) string {
	return successColor.Sprintln(a...)
}

// ErrorSprintln returns an error message formatted in red with newline
func ErrorSprintln(a ...any) string {
	return errorColor.Sprintln(a...)
}

// InfoSprintln returns an informational message formatted in blue with newline
func InfoSprintln(a ...any) string {
	return infoColor.Sprintln(a...)
}

// WarningSprintln returns a warning message formatted in yellow with newline
func WarningSprintln(a ...any) string {
	return warningColor.Sprintln(a...)
}

// SuccessSprintf returns a success message formatted in green using format string
func SuccessSprintf(format string, a ...any) string {
	return successColor.Sprintf(format, a...)
}

// ErrorSprintf returns an error message formatted in red using format string
func ErrorSprintf(format string, a ...any) string {
	return errorColor.Sprintf(format, a...)
}

// InfoSprintf returns an informational message formatted in blue using format string
func InfoSprintf(format string, a ...any) string {
	return infoColor.Sprintf(format, a...)
}

// WarningSprintf returns a warning message formatted in yellow using format string
func WarningSprintf(format string, a ...any) string {
	return warningColor.Sprintf(format, a...)
}

// Bold prints text in bold
func Bold(format string, a ...any) {
	color.New(color.Bold).Printf(format, a...)
}

// BoldSprint returns text formatted in bold
func BoldSprint(a ...any) string {
	return color.New(color.Bold).Sprint(a...)
}

// Muted prints text in gray/muted color
func Muted(format string, a ...any) {
	color.New(color.FgHiBlack).Printf(format, a...)
}

// MutedSprint returns text formatted in gray/muted color
func MutedSprint(a ...any) string {
	return color.New(color.FgHiBlack).Sprint(a...)
}

// Print prints text without color formatting (passthrough for consistency)
func Print(format string, a ...any) {
	fmt.Printf(format, a...)
}

// Println prints text with newline without color formatting
func Println(a ...any) {
	fmt.Println(a...)
}

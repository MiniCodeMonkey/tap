package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

// progressStepPhases are the phases of a step line.
var progressStepPhases = map[string]bool{"render": true, "load": true, "parse": true, "bundle": true, "write": true}

// checkProgressLine checks one --progress json line against the shapes the
// docs promise: a step line, a download line, or the final "done" line.
func checkProgressLine(line string) error {
	var fields map[string]any
	if err := json.Unmarshal([]byte(line), &fields); err != nil {
		return fmt.Errorf("not a JSON object: %w: %s", err, line)
	}
	phase, ok := fields["phase"].(string)
	if !ok {
		return fmt.Errorf(`no string "phase": %s`, line)
	}
	switch {
	case progressStepPhases[phase]:
		done, doneOK := wholeNumber(fields["done"])
		total, totalOK := wholeNumber(fields["total"])
		if !doneOK || !totalOK || done < 0 || total < 1 || done > total {
			return fmt.Errorf("a step line needs whole numbers with 0 <= done <= total and total >= 1: %s", line)
		}
		return onlyFields(fields, line, "phase", "done", "total")
	case phase == "download":
		bytes, bytesOK := wholeNumber(fields["bytes"])
		totalBytes, totalOK := wholeNumber(fields["totalBytes"])
		if !bytesOK || !totalOK || bytes < 0 || bytes > totalBytes {
			return fmt.Errorf("a download line needs whole numbers with 0 <= bytes <= totalBytes: %s", line)
		}
		return onlyFields(fields, line, "phase", "bytes", "totalBytes")
	case phase == "done":
		succeeded, isBool := fields["ok"].(bool)
		if !isBool {
			return fmt.Errorf(`a done line needs a boolean "ok": %s`, line)
		}
		if succeeded {
			if _, hasError := fields["error"]; hasError {
				return fmt.Errorf(`a successful done line has no "error": %s`, line)
			}
			return nil
		}
		errorObject, isObject := fields["error"].(map[string]any)
		code, _ := errorObject["code"].(string)
		message, _ := errorObject["message"].(string)
		if !isObject || code == "" || message == "" {
			return fmt.Errorf(`a failed done line needs "error" with a code and a message: %s`, line)
		}
		return onlyFields(fields, line, "phase", "ok", "error")
	default:
		return fmt.Errorf("unknown phase %q: %s", phase, line)
	}
}

func wholeNumber(value any) (float64, bool) {
	number, ok := value.(float64)
	return number, ok && number == math.Trunc(number)
}

func onlyFields(fields map[string]any, line string, allowed ...string) error {
	for name := range fields {
		found := false
		for _, allowedName := range allowed {
			if name == allowedName {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("unexpected field %q: %s", name, line)
		}
	}
	return nil
}

// checkProgressOutput checks every JSON line in stderr against the schema,
// and that the last one is the "done" line. Lines that do not start with
// "{" are human log text, which --progress json leaves on stderr.
func checkProgressOutput(t *testing.T, stderr string) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, line := range strings.Split(stderr, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		if err := checkProgressLine(line); err != nil {
			t.Errorf("progress line fails the schema: %v", err)
			continue
		}
		var fields map[string]any
		_ = json.Unmarshal([]byte(line), &fields)
		lines = append(lines, fields)
	}
	if len(lines) == 0 || lines[len(lines)-1]["phase"] != "done" {
		t.Fatalf("the last progress line is not the done line:\n%s", stderr)
	}
	return lines
}

func TestCheckProgressLineAcceptsTheDocumentedShapes(t *testing.T) {
	for _, line := range []string{
		`{"phase":"render","done":7,"total":14}`,
		`{"phase":"parse","done":2,"total":4}`,
		`{"phase":"download","bytes":10,"totalBytes":100}`,
		`{"phase":"done","ok":true,"output":"talk.pdf","pages":3}`,
		`{"phase":"done","ok":false,"error":{"code":"deck_not_found","message":"no deck"}}`,
	} {
		if err := checkProgressLine(line); err != nil {
			t.Errorf("checkProgressLine(%s) error = %v", line, err)
		}
	}
}

func TestCheckProgressLineRejectsOtherShapes(t *testing.T) {
	for _, line := range []string{
		`not json`,
		`{"done":1,"total":2}`,
		`{"phase":"render","done":3,"total":2}`,
		`{"phase":"render","done":1.5,"total":2}`,
		`{"phase":"render","done":1,"total":2,"extra":true}`,
		`{"phase":"download","bytes":200,"totalBytes":100}`,
		`{"phase":"done"}`,
		`{"phase":"done","ok":false}`,
		`{"phase":"done","ok":true,"error":{"code":"x","message":"y"}}`,
		`{"phase":"thinking","done":1,"total":1}`,
	} {
		if err := checkProgressLine(line); err == nil {
			t.Errorf("checkProgressLine(%s) = nil, want an error", line)
		}
	}
}

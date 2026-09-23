package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// printJSONOK writes the --json result of a successful command:
// {"ok": true} followed by the fields of payload, in their declared order.
// payload must encode to a JSON object, or be nil for no fields.
func printJSONOK(w io.Writer, payload any) error {
	body, err := jsonEnvelope("JSON result", `{"ok":true`, payload, true)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	_, err = w.Write(body)
	return err
}

// jsonEnvelope splices the fields of payload into a JSON object that starts
// with prefix, such as `{"ok":true` or `{"phase":"done","ok":true`. payload
// must encode to a JSON object, or be nil for no fields. label names the
// value in error messages ("JSON result", "progress result"). indent
// pretty-prints the result with two-space indentation; a progress line
// stays on one line instead.
func jsonEnvelope(label, prefix string, payload any, indent bool) ([]byte, error) {
	body := []byte("{}")
	if payload != nil {
		var buf bytes.Buffer
		encoder := json.NewEncoder(&buf)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(payload); err != nil {
			return nil, internalError(codeInternal, fmt.Errorf("encoding the %s: %w", label, err))
		}
		body = bytes.TrimRight(buf.Bytes(), "\n")
	}
	if len(body) < 2 || body[0] != '{' {
		return nil, internalError(codeInternal, fmt.Errorf("the %s must be an object, got %s", label, body))
	}

	var combined bytes.Buffer
	combined.WriteString(prefix)
	if rest := body[1:]; string(rest) == "}" {
		combined.WriteByte('}')
	} else {
		combined.WriteByte(',')
		combined.Write(rest)
	}
	if !indent {
		return combined.Bytes(), nil
	}

	var indented bytes.Buffer
	if err := json.Indent(&indented, combined.Bytes(), "", "  "); err != nil {
		return nil, internalError(codeInternal, fmt.Errorf("formatting the %s: %w", label, err))
	}
	return indented.Bytes(), nil
}

// jsonError is the "error" field of a failed command's --json result.
type jsonError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// printJSONError writes the --json result of a failed command.
func printJSONError(w io.Writer, code, message string) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(struct {
		OK    bool      `json:"ok"`
		Error jsonError `json:"error"`
	}{Error: jsonError{Code: code, Message: message}})
}

// jsonRequested reports whether command has a --json flag that is set.
func jsonRequested(command *cobra.Command) bool {
	if command == nil {
		return false
	}
	flag := command.Flags().Lookup("json")
	return flag != nil && flag.Value.String() == "true"
}

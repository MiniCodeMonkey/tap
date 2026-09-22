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
	body := []byte("{}")
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return internalError(codeInternal, fmt.Errorf("encoding the JSON result: %w", err))
		}
		body = encoded
	}
	if len(body) < 2 || body[0] != '{' {
		return internalError(codeInternal, fmt.Errorf("the JSON result must be an object, got %s", body))
	}

	var combined bytes.Buffer
	combined.WriteString(`{"ok":true`)
	if rest := body[1:]; string(rest) == "}" {
		combined.WriteByte('}')
	} else {
		combined.WriteByte(',')
		combined.Write(rest)
	}

	var indented bytes.Buffer
	if err := json.Indent(&indented, combined.Bytes(), "", "  "); err != nil {
		return internalError(codeInternal, fmt.Errorf("formatting the JSON result: %w", err))
	}
	indented.WriteByte('\n')
	_, err := w.Write(indented.Bytes())
	return err
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

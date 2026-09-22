package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
)

func TestDeckSchemaJSON(t *testing.T) {
	exitCode, stdout, stderr := runTap(t, "deck", "schema", "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	var output struct {
		OK   bool               `json:"ok"`
		Keys []config.SchemaKey `json:"keys"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !output.OK || len(output.Keys) != len(config.Schema()) {
		t.Fatalf("output = %+v, want ok and every schema key", output)
	}

	byName := map[string]config.SchemaKey{}
	for _, key := range output.Keys {
		byName[key.Name] = key
	}
	if theme := byName["theme"]; theme.Default != "base" || !strings.Contains(strings.Join(theme.Values, ","), "terminal") {
		t.Errorf("theme = %+v, want default base and terminal among the values", theme)
	}
	if drivers := byName["drivers"]; drivers.Type != "map" || len(drivers.Keys) == 0 {
		t.Errorf("drivers = %+v, want a map with nested keys", drivers)
	}
}

func TestDeckSchemaTable(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "deck", "schema")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d", exitCode)
	}
	for _, want := range []string{"KEY", "aspectRatio", "recording.warnAfter", "drivers.<name>.connections.<name>.password", "themeColors.accent"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestDeckSchemaTakesNoArguments(t *testing.T) {
	if exitCode, _, _ := runTap(t, "deck", "schema", "talk.md"); exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
}

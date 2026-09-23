package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, found := values[name]
		return value, found
	}
}

func TestExpandEnvReplacesASetVariable(t *testing.T) {
	got, err := ExpandEnv("postgres://${DB_USER}@localhost/${DB_NAME}", "drivers.postgres.connections.demo.host",
		lookupFrom(map[string]string{"DB_USER": "admin", "DB_NAME": "shop"}))
	if err != nil || got != "postgres://admin@localhost/shop" {
		t.Errorf("ExpandEnv() = %q, %v", got, err)
	}
}

func TestExpandEnvFailsOnAnUnsetVariable(t *testing.T) {
	_, err := ExpandEnv("${DB_PASSWORD}", "drivers.mysql.connections.local.password", lookupFrom(nil))
	var unset *UnsetVariableError
	if !errors.As(err, &unset) || unset.Name != "DB_PASSWORD" {
		t.Fatalf("error = %v, want an UnsetVariableError for DB_PASSWORD", err)
	}
	want := "DB_PASSWORD is not set: drivers.mysql.connections.local.password uses ${DB_PASSWORD}. Set it in the environment or in a .env file next to the deck."
	if err.Error() != want {
		t.Errorf("message = %q, want %q", err.Error(), want)
	}
}

func TestExpandEnvKeepsAVariableThatIsSetToEmpty(t *testing.T) {
	got, err := ExpandEnv("x${EMPTY}y", "setting", lookupFrom(map[string]string{"EMPTY": ""}))
	if err != nil || got != "xy" {
		t.Errorf("ExpandEnv() = %q, %v; want \"xy\"", got, err)
	}
}

func TestExpandEnvEscape(t *testing.T) {
	got, err := ExpandEnv("$${HOME} is ${HOME}", "setting", lookupFrom(map[string]string{"HOME": "/home/me"}))
	if err != nil || got != "${HOME} is /home/me" {
		t.Errorf("ExpandEnv() = %q, %v", got, err)
	}
}

func TestExpandEnvLeavesOtherDollarSignsAlone(t *testing.T) {
	input := "pa$$word $PGUSER costs $5 $"
	got, err := ExpandEnv(input, "setting", lookupFrom(map[string]string{"PGUSER": "admin"}))
	if err != nil || got != input {
		t.Errorf("ExpandEnv() = %q, %v; want the input unchanged", got, err)
	}
}

func TestExpandEnvRejectsAMalformedReference(t *testing.T) {
	for _, input := range []string{"${DB_PASSWORD", "${not a name}", "${}"} {
		_, err := ExpandEnv(input, "drivers.mysql.connections.local.password", lookupFrom(nil))
		if err == nil || !strings.Contains(err.Error(), `write "$${" for a literal "${"`) {
			t.Errorf("ExpandEnv(%q) error = %v, want the escape hint", input, err)
		}
	}
}

func TestConnectionConfigExpanded(t *testing.T) {
	connection := ConnectionConfig{
		Host: "${DB_HOST}", User: "${DB_USER}", Password: "${DB_PASSWORD}",
		Database: "${DB_NAME}", Path: "${DB_PATH}", Port: 3306,
	}
	expanded, err := connection.Expanded("drivers.mysql.connections.local", lookupFrom(map[string]string{
		"DB_HOST": "db", "DB_USER": "admin", "DB_PASSWORD": "hunter2", "DB_NAME": "shop", "DB_PATH": "/data/x.db",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := ConnectionConfig{Host: "db", User: "admin", Password: "hunter2", Database: "shop", Path: "/data/x.db", Port: 3306}
	if expanded != want {
		t.Errorf("Expanded() = %+v, want %+v", expanded, want)
	}

	_, err = connection.Expanded("drivers.mysql.connections.local", lookupFrom(map[string]string{"DB_HOST": "db"}))
	var unset *UnsetVariableError
	if !errors.As(err, &unset) || unset.Setting != "drivers.mysql.connections.local.user" {
		t.Errorf("error = %v, want the unset user named", err)
	}
}

func TestDriverConfigExpandedCommand(t *testing.T) {
	settings := DriverConfig{Command: "${TAP_PYTHON}", Args: []string{"-c", "$${literal}"}}
	command, args, err := settings.ExpandedCommand("python", lookupFrom(map[string]string{"TAP_PYTHON": "python3"}))
	if err != nil || command != "python3" || strings.Join(args, " ") != "-c ${literal}" {
		t.Errorf("ExpandedCommand() = %q, %v, %v", command, args, err)
	}

	_, _, err = settings.ExpandedCommand("python", lookupFrom(nil))
	var unset *UnsetVariableError
	if !errors.As(err, &unset) || unset.Setting != "drivers.python.command" {
		t.Errorf("error = %v, want the unset command named", err)
	}
}

func TestLoadLeavesVariablesForWhenADriverRuns(t *testing.T) {
	t.Setenv("TAP_TEST_TITLE", "expanded title")
	t.Setenv("TAP_TEST_SECRET", "hunter2")
	path := filepath.Join(t.TempDir(), "deck.md")
	deck := "---\ntitle: \"${TAP_TEST_TITLE}\"\ndrivers:\n  mysql:\n    connections:\n      local:\n        password: \"${TAP_TEST_SECRET}\"\n---\n\n# One\n"
	if err := os.WriteFile(path, []byte(deck), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Title != "${TAP_TEST_TITLE}" {
		t.Errorf("title = %q: only driver settings expand, and only when a driver runs", cfg.Title)
	}
	if password := cfg.Drivers["mysql"].Connections["local"].Password; password != "${TAP_TEST_SECRET}" {
		t.Errorf("password = %q: the loaded config must not hold the secret, because the page receives it", password)
	}
}

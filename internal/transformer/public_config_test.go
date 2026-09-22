package transformer

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
)

// TestPublicConfigFieldsAreExplicit fails the moment a field is added to
// PublicConfig without this test being told about it, so an accidental
// re-embedding of the whole Config struct (which would pull in Drivers,
// and with it a DriverConfig's command, arguments and timeout and a
// ConnectionConfig's host, user, password, database, path and port)
// cannot pass silently. Adding a genuinely needed setting to PublicConfig
// means updating this list by hand, which is the point.
func TestPublicConfigFieldsAreExplicit(t *testing.T) {
	want := []string{
		"aspectRatio",
		"customTheme",
		"presenterLayout",
		"slideNumbers",
		"theme",
		"themeColors",
		"title",
		"transition",
	}
	sort.Strings(want)

	var got []string
	fieldType := reflect.TypeOf(PublicConfig{})
	for i := 0; i < fieldType.NumField(); i++ {
		tag := fieldType.Field(i).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			t.Fatalf("field %s has no json tag", fieldType.Field(i).Name)
		}
		got = append(got, name)
	}
	sort.Strings(got)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PublicConfig fields changed: got %v, want %v (update this test deliberately, field by field, when the page genuinely needs a new setting)", got, want)
	}
}

// TestPublicNeverCarriesDriverOrConnectionSettings builds a Config whose
// driver and connection settings are as sensitive as a deck's frontmatter
// can make them, including a literal password, and checks none of it
// survives into the client-facing view: neither the field names
// (Drivers/Connections/Command/Args/Timeout/Host/User/Password/Database/
// Path/Port never appear as JSON keys) nor the values themselves.
func TestPublicNeverCarriesDriverOrConnectionSettings(t *testing.T) {
	cfg := config.Config{
		Title: "Talk",
		Drivers: map[string]config.DriverConfig{
			"postgres": {
				Command: "psql",
				Args:    []string{"--quiet"},
				Timeout: 5,
				Connections: map[string]config.ConnectionConfig{
					"prod": {
						Host:     "db.internal.example.com",
						User:     "admin",
						Password: "hunter2literal",
						Database: "billing",
						Path:     "/var/run/postgres.sock",
						Port:     5432,
					},
				},
			},
		},
	}

	pres := &TransformedPresentation{Config: cfg}
	data, err := json.Marshal(pres.Public())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(data)

	for _, forbidden := range []string{
		"drivers", "connections", "command", "args", "timeout",
		"host", "user", "password", "database", "path", "port",
		"psql", "db.internal.example.com", "admin", "hunter2literal",
		"billing", "/var/run/postgres.sock", "5432", "postgres", "prod",
	} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(forbidden)) {
			t.Errorf("Public() output contains %q, it must carry no driver or connection setting: %s", forbidden, body)
		}
	}

	if !strings.Contains(body, `"title":"Talk"`) {
		t.Errorf("Public() dropped a setting the page genuinely reads: %s", body)
	}
}

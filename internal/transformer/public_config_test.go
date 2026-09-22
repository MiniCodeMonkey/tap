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
// survives into the client-facing view. The title deliberately contains
// "port" and "user" as ordinary English inside other words ("Import" and
// "Export"), so a naive substring sweep over the whole body would fail on
// the title alone; the check instead decodes the JSON and looks at the
// actual keys, plus the handful of attacker-chosen values that have no
// business appearing anywhere regardless of a real deck's wording.
func TestPublicNeverCarriesDriverOrConnectionSettings(t *testing.T) {
	cfg := config.Config{
		Title: "Import and Export",
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

	var decoded struct {
		Config map[string]json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	allowedKeys := map[string]bool{
		"title": true, "theme": true, "customTheme": true, "aspectRatio": true,
		"transition": true, "themeColors": true, "slideNumbers": true, "presenterLayout": true,
	}
	for key := range decoded.Config {
		if !allowedKeys[key] {
			t.Errorf("config carries unexpected key %q; a driver or connection setting may have reached the client: %v", key, decoded.Config)
		}
	}

	// Values a real deck's own wording could never coincidentally produce:
	// the literal secret, the driver's command and arguments, and the
	// connection's host, database and port.
	body := string(data)
	for _, secret := range []string{
		"hunter2literal", "db.internal.example.com", "billing",
		"/var/run/postgres.sock", "5432", "psql", "--quiet",
	} {
		if strings.Contains(body, secret) {
			t.Errorf("Public() output contains %q, it must carry no driver or connection setting: %s", secret, body)
		}
	}

	if !strings.Contains(body, `"title":"Import and Export"`) {
		t.Errorf("Public() dropped a setting the page genuinely reads: %s", body)
	}
}

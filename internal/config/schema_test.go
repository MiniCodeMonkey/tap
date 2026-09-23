package config

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// configKeyPaths records every frontmatter key path the Config struct
// reads, with the schema type its Go type must have. A map whose values
// are structs has entries under names the deck picks, written "<name>".
// A map of plain values has a fixed set of keys, listed in fixedMapKeys.
func configKeyPaths(t *testing.T, structType reflect.Type, prefix string, paths map[string]string) {
	t.Helper()
	for index := 0; index < structType.NumField(); index++ {
		field := structType.Field(index)
		name := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		path := prefix + name
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		switch fieldType.Kind() {
		case reflect.String:
			paths[path] = "string"
		case reflect.Bool:
			paths[path] = "boolean"
		case reflect.Int:
			paths[path] = "integer"
		case reflect.Slice:
			paths[path] = "list"
		case reflect.Struct:
			paths[path] = "object"
			configKeyPaths(t, fieldType, path+".", paths)
		case reflect.Map:
			if fieldType.Elem().Kind() == reflect.Struct {
				paths[path] = "map"
				configKeyPaths(t, fieldType.Elem(), path+".<name>.", paths)
				continue
			}
			keys, known := fixedMapKeys()[path]
			if !known {
				paths[path] = "a map with no fixed keys: add it to fixedMapKeys and Schema"
				continue
			}
			paths[path] = "object"
			for _, key := range keys {
				paths[path+"."+key] = "string"
			}
		default:
			t.Errorf("%s has Go kind %s, which the schema has no type for", path, fieldType.Kind())
		}
	}
}

// fixedMapKeys lists the keys of each Config map whose values are plain.
func fixedMapKeys() map[string][]string {
	names := make([]string, 0, len(themeColorKeys))
	for _, key := range themeColorKeys {
		names = append(names, key.name)
	}
	return map[string][]string{"themeColors": names}
}

// schemaKeyPaths records every key path and type in keys.
func schemaKeyPaths(keys []SchemaKey, prefix string, paths map[string]string) {
	for _, key := range keys {
		path := prefix + key.Name
		paths[path] = key.Type
		switch key.Type {
		case "object":
			schemaKeyPaths(key.Keys, path+".", paths)
		case "map":
			schemaKeyPaths(key.Keys, path+".<name>.", paths)
		}
	}
}

func TestSchemaCoversEveryConfigKey(t *testing.T) {
	want := map[string]string{}
	configKeyPaths(t, reflect.TypeOf(Config{}), "", want)
	got := map[string]string{}
	schemaKeyPaths(Schema(), "", got)

	var problems []string
	for path, wantType := range want {
		gotType, found := got[path]
		switch {
		case !found:
			problems = append(problems, "missing from Schema(): "+path)
		case gotType != wantType:
			problems = append(problems, "wrong type for "+path+": schema says "+gotType+", the struct needs "+wantType)
		}
	}
	for path := range got {
		if _, found := want[path]; !found {
			problems = append(problems, "in Schema() but not in the Config struct: "+path)
		}
	}
	sort.Strings(problems)
	for _, problem := range problems {
		t.Error(problem)
	}
}

// TestSchemaExcludesDeadFragmentsKey documents the decision on the
// frontmatter-level "fragments" key: nothing in tap reads Config.Fragments
// (only a slide's own fragments directive, parser.SlideDirectives.Fragments,
// does anything), so tap deck schema must not publish it as a working
// default. Schema() leaves it out entirely rather than publish a default
// the docs and the program's actual behaviour disagree on.
func TestSchemaExcludesDeadFragmentsKey(t *testing.T) {
	for _, key := range Schema() {
		if key.Name == "fragments" {
			t.Error(`Schema() has a "fragments" key, want none: nothing reads Config.Fragments`)
		}
	}
}

// findSchemaKey returns the key at a dotted path such as
// "recording.warnAfter" or "drivers.<name>.timeout".
func findSchemaKey(t *testing.T, path string) SchemaKey {
	t.Helper()
	keys := Schema()
	parts := strings.Split(path, ".")
	for index := 0; index < len(parts); index++ {
		if parts[index] == "<name>" {
			continue
		}
		found := false
		for _, key := range keys {
			if key.Name == parts[index] {
				if index == len(parts)-1 {
					return key
				}
				keys = key.Keys
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no schema key at %s", path)
		}
	}
	t.Fatalf("no schema key at %s", path)
	return SchemaKey{}
}

func TestSchemaValuesMatchValidation(t *testing.T) {
	if got, want := findSchemaKey(t, "theme").Values, themes.Slugs(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("theme values = %v, want themes.Slugs() %v", got, want)
	}

	tests := []struct {
		path    string
		set     func(cfg *Config, value string)
		invalid string
	}{
		{"aspectRatio", func(cfg *Config, value string) { cfg.AspectRatio = value }, "5:4"},
		{"transition", func(cfg *Config, value string) { cfg.Transition = value }, "spin"},
		{"presenterLayout", func(cfg *Config, value string) { cfg.PresenterLayout = value }, "wide"},
	}
	for _, tt := range tests {
		values := findSchemaKey(t, tt.path).Values
		if len(values) == 0 {
			t.Errorf("%s lists no values", tt.path)
		}
		for _, value := range values {
			cfg := DefaultConfig()
			tt.set(cfg, value)
			if err := cfg.Validate(); err != nil {
				t.Errorf("%s: schema value %q fails Validate: %v", tt.path, value, err)
			}
		}
		cfg := DefaultConfig()
		tt.set(cfg, tt.invalid)
		if cfg.Validate() == nil {
			t.Errorf("%s: %q passes Validate, so the schema's list is not what Validate checks", tt.path, tt.invalid)
		}
	}
}

func TestSchemaDefaultsMatchTheConfigDefaults(t *testing.T) {
	defaults := DefaultConfig()
	for path, want := range map[string]any{
		"theme":                   defaults.Theme,
		"aspectRatio":             defaults.AspectRatio,
		"transition":              defaults.Transition,
		"slideNumbers":            true,
		"drivers.<name>.timeout":  DefaultDriverTimeoutSeconds,
		"recording.output":        DefaultRecordingOutput,
		"recording.chapters":      true,
		"recording.showClicks":    false,
		"recording.display":       0,
	} {
		if got := findSchemaKey(t, path).Default; got != want {
			t.Errorf("%s default = %#v, want %#v", path, got, want)
		}
	}

	for path, want := range map[string]time.Duration{
		"recording.warnAfter": defaultWarnAfter,
		"recording.stopAfter": defaultStopAfter,
	} {
		text, isText := findSchemaKey(t, path).Default.(string)
		parsed, err := time.ParseDuration(text)
		if !isText || err != nil || parsed != want {
			t.Errorf("%s default = %#v, want a duration equal to %s", path, findSchemaKey(t, path).Default, want)
		}
	}
}

func TestEverySchemaKeyHasADescription(t *testing.T) {
	var check func(keys []SchemaKey, prefix string)
	check = func(keys []SchemaKey, prefix string) {
		for _, key := range keys {
			if key.Name == "" || strings.TrimSpace(key.Description) == "" {
				t.Errorf("%s%s has no name or no description", prefix, key.Name)
			}
			check(key.Keys, prefix+key.Name+".")
		}
	}
	check(Schema(), "")
}

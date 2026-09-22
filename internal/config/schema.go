package config

import "github.com/MiniCodeMonkey/tap/internal/themes"

// SchemaKey describes one frontmatter key tap understands, for tools that
// build a form or completions from it (tap deck schema --json).
//
// Type is "string", "boolean", "integer", "list" (of strings), "object"
// (a fixed set of nested keys) or "map" (entries under names the deck
// picks, each with the nested keys). Default is the value tap uses when
// the key is left out, or nil when there is none. Values lists the
// allowed values when only some values are allowed.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type SchemaKey struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Default     any         `json:"default"`
	Values      []string    `json:"values,omitempty"`
	Description string      `json:"description"`
	Keys        []SchemaKey `json:"keys,omitempty"`
}

// Schema returns every frontmatter key tap understands, in the order the
// docs list them. The allowed values and the defaults come from the same
// lists and constants that Validate and DefaultConfig use, and
// TestSchemaCoversEveryConfigKey fails when a Config field has no key
// here.
func Schema() []SchemaKey {
	defaults := DefaultConfig()
	return []SchemaKey{
		{Name: "title", Type: "string", Description: "The deck's title, used in the PDF's metadata, in recording file names, and by themes that print it."},
		{Name: "author", Type: "string", Description: "The speaker's name, stored in the PDF's metadata."},
		{Name: "date", Type: "string", Description: "The date of the talk, as free text."},
		{Name: "theme", Type: "string", Default: defaults.Theme, Values: themes.Slugs(), Description: "The built-in theme. An unknown name falls back to base with a warning."},
		{Name: "customTheme", Type: "string", Description: "A CSS file, relative to the deck, loaded after the theme."},
		{Name: "themeColors", Type: "object", Description: "CSS colors that replace the theme's own.", Keys: themeColorSchemaKeys()},
		{Name: "aspectRatio", Type: "string", Default: defaults.AspectRatio, Values: aspectRatioValues, Description: "The shape of every slide."},
		{Name: "transition", Type: "string", Default: defaults.Transition, Values: transitionValues, Description: "The animation between slides. A slide's transition directive overrides it."},
		{Name: "slideNumbers", Type: "boolean", Default: true, Description: "Whether the theme draws a slide number on every slide."},
		{Name: "presenterLayout", Type: "string", Values: presenterLayoutValues, Description: "The layout the presenter view opens in, unless the device has chosen one."},
		{Name: "drivers", Type: "map", Description: "Live code drivers by name, with their settings.", Keys: driverSchemaKeys()},
		{Name: "recording", Type: "object", Description: "Screen recording settings for tap dev and tap present.", Keys: recordingSchemaKeys()},
	}
}

// themeColorSchemaKeys describes the themeColors keys.
func themeColorSchemaKeys() []SchemaKey {
	keys := make([]SchemaKey, 0, len(themeColorKeys))
	for _, key := range themeColorKeys {
		keys = append(keys, SchemaKey{Name: key.name, Type: "string", Description: "A CSS color for " + key.property + "."})
	}
	return keys
}

// driverSchemaKeys describes the settings of one entry under drivers.
func driverSchemaKeys() []SchemaKey {
	return []SchemaKey{
		{Name: "command", Type: "string", Description: "The program a custom driver runs. The code goes to its standard input."},
		{Name: "args", Type: "list", Description: "Arguments passed to the command."},
		{Name: "timeout", Type: "integer", Default: DefaultDriverTimeoutSeconds, Description: "Seconds before a run is stopped."},
		{Name: "connections", Type: "map", Description: "Named connections, which a code block picks with connection: <name>.", Keys: []SchemaKey{
			{Name: "host", Type: "string", Description: "The database host. ${NAME} reads it from the environment or .env."},
			{Name: "user", Type: "string", Description: "The user name. ${NAME} reads it from the environment or .env."},
			{Name: "password", Type: "string", Description: "The password. Write ${NAME} to read it from the environment or .env instead of the deck."},
			{Name: "database", Type: "string", Description: "The database name. ${NAME} reads it from the environment or .env."},
			{Name: "path", Type: "string", Description: "A database file, relative to the deck, for sqlite. ${NAME} reads it from the environment or .env."},
			{Name: "port", Type: "integer", Description: "The database port."},
		}},
	}
}

// recordingSchemaKeys describes the keys under recording.
func recordingSchemaKeys() []SchemaKey {
	return []SchemaKey{
		{Name: "output", Type: "string", Default: DefaultRecordingOutput, Description: "The folder recordings are written to, relative to the deck."},
		{Name: "audio", Type: "string", Default: "default", Description: "The microphone: default, none for a silent recording, or a CoreAudio device UID."},
		{Name: "warnAfter", Type: "string", Default: "90m", Description: "How long a recording runs before tap warns about it, as a duration such as 90m."},
		{Name: "stopAfter", Type: "string", Default: "3h", Description: "How long a recording runs before it stops itself, as a duration such as 3h, or off."},
		{Name: "display", Type: "integer", Default: 0, Description: "The display the record picker preselects. 0 is the main display."},
		{Name: "showClicks", Type: "boolean", Default: false, Description: "Draw mouse clicks in the recording."},
		{Name: "chapters", Type: "boolean", Default: true, Description: "Write a chapter list of slide timings next to each recording."},
	}
}

package cli

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// flagConventions is the one short form each shared flag has on every
// command. An empty string means the flag has no short form.
var flagConventions = map[string]string{
	"output": "o",
	"theme":  "t",
	"port":   "p",
	"yes":    "y",
	"json":   "",
}

// removedFlags must not come back on any command.
var removedFlags = []string{"out", "deck", "verbose"}

// expectedCommands is every visible command. A part of tap that adds a
// command adds it here.
var expectedCommands = []string{
	"tap build",
	"tap component",
	"tap component new",
	"tap deck",
	"tap deck schema",
	"tap dev",
	"tap export",
	"tap export images",
	"tap export pdf",
	"tap image",
	"tap image add",
	"tap image generate",
	"tap image regenerate",
	"tap new",
	"tap present",
	"tap serve",
	"tap slide",
	"tap slide add",
	"tap slide list",
	"tap theme",
	"tap theme list",
	"tap theme set",
	"tap theme show",
}

// visibleCommands returns every command under command that a person can
// see in help, leaving out hidden commands and cobra's own help and
// completion commands.
func visibleCommands(command *cobra.Command) []*cobra.Command {
	var found []*cobra.Command
	for _, child := range command.Commands() {
		if child.Hidden || child.Name() == "help" || child.Name() == "completion" {
			continue
		}
		found = append(found, child)
		found = append(found, visibleCommands(child)...)
	}
	return found
}

func TestCommandTree(t *testing.T) {
	var paths []string
	for _, command := range visibleCommands(rootCmd) {
		paths = append(paths, command.CommandPath())
	}
	sort.Strings(paths)
	if strings.Join(paths, "\n") != strings.Join(expectedCommands, "\n") {
		t.Errorf("commands:\n%s\n\nwant:\n%s", strings.Join(paths, "\n"), strings.Join(expectedCommands, "\n"))
	}
}

// themeSlugInExampleRe finds theme slugs named in --help text, either as an
// argument to "theme set" or as a value in a printed JSON "theme" field.
var themeSlugInExampleRe = regexp.MustCompile(`theme set (\S+)|"theme":\s*"([^"]+)"`)

// TestHelpTextThemeExamplesAreRealThemes fails if any command's --help text
// names a theme slug, in a "theme set" example or a JSON "theme" field, that
// is not one of the real themes. A stale slug in a help example is what a
// person copies and runs, so it must resolve.
func TestHelpTextThemeExamplesAreRealThemes(t *testing.T) {
	for _, command := range visibleCommands(rootCmd) {
		path := command.CommandPath()
		args := append(strings.Fields(strings.TrimPrefix(path, "tap")), "--help")
		exitCode, stdout, stderr := runTap(t, args...)
		if exitCode != exitOK {
			t.Fatalf("%s --help exited %d: %s", path, exitCode, stderr)
		}

		for _, match := range themeSlugInExampleRe.FindAllStringSubmatch(stdout, -1) {
			slug := match[1]
			if slug == "" {
				slug = match[2]
			}
			if slug == "" || slug == "<slug>" {
				continue
			}
			if !themes.IsValid(slug) {
				t.Errorf("%s --help names theme slug %q, which is not a real theme (run tap theme list)", path, slug)
			}
		}
	}
}

func TestEveryCommandFollowsTheConventions(t *testing.T) {
	for _, command := range visibleCommands(rootCmd) {
		path := command.CommandPath()
		t.Run(path, func(t *testing.T) {
			args := append(strings.Fields(strings.TrimPrefix(path, "tap")), "--help")
			exitCode, stdout, stderr := runTap(t, args...)
			if exitCode != exitOK {
				t.Fatalf("%s --help exited %d: %s", path, exitCode, stderr)
			}
			if !strings.Contains(stdout, "Usage:") {
				t.Errorf("%s --help printed no usage:\n%s", path, stdout)
			}

			command.Flags().VisitAll(func(flag *pflag.Flag) {
				if want, ok := flagConventions[flag.Name]; ok && flag.Shorthand != want {
					t.Errorf("--%s has short form %q, want %q", flag.Name, flag.Shorthand, want)
				}
				for _, removed := range removedFlags {
					if flag.Name == removed {
						t.Errorf("--%s was removed and must not come back", removed)
					}
				}
			})
		})
	}
}

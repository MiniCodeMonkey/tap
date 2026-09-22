// Package cli provides the command-line interface for Tap.
package cli

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"
)

//go:embed templates/component.jsx.tmpl templates/component.tsx.tmpl templates/inline.jsx.tmpl templates/inline.tsx.tmpl templates/tap-env.d.ts templates/tap-shims.d.ts
var componentTemplatesFS embed.FS

// componentNamePattern is the PascalCase identifier tap component new requires.
var componentNamePattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// Flags for the component new command
var (
	componentInline bool
	componentTS     bool
	componentJSON   bool
)

// componentCmd groups the commands for deck-supplied React components.
var componentCmd = &cobra.Command{
	Use:   "component",
	Short: "Work with a deck's React components",
	Args:  cobra.ArbitraryArgs,
	RunE:  runUnknownGroupSubcommand,
}

// componentNewCmd scaffolds a component from a template.
var componentNewCmd = &cobra.Command{
	Use:   "new <Name> [deck]",
	Short: "Scaffold a deck-supplied React component",
	Long: `Scaffold a deck-supplied React component from a template.

Writes a whole-slide component to slides/<Name>.jsx by default, or, with
--inline, an inline block component to components/<Name>.jsx. With --ts,
the file is a .tsx, and tap-env.d.ts and tap-shims.d.ts are written next
to the deck (each only when it does not already exist), so editors and
LLM type checks work without installing anything.

<Name> must be a PascalCase identifier, for example RollingDeploy. [deck]
is a deck file or a deck folder; the default is the current folder.

Examples:
  tap component new RollingDeploy                # slides/RollingDeploy.jsx
  tap component new LatencyDrop --inline         # components/LatencyDrop.jsx
  tap component new RollingDeploy --ts           # slides/RollingDeploy.tsx
  tap component new RollingDeploy talks/deck.md  # next to talks/deck.md`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runComponentNew,
}

func init() {
	rootCmd.AddCommand(componentCmd)
	componentCmd.AddCommand(componentNewCmd)

	componentNewCmd.Flags().BoolVar(&componentInline, "inline", false, "scaffold an inline block component instead of a whole-slide one")
	componentNewCmd.Flags().BoolVar(&componentTS, "ts", false, "write a .tsx file and a tap-env.d.ts next to the deck")
	componentNewCmd.Flags().BoolVar(&componentJSON, "json", false, "print the written files and the snippet as JSON")
}

// componentScaffold is what tap component new wrote, and the markdown
// snippet that uses the component.
type componentScaffold struct {
	Files   []string `json:"files"`
	Snippet string   `json:"snippet"`
}

func runComponentNew(cmd *cobra.Command, args []string) error {
	var deckArg string
	if len(args) > 1 {
		deckArg = args[1]
	}
	result, err := scaffoldComponent(args[0], deckArg)
	if err != nil {
		return err
	}
	if componentJSON {
		return printJSONOK(cmd.OutOrStdout(), result)
	}
	for _, file := range result.Files {
		fmt.Println(file)
	}
	fmt.Println()
	fmt.Print(result.Snippet)
	return nil
}

// scaffoldComponent writes a component from a template into the deck at
// deckArg (a deck file or a deck folder), and returns the files it wrote
// and the markdown snippet that uses the component.
func scaffoldComponent(name, deckArg string) (componentScaffold, error) {
	if !componentNamePattern.MatchString(name) {
		return componentScaffold{}, userError(codeUsage, fmt.Errorf("invalid component name %q: must be a PascalCase identifier, for example RollingDeploy", name))
	}

	targetDir, err := resolveDeckFolder(deckArg)
	if err != nil {
		return componentScaffold{}, err
	}

	extension := "jsx"
	templateName := "templates/component.jsx.tmpl"
	subfolder := "slides"
	if componentTS {
		extension = "tsx"
		templateName = "templates/component.tsx.tmpl"
	}
	if componentInline {
		subfolder = "components"
		if componentTS {
			templateName = "templates/inline.tsx.tmpl"
		} else {
			templateName = "templates/inline.jsx.tmpl"
		}
	}

	componentPath := filepath.Join(targetDir, subfolder, name+"."+extension)
	if _, err := os.Stat(componentPath); err == nil {
		return componentScaffold{}, userError(codeExists, fmt.Errorf("component file already exists: %s", componentPath))
	}

	rendered, err := renderComponentTemplate(templateName, name)
	if err != nil {
		return componentScaffold{}, err
	}

	if err := os.MkdirAll(filepath.Dir(componentPath), 0o755); err != nil {
		return componentScaffold{}, fmt.Errorf("failed to create %s: %w", filepath.Dir(componentPath), err)
	}
	if err := os.WriteFile(componentPath, rendered, 0o644); err != nil {
		return componentScaffold{}, fmt.Errorf("failed to write %s: %w", componentPath, err)
	}

	written := []string{componentPath}

	if componentTS {
		envPath := filepath.Join(targetDir, "tap-env.d.ts")
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			if err := writeEmbeddedTemplate(envPath, "templates/tap-env.d.ts"); err != nil {
				return componentScaffold{}, err
			}
			written = append(written, envPath)
		}

		// The stand-in types conflict with @types/react's real ones, so
		// skip writing them once an author has installed that package
		// next to the deck (or in an ancestor's node_modules).
		shimsPath := filepath.Join(targetDir, "tap-shims.d.ts")
		if _, err := os.Stat(shimsPath); os.IsNotExist(err) && !hasTypesReactAncestor(targetDir) {
			if err := writeEmbeddedTemplate(shimsPath, "templates/tap-shims.d.ts"); err != nil {
				return componentScaffold{}, err
			}
			written = append(written, shimsPath)
		}
	}

	return componentScaffold{Files: written, Snippet: componentSnippet(name, extension, componentInline)}, nil
}

// componentNamePlaceholder marks the component's name in a template. Plain
// substitution, not text/template, because the templates are JSX: object
// literals like style={{ ... }} use the same "{{" delimiter Go's template
// package looks for.
const componentNamePlaceholder = "__COMPONENT_NAME__"

// renderComponentTemplate renders the named embedded template with every
// componentNamePlaceholder replaced by name.
func renderComponentTemplate(templateName, name string) ([]byte, error) {
	text, err := componentTemplatesFS.ReadFile(templateName)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s template: %w", templateName, err)
	}

	return bytes.ReplaceAll(text, []byte(componentNamePlaceholder), []byte(name)), nil
}

// writeEmbeddedTemplate writes the embedded template at templateName to
// destPath verbatim.
func writeEmbeddedTemplate(destPath, templateName string) error {
	content, err := componentTemplatesFS.ReadFile(templateName)
	if err != nil {
		return fmt.Errorf("failed to read %s template: %w", templateName, err)
	}
	if err := os.WriteFile(destPath, content, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", destPath, err)
	}
	return nil
}

// hasTypesReactAncestor reports whether node_modules/@types/react exists
// in dir or any of its ancestors, up to the file system root. tap-shims.d.ts
// only makes sense when nothing has provided real React types yet.
func hasTypesReactAncestor(dir string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	for {
		if info, err := os.Stat(filepath.Join(absDir, "node_modules", "@types", "react")); err == nil && info.IsDir() {
			return true
		}
		parent := filepath.Dir(absDir)
		if parent == absDir {
			return false
		}
		absDir = parent
	}
}

// componentSnippet returns the markdown snippet to paste into the deck,
// using the authoring forms from the deck components spec. Paths always
// start with "./" and use forward slashes, regardless of the host OS.
func componentSnippet(name, extension string, inline bool) string {
	if inline {
		componentPath := fmt.Sprintf("./components/%s.%s", name, extension)
		return fmt.Sprintf("```component %s\n{ \"label\": \"Requests per second\", \"value\": 1200 }\n```\n", componentPath)
	}

	componentPath := fmt.Sprintf("./slides/%s.%s", name, extension)
	return fmt.Sprintf("<!--\nlayout: %s\n-->\n\n# Title\n", componentPath)
}

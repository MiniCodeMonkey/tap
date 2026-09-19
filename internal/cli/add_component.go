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

// componentNamePattern is the PascalCase identifier tap add component requires.
var componentNamePattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// Flags for the add component command
var (
	addComponentInline bool
	addComponentTS     bool
	addComponentDeck   string
)

// addComponentCmd represents "tap add component"
var addComponentCmd = &cobra.Command{
	Use:   "component <Name>",
	Short: "Scaffold a deck-supplied React component",
	Long: `Scaffold a deck-supplied React component from a template.

Writes a whole-slide component to slides/<Name>.jsx by default, or, with
--inline, an inline block component to components/<Name>.jsx. With --ts,
the file is a .tsx, and tap-env.d.ts and tap-shims.d.ts are written next
to the deck (each only when it does not already exist), so editors and
LLM type checks work without installing anything.

<Name> must be a PascalCase identifier, for example RollingDeploy.

Examples:
  tap add component RollingDeploy                  # slides/RollingDeploy.jsx
  tap add component LatencyDrop --inline            # components/LatencyDrop.jsx
  tap add component RollingDeploy --ts               # slides/RollingDeploy.tsx
  tap add component RollingDeploy --deck deck.md      # relative to deck.md's folder`,
	Args: cobra.ExactArgs(1),
	Run:  runAddComponent,
}

func init() {
	addCmd.AddCommand(addComponentCmd)

	addComponentCmd.Flags().BoolVar(&addComponentInline, "inline", false, "scaffold an inline block component instead of a whole-slide one")
	addComponentCmd.Flags().BoolVar(&addComponentTS, "ts", false, "write a .tsx file and a tap-env.d.ts next to the deck")
	addComponentCmd.Flags().StringVar(&addComponentDeck, "deck", "", "deck file the component belongs to (default: the current directory)")
}

func runAddComponent(cmd *cobra.Command, args []string) {
	if err := runAddComponentE(args[0]); err != nil {
		Errorln("Error:", err)
		os.Exit(1)
	}
}

func runAddComponentE(name string) error {
	if !componentNamePattern.MatchString(name) {
		return fmt.Errorf("invalid component name %q: must be a PascalCase identifier, for example RollingDeploy", name)
	}

	targetDir := "."
	if addComponentDeck != "" {
		if _, err := os.Stat(addComponentDeck); os.IsNotExist(err) {
			return fmt.Errorf("deck not found: %s", addComponentDeck)
		}
		targetDir = filepath.Dir(addComponentDeck)
	}

	extension := "jsx"
	templateName := "templates/component.jsx.tmpl"
	subfolder := "slides"
	if addComponentTS {
		extension = "tsx"
		templateName = "templates/component.tsx.tmpl"
	}
	if addComponentInline {
		subfolder = "components"
		if addComponentTS {
			templateName = "templates/inline.tsx.tmpl"
		} else {
			templateName = "templates/inline.jsx.tmpl"
		}
	}

	componentPath := filepath.Join(targetDir, subfolder, name+"."+extension)
	if _, err := os.Stat(componentPath); err == nil {
		return fmt.Errorf("component file already exists: %s", componentPath)
	}

	rendered, err := renderComponentTemplate(templateName, name)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(componentPath), 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(componentPath), err)
	}
	if err := os.WriteFile(componentPath, rendered, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", componentPath, err)
	}

	written := []string{componentPath}

	if addComponentTS {
		envPath := filepath.Join(targetDir, "tap-env.d.ts")
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			if err := writeEmbeddedTemplate(envPath, "templates/tap-env.d.ts"); err != nil {
				return err
			}
			written = append(written, envPath)
		}

		// The stand-in types conflict with @types/react's real ones, so
		// skip writing them once an author has installed that package
		// next to the deck (or in an ancestor's node_modules).
		shimsPath := filepath.Join(targetDir, "tap-shims.d.ts")
		if _, err := os.Stat(shimsPath); os.IsNotExist(err) && !hasTypesReactAncestor(targetDir) {
			if err := writeEmbeddedTemplate(shimsPath, "templates/tap-shims.d.ts"); err != nil {
				return err
			}
			written = append(written, shimsPath)
		}
	}

	for _, file := range written {
		fmt.Println(file)
	}
	fmt.Println()
	fmt.Print(componentSnippet(name, extension, addComponentInline))

	return nil
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

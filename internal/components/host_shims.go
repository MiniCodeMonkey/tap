package components

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// hostModuleNames lists every module name that resolves to the host object
// at runtime instead of being bundled.
var hostModuleNames = []string{
	"react",
	"react/jsx-runtime",
	"react-dom",
	"react-dom/client",
	"motion",
	"motion/react",
	"tap",
}

// HostModuleNames returns the module names a deck component may import that
// the host provides through window.__TAP_HOST__ instead of bundling.
func HostModuleNames() []string {
	names := make([]string, len(hostModuleNames))
	copy(names, hostModuleNames)
	return names
}

// hostModuleBases lists the top-level package names tap provides through
// the host shim, so a subpath esbuild could not resolve (for example
// "react/jsx-dev-runtime", which is not itself a shimmed entry point) is
// still recognized as belonging to tap rather than to a missing npm
// package.
var hostModuleBases = []string{"react", "react-dom", "motion", "tap"}

// isHostModuleOrSubpath reports whether name is exactly one of
// hostModuleNames or a subpath of one of hostModuleBases, so an unresolved
// import error for it can point at tap's own entry points instead of
// suggesting npm install.
func isHostModuleOrSubpath(name string) bool {
	for _, hostName := range hostModuleNames {
		if name == hostName {
			return true
		}
	}
	for _, base := range hostModuleBases {
		if strings.HasPrefix(name, base+"/") {
			return true
		}
	}
	return false
}

const hostNamespace = "tap-host"

// hostShimPlugin resolves host module names to a virtual CommonJS module
// that reads the shared instance off window.__TAP_HOST__, so a deck
// component never bundles its own copy of React, Motion, or the tap helper
// module.
func hostShimPlugin() api.Plugin {
	pattern := "^(" + escapeAlternatives(hostModuleNames) + ")$"
	filter := regexp.MustCompile(pattern)

	return api.Plugin{
		Name: "tap-host-shim",
		Setup: func(build api.PluginBuild) {
			build.OnResolve(api.OnResolveOptions{Filter: pattern}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
				if !filter.MatchString(args.Path) {
					return api.OnResolveResult{}, nil
				}
				return api.OnResolveResult{
					Path:      args.Path,
					Namespace: hostNamespace,
				}, nil
			})

			build.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: hostNamespace}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
				contents := fmt.Sprintf("module.exports = window.__TAP_HOST__[%q];", args.Path)
				return api.OnLoadResult{
					Contents: &contents,
					Loader:   api.LoaderJS,
				}, nil
			})
		},
	}
}

// escapeAlternatives builds a regexp alternation from literal module names,
// escaping the slash-bearing names so the pattern matches them exactly.
func escapeAlternatives(names []string) string {
	pattern := ""
	for i, name := range names {
		if i > 0 {
			pattern += "|"
		}
		pattern += regexp.QuoteMeta(name)
	}
	return pattern
}

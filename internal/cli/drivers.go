package cli

import (
	"context"
	"os"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/driver"
)

// buildDriverRegistry returns the drivers a deck's live code blocks run
// with: the four built-in drivers, and a custom driver for each entry in
// the deck's drivers: map that sets a command. Every driver runs in the
// deck's folder. A custom command's ${NAME} expands here, so a reload
// picks up a changed .env.
func buildDriverRegistry(cfg *config.Config, baseDir string) *driver.Registry {
	registry := driver.NewRegistry()
	registry.Register(driver.NewShellDriver(baseDir))
	registry.Register(driver.NewSQLiteDriver(baseDir))
	registry.Register(driver.NewMySQLDriver(baseDir))
	registry.Register(driver.NewPostgresDriver(baseDir))

	custom := make(map[string]driver.DriverConfigInput, len(cfg.Drivers))
	for name, settings := range cfg.Drivers {
		// A drivers: entry with a built-in name configures that driver
		// and runs no command of its own.
		if registry.Has(name) || settings.Command == "" {
			continue
		}
		command, args, err := settings.ExpandedCommand(name, os.LookupEnv)
		if err != nil {
			registry.Register(unavailableDriver{name: name, err: err})
			continue
		}
		custom[name] = driver.DriverConfigInput{
			Args:       args,
			Command:    command,
			WorkingDir: baseDir,
			Timeout:    settings.Timeout,
		}
	}
	driver.RegisterCustomDrivers(registry, custom)
	return registry
}

// builtInDriverNames are the drivers tap ships. A drivers: entry with one
// of these names configures the built-in driver.
var builtInDriverNames = []string{"mysql", "postgres", "shell", "sqlite"}

// unavailableDriver stands in for a custom driver whose settings cannot be
// used, such as a command that names an unset variable. Every block that
// uses it fails with that reason.
type unavailableDriver struct {
	err  error
	name string
}

func (d unavailableDriver) Name() string { return d.name }

func (d unavailableDriver) Execute(context.Context, string, map[string]string) driver.Result {
	return driver.Result{Success: false, Error: d.err.Error()}
}

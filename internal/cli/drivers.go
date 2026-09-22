package cli

import (
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/driver"
)

// buildDriverRegistry returns the drivers a deck's live code blocks run
// with: the four built-in drivers, and a custom driver for each entry in
// the deck's drivers: map that sets a command. Every driver runs in the
// deck's folder.
func buildDriverRegistry(cfg *config.Config, baseDir string) *driver.Registry {
	registry := driver.NewRegistry()
	registry.Register(driver.NewShellDriver(baseDir))
	registry.Register(driver.NewSQLiteDriver(baseDir))
	registry.Register(driver.NewMySQLDriver(baseDir))
	registry.Register(driver.NewPostgresDriver(baseDir))

	custom := make(map[string]driver.DriverConfigInput, len(cfg.Drivers))
	for name, settings := range cfg.Drivers {
		custom[name] = driver.DriverConfigInput{
			Args:       settings.Args,
			Command:    settings.Command,
			WorkingDir: baseDir,
			Timeout:    settings.Timeout,
		}
	}
	driver.RegisterCustomDrivers(registry, custom)
	return registry
}

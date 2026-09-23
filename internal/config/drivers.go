package config

import (
	"fmt"
	"slices"
	"strings"
)

// DeclaredDrivers returns the names in the frontmatter drivers map,
// sorted. A deck may run a live code block only through one of them.
func (c *Config) DeclaredDrivers() []string {
	names := make([]string, 0, len(c.Drivers))
	for name := range c.Drivers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// DriverDeclared reports whether the drivers map has an entry for name.
func (c *Config) DriverDeclared(name string) bool {
	_, declared := c.Drivers[name]
	return declared
}

// UndeclaredDriverMessage says what to add to the frontmatter so a live
// code block with driver name can run. With no drivers map at all, it
// shows the whole block to paste, with every driver in usedDrivers.
func (c *Config) UndeclaredDriverMessage(name string, usedDrivers []string) string {
	if len(c.Drivers) > 0 {
		return fmt.Sprintf(`This deck does not declare the %s driver. Add "%s: {}" under drivers in the frontmatter.`, name, name)
	}
	names := append([]string{name}, usedDrivers...)
	slices.Sort(names)
	names = slices.Compact(names)

	var message strings.Builder
	fmt.Fprintf(&message, "This deck does not declare the %s driver. Add this to the frontmatter:\n\ndrivers:", name)
	for _, used := range names {
		fmt.Fprintf(&message, "\n  %s: {}", used)
	}
	return message.String()
}

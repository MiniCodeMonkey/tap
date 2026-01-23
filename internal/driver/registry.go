package driver

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages driver registration and lookup.
// It is safe for concurrent access.
type Registry struct {
	drivers map[string]Driver
	mu      sync.RWMutex
}

// NewRegistry creates a new empty driver registry.
func NewRegistry() *Registry {
	return &Registry{
		drivers: make(map[string]Driver),
	}
}

// Register adds a driver to the registry.
// If a driver with the same name already exists, it will be replaced.
func (r *Registry) Register(driver Driver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drivers[driver.Name()] = driver
}

// Get returns a driver by name.
// Returns nil if no driver with that name is registered.
func (r *Registry) Get(name string) Driver {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.drivers[name]
}

// Has checks if a driver with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.drivers[name]
	return exists
}

// List returns the names of all registered drivers.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.drivers))
	for name := range r.drivers {
		names = append(names, name)
	}
	return names
}

// Execute runs code using the specified driver.
// Returns an error result if the driver is not found.
func (r *Registry) Execute(ctx context.Context, driverName, code string, config map[string]string) Result {
	driver := r.Get(driverName)
	if driver == nil {
		return Result{
			Success: false,
			Error:   fmt.Sprintf("driver not found: %s", driverName),
		}
	}
	return driver.Execute(ctx, code, config)
}

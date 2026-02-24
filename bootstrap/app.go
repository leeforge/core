package bootstrap

import (
	"fmt"

	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/host"
	"github.com/leeforge/core/modules"
)

// BootstrapAllModules wires core-owned module bootstrap into host registration.
func BootstrapAllModules(router chi.Router, opts *host.CoreOptions) error {
	if router == nil {
		return fmt.Errorf("router is nil")
	}

	effective := host.CoreOptions{}
	if opts != nil {
		effective = *opts
	}
	if effective.ModuleBootstrapper == nil {
		effective.ModuleBootstrapper = modules.Bootstrap
	}
	return host.RegisterAllChi(router, effective)
}

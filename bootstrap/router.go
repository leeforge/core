package bootstrap

import (
	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/host"
)

// NewChiRouter builds a chi router with all core runtime wiring applied.
func NewChiRouter(opts *host.CoreOptions) (chi.Router, error) {
	r := chi.NewRouter()
	if err := BootstrapAllModules(r, opts); err != nil {
		return nil, err
	}
	return r, nil
}

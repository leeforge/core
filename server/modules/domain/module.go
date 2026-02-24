package domain

import (
	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"

	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

type DomainModule struct {
	logger  logging.Logger
	handler *DomainHandler
}

func NewDomainModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	handler := NewDomainHandler(deps.DomainService, logger)

	return &DomainModule{
		logger:  logger,
		handler: handler,
	}
}

func (m *DomainModule) Name() string {
	return "domain"
}

func (m *DomainModule) RegisterPublicRoutes(_ chi.Router) {
	// No public routes for domain module
}

func (m *DomainModule) RegisterPrivateRoutes(r chi.Router) {
	r.Route("/domains", func(r chi.Router) {
		framePerm.Get(r, "/me", m.handler.ListMyDomains, framePerm.Private("List my domains", "domain.read"))
	})
}

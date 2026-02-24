package core

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/leeforge/core/server/config"
	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/permissionsync"
	"github.com/leeforge/core/server/pkg/auth"
	"github.com/leeforge/core/server/pkg/jwt"

	frameCaptcha "github.com/leeforge/framework/captcha"
	frameEnt "github.com/leeforge/framework/ent"
	"github.com/leeforge/framework/logging"
)

// Module defines the interface that all feature modules must implement.
type Module interface {
	// Name returns the unique name of the module.
	Name() string

	// RegisterPublicRoutes registers public API endpoints (no authentication required).
	RegisterPublicRoutes(router chi.Router)

	// RegisterPrivateRoutes registers private API endpoints (JWT authentication required).
	RegisterPrivateRoutes(router chi.Router)
}

// ModuleFactory defines a function that creates a new module instance.
type ModuleFactory func(logger logging.Logger, dependencies *Dependencies) Module

// Dependencies holds common dependencies required by modules.
type Dependencies struct {
	Client         *ent.Client             // Backend Ent client
	FrameClient    *frameEnt.Client        // Framework Ent client
	Config         *config.Config          // Configuration
	PermManager    *auth.PermissionManager // Permission manager (RBAC/ABAC)
	PermSyncer     *permissionsync.Syncer  // Permission syncer
	JWTService     *jwt.JWTService         // JWT service for impersonation/switch-tenant
	CaptchaService frameCaptcha.Service    // Captcha service for verification
	Router         chi.Router              // Root router for snapshotting

	// Middleware functions (optional, for modules that need them).
	APIKeyMiddleware func(http.Handler) http.Handler

	// Common services can be added here if needed across modules.
	DomainService       DomainResolver
	InvitationProviders *InvitationProviderRegistry
}

// BootstrapModules loads and registers all modules.
func BootstrapModules(
	publicRouter chi.Router,
	privateRouter chi.Router,
	logger logging.Logger,
	deps *Dependencies,
	factories ...ModuleFactory,
) {
	for _, factory := range factories {
		module := factory(logger, deps)
		logger.Info("Registering module", zap.String("name", module.Name()))

		owner := module.Name()
		pub := publicRouter
		priv := privateRouter
		if scoped, ok := publicRouter.(interface{ WithOwner(string) chi.Router }); ok {
			pub = scoped.WithOwner(owner)
		}
		if scoped, ok := privateRouter.(interface{ WithOwner(string) chi.Router }); ok {
			priv = scoped.WithOwner(owner)
		}

		// Register public routes (no auth).
		module.RegisterPublicRoutes(pub)

		// Register private routes (with JWT auth).
		module.RegisterPrivateRoutes(priv)
	}
}

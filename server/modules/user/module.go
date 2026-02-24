package user

import (
	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"

	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

type UserModule struct {
	logger  logging.Logger
	handler *UserHandler
}

func NewUserModule(logger logging.Logger, deps *core.Dependencies) core.Module {
	// Initialize Services
	userSvc := NewUserService(deps.Client, logger)
	inviteSvc := NewInvitationService(
		deps.Client,
		logger,
		deps.JWTService,
		deps.DomainService,
		deps.InvitationProviders,
	)
	if deps != nil && deps.PermManager != nil {
		userSvc.WithRBACManager(deps.PermManager.RBACManager)
		inviteSvc.WithRBACManager(deps.PermManager.RBACManager)
	}
	resetSvc := NewPasswordResetService(deps.Client, logger, deps.JWTService)

	// Initialize Handler
	handler := NewUserHandler(userSvc, inviteSvc, resetSvc, logger)

	return &UserModule{
		logger:  logger,
		handler: handler,
	}
}

func (m *UserModule) Name() string {
	return "user"
}

// RegisterPublicRoutes registers public user endpoints.
func (m *UserModule) RegisterPublicRoutes(r chi.Router) {
	framePerm.Get(
		r,
		"/users/invitations/validate",
		m.handler.ValidateInvitation,
		framePerm.Public("Validate invitation token", "user.invitation.validate"),
	)
	framePerm.Post(
		r,
		"/users/invitations/activate",
		m.handler.ActivateInvitation,
		framePerm.Public("Activate invitation", "user.invitation.activate"),
	)
	framePerm.Get(
		r,
		"/users/password-resets/validate",
		m.handler.ValidatePasswordReset,
		framePerm.Public("Validate password reset token", "user.password.reset.validate"),
	)
	framePerm.Post(
		r,
		"/users/password-resets/activate",
		m.handler.ActivatePasswordReset,
		framePerm.Public("Activate password reset", "user.password.reset.activate"),
	)
}

// RegisterPrivateRoutes registers protected user endpoints (profile, user management)
func (m *UserModule) RegisterPrivateRoutes(r chi.Router) {
	// Profile endpoints (Self-service)
	r.Route("/profile", func(r chi.Router) {
		framePerm.Get(r, "/", m.handler.GetProfile, framePerm.Private("Get user profile", "user.read"))
		framePerm.Put(r, "/", m.handler.UpdateProfile, framePerm.Private("Update user profile", "user.write"))
		framePerm.Put(r, "/settings", m.handler.UpdateSettings, framePerm.Private("Update user settings", "user.write"))
	})

	// User Management Endpoints (Admin)
	r.Route("/users", func(r chi.Router) {
		framePerm.Get(r, "/", m.handler.ListUsers, framePerm.Private("List users", "user.read"))

		r.Route("/{id}", func(r chi.Router) {
			framePerm.Delete(r, "/", m.handler.DeleteUser, framePerm.Private("Delete user", "user.manage"))
			framePerm.Post(r, "/restore", m.handler.RestoreUser, framePerm.Private("Restore user", "user.manage"))
			framePerm.Post(r, "/freeze", m.handler.FreezeUser, framePerm.Private("Freeze user", "user.manage"))
			framePerm.Post(r, "/roles", m.handler.AssignRoles, framePerm.Private("Assign roles to user", "user.manage"))
			framePerm.Post(
				r,
				"/password-resets",
				m.handler.CreatePasswordReset,
				framePerm.Private("Create password reset", "user.password.reset.create"),
			)
		})
	})

	// Invitation endpoints (Admin)
	r.Route("/users/invitations", func(r chi.Router) {
		framePerm.Post(r, "/", m.handler.CreateInvitation, framePerm.Private("Create invitation", "user.manage"))
		framePerm.Get(r, "/", m.handler.ListInvitations, framePerm.Private("List invitations", "user.read"))
		r.Route("/{id}", func(r chi.Router) {
			framePerm.Get(r, "/", m.handler.GetInvitation, framePerm.Private("Get invitation", "user.read"))
			framePerm.Delete(r, "/", m.handler.RevokeInvitation, framePerm.Private("Revoke invitation", "user.manage"))
		})
	})
}

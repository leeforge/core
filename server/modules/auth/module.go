package auth

import (
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/pkg/jwt"

	frameCaptcha "github.com/leeforge/framework/captcha"
	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

type AuthModule struct {
	logger     logging.Logger
	handler    *AuthHandler
	jwtService *jwt.JWTService
}

func NewAuthModule(logger logging.Logger, deps *core.Dependencies) *AuthModule {
	// Initialize JWT Service
	jwtConfig := jwt.Config{
		AccessSecret:  deps.Config.Security.JWTSecret,
		RefreshSecret: deps.Config.Security.JWTSecret,
		InviteSecret:  deps.Config.Security.JWTSecret,
		AccessExpiry:  time.Duration(deps.Config.Security.TokenExpiry) * time.Hour,
		RefreshExpiry: time.Duration(deps.Config.Security.RefreshExpiry) * time.Hour,
		InviteExpiry:  7 * 24 * time.Hour,
	}
	jwtService := jwt.NewJWTService(jwtConfig)

	// Initialize Service
	svc := NewAuthService(deps.Client, logger, jwtService, deps.Config)

	// Initialize Handler with captcha service (if available)
	var captchaService frameCaptcha.Service
	if deps.CaptchaService != nil {
		captchaService = deps.CaptchaService
	}
	handler := NewAuthHandler(svc, logger, captchaService, deps.Config.Captcha.Enabled)

	return &AuthModule{
		logger:     logger,
		handler:    handler,
		jwtService: jwtService,
	}
}

func (m *AuthModule) Name() string {
	return "auth"
}

// RegisterPublicRoutes registers public auth endpoints (login, register)
func (m *AuthModule) RegisterPublicRoutes(r chi.Router) {
	framePerm.Post(r, "/auth/register", m.handler.Register, framePerm.Public("Register new user", "auth.register"))
	framePerm.Post(r, "/auth/login", m.handler.Login, framePerm.Public("User login", "auth.login"))
	framePerm.Post(r, "/auth/refresh", m.handler.Refresh, framePerm.Public("Refresh access token", "auth.refresh"))
}

// RegisterPrivateRoutes registers protected auth endpoints (logout, change password)
func (m *AuthModule) RegisterPrivateRoutes(r chi.Router) {
	framePerm.Post(r, "/auth/change-password", m.handler.ChangePassword, framePerm.Private("Change password", "auth.password.change"))
	framePerm.Post(r, "/auth/logout", m.handler.Logout, framePerm.Private("User logout", "auth.logout"))
}

// GetJWTService exposes the JWT service for middleware creation in main app
func (m *AuthModule) GetJWTService() *jwt.JWTService {
	return m.jwtService
}

// Ensure AuthModule implements Module interface
var _ core.Module = (*AuthModule)(nil)

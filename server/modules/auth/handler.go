package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/config"
	"github.com/leeforge/core/server/httplog"

	frameCaptcha "github.com/leeforge/framework/captcha"
	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

type AuthHandler struct {
	authService    authService
	logger         logging.Logger
	captchaService frameCaptcha.Service
	captchaEnabled bool
}

type authService interface {
	Register(ctx context.Context, username, email, password, nickname string) (*UserDTO, error)
	Login(ctx context.Context, username, password, tenantID string) (*LoginResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (string, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error
	Logout(ctx context.Context, token string) error
	GetConfig() *config.SecurityConfig
}

func NewAuthHandler(
	authService authService,
	logger logging.Logger,
	captchaService frameCaptcha.Service,
	captchaEnabled bool,
) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		logger:         logger,
		captchaService: captchaService,
		captchaEnabled: captchaEnabled,
	}
}

// RegisterRequest represents registration data
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
}

// LoginRequest represents login data
type LoginRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	RememberMe    bool   `json:"rememberMe"`    // 是否长期记住（控制Cookie持久化）
	CaptchaID     string `json:"captchaId"`     // 验证码 ID
	CaptchaAnswer string `json:"captchaAnswer"` // 验证码答案
}

// RefreshRequest represents token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// ChangePasswordRequest represents password change request
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// Register handles user registration
// @Summary Register a new user
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration info"
// @Success 200 {object} UserDTO
// @Router /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	// Basic validation
	if req.Username == "" || req.Password == "" || req.Email == "" {
		responder.BadRequest(w, r, "Username, password and email are required")
		return
	}

	user, err := h.authService.Register(r.Context(), req.Username, req.Email, req.Password, req.Nickname)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserAlreadyExists):
			responder.Conflict(w, r, "User already exists")
			return
		case errors.Is(err, ErrTenantRequired):
			responder.BadRequest(w, r, "Tenant selection required")
			return
		}
		httplog.Error(h.logger, r, "Register failed", err)
		responder.InternalServerError(w, r, "Failed to register user")
		return
	}

	responder.OK(w, r, user)
}

// Login handles user login
// @Summary Login
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		responder.BadRequest(w, r, "Username and password are required")
		return
	}

	// 🎯 验证码仅在启用时才参与登录流程
	if h.captchaService != nil && h.captchaEnabled {
		if req.CaptchaID == "" || req.CaptchaAnswer == "" {
			responder.BadRequest(w, r, "Captcha is required")
			return
		}

		// 使用 IP 作为标识符
		identifier := r.RemoteAddr

		result, err := h.captchaService.Verify(r.Context(), req.CaptchaID, req.CaptchaAnswer, identifier)
		if err != nil {
			// 验证码验证失败（如限流、过期等）
			if err == frameCaptcha.ErrRateLimitExceeded {
				responder.TooManyRequests(w, r, "Captcha verification rate limited")
				return
			}
			responder.BadRequest(w, r, "Invalid captcha")
			return
		}

		// 检查验证码是否有效
		if !result.Valid {
			responder.BadRequest(w, r, result.FailureReason)
			return
		}
	}

	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))

	result, err := h.authService.Login(r.Context(), req.Username, req.Password, tenantID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound), errors.Is(err, ErrInvalidPassword), errors.Is(err, ErrUserInactive):
			responder.Unauthorized(w, r, "Invalid username or password")
			return
		case errors.Is(err, ErrTenantRequired):
			responder.BadRequest(w, r, "Tenant selection required")
			return
		case errors.Is(err, ErrTenantNotMember):
			responder.Forbidden(w, r, "Tenant access denied")
			return
		}
		httplog.Error(h.logger, r, "Login failed", err)
		responder.InternalServerError(w, r, "Login failed")
		return
	}

	// 🎯 动态检测客户端类型
	clientType := DetectClientType(r)

	h.logger.Info("Login successful",
		zap.String("username", req.Username),
		zap.String("clientType", string(clientType)),
		zap.String("userAgent", r.Header.Get("User-Agent")),
		zap.String("xClientType", r.Header.Get("X-Client-Type")),
	)

	// 根据客户端类型返回不同响应
	switch clientType {
	case ClientTypeWeb:
		// ✅ Web端：使用httpOnly Cookie
		h.setRefreshTokenCookie(w, result.RefreshToken, req.RememberMe)

		// 响应体只返回Access Token
		responder.OK(w, r, map[string]any{
			"accessToken": result.AccessToken,
			"expiresIn":   result.ExpiresIn,
			"user":        result.User,
		})

	case ClientTypeMobile, ClientTypeUnknown:
		// ✅ 移动端/未知：返回双Token（兼容模式）
		responder.OK(w, r, result)
	}
}

// Refresh handles token refresh
// @Summary Refresh Access Token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh Token"
// @Success 200 {object} map[string]string
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var refreshToken string

	// 🎯 根据客户端类型读取Token
	clientType := DetectClientType(r)

	switch clientType {
	case ClientTypeWeb:
		// Web端：从Cookie读取
		cookie, err := r.Cookie("refresh_token")
		if err == nil && cookie.Value != "" {
			refreshToken = cookie.Value
		} else {
			h.logger.Warn("Web client missing refresh token cookie",
				zap.String("userAgent", r.Header.Get("User-Agent")),
			)
			responder.Unauthorized(w, r, "Refresh token not found")
			return
		}

	case ClientTypeMobile, ClientTypeUnknown:
		// 移动端/未知：从请求体读取
		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			responder.BadRequest(w, r, "Invalid request body")
			return
		}
		refreshToken = req.RefreshToken
	}

	if refreshToken == "" {
		responder.BadRequest(w, r, "Refresh token is required")
		return
	}

	newAccessToken, err := h.authService.RefreshToken(r.Context(), refreshToken)
	if err != nil {
		// 如果是Web端，清除无效的Cookie
		if clientType == ClientTypeWeb {
			h.clearRefreshTokenCookie(w)
		}

		h.logger.Warn("Token refresh failed",
			zap.Error(err),
			zap.String("clientType", string(clientType)),
		)

		if errors.Is(err, ErrTenantNotMember) {
			responder.Forbidden(w, r, "Tenant access denied")
			return
		}

		responder.Unauthorized(w, r, "Invalid or expired refresh token")
		return
	}

	h.logger.Info("Token refreshed successfully",
		zap.String("clientType", string(clientType)),
	)

	responder.OK(w, r, map[string]string{
		"accessToken": newAccessToken,
	})
}

// ChangePassword handles password change
// @Summary Change Password
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body ChangePasswordRequest true "Password change info"
// @Success 200 {object} map[string]string
// @Router /auth/change-password [post]
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BadRequest(w, r, "Invalid request body")
		return
	}

	userID, ok := core.GetUserID(r.Context())
	if !ok {
		responder.Unauthorized(w, r, "User not found in context")
		return
	}

	if err := h.authService.ChangePassword(r.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		if err == ErrInvalidPassword {
			responder.BadRequest(w, r, "Invalid old password")
			return
		}
		httplog.Error(h.logger, r, "Change password failed", err)
		responder.InternalServerError(w, r, "Failed to change password")
		return
	}

	responder.OK(w, r, map[string]string{"message": "Password changed successfully"})
}

// Logout handles user logout
// @Summary Logout
// @Tags Auth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// 1. 从Authorization头提取Access Token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		responder.Unauthorized(w, r, "Missing authorization header")
		return
	}

	// 2. 解析Bearer Token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		responder.BadRequest(w, r, "Invalid authorization format")
		return
	}

	token := parts[1]

	// 3. 将Access Token加入黑名单
	if err := h.authService.Logout(r.Context(), token); err != nil {
		httplog.Error(h.logger, r, "Logout failed", err)
		responder.InternalServerError(w, r, "Failed to logout")
		return
	}

	// 4. 动态检测客户端类型，Web端清除Cookie
	clientType := DetectClientType(r)
	if clientType == ClientTypeWeb {
		h.clearRefreshTokenCookie(w)
	}

	h.logger.Info("Logout successful",
		zap.String("clientType", string(clientType)),
	)

	responder.OK(w, r, map[string]string{"message": "Logged out successfully"})
}

// setRefreshTokenCookie 设置httpOnly Cookie
func (h *AuthHandler) setRefreshTokenCookie(w http.ResponseWriter, refreshToken string, rememberMe bool) {
	config := h.authService.GetConfig()

	// 计算过期时间
	maxAge := config.RefreshExpiry * 3600 // 转为秒
	if !rememberMe {
		maxAge = 0 // Session Cookie（浏览器关闭时删除）
	}

	// SameSite转换
	sameSite := http.SameSiteLaxMode
	switch strings.ToLower(config.Cookie.SameSite) {
	case "strict":
		sameSite = http.SameSiteStrictMode
	case "none":
		sameSite = http.SameSiteNoneMode
	}

	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     config.Cookie.Path,
		Domain:   config.Cookie.Domain,
		MaxAge:   maxAge,
		Secure:   config.Cookie.Secure, // 生产环境必须true
		HttpOnly: true,                 // 防止JS访问
		SameSite: sameSite,             // 防CSRF
	}

	http.SetCookie(w, cookie)

	h.logger.Debug("Refresh token cookie set",
		zap.String("path", cookie.Path),
		zap.Bool("secure", cookie.Secure),
		zap.Int("maxAge", maxAge),
	)
}

// clearRefreshTokenCookie 清除Cookie
func (h *AuthHandler) clearRefreshTokenCookie(w http.ResponseWriter) {
	config := h.authService.GetConfig()

	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     config.Cookie.Path,
		MaxAge:   -1, // 立即删除
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
}

// SetupAuthRoutes registers auth routes
func SetupAuthRoutes(r chi.Router, authHandler *AuthHandler, jwtMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(jwtMiddleware)
			r.Post("/change-password", authHandler.ChangePassword)
			r.Post("/logout", authHandler.Logout)
		})
	})
}

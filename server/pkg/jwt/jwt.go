package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	// ErrInvalidToken is returned when token is invalid
	ErrInvalidToken = errors.New("invalid token")

	// ErrExpiredToken is returned when token is expired
	ErrExpiredToken = errors.New("token has expired")

	// ErrInvalidClaims is returned when claims are invalid
	ErrInvalidClaims = errors.New("invalid token claims")
)

// Claims represents JWT claims
type Claims struct {
	UserID            uuid.UUID `json:"userId"` // User's UUID (primary key)
	Username          string    `json:"username"`
	Email             string    `json:"email"`
	TenantID          string    `json:"tenantId,omitempty"`
	ActingTenantID    string    `json:"actingTenantId,omitempty"`
	Roles             []string  `json:"roles"` // User roles
	IsSuperAdmin      bool      `json:"isSuperAdmin,omitempty"`
	Type              string    `json:"type"` // "access" or "refresh"
	IsImpersonating   bool      `json:"isImpersonating,omitempty"`
	ImpersonateReason string    `json:"impersonateReason,omitempty"`
	ImpersonateExpiry int64     `json:"impersonateExpiry,omitempty"`
	jwt.RegisteredClaims
}

// JWTService handles JWT token operations
type JWTService struct {
	accessSecret  []byte
	refreshSecret []byte
	inviteSecret  []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	inviteExpiry  time.Duration
	blacklist     *TokenBlacklist
}

// Config for JWT service
type Config struct {
	AccessSecret  string
	RefreshSecret string
	InviteSecret  string
	AccessExpiry  time.Duration // e.g., 15 * time.Minute
	RefreshExpiry time.Duration // e.g., 7 * 24 * time.Hour
	InviteExpiry  time.Duration // e.g., 7 * 24 * time.Hour
}

// NewJWTService creates a new JWT service
func NewJWTService(cfg Config) *JWTService {
	return &JWTService{
		accessSecret:  []byte(cfg.AccessSecret),
		refreshSecret: []byte(cfg.RefreshSecret),
		inviteSecret:  []byte(cfg.InviteSecret),
		accessExpiry:  cfg.AccessExpiry,
		refreshExpiry: cfg.RefreshExpiry,
		inviteExpiry:  cfg.InviteExpiry,
		blacklist:     NewTokenBlacklist(),
	}
}

// GenerateAccessToken generates an access token
func (s *JWTService) GenerateAccessToken(
	userID uuid.UUID,
	username, email, tenantID string,
	roles []string,
	isSuperAdmin bool,
) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:       userID,
		Username:     username,
		Email:        email,
		TenantID:     tenantID,
		Roles:        roles,
		IsSuperAdmin: isSuperAdmin,
		Type:         "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "leeforge-backend",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.accessSecret)
}

// GenerateImpersonationAccessToken 生成带代管上下文的访问令牌。
func (s *JWTService) GenerateImpersonationAccessToken(
	userID uuid.UUID,
	username, email string,
	roles []string,
	isSuperAdmin bool,
	actingTenantID, reason string,
	expiresAt time.Time,
) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:            userID,
		Username:          username,
		Email:             email,
		TenantID:          "",
		ActingTenantID:    actingTenantID,
		Roles:             roles,
		IsSuperAdmin:      isSuperAdmin,
		Type:              "access",
		IsImpersonating:   true,
		ImpersonateReason: reason,
		ImpersonateExpiry: expiresAt.Unix(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "leeforge-backend",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.accessSecret)
}

// GenerateRefreshToken generates a refresh token
func (s *JWTService) GenerateRefreshToken(
	userID uuid.UUID,
	username, email, tenantID string,
	roles []string,
	isSuperAdmin bool,
) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:       userID,
		Username:     username,
		Email:        email,
		TenantID:     tenantID,
		Roles:        roles,
		IsSuperAdmin: isSuperAdmin,
		Type:         "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "leeforge-backend",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.refreshSecret)
}

// ValidateAccessToken validates an access token
func (s *JWTService) ValidateAccessToken(tokenString string) (*Claims, error) {
	return s.validateToken(tokenString, s.accessSecret, "access")
}

// ValidateRefreshToken validates a refresh token
func (s *JWTService) ValidateRefreshToken(tokenString string) (*Claims, error) {
	return s.validateToken(tokenString, s.refreshSecret, "refresh")
}

// validateToken validates a token with the given secret
func (s *JWTService) validateToken(tokenString string, secret []byte, expectedType string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	// Verify token type
	if claims.Type != expectedType {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// RefreshAccessToken generates a new access token from a refresh token
func (s *JWTService) RefreshAccessToken(refreshTokenString string) (string, error) {
	// Validate refresh token
	claims, err := s.ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return "", err
	}

	// Generate new access token
	return s.GenerateAccessToken(
		claims.UserID,
		claims.Username,
		claims.Email,
		claims.TenantID,
		claims.Roles,
		claims.IsSuperAdmin,
	)
}

// InvalidateToken adds a token to the blacklist
func (s *JWTService) InvalidateToken(tokenString string) error {
	// Parse token to get expiration time
	claims, err := s.ValidateAccessToken(tokenString)
	if err != nil {
		// If token is already invalid/expired, no need to blacklist
		return nil
	}

	// Add to blacklist with expiration time
	if claims.ExpiresAt != nil {
		s.blacklist.Add(tokenString, claims.ExpiresAt.Time)
	}

	return nil
}

// IsTokenBlacklisted checks if a token is blacklisted
func (s *JWTService) IsTokenBlacklisted(tokenString string) bool {
	return s.blacklist.IsBlacklisted(tokenString)
}

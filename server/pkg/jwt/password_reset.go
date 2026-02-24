package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const passwordResetTokenType = "password_reset"

// PasswordResetClaimsInput defines payload fields for creating password-reset JWT.
type PasswordResetClaimsInput struct {
	JTI    string
	UserID uuid.UUID
	Email  string
}

// PasswordResetClaims defines JWT claims for password-reset flow.
type PasswordResetClaims struct {
	UserID uuid.UUID `json:"userId"`
	Email  string    `json:"email"`
	Type   string    `json:"type"`
	jwt.RegisteredClaims
}

// GeneratePasswordResetToken creates a password-reset JWT.
func (s *JWTService) GeneratePasswordResetToken(in PasswordResetClaimsInput) (string, error) {
	now := time.Now()
	claims := PasswordResetClaims{
		UserID: in.UserID,
		Email:  in.Email,
		Type:   passwordResetTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        in.JTI,
			ExpiresAt: jwt.NewNumericDate(now.Add(s.inviteExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "leeforge-backend",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.inviteSecret)
}

// ValidatePasswordResetToken validates a password-reset JWT.
func (s *JWTService) ValidatePasswordResetToken(tokenString string) (*PasswordResetClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &PasswordResetClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.inviteSecret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*PasswordResetClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}
	if claims.Type != passwordResetTokenType {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

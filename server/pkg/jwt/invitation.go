package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const inviteTokenType = "invite"

// InviteClaimsInput defines payload fields for creating an invite token.
type InviteClaimsInput struct {
	JTI        string
	Email      string
	DomainType string
	DomainKey  string
	RoleIDs    []string
}

// InviteClaims defines JWT claims for one-time invitation activation.
type InviteClaims struct {
	Email      string   `json:"email"`
	DomainType string   `json:"domainType"`
	DomainKey  string   `json:"domainKey"`
	RoleIDs    []string `json:"roleIds"`
	Type       string   `json:"type"`
	jwt.RegisteredClaims
}

// GenerateInviteToken creates an invitation token.
func (s *JWTService) GenerateInviteToken(in InviteClaimsInput) (string, error) {
	now := time.Now()
	claims := InviteClaims{
		Email:      in.Email,
		DomainType: in.DomainType,
		DomainKey:  in.DomainKey,
		RoleIDs:    in.RoleIDs,
		Type:       inviteTokenType,
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

// ValidateInviteToken validates an invitation token.
func (s *JWTService) ValidateInviteToken(tokenString string) (*InviteClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &InviteClaims{}, func(token *jwt.Token) (any, error) {
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

	claims, ok := token.Claims.(*InviteClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}
	if claims.Type != inviteTokenType {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

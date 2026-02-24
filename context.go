package core

import (
	"context"

	"github.com/google/uuid"
)

type IdentityType string

const (
	IdentityTypeJWT    IdentityType = "jwt"
	IdentityTypeAPIKey IdentityType = "api_key"
)

type Identity struct {
	UserID   uuid.UUID
	Type     IdentityType
	TenantID string
}

type contextKey string

const (
	identityKey contextKey = "identity"
	tenantIDKey contextKey = "tenant_id"
)

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

func GetIdentity(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	if id, ok := GetIdentity(ctx); ok {
		return id.UserID, true
	}
	return uuid.Nil, false
}

func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

func GetTenantID(ctx context.Context) (string, bool) {
	if id, ok := GetIdentity(ctx); ok && id.TenantID != "" {
		return id.TenantID, true
	}
	tenantID, ok := ctx.Value(tenantIDKey).(string)
	return tenantID, ok && tenantID != ""
}

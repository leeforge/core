package core

import (
	"context"

	"github.com/google/uuid"
)

// IdentityType defines the source of authentication
type IdentityType string

const (
	IdentityTypeJWT    IdentityType = "jwt"
	IdentityTypeAPIKey IdentityType = "api_key"
)

// Identity represents a unified authenticated entity (User or Service)
type Identity struct {
	UserID       uuid.UUID      `json:"userId"`
	Username     string         `json:"username,omitempty"`
	Type         IdentityType   `json:"type"`
	Roles        []string       `json:"roles,omitempty"`
	Permissions  []string       `json:"permissions,omitempty"`
	TenantID     string         `json:"tenantId,omitempty"`
	IsSuperAdmin bool           `json:"isSuperAdmin"`
	DataFilters  map[string]any `json:"dataFilters,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type contextKey string

const (
	identityKey  contextKey = "identity"
	tenantIDKey  contextKey = "tenant_id"
	projectIDKey contextKey = "project_id"
)

// WithIdentity injects an Identity into the context
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// WithoutIdentity removes identity from context for system/global operations.
func WithoutIdentity(ctx context.Context) context.Context {
	return context.WithValue(ctx, identityKey, nil)
}

// GetIdentity extracts the Identity from the context
func GetIdentity(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

// GetUserID is a convenience helper to get only the UserID
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	if id, ok := GetIdentity(ctx); ok {
		return id.UserID, true
	}
	return uuid.Nil, false
}

// WithTenantID injects a tenant ID into the context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// WithoutTenant clears tenant scope from both identity and context.
func WithoutTenant(ctx context.Context) context.Context {
	if id, ok := GetIdentity(ctx); ok {
		id.TenantID = ""
		ctx = WithIdentity(ctx, id)
	}
	return WithTenantID(ctx, "")
}

// WithProjectID injects a project ID into the context.
func WithProjectID(ctx context.Context, projectID string) context.Context {
	return context.WithValue(ctx, projectIDKey, projectID)
}

// WithoutProject clears project scope from context.
func WithoutProject(ctx context.Context) context.Context {
	return WithProjectID(ctx, "")
}

// RequireJWT checks if the current identity is JWT-authenticated.
// Returns the identity if valid, or false if not JWT.
func RequireJWT(ctx context.Context) (Identity, bool) {
	identity, ok := GetIdentity(ctx)
	if !ok || identity.Type != IdentityTypeJWT {
		return Identity{}, false
	}
	return identity, true
}

// GetTenantID extracts tenant ID from identity or context
func GetTenantID(ctx context.Context) (string, bool) {
	if id, ok := GetIdentity(ctx); ok && id.TenantID != "" {
		return id.TenantID, true
	}
	if tenantID, ok := ctx.Value(tenantIDKey).(string); ok && tenantID != "" {
		return tenantID, true
	}
	return "", false
}

// GetProjectID extracts project ID from context.
func GetProjectID(ctx context.Context) (string, bool) {
	if projectID, ok := ctx.Value(projectIDKey).(string); ok && projectID != "" {
		return projectID, true
	}
	return "", false
}

const domainIDKey contextKey = "domain_id"

// WithDomainID injects a domain ID into the context.
func WithDomainID(ctx context.Context, domainID string) context.Context {
	return context.WithValue(ctx, domainIDKey, domainID)
}

// GetDomainID extracts domain ID from context.
func GetDomainID(ctx context.Context) (string, bool) {
	if domainID, ok := ctx.Value(domainIDKey).(string); ok && domainID != "" {
		return domainID, true
	}
	return "", false
}

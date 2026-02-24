package core

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DomainType represents top-level acting domain type.
type DomainType string

const (
	DomainPlatform DomainType = "platform"
)

// ResolvedDomain holds the resolved domain information from the database.
type ResolvedDomain struct {
	DomainID    uuid.UUID `json:"domainId"`
	TypeCode    string    `json:"typeCode"`
	Key         string    `json:"key"`
	DisplayName string    `json:"displayName"`
}

// DomainTypeInfo holds basic domain type metadata.
type DomainTypeInfo struct {
	ID   uuid.UUID
	Code string
	Name string
}

// UserDomainInfo holds a user's domain membership information.
type UserDomainInfo struct {
	DomainID    uuid.UUID `json:"domainId"`
	TypeCode    string    `json:"typeCode"`
	Key         string    `json:"key"`
	DisplayName string    `json:"displayName"`
	MemberRole  string    `json:"memberRole"`
	IsDefault   bool      `json:"isDefault"`
}

// DomainResolver defines read operations for resolving acting domains.
type DomainResolver interface {
	ResolveDomain(ctx context.Context, typeCode, key string) (*ResolvedDomain, error)
	ResolveDomainByID(ctx context.Context, domainID uuid.UUID) (*ResolvedDomain, error)
	CheckMembership(ctx context.Context, domainID, subjectID uuid.UUID) (bool, error)
	GetUserDefaultDomain(ctx context.Context, userID uuid.UUID) (*ResolvedDomain, error)
	GetDomainString(typeCode, key string) string
	ListUserDomains(ctx context.Context, userID uuid.UUID) ([]*UserDomainInfo, error)
}

// DomainWriter extends DomainResolver with write operations for plugins
// that need to create domains and manage memberships.
type DomainWriter interface {
	DomainResolver
	EnsureDomain(ctx context.Context, typeCode, key, displayName string) (*ResolvedDomain, error)
	AddMembership(ctx context.Context, domainID, subjectID uuid.UUID, memberRole string, isDefault bool) error
	RemoveMembership(ctx context.Context, domainID, subjectID uuid.UUID) error
}

// ActingContext stores request-time domain and impersonation metadata.
type ActingContext struct {
	ActorID           uuid.UUID
	Domain            *ResolvedDomain
	IsImpersonating   bool
	ImpersonateReason string
	ImpersonateExpiry time.Time
}

// CasbinDomain returns the Casbin domain string.
func (ac *ActingContext) CasbinDomain() string {
	if ac == nil || ac.Domain == nil {
		return ""
	}
	return ac.Domain.TypeCode + ":" + ac.Domain.Key
}

// IsPlatformDomain reports whether the request is running in platform domain.
func (ac *ActingContext) IsPlatformDomain() bool {
	if ac == nil || ac.Domain == nil {
		return false
	}
	return ac.Domain.TypeCode == string(DomainPlatform)
}

// IsDomainType reports whether the resolved domain matches the given type code.
func (ac *ActingContext) IsDomainType(typeCode string) bool {
	if ac == nil || ac.Domain == nil {
		return false
	}
	return ac.Domain.TypeCode == typeCode
}

type actingContextKey struct{}

// WithActingContext injects ActingContext into context.
func WithActingContext(ctx context.Context, ac *ActingContext) context.Context {
	return context.WithValue(ctx, actingContextKey{}, ac)
}

// GetActingContext returns ActingContext if present.
func GetActingContext(ctx context.Context) *ActingContext {
	if ac, ok := ctx.Value(actingContextKey{}).(*ActingContext); ok {
		return ac
	}
	return nil
}

// MustGetActingContext returns ActingContext and panics if missing.
func MustGetActingContext(ctx context.Context) *ActingContext {
	ac := GetActingContext(ctx)
	if ac == nil {
		panic("acting context not found in context")
	}
	return ac
}

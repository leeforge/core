package core

import (
	"context"

	"github.com/google/uuid"
)

type DomainType string

const (
	DomainPlatform DomainType = "platform"
)

type ResolvedDomain struct {
	DomainID    uuid.UUID `json:"domainId"`
	TypeCode    string    `json:"typeCode"`
	Key         string    `json:"key"`
	DisplayName string    `json:"displayName"`
}

type UserDomainInfo struct {
	DomainID    uuid.UUID `json:"domainId"`
	TypeCode    string    `json:"typeCode"`
	Key         string    `json:"key"`
	DisplayName string    `json:"displayName"`
	MemberRole  string    `json:"memberRole"`
	IsDefault   bool      `json:"isDefault"`
}

type DomainResolver interface {
	ResolveDomain(ctx context.Context, typeCode, key string) (*ResolvedDomain, error)
	ResolveDomainByID(ctx context.Context, domainID uuid.UUID) (*ResolvedDomain, error)
	CheckMembership(ctx context.Context, domainID, subjectID uuid.UUID) (bool, error)
	GetUserDefaultDomain(ctx context.Context, userID uuid.UUID) (*ResolvedDomain, error)
	GetDomainString(typeCode, key string) string
	ListUserDomains(ctx context.Context, userID uuid.UUID) ([]*UserDomainInfo, error)
}

type DomainWriter interface {
	DomainResolver
	EnsureDomain(ctx context.Context, typeCode, key, displayName string) (*ResolvedDomain, error)
	AddMembership(ctx context.Context, domainID, subjectID uuid.UUID, memberRole string, isDefault bool) error
	RemoveMembership(ctx context.Context, domainID, subjectID uuid.UUID) error
}

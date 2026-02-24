package core

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const InvitationProviderRegistryServiceKey = "invitation.provider.registry"

// InvitationCreateRequest represents domain validation input before issuing invite JWT.
type InvitationCreateRequest struct {
	Username   string
	Email      string
	DomainType string
	DomainKey  string
	RoleIDs    []string
	CreatedBy  uuid.UUID
}

// InvitationActivatedRequest represents callback input after invitation activation succeeds.
type InvitationActivatedRequest struct {
	InvitationID uuid.UUID
	ActivatedBy  uuid.UUID
	DomainType   string
	DomainKey    string
	RoleIDs      []string
}

// InvitationDomainProvider handles domain-specific invitation behaviors.
type InvitationDomainProvider interface {
	TypeCode() string
	ValidateCreate(ctx context.Context, req InvitationCreateRequest) error
	OnActivated(ctx context.Context, req InvitationActivatedRequest) error
}

// InvitationProviderRegistry stores invitation providers by domain type code.
type InvitationProviderRegistry struct {
	mu sync.RWMutex
	m  map[string]InvitationDomainProvider
}

func NewInvitationProviderRegistry() *InvitationProviderRegistry {
	return &InvitationProviderRegistry{
		m: make(map[string]InvitationDomainProvider),
	}
}

func (r *InvitationProviderRegistry) Register(p InvitationDomainProvider) error {
	if p == nil {
		return fmt.Errorf("invitation provider is nil")
	}

	typeCode := normalizeInvitationTypeCode(p.TypeCode())
	if typeCode == "" {
		return fmt.Errorf("invitation provider type code is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.m[typeCode]; exists {
		return fmt.Errorf("invitation provider %q already registered", typeCode)
	}
	r.m[typeCode] = p
	return nil
}

func (r *InvitationProviderRegistry) Resolve(typeCode string) (InvitationDomainProvider, bool) {
	key := normalizeInvitationTypeCode(typeCode)
	if key == "" {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.m[key]
	return p, ok
}

func normalizeInvitationTypeCode(typeCode string) string {
	return strings.ToLower(strings.TrimSpace(typeCode))
}

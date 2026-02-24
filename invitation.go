package core

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const InvitationProviderRegistryServiceKey = "invitation.provider.registry"

type InvitationCreateRequest struct {
	Username   string
	Email      string
	DomainType string
	DomainKey  string
	RoleIDs    []string
	CreatedBy  uuid.UUID
}

type InvitationActivatedRequest struct {
	InvitationID uuid.UUID
	ActivatedBy  uuid.UUID
	DomainType   string
	DomainKey    string
	RoleIDs      []string
}

type InvitationDomainProvider interface {
	TypeCode() string
	ValidateCreate(ctx context.Context, req InvitationCreateRequest) error
	OnActivated(ctx context.Context, req InvitationActivatedRequest) error
}

type InvitationProviderRegistry struct {
	mu sync.RWMutex
	m  map[string]InvitationDomainProvider
}

func NewInvitationProviderRegistry() *InvitationProviderRegistry {
	return &InvitationProviderRegistry{m: make(map[string]InvitationDomainProvider)}
}

func (r *InvitationProviderRegistry) Register(p InvitationDomainProvider) error {
	if p == nil {
		return fmt.Errorf("invitation provider is nil")
	}
	typeCode := strings.ToLower(strings.TrimSpace(p.TypeCode()))
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
	key := strings.ToLower(strings.TrimSpace(typeCode))
	if key == "" {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.m[key]
	return p, ok
}

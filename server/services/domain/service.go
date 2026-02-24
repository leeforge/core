package domain

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/domain"
	"github.com/leeforge/core/server/ent/domainmembership"
	"github.com/leeforge/core/server/ent/domaintype"

	"github.com/leeforge/framework/logging"
)

// Service manages domain lifecycle operations.
type Service struct {
	client *ent.Client
	logger logging.Logger
}

// NewService creates a new Domain service.
func NewService(client *ent.Client, logger logging.Logger) *Service {
	return &Service{
		client: client,
		logger: logger,
	}
}

// EnsureDomainType creates or updates a domain type by code.
func (s *Service) EnsureDomainType(ctx context.Context, code, displayName string) (*ent.DomainType, error) {
	existing, err := s.client.DomainType.Query().
		Where(domaintype.CodeEQ(code)).
		Only(ctx)
	if err == nil {
		return existing, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("query domain type %q: %w", code, err)
	}

	dt, err := s.client.DomainType.Create().
		SetCode(code).
		SetDisplayName(displayName).
		SetStatus(domaintype.StatusActive).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create domain type %q: %w", code, err)
	}
	s.logger.Info("Created domain type", zap.String("code", code))
	return dt, nil
}

// EnsureDomain creates or returns a domain by type_code + key.
// Returns *core.ResolvedDomain to satisfy core.DomainWriter interface.
func (s *Service) EnsureDomain(ctx context.Context, typeCode, key, displayName string) (*core.ResolvedDomain, error) {
	d, err := s.ensureDomainEnt(ctx, typeCode, key, displayName)
	if err != nil {
		return nil, err
	}
	return &core.ResolvedDomain{
		DomainID:    d.ID,
		TypeCode:    d.TypeCode,
		Key:         d.Key,
		DisplayName: d.DisplayName,
	}, nil
}

// ensureDomainEnt creates or returns an ent.Domain by type_code + key.
func (s *Service) ensureDomainEnt(ctx context.Context, typeCode, key, displayName string) (*ent.Domain, error) {
	existing, err := s.client.Domain.Query().
		Where(
			domain.TypeCodeEQ(typeCode),
			domain.KeyEQ(key),
		).
		Only(ctx)
	if err == nil {
		return existing, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("query domain %s:%s: %w", typeCode, key, err)
	}

	d, err := s.client.Domain.Create().
		SetTypeCode(typeCode).
		SetKey(key).
		SetDisplayName(displayName).
		SetStatus(domain.StatusActive).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create domain %s:%s: %w", typeCode, key, err)
	}
	s.logger.Info("Created domain", zap.String("typeCode", typeCode), zap.String("key", key))
	return d, nil
}

// ResolveDomain looks up a domain by type_code + key and returns a ResolvedDomain.
func (s *Service) ResolveDomain(ctx context.Context, typeCode, key string) (*core.ResolvedDomain, error) {
	d, err := s.client.Domain.Query().
		Where(
			domain.TypeCodeEQ(typeCode),
			domain.KeyEQ(key),
			domain.StatusEQ(domain.StatusActive),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("domain %s:%s not found", typeCode, key)
		}
		return nil, fmt.Errorf("resolve domain %s:%s: %w", typeCode, key, err)
	}
	return &core.ResolvedDomain{
		DomainID:    d.ID,
		TypeCode:    d.TypeCode,
		Key:         d.Key,
		DisplayName: d.DisplayName,
	}, nil
}

// ResolveDomainByID looks up a domain by its UUID.
func (s *Service) ResolveDomainByID(ctx context.Context, domainID uuid.UUID) (*core.ResolvedDomain, error) {
	d, err := s.client.Domain.Get(ctx, domainID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("domain %s not found", domainID)
		}
		return nil, fmt.Errorf("resolve domain by ID %s: %w", domainID, err)
	}
	return &core.ResolvedDomain{
		DomainID:    d.ID,
		TypeCode:    d.TypeCode,
		Key:         d.Key,
		DisplayName: d.DisplayName,
	}, nil
}

// CheckMembership returns true if subject belongs to the given domain.
func (s *Service) CheckMembership(ctx context.Context, domainID, subjectID uuid.UUID) (bool, error) {
	return s.client.DomainMembership.Query().
		Where(
			domainmembership.DomainIDEQ(domainID),
			domainmembership.SubjectIDEQ(subjectID),
			domainmembership.StatusEQ(domainmembership.StatusActive),
		).
		Exist(ctx)
}

// AddMembership creates a domain membership record.
func (s *Service) AddMembership(ctx context.Context, domainID, subjectID uuid.UUID, memberRole string, isDefault bool) error {
	// Check if already exists
	exists, err := s.client.DomainMembership.Query().
		Where(
			domainmembership.DomainIDEQ(domainID),
			domainmembership.SubjectTypeEQ(domainmembership.SubjectTypeUser),
			domainmembership.SubjectIDEQ(subjectID),
		).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("query membership: %w", err)
	}
	if exists {
		return nil
	}

	_, err = s.client.DomainMembership.Create().
		SetDomainID(domainID).
		SetSubjectType(domainmembership.SubjectTypeUser).
		SetSubjectID(subjectID).
		SetMemberRole(memberRole).
		SetIsDefault(isDefault).
		SetStatus(domainmembership.StatusActive).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create membership: %w", err)
	}
	return nil
}

// RemoveMembership soft-deletes a domain membership record.
func (s *Service) RemoveMembership(ctx context.Context, domainID, subjectID uuid.UUID) error {
	count, err := s.client.DomainMembership.Update().
		Where(
			domainmembership.DomainIDEQ(domainID),
			domainmembership.SubjectIDEQ(subjectID),
			domainmembership.StatusEQ(domainmembership.StatusActive),
		).
		SetStatus(domainmembership.StatusInactive).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("remove membership: %w", err)
	}
	if count == 0 {
		return nil // already removed or never existed
	}
	return nil
}

// GetUserDefaultDomain returns the user's default domain.
func (s *Service) GetUserDefaultDomain(ctx context.Context, userID uuid.UUID) (*core.ResolvedDomain, error) {
	membership, err := s.client.DomainMembership.Query().
		Where(
			domainmembership.SubjectTypeEQ(domainmembership.SubjectTypeUser),
			domainmembership.SubjectIDEQ(userID),
			domainmembership.IsDefaultEQ(true),
			domainmembership.StatusEQ(domainmembership.StatusActive),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query default domain membership: %w", err)
	}

	return s.ResolveDomainByID(ctx, membership.DomainID)
}

// GetDomainString returns the Casbin-compatible domain string "<type_code>:<key>".
func (s *Service) GetDomainString(typeCode, key string) string {
	return typeCode + ":" + key
}

// ListUserDomains returns all active domains the user is a member of.
func (s *Service) ListUserDomains(ctx context.Context, userID uuid.UUID) ([]*core.UserDomainInfo, error) {
	memberships, err := s.client.DomainMembership.Query().
		Where(
			domainmembership.SubjectTypeEQ(domainmembership.SubjectTypeUser),
			domainmembership.SubjectIDEQ(userID),
			domainmembership.StatusEQ(domainmembership.StatusActive),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query user memberships: %w", err)
	}

	if len(memberships) == 0 {
		return []*core.UserDomainInfo{}, nil
	}

	domainIDs := make([]uuid.UUID, 0, len(memberships))
	for _, m := range memberships {
		domainIDs = append(domainIDs, m.DomainID)
	}

	domains, err := s.client.Domain.Query().
		Where(
			domain.IDIn(domainIDs...),
			domain.StatusEQ(domain.StatusActive),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query domains: %w", err)
	}

	domainMap := make(map[uuid.UUID]*ent.Domain, len(domains))
	for _, d := range domains {
		domainMap[d.ID] = d
	}

	result := make([]*core.UserDomainInfo, 0, len(memberships))
	for _, m := range memberships {
		d, ok := domainMap[m.DomainID]
		if !ok {
			continue
		}
		result = append(result, &core.UserDomainInfo{
			DomainID:    d.ID,
			TypeCode:    d.TypeCode,
			Key:         d.Key,
			DisplayName: d.DisplayName,
			MemberRole:  m.MemberRole,
			IsDefault:   m.IsDefault,
		})
	}

	return result, nil
}

// Compile-time interface checks.
var (
	_ core.DomainResolver = (*Service)(nil)
	_ core.DomainWriter   = (*Service)(nil)
)

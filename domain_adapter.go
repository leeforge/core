package core

import (
	"context"

	"github.com/google/uuid"
	corecore "github.com/leeforge/core/core"
	domainSvc "github.com/leeforge/core/server/services/domain"
)

type domainWriterAdapter struct {
	service *domainSvc.Service
}

func newDomainWriterAdapter(service *domainSvc.Service) *domainWriterAdapter {
	return &domainWriterAdapter{service: service}
}

func (d *domainWriterAdapter) EnsureDomain(ctx context.Context, typeCode, key, displayName string) (*ResolvedDomain, error) {
	if d == nil || d.service == nil {
		return nil, nil
	}
	resolved, err := d.service.EnsureDomain(ctx, typeCode, key, displayName)
	if err != nil {
		return nil, err
	}
	return toResolvedDomain(resolved), nil
}

func (d *domainWriterAdapter) AddMembership(ctx context.Context, domainID, subjectID uuid.UUID, memberRole string, isDefault bool) error {
	if d == nil || d.service == nil {
		return nil
	}
	return d.service.AddMembership(ctx, domainID, subjectID, memberRole, isDefault)
}

func (d *domainWriterAdapter) RemoveMembership(ctx context.Context, domainID, subjectID uuid.UUID) error {
	if d == nil || d.service == nil {
		return nil
	}
	return d.service.RemoveMembership(ctx, domainID, subjectID)
}

func (d *domainWriterAdapter) ResolveDomain(ctx context.Context, typeCode, key string) (*ResolvedDomain, error) {
	if d == nil || d.service == nil {
		return nil, nil
	}
	resolved, err := d.service.ResolveDomain(ctx, typeCode, key)
	if err != nil {
		return nil, err
	}
	return toResolvedDomain(resolved), nil
}

func (d *domainWriterAdapter) ResolveDomainByID(ctx context.Context, domainID uuid.UUID) (*ResolvedDomain, error) {
	if d == nil || d.service == nil {
		return nil, nil
	}
	resolved, err := d.service.ResolveDomainByID(ctx, domainID)
	if err != nil {
		return nil, err
	}
	return toResolvedDomain(resolved), nil
}

func (d *domainWriterAdapter) CheckMembership(ctx context.Context, domainID, subjectID uuid.UUID) (bool, error) {
	if d == nil || d.service == nil {
		return false, nil
	}
	return d.service.CheckMembership(ctx, domainID, subjectID)
}

func (d *domainWriterAdapter) GetUserDefaultDomain(ctx context.Context, userID uuid.UUID) (*ResolvedDomain, error) {
	if d == nil || d.service == nil {
		return nil, nil
	}
	resolved, err := d.service.GetUserDefaultDomain(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toResolvedDomain(resolved), nil
}

func (d *domainWriterAdapter) GetDomainString(typeCode, key string) string {
	if d == nil || d.service == nil {
		return ""
	}
	return d.service.GetDomainString(typeCode, key)
}

func (d *domainWriterAdapter) ListUserDomains(ctx context.Context, userID uuid.UUID) ([]*UserDomainInfo, error) {
	if d == nil || d.service == nil {
		return nil, nil
	}
	items, err := d.service.ListUserDomains(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []*UserDomainInfo{}, nil
	}
	result := make([]*UserDomainInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toUserDomainInfo(item))
	}
	return result, nil
}

func toResolvedDomain(src *corecore.ResolvedDomain) *ResolvedDomain {
	if src == nil {
		return nil
	}
	return &ResolvedDomain{
		DomainID:    src.DomainID,
		TypeCode:    src.TypeCode,
		Key:         src.Key,
		DisplayName: src.DisplayName,
	}
}

func toUserDomainInfo(src *corecore.UserDomainInfo) *UserDomainInfo {
	if src == nil {
		return nil
	}
	return &UserDomainInfo{
		DomainID:    src.DomainID,
		TypeCode:    src.TypeCode,
		Key:         src.Key,
		DisplayName: src.DisplayName,
		MemberRole:  src.MemberRole,
		IsDefault:   src.IsDefault,
	}
}

var _ DomainWriter = (*domainWriterAdapter)(nil)

package datascope

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/leeforge/core/core"
)

var (
	// ErrNoRole indicates that the user has no roles in the acting domain and should not be granted data access.
	ErrNoRole = errors.New("no roles in acting domain")
)

// Service 数据范围服务
type Service struct {
	enforcer       PolicyEnforcer
	roleScopeStore RoleScopeStore

	resolverMu     sync.RWMutex
	scopeResolvers map[ScopeType]ScopeResolver
}

// PolicyEnforcer abstracts policy lookups needed by data-scope resolution.
type PolicyEnforcer interface {
	GetRolesForUserInDomain(userID, domain string) []string
	GetFilteredNamedPolicy(ptype string, fieldIndex int, fieldValues ...string) [][]string
}

// RoleScopeStore provides default scope lookup for roles in a domain.
type RoleScopeStore interface {
	ListRoleDefaultScopes(ctx context.Context, domainID uuid.UUID, roleCodes []string) ([]string, error)
}

// NewService 创建数据范围服务
func NewService(enforcer PolicyEnforcer, roleScopeStore RoleScopeStore) *Service {
	return &Service{
		enforcer:       enforcer,
		roleScopeStore: roleScopeStore,
		scopeResolvers: make(map[ScopeType]ScopeResolver),
	}
}

// RegisterScopeResolver registers a custom scope resolver by scope type.
func (s *Service) RegisterScopeResolver(r ScopeResolver) {
	if s == nil || r == nil {
		return
	}

	scopeTypes := r.ScopeTypes()
	if len(scopeTypes) == 0 {
		return
	}

	s.resolverMu.Lock()
	defer s.resolverMu.Unlock()
	for _, scopeType := range scopeTypes {
		if scopeType == "" || isBaseScopeType(scopeType) {
			continue
		}
		s.scopeResolvers[scopeType] = r
	}
}

// GetUserDataScope resolves the user's data scope for a given resource in the specified domain.
func (s *Service) GetUserDataScope(
	ctx context.Context,
	userID uuid.UUID,
	domain string,
	resourceKey string,
) (*FilterCondition, error) {
	if domain == "" {
		return &FilterCondition{Type: ScopeAll}, nil
	}

	roles, err := s.resolveDomainRoles(ctx, userID, domain)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, ErrNoRole
	}

	var domainID uuid.UUID
	if did, ok := core.GetDomainID(ctx); ok {
		if parsed, err := uuid.Parse(did); err == nil {
			domainID = parsed
		}
	}

	maxScope := ScopeSelf

	for _, roleCode := range roles {
		roleCandidates := buildRoleCandidates(roleCode)
		for _, subject := range roleCandidates {
			policies := s.enforcer.GetFilteredNamedPolicy("p2", 0, subject, domain, resourceKey)
			for _, p := range policies {
				if len(p) < 4 {
					continue
				}
				scope := ScopeType(p[3])
				if !isBaseScopeType(scope) {
					scopeValue := ""
					if len(p) > 4 {
						scopeValue = p[4]
					}
					resolved, matched, err := s.resolveScopeByResolver(ctx, userID, domainID, scope, scopeValue)
					if err != nil {
						return nil, err
					}
					if matched && resolved != nil {
						if resolved.Type == "" {
							resolved.Type = scope
						}
						if resolved.UserID == uuid.Nil {
							resolved.UserID = userID
						}
						return resolved, nil
					}
					continue
				}
				if ScopePriority[scope] > ScopePriority[maxScope] {
					maxScope = scope
				}
			}
		}
	}

	if maxScope == ScopeSelf {
		if fallbackCondition, ok, err := s.resolveRoleDefaultScope(ctx, userID, domainID, roles); err != nil {
			return nil, err
		} else if ok {
			return fallbackCondition, nil
		}
	}

	return &FilterCondition{Type: maxScope, UserID: userID}, nil
}

func (s *Service) resolveRoleDefaultScope(
	ctx context.Context,
	userID uuid.UUID,
	domainID uuid.UUID,
	roles []string,
) (*FilterCondition, bool, error) {
	roleCodes := normalizeRoleCodes(roles)
	if len(roleCodes) == 0 {
		return nil, false, nil
	}
	if domainID == uuid.Nil {
		return nil, false, nil
	}
	if s.roleScopeStore == nil {
		return nil, false, nil
	}

	roleScopes, err := s.roleScopeStore.ListRoleDefaultScopes(ctx, domainID, roleCodes)
	if err != nil || len(roleScopes) == 0 {
		return nil, false, nil
	}

	maxBaseScope := ScopeSelf
	pluginScopes := make([]ScopeType, 0, len(roleScopes))
	seenPluginScope := make(map[ScopeType]struct{}, len(roleScopes))

	for _, rawScope := range roleScopes {
		scope := ScopeType(strings.ToUpper(strings.TrimSpace(rawScope)))
		if scope == "" {
			continue
		}
		if isBaseScopeType(scope) {
			if ScopePriority[scope] > ScopePriority[maxBaseScope] {
				maxBaseScope = scope
			}
			continue
		}
		if _, exists := seenPluginScope[scope]; exists {
			continue
		}
		seenPluginScope[scope] = struct{}{}
		pluginScopes = append(pluginScopes, scope)
	}

	if maxBaseScope == ScopeAll {
		return &FilterCondition{Type: ScopeAll, UserID: userID}, true, nil
	}

	for _, scope := range pluginScopes {
		resolved, matched, err := s.resolveScopeByResolver(ctx, userID, domainID, scope, "")
		if err != nil {
			return nil, true, err
		}
		if !matched || resolved == nil {
			continue
		}
		if resolved.Type == "" {
			resolved.Type = scope
		}
		if resolved.UserID == uuid.Nil {
			resolved.UserID = userID
		}
		return resolved, true, nil
	}

	return nil, false, nil
}

func (s *Service) resolveDomainRoles(ctx context.Context, userID uuid.UUID, domain string) ([]string, error) {
	if s.enforcer == nil {
		return nil, ErrNoRole
	}
	roles := s.enforcer.GetRolesForUserInDomain(userID.String(), domain)
	if len(roles) == 0 {
		return nil, nil
	}
	cleaned := make([]string, 0, len(roles))
	for _, roleCode := range roles {
		trimmed := strings.TrimSpace(roleCode)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return cleaned, nil
}

func buildRoleCandidates(roleCode string) []string {
	candidates := []string{roleCode}
	if strings.HasPrefix(roleCode, "role:") {
		candidates = append(candidates, strings.TrimPrefix(roleCode, "role:"))
	} else {
		candidates = append(candidates, "role:"+roleCode)
	}
	return candidates
}

func normalizeRoleCodes(roles []string) []string {
	if len(roles) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(roles))
	codes := make([]string, 0, len(roles))
	for _, roleCode := range roles {
		code := strings.TrimSpace(strings.TrimPrefix(roleCode, "role:"))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	return codes
}

func (s *Service) resolveScopeByResolver(
	ctx context.Context,
	userID uuid.UUID,
	domainID uuid.UUID,
	scopeType ScopeType,
	scopeValue string,
) (*FilterCondition, bool, error) {
	s.resolverMu.RLock()
	resolver, ok := s.scopeResolvers[scopeType]
	s.resolverMu.RUnlock()
	if !ok {
		return nil, false, nil
	}

	fc, err := resolver.Resolve(ctx, userID, domainID, scopeType, scopeValue)
	if err != nil {
		return nil, true, err
	}
	return fc, true, nil
}

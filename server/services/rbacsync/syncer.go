package rbacsync

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/server/ent"

	"github.com/leeforge/framework/auth/rbac"
	"github.com/leeforge/framework/logging"
)

const (
	defaultPolicyAction = "execute"
	platformRootDomain  = "platform:root"
)

// FullResync rebuilds Casbin p/g policies from roles and user-role assignments in DB.
// It keeps p2 policies untouched.
func FullResync(ctx context.Context, client *ent.Client, rbacManager *rbac.RBACManager, logger logging.Logger) error {
	if client == nil || rbacManager == nil || rbacManager.Enforcer() == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	enforcer := rbacManager.Enforcer()

	domains, err := loadDomainMap(ctx, client)
	if err != nil {
		return fmt.Errorf("load domains: %w", err)
	}

	roles, err := client.Role.Query().WithParent().All(ctx)
	if err != nil {
		return fmt.Errorf("query roles: %w", err)
	}

	users, err := client.User.Query().WithRoles().All(ctx)
	if err != nil {
		return fmt.Errorf("query users with roles: %w", err)
	}

	policies := make([][]string, 0, len(roles)*2)
	groupings := make([][]string, 0, len(roles)+len(users)*2)
	roleDomain := make(map[uuid.UUID]string, len(roles))

	for _, roleObj := range roles {
		domain := resolveRoleDomain(roleObj, domains)
		if domain == "" {
			if logger != nil {
				logger.Warn("Skip role policy sync due to unresolved role domain",
					zap.String("role_id", roleObj.ID.String()),
					zap.String("role_code", roleObj.Code),
				)
			}
			continue
		}
		roleDomain[roleObj.ID] = domain

		for _, code := range normalizePermissionCodes(roleObj.Permissions) {
			policies = append(policies, []string{roleObj.Code, domain, code, defaultPolicyAction})
		}

		if roleObj.Edges.Parent != nil && strings.TrimSpace(roleObj.Edges.Parent.Code) != "" {
			groupings = append(groupings, []string{roleObj.Code, roleObj.Edges.Parent.Code, domain})
		}
	}

	for _, userObj := range users {
		for _, roleObj := range userObj.Edges.Roles {
			roleCode := strings.TrimSpace(roleObj.Code)
			if roleCode == "" {
				continue
			}

			domain := roleDomain[roleObj.ID]
			if domain == "" {
				domain = resolveRoleDomain(roleObj, domains)
			}
			if domain == "" {
				if logger != nil {
					logger.Warn("Skip user-role grouping sync due to unresolved role domain",
						zap.String("user_id", userObj.ID.String()),
						zap.String("role_id", roleObj.ID.String()),
						zap.String("role_code", roleCode),
					)
				}
				continue
			}
			groupings = append(groupings, []string{userObj.ID.String(), roleCode, domain})
		}
	}

	policies = deduplicateTuples(policies)
	groupings = deduplicateTuples(groupings)

	existingPolicies := enforcer.GetNamedPolicy("p")
	if len(existingPolicies) > 0 {
		if _, err := enforcer.RemoveNamedPolicies("p", existingPolicies); err != nil {
			return fmt.Errorf("clear named p policies: %w", err)
		}
	}
	existingGroupings := enforcer.GetNamedGroupingPolicy("g")
	if len(existingGroupings) > 0 {
		if _, err := enforcer.RemoveNamedGroupingPolicies("g", existingGroupings); err != nil {
			return fmt.Errorf("clear named g policies: %w", err)
		}
	}

	for _, tuple := range policies {
		if len(tuple) != 4 {
			continue
		}
		if _, err := enforcer.AddNamedPolicy("p", tuple[0], tuple[1], tuple[2], tuple[3]); err != nil {
			return fmt.Errorf("add named p policy %v: %w", tuple, err)
		}
	}
	for _, tuple := range groupings {
		if len(tuple) != 3 {
			continue
		}
		if _, err := enforcer.AddNamedGroupingPolicy("g", tuple[0], tuple[1], tuple[2]); err != nil {
			return fmt.Errorf("add named g policy %v: %w", tuple, err)
		}
	}

	if err := rbacManager.SavePolicy(); err != nil {
		return fmt.Errorf("save casbin policy: %w", err)
	}

	if logger != nil {
		logger.Info("RBAC policies resynced from database",
			zap.Int("roles", len(roles)),
			zap.Int("users", len(users)),
			zap.Int("p_policies", len(policies)),
			zap.Int("g_policies", len(groupings)),
		)
	}

	return nil
}

func loadDomainMap(ctx context.Context, client *ent.Client) (map[uuid.UUID]string, error) {
	items, err := client.Domain.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	domains := make(map[uuid.UUID]string, len(items))
	for _, item := range items {
		typeCode := strings.TrimSpace(item.TypeCode)
		key := strings.TrimSpace(item.Key)
		if typeCode == "" || key == "" {
			continue
		}
		domains[item.ID] = typeCode + ":" + key
	}
	return domains, nil
}

func resolveRoleDomain(roleObj *ent.Role, domains map[uuid.UUID]string) string {
	if roleObj == nil {
		return ""
	}
	if roleObj.OwnerDomainID == nil {
		return platformRootDomain
	}
	if domain, ok := domains[*roleObj.OwnerDomainID]; ok {
		return domain
	}
	return ""
}

func normalizePermissionCodes(codes []string) []string {
	if len(codes) == 0 {
		return nil
	}
	unique := make(map[string]struct{}, len(codes))
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		normalized := strings.TrimSpace(code)
		if normalized == "" {
			continue
		}
		if _, exists := unique[normalized]; exists {
			continue
		}
		unique[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func deduplicateTuples(tuples [][]string) [][]string {
	if len(tuples) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tuples))
	out := make([][]string, 0, len(tuples))
	for _, tuple := range tuples {
		key := strings.Join(tuple, "\x1f")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tuple)
	}
	return out
}

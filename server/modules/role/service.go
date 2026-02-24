package role

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	casbinlib "github.com/casbin/casbin/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/permission"
	"github.com/leeforge/core/server/ent/role"
	"github.com/leeforge/core/server/ent/user"
	"github.com/leeforge/core/server/pkg/errors"
	"github.com/leeforge/core/server/services/rbacsync"

	"github.com/leeforge/framework/auth/rbac"
	"github.com/leeforge/framework/logging"
)

type RoleService struct {
	client      *ent.Client
	logger      logging.Logger
	rbacManager *rbac.RBACManager
}

const (
	superAdminRoleCode   = "super_admin"
	defaultRoleDataScope = "SELF"
)

var scopeTokenPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

type CreateRoleInput struct {
	Name             string
	Code             string
	Description      string
	Sort             int
	ParentID         *uuid.UUID
	Permissions      []string
	DefaultDataScope string
}

type UpdateRoleInput struct {
	ID               uuid.UUID
	Name             *string
	Description      *string
	Sort             *int
	ParentID         *uuid.UUID
	HasParentID      bool
	DefaultDataScope *string
}

func NewRoleService(client *ent.Client, logger logging.Logger, rbacManager *rbac.RBACManager) *RoleService {
	return &RoleService{
		client:      client,
		logger:      logger,
		rbacManager: rbacManager,
	}
}

// GetRole retrieves a role by ID with menus loaded.
func (s *RoleService) GetRole(ctx context.Context, id uuid.UUID) (*ent.Role, error) {
	r, err := s.client.Role.Query().
		Where(role.ID(id)).
		WithMenus().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Role not found", err)
		}
		return nil, errors.NewInternalError("Failed to query role", err)
	}
	return r, nil
}

// ListRoles retrieves all roles with pagination.
func (s *RoleService) ListRoles(ctx context.Context, page, pageSize int) ([]*ent.Role, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := s.client.Role.Query()
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errors.NewInternalError("Failed to count roles", err)
	}

	roles, err := query.
		Order(ent.Desc(role.FieldSort)).
		Offset(offset).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, errors.NewInternalError("Failed to query roles", err)
	}

	return roles, total, nil
}

// CreateRole creates a new role.
func (s *RoleService) CreateRole(
	ctx context.Context,
	name, code, description string,
	sort int,
	parentID *uuid.UUID,
	permissionCodes []string,
) (*ent.Role, error) {
	return s.createRoleInternal(ctx, CreateRoleInput{
		Name:        name,
		Code:        code,
		Description: description,
		Sort:        sort,
		ParentID:    parentID,
		Permissions: permissionCodes,
	})
}

func (s *RoleService) CreateRoleV2(ctx context.Context, in CreateRoleInput) (*ent.Role, error) {
	return s.createRoleInternal(ctx, in)
}

func (s *RoleService) createRoleInternal(ctx context.Context, in CreateRoleInput) (*ent.Role, error) {
	// 验证父角色是否存在
	if in.ParentID != nil {
		exists, err := s.client.Role.Query().Where(role.ID(*in.ParentID)).Exist(ctx)
		if err != nil {
			return nil, errors.NewInternalError("Failed to verify parent role", err)
		}
		if !exists {
			return nil, errors.NewNotFoundError("Parent role not found", nil)
		}
	}

	scope, err := parseDefaultRoleScope(in.DefaultDataScope)
	if err != nil {
		return nil, err
	}

	normalized := normalizePermissionCodes(in.Permissions)
	if len(normalized) > 0 {
		existing, err := s.client.Permission.Query().
			Where(
				permission.CodeIn(normalized...),
				permission.StatusEQ(permission.StatusActive),
			).
			All(ctx)
		if err != nil {
			return nil, errors.NewInternalError("Failed to validate permission codes", err)
		}
		if len(existing) != len(normalized) {
			return nil, errors.NewBadRequestError("Invalid or inactive permission codes", nil)
		}
	}

	if len(normalized) == 0 {
		normalized = []string{}
	}

	builder := s.client.Role.Create().
		SetName(in.Name).
		SetCode(in.Code).
		SetDescription(in.Description).
		SetSort(in.Sort).
		SetIsSystem(false).
		SetPermissions(normalized).
		SetDefaultDataScope(scope)

	// 设置父角色（如果有）
	if in.ParentID != nil {
		builder.SetParentID(*in.ParentID)
	}

	r, err := builder.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, errors.NewConflictError("Role name or code already exists", err)
		}
		return nil, errors.NewInternalError("Failed to create role", err)
	}

	// 如果有父角色，同步 Casbin 角色继承关系
	if in.ParentID != nil {
		if err := s.syncRoleInheritanceToCasbin(ctx, r); err != nil {
			s.logger.Warn("Failed to sync role inheritance to Casbin", zap.Error(err))
		}
	}
	s.syncRBACPolicies(ctx, "create role")

	return r, nil
}

// UpdateRole updates an existing role.
func (s *RoleService) UpdateRole(ctx context.Context, id uuid.UUID, name, description string, sort int, parentID *uuid.UUID) (*ent.Role, error) {
	return s.UpdateRoleV2(ctx, UpdateRoleInput{
		ID:          id,
		Name:        &name,
		Description: &description,
		Sort:        &sort,
		ParentID:    parentID,
		HasParentID: true,
	})
}

func (s *RoleService) UpdateRoleV2(ctx context.Context, in UpdateRoleInput) (*ent.Role, error) {
	r, err := s.client.Role.Get(ctx, in.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Role not found", err)
		}
		return nil, errors.NewInternalError("Failed to query role", err)
	}

	if r.Code == superAdminRoleCode {
		return nil, errors.NewForbiddenError("Super admin role cannot be modified", nil)
	}

	name := r.Name
	if in.Name != nil {
		name = *in.Name
	}

	description := r.Description
	if in.Description != nil {
		description = *in.Description
	}

	sortValue := r.Sort
	if in.Sort != nil {
		sortValue = *in.Sort
	}

	// 验证父角色
	if in.HasParentID && in.ParentID != nil {
		// 不能将自己设为父角色
		if *in.ParentID == in.ID {
			return nil, errors.NewBadRequestError("Cannot set self as parent role", nil)
		}

		// 验证父角色是否存在
		exists, err := s.client.Role.Query().Where(role.ID(*in.ParentID)).Exist(ctx)
		if err != nil {
			return nil, errors.NewInternalError("Failed to verify parent role", err)
		}
		if !exists {
			return nil, errors.NewNotFoundError("Parent role not found", nil)
		}

		// 防止循环引用：检查目标父角色是否是当前角色的子孙
		isDescendant, err := s.isDescendantOf(ctx, *in.ParentID, in.ID)
		if err != nil {
			return nil, errors.NewInternalError("Failed to check role hierarchy", err)
		}
		if isDescendant {
			return nil, errors.NewBadRequestError("Cannot create circular role hierarchy", nil)
		}
	}

	// Update builder
	updater := s.client.Role.UpdateOne(r).
		SetName(name).
		SetDescription(description).
		SetSort(sortValue)

	// 更新父角色（如果提供了）
	if in.HasParentID {
		if in.ParentID != nil {
			updater.SetParentID(*in.ParentID)
		} else {
			updater.ClearParent()
		}
	}

	if in.DefaultDataScope != nil {
		scope, err := parseDefaultRoleScope(*in.DefaultDataScope)
		if err != nil {
			return nil, err
		}
		updater.SetDefaultDataScope(scope)
	}

	// Note: We don't allow updating 'code' or 'is_system' for existing roles to prevent breaking logic

	updated, err := updater.Save(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to update role", err)
	}

	// 同步 Casbin 角色继承关系
	if in.HasParentID {
		if err := s.syncRoleInheritanceToCasbin(ctx, updated); err != nil {
			s.logger.Warn("Failed to sync role inheritance to Casbin", zap.Error(err))
		}
	}
	s.syncRBACPolicies(ctx, "update role")

	return updated, nil
}

// DeleteRole deletes a role if it's not a system role and has no associated users.
func (s *RoleService) DeleteRole(ctx context.Context, id uuid.UUID) error {
	r, err := s.client.Role.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Role not found", err)
		}
		return errors.NewInternalError("Failed to query role", err)
	}

	if r.Code == superAdminRoleCode {
		return errors.NewForbiddenError("Super admin role cannot be deleted", nil)
	}

	if r.IsSystem {
		return errors.NewForbiddenError("System roles cannot be deleted", nil)
	}

	// Check for associated users
	hasUsers, err := s.client.User.Query().
		Where(user.HasRolesWith(role.ID(id))).
		Exist(ctx)
	if err != nil {
		return errors.NewInternalError("Failed to check role associations", err)
	}
	if hasUsers {
		return errors.NewConflictError("Role cannot be deleted as it is associated with users", nil)
	}

	// Delete from database
	err = s.client.Role.DeleteOne(r).Exec(ctx)
	if err != nil {
		return errors.NewInternalError("Failed to delete role", err)
	}

	// Clean up Casbin policies for this role
	// This will be handled in a more centralized way later,
	// but for now we should remove all policies where sub = role.code
	// Note: This needs Enforcer instance which would call adapter.RemoveFilteredPolicy
	s.syncRBACPolicies(ctx, "delete role")

	return nil
}

// CopyRole clones a role and its permissions to a new role.
func (s *RoleService) CopyRole(ctx context.Context, sourceID uuid.UUID, newName, newCode string) (*ent.Role, error) {
	source, err := s.client.Role.Query().
		Where(role.ID(sourceID)).
		WithMenus().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Source role not found", err)
		}
		return nil, errors.NewInternalError("Failed to query source role", err)
	}

	// Start transaction
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to start transaction", err)
	}

	// 1. Create new role
	newRole, err := tx.Role.Create().
		SetName(newName).
		SetCode(newCode).
		SetDescription(fmt.Sprintf("Copy of %s: %s", source.Name, source.Description)).
		SetSort(source.Sort).
		SetIsSystem(false).
		SetPermissions(source.Permissions).
		Save(ctx)
	if err != nil {
		tx.Rollback()
		if ent.IsConstraintError(err) {
			return nil, errors.NewConflictError("Role code already exists", err)
		}
		return nil, errors.NewInternalError("Failed to create new role", err)
	}

	// 2. Copy menu associations
	menuIDs := make([]uuid.UUID, len(source.Edges.Menus))
	for i, m := range source.Edges.Menus {
		menuIDs[i] = m.ID
	}
	if len(menuIDs) > 0 {
		err = tx.Role.UpdateOne(newRole).
			AddMenuIDs(menuIDs...).
			Exec(ctx)
		if err != nil {
			tx.Rollback()
			return nil, errors.NewInternalError("Failed to copy menu permissions", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, errors.NewInternalError("Failed to commit transaction", err)
	}
	s.syncRBACPolicies(ctx, "copy role")

	return newRole, nil
}

// SetRoleMenus updates the menus associated with a role.
func (s *RoleService) SetRoleMenus(ctx context.Context, roleID uuid.UUID, menuIDs []uuid.UUID) error {
	r, err := s.client.Role.Get(ctx, roleID)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Role not found", err)
		}
		return errors.NewInternalError("Failed to query role", err)
	}

	if r.Code == superAdminRoleCode {
		return errors.NewForbiddenError("Super admin role cannot be modified", nil)
	}

	// 验证菜单权限是否在父角色的菜单权限范围内
	if err := s.validateMenuPermissionsAgainstParent(ctx, roleID, menuIDs); err != nil {
		return err
	}

	// Update associations
	err = s.client.Role.UpdateOne(r).
		ClearMenus().
		AddMenuIDs(menuIDs...).
		Exec(ctx)
	if err != nil {
		return errors.NewInternalError("Failed to update menu permissions", err)
	}

	return nil
}

// SetRolePermissions updates the permission codes associated with a role.
func (s *RoleService) SetRolePermissions(ctx context.Context, roleID uuid.UUID, codes []string) error {
	r, err := s.client.Role.Get(ctx, roleID)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Role not found", err)
		}
		return errors.NewInternalError("Failed to query role", err)
	}

	if r.Code == superAdminRoleCode {
		return errors.NewForbiddenError("Super admin role cannot be modified", nil)
	}

	normalized := normalizePermissionCodes(codes)

	if len(normalized) > 0 {
		existing, err := s.client.Permission.Query().
			Where(
				permission.CodeIn(normalized...),
				permission.StatusEQ(permission.StatusActive),
			).
			All(ctx)
		if err != nil {
			return errors.NewInternalError("Failed to validate permission codes", err)
		}

		if len(existing) != len(normalized) {
			return errors.NewBadRequestError("Invalid or inactive permission codes", nil)
		}
	}

	if err := s.validatePermissionCodesAgainstParent(ctx, roleID, normalized); err != nil {
		return err
	}

	if err := s.client.Role.UpdateOne(r).
		SetPermissions(normalized).
		Exec(ctx); err != nil {
		return errors.NewInternalError("Failed to update role permissions", err)
	}
	s.syncRBACPolicies(ctx, "set role permissions")

	return nil
}

func (s *RoleService) UpsertRoleDataScopeRule(ctx context.Context, roleID uuid.UUID, in RoleDataScopeRuleInput) error {
	r, err := s.client.Role.Get(ctx, roleID)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Role not found", err)
		}
		return errors.NewInternalError("Failed to query role", err)
	}

	enforcer, err := s.getRBACEnforcer()
	if err != nil {
		return err
	}

	domain := strings.TrimSpace(in.Domain)
	resourceKey := strings.TrimSpace(in.ResourceKey)
	if domain == "" || resourceKey == "" {
		return errors.NewBadRequestError("Domain and resourceKey are required", nil)
	}

	scopeType, err := parsePolicyScopeType(in.ScopeType)
	if err != nil {
		return err
	}

	_, err = enforcer.RemoveFilteredNamedPolicy("p2", 0, r.Code, domain, resourceKey)
	if err != nil {
		return errors.NewInternalError("Failed to remove existing data scope rule", err)
	}

	_, err = enforcer.AddNamedPolicy("p2", r.Code, domain, resourceKey, scopeType, strings.TrimSpace(in.ScopeValue))
	if err != nil {
		return errors.NewInternalError("Failed to save data scope rule", err)
	}

	if err := s.rbacManager.SavePolicy(); err != nil {
		return errors.NewInternalError("Failed to persist data scope rule", err)
	}

	return nil
}

func (s *RoleService) ListRoleDataScopeRules(ctx context.Context, roleID uuid.UUID) ([]RoleDataScopeRule, error) {
	r, err := s.client.Role.Get(ctx, roleID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errors.NewNotFoundError("Role not found", err)
		}
		return nil, errors.NewInternalError("Failed to query role", err)
	}

	enforcer, err := s.getRBACEnforcer()
	if err != nil {
		return nil, err
	}

	policies := enforcer.GetFilteredNamedPolicy("p2", 0, r.Code)
	rules := make([]RoleDataScopeRule, 0, len(policies))
	for _, p := range policies {
		if len(p) < 4 {
			continue
		}
		rule := RoleDataScopeRule{
			Domain:      p[1],
			ResourceKey: p[2],
			ScopeType:   p[3],
		}
		if len(p) >= 5 {
			rule.ScopeValue = p[4]
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func (s *RoleService) DeleteRoleDataScopeRule(ctx context.Context, roleID uuid.UUID, domain, resourceKey string) error {
	r, err := s.client.Role.Get(ctx, roleID)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Role not found", err)
		}
		return errors.NewInternalError("Failed to query role", err)
	}

	enforcer, err := s.getRBACEnforcer()
	if err != nil {
		return err
	}

	trimmedDomain := strings.TrimSpace(domain)
	trimmedResource := strings.TrimSpace(resourceKey)
	if trimmedDomain == "" || trimmedResource == "" {
		return errors.NewBadRequestError("Domain and resourceKey are required", nil)
	}

	_, err = enforcer.RemoveFilteredNamedPolicy("p2", 0, r.Code, trimmedDomain, trimmedResource)
	if err != nil {
		return errors.NewInternalError("Failed to delete data scope rule", err)
	}

	if err := s.rbacManager.SavePolicy(); err != nil {
		return errors.NewInternalError("Failed to persist data scope rule deletion", err)
	}

	return nil
}

// ListRoleUsers retrieves users associated with a role.
func (s *RoleService) ListRoleUsers(ctx context.Context, roleID uuid.UUID, page, pageSize int) ([]*ent.User, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := s.client.User.Query().
		Where(user.HasRolesWith(role.ID(roleID)))

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errors.NewInternalError("Failed to count role users", err)
	}

	users, err := query.
		Offset(offset).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, errors.NewInternalError("Failed to query role users", err)
	}

	return users, total, nil
}

// GetRoleTree retrieves all roles as a tree structure.
func (s *RoleService) GetRoleTree(ctx context.Context) ([]*RoleTreeNode, error) {
	// 查询所有角色
	roles, err := s.client.Role.Query().
		WithParent().
		Order(ent.Asc(role.FieldSort)).
		All(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to query roles", err)
	}

	// 构建角色 map
	roleMap := make(map[uuid.UUID]*RoleTreeNode)
	for _, r := range roles {
		node := &RoleTreeNode{
			ID:          r.ID,
			Name:        r.Name,
			Code:        r.Code,
			Description: r.Description,
			Sort:        r.Sort,
			IsSystem:    r.IsSystem,
			Children:    []*RoleTreeNode{},
		}
		// 获取父角色 ID
		if r.Edges.Parent != nil {
			parentID := r.Edges.Parent.ID
			node.ParentID = &parentID
		}
		roleMap[r.ID] = node
	}

	// 构建树形结构
	var roots []*RoleTreeNode
	for _, node := range roleMap {
		if node.ParentID == nil {
			// 顶层角色
			roots = append(roots, node)
		} else {
			// 添加到父节点的 children
			if parent, ok := roleMap[*node.ParentID]; ok {
				parent.Children = append(parent.Children, node)
			}
		}
	}

	return roots, nil
}

// GetRoleChildren retrieves all child roles of a given role.
func (s *RoleService) GetRoleChildren(ctx context.Context, roleID uuid.UUID, recursive bool) ([]*ent.Role, error) {
	// 验证角色是否存在
	exists, err := s.client.Role.Query().Where(role.ID(roleID)).Exist(ctx)
	if err != nil {
		return nil, errors.NewInternalError("Failed to verify role", err)
	}
	if !exists {
		return nil, errors.NewNotFoundError("Role not found", nil)
	}

	if !recursive {
		// 只查询直接子角色
		children, err := s.client.Role.Query().
			Where(role.HasParentWith(role.ID(roleID))).
			Order(ent.Asc(role.FieldSort)).
			All(ctx)
		if err != nil {
			return nil, errors.NewInternalError("Failed to query children roles", err)
		}
		return children, nil
	}

	// 递归查询所有子孙角色
	var allChildren []*ent.Role
	var collectChildren func(uuid.UUID) error
	collectChildren = func(parentID uuid.UUID) error {
		children, err := s.client.Role.Query().
			Where(role.HasParentWith(role.ID(parentID))).
			Order(ent.Asc(role.FieldSort)).
			All(ctx)
		if err != nil {
			return err
		}

		allChildren = append(allChildren, children...)
		for _, child := range children {
			if err := collectChildren(child.ID); err != nil {
				return err
			}
		}
		return nil
	}

	if err := collectChildren(roleID); err != nil {
		return nil, errors.NewInternalError("Failed to query descendant roles", err)
	}

	return allChildren, nil
}

// GetRoleParents retrieves the parent role chain of a given role.
func (s *RoleService) GetRoleParents(ctx context.Context, roleID uuid.UUID) ([]*ent.Role, error) {
	var parents []*ent.Role

	currentID := roleID
	for {
		r, err := s.client.Role.Query().
			Where(role.ID(currentID)).
			WithParent().
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return nil, errors.NewNotFoundError("Role not found", err)
			}
			return nil, errors.NewInternalError("Failed to query role", err)
		}

		// 没有父角色了，结束
		if r.Edges.Parent == nil {
			break
		}

		parents = append([]*ent.Role{r.Edges.Parent}, parents...) // 插入到前面
		currentID = r.Edges.Parent.ID

		// 防止无限循环（理论上数据库约束应该避免这种情况）
		if len(parents) > 100 {
			return nil, errors.NewInternalError("Role hierarchy too deep or circular", nil)
		}
	}

	return parents, nil
}

// isDescendantOf checks if candidateParent is a descendant of role.
// This prevents circular role hierarchies.
func (s *RoleService) isDescendantOf(ctx context.Context, candidateParent, roleID uuid.UUID) (bool, error) {
	descendants, err := s.GetRoleChildren(ctx, roleID, true)
	if err != nil {
		return false, err
	}

	for _, d := range descendants {
		if d.ID == candidateParent {
			return true, nil
		}
	}
	return false, nil
}

// syncRoleInheritanceToCasbin syncs role inheritance relationship to Casbin.
func (s *RoleService) syncRoleInheritanceToCasbin(ctx context.Context, r *ent.Role) error {
	if s.rbacManager == nil {
		return nil
	}

	// 查询角色及其父角色
	roleWithParent, err := s.client.Role.Query().
		Where(role.ID(r.ID)).
		WithParent().
		Only(ctx)
	if err != nil {
		return errors.NewInternalError("Failed to query role with parent", err)
	}

	// 先移除旧的角色继承关系（g2 规则）
	// Note: framework/auth/rbac 需要提供 RemoveRoleInheritance 方法
	// 这里假设有这个方法，如果没有则需要添加
	var domain string
	if roleWithParent.OwnerDomainID != nil {
		domain = fmt.Sprintf("domain:%s", roleWithParent.OwnerDomainID.String())
	} else {
		domain = "platform:root"
	}
	if err := s.rbacManager.RemoveRoleInheritance(ctx, r.Code, domain); err != nil {
		s.logger.Warn("Failed to remove old role inheritance", zap.Error(err))
	}

	// 如果有父角色，添加新的继承关系
	if roleWithParent.Edges.Parent != nil {
		parentCode := roleWithParent.Edges.Parent.Code
		// 在 Casbin 中添加角色继承：g2, child_role, parent_role
		if err := s.rbacManager.AddRoleInheritance(ctx, r.Code, parentCode, domain); err != nil {
			return errors.NewInternalError("Failed to add role inheritance to Casbin", err)
		}
	}

	// 保存策略
	if err := s.rbacManager.SavePolicy(); err != nil {
		return errors.NewInternalError("Failed to save Casbin policy", err)
	}

	s.logger.Info("Role inheritance synced to Casbin",
		zap.String("role", r.Code),
		zap.String("parent", func() string {
			if roleWithParent.Edges.Parent != nil {
				return roleWithParent.Edges.Parent.Code
			}
			return "none"
		}()),
	)

	return nil
}

func parseDefaultRoleScope(input string) (string, error) {
	scope := strings.ToUpper(strings.TrimSpace(input))
	if scope == "" {
		return defaultRoleDataScope, nil
	}
	if !scopeTokenPattern.MatchString(scope) {
		return "", errors.NewBadRequestError("Invalid default data scope", nil)
	}
	return scope, nil
}

func parsePolicyScopeType(input string) (string, error) {
	scope := strings.ToUpper(strings.TrimSpace(input))
	if scope == "" || !scopeTokenPattern.MatchString(scope) {
		return "", errors.NewBadRequestError("Invalid scope type", nil)
	}
	return scope, nil
}

func (s *RoleService) getRBACEnforcer() (*casbinlib.Enforcer, error) {
	if s.rbacManager == nil || s.rbacManager.Enforcer() == nil {
		return nil, errors.NewInternalError("RBAC manager not configured", nil)
	}
	return s.rbacManager.Enforcer(), nil
}

// validatePermissionCodesAgainstParent validates that permission codes are within parent's scope.
func (s *RoleService) validatePermissionCodesAgainstParent(ctx context.Context, roleID uuid.UUID, codes []string) error {
	if len(codes) == 0 {
		return nil
	}

	r, err := s.client.Role.Query().
		Where(role.ID(roleID)).
		WithParent().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errors.NewNotFoundError("Role not found", err)
		}
		return errors.NewInternalError("Failed to query role", err)
	}

	if r.Edges.Parent == nil {
		return nil
	}

	parentPerms := r.Edges.Parent.Permissions
	parentPermMap := make(map[string]struct{}, len(parentPerms))
	for _, code := range parentPerms {
		parentPermMap[code] = struct{}{}
	}

	for _, code := range codes {
		if _, ok := parentPermMap[code]; !ok {
			return errors.NewForbiddenError("Cannot assign permissions that parent role does not have", nil)
		}
	}

	return nil
}

func normalizePermissionCodes(codes []string) []string {
	if len(codes) == 0 {
		return nil
	}
	unique := make(map[string]struct{}, len(codes))
	result := make([]string, 0, len(codes))
	for _, code := range codes {
		normalized := strings.ToLower(strings.TrimSpace(code))
		if normalized == "" {
			continue
		}
		if _, exists := unique[normalized]; exists {
			continue
		}
		unique[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func (s *RoleService) syncRBACPolicies(ctx context.Context, reason string) {
	if s.client == nil || s.rbacManager == nil || s.rbacManager.Enforcer() == nil {
		return
	}
	syncCtx := core.WithoutTenant(ctx)
	if err := rbacsync.FullResync(syncCtx, s.client, s.rbacManager, s.logger); err != nil {
		s.logger.Warn("Failed to resync RBAC policies after role mutation",
			zap.String("reason", reason),
			zap.Error(err),
		)
	}
}

// validateMenuPermissionsAgainstParent validates that menu permissions being assigned are within parent's scope.
func (s *RoleService) validateMenuPermissionsAgainstParent(ctx context.Context, roleID uuid.UUID, menuIDs []uuid.UUID) error {
	// 查询角色及其父角色
	r, err := s.client.Role.Query().
		Where(role.ID(roleID)).
		WithParent(func(q *ent.RoleQuery) {
			q.WithMenus()
		}).
		Only(ctx)
	if err != nil {
		return errors.NewInternalError("Failed to query role", err)
	}

	// 如果没有父角色，无需验证
	if r.Edges.Parent == nil {
		return nil
	}

	// 获取父角色的所有菜单权限 ID
	parentMenus := r.Edges.Parent.Edges.Menus
	parentMenuMap := make(map[uuid.UUID]bool)
	for _, m := range parentMenus {
		parentMenuMap[m.ID] = true
	}

	// 验证每个菜单权限是否在父角色的菜单权限范围内
	for _, menuID := range menuIDs {
		if !parentMenuMap[menuID] {
			return errors.NewForbiddenError("Cannot assign menu permissions that parent role does not have", nil)
		}
	}

	return nil
}

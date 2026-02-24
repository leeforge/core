package auth

import (
	"context"
	"fmt"

	casbinlib "github.com/casbin/casbin/v2"
	frameAuth "github.com/leeforge/framework/auth"
	"github.com/leeforge/framework/auth/abac"
	"github.com/leeforge/framework/auth/rbac"
	"github.com/leeforge/framework/logging"
	"go.uber.org/zap"
)

// PermissionManager 权限管理器
// 封装 framework/auth 的 RBAC/ABAC 功能，提供统一的权限管理接口
type PermissionManager struct {
	authCore    *frameAuth.AuthCore
	RBACManager *rbac.RBACManager
	ABACManager *abac.ABACManager
	logger      logging.Logger
}

// NewPermissionManager 创建权限管理器
// 参数:
//   - authCore: framework/auth 实例（已初始化）
//   - logger: 日志记录器
func NewPermissionManager(
	authCore *frameAuth.AuthCore,
	logger logging.Logger,
) (*PermissionManager, error) {
	logger.Info("Permission manager initialized successfully",
		zap.Bool("rbac_enabled", true),
		zap.Bool("abac_enabled", true),
	)

	return &PermissionManager{
		authCore:    authCore,
		RBACManager: authCore.RBACManager,
		ABACManager: authCore.ABACManager,
		logger:      logger,
	}, nil
}

// CheckUserPermission 检查用户是否有权限访问指定资源
// 参数:
//   - ctx: 上下文
//   - userUUID: 用户 UUID
//   - domain: 域 (platform 或 tenant:{id})
//   - resource: 资源标识 (如 "user", "role", "menu")
//   - action: 操作类型 (如 "read", "write", "delete")
//
// 返回:
//   - bool: 是否有权限
//   - error: 错误信息
func (m *PermissionManager) CheckUserPermission(
	ctx context.Context,
	userUUID string,
	domain string,
	resource string,
	action string,
) (bool, error) {
	return m.RBACManager.CheckPermission(ctx, userUUID, domain, resource, action)
}

// AssignRoleToUser 分配角色给用户
// 参数:
//   - ctx: 上下文
//   - userUUID: 用户 UUID
//   - roleCode: 角色代码 (如 "admin", "user", "guest")
//   - domain: 域 (platform 或 tenant:{id})
func (m *PermissionManager) AssignRoleToUser(ctx context.Context, userUUID string, roleCode string, domain string) error {
	err := m.RBACManager.AssignRole(ctx, userUUID, roleCode, domain)
	if err != nil {
		m.logger.Error("Failed to assign role to user",
			zap.String("user_uuid", userUUID),
			zap.String("role_code", roleCode),
			zap.Error(err),
		)
		return err
	}

	m.logger.Info("Role assigned to user successfully",
		zap.String("user_uuid", userUUID),
		zap.String("role_code", roleCode),
	)
	return nil
}

// RevokeRoleFromUser 撤销用户角色
func (m *PermissionManager) RevokeRoleFromUser(ctx context.Context, userUUID string, roleCode string, domain string) error {
	err := m.RBACManager.RevokeRole(ctx, userUUID, roleCode, domain)
	if err != nil {
		m.logger.Error("Failed to revoke role from user",
			zap.String("user_uuid", userUUID),
			zap.String("role_code", roleCode),
			zap.Error(err),
		)
		return err
	}

	m.logger.Info("Role revoked from user successfully",
		zap.String("user_uuid", userUUID),
		zap.String("role_code", roleCode),
	)
	return nil
}

// GetUserRoles 获取用户的所有角色
func (m *PermissionManager) GetUserRoles(ctx context.Context, userUUID, domain string) ([]*rbac.Role, error) {
	return m.RBACManager.GetUserRoles(ctx, userUUID, domain)
}

// AddRolePermission 为角色添加权限
// 参数:
//   - ctx: 上下文
//   - roleCode: 角色代码
//   - domain: 域 (platform 或 tenant:{id})
//   - resource: 资源标识
//   - action: 操作类型
func (m *PermissionManager) AddRolePermission(
	ctx context.Context,
	roleCode string,
	domain string,
	resource string,
	action string,
) error {
	err := m.RBACManager.AddPermission(ctx, roleCode, domain, resource, action)
	if err != nil {
		m.logger.Error("Failed to add permission to role",
			zap.String("role_code", roleCode),
			zap.String("resource", resource),
			zap.String("action", action),
			zap.Error(err),
		)
		return err
	}

	m.logger.Info("Permission added to role successfully",
		zap.String("role_code", roleCode),
		zap.String("resource", resource),
		zap.String("action", action),
	)
	return nil
}

// RemoveRolePermission 移除角色的权限
func (m *PermissionManager) RemoveRolePermission(
	ctx context.Context,
	roleCode string,
	domain string,
	resource string,
	action string,
) error {
	err := m.RBACManager.RemovePermission(ctx, roleCode, domain, resource, action)
	if err != nil {
		m.logger.Error("Failed to remove permission from role",
			zap.String("role_code", roleCode),
			zap.String("resource", resource),
			zap.String("action", action),
			zap.Error(err),
		)
		return err
	}

	m.logger.Info("Permission removed from role successfully",
		zap.String("role_code", roleCode),
		zap.String("resource", resource),
		zap.String("action", action),
	)
	return nil
}

// GetRolePermissions 获取角色的所有权限
func (m *PermissionManager) GetRolePermissions(ctx context.Context, roleCode, domain string) ([]*rbac.Permission, error) {
	return m.RBACManager.GetPermissions(ctx, roleCode, domain)
}

// SyncRolePermissions 同步角色权限到 Casbin
// 用于批量更新角色权限，会先清除旧权限再添加新权限
func (m *PermissionManager) SyncRolePermissions(
	ctx context.Context,
	roleCode string,
	domain string,
	permissions []rbac.Permission,
) error {
	// 1. 移除角色的所有权限
	existingPerms, err := m.RBACManager.GetPermissions(ctx, roleCode, domain)
	if err != nil {
		return fmt.Errorf("failed to get existing permissions: %w", err)
	}

	for _, perm := range existingPerms {
		err := m.RBACManager.RemovePermission(ctx, perm.RoleCode, domain, perm.Resource, perm.Action)
		if err != nil {
			return fmt.Errorf("failed to remove old permission: %w", err)
		}
	}

	// 2. 添加新权限
	for _, perm := range permissions {
		err := m.RBACManager.AddPermission(ctx, roleCode, domain, perm.Resource, perm.Action)
		if err != nil {
			return fmt.Errorf("failed to add new permission: %w", err)
		}
	}

	// 3. 保存策略
	if err := m.RBACManager.SavePolicy(); err != nil {
		return fmt.Errorf("failed to save policy: %w", err)
	}

	m.logger.Info("Role permissions synced successfully",
		zap.String("role_code", roleCode),
		zap.Int("permission_count", len(permissions)),
	)
	return nil
}

// CreateABACPolicy 创建 ABAC 策略
func (m *PermissionManager) CreateABACPolicy(ctx context.Context, rule abac.PolicyRule) error {
	return m.ABACManager.CreatePolicy(ctx, rule)
}

// CheckABACPermission 检查基于属性的权限
func (m *PermissionManager) CheckABACPermission(
	ctx context.Context,
	userAttrs map[string]any,
	resourceAttrs map[string]any,
	action string,
	contextAttrs map[string]any,
) (bool, error) {
	return m.ABACManager.CheckPermission(ctx, userAttrs, resourceAttrs, action, contextAttrs)
}

// CreateUnifiedAuthMiddleware 创建统一认证中间件
// 集成 JWT、API Key、RBAC、ABAC 等所有认证和授权功能
func (m *PermissionManager) CreateUnifiedAuthMiddleware(
	config frameAuth.AuthConfig,
	apiKeyStore frameAuth.APIKeyStore,
) *frameAuth.UnifiedAuthMiddleware {
	return frameAuth.NewUnifiedAuthMiddleware(
		config,
		apiKeyStore,
		m.RBACManager,
		m.ABACManager,
		m.logger.Zap(),
	)
}

// LoadPolicy 从数据库加载策略到内存
func (m *PermissionManager) LoadPolicy() error {
	return m.RBACManager.LoadPolicy()
}

// SavePolicy 保存策略到数据库
func (m *PermissionManager) SavePolicy() error {
	return m.RBACManager.SavePolicy()
}

// Enforcer returns the underlying Casbin enforcer for advanced policy operations.
func (m *PermissionManager) Enforcer() *casbinlib.Enforcer {
	return m.RBACManager.Enforcer()
}

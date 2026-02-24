package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/leeforge/framework/logging"
	"go.uber.org/zap"

	"github.com/leeforge/core/core"
)

// QueryFilterConfig 查询过滤配置
type QueryFilterConfig struct {
	Logger          logging.Logger
	SuperAdminRoles []string
	BypassRoles     []string
}

// DefaultQueryFilterConfig 默认查询过滤配置
func DefaultQueryFilterConfig(logger logging.Logger) *QueryFilterConfig {
	return &QueryFilterConfig{
		Logger:          logger,
		SuperAdminRoles: []string{"super_admin", "admin"},
		BypassRoles:     []string{},
	}
}

// QueryFilter 查询过滤器
type QueryFilter struct {
	config *QueryFilterConfig
}

// NewQueryFilter 创建查询过滤器
func NewQueryFilter(config *QueryFilterConfig) *QueryFilter {
	if config == nil {
		config = DefaultQueryFilterConfig(nil)
	}
	return &QueryFilter{config: config}
}

// ShouldFilter 判断是否需要过滤
// 返回 (needFilter bool, userID uuid.UUID)
func (qf *QueryFilter) ShouldFilter(ctx context.Context) (bool, uuid.UUID) {
	identity, ok := core.GetIdentity(ctx)
	if !ok {
		// 没有身份信息，不过滤（系统调用）
		return false, uuid.Nil
	}

	// 检查是否是超级管理员或绕过角色
	if qf.shouldBypass(identity) {
		if qf.config.Logger != nil {
			qf.config.Logger.Debug("Bypass data permission filter",
				zap.String("user_id", identity.UserID.String()),
				zap.Strings("roles", identity.Roles),
			)
		}
		return false, identity.UserID
	}

	// 普通用户需要过滤
	if qf.config.Logger != nil {
		qf.config.Logger.Debug("Apply data permission filter",
			zap.String("user_id", identity.UserID.String()),
		)
	}
	return true, identity.UserID
}

// shouldBypass 检查是否应该绕过过滤
func (qf *QueryFilter) shouldBypass(identity core.Identity) bool {
	for _, role := range identity.Roles {
		// 检查超级管理员角色
		for _, superRole := range qf.config.SuperAdminRoles {
			if role == superRole {
				return true
			}
		}
		// 检查绕过角色
		for _, bypassRole := range qf.config.BypassRoles {
			if role == bypassRole {
				return true
			}
		}
	}
	return false
}

// GetUserID 获取当前用户ID（不检查角色）
func (qf *QueryFilter) GetUserID(ctx context.Context) (uuid.UUID, bool) {
	identity, ok := core.GetIdentity(ctx)
	if !ok {
		return uuid.Nil, false
	}
	return identity.UserID, true
}

// GetIdentity 获取当前用户身份
func (qf *QueryFilter) GetIdentity(ctx context.Context) (core.Identity, bool) {
	return core.GetIdentity(ctx)
}

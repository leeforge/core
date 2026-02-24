package dataperm

import (
	"context"

	"github.com/google/uuid"

	"github.com/leeforge/core/core"
)

// Filter 数据权限过滤器
type Filter struct {
	superAdminRoles []string
	bypassRoles     []string
}

// NewFilter 创建过滤器
func NewFilter() *Filter {
	return &Filter{
		superAdminRoles: []string{"super_admin", "admin"},
		bypassRoles:     []string{},
	}
}

// WithSuperAdminRoles 设置超级管理员角色
func (f *Filter) WithSuperAdminRoles(roles []string) *Filter {
	f.superAdminRoles = roles
	return f
}

// WithBypassRoles 设置绕过角色
func (f *Filter) WithBypassRoles(roles []string) *Filter {
	f.bypassRoles = roles
	return f
}

// ShouldFilterByCreator 判断是否需要按创建者过滤
// 返回 (needFilter bool, creatorID uuid.UUID)
//
// 使用示例：
//
//	if needFilter, userID := filter.ShouldFilterByCreator(ctx); needFilter {
//	    query = query.Where(entity.CreatedByID(userID))
//	}
func (f *Filter) ShouldFilterByCreator(ctx context.Context) (bool, uuid.UUID) {
	identity, ok := core.GetIdentity(ctx)
	if !ok {
		// 没有身份信息，不过滤（系统调用）
		return false, uuid.Nil
	}

	// 检查是否是超级管理员或绕过角色
	if f.shouldBypass(identity) {
		return false, identity.UserID
	}

	// 普通用户需要过滤
	return true, identity.UserID
}

// GetCreatorID 获取当前创建者ID（用于创建时设置）
// 如果没有身份信息，返回 uuid.Nil
func (f *Filter) GetCreatorID(ctx context.Context) uuid.UUID {
	identity, ok := core.GetIdentity(ctx)
	if !ok {
		return uuid.Nil
	}
	return identity.UserID
}

// IsAdmin 判断当前用户是否是管理员
func (f *Filter) IsAdmin(ctx context.Context) bool {
	identity, ok := core.GetIdentity(ctx)
	if !ok {
		return false
	}

	for _, role := range identity.Roles {
		for _, adminRole := range f.superAdminRoles {
			if role == adminRole {
				return true
			}
		}
	}
	return false
}

// HasRole 判断当前用户是否有指定角色
func (f *Filter) HasRole(ctx context.Context, targetRole string) bool {
	identity, ok := core.GetIdentity(ctx)
	if !ok {
		return false
	}

	for _, role := range identity.Roles {
		if role == targetRole {
			return true
		}
	}
	return false
}

// shouldBypass 检查是否应该绕过过滤
func (f *Filter) shouldBypass(identity core.Identity) bool {
	for _, role := range identity.Roles {
		// 检查超级管理员角色
		for _, superRole := range f.superAdminRoles {
			if role == superRole {
				return true
			}
		}
		// 检查绕过角色
		for _, bypassRole := range f.bypassRoles {
			if role == bypassRole {
				return true
			}
		}
	}
	return false
}

// 全局默认过滤器实例
var defaultFilter = NewFilter()

// ShouldFilterByCreator 全局方法：判断是否需要按创建者过滤
func ShouldFilterByCreator(ctx context.Context) (bool, uuid.UUID) {
	return defaultFilter.ShouldFilterByCreator(ctx)
}

// GetCreatorID 全局方法：获取当前创建者ID
func GetCreatorID(ctx context.Context) uuid.UUID {
	return defaultFilter.GetCreatorID(ctx)
}

// IsAdmin 全局方法：判断当前用户是否是管理员
func IsAdmin(ctx context.Context) bool {
	return defaultFilter.IsAdmin(ctx)
}

// HasRole 全局方法：判断当前用户是否有指定角色
func HasRole(ctx context.Context, role string) bool {
	return defaultFilter.HasRole(ctx, role)
}

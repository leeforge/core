package role

import "github.com/google/uuid"

// CreateRoleRequest represents the request body for creating a role.
type CreateRoleRequest struct {
	Name             string     `json:"name" binding:"required"`
	Code             string     `json:"code" binding:"required"`
	DefaultDataScope string     `json:"defaultDataScope,omitempty"`
	Description      string     `json:"description,omitempty"`
	Sort             int        `json:"sort"`
	ParentID         *uuid.UUID `json:"parentId,omitempty"` // 父角色 ID（可选）
	Permissions      []string   `json:"permissions,omitempty"`
}

// UpdateRoleRequest represents the request body for updating a role.
type UpdateRoleRequest struct {
	Name             *string    `json:"name,omitempty"`
	Description      *string    `json:"description,omitempty"`
	Sort             *int       `json:"sort,omitempty"`
	ParentID         *uuid.UUID `json:"parentId,omitempty"` // 父角色 ID（可选，nil 表示移除父角色）
	DefaultDataScope *string    `json:"defaultDataScope,omitempty"`
}

// CopyRoleRequest represents the request body for cloning a role.
type CopyRoleRequest struct {
	Name string `json:"name" binding:"required"`
	Code string `json:"code" binding:"required"`
}

// SetRoleMenusRequest represents the request body for setting role menu permissions.
type SetRoleMenusRequest struct {
	MenuIDs []uuid.UUID `json:"menuIds" binding:"required"`
}

// SetRolePermissionsRequest represents the request body for setting role permission codes.
type SetRolePermissionsRequest struct {
	PermissionCodes []string `json:"permissionCodes" binding:"required"`
}

// RoleTreeNode represents a role node in the tree structure.
type RoleTreeNode struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Code        string          `json:"code"`
	Description string          `json:"description,omitempty"`
	Sort        int             `json:"sort"`
	IsSystem    bool            `json:"isSystem"`
	ParentID    *uuid.UUID      `json:"parentId,omitempty"`
	Children    []*RoleTreeNode `json:"children,omitempty"`
}

package user

import (
	"github.com/google/uuid"
)

// ==================== User DTOs ====================

// UserProfileDTO represents user profile with roles and permissions
type UserProfileDTO struct {
	ID           uuid.UUID      `json:"id"`
	Username     string         `json:"username"`
	Email        string         `json:"email"`
	Nickname     string         `json:"nickname"`
	Phone        string         `json:"phone,omitempty"`
	Avatar       string         `json:"avatar,omitempty"`
	Status       string         `json:"status"`
	Settings     map[string]any `json:"settings,omitempty"`
	IsSuperAdmin bool           `json:"isSuperAdmin,omitempty"`
	Roles        []RoleDTO      `json:"roles,omitempty"`
}

// RoleDTO represents a role with permissions
type RoleDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Permissions []string  `json:"permissions,omitempty"`
}

// UserBasicDTO represents basic user information
type UserBasicDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Status   string    `json:"status,omitempty"`
}

// UserListFilters represents filters for listing users
type UserListFilters struct {
	Query          string    // Search query (username, email, phone)
	Status         string    // Status filter (active, suspended, inactive)
	RoleID         uuid.UUID // Role ID filter
	IncludeDeleted bool      // Include soft-deleted users
	Page           int       // Page number (1-based)
	PageSize       int       // Items per page
}

// UserListResult represents paginated user list result
type UserListResult struct {
	Users      []*UserProfileDTO `json:"users"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"pageSize"`
	TotalPages int               `json:"totalPages"`
}

// UpdateProfileRequest represents profile update data
type UpdateProfileRequest struct {
	Nickname string `json:"nickname,omitempty"`
	Email    string `json:"email,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
}

// RestoreUserRequest represents the restore user request
type RestoreUserRequest struct {
	// Empty body - ID comes from URL
}

// FreezeUserRequest represents the freeze/unfreeze user request
type FreezeUserRequest struct {
	Status string `json:"status"` // "suspended" or "active"
}

// AssignRolesRequest represents the assign roles request
type AssignRolesRequest struct {
	RoleIDs []string `json:"roleIds"` // Array of role UUIDs
}

// UpdateSettingsRequest represents the update settings request
type UpdateSettingsRequest struct {
	Settings map[string]any `json:"settings"`
}

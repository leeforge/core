package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/role"
	"github.com/leeforge/core/server/ent/user"
	"github.com/leeforge/core/server/services/rbacsync"

	"github.com/leeforge/framework/auth/rbac"
	"github.com/leeforge/framework/logging"
)

// Sentinel errors
var (
	ErrUserNotFound = errors.New("user not found")
	// ErrSuperAdminProtected indicates operations are blocked on super admin users.
	ErrSuperAdminProtected = errors.New("super admin user is protected")
)

const superAdminRoleCode = "super_admin"

// UserService provides user management operations
type UserService struct {
	client      *ent.Client
	logger      logging.Logger
	rbacManager *rbac.RBACManager
}

// NewUserService creates a new instance of UserService
func NewUserService(client *ent.Client, logger logging.Logger) *UserService {
	return &UserService{
		client: client,
		logger: logger,
	}
}

func (s *UserService) WithRBACManager(rbacManager *rbac.RBACManager) *UserService {
	s.rbacManager = rbacManager
	return s
}

// DeleteUser performs soft-delete on user record.
func (s *UserService) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	s.logger.Info("Deleting user", zap.String("user_id", userID.String()))

	if err := s.guardSuperAdminOperation(ctx, userID); err != nil {
		return err
	}

	_, err := s.client.User.UpdateOneID(userID).
		SetDeletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("user not found: %w", ErrUserNotFound)
		}
		return fmt.Errorf("failed to delete user: %w", err)
	}

	s.logger.Info("User deleted successfully", zap.String("user_id", userID.String()))
	return nil
}

// RestoreUser restores a soft-deleted user
func (s *UserService) RestoreUser(ctx context.Context, userID uuid.UUID) error {
	s.logger.Info("Restoring user", zap.String("user_id", userID.String()))

	// Check if user exists and is deleted
	u, err := s.client.User.Query().
		Where(user.ID(userID)).
		Where(user.DeletedAtNotNil()).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("user not found or not deleted: %w", ErrUserNotFound)
		}
		return fmt.Errorf("failed to query user: %w", err)
	}

	if _, err := s.client.User.UpdateOneID(u.ID).ClearDeletedAt().Save(ctx); err != nil {
		return fmt.Errorf("failed to restore user: %w", err)
	}

	s.logger.Info("User restored successfully", zap.String("user_id", userID.String()))
	return nil
}

// FreezeUser sets user status to "suspended"
func (s *UserService) FreezeUser(ctx context.Context, userID uuid.UUID) error {
	s.logger.Info("Freezing user", zap.String("user_id", userID.String()))

	if err := s.guardSuperAdminOperation(ctx, userID); err != nil {
		return err
	}

	_, err := s.client.User.UpdateOneID(userID).
		SetStatus(user.StatusSuspended).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("user not found: %w", ErrUserNotFound)
		}
		return fmt.Errorf("failed to freeze user: %w", err)
	}

	s.logger.Info("User frozen successfully", zap.String("user_id", userID.String()))
	return nil
}

// UnfreezeUser sets user status to "active"
func (s *UserService) UnfreezeUser(ctx context.Context, userID uuid.UUID) error {
	s.logger.Info("Unfreezing user", zap.String("user_id", userID.String()))

	if err := s.guardSuperAdminOperation(ctx, userID); err != nil {
		return err
	}

	_, err := s.client.User.UpdateOneID(userID).
		SetStatus(user.StatusActive).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("user not found: %w", ErrUserNotFound)
		}
		return fmt.Errorf("failed to unfreeze user: %w", err)
	}

	s.logger.Info("User unfrozen successfully", zap.String("user_id", userID.String()))
	return nil
}

// AssignRoles assigns multiple roles to a user
func (s *UserService) AssignRoles(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) error {
	s.logger.Info("Assigning roles to user",
		zap.String("user_id", userID.String()),
		zap.Int("role_count", len(roleIDs)))

	// guardSuperAdminOperation also verifies user existence.
	if err := s.guardSuperAdminOperation(ctx, userID); err != nil {
		return err
	}

	// Clear existing roles and add new ones
	update := s.client.User.UpdateOneID(userID).ClearRoles()

	if len(roleIDs) > 0 {
		update = update.AddRoleIDs(roleIDs...)
	}

	_, err := update.Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to assign roles: %w", err)
	}

	s.logger.Info("Roles assigned successfully",
		zap.String("user_id", userID.String()),
		zap.Int("role_count", len(roleIDs)))
	s.syncRBACPolicies(ctx, "assign roles")
	return nil
}

// GetProfile retrieves user profile with roles and permissions loaded
func (s *UserService) GetProfile(ctx context.Context, userID uuid.UUID) (*UserProfileDTO, error) {
	s.logger.Debug("Getting user profile", zap.String("user_id", userID.String()))

	u, err := s.client.User.Query().
		Where(user.ID(userID)).
		WithRoles().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("user not found: %w", ErrUserNotFound)
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	return s.toUserProfileDTO(u), nil
}

// UpdateProfile updates user profile information
func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, req UpdateProfileRequest) (*UserProfileDTO, error) {
	s.logger.Info("Updating user profile", zap.String("user_id", userID.String()))

	if req.Email != "" || req.Phone != "" {
		isSuper, err := s.isSuperAdminUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		if isSuper {
			req.Email = ""
			req.Phone = ""
		}
	}

	update := s.client.User.UpdateOneID(userID)

	if req.Nickname != "" {
		update = update.SetNickname(req.Nickname)
	}
	if req.Email != "" {
		update = update.SetEmail(req.Email)
	}
	if req.Phone != "" {
		update = update.SetPhone(req.Phone)
	}
	if req.Avatar != "" {
		update = update.SetAvatar(req.Avatar)
	}

	u, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("user not found: %w", ErrUserNotFound)
		}
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	// Reload with roles
	u, err = s.client.User.Query().
		Where(user.ID(userID)).
		WithRoles().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to reload user: %w", err)
	}

	s.logger.Info("Profile updated successfully", zap.String("user_id", userID.String()))
	return s.toUserProfileDTO(u), nil
}

// UpdateSettings updates user settings JSON
func (s *UserService) UpdateSettings(ctx context.Context, userID uuid.UUID, settings map[string]any) error {
	s.logger.Info("Updating user settings", zap.String("user_id", userID.String()))

	_, err := s.client.User.UpdateOneID(userID).
		SetMetadata(convertToStringMap(settings)).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("user not found: %w", ErrUserNotFound)
		}
		return fmt.Errorf("failed to update settings: %w", err)
	}

	s.logger.Info("Settings updated successfully", zap.String("user_id", userID.String()))
	return nil
}

func (s *UserService) guardSuperAdminOperation(ctx context.Context, userID uuid.UUID) error {
	isSuper, err := s.isSuperAdminUser(ctx, userID)
	if err != nil {
		return err
	}
	if isSuper {
		return ErrSuperAdminProtected
	}
	return nil
}

func (s *UserService) isSuperAdminUser(ctx context.Context, userID uuid.UUID) (bool, error) {
	u, err := s.client.User.Query().
		Where(user.ID(userID)).
		Select(user.FieldID, user.FieldIsSuperAdmin).
		WithRoles(func(q *ent.RoleQuery) {
			q.Select(role.FieldCode)
		}).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return false, fmt.Errorf("user not found: %w", ErrUserNotFound)
		}
		return false, fmt.Errorf("failed to query user: %w", err)
	}

	if u.IsSuperAdmin {
		return true, nil
	}
	for _, r := range u.Edges.Roles {
		if r.Code == superAdminRoleCode {
			return true, nil
		}
	}
	return false, nil
}

// ListUsers retrieves users with enhanced search and filtering
func (s *UserService) ListUsers(ctx context.Context, filters UserListFilters) (*UserListResult, error) {
	s.logger.Debug("Listing users", zap.Any("filters", filters))

	// Default pagination
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PageSize < 1 {
		filters.PageSize = 20
	}
	if filters.PageSize > 100 {
		filters.PageSize = 100 // Max page size
	}

	// Build query
	query := s.client.User.Query()

	// Apply soft delete filter
	if filters.IncludeDeleted {
		// Include all users (don't filter by deleted_at)
	} else {
		// Exclude soft-deleted users (default behavior)
		query = query.Where(user.DeletedAtIsNil())
	}

	// Search filter
	if filters.Query != "" {
		searchTerm := "%" + strings.ToLower(filters.Query) + "%"
		query = query.Where(
			user.Or(
				user.UsernameContains(searchTerm),
				user.EmailContains(searchTerm),
				user.PhoneContains(searchTerm),
				user.NicknameContains(searchTerm),
			),
		)
	}

	// Status filter
	if filters.Status != "" {
		switch filters.Status {
		case "active":
			query = query.Where(user.StatusEQ(user.StatusActive))
		case "inactive":
			query = query.Where(user.StatusEQ(user.StatusInactive))
		case "suspended":
			query = query.Where(user.StatusEQ(user.StatusSuspended))
		}
	}

	// Role filter
	if filters.RoleID != uuid.Nil {
		query = query.Where(user.HasRolesWith(role.ID(filters.RoleID)))
	}

	// Count total
	total, err := query.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// Apply pagination
	offset := (filters.Page - 1) * filters.PageSize
	users, err := query.
		WithRoles().
		Offset(offset).
		Limit(filters.PageSize).
		Order(ent.Desc(user.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}

	// Convert to DTOs
	userDTOs := make([]*UserProfileDTO, len(users))
	for i, u := range users {
		userDTOs[i] = s.toUserProfileDTO(u)
	}

	totalPages := (total + filters.PageSize - 1) / filters.PageSize

	return &UserListResult{
		Users:      userDTOs,
		Total:      total,
		Page:       filters.Page,
		PageSize:   filters.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(ctx context.Context, userID uuid.UUID) (*ent.User, error) {
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("user not found: %w", ErrUserNotFound)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return u, nil
}

// toUserProfileDTO converts ent.User to UserProfileDTO
func (s *UserService) toUserProfileDTO(u *ent.User) *UserProfileDTO {
	dto := &UserProfileDTO{
		ID:           u.ID,
		Username:     u.Username,
		Email:        u.Email,
		Nickname:     u.Nickname,
		Phone:        u.Phone,
		Avatar:       u.Avatar,
		IsSuperAdmin: u.IsSuperAdmin,
		Status:       string(u.Status),
		Settings:     convertFromStringMap(u.Metadata),
	}

	// Add roles if loaded
	if u.Edges.Roles != nil {
		dto.Roles = make([]RoleDTO, len(u.Edges.Roles))
		for i, r := range u.Edges.Roles {
			dto.Roles[i] = RoleDTO{
				ID:          r.ID,
				Name:        r.Name,
				Description: r.Description,
				Permissions: r.Permissions,
			}
		}
	}

	return dto
}

// convertToStringMap converts map[string]any to map[string]string for Ent
func convertToStringMap(input map[string]any) map[string]string {
	result := make(map[string]string)
	for k, v := range input {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

// convertFromStringMap converts map[string]string to map[string]any for response
func convertFromStringMap(input map[string]string) map[string]any {
	result := make(map[string]any)
	for k, v := range input {
		result[k] = v
	}
	return result
}

func (s *UserService) syncRBACPolicies(ctx context.Context, reason string) {
	if s.client == nil || s.rbacManager == nil || s.rbacManager.Enforcer() == nil {
		return
	}
	if err := rbacsync.FullResync(core.WithoutTenant(ctx), s.client, s.rbacManager, s.logger); err != nil {
		s.logger.Warn("Failed to resync RBAC policies after user-role mutation",
			zap.String("reason", reason),
			zap.Error(err),
		)
	}
}

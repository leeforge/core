package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/httplog"
	"github.com/leeforge/core/server/pkg/crypto"

	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

type UserHandler struct {
	userService          *UserService
	invitationService    *InvitationService
	passwordResetService *PasswordResetService
	logger               logging.Logger
}

func NewUserHandler(
	userService *UserService,
	invitationService *InvitationService,
	passwordResetService *PasswordResetService,
	logger logging.Logger,
) *UserHandler {
	return &UserHandler{
		userService:          userService,
		invitationService:    invitationService,
		passwordResetService: passwordResetService,
		logger:               logger,
	}
}

// DeleteUser handles soft-deleting a user.
// @Summary Delete a user
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]string
// @Router /users/{id} [delete]
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid user ID")
		return
	}

	if err := h.userService.DeleteUser(r.Context(), userID); err != nil {
		httplog.Error(h.logger, r, "Failed to delete user", err, zap.String("user_id", userID.String()))
		if errors.Is(err, ErrSuperAdminProtected) {
			responder.Forbidden(w, r, "Super admin user cannot be deleted")
			return
		}
		if errors.Is(err, ErrUserNotFound) {
			responder.NotFound(w, r, "User not found")
			return
		}
		responder.DatabaseError(w, r, "Failed to delete user")
		return
	}

	responder.OK(w, r, map[string]string{"message": "User deleted successfully"})
}

// RestoreUser handles restoring a soft-deleted user
// @Summary Restore a soft-deleted user
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]string
// @Router /users/{id}/restore [post]
func (h *UserHandler) RestoreUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid user ID")
		return
	}

	if err := h.userService.RestoreUser(r.Context(), userID); err != nil {
		httplog.Error(h.logger, r, "Failed to restore user", err, zap.String("user_id", userID.String()))
		if errors.Is(err, ErrUserNotFound) {
			responder.NotFound(w, r, "User not found or not deleted")
			return
		}
		responder.DatabaseError(w, r, "Failed to restore user")
		return
	}

	responder.OK(w, r, map[string]string{"message": "User restored successfully"})
}

// FreezeUser handles freezing/unfreezing a user
// @Summary Freeze or unfreeze a user
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body FreezeUserRequest true "Freeze request"
// @Success 200 {object} map[string]string
// @Router /users/{id}/freeze [post]
func (h *UserHandler) FreezeUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid user ID")
		return
	}

	var req FreezeUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	// Determine action based on status
	var action error
	var message string

	switch req.Status {
	case "suspended":
		action = h.userService.FreezeUser(r.Context(), userID)
		message = "User frozen successfully"
	case "active":
		action = h.userService.UnfreezeUser(r.Context(), userID)
		message = "User unfrozen successfully"
	default:
		responder.BadRequest(w, r, "Invalid status. Use 'suspended' or 'active'")
		return
	}

	if action != nil {
		httplog.Error(h.logger, r, "Failed to update user status", action, zap.String("user_id", userID.String()))
		if errors.Is(action, ErrSuperAdminProtected) {
			responder.Forbidden(w, r, "Super admin user cannot be modified")
			return
		}
		if errors.Is(action, ErrUserNotFound) {
			responder.NotFound(w, r, "User not found")
			return
		}
		responder.DatabaseError(w, r, "Failed to update user status")
		return
	}

	responder.OK(w, r, map[string]string{"message": message})
}

// AssignRoles handles assigning roles to a user
// @Summary Assign roles to a user
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body AssignRolesRequest true "Role IDs"
// @Success 200 {object} map[string]string
// @Router /users/{id}/roles [post]
func (h *UserHandler) AssignRoles(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid user ID")
		return
	}

	var req AssignRolesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	// Parse role IDs
	roleIDs := make([]uuid.UUID, len(req.RoleIDs))
	for i, idStr := range req.RoleIDs {
		roleID, err := uuid.Parse(idStr)
		if err != nil {
			responder.BadRequest(w, r, "Invalid role ID format")
			return
		}
		roleIDs[i] = roleID
	}

	if err := h.userService.AssignRoles(r.Context(), userID, roleIDs); err != nil {
		httplog.Error(h.logger, r, "Failed to assign roles", err, zap.String("user_id", userID.String()))
		if errors.Is(err, ErrSuperAdminProtected) {
			responder.Forbidden(w, r, "Super admin user cannot be modified")
			return
		}
		if errors.Is(err, ErrUserNotFound) {
			responder.NotFound(w, r, "User not found")
			return
		}
		responder.DatabaseError(w, r, "Failed to assign roles")
		return
	}

	responder.OK(w, r, map[string]string{"message": "Roles assigned successfully"})
}

// GetProfile handles getting current user's profile
// @Summary Get current user profile
// @Tags Profile
// @Accept json
// @Produce json
// @Success 200 {object} UserProfileDTO
// @Router /profile [get]
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := core.GetUserID(r.Context())
	if !ok {
		responder.Unauthorized(w, r, "User not found in context")
		return
	}

	profile, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		httplog.Error(h.logger, r, "Failed to get profile", err, zap.String("user_id", userID.String()))
		if errors.Is(err, ErrUserNotFound) {
			responder.NotFound(w, r, "User not found")
			return
		}
		responder.DatabaseError(w, r, "Failed to get profile")
		return
	}

	responder.OK(w, r, profile)
}

// UpdateProfile handles updating current user's profile
// @Summary Update current user profile
// @Tags Profile
// @Accept json
// @Produce json
// @Param request body UpdateProfileRequest true "Profile data"
// @Success 200 {object} UserProfileDTO
// @Router /profile [put]
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := core.GetUserID(r.Context())
	if !ok {
		responder.Unauthorized(w, r, "User not found in context")
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	// Convert to service request
	serviceReq := UpdateProfileRequest{
		Nickname: req.Nickname,
		Email:    req.Email,
		Phone:    req.Phone,
		Avatar:   req.Avatar,
	}

	profile, err := h.userService.UpdateProfile(r.Context(), userID, serviceReq)
	if err != nil {
		httplog.Error(h.logger, r, "Failed to update profile", err, zap.String("user_id", userID.String()))
		if errors.Is(err, ErrSuperAdminProtected) {
			responder.Forbidden(w, r, "Super admin user cannot modify sensitive information")
			return
		}
		if errors.Is(err, ErrUserNotFound) {
			responder.NotFound(w, r, "User not found")
			return
		}
		responder.DatabaseError(w, r, "Failed to update profile")
		return
	}

	responder.OK(w, r, profile)
}

// UpdateSettings handles updating current user's settings
// @Summary Update current user settings
// @Tags Profile
// @Accept json
// @Produce json
// @Param request body UpdateSettingsRequest true "Settings data"
// @Success 200 {object} map[string]string
// @Router /profile/settings [put]
func (h *UserHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := core.GetUserID(r.Context())
	if !ok {
		responder.Unauthorized(w, r, "User not found in context")
		return
	}

	var req UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	if err := h.userService.UpdateSettings(r.Context(), userID, req.Settings); err != nil {
		httplog.Error(h.logger, r, "Failed to update settings", err, zap.String("user_id", userID.String()))
		if errors.Is(err, ErrUserNotFound) {
			responder.NotFound(w, r, "User not found")
			return
		}
		responder.DatabaseError(w, r, "Failed to update settings")
		return
	}

	responder.OK(w, r, map[string]string{"message": "Settings updated successfully"})
}

// ListUsers handles listing users with filters
// @Summary List users with filters
// @Tags Users
// @Accept json
// @Produce json
// @Param q query string false "Search query (username, email, phone)"
// @Param status query string false "Status filter (active, suspended, inactive)"
// @Param role_id query string false "Role ID filter"
// @Param include_deleted query boolean false "Include soft-deleted users"
// @Param page query int false "Page number (default:1)"
// @Param pageSize query int false "Page size (default: 20, max: 100)"
// @Success 200 {object} UserListResult
// @Router /users [get]
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	filters := UserListFilters{
		Query:          r.URL.Query().Get("q"),
		Status:         r.URL.Query().Get("status"),
		IncludeDeleted: r.URL.Query().Get("include_deleted") == "true",
	}

	// Parse role_id if provided
	if roleIDStr := r.URL.Query().Get("role_id"); roleIDStr != "" {
		roleID, err := uuid.Parse(roleIDStr)
		if err != nil {
			responder.BadRequest(w, r, "Invalid role_id format")
			return
		}
		filters.RoleID = roleID
	}

	// Parse page
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			responder.BadRequest(w, r, "Invalid page number")
			return
		}
		filters.Page = page
	} else {
		filters.Page = 1
	}

	// Parse pageSize
	if pageSizeStr := r.URL.Query().Get("pageSize"); pageSizeStr != "" {
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 {
			responder.BadRequest(w, r, "Invalid page size")
			return
		}
		filters.PageSize = pageSize
	} else {
		filters.PageSize = 20
	}

	result, err := h.userService.ListUsers(r.Context(), filters)
	if err != nil {
		httplog.Error(h.logger, r, "Failed to list users", err)
		responder.DatabaseError(w, r, "Failed to list users")
		return
	}

	// Return with pagination metadata
	pagination := &responder.PaginationMeta{
		Page:       result.Page,
		PageSize:   result.PageSize,
		Total:      int64(result.Total),
		TotalPages: result.TotalPages,
		HasMore:    result.Page < result.TotalPages,
	}

	responder.WriteList(w, r, http.StatusOK, result.Users, pagination)
}

func (h *UserHandler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	if h.invitationService == nil {
		responder.DatabaseError(w, r, "Invitation service not configured")
		return
	}

	identity, ok := core.GetIdentity(r.Context())
	if !ok || identity.UserID == uuid.Nil {
		responder.Unauthorized(w, r, "Unauthorized")
		return
	}

	var req CreateInvitationAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	resp, err := h.invitationService.CreateInvitation(r.Context(), CreateInvitationRequest{
		Username:   req.Username,
		Email:      req.Email,
		DomainType: req.DomainType,
		DomainKey:  req.DomainKey,
		RoleIDs:    req.RoleIDs,
		CreatedBy:  identity.UserID,
	})
	if err != nil {
		h.mapInvitationError(w, r, "Failed to create invitation", err)
		return
	}

	responder.Created(w, r, resp)
}

func (h *UserHandler) ValidateInvitation(w http.ResponseWriter, r *http.Request) {
	if h.invitationService == nil {
		responder.DatabaseError(w, r, "Invitation service not configured")
		return
	}

	inviteJWT := strings.TrimSpace(r.URL.Query().Get("inviteJwt"))
	if inviteJWT == "" {
		// Compatibility with previous token query name.
		inviteJWT = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if inviteJWT == "" {
		responder.BadRequest(w, r, "inviteJwt is required")
		return
	}

	resp, err := h.invitationService.ValidateInvitation(r.Context(), ValidateInvitationRequest{
		InviteJWT: inviteJWT,
	})
	if err != nil {
		h.mapInvitationError(w, r, "Failed to validate invitation", err)
		return
	}

	responder.OK(w, r, resp)
}

func (h *UserHandler) ActivateInvitation(w http.ResponseWriter, r *http.Request) {
	if h.invitationService == nil {
		responder.DatabaseError(w, r, "Invitation service not configured")
		return
	}

	var req ActivateInvitationAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	resp, err := h.invitationService.ActivateInvitation(r.Context(), ActivateInvitationRequest{
		InviteJWT:       req.InviteJWT,
		Nickname:        req.Nickname,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	})
	if err != nil {
		h.mapInvitationError(w, r, "Failed to activate invitation", err)
		return
	}

	responder.OK(w, r, resp)
}

func (h *UserHandler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	if h.invitationService == nil {
		responder.DatabaseError(w, r, "Invitation service not configured")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	resp, err := h.invitationService.ListInvitations(r.Context(), ListInvitationFilters{
		DomainType: strings.TrimSpace(r.URL.Query().Get("domainType")),
		DomainKey:  strings.TrimSpace(r.URL.Query().Get("domainKey")),
		Status:     strings.TrimSpace(r.URL.Query().Get("status")),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		h.mapInvitationError(w, r, "Failed to list invitations", err)
		return
	}

	responder.OK(w, r, resp)
}

func (h *UserHandler) GetInvitation(w http.ResponseWriter, r *http.Request) {
	if h.invitationService == nil {
		responder.DatabaseError(w, r, "Invitation service not configured")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid invitation ID")
		return
	}

	resp, err := h.invitationService.GetInvitation(r.Context(), id)
	if err != nil {
		h.mapInvitationError(w, r, "Failed to get invitation", err)
		return
	}

	responder.OK(w, r, resp)
}

func (h *UserHandler) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	if h.invitationService == nil {
		responder.DatabaseError(w, r, "Invitation service not configured")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid invitation ID")
		return
	}

	if err := h.invitationService.RevokeInvitation(r.Context(), id); err != nil {
		h.mapInvitationError(w, r, "Failed to revoke invitation", err)
		return
	}

	responder.OK(w, r, map[string]string{"message": "Invitation revoked successfully"})
}

// CreatePasswordReset handles creating reset token for target user.
// @Summary Create password reset token
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 201 {object} CreatePasswordResetResponse
// @Router /users/{id}/password-resets [post]
func (h *UserHandler) CreatePasswordReset(w http.ResponseWriter, r *http.Request) {
	if h.passwordResetService == nil {
		responder.DatabaseError(w, r, "Password reset service not configured")
		return
	}

	identity, ok := core.GetIdentity(r.Context())
	if !ok || identity.UserID == uuid.Nil {
		responder.Unauthorized(w, r, "Unauthorized")
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid user ID")
		return
	}

	resp, err := h.passwordResetService.CreatePasswordReset(r.Context(), CreatePasswordResetRequest{
		UserID:    userID,
		CreatedBy: identity.UserID,
	})
	if err != nil {
		h.mapPasswordResetError(w, r, "Failed to create password reset", err)
		return
	}

	responder.Created(w, r, resp)
}

// ValidatePasswordReset validates a reset JWT.
// @Summary Validate password reset token
// @Tags Users
// @Accept json
// @Produce json
// @Param resetJwt query string true "Reset JWT"
// @Success 200 {object} ValidatePasswordResetResponse
// @Router /users/password-resets/validate [get]
func (h *UserHandler) ValidatePasswordReset(w http.ResponseWriter, r *http.Request) {
	if h.passwordResetService == nil {
		responder.DatabaseError(w, r, "Password reset service not configured")
		return
	}

	resetJWT := strings.TrimSpace(r.URL.Query().Get("resetJwt"))
	if resetJWT == "" {
		resetJWT = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if resetJWT == "" {
		responder.BadRequest(w, r, "resetJwt is required")
		return
	}

	resp, err := h.passwordResetService.ValidatePasswordReset(r.Context(), ValidatePasswordResetRequest{
		ResetJWT: resetJWT,
	})
	if err != nil {
		h.mapPasswordResetError(w, r, "Failed to validate password reset", err)
		return
	}

	responder.OK(w, r, resp)
}

// ActivatePasswordReset consumes reset JWT and updates password.
// @Summary Activate password reset
// @Tags Users
// @Accept json
// @Produce json
// @Param request body ActivatePasswordResetAPIRequest true "Activation payload"
// @Success 200 {object} ActivatePasswordResetResponse
// @Router /users/password-resets/activate [post]
func (h *UserHandler) ActivatePasswordReset(w http.ResponseWriter, r *http.Request) {
	if h.passwordResetService == nil {
		responder.DatabaseError(w, r, "Password reset service not configured")
		return
	}

	var req ActivatePasswordResetAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	resp, err := h.passwordResetService.ActivatePasswordReset(r.Context(), ActivatePasswordResetRequest{
		ResetJWT:        req.ResetJWT,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	})
	if err != nil {
		h.mapPasswordResetError(w, r, "Failed to activate password reset", err)
		return
	}

	responder.OK(w, r, resp)
}

func (h *UserHandler) mapInvitationError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	switch {
	case errors.Is(err, ErrInvitationNotFound):
		responder.NotFound(w, r, "Invitation not found")
	case errors.Is(err, ErrInvitationExpired):
		responder.BadRequest(w, r, "Invitation expired")
	case errors.Is(err, ErrInvitationUsed):
		responder.BadRequest(w, r, "Invitation already used")
	case errors.Is(err, ErrInvitationRevoked):
		responder.BadRequest(w, r, "Invitation revoked")
	case errors.Is(err, ErrInvitationInvalid), errors.Is(err, ErrInvitationBadStatus):
		responder.BadRequest(w, r, "Invalid invitation token")
	case errors.Is(err, crypto.ErrPasswordMismatch):
		responder.BadRequest(w, r, "Passwords do not match")
	case errors.Is(err, crypto.ErrPasswordTooShort), errors.Is(err, crypto.ErrPasswordTooLong):
		responder.BadRequest(w, r, err.Error())
	default:
		httplog.Error(h.logger, r, msg, err)
		responder.DatabaseError(w, r, msg)
	}
}

func (h *UserHandler) mapPasswordResetError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	switch {
	case errors.Is(err, ErrPasswordResetNotFound):
		responder.NotFound(w, r, "Password reset token not found")
	case errors.Is(err, ErrPasswordResetExpired):
		responder.BadRequest(w, r, "Password reset token expired")
	case errors.Is(err, ErrPasswordResetUsed):
		responder.BadRequest(w, r, "Password reset token already used")
	case errors.Is(err, ErrPasswordResetRevoked):
		responder.BadRequest(w, r, "Password reset token revoked")
	case errors.Is(err, ErrPasswordResetInvalid), errors.Is(err, ErrPasswordResetBadStatus):
		responder.BadRequest(w, r, "Invalid password reset token")
	case errors.Is(err, crypto.ErrPasswordMismatch):
		responder.BadRequest(w, r, "Passwords do not match")
	case errors.Is(err, crypto.ErrPasswordTooShort), errors.Is(err, crypto.ErrPasswordTooLong):
		responder.BadRequest(w, r, err.Error())
	default:
		httplog.Error(h.logger, r, msg, err)
		responder.DatabaseError(w, r, msg)
	}
}

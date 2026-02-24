package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/config"
	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/domain"
	"github.com/leeforge/core/server/ent/domainmembership"
	"github.com/leeforge/core/server/ent/user"
	"github.com/leeforge/core/server/pkg/crypto"
	"github.com/leeforge/core/server/pkg/jwt"
	"github.com/leeforge/core/server/services/audit"

	"github.com/leeforge/framework/logging"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserInactive      = errors.New("user is inactive")
	ErrTenantRequired    = errors.New("tenant selection required")
	ErrTenantNotMember   = errors.New("user is not a member of tenant")
)

type LoginResponse struct {
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken,omitempty"` // Web端不返回（使用Cookie）
	ExpiresIn    int      `json:"expiresIn"`              // seconds
	User         *UserDTO `json:"user"`
}

type UserDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Nickname string    `json:"nickname"`
	Email    string    `json:"email"`
	Avatar   string    `json:"avatar,omitempty"`
	Roles    []string  `json:"roles"`
	TenantID string    `json:"tenantId,omitempty"`
}

type AuthService struct {
	client     *ent.Client
	logger     logging.Logger
	jwtService *jwt.JWTService
	audit      *audit.Service
	config     *config.Config
}

func NewAuthService(client *ent.Client, logger logging.Logger, jwtService *jwt.JWTService, cfg *config.Config) *AuthService {
	return &AuthService{
		client:     client,
		logger:     logger,
		jwtService: jwtService,
		audit:      audit.NewService(client, logger),
		config:     cfg,
	}
}

// GetConfig 获取配置
func (s *AuthService) GetConfig() *config.SecurityConfig {
	return &s.config.Security
}

// Register creates a new user
func (s *AuthService) Register(ctx context.Context, username, email, password, nickname string) (*UserDTO, error) {
	tenantID, _ := core.GetTenantID(ctx)
	if tenantID == "" {
		return nil, ErrTenantRequired
	}

	// Resolve domain UUID for owner_domain_id filtering.
	var domainUUID *uuid.UUID
	if did, ok := core.GetDomainID(ctx); ok {
		if parsed, err := uuid.Parse(did); err == nil {
			domainUUID = &parsed
		}
	}

	// Check if user exists within the same domain scope.
	query := s.client.User.Query().
		Where(
			user.Or(
				user.UsernameEQ(username),
				user.EmailEQ(email),
			),
		)
	if domainUUID != nil {
		query = query.Where(user.OwnerDomainIDEQ(*domainUUID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}
	if exist {
		return nil, ErrUserAlreadyExists
	}

	// Hash password
	hashedPassword, err := crypto.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user (owner_domain_id will be set by DomainHook if acting context is present).
	u, err := s.client.User.Create().
		SetUsername(username).
		SetEmail(email).
		SetPasswordHash(hashedPassword).
		SetNickname(nickname).
		SetStatus(user.StatusActive).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Best-effort: ensure tenant membership for the requested tenant.
	_ = s.ensureMembership(ctx, u.ID, tenantID, false)

	return s.toUserDTO(u), s.recordAuthEvent(ctx, u.ID, tenantIDForAudit(tenantID), "auth.register", "success", "")
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, username, password, tenantID string) (*LoginResponse, error) {
	tenantID = strings.TrimSpace(tenantID)
	queryCtx := core.WithoutTenant(ctx)

	// Find user by username with roles
	var u *ent.User
	var err error

	if tenantID != "" {
		// Resolve tenant domain to get the domain UUID for filtering.
		tenantDomain, domErr := s.client.Domain.Query().
			Where(
				domain.TypeCodeEQ("tenant"),
				domain.KeyEQ(tenantID),
				domain.DeletedAtIsNil(),
			).
			Only(queryCtx)

		if domErr != nil {
			if ent.IsNotFound(domErr) {
				return nil, ErrUserNotFound
			}
			return nil, fmt.Errorf("failed to resolve tenant domain: %w", domErr)
		}

		u, err = s.client.User.Query().
			Where(
				user.UsernameEQ(username),
				user.OwnerDomainIDEQ(tenantDomain.ID),
				user.DeletedAtIsNil(),
			).
			WithRoles().
			First(queryCtx)
		if err != nil && ent.IsNotFound(err) {
			// Keep backward compatibility for legacy super admins without owner_domain_id.
			u, err = s.findSuperAdminUser(queryCtx, username)
		}
	} else {
		platformDomain, domErr := s.client.Domain.Query().
			Where(
				domain.TypeCodeEQ("platform"),
				domain.KeyEQ("root"),
				domain.DeletedAtIsNil(),
			).
			Only(queryCtx)
		if domErr != nil {
			if ent.IsNotFound(domErr) {
				return nil, ErrUserNotFound
			}
			return nil, fmt.Errorf("failed to resolve platform domain: %w", domErr)
		}

		u, err = s.client.User.Query().
			Where(
				user.UsernameEQ(username),
				user.OwnerDomainIDEQ(platformDomain.ID),
				user.DeletedAtIsNil(),
			).
			WithRoles().
			First(queryCtx)
		if err != nil && ent.IsNotFound(err) {
			// Keep backward compatibility for legacy super admins without owner_domain_id.
			u, err = s.findSuperAdminUser(queryCtx, username)
		}
	}
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// Verify password
	if !crypto.ComparePassword(u.PasswordHash, password) {
		s.recordAuthEvent(queryCtx, u.ID, tenantIDForAudit(tenantID), "auth.login", "failed", "invalid_password")
		return nil, ErrInvalidPassword
	}
	if u.Status != user.StatusActive {
		s.recordAuthEvent(queryCtx, u.ID, tenantIDForAudit(tenantID), "auth.login", "failed", "user_inactive")
		return nil, ErrUserInactive
	}

	// Extract roles
	var roles []string
	if u.Edges.Roles != nil {
		for _, r := range u.Edges.Roles {
			roles = append(roles, r.Code)
		}
	}

	isSuperAdmin := u.IsSuperAdmin
	selectedTenant := tenantID

	userDTO := s.toUserDTO(u)
	userDTO.TenantID = selectedTenant

	// Generate tokens
	accessToken, err := s.jwtService.GenerateAccessToken(
		u.ID,
		u.Username,
		u.Email,
		selectedTenant,
		roles,
		isSuperAdmin,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(
		u.ID,
		u.Username,
		u.Email,
		selectedTenant,
		roles,
		isSuperAdmin,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    15 * 60, // 15 minutes
		User:         userDTO,
	}, s.recordAuthEvent(queryCtx, u.ID, tenantIDForAudit(selectedTenant), "auth.login", "success", "")
}

// RefreshToken refreshes the access token
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	claims, err := s.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", err
	}
	tenantID := strings.TrimSpace(claims.TenantID)

	if claims.IsSuperAdmin {
		token, err := s.jwtService.GenerateAccessToken(
			claims.UserID,
			claims.Username,
			claims.Email,
			claims.TenantID,
			claims.Roles,
			claims.IsSuperAdmin,
		)
		s.recordAuthEvent(ctx, claims.UserID, tenantIDForAudit(claims.TenantID), "auth.refresh", outcomeFromError(err), reasonFromError(err))
		return token, err
	}

	// Tenant is resolved by domain middlewares/plugins on authenticated requests.
	// Non-super-admin platform users can carry an empty tenant claim.
	if tenantID == "" {
		token, err := s.jwtService.GenerateAccessToken(
			claims.UserID,
			claims.Username,
			claims.Email,
			tenantID,
			claims.Roles,
			claims.IsSuperAdmin,
		)
		s.recordAuthEvent(ctx, claims.UserID, tenantIDForAudit(tenantID), "auth.refresh", outcomeFromError(err), reasonFromError(err))
		return token, err
	}

	// Resolve tenant domain
	tenantDomain, err := s.client.Domain.Query().
		Where(
			domain.TypeCodeEQ("tenant"),
			domain.KeyEQ(tenantID),
			domain.DeletedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to resolve tenant domain: %w", err)
	}

	exists, err := s.client.DomainMembership.Query().
		Where(
			domainmembership.DomainIDEQ(tenantDomain.ID),
			domainmembership.SubjectIDEQ(claims.UserID),
			domainmembership.SubjectTypeEQ(domainmembership.SubjectTypeUser),
			domainmembership.StatusEQ(domainmembership.StatusActive),
			domainmembership.DeletedAtIsNil(),
		).
		Exist(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to verify tenant membership: %w", err)
	}
	if !exists {
		s.recordAuthEvent(ctx, claims.UserID, tenantIDForAudit(tenantID), "auth.refresh", "failed", "tenant_not_member")
		return "", ErrTenantNotMember
	}

	token, err := s.jwtService.GenerateAccessToken(
		claims.UserID,
		claims.Username,
		claims.Email,
		tenantID,
		claims.Roles,
		claims.IsSuperAdmin,
	)
	s.recordAuthEvent(ctx, claims.UserID, tenantIDForAudit(tenantID), "auth.refresh", outcomeFromError(err), reasonFromError(err))
	return token, err
}

func (s *AuthService) findSuperAdminUser(ctx context.Context, username string) (*ent.User, error) {
	return s.client.User.Query().
		Where(
			user.UsernameEQ(username),
			user.IsSuperAdminEQ(true),
			user.DeletedAtIsNil(),
		).
		WithRoles().
		First(ctx)
}

// ChangePassword changes user password
func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return err
	}

	if !crypto.ComparePassword(u.PasswordHash, oldPassword) {
		s.recordAuthEvent(ctx, userID, tenantIDForAudit(ownerDomainIDToString(u.OwnerDomainID)), "auth.password.change", "failed", "invalid_old_password")
		return ErrInvalidPassword
	}

	hashedPassword, err := crypto.HashPassword(newPassword)
	if err != nil {
		s.recordAuthEvent(ctx, userID, tenantIDForAudit(ownerDomainIDToString(u.OwnerDomainID)), "auth.password.change", "failed", "hash_password_failed")
		return err
	}

	_, err = s.client.User.UpdateOne(u).
		SetPasswordHash(hashedPassword).
		Save(ctx)

	s.recordAuthEvent(ctx, userID, tenantIDForAudit(ownerDomainIDToString(u.OwnerDomainID)), "auth.password.change", outcomeFromError(err), reasonFromError(err))
	return err
}

// Logout invalidates the access token
func (s *AuthService) Logout(ctx context.Context, token string) error {
	claims, _ := s.jwtService.ValidateAccessToken(token)
	err := s.jwtService.InvalidateToken(token)
	if claims != nil {
		s.recordAuthEvent(ctx, claims.UserID, tenantIDForAudit(claims.TenantID), "auth.logout", outcomeFromError(err), reasonFromError(err))
	}
	return err
}

func (s *AuthService) toUserDTO(u *ent.User) *UserDTO {
	var roles []string
	if u.Edges.Roles != nil {
		for _, r := range u.Edges.Roles {
			roles = append(roles, r.Code)
		}
	}

	return &UserDTO{
		ID:       u.ID,
		Username: u.Username,
		Nickname: u.Nickname,
		Email:    u.Email,
		Avatar:   u.Avatar,
		Roles:    roles,
		TenantID: ownerDomainIDToString(u.OwnerDomainID),
	}
}

func (s *AuthService) ensureMembership(ctx context.Context, userID uuid.UUID, tenantID string, forceDefault bool) error {
	if tenantID == "" {
		return nil
	}

	// Resolve tenant domain
	tenantDomain, err := s.client.Domain.Query().
		Where(
			domain.TypeCodeEQ("tenant"),
			domain.KeyEQ(tenantID),
			domain.DeletedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// Domain doesn't exist yet; skip membership creation
			return nil
		}
		return fmt.Errorf("failed to resolve tenant domain: %w", err)
	}

	// Check if DomainMembership already exists for this domain + user
	existing, err := s.client.DomainMembership.Query().
		Where(
			domainmembership.DomainIDEQ(tenantDomain.ID),
			domainmembership.SubjectIDEQ(userID),
			domainmembership.SubjectTypeEQ(domainmembership.SubjectTypeUser),
		).
		First(ctx)
	if err == nil {
		// Membership exists; reactivate if needed
		if existing.DeletedAt.IsZero() && existing.Status == domainmembership.StatusActive {
			if forceDefault && !existing.IsDefault {
				if err := s.resetDefaultDomain(ctx, userID, existing.ID); err != nil {
					return err
				}
			}
			return nil
		}
		updated := s.client.DomainMembership.UpdateOneID(existing.ID).
			ClearDeletedAt().
			SetStatus(domainmembership.StatusActive)
		if forceDefault {
			updated.SetIsDefault(true)
		}
		saved, err := updated.Save(ctx)
		if err != nil {
			return err
		}
		if forceDefault {
			return s.resetDefaultDomain(ctx, userID, saved.ID)
		}
		return nil
	}
	if !ent.IsNotFound(err) {
		return err
	}

	// No existing membership; create one
	isDefault := forceDefault
	if !isDefault {
		hasDefault, err := s.client.DomainMembership.Query().
			Where(
				domainmembership.SubjectIDEQ(userID),
				domainmembership.SubjectTypeEQ(domainmembership.SubjectTypeUser),
				domainmembership.IsDefaultEQ(true),
				domainmembership.DeletedAtIsNil(),
			).
			Exist(ctx)
		if err != nil {
			return err
		}
		isDefault = !hasDefault
	}

	created, err := s.client.DomainMembership.Create().
		SetDomainID(tenantDomain.ID).
		SetSubjectID(userID).
		SetSubjectType(domainmembership.SubjectTypeUser).
		SetStatus(domainmembership.StatusActive).
		SetIsDefault(isDefault).
		Save(ctx)
	if err != nil {
		return err
	}

	if forceDefault || isDefault {
		return s.resetDefaultDomain(ctx, userID, created.ID)
	}

	return nil
}

func (s *AuthService) resetDefaultDomain(ctx context.Context, userID uuid.UUID, defaultID uuid.UUID) error {
	_, err := s.client.DomainMembership.Update().
		Where(
			domainmembership.SubjectIDEQ(userID),
			domainmembership.SubjectTypeEQ(domainmembership.SubjectTypeUser),
			domainmembership.IDNEQ(defaultID),
		).
		SetIsDefault(false).
		Save(ctx)
	return err
}

func (s *AuthService) selectTenant(ctx context.Context, userID uuid.UUID, requestedTenant string, fallbackTenant string) (string, error) {
	if requestedTenant != "" {
		// Resolve tenant domain and check membership
		tenantDomain, err := s.client.Domain.Query().
			Where(
				domain.TypeCodeEQ("tenant"),
				domain.KeyEQ(requestedTenant),
				domain.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return "", ErrTenantNotMember
			}
			return "", fmt.Errorf("failed to resolve tenant domain: %w", err)
		}

		exists, err := s.client.DomainMembership.Query().
			Where(
				domainmembership.DomainIDEQ(tenantDomain.ID),
				domainmembership.SubjectIDEQ(userID),
				domainmembership.SubjectTypeEQ(domainmembership.SubjectTypeUser),
				domainmembership.StatusEQ(domainmembership.StatusActive),
				domainmembership.DeletedAtIsNil(),
			).
			Exist(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to verify tenant membership: %w", err)
		}
		if !exists {
			return "", ErrTenantNotMember
		}
		return requestedTenant, nil
	}

	if s.config != nil && !s.config.AccessControl.MultiTenancy.Enabled {
		if defaultTenantID := strings.TrimSpace(s.config.AccessControl.MultiTenancy.DefaultTenantID); defaultTenantID != "" {
			return defaultTenantID, nil
		}
		if fallback := strings.TrimSpace(fallbackTenant); fallback != "" {
			return fallback, nil
		}

		// Find the user's default domain membership
		defaultMembership, err := s.client.DomainMembership.Query().
			Where(
				domainmembership.SubjectIDEQ(userID),
				domainmembership.SubjectTypeEQ(domainmembership.SubjectTypeUser),
				domainmembership.IsDefaultEQ(true),
				domainmembership.StatusEQ(domainmembership.StatusActive),
				domainmembership.DeletedAtIsNil(),
			).
			WithDomain().
			First(ctx)
		if err == nil && defaultMembership != nil {
			if d := defaultMembership.Edges.Domain; d != nil && d.TypeCode == "tenant" {
				return d.Key, nil
			}
		}
		if err != nil && !ent.IsNotFound(err) {
			return "", fmt.Errorf("failed to resolve default tenant membership: %w", err)
		}
	}

	return "", ErrTenantRequired
}

func (s *AuthService) recordAuthEvent(
	ctx context.Context,
	userID uuid.UUID,
	tenantID string,
	action string,
	outcome string,
	reason string,
) error {
	if s.audit == nil {
		return nil
	}
	payload := map[string]any{
		"outcome": outcome,
	}
	if reason != "" {
		payload["reason"] = reason
	}
	if err := s.audit.RecordWithActor(ctx, userID, tenantID, audit.Event{
		Action:   action,
		Resource: "auth",
		After:    payload,
	}); err != nil {
		s.logger.Warn("Failed to record auth audit event")
	}
	return nil
}

func outcomeFromError(err error) string {
	if err != nil {
		return "failed"
	}
	return "success"
}

func reasonFromError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func tenantIDForAudit(tenantID string) string {
	if strings.TrimSpace(tenantID) == "" {
		return "platform"
	}
	return tenantID
}

// ownerDomainIDToString converts a *uuid.UUID to a string (empty string if nil).
func ownerDomainIDToString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

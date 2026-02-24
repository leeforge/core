package user

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/domainmembership"
	"github.com/leeforge/core/server/ent/invitationtoken"
	"github.com/leeforge/core/server/ent/user"
	"github.com/leeforge/core/server/pkg/crypto"
	"github.com/leeforge/core/server/pkg/jwt"
	"github.com/leeforge/core/server/services/rbacsync"

	"github.com/leeforge/framework/auth/rbac"
	"github.com/leeforge/framework/logging"
)

const (
	invitationStatusPending = "pending"
	invitationStatusUsed    = "used"
	invitationStatusRevoked = "revoked"
	invitationStatusExpired = "expired"
)

var (
	ErrInvitationNotFound  = errors.New("invitation not found")
	ErrInvitationInvalid   = errors.New("invitation token is invalid")
	ErrInvitationExpired   = errors.New("invitation has expired")
	ErrInvitationUsed      = errors.New("invitation already used")
	ErrInvitationRevoked   = errors.New("invitation has been revoked")
	ErrInvitationBadStatus = errors.New("invitation status is invalid")
)

// InvitationService handles invite-jwt based user onboarding.
type InvitationService struct {
	client         *ent.Client
	logger         logging.Logger
	jwt            *jwt.JWTService
	domainResolver core.DomainResolver
	providers      *core.InvitationProviderRegistry
	rbacManager    *rbac.RBACManager
}

func NewInvitationService(
	client *ent.Client,
	logger logging.Logger,
	jwtService *jwt.JWTService,
	domainResolver core.DomainResolver,
	providers *core.InvitationProviderRegistry,
) *InvitationService {
	return &InvitationService{
		client:         client,
		logger:         logger,
		jwt:            jwtService,
		domainResolver: domainResolver,
		providers:      providers,
	}
}

func (s *InvitationService) WithRBACManager(rbacManager *rbac.RBACManager) *InvitationService {
	s.rbacManager = rbacManager
	return s
}

type CreateInvitationRequest struct {
	Username   string
	Email      string
	DomainType string
	DomainKey  string
	RoleIDs    []string
	CreatedBy  uuid.UUID
}

type CreateInvitationResponse struct {
	ID        uuid.UUID `json:"id"`
	InviteJWT string    `json:"inviteJwt"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type ActivateInvitationRequest struct {
	InviteJWT       string
	Nickname        string
	Password        string
	ConfirmPassword string
}

type ActivateInvitationResponse struct {
	User UserBasicDTO `json:"user"`
}

type ValidateInvitationRequest struct {
	InviteJWT string
}

type ValidateInvitationResponse struct {
	Valid      bool      `json:"valid"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	DomainType string    `json:"domainType"`
	DomainKey  string    `json:"domainKey"`
	RoleIDs    []string  `json:"roleIds"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type ListInvitationFilters struct {
	DomainType string
	DomainKey  string
	Status     string
	Page       int
	PageSize   int
}

type InvitationItem struct {
	ID         uuid.UUID  `json:"id"`
	Username   string     `json:"username,omitempty"`
	Email      string     `json:"email"`
	DomainType string     `json:"domainType"`
	DomainKey  string     `json:"domainKey"`
	RoleIDs    []string   `json:"roleIds"`
	Status     string     `json:"status"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	UsedAt     *time.Time `json:"usedAt,omitempty"`
}

type ListInvitationResult struct {
	Items      []InvitationItem `json:"items"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	TotalPages int              `json:"totalPages"`
}

func (s *InvitationService) CreateInvitation(
	ctx context.Context,
	req CreateInvitationRequest,
) (*CreateInvitationResponse, error) {
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	if username == "" ||
		email == "" ||
		strings.TrimSpace(req.DomainType) == "" ||
		strings.TrimSpace(req.DomainKey) == "" {
		return nil, ErrInvitationInvalid
	}

	resolved, err := s.domainResolver.ResolveDomain(ctx, req.DomainType, req.DomainKey)
	if err != nil {
		return nil, fmt.Errorf("resolve invitation domain: %w", err)
	}

	if provider, ok := s.resolveProvider(req.DomainType); ok {
		if err := provider.ValidateCreate(ctx, core.InvitationCreateRequest{
			Username:   username,
			Email:      email,
			DomainType: req.DomainType,
			DomainKey:  req.DomainKey,
			RoleIDs:    req.RoleIDs,
			CreatedBy:  req.CreatedBy,
		}); err != nil {
			return nil, err
		}
	}

	roleIDs, err := parseUUIDs(req.RoleIDs)
	if err != nil {
		return nil, ErrInvitationInvalid
	}

	jti := uuid.NewString()
	inviteJWT, err := s.jwt.GenerateInviteToken(jwt.InviteClaimsInput{
		JTI:        jti,
		Email:      email,
		DomainType: req.DomainType,
		DomainKey:  req.DomainKey,
		RoleIDs:    req.RoleIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("generate invite token: %w", err)
	}

	claims, err := s.jwt.ValidateInviteToken(inviteJWT)
	if err != nil {
		return nil, fmt.Errorf("validate generated invite token: %w", err)
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("start transaction: %w", err)
	}

	inactiveUser, err := tx.User.Create().
		SetUsername(username).
		SetEmail(email).
		SetStatus(user.StatusInactive).
		SetOwnerDomainID(resolved.DomainID).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("create inactive invited user: %w", err)
	}

	if len(roleIDs) > 0 {
		if err := tx.User.UpdateOneID(inactiveUser.ID).AddRoleIDs(roleIDs...).Exec(ctx); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("assign invited user roles: %w", err)
		}
	}

	builder := tx.InvitationToken.Create().
		SetTenantID(resolved.DomainID).
		SetJti(jti).
		SetToken(inviteJWT).
		SetTokenHash(sha256Hex(inviteJWT)).
		SetEmail(email).
		SetDomainType(req.DomainType).
		SetDomainKey(req.DomainKey).
		SetRoleIds(req.RoleIDs).
		SetStatus(invitationStatusPending).
		SetExpiresAt(claims.ExpiresAt.Time).
		SetUserID(inactiveUser.ID)
	if req.CreatedBy != uuid.Nil {
		builder = builder.SetCreatedByID(req.CreatedBy)
	}

	row, err := builder.Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("persist invitation token: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit invitation creation: %w", err)
	}
	s.syncRBACPolicies(ctx, "create invitation")

	return &CreateInvitationResponse{
		ID:        row.ID,
		InviteJWT: inviteJWT,
		Username:  inactiveUser.Username,
		Email:     row.Email,
		ExpiresAt: row.ExpiresAt,
	}, nil
}

func (s *InvitationService) ActivateInvitation(
	ctx context.Context,
	req ActivateInvitationRequest,
) (*ActivateInvitationResponse, error) {
	if strings.TrimSpace(req.InviteJWT) == "" {
		return nil, ErrInvitationInvalid
	}
	if req.Password != req.ConfirmPassword {
		return nil, crypto.ErrPasswordMismatch
	}
	if err := crypto.ValidatePasswordStrength(req.Password); err != nil {
		return nil, err
	}

	claims, err := s.jwt.ValidateInviteToken(req.InviteJWT)
	if err != nil {
		if errors.Is(err, jwt.ErrExpiredToken) {
			return nil, ErrInvitationExpired
		}
		return nil, ErrInvitationInvalid
	}

	tokenHash := sha256Hex(req.InviteJWT)
	inv, err := s.client.InvitationToken.Query().
		Where(invitationtoken.JtiEQ(claims.ID)).
		WithUser().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrInvitationNotFound
		}
		return nil, fmt.Errorf("query invitation token: %w", err)
	}

	if inv.TokenHash != tokenHash {
		return nil, ErrInvitationInvalid
	}
	if inv.Status != invitationStatusPending {
		switch inv.Status {
		case invitationStatusUsed:
			return nil, ErrInvitationUsed
		case invitationStatusRevoked:
			return nil, ErrInvitationRevoked
		case invitationStatusExpired:
			return nil, ErrInvitationExpired
		default:
			return nil, ErrInvitationBadStatus
		}
	}
	if time.Now().After(inv.ExpiresAt) {
		if _, updateErr := s.client.InvitationToken.UpdateOneID(inv.ID).
			SetStatus(invitationStatusExpired).
			Save(ctx); updateErr != nil {
			s.logger.Warn("mark invitation expired failed", zap.Error(updateErr), zap.String("invitation_id", inv.ID.String()))
		}
		return nil, ErrInvitationExpired
	}

	resolved, err := s.domainResolver.ResolveDomain(ctx, claims.DomainType, claims.DomainKey)
	if err != nil {
		return nil, fmt.Errorf("resolve invite domain: %w", err)
	}
	if inv.Edges.User == nil {
		return nil, ErrInvitationInvalid
	}

	hashedPassword, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("start transaction: %w", err)
	}

	updateBuilder := tx.User.UpdateOneID(inv.Edges.User.ID).
		SetPasswordHash(hashedPassword).
		SetStatus(user.StatusActive).
		SetOwnerDomainID(resolved.DomainID)
	if strings.TrimSpace(req.Nickname) != "" {
		updateBuilder = updateBuilder.SetNickname(strings.TrimSpace(req.Nickname))
	}
	activatedUser, err := updateBuilder.Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("activate invited user: %w", err)
	}

	exists, err := tx.DomainMembership.Query().
		Where(
			domainmembership.DomainIDEQ(resolved.DomainID),
			domainmembership.SubjectTypeEQ(domainmembership.SubjectTypeUser),
			domainmembership.SubjectIDEQ(activatedUser.ID),
			domainmembership.StatusEQ(domainmembership.StatusActive),
		).
		Exist(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("check membership: %w", err)
	}
	if !exists {
		hasAnyActiveMembership, err := tx.DomainMembership.Query().
			Where(
				domainmembership.SubjectTypeEQ(domainmembership.SubjectTypeUser),
				domainmembership.SubjectIDEQ(activatedUser.ID),
				domainmembership.StatusEQ(domainmembership.StatusActive),
			).
			Exist(ctx)
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("check existing memberships: %w", err)
		}

		if _, err := tx.DomainMembership.Create().
			SetDomainID(resolved.DomainID).
			SetSubjectType(domainmembership.SubjectTypeUser).
			SetSubjectID(activatedUser.ID).
			SetMemberRole("member").
			SetStatus(domainmembership.StatusActive).
			SetIsDefault(!hasAnyActiveMembership).
			Save(ctx); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("create membership: %w", err)
		}
	}

	now := time.Now()
	if _, err := tx.InvitationToken.UpdateOneID(inv.ID).
		SetStatus(invitationStatusUsed).
		SetIsUsed(true).
		SetUsedAt(now).
		SetUserID(activatedUser.ID).
		Save(ctx); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("mark invitation used: %w", err)
	}

	if provider, ok := s.resolveProvider(claims.DomainType); ok {
		if err := provider.OnActivated(ctx, core.InvitationActivatedRequest{
			InvitationID: inv.ID,
			ActivatedBy:  activatedUser.ID,
			DomainType:   claims.DomainType,
			DomainKey:    claims.DomainKey,
			RoleIDs:      claims.RoleIDs,
		}); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit invitation activation: %w", err)
	}
	s.syncRBACPolicies(ctx, "activate invitation")

	return &ActivateInvitationResponse{
		User: UserBasicDTO{
			ID:       activatedUser.ID,
			Username: activatedUser.Username,
			Email:    activatedUser.Email,
			Status:   string(activatedUser.Status),
		},
	}, nil
}

func (s *InvitationService) ValidateInvitation(
	ctx context.Context,
	req ValidateInvitationRequest,
) (*ValidateInvitationResponse, error) {
	if strings.TrimSpace(req.InviteJWT) == "" {
		return nil, ErrInvitationInvalid
	}

	claims, err := s.jwt.ValidateInviteToken(req.InviteJWT)
	if err != nil {
		if errors.Is(err, jwt.ErrExpiredToken) {
			return nil, ErrInvitationExpired
		}
		return nil, ErrInvitationInvalid
	}

	inv, err := s.client.InvitationToken.Query().
		Where(invitationtoken.JtiEQ(claims.ID)).
		WithUser().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrInvitationNotFound
		}
		return nil, fmt.Errorf("query invitation by jti: %w", err)
	}

	if inv.TokenHash != sha256Hex(req.InviteJWT) {
		return nil, ErrInvitationInvalid
	}
	if inv.Status != invitationStatusPending {
		switch inv.Status {
		case invitationStatusUsed:
			return nil, ErrInvitationUsed
		case invitationStatusRevoked:
			return nil, ErrInvitationRevoked
		case invitationStatusExpired:
			return nil, ErrInvitationExpired
		default:
			return nil, ErrInvitationBadStatus
		}
	}
	if time.Now().After(inv.ExpiresAt) {
		return nil, ErrInvitationExpired
	}

	return &ValidateInvitationResponse{
		Valid:      true,
		Username:   usernameFromInvitation(inv),
		Email:      claims.Email,
		DomainType: claims.DomainType,
		DomainKey:  claims.DomainKey,
		RoleIDs:    claims.RoleIDs,
		ExpiresAt:  inv.ExpiresAt,
	}, nil
}

func (s *InvitationService) ListInvitations(
	ctx context.Context,
	filters ListInvitationFilters,
) (*ListInvitationResult, error) {
	query := s.client.InvitationToken.Query().WithUser()
	if filters.DomainType != "" {
		query = query.Where(invitationtoken.DomainTypeEQ(filters.DomainType))
	}
	if filters.DomainKey != "" {
		query = query.Where(invitationtoken.DomainKeyEQ(filters.DomainKey))
	}
	if filters.Status != "" {
		query = query.Where(invitationtoken.StatusEQ(filters.Status))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count invitations: %w", err)
	}

	page := filters.Page
	if page < 1 {
		page = 1
	}
	pageSize := filters.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	rows, err := query.
		Order(ent.Desc(invitationtoken.FieldCreatedAt)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}

	items := make([]InvitationItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, InvitationItem{
			ID:         row.ID,
			Username:   usernameFromInvitation(row),
			Email:      row.Email,
			DomainType: row.DomainType,
			DomainKey:  row.DomainKey,
			RoleIDs:    row.RoleIds,
			Status:     row.Status,
			ExpiresAt:  row.ExpiresAt,
			UsedAt:     row.UsedAt,
		})
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	return &ListInvitationResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *InvitationService) GetInvitation(ctx context.Context, id uuid.UUID) (*InvitationItem, error) {
	row, err := s.client.InvitationToken.Query().
		Where(invitationtoken.IDEQ(id)).
		WithUser().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrInvitationNotFound
		}
		return nil, fmt.Errorf("get invitation: %w", err)
	}

	return &InvitationItem{
		ID:         row.ID,
		Username:   usernameFromInvitation(row),
		Email:      row.Email,
		DomainType: row.DomainType,
		DomainKey:  row.DomainKey,
		RoleIDs:    row.RoleIds,
		Status:     row.Status,
		ExpiresAt:  row.ExpiresAt,
		UsedAt:     row.UsedAt,
	}, nil
}

func (s *InvitationService) RevokeInvitation(ctx context.Context, id uuid.UUID) error {
	row, err := s.client.InvitationToken.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrInvitationNotFound
		}
		return fmt.Errorf("get invitation for revoke: %w", err)
	}

	if row.Status == invitationStatusUsed {
		return ErrInvitationUsed
	}
	if row.Status == invitationStatusRevoked {
		return nil
	}

	if _, err := s.client.InvitationToken.UpdateOneID(id).
		SetStatus(invitationStatusRevoked).
		Save(ctx); err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	return nil
}

func (s *InvitationService) resolveProvider(typeCode string) (core.InvitationDomainProvider, bool) {
	if s.providers == nil {
		return nil, false
	}
	return s.providers.Resolve(typeCode)
}

func (s *InvitationService) syncRBACPolicies(ctx context.Context, reason string) {
	if s.client == nil || s.rbacManager == nil || s.rbacManager.Enforcer() == nil {
		return
	}
	if err := rbacsync.FullResync(core.WithoutTenant(ctx), s.client, s.rbacManager, s.logger); err != nil {
		s.logger.Warn("Failed to resync RBAC policies after invitation mutation",
			zap.String("reason", reason),
			zap.Error(err),
		)
	}
}

func parseUUIDs(raw []string) ([]uuid.UUID, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]uuid.UUID, 0, len(raw))
	for _, one := range raw {
		id, err := uuid.Parse(one)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func sha256Hex(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func usernameFromInvitation(inv *ent.InvitationToken) string {
	if inv == nil || inv.Edges.User == nil {
		return ""
	}
	return inv.Edges.User.Username
}

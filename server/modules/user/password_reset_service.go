package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/passwordresettoken"
	"github.com/leeforge/core/server/pkg/crypto"
	"github.com/leeforge/core/server/pkg/jwt"

	"github.com/leeforge/framework/logging"
)

const (
	passwordResetStatusPending = "pending"
	passwordResetStatusUsed    = "used"
	passwordResetStatusRevoked = "revoked"
	passwordResetStatusExpired = "expired"
)

var (
	ErrPasswordResetNotFound  = errors.New("password reset token not found")
	ErrPasswordResetInvalid   = errors.New("password reset token is invalid")
	ErrPasswordResetExpired   = errors.New("password reset token has expired")
	ErrPasswordResetUsed      = errors.New("password reset token already used")
	ErrPasswordResetRevoked   = errors.New("password reset token has been revoked")
	ErrPasswordResetBadStatus = errors.New("password reset token status is invalid")
)

// PasswordResetService handles invite-style reset-jwt workflow.
type PasswordResetService struct {
	client *ent.Client
	logger logging.Logger
	jwt    *jwt.JWTService
}

func NewPasswordResetService(
	client *ent.Client,
	logger logging.Logger,
	jwtService *jwt.JWTService,
) *PasswordResetService {
	return &PasswordResetService{
		client: client,
		logger: logger,
		jwt:    jwtService,
	}
}

type CreatePasswordResetRequest struct {
	UserID    uuid.UUID
	CreatedBy uuid.UUID
}

type CreatePasswordResetResponse struct {
	ID        uuid.UUID `json:"id"`
	ResetJWT  string    `json:"resetJwt"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type ValidatePasswordResetRequest struct {
	ResetJWT string
}

type ValidatePasswordResetResponse struct {
	Valid     bool      `json:"valid"`
	UserID    uuid.UUID `json:"userId"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type ActivatePasswordResetRequest struct {
	ResetJWT        string
	Password        string
	ConfirmPassword string
}

type ActivatePasswordResetResponse struct {
	User UserBasicDTO `json:"user"`
}

func (s *PasswordResetService) CreatePasswordReset(
	ctx context.Context,
	req CreatePasswordResetRequest,
) (*CreatePasswordResetResponse, error) {
	if req.UserID == uuid.Nil {
		return nil, ErrPasswordResetInvalid
	}

	targetUser, err := s.client.User.Get(ctx, req.UserID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrPasswordResetNotFound
		}
		return nil, fmt.Errorf("query target user: %w", err)
	}

	jti := uuid.NewString()
	resetJWT, err := s.jwt.GeneratePasswordResetToken(jwt.PasswordResetClaimsInput{
		JTI:    jti,
		UserID: targetUser.ID,
		Email:  targetUser.Email,
	})
	if err != nil {
		return nil, fmt.Errorf("generate password reset token: %w", err)
	}

	claims, err := s.jwt.ValidatePasswordResetToken(resetJWT)
	if err != nil {
		return nil, fmt.Errorf("validate generated password reset token: %w", err)
	}

	builder := s.client.PasswordResetToken.Create().
		SetJti(jti).
		SetToken(resetJWT).
		SetTokenHash(sha256Hex(resetJWT)).
		SetEmail(targetUser.Email).
		SetExpiresAt(claims.ExpiresAt.Time).
		SetStatus(passwordResetStatusPending).
		SetUserID(targetUser.ID)
	if req.CreatedBy != uuid.Nil {
		builder = builder.SetCreatedByID(req.CreatedBy)
	}

	row, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("persist password reset token: %w", err)
	}

	return &CreatePasswordResetResponse{
		ID:        row.ID,
		ResetJWT:  resetJWT,
		Email:     row.Email,
		ExpiresAt: row.ExpiresAt,
	}, nil
}

func (s *PasswordResetService) ValidatePasswordReset(
	ctx context.Context,
	req ValidatePasswordResetRequest,
) (*ValidatePasswordResetResponse, error) {
	if strings.TrimSpace(req.ResetJWT) == "" {
		return nil, ErrPasswordResetInvalid
	}

	claims, err := s.jwt.ValidatePasswordResetToken(req.ResetJWT)
	if err != nil {
		if errors.Is(err, jwt.ErrExpiredToken) {
			return nil, ErrPasswordResetExpired
		}
		return nil, ErrPasswordResetInvalid
	}

	row, err := s.client.PasswordResetToken.Query().
		Where(passwordresettoken.JtiEQ(claims.ID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrPasswordResetNotFound
		}
		return nil, fmt.Errorf("query password reset token: %w", err)
	}

	if row.TokenHash != sha256Hex(req.ResetJWT) {
		return nil, ErrPasswordResetInvalid
	}
	if row.Status != passwordResetStatusPending {
		return nil, passwordResetStatusError(row.Status)
	}
	if time.Now().After(row.ExpiresAt) {
		return nil, ErrPasswordResetExpired
	}

	return &ValidatePasswordResetResponse{
		Valid:     true,
		UserID:    claims.UserID,
		Email:     claims.Email,
		ExpiresAt: row.ExpiresAt,
	}, nil
}

func (s *PasswordResetService) ActivatePasswordReset(
	ctx context.Context,
	req ActivatePasswordResetRequest,
) (*ActivatePasswordResetResponse, error) {
	if strings.TrimSpace(req.ResetJWT) == "" {
		return nil, ErrPasswordResetInvalid
	}
	if req.Password != req.ConfirmPassword {
		return nil, crypto.ErrPasswordMismatch
	}
	if err := crypto.ValidatePasswordStrength(req.Password); err != nil {
		return nil, err
	}

	claims, err := s.jwt.ValidatePasswordResetToken(req.ResetJWT)
	if err != nil {
		if errors.Is(err, jwt.ErrExpiredToken) {
			return nil, ErrPasswordResetExpired
		}
		return nil, ErrPasswordResetInvalid
	}

	row, err := s.client.PasswordResetToken.Query().
		Where(passwordresettoken.JtiEQ(claims.ID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrPasswordResetNotFound
		}
		return nil, fmt.Errorf("query password reset token for activation: %w", err)
	}

	if row.TokenHash != sha256Hex(req.ResetJWT) {
		return nil, ErrPasswordResetInvalid
	}
	if row.Status != passwordResetStatusPending {
		return nil, passwordResetStatusError(row.Status)
	}
	if time.Now().After(row.ExpiresAt) {
		if _, updateErr := s.client.PasswordResetToken.UpdateOneID(row.ID).
			SetStatus(passwordResetStatusExpired).
			Save(ctx); updateErr != nil {
			s.logger.Warn("mark password reset expired failed",
				zap.Error(updateErr),
				zap.String("password_reset_id", row.ID.String()),
			)
		}
		return nil, ErrPasswordResetExpired
	}

	hashedPassword, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("start transaction: %w", err)
	}

	activatedUser, err := tx.User.UpdateOneID(claims.UserID).
		SetPasswordHash(hashedPassword).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		if ent.IsNotFound(err) {
			return nil, ErrPasswordResetNotFound
		}
		return nil, fmt.Errorf("update user password: %w", err)
	}

	updated, err := tx.PasswordResetToken.Update().
		Where(
			passwordresettoken.IDEQ(row.ID),
			passwordresettoken.StatusEQ(passwordResetStatusPending),
		).
		SetStatus(passwordResetStatusUsed).
		SetIsUsed(true).
		SetUsedAt(time.Now()).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("mark password reset token used: %w", err)
	}
	if updated == 0 {
		_ = tx.Rollback()
		return nil, ErrPasswordResetUsed
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit password reset activation: %w", err)
	}

	return &ActivatePasswordResetResponse{
		User: UserBasicDTO{
			ID:       activatedUser.ID,
			Username: activatedUser.Username,
			Email:    activatedUser.Email,
			Status:   string(activatedUser.Status),
		},
	}, nil
}

func passwordResetStatusError(status string) error {
	switch status {
	case passwordResetStatusUsed:
		return ErrPasswordResetUsed
	case passwordResetStatusExpired:
		return ErrPasswordResetExpired
	case passwordResetStatusRevoked:
		return ErrPasswordResetRevoked
	default:
		return ErrPasswordResetBadStatus
	}
}

package apikey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/core/server/ent"
	entapikey "github.com/leeforge/core/server/ent/apikey"
	"github.com/leeforge/core/server/ent/role"
	"github.com/leeforge/core/server/ent/user"
	"github.com/leeforge/core/server/pkg/apikey"

	"github.com/leeforge/framework/logging"
)

var (
	// ErrAPIKeyNotFound indicates the API key was not found
	ErrAPIKeyNotFound = errors.New("api key not found")
	// ErrAPIKeyInactive indicates the API key is not active
	ErrAPIKeyInactive = errors.New("api key is inactive")
	// ErrAPIKeyExpired indicates the API key has expired
	ErrAPIKeyExpired = errors.New("api key has expired")
	// ErrInvalidAPIKey indicates the API key is invalid
	ErrInvalidAPIKey = errors.New("invalid api key")
)

// APIKeyService handles API key business logic
type APIKeyService struct {
	client *ent.Client
	logger logging.Logger
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(client *ent.Client, logger logging.Logger) *APIKeyService {
	return &APIKeyService{
		client: client,
		logger: logger,
	}
}

// Create creates a new API key
func (s *APIKeyService) Create(
	ctx context.Context,
	userID uuid.UUID,
	name string,
	roleCodes []string, // User's roles from JWT
	description string,
	expiresAt *time.Time,
	rateLimit int,
	dailyLimit int,
	environment string,
	projectID *uuid.UUID,
) (*CreateAPIKeyResponse, error) {
	// Generate API key
	key, err := apikey.GenerateAPIKey(environment)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Hash the key for storage
	secretHash, err := apikey.HashSecret(key)
	if err != nil {
		return nil, fmt.Errorf("failed to hash API key: %w", err)
	}

	// Set defaults
	if rateLimit <= 0 {
		rateLimit = 60
	}
	if dailyLimit <= 0 {
		dailyLimit = 10000
	}

	// Query the first matching role from user's roles
	var roleID *uuid.UUID
	if len(roleCodes) > 0 {
		// Try to find the first role that matches any of the user's role codes
		firstRole, err := s.client.Role.Query().
			Where(role.CodeIn(roleCodes...)).
			Order(ent.Asc(role.FieldSort)). // Use the highest priority role (lowest sort value)
			First(ctx)
		if err == nil {
			roleID = &firstRole.ID
			s.logger.Info("Assigning role to API key",
				zap.String("role_id", firstRole.ID.String()),
				zap.String("role_code", firstRole.Code))
		} else if !ent.IsNotFound(err) {
			return nil, fmt.Errorf("failed to query role: %w", err)
		}
		// If role not found, continue without role (roleID remains nil)
	}

	// Create API key in database
	builder := s.client.ApiKey.Create().
		SetName(name).
		SetDescription(description).
		SetKey(key). // Store the full key for lookup
		SetSecretHash(secretHash).
		SetRateLimit(rateLimit).
		SetDailyLimit(dailyLimit).
		SetOwnerID(userID)

	if projectID != nil {
		builder.SetProjectID(*projectID)
	}

	if expiresAt != nil {
		builder = builder.SetExpiresAt(*expiresAt)
	}

	if roleID != nil {
		builder = builder.SetRoleID(*roleID)
	}

	ak, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return &CreateAPIKeyResponse{
		APIKey:    s.toDTO(ak),
		SecretKey: key, // Return raw key only once
	}, nil
}

// List returns all API keys for a user (paginated)
func (s *APIKeyService) List(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*APIKeyDTO, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Count total
	total, err := s.client.ApiKey.Query().
		Where(entapikey.HasOwnerWith(user.ID(userID))).
		Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count API keys: %w", err)
	}

	// Query with pagination
	offset := (page - 1) * pageSize
	keys, err := s.client.ApiKey.Query().
		Where(entapikey.HasOwnerWith(user.ID(userID))).
		Order(ent.Desc(entapikey.FieldCreatedAt)).
		Limit(pageSize).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list API keys: %w", err)
	}

	dtos := make([]*APIKeyDTO, len(keys))
	for i, k := range keys {
		dtos[i] = s.toDTO(k)
	}

	return dtos, total, nil
}

// Get retrieves a single API key by ID (never returns raw key)
func (s *APIKeyService) Get(ctx context.Context, id uuid.UUID) (*APIKeyDTO, error) {
	ak, err := s.client.ApiKey.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return s.toDTO(ak), nil
}

// Update updates API key properties
func (s *APIKeyService) Update(
	ctx context.Context,
	id uuid.UUID,
	name string,
	description string,
	rateLimit int,
	dailyLimit int,
) (*APIKeyDTO, error) {
	ak, err := s.client.ApiKey.UpdateOneID(id).
		SetName(name).
		SetDescription(description).
		SetRateLimit(rateLimit).
		SetDailyLimit(dailyLimit).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to update API key: %w", err)
	}

	return s.toDTO(ak), nil
}

// Revoke deactivates an API key
func (s *APIKeyService) Revoke(ctx context.Context, id uuid.UUID) error {
	_, err := s.client.ApiKey.UpdateOneID(id).
		SetIsActive(false).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrAPIKeyNotFound
		}
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	return nil
}

// Delete soft-deletes an API key
func (s *APIKeyService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.client.ApiKey.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrAPIKeyNotFound
		}
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	return nil
}

// Rotate regenerates the API key secret
func (s *APIKeyService) Rotate(ctx context.Context, id uuid.UUID) (*CreateAPIKeyResponse, error) {
	// Get existing key
	ak, err := s.client.ApiKey.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// Extract environment from old key
	env, err := apikey.ExtractEnvironment(ak.Key)
	if err != nil {
		// Default to live if we can't extract
		env = apikey.EnvLive
	}

	// Generate new key
	newKey, err := apikey.GenerateAPIKey(env)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new API key: %w", err)
	}

	// Hash new key
	newHash, err := apikey.HashSecret(newKey)
	if err != nil {
		return nil, fmt.Errorf("failed to hash new API key: %w", err)
	}

	// Update key
	ak, err = s.client.ApiKey.UpdateOne(ak).
		SetKey(newKey).
		SetSecretHash(newHash).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to rotate API key: %w", err)
	}

	return &CreateAPIKeyResponse{
		APIKey:    s.toDTO(ak),
		SecretKey: newKey, // Return new key only once
	}, nil
}

// Validate validates an API key and returns its info
func (s *APIKeyService) Validate(ctx context.Context, key string) (*ValidatedAPIKey, error) {
	// Validate format
	if err := apikey.ValidateFormat(key); err != nil {
		return nil, ErrInvalidAPIKey
	}

	// Find API key by key field, with owner and owner's roles
	ak, err := s.client.ApiKey.Query().
		Where(entapikey.KeyEQ(key)).
		WithOwner(func(q *ent.UserQuery) {
			q.WithRoles()
		}).
		WithRole().
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to query API key: %w", err)
	}

	// Check if active
	if !ak.IsActive {
		return nil, ErrAPIKeyInactive
	}

	// Check if expired
	if !ak.ExpiresAt.IsZero() && time.Now().After(ak.ExpiresAt) {
		return nil, ErrAPIKeyExpired
	}

	// Verify hash
	if !apikey.CompareSecret(ak.SecretHash, key) {
		return nil, ErrInvalidAPIKey
	}

	// Extract owner info and roles
	ownerID := uuid.Nil
	isSuperAdmin := false
	var roles []string
	if owner := ak.Edges.Owner; owner != nil {
		ownerID = owner.ID
		isSuperAdmin = owner.IsSuperAdmin
		// Extract role codes from owner's roles
		if ownerRoles := owner.Edges.Roles; ownerRoles != nil {
			for _, r := range ownerRoles {
				roles = append(roles, r.Code)
			}
		}
	}

	// Extract role ID
	var roleID *uuid.UUID
	if role := ak.Edges.Role; role != nil {
		roleID = &role.ID
	}

	// Extract tenant/domain ID from OwnerDomainID.
	var tenantID string
	if ak.OwnerDomainID != nil {
		tenantID = ak.OwnerDomainID.String()
	}

	return &ValidatedAPIKey{
		ID:           ak.ID,
		Name:         ak.Name,
		Permissions:  ak.Permissions,
		DataFilters:  ak.DataFilters,
		RateLimit:    ak.RateLimit,
		DailyLimit:   ak.DailyLimit,
		UsageCount:   ak.UsageCount,
		OwnerID:      ownerID,
		RoleID:       roleID,
		Roles:        roles,
		TenantID:     tenantID,
		ProjectID:    ak.ProjectID,
		IsSuperAdmin: isSuperAdmin,
	}, nil
}

// GetStats retrieves usage statistics for an API key
func (s *APIKeyService) GetStats(ctx context.Context, id uuid.UUID) (*APIKeyStatsDTO, error) {
	ak, err := s.client.ApiKey.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return &APIKeyStatsDTO{
		UsageCount: ak.UsageCount,
		LastUsedAt: &ak.LastUsedAt,
		LastUsedIP: ak.LastUsedIP,
		IsActive:   ak.IsActive,
		ExpiresAt:  &ak.ExpiresAt,
	}, nil
}

// IncrementUsage tracks API key usage
func (s *APIKeyService) IncrementUsage(ctx context.Context, id uuid.UUID, ip string) error {
	now := time.Now()
	_, err := s.client.ApiKey.UpdateOneID(id).
		AddUsageCount(1).
		SetLastUsedAt(now).
		SetLastUsedIP(ip).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to increment usage: %w", err)
	}

	return nil
}

// toDTO converts ent.ApiKey to APIKeyDTO (masks the secret)
func (s *APIKeyService) toDTO(ak *ent.ApiKey) *APIKeyDTO {
	// Mask the key - show only prefix
	maskedKey := ak.Key
	if len(ak.Key) > 12 {
		maskedKey = ak.Key[:12] + "****"
	}

	return &APIKeyDTO{
		ID:          ak.ID,
		Name:        ak.Name,
		Description: ak.Description,
		Key:         maskedKey,
		Permissions: ak.Permissions,
		DataFilters: ak.DataFilters,
		RateLimit:   ak.RateLimit,
		DailyLimit:  ak.DailyLimit,
		UsageCount:  ak.UsageCount,
		LastUsedAt:  &ak.LastUsedAt,
		LastUsedIP:  ak.LastUsedIP,
		ExpiresAt:   &ak.ExpiresAt,
		ProjectID:   ak.ProjectID,
		IsActive:    ak.IsActive,
		CreatedAt:   ak.CreatedAt,
		UpdatedAt:   ak.UpdatedAt,
	}
}

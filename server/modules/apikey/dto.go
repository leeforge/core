package apikey

import (
	"time"

	"github.com/google/uuid"
)

// CreateAPIKeyRequest represents the request to create an API key
type CreateAPIKeyRequest struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"` // nil means no expiration
	RateLimit   int        `json:"rateLimit,omitempty"`
	DailyLimit  int        `json:"dailyLimit,omitempty"`
	Environment string     `json:"environment,omitempty"`
	ProjectID   string     `json:"projectId,omitempty"`
}

// UpdateAPIKeyRequest represents the request to update an API key
type UpdateAPIKeyRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	RateLimit   int    `json:"rateLimit,omitempty"`
	DailyLimit  int    `json:"dailyLimit,omitempty"`
}

// ValidateAPIKeyRequest represents the request to validate an API key
type ValidateAPIKeyRequest struct {
	Key string `json:"key"`
}

// ValidateAPIKeyResponse represents the validation response
type ValidateAPIKeyResponse struct {
	Valid       bool           `json:"valid"`
	KeyID       uuid.UUID      `json:"keyId,omitempty"`
	Name        string         `json:"name,omitempty"`
	Permissions []string       `json:"permissions,omitempty"`
	DataFilters map[string]any `json:"dataFilters,omitempty"`
	RateLimit   int            `json:"rateLimit,omitempty"`
	DailyLimit  int            `json:"dailyLimit,omitempty"`
	OwnerID     uuid.UUID      `json:"ownerId,omitempty"`
	RoleID      *uuid.UUID     `json:"roleId,omitempty"`
	ProjectID   *uuid.UUID     `json:"projectId,omitempty"`
}

// APIKeyDTO represents an API key response (never includes raw key)
type APIKeyDTO struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Key         string         `json:"key"` // Only the prefix, e.g., "pk_live_****"
	Permissions []string       `json:"permissions,omitempty"`
	DataFilters map[string]any `json:"dataFilters,omitempty"`
	RateLimit   int            `json:"rateLimit"`
	DailyLimit  int            `json:"dailyLimit"`
	UsageCount  int64          `json:"usageCount"`
	LastUsedAt  *time.Time     `json:"lastUsedAt,omitempty"`
	LastUsedIP  string         `json:"lastUsedIp,omitempty"`
	ExpiresAt   *time.Time     `json:"expiresAt,omitempty"`
	ProjectID   *uuid.UUID     `json:"projectId,omitempty"`
	IsActive    bool           `json:"isActive"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// CreateAPIKeyResponse includes the raw key (only returned once)
type CreateAPIKeyResponse struct {
	APIKey    *APIKeyDTO `json:"apiKey"`
	SecretKey string     `json:"secretKey"` // Raw key, shown only once
}

// APIKeyStatsDTO represents usage statistics
type APIKeyStatsDTO struct {
	UsageCount int64      `json:"usageCount"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	LastUsedIP string     `json:"lastUsedIp,omitempty"`
	IsActive   bool       `json:"isActive"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
}

// ValidatedAPIKey represents a validated API key with loaded relationships
type ValidatedAPIKey struct {
	ID           uuid.UUID
	Name         string
	Permissions  []string
	DataFilters  map[string]any
	RateLimit    int
	DailyLimit   int
	UsageCount   int64
	OwnerID      uuid.UUID
	RoleID       *uuid.UUID
	Roles        []string
	TenantID     string
	ProjectID    *uuid.UUID
	IsSuperAdmin bool
}

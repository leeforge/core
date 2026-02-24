package apikey

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// EnvLive represents production environment
	EnvLive = "live"
	// EnvTest represents test environment
	EnvTest = "test"
	// KeyPrefix is the prefix for all API keys
	KeyPrefix = "pk_"
	// SecretLength is the length of the random secret part
	SecretLength = 32
)

var (
	// ErrInvalidFormat indicates the API key format is invalid
	ErrInvalidFormat = errors.New("invalid API key format")
	// ErrInvalidEnvironment indicates the environment is invalid
	ErrInvalidEnvironment = errors.New("invalid environment, must be 'live' or 'test'")
)

// GenerateAPIKey generates a secure API key with format: pk_{env}_{32_char_base64}
// Similar to Stripe's API key format.
//
// Example:
//   - Live: pk_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
//   - Test: pk_test_x9y8z7w6v5u4t3s2r1q0p9o8n7m6l5k4
//
// Parameters:
//   - env: Environment type ("live" or "test")
//
// Returns:
//   - string: Generated API key
//   - error: Error if generation fails
func GenerateAPIKey(env string) (string, error) {
	// Validate environment
	if env != EnvLive && env != EnvTest {
		return "", ErrInvalidEnvironment
	}

	// Generate random bytes
	secret := make([]byte, SecretLength)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}

	// Encode to base64 and remove padding
	encoded := base64.RawURLEncoding.EncodeToString(secret)

	// Format: pk_{env}_{secret}
	key := fmt.Sprintf("%s%s_%s", KeyPrefix, env, encoded)
	return key, nil
}

// HashSecret hashes the API key using bcrypt for secure storage.
// The cost factor is set to bcrypt.DefaultCost (10).
//
// Example:
//
//	hash, err := HashSecret("pk_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6")
//
// Parameters:
//   - key: The API key to hash
//
// Returns:
//   - string: Bcrypt hash of the API key
//   - error: Error if hashing fails
func HashSecret(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key: %w", err)
	}
	return string(hash), nil
}

// CompareSecret verifies an API key against its bcrypt hash.
//
// Example:
//
//	valid := CompareSecret(hash, "pk_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6")
//
// Parameters:
//   - hash: The bcrypt hash stored in database
//   - key: The API key to verify
//
// Returns:
//   - bool: true if the key matches the hash, false otherwise
func CompareSecret(hash, key string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
	return err == nil
}

// ValidateFormat validates that an API key has the correct format.
// Expected format: pk_{env}_{base64_secret}
//
// Example:
//
//	err := ValidateFormat("pk_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6")
//
// Parameters:
//   - key: The API key to validate
//
// Returns:
//   - error: ErrInvalidFormat if format is invalid, nil otherwise
func ValidateFormat(key string) error {
	// Check prefix
	if !strings.HasPrefix(key, KeyPrefix) {
		return fmt.Errorf("%w: must start with '%s'", ErrInvalidFormat, KeyPrefix)
	}

	// Split into parts (limit to 3 parts since base64 URL encoding may contain underscores)
	parts := strings.SplitN(key, "_", 3)
	if len(parts) != 3 {
		return fmt.Errorf("%w: expected format pk_{env}_{secret}", ErrInvalidFormat)
	}

	// Validate environment
	env := parts[1]
	if env != EnvLive && env != EnvTest {
		return fmt.Errorf("%w: environment must be 'live' or 'test'", ErrInvalidFormat)
	}

	// Validate secret part exists and has reasonable length
	secret := parts[2]
	if len(secret) < 20 {
		return fmt.Errorf("%w: secret part is too short", ErrInvalidFormat)
	}

	return nil
}

// ExtractEnvironment extracts the environment from an API key.
//
// Example:
//
//	env, err := ExtractEnvironment("pk_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6")
//	// env = "live"
//
// Parameters:
//   - key: The API key
//
// Returns:
//   - string: Environment ("live" or "test")
//   - error: Error if key format is invalid
func ExtractEnvironment(key string) (string, error) {
	if err := ValidateFormat(key); err != nil {
		return "", err
	}

	parts := strings.SplitN(key, "_", 3)
	return parts[1], nil
}

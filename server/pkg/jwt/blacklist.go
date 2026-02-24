package jwt

import (
	"sync"
	"time"
)

// TokenBlacklist manages invalidated tokens
type TokenBlacklist struct {
	tokens map[string]time.Time // token -> expiration time
	mu     sync.RWMutex
}

// NewTokenBlacklist creates a new token blacklist
func NewTokenBlacklist() *TokenBlacklist {
	bl := &TokenBlacklist{
		tokens: make(map[string]time.Time),
	}

	// Start cleanup goroutine
	go bl.cleanup()

	return bl
}

// Add adds a token to the blacklist
func (bl *TokenBlacklist) Add(token string, expiresAt time.Time) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.tokens[token] = expiresAt
}

// IsBlacklisted checks if a token is blacklisted
func (bl *TokenBlacklist) IsBlacklisted(token string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	expiresAt, exists := bl.tokens[token]
	if !exists {
		return false
	}

	// If token has expired, it's no longer relevant
	if time.Now().After(expiresAt) {
		return false
	}

	return true
}

// cleanup removes expired tokens periodically
func (bl *TokenBlacklist) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		bl.mu.Lock()
		now := time.Now()
		for token, expiresAt := range bl.tokens {
			if now.After(expiresAt) {
				delete(bl.tokens, token)
			}
		}
		bl.mu.Unlock()
	}
}

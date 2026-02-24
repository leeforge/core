package ratelimit

import (
	"sync"
	"time"
)

// window represents a sliding window of timestamps for rate limiting
type window struct {
	timestamps []time.Time
	mu         sync.Mutex
}

// RateLimiter implements an in-memory rate limiter using sliding window algorithm
type RateLimiter struct {
	windows map[string]*window
	mu      sync.RWMutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		windows: make(map[string]*window),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request is allowed based on the rate limit
// Returns true if the request is allowed, false if rate limit exceeded
func (rl *RateLimiter) Allow(key string, limit int, windowDuration time.Duration) bool {
	now := time.Now()

	rl.mu.Lock()
	w, exists := rl.windows[key]
	if !exists {
		w = &window{
			timestamps: make([]time.Time, 0),
		}
		rl.windows[key] = w
	}
	rl.mu.Unlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	// Remove timestamps outside the window
	cutoff := now.Add(-windowDuration)
	validTimestamps := make([]time.Time, 0)
	for _, ts := range w.timestamps {
		if ts.After(cutoff) {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	w.timestamps = validTimestamps

	// Check if limit exceeded
	if len(w.timestamps) >= limit {
		return false
	}

	// Add current timestamp
	w.timestamps = append(w.timestamps, now)
	return true
}

// Reset removes all tracking data for a specific key
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.windows, key)
}

// cleanup periodically removes expired entries to prevent memory leak
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, w := range rl.windows {
			w.mu.Lock()
			// Remove windows that have no recent activity (older than 1 hour)
			if len(w.timestamps) == 0 || now.Sub(w.timestamps[len(w.timestamps)-1]) > time.Hour {
				delete(rl.windows, key)
			}
			w.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// GetCount returns the current count for a key within the window
func (rl *RateLimiter) GetCount(key string, windowDuration time.Duration) int {
	rl.mu.RLock()
	w, exists := rl.windows[key]
	rl.mu.RUnlock()

	if !exists {
		return 0
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-windowDuration)
	count := 0
	for _, ts := range w.timestamps {
		if ts.After(cutoff) {
			count++
		}
	}

	return count
}

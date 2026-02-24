package auth

import (
	"sync"
	"time"

	"github.com/leeforge/framework/auth/rbac"
)

// SimpleCacheAdapter 简单的内存缓存适配器
// 实现 framework/auth/rbac.CacheAdapter 接口
type SimpleCacheAdapter struct {
	cache map[string]*cacheItem
	mu    sync.RWMutex
}

type cacheItem struct {
	value     any
	expiresAt time.Time
}

// NewSimpleCacheAdapter 创建简单缓存适配器
func NewSimpleCacheAdapter() rbac.CacheAdapter {
	adapter := &SimpleCacheAdapter{
		cache: make(map[string]*cacheItem),
	}

	// 启动后台清理过期缓存
	go adapter.cleanupExpired()

	return adapter
}

// Get 获取缓存
func (c *SimpleCacheAdapter) Get(key string) (any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.cache[key]
	if !exists {
		return nil, ErrCacheNotFound
	}

	if time.Now().After(item.expiresAt) {
		return nil, ErrCacheExpired
	}

	return item.value, nil
}

// Set 设置缓存
// ttl: 过期时间（秒）
func (c *SimpleCacheAdapter) Set(key string, value any, ttl int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = &cacheItem{
		value:     value,
		expiresAt: time.Now().Add(time.Duration(ttl) * time.Second),
	}

	return nil
}

// Delete 删除缓存
func (c *SimpleCacheAdapter) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
	return nil
}

// cleanupExpired 定期清理过期缓存
func (c *SimpleCacheAdapter) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.cache {
			if now.After(item.expiresAt) {
				delete(c.cache, key)
			}
		}
		c.mu.Unlock()
	}
}

// 缓存错误定义
var (
	ErrCacheNotFound = &CacheError{Message: "cache not found"}
	ErrCacheExpired  = &CacheError{Message: "cache expired"}
)

// CacheError 缓存错误
type CacheError struct {
	Message string
}

func (e *CacheError) Error() string {
	return e.Message
}

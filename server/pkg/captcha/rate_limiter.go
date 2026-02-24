package captcha

import (
	"context"
	"fmt"
	"time"

	frameCaptcha "github.com/leeforge/framework/captcha"
	"github.com/redis/go-redis/v9"
)

const (
	generateKeyPrefix = "captcha:gen:"    // 生成限流 key 前缀
	verifyKeyPrefix   = "captcha:verify:" // 验证限流 key 前缀
	failureKeyPrefix  = "captcha:fail:"   // 失败记录 key 前缀
)

// RedisRateLimiter Redis 限流器
type RedisRateLimiter struct {
	client *redis.Client
	config *frameCaptcha.Config
}

// NewRedisRateLimiter 创建 Redis 限流器
func NewRedisRateLimiter(client *redis.Client, config *frameCaptcha.Config) frameCaptcha.RateLimiter {
	return &RedisRateLimiter{
		client: client,
		config: config,
	}
}

func (r *RedisRateLimiter) AllowGenerate(ctx context.Context, identifier string) error {
	key := fmt.Sprintf("%s%s", generateKeyPrefix, identifier)

	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("限流检查失败: %w", err)
	}

	if err := r.ensureWindowTTL(ctx, key, count, r.config.GenerateWindow); err != nil {
		return fmt.Errorf("设置生成限流窗口失败: %w", err)
	}

	if count > int64(r.config.GenerateLimit) {
		return frameCaptcha.ErrRateLimitExceeded
	}

	return nil
}

func (r *RedisRateLimiter) AllowVerify(ctx context.Context, identifier string) error {
	failKey := fmt.Sprintf("%s%s", failureKeyPrefix, identifier)

	count, err := r.client.Get(ctx, failKey).Int()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("检查尝试次数失败: %w", err)
	}

	if count >= r.config.MaxAttempts {
		return frameCaptcha.ErrRateLimitExceeded
	}

	return nil
}

func (r *RedisRateLimiter) RecordFailure(ctx context.Context, identifier string) error {
	key := fmt.Sprintf("%s%s", failureKeyPrefix, identifier)

	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	if err := r.ensureWindowTTL(ctx, key, count, r.config.AttemptWindow); err != nil {
		return fmt.Errorf("设置失败计数窗口失败: %w", err)
	}

	return nil
}

func (r *RedisRateLimiter) Reset(ctx context.Context, identifier string) error {
	keys := []string{
		fmt.Sprintf("%s%s", generateKeyPrefix, identifier),
		fmt.Sprintf("%s%s", verifyKeyPrefix, identifier),
		fmt.Sprintf("%s%s", failureKeyPrefix, identifier),
	}

	return r.client.Del(ctx, keys...).Err()
}

func (r *RedisRateLimiter) ensureWindowTTL(ctx context.Context, key string, count int64, window time.Duration) error {
	// First write in the window should always attach expiration.
	if count == 1 {
		return r.client.Expire(ctx, key, window).Err()
	}

	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return err
	}

	// ttl < 0 means no expiry (or key missing); repair to avoid stale permanent lockouts.
	if ttl < 0 {
		return r.client.Expire(ctx, key, window).Err()
	}

	return nil
}

package captcha

import (
	"context"
	"fmt"
	"time"

	frameCaptcha "github.com/leeforge/framework/captcha"
	"github.com/redis/go-redis/v9"
)

const captchaKeyPrefix = "captcha:"

// RedisStore Redis 存储实现
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore 创建 Redis 存储
func NewRedisStore(client *redis.Client) frameCaptcha.Store {
	return &RedisStore{client: client}
}

func (s *RedisStore) Save(ctx context.Context, id string, answer string, ttl time.Duration) error {
	key := s.key(id)
	return s.client.Set(ctx, key, answer, ttl).Err()
}

func (s *RedisStore) Get(ctx context.Context, id string) (string, error) {
	key := s.key(id)
	val, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", frameCaptcha.ErrCaptchaNotFound
	}
	if err != nil {
		return "", fmt.Errorf("获取验证码失败: %w", err)
	}
	return val, nil
}

func (s *RedisStore) Delete(ctx context.Context, id string) error {
	key := s.key(id)
	return s.client.Del(ctx, key).Err()
}

func (s *RedisStore) Exists(ctx context.Context, id string) (bool, error) {
	key := s.key(id)
	count, err := s.client.Exists(ctx, key).Result()
	return count > 0, err
}

func (s *RedisStore) key(id string) string {
	return fmt.Sprintf("%s%s", captchaKeyPrefix, id)
}

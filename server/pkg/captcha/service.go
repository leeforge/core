package captcha

import (
	"context"
	"fmt"
	"strings"

	frameCaptcha "github.com/leeforge/framework/captcha"
)

type service struct {
	store       frameCaptcha.Store
	generator   frameCaptcha.Generator
	rateLimiter frameCaptcha.RateLimiter
	config      *frameCaptcha.Config
}

// NewService 创建验证码服务
func NewService(
	store frameCaptcha.Store,
	generator frameCaptcha.Generator,
	rateLimiter frameCaptcha.RateLimiter,
	config *frameCaptcha.Config,
) frameCaptcha.Service {
	return &service{
		store:       store,
		generator:   generator,
		rateLimiter: rateLimiter,
		config:      config,
	}
}

func (s *service) Generate(ctx context.Context, captchaType frameCaptcha.CaptchaType, identifier string) (*frameCaptcha.CaptchaData, error) {
	// 未启用时返回禁用状态
	if !s.config.Enabled {
		return &frameCaptcha.CaptchaData{
			ID:   "disabled",
			Type: captchaType,
		}, nil
	}

	// 检查生成限流
	if err := s.rateLimiter.AllowGenerate(ctx, identifier); err != nil {
		return nil, err
	}

	// 生成验证码
	data, answer, err := s.generator.Generate(ctx, captchaType)
	if err != nil {
		return nil, err
	}

	// 存储答案
	if err := s.store.Save(ctx, data.ID, answer, s.config.TTL); err != nil {
		return nil, fmt.Errorf("保存验证码失败: %w", err)
	}

	return data, nil
}

func (s *service) Verify(ctx context.Context, id string, answer string, identifier string) (*frameCaptcha.VerifyResult, error) {
	result := &frameCaptcha.VerifyResult{Valid: false}

	// 未启用时直接通过
	if !s.config.Enabled {
		result.Valid = true
		return result, nil
	}

	// 检查验证限流
	if err := s.rateLimiter.AllowVerify(ctx, identifier); err != nil {
		result.FailureReason = "尝试次数过多"
		return result, err
	}

	// 获取存储的答案
	storedAnswer, err := s.store.Get(ctx, id)
	if err != nil {
		if err == frameCaptcha.ErrCaptchaNotFound {
			result.FailureReason = "验证码不存在或已过期"
			return result, nil
		} else {
			result.FailureReason = "验证失败"
		}
		return result, err
	}

	// 比较答案（不区分大小写，去除空格）
	if strings.TrimSpace(strings.ToLower(answer)) != strings.TrimSpace(strings.ToLower(storedAnswer)) {
		if recordErr := s.rateLimiter.RecordFailure(ctx, identifier); recordErr != nil {
			return result, fmt.Errorf("记录验证码失败次数失败: %w", recordErr)
		}
		result.FailureReason = "答案错误"
		return result, nil
	}

	// 验证通过 - 清理
	s.store.Delete(ctx, id)
	s.rateLimiter.Reset(ctx, identifier)

	result.Valid = true
	return result, nil
}

package captcha

import (
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/leeforge/core/core"
	pkgCaptcha "github.com/leeforge/core/server/pkg/captcha"

	frameCaptcha "github.com/leeforge/framework/captcha"
	"github.com/leeforge/framework/logging"
	framePerm "github.com/leeforge/framework/permission"
)

// Module 验证码模块
type Module struct {
	service frameCaptcha.Service
	config  *frameCaptcha.Config
	logger  logging.Logger
}

// NewModule 创建验证码模块
func NewModule(redisClient *redis.Client, config *frameCaptcha.Config) core.Module {
	// 依赖注入
	store := pkgCaptcha.NewRedisStore(redisClient)
	generator := pkgCaptcha.NewMathGenerator(config.Math)
	rateLimiter := pkgCaptcha.NewRedisRateLimiter(redisClient, config)
	service := pkgCaptcha.NewService(store, generator, rateLimiter, config)

	return &Module{
		service: service,
		config:  config,
	}
}

// Name 返回模块名称
func (m *Module) Name() string {
	return "captcha"
}

// RegisterPublicRoutes 注册公共路由
func (m *Module) RegisterPublicRoutes(router chi.Router) {
	handler := NewHandler(m.service, m.config)

	router.Route("/captcha", func(r chi.Router) {
		framePerm.Get(r, "/", handler.Generate, framePerm.Public("Generate captcha", "captcha.generate"))
		framePerm.Post(r, "/verify", handler.Verify, framePerm.Public("Verify captcha", "captcha.verify"))
	})
}

// RegisterPrivateRoutes 注册私有路由（验证码模块不需要私有路由）
func (m *Module) RegisterPrivateRoutes(router chi.Router) {
	// 验证码模块不需要私有路由
}

// GetService 返回 captcha 服务（用于注入到其他模块）
func (m *Module) GetService() frameCaptcha.Service {
	return m.service
}

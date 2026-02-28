package modules

import (
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-viper/mapstructure/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	corecore "github.com/leeforge/core/core"
	coremw "github.com/leeforge/core/middleware"

	"github.com/leeforge/core/server/config"
	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/modules/apikey"
	"github.com/leeforge/core/server/modules/auditlog"
	"github.com/leeforge/core/server/modules/auth"
	"github.com/leeforge/core/server/modules/captcha"
	"github.com/leeforge/core/server/modules/dictionary"
	domainmod "github.com/leeforge/core/server/modules/domain"
	initmod "github.com/leeforge/core/server/modules/init"
	"github.com/leeforge/core/server/modules/mcp"
	"github.com/leeforge/core/server/modules/media"
	"github.com/leeforge/core/server/modules/menu"
	"github.com/leeforge/core/server/modules/operationlog"
	"github.com/leeforge/core/server/modules/permission"
	"github.com/leeforge/core/server/modules/role"
	"github.com/leeforge/core/server/modules/schema"
	"github.com/leeforge/core/server/modules/systemerror"
	"github.com/leeforge/core/server/modules/user"
	"github.com/leeforge/core/server/permissionsync"
	pkgAuth "github.com/leeforge/core/server/pkg/auth"
	domainSvc "github.com/leeforge/core/server/services/domain"

	frameCaptcha "github.com/leeforge/framework/captcha"
	frameLogging "github.com/leeforge/framework/logging"
)

const defaultBusinessPrefix = "/api/v1"

func registerBusinessModules(router chi.Router, rawConfig any, logger *zap.Logger, extraFactories ...corecore.ModuleFactory) error {
	if router == nil {
		return fmt.Errorf("router is nil")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	cfg, err := decodeConfig(rawConfig)
	if err != nil {
		return fmt.Errorf("decode config: %w", err)
	}

	baseLogger := frameLogging.FromZap(logger)
	cfg, client := resolveRuntimeInput(rawConfig, cfg)
	deps := &corecore.Dependencies{
		Client:      client,
		Config:      cfg,
		PermManager: &pkgAuth.PermissionManager{},
		PermSyncer:  permissionsync.NewSyncer(client, baseLogger),
	}
	deps.DomainService = domainSvc.NewService(client, baseLogger)
	deps.InvitationProviders = corecore.NewInvitationProviderRegistry()

	captchaCfg := &frameCaptcha.Config{
		Enabled:        cfg.Captcha.Enabled,
		TTL:            parseDuration(cfg.Captcha.TTL, 5*time.Minute),
		GenerateLimit:  cfg.Captcha.GenerateLimit,
		GenerateWindow: parseDuration(cfg.Captcha.GenerateWindow, time.Minute),
		MaxAttempts:    cfg.Captcha.MaxAttempts,
		AttemptWindow:  parseDuration(cfg.Captcha.AttemptWindow, 5*time.Minute),
		Math: frameCaptcha.MathConfig{
			Width:           cfg.Captcha.Math.Width,
			Height:          cfg.Captcha.Math.Height,
			NoiseCount:      cfg.Captcha.Math.NoiseCount,
			ShowLineOptions: cfg.Captcha.Math.ShowLineOptions,
		},
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Cache.Addr(),
		Password: cfg.Cache.Password,
		DB:       cfg.Cache.DB,
	})
	captchaModule := captcha.NewModule(redisClient, captchaCfg).(*captcha.Module)
	deps.CaptchaService = captchaModule.GetService()

	authModule := auth.NewAuthModule(baseLogger, deps)
	deps.JWTService = authModule.GetJWTService()

	pluginRouter := resolvePluginRouter(rawConfig)

	router.Route(defaultBusinessPrefix, func(api chi.Router) {
		deps.Router = api

		// Public routes — no authentication required.
		authModule.RegisterPublicRoutes(api)

		// Private routes — JWT authentication + domain resolution.
		private := api.With(
			auth.JWTMiddleware(authModule.GetJWTService()),
			coremw.DomainResolverMiddleware(&coremw.DomainResolverConfig{
				Logger:        baseLogger,
				DomainService: deps.DomainService,
			}),
		)
		authModule.RegisterPrivateRoutes(private)

		// Merge plugin routes into the private sub-router (requires auth + domain context).
		mergePluginRoutes(private, pluginRouter)

		allFactories := []corecore.ModuleFactory{
			initmod.NewInitModule,
			user.NewUserModule,
			role.NewRoleModule,
			permission.NewPermissionModule,
			menu.NewMenuModule,
			dictionary.NewDictionaryModule,
			domainmod.NewDomainModule,
			media.NewMediaModule,
			apikey.NewAPIKeyModule,
			schema.NewSchemaModule,
			mcp.NewMCPModule,
			auditlog.NewAuditLogModule,
			operationlog.NewOperationLogModule,
			systemerror.NewSystemErrorModule,
			func(frameLogging.Logger, *corecore.Dependencies) corecore.Module {
				return captchaModule
			},
		}
		allFactories = append(allFactories, extraFactories...)
		corecore.BootstrapModules(
			api,
			private,
			baseLogger,
			deps,
			allFactories...,
		)
	})

	return nil
}

func decodeConfig(raw any) (*config.Config, error) {
	if wrapped, ok := raw.(map[string]any); ok {
		if nested, exists := wrapped["config"]; exists {
			return decodeConfig(nested)
		}
	}
	if raw == nil {
		cfg := config.Config{}
		config.ApplyDefaults(&cfg)
		return &cfg, nil
	}
	if cfg, ok := raw.(*config.Config); ok && cfg != nil {
		copyCfg := *cfg
		config.ApplyDefaults(&copyCfg)
		return &copyCfg, nil
	}
	if cfg, ok := raw.(config.Config); ok {
		copyCfg := cfg
		config.ApplyDefaults(&copyCfg)
		return &copyCfg, nil
	}

	var out config.Config
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "mapstructure",
		Result:           &out,
		WeaklyTypedInput: true,
	})
	if err != nil {
		return nil, err
	}
	if err := decoder.Decode(raw); err != nil {
		return nil, err
	}
	config.ApplyDefaults(&out)
	return &out, nil
}

func resolveRuntimeInput(raw any, decoded *config.Config) (*config.Config, *ent.Client) {
	cfg := decoded
	var client *ent.Client

	if wrapped, ok := raw.(map[string]any); ok {
		if nestedCfg, exists := wrapped["config"]; exists {
			if parsedCfg, err := decodeConfig(nestedCfg); err == nil {
				cfg = parsedCfg
			}
		}
		if cli, exists := wrapped["client"]; exists {
			if typed, ok := cli.(*ent.Client); ok {
				client = typed
			}
		}
	}

	return cfg, client
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}

func resolvePluginRouter(raw any) chi.Router {
	wrapped, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	pr, exists := wrapped["pluginRouter"]
	if !exists {
		return nil
	}
	typed, ok := pr.(chi.Router)
	if !ok {
		return nil
	}
	return typed
}

func mergePluginRoutes(target chi.Router, source chi.Router) {
	if source == nil {
		return
	}
	pluginTarget := target
	if scoped, ok := any(target).(interface{ WithOwner(string) chi.Router }); ok {
		pluginTarget = scoped.WithOwner("plugins")
	}
	pluginTarget.Mount("/", source)
}

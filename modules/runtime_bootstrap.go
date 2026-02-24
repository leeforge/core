package modules

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-viper/mapstructure/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	corecore "github.com/leeforge/core/core"
	"github.com/leeforge/core/host"

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

func registerBusinessModules(router chi.Router, rawConfig any, logger *zap.Logger) error {
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
	apiRouter := host.NewRouteRegistryRouter(chi.NewRouter(), "core")
	deps := &corecore.Dependencies{
		Client:      client,
		Config:      cfg,
		PermManager: &pkgAuth.PermissionManager{},
		PermSyncer:  permissionsync.NewSyncer(client, baseLogger),
		Router:      apiRouter,
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

	authModule.RegisterPublicRoutes(apiRouter)
	privateRouter := apiRouter.Group(func(pr chi.Router) {
		authModule.RegisterPrivateRoutes(pr)
	})

	corecore.BootstrapModules(
		apiRouter,
		privateRouter,
		baseLogger,
		deps,
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
	)

	if err := mergeRoutesWithPrefix(router, apiRouter, defaultBusinessPrefix); err != nil {
		return fmt.Errorf("merge business routes: %w", err)
	}
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

type routeOwnerLookup interface {
	LookupRouteOwner(method, path string) (string, bool)
}

type ownerScopedRouter interface {
	WithOwner(owner string) chi.Router
}

func mergeRoutesWithPrefix(dst chi.Router, src chi.Routes, prefix string) error {
	var lookup routeOwnerLookup
	if typed, ok := src.(routeOwnerLookup); ok {
		lookup = typed
	}
	return chi.Walk(src, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		fullPath := joinPath(prefix, route)
		target := dst
		if lookup != nil {
			if owner, ok := lookup.LookupRouteOwner(method, route); ok {
				if scoped, ok := dst.(ownerScopedRouter); ok {
					target = scoped.WithOwner(owner)
				}
			}
		}
		target.Method(method, fullPath, chain(handler, middlewares...))
		return nil
	})
}

func chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	wrapped := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}

func joinPath(prefix, route string) string {
	left := strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	right := strings.TrimSpace(route)
	if right == "" {
		right = "/"
	}
	if !strings.HasPrefix(right, "/") {
		right = "/" + right
	}
	if right != "/" {
		right = strings.TrimSuffix(right, "/")
	}
	if left == "" {
		return right
	}
	return left + right
}

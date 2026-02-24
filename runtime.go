package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"entgo.io/ent/dialect"
	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/leeforge/core/host"
	"github.com/leeforge/core/modules"
	"github.com/leeforge/core/server/config"
	"github.com/leeforge/core/server/ent"
	domainSvc "github.com/leeforge/core/server/services/domain"
	frameworkconfig "github.com/leeforge/framework/config"
	frameLogging "github.com/leeforge/framework/logging"
	frameplugin "github.com/leeforge/framework/plugin"
	frameworkruntime "github.com/leeforge/framework/runtime"
	frameworkmigration "github.com/leeforge/framework/runtime/migration"
)

type PluginRegistrar func(rt *frameworkruntime.Runtime, services *frameplugin.ServiceRegistry, logger *zap.Logger) error

type RuntimeOptions struct {
	ConfigPath       string
	Modules          []host.ModuleBootstrapper
	ResourceProvider ResourceProvider
	SkipPlugins      bool
	SkipMigrate      bool
	BasePath         string
	Logger           *zap.Logger
	PluginRegistrar  PluginRegistrar
}

type Runtime interface {
	Router() chi.Router
	Handler() http.Handler
	Shutdown(context.Context) error
}

type RuntimeResources struct {
	CoreClient      *ent.Client
	MigrationRunner func(context.Context) error
	Closers         []io.Closer
}

type ResourceInput struct {
	Config *config.Config
	Logger *zap.Logger
}

type ResourceProvider interface {
	Build(context.Context, ResourceInput) (*RuntimeResources, error)
}

type runtime struct {
	router        chi.Router
	handler       http.Handler
	resources     *RuntimeResources
	pluginRuntime *frameworkruntime.Runtime
}

func (r *runtime) Router() chi.Router {
	return r.router
}

func (r *runtime) Handler() http.Handler {
	return r.handler
}

func (r *runtime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	var result error
	if r.pluginRuntime != nil {
		result = multierr.Append(result, r.pluginRuntime.Shutdown(ctx))
	}
	if r.resources == nil {
		return result
	}
	for _, closer := range r.resources.Closers {
		if closer == nil {
			continue
		}
		result = multierr.Append(result, closer.Close())
	}
	return result
}

func BuildRuntime(ctx context.Context, opts RuntimeOptions) (Runtime, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.ConfigPath == "" {
		return nil, &ErrConfigLoad{Cause: fmt.Errorf("config path is required")}
	}

	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	cfg, err := loadRuntimeConfig(opts.ConfigPath)
	if err != nil {
		return nil, &ErrConfigLoad{Cause: err}
	}

	provider := opts.ResourceProvider
	if provider == nil {
		provider = defaultResourceProvider{}
	}
	resources, err := provider.Build(ctx, ResourceInput{Config: cfg, Logger: logger})
	if err != nil {
		return nil, &ErrResourceInit{Cause: err}
	}
	if resources == nil {
		resources = &RuntimeResources{}
	}

	if resources.CoreClient != nil {
		resources.Closers = append(resources.Closers, resources.CoreClient)
	}

	runtimeConfig := map[string]any{
		"config": cfg,
	}
	if resources.CoreClient != nil {
		runtimeConfig["client"] = resources.CoreClient
	}

	registrar := opts.PluginRegistrar
	if opts.SkipPlugins {
		registrar = nil
	}

	router := host.NewRouteRegistryRouter(chi.NewRouter(), "runtime")
	pluginRuntime, err := bootstrapPlugins(ctx, router, opts.BasePath, resources, logger, registrar)
	if err != nil {
		return nil, err
	}

	coreModules := buildModuleBootstrapper(opts.Modules)

	if err := host.RegisterAllChi(router, host.CoreOptions{
		Logger:             logger,
		Config:             runtimeConfig,
		ModuleBootstrapper: coreModules,
		MigrationRunner:    resources.MigrationRunner,
		SkipMigrate:        opts.SkipMigrate,
		BasePath:           opts.BasePath,
	}); err != nil {
		return nil, err
	}
	if err := host.RegisterSwaggerChi(router, &host.SwaggerOptions{BasePath: opts.BasePath}); err != nil {
		return nil, err
	}

	return &runtime{
		router:        router,
		handler:       router,
		resources:     resources,
		pluginRuntime: pluginRuntime,
	}, nil
}

type defaultResourceProvider struct{}

func (defaultResourceProvider) Build(_ context.Context, input ResourceInput) (*RuntimeResources, error) {
	if input.Config == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if err := input.Config.Database.Validate(); err != nil {
		return nil, fmt.Errorf("validate database config: %w", err)
	}

	driver, err := resolveEntDriver(input.Config.Database.DSN())
	if err != nil {
		return nil, err
	}

	client, err := ent.Open(driver, input.Config.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("open core ent client: %w", err)
	}

	res := &RuntimeResources{
		CoreClient: client,
		Closers:    []io.Closer{client},
	}
	if input.Config.Database.AutoMigrate {
		res.MigrationRunner = frameworkmigration.NewManager(
			frameworkmigration.NewEntStrategy(func(ctx context.Context) error {
				return client.Schema.Create(ctx)
			}),
		).Run
	}
	return res, nil
}

func resolveEntDriver(dsn string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(dsn))
	switch {
	case strings.HasPrefix(lower, "postgres://"), strings.HasPrefix(lower, "postgresql://"):
		return dialect.Postgres, nil
	case strings.HasPrefix(lower, "sqlite://"), strings.HasPrefix(lower, "file:"):
		return dialect.SQLite, nil
	case strings.HasPrefix(lower, "mysql://"), strings.HasPrefix(lower, "mariadb://"):
		return dialect.MySQL, nil
	default:
		return "", fmt.Errorf("unsupported database dsn: expected postgres/sqlite/mysql scheme")
	}
}

func buildModuleBootstrapper(extra []host.ModuleBootstrapper) host.ModuleBootstrapper {
	return func(router chi.Router, cfg any, logger *zap.Logger) error {
		if err := modules.Bootstrap(router, cfg, logger); err != nil {
			return err
		}
		for i, bootstrap := range extra {
			if bootstrap == nil {
				continue
			}
			target := router
			if scoped, ok := any(router).(interface{ WithOwner(string) chi.Router }); ok {
				target = scoped.WithOwner(fmt.Sprintf("module[%d]", i))
			}
			if err := bootstrap(target, cfg, logger); err != nil {
				return err
			}
		}
		return nil
	}
}

func bootstrapPlugins(
	ctx context.Context,
	router chi.Router,
	basePath string,
	resources *RuntimeResources,
	logger *zap.Logger,
	registrar PluginRegistrar,
) (*frameworkruntime.Runtime, error) {
	if registrar == nil {
		return nil, nil
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if router == nil {
		return nil, &host.ErrPluginBootstrap{Cause: fmt.Errorf("plugin router is nil")}
	}

	base := strings.TrimSpace(basePath)
	if base == "" {
		base = host.DefaultBasePath
	}
	pluginRouter := chi.NewRouter()
	pluginRT := frameworkruntime.NewRuntime(frameworkruntime.Config{
		Router: pluginRouter,
		Logger: logger,
	})

	services := pluginRT.Services()
	if services == nil {
		return nil, &host.ErrPluginBootstrap{Cause: fmt.Errorf("plugin service registry is nil")}
	}

	if resources != nil && resources.CoreClient != nil {
		domainService := domainSvc.NewService(resources.CoreClient, frameLogging.FromZap(logger))
		if err := services.Register("domain.service", newDomainWriterAdapter(domainService)); err != nil {
			return nil, &host.ErrPluginBootstrap{Cause: fmt.Errorf("register domain service: %w", err)}
		}
	}
	invites := NewInvitationProviderRegistry()
	if err := services.Register(InvitationProviderRegistryServiceKey, invites); err != nil {
		return nil, &host.ErrPluginBootstrap{Cause: fmt.Errorf("register invitation registry: %w", err)}
	}

	if err := registrar(pluginRT, services, logger); err != nil {
		return nil, &host.ErrPluginBootstrap{Cause: err}
	}

	if err := pluginRT.Bootstrap(ctx); err != nil {
		return nil, &host.ErrPluginBootstrap{Cause: fmt.Errorf("bootstrap plugins: %w", err)}
	}

	if scoped, ok := router.(interface {
		WithOwner(string) chi.Router
	}); ok {
		scoped.WithOwner("plugins").Mount(base, pluginRouter)
	} else {
		router.Mount(base, pluginRouter)
	}

	return pluginRT, nil
}

func loadRuntimeConfig(configPath string) (*config.Config, error) {
	result := &config.Config{}
	for _, section := range runtimeConfigSections {
		if err := loadConfigSection(configPath, section, result); err != nil {
			return nil, err
		}
	}
	config.ApplyDefaults(result)
	if err := result.Database.Validate(); err != nil {
		return nil, fmt.Errorf("validate database config: %w", err)
	}
	return result, nil
}

var runtimeConfigSections = []string{
	"server",
	"database",
	"cache",
	"log",
	"tracing",
	"metrics",
	"security",
	"access_control",
	"frontend",
	"captcha",
	"init",
}

func loadConfigSection(configPath, section string, result *config.Config) error {
	opts := frameworkconfig.DefaultConfigOptions()
	opts.BasePath = configPath
	opts.FileName = section
	opts.FileType = "yaml"
	opts.EnvPrefix = "LEEFORGE"
	opts.WatchAble = false
	opts.LoadAll = false

	cfg, err := frameworkconfig.NewConfig(opts)
	if err != nil {
		return fmt.Errorf("create config section %q: %w", section, err)
	}
	if err := cfg.Bind(result); err != nil {
		return fmt.Errorf("bind config section %q: %w", section, err)
	}
	return nil
}

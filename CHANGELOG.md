# Changelog

## [Unreleased]

### Changed
- **BREAKING**: Unified module registration to use `ModuleFactory` interface only
  - Removed `Modules` field from `RuntimeOptions` (was `[]host.ModuleBootstrapper`)
  - Removed `ModuleBootstrapper` from bootstrap pipeline
  - All external modules now must use `ModuleFactory` interface for consistent dependency injection
  - Removed `BootstrapWithExtras` function from `modules.Bootstrap` — now accepts variadic `ModuleFactory` arguments directly

### Benefits
- Single, clear module registration path
- All modules have access to `Dependencies` for consistent DI
- Reduced code complexity and cognitive overhead
- Easier to extend with new modules
- Improved testability with unified module interface

### Migration Guide

If you were registering modules via `RuntimeOptions.Modules`:

```go
// Before
rt, _ := BuildRuntime(ctx, RuntimeOptions{
  Modules: []host.ModuleBootstrapper{
    func(router, cfg, logger) error { /* ... */ }
  },
})

// After
rt, _ := BuildRuntime(ctx, RuntimeOptions{
  ModuleFactories: []corecore.ModuleFactory{
    func(logger, deps) corecore.Module { /* ... */ }
  },
})
```

The new approach provides better dependency injection through `Dependencies` struct:
```go
type ModuleFactory func(frameLogging.Logger, *Dependencies) Module

type Dependencies struct {
  Client              *ent.Client
  Config              *config.Config
  PermManager         *pkgAuth.PermissionManager
  PermSyncer          permissionsync.Syncer
  DomainService       *domainSvc.Service
  InvitationProviders InvitationProviderRegistry
  CaptchaService      frameCaptcha.Service
  JWTService          *auth.JWTService
  Router              chi.Router
}
```

All business modules and external modules now follow the same registration pattern:

```go
// Example: Creating a custom module
func NewMyModule(logger frameLogging.Logger, deps *corecore.Dependencies) corecore.Module {
  return &MyModule{
    logger:   logger,
    client:   deps.Client,
    service:  deps.DomainService,
  }
}

// Register it
opts := core.RuntimeOptions{
  ConfigPath: "./configs",
  ModuleFactories: []corecore.ModuleFactory{
    NewMyModule,
  },
}
```

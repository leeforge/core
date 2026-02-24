package host

import (
	"context"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type PluginRegistrar func(rt RuntimeAdapter, cfg any, logger *zap.Logger) error
type ModuleBootstrapper func(router chi.Router, cfg any, logger *zap.Logger) error

type CoreOptions struct {
	Logger             *zap.Logger
	Config             any
	PluginRegistrar    PluginRegistrar
	ModuleBootstrapper ModuleBootstrapper
	MigrationRunner    func(context.Context) error
	SkipMigrate        bool
	BasePath           string
}

type RuntimeAdapter interface {
	RegisterPlugin(name string, p any) error
	Router() chi.Router
}

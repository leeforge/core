package host

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type pluginManifestItem struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
}

type pluginManifest struct {
	Plugins []pluginManifestItem `yaml:"plugins"`
}

// NewStaticPluginRegistrar registers the provided plugin names in order.
func NewStaticPluginRegistrar(names ...string) PluginRegistrar {
	copied := append([]string(nil), names...)
	return func(rt RuntimeAdapter, _ any, logger *zap.Logger) error {
		if rt == nil {
			return fmt.Errorf("plugin runtime is nil")
		}
		if logger == nil {
			logger = zap.NewNop()
		}
		seen := make(map[string]struct{}, len(copied))
		for _, raw := range copied {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			if err := rt.RegisterPlugin(name, nil); err != nil {
				return fmt.Errorf("register plugin %s: %w", name, err)
			}
			logger.Info("plugin registered via static registrar", zap.String("plugin", name))
			seen[name] = struct{}{}
		}
		return nil
	}
}

// NewManifestPluginRegistrar loads plugin names from plugins.yaml.
func NewManifestPluginRegistrar(manifestPath string) PluginRegistrar {
	path := strings.TrimSpace(manifestPath)
	return func(rt RuntimeAdapter, _ any, logger *zap.Logger) error {
		if rt == nil {
			return fmt.Errorf("plugin runtime is nil")
		}
		if logger == nil {
			logger = zap.NewNop()
		}
		if path == "" {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read plugin manifest: %w", err)
		}

		var manifest pluginManifest
		if err := yaml.Unmarshal(b, &manifest); err != nil {
			return fmt.Errorf("parse plugin manifest: %w", err)
		}

		names := make([]string, 0, len(manifest.Plugins))
		for _, p := range manifest.Plugins {
			if !p.Enabled {
				continue
			}
			names = append(names, p.Name)
		}
		return NewStaticPluginRegistrar(names...)(rt, nil, logger)
	}
}

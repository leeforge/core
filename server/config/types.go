package config

import (
	"github.com/leeforge/framework/logging"
)

type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Cache         CacheConfig         `mapstructure:"cache"`
	Log           LogConfig           `mapstructure:"log"`
	Tracing       TracingConfig       `mapstructure:"tracing"`
	Metrics       MetricsConfig       `mapstructure:"metrics"`
	Security      SecurityConfig      `mapstructure:"security"`
	AccessControl AccessControlConfig `mapstructure:"access_control"`
	Plugins       PluginsConfig       `mapstructure:"plugins"`
	Init          InitConfig          `mapstructure:"init"`
	Frontend      FrontendConfig      `mapstructure:"frontend"`
	Captcha       CaptchaConfig       `mapstructure:"captcha"`
}

type PluginsConfig struct {
	OU FeatureToggle `mapstructure:"ou"`
}

type AccessControlConfig struct {
	MultiTenancy MultiTenancyConfig `mapstructure:"multi_tenancy"` // Deprecated: use Domain config
	Project      ProjectConfig      `mapstructure:"project"`       // Deprecated: managed by tenant plugin
	Domain       DomainConfig       `mapstructure:"domain"`
	ABAC         FeatureToggle      `mapstructure:"abac"`
	Share        FeatureToggle      `mapstructure:"share"`
	Quota        FeatureToggle      `mapstructure:"quota"`
	Audit        FeatureToggle      `mapstructure:"audit"`
}

// DomainConfig configures the domain kernel behavior.
type DomainConfig struct {
	// Mode controls domain resolution: "domain" (new) or "tenant" (legacy compat).
	Mode string `mapstructure:"mode"`
	// DefaultDomainID is the fallback domain ID when none is resolved.
	DefaultDomainID string `mapstructure:"default_domain_id"`
}

// Deprecated: MultiTenancyConfig is superseded by DomainConfig.
type MultiTenancyConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	DefaultTenantID string `mapstructure:"default_tenant_id"`
}

// Deprecated: ProjectConfig is managed by the tenant plugin.
type ProjectConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	DefaultProjectID string `mapstructure:"default_project_id"`
	RequireProject   bool   `mapstructure:"require_project"`
}

type FeatureToggle struct {
	Enabled bool `mapstructure:"enabled"`
}

type InitConfig struct {
	SecretKey string `mapstructure:"secret_key"`
}

type FrontendConfig struct {
	URL string `mapstructure:"url"`
}

type LogConfig struct {
	Director       string `mapstructure:"director" json:"director" yaml:"director" toml:"director"`
	MessageKey     string `mapstructure:"message-key" json:"messageKey" yaml:"message-key" toml:"message-key"`
	LevelKey       string `mapstructure:"level-key" json:"levelKey" yaml:"level-key" toml:"level-key"`
	TimeKey        string `mapstructure:"time-key" json:"timeKey" yaml:"time-key" toml:"time-key"`
	NameKey        string `mapstructure:"name-key" json:"nameKey" yaml:"name-key" toml:"name-key"`
	CallerKey      string `mapstructure:"caller-key" json:"callerKey" yaml:"caller-key" toml:"caller-key"`
	LineEnding     string `mapstructure:"line-ending" json:"lineEnding" yaml:"line-ending" toml:"line-ending"`
	StacktraceKey  string `mapstructure:"stacktrace-key" json:"stacktraceKey" yaml:"stacktrace-key" toml:"stacktrace-key"`
	Level          string `mapstructure:"level" json:"level" yaml:"level" toml:"level"`
	EncodeLevel    string `mapstructure:"encode-level" json:"encodeLevel" yaml:"encode-level" toml:"encode-level"`
	Prefix         string `mapstructure:"prefix" json:"prefix" yaml:"prefix" toml:"prefix"`
	TimeFormat     string `mapstructure:"time-format" json:"timeFormat" yaml:"time-format" toml:"time-format"`
	Format         string `mapstructure:"format" json:"format" yaml:"format" toml:"format"`
	LogInTerminal  bool   `mapstructure:"log-in-terminal" json:"logInTerminal" yaml:"log-in-terminal" toml:"log-in-terminal"`
	MaxAge         int    `mapstructure:"max-age" json:"maxAge" yaml:"max-age" toml:"max-age"`
	MaxSize        int    `mapstructure:"max-size" json:"maxSize" yaml:"max-size" toml:"max-size"`
	MaxBackups     int    `mapstructure:"max-backups" json:"maxBackups" yaml:"max-backups" toml:"max-backups"`
	Compress       bool   `mapstructure:"compress" json:"compress" yaml:"compress" toml:"compress"`
	ShowLineNumber bool   `mapstructure:"show-line-number" json:"showLineNumber" yaml:"show-line-number" toml:"show-line-number"`
}

// ToLoggingConfig converts LogConfig to framework/logging.Config.
func (c LogConfig) ToLoggingConfig() logging.Config {
	return logging.Config{
		Director:       c.Director,
		MessageKey:     c.MessageKey,
		LevelKey:       c.LevelKey,
		TimeKey:        c.TimeKey,
		NameKey:        c.NameKey,
		CallerKey:      c.CallerKey,
		LineEnding:     c.LineEnding,
		StacktraceKey:  c.StacktraceKey,
		Level:          c.Level,
		EncodeLevel:    c.EncodeLevel,
		Prefix:         c.Prefix,
		TimeFormat:     c.TimeFormat,
		Format:         c.Format,
		LogInTerminal:  c.LogInTerminal,
		MaxAge:         c.MaxAge,
		MaxSize:        c.MaxSize,
		MaxBackups:     c.MaxBackups,
		Compress:       c.Compress,
		ShowLineNumber: c.ShowLineNumber,
	}
}

type TracingConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
}

type MetricsConfig struct {
	Port string `mapstructure:"port"`
}

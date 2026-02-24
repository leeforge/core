package config

// ApplyDefaults fills in runtime defaults for optional settings.
func ApplyDefaults(cfg *Config) {
	if cfg == nil {
		return
	}
	if cfg.Security.JWTSecret == "" {
		cfg.Security.JWTSecret = "leeforge-example-secret"
	}
	if cfg.Security.TokenExpiry <= 0 {
		cfg.Security.TokenExpiry = 24
	}
	if cfg.Security.RefreshExpiry <= 0 {
		cfg.Security.RefreshExpiry = 72
	}
	if cfg.Cache.Host == "" {
		cfg.Cache.Host = "127.0.0.1"
	}
	if cfg.Cache.Port == "" {
		cfg.Cache.Port = "6379"
	}
	if cfg.Captcha.TTL == "" {
		cfg.Captcha.TTL = "5m"
	}
	if cfg.Captcha.GenerateWindow == "" {
		cfg.Captcha.GenerateWindow = "1m"
	}
	if cfg.Captcha.AttemptWindow == "" {
		cfg.Captcha.AttemptWindow = "5m"
	}
}

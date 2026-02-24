package config

type SecurityConfig struct {
	JWTSecret       string       `mapstructure:"jwt_secret"`
	TokenExpiry     int          `mapstructure:"token_expiry"`
	RefreshExpiry   int          `mapstructure:"refresh_expiry"`
	PasswordCost    int          `mapstructure:"password_cost"`
	EnableRateLimit bool         `mapstructure:"enable_rate_limit"`
	RateLimit       int          `mapstructure:"rate_limit"`
	Cookie          CookieConfig `mapstructure:"cookie"`
}

type CookieConfig struct {
	Secure   bool   `mapstructure:"secure"`
	SameSite string `mapstructure:"same_site"`
	Domain   string `mapstructure:"domain"`
	Path     string `mapstructure:"path"`
}

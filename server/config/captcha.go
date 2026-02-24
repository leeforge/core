package config

type CaptchaConfig struct {
	Enabled        bool       `mapstructure:"enabled"`
	TTL            string     `mapstructure:"ttl"`
	GenerateLimit  int        `mapstructure:"generate_limit"`
	GenerateWindow string     `mapstructure:"generate_window"`
	MaxAttempts    int        `mapstructure:"max_attempts"`
	AttemptWindow  string     `mapstructure:"attempt_window"`
	Math           MathConfig `mapstructure:"math"`
}

type MathConfig struct {
	Width           int `mapstructure:"width"`
	Height          int `mapstructure:"height"`
	NoiseCount      int `mapstructure:"noise_count"`
	ShowLineOptions int `mapstructure:"show_line_options"`
}

package config

import (
	"fmt"
	"os"

	frameconfig "github.com/leeforge/framework/config"
)

func LoadConfig() *Config {
	configPath := "./configs"
	if env := os.Getenv("CONFIG_PATH"); env != "" {
		configPath = env
	}

	opts := frameconfig.DefaultConfigOptions()
	opts.BasePath = configPath
	opts.FileName = "config"
	opts.FileType = "yaml"
	opts.EnvPrefix = "LEEFORGE"
	opts.WatchAble = false
	opts.LoadAll = true

	cfg, err := frameconfig.NewConfig(opts)
	if err != nil {
		panic(fmt.Sprintf("Failed to create config: %v", err))
	}

	var config Config
	if err := cfg.Bind(&config); err != nil {
		panic(fmt.Sprintf("Failed to bind config: %v", err))
	}

	if err := cfg.Validate(); err != nil {
		panic(fmt.Sprintf("Config validation failed: %v", err))
	}

	if err := config.Database.Validate(); err != nil {
		panic(fmt.Sprintf("Config validation failed: %v", err))
	}

	return &config
}

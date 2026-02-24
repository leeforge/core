package config

import "fmt"

type CacheConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

func (c *CacheConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

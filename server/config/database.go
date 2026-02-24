package config

import (
	"fmt"
	"net"
	"strings"
)

type DatabaseConfig struct {
	// URL is kept for backward compatibility; if set, it takes precedence over the fields below.
	URL string `mapstructure:"url"`

	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`
	Params   string `mapstructure:"params"`

	AutoMigrate  bool `mapstructure:"auto_migrate"`
	MaxOpenConns int  `mapstructure:"max_open_conns"`
	MaxIdleConns int  `mapstructure:"max_idle_conns"`
}

func (c DatabaseConfig) DSN() string {
	if c.URL != "" {
		return c.URL
	}

	port := c.Port
	if port == "" {
		port = "5432"
	}

	sslmode := c.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}

	hostPort := c.Host
	if port != "" {
		hostPort = net.JoinHostPort(c.Host, port)
	}

	auth := ""
	if c.Username != "" {
		if c.Password != "" {
			auth = fmt.Sprintf("%s:%s@", c.Username, c.Password)
		} else {
			auth = fmt.Sprintf("%s@", c.Username)
		}
	}

	query := fmt.Sprintf("sslmode=%s", sslmode)
	if c.Params != "" {
		params := strings.TrimSpace(c.Params)
		params = strings.TrimPrefix(params, "?")
		params = strings.TrimPrefix(params, "&")
		if params != "" {
			query = fmt.Sprintf("%s&%s", query, params)
		}
	}

	return fmt.Sprintf("postgres://%s%s/%s?%s", auth, hostPort, c.Name, query)
}

func (c DatabaseConfig) Validate() error {
	if c.URL != "" {
		return nil
	}
	if c.Name == "" {
		return fmt.Errorf("database.name cannot be empty")
	}
	if c.Host == "" {
		return fmt.Errorf("database.host cannot be empty")
	}
	if c.Port == "" {
		return fmt.Errorf("database.port cannot be empty")
	}
	return nil
}

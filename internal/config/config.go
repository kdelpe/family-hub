// Package config loads and validates the app's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
)

// Config grows phase by phase; architecture §6.5 lists every planned variable.
type Config struct {
	DatabaseURL string
	HTTPAddr    string
	AutoMigrate bool
}

// Load reads settings through getenv (os.Getenv outside tests) and reports every problem at once.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL: getenv("DATABASE_URL"),
		HTTPAddr:    getenv("HTTP_ADDR"),
	}
	var errs []error

	// Never echo DATABASE_URL in errors: it contains the password.
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	} else if u, err := url.Parse(cfg.DatabaseURL); err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		errs = append(errs, errors.New("DATABASE_URL must be a postgres:// URL with a host"))
	}

	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	} else if _, port, err := net.SplitHostPort(cfg.HTTPAddr); err != nil || port == "" {
		errs = append(errs, fmt.Errorf("HTTP_ADDR must be host:port or :port, got %q", cfg.HTTPAddr))
	}

	if v := getenv("AUTO_MIGRATE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("AUTO_MIGRATE must be true or false, got %q", v))
		}
		cfg.AutoMigrate = b
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid config: %w", errors.Join(errs...))
	}
	return cfg, nil
}

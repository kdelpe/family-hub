// Package config loads and validates the app's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
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
		DatabaseURL: strings.TrimSpace(getenv("DATABASE_URL")),
		HTTPAddr:    strings.TrimSpace(getenv("HTTP_ADDR")),
	}
	var errs []error

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	} else if !isPostgresURL(cfg.DatabaseURL) {
		errs = append(errs, errors.New("DATABASE_URL must be a postgres:// URL with a host or ?host= socket path"))
	}

	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	} else if !validListenAddr(cfg.HTTPAddr) {
		errs = append(errs, fmt.Errorf("HTTP_ADDR must be host:port or :port with a port from 1 to 65535, got %q", cfg.HTTPAddr))
	}

	switch v := strings.TrimSpace(getenv("AUTO_MIGRATE")); v {
	case "", "false":
	case "true":
		cfg.AutoMigrate = true
	default:
		errs = append(errs, fmt.Errorf("AUTO_MIGRATE must be true or false, got %q", v))
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid config: %w", errors.Join(errs...))
	}
	return cfg, nil
}

// URL form only: golang-migrate can't take keyword/value strings. url.Parse errors are
// never returned because they quote the raw URL, password included.
func isPostgresURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return false
	}
	return u.Host != "" || u.Query().Get("host") != ""
}

func validListenAddr(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}

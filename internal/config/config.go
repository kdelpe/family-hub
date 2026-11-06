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

	autoMigrate, err := parseBool("AUTO_MIGRATE", getenv("AUTO_MIGRATE"))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.AutoMigrate = autoMigrate

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid config: %w", errors.Join(errs...))
	}
	return cfg, nil
}

// URL form only, one format for the pool and migrations. The prefix check is case-sensitive
// like pgx's. url.Parse errors are never returned: they quote the raw URL, password included.
func isPostgresURL(raw string) bool {
	if !strings.HasPrefix(raw, "postgres://") && !strings.HasPrefix(raw, "postgresql://") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	// u.Query() silently drops badly encoded parameters, so parse strictly.
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return false
	}
	return u.Hostname() != "" || query.Get("host") != ""
}

func validListenAddr(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	// Round-trip so "+8080" and "08080" fail here rather than at net.Listen.
	n, err := strconv.Atoi(port)
	return err == nil && strconv.Itoa(n) == port && n >= 1 && n <= 65535
}

// parseBool takes true or false in any case; unset means false.
func parseBool(name, raw string) (bool, error) {
	switch v := strings.ToLower(strings.TrimSpace(raw)); v {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be true or false, got %q", name, raw)
	}
}

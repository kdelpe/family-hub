// Package config loads and validates the app's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
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

	// pgx decides validity so we accept exactly what the driver does. Its error is dropped:
	// it masks passwords in URLs but echoes input it can't parse at all.
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	} else if _, err := pgxpool.ParseConfig(cfg.DatabaseURL); err != nil {
		errs = append(errs, errors.New("DATABASE_URL is not a valid Postgres connection string (percent-encode special characters in the password)"))
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

func validListenAddr(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}

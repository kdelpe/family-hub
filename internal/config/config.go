// Package config loads and validates the app's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net"
	netmail "net/mail"
	"net/url"
	"strconv"
	"strings"

	"github.com/kdelpe/family-hub/internal/mail"
)

const (
	defaultMailFrom = "Family Hub <noreply@family.example.com>"
	implicitTLSPort = "465"
	maxHostLen      = 253
	maxLabelLen     = 63
)

type Config struct {
	DatabaseURL  string
	HTTPAddr     string
	AutoMigrate  bool
	CookieSecure bool
	TrustedProxy bool
	// Only serve loads it (LoadSMTP), so a mail mistake never blocks migrate up or admin create.
	SMTP mail.SMTPConfig
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

	autoMigrate, err := parseBool("AUTO_MIGRATE", getenv("AUTO_MIGRATE"), false)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.AutoMigrate = autoMigrate

	cookieSecure, err := parseBool("COOKIE_SECURE", getenv("COOKIE_SECURE"), true)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.CookieSecure = cookieSecure

	trustedProxy, err := parseBool("TRUSTED_PROXY", getenv("TRUSTED_PROXY"), false)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.TrustedProxy = trustedProxy

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

// LoadSMTP reads the mail settings. Host and port have no default, so a server deployed without
// them fails at startup, not at its first invite. Errors never quote a value: it may hold a secret.
func LoadSMTP(getenv func(string) string) (mail.SMTPConfig, error) {
	cfg := mail.SMTPConfig{
		Host:     strings.ToLower(strings.TrimSpace(getenv("SMTP_HOST"))),
		Port:     strings.TrimSpace(getenv("SMTP_PORT")),
		Username: strings.TrimSpace(getenv("SMTP_USER")),
		Password: getenv("SMTP_PASS"),
	}
	var errs []error
	switch {
	case cfg.Host == "":
		errs = append(errs, errors.New("SMTP_HOST is required"))
	case net.ParseIP(cfg.Host) == nil && !hostName(cfg.Host):
		errs = append(errs, errors.New("SMTP_HOST must be a bare host name or IP address, without scheme, credentials or port"))
	}
	switch {
	case cfg.Port == "":
		errs = append(errs, errors.New("SMTP_PORT is required"))
	case cfg.Port == implicitTLSPort:
		errs = append(errs, errors.New("SMTP_PORT 465 (implicit TLS) isn't supported; use the provider's STARTTLS port, usually 587"))
	case !validListenAddr(":" + cfg.Port):
		errs = append(errs, errors.New("SMTP_PORT must be a port from 1 to 65535"))
	}
	if (cfg.Username == "") != (cfg.Password == "") {
		errs = append(errs, errors.New("SMTP_USER and SMTP_PASS must be set together"))
	}
	rawFrom := strings.TrimSpace(getenv("MAIL_FROM"))
	if rawFrom == "" {
		rawFrom = defaultMailFrom
	}
	from, err := netmail.ParseAddress(rawFrom)
	if err != nil {
		errs = append(errs, errors.New(`MAIL_FROM must be an address like "Family Hub <noreply@family.example.com>"`))
	} else {
		cfg.From = *from
	}
	if len(errs) > 0 {
		return mail.SMTPConfig{}, fmt.Errorf("invalid mail config: %w", errors.Join(errs...))
	}
	return cfg, nil
}

// A DNS name (RFC 1123): dot-separated labels of lowercase letters, digits and inner hyphens.
func hostName(host string) bool {
	if len(host) > maxHostLen {
		return false
	}
	for label := range strings.SplitSeq(host, ".") {
		if label == "" || len(label) > maxLabelLen || label[0] == '-' || label[len(label)-1] == '-' ||
			strings.Trim(label, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" {
			return false
		}
	}
	return true
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

func parseBool(name, raw string, unset bool) (bool, error) {
	switch v := strings.ToLower(strings.TrimSpace(raw)); v {
	case "":
		return unset, nil
	case "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be true or false, got %q", name, raw)
	}
}

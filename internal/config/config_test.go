package config

import (
	netmail "net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/mail"
)

const validDatabaseURL = "postgres://app:secret@localhost:5432/familyhub?sslmode=disable"

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		want        Config
		wantErrMsgs []string
	}{
		{
			name: "defaults",
			env:  map[string]string{"DATABASE_URL": validDatabaseURL},
			want: Config{DatabaseURL: validDatabaseURL, HTTPAddr: ":8080", CookieSecure: true},
		},
		{
			name: "all set",
			env:  map[string]string{"DATABASE_URL": validDatabaseURL, "HTTP_ADDR": "127.0.0.1:9000", "AUTO_MIGRATE": "true"},
			want: Config{DatabaseURL: validDatabaseURL, HTTPAddr: "127.0.0.1:9000", AutoMigrate: true, CookieSecure: true},
		},
		{
			name: "postgresql scheme",
			env:  map[string]string{"DATABASE_URL": "postgresql://app@db/familyhub"},
			want: Config{DatabaseURL: "postgresql://app@db/familyhub", HTTPAddr: ":8080", CookieSecure: true},
		},
		{
			name: "unix socket URL",
			env:  map[string]string{"DATABASE_URL": "postgres:///familyhub?host=/var/run/postgresql"},
			want: Config{DatabaseURL: "postgres:///familyhub?host=/var/run/postgresql", HTTPAddr: ":8080", CookieSecure: true},
		},
		{
			name: "values are trimmed",
			env:  map[string]string{"DATABASE_URL": " " + validDatabaseURL + "\r", "HTTP_ADDR": " :9000 "},
			want: Config{DatabaseURL: validDatabaseURL, HTTPAddr: ":9000", CookieSecure: true},
		},
		{
			name: "AUTO_MIGRATE with surrounding spaces",
			env:  map[string]string{"DATABASE_URL": validDatabaseURL, "AUTO_MIGRATE": " true "},
			want: Config{DatabaseURL: validDatabaseURL, HTTPAddr: ":8080", AutoMigrate: true, CookieSecure: true},
		},
		{
			name: "AUTO_MIGRATE any case",
			env:  map[string]string{"DATABASE_URL": validDatabaseURL, "AUTO_MIGRATE": "True"},
			want: Config{DatabaseURL: validDatabaseURL, HTTPAddr: ":8080", AutoMigrate: true, CookieSecure: true},
		},
		{
			name: "AUTO_MIGRATE false",
			env:  map[string]string{"DATABASE_URL": validDatabaseURL, "AUTO_MIGRATE": "false"},
			want: Config{DatabaseURL: validDatabaseURL, HTTPAddr: ":8080", CookieSecure: true},
		},
		{
			name: "TRUSTED_PROXY behind Caddy",
			env:  map[string]string{"DATABASE_URL": validDatabaseURL, "TRUSTED_PROXY": "true"},
			want: Config{DatabaseURL: validDatabaseURL, HTTPAddr: ":8080", CookieSecure: true, TrustedProxy: true},
		},
		{
			name: "COOKIE_SECURE false for local http",
			env:  map[string]string{"DATABASE_URL": validDatabaseURL, "COOKIE_SECURE": "false"},
			want: Config{DatabaseURL: validDatabaseURL, HTTPAddr: ":8080"},
		},
		{
			name:        "missing DATABASE_URL",
			env:         map[string]string{},
			wantErrMsgs: []string{"DATABASE_URL is required"},
		},
		{
			name:        "DATABASE_URL wrong scheme",
			env:         map[string]string{"DATABASE_URL": "mysql://app:secret@localhost/familyhub"},
			wantErrMsgs: []string{"DATABASE_URL must be a postgres:// URL"},
		},
		{
			name:        "DATABASE_URL without host",
			env:         map[string]string{"DATABASE_URL": "postgres:///familyhub"},
			wantErrMsgs: []string{"DATABASE_URL must be a postgres:// URL"},
		},
		{
			name:        "DATABASE_URL uppercase scheme",
			env:         map[string]string{"DATABASE_URL": "POSTGRES://app:secret@localhost/familyhub"},
			wantErrMsgs: []string{"DATABASE_URL must be a postgres:// URL"},
		},
		{
			name:        "DATABASE_URL port without hostname",
			env:         map[string]string{"DATABASE_URL": "postgres://app:secret@:5432/familyhub"},
			wantErrMsgs: []string{"DATABASE_URL must be a postgres:// URL"},
		},
		{
			name:        "DATABASE_URL badly encoded query",
			env:         map[string]string{"DATABASE_URL": "postgres://db/familyhub?sslmode=verify%ZZ"},
			wantErrMsgs: []string{"DATABASE_URL must be a postgres:// URL"},
		},
		{
			name:        "DATABASE_URL socket path with badly encoded query",
			env:         map[string]string{"DATABASE_URL": "postgres:///familyhub?host=/var/run/postgresql&sslmode=%ZZ"},
			wantErrMsgs: []string{"DATABASE_URL must be a postgres:// URL"},
		},
		{
			name:        "DATABASE_URL keyword/value form",
			env:         map[string]string{"DATABASE_URL": "host=db user=app dbname=familyhub"},
			wantErrMsgs: []string{"DATABASE_URL must be a postgres:// URL"},
		},
		{
			name:        "HTTP_ADDR without port",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "HTTP_ADDR": "localhost"},
			wantErrMsgs: []string{`HTTP_ADDR must be host:port or :port with a port from 1 to 65535, got "localhost"`},
		},
		{
			name:        "HTTP_ADDR with empty port",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "HTTP_ADDR": "localhost:"},
			wantErrMsgs: []string{"HTTP_ADDR must be"},
		},
		{
			name:        "HTTP_ADDR port out of range",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "HTTP_ADDR": ":99999"},
			wantErrMsgs: []string{"HTTP_ADDR must be"},
		},
		{
			name:        "HTTP_ADDR port not a number",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "HTTP_ADDR": ":http"},
			wantErrMsgs: []string{"HTTP_ADDR must be"},
		},
		{
			name:        "HTTP_ADDR port with sign",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "HTTP_ADDR": ":+8080"},
			wantErrMsgs: []string{"HTTP_ADDR must be"},
		},
		{
			name:        "HTTP_ADDR port with leading zero",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "HTTP_ADDR": ":08080"},
			wantErrMsgs: []string{"HTTP_ADDR must be"},
		},
		{
			name:        "HTTP_ADDR port zero",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "HTTP_ADDR": ":0"},
			wantErrMsgs: []string{"HTTP_ADDR must be"},
		},
		{
			name:        "AUTO_MIGRATE not a bool",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "AUTO_MIGRATE": "yes"},
			wantErrMsgs: []string{`AUTO_MIGRATE must be true or false, got "yes"`},
		},
		{
			name:        "AUTO_MIGRATE 1 is rejected",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "AUTO_MIGRATE": "1"},
			wantErrMsgs: []string{`AUTO_MIGRATE must be true or false, got "1"`},
		},
		{
			name:        "TRUSTED_PROXY not a bool",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "TRUSTED_PROXY": "caddy"},
			wantErrMsgs: []string{`TRUSTED_PROXY must be true or false, got "caddy"`},
		},
		{
			name:        "COOKIE_SECURE not a bool",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "COOKIE_SECURE": "no"},
			wantErrMsgs: []string{`COOKIE_SECURE must be true or false, got "no"`},
		},
		{
			name:        "reports every problem at once",
			env:         map[string]string{"HTTP_ADDR": "nope", "AUTO_MIGRATE": "maybe"},
			wantErrMsgs: []string{"DATABASE_URL is required", "HTTP_ADDR must be", "AUTO_MIGRATE must be"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(func(key string) string { return tt.env[key] })

			if len(tt.wantErrMsgs) == 0 {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
				return
			}
			require.Error(t, err)
			for _, msg := range tt.wantErrMsgs {
				assert.Contains(t, err.Error(), msg)
			}
			assert.Equal(t, Config{}, got)
		})
	}
}

func TestLoadNeverEchoesDatabaseURL(t *testing.T) {
	tests := []struct {
		name        string
		databaseURL string
	}{
		{name: "wrong scheme", databaseURL: "mysql://app:hunter2@localhost/familyhub"},
		{name: "keyword/value form", databaseURL: "host=db user=app password=hunter2"},
		{name: "unparseable URL", databaseURL: "postgres://app:hunter2@local host/familyhub"},
		{name: "badly encoded query", databaseURL: "postgres://app:hunter2@db/familyhub?sslmode=%ZZ"},
		{name: "not a connection string at all", databaseURL: "hunter2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(func(key string) string {
				if key == "DATABASE_URL" {
					return tt.databaseURL
				}
				return ""
			})

			require.Error(t, err)
			assert.NotContains(t, err.Error(), "hunter2")
		})
	}
}

func TestLoadSMTP(t *testing.T) {
	defaultFrom := netmail.Address{Name: "Family Hub", Address: "noreply@family.example.com"}
	tests := []struct {
		name        string
		env         map[string]string
		want        mail.SMTPConfig
		wantErrMsgs []string
	}{
		{
			name: "local Mailpit",
			env:  map[string]string{"SMTP_HOST": "localhost", "SMTP_PORT": "1025"},
			want: mail.SMTPConfig{Host: "localhost", Port: "1025", From: defaultFrom},
		},
		{
			name: "a provider",
			env: map[string]string{
				"SMTP_HOST": " smtp.example.com ", "SMTP_PORT": "587",
				"SMTP_USER": "resend", "SMTP_PASS": " pass with spaces ", "MAIL_FROM": "Family <hello@family.example.com>",
			},
			want: mail.SMTPConfig{
				Host: "smtp.example.com", Port: "587", Username: "resend", Password: " pass with spaces ",
				From: netmail.Address{Name: "Family", Address: "hello@family.example.com"},
			},
		},
		{
			name: "host in capitals is lowercased",
			env:  map[string]string{"SMTP_HOST": "LocalHost", "SMTP_PORT": "1025"},
			want: mail.SMTPConfig{Host: "localhost", Port: "1025", From: defaultFrom},
		},
		{
			name: "IPv6 literal",
			env:  map[string]string{"SMTP_HOST": "::1", "SMTP_PORT": "1025"},
			want: mail.SMTPConfig{Host: "::1", Port: "1025", From: defaultFrom},
		},
		{
			name:        "nothing set",
			env:         map[string]string{},
			wantErrMsgs: []string{"SMTP_HOST is required", "SMTP_PORT is required"},
		},
		{
			name:        "user without password",
			env:         map[string]string{"SMTP_HOST": "localhost", "SMTP_PORT": "1025", "SMTP_USER": "resend"},
			wantErrMsgs: []string{"SMTP_USER and SMTP_PASS must be set together"},
		},
		{
			name:        "password without user",
			env:         map[string]string{"SMTP_HOST": "localhost", "SMTP_PORT": "1025", "SMTP_PASS": "x"},
			wantErrMsgs: []string{"SMTP_USER and SMTP_PASS must be set together"},
		},
		{
			name:        "port not a number",
			env:         map[string]string{"SMTP_HOST": "localhost", "SMTP_PORT": "smtp"},
			wantErrMsgs: []string{"SMTP_PORT must be a port"},
		},
		{
			name:        "port 465",
			env:         map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "465"},
			wantErrMsgs: []string{"SMTP_PORT 465 (implicit TLS) isn't supported"},
		},
		{
			name:        "host is a URL",
			env:         map[string]string{"SMTP_HOST": "smtp://smtp.example.com", "SMTP_PORT": "587"},
			wantErrMsgs: []string{"SMTP_HOST must be a bare host name"},
		},
		{
			name:        "host with a port",
			env:         map[string]string{"SMTP_HOST": "smtp.example.com:587", "SMTP_PORT": "587"},
			wantErrMsgs: []string{"SMTP_HOST must be a bare host name"},
		},
		{
			name:        "host with an underscore",
			env:         map[string]string{"SMTP_HOST": "smtp_example", "SMTP_PORT": "587"},
			wantErrMsgs: []string{"SMTP_HOST must be a bare host name"},
		},
		{
			name:        "host of dots",
			env:         map[string]string{"SMTP_HOST": "..", "SMTP_PORT": "587"},
			wantErrMsgs: []string{"SMTP_HOST must be a bare host name"},
		},
		{
			name: "longest host",
			env:  map[string]string{"SMTP_HOST": strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61), "SMTP_PORT": "587"},
			want: mail.SMTPConfig{Host: strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61), Port: "587", From: defaultFrom},
		},
		{
			name:        "host one too long",
			env:         map[string]string{"SMTP_HOST": strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 62), "SMTP_PORT": "587"},
			wantErrMsgs: []string{"SMTP_HOST must be a bare host name"},
		},
		{
			name:        "label too long",
			env:         map[string]string{"SMTP_HOST": strings.Repeat("a", 64) + ".example.com", "SMTP_PORT": "587"},
			wantErrMsgs: []string{"SMTP_HOST must be a bare host name"},
		},
		{
			name:        "label starting with a hyphen",
			env:         map[string]string{"SMTP_HOST": "-smtp.example.com", "SMTP_PORT": "587"},
			wantErrMsgs: []string{"SMTP_HOST must be a bare host name"},
		},
		{
			name:        "label ending with a hyphen",
			env:         map[string]string{"SMTP_HOST": "smtp-.example.com", "SMTP_PORT": "587"},
			wantErrMsgs: []string{"SMTP_HOST must be a bare host name"},
		},
		{
			name:        "host with an empty label",
			env:         map[string]string{"SMTP_HOST": "smtp..example.com", "SMTP_PORT": "587"},
			wantErrMsgs: []string{"SMTP_HOST must be a bare host name"},
		},
		{
			name:        "MAIL_FROM not an address",
			env:         map[string]string{"SMTP_HOST": "localhost", "SMTP_PORT": "1025", "MAIL_FROM": "Family Hub"},
			wantErrMsgs: []string{"MAIL_FROM must be an address"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadSMTP(func(key string) string { return tt.env[key] })

			if len(tt.wantErrMsgs) == 0 {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
				return
			}
			require.Error(t, err)
			for _, msg := range tt.wantErrMsgs {
				assert.Contains(t, err.Error(), msg)
			}
			assert.Equal(t, mail.SMTPConfig{}, got)
		})
	}
}

func TestLoadSMTPNeverEchoesSecrets(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "credentials pasted into SMTP_HOST", env: map[string]string{"SMTP_HOST": "resend:hunter2@smtp.example.com", "SMTP_PORT": "587"}},
		{name: "password with a bad port", env: map[string]string{"SMTP_HOST": "localhost", "SMTP_USER": "resend", "SMTP_PASS": "hunter2", "SMTP_PORT": "x"}},
		{name: "password without user", env: map[string]string{"SMTP_HOST": "localhost", "SMTP_PORT": "1025", "SMTP_PASS": "hunter2"}},
		{name: "password pasted into MAIL_FROM", env: map[string]string{"SMTP_HOST": "localhost", "SMTP_PORT": "1025", "MAIL_FROM": "hunter2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadSMTP(func(key string) string { return tt.env[key] })

			require.Error(t, err)
			assert.NotContains(t, err.Error(), "hunter2")
		})
	}
}

func TestLoadBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{name: "https origin", raw: "https://family.example.com", want: "https://family.example.com"},
		{name: "local dev with a port", raw: "http://localhost:5173", want: "http://localhost:5173"},
		{name: "trailing slash dropped", raw: " https://family.example.com/ ", want: "https://family.example.com"},
		{name: "scheme and host lowercased", raw: "HTTPS://Family.Example.COM", want: "https://family.example.com"},
		{name: "an IPv4 address", raw: "http://192.0.2.10:8080", want: "http://192.0.2.10:8080"},
		{name: "an IPv6 address", raw: "http://[::1]:5173", want: "http://[::1]:5173"},
		{name: "an IPv6 address without a port", raw: "https://[2001:DB8::1]", want: "https://[2001:db8::1]"},
		{name: "an IPv6 address written the browser's way", raw: "http://[2001:db8:0:0::1]:8080", want: "http://[2001:db8::1]:8080"},
		{name: "an IPv4-mapped IPv6 address", raw: "http://[::ffff:192.0.2.1]", wantErr: "BASE_URL must be an origin"},
		{name: "an IPv4 address with leading zeros", raw: "http://192.168.001.010:8080", wantErr: "BASE_URL must be an origin"},
		{name: "a hex IPv4 address", raw: "http://0x7f.0.0.1", wantErr: "BASE_URL must be an origin"},
		{name: "a name that starts with 0x", raw: "http://0x-hub:5173", want: "http://0x-hub:5173"},
		{name: "a scoped IPv6 address", raw: "http://[fe80::1%25eth0]", wantErr: "BASE_URL must be an origin"},
		{name: "a short IPv4 address", raw: "http://1.2.3", wantErr: "BASE_URL must be an origin"},
		{name: "default https port dropped", raw: "https://family.example.com:443", want: "https://family.example.com"},
		{name: "default http port dropped", raw: "http://localhost:80/", want: "http://localhost"},
		{name: "https on port 80 kept", raw: "https://family.example.com:80", want: "https://family.example.com:80"},
		{name: "an empty port", raw: "https://family.example.com:", wantErr: "BASE_URL must be an origin"},
		{name: "a port out of range", raw: "https://family.example.com:99999", wantErr: "BASE_URL must be an origin"},
		{name: "not a host name", raw: "https://family_hub.example.com", wantErr: "BASE_URL must be an origin"},
		{name: "unset", raw: "", wantErr: "BASE_URL is required"},
		{name: "no scheme", raw: "family.example.com", wantErr: "BASE_URL must be an origin"},
		{name: "other scheme", raw: "ftp://family.example.com", wantErr: "BASE_URL must be an origin"},
		{name: "a path", raw: "https://family.example.com/app", wantErr: "BASE_URL must be an origin"},
		{name: "a query", raw: "https://family.example.com/?x=1", wantErr: "BASE_URL must be an origin"},
		{name: "a fragment", raw: "https://family.example.com/#top", wantErr: "BASE_URL must be an origin"},
		{name: "credentials", raw: "https://user:hunter2@family.example.com", wantErr: "BASE_URL must be an origin"},
		{name: "no host", raw: "https://:443", wantErr: "BASE_URL must be an origin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadBaseURL(func(key string) string {
				if key == "BASE_URL" {
					return tt.raw
				}
				return ""
			})

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.NotContains(t, err.Error(), "hunter2")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadTimezone(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "the default", raw: "", want: "America/New_York"},
		{name: "another zone", raw: " Europe/Paris ", want: "Europe/Paris"},
		{name: "UTC", raw: "UTC", want: "UTC"},
		{name: "an unknown name", raw: "Mars/Olympus", wantErr: true},
		{name: "the server's own setting", raw: "Local", wantErr: true},
		{name: "the server's own setting in any case", raw: "local", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := LoadTimezone(func(string) string { return tt.raw })

			if tt.wantErr {
				require.ErrorContains(t, err, "APP_TIMEZONE must be an IANA timezone")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, loc.String())
		})
	}
}

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			want: Config{DatabaseURL: validDatabaseURL, HTTPAddr: ":8080"},
		},
		{
			name: "all set",
			env:  map[string]string{"DATABASE_URL": validDatabaseURL, "HTTP_ADDR": "127.0.0.1:9000", "AUTO_MIGRATE": "true"},
			want: Config{DatabaseURL: validDatabaseURL, HTTPAddr: "127.0.0.1:9000", AutoMigrate: true},
		},
		{
			name: "postgresql scheme",
			env:  map[string]string{"DATABASE_URL": "postgresql://app@db/familyhub"},
			want: Config{DatabaseURL: "postgresql://app@db/familyhub", HTTPAddr: ":8080"},
		},
		{
			name:        "missing DATABASE_URL",
			env:         map[string]string{},
			wantErrMsgs: []string{"DATABASE_URL is required"},
		},
		{
			name:        "DATABASE_URL wrong scheme",
			env:         map[string]string{"DATABASE_URL": "mysql://app:secret@localhost/familyhub"},
			wantErrMsgs: []string{"DATABASE_URL must be a postgres:// URL with a host"},
		},
		{
			name:        "DATABASE_URL without host",
			env:         map[string]string{"DATABASE_URL": "postgres:///familyhub"},
			wantErrMsgs: []string{"DATABASE_URL must be a postgres:// URL with a host"},
		},
		{
			name:        "HTTP_ADDR without port",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "HTTP_ADDR": "localhost"},
			wantErrMsgs: []string{`HTTP_ADDR must be host:port or :port, got "localhost"`},
		},
		{
			name:        "HTTP_ADDR with empty port",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "HTTP_ADDR": "localhost:"},
			wantErrMsgs: []string{`HTTP_ADDR must be host:port or :port, got "localhost:"`},
		},
		{
			name:        "AUTO_MIGRATE not a bool",
			env:         map[string]string{"DATABASE_URL": validDatabaseURL, "AUTO_MIGRATE": "yes"},
			wantErrMsgs: []string{`AUTO_MIGRATE must be true or false, got "yes"`},
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
	_, err := Load(func(key string) string {
		if key == "DATABASE_URL" {
			return "mysql://app:hunter2@localhost/familyhub"
		}
		return ""
	})

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "hunter2")
}

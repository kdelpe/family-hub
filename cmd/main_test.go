package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/admin"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErr    error
		wantErrMsg string
		wantStdout string
	}{
		{name: "no args defaults to serve", args: nil, wantErrMsg: "serve: invalid config: DATABASE_URL is required"},
		{name: "serve needs a config", args: []string{"serve"}, wantErrMsg: "serve: invalid config: DATABASE_URL is required"},
		{name: "migrate needs a config", args: []string{"migrate", "up"}, wantErrMsg: "DATABASE_URL is required"},
		{name: "migrate bad args", args: []string{"migrate", "sideways"}, wantErr: errMigrateUsage, wantErrMsg: "migrate:"},
		{name: "migrate down is not a command", args: []string{"migrate", "down"}, wantErr: errMigrateUsage, wantErrMsg: "migrate:"},
		{name: "admin create bad args", args: []string{"admin", "create"}, wantErr: admin.ErrCreateUsage, wantErrMsg: "admin create:"},
		{name: "admin without a subcommand", args: []string{"admin"}, wantErr: admin.ErrCreateUsage, wantErrMsg: "admin:"},
		{name: "admin unknown subcommand", args: []string{"admin", "delete"}, wantErr: admin.ErrCreateUsage, wantErrMsg: "admin:"},
		{name: "-h prints usage", args: []string{"-h"}, wantStdout: usage},
		{name: "--help prints usage", args: []string{"--help"}, wantStdout: usage},
		{name: "help prints usage", args: []string{"help"}, wantStdout: usage},
		{name: "unknown command", args: []string{"bogus"}, wantErrMsg: `unknown command "bogus"`},
		{name: "unknown flag", args: []string{"-v"}, wantErrMsg: `unknown command "-v"`},
		{name: "empty command", args: []string{""}, wantErrMsg: `unknown command ""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "")
			var stdout bytes.Buffer

			err := run(t.Context(), tt.args, &stdout)

			if tt.wantErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			}
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}
			assert.Equal(t, tt.wantStdout, stdout.String())
		})
	}
}

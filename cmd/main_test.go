package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/kdelpe/family-hub/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErr    error
		wantErrMsg string
		wantStdout string
	}{
		{name: "no args defaults to serve", args: nil, wantErr: errNotImplemented, wantErrMsg: "serve:"},
		{name: "serve", args: []string{"serve"}, wantErr: errNotImplemented, wantErrMsg: "serve:"},
		{name: "migrate needs a config", args: []string{"migrate", "up"}, wantErrMsg: "DATABASE_URL is required"},
		{name: "migrate bad args", args: []string{"migrate", "sideways"}, wantErr: database.ErrMigrateUsage, wantErrMsg: "migrate:"},
		{name: "admin", args: []string{"admin", "create"}, wantErr: errNotImplemented, wantErrMsg: "admin:"},
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

			err := run(tt.args, &stdout)

			if tt.wantErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			}
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}
			if !errors.Is(tt.wantErr, errNotImplemented) {
				assert.NotErrorIs(t, err, errNotImplemented)
			}
			assert.Equal(t, tt.wantStdout, stdout.String())
		})
	}
}

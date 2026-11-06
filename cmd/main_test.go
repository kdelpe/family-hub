package main

import (
	"bytes"
	"testing"

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
		{name: "-h prints usage", args: []string{"-h"}, wantStdout: usage},
		{name: "--help prints usage", args: []string{"--help"}, wantStdout: usage},
		{name: "subcommand -h prints usage", args: []string{"migrate", "-h"}, wantStdout: usage},
		{name: "unknown top-level flag", args: []string{"-v", "migrate", "up"}, wantErrMsg: "flag provided but not defined: -v (see family-hub -h)"},
		{name: "unknown command", args: []string{"bogus"}, wantErrMsg: `unknown command "bogus"`},
		{name: "empty command", args: []string{""}, wantErrMsg: `unknown command ""`},

		{name: "serve", args: []string{"serve"}, wantErr: errNotImplemented, wantErrMsg: "serve:"},
		{name: "serve unknown flag", args: []string{"serve", "--bogus"}, wantErrMsg: "serve: flag provided but not defined: -bogus"},
		{name: "serve extra argument", args: []string{"serve", "now"}, wantErrMsg: `serve: unexpected arguments ["now"]`},

		{name: "migrate up", args: []string{"migrate", "up"}, wantErr: errNotImplemented, wantErrMsg: "migrate:"},
		{name: "migrate down defaults to 1", args: []string{"migrate", "down"}, wantErr: errNotImplemented, wantErrMsg: "migrate:"},
		{name: "migrate down N", args: []string{"migrate", "down", "3"}, wantErr: errNotImplemented, wantErrMsg: "migrate:"},
		{name: "migrate version", args: []string{"migrate", "version"}, wantErr: errNotImplemented, wantErrMsg: "migrate:"},
		{name: "migrate force N", args: []string{"migrate", "force", "2"}, wantErr: errNotImplemented, wantErrMsg: "migrate:"},
		{name: "migrate force -1", args: []string{"migrate", "force", "-1"}, wantErr: errNotImplemented, wantErrMsg: "migrate:"},
		{name: "migrate missing action", args: []string{"migrate"}, wantErrMsg: `migrate: invalid action ""`},
		{name: "migrate unknown action", args: []string{"migrate", "sideways"}, wantErrMsg: `migrate: invalid action "sideways"`},
		{name: "migrate up extra argument", args: []string{"migrate", "up", "2"}, wantErrMsg: `migrate: invalid action "up 2"`},
		{name: "migrate down zero", args: []string{"migrate", "down", "0"}, wantErrMsg: "down: N must be a positive integer"},
		{name: "migrate down not a number", args: []string{"migrate", "down", "x"}, wantErrMsg: "down: N must be a positive integer"},
		{name: "migrate force missing N", args: []string{"migrate", "force"}, wantErrMsg: `migrate: invalid action "force"`},
		{name: "migrate force below -1", args: []string{"migrate", "force", "-2"}, wantErrMsg: "force: N must be an integer"},

		{name: "admin create", args: []string{"admin", "create", "--email", "admin@example.com"}, wantErr: errNotImplemented, wantErrMsg: "admin create:"},
		{name: "admin create without email", args: []string{"admin", "create"}, wantErrMsg: "admin create: --email is required"},
		{name: "admin create blank email", args: []string{"admin", "create", "--email", " "}, wantErrMsg: "admin create: --email is required"},
		{name: "admin create extra argument", args: []string{"admin", "create", "--email", "a@example.com", "x"}, wantErrMsg: "admin create: unexpected arguments"},
		{name: "admin without create", args: []string{"admin"}, wantErrMsg: "admin: want admin create"},
		{name: "admin unknown action", args: []string{"admin", "delete"}, wantErrMsg: "admin: want admin create"},

		{name: "seed", args: []string{"seed"}, wantErr: errNotImplemented, wantErrMsg: "seed:"},
		{name: "seed extra argument", args: []string{"seed", "big"}, wantErrMsg: "seed: unexpected arguments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			} else {
				assert.NotErrorIs(t, err, errNotImplemented)
			}
			assert.Equal(t, tt.wantStdout, stdout.String())
		})
	}
}

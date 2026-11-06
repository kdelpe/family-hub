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
		{name: "flags only defaults to serve", args: []string{"-addr", ":9000"}, wantErr: errNotImplemented, wantErrMsg: "serve:"},
		{name: "serve", args: []string{"serve"}, wantErr: errNotImplemented, wantErrMsg: "serve:"},
		{name: "migrate", args: []string{"migrate", "up"}, wantErr: errNotImplemented, wantErrMsg: "migrate:"},
		{name: "admin", args: []string{"admin", "create"}, wantErr: errNotImplemented, wantErrMsg: "admin:"},
		{name: "seed", args: []string{"seed"}, wantErr: errNotImplemented, wantErrMsg: "seed:"},
		{name: "version prints build version", args: []string{"version"}, wantStdout: "dev\n"},
		{name: "unknown command", args: []string{"bogus"}, wantErrMsg: `unknown command "bogus"`},
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
			}
			assert.Equal(t, tt.wantStdout, stdout.String())
		})
	}
}

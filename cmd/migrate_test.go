package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMigrateArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErrMsg string
	}{
		{name: "up", args: []string{"up"}},
		{name: "down defaults to one step", args: []string{"down"}},
		{name: "down N", args: []string{"down", "3"}},
		{name: "no action", args: nil, wantErrMsg: errMigrateUsage.Error()},
		{name: "unknown action", args: []string{"sideways"}, wantErrMsg: errMigrateUsage.Error()},
		{name: "up with extra argument", args: []string{"up", "2"}, wantErrMsg: errMigrateUsage.Error()},
		{name: "down with two arguments", args: []string{"down", "1", "2"}, wantErrMsg: errMigrateUsage.Error()},
		{name: "down zero", args: []string{"down", "0"}, wantErrMsg: `down: steps must be a positive integer, got "0"`},
		{name: "down not a number", args: []string{"down", "x"}, wantErrMsg: `down: steps must be a positive integer, got "x"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run, err := parseMigrateArgs(tt.args)

			if tt.wantErrMsg == "" {
				require.NoError(t, err)
				assert.NotNil(t, run)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tt.wantErrMsg, err.Error())
			assert.Nil(t, run)
		})
	}
}

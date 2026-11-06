package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMigrateCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		want       MigrateCommand
		wantErrMsg string
	}{
		{name: "up", args: []string{"up"}, want: MigrateCommand{}},
		{name: "down defaults to one step", args: []string{"down"}, want: MigrateCommand{down: true, steps: 1}},
		{name: "down N", args: []string{"down", "3"}, want: MigrateCommand{down: true, steps: 3}},
		{name: "no action", args: nil, wantErrMsg: ErrMigrateUsage.Error()},
		{name: "unknown action", args: []string{"sideways"}, wantErrMsg: ErrMigrateUsage.Error()},
		{name: "up with extra argument", args: []string{"up", "2"}, wantErrMsg: ErrMigrateUsage.Error()},
		{name: "down with two arguments", args: []string{"down", "1", "2"}, wantErrMsg: ErrMigrateUsage.Error()},
		{name: "down zero", args: []string{"down", "0"}, wantErrMsg: `down: steps must be a positive integer, got "0"`},
		{name: "down not a number", args: []string{"down", "x"}, wantErrMsg: `down: steps must be a positive integer, got "x"`},
		{name: "down with sign", args: []string{"down", "+3"}, wantErrMsg: `down: steps must be a positive integer, got "+3"`},
		{name: "down with leading zero", args: []string{"down", "007"}, wantErrMsg: `down: steps must be a positive integer, got "007"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMigrateCommand(tt.args)

			if tt.wantErrMsg == "" {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tt.wantErrMsg, err.Error())
		})
	}
}

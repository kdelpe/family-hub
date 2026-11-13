package admin

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLineReader(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLines []string
		wantErr   error
	}{
		{name: "enter in raw mode", input: "pw one\r", wantLines: []string{"pw one"}},
		{name: "newline", input: "pw one\n", wantLines: []string{"pw one"}},
		{name: "two lines pasted at once", input: "pw one\r\npw two\r\n", wantLines: []string{"pw one", "pw two"}},
		{name: "an empty line is kept", input: "\r\rpw\r", wantLines: []string{"", "", "pw"}},
		{name: "tab and unicode kept", input: "a\tb café\r", wantLines: []string{"a\tb café"}},
		{name: "backspace removes a character", input: "pwx\x7f\r", wantLines: []string{"pw"}},
		{name: "backspace removes a whole rune", input: "café\b\r", wantLines: []string{"caf"}},
		{name: "backspace on empty", input: "\x7fpw\r", wantLines: []string{"pw"}},
		{name: "ctrl-c aborts", input: "pw\x03", wantErr: errAborted},
		{name: "ctrl-d aborts", input: "\x04", wantErr: errAborted},
		{name: "end of input aborts", input: "pw", wantErr: errAborted},
		{name: "arrow key refused", input: "pwx\x1b[Dy\r", wantErr: errEditingKey},
		{name: "ctrl-u refused", input: "pw\x15\r", wantErr: errEditingKey},
		{name: "invalid UTF-8 refused", input: "p\xe4ss\r", wantErr: errInvalidUTF8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lineReader{r: bufio.NewReader(strings.NewReader(tt.input))}

			for _, want := range tt.wantLines {
				got, err := l.read()
				require.NoError(t, err)
				assert.Equal(t, want, got)
			}
			if tt.wantErr != nil {
				_, err := l.read()
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

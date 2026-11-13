package admin

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"unicode"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/kdelpe/family-hub/internal/auth"
)

const (
	keyCtrlC     = 3
	keyCtrlD     = 4
	keyCtrlZ     = 26
	keyCtrlQuit  = 28
	keyBackspace = 8
	keyDelete    = 127
	// Far above the password limit, which CheckNewPassword applies; this only bounds a huge paste.
	maxLineRunes = 1024
)

var (
	errAborted     = errors.New("aborted")
	errEditingKey  = errors.New("only typing and Backspace work at the password prompt")
	errInvalidUTF8 = errors.New("the terminal sent text that isn't UTF-8")
)

type stdinTerminal struct {
	in  *os.File
	out *os.File
	// Shared by every prompt, so input typed or pasted past one line waits for the next.
	lines *lineReader
	raw   *term.State
}

// StdinTerminal reads from standard input and prompts on standard error, keeping stdout clean.
func StdinTerminal() Terminal {
	return &stdinTerminal{in: os.Stdin, out: os.Stderr, lines: &lineReader{r: bufio.NewReader(os.Stdin)}}
}

func (t *stdinTerminal) IsTerminal() bool {
	return term.IsTerminal(int(t.in.Fd())) && term.IsTerminal(int(t.out.Fd()))
}

func (t *stdinTerminal) ReadPassword(ctx context.Context, prompt string) (string, error) {
	// Raw mode turns off echo and makes Ctrl-C a key. It stays on until Restore, so nothing typed
	// between two prompts is echoed.
	if t.raw == nil {
		state, err := term.MakeRaw(int(t.in.Fd()))
		if err != nil {
			return "", err
		}
		t.raw = state
	}
	_, _ = io.WriteString(t.out, prompt)

	type result struct {
		password string
		err      error
	}
	done := make(chan result, 1)
	go func() {
		password, err := t.lines.read()
		done <- result{password, err}
	}()
	var r result
	select {
	case r = <-done:
	case <-ctx.Done():
		r.err = ctx.Err()
	}
	_, _ = io.WriteString(t.out, "\r\n")
	return r.password, r.err
}

func (t *stdinTerminal) Restore() error {
	if t.raw == nil {
		return nil
	}
	state := t.raw
	t.raw = nil
	return term.Restore(int(t.in.Fd()), state)
}

// lineReader refuses editing keys like arrows, which would land in the hidden password unseen,
// and is shared by both prompts so a pasted second line isn't lost.
type lineReader struct {
	r       *bufio.Reader
	afterCR bool
}

func (l *lineReader) read() (string, error) {
	var line []rune
	for {
		c, size, err := l.r.ReadRune()
		if errors.Is(err, io.EOF) {
			return "", errAborted
		}
		if err != nil {
			return "", err
		}
		skipLF := l.afterCR && c == '\n'
		l.afterCR = c == '\r'
		switch {
		case skipLF:
		case c == '\r' || c == '\n':
			return string(line), nil
		case c == keyCtrlC || c == keyCtrlD || c == keyCtrlZ || c == keyCtrlQuit:
			return "", errAborted
		case c == keyBackspace || c == keyDelete:
			if len(line) > 0 {
				line = line[:len(line)-1]
			}
		case c == utf8.RuneError && size == 1:
			return "", errInvalidUTF8
		case c != '\t' && unicode.IsControl(c):
			return "", errEditingKey
		case len(line) >= maxLineRunes:
			return "", auth.ErrPasswordTooLong
		default:
			line = append(line, c)
		}
	}
}

package admin

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/term"
)

const (
	keyCtrlC     = 3
	keyCtrlD     = 4
	keyBackspace = 8
	keyDelete    = 127
)

var errAborted = errors.New("aborted")

type stdinTerminal struct {
	in  *os.File
	out *os.File
	// Shared by every prompt, so input typed or pasted past one line waits for the next.
	lines *lineReader
}

// StdinTerminal reads from standard input and prompts on standard error, keeping stdout clean.
func StdinTerminal() Terminal {
	return &stdinTerminal{in: os.Stdin, out: os.Stderr, lines: &lineReader{r: bufio.NewReader(os.Stdin)}}
}

func (t *stdinTerminal) IsTerminal() bool {
	return term.IsTerminal(int(t.in.Fd())) && term.IsTerminal(int(t.out.Fd()))
}

func (t *stdinTerminal) ReadPassword(ctx context.Context, prompt string) (string, error) {
	// Raw mode turns off echo and makes Ctrl-C a key, rather than a SIGINT main would only turn into a cancel.
	fd := int(t.in.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer func() { _ = term.Restore(fd, state) }()
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
		// Not ctx.Err(): main treats a canceled command as a clean stop, and no admin was created.
		r.err = errAborted
	}
	_, _ = io.WriteString(t.out, "\r\n")
	return r.password, r.err
}

// lineReader reads one line of raw-mode input. Unlike x/term's line editor, it keeps every
// character, such as a Tab in a pasted password, which the web form would send too.
type lineReader struct {
	r       *bufio.Reader
	afterCR bool
}

func (l *lineReader) read() (string, error) {
	var line []rune
	for {
		c, _, err := l.r.ReadRune()
		if errors.Is(err, io.EOF) {
			return "", errAborted
		}
		if err != nil {
			return "", err
		}
		skipLF := l.afterCR && c == '\n' && len(line) == 0
		l.afterCR = c == '\r'
		switch {
		case skipLF:
		case c == '\r' || c == '\n':
			return string(line), nil
		case c == keyCtrlC || c == keyCtrlD:
			return "", errAborted
		case c == keyBackspace || c == keyDelete:
			if len(line) > 0 {
				line = line[:len(line)-1]
			}
		default:
			line = append(line, c)
		}
	}
}

package admin

import (
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/term"
)

var ErrAborted = errors.New("aborted")

type stdinTerminal struct {
	in  *os.File
	out io.Writer
	// One for every prompt: it buffers what was typed or pasted past the line it returned.
	lines *term.Terminal
}

// StdinTerminal reads from standard input and prompts on standard error, keeping stdout clean.
func StdinTerminal() Terminal {
	return stdinTerminal{in: os.Stdin, out: os.Stderr, lines: term.NewTerminal(struct {
		io.Reader
		io.Writer
	}{os.Stdin, os.Stderr}, "")}
}

func (t stdinTerminal) IsTerminal() bool {
	return term.IsTerminal(t.fd())
}

func (t stdinTerminal) ReadPassword(ctx context.Context, prompt string) (string, error) {
	// Raw mode is set here, not in the reader goroutine, so the deferred restore can't race it.
	// It also turns Ctrl-C into a key that ends the read, rather than a SIGINT nothing would see.
	state, err := term.MakeRaw(t.fd())
	if err != nil {
		return "", err
	}
	defer func() { _ = term.Restore(t.fd(), state) }()

	type result struct {
		password string
		err      error
	}
	done := make(chan result, 1)
	go func() {
		password, err := t.lines.ReadPassword(prompt)
		done <- result{password, err}
	}()
	select {
	case r := <-done:
		if errors.Is(r.err, io.EOF) {
			_, _ = io.WriteString(t.out, "\r\n")
			return "", ErrAborted
		}
		return r.password, r.err
	case <-ctx.Done():
		_, _ = io.WriteString(t.out, "\r\n")
		return "", ctx.Err()
	}
}

func (t stdinTerminal) fd() int {
	return int(t.in.Fd())
}

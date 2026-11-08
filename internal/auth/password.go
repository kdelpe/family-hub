// Package auth holds sign-in logic: passwords now, then tokens, sessions and Google sign-in (architecture §5).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 128
	// With the current 19 MiB per hash, this caps password work at about 76 MiB.
	maxConcurrentHashes = 4
	hashWait            = 2 * time.Second
	saltLen             = 16
	keyLen              = 32
	// Bounds on a stored hash's parameters, so no hash can make a verify take gigabytes or minutes.
	maxMemoryKiB = 256 * 1024
	maxTime      = 10
	maxThreads   = 8
	maxKeyLen    = 64
)

// OWASP's minimum for Argon2id: 19 MiB, 2 iterations, 1 thread.
var currentParams = params{memory: 19 * 1024, time: 2, threads: 1}

var (
	ErrPasswordTooShort  = fmt.Errorf("password must be at least %d characters", minPasswordLen)
	ErrPasswordTooLong   = fmt.Errorf("password must be at most %d characters", maxPasswordLen)
	ErrPasswordTooCommon = errors.New("password is too common")
	// ErrBusy means every hash slot stayed taken for hashWait; the caller answers 503.
	ErrBusy = errors.New("too many password checks at once")

	errMalformedHash = errors.New("malformed password hash")
)

//go:embed common_passwords.txt
var commonPasswordList string

var commonPasswords = sync.OnceValue(func() map[string]struct{} {
	set := make(map[string]struct{})
	header := true
	for line := range strings.Lines(commonPasswordList) {
		line = strings.TrimSuffix(line, "\n")
		// Only the leading lines are comments; an entry may itself start with "# ".
		if header = header && strings.HasPrefix(line, "# "); !header && line != "" {
			set[line] = struct{}{}
		}
	}
	return set
})

type params struct {
	memory  uint32
	time    uint32
	threads uint8
}

// CheckNewPassword applies the rules for a password being set; existing ones are only verified.
func CheckNewPassword(password, email string) error {
	password = norm.NFC.String(password)
	switch n := utf8.RuneCountInString(password); {
	case n < minPasswordLen:
		return ErrPasswordTooShort
	case n > maxPasswordLen:
		return ErrPasswordTooLong
	}
	lower := strings.ToLower(password)
	email = strings.ToLower(strings.TrimSpace(norm.NFC.String(email)))
	local, _, _ := strings.Cut(email, "@")
	if _, ok := commonPasswords()[lower]; ok || lower == email || lower == local || lettersOnly(lower) == "familyhub" {
		return ErrPasswordTooCommon
	}
	return nil
}

// "Family-Hub", "family_hub" and "familyhub1" all count as the app's name.
func lettersOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return r
		}
		return -1
	}, s)
}

// Hasher runs Argon2id, at most maxConcurrentHashes at a time.
type Hasher struct {
	slots chan struct{}
}

func NewHasher() *Hasher {
	return &Hasher{slots: make(chan struct{}, maxConcurrentHashes)}
}

// Hash returns a PHC string: $argon2id$v=19$m=…,t=…,p=…$salt$hash.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	return h.hash(ctx, password, currentParams)
}

func (h *Hasher) hash(ctx context.Context, password string, p params) (string, error) {
	if tooLong(password) {
		return "", ErrPasswordTooLong
	}
	salt := make([]byte, saltLen)
	_, _ = rand.Read(salt)
	key, err := h.derive(ctx, password, salt, p, keyLen)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version,
		p.memory, p.time, p.threads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// Verify reports whether password matches encoded, and whether encoded should be replaced
// by a fresh Hash because it was made with other parameters.
func (h *Hasher) Verify(ctx context.Context, encoded, password string) (ok, rehash bool, err error) {
	// No stored password is longer, so it can't match; refused before it takes a slot.
	if tooLong(password) {
		return false, false, nil
	}
	p, salt, want, err := parseHash(encoded)
	if err != nil {
		// Still one Argon2 run, so a broken stored hash answers as slowly as any other failure.
		_, _ = h.derive(ctx, password, make([]byte, saltLen), currentParams, keyLen)
		return false, false, err
	}
	got, err := h.derive(ctx, password, salt, p, uint32(len(want)))
	if err != nil {
		return false, false, err
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, false, nil
	}
	return true, p != currentParams || len(want) != keyLen, nil
}

func (h *Hasher) derive(ctx context.Context, password string, salt []byte, p params, n uint32) ([]byte, error) {
	if err := h.acquire(ctx); err != nil {
		return nil, err
	}
	defer func() { <-h.slots }()
	return argon2.IDKey([]byte(norm.NFC.String(password)), salt, p.time, p.memory, p.threads, n), nil
}

func (h *Hasher) acquire(ctx context.Context) error {
	select {
	case h.slots <- struct{}{}:
		return nil
	default:
	}
	wait := time.NewTimer(hashWait)
	defer wait.Stop()
	select {
	case h.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-wait.C:
		return ErrBusy
	}
}

// The byte check comes first, so megabytes are never normalized; 128 characters fit in 512 bytes.
func tooLong(password string) bool {
	return len(password) > 4*maxPasswordLen || utf8.RuneCountInString(norm.NFC.String(password)) > maxPasswordLen
}

func parseHash(encoded string) (p params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return params{}, nil, nil, errMalformedHash
	}
	var rest string
	if n, _ := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d%s", &p.memory, &p.time, &p.threads, &rest); n != 3 ||
		p.memory == 0 || p.memory > maxMemoryKiB || p.time == 0 || p.time > maxTime || p.threads == 0 || p.threads > maxThreads {
		return params{}, nil, nil, errMalformedHash
	}
	enc := base64.RawStdEncoding
	if salt, err = enc.DecodeString(parts[4]); err != nil {
		return params{}, nil, nil, errMalformedHash
	}
	if key, err = enc.DecodeString(parts[5]); err != nil || len(key) == 0 || len(key) > maxKeyLen {
		return params{}, nil, nil, errMalformedHash
	}
	return p, salt, key, nil
}

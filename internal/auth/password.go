// Package auth handles sign-in (architecture §5).
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
	minPasswordLen      = 8
	maxPasswordLen      = 128
	maxConcurrentHashes = 4
	hashWait            = 2 * time.Second
	saltLen             = 16
	keyLen              = 32
	maxStoredMemoryKiB  = 256 * 1024
	maxStoredTime       = 10
	maxStoredThreads    = 8
	minStoredSaltLen    = 8
	maxStoredSaltLen    = 64
	minStoredKeyLen     = 16
	maxStoredKeyLen     = 64
	maxStoredHashLen    = 256
)

var currentParams = params{memory: 19 * 1024, time: 2, threads: 1}

var (
	ErrPasswordTooShort  = fmt.Errorf("password must be at least %d characters", minPasswordLen)
	ErrPasswordTooLong   = fmt.Errorf("password must be at most %d characters", maxPasswordLen)
	ErrPasswordTooCommon = errors.New("password is too common")
	ErrBusy              = errors.New("too many password checks at once")

	errMalformedHash = errors.New("malformed password hash")
)

//go:embed common_passwords.txt
var commonPasswordList string

var commonPasswords = sync.OnceValue(func() map[string]struct{} {
	set := make(map[string]struct{})
	header := true
	for line := range strings.Lines(commonPasswordList) {
		line = strings.TrimSuffix(line, "\n")
		// Only the leading lines are a header: an entry may itself start with "# ".
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

func CheckNewPassword(password, email string) error {
	if tooLong(password) {
		return ErrPasswordTooLong
	}
	password = norm.NFC.String(password)
	if utf8.RuneCountInString(password) < minPasswordLen {
		return ErrPasswordTooShort
	}
	lower := strings.ToLower(password)
	email = NormalizeEmail(email)
	local, _, _ := strings.Cut(email, "@")
	if _, ok := commonPasswords()[lower]; ok || lower == email || lower == local || lettersOnly(lower) == "familyhub" {
		return ErrPasswordTooCommon
	}
	return nil
}

func lettersOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return r
		}
		return -1
	}, s)
}

type Hasher struct {
	slots chan struct{}
}

func NewHasher() *Hasher {
	return &Hasher{slots: make(chan struct{}, maxConcurrentHashes)}
}

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

func (h *Hasher) Verify(ctx context.Context, encoded, password string) (ok, rehash bool, err error) {
	if tooLong(password) {
		return false, false, nil
	}
	p, salt, want, err := parseHash(encoded)
	if err != nil {
		// So a broken stored hash answers as slowly as any other failed login.
		if _, derr := h.derive(ctx, password, make([]byte, saltLen), currentParams, keyLen); derr != nil {
			return false, false, derr
		}
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

// verifyNothing costs what a failed Verify does, for sign-ins with no usable hash.
func (h *Hasher) verifyNothing(ctx context.Context, password string) error {
	if tooLong(password) {
		return nil
	}
	_, err := h.derive(ctx, password, make([]byte, saltLen), currentParams, keyLen)
	return err
}

func (h *Hasher) derive(ctx context.Context, password string, salt []byte, p params, n uint32) ([]byte, error) {
	if err := h.acquire(ctx); err != nil {
		return nil, err
	}
	defer func() { <-h.slots }()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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

func tooLong(password string) bool {
	var it norm.Iter
	it.InitString(norm.NFC, password)
	n := 0
	for !it.Done() {
		if n += utf8.RuneCount(it.Next()); n > maxPasswordLen {
			return true
		}
	}
	return false
}

func parseHash(encoded string) (p params, salt, key []byte, err error) {
	if len(encoded) > maxStoredHashLen {
		return params{}, nil, nil, errMalformedHash
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return params{}, nil, nil, errMalformedHash
	}
	var rest string
	if n, _ := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d%s", &p.memory, &p.time, &p.threads, &rest); n != 3 ||
		p.memory == 0 || p.memory > maxStoredMemoryKiB || p.time == 0 || p.time > maxStoredTime || p.threads == 0 || p.threads > maxStoredThreads {
		return params{}, nil, nil, errMalformedHash
	}
	enc := base64.RawStdEncoding
	if salt, err = enc.DecodeString(parts[4]); err != nil || len(salt) < minStoredSaltLen || len(salt) > maxStoredSaltLen {
		return params{}, nil, nil, errMalformedHash
	}
	if key, err = enc.DecodeString(parts[5]); err != nil || len(key) < minStoredKeyLen || len(key) > maxStoredKeyLen {
		return params{}, nil, nil, errMalformedHash
	}
	return p, salt, key, nil
}

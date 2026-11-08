package auth

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckNewPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "8 characters", password: "rosa1962"},
		{name: "a passphrase", password: "grandma's garden in june"},
		{name: "7 characters", password: "rosa196", wantErr: ErrPasswordTooShort},
		{name: "length counts characters, not bytes", password: "ééééééé", wantErr: ErrPasswordTooShort},
		{name: "128 characters", password: strings.Repeat("a", 127) + "b"},
		{name: "129 characters", password: strings.Repeat("a", 128) + "b", wantErr: ErrPasswordTooLong},
		{name: "a common password", password: "password1", wantErr: ErrPasswordTooCommon},
		{name: "a common password in other case", password: "PassWord1", wantErr: ErrPasswordTooCommon},
		{name: "the email", password: "Ana.Silva@Example.com", wantErr: ErrPasswordTooCommon},
		{name: "the email's name part", password: "ana.silva", wantErr: ErrPasswordTooCommon},
		{name: "the app's name", password: "Family Hub", wantErr: ErrPasswordTooCommon},
		{name: "the app's name with punctuation and digits", password: "Family-Hub_1", wantErr: ErrPasswordTooCommon},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorIs(t, CheckNewPassword(tt.password, " Ana.Silva@example.com "), tt.wantErr)
		})
	}
}

func TestCommonPasswordsSkipsTheHeader(t *testing.T) {
	for p := range commonPasswords() {
		require.False(t, strings.HasPrefix(p, "# the ncsc") || strings.HasPrefix(p, "# https://"), p)
	}
	assert.Contains(t, commonPasswords(), "123456789")
}

func TestHasherVerify(t *testing.T) {
	h := NewHasher()
	encoded, err := h.Hash(t.Context(), "rosa garden 1962")
	require.NoError(t, err)
	older := hashWith(t, params{memory: 12 * 1024, time: 3, threads: 1}, "rosa garden 1962")

	tests := []struct {
		name       string
		encoded    string
		password   string
		wantOK     bool
		wantRehash bool
		wantErr    error
	}{
		{name: "the right password", encoded: encoded, password: "rosa garden 1962", wantOK: true},
		{name: "a wrong password", encoded: encoded, password: "rosa garden 1963"},
		{name: "older parameters ask for a rehash", encoded: older, password: "rosa garden 1962", wantOK: true, wantRehash: true},
		{name: "no rehash for a wrong password", encoded: older, password: "nope nope nope"},
		{name: "not a PHC string", encoded: "plain", password: "x", wantErr: errMalformedHash},
		{name: "another algorithm", encoded: strings.Replace(encoded, "argon2id", "argon2i", 1), password: "x", wantErr: errMalformedHash},
		{name: "another version", encoded: strings.Replace(encoded, "v=19", "v=16", 1), password: "x", wantErr: errMalformedHash},
		{name: "trailing junk in the parameters", encoded: strings.Replace(encoded, "p=1", "p=1x", 1), password: "x", wantErr: errMalformedHash},
		{name: "zero memory", encoded: strings.Replace(encoded, "m=19456", "m=0", 1), password: "x", wantErr: errMalformedHash},
		{name: "a one-byte key", encoded: withPart(encoded, 5, "AA"), password: "x", wantErr: errMalformedHash},
		{name: "a short salt", encoded: withPart(encoded, 4, "AAAA"), password: "x", wantErr: errMalformedHash},
		{name: "a long salt", encoded: withPart(encoded, 4, strings.Repeat("A", 100)), password: "x", wantErr: errMalformedHash},
		{name: "an oversized hash string", encoded: encoded + strings.Repeat("A", 256), password: "x", wantErr: errMalformedHash},
		{name: "memory beyond the bound", encoded: strings.Replace(encoded, "m=19456", "m=4194304", 1), password: "x", wantErr: errMalformedHash},
		{name: "a password longer than any stored one", encoded: encoded, password: strings.Repeat("a", 129)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, rehash, err := h.Verify(t.Context(), tt.encoded, tt.password)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantRehash, rehash)
		})
	}
}

func TestHashIsNFCNormalized(t *testing.T) {
	h := NewHasher()
	composed, decomposed := "café au lait 1962", "café au lait 1962"
	encoded, err := h.Hash(t.Context(), composed)
	require.NoError(t, err)

	ok, _, err := h.Verify(t.Context(), encoded, decomposed)

	require.NoError(t, err)
	assert.True(t, ok)
}

func TestLengthCountsNormalizedCharacters(t *testing.T) {
	composed := strings.Repeat("\u1ed9", maxPasswordLen)
	decomposed := strings.Repeat("o\u0323\u0302", maxPasswordLen)
	require.Greater(t, len(decomposed), 4*maxPasswordLen)
	h := NewHasher()
	encoded, err := h.Hash(t.Context(), composed)
	require.NoError(t, err)

	ok, _, err := h.Verify(t.Context(), encoded, decomposed)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, CheckNewPassword(decomposed, "ana@example.com"))
}

func TestACanceledRequestDoesNotHash(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := NewHasher().Hash(ctx, "rosa garden 1962")

	assert.ErrorIs(t, err, context.Canceled)
}

func TestHashRefusesATooLongPassword(t *testing.T) {
	_, err := NewHasher().Hash(t.Context(), strings.Repeat("a", 129))

	assert.ErrorIs(t, err, ErrPasswordTooLong)
}

func TestVerifyRunsArgon2EvenForABrokenHash(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHasher()
		for range maxConcurrentHashes {
			h.slots <- struct{}{}
		}
		start := time.Now()

		_, _, err := h.Verify(t.Context(), "plain", "rosa garden 1962")

		require.ErrorIs(t, err, ErrBusy)
		assert.Equal(t, hashWait, time.Since(start), "it waited for a slot, so it would have hashed")
	})
}

func TestHashUsesAFreshSalt(t *testing.T) {
	h := NewHasher()
	a, err := h.Hash(t.Context(), "rosa garden 1962")
	require.NoError(t, err)
	b, err := h.Hash(t.Context(), "rosa garden 1962")
	require.NoError(t, err)

	assert.NotEqual(t, a, b)
	assert.True(t, strings.HasPrefix(a, "$argon2id$v=19$m=19456,t=2,p=1$"), a)
}

func TestHasherWaitsForAFreeSlot(t *testing.T) {
	tests := []struct {
		name       string
		cancel     bool
		wantErr    error
		wantWaited time.Duration
	}{
		{name: "busy after the wait", wantErr: ErrBusy, wantWaited: hashWait},
		{name: "a canceled request stops waiting", cancel: true, wantErr: context.Canceled, wantWaited: time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := NewHasher()
				for range maxConcurrentHashes {
					h.slots <- struct{}{}
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if tt.cancel {
					time.AfterFunc(time.Second, cancel)
				}
				start := time.Now()

				_, err := h.Hash(ctx, "rosa garden 1962")

				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, tt.wantWaited, time.Since(start))
			})
		})
	}
}

func TestHasherProceedsWhenASlotFrees(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHasher()
		for range maxConcurrentHashes {
			h.slots <- struct{}{}
		}
		time.AfterFunc(time.Second, func() { <-h.slots })

		_, err := h.Hash(t.Context(), "rosa garden 1962")

		require.NoError(t, err)
	})
}

func withPart(encoded string, i int, value string) string {
	parts := strings.Split(encoded, "$")
	parts[i] = value
	return strings.Join(parts, "$")
}

func hashWith(t *testing.T, p params, password string) string {
	t.Helper()
	encoded, err := NewHasher().hash(t.Context(), password, p)
	require.NoError(t, err)
	require.Contains(t, encoded, fmt.Sprintf("m=%d,t=%d", p.memory, p.time))
	return encoded
}

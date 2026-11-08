package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kdelpe/family-hub/internal/ratelimit"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        []string
		viaProxy   bool
		want       string
	}{
		{name: "IPv4 direct", remoteAddr: "203.0.113.7:51000", want: "203.0.113.7"},
		{name: "IPv6 cut to its /64", remoteAddr: "[2001:db8:1:2:aaaa:bbbb:cccc:dddd]:443", want: "2001:db8:1:2::/64"},
		{name: "IPv4-mapped IPv6 is IPv4", remoteAddr: "[::ffff:203.0.113.7]:443", want: "203.0.113.7"},
		{name: "forwarded header ignored without a proxy", remoteAddr: "203.0.113.7:51000", xff: []string{"198.51.100.1"}, want: "203.0.113.7"},
		{name: "proxy's entry is the last one", remoteAddr: "127.0.0.1:9000", xff: []string{"6.6.6.6, 198.51.100.1"}, viaProxy: true, want: "198.51.100.1"},
		{name: "last of several headers", remoteAddr: "127.0.0.1:9000", xff: []string{"6.6.6.6", "198.51.100.1"}, viaProxy: true, want: "198.51.100.1"},
		{name: "garbage entry keeps the socket address", remoteAddr: "127.0.0.1:9000", xff: []string{"not-an-ip"}, viaProxy: true, want: "127.0.0.1"},
		{name: "no header keeps the socket address", remoteAddr: "127.0.0.1:9000", viaProxy: true, want: "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Header["X-Forwarded-For"] = tt.xff
			req.Header.Set("X-Real-Ip", "6.6.6.6")
			var got string
			h := http.Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = ClientIP(r) }))
			if tt.viaProxy {
				h = ClientIPFromProxy(h)
			}

			h.ServeHTTP(httptest.NewRecorder(), req)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLimitByIP(t *testing.T) {
	limited := LimitByIP(ratelimit.New(1, time.Minute))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	send := func(addr string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		limited.ServeHTTP(rec, req)
		return rec
	}

	assert.Equal(t, http.StatusNoContent, send("203.0.113.7:1").Code)
	rec := send("203.0.113.7:2")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "60", rec.Header().Get("Retry-After"))
	assert.JSONEq(t, `{"error":{"code":"rate_limited","message":"too many attempts, try again later"}}`, rec.Body.String())
	assert.Equal(t, http.StatusNoContent, send("198.51.100.1:1").Code, "another IP")
}

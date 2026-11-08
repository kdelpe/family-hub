package httpx

import (
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/kdelpe/family-hub/internal/ratelimit"
)

// ClientIPFromProxy trusts only the last X-Forwarded-For entry: the one our proxy appended.
// chi's RealIP prefers True-Client-IP and X-Real-IP, which a client can send itself.
func ClientIPFromProxy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
			entries := strings.Split(values[len(values)-1], ",")
			if ip, err := netip.ParseAddr(strings.TrimSpace(entries[len(entries)-1])); err == nil {
				r.RemoteAddr = netip.AddrPortFrom(ip, 0).String()
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP is the client's address, with IPv6 cut to its /64: one household usually has a whole /64.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	ip = ip.Unmap()
	if ip.Is6() {
		prefix, _ := ip.Prefix(64)
		return prefix.String()
	}
	return ip.String()
}

func LimitByIP(l *ratelimit.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ok, wait := l.Allow(ClientIP(r)); !ok {
				TooManyRequests(w, wait)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func TooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
	Error(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
}

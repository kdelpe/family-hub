// Package httpx holds HTTP helpers shared by the router and handlers.
package httpx

import "net/http"

// default-src doesn't cover framing, so frame-ancestors is set too.
const contentSecurityPolicy = "default-src 'self'; frame-ancestors 'none'"

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Robots-Tag", "noindex")
		next.ServeHTTP(w, r)
	})
}

// NoStore keeps API answers out of every cache: they depend on who is signed in, and contacts must
// be fetched fresh each time (§5.2).
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

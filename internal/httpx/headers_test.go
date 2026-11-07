package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "content security policy", header: "Content-Security-Policy", want: "default-src 'self'; frame-ancestors 'none'"},
		{name: "no MIME sniffing", header: "X-Content-Type-Options", want: "nosniff"},
		{name: "referrer stays on the site", header: "Referrer-Policy", want: "same-origin"},
		{name: "no search indexing", header: "X-Robots-Tag", want: "noindex"},
	}

	rec := httptest.NewRecorder()
	SecurityHeaders(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, rec.Header().Get(tt.header))
		})
	}
}

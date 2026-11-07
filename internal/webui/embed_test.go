package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestHandler(t *testing.T) {
	files := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>app</title>")},
		"favicon.ico":   {Data: []byte("icon")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	tests := []struct {
		name         string
		path         string
		wantStatus   int
		wantBody     string
		wantCache    string
		wantLocation string
	}{
		{name: "root is the app", path: "/", wantStatus: 200, wantBody: "<!doctype html><title>app</title>", wantCache: indexCache},
		{name: "a client-side route is the app", path: "/persons/42", wantStatus: 200, wantBody: "<!doctype html><title>app</title>", wantCache: indexCache},
		{name: "a fingerprinted asset is cached for good", path: "/assets/app.js", wantStatus: 200, wantBody: "console.log(1)", wantCache: assetsCache},
		{name: "another file is served as is", path: "/favicon.ico", wantStatus: 200, wantBody: "icon"},
		{name: "a directory is the app, not a listing", path: "/assets/", wantStatus: 200, wantBody: "<!doctype html><title>app</title>", wantCache: indexCache},
		{name: "a missing asset is a 404", path: "/assets/old-hash.js", wantStatus: 404},
		{name: "a missing file is a 404", path: "/logo.png", wantStatus: 404},
		{name: "a path with .. is cleaned", path: "/persons/../tree", wantStatus: 200, wantBody: "<!doctype html><title>app</title>", wantCache: indexCache},
		{name: "index.html redirects to the root", path: "/index.html", wantStatus: http.StatusMovedPermanently, wantLocation: "./"},
	}

	h := handler(files)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody != "" {
				assert.Equal(t, tt.wantBody, rec.Body.String())
			}
			assert.Equal(t, tt.wantCache, rec.Header().Get("Cache-Control"))
			assert.Equal(t, tt.wantLocation, rec.Header().Get("Location"))
		})
	}
}

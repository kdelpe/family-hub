// Package webui serves the React app that Vite builds into dist (a stub from `make ui-stub` until then).
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const (
	// Vite fingerprints everything under assets/, so a changed file gets a new name.
	assetsCache = "public, max-age=31536000, immutable"
	indexCache  = "no-cache"
)

//go:embed all:dist
var dist embed.FS

func Handler() http.Handler {
	files, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return handler(files)
}

// A path that isn't a file gets index.html, so client-side routes load; so does a directory, so none is listed.
// A missing file (one with an extension) is a 404, so a stale tab's old asset fails clearly instead of getting HTML.
func handler(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		// Serve the cleaned path: net/http refuses one with ".." in it.
		r = r.Clone(r.Context())
		r.URL.Path = "/" + name
		info, err := fs.Stat(files, name)
		if err != nil && path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		if err != nil || info.IsDir() {
			w.Header().Set("Cache-Control", indexCache)
			http.ServeFileFS(w, r, files, "index.html")
			return
		}
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", assetsCache)
		}
		fileServer.ServeHTTP(w, r)
	})
}

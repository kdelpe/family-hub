package httpx

import (
	"io"
	"net/http"
)

// NotFound answers an unknown /api path with a JSON 404, never the app's index.html.
func NotFound(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = io.WriteString(w, `{"error":{"code":"not_found","message":"not found"}}`+"\n")
}

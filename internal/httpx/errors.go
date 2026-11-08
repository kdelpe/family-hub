package httpx

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: errorDetail{Code: code, Message: message}})
}

// NotFound answers an unknown /api path with a JSON 404, never the app's index.html.
func NotFound(w http.ResponseWriter, _ *http.Request) {
	Error(w, http.StatusNotFound, "not_found", "not found")
}

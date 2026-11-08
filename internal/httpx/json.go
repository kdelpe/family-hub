package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

const maxJSONBody = 1 << 20

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON reads one JSON object into dst; on false it has already answered.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mediaType != "application/json" {
		Error(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "send application/json")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil {
		if err = dec.Decode(&struct{}{}); err == nil {
			err = errors.New("more than one JSON value")
		} else if errors.Is(err, io.EOF) {
			err = nil
		}
	}
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		Error(w, http.StatusRequestEntityTooLarge, "too_large", "request body too large")
		return false
	case err != nil:
		Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return false
	}
	return true
}

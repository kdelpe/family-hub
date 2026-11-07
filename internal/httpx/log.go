package httpx

import (
	"crypto/rand"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"runtime/debug"
	"slices"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

const requestIDHeader = "X-Request-Id"

// RequestID gives each request a new ID, in its response header and its logger.
// One sent by the client is ignored, so it can't forge or reuse IDs in the logs.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := rand.Text()
		w.Header().Set(requestIDHeader, id)
		logger := zerolog.Ctx(r.Context()).With().Str("request_id", id).Logger()
		next.ServeHTTP(w, r.WithContext(logger.WithContext(r.Context())))
	})
}

// RequestLogger logs one line per request, leaving out the successful ones to quiet paths.
// The path is logged without its query, which can carry search terms.
func RequestLogger(quiet ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			// Still true in the defer only if next panicked: Recover lets through just http.ErrAbortHandler.
			aborted := true
			defer func() {
				status := ww.Status()
				if status == 0 && !aborted {
					status = http.StatusOK
				}
				if !aborted && status < http.StatusBadRequest && slices.Contains(quiet, r.URL.Path) {
					return
				}
				logger := zerolog.Ctx(r.Context())
				event := logger.Info()
				if aborted {
					event = logger.Warn().Bool("aborted", true)
				}
				event.
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Int("status", status).
					Int("bytes", ww.BytesWritten()).
					Dur("duration_ms", time.Since(start)).
					Msg("request")
			}()
			next.ServeHTTP(ww, r)
			aborted = false
		})
	}
}

// Recover logs a handler's panic with its stack and answers 500 with only the headers set before it.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww, ok := w.(middleware.WrapResponseWriter)
		if !ok {
			ww = middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		}
		// The handler's own headers, like a cookie it set, mustn't go out with the 500.
		before := ww.Header().Clone()
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			// net/http's own signal to drop the connection; it stays quiet only for the exact value, not a wrapper.
			if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(http.ErrAbortHandler)
			}
			zerolog.Ctx(r.Context()).Error().
				Str("panic", fmt.Sprint(v)).
				Str("stack", string(debug.Stack())).
				Msg("handler panicked")
			// Once the response has started it's too late for a 500; dropping the connection shows it's incomplete.
			if ww.Status() != 0 {
				panic(http.ErrAbortHandler)
			}
			h := ww.Header()
			clear(h)
			maps.Copy(h, before)
			ww.WriteHeader(http.StatusInternalServerError)
		}()
		next.ServeHTTP(ww, r)
	})
}

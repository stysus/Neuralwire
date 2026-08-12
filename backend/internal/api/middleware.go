package api

import (
	"context"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// contextKey is a private type for request context values.
type contextKey string

// ctxUsername carries the authenticated username through the request.
const ctxUsername contextKey = "username"

// requireAuth protects the admin API. Requests without an Authorization
// header get 401; requests with an invalid or expired token get 403.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			s.writeError(w, http.StatusUnauthorized, "unauthorized: missing bearer token")
			return
		}

		username, err := s.auth.Validate(token)
		if err != nil {
			s.writeError(w, http.StatusForbidden, "forbidden: invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), ctxUsername, username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// bearerToken extracts the token from an "Authorization: Bearer <token>"
// header. It reports false when the header is absent or malformed.
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}

// cors adds scoped CORS headers for the configured origins and
// short-circuits preflight requests. The origin is echoed back only when it
// is in the allow list, so browsers block everything else.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// originAllowed reports whether the request Origin is in the allow list.
func (s *Server) originAllowed(origin string) bool {
	for _, allowed := range s.allowOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

// log logs each request with status, method, path and duration.
func (s *Server) log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

// recover converts panics into 500 responses so one bad handler cannot
// crash the process.
func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Printf("api: panic serving %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				s.writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

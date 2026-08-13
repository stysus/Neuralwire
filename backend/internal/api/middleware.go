package api

import (
	"context"
	"net/http"
	"runtime/debug"
	"strconv"
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

// csrfProtect defends the admin API against cross-site request forgery.
// Browsers automatically attach the Origin header on state-changing requests
// (POST/PUT/DELETE); we require it to be an allowed origin. Non-browser
// clients (curl, servers) typically send no Origin header and are allowed,
// since the bearer token itself already gates access. This is defense-in-depth
// on top of the token (which browsers do not auto-send on cross-origin
// requests).
func (s *Server) csrfProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			origin := r.Header.Get("Origin")
			// No Origin => non-browser client (curl, server-to-server). Allow.
			if origin != "" && !s.originAllowed(origin) {
				s.writeError(w, http.StatusForbidden, "forbidden: cross-origin request rejected")
				return
			}
		}
		next.ServeHTTP(w, r)
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

// clientIP extracts the client's IP from the request. X-Forwarded-For is only
// trusted when the server sits behind a trusted reverse proxy (TrustProxy);
// otherwise attackers could spoof the header and bypass per-IP rate limits.
// Ports are stripped.
func (s *Server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			if i := strings.IndexByte(xff, ','); i >= 0 {
				xff = xff[:i]
			}
			xff = strings.TrimSpace(xff)
			if xff != "" {
				return xff
			}
		}
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return strings.Trim(host, "[]")
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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

// log logs each request with status, method, path, client IP, and duration
// using the structured slog logger. Response bodies are never logged.
func (s *Server) log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", s.clientIP(r),
		)
	})
}

// recover converts panics into 500 responses so one bad handler cannot
// crash the process.
func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.slog.Error("api: panic serving request",
					"method", r.Method,
					"path", r.URL.Path,
					"panic", rec,
					"stack", string(debug.Stack()),
				)
				s.writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// rateLimit applies the global per-IP limit to every request, slowing
// scanners and bots. Health checks and OPTIONS preflights are exempt so
// monitoring and CORS preflight are never throttled. Limited requests get
// 429 with Retry-After.
func (s *Server) rateLimit(next http.Handler) http.Handler {
	if s.globalLimiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		ok, retryAfter := s.globalLimiter.Allow(s.clientIP(r))
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			s.writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeaders adds hardening headers to every response:
//   - X-Content-Type-Options: nosniff — prevents MIME sniffing
//   - X-Frame-Options: DENY — prevents clickjacking
//   - Referrer-Policy: strict-origin-when-cross-origin — limits referrer leaks
//   - Permissions-Policy — disables unused browser features
//   - Content-Security-Policy — restricts script/image sources (anti-XSS)
//
// Note: CSP is intentionally permissive for images (img-src * data:) because
// cover images come from many third-party CDNs. Admin-created content is
// rendered via {@html}; CSP default-src 'self' still blocks inline scripts.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https: http:; font-src 'self' data:; "+
				"connect-src 'self' http://localhost:8080; frame-ancestors 'none'; base-uri 'self'")
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

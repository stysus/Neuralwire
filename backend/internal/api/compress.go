package api

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

// compressibleTypes lists the media types we bother compressing. Everything
// else (images, audio, video, archives) is already compact and would waste CPU.
var compressibleTypes = []string{
	"application/json",
	"application/javascript",
	"application/xml",
	"text/plain",
	"text/html",
	"text/css",
	"text/xml",
	"text/javascript",
	"image/svg+xml",
}

// isCompressible reports whether the response content type can be compressed.
func isCompressible(contentType string) bool {
	if contentType == "" {
		return false
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	for _, t := range compressibleTypes {
		if ct == t {
			return true
		}
	}
	return false
}

// compressResponseWriter transparently gzip- or brotli-compresses the body for
// clients that advertise support via Accept-Encoding. Headers are set before
// the underlying WriteHeader so proxies and clients see the correct encoding.
type compressResponseWriter struct {
	http.ResponseWriter
	enc       *compressor
	wroteHead bool
	skipped   bool
}

type compressor struct {
	write func([]byte) (int, error)
	close func() error
}

// WriteHeader records the status. Only body-bearing responses with a
// compressible content type are wrapped; 1xx/204/304 and already-encoded
// responses pass through untouched.
func (w *compressResponseWriter) WriteHeader(code int) {
	if w.wroteHead {
		return
	}
	w.wroteHead = true
	if code >= 200 && code != http.StatusNoContent && code != http.StatusNotModified &&
		w.enc != nil && isCompressible(w.Header().Get("Content-Type")) &&
		w.Header().Get("Content-Encoding") == "" {
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Set("Content-Encoding", w.Header().Get("X-NW-Encoding"))
		w.Header().Del("X-NW-Encoding")
		w.Header().Del("Content-Length")
		w.ResponseWriter.WriteHeader(code)
		return
	}
	w.skipped = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *compressResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHead {
		w.WriteHeader(http.StatusOK)
	}
	if w.skipped || w.enc == nil {
		return w.ResponseWriter.Write(b)
	}
	return w.enc.write(b)
}

// close flushes any buffered compressed data to the underlying writer.
func (w *compressResponseWriter) close() {
	if w.enc != nil && !w.skipped {
		_ = w.enc.close()
	}
}

// gzipCompressor wraps w in a gzip encoder.
func gzipCompressor(w io.Writer) *compressor {
	gz := gzip.NewWriter(w)
	return &compressor{write: gz.Write, close: gz.Close}
}

// brotliCompressor wraps w in a brotli encoder.
func brotliCompressor(w io.Writer) *compressor {
	br := brotli.NewWriter(w)
	return &compressor{write: br.Write, close: br.Close}
}

// compress negotiates gzip or brotli from the request's Accept-Encoding header
// and wraps the response writer accordingly.
func (s *Server) compress(next http.Handler) http.Handler {
	if s.disableCompression {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := acceptCompression(r)
		if enc == "" {
			next.ServeHTTP(w, r)
			return
		}
		// X-NW-Encoding is a temporary header: WriteHeader reads it to pick the
		// real Content-Encoding value, then removes it.
		w.Header().Set("X-NW-Encoding", enc)
		var comp *compressor
		if enc == "br" {
			comp = brotliCompressor(w)
		} else {
			comp = gzipCompressor(w)
		}
		cw := &compressResponseWriter{ResponseWriter: w, enc: comp}
		defer cw.close()
		next.ServeHTTP(cw, r)
	})
}

// acceptCompression returns the strongest supported encoding from
// Accept-Encoding, or "" when the client wants no compression. Brotli is
// preferred over gzip.
func acceptCompression(r *http.Request) string {
	header := strings.ToLower(r.Header.Get("Accept-Encoding"))
	if header == "" || strings.Contains(header, "identity") {
		return ""
	}
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		enc := part
		if i := strings.IndexByte(part, ';'); i >= 0 {
			enc = strings.TrimSpace(part[:i])
		}
		if enc == "br" {
			return "br"
		}
	}
	if strings.Contains(header, "gzip") {
		return "gzip"
	}
	return ""
}

// --- ETag / conditional requests ------------------------------------------

// etagResponseWriter buffers the full response body so we can compute a strong
// ETag and serve 304 Not Modified to clients that already hold a fresh copy.
type etagResponseWriter struct {
	http.ResponseWriter
	buf        bytes.Buffer
	status     int
	wroteHead  bool
	skipBuffer bool
}

func (w *etagResponseWriter) WriteHeader(code int) {
	if w.wroteHead {
		return
	}
	w.wroteHead = true
	w.status = code
}

func (w *etagResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHead {
		w.WriteHeader(http.StatusOK)
	}
	return w.buf.Write(b)
}

// etag computes a sha256-based ETag for successful GET responses and honors
// If-None-Match with a 304. Errors and state-changing requests are never
// cached and pass through unchanged.
func (s *Server) etag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		ew := &etagResponseWriter{ResponseWriter: w}
		next.ServeHTTP(ew, r)

		if ew.status == 0 {
			ew.status = http.StatusOK
		}
		// Only 200 responses are cacheable; anything else (errors, redirects)
		// is streamed straight through.
		if ew.status != http.StatusOK || ew.buf.Len() == 0 {
			w.WriteHeader(ew.status)
			if ew.buf.Len() > 0 {
				_, _ = w.Write(ew.buf.Bytes())
			}
			return
		}

		sum := sha256.Sum256(ew.buf.Bytes())
		etag := `"` + hex.EncodeToString(sum[:16]) + `"`
		w.Header().Set("ETag", etag)

		if match := r.Header.Get("If-None-Match"); match != "" && etagMatches(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.WriteHeader(ew.status)
		_, _ = w.Write(ew.buf.Bytes())
	})
}

// etagMatches compares an If-None-Match header (possibly a comma-separated
// list with W/ prefixes) against our computed ETag.
func etagMatches(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "W/")
		if part == etag {
			return true
		}
	}
	return false
}

// --- Cache-Control headers ------------------------------------------------

// cacheControl sets a sensible Cache-Control header per route:
//   - /api/health and /api/admin/* are never cached (sensitive / monitoring)
//   - /api/news/trending is server-cached for minutes, so browsers may reuse
//     it briefly too
//   - everything else is marked no-cache so clients must revalidate with the
//     ETag before reusing a cached copy
func (s *Server) cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/api/health" || strings.HasPrefix(path, "/api/admin/"):
			w.Header().Set("Cache-Control", "no-store")
		case path == "/api/news/trending":
			w.Header().Set("Cache-Control", "public, max-age=60")
		case isStaticPath(path):
			w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
		default:
			w.Header().Set("Cache-Control", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

// isStaticPath reports whether the path looks like a static asset that can be
// cached long-term. The API serves no static files today, but the backend may
// embed the built frontend in production.
func isStaticPath(path string) bool {
	for _, ext := range []string{".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico", ".woff", ".woff2", ".ttf", ".eot"} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/assets/")
}

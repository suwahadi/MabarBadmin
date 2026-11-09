package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)
			if strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/healthz" {
				return
			}
			log.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status,
				"bytes", sw.bytes, "duration_ms", time.Since(start).Milliseconds(), "ip", ClientIP(r))
		})
	}
}

func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					if v == http.ErrAbortHandler {
						panic(v)
					}
					log.Error("panic", "err", v, "path", r.URL.Path, "stack", string(debug.Stack()))
					http.Error(w, "Terjadi kesalahan pada server.", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets a strict CSP; scripts only load from our origin (Alpine runs its CSP build).
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// MaxBody caps request bodies a little above the 10 MB upload limit; Caddy enforces its own cap too.
func MaxBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP trusts X-Forwarded-For only from loopback, where Caddy runs.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			return strings.TrimSpace(strings.Split(fwd, ",")[0])
		}
	}
	return host
}

type csrfKey struct{}

const csrfCookie = "csrf"

// CSRF pairs Go's cross-origin protection with a per-browser form token: the token is an HMAC
// of a random cookie value, sent back as a form field or the X-CSRF-Token header (HTMX).
type CSRF struct {
	Secret []byte
	Secure bool
	OnFail http.Handler
}

func (c CSRF) token(seed string) string {
	m := hmac.New(sha256.New, c.Secret)
	m.Write([]byte(seed))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (c CSRF) Handler(next http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()
	cop.SetDenyHandler(c.OnFail)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seed := ""
		if ck, err := r.Cookie(csrfCookie); err == nil && len(ck.Value) >= 32 {
			seed = ck.Value
		} else {
			b := make([]byte, 32)
			_, _ = rand.Read(b)
			seed = base64.RawURLEncoding.EncodeToString(b)
			http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: seed, Path: "/", HttpOnly: true, Secure: c.Secure,
				SameSite: http.SameSiteLaxMode, MaxAge: 60 * 60 * 24 * 365})
		}
		want := c.token(seed)
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			// Parse uploads with a 1 MB memory budget so larger files spill to a temp file instead
			// of the default 32 MB held in RAM per request.
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				_ = r.ParseMultipartForm(1 << 20)
			}
			got := r.Header.Get("X-CSRF-Token")
			if got == "" {
				got = r.FormValue("csrf_token")
			}
			if !hmac.Equal([]byte(got), []byte(want)) {
				c.OnFail.ServeHTTP(w, r)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), csrfKey{}, want)))
	})
	return cop.Handler(inner)
}

func CSRFToken(ctx context.Context) string {
	v, _ := ctx.Value(csrfKey{}).(string)
	return v
}

// RateLimiter allows a fixed number of attempts per key inside a sliding window.
type RateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, hits: map[string][]time.Time{}}
}

func (l *RateLimiter) prune(key string, now time.Time) []time.Time {
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.hits, key)
		return nil
	}
	l.hits[key] = kept
	return kept
}

// Blocked reports whether any key reached its limit.
func (l *RateLimiter) Blocked(keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for _, k := range keys {
		if len(l.prune(k, now)) >= l.limit {
			return true
		}
	}
	return false
}

func (l *RateLimiter) Hit(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for _, k := range keys {
		l.hits[k] = append(l.prune(k, now), now)
	}
}

func (l *RateLimiter) Reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		delete(l.hits, k)
	}
}

func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

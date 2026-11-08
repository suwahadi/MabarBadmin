package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	l := NewRateLimiter(2, time.Minute)
	l.Hit("a")
	if l.Blocked("a") {
		t.Fatal("blocked too early")
	}
	l.Hit("a")
	if !l.Blocked("a", "b") {
		t.Fatal("should block after limit")
	}
	l.Reset("a")
	if l.Blocked("a") {
		t.Fatal("reset failed")
	}
}

func TestCSRF(t *testing.T) {
	c := CSRF{Secret: []byte("01234567890123456789012345678901"), OnFail: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})}
	var token string
	h := c.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { token = CSRFToken(r.Context()) }))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	cookie := rec.Result().Cookies()[0]
	if token == "" {
		t.Fatal("no token")
	}

	post := func(tok string) int {
		req := httptest.NewRequest("POST", "/x", strings.NewReader(url.Values{"csrf_token": {tok}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := post(token); code != http.StatusOK {
		t.Fatalf("valid token got %d", code)
	}
	if code := post("bad"); code != http.StatusForbidden {
		t.Fatalf("bad token got %d", code)
	}
}

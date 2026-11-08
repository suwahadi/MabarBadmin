package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/auth"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/middleware"
	"github.com/suwahadi/mabarbadmin/internal/service"
)

const sessionCookie = "sid"

type userKey struct{}

func currentUser(r *http.Request) *sqlc.User {
	u, _ := r.Context().Value(userKey{}).(*sqlc.User)
	return u
}

func (a *App) loadUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") || strings.HasPrefix(r.URL.Path, "/media/") {
			next.ServeHTTP(w, r)
			return
		}
		if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			if u, err := a.Q.GetAuthSessionUser(r.Context(), auth.HashToken(c.Value)); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), userKey{}, &u))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: a.Cfg.SecureCookies(),
		SameSite: http.SameSiteLaxMode, MaxAge: int(service.SessionTTL / time.Second)})
}

func (a *App) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: a.Cfg.SecureCookies(),
		SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func (a *App) requireMember(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		if u == nil {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		if u.MustChangePassword && r.URL.Path != "/me/password" {
			http.Redirect(w, r, "/me/password", http.StatusSeeOther)
			return
		}
		h(w, r)
	})
}

func (a *App) requireAdmin(h http.HandlerFunc) http.Handler {
	return a.requireMember(func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r).Role != "admin" {
			a.renderError(w, r, http.StatusForbidden, "Halaman ini khusus admin.")
			return
		}
		h(w, r)
	})
}

func (a *App) adminActor(r *http.Request) service.Actor {
	return service.AdminActor(currentUser(r).ID)
}

func setFlash(w http.ResponseWriter, kind, msg string) {
	v := base64.RawURLEncoding.EncodeToString([]byte(kind + "|" + msg))
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: v, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 60})
}

func popFlash(w http.ResponseWriter, r *http.Request) *Flash {
	c, err := r.Cookie("flash")
	if err != nil {
		return nil
	}
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: "", Path: "/", MaxAge: -1})
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil
	}
	kind, msg, ok := strings.Cut(string(raw), "|")
	if !ok {
		return nil
	}
	return &Flash{Kind: kind, Msg: msg}
}

func (a *App) redirect(w http.ResponseWriter, r *http.Request, to string) {
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", to)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// done reports a successful action with a flash message and redirects.
func (a *App) done(w http.ResponseWriter, r *http.Request, to, msg string) {
	setFlash(w, "ok", msg)
	a.redirect(w, r, to)
}

// fail turns a service error into a flash message, or a 500 page for unexpected errors.
func (a *App) fail(w http.ResponseWriter, r *http.Request, to string, err error) {
	if msg, ok := service.IsUserError(err); ok {
		setFlash(w, "error", msg)
		a.redirect(w, r, to)
		return
	}
	if errors.Is(err, service.ErrNotFound) {
		a.notFound(w, r)
		return
	}
	a.serverError(w, r, err)
}

func (a *App) notFound(w http.ResponseWriter, r *http.Request) {
	a.renderError(w, r, http.StatusNotFound, "Halaman yang kamu cari tidak ditemukan.")
}

func (a *App) serverError(w http.ResponseWriter, r *http.Request, err error) {
	a.Log.Error("server error", "err", err, "path", r.URL.Path)
	http.Error(w, "Terjadi kesalahan pada server. Coba lagi sebentar.", http.StatusInternalServerError)
}

func (a *App) renderError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	v := a.newView(w, r, http.StatusText(status))
	v.Admin = false
	v.Meta.NoIndex = true
	v.Data["Status"] = status
	v.Data["Message"] = msg
	a.render(w, r, status, "public/error", v)
}

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func formInt64(r *http.Request, key string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue(key)), 10, 64)
	return n
}

// formMoney accepts "60.000", "Rp 60.000", or "60000".
func formMoney(r *http.Request, key string) int64 {
	var b strings.Builder
	for _, c := range r.FormValue(key) {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	n, _ := strconv.ParseInt(b.String(), 10, 64)
	return n
}

func formBool(r *http.Request, key string) bool {
	v := r.FormValue(key)
	return v == "on" || v == "true" || v == "1"
}

func (a *App) formTime(r *http.Request, key string) (time.Time, bool) {
	v := strings.TrimSpace(r.FormValue(key))
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02T15:04", v, a.Loc())
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func (a *App) formTimePtr(r *http.Request, key string) *time.Time {
	if t, ok := a.formTime(r, key); ok {
		return &t
	}
	return nil
}

func (a *App) formDate(r *http.Request, key string) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(r.FormValue(key)), a.Loc())
	return t, err == nil
}

// formFile returns the uploaded file or nil when the field is empty.
func formFile(r *http.Request, key string) (service.Upload, func()) {
	f, h, err := r.FormFile(key)
	if err != nil || h.Size == 0 {
		if f != nil {
			_ = f.Close()
		}
		return service.Upload{}, func() {}
	}
	return service.Upload{Reader: f}, func() { _ = f.Close() }
}

func safeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") && !strings.HasPrefix(next, "/\\") {
		return next
	}
	return ""
}

func (a *App) csrf(r *http.Request) string { return middleware.CSRFToken(r.Context()) }

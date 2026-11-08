package handler

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/config"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/middleware"
	"github.com/suwahadi/mabarbadmin/internal/service"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

type App struct {
	Cfg      config.Config
	Svc      *service.Service
	Q        *sqlc.Queries
	Settings *settings.Store
	Log      *slog.Logger
	Version  string

	views      *views
	staticFS   fs.FS
	loginLimit *middleware.RateLimiter

	loc atomic.Pointer[time.Location]

	cacheMu   sync.Mutex
	siteCache *Site
	homeCache *homeData
	homeAt    time.Time
}

func New(cfg config.Config, svc *service.Service, log *slog.Logger, version string, templates, static fs.FS) (*App, error) {
	a := &App{
		Cfg: cfg, Svc: svc, Q: svc.Q, Settings: svc.Settings, Log: log, Version: version,
		loginLimit: middleware.NewRateLimiter(5, 15*time.Minute),
	}
	sub, err := fs.Sub(static, "static")
	if err != nil {
		return nil, err
	}
	a.staticFS = sub
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	a.loc.Store(jakarta)
	if a.views, err = loadViews(templates, sub, a.funcs()); err != nil {
		return nil, err
	}
	return a, nil
}

// Invalidate drops every in-memory cache after an admin change so it shows up immediately.
func (a *App) Invalidate() {
	a.Settings.Invalidate()
	a.cacheMu.Lock()
	a.siteCache = nil
	a.homeCache = nil
	a.cacheMu.Unlock()
}

func (a *App) settings(ctx context.Context) settings.Values {
	v, err := a.Settings.Get(ctx)
	if err != nil {
		a.Log.Error("load settings", "err", err)
		return settings.Values{}
	}
	a.loc.Store(v.Location())
	return v
}

func (a *App) Loc() *time.Location { return a.loc.Load() }

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	a.routes(mux)
	csrf := middleware.CSRF{Secret: a.Cfg.SessionSecret, Secure: a.Cfg.SecureCookies(), OnFail: http.HandlerFunc(a.csrfFailed)}
	return middleware.Chain(mux,
		middleware.Recover(a.Log),
		middleware.Logging(a.Log),
		middleware.SecurityHeaders,
		middleware.MaxBody(12<<20),
		csrf.Handler,
		a.loadUser,
		a.invalidateOnAdminWrite,
	)
}

func (a *App) csrfFailed(w http.ResponseWriter, r *http.Request) {
	a.renderError(w, r, http.StatusForbidden, "Sesi formulir kedaluwarsa. Muat ulang halaman lalu coba lagi.")
}

func (a *App) invalidateOnAdminWrite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if r.Method == http.MethodPost {
			a.cacheMu.Lock()
			a.homeCache = nil
			a.cacheMu.Unlock()
		}
	})
}

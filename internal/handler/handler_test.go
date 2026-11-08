package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/suwahadi/mabarbadmin/internal/config"
	"github.com/suwahadi/mabarbadmin/internal/content"
	"github.com/suwahadi/mabarbadmin/internal/database"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/media"
	"github.com/suwahadi/mabarbadmin/internal/service"
	"github.com/suwahadi/mabarbadmin/internal/settings"
	"github.com/suwahadi/mabarbadmin/web"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, pool, "up"); err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(pool)
	if err := settings.Seed(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	if err := content.SeedPages(ctx, q); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.New(pool, settings.NewStore(q), media.NewService(t.TempDir(), log), log)
	cfg := config.Config{BaseURL: "https://mabarbadmin.web.id", SessionSecret: []byte("test-secret-test-secret-test-secret!")}
	app, err := New(cfg, svc, log, "test", web.Templates, web.Static)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestPublicRoutes(t *testing.T) {
	app := newTestApp(t)
	h := app.Handler()
	for _, path := range []string{"/", "/sessions", "/about", "/contact", "/terms", "/privacy", "/faq", "/leaderboard", "/announcements", "/login", "/register", "/healthz"} {
		rec := get(t, h, path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", path, rec.Code)
		}
	}
	if rec := get(t, h, "/does-not-exist"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown slug: %d", rec.Code)
	}
	if rec := get(t, h, "/admin"); rec.Code != http.StatusSeeOther {
		t.Errorf("admin without login: %d", rec.Code)
	}

	home := get(t, h, "/").Body.String()
	for _, want := range []string{`<html lang="id">`, `"@type":"Organization"`, `"@type":"WebSite"`, `og:locale" content="id_ID"`, `rel="canonical" href="https://mabarbadmin.web.id/"`, "MabarBadmin"} {
		if !strings.Contains(home, want) {
			t.Errorf("home missing %q", want)
		}
	}
	if strings.Count(home, "<h1") != 1 {
		t.Errorf("home must have exactly one h1, got %d", strings.Count(home, "<h1"))
	}
	faq := get(t, h, "/faq").Body.String()
	if !strings.Contains(faq, `"@type":"FAQPage"`) || !strings.Contains(faq, `"@type":"BreadcrumbList"`) {
		t.Error("faq JSON-LD missing")
	}
	robots := get(t, h, "/robots.txt").Body.String()
	if !strings.Contains(robots, "Disallow: /admin") || !strings.Contains(robots, "Sitemap: https://mabarbadmin.web.id/sitemap.xml") {
		t.Errorf("robots: %s", robots)
	}
	if sm := get(t, h, "/sitemap.xml").Body.String(); !strings.Contains(sm, "<loc>https://mabarbadmin.web.id/about</loc>") {
		t.Errorf("sitemap: %s", sm)
	}
	csp := get(t, h, "/").Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe-eval") {
		t.Errorf("csp: %s", csp)
	}
}

func TestIndexingSwitch(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	if err := app.Settings.Save(ctx, app.Q, map[string]any{settings.SEOAllowIndexing: false}); err != nil {
		t.Fatal(err)
	}
	app.Invalidate()
	h := app.Handler()
	if robots := get(t, h, "/robots.txt").Body.String(); !strings.Contains(robots, "Disallow: /\n") {
		t.Errorf("robots: %s", robots)
	}
	if !strings.Contains(get(t, h, "/").Body.String(), `name="robots" content="noindex`) {
		t.Error("noindex meta missing")
	}
}

func TestPostWithoutCSRFIsRejected(t *testing.T) {
	app := newTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(url.Values{"phone": {"0812"}, "password": {"x"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestPageSlugRedirect(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	custom, err := app.Svc.SavePage(ctx, service.AdminActor(0), 0, service.PageInput{Slug: "cara-bayar", Title: "Cara Bayar", Published: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.Svc.SavePage(ctx, service.AdminActor(0), custom.ID, service.PageInput{Slug: "cara-membayar", Title: custom.Title, Published: true})
	if err != nil {
		t.Fatal(err)
	}
	rec := get(t, app.Handler(), "/cara-bayar")
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/cara-membayar" {
		t.Fatalf("got %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if _, err := app.Svc.SavePage(ctx, service.AdminActor(0), 0, service.PageInput{Slug: "admin", Title: "X"}); err == nil {
		t.Fatal("reserved slug accepted")
	}
}

func TestDefaultPagesAreProtected(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	faq, err := app.Q.GetPageBySlug(ctx, "faq")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Svc.DeletePage(ctx, service.AdminActor(0), faq.ID); err == nil {
		t.Fatal("default page must not be deletable")
	}
	if _, err := app.Svc.SavePage(ctx, service.AdminActor(0), faq.ID, service.PageInput{Slug: "tanya-jawab", Title: faq.Title, Published: true}); err == nil {
		t.Fatal("default page slug must not change")
	}
	custom, err := app.Svc.SavePage(ctx, service.AdminActor(0), 0, service.PageInput{Slug: "cara-bayar", Title: "Cara Bayar", Published: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Svc.DeletePage(ctx, service.AdminActor(0), custom.ID); err != nil {
		t.Fatalf("custom page should be deletable: %v", err)
	}
}

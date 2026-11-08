package handler

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/locale"
	"github.com/suwahadi/mabarbadmin/internal/media"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

func (a *App) healthz(w http.ResponseWriter, r *http.Request) {
	if err := a.Svc.Pool.Ping(r.Context()); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = fmt.Fprintf(w, "ok %s\n", a.Version)
}

var hashedAsset = regexp.MustCompile(`\.[0-9a-f]{8,}\.(css|js)$`)

func (a *App) staticHandler() http.Handler {
	files := http.FileServerFS(a.staticFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") || r.URL.Path == "manifest.json" {
			http.NotFound(w, r)
			return
		}
		if hashedAsset.MatchString(r.URL.Path) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		files.ServeHTTP(w, r)
	})
}

var mediaKeyPattern = regexp.MustCompile(`^[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{24}-\d+\.(webp|jpg|png)$`)

// publicMedia serves processed images; file names are random and never reused, so they are immutable.
func (a *App) publicMedia(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !mediaKeyPattern.MatchString(key) {
		http.NotFound(w, r)
		return
	}
	p := filepath.Join(a.Svc.Media.PublicDir(), filepath.FromSlash(key))
	f, err := os.Open(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(key)))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// servePrivateMedia streams a payment proof after the caller checked access.
func (a *App) servePrivateMedia(w http.ResponseWriter, r *http.Request, mediaID int64) {
	m, err := a.Q.GetMedia(r.Context(), mediaID)
	if err != nil || !m.IsPrivate {
		a.notFound(w, r)
		return
	}
	files := a.Svc.Media.Files(m)
	if len(files) == 0 {
		a.notFound(w, r)
		return
	}
	f, err := os.Open(files[len(files)-1])
	if err != nil {
		a.notFound(w, r)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Robots-Tag", "noindex")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

func (a *App) robots(w http.ResponseWriter, r *http.Request) {
	vals := a.settings(r.Context())
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if !vals.Bool(settings.SEOAllowIndexing) {
		_, _ = fmt.Fprint(w, "User-agent: *\nDisallow: /\n")
		return
	}
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	for _, p := range []string{"/admin", "/me", "/tickets", "/login", "/register"} {
		b.WriteString("Disallow: " + p + "\n")
	}
	b.WriteString("\nSitemap: " + a.Cfg.BaseURL + "/sitemap.xml\n")
	_, _ = fmt.Fprint(w, b.String())
}

type urlEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type urlSet struct {
	XMLName xml.Name   `xml:"urlset"`
	NS      string     `xml:"xmlns,attr"`
	URLs    []urlEntry `xml:"url"`
}

func (a *App) sitemap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	set := urlSet{NS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	set.URLs = append(set.URLs, urlEntry{Loc: a.Cfg.BaseURL + "/"}, urlEntry{Loc: a.Cfg.BaseURL + "/sessions"})
	pages, err := a.Q.ListPublishedPages(ctx)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	for _, p := range pages {
		set.URLs = append(set.URLs, urlEntry{Loc: a.Cfg.BaseURL + "/" + p.Slug, LastMod: p.UpdatedAt.Format("2006-01-02")})
	}
	sessions, err := a.Q.ListSitemapSessions(ctx)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	for _, s := range sessions {
		set.URLs = append(set.URLs, urlEntry{Loc: a.Cfg.BaseURL + "/sessions/" + s.Slug, LastMod: s.UpdatedAt.Format("2006-01-02")})
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_ = enc.Encode(set)
}

func (a *App) faviconMedia(r *http.Request) (sqlc.Medium, bool) {
	id := a.settings(r.Context()).MediaID(settings.FaviconMediaID)
	if id == nil {
		return sqlc.Medium{}, false
	}
	m, err := a.Q.GetMedia(r.Context(), *id)
	return m, err == nil
}

func (a *App) webmanifest(w http.ResponseWriter, r *http.Request) {
	site := a.site(r.Context())
	type icon struct {
		Src   string `json:"src"`
		Sizes string `json:"sizes"`
		Type  string `json:"type"`
	}
	var icons []icon
	if m, ok := a.faviconMedia(r); ok {
		for _, v := range media.Variants(m) {
			if v.Width == 192 || v.Width == 512 {
				icons = append(icons, icon{Src: "/media/" + v.File, Sizes: fmt.Sprintf("%dx%d", v.Width, v.Height), Type: "image/png"})
			}
		}
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"name": site.Name, "short_name": site.Name, "description": site.Tagline, "start_url": "/", "display": "standalone",
		"lang": "id", "background_color": "#ffffff", "theme_color": "#15803d", "icons": icons,
	})
}

func (a *App) faviconICO(w http.ResponseWriter, r *http.Request) {
	if m, ok := a.faviconMedia(r); ok {
		http.Redirect(w, r, variantURL(m, 32), http.StatusFound)
		return
	}
	if _, err := fs.Stat(a.staticFS, "img/favicon.svg"); err == nil {
		http.Redirect(w, r, "/static/img/favicon.svg", http.StatusFound)
		return
	}
	http.NotFound(w, r)
}

func datePhrase(t time.Time, loc *time.Location) string {
	return locale.FormatDate(t, loc) + " " + locale.FormatTime(t, loc)
}

func rupiahText(n int64) string { return locale.FormatRupiah(n) }

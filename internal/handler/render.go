package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/suwahadi/mabarbadmin/internal/content"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/locale"
	"github.com/suwahadi/mabarbadmin/internal/middleware"
	"github.com/suwahadi/mabarbadmin/internal/seo"
)

type views struct {
	pages    map[string]*template.Template
	manifest map[string]string
	sprite   template.HTML
}

// loadViews parses layouts and partials once, then clones them for every page so each page can
// define its own "content" block.
func loadViews(templates, static fs.FS, funcs template.FuncMap) (*views, error) {
	v := &views{pages: map[string]*template.Template{}, manifest: map[string]string{}}
	if raw, err := fs.ReadFile(static, "manifest.json"); err == nil {
		_ = json.Unmarshal(raw, &v.manifest)
	}
	if raw, err := fs.ReadFile(static, "icons.svg"); err == nil {
		v.sprite = template.HTML(raw)
	}
	funcs["asset"] = v.asset
	funcs["iconSprite"] = func() template.HTML { return v.sprite }

	base := template.New("base").Funcs(funcs)
	for _, pattern := range []string{"templates/layouts/*.html", "templates/partials/*.html"} {
		var err error
		if base, err = base.ParseFS(templates, pattern); err != nil {
			return nil, fmt.Errorf("parse %s: %w", pattern, err)
		}
	}
	err := fs.WalkDir(templates, "templates/pages", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		t, err := base.Clone()
		if err != nil {
			return err
		}
		if _, err := t.ParseFS(templates, p); err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		name := strings.TrimSuffix(strings.TrimPrefix(p, "templates/pages/"), ".html")
		v.pages[name] = t
		return nil
	})
	return v, err
}

func (v *views) asset(p string) string {
	if hashed, ok := v.manifest[p]; ok {
		return "/static/" + hashed
	}
	return "/static/" + p
}

type Flash struct {
	Kind string
	Msg  string
}

type View struct {
	Site         *Site
	Meta         seo.Meta
	User         *sqlc.User
	CSRF         string
	Flash        *Flash
	Path         string
	Admin        bool
	Section      string
	Announcement *sqlc.Announcement
	PreloadTags  template.HTML
	Data         map[string]any
}

func (a *App) newView(w http.ResponseWriter, r *http.Request, title string) *View {
	site := a.site(r.Context())
	v := &View{
		Site: site,
		User: currentUser(r),
		CSRF: middleware.CSRFToken(r.Context()),
		Path: r.URL.Path,
		Data: map[string]any{},
		Meta: seo.Meta{
			Title:       seo.Title(site.TitlePattern, title, site.Name),
			Description: site.Description,
			Keywords:    site.Keywords,
			Canonical:   a.Cfg.BaseURL + r.URL.Path,
			Image:       site.OGImage,
			Type:        "website",
			NoIndex:     !site.AllowIndexing,
		},
		Flash: popFlash(w, r),
	}
	if strings.HasPrefix(r.URL.Path, "/admin") {
		v.Admin = true
		v.Meta.NoIndex = true
	} else {
		v.Announcement = a.liveAnnouncement(r)
	}
	return v
}

func (a *App) render(w http.ResponseWriter, r *http.Request, status int, page string, v *View) {
	t, ok := a.views.pages[page]
	if !ok {
		a.serverError(w, r, fmt.Errorf("template %q not found", page))
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", v); err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if v.User != nil || v.Admin {
		w.Header().Set("Cache-Control", "private, no-store")
	}
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// renderBlock writes one named block of a page, used for HTMX partial responses.
func (a *App) renderBlock(w http.ResponseWriter, r *http.Request, page, block string, data any) {
	t, ok := a.views.pages[page]
	if !ok {
		a.serverError(w, r, fmt.Errorf("template %q not found", page))
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, block, data); err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = buf.WriteTo(w)
}

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

func (a *App) funcs() template.FuncMap {
	loc := a.Loc
	return template.FuncMap{
		"rupiah":     locale.FormatRupiah,
		"thousands":  locale.FormatThousands,
		"datetime":   func(t time.Time) string { return locale.FormatDateTime(t, loc()) },
		"date":       func(t time.Time) string { return locale.FormatDate(t, loc()) },
		"shortdate":  func(t time.Time) string { return locale.FormatShortDate(t, loc()) },
		"clock":      func(t time.Time) string { return locale.FormatTime(t, loc()) },
		"timerange":  func(s, e time.Time) string { return locale.FormatTimeRange(s, e, loc()) },
		"month":      locale.FormatMonth,
		"weekday":    func(d int16) string { return locale.WeekdayName(time.Weekday(d)) },
		"day":        func(t time.Time) int { return t.In(loc()).Day() },
		"monthShort": func(t time.Time) string { return locale.MonthName(t.In(loc()).Month())[:3] },
		"inputDT": func(t any) string {
			switch v := t.(type) {
			case time.Time:
				if v.IsZero() {
					return ""
				}
				return v.In(loc()).Format("2006-01-02T15:04")
			case *time.Time:
				if v == nil {
					return ""
				}
				return v.In(loc()).Format("2006-01-02T15:04")
			}
			return ""
		},
		"inputDate": func(t time.Time) string { return t.In(loc()).Format("2006-01-02") },
		"phone":     locale.FormatPhone,
		"maskPhone": locale.MaskPhone,
		"waLink":    waLink,
		"status":    regStatusLabel,
		"payStatus": paymentStatusLabel,
		"payTone":   paymentTone,
		"sessStatus": func(s string) string {
			return sessionStatusLabel[s]
		},
		"level":  levelLabel,
		"method": methodLabel,
		"icon":   icon,
		"md":     content.Render,
		"add":    func(a, b int) int { return a + b },
		"sub":    func(a, b int) int { return a - b },
		"add64":  func(a, b int64) int64 { return a + b },
		"sub64":  func(a, b int64) int64 { return a - b },
		"int":    func(v any) int { return toInt(v) },
		"pct":    func(part, total any) int { return percent(toInt(part), toInt(total)) },
		"seq":    seq,
		"list":   func(v ...string) []string { return v },
		"clockText": func(t pgtype.Time) string {
			m := t.Microseconds / 60e6
			return fmt.Sprintf("%02d:%02d", m/60, m%60)
		},
		"dict":  dict,
		"deref": deref,
		"derefT": func(t *time.Time) time.Time {
			if t == nil {
				return time.Time{}
			}
			return *t
		},
		"hasPrefix": strings.HasPrefix,
		"join":      strings.Join,
		"nl2br": func(s string) template.HTML {
			return template.HTML(strings.ReplaceAll(template.HTMLEscapeString(s), "\n", "<br>"))
		},
		"base":   path.Base,
		"lower":  strings.ToLower,
		"now":    time.Now,
		"before": func(a, b time.Time) bool { return a.Before(b) },
		"isZero": func(t time.Time) bool { return t.IsZero() },
		"medal":  medal,
		"bytes":  formatBytes,
	}
}

func icon(name string, class ...string) template.HTML {
	c := "icon"
	if len(class) > 0 {
		c += " " + class[0]
	}
	return template.HTML(fmt.Sprintf(`<svg class="%s" aria-hidden="true" focusable="false"><use href="#icon-%s"></use></svg>`,
		template.HTMLEscapeString(c), template.HTMLEscapeString(name)))
}

func medal(rank int) string {
	switch rank {
	case 1:
		return "medal-gold"
	case 2:
		return "medal-silver"
	case 3:
		return "medal-bronze"
	}
	return ""
}

func waLink(phone string) string {
	p, err := locale.NormalizePhone(phone)
	if err != nil {
		return ""
	}
	return "https://wa.me/" + p
}

func dict(kv ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(kv); i += 2 {
		k, _ := kv[i].(string)
		m[k] = kv[i+1]
	}
	return m
}

func deref(v any) any {
	switch p := v.(type) {
	case *int64:
		if p == nil {
			return int64(0)
		}
		return *p
	case *string:
		if p == nil {
			return ""
		}
		return *p
	}
	return v
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	}
	return 0
}

func percent(part, total int) int {
	if total <= 0 {
		return 0
	}
	p := part * 100 / total
	return min(max(p, 0), 100)
}

var regStatusLabels = map[string]string{
	"registered": "Terdaftar", "waitlisted": "Waiting list", "cancelled": "Batal",
	"transferred": "Dioper", "attended": "Hadir", "no_show": "Tidak datang",
}

var paymentStatusLabels = map[string]string{
	"unpaid": "Belum bayar", "pending": "Menunggu konfirmasi", "paid": "Lunas", "rejected": "Ditolak",
	"refund_pending": "Menunggu refund", "refunded": "Sudah di-refund", "forfeited": "Hangus",
}

var sessionStatusLabel = map[string]string{
	"draft": "Draf", "published": "Terbit", "closed": "Ditutup", "cancelled": "Dibatalkan",
}

var levelLabels = map[string]string{"beginner": "Pemula", "intermediate": "Menengah", "advanced": "Mahir"}

func regStatusLabel(s string) string     { return regStatusLabels[s] }
func paymentStatusLabel(s string) string { return paymentStatusLabels[s] }
func levelLabel(s string) string         { return levelLabels[s] }

func methodLabel(m string) string {
	switch m {
	case "qris":
		return "QRIS"
	case "bank_transfer":
		return "Transfer bank"
	case "cash":
		return "Tunai/manual"
	}
	return m
}

func paymentTone(s string) string {
	switch s {
	case "paid", "refunded":
		return "ok"
	case "pending", "refund_pending":
		return "warn"
	case "rejected", "forfeited":
		return "bad"
	}
	return "muted"
}

func seq(n int) []int {
	if n < 0 {
		n = 0
	}
	s := make([]int, n)
	for i := range s {
		s[i] = i + 1
	}
	return s
}

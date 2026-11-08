package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/auth"
	"github.com/suwahadi/mabarbadmin/internal/content"
	"github.com/suwahadi/mabarbadmin/internal/database"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
	"github.com/suwahadi/mabarbadmin/internal/middleware"
	"github.com/suwahadi/mabarbadmin/internal/seo"
	"github.com/suwahadi/mabarbadmin/internal/service"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

type HeroSlide struct {
	sqlc.HeroSlide
	Desktop *Picture
	Mobile  *Picture
}

type homeData struct {
	Slides      []HeroSlide
	Upcoming    []sqlc.ListUpcomingPublishedRow
	Leaderboard []service.LeaderboardEntry
}

func (a *App) loadHome(r *http.Request) (*homeData, error) {
	a.cacheMu.Lock()
	if a.homeCache != nil && time.Since(a.homeAt) < time.Minute {
		h := a.homeCache
		a.cacheMu.Unlock()
		return h, nil
	}
	a.cacheMu.Unlock()

	ctx := r.Context()
	vals := a.settings(ctx)
	h := &homeData{}
	upcoming, err := a.Q.ListUpcomingPublished(ctx, int32(vals.Int(settings.HomeUpcomingCount))+1)
	if err != nil {
		return nil, err
	}
	h.Upcoming = upcoming
	slides, err := a.Q.ListLiveHeroSlides(ctx)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, s := range slides {
		ids = append(ids, s.ImageMediaID)
		if s.MobileImageMediaID != nil {
			ids = append(ids, *s.MobileImageMediaID)
		}
	}
	byID := map[int64]sqlc.Medium{}
	if len(ids) > 0 {
		ms, err := a.Q.GetMediaByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			byID[m.ID] = m
		}
	}
	for _, s := range slides {
		hs := HeroSlide{HeroSlide: s, Desktop: pictureOf(byID[s.ImageMediaID])}
		if s.MobileImageMediaID != nil {
			hs.Mobile = pictureOf(byID[*s.MobileImageMediaID])
		}
		if hs.Desktop != nil {
			h.Slides = append(h.Slides, hs)
		}
	}
	if vals.Bool(settings.LeaderboardPublic) {
		h.Leaderboard, _, _ = a.Svc.Leaderboard(ctx, "this_month", 5, vals.Location())
	}
	a.cacheMu.Lock()
	a.homeCache, a.homeAt = h, time.Now()
	a.cacheMu.Unlock()
	return h, nil
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	h, err := a.loadHome(r)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.newView(w, r, "")
	site := v.Site
	v.Data["Home"] = h
	if len(h.Slides) > 0 {
		v.PreloadTags = preloadTags(h.Slides[0])
	}
	sameAs := make([]string, 0, len(site.Social))
	for _, s := range site.Social {
		sameAs = append(sameAs, s.URL)
	}
	logo := ""
	if site.LogoLight != nil {
		logo = a.Cfg.BaseURL + site.LogoLight.Src
	}
	v.Meta.Canonical = a.Cfg.BaseURL + "/"
	v.Meta.JSONLD = append(v.Meta.JSONLD,
		seo.JSONLD(seo.Organization(site.Name, a.Cfg.BaseURL+"/", logo, sameAs)),
		seo.JSONLD(seo.WebSite(site.Name, a.Cfg.BaseURL+"/")))
	a.render(w, r, http.StatusOK, "public/home", v)
}

func (a *App) sessionsList(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListUpcomingPublished(r.Context(), 100)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.newView(w, r, "Jadwal Mabar")
	v.Data["Sessions"] = rows
	a.render(w, r, http.StatusOK, "public/sessions", v)
}

func (a *App) findSession(w http.ResponseWriter, r *http.Request) (sqlc.Session, bool) {
	slug := r.PathValue("slug")
	sess, err := a.Q.GetSessionBySlug(r.Context(), slug)
	if err == nil && sess.Status != domain.SessionDraft {
		return sess, true
	}
	if err == nil || database.IsNotFound(err) {
		if to, rerr := a.Q.GetSlugRedirect(r.Context(), sqlc.GetSlugRedirectParams{EntityType: "session", OldSlug: slug}); rerr == nil {
			target := strings.Replace(r.URL.Path, "/sessions/"+slug, "/sessions/"+to, 1)
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return sess, false
		}
		a.notFound(w, r)
		return sess, false
	}
	a.serverError(w, r, err)
	return sess, false
}

func (a *App) sessionShow(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.findSession(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	page, err := a.loadSessionPage(ctx, sess)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.newView(w, r, "")
	loc := a.Loc()
	autoTitle := fmt.Sprintf("Mabar %s, %s di %s", sess.Title, strings.ToLower(datePhrase(sess.StartsAt, loc)), page.Venue.Name)
	v.Meta.Title = seo.Title(v.Site.TitlePattern, seo.First(sess.MetaTitle, autoTitle), v.Site.Name)
	autoDesc := fmt.Sprintf("%s, %s di %s. HTM %s. Sisa %d dari %d slot. Daftar dan bayar lewat link ini.",
		sess.Title, datePhrase(sess.StartsAt, loc), page.Venue.Name, rupiahText(sess.Price), max(page.Remaining, 0), sess.Capacity)
	v.Meta.Description = seo.Truncate(seo.First(sess.MetaDescription, autoDesc, v.Site.Description), 160)
	v.Meta.Type = "event"
	v.Meta.Canonical = a.Cfg.BaseURL + "/sessions/" + sess.Slug
	if sess.Status != domain.SessionPublished || sess.EndsAt.Before(time.Now()) {
		v.Meta.NoIndex = true
	}
	v.Meta.JSONLD = append(v.Meta.JSONLD, seo.JSONLD(seo.SportsEvent(seo.Event{
		Name: autoTitle, Description: v.Meta.Description, URL: v.Meta.Canonical, Image: v.Site.OGImage,
		Start: sess.StartsAt.In(loc).Format(time.RFC3339), End: sess.EndsAt.In(loc).Format(time.RFC3339),
		VenueName: page.Venue.Name, VenueAddress: page.Venue.Address, Price: sess.Price,
		Available: page.Remaining > 0, Cancelled: sess.Status == domain.SessionCancelled,
		OrganizerName: v.Site.Name, OrganizerURL: a.Cfg.BaseURL, MaxCapacity: int(sess.Capacity), RemainingCapacity: max(page.Remaining, 0),
	})))
	v.Data["Page"] = page
	v.Data["Open"] = page.Open(time.Now())
	if u := v.User; u != nil {
		for _, reg := range page.All {
			if reg.UserID != nil && *reg.UserID == u.ID && reg.Status != domain.StatusTransferred {
				panel, err := a.seatPanel(ctx, reg, fmt.Sprintf("/me/registrations/%d", reg.ID), "/me/uploads", time.Now())
				if err != nil {
					a.serverError(w, r, err)
					return
				}
				panel.CSRF = v.CSRF
				panel.NewTicket = newTicketLink(a.Cfg.BaseURL, r.URL.Query().Get("new_ticket"))
				v.Data["Panel"] = panel
				break
			}
		}
	}
	a.render(w, r, http.StatusOK, "public/session", v)
}

func newTicketLink(base, token string) string {
	if token == "" || len(token) > 100 {
		return ""
	}
	return base + "/tickets/" + url.PathEscape(token)
}

func (a *App) sessionJoin(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.findSession(w, r)
	if !ok {
		return
	}
	back := "/sessions/" + sess.Slug
	u := currentUser(r)
	in := service.RegisterInput{Name: r.FormValue("name"), Phone: r.FormValue("phone"), Level: r.FormValue("level")}
	actor := service.GuestActor()
	if u != nil {
		in = service.RegisterInput{UserID: &u.ID, Name: u.Name, Phone: u.Phone, Level: u.Level}
		actor = service.MemberActor(u.ID)
	}
	res, err := a.Svc.Register(r.Context(), actor, sess.ID, in)
	if err != nil {
		if msg, ok := service.IsUserError(err); ok && u == nil && strings.Contains(msg, "masuk dulu") {
			setFlash(w, "error", msg)
			a.redirect(w, r, "/login?next="+url.QueryEscape(back))
			return
		}
		a.fail(w, r, back+"#daftar", err)
		return
	}
	msg := "Kamu terdaftar. Silakan bayar agar slotmu aman."
	if res.Registration.Status == domain.StatusWaitlisted {
		msg = "Kuota penuh, kamu masuk waiting list. Yang sudah lunas diprioritaskan naik ke slot."
	}
	if u != nil {
		a.done(w, r, back+"#tiket", msg)
		return
	}
	a.done(w, r, "/tickets/"+res.TicketToken+"?new=1", msg)
}

func (a *App) ticketReg(w http.ResponseWriter, r *http.Request) (sqlc.Registration, string, bool) {
	token := r.PathValue("token")
	reg, err := a.Q.GetRegistrationByTicket(r.Context(), auth.HashToken(token))
	if err != nil {
		if database.IsNotFound(err) {
			a.renderError(w, r, http.StatusNotFound, "Link tiket tidak valid. Periksa kembali link yang kamu simpan.")
		} else {
			a.serverError(w, r, err)
		}
		return reg, token, false
	}
	return reg, token, true
}

func (a *App) ticketShow(w http.ResponseWriter, r *http.Request) {
	reg, token, ok := a.ticketReg(w, r)
	if !ok {
		return
	}
	base := "/tickets/" + token
	panel, err := a.seatPanel(r.Context(), reg, base, base+"/uploads", time.Now())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.newView(w, r, "Tiket "+reg.DisplayName)
	v.Meta.NoIndex = true
	panel.CSRF = v.CSRF
	panel.NewTicket = newTicketLink(a.Cfg.BaseURL, r.URL.Query().Get("new_ticket"))
	v.Data["Panel"] = panel
	v.Data["TicketURL"] = a.Cfg.BaseURL + base
	v.Data["IsNew"] = r.URL.Query().Get("new") == "1"
	w.Header().Set("Referrer-Policy", "no-referrer")
	a.render(w, r, http.StatusOK, "public/ticket", v)
}

func (a *App) ticketUpload(w http.ResponseWriter, r *http.Request) {
	reg, token, ok := a.ticketReg(w, r)
	if !ok {
		return
	}
	a.handleUpload(w, r, service.GuestActor(), reg.ID, "/tickets/"+token)
}

func (a *App) handleUpload(w http.ResponseWriter, r *http.Request, actor service.Actor, regID int64, back string) {
	up, closeFile := formFile(r, "proof")
	defer closeFile()
	if up.Reader == nil {
		setFlash(w, "error", "Pilih file bukti bayar (JPG, PNG, atau WebP, maks 5 MB).")
		a.redirect(w, r, back)
		return
	}
	if err := a.Svc.UploadProof(r.Context(), actor, regID, r.FormValue("payment_method"), up.Reader); err != nil {
		a.fail(w, r, back, err)
		return
	}
	a.done(w, r, back, "Bukti bayar terkirim. Admin akan mengonfirmasi secepatnya.")
}

func refundAccount(r *http.Request) service.RefundAccount {
	return service.RefundAccount{BankName: r.FormValue("refund_bank_name"), AccountNumber: r.FormValue("refund_account_number"), AccountName: r.FormValue("refund_account_name")}
}

func (a *App) ticketCancel(w http.ResponseWriter, r *http.Request) {
	reg, token, ok := a.ticketReg(w, r)
	if !ok {
		return
	}
	a.handleCancel(w, r, service.GuestActor(), reg, "/tickets/"+token)
}

func (a *App) handleCancel(w http.ResponseWriter, r *http.Request, actor service.Actor, reg sqlc.Registration, back string) {
	outcome, err := a.Svc.Cancel(r.Context(), actor, reg.ID, refundAccount(r))
	if err != nil {
		a.fail(w, r, back, err)
		return
	}
	msg := "Pendaftaran dibatalkan."
	switch outcome {
	case domain.PaymentRefundPending:
		msg = "Pendaftaran dibatalkan. Refund masuk antrean admin."
	case domain.PaymentForfeited:
		msg = "Pendaftaran dibatalkan. Karena lewat batas refund, pembayaran hangus."
	}
	a.done(w, r, back, msg)
}

func (a *App) ticketTransfer(w http.ResponseWriter, r *http.Request) {
	reg, token, ok := a.ticketReg(w, r)
	if !ok {
		return
	}
	a.handleTransfer(w, r, service.GuestActor(), reg, "/tickets/"+token)
}

func (a *App) handleTransfer(w http.ResponseWriter, r *http.Request, actor service.Actor, reg sqlc.Registration, back string) {
	res, err := a.Svc.Transfer(r.Context(), actor, reg.ID, r.FormValue("name"), r.FormValue("phone"), r.FormValue("level"))
	if err != nil {
		a.fail(w, r, back, err)
		return
	}
	sep := "?"
	if strings.Contains(back, "?") {
		sep = "&"
	}
	a.done(w, r, back+sep+"new_ticket="+url.QueryEscape(res.TicketToken), "Slot sudah dioper. Kirim link tiket di bawah ke "+res.Registration.DisplayName+".")
}

func (a *App) ticketRefundAccount(w http.ResponseWriter, r *http.Request) {
	reg, token, ok := a.ticketReg(w, r)
	if !ok {
		return
	}
	back := "/tickets/" + token
	if err := a.Svc.UpdateRefundAccount(r.Context(), service.GuestActor(), reg.ID, refundAccount(r)); err != nil {
		a.fail(w, r, back, err)
		return
	}
	a.done(w, r, back, "Rekening refund disimpan.")
}

func (a *App) ticketProofImage(w http.ResponseWriter, r *http.Request) {
	reg, _, ok := a.ticketReg(w, r)
	if !ok {
		return
	}
	a.serveProof(w, r, pathID(r), func(p sqlc.GetProofByMediaRow) bool { return p.RegistrationID == reg.ID })
}

func (a *App) rotationBoard(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.findSession(w, r)
	if !ok {
		return
	}
	data, err := a.rotationData(r, sess)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if isHTMX(r) {
		a.renderBlock(w, r, "public/rotation", "board", data)
		return
	}
	v := a.newView(w, r, "Rotasi "+sess.Title)
	v.Meta.NoIndex = true
	v.Data["Board"] = data
	a.render(w, r, http.StatusOK, "public/rotation", v)
}

func (a *App) announcements(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListPublicAnnouncements(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.newView(w, r, "Pengumuman")
	v.Data["Announcements"] = rows
	a.render(w, r, http.StatusOK, "public/announcements", v)
}

func (a *App) leaderboard(w http.ResponseWriter, r *http.Request) {
	vals := a.settings(r.Context())
	if !vals.Bool(settings.LeaderboardPublic) {
		a.notFound(w, r)
		return
	}
	entries, key, err := a.Svc.Leaderboard(r.Context(), r.URL.Query().Get("period"), 10, vals.Location())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.newView(w, r, "Peringkat Member")
	v.Meta.Canonical = a.Cfg.BaseURL + "/leaderboard"
	v.Data["Entries"] = entries
	v.Data["Period"] = key
	v.Data["Periods"] = service.Periods
	a.render(w, r, http.StatusOK, "public/leaderboard", v)
}

func (a *App) loginForm(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/me", http.StatusSeeOther)
		return
	}
	v := a.newView(w, r, "Masuk")
	v.Meta.NoIndex = true
	v.Data["Next"] = safeNext(r.URL.Query().Get("next"))
	a.render(w, r, http.StatusOK, "public/login", v)
}

func (a *App) loginSubmit(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.FormValue("phone"))
	next := safeNext(r.FormValue("next"))
	keys := []string{"ip:" + middleware.ClientIP(r), "phone:" + phone}
	back := "/login"
	if next != "" {
		back += "?next=" + url.QueryEscape(next)
	}
	if a.loginLimit.Blocked(keys...) {
		setFlash(w, "error", "Terlalu banyak percobaan masuk. Coba lagi 15 menit lagi.")
		a.redirect(w, r, back)
		return
	}
	u, token, err := a.Svc.Authenticate(r.Context(), phone, r.FormValue("password"), r.UserAgent())
	if err != nil {
		a.loginLimit.Hit(keys...)
		a.fail(w, r, back, err)
		return
	}
	a.loginLimit.Reset("phone:" + phone)
	a.setSessionCookie(w, token)
	switch {
	case u.MustChangePassword:
		next = "/me/password"
	case next == "" && u.Role == "admin":
		next = "/admin"
	case next == "":
		next = "/me"
	}
	a.done(w, r, next, "Halo, "+u.Name+"!")
}

func (a *App) registerForm(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/me", http.StatusSeeOther)
		return
	}
	v := a.newView(w, r, "Daftar Member")
	v.Meta.NoIndex = true
	v.Data["Next"] = safeNext(r.URL.Query().Get("next"))
	a.render(w, r, http.StatusOK, "public/register", v)
}

func (a *App) registerSubmit(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.FormValue("next"))
	if r.FormValue("password") != r.FormValue("password_confirm") {
		setFlash(w, "error", "Konfirmasi password tidak sama.")
		a.redirect(w, r, "/register")
		return
	}
	if a.loginLimit.Blocked("signup:" + middleware.ClientIP(r)) {
		setFlash(w, "error", "Terlalu banyak pendaftaran dari jaringan ini. Coba lagi nanti.")
		a.redirect(w, r, "/register")
		return
	}
	u, err := a.Svc.CreateUser(r.Context(), service.SignupInput{
		Name: r.FormValue("name"), Phone: r.FormValue("phone"), Password: r.FormValue("password"), Level: r.FormValue("level"),
	})
	if err != nil {
		a.fail(w, r, "/register", err)
		return
	}
	a.loginLimit.Hit("signup:" + middleware.ClientIP(r))
	token, err := a.Svc.StartSession(r.Context(), u.ID, r.UserAgent())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.setSessionCookie(w, token)
	if next == "" {
		next = "/me"
	}
	a.done(w, r, next, "Akun berhasil dibuat. Selamat bergabung!")
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = a.Q.DeleteAuthSession(r.Context(), auth.HashToken(c.Value))
	}
	a.clearSessionCookie(w)
	a.done(w, r, "/", "Kamu sudah keluar.")
}

func (a *App) staticPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ctx := r.Context()
	page, err := a.Q.GetPageBySlug(ctx, slug)
	if err != nil || !page.IsPublished {
		if to, rerr := a.Q.GetSlugRedirect(ctx, sqlc.GetSlugRedirectParams{EntityType: "page", OldSlug: slug}); rerr == nil {
			http.Redirect(w, r, "/"+to, http.StatusMovedPermanently)
			return
		}
		if err != nil && !database.IsNotFound(err) {
			a.serverError(w, r, err)
			return
		}
		a.notFound(w, r)
		return
	}
	v := a.newView(w, r, seo.First(page.MetaTitle, page.Title))
	plain := content.PlainText(page.BodyMarkdown)
	v.Meta.Description = seo.Truncate(seo.First(page.MetaDescription, plain, v.Site.Description), 160)
	v.Meta.Keywords = seo.First(page.MetaKeywords, v.Site.Keywords)
	v.Meta.Canonical = a.Cfg.BaseURL + "/" + page.Slug
	var ids []int64
	if page.CoverMediaID != nil {
		ids = append(ids, *page.CoverMediaID)
	}
	if page.OgMediaID != nil {
		ids = append(ids, *page.OgMediaID)
	}
	if len(ids) > 0 {
		ms, err := a.Q.GetMediaByIDs(ctx, ids)
		if err == nil {
			for _, m := range ms {
				if page.CoverMediaID != nil && m.ID == *page.CoverMediaID {
					v.Data["Cover"] = pictureOf(m)
				}
				if page.OgMediaID != nil && m.ID == *page.OgMediaID {
					v.Meta.Image = a.Cfg.BaseURL + variantURL(m, 1200)
				}
			}
		}
	}
	v.Meta.JSONLD = append(v.Meta.JSONLD, seo.JSONLD(seo.BreadcrumbList([]seo.Crumb{
		{Name: "Beranda", URL: a.Cfg.BaseURL + "/"}, {Name: page.Title, URL: v.Meta.Canonical},
	})))
	if page.Slug == "faq" {
		items, err := a.Q.ListPublishedFAQ(ctx)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		qa := make([]seo.QA, len(items))
		for i, it := range items {
			qa[i] = seo.QA{Question: it.Question, Answer: strings.TrimSpace(content.PlainText(it.AnswerMarkdown))}
		}
		if len(qa) > 0 {
			v.Meta.JSONLD = append(v.Meta.JSONLD, seo.JSONLD(seo.FAQPage(qa)))
		}
		v.Data["FAQ"] = items
	}
	v.Data["Page"] = page
	v.Data["IsContact"] = page.Slug == "contact"
	a.render(w, r, http.StatusOK, "public/page", v)
}

func (a *App) serveProof(w http.ResponseWriter, r *http.Request, mediaID int64, allowed func(sqlc.GetProofByMediaRow) bool) {
	proof, err := a.Q.GetProofByMedia(r.Context(), &mediaID)
	if err != nil {
		if database.IsNotFound(err) {
			a.notFound(w, r)
			return
		}
		a.serverError(w, r, err)
		return
	}
	if !allowed(proof) {
		a.notFound(w, r)
		return
	}
	a.servePrivateMedia(w, r, mediaID)
}

// preloadTags builds the LCP image preload links. html/template treats imagesrcset as a single
// URL and would percent-encode its spaces, so the tag is assembled here with explicit escaping.
func preloadTags(s HeroSlide) template.HTML {
	link := func(p *Picture, media string) string {
		m := ""
		if media != "" {
			m = ` media="` + template.HTMLEscapeString(media) + `"`
		}
		return `<link rel="preload" as="image" imagesrcset="` + template.HTMLEscapeString(p.SrcSet) +
			`" imagesizes="100vw" fetchpriority="high"` + m + `>`
	}
	if s.Mobile == nil {
		return template.HTML(link(s.Desktop, ""))
	}
	return template.HTML(link(s.Desktop, "(min-width: 641px)") + "\n" + link(s.Mobile, "(max-width: 640px)"))
}

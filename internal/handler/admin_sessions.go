package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/suwahadi/mabarbadmin/internal/broadcast"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
	"github.com/suwahadi/mabarbadmin/internal/locale"
	"github.com/suwahadi/mabarbadmin/internal/service"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

func (a *App) adminView(w http.ResponseWriter, r *http.Request, title, section string) *View {
	v := a.newView(w, r, title)
	v.Section = section
	v.Data["Title"] = title
	return v
}

func (a *App) adminDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	d, err := a.Svc.Dashboard(ctx, a.Loc())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	board, key, err := a.Svc.Leaderboard(ctx, r.URL.Query().Get("period"), 10, a.Loc())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Dasbor", "dashboard")
	v.Data["D"] = d
	v.Data["Board"] = board
	v.Data["Period"] = key
	v.Data["Periods"] = service.Periods
	var maxAtt int64 = 1
	for _, x := range d.Attendance {
		maxAtt = max(maxAtt, x.Attended)
	}
	var maxMoney int64 = 1
	for _, m := range d.Months {
		maxMoney = max(maxMoney, m.Income, m.Costs)
	}
	v.Data["MaxAtt"] = maxAtt
	v.Data["MaxMoney"] = maxMoney
	a.render(w, r, http.StatusOK, "admin/dashboard", v)
}

func (a *App) adminLeaderboard(w http.ResponseWriter, r *http.Request) {
	board, key, err := a.Svc.Leaderboard(r.Context(), r.URL.Query().Get("period"), 200, a.Loc())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Peringkat Member", "members")
	v.Data["Board"] = board
	v.Data["Period"] = key
	v.Data["Periods"] = service.Periods
	a.render(w, r, http.StatusOK, "admin/leaderboard", v)
}

func (a *App) adminVenues(w http.ResponseWriter, r *http.Request) {
	venues, err := a.Q.ListVenues(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Venue", "sessions")
	v.Data["Venues"] = venues
	a.render(w, r, http.StatusOK, "admin/venues", v)
}

func (a *App) adminVenueSave(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		setFlash(w, "error", "Nama venue wajib diisi.")
		a.redirect(w, r, "/admin/venues")
		return
	}
	addr, maps := strings.TrimSpace(r.FormValue("address")), strings.TrimSpace(r.FormValue("maps_url"))
	var err error
	if id := pathID(r); id > 0 {
		err = a.Q.UpdateVenue(r.Context(), sqlc.UpdateVenueParams{ID: id, Name: name, Address: addr, MapsUrl: maps})
	} else {
		_, err = a.Q.CreateVenue(r.Context(), sqlc.CreateVenueParams{Name: name, Address: addr, MapsUrl: maps})
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.done(w, r, "/admin/venues", "Venue disimpan.")
}

func (a *App) adminVenueDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.Q.DeleteVenue(r.Context(), pathID(r)); err != nil {
		setFlash(w, "error", "Venue masih dipakai sesi atau template, jadi tidak bisa dihapus.")
		a.redirect(w, r, "/admin/venues")
		return
	}
	a.done(w, r, "/admin/venues", "Venue dihapus.")
}

func (a *App) adminTemplates(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListTemplates(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Template Sesi", "sessions")
	v.Data["Templates"] = rows
	dates := map[int64]time.Time{}
	for _, t := range rows {
		dates[t.ID] = service.NextTemplateDate(t.Weekday, time.Now(), a.Loc())
	}
	v.Data["NextDates"] = dates
	a.render(w, r, http.StatusOK, "admin/templates", v)
}

func (a *App) adminTemplateForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	venues, err := a.Q.ListVenues(ctx)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	vals := a.settings(ctx)
	t := sqlc.SessionTemplate{
		Weekday: 1, CourtCount: 2, Capacity: int32(2 * vals.Int(settings.CapacityPerCourt)), PointRule: vals.String(settings.PointRule),
		ShuttlecockIncluded: true, UniqueCodeEnabled: vals.Bool(settings.UniqueCodeDefault),
		StartTime: clockOf(19, 0), EndTime: clockOf(22, 0), LevelLabel: "Semua level",
	}
	if id := pathID(r); id > 0 {
		if t, err = a.Q.GetTemplate(ctx, id); err != nil {
			a.notFound(w, r)
			return
		}
	}
	v := a.adminView(w, r, "Template Sesi", "sessions")
	v.Data["T"] = t
	v.Data["Venues"] = venues
	v.Data["Weekdays"] = []int16{1, 2, 3, 4, 5, 6, 0}
	v.Data["PerCourt"] = vals.Int(settings.CapacityPerCourt)
	a.render(w, r, http.StatusOK, "admin/template_form", v)
}

func clockOf(h, m int) pgtype.Time {
	return pgtype.Time{Microseconds: int64(h*3600+m*60) * 1e6, Valid: true}
}

func parseClock(s string) (pgtype.Time, bool) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return pgtype.Time{}, false
	}
	return clockOf(t.Hour(), t.Minute()), true
}

func (a *App) adminTemplateSave(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	back := "/admin/templates/new"
	if id > 0 {
		back = fmt.Sprintf("/admin/templates/%d", id)
	}
	start, ok1 := parseClock(r.FormValue("start_time"))
	end, ok2 := parseClock(r.FormValue("end_time"))
	name := strings.TrimSpace(r.FormValue("name"))
	courts, capacity := int32(formInt64(r, "court_count")), int32(formInt64(r, "capacity"))
	weekday := int16(formInt64(r, "weekday"))
	venueID := formInt64(r, "venue_id")
	switch {
	case name == "" || venueID == 0:
		setFlash(w, "error", "Nama template dan venue wajib diisi.")
	case !ok1 || !ok2:
		setFlash(w, "error", "Format jam harus JJ:MM.")
	case courts < 1 || capacity < 1:
		setFlash(w, "error", "Jumlah lapangan dan kuota minimal 1.")
	case weekday < 0 || weekday > 6:
		setFlash(w, "error", "Hari tidak valid.")
	default:
		var err error
		p := sqlc.CreateTemplateParams{
			Name: name, VenueID: venueID, Weekday: weekday, StartTime: start, EndTime: end, CourtCount: courts, Capacity: capacity,
			Price: formMoney(r, "price"), PointRule: strings.TrimSpace(r.FormValue("point_rule")),
			LevelLabel: strings.TrimSpace(r.FormValue("level_label")), ShuttlecockIncluded: formBool(r, "shuttlecock_included"),
			UniqueCodeEnabled: formBool(r, "unique_code_enabled"), Notes: strings.TrimSpace(r.FormValue("notes")),
		}
		if id > 0 {
			err = a.Q.UpdateTemplate(r.Context(), sqlc.UpdateTemplateParams{
				ID: id, Name: p.Name, VenueID: p.VenueID, Weekday: p.Weekday, StartTime: p.StartTime, EndTime: p.EndTime,
				CourtCount: p.CourtCount, Capacity: p.Capacity, Price: p.Price, PointRule: p.PointRule, LevelLabel: p.LevelLabel,
				ShuttlecockIncluded: p.ShuttlecockIncluded, UniqueCodeEnabled: p.UniqueCodeEnabled, Notes: p.Notes,
			})
		} else {
			_, err = a.Q.CreateTemplate(r.Context(), p)
		}
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		a.done(w, r, "/admin/templates", "Template disimpan.")
		return
	}
	a.redirect(w, r, back)
}

func (a *App) adminTemplateDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.Q.DeleteTemplate(r.Context(), pathID(r)); err != nil {
		a.serverError(w, r, err)
		return
	}
	a.done(w, r, "/admin/templates", "Template dihapus.")
}

// adminTemplateCreateSession creates a session from a template; the admin only picks the date.
func (a *App) adminTemplateCreateSession(w http.ResponseWriter, r *http.Request) {
	date, ok := a.formDate(r, "date")
	if !ok {
		setFlash(w, "error", "Pilih tanggal sesi.")
		a.redirect(w, r, "/admin/templates")
		return
	}
	in, err := a.Svc.TemplateSessionInput(r.Context(), pathID(r), date)
	if err != nil {
		a.fail(w, r, "/admin/templates", err)
		return
	}
	in.Publish = r.FormValue("action") == "publish"
	sess, err := a.Svc.CreateSession(r.Context(), a.adminActor(r), in)
	if err != nil {
		a.fail(w, r, "/admin/templates", err)
		return
	}
	msg := "Draf sesi dibuat."
	if in.Publish {
		msg = "Sesi dipublikasikan. Salin teks broadcast lalu kirim ke grup."
	}
	a.done(w, r, fmt.Sprintf("/admin/sessions/%d", sess.ID), msg)
}

func (a *App) adminSessions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	rows, err := a.Q.ListSessionsAdmin(r.Context(), status)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Sesi", "sessions")
	v.Data["Sessions"] = rows
	v.Data["Status"] = status
	a.render(w, r, http.StatusOK, "admin/sessions", v)
}

func (a *App) adminSessionForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	venues, err := a.Q.ListVenues(ctx)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	vals := a.settings(ctx)
	loc := a.Loc()
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day()+1, 19, 0, 0, 0, loc)
	s := sqlc.Session{
		StartsAt: start, EndsAt: start.Add(3 * time.Hour), CourtCount: 2, Capacity: int32(2 * vals.Int(settings.CapacityPerCourt)),
		UniqueCodeEnabled: vals.Bool(settings.UniqueCodeDefault), PointRule: vals.String(settings.PointRule), ShuttlecockIncluded: true,
		RefundDeadlineAt: domain.RefundDeadline(start, int(vals.Int(settings.RefundDeadlineDays)), loc),
	}
	if id := pathID(r); id > 0 {
		if s, err = a.Q.GetSession(ctx, id); err != nil {
			a.notFound(w, r)
			return
		}
	}
	v := a.adminView(w, r, "Sesi", "sessions")
	v.Data["S"] = s
	v.Data["Venues"] = venues
	v.Data["PerCourt"] = vals.Int(settings.CapacityPerCourt)
	a.render(w, r, http.StatusOK, "admin/session_form", v)
}

func (a *App) adminSessionSave(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	back := "/admin/sessions/new"
	if id > 0 {
		back = fmt.Sprintf("/admin/sessions/%d/edit", id)
	}
	start, ok1 := a.formTime(r, "starts_at")
	end, ok2 := a.formTime(r, "ends_at")
	if !ok1 || !ok2 {
		setFlash(w, "error", "Isi tanggal dan jam mulai serta selesai.")
		a.redirect(w, r, back)
		return
	}
	in := service.SessionInput{
		Title: r.FormValue("title"), Slug: strings.TrimSpace(r.FormValue("slug")), VenueID: formInt64(r, "venue_id"),
		StartsAt: start, EndsAt: end, CourtCount: int32(formInt64(r, "court_count")), Capacity: int32(formInt64(r, "capacity")),
		Price: formMoney(r, "price"), UniqueCodeEnabled: formBool(r, "unique_code_enabled"),
		PaymentDeadlineAt: a.formTimePtr(r, "payment_deadline_at"), RefundDeadlineAt: a.formTimePtr(r, "refund_deadline_at"),
		PointRule: strings.TrimSpace(r.FormValue("point_rule")), LevelLabel: strings.TrimSpace(r.FormValue("level_label")),
		Shuttlecock: formBool(r, "shuttlecock_included"), Notes: strings.TrimSpace(r.FormValue("notes")),
		MetaTitle: strings.TrimSpace(r.FormValue("meta_title")), MetaDescription: strings.TrimSpace(r.FormValue("meta_description")),
		Publish: r.FormValue("action") == "publish",
	}
	var (
		sess sqlc.Session
		err  error
	)
	if id > 0 {
		sess, err = a.Svc.UpdateSession(r.Context(), a.adminActor(r), id, in)
	} else {
		sess, err = a.Svc.CreateSession(r.Context(), a.adminActor(r), in)
	}
	if err != nil {
		a.fail(w, r, back, err)
		return
	}
	a.done(w, r, fmt.Sprintf("/admin/sessions/%d", sess.ID), "Sesi disimpan.")
}

func (a *App) adminSessionShow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess, err := a.Q.GetSession(ctx, pathID(r))
	if err != nil {
		a.notFound(w, r)
		return
	}
	page, err := a.loadSessionPage(ctx, sess)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	activity, err := a.Q.ListSessionActivity(ctx, sess.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, sess.Title, "sessions")
	v.Data["Page"] = page
	v.Data["Activity"] = activity
	v.Data["Broadcast"] = a.broadcastText(ctx, page)
	v.Data["PublicURL"] = a.Cfg.BaseURL + "/sessions/" + sess.Slug
	a.render(w, r, http.StatusOK, "admin/session", v)
}

func (a *App) broadcastText(ctx context.Context, page SessionPage) string {
	vals := a.settings(ctx)
	site := a.site(ctx)
	loc := a.Loc()
	sess := page.Session
	d := broadcast.Data{
		AppName: site.Name, Title: sess.Title, Date: locale.FormatDate(sess.StartsAt, loc),
		TimeRange: locale.FormatTimeRange(sess.StartsAt, sess.EndsAt, loc), Venue: page.Venue.Name, Address: page.Venue.Address,
		MapsURL: page.Venue.MapsUrl, CourtCount: int(sess.CourtCount), Capacity: int(sess.Capacity),
		Price: locale.FormatRupiah(sess.Price), PointRule: sess.PointRule, Level: sess.LevelLabel, Notes: sess.Notes,
		SessionURL: strings.TrimRight(site.SiteURL, "/") + "/sessions/" + sess.Slug,
	}
	if sess.UniqueCodeEnabled {
		d.Price += " + kode unik"
	}
	d.Shuttlecock = "Termasuk HTM"
	if !sess.ShuttlecockIncluded {
		d.Shuttlecock = "Bayar terpisah"
	}
	var pay []string
	if site.BankName != "" && site.BankAccount != "" {
		pay = append(pay, fmt.Sprintf("🏦 %s %s a.n. %s", site.BankName, site.BankAccount, site.BankHolder))
	}
	if site.QRIS != nil {
		pay = append(pay, "📱 QRIS tersedia di link sesi")
	}
	pay = append(pay, "Upload bukti bayar lewat link sesi. Yang lunas diprioritaskan.")
	d.PaymentInfo = strings.Join(pay, "\n")
	for _, reg := range page.Registered {
		d.Participants = append(d.Participants, broadcast.Line{Name: reg.DisplayName, Paid: reg.PaymentStatus == domain.PaymentPaid})
	}
	for _, reg := range page.Waitlist {
		d.Waitlist = append(d.Waitlist, broadcast.Line{Name: reg.DisplayName, Paid: reg.PaymentStatus == domain.PaymentPaid})
	}
	for _, s := range site.Social {
		d.Social = append(d.Social, s.Name+": "+s.URL)
	}
	return broadcast.Render(vals.String(settings.BroadcastTemplate), d)
}

func (a *App) adminBroadcast(w http.ResponseWriter, r *http.Request) {
	sess, err := a.Q.GetSession(r.Context(), pathID(r))
	if err != nil {
		a.notFound(w, r)
		return
	}
	page, err := a.loadSessionPage(r.Context(), sess)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	text := a.broadcastText(r.Context(), page)
	if isHTMX(r) || r.URL.Query().Get("format") == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(text))
		return
	}
	v := a.adminView(w, r, "Teks Broadcast", "sessions")
	v.Data["Session"] = sess
	v.Data["Text"] = text
	a.render(w, r, http.StatusOK, "admin/broadcast", v)
}

func (a *App) sessionAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, service.Actor, int64) error, msg string) {
	id := pathID(r)
	back := fmt.Sprintf("/admin/sessions/%d", id)
	if err := fn(r.Context(), a.adminActor(r), id); err != nil {
		a.fail(w, r, back, err)
		return
	}
	a.done(w, r, back, msg)
}

func (a *App) adminSessionPublish(w http.ResponseWriter, r *http.Request) {
	a.sessionAction(w, r, a.Svc.PublishSession, "Sesi dipublikasikan.")
}

func (a *App) adminSessionClose(w http.ResponseWriter, r *http.Request) {
	a.sessionAction(w, r, a.Svc.CloseSession, "Sesi ditutup. Waiting list yang lunas masuk antrean refund.")
}

func (a *App) adminSessionCancel(w http.ResponseWriter, r *http.Request) {
	a.sessionAction(w, r, a.Svc.CancelSession, "Sesi dibatalkan. Semua pembayaran lunas masuk antrean refund.")
}

func (a *App) adminSessionDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.Svc.DeleteDraftSession(r.Context(), pathID(r)); err != nil {
		a.fail(w, r, fmt.Sprintf("/admin/sessions/%d", pathID(r)), err)
		return
	}
	a.done(w, r, "/admin/sessions", "Draf sesi dihapus.")
}

func (a *App) adminSessionCosts(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	back := fmt.Sprintf("/admin/sessions/%d", id)
	if r.FormValue("from") == "game-day" {
		back += "/game-day"
	}
	err := a.Svc.SetCosts(r.Context(), a.adminActor(r), id, formMoney(r, "court_cost"), formMoney(r, "shuttlecock_cost"), formMoney(r, "other_cost"))
	if err != nil {
		a.fail(w, r, back, err)
		return
	}
	a.done(w, r, back, "Biaya sesi disimpan.")
}

func (a *App) adminAddRegistration(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	back := fmt.Sprintf("/admin/sessions/%d", id)
	res, err := a.Svc.Register(r.Context(), a.adminActor(r), id, service.RegisterInput{
		Name: r.FormValue("name"), Phone: r.FormValue("phone"), Level: r.FormValue("level"), ByAdmin: true,
	})
	if err != nil {
		a.fail(w, r, back, err)
		return
	}
	if formBool(r, "paid") {
		if err := a.Svc.MarkPaidManually(r.Context(), a.adminActor(r), res.Registration.ID); err != nil {
			a.fail(w, r, back, err)
			return
		}
	}
	msg := fmt.Sprintf("%s ditambahkan sebagai %s.", res.Registration.DisplayName, strings.ToLower(regStatusLabel(res.Registration.Status)))
	if res.Registration.UserID == nil {
		msg += " Link tiket: " + a.Cfg.BaseURL + "/tickets/" + res.TicketToken
	}
	a.done(w, r, back, msg)
}

func (a *App) regBack(r *http.Request, regID int64) string {
	if b := safeNext(r.FormValue("back")); b != "" {
		return b
	}
	if reg, err := a.Q.GetRegistration(r.Context(), regID); err == nil {
		return fmt.Sprintf("/admin/sessions/%d", reg.SessionID)
	}
	return "/admin/sessions"
}

func (a *App) regAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, service.Actor, int64) error, msg string) {
	id := pathID(r)
	back := a.regBack(r, id)
	if err := fn(r.Context(), a.adminActor(r), id); err != nil {
		a.fail(w, r, back, err)
		return
	}
	a.done(w, r, back, msg)
}

func (a *App) adminRegMove(w http.ResponseWriter, r *http.Request) {
	to := r.FormValue("to")
	a.regAction(w, r, func(ctx context.Context, act service.Actor, id int64) error { return a.Svc.AdminMove(ctx, act, id, to) },
		"Posisi peserta diperbarui.")
}

func (a *App) adminRegMarkPaid(w http.ResponseWriter, r *http.Request) {
	a.regAction(w, r, a.Svc.MarkPaidManually, "Peserta ditandai lunas.")
}

func (a *App) adminRegCancel(w http.ResponseWriter, r *http.Request) {
	a.regAction(w, r, a.Svc.AdminCancel, "Peserta dibatalkan.")
}

func (a *App) adminRegDelete(w http.ResponseWriter, r *http.Request) {
	a.regAction(w, r, a.Svc.AdminDelete, "Peserta dihapus.")
}

func (a *App) adminRegLevel(w http.ResponseWriter, r *http.Request) {
	level := r.FormValue("level")
	id := pathID(r)
	if err := a.Svc.SetRegistrationLevel(r.Context(), a.adminActor(r), id, level); err != nil {
		a.fail(w, r, a.regBack(r, id), err)
		return
	}
	if isHTMX(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	a.done(w, r, a.regBack(r, id), "Level dikoreksi.")
}

// adminRegAttendance toggles presence; HTMX requests get the updated checklist row only.
func (a *App) adminRegAttendance(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	reg, err := a.Svc.SetAttendance(r.Context(), a.adminActor(r), id, r.FormValue("status"))
	if err != nil {
		if isHTMX(r) {
			if msg, ok := service.IsUserError(err); ok {
				http.Error(w, msg, http.StatusUnprocessableEntity)
				return
			}
		}
		a.fail(w, r, a.regBack(r, id), err)
		return
	}
	if isHTMX(r) {
		a.renderBlock(w, r, "admin/game_day", "attendance-row", map[string]any{"R": reg, "CSRF": a.csrf(r)})
		return
	}
	a.done(w, r, a.regBack(r, id), "Kehadiran disimpan.")
}

func (a *App) adminGameDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess, err := a.Q.GetSession(ctx, pathID(r))
	if err != nil {
		a.notFound(w, r)
		return
	}
	page, err := a.loadSessionPage(ctx, sess)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	board, err := a.rotationData(r, sess)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Hari H · "+sess.Title, "sessions")
	v.Data["Page"] = page
	v.Data["Board"] = board
	present := 0
	for _, reg := range page.Registered {
		if reg.Status == domain.StatusAttended {
			present++
		}
	}
	v.Data["Present"] = present
	a.render(w, r, http.StatusOK, "admin/game_day", v)
}

func (a *App) adminRotationNext(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	back := fmt.Sprintf("/admin/sessions/%d/game-day#rotasi", id)
	if err := a.Svc.NextRotationRound(r.Context(), a.adminActor(r), id, int(formInt64(r, "courts"))); err != nil {
		a.fail(w, r, back, err)
		return
	}
	a.done(w, r, back, "Ronde baru dibuat.")
}

func (a *App) adminRotationUndo(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	back := fmt.Sprintf("/admin/sessions/%d/game-day#rotasi", id)
	if err := a.Svc.UndoRotationRound(r.Context(), a.adminActor(r), id); err != nil {
		a.fail(w, r, back, err)
		return
	}
	a.done(w, r, back, "Ronde terakhir dihapus.")
}

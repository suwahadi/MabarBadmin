package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
	"github.com/suwahadi/mabarbadmin/internal/service"
)

func (a *App) meShow(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	ctx := r.Context()
	counts, err := a.Q.ListSessionCountsForUser(ctx, &u.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	board, _, err := a.Svc.Leaderboard(ctx, "this_month", 1000, a.Loc())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	rank := 0
	for _, e := range board {
		if e.UserID == u.ID {
			rank = e.Rank
		}
	}
	regs, err := a.Q.ListUserRegistrations(ctx, &u.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.newView(w, r, "Akun Saya")
	v.Meta.NoIndex = true
	v.Data["Counts"] = counts
	v.Data["Rank"] = rank
	v.Data["Registrations"] = regs
	a.render(w, r, http.StatusOK, "member/me", v)
}

func (a *App) meRegistrations(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	regs, err := a.Q.ListUserRegistrations(r.Context(), &u.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.newView(w, r, "Riwayat Sesi")
	v.Meta.NoIndex = true
	v.Data["Registrations"] = regs
	a.render(w, r, http.StatusOK, "member/registrations", v)
}

// ownRegistration loads a registration that belongs to the signed-in member.
func (a *App) ownRegistration(w http.ResponseWriter, r *http.Request) (sqlc.Registration, string, bool) {
	u := currentUser(r)
	reg, err := a.Q.GetRegistration(r.Context(), pathID(r))
	if err != nil || reg.UserID == nil || *reg.UserID != u.ID {
		a.notFound(w, r)
		return reg, "", false
	}
	sess, err := a.Q.GetSession(r.Context(), reg.SessionID)
	if err != nil {
		a.serverError(w, r, err)
		return reg, "", false
	}
	return reg, "/sessions/" + sess.Slug, true
}

func (a *App) meUpload(w http.ResponseWriter, r *http.Request) {
	reg, back, ok := a.ownRegistration(w, r)
	if !ok {
		return
	}
	a.handleUpload(w, r, service.MemberActor(currentUser(r).ID), reg.ID, back+"#tiket")
}

func (a *App) meCancel(w http.ResponseWriter, r *http.Request) {
	reg, back, ok := a.ownRegistration(w, r)
	if !ok {
		return
	}
	a.handleCancel(w, r, service.MemberActor(currentUser(r).ID), reg, back+"#tiket")
}

func (a *App) meTransfer(w http.ResponseWriter, r *http.Request) {
	reg, back, ok := a.ownRegistration(w, r)
	if !ok {
		return
	}
	res, err := a.Svc.Transfer(r.Context(), service.MemberActor(currentUser(r).ID), reg.ID, r.FormValue("name"), r.FormValue("phone"), r.FormValue("level"))
	if err != nil {
		a.fail(w, r, back+"#tiket", err)
		return
	}
	v := a.newView(w, r, "Slot Dioper")
	v.Meta.NoIndex = true
	v.Data["Name"] = res.Registration.DisplayName
	v.Data["Link"] = newTicketLink(a.Cfg.BaseURL, res.TicketToken)
	v.Data["Back"] = back
	a.render(w, r, http.StatusOK, "member/transferred", v)
}

func (a *App) meRefundAccount(w http.ResponseWriter, r *http.Request) {
	reg, back, ok := a.ownRegistration(w, r)
	if !ok {
		return
	}
	if err := a.Svc.UpdateRefundAccount(r.Context(), service.MemberActor(currentUser(r).ID), reg.ID, refundAccount(r)); err != nil {
		a.fail(w, r, back+"#tiket", err)
		return
	}
	a.done(w, r, back+"#tiket", "Rekening refund disimpan.")
}

func (a *App) meProofImage(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	a.serveProof(w, r, pathID(r), func(p sqlc.GetProofByMediaRow) bool {
		return (p.UserID != nil && *p.UserID == u.ID) || u.Role == "admin"
	})
}

func (a *App) meProfile(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	err := a.Svc.UpdateProfile(r.Context(), u.ID, service.ProfileInput{
		Name: r.FormValue("name"), Level: r.FormValue("level"), RefundBank: r.FormValue("refund_bank_name"),
		RefundAccount: r.FormValue("refund_account_number"), RefundAccountNm: r.FormValue("refund_account_name"),
	})
	if err != nil {
		a.fail(w, r, "/me", err)
		return
	}
	a.done(w, r, "/me", "Profil disimpan.")
}

func (a *App) mePasswordForm(w http.ResponseWriter, r *http.Request) {
	v := a.newView(w, r, "Ganti Password")
	v.Meta.NoIndex = true
	a.render(w, r, http.StatusOK, "member/password", v)
}

func (a *App) mePasswordSubmit(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if r.FormValue("new_password") != r.FormValue("new_password_confirm") {
		setFlash(w, "error", "Konfirmasi password baru tidak sama.")
		a.redirect(w, r, "/me/password")
		return
	}
	if err := a.Svc.ChangePassword(r.Context(), u.ID, r.FormValue("current_password"), r.FormValue("new_password")); err != nil {
		a.fail(w, r, "/me/password", err)
		return
	}
	a.done(w, r, "/me", "Password berhasil diganti.")
}

type BoardMatch struct {
	Court int
	TeamA [2]sqlc.Registration
	TeamB [2]sqlc.Registration
}

type BoardRound struct {
	Number  int
	Matches []BoardMatch
	Sitting []sqlc.Registration
}

type RotationData struct {
	Session sqlc.Session
	Venue   sqlc.Venue
	Rounds  []BoardRound
	Current *BoardRound
	Updated time.Time
	URL     string
}

func (a *App) rotationData(r *http.Request, sess sqlc.Session) (RotationData, error) {
	ctx := r.Context()
	d := RotationData{Session: sess, Updated: time.Now(), URL: fmt.Sprintf("%s/sessions/%s/rotation", a.Cfg.BaseURL, sess.Slug)}
	var err error
	if d.Venue, err = a.Q.GetVenue(ctx, sess.VenueID); err != nil {
		return d, err
	}
	regs, err := a.Q.ListSessionRegistrations(ctx, sess.ID)
	if err != nil {
		return d, err
	}
	byID := map[int64]sqlc.Registration{}
	var present, holders []sqlc.Registration
	for _, reg := range regs {
		byID[reg.ID] = reg
		if reg.Status == domain.StatusAttended {
			present = append(present, reg)
		}
		if reg.Status == domain.StatusAttended || reg.Status == domain.StatusRegistered {
			holders = append(holders, reg)
		}
	}
	if len(present) == 0 {
		present = holders
	}
	rounds, err := service.Rotation(ctx, a.Q, sess.ID)
	if err != nil {
		return d, err
	}
	for _, rd := range rounds {
		br := BoardRound{Number: rd.Number}
		playing := map[int64]bool{}
		for _, m := range rd.Matches {
			bm := BoardMatch{Court: m.Court}
			for i, id := range m.TeamA {
				bm.TeamA[i] = byID[id]
				playing[id] = true
			}
			for i, id := range m.TeamB {
				bm.TeamB[i] = byID[id]
				playing[id] = true
			}
			br.Matches = append(br.Matches, bm)
		}
		for _, p := range present {
			if !playing[p.ID] {
				br.Sitting = append(br.Sitting, p)
			}
		}
		d.Rounds = append(d.Rounds, br)
	}
	if n := len(d.Rounds); n > 0 {
		d.Current = &d.Rounds[n-1]
	}
	return d, nil
}

package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/locale"
	"github.com/suwahadi/mabarbadmin/internal/service"
)

func (a *App) adminPayments(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "approved" && status != "rejected" {
		status = "pending"
	}
	rows, err := a.Q.ListPaymentQueue(r.Context(), status)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Pembayaran", "payments")
	v.Data["Proofs"] = rows
	v.Data["Status"] = status
	a.render(w, r, http.StatusOK, "admin/payments", v)
}

// proofResult answers approve/reject: HTMX swaps the queue row, a normal form redirects back.
func (a *App) proofResult(w http.ResponseWriter, r *http.Request, err error, msg string) {
	back := safeNext(r.FormValue("back"))
	if back == "" {
		back = "/admin/payments"
	}
	if isHTMX(r) {
		if err != nil {
			if m, ok := service.IsUserError(err); ok {
				a.renderBlock(w, r, "admin/payments", "proof-done", map[string]any{"Msg": m, "Error": true})
				return
			}
			a.serverError(w, r, err)
			return
		}
		a.renderBlock(w, r, "admin/payments", "proof-done", map[string]any{"Msg": msg})
		return
	}
	if err != nil {
		a.fail(w, r, back, err)
		return
	}
	a.done(w, r, back, msg)
}

func (a *App) adminProofApprove(w http.ResponseWriter, r *http.Request) {
	err := a.Svc.ApproveProof(r.Context(), a.adminActor(r), pathID(r))
	a.proofResult(w, r, err, "Pembayaran dikonfirmasi.")
}

func (a *App) adminProofReject(w http.ResponseWriter, r *http.Request) {
	err := a.Svc.RejectProof(r.Context(), a.adminActor(r), pathID(r), r.FormValue("review_note"))
	a.proofResult(w, r, err, "Bukti bayar ditolak.")
}

func (a *App) adminProofImage(w http.ResponseWriter, r *http.Request) {
	a.serveProof(w, r, pathID(r), func(sqlc.GetProofByMediaRow) bool { return true })
}

func (a *App) adminRefunds(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "completed" {
		status = "pending"
	}
	rows, err := a.Q.ListRefunds(r.Context(), status)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Refund", "payments")
	v.Data["Refunds"] = rows
	v.Data["Status"] = status
	a.render(w, r, http.StatusOK, "admin/refunds", v)
}

func (a *App) adminRefundComplete(w http.ResponseWriter, r *http.Request) {
	if err := a.Svc.CompleteRefund(r.Context(), a.adminActor(r), pathID(r)); err != nil {
		a.fail(w, r, "/admin/refunds", err)
		return
	}
	a.done(w, r, "/admin/refunds", "Refund ditandai sudah ditransfer.")
}

func (a *App) financeMonth(r *http.Request) time.Time {
	if t, err := time.ParseInLocation("2006-01", r.URL.Query().Get("month"), a.Loc()); err == nil {
		return t
	}
	return time.Now().In(a.Loc())
}

func (a *App) adminFinance(w http.ResponseWriter, r *http.Request) {
	month := a.financeMonth(r)
	rep, err := a.Svc.FinanceMonth(r.Context(), month, a.Loc())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Keuangan", "finance")
	v.Data["Report"] = rep
	v.Data["Month"] = rep.Month.Format("2006-01")
	v.Data["Prev"] = rep.Month.AddDate(0, -1, 0).Format("2006-01")
	v.Data["Next"] = rep.Month.AddDate(0, 1, 0).Format("2006-01")
	a.render(w, r, http.StatusOK, "admin/finance", v)
}

func (a *App) adminFinanceCSV(w http.ResponseWriter, r *http.Request) {
	month := a.financeMonth(r)
	rep, err := a.Svc.FinanceMonth(r.Context(), month, a.Loc())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="keuangan-%s.csv"`, rep.Month.Format("2006-01")))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"tanggal", "sesi", "status", "hadir", "pemasukan", "refund", "sewa_lapangan", "shuttlecock", "biaya_lain", "saldo_bersih"})
	n := func(v int64) string { return strconv.FormatInt(v, 10) }
	for _, s := range rep.Sessions {
		_ = cw.Write([]string{locale.FormatShortDate(s.StartsAt, a.Loc()), s.Title, s.Status, n(s.Attended), n(s.Income), n(s.Refunds),
			n(s.CourtCost), n(s.ShuttlecockCost), n(s.OtherCost), n(s.Net())})
	}
	_ = cw.Write([]string{"", "TOTAL", "", "", n(rep.Income), n(rep.Refunds), "", "", n(rep.Costs), n(rep.Net)})
	cw.Flush()
}

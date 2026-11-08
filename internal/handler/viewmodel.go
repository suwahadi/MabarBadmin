package handler

import (
	"context"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
)

// SessionPage gathers everything templates need to show one session.
type SessionPage struct {
	Session    sqlc.Session
	Venue      sqlc.Venue
	Registered []sqlc.Registration
	Waitlist   []sqlc.Registration
	Others     []sqlc.Registration
	All        []sqlc.Registration
	PaidCount  int
	Remaining  int
}

func (p SessionPage) Open(now time.Time) bool {
	return domain.CanRegister(p.Session.Status, p.Session.StartsAt, now) == nil
}

func (p SessionPage) Full() bool { return p.Remaining <= 0 }

func (a *App) loadSessionPage(ctx context.Context, sess sqlc.Session) (SessionPage, error) {
	p := SessionPage{Session: sess}
	var err error
	if p.Venue, err = a.Q.GetVenue(ctx, sess.VenueID); err != nil {
		return p, err
	}
	regs, err := a.Q.ListSessionRegistrations(ctx, sess.ID)
	if err != nil {
		return p, err
	}
	p.All = regs
	var seats []domain.Seat
	byID := map[int64]sqlc.Registration{}
	for _, r := range regs {
		switch {
		case domain.HoldsSlot(r.Status):
			p.Registered = append(p.Registered, r)
			if r.PaymentStatus == domain.PaymentPaid {
				p.PaidCount++
			}
		case r.Status == domain.StatusWaitlisted:
			seat := domain.Seat{ID: r.ID, Status: r.Status, QueuedAt: r.QueuedAt, Paid: r.PaymentStatus == domain.PaymentPaid}
			if r.PaidAt != nil {
				seat.PaidAt = *r.PaidAt
			}
			seats = append(seats, seat)
			byID[r.ID] = r
		default:
			p.Others = append(p.Others, r)
		}
	}
	domain.SortWaitlist(seats)
	for _, s := range seats {
		p.Waitlist = append(p.Waitlist, byID[s.ID])
	}
	p.Remaining = int(sess.Capacity) - len(p.Registered)
	return p, nil
}

// SeatPanel drives the shared ticket/member panel: status, payment, cancel, and transfer.
type SeatPanel struct {
	Reg        sqlc.Registration
	Session    sqlc.Session
	Venue      sqlc.Venue
	Base       string
	ProofBase  string
	Proofs     []sqlc.PaymentProof
	Refund     *sqlc.Refund
	CanUpload  bool
	CanChange  bool
	WillRefund bool
	NewTicket  string
	Position   int
	Site       *Site
	CSRF       string
	IsGuest    bool
	Transferee *sqlc.Registration
}

func (a *App) seatPanel(ctx context.Context, reg sqlc.Registration, base, proofBase string, now time.Time) (SeatPanel, error) {
	sess, err := a.Q.GetSession(ctx, reg.SessionID)
	if err != nil {
		return SeatPanel{}, err
	}
	p := SeatPanel{Reg: reg, Session: sess, Base: base, ProofBase: proofBase, Site: a.site(ctx), IsGuest: reg.UserID == nil}
	if p.Venue, err = a.Q.GetVenue(ctx, sess.VenueID); err != nil {
		return p, err
	}
	if p.Proofs, err = a.Q.ListProofsForRegistration(ctx, reg.ID); err != nil {
		return p, err
	}
	if ref, err := a.Q.GetRefundByRegistration(ctx, reg.ID); err == nil {
		p.Refund = &ref
	}
	if reg.TransferredToID != nil {
		if t, err := a.Q.GetRegistration(ctx, *reg.TransferredToID); err == nil {
			p.Transferee = &t
		}
	}
	p.CanUpload = domain.CanUploadProof(sess.Status, reg.Status, reg.PaymentStatus, sess.EndsAt, now) == nil
	p.CanChange = domain.CanChangeSeat(sess.Status, reg.Status, sess.StartsAt, now) == nil
	p.WillRefund = domain.CancelPaymentOutcome(reg.Status, reg.PaymentStatus, now, sess.RefundDeadlineAt) == domain.PaymentRefundPending
	if reg.Status == domain.StatusWaitlisted {
		sp, err := a.loadSessionPage(ctx, sess)
		if err == nil {
			for i, w := range sp.Waitlist {
				if w.ID == reg.ID {
					p.Position = i + 1
				}
			}
		}
	}
	return p, nil
}

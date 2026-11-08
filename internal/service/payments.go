package service

import (
	"context"
	"io"
	"strings"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
)

func validMethod(m string) bool { return m == "qris" || m == "bank_transfer" }

// UploadProof stores a payment proof through the media helper and marks the payment as pending.
func (s *Service) UploadProof(ctx context.Context, a Actor, regID int64, method string, file io.Reader) error {
	if !validMethod(method) {
		return userErr("Pilih metode pembayaran (QRIS atau transfer bank).")
	}
	return s.tx(ctx, func(q *sqlc.Queries) error {
		sess, reg, err := s.lockRegistration(ctx, q, regID)
		if err != nil {
			return err
		}
		if err := domain.CanUploadProof(sess.Status, reg.Status, reg.PaymentStatus, sess.EndsAt, s.Now()); err != nil {
			return userErr(capitalize(err.Error()) + ".")
		}
		m, err := s.Media.Save(ctx, q, file, "payment_proof", "Bukti bayar "+reg.DisplayName)
		if err != nil {
			return userErr(capitalize(err.Error()) + ".")
		}
		proof, err := q.CreatePaymentProof(ctx, sqlc.CreatePaymentProofParams{
			RegistrationID: reg.ID, Method: method, MediaID: &m.ID, Status: "pending", Amount: reg.AmountDue,
		})
		if err != nil {
			return err
		}
		if err := q.UpdatePaymentStatus(ctx, sqlc.UpdatePaymentStatusParams{ID: reg.ID, PaymentStatus: domain.PaymentPending}); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "registration", reg.ID, "proof_uploaded", map[string]any{"proof_id": proof.ID, "method": method})
	})
}

// currentHolder follows slot transfers so a proof uploaded before a transfer settles the seat
// that now carries that payment.
func currentHolder(ctx context.Context, q *sqlc.Queries, reg sqlc.Registration) (sqlc.Registration, error) {
	for i := 0; reg.Status == domain.StatusTransferred && reg.TransferredToID != nil && i < 20; i++ {
		next, err := q.GetRegistration(ctx, *reg.TransferredToID)
		if err != nil {
			return reg, err
		}
		reg = next
	}
	return reg, nil
}

func (s *Service) proofContext(ctx context.Context, q *sqlc.Queries, proofID int64) (sqlc.PaymentProof, sqlc.Session, sqlc.Registration, error) {
	proof, err := q.GetPaymentProof(ctx, proofID)
	if err != nil {
		return proof, sqlc.Session{}, sqlc.Registration{}, notFound(err)
	}
	sess, reg, err := s.lockRegistration(ctx, q, proof.RegistrationID)
	if err != nil {
		return proof, sess, reg, err
	}
	proof, err = q.GetPaymentProofForUpdate(ctx, proofID)
	if err != nil {
		return proof, sess, reg, err
	}
	reg, err = currentHolder(ctx, q, reg)
	return proof, sess, reg, err
}

func (s *Service) ApproveProof(ctx context.Context, a Actor, proofID int64) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		proof, sess, reg, err := s.proofContext(ctx, q, proofID)
		if err != nil {
			return err
		}
		if proof.Status != "pending" {
			return userErr("Bukti ini sudah ditinjau.")
		}
		if err := q.ReviewPaymentProof(ctx, sqlc.ReviewPaymentProofParams{ID: proof.ID, Status: "approved"}); err != nil {
			return err
		}
		if err := logActivity(ctx, q, a, "registration", reg.ID, "payment_approved", map[string]any{"proof_id": proof.ID}); err != nil {
			return err
		}
		return s.settlePayment(ctx, q, a, sess, reg)
	})
}

func (s *Service) RejectProof(ctx context.Context, a Actor, proofID int64, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return userErr("Alasan penolakan wajib diisi.")
	}
	return s.tx(ctx, func(q *sqlc.Queries) error {
		proof, _, reg, err := s.proofContext(ctx, q, proofID)
		if err != nil {
			return err
		}
		if proof.Status != "pending" {
			return userErr("Bukti ini sudah ditinjau.")
		}
		if err := q.ReviewPaymentProof(ctx, sqlc.ReviewPaymentProofParams{ID: proof.ID, Status: "rejected", ReviewNote: note}); err != nil {
			return err
		}
		if reg.PaymentStatus == domain.PaymentPending {
			if err := q.UpdatePaymentStatus(ctx, sqlc.UpdatePaymentStatusParams{ID: reg.ID, PaymentStatus: domain.PaymentRejected}); err != nil {
				return err
			}
		}
		return logActivity(ctx, q, a, "registration", reg.ID, "payment_rejected", map[string]any{"proof_id": proof.ID, "note": note})
	})
}

// MarkPaidManually records a payment the admin received outside the app (cash or DM transfer).
func (s *Service) MarkPaidManually(ctx context.Context, a Actor, regID int64) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		sess, reg, err := s.lockRegistration(ctx, q, regID)
		if err != nil {
			return err
		}
		if !domain.IsActive(reg.Status) && !domain.HoldsSlot(reg.Status) {
			return userErr("Peserta ini sudah tidak aktif.")
		}
		if reg.PaymentStatus == domain.PaymentPaid {
			return userErr("Peserta ini sudah lunas.")
		}
		if reg.PaymentStatus == domain.PaymentPending {
			return userErr("Ada bukti yang menunggu konfirmasi. Tinjau dari antrean pembayaran.")
		}
		proof, err := q.CreatePaymentProof(ctx, sqlc.CreatePaymentProofParams{
			RegistrationID: reg.ID, Method: "cash", Status: "approved", Amount: reg.AmountDue,
		})
		if err != nil {
			return err
		}
		if err := q.ReviewPaymentProof(ctx, sqlc.ReviewPaymentProofParams{ID: proof.ID, Status: "approved", ReviewNote: "Ditandai lunas oleh admin"}); err != nil {
			return err
		}
		if err := logActivity(ctx, q, a, "registration", reg.ID, "payment_marked_paid", nil); err != nil {
			return err
		}
		return s.settlePayment(ctx, q, a, sess, reg)
	})
}

// settlePayment applies a confirmed payment. For an active seat it becomes paid and triggers the
// paid-priority swap (PRD S4). Money that arrives for a seat that no longer plays goes to refund
// or is forfeited following the cancel rules.
func (s *Service) settlePayment(ctx context.Context, q *sqlc.Queries, a Actor, sess sqlc.Session, reg sqlc.Registration) error {
	now := s.Now()
	switch {
	case sess.Status == domain.SessionCancelled,
		sess.Status == domain.SessionClosed && reg.Status == domain.StatusWaitlisted:
		return s.queueRefund(ctx, q, a, reg, RefundAccount{})
	case reg.Status == domain.StatusCancelled:
		cancelledAt := now
		if reg.CancelledAt != nil {
			cancelledAt = *reg.CancelledAt
		}
		outcome := domain.CancelPaymentOutcome(domain.StatusRegistered, domain.PaymentPaid, cancelledAt, sess.RefundDeadlineAt)
		if outcome == domain.PaymentRefundPending {
			return s.queueRefund(ctx, q, a, reg, RefundAccount{})
		}
		return q.UpdatePaymentStatus(ctx, sqlc.UpdatePaymentStatusParams{ID: reg.ID, PaymentStatus: outcome, PaidAt: &now})
	}
	if err := q.UpdatePaymentStatus(ctx, sqlc.UpdatePaymentStatusParams{ID: reg.ID, PaymentStatus: domain.PaymentPaid, PaidAt: &now}); err != nil {
		return err
	}
	return s.rebalance(ctx, q, sess, nil)
}

// queueRefund marks a registration refund_pending and creates its refund entry. Missing bank
// details fall back to the member's default refund account.
func (s *Service) queueRefund(ctx context.Context, q *sqlc.Queries, a Actor, reg sqlc.Registration, acct RefundAccount) error {
	if acct.empty() && reg.UserID != nil {
		if u, err := q.GetUser(ctx, *reg.UserID); err == nil {
			acct = RefundAccount{BankName: u.RefundBankName, AccountNumber: u.RefundAccountNumber, AccountName: u.RefundAccountName}
		}
	}
	if err := q.UpdatePaymentStatus(ctx, sqlc.UpdatePaymentStatusParams{ID: reg.ID, PaymentStatus: domain.PaymentRefundPending, PaidAt: reg.PaidAt}); err != nil {
		return err
	}
	ref, err := q.CreateRefund(ctx, sqlc.CreateRefundParams{
		RegistrationID: reg.ID, Amount: reg.AmountDue, BankName: strings.TrimSpace(acct.BankName),
		AccountNumber: strings.TrimSpace(acct.AccountNumber), AccountName: strings.TrimSpace(acct.AccountName),
	})
	if err != nil {
		return err
	}
	return logActivity(ctx, q, a, "registration", reg.ID, "refund_queued", map[string]any{"refund_id": ref.ID, "amount": reg.AmountDue})
}

func (s *Service) UpdateRefundAccount(ctx context.Context, a Actor, regID int64, acct RefundAccount) error {
	if acct.empty() {
		return userErr("Lengkapi nama bank, nomor rekening, dan atas nama.")
	}
	return s.tx(ctx, func(q *sqlc.Queries) error {
		if err := q.UpdateRefundAccount(ctx, sqlc.UpdateRefundAccountParams{
			RegistrationID: regID, BankName: strings.TrimSpace(acct.BankName),
			AccountNumber: strings.TrimSpace(acct.AccountNumber), AccountName: strings.TrimSpace(acct.AccountName),
		}); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "registration", regID, "refund_account_updated", nil)
	})
}

func (s *Service) CompleteRefund(ctx context.Context, a Actor, refundID int64) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		ref, err := q.GetRefund(ctx, refundID)
		if err != nil {
			return notFound(err)
		}
		if ref.Status != "pending" {
			return userErr("Refund ini sudah ditandai selesai.")
		}
		if err := q.CompleteRefund(ctx, refundID); err != nil {
			return err
		}
		reg, err := q.GetRegistration(ctx, ref.RegistrationID)
		if err != nil {
			return err
		}
		if err := q.UpdatePaymentStatus(ctx, sqlc.UpdatePaymentStatusParams{ID: reg.ID, PaymentStatus: domain.PaymentRefunded, PaidAt: reg.PaidAt}); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "registration", reg.ID, "refunded", map[string]any{"refund_id": refundID})
	})
}

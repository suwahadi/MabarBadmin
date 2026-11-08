package service

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"

	"github.com/suwahadi/mabarbadmin/internal/auth"
	"github.com/suwahadi/mabarbadmin/internal/database"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
	"github.com/suwahadi/mabarbadmin/internal/locale"
)

var ErrPhoneIsMember = errors.New("nomor WA ini sudah terdaftar sebagai member, silakan masuk dulu")

type RegisterInput struct {
	UserID *int64
	Name   string
	Phone  string
	Level  string
	// ByAdmin allows manual additions after the session started and links member accounts by phone.
	ByAdmin bool
}

type RegisterResult struct {
	Registration sqlc.Registration
	TicketToken  string
}

type RefundAccount struct {
	BankName      string
	AccountNumber string
	AccountName   string
}

func (r RefundAccount) empty() bool {
	return strings.TrimSpace(r.BankName) == "" || strings.TrimSpace(r.AccountNumber) == "" || strings.TrimSpace(r.AccountName) == ""
}

func (s *Service) Register(ctx context.Context, a Actor, sessionID int64, in RegisterInput) (RegisterResult, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 60 {
		return RegisterResult{}, userErr("Nama wajib diisi (maks 60 karakter).")
	}
	if !domain.ValidLevel(in.Level) {
		return RegisterResult{}, userErr("Pilih level.")
	}
	phone, err := locale.NormalizePhone(in.Phone)
	if err != nil {
		return RegisterResult{}, userErr("Nomor WA tidak valid. Contoh: 0812-3456-7890.")
	}
	userID := in.UserID
	if userID == nil {
		u, err := s.Q.GetUserByPhone(ctx, phone)
		switch {
		case err == nil && in.ByAdmin:
			userID = &u.ID
		case err == nil:
			return RegisterResult{}, &UserError{Msg: ErrPhoneIsMember.Error() + "."}
		case !database.IsNotFound(err):
			return RegisterResult{}, err
		}
	}

	var res RegisterResult
	err = s.tx(ctx, func(q *sqlc.Queries) error {
		sess, err := q.GetSessionForUpdate(ctx, sessionID)
		if err != nil {
			return notFound(err)
		}
		if in.ByAdmin {
			if sess.Status != domain.SessionPublished && sess.Status != domain.SessionDraft {
				return userErr("Sesi sudah ditutup atau dibatalkan.")
			}
		} else if err := domain.CanRegister(sess.Status, sess.StartsAt, s.Now()); err != nil {
			return userErr(capitalize(err.Error()) + ".")
		}
		if _, err := q.GetRegistrationBySessionPhone(ctx, sqlc.GetRegistrationBySessionPhoneParams{SessionID: sessionID, Phone: phone}); err == nil {
			return userErr(capitalize(domain.ErrAlreadyRegistered.Error()) + ".")
		} else if !database.IsNotFound(err) {
			return err
		}
		code, err := s.uniqueCode(ctx, q, sess)
		if err != nil {
			return err
		}
		token, hash, err := auth.NewToken()
		if err != nil {
			return err
		}
		now := s.Now()
		reg, err := q.CreateRegistration(ctx, sqlc.CreateRegistrationParams{
			SessionID: sessionID, UserID: userID, DisplayName: in.Name, Phone: phone, Level: in.Level,
			Status: domain.StatusWaitlisted, PaymentStatus: domain.PaymentUnpaid, AmountDue: sess.Price + int64(code),
			UniqueCode: code, TicketTokenHash: hash, RegisteredAt: now, QueuedAt: now,
		})
		if err != nil {
			return err
		}
		if err := logActivity(ctx, q, a, "registration", reg.ID, "registered", map[string]any{"session_id": sessionID}); err != nil {
			return err
		}
		if err := s.rebalance(ctx, q, sess, nil); err != nil {
			return err
		}
		reg, err = q.GetRegistration(ctx, reg.ID)
		res = RegisterResult{Registration: reg, TicketToken: token}
		return err
	})
	return res, err
}

func (s *Service) uniqueCode(ctx context.Context, q *sqlc.Queries, sess sqlc.Session) (int32, error) {
	if !sess.UniqueCodeEnabled {
		return 0, nil
	}
	codes, err := q.ListSessionUniqueCodes(ctx, sess.ID)
	if err != nil {
		return 0, err
	}
	used := map[int32]bool{}
	for _, c := range codes {
		used[c] = true
	}
	var code int32
	var pickErr error
	s.withRand(func(r *rand.Rand) { code, pickErr = domain.PickUniqueCode(used, r) })
	if pickErr != nil {
		return 0, userErr(capitalize(pickErr.Error()) + ".")
	}
	return code, nil
}

// lockRegistration loads a registration and locks its session row so slot changes are serialized.
func (s *Service) lockRegistration(ctx context.Context, q *sqlc.Queries, regID int64) (sqlc.Session, sqlc.Registration, error) {
	reg, err := q.GetRegistration(ctx, regID)
	if err != nil {
		return sqlc.Session{}, sqlc.Registration{}, notFound(err)
	}
	sess, err := q.GetSessionForUpdate(ctx, reg.SessionID)
	if err != nil {
		return sqlc.Session{}, sqlc.Registration{}, err
	}
	reg, err = q.GetRegistration(ctx, regID)
	return sess, reg, err
}

// Cancel applies PRD S5. A paid participant cancelling before the refund deadline (or from the
// waiting list) is queued for refund; after the deadline the payment is forfeited.
func (s *Service) Cancel(ctx context.Context, a Actor, regID int64, acct RefundAccount) (string, error) {
	var outcome string
	err := s.tx(ctx, func(q *sqlc.Queries) error {
		sess, reg, err := s.lockRegistration(ctx, q, regID)
		if err != nil {
			return err
		}
		if err := domain.CanChangeSeat(sess.Status, reg.Status, sess.StartsAt, s.Now()); err != nil {
			return userErr(capitalize(err.Error()) + ".")
		}
		outcome = domain.CancelPaymentOutcome(reg.Status, reg.PaymentStatus, s.Now(), sess.RefundDeadlineAt)
		if err := q.CancelRegistration(ctx, sqlc.CancelRegistrationParams{ID: reg.ID, PaymentStatus: outcome}); err != nil {
			return err
		}
		if err := logActivity(ctx, q, a, "registration", reg.ID, "cancelled", map[string]any{"payment_status": outcome}); err != nil {
			return err
		}
		if outcome == domain.PaymentRefundPending {
			if err := s.queueRefund(ctx, q, a, reg, acct); err != nil {
				return err
			}
		}
		return s.rebalance(ctx, q, sess, nil)
	})
	return outcome, err
}

// AdminCancel removes an active participant on the admin's behalf; paid money is always refunded.
func (s *Service) AdminCancel(ctx context.Context, a Actor, regID int64) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		sess, reg, err := s.lockRegistration(ctx, q, regID)
		if err != nil {
			return err
		}
		if !domain.IsActive(reg.Status) {
			return userErr("Peserta ini sudah tidak aktif.")
		}
		outcome := reg.PaymentStatus
		if outcome == domain.PaymentPaid {
			outcome = domain.PaymentRefundPending
		}
		if err := q.CancelRegistration(ctx, sqlc.CancelRegistrationParams{ID: reg.ID, PaymentStatus: outcome}); err != nil {
			return err
		}
		if err := logActivity(ctx, q, a, "registration", reg.ID, "cancelled_by_admin", map[string]any{"payment_status": outcome}); err != nil {
			return err
		}
		if outcome == domain.PaymentRefundPending {
			if err := s.queueRefund(ctx, q, a, reg, RefundAccount{}); err != nil {
				return err
			}
		}
		return s.rebalance(ctx, q, sess, nil)
	})
}

// AdminDelete removes a registration that never involved money, e.g. a duplicate manual entry.
func (s *Service) AdminDelete(ctx context.Context, a Actor, regID int64) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		sess, reg, err := s.lockRegistration(ctx, q, regID)
		if err != nil {
			return err
		}
		if reg.PaymentStatus != domain.PaymentUnpaid && reg.PaymentStatus != domain.PaymentRejected {
			return userErr("Peserta yang sudah membayar atau menunggu konfirmasi tidak bisa dihapus. Gunakan Batalkan.")
		}
		if reg.Status == domain.StatusTransferred {
			return userErr("Riwayat oper slot tidak bisa dihapus.")
		}
		if err := q.DeleteRegistration(ctx, reg.ID); err != nil {
			return err
		}
		if err := logActivity(ctx, q, a, "session", sess.ID, "registration_deleted", map[string]any{"name": reg.DisplayName, "phone": reg.Phone}); err != nil {
			return err
		}
		return s.rebalance(ctx, q, sess, nil)
	})
}

type TransferResult struct {
	Registration sqlc.Registration
	TicketToken  string
}

// Transfer hands an active seat, including its payment status, to someone else (PRD S6).
func (s *Service) Transfer(ctx context.Context, a Actor, regID int64, name, rawPhone, level string) (TransferResult, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 60 {
		return TransferResult{}, userErr("Nama pengganti wajib diisi (maks 60 karakter).")
	}
	if !domain.ValidLevel(level) {
		return TransferResult{}, userErr("Pilih level pengganti.")
	}
	phone, err := locale.NormalizePhone(rawPhone)
	if err != nil {
		return TransferResult{}, userErr("Nomor WA pengganti tidak valid.")
	}
	var userID *int64
	if u, err := s.Q.GetUserByPhone(ctx, phone); err == nil {
		userID = &u.ID
	} else if !database.IsNotFound(err) {
		return TransferResult{}, err
	}

	var res TransferResult
	err = s.tx(ctx, func(q *sqlc.Queries) error {
		sess, reg, err := s.lockRegistration(ctx, q, regID)
		if err != nil {
			return err
		}
		if err := domain.CanChangeSeat(sess.Status, reg.Status, sess.StartsAt, s.Now()); err != nil {
			return userErr(capitalize(err.Error()) + ".")
		}
		if _, err := q.GetRegistrationBySessionPhone(ctx, sqlc.GetRegistrationBySessionPhoneParams{SessionID: sess.ID, Phone: phone}); err == nil {
			return userErr("Nomor WA pengganti sudah tercatat di sesi ini.")
		} else if !database.IsNotFound(err) {
			return err
		}
		token, hash, err := auth.NewToken()
		if err != nil {
			return err
		}
		next, err := q.CreateRegistration(ctx, sqlc.CreateRegistrationParams{
			SessionID: sess.ID, UserID: userID, DisplayName: name, Phone: phone, Level: level, Status: reg.Status,
			PaymentStatus: reg.PaymentStatus, AmountDue: reg.AmountDue, UniqueCode: reg.UniqueCode,
			TicketTokenHash: hash, RegisteredAt: reg.RegisteredAt, QueuedAt: reg.QueuedAt, PaidAt: reg.PaidAt,
		})
		if err != nil {
			return err
		}
		if err := q.MarkTransferred(ctx, sqlc.MarkTransferredParams{ID: reg.ID, TransferredToID: &next.ID}); err != nil {
			return err
		}
		if err := logActivity(ctx, q, a, "registration", reg.ID, "transferred", map[string]any{"to_registration_id": next.ID}); err != nil {
			return err
		}
		if err := logActivity(ctx, q, a, "registration", next.ID, "received_transfer", map[string]any{"from_registration_id": reg.ID}); err != nil {
			return err
		}
		res = TransferResult{Registration: next, TicketToken: token}
		return nil
	})
	return res, err
}

// AdminMove places a participant in a slot or on the waiting list by hand. Moving into a full
// session bumps the latest unpaid participant; the regular allocation rules still apply afterwards.
func (s *Service) AdminMove(ctx context.Context, a Actor, regID int64, to string) error {
	if to != domain.StatusRegistered && to != domain.StatusWaitlisted {
		return userErr("Tujuan pindah tidak valid.")
	}
	return s.tx(ctx, func(q *sqlc.Queries) error {
		sess, reg, err := s.lockRegistration(ctx, q, regID)
		if err != nil {
			return err
		}
		if !domain.IsActive(reg.Status) {
			return userErr("Peserta ini sudah tidak aktif.")
		}
		if reg.Status == to {
			return nil
		}
		var hold map[int64]bool
		if to == domain.StatusRegistered {
			regs, err := q.ListSessionRegistrations(ctx, sess.ID)
			if err != nil {
				return err
			}
			var seats []domain.Seat
			holding := 0
			for _, r := range regs {
				if r.Status == domain.StatusRegistered {
					holding++
					seats = append(seats, domain.Seat{ID: r.ID, Status: r.Status, Paid: r.PaymentStatus == domain.PaymentPaid, QueuedAt: r.QueuedAt})
				}
			}
			if holding >= int(sess.Capacity) {
				bump := latestUnpaid(seats)
				if bump == 0 {
					return userErr("Kuota penuh dan semua peserta sudah lunas. Naikkan kuota dulu.")
				}
				if err := q.UpdateRegistrationStatus(ctx, sqlc.UpdateRegistrationStatusParams{ID: bump, Status: domain.StatusWaitlisted}); err != nil {
					return err
				}
				if err := logActivity(ctx, q, a, "registration", bump, "moved_to_waitlist", nil); err != nil {
					return err
				}
				hold = map[int64]bool{bump: true}
			}
			if err := q.UpdateRegistrationStatus(ctx, sqlc.UpdateRegistrationStatusParams{ID: reg.ID, Status: to}); err != nil {
				return err
			}
		} else {
			if err := q.UpdateRegistrationPlacement(ctx, sqlc.UpdateRegistrationPlacementParams{ID: reg.ID, Status: to, QueuedAt: s.Now()}); err != nil {
				return err
			}
			hold = map[int64]bool{reg.ID: true}
		}
		if err := logActivity(ctx, q, a, "registration", reg.ID, "moved_by_admin", map[string]any{"to": to}); err != nil {
			return err
		}
		return s.rebalance(ctx, q, sess, hold)
	})
}

func latestUnpaid(seats []domain.Seat) int64 {
	var best *domain.Seat
	for i := range seats {
		st := &seats[i]
		if st.Paid {
			continue
		}
		if best == nil || st.QueuedAt.After(best.QueuedAt) || (st.QueuedAt.Equal(best.QueuedAt) && st.ID > best.ID) {
			best = st
		}
	}
	if best == nil {
		return 0
	}
	return best.ID
}

// SetAttendance marks a slot holder as attended, no-show, or back to registered.
func (s *Service) SetAttendance(ctx context.Context, a Actor, regID int64, status string) (sqlc.Registration, error) {
	if status != domain.StatusAttended && status != domain.StatusNoShow && status != domain.StatusRegistered {
		return sqlc.Registration{}, userErr("Status kehadiran tidak valid.")
	}
	var out sqlc.Registration
	err := s.tx(ctx, func(q *sqlc.Queries) error {
		sess, reg, err := s.lockRegistration(ctx, q, regID)
		if err != nil {
			return err
		}
		if sess.Status != domain.SessionPublished && sess.Status != domain.SessionClosed {
			return userErr("Absensi hanya untuk sesi yang terbit atau sudah ditutup.")
		}
		if !domain.HoldsSlot(reg.Status) {
			return userErr("Hanya peserta yang memegang slot yang bisa diabsen.")
		}
		if sess.Status == domain.SessionClosed && status == domain.StatusRegistered {
			status = domain.StatusNoShow
		}
		if err := q.UpdateRegistrationStatus(ctx, sqlc.UpdateRegistrationStatusParams{ID: reg.ID, Status: status}); err != nil {
			return err
		}
		if err := logActivity(ctx, q, a, "registration", reg.ID, "attendance", map[string]any{"status": status}); err != nil {
			return err
		}
		out, err = q.GetRegistration(ctx, reg.ID)
		return err
	})
	return out, err
}

func (s *Service) SetRegistrationLevel(ctx context.Context, a Actor, regID int64, level string) error {
	if !domain.ValidLevel(level) {
		return userErr("Level tidak valid.")
	}
	return s.tx(ctx, func(q *sqlc.Queries) error {
		if err := q.UpdateRegistrationLevel(ctx, sqlc.UpdateRegistrationLevelParams{ID: regID, Level: level}); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "registration", regID, "level_corrected", map[string]any{"level": level})
	})
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

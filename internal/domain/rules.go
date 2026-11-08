package domain

import (
	"errors"
	"math/rand/v2"
	"time"
)

var (
	ErrSessionNotOpen     = errors.New("sesi tidak menerima pendaftaran")
	ErrSessionStarted     = errors.New("sesi sudah dimulai")
	ErrNotActive          = errors.New("pendaftaran ini sudah tidak aktif")
	ErrUniqueCodesUsed    = errors.New("kode unik untuk sesi ini sudah habis")
	ErrAlreadyRegistered  = errors.New("nomor WA ini sudah terdaftar di sesi ini")
	ErrPaymentNotAccepted = errors.New("pembayaran untuk pendaftaran ini tidak bisa diproses")
)

// RefundDeadline is 00:00 local time, daysBefore days before the session date.
// A session on Monday 9 Nov 21:00 with daysBefore=1 gets Sunday 8 Nov 00:00.
func RefundDeadline(startsAt time.Time, daysBefore int, loc *time.Location) time.Time {
	l := startsAt.In(loc)
	day := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
	return day.AddDate(0, 0, -daysBefore)
}

// PaymentDeadline returns nil when payment is flexible until the session starts (hours == 0).
func PaymentDeadline(startsAt time.Time, hours int) *time.Time {
	if hours <= 0 {
		return nil
	}
	t := startsAt.Add(-time.Duration(hours) * time.Hour)
	return &t
}

// CanRegister reports whether new sign-ups are accepted.
func CanRegister(sessionStatus string, startsAt, now time.Time) error {
	if sessionStatus != SessionPublished {
		return ErrSessionNotOpen
	}
	if !now.Before(startsAt) {
		return ErrSessionStarted
	}
	return nil
}

// CanChangeSeat covers cancel and slot transfer, both allowed until the session starts.
func CanChangeSeat(sessionStatus, regStatus string, startsAt, now time.Time) error {
	if !IsActive(regStatus) {
		return ErrNotActive
	}
	if sessionStatus != SessionPublished && sessionStatus != SessionDraft {
		return ErrSessionNotOpen
	}
	if !now.Before(startsAt) {
		return ErrSessionStarted
	}
	return nil
}

// CanUploadProof allows proof uploads for unpaid or rejected active registrations until the session ends.
func CanUploadProof(sessionStatus, regStatus, paymentStatus string, endsAt, now time.Time) error {
	if !IsActive(regStatus) {
		return ErrNotActive
	}
	if sessionStatus != SessionPublished {
		return ErrSessionNotOpen
	}
	if !now.Before(endsAt) {
		return ErrSessionStarted
	}
	if paymentStatus != PaymentUnpaid && paymentStatus != PaymentRejected {
		return ErrPaymentNotAccepted
	}
	return nil
}

// CancelPaymentOutcome decides what happens to the money when a participant cancels (PRD S5).
// Waitlisted participants never held a slot, so a paid waitlist seat is always refunded.
func CancelPaymentOutcome(regStatus, paymentStatus string, now, refundDeadline time.Time) string {
	if paymentStatus != PaymentPaid {
		return paymentStatus
	}
	if regStatus == StatusWaitlisted || now.Before(refundDeadline) {
		return PaymentRefundPending
	}
	return PaymentForfeited
}

// PickUniqueCode returns a random code in 1..499 not used in the session yet.
func PickUniqueCode(used map[int32]bool, r *rand.Rand) (int32, error) {
	const maxCode = 499
	if len(used) >= maxCode {
		return 0, ErrUniqueCodesUsed
	}
	for range 64 {
		c := int32(r.IntN(maxCode) + 1)
		if !used[c] {
			return c, nil
		}
	}
	free := make([]int32, 0, maxCode)
	for c := int32(1); c <= maxCode; c++ {
		if !used[c] {
			free = append(free, c)
		}
	}
	if len(free) == 0 {
		return 0, ErrUniqueCodesUsed
	}
	return free[r.IntN(len(free))], nil
}

// DefaultCapacity is court count times players per court.
func DefaultCapacity(courts, perCourt int) int {
	return courts * perCourt
}

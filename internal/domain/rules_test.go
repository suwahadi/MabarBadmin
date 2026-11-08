package domain

import (
	"errors"
	"math/rand/v2"
	"testing"
	"time"
)

func jakarta(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestRefundDeadline(t *testing.T) {
	loc := jakarta(t)
	start := time.Date(2026, 11, 9, 21, 0, 0, 0, loc)
	got := RefundDeadline(start, 1, loc)
	want := time.Date(2026, 11, 8, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if got := RefundDeadline(start, 0, loc); !got.Equal(time.Date(2026, 11, 9, 0, 0, 0, 0, loc)) {
		t.Fatalf("H-0 got %v", got)
	}
}

func TestPaymentDeadline(t *testing.T) {
	start := time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)
	if PaymentDeadline(start, 0) != nil {
		t.Fatal("0 hours must be flexible")
	}
	if d := PaymentDeadline(start, 48); d == nil || !d.Equal(start.Add(-48*time.Hour)) {
		t.Fatalf("got %v", d)
	}
}

func TestCancelPaymentOutcome(t *testing.T) {
	loc := jakarta(t)
	deadline := time.Date(2026, 11, 8, 0, 0, 0, 0, loc)
	before := deadline.Add(-time.Minute)
	after := deadline.Add(time.Minute)
	cases := []struct {
		status, payment string
		now             time.Time
		want            string
	}{
		{StatusRegistered, PaymentPaid, before, PaymentRefundPending},
		{StatusRegistered, PaymentPaid, after, PaymentForfeited},
		{StatusWaitlisted, PaymentPaid, after, PaymentRefundPending},
		{StatusRegistered, PaymentUnpaid, after, PaymentUnpaid},
		{StatusRegistered, PaymentPending, before, PaymentPending},
		{StatusRegistered, PaymentPaid, deadline, PaymentForfeited},
	}
	for _, c := range cases {
		if got := CancelPaymentOutcome(c.status, c.payment, c.now, deadline); got != c.want {
			t.Errorf("%s/%s at %v: got %s want %s", c.status, c.payment, c.now, got, c.want)
		}
	}
}

func TestCanRegisterAndChangeSeat(t *testing.T) {
	start := t0.Add(time.Hour)
	if err := CanRegister(SessionPublished, start, t0); err != nil {
		t.Fatal(err)
	}
	if err := CanRegister(SessionDraft, start, t0); !errors.Is(err, ErrSessionNotOpen) {
		t.Fatalf("got %v", err)
	}
	if err := CanRegister(SessionPublished, start, start); !errors.Is(err, ErrSessionStarted) {
		t.Fatalf("got %v", err)
	}
	if err := CanChangeSeat(SessionPublished, StatusRegistered, start, t0); err != nil {
		t.Fatal(err)
	}
	if err := CanChangeSeat(SessionPublished, StatusCancelled, start, t0); !errors.Is(err, ErrNotActive) {
		t.Fatalf("got %v", err)
	}
	if err := CanChangeSeat(SessionClosed, StatusRegistered, start, t0); !errors.Is(err, ErrSessionNotOpen) {
		t.Fatalf("got %v", err)
	}
	if err := CanChangeSeat(SessionPublished, StatusWaitlisted, start, start.Add(time.Second)); !errors.Is(err, ErrSessionStarted) {
		t.Fatalf("got %v", err)
	}
}

func TestCanUploadProof(t *testing.T) {
	end := t0.Add(3 * time.Hour)
	if err := CanUploadProof(SessionPublished, StatusRegistered, PaymentUnpaid, end, t0); err != nil {
		t.Fatal(err)
	}
	if err := CanUploadProof(SessionPublished, StatusWaitlisted, PaymentRejected, end, t0); err != nil {
		t.Fatal(err)
	}
	if err := CanUploadProof(SessionPublished, StatusRegistered, PaymentPending, end, t0); !errors.Is(err, ErrPaymentNotAccepted) {
		t.Fatalf("got %v", err)
	}
	if err := CanUploadProof(SessionPublished, StatusCancelled, PaymentUnpaid, end, t0); !errors.Is(err, ErrNotActive) {
		t.Fatalf("got %v", err)
	}
	if err := CanUploadProof(SessionClosed, StatusRegistered, PaymentUnpaid, end, t0); !errors.Is(err, ErrSessionNotOpen) {
		t.Fatalf("got %v", err)
	}
	if err := CanUploadProof(SessionPublished, StatusRegistered, PaymentUnpaid, end, end); !errors.Is(err, ErrSessionStarted) {
		t.Fatalf("got %v", err)
	}
}

func TestPickUniqueCode(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	used := map[int32]bool{}
	for range 499 {
		c, err := PickUniqueCode(used, r)
		if err != nil {
			t.Fatal(err)
		}
		if c < 1 || c > 499 || used[c] {
			t.Fatalf("bad code %d", c)
		}
		used[c] = true
	}
	if _, err := PickUniqueCode(used, r); !errors.Is(err, ErrUniqueCodesUsed) {
		t.Fatalf("expected exhaustion, got %v", err)
	}
}

func TestStatusHelpers(t *testing.T) {
	if !HoldsSlot(StatusAttended) || HoldsSlot(StatusWaitlisted) {
		t.Fatal("HoldsSlot")
	}
	if !IsActive(StatusWaitlisted) || IsActive(StatusTransferred) {
		t.Fatal("IsActive")
	}
	if !CountsAsIncome(PaymentForfeited) || CountsAsIncome(PaymentPending) {
		t.Fatal("CountsAsIncome")
	}
	if !ValidLevel(LevelAdvanced) || ValidLevel("pro") {
		t.Fatal("ValidLevel")
	}
	if LevelWeight(LevelBeginner) != 1 || LevelWeight(LevelIntermediate) != 2 || LevelWeight(LevelAdvanced) != 3 {
		t.Fatal("LevelWeight")
	}
	if DefaultCapacity(3, 4) != 12 {
		t.Fatal("DefaultCapacity")
	}
}

func TestSlug(t *testing.T) {
	if got := Slugify("Senin Malam GBA 2026-11-09"); got != "senin-malam-gba-2026-11-09" {
		t.Fatalf("got %q", got)
	}
	if got := Slugify("  Cara   Bayar!! "); got != "cara-bayar" {
		t.Fatalf("got %q", got)
	}
	if err := ValidatePageSlug("cara-bayar"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePageSlug("admin"); !errors.Is(err, ErrReservedSlug) {
		t.Fatalf("got %v", err)
	}
	if err := ValidatePageSlug("Bad Slug"); !errors.Is(err, ErrInvalidSlug) {
		t.Fatalf("got %v", err)
	}
}

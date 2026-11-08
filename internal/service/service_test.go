package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/suwahadi/mabarbadmin/internal/database"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
	"github.com/suwahadi/mabarbadmin/internal/media"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

// newTestService connects to TEST_DATABASE_URL and recreates the schema so each test starts clean.
func newTestService(t *testing.T) *Service {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	resetSchema(t, pool)
	q := sqlc.New(pool)
	if err := settings.Seed(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(pool, settings.NewStore(q), media.NewService(t.TempDir(), log), log)
}

var schemaMu sync.Mutex

func resetSchema(t *testing.T, pool *pgxpool.Pool) {
	schemaMu.Lock()
	defer schemaMu.Unlock()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, pool, "up"); err != nil {
		t.Fatal(err)
	}
}

func admin() Actor { return AdminActor(0) }

func makeSession(t *testing.T, s *Service, capacity int32, startsIn time.Duration) sqlc.Session {
	t.Helper()
	ctx := context.Background()
	v, err := s.Q.CreateVenue(ctx, sqlc.CreateVenueParams{Name: "GOR Test", Address: "Jl. Test"})
	if err != nil {
		t.Fatal(err)
	}
	start := s.Now().Add(startsIn)
	sess, err := s.CreateSession(ctx, admin(), SessionInput{
		Title: "Senin Malam", VenueID: v.ID, StartsAt: start, EndsAt: start.Add(3 * time.Hour), CourtCount: 1,
		Capacity: capacity, Price: 60000, UniqueCodeEnabled: true, Publish: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

func join(t *testing.T, s *Service, sessionID int64, n int) sqlc.Registration {
	t.Helper()
	res, err := s.Register(context.Background(), GuestActor(), sessionID, RegisterInput{
		Name: fmt.Sprintf("Pemain %d", n), Phone: fmt.Sprintf("0812000000%02d", n), Level: domain.LevelIntermediate,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.Registration
}

func reg(t *testing.T, s *Service, id int64) sqlc.Registration {
	t.Helper()
	r, err := s.Q.GetRegistration(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func pngBytes(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func payWithProof(t *testing.T, s *Service, regID int64) {
	t.Helper()
	ctx := context.Background()
	if err := s.UploadProof(ctx, GuestActor(), regID, "qris", bytes.NewReader(pngBytes(40, 40))); err != nil {
		t.Fatal(err)
	}
	proofs, err := s.Q.ListProofsForRegistration(ctx, regID)
	if err != nil || len(proofs) == 0 {
		t.Fatalf("proof not stored: %v", err)
	}
	if err := s.ApproveProof(ctx, admin(), proofs[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterQuotaAndWaitlist(t *testing.T) {
	s := newTestService(t)
	sess := makeSession(t, s, 2, 48*time.Hour)
	a := join(t, s, sess.ID, 1)
	b := join(t, s, sess.ID, 2)
	c := join(t, s, sess.ID, 3)
	if a.Status != domain.StatusRegistered || b.Status != domain.StatusRegistered || c.Status != domain.StatusWaitlisted {
		t.Fatalf("statuses %s %s %s", a.Status, b.Status, c.Status)
	}
	if a.UniqueCode == b.UniqueCode || a.UniqueCode < 1 || a.UniqueCode > 499 {
		t.Fatalf("unique codes %d %d", a.UniqueCode, b.UniqueCode)
	}
	if a.AmountDue != 60000+int64(a.UniqueCode) {
		t.Fatalf("amount due %d", a.AmountDue)
	}
	_, err := s.Register(context.Background(), GuestActor(), sess.ID, RegisterInput{Name: "Dobel", Phone: "+62 812 0000 0001", Level: domain.LevelBeginner})
	if msg, ok := IsUserError(err); !ok {
		t.Fatalf("duplicate phone must be a user error, got %v", err)
	} else if msg == "" {
		t.Fatal("empty message")
	}
}

func TestGuestWithMemberPhoneMustLogin(t *testing.T) {
	s := newTestService(t)
	sess := makeSession(t, s, 4, 48*time.Hour)
	if _, err := s.CreateUser(context.Background(), SignupInput{Name: "Member", Phone: "081200000001", Password: "rahasia123"}); err != nil {
		t.Fatal(err)
	}
	_, err := s.Register(context.Background(), GuestActor(), sess.ID, RegisterInput{Name: "X", Phone: "081200000001", Level: domain.LevelBeginner})
	if _, ok := IsUserError(err); !ok {
		t.Fatalf("expected login prompt, got %v", err)
	}
}

func TestPaidWaitlistSwapsLatestUnpaid(t *testing.T) {
	s := newTestService(t)
	sess := makeSession(t, s, 2, 48*time.Hour)
	a := join(t, s, sess.ID, 1)
	b := join(t, s, sess.ID, 2)
	c := join(t, s, sess.ID, 3)
	payWithProof(t, s, c.ID)
	if got := reg(t, s, c.ID); got.Status != domain.StatusRegistered || got.PaymentStatus != domain.PaymentPaid {
		t.Fatalf("paid waitlist should be promoted, got %s/%s", got.Status, got.PaymentStatus)
	}
	if got := reg(t, s, b.ID); got.Status != domain.StatusWaitlisted {
		t.Fatalf("latest unpaid should be bumped, got %s", got.Status)
	}
	if got := reg(t, s, a.ID); got.Status != domain.StatusRegistered {
		t.Fatalf("earliest unpaid keeps slot, got %s", got.Status)
	}
}

func TestRejectedProofCanBeReuploaded(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	sess := makeSession(t, s, 2, 48*time.Hour)
	a := join(t, s, sess.ID, 1)
	if err := s.UploadProof(ctx, GuestActor(), a.ID, "bank_transfer", bytes.NewReader(pngBytes(30, 30))); err != nil {
		t.Fatal(err)
	}
	if err := s.UploadProof(ctx, GuestActor(), a.ID, "bank_transfer", bytes.NewReader(pngBytes(30, 30))); err == nil {
		t.Fatal("second upload while pending must fail")
	}
	proofs, _ := s.Q.ListProofsForRegistration(ctx, a.ID)
	if err := s.RejectProof(ctx, admin(), proofs[0].ID, ""); err == nil {
		t.Fatal("reject without note must fail")
	}
	if err := s.RejectProof(ctx, admin(), proofs[0].ID, "Nominal tidak cocok"); err != nil {
		t.Fatal(err)
	}
	if got := reg(t, s, a.ID); got.PaymentStatus != domain.PaymentRejected {
		t.Fatalf("got %s", got.PaymentStatus)
	}
	if err := s.UploadProof(ctx, GuestActor(), a.ID, "qris", bytes.NewReader(pngBytes(30, 30))); err != nil {
		t.Fatalf("re-upload after rejection: %v", err)
	}
	if err := s.UploadProof(ctx, GuestActor(), a.ID, "qris", bytes.NewReader([]byte("not an image"))); err == nil {
		t.Fatal("non-image must be rejected")
	}
}

func TestCancelBeforeAndAfterRefundDeadline(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	sess := makeSession(t, s, 2, 72*time.Hour)
	a := join(t, s, sess.ID, 1)
	b := join(t, s, sess.ID, 2)
	c := join(t, s, sess.ID, 3)
	d := join(t, s, sess.ID, 4)
	payWithProof(t, s, a.ID)
	payWithProof(t, s, b.ID)
	payWithProof(t, s, d.ID)

	outcome, err := s.Cancel(ctx, GuestActor(), a.ID, RefundAccount{BankName: "BCA", AccountNumber: "123", AccountName: "A"})
	if err != nil || outcome != domain.PaymentRefundPending {
		t.Fatalf("outcome %s err %v", outcome, err)
	}
	ref, err := s.Q.GetRefundByRegistration(ctx, a.ID)
	if err != nil || ref.Amount != a.AmountDue || ref.BankName != "BCA" {
		t.Fatalf("refund %+v err %v", ref, err)
	}
	if got := reg(t, s, d.ID); got.Status != domain.StatusRegistered {
		t.Fatalf("paid waitlist should fill the freed slot first, got %s", got.Status)
	}
	if got := reg(t, s, c.ID); got.Status != domain.StatusWaitlisted {
		t.Fatalf("unpaid waitlist stays, got %s", got.Status)
	}

	s.Now = func() time.Time { return sess.RefundDeadlineAt.Add(time.Minute) }
	outcome, err = s.Cancel(ctx, GuestActor(), b.ID, RefundAccount{})
	if err != nil || outcome != domain.PaymentForfeited {
		t.Fatalf("after deadline: outcome %s err %v", outcome, err)
	}
	if got := reg(t, s, c.ID); got.Status != domain.StatusRegistered {
		t.Fatalf("unpaid waitlist fills when no paid waitlist left, got %s", got.Status)
	}
	if err := s.CompleteRefund(ctx, admin(), ref.ID); err != nil {
		t.Fatal(err)
	}
	if got := reg(t, s, a.ID); got.PaymentStatus != domain.PaymentRefunded {
		t.Fatalf("got %s", got.PaymentStatus)
	}
}

func TestTransferMovesSeatWithPayment(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	sess := makeSession(t, s, 2, 48*time.Hour)
	a := join(t, s, sess.ID, 1)
	payWithProof(t, s, a.ID)
	res, err := s.Transfer(ctx, GuestActor(), a.ID, "Pengganti", "081299999999", domain.LevelBeginner)
	if err != nil {
		t.Fatal(err)
	}
	if res.TicketToken == "" {
		t.Fatal("new ticket token expected")
	}
	next := reg(t, s, res.Registration.ID)
	if next.Status != domain.StatusRegistered || next.PaymentStatus != domain.PaymentPaid || next.AmountDue != a.AmountDue {
		t.Fatalf("next %+v", next)
	}
	old := reg(t, s, a.ID)
	if old.Status != domain.StatusTransferred || old.TransferredToID == nil || *old.TransferredToID != next.ID {
		t.Fatalf("old %+v", old)
	}
	if _, err := s.Transfer(ctx, GuestActor(), next.ID, "Lagi", "081299999999", domain.LevelBeginner); err == nil {
		t.Fatal("transfer to a phone already in the session must fail")
	}
}

func TestPendingProofFollowsTransfer(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	sess := makeSession(t, s, 2, 48*time.Hour)
	a := join(t, s, sess.ID, 1)
	if err := s.UploadProof(ctx, GuestActor(), a.ID, "qris", bytes.NewReader(pngBytes(30, 30))); err != nil {
		t.Fatal(err)
	}
	res, err := s.Transfer(ctx, GuestActor(), a.ID, "Pengganti", "081299999999", domain.LevelBeginner)
	if err != nil {
		t.Fatal(err)
	}
	proofs, _ := s.Q.ListProofsForRegistration(ctx, a.ID)
	if err := s.ApproveProof(ctx, admin(), proofs[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := reg(t, s, res.Registration.ID); got.PaymentStatus != domain.PaymentPaid {
		t.Fatalf("approval must settle the transferred seat, got %s", got.PaymentStatus)
	}
}

func TestCloseSession(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	sess := makeSession(t, s, 1, 48*time.Hour)
	a := join(t, s, sess.ID, 1)
	b := join(t, s, sess.ID, 2)
	payWithProof(t, s, a.ID)
	payWithProof(t, s, b.ID)
	if got := reg(t, s, b.ID); got.Status != domain.StatusWaitlisted {
		t.Fatalf("got %s", got.Status)
	}
	if err := s.CloseSession(ctx, admin(), sess.ID); err != nil {
		t.Fatal(err)
	}
	if got := reg(t, s, a.ID); got.Status != domain.StatusNoShow {
		t.Fatalf("unmarked registered becomes no_show, got %s", got.Status)
	}
	if got := reg(t, s, b.ID); got.PaymentStatus != domain.PaymentRefundPending {
		t.Fatalf("paid waitlist goes to refund, got %s", got.PaymentStatus)
	}
	if _, err := s.SetAttendance(ctx, admin(), a.ID, domain.StatusAttended); err != nil {
		t.Fatalf("attendance correction after close: %v", err)
	}
	rep, err := s.FinanceMonth(ctx, sess.StartsAt, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Sessions) == 0 || rep.Sessions[0].Income != a.AmountDue+b.AmountDue || rep.Sessions[0].Refunds != b.AmountDue {
		t.Fatalf("finance %+v", rep.Sessions)
	}
}

func TestCancelSessionRefundsEveryone(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	sess := makeSession(t, s, 1, 48*time.Hour)
	a := join(t, s, sess.ID, 1)
	b := join(t, s, sess.ID, 2)
	payWithProof(t, s, a.ID)
	payWithProof(t, s, b.ID)
	if err := s.CancelSession(ctx, admin(), sess.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{a.ID, b.ID} {
		if got := reg(t, s, id); got.PaymentStatus != domain.PaymentRefundPending {
			t.Fatalf("registration %d: %s", id, got.PaymentStatus)
		}
	}
}

func TestAdminMoveAndManualPayment(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	sess := makeSession(t, s, 1, 48*time.Hour)
	a := join(t, s, sess.ID, 1)
	b := join(t, s, sess.ID, 2)
	if err := s.AdminMove(ctx, admin(), b.ID, domain.StatusRegistered); err != nil {
		t.Fatal(err)
	}
	if reg(t, s, b.ID).Status != domain.StatusRegistered || reg(t, s, a.ID).Status != domain.StatusWaitlisted {
		t.Fatal("admin move to slot should bump the latest unpaid other participant")
	}
	if err := s.MarkPaidManually(ctx, admin(), b.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.AdminMove(ctx, admin(), a.ID, domain.StatusRegistered); err == nil {
		t.Fatal("moving into a full session of paid players must fail")
	}
	if err := s.AdminDelete(ctx, admin(), b.ID); err == nil {
		t.Fatal("paid registration must not be deleted")
	}
	if err := s.AdminDelete(ctx, admin(), a.ID); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentJoinsNeverExceedQuota(t *testing.T) {
	s := newTestService(t)
	sess := makeSession(t, s, 5, 48*time.Hour)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := s.Register(context.Background(), GuestActor(), sess.ID, RegisterInput{
				Name: fmt.Sprintf("P%d", n), Phone: fmt.Sprintf("08130000%04d", n), Level: domain.LevelBeginner,
			})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	regs, _ := s.Q.ListSessionRegistrations(context.Background(), sess.ID)
	held := 0
	for _, r := range regs {
		if r.Status == domain.StatusRegistered {
			held++
		}
	}
	if held != 5 || len(regs) != 20 {
		t.Fatalf("held %d of %d", held, len(regs))
	}
}

func TestRotationRounds(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	sess := makeSession(t, s, 10, time.Hour)
	for i := 1; i <= 6; i++ {
		r := join(t, s, sess.ID, i)
		if _, err := s.SetAttendance(ctx, admin(), r.ID, domain.StatusAttended); err != nil {
			t.Fatal(err)
		}
	}
	for range 3 {
		if err := s.NextRotationRound(ctx, admin(), sess.ID, 1); err != nil {
			t.Fatal(err)
		}
	}
	rounds, err := Rotation(ctx, s.Q, sess.ID)
	if err != nil || len(rounds) != 3 || len(rounds[0].Matches) != 1 {
		t.Fatalf("rounds %+v err %v", rounds, err)
	}
	if err := s.UndoRotationRound(ctx, admin(), sess.ID); err != nil {
		t.Fatal(err)
	}
	rounds, _ = Rotation(ctx, s.Q, sess.ID)
	if len(rounds) != 2 {
		t.Fatalf("undo left %d rounds", len(rounds))
	}
}

func TestLeaderboardAndGuestMerge(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	sess := makeSession(t, s, 4, time.Hour)
	g := join(t, s, sess.ID, 7)
	if _, err := s.SetAttendance(ctx, admin(), g.ID, domain.StatusAttended); err != nil {
		t.Fatal(err)
	}
	u, err := s.CreateUser(ctx, SignupInput{Name: "Dulu Tamu", Phone: "081200000007", Password: "rahasia123"})
	if err != nil {
		t.Fatal(err)
	}
	if got := reg(t, s, g.ID); got.UserID == nil || *got.UserID != u.ID {
		t.Fatal("guest history must be linked to the new member")
	}
	board, _, err := s.Leaderboard(ctx, "all", 10, time.UTC)
	if err != nil || len(board) != 1 || board[0].Attended != 1 || board[0].Percent != 100 {
		t.Fatalf("board %+v err %v", board, err)
	}
}

func TestAuthenticate(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	if _, err := s.CreateUser(ctx, SignupInput{Name: "A", Phone: "081200000011", Password: "rahasia123"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(ctx, "0812-0000-0011", "salah123", "test"); err == nil {
		t.Fatal("wrong password accepted")
	}
	u, token, err := s.Authenticate(ctx, "0812-0000-0011", "rahasia123", "test")
	if err != nil || token == "" {
		t.Fatal(err)
	}
	temp, err := s.ResetPassword(ctx, admin(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	u2, _, err := s.Authenticate(ctx, "081200000011", temp, "test")
	if err != nil || !u2.MustChangePassword {
		t.Fatalf("reset login %v must-change %v", err, u2.MustChangePassword)
	}
}

func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestReplacingLogoRemovesOldFiles(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	if err := s.SetSettingMedia(ctx, admin(), settings.LogoLightMediaID, Upload{Reader: bytes.NewReader(pngBytes(400, 200))}); err != nil {
		t.Fatal(err)
	}
	first := listFiles(t, s.Media.PublicDir())
	if len(first) != 2 {
		t.Fatalf("logo should have 2 variants, got %v", first)
	}
	if err := s.SetSettingMedia(ctx, admin(), settings.LogoLightMediaID, Upload{Reader: bytes.NewReader(pngBytes(400, 200))}); err != nil {
		t.Fatal(err)
	}
	second := listFiles(t, s.Media.PublicDir())
	if len(second) != 2 {
		t.Fatalf("old logo files must be gone, got %v", second)
	}
	for _, f := range first {
		if _, err := os.Stat(f); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("old file %s still exists", f)
		}
	}
}

func TestDeletingSlideRemovesAllVariants(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	err := s.CreateHeroSlide(ctx, admin(), HeroInput{
		Title: "Mabar", Active: true,
		Image:       Upload{Reader: bytes.NewReader(pngBytes(1700, 900)), Alt: "Lapangan"},
		MobileImage: Upload{Reader: bytes.NewReader(pngBytes(900, 1200))},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(listFiles(t, s.Media.PublicDir())); n != 5 {
		t.Fatalf("expected 3 desktop + 2 mobile variants, got %d", n)
	}
	slides, _ := s.Q.ListHeroSlides(ctx)
	if err := s.DeleteHeroSlide(ctx, admin(), slides[0].ID); err != nil {
		t.Fatal(err)
	}
	if n := len(listFiles(t, s.Media.PublicDir())); n != 0 {
		t.Fatalf("all variants must be deleted, %d left", n)
	}
}

func TestRollbackKeepsOldFiles(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	if err := s.SetSettingMedia(ctx, admin(), settings.LogoLightMediaID, Upload{Reader: bytes.NewReader(pngBytes(300, 100))}); err != nil {
		t.Fatal(err)
	}
	before := listFiles(t, s.Media.PublicDir())
	vals, _ := s.Settings.Get(ctx)
	oldID := vals.MediaID(settings.LogoLightMediaID)

	boom := errors.New("boom")
	err := s.withMedia(ctx, func(q *sqlc.Queries, drop func(*int64) error) error {
		if _, err := s.Media.Save(ctx, q, bytes.NewReader(pngBytes(300, 100)), "logo", ""); err != nil {
			return err
		}
		if err := drop(oldID); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	for _, f := range before {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("old file %s removed despite rollback", f)
		}
	}
	if _, err := s.Q.GetMedia(ctx, *oldID); err != nil {
		t.Fatal("old media row must survive rollback")
	}
	res, err := s.Media.Sweep(ctx, s.Q, -time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if res.Files != 2 {
		t.Fatalf("sweeper should remove the 2 orphan files of the rolled-back upload, removed %d", res.Files)
	}
	for _, f := range before {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("sweeper removed a referenced file %s", f)
		}
	}
}

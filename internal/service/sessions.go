package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

type SessionInput struct {
	Title             string
	Slug              string
	TemplateID        *int64
	VenueID           int64
	StartsAt          time.Time
	EndsAt            time.Time
	CourtCount        int32
	Capacity          int32
	Price             int64
	UniqueCodeEnabled bool
	PaymentDeadlineAt *time.Time
	RefundDeadlineAt  *time.Time
	PointRule         string
	LevelLabel        string
	Shuttlecock       bool
	Notes             string
	MetaTitle         string
	MetaDescription   string
	Publish           bool
}

func (in *SessionInput) validate() error {
	in.Title = strings.TrimSpace(in.Title)
	switch {
	case in.Title == "":
		return userErr("Judul sesi wajib diisi.")
	case in.VenueID == 0:
		return userErr("Pilih venue.")
	case !in.EndsAt.After(in.StartsAt):
		return userErr("Jam selesai harus setelah jam mulai.")
	case in.CourtCount < 1:
		return userErr("Jumlah lapangan minimal 1.")
	case in.Capacity < 1:
		return userErr("Kuota minimal 1.")
	case in.Price < 0:
		return userErr("HTM tidak boleh negatif.")
	}
	if in.Slug != "" && domain.Slugify(in.Slug) != in.Slug {
		return userErr(domain.ErrInvalidSlug.Error() + ".")
	}
	return nil
}

func (s *Service) uniqueSessionSlug(ctx context.Context, q *sqlc.Queries, base string, selfID int64) (string, error) {
	if base == "" {
		base = "sesi"
	}
	slug := base
	for i := 2; ; i++ {
		exists, err := q.SessionSlugExists(ctx, sqlc.SessionSlugExistsParams{Slug: slug, ID: selfID})
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func (s *Service) CreateSession(ctx context.Context, a Actor, in SessionInput) (sqlc.Session, error) {
	if err := in.validate(); err != nil {
		return sqlc.Session{}, err
	}
	vals, err := s.Settings.Get(ctx)
	if err != nil {
		return sqlc.Session{}, err
	}
	loc := vals.Location()
	refund := domain.RefundDeadline(in.StartsAt, int(vals.Int(settings.RefundDeadlineDays)), loc)
	if in.RefundDeadlineAt != nil {
		refund = *in.RefundDeadlineAt
	}
	status := domain.SessionDraft
	if in.Publish {
		status = domain.SessionPublished
	}
	var out sqlc.Session
	err = s.tx(ctx, func(q *sqlc.Queries) error {
		base := in.Slug
		if base == "" {
			base = domain.Slugify(in.Title + " " + in.StartsAt.In(loc).Format("2006-01-02"))
		}
		slug, err := s.uniqueSessionSlug(ctx, q, base, 0)
		if err != nil {
			return err
		}
		out, err = q.CreateSession(ctx, sqlc.CreateSessionParams{
			Slug: slug, Title: in.Title, TemplateID: in.TemplateID, VenueID: in.VenueID, StartsAt: in.StartsAt,
			EndsAt: in.EndsAt, CourtCount: in.CourtCount, Capacity: in.Capacity, Price: in.Price,
			UniqueCodeEnabled: in.UniqueCodeEnabled, PaymentDeadlineAt: in.PaymentDeadlineAt, RefundDeadlineAt: refund,
			Status: status, PointRule: in.PointRule, LevelLabel: in.LevelLabel, ShuttlecockIncluded: in.Shuttlecock,
			Notes: in.Notes, MetaTitle: in.MetaTitle, MetaDescription: in.MetaDescription,
		})
		if err != nil {
			return err
		}
		return logActivity(ctx, q, a, "session", out.ID, "created", map[string]any{"status": status})
	})
	return out, err
}

// TemplateSessionInput fills a session form from a template for the given local date.
func (s *Service) TemplateSessionInput(ctx context.Context, templateID int64, date time.Time) (SessionInput, error) {
	t, err := s.Q.GetTemplate(ctx, templateID)
	if err != nil {
		return SessionInput{}, notFound(err)
	}
	vals, err := s.Settings.Get(ctx)
	if err != nil {
		return SessionInput{}, err
	}
	loc := vals.Location()
	start := atClock(date, t.StartTime, loc)
	end := atClock(date, t.EndTime, loc)
	if !end.After(start) {
		end = end.AddDate(0, 0, 1)
	}
	id := t.ID
	return SessionInput{
		Title: t.Name, TemplateID: &id, VenueID: t.VenueID, StartsAt: start, EndsAt: end,
		CourtCount: t.CourtCount, Capacity: t.Capacity, Price: t.Price, UniqueCodeEnabled: t.UniqueCodeEnabled,
		PaymentDeadlineAt: domain.PaymentDeadline(start, int(vals.Int(settings.PaymentDeadlineHrs))),
		PointRule:         t.PointRule, LevelLabel: t.LevelLabel, Shuttlecock: t.ShuttlecockIncluded, Notes: t.Notes,
	}, nil
}

// NextTemplateDate returns the next local date (today included) that falls on the template weekday.
func NextTemplateDate(weekday int16, now time.Time, loc *time.Location) time.Time {
	l := now.In(loc)
	d := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
	for d.Weekday() != time.Weekday(weekday) {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

func atClock(date time.Time, clock pgtype.Time, loc *time.Location) time.Time {
	secs := clock.Microseconds / 1e6
	return time.Date(date.Year(), date.Month(), date.Day(), int(secs/3600), int(secs%3600/60), 0, 0, loc)
}

func (s *Service) UpdateSession(ctx context.Context, a Actor, id int64, in SessionInput) (sqlc.Session, error) {
	if err := in.validate(); err != nil {
		return sqlc.Session{}, err
	}
	vals, err := s.Settings.Get(ctx)
	if err != nil {
		return sqlc.Session{}, err
	}
	var out sqlc.Session
	err = s.tx(ctx, func(q *sqlc.Queries) error {
		cur, err := q.GetSessionForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if cur.Status == domain.SessionClosed || cur.Status == domain.SessionCancelled {
			return userErr("Sesi yang sudah ditutup atau dibatalkan tidak bisa diubah.")
		}
		slug := cur.Slug
		if in.Slug != "" && in.Slug != cur.Slug {
			slug, err = s.uniqueSessionSlug(ctx, q, in.Slug, id)
			if err != nil {
				return err
			}
		}
		refund := cur.RefundDeadlineAt
		if in.RefundDeadlineAt != nil {
			refund = *in.RefundDeadlineAt
		} else if !in.StartsAt.Equal(cur.StartsAt) {
			refund = domain.RefundDeadline(in.StartsAt, int(vals.Int(settings.RefundDeadlineDays)), vals.Location())
		}
		out, err = q.UpdateSession(ctx, sqlc.UpdateSessionParams{
			ID: id, Slug: slug, Title: in.Title, VenueID: in.VenueID, StartsAt: in.StartsAt, EndsAt: in.EndsAt,
			CourtCount: in.CourtCount, Capacity: in.Capacity, Price: in.Price, UniqueCodeEnabled: in.UniqueCodeEnabled,
			PaymentDeadlineAt: in.PaymentDeadlineAt, RefundDeadlineAt: refund, PointRule: in.PointRule,
			LevelLabel: in.LevelLabel, ShuttlecockIncluded: in.Shuttlecock, Notes: in.Notes, MetaTitle: in.MetaTitle,
			MetaDescription: in.MetaDescription,
		})
		if err != nil {
			return err
		}
		if slug != cur.Slug {
			if err := s.recordSlugChange(ctx, q, "session", cur.Slug, slug); err != nil {
				return err
			}
		}
		if err := logActivity(ctx, q, a, "session", id, "updated", map[string]any{"capacity": in.Capacity}); err != nil {
			return err
		}
		if cur.Status == domain.SessionPublished {
			return s.rebalance(ctx, q, out, nil)
		}
		return nil
	})
	return out, err
}

func (s *Service) recordSlugChange(ctx context.Context, q *sqlc.Queries, entity, oldSlug, newSlug string) error {
	if err := q.DeleteSlugRedirect(ctx, sqlc.DeleteSlugRedirectParams{EntityType: entity, OldSlug: newSlug}); err != nil {
		return err
	}
	if err := q.RepointSlugRedirects(ctx, sqlc.RepointSlugRedirectsParams{EntityType: entity, NewSlug: oldSlug, NewSlug_2: newSlug}); err != nil {
		return err
	}
	return q.CreateSlugRedirect(ctx, sqlc.CreateSlugRedirectParams{EntityType: entity, OldSlug: oldSlug, NewSlug: newSlug})
}

func (s *Service) PublishSession(ctx context.Context, a Actor, id int64) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		sess, err := q.GetSessionForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if sess.Status != domain.SessionDraft {
			return userErr("Hanya draf yang bisa dipublikasikan.")
		}
		if err := q.SetSessionStatus(ctx, sqlc.SetSessionStatusParams{ID: id, Status: domain.SessionPublished}); err != nil {
			return err
		}
		sess.Status = domain.SessionPublished
		if err := logActivity(ctx, q, a, "session", id, "published", nil); err != nil {
			return err
		}
		return s.rebalance(ctx, q, sess, nil)
	})
}

func (s *Service) SetCosts(ctx context.Context, a Actor, id int64, court, shuttle, other int64) error {
	if court < 0 || shuttle < 0 || other < 0 {
		return userErr("Biaya tidak boleh negatif.")
	}
	return s.tx(ctx, func(q *sqlc.Queries) error {
		if _, err := q.GetSessionForUpdate(ctx, id); err != nil {
			return notFound(err)
		}
		if err := q.UpdateSessionCosts(ctx, sqlc.UpdateSessionCostsParams{ID: id, CourtCost: court, ShuttlecockCost: shuttle, OtherCost: other}); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "session", id, "costs_updated", map[string]any{"court": court, "shuttlecock": shuttle, "other": other})
	})
}

// CloseSession applies rule 7: unmarked registered participants become no-show and paid waiting
// list participants move to the refund queue.
func (s *Service) CloseSession(ctx context.Context, a Actor, id int64) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		sess, err := q.GetSessionForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if sess.Status != domain.SessionPublished {
			return userErr("Hanya sesi yang sedang terbit yang bisa ditutup.")
		}
		if err := q.MarkUnattendedNoShow(ctx, id); err != nil {
			return err
		}
		paidWaitlist, err := q.ListPaidWaitlisted(ctx, id)
		if err != nil {
			return err
		}
		for _, r := range paidWaitlist {
			if err := s.queueRefund(ctx, q, a, r, RefundAccount{}); err != nil {
				return err
			}
		}
		if err := q.SetSessionStatus(ctx, sqlc.SetSessionStatusParams{ID: id, Status: domain.SessionClosed}); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "session", id, "closed", map[string]any{"waitlist_refunds": len(paidWaitlist)})
	})
}

// CancelSession refunds every paid participant because the session will not take place.
func (s *Service) CancelSession(ctx context.Context, a Actor, id int64) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		sess, err := q.GetSessionForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if sess.Status != domain.SessionPublished && sess.Status != domain.SessionDraft {
			return userErr("Sesi ini sudah ditutup atau dibatalkan.")
		}
		paid, err := q.ListPaidInSession(ctx, id)
		if err != nil {
			return err
		}
		for _, r := range paid {
			if err := s.queueRefund(ctx, q, a, r, RefundAccount{}); err != nil {
				return err
			}
		}
		if err := q.SetSessionStatus(ctx, sqlc.SetSessionStatusParams{ID: id, Status: domain.SessionCancelled}); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "session", id, "cancelled", map[string]any{"refunds": len(paid)})
	})
}

func (s *Service) DeleteDraftSession(ctx context.Context, id int64) error {
	sess, err := s.Q.GetSession(ctx, id)
	if err != nil {
		return notFound(err)
	}
	if sess.Status != domain.SessionDraft {
		return userErr("Hanya draf yang bisa dihapus.")
	}
	return s.Q.DeleteSession(ctx, id)
}

// rebalance runs the allocation rules for a locked session and persists the resulting moves.
func (s *Service) rebalance(ctx context.Context, q *sqlc.Queries, sess sqlc.Session, hold map[int64]bool) error {
	if sess.Status != domain.SessionPublished && sess.Status != domain.SessionDraft {
		return nil
	}
	regs, err := q.ListSessionRegistrations(ctx, sess.ID)
	if err != nil {
		return err
	}
	var seats []domain.Seat
	for _, r := range regs {
		if !domain.IsActive(r.Status) {
			continue
		}
		seat := domain.Seat{ID: r.ID, Status: r.Status, QueuedAt: r.QueuedAt, Paid: r.PaymentStatus == domain.PaymentPaid}
		if r.PaidAt != nil {
			seat.PaidAt = *r.PaidAt
		}
		seats = append(seats, seat)
	}
	deadlinePassed := sess.PaymentDeadlineAt != nil && !s.Now().Before(*sess.PaymentDeadlineAt)
	moves := domain.Allocate(int(sess.Capacity), seats, domain.AllocateOptions{DeadlinePassed: deadlinePassed, Hold: hold})
	for _, m := range moves {
		if err := q.UpdateRegistrationStatus(ctx, sqlc.UpdateRegistrationStatusParams{ID: m.ID, Status: m.To}); err != nil {
			return err
		}
		action := "promoted"
		if m.To == domain.StatusWaitlisted {
			action = "moved_to_waitlist"
		}
		if err := logActivity(ctx, q, SystemActor(), "registration", m.ID, action, nil); err != nil {
			return err
		}
	}
	return nil
}

// EnforcePaymentDeadline re-runs allocation for sessions whose explicit payment deadline passed.
func (s *Service) EnforcePaymentDeadline(ctx context.Context, id int64) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		sess, err := q.GetSessionForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		return s.rebalance(ctx, q, sess, nil)
	})
}

package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/service"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

type Scheduler struct {
	Svc *service.Service
	Log *slog.Logger
}

// Run executes periodic jobs until ctx is cancelled. Jobs are idempotent, so running them on
// every instance start is safe.
func (s *Scheduler) Run(ctx context.Context) {
	frequent := time.NewTicker(5 * time.Minute)
	daily := time.NewTicker(24 * time.Hour)
	weekly := time.NewTicker(7 * 24 * time.Hour)
	defer frequent.Stop()
	defer daily.Stop()
	defer weekly.Stop()

	startup := time.NewTimer(30 * time.Second)
	defer startup.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-startup.C:
			s.sessions(ctx)
			s.retention(ctx)
			s.sweep(ctx)
		case <-frequent.C:
			s.sessions(ctx)
		case <-daily.C:
			s.retention(ctx)
		case <-weekly.C:
			s.sweep(ctx)
		}
	}
}

func (s *Scheduler) sessions(ctx context.Context) {
	vals, err := s.Svc.Settings.Get(ctx)
	if err != nil {
		s.Log.Error("scheduler settings", "err", err)
		return
	}
	cutoff := time.Now().Add(-time.Duration(vals.Int(settings.AutoCloseHours)) * time.Hour)
	ids, err := s.Svc.Q.ListSessionsToAutoClose(ctx, cutoff)
	if err != nil {
		s.Log.Error("list sessions to close", "err", err)
	}
	for _, id := range ids {
		if err := s.Svc.CloseSession(ctx, service.SystemActor(), id); err != nil {
			s.Log.Error("auto close session", "session_id", id, "err", err)
			continue
		}
		s.Log.Info("session auto-closed", "session_id", id)
	}
	ids, err = s.Svc.Q.ListSessionsPastPaymentDeadline(ctx)
	if err != nil {
		s.Log.Error("list payment deadlines", "err", err)
	}
	for _, id := range ids {
		if err := s.Svc.EnforcePaymentDeadline(ctx, id); err != nil {
			s.Log.Error("enforce payment deadline", "session_id", id, "err", err)
		}
	}
	if err := s.Svc.Q.DeleteExpiredAuthSessions(ctx); err != nil {
		s.Log.Error("delete expired auth sessions", "err", err)
	}
}

func (s *Scheduler) retention(ctx context.Context) {
	vals, err := s.Svc.Settings.Get(ctx)
	if err != nil {
		return
	}
	before := time.Now().AddDate(0, -int(vals.Int(settings.ProofRetentionMos)), 0)
	n, err := s.Svc.PurgeExpiredProofs(ctx, before)
	if err != nil {
		s.Log.Error("purge payment proofs", "err", err)
		return
	}
	if n > 0 {
		s.Log.Info("payment proofs purged", "count", n)
	}
}

func (s *Scheduler) sweep(ctx context.Context) {
	res, err := s.Svc.Media.Sweep(ctx, s.Svc.Q, 24*time.Hour)
	if err != nil {
		s.Log.Error("media sweep", "err", err)
		return
	}
	s.Log.Info("media sweep", "rows", res.Rows, "files", res.Files, "bytes_freed", res.Bytes)
}

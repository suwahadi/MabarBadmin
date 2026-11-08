package service

import (
	"context"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
)

type Period struct {
	Key   string
	Label string
}

var Periods = []Period{
	{"this_month", "Bulan ini"},
	{"last_3_months", "3 bulan"},
	{"this_year", "Tahun ini"},
	{"all", "Semua"},
}

// PeriodRange converts a leaderboard period key to a [since, until) range in local time.
func PeriodRange(key string, now time.Time, loc *time.Location) (time.Time, time.Time, string) {
	l := now.In(loc)
	monthStart := time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, loc)
	until := now.Add(24 * time.Hour * 366 * 10)
	switch key {
	case "last_3_months":
		return monthStart.AddDate(0, -2, 0), until, key
	case "this_year":
		return time.Date(l.Year(), 1, 1, 0, 0, 0, 0, loc), until, key
	case "all":
		return time.Date(2000, 1, 1, 0, 0, 0, 0, loc), until, key
	default:
		return monthStart, monthStart.AddDate(0, 1, 0), "this_month"
	}
}

type LeaderboardEntry struct {
	Rank       int
	UserID     int64
	Name       string
	Attended   int64
	NoShow     int64
	Percent    int
	LastPlayed time.Time
}

func (s *Service) Leaderboard(ctx context.Context, period string, limit int32, loc *time.Location) ([]LeaderboardEntry, string, error) {
	since, until, key := PeriodRange(period, s.Now(), loc)
	rows, err := s.Q.Leaderboard(ctx, sqlc.LeaderboardParams{Since: since, Until: until, MaxRows: limit})
	if err != nil {
		return nil, key, err
	}
	out := make([]LeaderboardEntry, len(rows))
	for i, r := range rows {
		pct := 0
		if total := r.Attended + r.NoShow; total > 0 {
			pct = int(r.Attended * 100 / total)
		}
		out[i] = LeaderboardEntry{Rank: i + 1, UserID: r.UserID, Name: r.Name, Attended: r.Attended, NoShow: r.NoShow, Percent: pct, LastPlayed: r.LastPlayed}
	}
	return out, key, nil
}

type SessionFinance struct {
	ID              int64
	Slug            string
	Title           string
	StartsAt        time.Time
	Status          string
	Income          int64
	Refunds         int64
	CourtCost       int64
	ShuttlecockCost int64
	OtherCost       int64
	Attended        int64
	Filled          int64
	Capacity        int32
}

func (f SessionFinance) Costs() int64 { return f.CourtCost + f.ShuttlecockCost + f.OtherCost }

// Net is income minus refunds owed or paid and session costs. Forfeited payments stay income.
func (f SessionFinance) Net() int64 { return f.Income - f.Refunds - f.Costs() }

type FinanceReport struct {
	Month    time.Time
	Sessions []SessionFinance
	Income   int64
	Refunds  int64
	Costs    int64
	Net      int64
}

func MonthRange(month time.Time, loc *time.Location) (time.Time, time.Time) {
	m := month.In(loc)
	start := time.Date(m.Year(), m.Month(), 1, 0, 0, 0, 0, loc)
	return start, start.AddDate(0, 1, 0)
}

func (s *Service) FinanceRange(ctx context.Context, since, until time.Time) ([]SessionFinance, error) {
	rows, err := s.Q.SessionFinanceRows(ctx, sqlc.SessionFinanceRowsParams{Since: since, Until: until})
	if err != nil {
		return nil, err
	}
	out := make([]SessionFinance, len(rows))
	for i, r := range rows {
		out[i] = SessionFinance{
			ID: r.ID, Slug: r.Slug, Title: r.Title, StartsAt: r.StartsAt, Status: r.Status, Income: r.Income,
			Refunds: r.Refunds, CourtCost: r.CourtCost, ShuttlecockCost: r.ShuttlecockCost, OtherCost: r.OtherCost,
			Attended: r.Attended, Filled: r.Filled, Capacity: r.Capacity,
		}
	}
	return out, nil
}

func (s *Service) FinanceMonth(ctx context.Context, month time.Time, loc *time.Location) (FinanceReport, error) {
	since, until := MonthRange(month, loc)
	rows, err := s.FinanceRange(ctx, since, until)
	if err != nil {
		return FinanceReport{}, err
	}
	rep := FinanceReport{Month: since, Sessions: rows}
	for _, r := range rows {
		rep.Income += r.Income
		rep.Refunds += r.Refunds
		rep.Costs += r.Costs()
	}
	rep.Net = rep.Income - rep.Refunds - rep.Costs
	return rep, nil
}

type MonthBar struct {
	Month  time.Time
	Income int64
	Costs  int64
}

type Dashboard struct {
	SessionsThisMonth int
	AttendedThisMonth int64
	NetThisMonth      int64
	AvgFillPercent    int
	PendingProofs     int64
	PendingRefunds    int64
	UnpaidNearest     int64
	Upcoming          []sqlc.ListUpcomingWithStatsRow
	Attendance        []sqlc.RecentAttendanceRow
	Months            []MonthBar
}

func (s *Service) Dashboard(ctx context.Context, loc *time.Location) (Dashboard, error) {
	var d Dashboard
	now := s.Now()
	rep, err := s.FinanceMonth(ctx, now, loc)
	if err != nil {
		return d, err
	}
	d.SessionsThisMonth = len(rep.Sessions)
	d.NetThisMonth = rep.Net
	var fillSum, fillN int
	for _, r := range rep.Sessions {
		d.AttendedThisMonth += r.Attended
		if r.Capacity > 0 {
			fillSum += int(r.Filled * 100 / int64(r.Capacity))
			fillN++
		}
	}
	if fillN > 0 {
		d.AvgFillPercent = fillSum / fillN
	}
	if d.PendingProofs, err = s.Q.CountPendingProofs(ctx); err != nil {
		return d, err
	}
	if d.PendingRefunds, err = s.Q.CountPendingRefunds(ctx); err != nil {
		return d, err
	}
	if d.UnpaidNearest, err = s.Q.CountUnpaidNearest(ctx); err != nil {
		return d, err
	}
	if d.Upcoming, err = s.Q.ListUpcomingWithStats(ctx); err != nil {
		return d, err
	}
	att, err := s.Q.RecentAttendance(ctx)
	if err != nil {
		return d, err
	}
	for i, j := 0, len(att)-1; i < j; i, j = i+1, j-1 {
		att[i], att[j] = att[j], att[i]
	}
	d.Attendance = att

	first, _ := MonthRange(now, loc)
	first = first.AddDate(0, -5, 0)
	rows, err := s.FinanceRange(ctx, first, first.AddDate(0, 6, 0))
	if err != nil {
		return d, err
	}
	for i := range 6 {
		m := first.AddDate(0, i, 0)
		bar := MonthBar{Month: m}
		for _, r := range rows {
			rm := r.StartsAt.In(loc)
			if rm.Year() == m.Year() && rm.Month() == m.Month() {
				bar.Income += r.Income - r.Refunds
				bar.Costs += r.Costs()
			}
		}
		d.Months = append(d.Months, bar)
	}
	return d, nil
}

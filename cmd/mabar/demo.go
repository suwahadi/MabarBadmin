package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
	"github.com/suwahadi/mabarbadmin/internal/service"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

func clock(h, m int) pgtype.Time {
	return pgtype.Time{Microseconds: int64(h*3600+m*60) * 1e6, Valid: true}
}

// seedDemo creates sample data for local development; it refuses to run twice.
func seedDemo(ctx context.Context, svc *service.Service) error {
	if venues, err := svc.Q.ListVenues(ctx); err != nil {
		return err
	} else if len(venues) > 0 {
		return fmt.Errorf("demo data needs an empty database (found %d venues)", len(venues))
	}
	if err := svc.Settings.Save(ctx, svc.Q, map[string]any{
		settings.BankName: "BCA", settings.BankAccountNumber: "1234567890", settings.BankAccountName: "Admin Mabar",
		settings.ContactWhatsApp: "081234567890", settings.ContactEmail: "halo@mabarbadmin.web.id",
		settings.ContactAddress: "Jl. Contoh No. 1, Jakarta", settings.SocialInstagram: "https://instagram.com/mabarbadmin",
	}); err != nil {
		return err
	}
	vals, _ := svc.Settings.Get(ctx)
	loc := vals.Location()
	admin := service.SystemActor()

	gba, err := svc.Q.CreateVenue(ctx, sqlc.CreateVenueParams{Name: "GOR Bulutangkis Asri", Address: "Jl. Kenanga No. 12, Jakarta Selatan", MapsUrl: "https://maps.google.com/?q=GOR+Bulutangkis"})
	if err != nil {
		return err
	}
	tpl, err := svc.Q.CreateTemplate(ctx, sqlc.CreateTemplateParams{
		Name: "Senin malam GBA", VenueID: gba.ID, Weekday: 1, StartTime: clock(21, 0), EndTime: clock(0, 0), CourtCount: 3, Capacity: 12,
		Price: 60000, PointRule: "15x2", LevelLabel: "Semua level", ShuttlecockIncluded: true, UniqueCodeEnabled: true,
		Notes: "Datang 15 menit sebelum mulai. Sistem gantian, 4 orang per lapangan.",
	})
	if err != nil {
		return err
	}
	if _, err := svc.Q.CreateTemplate(ctx, sqlc.CreateTemplateParams{
		Name: "Kamis malam GBA", VenueID: gba.ID, Weekday: 4, StartTime: clock(19, 0), EndTime: clock(22, 0), CourtCount: 2, Capacity: 8,
		Price: 50000, PointRule: "15x2", LevelLabel: "Menengah ke atas", ShuttlecockIncluded: true, UniqueCodeEnabled: true,
	}); err != nil {
		return err
	}

	names := []string{"Budi", "Sari", "Andi", "Rina", "Dewi", "Fajar", "Gilang", "Hana", "Indra", "Joko", "Kiki", "Lukman", "Maya", "Nanda"}
	levels := []string{domain.LevelIntermediate, domain.LevelBeginner, domain.LevelAdvanced, domain.LevelIntermediate}
	var members []sqlc.User
	for i, n := range names[:8] {
		u, err := svc.CreateUser(ctx, service.SignupInput{Name: n, Phone: fmt.Sprintf("0812000010%02d", i), Password: "demo12345", Level: levels[i%len(levels)]})
		if err != nil {
			return err
		}
		members = append(members, u)
	}

	past := service.NextTemplateDate(1, time.Now().AddDate(0, 0, -7), loc).AddDate(0, 0, -7)
	in, err := svc.TemplateSessionInput(ctx, tpl.ID, past)
	if err != nil {
		return err
	}
	in.Publish = true
	realNow := svc.Now
	svc.Now = func() time.Time { return in.StartsAt.Add(-48 * time.Hour) }
	closed, err := svc.CreateSession(ctx, admin, in)
	if err != nil {
		return err
	}
	for i, u := range members {
		res, err := svc.Register(ctx, service.MemberActor(u.ID), closed.ID, service.RegisterInput{UserID: &u.ID, Name: u.Name, Phone: u.Phone, Level: u.Level})
		if err != nil {
			return err
		}
		if err := svc.MarkPaidManually(ctx, admin, res.Registration.ID); err != nil {
			return err
		}
		status := domain.StatusAttended
		if i == 5 {
			status = domain.StatusNoShow
		}
		if _, err := svc.SetAttendance(ctx, admin, res.Registration.ID, status); err != nil {
			return err
		}
	}
	svc.Now = realNow
	if err := svc.SetCosts(ctx, admin, closed.ID, 270000, 120000, 0); err != nil {
		return err
	}
	if err := svc.CloseSession(ctx, admin, closed.ID); err != nil {
		return err
	}

	next := service.NextTemplateDate(1, time.Now().AddDate(0, 0, 1), loc)
	in, err = svc.TemplateSessionInput(ctx, tpl.ID, next)
	if err != nil {
		return err
	}
	in.Publish = true
	upcoming, err := svc.CreateSession(ctx, admin, in)
	if err != nil {
		return err
	}
	for i, n := range names {
		var res service.RegisterResult
		if i < 4 {
			u := members[i]
			res, err = svc.Register(ctx, service.MemberActor(u.ID), upcoming.ID, service.RegisterInput{UserID: &u.ID, Name: u.Name, Phone: u.Phone, Level: u.Level})
		} else {
			res, err = svc.Register(ctx, service.GuestActor(), upcoming.ID, service.RegisterInput{Name: n, Phone: fmt.Sprintf("0813000020%02d", i), Level: levels[i%len(levels)]})
		}
		if err != nil {
			return err
		}
		if i%3 == 0 {
			if err := svc.MarkPaidManually(ctx, admin, res.Registration.ID); err != nil {
				return err
			}
		}
	}

	in2, err := svc.TemplateSessionInput(ctx, tpl.ID, next.AddDate(0, 0, 7))
	if err != nil {
		return err
	}
	in2.Publish = true
	if _, err := svc.CreateSession(ctx, admin, in2); err != nil {
		return err
	}
	_, err = svc.Q.CreateAnnouncement(ctx, sqlc.CreateAnnouncementParams{
		Message: "Sesi Senin minggu depan sudah dibuka. Yang lunas duluan diprioritaskan!", LinkUrl: "/sessions/" + upcoming.Slug, Style: "info", IsActive: true,
	})
	return err
}

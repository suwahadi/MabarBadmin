package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/locale"
	"github.com/suwahadi/mabarbadmin/internal/media"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

type Picture struct {
	Src    string
	SrcSet string
	Width  int
	Height int
	Alt    string
}

func pictureOf(m sqlc.Medium) *Picture {
	vs := media.Variants(m)
	if len(vs) == 0 {
		return nil
	}
	var set []string
	for _, v := range vs {
		set = append(set, fmt.Sprintf("/media/%s %dw", v.File, v.Width))
	}
	largest := vs[len(vs)-1]
	return &Picture{Src: "/media/" + largest.File, SrcSet: strings.Join(set, ", "), Width: largest.Width, Height: largest.Height, Alt: m.AltText}
}

// smallest returns the URL of the narrowest variant at least minWidth wide.
func variantURL(m sqlc.Medium, minWidth int) string {
	vs := media.Variants(m)
	for _, v := range vs {
		if v.Width >= minWidth {
			return "/media/" + v.File
		}
	}
	if len(vs) > 0 {
		return "/media/" + vs[len(vs)-1].File
	}
	return ""
}

type SocialLink struct {
	Name string
	URL  string
	Icon string
}

type FooterLink struct {
	Slug  string
	Title string
}

type Site struct {
	Name          string
	Tagline       string
	BaseURL       string
	SiteURL       string
	LogoLight     *Picture
	LogoDark      *Picture
	Favicon32     string
	Favicon180    string
	ContactWA     string
	ContactEmail  string
	Address       string
	MapsURL       string
	Social        []SocialLink
	FooterPages   []FooterLink
	TitlePattern  string
	Description   string
	Keywords      string
	OGImage       string
	GoogleVerify  string
	AllowIndexing bool
	Leaderboard   bool
	BankName      string
	BankAccount   string
	BankHolder    string
	QRIS          *Picture
	PointRule     string
	PerCourt      int
}

// site builds the identity shared by every template; it is cached until an admin change.
func (a *App) site(ctx context.Context) *Site {
	a.cacheMu.Lock()
	cached := a.siteCache
	a.cacheMu.Unlock()
	if cached != nil {
		return cached
	}
	v := a.settings(ctx)
	s := &Site{
		Name:          v.String(settings.AppName),
		Tagline:       v.String(settings.Tagline),
		BaseURL:       a.Cfg.BaseURL,
		SiteURL:       strings.TrimRight(v.String(settings.SiteURL), "/"),
		ContactWA:     v.String(settings.ContactWhatsApp),
		ContactEmail:  v.String(settings.ContactEmail),
		Address:       v.String(settings.ContactAddress),
		MapsURL:       v.String(settings.ContactMapsURL),
		TitlePattern:  v.String(settings.SEOTitlePattern),
		Description:   v.String(settings.SEODescription),
		Keywords:      v.String(settings.SEOKeywords),
		GoogleVerify:  v.String(settings.SEOGoogleVerify),
		AllowIndexing: v.Bool(settings.SEOAllowIndexing),
		Leaderboard:   v.Bool(settings.LeaderboardPublic),
		BankName:      v.String(settings.BankName),
		BankAccount:   v.String(settings.BankAccountNumber),
		BankHolder:    v.String(settings.BankAccountName),
		PointRule:     v.String(settings.PointRule),
		PerCourt:      int(v.Int(settings.CapacityPerCourt)),
	}
	if s.SiteURL == "" {
		s.SiteURL = a.Cfg.BaseURL
	}
	if p, err := locale.NormalizePhone(s.ContactWA); err == nil {
		s.ContactWA = p
	}
	for _, sl := range []struct{ key, name, icon string }{
		{settings.SocialInstagram, "Instagram", "instagram"},
		{settings.SocialTikTok, "TikTok", "tiktok"},
		{settings.SocialYouTube, "YouTube", "youtube"},
		{settings.SocialFacebook, "Facebook", "facebook"},
		{settings.SocialX, "X", "x-brand"},
		{settings.SocialAyo, "Ayo", "ayo"},
	} {
		if u := strings.TrimSpace(v.String(sl.key)); u != "" {
			s.Social = append(s.Social, SocialLink{Name: sl.name, URL: u, Icon: sl.icon})
		}
	}

	var ids []int64
	for _, k := range settings.MediaKeys() {
		if id := v.MediaID(k); id != nil {
			ids = append(ids, *id)
		}
	}
	byID := map[int64]sqlc.Medium{}
	if len(ids) > 0 {
		if rows, err := a.Q.GetMediaByIDs(ctx, ids); err == nil {
			for _, m := range rows {
				byID[m.ID] = m
			}
		}
	}
	get := func(key string) (sqlc.Medium, bool) {
		id := v.MediaID(key)
		if id == nil {
			return sqlc.Medium{}, false
		}
		m, ok := byID[*id]
		return m, ok
	}
	if m, ok := get(settings.LogoLightMediaID); ok {
		s.LogoLight = pictureOf(m)
	}
	if m, ok := get(settings.LogoDarkMediaID); ok {
		s.LogoDark = pictureOf(m)
	}
	if m, ok := get(settings.FaviconMediaID); ok {
		s.Favicon32 = variantURL(m, 32)
		s.Favicon180 = variantURL(m, 180)
	}
	if m, ok := get(settings.QRISMediaID); ok {
		s.QRIS = pictureOf(m)
	}
	if m, ok := get(settings.SEOOGMediaID); ok {
		s.OGImage = a.Cfg.BaseURL + variantURL(m, 1200)
	}
	if pages, err := a.Q.ListFooterPages(ctx); err == nil {
		for _, p := range pages {
			s.FooterPages = append(s.FooterPages, FooterLink{Slug: p.Slug, Title: p.Title})
		}
	}
	a.cacheMu.Lock()
	a.siteCache = s
	a.cacheMu.Unlock()
	return s
}

const dismissCookie = "dismissed_announcement"

// liveAnnouncement is rendered on the server (no CLS); a dismissed id stays hidden via cookie.
func (a *App) liveAnnouncement(r *http.Request) *sqlc.Announcement {
	ann, err := a.Q.GetLiveAnnouncement(r.Context())
	if err != nil {
		return nil
	}
	if c, err := r.Cookie(dismissCookie); err == nil && c.Value == fmt.Sprint(ann.ID) {
		return nil
	}
	return &ann
}

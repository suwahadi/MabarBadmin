package domain

import (
	"errors"
	"strings"
	"unicode"
)

// ReservedSlugs are root paths owned by system routes; static pages may not use them.
var ReservedSlugs = []string{
	"admin", "me", "sessions", "tickets", "login", "register", "logout", "leaderboard", "announcements",
	"media", "static", "healthz", "sitemap.xml", "robots.txt", "site.webmanifest", "favicon.ico",
}

var ErrInvalidSlug = errors.New("slug hanya boleh huruf kecil, angka, dan tanda hubung")
var ErrReservedSlug = errors.New("slug ini dipakai oleh sistem")

func Slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '/' || r == '.':
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func ValidatePageSlug(slug string) error {
	if slug == "" || Slugify(slug) != slug {
		return ErrInvalidSlug
	}
	for _, r := range ReservedSlugs {
		if r == slug {
			return ErrReservedSlug
		}
	}
	return nil
}

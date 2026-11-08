package service

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/suwahadi/mabarbadmin/internal/content"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

// Upload is an optional file from a form; a nil Reader means "keep the current image".
type Upload struct {
	Reader io.Reader
	Alt    string
}

func (u Upload) present() bool { return u.Reader != nil }

// withMedia runs fn in a transaction and removes the files of media rows fn deleted only after
// the commit succeeded, so a rollback never leaves an owner pointing at a missing file.
func (s *Service) withMedia(ctx context.Context, fn func(q *sqlc.Queries, drop func(id *int64) error) error) error {
	var files []string
	err := s.tx(ctx, func(q *sqlc.Queries) error {
		files = nil
		drop := func(id *int64) error {
			if id == nil {
				return nil
			}
			f, err := s.Media.Delete(ctx, q, *id)
			if err != nil {
				return err
			}
			files = append(files, f...)
			return nil
		}
		return fn(q, drop)
	})
	if err == nil {
		s.Media.Purge(files)
	}
	return err
}

func (s *Service) mediaUserErr(err error) error {
	if _, ok := IsUserError(err); ok {
		return err
	}
	msg := err.Error()
	if strings.Contains(msg, "ukuran") || strings.Contains(msg, "format") {
		return userErr(capitalize(msg) + ".")
	}
	return err
}

// SetSettingMedia replaces the image stored under a media setting key such as the logo.
func (s *Service) SetSettingMedia(ctx context.Context, a Actor, key string, up Upload) error {
	def, ok := settings.Lookup(key)
	if !ok || def.Kind != settings.KindMedia {
		return userErr("Pengaturan gambar tidak dikenal.")
	}
	vals, err := s.Settings.Get(ctx)
	if err != nil {
		return err
	}
	old := vals.MediaID(key)
	err = s.withMedia(ctx, func(q *sqlc.Queries, drop func(*int64) error) error {
		var newID any
		if up.present() {
			m, err := s.Media.Save(ctx, q, up.Reader, def.Preset, up.Alt)
			if err != nil {
				return s.mediaUserErr(err)
			}
			newID = m.ID
		}
		if err := s.Settings.Save(ctx, q, map[string]any{key: newID}); err != nil {
			return err
		}
		if err := drop(old); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "setting", 0, "media_updated", map[string]any{"key": key})
	})
	s.Settings.Invalidate()
	return err
}

type HeroInput struct {
	Title, Subtitle, CTALabel, CTAURL string
	Active                            bool
	StartsAt, EndsAt                  *time.Time
	Image, MobileImage                Upload
	RemoveMobile                      bool
}

func (s *Service) checkActiveSlides(ctx context.Context, q *sqlc.Queries, selfID int64, active bool) error {
	if !active {
		return nil
	}
	n, err := q.CountActiveHeroSlides(ctx, selfID)
	if err != nil {
		return err
	}
	if n >= 5 {
		return userErr("Maksimal 5 slide aktif. Nonaktifkan slide lain dulu.")
	}
	return nil
}

func (s *Service) CreateHeroSlide(ctx context.Context, a Actor, in HeroInput) error {
	if !in.Image.present() {
		return userErr("Gambar desktop wajib diunggah.")
	}
	if strings.TrimSpace(in.Image.Alt) == "" {
		return userErr("Teks alt gambar wajib diisi.")
	}
	return s.withMedia(ctx, func(q *sqlc.Queries, _ func(*int64) error) error {
		if err := s.checkActiveSlides(ctx, q, 0, in.Active); err != nil {
			return err
		}
		img, err := s.Media.Save(ctx, q, in.Image.Reader, "hero", in.Image.Alt)
		if err != nil {
			return s.mediaUserErr(err)
		}
		var mobileID *int64
		if in.MobileImage.present() {
			m, err := s.Media.Save(ctx, q, in.MobileImage.Reader, "hero_mobile", in.Image.Alt)
			if err != nil {
				return s.mediaUserErr(err)
			}
			mobileID = &m.ID
		}
		slides, err := q.ListHeroSlides(ctx)
		if err != nil {
			return err
		}
		h, err := q.CreateHeroSlide(ctx, sqlc.CreateHeroSlideParams{
			ImageMediaID: img.ID, MobileImageMediaID: mobileID, Title: in.Title, Subtitle: in.Subtitle,
			CtaLabel: in.CTALabel, CtaUrl: in.CTAURL, SortOrder: int32(len(slides) + 1), IsActive: in.Active,
			StartsAt: in.StartsAt, EndsAt: in.EndsAt,
		})
		if err != nil {
			return err
		}
		return logActivity(ctx, q, a, "hero_slide", h.ID, "created", nil)
	})
}

func (s *Service) UpdateHeroSlide(ctx context.Context, a Actor, id int64, in HeroInput) error {
	return s.withMedia(ctx, func(q *sqlc.Queries, drop func(*int64) error) error {
		cur, err := q.GetHeroSlide(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if err := s.checkActiveSlides(ctx, q, id, in.Active); err != nil {
			return err
		}
		imageID, mobileID := cur.ImageMediaID, cur.MobileImageMediaID
		var dropIDs []*int64
		if in.Image.present() {
			if strings.TrimSpace(in.Image.Alt) == "" {
				return userErr("Teks alt gambar wajib diisi.")
			}
			m, err := s.Media.Save(ctx, q, in.Image.Reader, "hero", in.Image.Alt)
			if err != nil {
				return s.mediaUserErr(err)
			}
			old := cur.ImageMediaID
			dropIDs = append(dropIDs, &old)
			imageID = m.ID
		} else if strings.TrimSpace(in.Image.Alt) != "" {
			if err := q.UpdateMediaAlt(ctx, sqlc.UpdateMediaAltParams{ID: imageID, AltText: strings.TrimSpace(in.Image.Alt)}); err != nil {
				return err
			}
		}
		if in.MobileImage.present() {
			m, err := s.Media.Save(ctx, q, in.MobileImage.Reader, "hero_mobile", in.Image.Alt)
			if err != nil {
				return s.mediaUserErr(err)
			}
			dropIDs = append(dropIDs, cur.MobileImageMediaID)
			mobileID = &m.ID
		} else if in.RemoveMobile {
			dropIDs = append(dropIDs, cur.MobileImageMediaID)
			mobileID = nil
		}
		if err := q.UpdateHeroSlide(ctx, sqlc.UpdateHeroSlideParams{
			ID: id, ImageMediaID: imageID, MobileImageMediaID: mobileID, Title: in.Title, Subtitle: in.Subtitle,
			CtaLabel: in.CTALabel, CtaUrl: in.CTAURL, IsActive: in.Active, StartsAt: in.StartsAt, EndsAt: in.EndsAt,
		}); err != nil {
			return err
		}
		for _, d := range dropIDs {
			if err := drop(d); err != nil {
				return err
			}
		}
		return logActivity(ctx, q, a, "hero_slide", id, "updated", nil)
	})
}

func (s *Service) DeleteHeroSlide(ctx context.Context, a Actor, id int64) error {
	return s.withMedia(ctx, func(q *sqlc.Queries, drop func(*int64) error) error {
		cur, err := q.GetHeroSlide(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if err := q.DeleteHeroSlide(ctx, id); err != nil {
			return err
		}
		img := cur.ImageMediaID
		if err := drop(&img); err != nil {
			return err
		}
		if err := drop(cur.MobileImageMediaID); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "hero_slide", id, "deleted", nil)
	})
}

type PageInput struct {
	Slug, Title, Body                 string
	Published, Footer                 bool
	SortOrder                         int32
	MetaTitle, MetaDesc, MetaKeywords string
	Cover, OGImage                    Upload
	RemoveCover, RemoveOG             bool
}

func (in *PageInput) validate() error {
	in.Title = strings.TrimSpace(in.Title)
	in.Slug = strings.TrimSpace(in.Slug)
	if in.Slug == "" {
		in.Slug = domain.Slugify(in.Title)
	}
	if in.Title == "" {
		return userErr("Judul halaman wajib diisi.")
	}
	if err := domain.ValidatePageSlug(in.Slug); err != nil {
		return userErr(capitalize(err.Error()) + ".")
	}
	if in.Cover.present() && strings.TrimSpace(in.Cover.Alt) == "" {
		return userErr("Teks alt gambar sampul wajib diisi.")
	}
	return nil
}

func (s *Service) SavePage(ctx context.Context, a Actor, id int64, in PageInput) (sqlc.Page, error) {
	if err := in.validate(); err != nil {
		return sqlc.Page{}, err
	}
	var out sqlc.Page
	err := s.withMedia(ctx, func(q *sqlc.Queries, drop func(*int64) error) error {
		exists, err := q.PageSlugExists(ctx, sqlc.PageSlugExistsParams{Slug: in.Slug, ID: id})
		if err != nil {
			return err
		}
		if exists {
			return userErr("Slug sudah dipakai halaman lain.")
		}
		var cur sqlc.Page
		if id > 0 {
			cur, err = q.GetPage(ctx, id)
			if err != nil {
				return notFound(err)
			}
			if content.IsDefaultPage(cur.Slug) && in.Slug != cur.Slug {
				return userErr("Slug halaman bawaan tidak bisa diubah.")
			}
		}
		coverID, ogID := cur.CoverMediaID, cur.OgMediaID
		var dropIDs []*int64
		if in.Cover.present() {
			m, err := s.Media.Save(ctx, q, in.Cover.Reader, "page_cover", in.Cover.Alt)
			if err != nil {
				return s.mediaUserErr(err)
			}
			dropIDs = append(dropIDs, coverID)
			coverID = &m.ID
		} else if in.RemoveCover {
			dropIDs = append(dropIDs, coverID)
			coverID = nil
		}
		if in.OGImage.present() {
			m, err := s.Media.Save(ctx, q, in.OGImage.Reader, "og_image", in.Title)
			if err != nil {
				return s.mediaUserErr(err)
			}
			dropIDs = append(dropIDs, ogID)
			ogID = &m.ID
		} else if in.RemoveOG {
			dropIDs = append(dropIDs, ogID)
			ogID = nil
		}
		if id == 0 {
			out, err = q.CreatePage(ctx, sqlc.CreatePageParams{
				Slug: in.Slug, Title: in.Title, BodyMarkdown: in.Body, CoverMediaID: coverID, IsPublished: in.Published,
				ShowInFooter: in.Footer, SortOrder: in.SortOrder, MetaTitle: in.MetaTitle, MetaDescription: in.MetaDesc,
				MetaKeywords: in.MetaKeywords, OgMediaID: ogID,
			})
		} else {
			out, err = q.UpdatePage(ctx, sqlc.UpdatePageParams{
				ID: id, Slug: in.Slug, Title: in.Title, BodyMarkdown: in.Body, CoverMediaID: coverID, IsPublished: in.Published,
				ShowInFooter: in.Footer, SortOrder: in.SortOrder, MetaTitle: in.MetaTitle, MetaDescription: in.MetaDesc,
				MetaKeywords: in.MetaKeywords, OgMediaID: ogID,
			})
		}
		if err != nil {
			return err
		}
		if id > 0 && cur.Slug != in.Slug {
			if err := s.recordSlugChange(ctx, q, "page", cur.Slug, in.Slug); err != nil {
				return err
			}
		}
		for _, d := range dropIDs {
			if err := drop(d); err != nil {
				return err
			}
		}
		return logActivity(ctx, q, a, "page", out.ID, "saved", map[string]any{"slug": in.Slug})
	})
	return out, err
}

func (s *Service) DeletePage(ctx context.Context, a Actor, id int64) error {
	return s.withMedia(ctx, func(q *sqlc.Queries, drop func(*int64) error) error {
		cur, err := q.GetPage(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if content.IsDefaultPage(cur.Slug) {
			return userErr("Halaman bawaan tidak bisa dihapus.")
		}
		if err := q.DeletePage(ctx, id); err != nil {
			return err
		}
		if err := drop(cur.CoverMediaID); err != nil {
			return err
		}
		if err := drop(cur.OgMediaID); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "page", id, "deleted", map[string]any{"slug": cur.Slug})
	})
}

// PurgeExpiredProofs deletes payment proof images past the retention period while keeping the
// payment records themselves.
func (s *Service) PurgeExpiredProofs(ctx context.Context, before time.Time) (int, error) {
	rows, err := s.Q.ListExpiredProofMedia(ctx, before)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		err := s.withMedia(ctx, func(q *sqlc.Queries, drop func(*int64) error) error {
			if err := q.ClearProofMedia(ctx, r.ID); err != nil {
				return err
			}
			return drop(r.MediaID)
		})
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

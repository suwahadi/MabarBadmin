package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/suwahadi/mabarbadmin/internal/content"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
	"github.com/suwahadi/mabarbadmin/internal/service"
	"github.com/suwahadi/mabarbadmin/internal/settings"
)

func (a *App) adminAnnouncements(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListAnnouncements(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Pengumuman", "more")
	v.Data["Announcements"] = rows
	a.render(w, r, http.StatusOK, "admin/announcements", v)
}

func validURL(s string) bool {
	if s == "" {
		return true
	}
	if strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") {
		return true
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

func (a *App) adminAnnouncementSave(w http.ResponseWriter, r *http.Request) {
	msg := strings.TrimSpace(r.FormValue("message"))
	link := strings.TrimSpace(r.FormValue("link_url"))
	style := r.FormValue("style")
	switch {
	case msg == "" || utf8.RuneCountInString(msg) > 160:
		setFlash(w, "error", "Teks pengumuman wajib diisi, maksimal 160 karakter.")
	case !validURL(link):
		setFlash(w, "error", "Link harus diawali https:// atau /.")
	default:
		if style != "important" {
			style = "info"
		}
		var err error
		start, end, active := a.formTimePtr(r, "starts_at"), a.formTimePtr(r, "ends_at"), formBool(r, "is_active")
		if id := pathID(r); id > 0 {
			err = a.Q.UpdateAnnouncement(r.Context(), sqlc.UpdateAnnouncementParams{ID: id, Message: msg, LinkUrl: link, Style: style, StartsAt: start, EndsAt: end, IsActive: active})
		} else {
			_, err = a.Q.CreateAnnouncement(r.Context(), sqlc.CreateAnnouncementParams{Message: msg, LinkUrl: link, Style: style, StartsAt: start, EndsAt: end, IsActive: active})
		}
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		a.done(w, r, "/admin/announcements", "Pengumuman disimpan.")
		return
	}
	a.redirect(w, r, "/admin/announcements")
}

func (a *App) adminAnnouncementDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.Q.DeleteAnnouncement(r.Context(), pathID(r)); err != nil {
		a.serverError(w, r, err)
		return
	}
	a.done(w, r, "/admin/announcements", "Pengumuman dihapus.")
}

func (a *App) adminMembers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	rows, err := a.Q.ListMembers(r.Context(), q)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Member", "members")
	v.Data["Members"] = rows
	v.Data["Query"] = q
	a.render(w, r, http.StatusOK, "admin/members", v)
}

func (a *App) adminMemberSave(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == currentUser(r).ID && !formBool(r, "is_active") {
		setFlash(w, "error", "Kamu tidak bisa menonaktifkan akunmu sendiri.")
		a.redirect(w, r, "/admin/members")
		return
	}
	if err := a.Svc.AdminUpdateUser(r.Context(), a.adminActor(r), id, r.FormValue("name"), r.FormValue("level"), formBool(r, "is_active")); err != nil {
		a.fail(w, r, "/admin/members", err)
		return
	}
	a.done(w, r, "/admin/members", "Data member disimpan.")
}

func (a *App) adminMemberReset(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	u, err := a.Q.GetUser(r.Context(), id)
	if err != nil {
		a.notFound(w, r)
		return
	}
	temp, err := a.Svc.ResetPassword(r.Context(), a.adminActor(r), id)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Reset Password", "members")
	v.Data["Member"] = u
	v.Data["Temp"] = temp
	a.render(w, r, http.StatusOK, "admin/member_reset", v)
}

type settingField struct {
	settings.Def
	Value   string
	Bool    bool
	Picture *Picture
}

func (a *App) adminSettingsIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/settings/identity", http.StatusSeeOther)
}

func (a *App) adminSettings(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	label, ok := settings.GroupLabel(group)
	if !ok {
		a.notFound(w, r)
		return
	}
	vals := a.settings(r.Context())
	var fields []settingField
	for _, d := range settings.GroupDefs(group) {
		f := settingField{Def: d}
		switch d.Kind {
		case settings.KindBool:
			f.Bool = vals.Bool(d.Key)
		case settings.KindInt:
			f.Value = strconv.FormatInt(vals.Int(d.Key), 10)
		case settings.KindMedia:
			if id := vals.MediaID(d.Key); id != nil {
				if m, err := a.Q.GetMedia(r.Context(), *id); err == nil {
					f.Picture = pictureOf(m)
				}
			}
		default:
			f.Value = vals.String(d.Key)
		}
		fields = append(fields, f)
	}
	v := a.adminView(w, r, "Pengaturan · "+label, "more")
	v.Data["Group"] = group
	v.Data["Groups"] = settings.Groups
	v.Data["Fields"] = fields
	a.render(w, r, http.StatusOK, "admin/settings", v)
}

func (a *App) adminSettingsSave(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	if _, ok := settings.GroupLabel(group); !ok {
		a.notFound(w, r)
		return
	}
	back := "/admin/settings/" + group
	ctx := r.Context()
	values := map[string]any{}
	for _, d := range settings.GroupDefs(group) {
		raw := strings.TrimSpace(r.FormValue(d.Key))
		switch d.Kind {
		case settings.KindBool:
			values[d.Key] = formBool(r, d.Key)
		case settings.KindInt:
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || n < d.Min || (d.Max > 0 && n > d.Max) {
				setFlash(w, "error", fmt.Sprintf("%s harus angka %d sampai %d.", d.Label, d.Min, d.Max))
				a.redirect(w, r, back)
				return
			}
			values[d.Key] = n
		case settings.KindURL:
			if !validURL(raw) {
				setFlash(w, "error", d.Label+" harus berupa URL lengkap (https://...).")
				a.redirect(w, r, back)
				return
			}
			values[d.Key] = raw
		case settings.KindZone:
			if _, err := time.LoadLocation(raw); err != nil {
				setFlash(w, "error", "Zona waktu tidak dikenal.")
				a.redirect(w, r, back)
				return
			}
			values[d.Key] = raw
		case settings.KindMedia:
			up, closeFile := formFile(r, d.Key)
			up.Alt = strings.TrimSpace(r.FormValue(d.Key + "_alt"))
			var err error
			switch {
			case up.Reader != nil:
				err = a.Svc.SetSettingMedia(ctx, a.adminActor(r), d.Key, up)
			case formBool(r, "remove_"+d.Key):
				err = a.Svc.SetSettingMedia(ctx, a.adminActor(r), d.Key, service.Upload{})
			}
			closeFile()
			if err != nil {
				a.fail(w, r, back, err)
				return
			}
		case settings.KindText:
			values[d.Key] = strings.ReplaceAll(r.FormValue(d.Key), "\r\n", "\n")
		default:
			values[d.Key] = raw
		}
	}
	if len(values) > 0 {
		if err := a.Settings.Save(ctx, a.Q, values); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	a.Invalidate()
	a.done(w, r, back, "Pengaturan disimpan.")
}

func (a *App) adminStorage(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.MediaUsageByPreset(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	var total int64
	for _, row := range rows {
		total += row.TotalBytes
	}
	v := a.adminView(w, r, "Pemakaian Penyimpanan", "more")
	v.Data["Rows"] = rows
	v.Data["Total"] = total
	v.Data["Groups"] = settings.Groups
	v.Data["Group"] = "storage"
	a.render(w, r, http.StatusOK, "admin/storage", v)
}

func (a *App) adminPages(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListPages(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "Halaman Statis", "more")
	v.Data["Pages"] = rows
	a.render(w, r, http.StatusOK, "admin/pages", v)
}

func (a *App) adminPageForm(w http.ResponseWriter, r *http.Request) {
	p := sqlc.Page{IsPublished: true, ShowInFooter: true}
	v := a.adminView(w, r, "Halaman Statis", "more")
	if id := pathID(r); id > 0 {
		var err error
		if p, err = a.Q.GetPage(r.Context(), id); err != nil {
			a.notFound(w, r)
			return
		}
		for key, mid := range map[string]*int64{"Cover": p.CoverMediaID, "OG": p.OgMediaID} {
			if mid != nil {
				if m, err := a.Q.GetMedia(r.Context(), *mid); err == nil {
					v.Data[key] = pictureOf(m)
				}
			}
		}
	}
	v.Data["P"] = p
	v.Data["Reserved"] = strings.Join(domain.ReservedSlugs, ",")
	v.Data["IsDefault"] = p.ID > 0 && content.IsDefaultPage(p.Slug)
	a.render(w, r, http.StatusOK, "admin/page_form", v)
}

func (a *App) adminPageSave(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	back := "/admin/pages/new"
	if id > 0 {
		back = fmt.Sprintf("/admin/pages/%d", id)
	}
	cover, c1 := formFile(r, "cover")
	defer c1()
	og, c2 := formFile(r, "og_image")
	defer c2()
	cover.Alt = r.FormValue("cover_alt")
	p, err := a.Svc.SavePage(r.Context(), a.adminActor(r), id, service.PageInput{
		Slug: r.FormValue("slug"), Title: r.FormValue("title"), Body: strings.ReplaceAll(r.FormValue("body_markdown"), "\r\n", "\n"),
		Published: formBool(r, "is_published"), Footer: formBool(r, "show_in_footer"), SortOrder: int32(formInt64(r, "sort_order")),
		MetaTitle: strings.TrimSpace(r.FormValue("meta_title")), MetaDesc: strings.TrimSpace(r.FormValue("meta_description")),
		MetaKeywords: strings.TrimSpace(r.FormValue("meta_keywords")), Cover: cover, OGImage: og,
		RemoveCover: formBool(r, "remove_cover"), RemoveOG: formBool(r, "remove_og"),
	})
	if err != nil {
		a.fail(w, r, back, err)
		return
	}
	a.Invalidate()
	a.done(w, r, fmt.Sprintf("/admin/pages/%d", p.ID), "Halaman disimpan.")
}

func (a *App) adminPageDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.Svc.DeletePage(r.Context(), a.adminActor(r), pathID(r)); err != nil {
		a.fail(w, r, "/admin/pages", err)
		return
	}
	a.Invalidate()
	a.done(w, r, "/admin/pages", "Halaman dihapus.")
}

func (a *App) adminPagePreview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(content.Render(r.FormValue("body_markdown"))))
}

func (a *App) adminFAQ(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListFAQ(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	v := a.adminView(w, r, "FAQ", "more")
	v.Data["Items"] = rows
	a.render(w, r, http.StatusOK, "admin/faq", v)
}

func (a *App) adminFAQSave(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.FormValue("question"))
	ans := strings.TrimSpace(strings.ReplaceAll(r.FormValue("answer_markdown"), "\r\n", "\n"))
	if q == "" || ans == "" {
		setFlash(w, "error", "Pertanyaan dan jawaban wajib diisi.")
		a.redirect(w, r, "/admin/faq")
		return
	}
	var err error
	if id := pathID(r); id > 0 {
		err = a.Q.UpdateFAQ(r.Context(), sqlc.UpdateFAQParams{ID: id, Question: q, AnswerMarkdown: ans, IsPublished: formBool(r, "is_published")})
	} else {
		var n int64
		if n, err = a.Q.CountFAQ(r.Context()); err == nil {
			_, err = a.Q.CreateFAQ(r.Context(), sqlc.CreateFAQParams{Question: q, AnswerMarkdown: ans, SortOrder: int32(n + 1), IsPublished: formBool(r, "is_published")})
		}
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.done(w, r, "/admin/faq", "FAQ disimpan.")
}

func (a *App) adminFAQDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.Q.DeleteFAQ(r.Context(), pathID(r)); err != nil {
		a.serverError(w, r, err)
		return
	}
	a.done(w, r, "/admin/faq", "FAQ dihapus.")
}

// moveInOrder returns ids with the target moved one step up or down.
func moveInOrder(ids []int64, target int64, dir string) []int64 {
	for i, id := range ids {
		if id != target {
			continue
		}
		j := i - 1
		if dir == "down" {
			j = i + 1
		}
		if j >= 0 && j < len(ids) {
			ids[i], ids[j] = ids[j], ids[i]
		}
		break
	}
	return ids
}

func (a *App) adminFAQReorder(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListFAQ(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	for i, id := range moveInOrder(ids, formInt64(r, "id"), r.FormValue("dir")) {
		if err := a.Q.SetFAQOrder(r.Context(), sqlc.SetFAQOrderParams{ID: id, SortOrder: int32(i + 1)}); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	a.redirect(w, r, "/admin/faq")
}

func (a *App) adminHeroSlides(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListHeroSlides(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	var ids []int64
	for _, s := range rows {
		ids = append(ids, s.ImageMediaID)
	}
	thumbs := map[int64]string{}
	if len(ids) > 0 {
		if ms, err := a.Q.GetMediaByIDs(r.Context(), ids); err == nil {
			for _, m := range ms {
				thumbs[m.ID] = variantURL(m, 480)
			}
		}
	}
	v := a.adminView(w, r, "Hero Slider", "more")
	v.Data["Slides"] = rows
	v.Data["Thumbs"] = thumbs
	a.render(w, r, http.StatusOK, "admin/hero", v)
}

func (a *App) adminHeroForm(w http.ResponseWriter, r *http.Request) {
	s := sqlc.HeroSlide{IsActive: true}
	v := a.adminView(w, r, "Hero Slider", "more")
	if id := pathID(r); id > 0 {
		var err error
		if s, err = a.Q.GetHeroSlide(r.Context(), id); err != nil {
			a.notFound(w, r)
			return
		}
		if m, err := a.Q.GetMedia(r.Context(), s.ImageMediaID); err == nil {
			v.Data["Desktop"] = pictureOf(m)
		}
		if s.MobileImageMediaID != nil {
			if m, err := a.Q.GetMedia(r.Context(), *s.MobileImageMediaID); err == nil {
				v.Data["Mobile"] = pictureOf(m)
			}
		}
	}
	v.Data["S"] = s
	a.render(w, r, http.StatusOK, "admin/hero_form", v)
}

func (a *App) adminHeroSave(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	back := "/admin/hero-slides/new"
	if id > 0 {
		back = fmt.Sprintf("/admin/hero-slides/%d", id)
	}
	ctaURL := strings.TrimSpace(r.FormValue("cta_url"))
	if !validURL(ctaURL) {
		setFlash(w, "error", "URL tombol harus diawali https:// atau /.")
		a.redirect(w, r, back)
		return
	}
	img, c1 := formFile(r, "image")
	defer c1()
	mob, c2 := formFile(r, "mobile_image")
	defer c2()
	img.Alt = r.FormValue("alt_text")
	in := service.HeroInput{
		Title: strings.TrimSpace(r.FormValue("title")), Subtitle: strings.TrimSpace(r.FormValue("subtitle")),
		CTALabel: strings.TrimSpace(r.FormValue("cta_label")), CTAURL: ctaURL, Active: formBool(r, "is_active"),
		StartsAt: a.formTimePtr(r, "starts_at"), EndsAt: a.formTimePtr(r, "ends_at"), Image: img, MobileImage: mob,
		RemoveMobile: formBool(r, "remove_mobile"),
	}
	var err error
	if id > 0 {
		err = a.Svc.UpdateHeroSlide(r.Context(), a.adminActor(r), id, in)
	} else {
		err = a.Svc.CreateHeroSlide(r.Context(), a.adminActor(r), in)
	}
	if err != nil {
		a.fail(w, r, back, err)
		return
	}
	a.done(w, r, "/admin/hero-slides", "Slide disimpan.")
}

func (a *App) adminHeroDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.Svc.DeleteHeroSlide(r.Context(), a.adminActor(r), pathID(r)); err != nil {
		a.fail(w, r, "/admin/hero-slides", err)
		return
	}
	a.done(w, r, "/admin/hero-slides", "Slide dan semua variannya dihapus.")
}

func (a *App) adminHeroReorder(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Q.ListHeroSlides(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	for i, id := range moveInOrder(ids, formInt64(r, "id"), r.FormValue("dir")) {
		if err := a.Q.SetHeroSlideOrder(r.Context(), sqlc.SetHeroSlideOrderParams{ID: id, SortOrder: int32(i + 1)}); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	a.redirect(w, r, "/admin/hero-slides")
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

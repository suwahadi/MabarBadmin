-- name: ListPages :many
SELECT * FROM pages ORDER BY sort_order, title;

-- name: ListFooterPages :many
SELECT slug, title FROM pages WHERE is_published AND show_in_footer ORDER BY sort_order, title;

-- name: ListPublishedPages :many
SELECT slug, updated_at FROM pages WHERE is_published ORDER BY sort_order;

-- name: GetPage :one
SELECT * FROM pages WHERE id = $1;

-- name: GetPageBySlug :one
SELECT * FROM pages WHERE slug = $1;

-- name: PageSlugExists :one
SELECT EXISTS (SELECT 1 FROM pages WHERE slug = $1 AND id <> $2);

-- name: CreatePage :one
INSERT INTO pages (slug, title, body_markdown, cover_media_id, is_published, show_in_footer, sort_order,
                   meta_title, meta_description, meta_keywords, og_media_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: InsertPageIfMissing :exec
INSERT INTO pages (slug, title, body_markdown, is_published, show_in_footer, sort_order)
VALUES ($1, $2, $3, true, true, $4)
ON CONFLICT (slug) DO NOTHING;

-- name: UpdatePage :one
UPDATE pages
SET slug = $2, title = $3, body_markdown = $4, cover_media_id = $5, is_published = $6, show_in_footer = $7,
    sort_order = $8, meta_title = $9, meta_description = $10, meta_keywords = $11, og_media_id = $12,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeletePage :exec
DELETE FROM pages WHERE id = $1;

-- name: ListFAQ :many
SELECT * FROM faq_items ORDER BY sort_order, id;

-- name: ListPublishedFAQ :many
SELECT * FROM faq_items WHERE is_published ORDER BY sort_order, id;

-- name: CountFAQ :one
SELECT count(*) FROM faq_items;

-- name: GetFAQ :one
SELECT * FROM faq_items WHERE id = $1;

-- name: CreateFAQ :one
INSERT INTO faq_items (question, answer_markdown, sort_order, is_published)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateFAQ :exec
UPDATE faq_items SET question = $2, answer_markdown = $3, is_published = $4 WHERE id = $1;

-- name: SetFAQOrder :exec
UPDATE faq_items SET sort_order = $2 WHERE id = $1;

-- name: DeleteFAQ :exec
DELETE FROM faq_items WHERE id = $1;

-- name: ListHeroSlides :many
SELECT * FROM hero_slides ORDER BY sort_order, id;

-- name: ListLiveHeroSlides :many
SELECT * FROM hero_slides
WHERE is_active AND (starts_at IS NULL OR starts_at <= now()) AND (ends_at IS NULL OR ends_at > now())
ORDER BY sort_order, id
LIMIT 5;

-- name: CountActiveHeroSlides :one
SELECT count(*) FROM hero_slides WHERE is_active AND id <> $1;

-- name: GetHeroSlide :one
SELECT * FROM hero_slides WHERE id = $1;

-- name: CreateHeroSlide :one
INSERT INTO hero_slides (image_media_id, mobile_image_media_id, title, subtitle, cta_label, cta_url, sort_order,
                         is_active, starts_at, ends_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateHeroSlide :exec
UPDATE hero_slides
SET image_media_id = $2, mobile_image_media_id = $3, title = $4, subtitle = $5, cta_label = $6, cta_url = $7,
    is_active = $8, starts_at = $9, ends_at = $10
WHERE id = $1;

-- name: SetHeroSlideOrder :exec
UPDATE hero_slides SET sort_order = $2 WHERE id = $1;

-- name: DeleteHeroSlide :exec
DELETE FROM hero_slides WHERE id = $1;

-- name: ListAnnouncements :many
SELECT * FROM announcements ORDER BY created_at DESC LIMIT 200;

-- name: ListPublicAnnouncements :many
SELECT * FROM announcements
WHERE is_active AND (starts_at IS NULL OR starts_at <= now())
ORDER BY created_at DESC LIMIT 50;

-- name: GetLiveAnnouncement :one
SELECT * FROM announcements
WHERE is_active AND (starts_at IS NULL OR starts_at <= now()) AND (ends_at IS NULL OR ends_at > now())
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetAnnouncement :one
SELECT * FROM announcements WHERE id = $1;

-- name: CreateAnnouncement :one
INSERT INTO announcements (message, link_url, style, starts_at, ends_at, is_active)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: UpdateAnnouncement :exec
UPDATE announcements SET message = $2, link_url = $3, style = $4, starts_at = $5, ends_at = $6, is_active = $7
WHERE id = $1;

-- name: DeleteAnnouncement :exec
DELETE FROM announcements WHERE id = $1;

-- name: CreateSlugRedirect :exec
INSERT INTO slug_redirects (entity_type, old_slug, new_slug) VALUES ($1, $2, $3)
ON CONFLICT (entity_type, old_slug) DO UPDATE SET new_slug = EXCLUDED.new_slug, created_at = now();

-- name: RepointSlugRedirects :exec
UPDATE slug_redirects SET new_slug = $3 WHERE entity_type = $1 AND new_slug = $2;

-- name: DeleteSlugRedirect :exec
DELETE FROM slug_redirects WHERE entity_type = $1 AND old_slug = $2;

-- name: GetSlugRedirect :one
SELECT new_slug FROM slug_redirects WHERE entity_type = $1 AND old_slug = $2;

-- name: CreateSession :one
INSERT INTO sessions (slug, title, template_id, venue_id, starts_at, ends_at, court_count, capacity, price,
                      unique_code_enabled, payment_deadline_at, refund_deadline_at, status, point_rule,
                      level_label, shuttlecock_included, notes, meta_title, meta_description)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
RETURNING *;

-- name: UpdateSession :one
UPDATE sessions
SET slug = $2, title = $3, venue_id = $4, starts_at = $5, ends_at = $6, court_count = $7, capacity = $8,
    price = $9, unique_code_enabled = $10, payment_deadline_at = $11, refund_deadline_at = $12,
    point_rule = $13, level_label = $14, shuttlecock_included = $15, notes = $16, meta_title = $17,
    meta_description = $18, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetSessionStatus :exec
UPDATE sessions
SET status = $2,
    closed_at = CASE WHEN $2 IN ('closed', 'cancelled') THEN now() ELSE closed_at END,
    updated_at = now()
WHERE id = $1;

-- name: UpdateSessionCosts :exec
UPDATE sessions SET court_cost = $2, shuttlecock_cost = $3, other_cost = $4, updated_at = now() WHERE id = $1;

-- name: GetSession :one
SELECT * FROM sessions WHERE id = $1;

-- name: GetSessionForUpdate :one
SELECT * FROM sessions WHERE id = $1 FOR UPDATE;

-- name: GetSessionBySlug :one
SELECT * FROM sessions WHERE slug = $1;

-- name: SessionSlugExists :one
SELECT EXISTS (SELECT 1 FROM sessions WHERE slug = $1 AND id <> $2);

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1 AND status = 'draft';

-- name: ListSessionsAdmin :many
SELECT s.*, v.name AS venue_name,
       (SELECT count(*) FROM registrations r WHERE r.session_id = s.id AND r.status IN ('registered', 'attended', 'no_show'))::bigint AS registered_count,
       (SELECT count(*) FROM registrations r WHERE r.session_id = s.id AND r.status = 'waitlisted')::bigint AS waitlisted_count,
       (SELECT count(*) FROM registrations r WHERE r.session_id = s.id AND r.status IN ('registered', 'attended', 'no_show') AND r.payment_status = 'paid')::bigint AS paid_count
FROM sessions s JOIN venues v ON v.id = s.venue_id
WHERE (sqlc.arg(status)::text = '' OR s.status = sqlc.arg(status)::text)
ORDER BY s.starts_at DESC
LIMIT 200;

-- name: ListUpcomingPublished :many
SELECT s.*, v.name AS venue_name, v.address AS venue_address, v.maps_url AS venue_maps_url,
       (SELECT count(*) FROM registrations r WHERE r.session_id = s.id AND r.status = 'registered')::bigint AS registered_count,
       (SELECT count(*) FROM registrations r WHERE r.session_id = s.id AND r.status = 'waitlisted')::bigint AS waitlisted_count
FROM sessions s JOIN venues v ON v.id = s.venue_id
WHERE s.status = 'published' AND s.ends_at > now()
ORDER BY s.starts_at
LIMIT sqlc.arg(max_rows);

-- name: ListUpcomingWithStats :many
SELECT s.*, v.name AS venue_name,
       (SELECT count(*) FROM registrations r WHERE r.session_id = s.id AND r.status IN ('registered', 'attended', 'no_show'))::bigint AS registered_count,
       (SELECT count(*) FROM registrations r WHERE r.session_id = s.id AND r.status IN ('registered', 'attended', 'no_show') AND r.payment_status = 'paid')::bigint AS paid_count,
       (SELECT count(*) FROM registrations r WHERE r.session_id = s.id AND r.status IN ('registered', 'attended', 'no_show') AND r.payment_status <> 'paid')::bigint AS unpaid_count
FROM sessions s JOIN venues v ON v.id = s.venue_id
WHERE s.status IN ('draft', 'published') AND s.ends_at > now()
ORDER BY s.starts_at
LIMIT 3;

-- name: ListSessionsToAutoClose :many
SELECT id FROM sessions
WHERE status = 'published' AND ends_at < sqlc.arg(before)::timestamptz;

-- name: ListSessionsPastPaymentDeadline :many
SELECT id FROM sessions
WHERE status = 'published' AND payment_deadline_at IS NOT NULL AND payment_deadline_at < now() AND starts_at > now();

-- name: ListSitemapSessions :many
SELECT slug, updated_at FROM sessions WHERE status = 'published' AND ends_at > now() ORDER BY starts_at;

-- name: ListClosedSessionsBefore :many
SELECT id FROM sessions WHERE status IN ('closed', 'cancelled') AND closed_at < sqlc.arg(before)::timestamptz;

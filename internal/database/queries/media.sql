-- name: CreateMedia :one
INSERT INTO media (preset, storage_key, width, height, bytes, variants, alt_text, is_private)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetMedia :one
SELECT * FROM media WHERE id = $1;

-- name: GetMediaByIDs :many
SELECT * FROM media WHERE id = ANY(sqlc.arg(ids)::bigint[]);

-- name: DeleteMedia :one
DELETE FROM media WHERE id = $1 RETURNING *;

-- name: UpdateMediaAlt :exec
UPDATE media SET alt_text = $2 WHERE id = $1;

-- name: ListAllMediaKeys :many
SELECT id, storage_key, variants, created_at FROM media;

-- name: ListOrphanMedia :many
SELECT m.* FROM media m
WHERE m.created_at < sqlc.arg(older_than)::timestamptz
  AND NOT EXISTS (SELECT 1 FROM pages p WHERE p.cover_media_id = m.id OR p.og_media_id = m.id)
  AND NOT EXISTS (SELECT 1 FROM hero_slides h WHERE h.image_media_id = m.id OR h.mobile_image_media_id = m.id)
  AND NOT EXISTS (SELECT 1 FROM payment_proofs pp WHERE pp.media_id = m.id)
  AND NOT EXISTS (SELECT 1 FROM settings s WHERE jsonb_typeof(s.value) = 'number' AND s.key LIKE '%media_id' AND (s.value)::text = m.id::text);

-- name: MediaUsageByPreset :many
SELECT preset, count(*)::bigint AS files, coalesce(sum(bytes), 0)::bigint AS total_bytes
FROM media GROUP BY preset ORDER BY preset;
